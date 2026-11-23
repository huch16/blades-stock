package agent

import (
	"blades-stock/backend/db"
	"time"
)

type AgentMemory struct {
	ID        uint   `json:"id" gorm:"primarykey"`
	SessionID string `json:"sessionId" gorm:"index;size:64"`
	Role      string `json:"role" gorm:"size:16"`
	Content   string `json:"content" gorm:"type:text"`
	Category  string `json:"category" gorm:"index;size:32;default:''"`
	Summary   string `json:"summary" gorm:"type:text"`
	// Embedding vector stored as little-endian float32 BLOB
	Embedding []byte    `json:"-" gorm:"column:embedding"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (AgentMemory) TableName() string { return "agent_memories" }

// AgentSession 是 agent_sessions 表的旧模型定义：全仓无任何读写，
// 只有 AutoMigrate 建表（第五轮核实）。自本轮起不再自动建表——
// 旧库里已存在的空表不受影响（不删表、不碰数据），新库不再建该表。
// 结构保留仅作遗留表形记录；如需彻底删除表，必须由用户另行决定。
type AgentSession struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	SessionID string    `json:"sessionId" gorm:"uniqueIndex;size:64"`
	Title     string    `json:"title" gorm:"size:256"`
	Summary   string    `json:"summary" gorm:"type:text"`
	FactCount int       `json:"factCount" gorm:"default:0"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (AgentSession) TableName() string { return "agent_sessions" }

type AgentTrace struct {
	ID           uint      `json:"id" gorm:"primarykey"`
	SessionID    string    `json:"sessionId" gorm:"index;size:64"`
	Question     string    `json:"question" gorm:"type:text"`
	Answer       string    `json:"answer" gorm:"type:text"`
	ToolsUsed    string    `json:"toolsUsed" gorm:"type:text"`
	Score        int       `json:"score" gorm:"default:0"`
	Accuracy     int       `json:"accuracy" gorm:"default:0"`
	Relevance    int       `json:"relevance" gorm:"default:0"`
	ToolUse      int       `json:"toolUse" gorm:"default:0"`
	Completeness int       `json:"completeness" gorm:"default:0"`
	Feedback     string    `json:"feedback" gorm:"type:text"`
	DurationMs   int64     `json:"durationMs" gorm:"default:0"`
	CreatedAt    time.Time `json:"createdAt"`
}

func (AgentTrace) TableName() string { return "agent_traces" }

func init() {
	// 注意：AgentSession 已从迁移列表移除（第五轮，见上）：该表全仓
	// 无读写、是空表；只停掉新库建表，旧库已存在的表原样保留。
	db.AutoMigrateList = append(db.AutoMigrateList,
		&AgentMemory{},
		&AgentTrace{},
	)
}
