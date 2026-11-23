package agent

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/go-kratos/blades"
)

// 第四轮 B 组（运行：并发、超时、重试、流式、泄漏）回归测试。
// 覆盖：IsPermanentAuthError 新增形态、冷却表过期清理、
// Generate 180s 硬上限的变量化与外层 ctx 优先。

func TestIsPermanentAuthError_Round4(t *testing.T) {
	cases := []struct {
		errStr   string
		expected bool
	}{
		// 新增：计费/支付类硬信号（即使伴随 429 也是永久，账单问题不会自愈）。
		{"402 Payment Required: billing limit reached", true},
		{"HTTP 402: payment required", true},
		{"429: insufficient balance, please top up", true},
		{"balance_insufficient: account out of funds", true},
		{"insufficient funds for this request", true},
		{"billing hard limit has been reached", true},
		{"billing issue: please update payment method", true},
		// 新增：账户停用类。
		{"account suspended: policy violation", true},
		{"account_suspended", true},
		{"account disabled by administrator", true},
		{"account_disabled", true},
		// 新增：鉴权文案变体。
		{"incorrect api key provided", true},
		{"incorrect_api_key", true},
		{"api key not valid, please check", true},
		{"authentication failed: bad credentials", true},
		{"auth failed for this request", true},
		{"token expired, please renew", true},
		{"api key expired", true},
		{"invalid credentials supplied", true},

		// 护栏：billing limit 是硬性计费上限（与 insufficient_quota 同级），
		// 即使伴随 429 / try again later 也按永久处理——账单上限不会
		// 随时间自愈；若某网关滥用该词写瞬时限流，需凭原始错误文本再校准
		//（与 insufficient_quota 的处理一致，见 INTELLIGENCE §7 第 2 条）。
		{"429 Too Many Requests: billing limit, try again later", true},
		// 护栏：billing 服务本身瞬时不可用不是账户欠费。
		{"billing service temporarily unavailable", false},
		// 护栏：既有非永久形态不受新标记牵连。
		{"connection reset by peer", false},
		{"503 Service Unavailable", false},
		{"model overloaded, try again later", false},
	}

	for _, c := range cases {
		if got := IsPermanentAuthError(errors.New(c.errStr)); got != c.expected {
			t.Errorf("IsPermanentAuthError(%q) = %v, want %v", c.errStr, got, c.expected)
		}
	}
}

// TestMarkModelPermanentFailure_PrunesExpired 守卫冷却表过期清理：
// MarkModelPermanentFailure 写入时顺带清掉已过期的条目，表不再只增不减。
func TestMarkModelPermanentFailure_PrunesExpired(t *testing.T) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	expiredName := "prune-expired-" + suffix
	liveName := "prune-live-" + suffix
	newName := "prune-new-" + suffix
	unmarkModelCooldownForTest(t, expiredName, liveName, newName)

	// 直接注入一条已过期与一条未过期的冷却记录（同包测试可访问全局表）。
	globalDeadModelMu.Lock()
	globalDeadModelCooldown[cooldownKeyFromName(expiredName)] = time.Now().Add(-time.Minute)
	globalDeadModelCooldown[cooldownKeyFromName(liveName)] = time.Now().Add(30 * time.Minute)
	globalDeadModelMu.Unlock()

	if IsModelInCooldown(expiredName) {
		t.Fatalf("过期条目本应已不在冷却期")
	}
	if !IsModelInCooldown(liveName) {
		t.Fatalf("未过期条目应处于冷却期")
	}

	MarkModelPermanentFailure(newName)

	globalDeadModelMu.RLock()
	_, expiredStillThere := globalDeadModelCooldown[cooldownKeyFromName(expiredName)]
	_, liveStillThere := globalDeadModelCooldown[cooldownKeyFromName(liveName)]
	_, newThere := globalDeadModelCooldown[cooldownKeyFromName(newName)]
	globalDeadModelMu.RUnlock()

	if expiredStillThere {
		t.Errorf("写入新冷却条目后，已过期条目应被清理")
	}
	if !liveStillThere {
		t.Errorf("写入新冷却条目后，未过期条目应保留")
	}
	if !newThere {
		t.Errorf("新标记的模型应进入冷却表")
	}
}

// TestFallbackGenerate_TimeoutCap 守卫 Generate 的单次尝试上限：
// (1) 外层无 deadline 时，fallbackGenerateTimeout（测试里收窄）掐断挂起的
// 主模型并走备用；(2) 外层 ctx 更紧（如 reflection 的 60s）时，外层
// deadline 优先，上限本身不生效——即 180s 目前 inert 的机制证明。
func TestFallbackGenerate_TimeoutCap(t *testing.T) {
	req := &blades.ModelRequest{
		Messages: []*blades.Message{blades.UserMessage("test")},
	}

	t.Run("cap fires when outer ctx has no deadline", func(t *testing.T) {
		old := fallbackGenerateTimeout
		fallbackGenerateTimeout = 50 * time.Millisecond
		defer func() { fallbackGenerateTimeout = old }()

		primary := NewStallStreamProvider("stall-primary")
		fallback := NewMockModelProvider("fallback-ok", "fallback answer")
		fb := NewFallbackModelProvider(primary, "stall-primary",
			[]blades.ModelProvider{fallback}, []string{"fallback-ok"})

		start := time.Now()
		resp, err := fb.Generate(context.Background(), req)
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("挂起主模型被上限掐断后应走备用成功，got err: %v", err)
		}
		if resp == nil || resp.Message == nil || resp.Message.Text() != "fallback answer" {
			t.Fatalf("应拿到备用模型答案，got %+v", resp)
		}
		if elapsed > 5*time.Second {
			t.Fatalf("上限 50ms 时 Generate 应在毫秒级返回，实际 %v", elapsed)
		}
		if primary.Calls() != 1 || fallback.Name() != "fallback-ok" {
			t.Fatalf("主模型应恰好被调 1 次，实际 %d", primary.Calls())
		}
	})

	t.Run("outer ctx deadline wins when tighter than cap", func(t *testing.T) {
		old := fallbackGenerateTimeout
		fallbackGenerateTimeout = 10 * time.Second
		defer func() { fallbackGenerateTimeout = old }()

		primary := NewStallStreamProvider("stall-primary")
		fallback := NewMockModelProvider("fallback-ok", "fallback answer")
		fb := NewFallbackModelProvider(primary, "stall-primary",
			[]blades.ModelProvider{fallback}, []string{"fallback-ok"})

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()
		start := time.Now()
		resp, err := fb.Generate(ctx, req)
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("外层 30ms 到期后应走备用成功，got err: %v", err)
		}
		if resp == nil || resp.Message == nil || resp.Message.Text() != "fallback answer" {
			t.Fatalf("应拿到备用模型答案，got %+v", resp)
		}
		// 外层 30ms 到期即应切换：若错误地等到 10s 上限才会返回，这里会超时失败。
		if elapsed > 5*time.Second {
			t.Fatalf("外层 deadline 更紧时 Generate 应在毫秒级返回，实际 %v", elapsed)
		}
	})
}
