package agent

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-kratos/blades"
)

// unmarkModelCooldownForTest 是测试专用的冷却表清理手段：每个用例用唯一模型名
// 隔离全局冷却表（MarkModelPermanentFailure 是进程级状态），用例结束时删掉
// 本用例写入的条目，避免污染其它测试。不改任何生产语义。
func unmarkModelCooldownForTest(t *testing.T, names ...string) {
	t.Helper()
	t.Cleanup(func() {
		globalDeadModelMu.Lock()
		defer globalDeadModelMu.Unlock()
		for _, n := range names {
			delete(globalDeadModelCooldown, n)
			delete(globalDeadModelCooldown, CleanFallbackModelName(n))
		}
	})
}

func TestPromoteCooldownPrimary(t *testing.T) {
	newProviders := func() (blades.ModelProvider, blades.ModelProvider, blades.ModelProvider) {
		primary := NewMockModelProvider("primary-mock", "primary-ok")
		fb1 := NewMockModelProvider("fb1-mock", "fb1-ok")
		fb2 := NewMockModelProvider("fb2-mock", "fb2-ok")
		return primary, fb1, fb2
	}

	t.Run("主模型冷却+有备用→首个备用提升为主模型，其余顺延", func(t *testing.T) {
		primaryName := fmt.Sprintf("d-test-primary-%d", time.Now().UnixNano())
		fbName1 := fmt.Sprintf("d-test-fb1-%d(https://api.example.com/v1)", time.Now().UnixNano())
		fbName2 := fmt.Sprintf("d-test-fb2-%d(https://api.example.com/v1)", time.Now().UnixNano())
		unmarkModelCooldownForTest(t, primaryName)
		MarkModelPermanentFailure(primaryName)
		if !IsModelInCooldown(primaryName) {
			t.Fatalf("前置条件失败：主模型 %q 应处于冷却期", primaryName)
		}

		primary, fb1, fb2 := newProviders()
		gotPrimary, gotName, gotFallbacks, gotNames := promoteCooldownPrimary(
			primary, primaryName,
			[]blades.ModelProvider{fb1, fb2}, []string{fbName1, fbName2},
		)
		if gotPrimary != fb1 {
			t.Errorf("提升后的主模型不是首个备用")
		}
		if gotName != fbName1 {
			t.Errorf("提升后的主模型名 = %q; want %q", gotName, fbName1)
		}
		if len(gotFallbacks) != 1 || gotFallbacks[0] != fb2 {
			t.Errorf("剩余备用应只剩第二个备用，got %d 个", len(gotFallbacks))
		}
		if len(gotNames) != 1 || gotNames[0] != fbName2 {
			t.Errorf("剩余备用名应只剩第二个，got %v", gotNames)
		}
		// 提升后的组合仍能正常报出被提升模型的名字
		composite := NewFallbackModelProvider(gotPrimary, gotName, gotFallbacks, gotNames)
		if composite.Name() != CleanFallbackModelName(fbName1) {
			t.Errorf("组合 Name() = %q; want %q", composite.Name(), CleanFallbackModelName(fbName1))
		}
	})

	t.Run("主模型冷却+只剩一个备用→提升后剩余链为空，直接返回被提升模型", func(t *testing.T) {
		primaryName := fmt.Sprintf("d-test-primary-%d", time.Now().UnixNano())
		fbName1 := fmt.Sprintf("d-test-fb1-%d(https://api.example.com/v1)", time.Now().UnixNano())
		unmarkModelCooldownForTest(t, primaryName)
		MarkModelPermanentFailure(primaryName)

		primary, fb1, _ := newProviders()
		gotPrimary, gotName, gotFallbacks, gotNames := promoteCooldownPrimary(
			primary, primaryName,
			[]blades.ModelProvider{fb1}, []string{fbName1},
		)
		if gotPrimary != fb1 || gotName != fbName1 {
			t.Errorf("应提升唯一备用为主模型，got name=%q", gotName)
		}
		if len(gotFallbacks) != 0 || len(gotNames) != 0 {
			t.Errorf("剩余备用链应为空，got %d providers / %d names", len(gotFallbacks), len(gotNames))
		}
		// NewFallbackModelProvider 在 fallbacks 为空时直接返回 primary（即被提升的备用）
		if wrapped := NewFallbackModelProvider(gotPrimary, gotName, gotFallbacks, gotNames); wrapped != fb1 {
			t.Errorf("单备用提升后 NewFallbackModelProvider 应直接返回被提升模型本身")
		}
	})

	t.Run("主模型冷却+无备用→仍用主模型（保持现状）", func(t *testing.T) {
		primaryName := fmt.Sprintf("d-test-primary-%d", time.Now().UnixNano())
		unmarkModelCooldownForTest(t, primaryName)
		MarkModelPermanentFailure(primaryName)

		primary, _, _ := newProviders()
		gotPrimary, gotName, gotFallbacks, gotNames := promoteCooldownPrimary(
			primary, primaryName, nil, nil,
		)
		if gotPrimary != primary || gotName != primaryName {
			t.Errorf("无备用时应保持主模型不变")
		}
		if len(gotFallbacks) != 0 || len(gotNames) != 0 {
			t.Errorf("无备用时剩余链应为空")
		}
	})

	t.Run("主模型未冷却→行为不变", func(t *testing.T) {
		primaryName := fmt.Sprintf("d-test-primary-%d", time.Now().UnixNano())
		fbName1 := fmt.Sprintf("d-test-fb1-%d(https://api.example.com/v1)", time.Now().UnixNano())
		fbName2 := fmt.Sprintf("d-test-fb2-%d(https://api.example.com/v1)", time.Now().UnixNano())
		unmarkModelCooldownForTest(t, primaryName)
		if IsModelInCooldown(primaryName) {
			t.Fatalf("前置条件失败：主模型 %q 不应处于冷却期", primaryName)
		}

		primary, fb1, fb2 := newProviders()
		gotPrimary, gotName, gotFallbacks, gotNames := promoteCooldownPrimary(
			primary, primaryName,
			[]blades.ModelProvider{fb1, fb2}, []string{fbName1, fbName2},
		)
		if gotPrimary != primary || gotName != primaryName {
			t.Errorf("未冷却时应保持主模型不变")
		}
		if len(gotFallbacks) != 2 || len(gotNames) != 2 {
			t.Errorf("未冷却时备用链应原样保留，got %d providers / %d names", len(gotFallbacks), len(gotNames))
		}
	})
}
