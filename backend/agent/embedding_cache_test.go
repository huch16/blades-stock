package agent

import (
	"reflect"
	"testing"
)

func TestEmbeddingCacheLRU(t *testing.T) {
	model := "text-embedding-3-small"
	text := "测试股票研报向量分析"
	vec := []float32{0.12, -0.45, 0.88, 0.05}

	// 1. 初始应未命中
	if _, ok := GetCachedEmbedding(model, text); ok {
		t.Logf("already cached in previous run")
	}

	// 2. 存入缓存
	SaveCachedEmbedding(model, text, vec)

	// 3. 从 LRU 应立即命中且数据一致
	cached, hit := GetCachedEmbedding(model, text)
	if !hit {
		t.Fatalf("expected LRU cache hit, got miss")
	}
	if !reflect.DeepEqual(cached, vec) {
		t.Fatalf("expected cached vector %v, got %v", vec, cached)
	}

	// 4. 不同文本应产生不同 hash 且不碰撞
	text2 := "不同的文本"
	if _, hit2 := GetCachedEmbedding(model, text2); hit2 {
		t.Fatalf("expected miss for different text")
	}
}

func TestComputeEmbeddingHash(t *testing.T) {
	h1 := computeEmbeddingHash("model-a", "text 1")
	h2 := computeEmbeddingHash("model-a", "text 1")
	h3 := computeEmbeddingHash("model-b", "text 1")

	if h1 != h2 {
		t.Errorf("expected deterministic hash: %s != %s", h1, h2)
	}
	if h1 == h3 {
		t.Errorf("expected different hash for different models: %s == %s", h1, h3)
	}
}
