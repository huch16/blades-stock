package agent

import (
	"context"
	"strings"
	"testing"
)

// serenityRetryHarness records every round the retry helper runs, so tests
// can assert which card each round used and that a failed first round's
// partial output never leaks into the final result.
type serenityRetryHarness struct {
	calls     []int
	rounds    []reportStreamResult
	pick      func(ctx context.Context, primaryID int) (int, string, bool)
	pickCalls []int
}

func (h *serenityRetryHarness) chatRound(ctx context.Context, prompt, sysPrompt string, aiConfigId int) reportStreamResult {
	h.calls = append(h.calls, aiConfigId)
	i := len(h.calls) - 1
	if i < len(h.rounds) {
		return h.rounds[i]
	}
	return reportStreamResult{FinishReason: "stop"}
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

	res := svc.runSerenityGeneration(context.Background(), "prompt", "sysPrompt", 3)

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

	res := svc.runSerenityGeneration(context.Background(), "prompt", "sysPrompt", 3)

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

	res := svc.runSerenityGeneration(context.Background(), "prompt", "sysPrompt", 3)

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

	res := svc.runSerenityGeneration(context.Background(), "prompt", "sysPrompt", 3)

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

	res := svc.runSerenityGeneration(context.Background(), "prompt", "sysPrompt", 3)

	if len(h.calls) != 1 {
		t.Fatalf("截断时应只跑一轮，实际 %d 轮", len(h.calls))
	}
	if !res.Truncated {
		t.Error("截断标记应原样保留给调用方处理")
	}
}
