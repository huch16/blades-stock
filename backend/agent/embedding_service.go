package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"blades-stock/backend/logger"
	"io"
	"net/http"
	"strings"
)

type EmbeddingService struct {
	baseURL   string
	apiKey    string
	modelName string
	client    *http.Client
}

func NewEmbeddingService(baseURL, apiKey, modelName string) *EmbeddingService {
	baseURL = strings.TrimRight(baseURL, "/")
	baseURL = strings.TrimSuffix(baseURL, "/chat/completions")
	baseURL = strings.TrimSuffix(baseURL, "/v1")
	return &EmbeddingService{
		baseURL:   baseURL + "/v1",
		apiKey:    apiKey,
		modelName: modelName,
		client:    &http.Client{},
	}
}

type embeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

func (s *EmbeddingService) Embed(text string) ([]float32, error) {
	if s == nil || s.baseURL == "" || s.modelName == "" {
		return nil, fmt.Errorf("embedding service not configured")
	}

	body := embeddingRequest{
		Model: s.modelName,
		Input: text,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	req, err := http.NewRequest("POST", s.baseURL+"/embeddings", bytes.NewReader(data))
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

	var result embeddingResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal embedding response: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("embedding response has no data")
	}

	logger.SugaredLogger.Debugf("embedding %d tokens for %d chars", result.Usage.TotalTokens, len(text))
	return result.Data[0].Embedding, nil
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
	return float32(dot / (sqrt64(normA) * sqrt64(normB)))
}

func sqrt64(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x / 2.0
	for i := 0; i < 10; i++ {
		z -= (z*z - x) / (2 * z)
	}
	return z
}
