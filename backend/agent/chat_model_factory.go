package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/logger"

	"github.com/duke-git/lancet/v2/strutil"
	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/contrib/anthropic"
	"github.com/go-kratos/blades/contrib/gemini"
	"github.com/go-kratos/blades/contrib/openai"
	openaiOption "github.com/openai/openai-go/v3/option"
	"google.golang.org/genai"
)

var (
	modelProviderCache sync.Map
)

type cachedProvider struct {
	provider blades.ModelProvider
}

// modelProviderCacheLimit 是 modelProviderCache 的条目上限：缓存键含
// UpdatedAt，每次保存卡片都会产生一个新键、旧键永不失效；不设上限则
// 每次编辑都永久泄漏一个 provider（内含 http client 等）。上限 64 远
// 高于注册表卡片数；超限时整表清空后重建——重建只是构造客户端，无
// 网络开销，简单且安全。计数-清空之间有良性竞态（两个 goroutine 可能
// 各清一次），最坏只是多一次重建。
//
// 必须是 atomic.Int64 而不是普通变量：测试会临时收窄它，而
// createChatModel 可能被 ChatMemoryService 的异步摘要 goroutine
// （summarizeHistoryAsync → defaultLLMReflectionFunc）调用——普通
// 变量会在测试恢复限额时与该 goroutine 的读形成数据竞争（-race 已
// 抓到：TestCompressHistoryWritesRollingSummary 的异步 goroutine 读
// 限额 vs 本包测试 defer 恢复限额的写）。
var modelProviderCacheLimit atomic.Int64

func init() {
	modelProviderCacheLimit.Store(64)
}

func modelProviderCacheSize() int {
	n := 0
	modelProviderCache.Range(func(_, _ any) bool {
		n++
		return true
	})
	return n
}

func storeCachedProvider(key string, provider blades.ModelProvider) {
	if modelProviderCacheSize() >= int(modelProviderCacheLimit.Load()) {
		modelProviderCache.Range(func(k, _ any) bool {
			modelProviderCache.Delete(k)
			return true
		})
	}
	modelProviderCache.Store(key, cachedProvider{provider: provider})
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
	// 这里的 timeout 是**每一次 LLM 请求**的上限（不是整个 Agent 运行的上限）。
	//
	// 踩坑记录：曾建议用户把卡片「请求超时」调到 120 秒以“快速失败”，结果
	// 慢速推理模型的**单次**响应就要 200~400 秒，于是每一次调用都被自己的
	// 配置掐断，用户看到 `context deadline exceeded`，且旧提示语还把它
	// 误报为“服务商繁忙”。低于 120 秒几乎必然误杀正常请求，故在此留痕。
	if timeout < 120*time.Second {
		logger.SugaredLogger.Warnf(
			"[ModelFactory] 卡片「%s」的单次请求超时仅 %v，低于常见推理模型的单次响应耗时，"+
				"极可能出现 context deadline exceeded。建议在「高级调度参数」调大至 300 秒以上。",
			aiConfig.Name, timeout)
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

	// OpenAI 兼容分支的构造器：DeepSeek / VolcArk / DashScope /
	// OpenRouter / Ollama 与裸 OpenAI 兼容网关全部走同一条路（openai-go
	// 以 BaseURL 为基准拼 /chat/completions）。此前 switch 里只有
	// Anthropic 与 Gemini 两个显式 case，其余 6 种 provider 常量（含
	// providerDeepSeek）全靠 default 兜底——detect 能判出 DeepSeek，
	// switch 却没有它的 case，是死分支。现改为显式列出，default 只作
	// 防御性兜底（detect 只返回上述 8 个值，理论不可达），行为零变化。
	// 注意 Ollama 也在其中：它今天同样走 default 的 OpenAI 兼容构造
	// （BaseURL 原样拼 /chat/completions），本次只显式化、不改行为。
	buildOpenAICompatible := func() blades.ModelProvider {
		// max_tokens 是输出 token 上限，不是上下文窗口。
		// blades/contrib/openai 中 MaxOutputTokens 会自动映射为 params.MaxCompletionTokens。
		// 对所有 OpenAI 兼容模型统一 cap 到 32768，兼顾安全与实用性。
		outputTok := maxTok
		if outputTok <= 0 || outputTok > 32768 {
			outputTok = 32768
		}
		extraFields := map[string]any{}
		if aiConfig.Thinking {
			extraFields["reasoning_effort"] = "medium"
		}
		reqOpts := []openaiOption.RequestOption{
			sanitizeToolsMiddleware(),
		}
		if timeout > 0 {
			reqOpts = append(reqOpts, openaiOption.WithRequestTimeout(timeout))
		}
		cfg := openai.Config{
			APIKey:          aiConfig.ApiKey,
			BaseURL:         baseURL,
			Temperature:     temperature,
			MaxOutputTokens: int64(outputTok),
			ExtraFields:     extraFields,
			RequestOptions:  reqOpts,
		}
		return openai.NewModel(aiConfig.ModelName, cfg)
	}

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

	case providerOpenAICompatible, providerVolcArk, providerDashScope, providerOpenRouter, providerOllama, providerDeepSeek:
		provider = buildOpenAICompatible()

	default:
		provider = buildOpenAICompatible()
	}

	if err != nil {
		return nil, err
	}

	wrapped := NewModelProviderWrapper(provider, &aiConfig)
	storeCachedProvider(key, wrapped)
	return wrapped, nil
}

