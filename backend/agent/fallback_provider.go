package agent

import (
	"context"
	"encoding/json"
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

// IsPermanentAuthError 判定错误是否为不可逆的 API Key/鉴权失效或账户余额/用量耗尽 (401/403/429 quota/token_rejected)
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
		strings.Contains(s, "forbidden") ||
		strings.Contains(s, "insufficient_quota") ||
		strings.Contains(s, "quota exceeded") ||
		strings.Contains(s, "quota_exceeded") ||
		strings.Contains(s, "用量上限") ||
		strings.Contains(s, "token plan") ||
		strings.Contains(s, "2056") || // MiniMax error code for token plan quota exhaustion
		strings.Contains(s, "balance exhausted") ||
		strings.Contains(s, "account_deactivated") ||
		strings.Contains(s, "欠费")
}

var (
	globalDeadModelMu       sync.RWMutex
	globalDeadModelCooldown = make(map[string]time.Time) // modelIdentifier -> cooldownUntil
)

// MarkModelPermanentFailure 记录某个模型发生永久鉴权或用量耗尽失败，进入 30 分钟全局熔断冷却隔离
func MarkModelPermanentFailure(modelIdentifier string) {
	globalDeadModelMu.Lock()
	defer globalDeadModelMu.Unlock()
	clean := CleanFallbackModelName(modelIdentifier)
	globalDeadModelCooldown[clean] = time.Now().Add(30 * time.Minute)
	globalDeadModelCooldown[modelIdentifier] = time.Now().Add(30 * time.Minute)
	logger.SugaredLogger.Warnf("🚫 [Fallback] 模型 [%s] 发生不可逆鉴权或用量耗尽错误，已自动进入全局熔断隔离池 (冷却30分钟)", clean)
}

// IsModelInCooldown 检查模型是否处于全局熔断冷却期
func IsModelInCooldown(modelIdentifier string) bool {
	globalDeadModelMu.RLock()
	defer globalDeadModelMu.RUnlock()
	clean := CleanFallbackModelName(modelIdentifier)
	if until, ok := globalDeadModelCooldown[clean]; ok && time.Now().Before(until) {
		return true
	}
	if until, ok := globalDeadModelCooldown[modelIdentifier]; ok && time.Now().Before(until) {
		return true
	}
	return false
}

