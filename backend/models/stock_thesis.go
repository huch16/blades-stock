package models

import (
	"time"
)

// 买方 5 词定性判定枚举
const (
	VerdictNoChange         = "NO_CHANGE"           // 维持现状 / 无变化
	VerdictThesisStrengthen = "THESIS_STRENGTHENED" // 逻辑强化
	VerdictThesisWeaken     = "THESIS_WEAKENED"     // 逻辑削弱
	VerdictThesisBroken     = "THESIS_BROKEN"       // 逻辑证伪
	VerdictNewThesis        = "NEW_THESIS"          // 新论点 / 首次建档
)

// StockThesisLedger 标的买方认知档案账本 (单标的持久化最新认知)
type StockThesisLedger struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	StockCode    string    `gorm:"index;size:32;not null" json:"stockCode"` // 标准代码 (小写，如 sh600584 或 600584)
	StockName    string    `gorm:"size:64" json:"stockName"`
	Verdict      string    `gorm:"size:32;index" json:"verdict"`     // 枚举代码 (THESIS_STRENGTHENED 等)
	VerdictText  string    `gorm:"size:32" json:"verdictText"`       // 界面展示文本 ("逻辑强化" 等)
	CoreThesis   string    `gorm:"type:text" json:"coreThesis"`     // 核心投资假说/逻辑 (1-3句话)
	KeyCatalysts string    `gorm:"type:text" json:"keyCatalysts"`   // 重点跟踪催化剂与验证点
	RiskFactors  string    `gorm:"type:text" json:"riskFactors"`    // 证伪条件与核心风险
	ModelName    string    `gorm:"size:64" json:"modelName"`        // 给出判定的模型名称
	LastReportID uint      `gorm:"index" json:"lastReportId"`        // 关联的 AI 研报记录 ID
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (receiver StockThesisLedger) TableName() string {
	return "stock_thesis_ledger"
}

// StockVerdictHistory 认知演化历史时间线 (每次 AI 分析产生新结论时沉淀一条历史快照)
type StockVerdictHistory struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	StockCode    string    `gorm:"index;size:32;not null" json:"stockCode"`
	StockName    string    `gorm:"size:64" json:"stockName"`
	Verdict      string    `gorm:"size:32" json:"verdict"`
	VerdictText  string    `gorm:"size:32" json:"verdictText"`
	CoreThesis   string    `gorm:"type:text" json:"coreThesis"`
	KeyCatalysts string    `gorm:"type:text" json:"keyCatalysts"`
	RiskFactors  string    `gorm:"type:text" json:"riskFactors"`
	ModelName    string    `gorm:"size:64" json:"modelName"`
	ReportID     uint      `gorm:"index" json:"reportId"`
	TriggerEvent string    `gorm:"size:64" json:"triggerEvent"` // 触发源: 如 "AI单股诊断", "财报复盘"
	CreatedAt    time.Time `json:"createdAt"`
}

func (receiver StockVerdictHistory) TableName() string {
	return "stock_verdict_history"
}
