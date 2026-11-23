package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
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
//
// 分类必须区分「永久」与「瞬时」，因为它同时驱动两件事：30 分钟全局熔断
// （MarkModelPermanentFailure）与同凭证备用模型的排除
// （BuildFallbackProvidersFromDB 跳过共享 ApiKey+BaseUrl 的配置）。
// 把一次瞬时 429 误判为永久，会同时锁死主模型与走同一网关的备用模型。
//
// 规则：
//   - 401/403 与鉴权类文案 → 永久；
//   - 硬性计费/停用信号（insufficient_quota、balance exhausted、account_deactivated、
//     欠费、token plan、MiniMax 2056）→ 永久，即使伴随 429（OpenAI 的
//     insufficient_quota 本来就是以 429 返回的，但含义是账单耗尽而非瞬时限流）；
//   - 模糊的额度文案（quota exceeded、用量上限）→ 只有在**不带** 429/限流/容量信号时
//     才算永久；一旦带有 429、rate_limit、too many requests、try again later、
//     at/over capacity、overloaded 等信号，就按瞬时容量/限流处理。
//     网关（如 TokenLab“容量满”）常把 429 瞬时限流写成 quota exceeded，旧实现
//     在此直接判永久，会让一次容量抖动锁死整条同网关备用链。
func IsPermanentAuthError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())

	// 硬性计费/停用信号：即使伴随 429 也视为永久。
	if strings.Contains(s, "insufficient_quota") ||
		strings.Contains(s, "balance exhausted") ||
		strings.Contains(s, "insufficient balance") ||
		strings.Contains(s, "balance_insufficient") ||
		strings.Contains(s, "insufficient funds") ||
		strings.Contains(s, "account_deactivated") ||
		strings.Contains(s, "account suspended") ||
		strings.Contains(s, "account_suspended") ||
		strings.Contains(s, "account disabled") ||
		strings.Contains(s, "account_disabled") ||
		strings.Contains(s, "token plan") ||
		strings.Contains(s, "billing limit") ||
		strings.Contains(s, "billing hard limit") ||
		strings.Contains(s, "billing issue") ||
		strings.Contains(s, "billing error") ||
		strings.Contains(s, "billing exhausted") ||
		strings.Contains(s, "402") ||
		strings.Contains(s, "payment required") ||
		strings.Contains(s, "2056") || // MiniMax error code for token plan quota exhaustion
		strings.Contains(s, "欠费") {
		return true
	}

	// 401/403 鉴权失败：永久。
	if strings.Contains(s, "401") ||
		strings.Contains(s, "403") ||
		strings.Contains(s, "token_rejected") ||
		strings.Contains(s, "invalid token") ||
		strings.Contains(s, "invalid_api_key") ||
		strings.Contains(s, "incorrect api key") ||
		strings.Contains(s, "incorrect_api_key") ||
		strings.Contains(s, "api key not valid") ||
		strings.Contains(s, "authentication failed") ||
		strings.Contains(s, "auth failed") ||
		strings.Contains(s, "token expired") ||
		strings.Contains(s, "key expired") ||
		strings.Contains(s, "api key expired") ||
		strings.Contains(s, "invalid credentials") ||
		strings.Contains(s, "invalid credential") ||
		strings.Contains(s, "unauthorized") ||
		strings.Contains(s, "forbidden") {
		return true
	}

	// 模糊的额度文案：仅在没有瞬时限流/容量信号时才按用量耗尽处理。
	if strings.Contains(s, "quota exceeded") ||
		strings.Contains(s, "quota_exceeded") ||
		strings.Contains(s, "用量上限") {
		return !isTransientRateLimitSignal(s)
	}
	return false
}

