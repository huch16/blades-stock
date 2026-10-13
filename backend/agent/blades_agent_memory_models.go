package agent

import (
	"blades-stock/backend/db"
	"encoding/binary"
	"math"
	"time"
)

type AgentMemory struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	SessionID string    `json:"sessionId" gorm:"index;size:64"`
	Role      string    `json:"role" gorm:"size:16"`
	Content   string    `json:"content" gorm:"type:text"`
	Category  string    `json:"category" gorm:"index;size:32;default:''"`
	Summary   string    `json:"summary" gorm:"type:text"`
	// Embedding vector stored as little-endian float32 BLOB
	Embedding []byte    `json:"-" gorm:"column:embedding"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (AgentMemory) TableName() string { return "agent_memories" }

// Float32sToBytesLE converts []float32 to little-endian []byte
func Float32sToBytesLE(v []float32) []byte {
	if len(v) == 0 {
		return nil
	}
	b := make([]byte, 4*len(v))
	for i, f := range v {
		binary.LittleEndian.PutUint32(b[i*4:(i+1)*4], math.Float32bits(f))
	}
	return b
}

// BytesToFloat32sLE converts little-endian []byte to []float32
func BytesToFloat32sLE(b []byte) []float32 {
	if len(b)%4 != 0 || len(b) == 0 {
		return nil
	}
	n := len(b) / 4
	out := make([]float32, n)
	for i := 0; i < n; i++ {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:(i+1)*4]))
	}
	return out
}

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
