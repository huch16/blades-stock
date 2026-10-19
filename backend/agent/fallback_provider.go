package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/logger"
	"github.com/go-kratos/blades"
)

// FallbackNotifier 是模型故障转移时的通知回调函数
type FallbackNotifier func(curModel, nextModel string, err error)

type fallbackNotifierKey struct{}

// WithFallbackNotifier 向 Context 注入模型容灾转移通知函数
func WithFallbackNotifier(ctx context.Context, fn FallbackNotifier) context.Context {
	return context.WithValue(ctx, fallbackNotifierKey{}, fn)
}

// GetFallbackNotifier 从 Context 获取模型容灾转移通知函数
func GetFallbackNotifier(ctx context.Context) FallbackNotifier {
	if v := ctx.Value(fallbackNotifierKey{}); v != nil {
		if fn, ok := v.(FallbackNotifier); ok {
			return fn
		}
	}
	return nil
}

// IsPermanentAuthError 判定错误是否为不可逆的 API Key/鉴权失效 (401/403/token_rejected)
func IsPermanentAuthError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "401") ||
		strings.Contains(s, "403") ||
		strings.Contains(s, "token_rejected") ||
		strings.Contains(s, "invalid token") ||
		strings.Contains(s, "invalid_api_key") ||
		strings.Contains(s, "unauthorized") ||
		strings.Contains(s, "forbidden")
}

// CleanFallbackModelName 从附带 BaseUrl 的名称中提取清晰的模型名
// 例如 "MiniMax-M3(https://api.minimax.chat/v1)" -> "MiniMax-M3"
func CleanFallbackModelName(name string) string {
	idx := strings.Index(name, "(")
	if idx > 0 {
		return strings.TrimSpace(name[:idx])
	}
	return strings.TrimSpace(name)
}

// FallbackModelProvider 实现多模型容灾降级
// 当主模型执行失败、超时或限流时，自动无缝尝试备用模型列表
// 具备“粘性提升”（Sticky Promotion）和“死锁黑名单”机制，避免在多步骤 Agent 循环中重复在已知失败模型上空等超时
type FallbackModelProvider struct {
	primary           blades.ModelProvider
	fallbacks         []blades.ModelProvider
	names             []string
	activeIdx         int32
	permanentlyFailed map[int]bool
	mu                sync.RWMutex
}

// NewFallbackModelProvider 创建带降级容灾的 ModelProvider
func NewFallbackModelProvider(primary blades.ModelProvider, primaryName string, fallbacks []blades.ModelProvider, fallbackNames []string) blades.ModelProvider {
	if len(fallbacks) == 0 {
		return primary
	}
	return &FallbackModelProvider{
		primary:           primary,
		fallbacks:         fallbacks,
		names:             append([]string{primaryName}, fallbackNames...),
		permanentlyFailed: make(map[int]bool),
	}
}

func (f *FallbackModelProvider) Name() string {
	f.mu.RLock()
	idx := int(f.activeIdx)
	if idx >= 0 && idx < len(f.names) {
		name := f.names[idx]
		f.mu.RUnlock()
		return CleanFallbackModelName(name)
	}
	f.mu.RUnlock()
	if len(f.names) > 0 {
		return CleanFallbackModelName(f.names[0])
	}
	return "fallback-composite-provider"
}

