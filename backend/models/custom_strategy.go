package models

import "time"

type CustomStrategy struct {
	ID          uint      `json:"id" gorm:"primarykey"`
	CreatedAt   time.Time `json:"createdAt" wails:"-"`
	UpdatedAt   time.Time `json:"updatedAt" wails:"-"`
	Name        string    `json:"name" gorm:"size:255;not null"`
	Query       string    `json:"query" gorm:"type:text;not null"`
	Description string    `json:"description" gorm:"size:500"`
	SortOrder   int       `json:"sortOrder" gorm:"default:0"`
}

func (CustomStrategy) TableName() string {
	return "custom_strategies"
}

type CustomStrategyQuery struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Name     string `json:"name"`
}

type CustomStrategyPageData struct {
	List       []CustomStrategy `json:"list"`
	Total      int64            `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"pageSize"`
	TotalPages int              `json:"totalPages"`
}
