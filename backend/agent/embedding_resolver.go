package agent

import (
	"crypto/sha1"
	"fmt"
	"strings"
	"sync"

	"blades-stock/backend/data"
	"blades-stock/backend/logger"
)

var (
	embedPingMu    sync.Mutex
	embedPingCache = map[string]bool{}
)

// embeddingCandidate is a resolved (url, apikey, model) triple before any
// network verification.
type embeddingCandidate struct {
	baseURL string
	apiKey  string
	model   string
	source  string
}

// ResolveEmbeddingService returns the embedding service used for semantic
// retrieval / skill matching / long-term memory.
//
// Resolution order:
//  1. Global standalone vector config (settings): explicit URL wins directly.
//  2. Global enabled but URL empty:
//     a. any AI config explicitly configured for embedding (EmbeddingEnabled + model);
//     b. provider-aware defaults for known embedding-capable providers
//        (Ollama / DashScope / VolcArk / OpenAI / SiliconFlow);
//  3. Selected chat config's own embedding settings;
//  4. Any AI config with embedding settings.
//
// Chat-only providers (NVIDIA / MiniMax / generic OpenAI-compatible) are never
// reused as embedding endpoints. The resolved endpoint is health-checked once
// (cached per endpoint) and returns nil on failure so the caller degrades to
// keyword matching instead of 404-spamming every request.
func ResolveEmbeddingService(settingConfig *data.SettingConfig, selected *data.AIConfig) *EmbeddingService {
	c := resolveEmbeddingCandidate(settingConfig, selected)
	if c == nil {
		if settingConfig != nil && settingConfig.EmbeddingEnabled {
			logger.SugaredLogger.Warn("向量模型已启用，但找不到可用的 Embedding 端点（纯 Chat 提供商不可复用），已降级为关键词匹配")
		}
		return nil
	}
	svc := NewEmbeddingService(c.baseURL, c.apiKey, c.model)
	return verifyEmbedding(svc)
}

// resolveEmbeddingCandidate resolves the embedding (url, apikey, model) purely
// from settings without any network call.
func resolveEmbeddingCandidate(settingConfig *data.SettingConfig, selected *data.AIConfig) *embeddingCandidate {
	if settingConfig == nil {
		return nil
	}

	if settingConfig.EmbeddingEnabled && strings.TrimSpace(settingConfig.EmbeddingModelName) != "" {
		if url := strings.TrimSpace(settingConfig.EmbeddingBaseUrl); url != "" {
			return &embeddingCandidate{
				baseURL: url,
				apiKey:  settingConfig.EmbeddingApiKey,
				model:   sanitizeEmbeddingModel(url, settingConfig.EmbeddingModelName),
				source:  "全局向量配置",
			}
		}
		return resolveEmbeddingEndpoint(settingConfig, selected, sanitizeEmbeddingModel(settingConfig.EmbeddingBaseUrl, settingConfig.EmbeddingModelName))
	}

	if selected != nil && selected.EmbeddingEnabled && strings.TrimSpace(selected.EmbeddingModelName) != "" {
		url := strings.TrimSpace(selected.EmbeddingBaseUrl)
		if url == "" {
			url = selected.BaseUrl
		}
		return &embeddingCandidate{
			baseURL: url,
			apiKey:  selected.ApiKey,
			model:   sanitizeEmbeddingModel(url, selected.EmbeddingModelName),
			source:  "所选 AI 配置的 Embedding 设置",
		}
	}

	for _, cfg := range settingConfig.AiConfigs {
		if cfg != nil && cfg.EmbeddingEnabled && strings.TrimSpace(cfg.EmbeddingModelName) != "" {
			url := strings.TrimSpace(cfg.EmbeddingBaseUrl)
			if url == "" {
				url = cfg.BaseUrl
			}
			return &embeddingCandidate{
				baseURL: url,
				apiKey:  cfg.ApiKey,
				model:   sanitizeEmbeddingModel(url, cfg.EmbeddingModelName),
				source:  "扫描到的 Embedding 配置",
			}
		}
	}

	return nil
}

