package models

import (
	"time"
)

// SkillVersion 表示技能的版本，用于版本控制和回滚。
type SkillVersion struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	SkillID      uint      `gorm:"index;not null" json:"skillId"` // 关联的技能ID
	VersionNumber int      `gorm:"not null" json:"versionNumber"` // 版本号，从1开始递增
	Content      string    `gorm:"type:text;not null" json:"content"` // 技能的完整JSON表示（包括所有字段）
	CreatedAt    time.Time `gorm:"not null" json:"createdAt"`
	IsActive     bool      `gorm:"default:false" json:"isActive"` // 是否为当前激活版本
}

func (SkillVersion) TableName() string {
	return "skill_versions"
}

// SkillVersionQuery 分页查询参数
type SkillVersionQuery struct {
	Page     int    `form:"page" json:"page"`           // 页码
	PageSize int    `form:"pageSize" json:"pageSize"`   // 每页大小
	SkillID  uint   `form:"skillId" json:"skillId,omitempty"` // 过滤特定技能的版本
	Active   *bool  `form:"active" json:"active,omitempty"`   // 过滤激活状态
}

// SkillVersionPageResp 分页查询响应
type SkillVersionPageResp struct {
	Code    int                      `json:"code"`
	Message string                   `json:"message"`
	Data    SkillVersionPageData     `json:"data"`
}

type SkillVersionPageData struct {
	List      []SkillVersion `json:"list"`
	Total     int64          `json:"total"`
	Page      int            `json:"page"`
	PageSize  int            `json:"pageSize"`
}