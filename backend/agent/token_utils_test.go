package agent

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestEstimateModelContextWindow(t *testing.T) {
	cases := []struct {
		model string
		want  int
	}{
		{"gpt-4o", 128000},
		{"gpt-4o-mini", 128000},
		{"gpt-4.1", 1047576},
		{"claude-3-5-sonnet", 200000},
		{"claude-sonnet-4", 200000},
		{"gemini-2.5-pro", 1048576},
		{"gemini-2.0-flash", 1048576},
		{"minimax-m3", 1048576},
		{"minimax-m1", 200000},
		{"deepseek-chat", 128000},
		{"deepseek-v3", 128000},
		{"deepseek-reasoner", 64000},
		{"deepseek-r1", 64000},
		{"qwen2.5-72b", 128000},
		{"moonshot-v1-8k", 128000},
		{"mistral-large-latest", 128000},
		{"", 32768},
		{"some-unknown-model", 32768},
	}
	for _, c := range cases {
		if got := estimateModelContextWindow(c.model); got != c.want {
			t.Errorf("estimateModelContextWindow(%q) = %d, want %d", c.model, got, c.want)
		}
	}
}

// TestGetMaxInputTokensNoLongerUsesOutputCap 验证输入预算由模型上下文窗口推导，
// 不再把 MaxTokens(输出上限)当窗口导致预算被压到 4000。
func TestGetMaxInputTokensNoLongerUsesOutputCap(t *testing.T) {
	unknown := estimateModelContextWindow("some-unknown-model")
	budget := getMaxInputTokens("some-unknown-model", false)
	if budget < 12000 {
		t.Errorf("unknown model input budget too small: %d", budget)
	}
	if budget > unknown {
		t.Errorf("budget exceeds context window: %d > %d", budget, unknown)
	}

	planBudget := getMaxInputTokens("gpt-4o", true)
	reactBudget := getMaxInputTokens("gpt-4o", false)
	if planBudget >= reactBudget {
		t.Errorf("plan_execute reserve should reduce budget: react=%d plan=%d", reactBudget, planBudget)
	}
	if reactBudget < 80000 {
		t.Errorf("gpt-4o should get a large input budget, got %d", reactBudget)
	}
}

// TestTrimHistoryMessages_ChineseTailValidUTF8 验证单条超长中文消息
// 被截到尾部时结果仍是合法 UTF-8：旧实现按字节切片
// content[len(content)-maxChars:] 可能切在汉字中间。
func TestTrimHistoryMessages_ChineseTailValidUTF8(t *testing.T) {
	// 单条消息 300 个汉字，预算 10 token → maxChars=20 rune。
	// 估算 token 远超预算，尾部回溯保留最后一条并截断。
	content := strings.Repeat("茅台股价今日大幅上涨创出历史新高，成交量明显放大。", 12)
	msgs := []map[string]string{{"role": "user", "content": content}}
	got := trimHistoryMessages(msgs, 10)
	if len(got) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got))
	}
	out := got[0]["content"]
	if !utf8.ValidString(out) {
		t.Errorf("truncated content is not valid UTF-8: %q", out)
	}
	if !strings.HasSuffix(out, "\n...(更早的内容已省略)") {
		t.Errorf("expected omission marker suffix, got %q", out)
	}
	// 保留的是尾部：应以原文最后的汉字结尾（省略标记之前）。
	trimmed := strings.TrimSuffix(out, "\n...(更早的内容已省略)")
	if !strings.HasSuffix(content, trimmed) {
		t.Errorf("truncated content should be a tail of the original: %q", trimmed)
	}
	if utf8.RuneCountInString(trimmed) > 20 {
		t.Errorf("tail should be at most 20 runes, got %d", utf8.RuneCountInString(trimmed))
	}
}
