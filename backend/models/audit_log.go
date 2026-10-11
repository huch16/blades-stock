package models

import "time"

// AuditLog records MCP/tool calls and agent-executed actions for audit and debugging.
// It is intentionally verbose: request/response bodies are stored for traceability.
// Keep the struct stable to ease migration and querying.
type AuditLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	// Optional identifiers
	UserID    string `gorm:"size:128" json:"user_id"`
	AgentID   string `gorm:"size:128" json:"agent_id"`
	SkillID   uint   `json:"skill_id"`

	// Tool / server info
	ToolName   string `gorm:"size:256" json:"tool_name"`
	ServerName string `gorm:"size:256" json:"server_name"`

	// Payloads (may be large)
	RequestBody string `gorm:"type:text" json:"request_body"`
	Response    string `gorm:"type:text" json:"response"`

	// Result metadata
	Success    bool  `json:"success"`
	ErrorMsg   string `gorm:"type:text" json:"error_msg"`
	DurationMs int64  `json:"duration_ms"`
}
