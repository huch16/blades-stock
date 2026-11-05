package models

import (
	"time"
)

// SkillChangeRequest 表示技能变更的审批请求
type SkillChangeRequest struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	CreatedAt  time.Time `gorm:"index" json:"createdAt"`
	SkillID    uint      `gorm:"index" json:"skillId"`                   // 关联的技能ID，0 表示新增技能
	ChangeType string    `gorm:"size:20" json:"changeType"`              // create, update, delete
	NewSkill   string    `gorm:"type:text" json:"newSkill"`              // 新技能数据（JSON）
	OldSkill   string    `gorm:"type:text" json:"oldSkill,omitempty"`    // 原技能数据（JSON，仅更新和删除时有效）
	Status     string    `gorm:"size:20;index:idx_status" json:"status"` // pending, approved, rejected
	ReviewerID string    `gorm:"size:64" json:"reviewerId,omitempty"`    // 审批人ID
	ReviewedAt time.Time `json:"reviewedAt,omitempty"`                   // 审批时间
}

func (SkillChangeRequest) TableName() string {
	return "skill_change_requests"
}

// SkillChangeQuery 分页查询参数
type SkillChangeQuery struct {
	Page       int    `form:"page" json:"page"`                       // 页码
	PageSize   int    `form:"pageSize" json:"pageSize"`               // 每页大小
	SkillID    uint   `form:"skillId" json:"skillId,omitempty"`       // 关联技能ID
	ChangeType string `form:"changeType" json:"changeType,omitempty"` // 变更类型
	Status     string `form:"status" json:"status,omitempty"`         // 审批状态
}

// SkillChangePageResp 分页查询响应
type SkillChangePageResp struct {
	Code    int                 `json:"code"`
	Message string              `json:"message"`
	Data    SkillChangePageData `json:"data"`
}

type SkillChangePageData struct {
	List     []SkillChangeRequest `json:"list"`
	Total    int64                `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"pageSize"`
}
