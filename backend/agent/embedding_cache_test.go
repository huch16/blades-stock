package agent

import (
	"reflect"
	"testing"
	"time"

	"blades-stock/backend/db"
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

// TestCleanupOldEmbeddingCache 验证过期清理只删旧行、不动新行：
// 旧实现里 embedding_caches 表没有任何清理入口，会随使用无界增长。
func TestCleanupOldEmbeddingCache(t *testing.T) {
	if err := db.Dao.AutoMigrate(&EmbeddingCache{}); err != nil {
		t.Fatalf("migrate embedding_caches: %v", err)
	}
	oldHash := computeEmbeddingHash("test-model-cleanup", "old text")
	newHash := computeEmbeddingHash("test-model-cleanup", "new text")
	t.Cleanup(func() {
		db.Dao.Where("hash IN ?", []string{oldHash, newHash}).Delete(&EmbeddingCache{})
	})
	if err := db.Dao.Create(&EmbeddingCache{
		Hash: oldHash, ModelName: "test-model-cleanup", Text: "old text",
		Embedding: []float32{1}, CreatedAt: time.Now().AddDate(0, 0, -40),
	}).Error; err != nil {
		t.Fatalf("insert old row: %v", err)
	}
	if err := db.Dao.Create(&EmbeddingCache{
		Hash: newHash, ModelName: "test-model-cleanup", Text: "new text",
		Embedding: []float32{2}, CreatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("insert new row: %v", err)
	}

	if n := CleanupOldEmbeddingCache(30 * 24 * time.Hour); n != 1 {
		t.Errorf("expected 1 row cleaned, got %d", n)
	}
	var count int64
	db.Dao.Model(&EmbeddingCache{}).Where("hash = ?", oldHash).Count(&count)
	if count != 0 {
		t.Errorf("old row should be deleted")
	}
	db.Dao.Model(&EmbeddingCache{}).Where("hash = ?", newHash).Count(&count)
	if count != 1 {
		t.Errorf("fresh row should be kept")
	}
}