// isTransientRateLimitSignal 判断错误文本是否带有 429/限流/容量不足的瞬时信号。
func isTransientRateLimitSignal(s string) bool {
	for _, m := range []string{
		"429",
		"rate_limit",
		"rate limit",
		"too many requests",
		"try again later",
		"at capacity",
		"over capacity",
		"overloaded",
		"capacity",
	} {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

var (
	globalDeadModelMu       sync.RWMutex
	globalDeadModelCooldown = make(map[string]time.Time) // 冷却键（模型名+网关身份）-> cooldownUntil
)

// apiKeyFingerprint 取 ApiKey 的短哈希指纹（前 12 位十六进制），用于冷却键区分
// 同一网关上不同凭证的卡片。只存哈希前缀，原始 Key 永不进入冷却表或日志。
func apiKeyFingerprint(apiKey string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(apiKey)))
	return hex.EncodeToString(sum[:])[:12]
}

// FallbackProviderName 生成备用链内部使用的模型身份名，格式为
// "模型名(BaseUrl#key指纹)"，例如 "deepseek-chat(https://api.deepseek.com/v1#e3b0c44298fc)"。
// 它是冷却键的载体：同一模型名在不同网关/不同凭证下得到不同名字，
// 从而在 MarkModelPermanentFailure / IsModelInCooldown 里独立冷却。
// 对外展示一律用 DisplayFallbackProviderName（去掉 #指纹部分）或
// CleanFallbackModelName（只留模型名）；指纹不进入运行记录与用户可见文案。
func FallbackProviderName(cfg data.AIConfig) string {
	return fmt.Sprintf("%s(%s#%s)",
		strings.TrimSpace(cfg.ModelName),
		normalizeBaseURL(cfg.BaseUrl),
		apiKeyFingerprint(cfg.ApiKey))
}

// parseCooldownIdentity 从备用链身份名解析（模型名，网关 BaseUrl，凭证指纹）。
// 兼容旧格式与裸模型名（测试/历史调用）：
//   - "Model(BaseUrl#fp)" → (Model, BaseUrl, fp)
//   - "Model(BaseUrl)"    → (Model, BaseUrl, "")，凭证未知
//   - "Model"             → (Model, "", "")，网关与凭证均未知
func parseCooldownIdentity(name string) (model, baseUrl, keyFp string) {
	name = strings.TrimSpace(name)
	open := strings.Index(name, "(")
	if open <= 0 {
		return name, "", ""
	}
	model = strings.TrimSpace(name[:open])
	inner := strings.TrimSpace(strings.TrimSuffix(name[open+1:], ")"))
	if i := strings.LastIndex(inner, "#"); i >= 0 {
		return model, normalizeBaseURL(inner[:i]), strings.TrimSpace(inner[i+1:])
	}
	return model, normalizeBaseURL(inner), ""
}

// DisplayFallbackProviderName 把内部身份名还原为展示用名（去掉 #凭证指纹）：
// "Model(BaseUrl#fp)" → "Model(BaseUrl)"；其它格式原样返回。
// 用于运行记录 FallbackChain 与日志，避免哈希指纹污染用户可见内容。
func DisplayFallbackProviderName(name string) string {
	model, baseUrl, _ := parseCooldownIdentity(name)
	if baseUrl == "" {
		return model
	}
	return fmt.Sprintf("%s(%s)", model, baseUrl)
}

// cooldownKeyFromName 由身份名导出冷却表键：模型名 + 网关（BaseUrl + 凭证指纹）。
// 旧实现以裸模型名（CleanFallbackModelName）与原始身份名双键存储，
// 本质仍是按模型名冷却：同一模型名在不同网关的卡片会互相牵连
// （A 网关鉴权失败 → B 网关同名卡也被排除 30 分钟）；
// 现改为模型+网关细分，同名不同网关/不同凭证独立冷却。
func cooldownKeyFromName(name string) string {
	model, baseUrl, keyFp := parseCooldownIdentity(name)
	return model + "\n" + baseUrl + "\n" + keyFp
}

