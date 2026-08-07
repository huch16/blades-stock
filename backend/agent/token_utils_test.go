package agent

import "testing"

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
