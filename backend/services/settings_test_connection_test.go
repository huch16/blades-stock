package services

import (
	"strings"
	"testing"
)

// 固化 v3.0.0 修复：连通性测试的错误分类与「输出预算不足」识别。
//
// 事故背景：TestAiConnection 曾硬编码 max_tokens=16。对推理模型
// （MiniMax-M3 / deepseek-reasoner / GLM-Z 等），16 个 token 会在  thinking
// 阶段就被耗尽，服务端返回 token 相关错误，用户看到的是「token 不够」
// 而误以为是自己配置错误。

func TestIsTokenBudgetError(t *testing.T) {
	yes := []string{
		"max_tokens=16 is too small",
		"max_tokens must be greater than 0",
		"the model consumes tokens during reasoning",
		"maximum context length exceeded",
		"output budget exhausted",
		"Token 数量不足",
	}
	for _, s := range yes {
		if !isTokenBudgetError(s) {
			t.Errorf("应识别为输出预算问题: %q", s)
		}
	}

	no := []string{
		"",
		"The model `foo` does not exist",
		"invalid api key",
		"rate limit reached",
	}
	for _, s := range no {
		if isTokenBudgetError(s) {
			t.Errorf("不应识别为输出预算问题: %q", s)
		}
	}
}

func TestClassifyAIError(t *testing.T) {
	cases := []struct {
		name      string
		status    int
		raw       string
		model     string
		wantSub   []string
		wantEqual string
	}{
		{
			name:    "模型不存在",
			status:  404,
			raw:     "invalid_request_error: The model `minimaxai/minimax-m9` does not exist",
			model:   "minimaxai/minimax-m9",
			wantSub: []string{"不存在模型", "minimaxai/minimax-m9", "拉取列表"},
		},
		{
			name:    "token 预算不足",
			status:  400,
			raw:     "max_tokens=16 is too small: the model consumes tokens during reasoning",
			model:   "m",
			wantSub: []string{"输出预算不足", "重试"},
		},
		{
			name:    "API Key 无效",
			status:  401,
			raw:     "invalid api key",
			model:   "m",
			wantSub: []string{"API Key 无效", "401"},
		},
		{
			name:    "404 路径问题",
			status:  404,
			raw:     "not found",
			model:   "m",
			wantSub: []string{"404", "/v1"},
		},
		{
			name:    "限流",
			status:  429,
			raw:     "rate limit",
			model:   "m",
			wantSub: []string{"429"},
		},
		{
			name:      "无原始信息时回退到状态码",
			status:    502,
			raw:       "",
			model:     "m",
			wantEqual: "HTTP 502",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := classifyAIError(c.status, c.raw, c.model)
			if c.wantEqual != "" {
				if got != c.wantEqual {
					t.Fatalf("got=%q want=%q", got, c.wantEqual)
				}
				return
			}
			for _, sub := range c.wantSub {
				if !strings.Contains(got, sub) {
					t.Errorf("提示缺少关键信息 %q: %q", sub, got)
				}
			}
		})
	}
}