// FormatFallbackReason 解析并格式化模型调用失败的具体原因，避免粗暴掩盖为“服务异常或网络超时”
func FormatFallbackReason(err error) string {
	if err == nil {
		return "未知异常"
	}
	s := err.Error()
	sLower := strings.ToLower(s)

	// 1. 尝试从常见 JSON 报错体中提取精炼的 message
	// 例如: {"message":"Invalid token. Please check your API key.","type":"token_rejected","code":"token_rejected"}
	extractJsonMsg := func(raw string) string {
		start := strings.Index(raw, "{")
		end := strings.LastIndex(raw, "}")
		if start >= 0 && end > start {
			var parsed struct {
				Error struct {
					Message string `json:"message"`
					Type    string `json:"type"`
					Code    any    `json:"code"`
				} `json:"error"`
				Message string `json:"message"`
			}
			jsonPart := raw[start : end+1]
			if err := json.Unmarshal([]byte(jsonPart), &parsed); err == nil {
				if parsed.Error.Message != "" {
					return parsed.Error.Message
				}
				if parsed.Message != "" {
					return parsed.Message
				}
			}
		}
		return ""
	}

	detail := extractJsonMsg(s)

	// 2. 分类判定与精准提示
	switch {
	case strings.Contains(sLower, "404") || strings.Contains(sLower, "model_not_found") || strings.Contains(sLower, "does not exist"):
		if detail != "" {
			return fmt.Sprintf("模型不存在或网关未映射 (HTTP 404: %s)", detail)
		}
		return "模型名称不存在或网关未映射该模型 (HTTP 404)"

	case IsPermanentAuthError(err):
		if detail != "" {
			return fmt.Sprintf("API Key 鉴权失败 (HTTP 401/403: %s)", detail)
		}
		return "API Key 鉴权失败或已过期 (HTTP 401/403)"

	case strings.Contains(sLower, "400") || strings.Contains(sLower, "invalid_request_error") || strings.Contains(sLower, "parameters"):
		if detail != "" {
			return fmt.Sprintf("请求参数异常或网关不支持Tools (HTTP 400: %s)", detail)
		}
		return "请求参数错误或模型不支持Function Calling工具链 (HTTP 400)"

	case strings.Contains(sLower, "429") || strings.Contains(sLower, "rate_limit") || strings.Contains(sLower, "quota"):
		if detail != "" {
			return fmt.Sprintf("网关限流或额度耗尽 (HTTP 429: %s)", detail)
		}
		return "调用频次超限或账户余额不足 (HTTP 429)"

	case strings.Contains(sLower, "500") || strings.Contains(sLower, "502") || strings.Contains(sLower, "503") || strings.Contains(sLower, "504"):
		if detail != "" {
			return fmt.Sprintf("网关服务器异常 (HTTP 5xx: %s)", detail)
		}
		return "AI网关服务器异常 (HTTP 5xx)"

	case strings.Contains(sLower, "deadline exceeded") || strings.Contains(sLower, "timeout") || strings.Contains(sLower, "timed out"):
		return "模型响应超时 (超过设定时限)"

	default:
		if detail != "" {
			return detail
		}
		// 截取前 80 个字符避免 UI 撑爆
		if len(s) > 80 {
			return s[:80] + "..."
		}
		return s
	}
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

// emptyOutputProbeCap 空答检测时最多累积的正文字节数。
// 只需判断「有没有正文」，无需完整正文，故设上限避免长报告重复驻留内存。
const emptyOutputProbeCap = 256 * 1024

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

		// 备用模型单次请求设置超时保护 (原 60s 硬编码极易导致大模型思考与复杂量化计算超时)
		// 动态调整为 180s (3分钟)，且不超过外层上下文剩余时限
		fbTimeout := 180 * time.Second
		if deadline, ok := ctx.Deadline(); ok {
			remain := time.Until(deadline)
			if remain > 15*time.Second && remain < fbTimeout {
				fbTimeout = remain
			}
		}
		fbCtx, cancel := context.WithTimeout(ctx, fbTimeout)
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
			MarkModelPermanentFailure(curName)
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

		// nextAlive 返回 from 起第一个未被永久隔离的下标；没有则返回 len(allProviders)。
		nextAlive := func(from int) int {
			for i := from; i < len(allProviders); i++ {
				f.mu.RLock()
				dead := f.permanentlyFailed[i]
				f.mu.RUnlock()
				if !dead {
					return i
				}
			}
			return len(allProviders)
		}

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
			// 空答检测所需的本地状态（只统计**本次尝试**）：
			//   · contentBuf  累积正文，用于识别「推理模型把输出预算耗在思考上、
			//     正文为空」这种**没有 error 的失败**；
			//   · toolPartSeen 用于排除「合法的工具调用轮次」——工具调用本身没有
			//     正文，绝不能因此判定为空答并触发降级。
			var contentBuf strings.Builder
			toolPartSeen := false

			gen(func(resp *blades.ModelResponse, err error) bool {
				if err != nil {
					nextIdx := nextAlive(idx + 1)

					// 如果第一个 chunk 就报错（且还有备用模型），标记失败由外层切换
					if !yieldedAny && nextIdx < len(allProviders) {
						failed = true
						nextName := names[nextIdx]

						if IsPermanentAuthError(err) {
							f.mu.Lock()
							f.permanentlyFailed[idx] = true
							f.mu.Unlock()
							MarkModelPermanentFailure(curName)
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

				// 统计正文与工具调用（供流结束后的空答判定使用）
				if resp != nil && resp.Message != nil {
					for _, part := range resp.Message.Parts {
						switch p := part.(type) {
						case blades.TextPart:
							if contentBuf.Len() < emptyOutputProbeCap {
								contentBuf.WriteString(p.Text)
							}
						case *blades.TextPart:
							if p != nil && contentBuf.Len() < emptyOutputProbeCap {
								contentBuf.WriteString(p.Text)
							}
						case blades.ToolPart:
							toolPartSeen = true
						case *blades.ToolPart:
							toolPartSeen = true
						}
					}
				}

				return yield(resp, nil)
			})

			// ── 空答检测（关键修复）───────────────────────────────────────────
			//
			// 生产铁证：运行记录同时出现
			//   「失败: 模型未产出正文（空响应）」与「备用链: nemotron-...」，
			//   即**备用链配置完好却从未启用**。
			//
			// 根因：推理模型（如 MiniMax-M3）把输出预算整段耗在思考(reasoning)上，
			// 流会**正常结束且没有任何 error**，只是正文为空。而这里原先只在
			// `err != nil && !yieldedAny` 时降级 —— 空答走不到那一支，于是主模型
			// 被粘性提升为活跃模型，后续每一步都继续空答。
			//
			// 现在：流正常结束、无正文（剔除 think/agent-action 后仍为空）、
			// 且无工具调用 → 判定为空答，切换备用模型重跑本次调用。
			if !failed && !toolPartSeen && strings.TrimSpace(cleanFinalContent(contentBuf.String())) == "" {
				emptyNext := nextAlive(idx + 1)
				if emptyNext < len(allProviders) {
					nextName := names[emptyNext]
					emptyErr := fmt.Errorf("模型返回空正文（空响应）")

					logger.SugaredLogger.Warnf("⚠️ [FallbackModelProvider] 模型 [%s] 流式结束但正文为空（无错误），判定为空答，自动降级切换至 [%s]...", curName, nextName)

					f.mu.Lock()
					f.activeIdx = int32(emptyNext)
					f.mu.Unlock()

					if notifier != nil {
						notifier(CleanFallbackModelName(curName), CleanFallbackModelName(nextName), emptyErr)
					}
					continue // 尝试下一个备用模型
				}
				logger.SugaredLogger.Warnf("⚠️ [FallbackModelProvider] 模型 [%s] 返回空正文且无可用备用模型，交由上层空答检测处理", curName)
			}

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

	primaryKey := fmt.Sprintf("%s|%s", strings.TrimSpace(currentConfig.ApiKey), strings.TrimSpace(currentConfig.BaseUrl))

	for _, cfgPtr := range settingConfig.AiConfigs {
		if cfgPtr == nil {
			continue
		}
		cfg := *cfgPtr
		modelKey := fmt.Sprintf("%s(%s)", cfg.ModelName, cfg.BaseUrl)
		cfgKey := fmt.Sprintf("%s|%s", strings.TrimSpace(cfg.ApiKey), strings.TrimSpace(cfg.BaseUrl))

		// 跳过当前主配置、空 Key 或已处于全局熔断隔离池的失效模型
		if cfg.ID == currentConfig.ID || strings.TrimSpace(cfg.ApiKey) == "" || IsModelInCooldown(modelKey) || IsModelInCooldown(cfg.ModelName) {
			continue
		}

		// 若主模型已因鉴权/用量耗尽熔断，跳过共享相同 ApiKey+BaseUrl 的备用配置，避免重复失败空转
		if (IsModelInCooldown(currentConfig.ModelName) || IsModelInCooldown(fmt.Sprintf("%s(%s)", currentConfig.ModelName, currentConfig.BaseUrl))) && cfgKey == primaryKey {
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