// sanitizeEmbeddingModel 自动将过期/下线的向量模型平滑升级至平台活跃主力模型
func sanitizeEmbeddingModel(baseURL, model string) string {
	model = strings.TrimSpace(model)
	baseLower := strings.ToLower(baseURL)
	if strings.Contains(baseLower, "nvidia.com") {
		// nv-embed-v1 与 nv-embedqa-e5-v5 均已于 2026 年下线 (HTTP 410)
		if model == "nvidia/nv-embed-v1" || model == "nvidia/nv-embedqa-e5-v5" || model == "baai/bge-m3" || model == "" {
			return "nvidia/nemotron-3-embed-1b"
		}
	}
	return model
}

// resolveEmbeddingEndpoint picks an endpoint when the global vector config is
// enabled without an explicit URL.
func resolveEmbeddingEndpoint(settingConfig *data.SettingConfig, selected *data.AIConfig, globalModel string) *embeddingCandidate {
	for _, cfg := range settingConfig.AiConfigs {
		if cfg != nil && cfg.EmbeddingEnabled && strings.TrimSpace(cfg.EmbeddingModelName) != "" {
			url := strings.TrimSpace(cfg.EmbeddingBaseUrl)
			if url == "" {
				url = cfg.BaseUrl
			}
			return &embeddingCandidate{
				baseURL: url,
				apiKey:  cfg.ApiKey,
				model:   cfg.EmbeddingModelName,
				source:  "扫描到的 Embedding 配置",
			}
		}
	}

	baseURL, model, ok := providerEmbeddingDefaults(selected)
	if !ok || baseURL == "" {
		return nil
	}
	if model == "" {
		model = globalModel
	}
	if model == "" {
		return nil
	}
	return &embeddingCandidate{
		baseURL: baseURL,
		apiKey:  selected.ApiKey,
		model:   model,
		source:  "provider 感知派生",
	}
}

// providerEmbeddingDefaults returns an OpenAI-compatible embeddings endpoint and
// a default model for known embedding-capable providers. ok=false means the
// provider is chat-only and must NOT be reused as an embedding endpoint.
func providerEmbeddingDefaults(selected *data.AIConfig) (baseURL, model string, ok bool) {
	if selected == nil {
		return "", "", false
	}
	base := normalizeBaseURL(selected.BaseUrl)
	if base == "" {
		return "", "", false
	}
	baseLower := strings.ToLower(base)

	switch detectChatModelProvider(baseLower, selected.ModelName) {
	case providerOllama:
		return base, "", true // keep the globally configured model (e.g. bge-m3)
	case providerDashScope:
		return base, "text-embedding-v3", true
	case providerVolcArk:
		return base, "doubao-embedding-large", true
	case providerOpenAICompatible:
		switch {
		case strings.Contains(baseLower, "openai.com"):
			return base, "text-embedding-3-small", true
		case strings.Contains(baseLower, "siliconflow"):
			return base, "BAAI/bge-m3", true
		}
	}
	return "", "", false
}

// verifyEmbedding health-checks the endpoint once per (url|model|apikey) and
// caches the verdict, so unreachable endpoints degrade to nil instead of
// spamming errors on every request.
func verifyEmbedding(svc *EmbeddingService) *EmbeddingService {
	if svc == nil {
		return nil
	}
	key := embeddingPingKey(svc)

	embedPingMu.Lock()
	ok, cached := embedPingCache[key]
	embedPingMu.Unlock()
	if cached {
		if !ok {
			return nil
		}
		return svc
	}

	if err := svc.Ping(); err != nil {
		setEmbeddingVerdict(key, false)
		logger.SugaredLogger.Warnf("向量服务健康检查失败(%s)，已降级为关键词匹配: %v", svc.Describe(), err)
		return nil
	}
	setEmbeddingVerdict(key, true)
	logger.SugaredLogger.Infof("向量服务健康检查通过: %s", svc.Describe())
	return svc
}

func embeddingPingKey(svc *EmbeddingService) string {
	apiHash := ""
	if svc.apiKey != "" {
		sum := sha1.Sum([]byte(svc.apiKey))
		apiHash = fmt.Sprintf("%x", sum[:8])
	}
	return svc.embeddingsURL + "|" + svc.modelName + "|" + apiHash
}

func setEmbeddingVerdict(key string, ok bool) {
	embedPingMu.Lock()
	defer embedPingMu.Unlock()
	if len(embedPingCache) > 128 {
		embedPingCache = map[string]bool{}
	}
	embedPingCache[key] = ok
}