// sanitizeToolsMiddleware 解决部分严格模型服务商与网关的请求参数兼容性问题：
// 1. 消除 max_tokens 与 max_completion_tokens 冲突（杜绝网关报 duplicate field `max_tokens`）
// 2. 移除部分模型不兼容的 reasoning_effort="none" 或 thinking.type="disabled"（如 MiniMax-M3.1 报错 2013）
// 3. 解决部分严格模型服务商（如 function call parameters 必须包含有效 object 和 properties）的问题
func sanitizeToolsMiddleware() openaiOption.RequestOption {
	return openaiOption.WithMiddleware(func(req *http.Request, nxt openaiOption.MiddlewareNext) (*http.Response, error) {
		if req.Body == nil {
			return nxt(req)
		}
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return nxt(req)
		}
		req.Body.Close()

		var bodyMap map[string]any
		if err := json.Unmarshal(bodyBytes, &bodyMap); err == nil {
			modified := false

			// 1. 消除 max_tokens 与 max_completion_tokens 冲突
			if _, hasTokens := bodyMap["max_tokens"]; hasTokens {
				if _, hasComp := bodyMap["max_completion_tokens"]; hasComp {
					delete(bodyMap, "max_tokens")
					modified = true
				}
			}

			// 2. 清理 MiniMax-M3.1 等自适应思考模型不兼容的 reasoning_effort="none" 或 thinking.type="disabled"
			if rEffort, ok := bodyMap["reasoning_effort"].(string); ok && rEffort == "none" {
				delete(bodyMap, "reasoning_effort")
				modified = true
			}
			if th, ok := bodyMap["thinking"].(map[string]any); ok {
				if tType, ok := th["type"].(string); ok && tType == "disabled" {
					delete(bodyMap, "thinking")
					modified = true
				}
			}

			// 3. 规范化 tools parameters 结构
			if rawTools, ok := bodyMap["tools"].([]any); ok && len(rawTools) > 0 {
				for _, t := range rawTools {
					toolMap, ok := t.(map[string]any)
					if !ok {
						continue
					}
					fnMap, ok := toolMap["function"].(map[string]any)
					if !ok {
						continue
					}
					params, hasParams := fnMap["parameters"]
					if !hasParams || params == nil {
						fnMap["parameters"] = map[string]any{
							"type":       "object",
							"properties": map[string]any{},
						}
						modified = true
					} else if pMap, isMap := params.(map[string]any); isMap {
						if len(pMap) == 0 {
							pMap["type"] = "object"
							pMap["properties"] = map[string]any{}
							modified = true
						} else if _, hasProps := pMap["properties"]; !hasProps && pMap["type"] == "object" {
							pMap["properties"] = map[string]any{}
							modified = true
						}
					}
				}
			}
			if modified {
				bodyBytes, _ = json.Marshal(bodyMap)
			}
		}

		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		req.ContentLength = int64(len(bodyBytes))

		resp, err := nxt(req)
		if resp != nil && resp.StatusCode >= 400 {
			respBytes, rErr := io.ReadAll(resp.Body)
			if rErr == nil {
				resp.Body.Close()
				resp.Body = io.NopCloser(bytes.NewReader(respBytes))
				logger.SugaredLogger.Errorf("❌ [HTTP %d] URL: %s | Resp: %s | Req: %s",
					resp.StatusCode, req.URL.String(), string(respBytes), string(bodyBytes))
			}
		}

		// 处理流式响应：清除网关可能附带的 : ping 等 SSE 注释与空行，防止 openai-go 抛出 unexpected end of JSON input
		if resp != nil && resp.Body != nil && resp.StatusCode < 300 {
			cType := strings.ToLower(resp.Header.Get("Content-Type"))
			isStream := strings.Contains(cType, "event-stream")
			if !isStream && bodyMap != nil {
				if sVal, ok := bodyMap["stream"].(bool); ok && sVal {
					isStream = true
				}
			}
			if isStream {
				resp.Body = newSSECleanReader(resp.Body)
			}
		}

		return resp, err
	})
}

// sseCleanReader 过滤 SSE 注释行（如 ": ping"、": keepalive"）与孤立空行，
// 防止 openai-go/v3 的 eventStreamDecoder 分发空 Data 事件导致 json.Unmarshal 崩溃
// 报 "unexpected end of JSON input"。
type sseCleanReader struct {
	reader  *bufio.Reader
	closer  io.Closer
	hasData bool
	buf     bytes.Buffer
}

func newSSECleanReader(rc io.ReadCloser) io.ReadCloser {
	return &sseCleanReader{
		reader: bufio.NewReader(rc),
		closer: rc,
	}
}

func (r *sseCleanReader) Read(p []byte) (int, error) {
	for r.buf.Len() == 0 {
		line, err := r.reader.ReadBytes('\n')
		if len(line) > 0 {
			trimmed := bytes.TrimRight(line, "\r\n")
			// 1. 如果是 SSE 注释（以 ':' 开头），完全跳过
			if len(trimmed) > 0 && trimmed[0] == ':' {
				continue
			}
			// 2. 如果是空行（事件块分隔符）
			if len(trimmed) == 0 {
				// 仅在当前事件块中确实包含有效 'data:' 时才下发事件分隔符
				if r.hasData {
					r.buf.Write(line)
					r.hasData = false
				}
				continue
			}
			// 3. 标记是否包含 data 行
			if bytes.HasPrefix(trimmed, []byte("data:")) {
				r.hasData = true
			}
			r.buf.Write(line)
		}
		if err != nil {
			if r.buf.Len() > 0 {
				break
			}
			return 0, err
		}
	}
	return r.buf.Read(p)
}

func (r *sseCleanReader) Close() error {
	return r.closer.Close()
}
