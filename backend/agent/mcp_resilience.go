// Package agent 的 MCP 工具调用韧性增强模块（P1.2）。
//
// 设计目标：在 wrapMCPToolHandler（已存在）基础上加入：
//   1. 错误分类：把"瞬时网络错误"和"业务错误"区分开
//   2. 智能重试：仅对可重试错误做指数退避重试，避免无谓浪费
//   3. 熔断：连续失败达到阈值后快速失败，避免拖慢整个 agent
//   4. 错误脱敏：避免把 token / bearer 等敏感信息回传给 LLM
//
// 设计原则：
//   - 与现有 wrapMCPToolHandler 并存：旧路径保留为 deprecated wrapper
//   - 默认 ON（通过 circuitbreaker.Get 全局注册），但 P1 灰度要求允许临时禁用
//   - 所有可调参数（重试次数、退避、熔断阈值）都是 const，可在一个文件里改
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"blades-stock/backend/circuitbreaker"
	"blades-stock/backend/logger"

	bladesTools "github.com/go-kratos/blades/tools"
)

// MCP 韧性配置（集中在此处方便未来调参）
const (
	// MaxRetryAttempts 单次请求的最大重试次数（不含首次）
	MaxRetryAttempts = 3

	// InitialBackoff 第一次重试前的等待
	InitialBackoff = 200 * time.Millisecond

	// BackoffFactor 指数退避倍率
	BackoffFactor = 2

	// CircuitBreakerNamePrefix 断路器注册名前缀，避免与其它数据源断路器冲突
	CircuitBreakerNamePrefix = "mcp:"

	// CircuitFailureThreshold 连续失败多少次触发熔断
	CircuitFailureThreshold = 3

	// CircuitCooldown 熔断后冷却时长
	CircuitCooldown = 30 * time.Second
)

// ErrKind 是 MCP 工具错误的分类。
// 分类粒度参考 HTTP 状态码语义，便于未来按分类聚合埋点。
type ErrKind int

const (
	// ErrKindUnknown 未分类错误，保守处理为不重试
	ErrKindUnknown ErrKind = iota
	// ErrKindTransient 瞬时错误（timeout / connection refused / EOF / 5xx）
	// 触发重试 + 进入熔断计数
	ErrKindTransient
	// ErrKindAuth 认证/授权错误（401/403），不重试（重试无意义），但计熔断失败
	ErrKindAuth
	// ErrKindNotFound 资源不存在（404），不重试，不计熔断
	ErrKindNotFound
	// ErrKindBadRequest 请求参数错误（400），不重试，不计熔断
	ErrKindBadRequest
)

// transientPatterns 命中即视为瞬时错误。
// 注意：keywords 必须 lowercase 化后再 Contains。
var transientPatterns = []string{
	"timeout", "timed out", "deadline exceeded",
	"connection refused", "connection reset", "broken pipe",
	"eof", "i/o timeout", "temporarily unavailable",
	"5xx", "internal server error", "bad gateway", "service unavailable", "gateway timeout",
	"rpc error", "transport", "dial tcp",
}

var authPatterns = []string{
	"401", "unauthorized", "403", "forbidden", "invalid token",
	"authentication failed", "missing credentials",
}

var notFoundPatterns = []string{
	"404", "not found", "no such file",
}

var badRequestPatterns = []string{
	"400", "bad request", "invalid argument", "missing required",
	"unmarshal", "invalid json", "invalid input",
}

// classifyMCPErr 把错误消息归类到 ErrKind。
// 实现要点：
//   - 仅基于 err.Error() 字符串（HTTP 错误信息通常会带状态码或关键词）
//   - 优先级：Auth > NotFound > BadRequest > Transient > Unknown
//   - err == nil 返回 ErrKindUnknown（调用方按"成功"路径处理）
func classifyMCPErr(err error) ErrKind {
	if err == nil {
		return ErrKindUnknown
	}
	msg := strings.ToLower(err.Error())
	for _, p := range authPatterns {
		if strings.Contains(msg, p) {
			return ErrKindAuth
		}
	}
	for _, p := range notFoundPatterns {
		if strings.Contains(msg, p) {
			return ErrKindNotFound
		}
	}
	for _, p := range badRequestPatterns {
		if strings.Contains(msg, p) {
			return ErrKindBadRequest
		}
	}
	for _, p := range transientPatterns {
		if strings.Contains(msg, p) {
			return ErrKindTransient
		}
	}
	return ErrKindUnknown
}

// isRetryable 决定该类错误是否值得重试。
// 设计：仅 Transient 重试；Auth/NotFound/BadRequest 重试无意义；Unknown 保守不重试。
func isRetryable(kind ErrKind) bool {
	return kind == ErrKindTransient
}

