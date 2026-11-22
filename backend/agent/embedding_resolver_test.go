package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"blades-stock/backend/data"
)

func testSettingConfig() *data.SettingConfig {
	return &data.SettingConfig{
		Settings: &data.Settings{},
	}
}

func TestNewEmbeddingServiceURLCleaning(t *testing.T) {
	cases := []struct {
		name      string
		base      string
		wantURL   string
		wantModel string
	}{
		{"openai-v1", "https://api.openai.com/v1", "https://api.openai.com/v1/embeddings", "text-embedding-3-small"},
		{"openai-no-trailing-slash", "https://api.openai.com/v1/", "https://api.openai.com/v1/embeddings", "text-embedding-3-small"},
		{"openai-with-chat-suffix", "https://api.openai.com/v1/chat/completions", "https://api.openai.com/v1/embeddings", "text-embedding-3-small"},
		{"volcark-v3", "https://ark.cn-beijing.volces.com/api/v3", "https://ark.cn-beijing.volces.com/api/v3/embeddings", "doubao-embedding-large"},
		{"volcark-v3-slash", "https://ark.cn-beijing.volces.com/api/v3/", "https://ark.cn-beijing.volces.com/api/v3/embeddings", "doubao-embedding-large"},
		{"ollama", "http://localhost:11434", "http://localhost:11434/v1/embeddings", "bge-m3"},
		{"ollama-with-v1", "http://localhost:11434/v1", "http://localhost:11434/v1/embeddings", "bge-m3"},
		{"nvidia", "https://integrate.api.nvidia.com/v1", "https://integrate.api.nvidia.com/v1/embeddings", "nvidia/nemotron-3-nano-omni-30b-a3b-reasoning"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewEmbeddingService(tc.base, "", tc.wantModel)
			if svc == nil {
				t.Fatal("svc nil")
			}
			if svc.embeddingsURL != tc.wantURL {
				t.Errorf("embeddingsURL = %q, want %q", svc.embeddingsURL, tc.wantURL)
			}
			if !svc.IsConfigured() {
				t.Errorf("expected IsConfigured() true for %q", tc.base)
			}
		})
	}
}

func TestNewEmbeddingServiceEmptyURL(t *testing.T) {
	svc := NewEmbeddingService("", "", "bge-m3")
	if svc == nil {
		t.Fatal("svc nil")
	}
	if svc.IsConfigured() {
		t.Error("empty baseURL should not be configured")
	}
}

func TestProviderEmbeddingDefaults(t *testing.T) {
	cases := []struct {
		name      string
		cfg       *data.AIConfig
		wantOK    bool
		wantModel string
	}{
		{"nil", nil, false, ""},
		{"nvidia", &data.AIConfig{BaseUrl: "https://integrate.api.nvidia.com/v1", ModelName: "deepseek-ai/deepseek-v4-pro"}, false, ""},
		{"minimax", &data.AIConfig{BaseUrl: "https://api.minimax.chat/v1", ModelName: "MiniMax-M3"}, false, ""},
		{"deepseek-official", &data.AIConfig{BaseUrl: "https://api.deepseek.com/v1", ModelName: "deepseek-chat"}, false, ""},
		{"gemini", &data.AIConfig{BaseUrl: "https://generativelanguage.googleapis.com", ModelName: "gemini-2.5-pro"}, false, ""},
		{"ollama", &data.AIConfig{BaseUrl: "http://localhost:11434", ModelName: "qwen2.5"}, true, ""},
		{"openai", &data.AIConfig{BaseUrl: "https://api.openai.com/v1", ModelName: "gpt-4o"}, true, "text-embedding-3-small"},
		{"dashscope", &data.AIConfig{BaseUrl: "https://dashscope.aliyuncs.com/compatible-mode/v1", ModelName: "qwen-plus"}, true, "text-embedding-v3"},
		{"volcark", &data.AIConfig{BaseUrl: "https://ark.cn-beijing.volces.com/api/v3", ModelName: "doubao-1.5-pro"}, true, "doubao-embedding-large"},
		{"siliconflow", &data.AIConfig{BaseUrl: "https://api.siliconflow.cn/v1", ModelName: "Qwen/Qwen3-32B"}, true, "BAAI/bge-m3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base, model, ok := providerEmbeddingDefaults(tc.cfg)
			if ok != tc.wantOK {
				t.Errorf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if tc.wantModel != "" && model != tc.wantModel {
				t.Errorf("model = %q, want %q", model, tc.wantModel)
			}
			if base == "" {
				t.Error("baseURL empty for ok=true")
			}
		})
	}
}

