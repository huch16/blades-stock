package agent

import (
	"context"
	"strings"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/logger"

	"github.com/go-kratos/blades"
)

// defaultLLMReflectionFunc builds an LLMReflectionFunc backed by the first
// enabled AI config. Returns nil when no usable config exists (caller falls
// back to keyword scoring). The model provider is reused via the shared cache.
func defaultLLMReflectionFunc() LLMReflectionFunc {
	settingConfig := data.GetSettingConfig()
	if settingConfig == nil {
		return nil
	}
	var cfg *data.AIConfig
	for _, c := range settingConfig.AiConfigs {
		if c != nil {
			cfg = c
			break
		}
	}
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
