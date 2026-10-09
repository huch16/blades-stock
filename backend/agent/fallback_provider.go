package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/logger"
	"github.com/go-kratos/blades"
)

// FallbackModelProvider 实现多模型容灾降级
// 当主模型执行失败、超时或限流时，自动无缝尝试备用模型列表
type FallbackModelProvider struct {
	primary   blades.ModelProvider
	fallbacks []blades.ModelProvider
	names     []string
}

// NewFallbackModelProvider 创建带降级容灾的 ModelProvider
func NewFallbackModelProvider(primary blades.ModelProvider, primaryName string, fallbacks []blades.ModelProvider, fallbackNames []string) blades.ModelProvider {
	if len(fallbacks) == 0 {
		return primary
	}
	return &FallbackModelProvider{
		primary:   primary,
		fallbacks: fallbacks,
		names:     append([]string{primaryName}, fallbackNames...),
	}
}

func (f *FallbackModelProvider) Name() string {
	if len(f.names) > 0 {
		return f.names[0]
	}
	return "fallback-composite-provider"
}

// Generate 实现 blades.ModelProvider 接口，具备故障自动转移能力
func (f *FallbackModelProvider) Generate(ctx context.Context, req *blades.ModelRequest) (*blades.ModelResponse, error) {
	// 1. 先尝试主模型
	resp, err := f.primary.Generate(ctx, req)
	if err == nil {
		return resp, nil
	}

	logger.SugaredLogger.Warnf("⚠️ [FallbackModelProvider] 主模型 %s 失败: %v, 开始尝试备用降级模型...", f.names[0], err)

	// 2. 依次尝试备用模型
	var lastErr error = err
	for idx, fallback := range f.fallbacks {
		modelName := "fallback-model"
		if idx+1 < len(f.names) {
			modelName = f.names[idx+1]
		}

		logger.SugaredLogger.Infof("🔄 [FallbackModelProvider] 正在切换至备用模型 [%d]: %s", idx+1, modelName)

		// 备用模型单次请求设置超时保护
		fbCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		fbResp, fbErr := fallback.Generate(fbCtx, req)
		cancel()

		if fbErr == nil {
			logger.SugaredLogger.Infof("✅ [FallbackModelProvider] 备用模型 %s 响应成功！", modelName)
			return fbResp, nil
		}

		logger.SugaredLogger.Warnf("❌ [FallbackModelProvider] 备用模型 %s 也失败了: %v", modelName, fbErr)
		lastErr = fbErr
	}

	return nil, fmt.Errorf("所有模型均调用失败 (包含 %d 个降级模型), 最后错误: %w", len(f.fallbacks), lastErr)
}

// NewStreaming 实现流式输出（如果主模型失败，则尝试备用模型的流）
func (f *FallbackModelProvider) NewStreaming(ctx context.Context, req *blades.ModelRequest) blades.Generator[*blades.ModelResponse, error] {
	// 尝试主模型流式
	gen := f.primary.NewStreaming(ctx, req)
	if gen != nil {
		return gen
	}

	for _, fallback := range f.fallbacks {
		fbGen := fallback.NewStreaming(ctx, req)
		if fbGen != nil {
			return fbGen
		}
	}

	return nil
}

// BuildFallbackProvidersFromDB 从数据库或全局配置中获取备用的 AIConfig 并实例化 Provider
func BuildFallbackProvidersFromDB(ctx context.Context, currentConfig data.AIConfig) ([]blades.ModelProvider, []string) {
	var fallbackProviders []blades.ModelProvider
	var fallbackNames []string

	settingConfig := data.GetSettingConfig()
	if settingConfig == nil || len(settingConfig.AiConfigs) <= 1 {
		return nil, nil
	}

	for _, cfgPtr := range settingConfig.AiConfigs {
		if cfgPtr == nil {
			continue
		}
		cfg := *cfgPtr
		// 跳过当前主配置和空 Key
		if cfg.ID == currentConfig.ID || strings.TrimSpace(cfg.ApiKey) == "" {
			continue
		}

		provider, err := createChatModel(ctx, cfg)
		if err == nil && provider != nil {
			fallbackProviders = append(fallbackProviders, provider)
			fallbackNames = append(fallbackNames, fmt.Sprintf("%s(%s)", cfg.ModelName, cfg.BaseUrl))
		}
	}

	return fallbackProviders, fallbackNames
}
