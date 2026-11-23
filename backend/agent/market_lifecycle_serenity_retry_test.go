package agent

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// serenityRetryHarness records every round the retry helper runs, so tests
// can assert which card each round used and that a failed first round's
// partial output never leaks into the final result.
type serenityRetryHarness struct {
	calls     []int
	rounds    []reportStreamResult
	pick      func(ctx context.Context, primaryID int) (int, string, bool)
	pickCalls []int
	// deadlines 记录每轮拿到的 ctx 的 deadline，用于断言重试轮拿到的是
	// 全新预算而非首轮剩下的。
	deadlines []time.Time
	// round1Delay 在首轮开始时睡这么久（0 = 不睡），用于把两轮的
	// deadline 拉开到可观测的差距。
	round1Delay time.Duration
}

func (h *serenityRetryHarness) chatRound(ctx context.Context, prompt, sysPrompt string, aiConfigId int) reportStreamResult {
	h.calls = append(h.calls, aiConfigId)
	if dl, ok := ctx.Deadline(); ok {
		h.deadlines = append(h.deadlines, dl)
	}
	if len(h.calls) == 1 && h.round1Delay > 0 {
		time.Sleep(h.round1Delay)
	}
	i := len(h.calls) - 1
	if i < len(h.rounds) {
		return h.rounds[i]
	}
	return reportStreamResult{FinishReason: "stop"}
}

// testReportGenCtx 是重试测试用的无超时工厂：轮次由替身控制，不涉
// 及真实 deadline。
func testReportGenCtx() (context.Context, context.CancelFunc) {
	return context.Background(), func() {}
}

func (h *serenityRetryHarness) picker(ctx context.Context, primaryID int) (int, string, bool) {
	h.pickCalls = append(h.pickCalls, primaryID)
	if h.pick != nil {
		return h.pick(ctx, primaryID)
	}
	return 0, "", false
}

func newSerenityRetryService(h *serenityRetryHarness) *MarketLifecycleService {
	return &MarketLifecycleService{
		ctx:                    context.Background(),
		serenityChatRound:      h.chatRound,
		serenityFallbackPicker: h.picker,
	}
}

// TestSerenityRetry_PartialThenFallbackSuccess 是任务一的核心用例：
// 主模型出了半份正文后断流 → 换备用卡整轮重跑成功。最终结果必须是
// 备用模型的完整输出，且不含上一轮的半份内容（不拼接保证）。
func TestSerenityRetry_PartialThenFallbackSuccess(t *testing.T) {
	h := &serenityRetryHarness{
		rounds: []reportStreamResult{
			{
				Reasoning: "首轮思考……",
				Content:   "半份正文：产业链上游……",
				StreamErr: "❌ stream error: stream ID 1; INTERNAL_ERROR; received from peer",
			},
			{
				Reasoning:    "备用轮思考……",
				Content:      "完整研报正文：产业链全景拆解……",
				FinishReason: "stop",
			},
		},
		pick: func(ctx context.Context, primaryID int) (int, string, bool) {
			if primaryID != 3 {
				t.Errorf("picker 应以主卡片 ID=3 查询，实际 %d", primaryID)
			}
			// 选卡来自 fallback 链首个可用备用，不硬编码模型名；
			// 这里只约定测试替身返回的 ID 与名字。
			return 7, "fallback-card", true
		},
	}
	svc := newSerenityRetryService(h)

	res := svc.runReportGeneration(context.Background(), "prompt", "sysPrompt", 3, "Serenity", testReportGenCtx)

	if len(h.calls) != 2 {
		t.Fatalf("应恰好跑两轮（首轮 + 一次重试），实际 %d 轮：%v", len(h.calls), h.calls)
	}
	if h.calls[0] != 3 || h.calls[1] != 7 {
		t.Errorf("两轮应分别用主卡片 3 与备用卡片 7，实际 %v", h.calls)
	}
	if res.StreamErr != "" {
		t.Fatalf("重试成功后不应再有错误，实际 %q", res.StreamErr)
	}
	if res.Content != "完整研报正文：产业链全景拆解……" {
		t.Errorf("最终正文应是备用轮的完整输出，实际 %q", res.Content)
	}
	if strings.Contains(res.Content, "半份正文") {
		t.Error("最终结果混入了首轮的半份内容，违反不拼接保证")
	}
	if strings.Contains(res.Reasoning, "首轮思考") {
		t.Error("最终思考过程混入了首轮内容，违反不拼接保证")
	}
}

