package agent

import (
	"context"
	"strings"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/logger"

	"github.com/go-kratos/blades"
)

// selectReflectionConfig picks the AI config used for reflection scoring.
// Preference order:
//  1. a credentialed card whose model name contains the configured preference
//     substring (default "deepseek" — the cheapest provider in the registry);
//  2. otherwise the first credentialed card.
//
// Cards without BaseUrl/ApiKey are skipped, aligned with LLMFromConfig's
// card-picking rule. Returns nil when no usable config exists.
func selectReflectionConfig(configs []*data.AIConfig, preference string) *data.AIConfig {
	var first *data.AIConfig
	pref := strings.ToLower(strings.TrimSpace(preference))
	for _, c := range configs {
		if c == nil || c.BaseUrl == "" || c.ApiKey == "" {
			continue
		}
		if first == nil {
			first = c
		}
		if pref != "" && strings.Contains(strings.ToLower(c.ModelName), pref) {
			return c
		}
	}
	return first
}

// defaultLLMReflectionFunc builds an LLMReflectionFunc backed by the cheapest
// usable AI config (see selectReflectionConfig). Returns nil when no usable
// config exists (caller falls back to keyword scoring). The model provider is
// reused via the shared cache.
func defaultLLMReflectionFunc() LLMReflectionFunc {
	settingConfig := data.GetSettingConfig()
	if settingConfig == nil {
		return nil
	}
	cfg := selectReflectionConfig(settingConfig.AiConfigs, defaultReflectionLLMGate.config().ModelPreference)
	if cfg == nil {
		return nil
	}

	// Reflection is a short, focused call — never use the thinking/reasoning
	// model, which is slower and unnecessary for scoring.
	refConfig := *cfg
	refConfig.Thinking = false
	refConfig.MaxTokens = 2048

	provider, err := createChatModel(context.Background(), refConfig)
	if err != nil {
		logger.SugaredLogger.Warnf("create reflection model failed, fallback to keyword scoring: %v", err)
		return nil
	}

	return func(systemPrompt, userMessage string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		resp, err := provider.Generate(ctx, &blades.ModelRequest{
			Messages: []*blades.Message{
				blades.SystemMessage(systemPrompt),
				blades.UserMessage(userMessage),
			},
		})
		if err != nil {
			return "", err
		}
		if resp == nil || resp.Message == nil {
			return "", nil
		}
		return strings.TrimSpace(resp.Message.Text()), nil
	}
}
