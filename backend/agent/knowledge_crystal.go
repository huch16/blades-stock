package agent

import (
	"blades-stock/backend/db"
	"blades-stock/backend/models"
	"time"
)

// KnowledgeCrystal 别名指向 models.KnowledgeCrystal
type KnowledgeCrystal = models.KnowledgeCrystal

// ComputeContentHash 别名指向 models.ComputeContentHash
var ComputeContentHash = models.ComputeContentHash

// DistillationStats 语义精炼与瘦身统计指标
type DistillationStats struct {
	TotalCrystals    int64     `json:"totalCrystals"`    // 当前已凝练经验结晶总数
	PrunedNewsCount  int       `json:"prunedNewsCount"`  // 累计去重并清理的冗余快讯条数
	PrunedMemories   int       `json:"prunedMemories"`   // 累计折叠并清理的瞬态对话工作记忆
	PrunedReports    int       `json:"prunedReports"`    // 累计精炼并归档的原始冗长研报
	AverageAccuracy  float64   `json:"averageAccuracy"`  // 结晶经验平均胜率评分
	TotalHits        int64     `json:"totalHits"`        // Agent 召回复用总次数
	LastRunAt        time.Time `json:"lastRunAt"`        // 上次精炼执行时间
	DatabaseSizeMB   float64   `json:"databaseSizeMB"`   // 当前 SQLite 数据库物理体积
	EstimatedFreedMB float64   `json:"estimatedFreedMB"` // 累计预估节省磁盘空间 (MB)
	CompressionRatio float64   `json:"compressionRatio"` // 语义数据压缩率 (如 85.4%)
}

func init() {
	db.AutoMigrateList = append(db.AutoMigrateList, &models.KnowledgeCrystal{})
}
