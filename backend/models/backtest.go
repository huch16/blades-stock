package models

import "gorm.io/gorm"

// BacktestResult 分析报告回测结果 — 对比AI预测与市场真实走势
type BacktestResult struct {
	gorm.Model
	ReportID     uint    `json:"reportId" gorm:"uniqueIndex"`    // 关联 ai_response_result.id
	StockCode    string  `json:"stockCode" gorm:"size:20;index"` // 股票代码
	StockName    string  `json:"stockName" gorm:"size:100"`      // 股票名称
	AnalysisDate string  `json:"analysisDate" gorm:"size:10"`    // 分析日期 YYYY-MM-DD
	Direction    string  `json:"direction" gorm:"size:20"`       // 预测方向: 看涨/看跌/中性/震荡
	TargetPrice  float64 `json:"targetPrice"`                    // 目标价
	VerifyDate   string  `json:"verifyDate" gorm:"size:10"`      // 验证日期
	VerifyPrice  float64 `json:"verifyPrice"`                    // 验证日收盘价
	PriceChange  float64 `json:"priceChange"`                    // 实际涨跌幅 %
	MaxDrawdown  float64 `json:"maxDrawdown"`                    // 期间最大回撤 %
	MaxUpside    float64 `json:"maxUpside"`                      // 期间最大涨幅 %
	IsAccurate   bool    `json:"isAccurate"`                     // 方向判断是否正确
	Confidence   float64 `json:"confidence"`                     // 置信度 0-1
	Summary      string  `json:"summary" gorm:"type:text"`       // 摘要
}

func (BacktestResult) TableName() string {
	return "backtest_results"
}
