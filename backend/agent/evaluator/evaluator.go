package evaluator

import (
	"time"
)

// EvaluatorReport represents the evaluation report
type EvaluatorReport struct {
	ID               uint      `json:"id" gorm:"primarykey"`
	CreatedAt        time.Time `json:"createdAt"`
	RunAt            time.Time `json:"runAt" gorm:"index"`
	PeriodStart      time.Time `json:"periodStart" gorm:"index"`
	PeriodEnd        time.Time `json:"periodEnd" gorm:"index"`
	TotalCases       int       `json:"totalCases"`
	MatchedCases     int       `json:"matchedCases"`
	ToolCorrect      int       `json:"toolCorrect"`
	ToolAccuracy     float64   `json:"toolAccuracy"`
	AvgLatencyMs     float64   `json:"avgLatencyMs"`
	RetrieverHitRate float64   `json:"retrieverHitRate"`
	DetailsJSON      string    `json:"detailsJson" gorm:"type:text"`
}

func (EvaluatorReport) TableName() string {
	return "evaluator_reports"
}

// NOTE: 本包曾经还有一套 golden-case 离线评估架子（Evaluator、
// NewEvaluator、loadGoldenCases、GoldenCase、CaseResult 及配套测试），
// 2026-11-23 第四轮核查时确认已死：全仓零生产调用者——没有 runner、
// 没有 golden 文件、没有任何地方构造它，唯一的测试也只测它自己。
// 已整套删除，只保留真正被 AutoMigrate 使用的 EvaluatorReport 模型。
// 若将来要恢复离线评估，应从“有 golden 文件 + 有 runner 入口”
// 一起恢复，而不是只恢复加载函数。
// 另：evaluator_reports 表目前只有建表迁移、从无写入（没有任何地方
// 创建 EvaluatorReport 行），恢复评估时需先补写入路径。