func TestResolveEmbeddingCandidateGlobalExplicitURL(t *testing.T) {
	s := testSettingConfig()
	s.EmbeddingEnabled = true
	s.EmbeddingModelName = "bge-m3"
	s.EmbeddingBaseUrl = "http://127.0.0.1:11434"
	s.EmbeddingApiKey = "sk-test"
	s.AiConfigs = []*data.AIConfig{
		{ID: 1, BaseUrl: "https://integrate.api.nvidia.com/v1", ModelName: "openai/gpt-oss-120b"},
	}
	c := resolveEmbeddingCandidate(s, s.AiConfigs[0])
	if c == nil {
		t.Fatal("candidate nil")
	}
	if c.baseURL != "http://127.0.0.1:11434" || c.model != "bge-m3" || c.apiKey != "sk-test" {
		t.Errorf("unexpected candidate: %+v", c)
	}
}

func TestResolveEmbeddingCandidateGlobalEmptyURL(t *testing.T) {
	selected := &data.AIConfig{ID: 1, BaseUrl: "https://integrate.api.nvidia.com/v1", ModelName: "openai/gpt-oss-120b"}

	t.Run("chat-only-selected-nil", func(t *testing.T) {
		s := testSettingConfig()
		s.EmbeddingEnabled = true
		s.EmbeddingModelName = "bge-m3"
		s.AiConfigs = []*data.AIConfig{selected}
		if c := resolveEmbeddingCandidate(s, selected); c != nil {
			t.Errorf("expected nil for NVIDIA chat-only selected, got %+v", c)
		}
	})

	t.Run("no-configs-nil", func(t *testing.T) {
		s := testSettingConfig()
		s.EmbeddingEnabled = true
		s.EmbeddingModelName = "bge-m3"
		if c := resolveEmbeddingCandidate(s, nil); c != nil {
			t.Errorf("expected nil with no configs, got %+v", c)
		}
	})

	t.Run("ollama-selected-derived-keeps-global-model", func(t *testing.T) {
		s := testSettingConfig()
		s.EmbeddingEnabled = true
		s.EmbeddingModelName = "bge-m3"
		ollama := &data.AIConfig{ID: 2, BaseUrl: "http://localhost:11434", ModelName: "qwen2.5"}
		s.AiConfigs = []*data.AIConfig{ollama}
		c := resolveEmbeddingCandidate(s, ollama)
		if c == nil {
			t.Fatal("candidate nil")
		}
		if c.baseURL != "http://localhost:11434" {
			t.Errorf("baseURL = %q", c.baseURL)
		}
		if c.model != "bge-m3" {
			t.Errorf("model = %q, want global bge-m3", c.model)
		}
		if c.source != "provider 感知派生" {
			t.Errorf("source = %q", c.source)
		}
	})

	t.Run("explicit-embedding-config-wins-over-provider-default", func(t *testing.T) {
		s := testSettingConfig()
		s.EmbeddingEnabled = true
		s.EmbeddingModelName = "bge-m3"
		nvidia := &data.AIConfig{ID: 1, BaseUrl: "https://integrate.api.nvidia.com/v1", ModelName: "openai/gpt-oss-120b"}
		emb := &data.AIConfig{ID: 3, BaseUrl: "https://api.siliconflow.cn/v1", ModelName: "Qwen/Qwen3-32B", EmbeddingEnabled: true, EmbeddingModelName: "BAAI/bge-m3"}
		s.AiConfigs = []*data.AIConfig{nvidia, emb}
		c := resolveEmbeddingCandidate(s, nvidia)
		if c == nil {
			t.Fatal("candidate nil")
		}
		if c.baseURL != "https://api.siliconflow.cn/v1" || c.model != "BAAI/bge-m3" {
			t.Errorf("unexpected candidate: %+v", c)
		}
	})
}

func TestResolveEmbeddingCandidateSelectedOwnEmbedding(t *testing.T) {
	s := testSettingConfig()
	selected := &data.AIConfig{ID: 1, BaseUrl: "https://api.siliconflow.cn/v1", ModelName: "Qwen/Qwen3-32B", EmbeddingEnabled: true, EmbeddingModelName: "BAAI/bge-m3"}
	s.AiConfigs = []*data.AIConfig{selected}
	c := resolveEmbeddingCandidate(s, selected)
	if c == nil {
		t.Fatal("candidate nil")
	}
	if c.baseURL != "https://api.siliconflow.cn/v1" || c.model != "BAAI/bge-m3" {
		t.Errorf("unexpected candidate: %+v", c)
	}
}

