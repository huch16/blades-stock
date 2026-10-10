package agent

import (
	"crypto/md5"
	"encoding/hex"
	"sync"
	"time"

	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	lru "github.com/hashicorp/golang-lru/v2/expirable"
	gorm_clause "gorm.io/gorm/clause"
)

// EmbeddingCache 存储在 SQLite 中的通用本地向量缓存模型
type EmbeddingCache struct {
	Hash      string    `gorm:"primaryKey;size:64"`
	ModelName string    `gorm:"index;size:128"`
	Text      string    `gorm:"type:text"`
	Embedding []float32 `gorm:"serializer:json"`
	CreatedAt time.Time
}

func (EmbeddingCache) TableName() string {
	return "embedding_caches"
}

// Memory LRU Cache: 最多缓存 1500 条常用向量，保留 24 小时
var (
	memEmbeddingLRU *lru.LRU[string, []float32]
	lruInitOnce     sync.Once
	dbMigrateOnce   sync.Once
)

func getEmbeddingLRU() *lru.LRU[string, []float32] {
	lruInitOnce.Do(func() {
		memEmbeddingLRU = lru.NewLRU[string, []float32](1500, nil, 24*time.Hour)
	})
	return memEmbeddingLRU
}

func ensureEmbeddingCacheTable() {
	dbMigrateOnce.Do(func() {
		if db.Dao != nil {
			if err := db.Dao.AutoMigrate(&EmbeddingCache{}); err != nil {
				logger.SugaredLogger.Warnf("AutoMigrate embedding_caches 失败: %v", err)
			}
		}
	})
}

// computeEmbeddingHash 计算 model + text 的唯一内容哈希
func computeEmbeddingHash(modelName, text string) string {
	h := md5.New()
	h.Write([]byte(modelName))
	h.Write([]byte(":"))
	h.Write([]byte(text))
	return hex.EncodeToString(h.Sum(nil))
}

// GetCachedEmbedding 先查内存 LRU，未命中查 SQLite
func GetCachedEmbedding(modelName, text string) ([]float32, bool) {
	if modelName == "" || text == "" {
		return nil, false
	}
	hash := computeEmbeddingHash(modelName, text)

	// 1. 内存 LRU 极速检查 (~0ms)
	lruCache := getEmbeddingLRU()
	if vec, ok := lruCache.Get(hash); ok && len(vec) > 0 {
		return vec, true
	}

	// 2. 本地 SQLite 检查 (<1ms)
	ensureEmbeddingCacheTable()
	if db.Dao == nil {
		return nil, false
	}

	var row EmbeddingCache
	if err := db.Dao.Where("hash = ?", hash).First(&row).Error; err == nil && len(row.Embedding) > 0 {
		lruCache.Add(hash, row.Embedding)
		return row.Embedding, true
	}

	return nil, false
}

// SaveCachedEmbedding 保存到内存 LRU 并持久化到 SQLite
func SaveCachedEmbedding(modelName, text string, vec []float32) {
	if modelName == "" || text == "" || len(vec) == 0 {
		return
	}
	hash := computeEmbeddingHash(modelName, text)

	// 1. 写入内存 LRU
	getEmbeddingLRU().Add(hash, vec)

	// 2. 异步安全持久化写入 SQLite，绝不阻塞流式调用
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Errorf("panic in SaveCachedEmbedding: %v", r)
			}
		}()

		ensureEmbeddingCacheTable()
		if db.Dao == nil {
			return
		}

		record := EmbeddingCache{
			Hash:      hash,
			ModelName: modelName,
			Text:      text,
			Embedding: vec,
			CreatedAt: time.Now(),
		}

		_ = db.Dao.Clauses(gorm_clause.OnConflict{
			Columns:   []gorm_clause.Column{{Name: "hash"}},
			DoUpdates: gorm_clause.AssignmentColumns([]string{"embedding"}),
		}).Create(&record).Error
	}()
}
