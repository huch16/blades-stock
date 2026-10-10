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

// NewStreaming 实现流式输出（具备故障自动转移到备用模型的能力）
func (f *FallbackModelProvider) NewStreaming(ctx context.Context, req *blades.ModelRequest) blades.Generator[*blades.ModelResponse, error] {
	return func(yield func(*blades.ModelResponse, error) bool) {
		allProviders := append([]blades.ModelProvider{f.primary}, f.fallbacks...)
		names := f.names

		for idx, prov := range allProviders {
			gen := prov.NewStreaming(ctx, req)
			if gen == nil {
				continue
			}

			yieldedAny := false
			failed := false

			gen(func(resp *blades.ModelResponse, err error) bool {
				if err != nil {
					// 如果第一个 chunk 就报错（且还有备用模型），标记失败由外层切换
					if !yieldedAny && idx+1 < len(allProviders) {
						failed = true
						curName := "unknown"
						if idx < len(names) {
							curName = names[idx]
						}
						nextName := "unknown"
						if idx+1 < len(names) {
							nextName = names[idx+1]
						}
						logger.SugaredLogger.Warnf("⚠️ [FallbackModelProvider] 模型 [%s] 流式调用首包失败: %v, 自动降级切换至 [%s]...", curName, err, nextName)
						return false // 停止当前 generator
					}
					// 已经吐过内容或无备选模型，正常将错误 yield 出去
					return yield(resp, err)
				}

				yieldedAny = true
				return yield(resp, nil)
			})

			// 如果没失败（正常结束），或者已经输出了内容，则结束整个 fallback 流程
			if !failed {
				return
			}
		}
	}
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