// Generate 实现 blades.ModelProvider 接口，具备故障自动转移能力
func (f *FallbackModelProvider) Generate(ctx context.Context, req *blades.ModelRequest) (*blades.ModelResponse, error) {
	allProviders := append([]blades.ModelProvider{f.primary}, f.fallbacks...)
	names := f.names

	f.mu.RLock()
	startIdx := int(f.activeIdx)
	f.mu.RUnlock()

	var lastErr error
	notifier := GetFallbackNotifier(ctx)

	for idx := startIdx; idx < len(allProviders); idx++ {
		f.mu.RLock()
		isDead := f.permanentlyFailed[idx]
		f.mu.RUnlock()
		if isDead {
			continue
		}

		prov := allProviders[idx]
		curName := names[idx]

		// 备用模型单次请求设置超时保护
		fbCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		resp, err := prov.Generate(fbCtx, req)
		cancel()

		if err == nil {
			f.mu.Lock()
			f.activeIdx = int32(idx)
			f.mu.Unlock()
			return resp, nil
		}

		lastErr = err
		logger.SugaredLogger.Warnf("⚠️ [FallbackModelProvider] 模型 [%s] 失败: %v", curName, err)

		if IsPermanentAuthError(err) {
			f.mu.Lock()
			f.permanentlyFailed[idx] = true
			f.mu.Unlock()
		}

		// 寻找下一个备选模型
		nextIdx := idx + 1
		for nextIdx < len(allProviders) {
			f.mu.RLock()
			nextDead := f.permanentlyFailed[nextIdx]
			f.mu.RUnlock()
			if !nextDead {
				break
			}
			nextIdx++
		}

		if nextIdx < len(allProviders) {
			nextName := names[nextIdx]
			f.mu.Lock()
			f.activeIdx = int32(nextIdx)
			f.mu.Unlock()
			logger.SugaredLogger.Warnf("🔄 [FallbackModelProvider] 正在切换至备用模型: %s", nextName)
			if notifier != nil {
				notifier(CleanFallbackModelName(curName), CleanFallbackModelName(nextName), err)
			}
		}
	}

	return nil, fmt.Errorf("所有可用模型均调用失败 (共 %d 个模型), 最后错误: %w", len(allProviders), lastErr)
}

// NewStreaming 实现流式输出（具备故障自动转移到备用模型的能力）
func (f *FallbackModelProvider) NewStreaming(ctx context.Context, req *blades.ModelRequest) blades.Generator[*blades.ModelResponse, error] {
	return func(yield func(*blades.ModelResponse, error) bool) {
		allProviders := append([]blades.ModelProvider{f.primary}, f.fallbacks...)
		names := f.names

		f.mu.RLock()
		startIdx := int(f.activeIdx)
		f.mu.RUnlock()

		notifier := GetFallbackNotifier(ctx)

		for idx := startIdx; idx < len(allProviders); idx++ {
			f.mu.RLock()
			isDead := f.permanentlyFailed[idx]
			f.mu.RUnlock()
			if isDead {
				continue
			}

			prov := allProviders[idx]
			curName := names[idx]

			gen := prov.NewStreaming(ctx, req)
			if gen == nil {
				continue
			}

			yieldedAny := false
			failed := false

			gen(func(resp *blades.ModelResponse, err error) bool {
				if err != nil {
					// 寻找下一个有效备选
					nextIdx := idx + 1
					for nextIdx < len(allProviders) {
						f.mu.RLock()
						nextDead := f.permanentlyFailed[nextIdx]
						f.mu.RUnlock()
						if !nextDead {
							break
						}
						nextIdx++
					}

					// 如果第一个 chunk 就报错（且还有备用模型），标记失败由外层切换
					if !yieldedAny && nextIdx < len(allProviders) {
						failed = true
						nextName := names[nextIdx]

						if IsPermanentAuthError(err) {
							f.mu.Lock()
							f.permanentlyFailed[idx] = true
							f.mu.Unlock()
						}

						// 永久粘性提升 activeIdx 为下一个可用模型，后续 Agent ReAct 步骤直接以此模型启动，杜绝死锁等待
						f.mu.Lock()
						f.activeIdx = int32(nextIdx)
						f.mu.Unlock()

						logger.SugaredLogger.Warnf("⚠️ [FallbackModelProvider] 模型 [%s] 流式调用首包失败: %v, 自动降级切换至 [%s]...", curName, err, nextName)
						if notifier != nil {
							notifier(CleanFallbackModelName(curName), CleanFallbackModelName(nextName), err)
						}
						return false // 停止当前 generator
					}
					// 已经吐过内容或无备选模型，正常将错误 yield 出去
					return yield(resp, err)
				}

				yieldedAny = true
				return yield(resp, nil)
			})

			// 成功吐出了内容，固定当前正常响应的模型作为后续步骤的活跃主模型
			if yieldedAny && !failed {
				f.mu.Lock()
				f.activeIdx = int32(idx)
				f.mu.Unlock()
				return
			}

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
