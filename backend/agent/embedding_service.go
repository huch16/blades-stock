package agent

import (
	"blades-stock/backend/logger"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

type EmbeddingService struct {
	embeddingsURL string // full /embeddings endpoint, provider-aware
	apiKey        string
	modelName     string
	client        *http.Client
}

func NewEmbeddingService(baseURL, apiKey, modelName string) *EmbeddingService {
	baseURL = strings.TrimSpace(baseURL)
	baseURL = strings.TrimRight(baseURL, "/")
	baseURL = strings.TrimSuffix(baseURL, "/chat/completions")

	embeddingsURL := ""
	if baseURL != "" {
		switch {
		case strings.Contains(strings.ToLower(baseURL), "volces.com"):
			// VolcArk OpenAI-compatible embeddings live at /api/v3/embeddings (no /v1 layer)
			embeddingsURL = baseURL + "/embeddings"
		case strings.HasSuffix(baseURL, "/v1"):
			embeddingsURL = baseURL + "/embeddings"
		default:
			embeddingsURL = baseURL + "/v1/embeddings"
		}
	}

	return &EmbeddingService{
		embeddingsURL: embeddingsURL,
		apiKey:        apiKey,
		modelName:     modelName,
		client:        &http.Client{Timeout: 40 * time.Second},
	}
}

type embeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type EmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

func (s *EmbeddingService) Embed(text string) ([]float32, error) {
	return s.EmbedWithContext(context.Background(), text)
}

func (s *EmbeddingService) EmbedWithContext(ctx context.Context, text string) ([]float32, error) {
	if s == nil || s.embeddingsURL == "" || s.modelName == "" {
		return nil, fmt.Errorf("embedding service not configured")
	}

	// 1. 本地持久化缓存命中检查 (LRU + SQLite)
	if cachedVec, ok := GetCachedEmbedding(s.modelName, text); ok && len(cachedVec) > 0 {
		logger.SugaredLogger.Debugf("[EmbeddingCache Hit] model=%s, textLen=%d", s.modelName, len(text))
		return cachedVec, nil
	}

	body := embeddingRequest{
		Model: s.modelName,
		Input: text,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.embeddingsURL, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.apiKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read embedding response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("embedding API status=%d: %s", resp.StatusCode, string(respBody))
	}

	var result EmbeddingResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal embedding response: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("embedding response has no data")
	}

	logger.SugaredLogger.Debugf("embedding %d tokens for %d chars", result.Usage.TotalTokens, len(text))
	vec := result.Data[0].Embedding
	// 异步持久化写入缓存
	SaveCachedEmbedding(s.modelName, text, vec)
	return vec, nil
}

func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(normA) * math.Sqrt(normB)))
}

// IsConfigured returns true if the embedding service has a valid endpoint and modelName.
func (s *EmbeddingService) IsConfigured() bool {
	return s != nil && s.embeddingsURL != "" && s.modelName != ""
}

// Ping health-checks the endpoint with a minimal embedding call and a strict 3-second timeout.
func (s *EmbeddingService) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := s.EmbedWithContext(ctx, "ping")
	return err
}

// Describe returns a compact loggable identity of the service.
func (s *EmbeddingService) Describe() string {
	if s == nil {
		return "nil"
	}
	return s.embeddingsURL + " model=" + s.modelName
}

// EmbedText is an alias for Embed to satisfy the Retriever interface.
func (s *EmbeddingService) EmbedText(ctx context.Context, text string) ([]float32, error) {
	return s.Embed(text)
}

// EmbedBatch embeds multiple texts in a single request (if supported by provider).
// Falls back to sequential embedding if batch not supported.
func (s *EmbeddingService) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if s == nil || s.embeddingsURL == "" || s.modelName == "" {
		return nil, fmt.Errorf("embedding service not configured")
	}

	results := make([][]float32, len(texts))
	for i, text := range texts {
		vec, err := s.Embed(text)
		if err != nil {
			return nil, fmt.Errorf("embed text %d: %w", i, err)
		}
		results[i] = vec
	}
	return results, nil
}
