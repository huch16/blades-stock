package models

import "time"

// IndexManifest 记录已索引文档的清单，便于增量、去重与审计。
type IndexManifest struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	DocID       string    `gorm:"index;not null" json:"docId"`    // 例如 "skill:123" / "doc:README.md" / "mem:45"
	Source      string    `gorm:"size:64;not null" json:"source"` // skill|docs|memory
	ContentHash string    `gorm:"size:128;not null" json:"contentHash"`
	Backend     string    `gorm:"size:32;not null" json:"backend"` // memory|redis|...
	IndexedAt   time.Time `gorm:"not null" json:"indexedAt"`
	CreatedAt   time.Time `json:"createdAt"`
}