// countsTowardBreaker 决定该类错误是否计入熔断失败计数。
// 设计：Auth 计入（认证问题持续会让所有调用挂掉），其它业务错误不计入。
func countsTowardBreaker(kind ErrKind) bool {
	return kind == ErrKindTransient || kind == ErrKindAuth
}

// backoff 计算第 attempt 次重试前的等待时间（attempt 从 0 开始）。
// 序列：200ms, 400ms, 800ms（MaxRetryAttempts=3 时）。
func backoff(attempt int) time.Duration {
	d := InitialBackoff
	for i := 0; i < attempt; i++ {
		d *= time.Duration(BackoffFactor)
	}
	return d
}

// sensitiveKeywords 是需要脱敏的字段名集合（大小写不敏感）。
// 采用编程式扫描而非单一正则，以适配多种格式：
//   - "Authorization: Bearer xyz..."    （HTTP header）
//   - "api_key=sk-1234567890"            （URL query / env）
//   - `{"Authorization": "Bearer ..."}`  （JSON）
//   - "password=mySecret123 in config"   （配置）
var sensitiveKeywords = []string{
	"token", "bearer", "api_key", "apikey", "api-key",
	"secret", "password", "authorization",
	"TUSHARE_TOKEN", "WUDAO_BEARER_TOKEN", "OPENAI_API_KEY",
}

// sanitizeWordBoundaryRegex 检测单词边界，避免误伤（例：xauthortokeny 不应被命中）。
// 构造一个匹配 `\b(keyword)\b` 的正则，编译时检查每个关键字。
func buildKeywordRegex() *regexp.Regexp {
	parts := make([]string, 0, len(sensitiveKeywords))
	for _, kw := range sensitiveKeywords {
		// Go regexp RE2 不支持 (?i) 与 \b 跨多 alternate 的任意性，这里手动为每个加 (?i)
		parts = append(parts, "(?i)\\b"+regexp.QuoteMeta(kw)+"\\b")
	}
	return regexp.MustCompile(strings.Join(parts, "|"))
}

var sensitiveKeywordRe = buildKeywordRegex()

// sanitizeError 把错误消息中的敏感字段替换为 ***。
// 仅在最终向 LLM 返回的 JSON 中调用，避免日志里也丢上下文。
// 注意：日志（logger.Warnf）仍记录原始错误，便于运维排查。
//
// 实现要点：
//   - 不试图用一条大正则匹配所有格式（JSON / header / URL 参数变体太多）
//   - 而是：定位关键字 → 从关键字末尾往后吞「引号 / 冒号 / 等号 / 空白」
//     → 再吞「一个值（不以空白 / 逗号 / 右括号 / 引号结尾的连续段）」
//   - 一次性扫描，避免重复匹配已脱敏区域
func sanitizeError(msg string) string {
	matches := sensitiveKeywordRe.FindAllStringSubmatchIndex(msg, -1)
	if len(matches) == 0 {
		return msg
	}

	var sb strings.Builder
	last := 0
	for _, m := range matches {
		start, end := m[0], m[1]
		if start < last {
			continue // 已在上一次吞并范围内，跳过
		}
		sb.WriteString(msg[last:start])
		// 写出关键字 + =***（关键词本身保留以便调试）
		sb.WriteString(msg[start:end])
		sb.WriteString("=***")

		// 从 end 开始跳过 [\"':=\\s]*
		i := end
		for i < len(msg) && (msg[i] == '"' || msg[i] == '\'' || msg[i] == ':' || msg[i] == '=' || msg[i] == ' ' || msg[i] == '\t') {
			i++
		}
		// 吞值：以空白 / 逗号 / 右括号 / 右引号 之外的字符为一段
		valStart := i
		for i < len(msg) && msg[i] != ' ' && msg[i] != '\t' && msg[i] != '\n' && msg[i] != ',' && msg[i] != '}' && msg[i] != '"' && msg[i] != '\'' {
			i++
		}
		// 如果吞到的值长度 < 6，仍视为匹配（避免太短的 token 被漏掉）
		if i-valStart == 0 {
			// 没吞到任何值，不推进 last，仅写完关键字部分
			last = end
		} else {
			last = i
		}
	}
	sb.WriteString(msg[last:])
	return sb.String()
}

