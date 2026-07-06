package agent

import (
	"context"
	"blades-stock/backend/data"
	"blades-stock/backend/logger"
	"strings"
	"sync"
	"time"

	"github.com/duke-git/lancet/v2/strutil"
	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/anthropic"
	"github.com/go-kratos/blades/contrib/gemini"
	"github.com/go-kratos/blades/contrib/openai"
	"google.golang.org/genai"
)

var (
	modelProviderCache sync.Map
)

type cachedProvider struct {
	provider blades.ModelProvider
}

func providerCacheKey(cfg data.AIConfig) string {
	var b strings.Builder
	b.WriteString(cfg.BaseUrl)
	b.WriteByte('\n')
	b.WriteString(cfg.ApiKey)
	b.WriteByte('\n')
	b.WriteString(cfg.ModelName)
	b.WriteByte('\n')
	b.WriteString(cfg.UpdatedAt.String())
	return b.String()
}

func InvalidateModelProviderCache() {
	modelProviderCache.Range(func(k, _ interface{}) bool {
		modelProviderCache.Delete(k)
		return true
	})
	logger.SugaredLogger.Info("model provider cache cleared")
}

type chatModelProvider int

const (
	providerOpenAICompatible chatModelProvider = iota
	providerVolcArk
	providerDashScope
	providerOpenRouter
	providerAnthropic
	providerOllama
	providerGemini
	providerDeepSeek
)

func normalizeBaseURL(base string) string {
	return strings.TrimSuffix(strings.TrimSpace(base), "/")
}

func normalizeChatModelBaseURL(base string) string {
	base = normalizeBaseURL(base)
	return strings.TrimSuffix(base, "/chat/completions")
}

func detectChatModelProvider(baseLower, modelName string) chatModelProvider {
	modelLower := strings.ToLower(strings.TrimSpace(modelName))

	if strings.Contains(baseLower, "volces.com") && strings.Contains(baseLower, "ark") {
		return providerVolcArk
	}
	if strings.Contains(baseLower, "dashscope.aliyuncs.com") ||
		strings.Contains(baseLower, "dashscope-intl.aliyuncs.com") {
		return providerDashScope
	}
	if strings.Contains(baseLower, "openrouter.ai") {
		return providerOpenRouter
	}
	if strings.Contains(baseLower, "anthropic.com") || strings.Contains(baseLower, "api.anthropic") {
		return providerAnthropic
	}
	if strings.Contains(baseLower, ":11434") || strings.Contains(baseLower, "ollama") {
		return providerOllama
	}
	if isGeminiGoogleAI(baseLower, modelLower) {
		return providerGemini
	}
	if strings.Contains(baseLower, "api.deepseek.com") ||
		strutil.ContainsAny(modelLower, []string{"deepseek", "deepseek-v", "deepseek-r", "deepseek-chat", "deepseek-coder", "deepseek-reasoner"}) {
		return providerDeepSeek
	}
	return providerOpenAICompatible
}

func isGeminiGoogleAI(baseLower, modelLower string) bool {
	if strings.Contains(baseLower, "generativelanguage.googleapis.com") ||
		strings.Contains(baseLower, "ai.google.dev") {
		return true
	}
	if strings.HasPrefix(modelLower, "gemini-") || strings.HasPrefix(modelLower, "gemini/") ||
		strings.HasPrefix(modelLower, "models/gemini") {
		return baseLower == "" || strings.Contains(baseLower, "googleapis.com")
	}
	return false
}

func createChatModel(ctx context.Context, aiConfig data.AIConfig) (blades.ModelProvider, error) {
	baseURL := normalizeChatModelBaseURL(aiConfig.BaseUrl)
	baseLower := strings.ToLower(baseURL)
	temperature := aiConfig.Temperature
	timeout := time.Duration(aiConfig.TimeOut) * time.Second
	if timeout <= 0 {
		timeout = 300 * time.Second
	}
	maxTok := aiConfig.MaxTokens
	if maxTok <= 0 {
		maxTok = 4096
	}

	key := providerCacheKey(aiConfig)
	if cached, ok := modelProviderCache.Load(key); ok {
		logger.SugaredLogger.Infof("reusing cached model provider for config %d (%s)", aiConfig.ID, aiConfig.ModelName)
		return cached.(cachedProvider).provider, nil
	}

	p := detectChatModelProvider(baseLower, aiConfig.ModelName)
	logger.SugaredLogger.Infof("createChatModel provider=%d base=%q model=%q", p, aiConfig.BaseUrl, aiConfig.ModelName)

	var provider blades.ModelProvider
	var err error

	switch p {
	case providerAnthropic:
		maxOut := aiConfig.MaxTokens
		if maxOut <= 0 || maxOut > 32768 {
			maxOut = 32768
		}
		provider = anthropic.NewModel(aiConfig.ModelName, anthropic.Config{
			APIKey:          aiConfig.ApiKey,
			BaseURL:         baseURL,
			MaxOutputTokens: int64(maxOut),
			Temperature:     temperature,
		})

	case providerGemini:
		cc := &genai.ClientConfig{APIKey: aiConfig.ApiKey}
		provider, err = gemini.NewModel(ctx, aiConfig.ModelName, gemini.Config{
			ClientConfig: *cc,
			Temperature:  float32(temperature),
		})

	default:
		// max_tokens 是输出 token 上限，不是上下文窗口。
		// MiniMax-M3(1M ctx) 的 API 限制 max_tokens≤524288，且实际输出不可能达到。
		// 对所有 OpenAI 兼容模型统一 cap 到 32768，兼顾安全与实用性。
		outputTok := maxTok
		if outputTok <= 0 || outputTok > 32768 {
			outputTok = 32768
		}
		extraFields := map[string]any{
			"max_tokens": outputTok,
		}
		if aiConfig.Thinking {
			extraFields["reasoning_effort"] = "medium"
		}
		cfg := openai.Config{
			APIKey:          aiConfig.ApiKey,
			BaseURL:         baseURL,
			Temperature:     temperature,
			MaxOutputTokens: int64(outputTok),
			ExtraFields:     extraFields,
		}
		provider = openai.NewModel(aiConfig.ModelName, cfg)
	}

	if err != nil {
		return nil, err
	}

	modelProviderCache.Store(key, cachedProvider{provider: provider})
	return provider, nil
}