// MarkModelPermanentFailure 记录某个模型发生永久鉴权或用量耗尽失败，进入 30 分钟全局熔断冷却隔离
func MarkModelPermanentFailure(modelIdentifier string) {
	globalDeadModelMu.Lock()
	defer globalDeadModelMu.Unlock()
	// 冷却表只增不减会无界增长：每次写入时顺带清掉已过期的条目。
	// 条目数上限本就是注册表卡片数（键 = 模型名+网关+凭证指纹），
	// 但过期条目若永不清理仍是泄漏；写入时清一次是 O(表大小) 的
	// 廉价操作，且只在永久失败（低频事件）时发生。Go 允许 range
	// 中 delete，安全。
	now := time.Now()
	for k, until := range globalDeadModelCooldown {
		if !now.Before(until) {
			delete(globalDeadModelCooldown, k)
		}
	}
	globalDeadModelCooldown[cooldownKeyFromName(modelIdentifier)] = now.Add(30 * time.Minute)
	logger.SugaredLogger.Warnf("🚫 [Fallback] 模型 [%s] 发生不可逆鉴权或用量耗尽错误，已自动进入全局熔断隔离池 (冷却30分钟)", CleanFallbackModelName(modelIdentifier))
}

// IsModelInCooldown 检查模型是否处于全局熔断冷却期
func IsModelInCooldown(modelIdentifier string) bool {
	globalDeadModelMu.RLock()
	defer globalDeadModelMu.RUnlock()
	if until, ok := globalDeadModelCooldown[cooldownKeyFromName(modelIdentifier)]; ok && time.Now().Before(until) {
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

// 流式停滞看门狗阈值（包级变量而非常量，便于单测收窄；生产值见注释）。
//
// 为什么需要它：流式传输可以“静默挂起”——连接不断、一个 chunk 都不来，
// 上层只能等到卡片级请求超时（默认 300 秒）才失败。看门狗在无任何 chunk
// 的静默期到达阈值时主动取消本次尝试，让降级链提前接管。
//   - 首包等待 120 秒：SSE 正常在连接建立后立刻下发 role 首包；推理模型
//     在思考期间也会持续下发 reasoning delta；120 秒零字节基本等于挂起，
//     远短于 300 秒请求超时，又留足了网关排队余量。
//   - 后续 chunk 间隔 180 秒：长思考/长输出的间隙也应有上限；一旦超过，
//     视为网关挂起而非“模型还在想”。
//
// 注意：看门狗只看“有没有 chunk 到达”，不看内容——reasoning delta 与空
// chunk 同样能证明连接存活（blades 的 OpenAI 适配层会把 reasoning
// delta 转成零 part 消息下发，本层仍能收到回调）。
//
// 阈值与其它超时的分层（由内到外）：看门狗首包 120s / 间隔 180s < 卡片
// 单次请求超时（默认 300s，Serenity 主用卡 600s）< Serenity / 盘前 /
// 盘后 / ETF 的 genCtx 10 分钟 < ChatWithContext 的 agent ctx 下限 900s。
// 看门狗永远最先触发，卡片超时与 genCtx 是后备防线；agent ctx 在这些
// 路径上基本 inert（只在交互聊天里作为整轮 900s 总上限才有意义）。
// 残余风险是经验值本身：若某网关批量缓冲 reasoning、超过 180s 才 flush
// 一次，慢思考流会被提前取消并换模型重做思考——降级本身无害（无实质
// 正文不拼接），只是浪费一次思考。阈值是包级变量，可调。
var (
	fallbackFirstChunkTimeout  = 120 * time.Second
	fallbackStreamStallTimeout = 180 * time.Second
)

// hasSubstantiveContent 判断本次尝试累计的正文里是否已有**可见的最终答案文本**。
//
// 与流末空答检测不同，这里刻意不用 cleanFinalContent 的“挽救”路径：
// cleanFinalContent 会在正文为空但原始文本很长时把 <think> 思考体本身
// 当作正文挽救回来，而思考体对用户不可见（agent_api 的 thinkFilter
// 会实时剥离），不能算“已输出内容”。这里只剥离 think 块与 agent-action
// 块，剩余非空白即为实质正文。
// 用途：流中途出错时，只有**没有**实质正文才允许降级重跑；一旦已有实质
// 正文，两个模型的部分输出绝不能无声拼成一段，必须把错误原样上抛。
func hasSubstantiveContent(raw string) bool {
	s := stripAllAgentActionBlocks(cleanThinkBlocks(raw))
	return strings.TrimSpace(s) != ""
}

// streamStallError 构造流式停滞错误：看门狗取消本次尝试后，底层一般只会
// 返回 context canceled，必须换成能说明真实原因的错误，否则上层与用户
// 看到的都是误导性的“上下文取消”。
func streamStallError(stallAfter time.Duration) error {
	return fmt.Errorf("模型流式响应停滞: %v 内未收到任何数据，疑似网关挂起或上游排队", stallAfter)
}

// fallbackGenerateTimeout 是 FallbackModelProvider.Generate 里单次非流式
// 尝试的硬上限（包级变量而非常量，便于单测收窄；生产值 180s）。
//
// 目前它是 inert 的：全仓唯一非流式调用方是 reflection_llm.go 的
// defaultLLMReflectionFunc，自带 60s ctx（比 180s 更紧），外层 deadline
// 会先到（remain < 180s 时 fbTimeout 直接取 remain，见 Generate）。
// 若将来有长报告走 Generate 且外层 ctx 比 180s 更宽，单次尝试会在
// 180s 被掐断后走下一个备用模型——这是有意的逐卡熔断，不是 bug；
// 真要放宽时调大本变量即可，不必改调用方。
var fallbackGenerateTimeout = 180 * time.Second

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
		// 动态调整为 fallbackGenerateTimeout (默认 180s)，且不超过外层上下文剩余时限
		fbTimeout := fallbackGenerateTimeout
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
//
// 降级触发规则（2026-11-22 修正，见根因说明）：
//   - 流出错时能否降级，不看“是否 yield 过任何 chunk”，而看“是否已产出
//     实质正文”（hasSubstantiveContent）。blades 的 OpenAI 适配层会对每个
//     SSE chunk 都 yield 一条消息，包括首包 role chunk（零 part）与
//     reasoning-only chunk（reasoning_content 不被映射，同样是零 part
//     消息）——推理模型在出正文前就会 yield 大量空消息，旧的 yieldedAny
//     判定会在“85 秒、0 字、中途流重置”时错误地拒绝降级。
//   - 一旦已有实质正文，绝不静默降级：两个模型的部分输出不能无声拼成
//     一段，错误原样上抛，由上层决定（Serenity 等研报路径会丢弃半份内容
//     并报错；交互对话同理不重试已送出的部分回答）。
//   - 有工具调用后出错同样不降级：重跑会重复执行工具（部分工具有副作用）。
//   - 流静默超过看门狗阈值（首包 120 秒/后续间隔 180 秒）时主动取消本次
//     尝试并按同一规则降级，不再等到 300 秒请求超时。
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

			// 本次尝试用独立的可取消 ctx：停滞看门狗只能取消本次尝试，
			// 绝不能取消外层 ctx（外层取消意味着用户/上层已主动放弃，
			// 此时不应再降级重跑）。
			attemptCtx, cancelAttempt := context.WithCancel(ctx)

			// ── 流式停滞看门狗 ──────────────────────────────────────
			// 任何 chunk 回调（哪怕是空消息）都会重置计时；只有“零字节
			// 静默”才会触发。触发后取消 attemptCtx，底层随即以错误结束，
			// 错误分支再按实质正文规则决定是否降级。
			// 阈值在主 goroutine 里一次读成局部变量再交给看门狗：
			// 看门狗 goroutine 绝不能直接读包级变量——它可能在调用方
			// 已经返回（测试里恢复阈值）后才被调度执行，直接读会与
			// 调用方的写形成数据竞争（-race 已抓到）。
			firstChunkTimeout := fallbackFirstChunkTimeout
			stallTimeout := fallbackStreamStallTimeout
			chunkSeen := make(chan struct{}, 1)
			watchdogDone := make(chan struct{})
			var stallFired atomic.Bool
			var stallAfter atomic.Int64 // 纳秒，记录触发时正在等的阈值
			stallAfter.Store(int64(firstChunkTimeout))
			go func() {
				timer := time.NewTimer(firstChunkTimeout)
				defer timer.Stop()
				for {
					select {
					case <-watchdogDone:
						return
					case <-chunkSeen:
						if !timer.Stop() {
							select {
							case <-timer.C:
							default:
							}
						}
						stallAfter.Store(int64(stallTimeout))
						timer.Reset(stallTimeout)
					case <-timer.C:
						stallFired.Store(true)
						logger.SugaredLogger.Warnf("⚠️ [FallbackModelProvider] 模型 [%s] 流式响应停滞 %v 未收到任何数据，主动取消本次尝试", curName, time.Duration(stallAfter.Load()))
						cancelAttempt()
						return
					}
				}
			}()
			stopWatchdog := func() {
				close(watchdogDone)
				cancelAttempt()
			}

			gen := prov.NewStreaming(attemptCtx, req)
			if gen == nil {
				stopWatchdog()
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
				// 任何回调都证明连接存活，重置看门狗计时。
				select {
				case chunkSeen <- struct{}{}:
				default:
				}

				if err != nil {
					// 看门狗触发的取消在底层只表现为 context canceled，
					// 换成能说明真实原因的错误再进入降级判定。
					if stallFired.Load() {
						logger.SugaredLogger.Warnf("⚠️ [FallbackModelProvider] 模型 [%s] 流式停滞取消，原始错误: %v", curName, err)
						err = streamStallError(time.Duration(stallAfter.Load()))
					}

					nextIdx := nextAlive(idx + 1)

					// ── 中途断流降级判定（2026-11-22 根因修复）───────
					//
					// 旧判定 `!yieldedAny` 是错的：blades 的 OpenAI 适配
					// 层对每个 SSE chunk 都 yield 一条消息，首包 role
					// chunk 与 reasoning-only chunk 都是零 part 的空
					// 消息——推理模型还没出一个正文词，yieldedAny 就
					// 已经是 true。生产事故“85 秒、0 字、流重置却不降
					// 级”正是这样发生的。
					//
					// 新判定：只有“尚未产出实质正文且无工具调用”才能
					// 降级重跑。已有实质正文时降级会把两个模型的部分
					// 输出无声拼成一段；已有工具调用时重跑会重复执行
					// 工具——这两种情况都必须把错误原样上抛。
					// 外层 ctx 已取消时也不降级（调用方已主动放弃）。
					canFallback := nextIdx < len(allProviders) &&
						ctx.Err() == nil &&
						!toolPartSeen &&
						!hasSubstantiveContent(contentBuf.String())
					if canFallback {
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

						if yieldedAny {
							logger.SugaredLogger.Warnf("⚠️ [FallbackModelProvider] 模型 [%s] 流式中途失败（仅空/reasoning chunk，无实质正文）: %v, 自动降级切换至 [%s]...", curName, err, nextName)
						} else {
							logger.SugaredLogger.Warnf("⚠️ [FallbackModelProvider] 模型 [%s] 流式调用首包失败: %v, 自动降级切换至 [%s]...", curName, err, nextName)
						}
						if notifier != nil {
							notifier(CleanFallbackModelName(curName), CleanFallbackModelName(nextName), err)
						}
						return false // 停止当前 generator
					}
					// 已有实质正文/工具调用或无备选模型，正常将错误 yield 出去
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
			stopWatchdog()

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
			// 注意：这里必须用 hasSubstantiveContent 而不是 cleanFinalContent——
			// cleanFinalContent 的“挽救”路径会把纯 think 长文本当作正文，
			// 而 think 体对用户不可见（thinkFilter 会实时剥离），纯 think
			// 长流会被误判为“有正文”而不降级，用户却什么都看不到。
			if !failed && !toolPartSeen && !hasSubstantiveContent(contentBuf.String()) {
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

// isFallbackCandidate reports whether cfg may serve as a fallback for
// currentConfig, applying the shared exclusions of the fallback chain:
// skip the primary itself, empty API keys, models in the 30-minute global
// cooldown pool, and — when the primary is itself in cooldown — configs
// that share the primary's ApiKey+BaseUrl (retrying them would repeat the
// same auth/quota failure). Shared by BuildFallbackProvidersFromDB and
// FirstFallbackAIConfig so the chain order and the Serenity retry picker
// can never drift apart.
func isFallbackCandidate(cfg, currentConfig data.AIConfig, primaryKey string, primaryInCooldown bool) bool {
	if cfg.ID == currentConfig.ID || strings.TrimSpace(cfg.ApiKey) == "" {
		return false
	}
	// 冷却键是“模型名 + 网关（BaseUrl + 凭证指纹）”，同名不同网关的卡片
	// 不再互相牵连（旧实现还按裸模型名查 IsModelInCooldown(cfg.ModelName)，
	// A 网关鉴权失败会把 B 网关同名卡一并排除 30 分钟）。
	if IsModelInCooldown(FallbackProviderName(cfg)) {
		return false
	}
	// 若主模型已因鉴权/用量耗尽熔断，跳过共享相同 ApiKey+BaseUrl 的备用配置，避免重复失败空转。
	// 注意：这条跳过成立的前提是熔断本身分类正确（见 IsPermanentAuthError）。
	// 容量型 429（如 TokenLab "at capacity"）是按模型/按时刻的瞬时问题，
	// 同一网关换一个模型完全可能成功——它绝不能让主模型进熔断，否则
	// 同网关的备用模型会被这里一并排除，整条链名存实亡。
	if primaryInCooldown {
		cfgKey := fmt.Sprintf("%s|%s", strings.TrimSpace(cfg.ApiKey), strings.TrimSpace(cfg.BaseUrl))
		if cfgKey == primaryKey {
			return false
		}
	}
	return true
}

// FirstFallbackAIConfig returns the first available fallback card for
// currentConfig, in registry order, using the same exclusions as
// BuildFallbackProvidersFromDB — but without instantiating a provider.
// It is the card selector for the Serenity whole-round retry: the retry
// round re-runs ChatWithContext with this card as primary, so the card
// gets its own fallback chain there. Selection is by registry/cooldown
// state only, never a hard-coded model; a card that later fails to
// connect surfaces as the retry round's own error.
func FirstFallbackAIConfig(currentConfig data.AIConfig) (data.AIConfig, bool) {
	settingConfig := data.GetSettingConfig()
	if settingConfig == nil || len(settingConfig.AiConfigs) == 0 {
		return data.AIConfig{}, false
	}
	primaryKey := fmt.Sprintf("%s|%s", strings.TrimSpace(currentConfig.ApiKey), strings.TrimSpace(currentConfig.BaseUrl))
	primaryInCooldown := IsModelInCooldown(FallbackProviderName(currentConfig))
	for _, cfgPtr := range settingConfig.AiConfigs {
		if cfgPtr == nil {
			continue
		}
		cfg := *cfgPtr
		if isFallbackCandidate(cfg, currentConfig, primaryKey, primaryInCooldown) {
			return cfg, true
		}
	}
	return data.AIConfig{}, false
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
	primaryInCooldown := IsModelInCooldown(FallbackProviderName(currentConfig))

	for _, cfgPtr := range settingConfig.AiConfigs {
		if cfgPtr == nil {
			continue
		}
		cfg := *cfgPtr
		if !isFallbackCandidate(cfg, currentConfig, primaryKey, primaryInCooldown) {
			continue
		}

		provider, err := createChatModel(ctx, cfg)
		if err == nil && provider != nil {
			fallbackProviders = append(fallbackProviders, provider)
			fallbackNames = append(fallbackNames, FallbackProviderName(cfg))
		}
	}

	return fallbackProviders, fallbackNames
}
