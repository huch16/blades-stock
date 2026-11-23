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

// embeddingCacheRetention 是 SQLite 持久化缓存行的保留期：LRU 本身
// 只有 24 小时 TTL，但 DB 行此前永不过期、无任何清理入口，表会
// 随使用无界增长。行是可再生的派生数据（未命中即重新 Embed），
// 删旧行只会造成一次缓存未命中，不丢任何原始数据。
const embeddingCacheRetention = 30 * 24 * time.Hour

var (
	embedCacheCleanupMu sync.Mutex
	embedCacheCleanupAt time.Time
)

// CleanupOldEmbeddingCache 删除早于 retention 的持久化缓存行，返回删除条数。
// 供维护入口调用；SaveCachedEmbedding 的异步写盘里也会顺带触发，
// 但每小时最多跑一次，避免每次 Embed 都扫表。
func CleanupOldEmbeddingCache(retention time.Duration) int {
	if db.Dao == nil {
		return 0
	}
	if retention <= 0 {
		retention = embeddingCacheRetention
	}
	ensureEmbeddingCacheTable()
	res := db.Dao.Where("created_at < ?", time.Now().Add(-retention)).Delete(&EmbeddingCache{})
	if res.Error != nil {
		logger.SugaredLogger.Warnf("清理过期 embedding 缓存失败: %v", res.Error)
		return 0
	}
	return int(res.RowsAffected)
}

// maybeCleanupOldEmbeddingCache 每小时最多触发一次过期清理。
func maybeCleanupOldEmbeddingCache() {
	embedCacheCleanupMu.Lock()
	defer embedCacheCleanupMu.Unlock()
	if time.Since(embedCacheCleanupAt) < time.Hour {
		return
	}
	embedCacheCleanupAt = time.Now()
	if n := CleanupOldEmbeddingCache(embeddingCacheRetention); n > 0 {
		logger.SugaredLogger.Infof("清理过期 embedding 缓存 %d 条", n)
	}
}

func getEmbeddingLRU() *lru.LRU[string, []float32] {
	lruInitOnce.Do(func() {
		memEmbeddingLRU = lru.NewLRU[string, []float32](1500, nil, 24*time.Hour)
	})
	return memEmbeddingLRU
}

func ensureEmbeddingCacheTable() {
	// 先判空再进 Once：Dao 尚未初始化时第一次调用就会把 Once 耗掉，
	// 之后 Dao 就绪也不会再迁移，embedding_caches 表永远建不出来。
	// （生产上 Dao 在启动期已就绪，此处只是加固时序。）
	if db.Dao == nil {
		return
	}
	dbMigrateOnce.Do(func() {
		if err := db.Dao.AutoMigrate(&EmbeddingCache{}); err != nil {
			logger.SugaredLogger.Warnf("AutoMigrate embedding_caches 失败: %v", err)
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

		// 冲突时同时刷新 created_at：保留期按“最后一次写入”算，
		// 否则一个仍在被反复重新 Embed 的热向量会因首次写入时间早
		// 而被过期清理误删（虽可重算，但没必要）。
		_ = db.Dao.Clauses(gorm_clause.OnConflict{
			Columns:   []gorm_clause.Column{{Name: "hash"}},
			DoUpdates: gorm_clause.AssignmentColumns([]string{"embedding", "created_at"}),
		}).Create(&record).Error

		// 顺带过期清理（每小时最多一次，见 maybeCleanupOldEmbeddingCache）。
		maybeCleanupOldEmbeddingCache()
	}()
}
