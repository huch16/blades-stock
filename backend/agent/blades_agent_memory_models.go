package agent

import (
	"blades-stock/backend/db"
	"time"
)

type AgentMemory struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	SessionID string    `json:"sessionId" gorm:"index;size:64"`
	Role      string    `json:"role" gorm:"size:16"`
	Content   string    `json:"content" gorm:"type:text"`
	Category  string    `json:"category" gorm:"index;size:32;default:''"`
	Summary   string    `json:"summary" gorm:"type:text"`
	CreatedAt time.Time `json:"createdAt"`
}

func (AgentMemory) TableName() string { return "agent_memories" }

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
	ID          uint      `json:"id" gorm:"primarykey"`
	SessionID   string    `json:"sessionId" gorm:"index;size:64"`
	Question    string    `json:"question" gorm:"type:text"`
	Answer      string    `json:"answer" gorm:"type:text"`
	ToolsUsed   string    `json:"toolsUsed" gorm:"type:text"`
	Score       int       `json:"score" gorm:"default:0"`
	Accuracy    int       `json:"accuracy" gorm:"default:0"`
	Relevance   int       `json:"relevance" gorm:"default:0"`
	ToolUse     int       `json:"toolUse" gorm:"default:0"`
	Completeness int      `json:"completeness" gorm:"default:0"`
	Feedback    string    `json:"feedback" gorm:"type:text"`
	DurationMs  int64     `json:"durationMs" gorm:"default:0"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (AgentTrace) TableName() string { return "agent_traces" }

func init() {
	db.AutoMigrateList = append(db.AutoMigrateList,
		&AgentMemory{},
		&AgentSession{},
		&AgentTrace{},
	)
}
