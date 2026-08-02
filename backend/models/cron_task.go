package models

import "time"

type CronTask struct {
	ID            uint       `json:"id" gorm:"primarykey"`
	CreatedAt     time.Time  `json:"createdAt" wails:"-"`
	UpdatedAt     time.Time  `json:"updatedAt" wails:"-"`
	Name          string     `json:"name" gorm:"size:255;not null"`
	CronExpr      string     `json:"cronExpr" gorm:"size:100;not null"`
	TaskType      string     `json:"taskType" gorm:"size:50;not null"` // stock_analysis, fund_analysis, news_fetch, custom
	Target        string     `json:"target" gorm:"size:255"`           // 股票代码或其他目标
	Params        string     `json:"params" gorm:"type:text"`          // JSON 格式的任务参数
	Enable        bool       `json:"enable" gorm:"default:true"`
	LastRunAt     *time.Time `json:"lastRunAt" wails:"-"`
	NextRunAt     *time.Time `json:"nextRunAt" wails:"-"`
	RunCount      int64      `json:"runCount" gorm:"default:0"`
	Status        string     `json:"status" gorm:"size:20;default:active"` // active, paused, error
	Description   string     `json:"description" gorm:"size:500"`
	LastRunResult string     `json:"lastRunResult" gorm:"size:500"`
}

func (CronTask) TableName() string {
	return "cron_tasks"
}

type CronTaskQuery struct {
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
	Name     string `json:"name"`
	TaskType string `json:"taskType"`
	Status   string `json:"status"`
	Enable   *bool  `json:"enable"`
}

type CronTaskPageResp struct {
	Total int        `json:"total"`
	Data  []CronTask `json:"data"`
}

type CronTaskPageData struct {
	List       []CronTask `json:"list"`
	TotalCount int        `json:"totalCount"`
	Page       int        `json:"page"`
	PageSize   int        `json:"pageSize"`
}