// TestSerenityRetry_BothRoundsFail 重试也失败时正常报错：返回第二轮
// 的错误（最新、最相关的失败原因），且只跑两轮、不无限循环。
func TestSerenityRetry_BothRoundsFail(t *testing.T) {
	h := &serenityRetryHarness{
		rounds: []reportStreamResult{
			{Content: "半份正文", StreamErr: "❌ 首轮流中断"},
			{Content: "", StreamErr: "❌ 备用轮超时"},
		},
		pick: func(ctx context.Context, primaryID int) (int, string, bool) {
			return 7, "fallback-card", true
		},
	}
	svc := newSerenityRetryService(h)

	res := svc.runReportGeneration(context.Background(), "prompt", "sysPrompt", 3, "Serenity", testReportGenCtx)

	if len(h.calls) != 2 {
		t.Fatalf("应恰好跑两轮，实际 %d 轮：%v", len(h.calls), h.calls)
	}
	if res.StreamErr != "❌ 备用轮超时" {
		t.Errorf("应返回第二轮的错误，实际 %q", res.StreamErr)
	}
}

// TestSerenityRetry_FirstRoundSuccess 首轮成功时绝不触发重试，也不
// 查询备用卡（避免无谓的注册表读取）。
func TestSerenityRetry_FirstRoundSuccess(t *testing.T) {
	h := &serenityRetryHarness{
		rounds: []reportStreamResult{
			{Content: "完整研报", FinishReason: "stop"},
		},
		pick: func(ctx context.Context, primaryID int) (int, string, bool) {
			t.Error("首轮成功时不应查询备用卡")
			return 0, "", false
		},
	}
	svc := newSerenityRetryService(h)

	res := svc.runReportGeneration(context.Background(), "prompt", "sysPrompt", 3, "Serenity", testReportGenCtx)

	if len(h.calls) != 1 {
		t.Fatalf("首轮成功应只跑一轮，实际 %d 轮", len(h.calls))
	}
	if res.Content != "完整研报" || res.StreamErr != "" {
		t.Errorf("结果应原样返回首轮输出，实际 %+v", res)
	}
}

// TestSerenityRetry_NoFallbackAvailable 首轮断流但无可用备用卡时，
// 直接返回首轮错误（不重试、不报错选卡失败）。
func TestSerenityRetry_NoFallbackAvailable(t *testing.T) {
	h := &serenityRetryHarness{
		rounds: []reportStreamResult{
			{Content: "半份正文", StreamErr: "❌ 首轮流中断"},
		},
		pick: func(ctx context.Context, primaryID int) (int, string, bool) {
			return 0, "", false
		},
	}
	svc := newSerenityRetryService(h)

	res := svc.runReportGeneration(context.Background(), "prompt", "sysPrompt", 3, "Serenity", testReportGenCtx)

	if len(h.calls) != 1 {
		t.Fatalf("无备用卡时应只跑一轮，实际 %d 轮", len(h.calls))
	}
	if res.StreamErr != "❌ 首轮流中断" {
		t.Errorf("应返回首轮错误，实际 %q", res.StreamErr)
	}
}

// TestSerenityRetry_TruncatedDoesNotRetry 截断不是流中断：换卡也大
// 概率在同样的 maxTokens 下再次截断，因此不重试。
func TestSerenityRetry_TruncatedDoesNotRetry(t *testing.T) {
	h := &serenityRetryHarness{
		rounds: []reportStreamResult{
			{Content: "半份正文", FinishReason: "length", Truncated: true},
		},
		pick: func(ctx context.Context, primaryID int) (int, string, bool) {
			t.Error("截断时不应查询备用卡")
			return 0, "", false
		},
	}
	svc := newSerenityRetryService(h)

	res := svc.runReportGeneration(context.Background(), "prompt", "sysPrompt", 3, "Serenity", testReportGenCtx)

	if len(h.calls) != 1 {
		t.Fatalf("截断时应只跑一轮，实际 %d 轮", len(h.calls))
	}
	if !res.Truncated {
		t.Error("截断标记应原样保留给调用方处理")
	}
}