func TestResolveEmbeddingCandidateScanFallback(t *testing.T) {
	s := testSettingConfig()
	selected := &data.AIConfig{ID: 1, BaseUrl: "https://integrate.api.nvidia.com/v1", ModelName: "openai/gpt-oss-120b"}
	other := &data.AIConfig{ID: 4, BaseUrl: "http://localhost:11434", ModelName: "qwen2.5", EmbeddingEnabled: true, EmbeddingModelName: "bge-m3"}
	s.AiConfigs = []*data.AIConfig{selected, other}
	c := resolveEmbeddingCandidate(s, selected)
	if c == nil {
		t.Fatal("candidate nil")
	}
	if c.baseURL != "http://localhost:11434" || c.model != "bge-m3" {
		t.Errorf("unexpected candidate: %+v", c)
	}
}

func TestResolveEmbeddingCandidateDisabled(t *testing.T) {
	s := testSettingConfig()
	s.AiConfigs = []*data.AIConfig{
		{ID: 1, BaseUrl: "https://integrate.api.nvidia.com/v1", ModelName: "openai/gpt-oss-120b"},
	}
	if c := resolveEmbeddingCandidate(s, s.AiConfigs[0]); c != nil {
		t.Errorf("expected nil when embedding disabled, got %+v", c)
	}
}

// TestVerifyEmbedding_FailureVerdictExpires 验证失败的健康检查结论
// 会过期重试：旧实现把失败 verdict 永久缓存，启动期一次瞬时抖动
// 会让向量服务在整个进程生命周期内永久降级。
func TestVerifyEmbedding_FailureVerdictExpires(t *testing.T) {
	embedPingMu.Lock()
	embedPingCache = map[string]embedVerdict{}
	embedPingMu.Unlock()
	oldTTL := embedFailureRetryAfter
	embedFailureRetryAfter = 0
	t.Cleanup(func() { embedFailureRetryAfter = oldTTL })

	var hits int32
	// 第一次请求失败，之后成功。用唯一模型名避免命中其它测试
	// 留下的 "ping" 缓存向量（缓存键含模型名）。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("boom"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data":  []map[string]interface{}{{"embedding": []float32{1, 0}}},
			"usage": map[string]int{"total_tokens": 1},
		})
	}))
	t.Cleanup(srv.Close)

	svc := NewEmbeddingService(srv.URL, "", "test-model-verify-ttl")
	if got := verifyEmbedding(svc); got != nil {
		t.Fatalf("first verify should fail and return nil, got %v", got)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("expected 1 ping attempt, got %d", hits)
	}
	// TTL 设为 0 后，失败结论立即过期：下一次应重新 Ping 并成功。
	if got := verifyEmbedding(svc); got == nil {
		t.Fatalf("expired failure verdict should be retried, got nil (hits=%d)", hits)
	}
	if atomic.LoadInt32(&hits) != 2 {
		t.Fatalf("expected exactly 2 ping attempts, got %d", hits)
	}
	// 成功结论长期缓存：不再 Ping。
	if got := verifyEmbedding(svc); got == nil {
		t.Fatalf("success verdict should be cached, got nil")
	}
	if atomic.LoadInt32(&hits) != 2 {
		t.Fatalf("success verdict must not re-ping, hits=%d", hits)
	}
}

// TestNewEmbeddingService_EmbeddingsSuffixNoDoubleAppend 验证 baseURL
// 已是完整 /embeddings 端点时不再追加一次。
func TestNewEmbeddingService_EmbeddingsSuffixNoDoubleAppend(t *testing.T) {
	svc := NewEmbeddingService("https://api.openai.com/v1/embeddings", "", "text-embedding-3-small")
	if svc.embeddingsURL != "https://api.openai.com/v1/embeddings" {
		t.Errorf("embeddingsURL = %q, want no double append", svc.embeddingsURL)
	}
}

// TestEmbedText_HonoursCancelledContext 验证 EmbedText 遵守 ctx，
// 而不是像旧实现那样在 context.Background() 上发请求。
func TestEmbedText_HonoursCancelledContext(t *testing.T) {
	srv := fakeEmbedServer(t, []float32{1, 0})
	es := NewEmbeddingService(srv.URL, "", "test-model-embedtext-ctx")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := es.EmbedText(ctx, "unique-text-never-embedded-before-xyz"); err == nil {
		t.Errorf("EmbedText with cancelled context should fail")
	}
}
