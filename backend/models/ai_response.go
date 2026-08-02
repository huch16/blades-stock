package models

import (
	"gorm.io/gorm"
	"gorm.io/plugin/soft_delete"
)

type AIResponseResult struct {
	gorm.Model
	ChatId    string                `json:"chatId"`
	ModelName string                `json:"modelName"`
	StockCode string                `json:"stockCode"`
	StockName string                `json:"stockName"`
	Question  string                `json:"question"`
	Content   string                `json:"content"`
	IsDel     soft_delete.DeletedAt `gorm:"softDelete:flag"`
}

func (receiver AIResponseResult) TableName() string {
	return "ai_response_result"
}

// AIResponseResultQuery 分页查询参数
type AIResponseResultQuery struct {
	Page      int    `form:"page" json:"page"`           // 页码
	PageSize  int    `form:"pageSize" json:"pageSize"`   // 每页大小
	ChatId    string `form:"chatId" json:"chatId"`       // 聊天ID筛选
	ModelName string `form:"modelName" json:"modelName"` // 模型名称筛选
	StockCode string `form:"stockCode" json:"stockCode"` // 股票代码筛选
	StockName string `form:"stockName" json:"stockName"` // 股票名称筛选
	Question  string `form:"question" json:"question"`   // 问题内容模糊搜索
	StartDate string `form:"startDate" json:"startDate"` // 开始日期
	EndDate   string `form:"endDate" json:"endDate"`     // 结束日期
}

// AIResponseResultPageResp 分页查询响应
type AIResponseResultPageResp struct {
	Code    int                      `json:"code"`
	Message string                   `json:"message"`
	Data    AIResponseResultPageData `json:"data"`
}

type AIResponseResultPageData struct {
	List       []AIResponseResult `json:"list"`
	Total      int64              `json:"total"`
	Page       int                `json:"page"`
	PageSize   int                `json:"pageSize"`
	TotalPages int                `json:"totalPages"`
}
