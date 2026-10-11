package models

import (
	"time"
)

// AuditLog 记录所有工具调用、MCP交互及Agent动作的结构化审计日志
type AuditLog struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time `gorm:"index" json:"createdAt"`
	UserID      string    `gorm:"size:64;index" json:"userId,omitempty"`
	AgentID     string    `gorm:"size:64;index" json:"agentId,omitempty"`
	SkillID     uint      `gorm:"index" json:"skillId,omitempty"`
	ToolName    string    `gorm:"size:128;index" json:"toolName"`
	ServerName  string    `gorm:"size:128;index" json:"serverName,omitempty"`
	RequestBody string    `gorm:"type:text" json:"requestBody"`
	Response    string    `gorm:"type:text" json:"response"`
	Success     bool      `gorm:"index" json:"success"`
	ErrorMsg    string    `gorm:"type:text" json:"errorMsg,omitempty"`
	DurationMs  int64     `json:"durationMs"`
}

// AuditLogQuery 分页查询参数
type AuditLogQuery struct {
	Page      int    `form:"page" json:"page"`           // 页码
	PageSize  int    `form:"pageSize" json:"pageSize"`   // 每页大小
	UserID    string `form:"userId" json:"userId,omitempty"`   // 用户ID筛选
	AgentID   string `form:"agentId" json:"agentId,omitempty"` // AgentID筛选
	SkillID   uint   `form:"skillId" json:"skillId,omitempty"` // SkillID筛选
	ToolName  string `form:"toolName" json:"toolName,omitempty"` // 工具名称筛选
	ServerName string `form:"serverName" json:"serverName,omitempty"` // MCP服务器名称筛选
	Success   *bool  `form:"success" json:"success,omitempty"`   // 是否成功
	StartDate string `form:"startDate" json:"startDate,omitempty"` // 开始日期 YYYY-MM-DD
	EndDate   string `form:"endDate" json:"endDate,omitempty"`     // 结束日期 YYYY-MM-DD
	Keyword   string `form:"keyword" json:"keyword,omitempty"`     // 关键字搜索（在请求或响应中）
}

// AuditLogPageResp 分页查询响应
type AuditLogPageResp struct {
	Code    int                      `json:"code"`
	Message string                   `json:"message"`
	Data    AuditLogPageData         `json:"data"`
}

type AuditLogPageData struct {
	List      []AuditLog `json:"list"`
	Total     int64      `json:"total"`
	Page      int        `json:"page"`
	PageSize  int        `json:"pageSize"`
}

func (AuditLog) TableName() string {
	return "audit_logs"
}