// TestReportGeneration_RetryGetsFreshDeadline 回归测试：重试轮必须拿到
// 一个全新的 10 分钟 ctx，而不是首轮剩下的预算。旧实现两轮共用一个
// ctx，首轮烧掉 9 分钟才断流时重试只剩约 1 分钟，几乎必撞 deadline
// exceeded，重试只在首轮快速失败时才有效。
func TestReportGeneration_RetryGetsFreshDeadline(t *testing.T) {
	h := &serenityRetryHarness{
		rounds: []reportStreamResult{
			{Content: "半份正文", StreamErr: "❌ 首轮流中断"},
			{Content: "完整研报", FinishReason: "stop"},
		},
		pick: func(ctx context.Context, primaryID int) (int, string, bool) {
			return 7, "fallback-card", true
		},
		round1Delay: 150 * time.Millisecond,
	}
	svc := newSerenityRetryService(h)
	newGenCtx := func() (context.Context, context.CancelFunc) {
		return context.WithTimeout(context.Background(), 10*time.Minute)
	}

	res := svc.runReportGeneration(context.Background(), "prompt", "sysPrompt", 3, "Serenity", newGenCtx)

	if len(h.calls) != 2 {
		t.Fatalf("应恰好跑两轮，实际 %d 轮", len(h.calls))
	}
	if res.StreamErr != "" || res.Content != "完整研报" {
		t.Fatalf("重试结果异常: %+v", res)
	}
	if len(h.deadlines) != 2 {
		t.Fatalf("两轮都应拿到带 deadline 的 ctx，实际 %d 个 deadline", len(h.deadlines))
	}
	if !h.deadlines[1].After(h.deadlines[0]) {
		t.Errorf("重试轮 deadline 应晚于首轮（全新 10 分钟预算），首轮 %v，重试 %v", h.deadlines[0], h.deadlines[1])
	}
	if gap := h.deadlines[1].Sub(h.deadlines[0]); gap < 100*time.Millisecond {
		t.Errorf("重试轮 deadline 只比首轮晚 %v，疑似复用了首轮 ctx（首轮睡了 150ms）", gap)
	}
}

// TestReportGenerators_WiredToRunReportGeneration 源码级接线守卫：
// 四个研报生成器都必须走 runReportGeneration（换卡整轮重试一次），
// 且不得再直连 ChatWithContext + collectReportStream（旧写法会绕开
// 重试）。读本包源码断言，避免将来有人把某一链路改回直连。
func TestReportGenerators_WiredToRunReportGeneration(t *testing.T) {
	src, err := os.ReadFile("market_lifecycle.go")
	if err != nil {
		t.Fatalf("read market_lifecycle.go: %v", err)
	}
	text := string(src)

	if strings.Contains(text, "runSerenityGeneration") {
		t.Error("runSerenityGeneration 应已更名为 runReportGeneration，源码中仍有旧名")
	}
	for _, dead := range []string{"Skill 7", "Skill 4", "Serenity-Skill4"} {
		if strings.Contains(text, dead) {
			t.Errorf("技能数字标签 %q 应已改为按名引用，源码中仍有残留", dead)
		}
	}

	generators := []struct {
		fn    string
		label string
	}{
		{"GeneratePreMarketBriefing", "盘前晨报"},
		{"GeneratePostMarketReview", "盘后复盘"},
		{"GenerateETFRotationReport", "ETF轮动"},
		{"TriggerSectorAnomalySerenityAnalysis", "Serenity"},
	}
	for _, g := range generators {
		start := strings.Index(text, "func (s *MarketLifecycleService) "+g.fn+"(")
		if start < 0 {
			t.Fatalf("找不到生成器 %s", g.fn)
		}
		rest := text[start:]
		end := strings.Index(rest[1:], "\nfunc ")
		body := rest
		if end > 0 {
			body = rest[:end+1]
		}
		if !strings.Contains(body, "runReportGeneration(") {
			t.Errorf("%s 未调用 runReportGeneration（无整轮重试）", g.fn)
		}
		if !strings.Contains(body, `"`+g.label+`"`) {
			t.Errorf("%s 调用 runReportGeneration 时未传 %q 标签", g.fn, g.label)
		}
		if strings.Contains(body, "ChatWithContext(genCtx") || strings.Contains(body, "collectReportStream(ch)") {
			t.Errorf("%s 仍在直连 ChatWithContext + collectReportStream，会绕开重试", g.fn)
		}
	}
}
