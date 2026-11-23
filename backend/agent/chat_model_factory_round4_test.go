package agent

import (
	"context"
	"fmt"
	"testing"
	"time"

	"blades-stock/backend/data"
)

// 第四轮 B 组（运行）回归测试：providerDeepSeek 显式分支、
// modelProviderCache 有界。

func TestDetectChatModelProvider_Round4(t *testing.T) {
	cases := []struct {
		name      string
		baseLower string
		model     string
		want      chatModelProvider
	}{
		{"deepseek base", "https://api.deepseek.com/v1", "deepseek-chat", providerDeepSeek},
		{"deepseek model name only", "", "deepseek-reasoner", providerDeepSeek},
		{"deepseek base any model", "https://api.deepseek.com", "custom-model", providerDeepSeek},
		{"deepseek-v series", "", "deepseek-v3.2", providerDeepSeek},
		{"volc ark", "https://ark.cn-beijing.volces.com/api/v3", "ep-xxx", providerVolcArk},
		{"dashscope", "https://dashscope.aliyuncs.com/compatible-mode/v1", "qwen-plus", providerDashScope},
		{"openrouter", "https://openrouter.ai/api/v1", "x", providerOpenRouter},
		{"anthropic", "https://api.anthropic.com/v1", "claude-x", providerAnthropic},
		{"ollama", "http://localhost:11434/v1", "llama3", providerOllama},
		{"gemini", "https://generativelanguage.googleapis.com/v1beta", "gemini-2.5-flash", providerGemini},
		{"openai", "https://api.openai.com/v1", "gpt-4o", providerOpenAICompatible},
		{"generic gateway", "https://aigw.woha.men/v1", "nemotron-x", providerOpenAICompatible},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := detectChatModelProvider(c.baseLower, c.model); got != c.want {
				t.Errorf("detectChatModelProvider(%q, %q) = %d, want %d", c.baseLower, c.model, got, c.want)
			}
		})
	}
}

// TestCreateChatModel_DeepSeekUsesOpenAICompatible 守卫显式 case 的行为
// 等价性：DeepSeek 卡仍走 OpenAI 兼容构造（纯构造，不碰网络），
// 且第二次调用命中同一缓存实例。
func TestCreateChatModel_DeepSeekUsesOpenAICompatible(t *testing.T) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	cfg := data.AIConfig{
		BaseUrl:     "https://api.deepseek.com/v1",
		ApiKey:      "sk-test-fake-key",
		ModelName:   "deepseek-chat-r4-" + suffix,
		MaxTokens:   4096,
		Temperature: 0.3,
		TimeOut:     300,
	}

	p1, err := createChatModel(context.Background(), cfg)
	if err != nil {
		t.Fatalf("DeepSeek 卡构造失败: %v", err)
	}
	if p1 == nil {
		t.Fatalf("DeepSeek 卡应返回非空 provider")
	}
	p2, err := createChatModel(context.Background(), cfg)
	if err != nil {
		t.Fatalf("第二次构造失败: %v", err)
	}
	if p2 != p1 {
		t.Fatalf("相同配置第二次调用应命中缓存，got 不同实例")
	}
}

// TestModelProviderCache_EvictsWhenOverLimit 守卫缓存有界：
// 键含 UpdatedAt，每次保存卡片都产生新键；超限时整表清空后重建，
// 缓存不再随编辑次数无界增长。
func TestModelProviderCache_EvictsWhenOverLimit(t *testing.T) {
	// 清空全局缓存使本用例 hermetic（缓存只是备忘，清空后按需重建）。
	modelProviderCache.Range(func(k, _ any) bool {
		modelProviderCache.Delete(k)
		return true
	})
	oldLimit := modelProviderCacheLimit.Load()
	modelProviderCacheLimit.Store(3)
	defer func() { modelProviderCacheLimit.Store(oldLimit) }()

	mkCfg := func(i int) data.AIConfig {
		return data.AIConfig{
			BaseUrl:     fmt.Sprintf("https://cache-limit-%d.invalid/v1", i),
			ApiKey:      fmt.Sprintf("sk-test-%d", i),
			ModelName:   fmt.Sprintf("cache-model-%d", i),
			MaxTokens:   4096,
			Temperature: 0.3,
			TimeOut:     300,
		}
	}

	for i := 0; i < 3; i++ {
		p, err := createChatModel(context.Background(), mkCfg(i))
		if err != nil || p == nil {
			t.Fatalf("构造卡 %d 失败: %v", i, err)
		}
	}
	if got := modelProviderCacheSize(); got != 3 {
		t.Fatalf("存 3 张卡后缓存应为 3 条，got %d", got)
	}

	// 第 4 张触发清空+重建：表里只剩第 4 张。
	p4, err := createChatModel(context.Background(), mkCfg(3))
	if err != nil || p4 == nil {
		t.Fatalf("构造第 4 张卡失败: %v", err)
	}
	if got := modelProviderCacheSize(); got != 1 {
		t.Fatalf("超限后缓存应只剩新存的 1 条，got %d", got)
	}
	// 第 4 张仍可命中缓存（清空后又存回去了）。
	p4again, err := createChatModel(context.Background(), mkCfg(3))
	if err != nil || p4again != p4 {
		t.Fatalf("第 4 张卡应命中缓存，got err=%v same=%v", err, p4again == p4)
	}
	// 被清掉的第 1 张按需重建，不报错。
	if _, err := createChatModel(context.Background(), mkCfg(0)); err != nil {
		t.Fatalf("被清掉的卡应能重建: %v", err)
	}
}