// mcpErrorJSON 构造一个对 LLM 友好的 JSON 错误响应。
// LLM 看到这个 JSON 会自己理解"工具失败了"并切换到其它工具或重试参数。
func mcpErrorJSON(toolName, serverName, rawErrMsg string) string {
	sanitized := sanitizeError(rawErrMsg)
	resp := map[string]interface{}{
		"error":   true,
		"tool":    toolName,
		"server":  serverName,
		"message": fmt.Sprintf("⚠️ MCP [%s] 工具调用失败: %s", serverName, sanitized),
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

// circuitOpenJSON 当断路器 Open 时立即返回，告知 LLM "数据源暂时挂掉，请换工具或稍后再试"。
func circuitOpenJSON(toolName, serverName string) string {
	resp := map[string]interface{}{
		"error":          true,
		"tool":           toolName,
		"server":         serverName,
		"message":        fmt.Sprintf("⚠️ MCP [%s] 暂时不可用（断路器开启），请尝试其它数据源或稍后重试", serverName),
		"circuit_open":   true,
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

// wrapMCPToolHandlerV2 是 wrapMCPToolHandler 的升级版，加入：
//   - 错误分类（仅 Transient 重试）
//   - 指数退避
//   - 熔断（circuitbreaker 包，per-server 注册）
//   - 错误脱敏（仅返回给 LLM 的 JSON 走 sanitize）
//
// 设计要点：
//   - 行为对 LLM 透明：成功路径返回值不变
//   - 失败路径：旧版本返回单条 JSON 错误；新版本也是单条 JSON 错误，但新增了 tool / server 字段便于 LLM 诊断
//   - 单次执行总耗时上限：MaxRetryAttempts * (InitialBackoff * BackoffFactor^MaxRetryAttempts)
//     默认：3 * 1.4s ≈ 4.2s（不含每次实际调用超时）— 与现有 5s 调用超时配合不会把外层 ctx.Done() 撞穿
// configuredBreakers 追踪哪些 serverName 已应用过默认配置，避免重复 Configure
// 覆盖测试 / 启动期预调阈值的场景。
var configuredBreakers sync.Map // map[string]struct{}

func wrapMCPToolHandlerV2(original bladesTools.Tool, serverName string) bladesTools.Tool {
	// V2 默认对每个 serverName 调一次 Configure，应用全局默认值。
	// 后续调用：直接 Get，不覆盖（避免重置已调阈值的实例）。
	breakerName := CircuitBreakerNamePrefix + serverName
	var breaker *circuitbreaker.CircuitBreaker
	if _, already := configuredBreakers.Load(breakerName); already {
		breaker = circuitbreaker.Get(breakerName)
	} else {
		breaker = circuitbreaker.Configure(breakerName, circuitbreaker.Config{
			Name:             breakerName,
			FailureThreshold: CircuitFailureThreshold,
			Cooldown:         CircuitCooldown,
		})
		configuredBreakers.Store(breakerName, struct{}{})
	}

	toolName := original.Name()

	return bladesTools.NewTool(
		toolName,
		original.Description(),
		bladesTools.HandleFunc(func(ctx context.Context, input string) (string, error) {
			// 1. 熔断检查：失败 N 次后直接快速失败
			if !breaker.Allow() {
				logger.SugaredLogger.Debugf("MCP 断路器开启，跳过 %s/%s", serverName, toolName)
				return circuitOpenJSON(toolName, serverName), nil
			}

			var lastErr error
			var lastErrKind ErrKind
			attempts := MaxRetryAttempts + 1 // 含首次

			for attempt := 0; attempt < attempts; attempt++ {
				// 退避（仅重试前等待；首次不等待）
				if attempt > 0 {
					wait := backoff(attempt - 1)
					select {
					case <-ctx.Done():
						return mcpErrorJSON(toolName, serverName, "context cancelled during backoff"), nil
					case <-time.After(wait):
					}
				}

				result, err := original.Handle(ctx, input)
				if err == nil {
					breaker.Success()
					return result, nil
				}
				lastErr = err
				lastErrKind = classifyMCPErr(err)

				// 仅 Transient 重试；其它直接跳出
				if !isRetryable(lastErrKind) {
					break
				}
				logger.SugaredLogger.Debugf("MCP transient error on %s/%s (attempt %d/%d): %v",
					serverName, toolName, attempt+1, attempts, err)
			}

			// 重试完 / 不可重试：记录熔断失败（如适用）并返回 JSON
			logger.SugaredLogger.Warnf("MCP 工具错误 [%s/%s kind=%v attempts=%d]: %v",
				serverName, toolName, lastErrKind, attempts, lastErr)
			if countsTowardBreaker(lastErrKind) {
				breaker.Failure()
			}
			return mcpErrorJSON(toolName, serverName, lastErr.Error()), nil
		}),
		bladesTools.WithInputSchema(original.InputSchema()),
		bladesTools.WithOutputSchema(original.OutputSchema()),
	)
}

// wrapMCPToolHandlerCompatOld 是为了在 main file 保留旧 wrapMCPToolHandler 时不冲突。
// 旧版本（agent.go 中的 wrapMCPToolHandler）仍然存在，本文件不再重复定义。
// 调用方现在应使用 wrapMCPToolHandlerV2。

// 防止 errors 包未被引用（未来若需要 sentinel error 校验可用）
var _ = errors.Is