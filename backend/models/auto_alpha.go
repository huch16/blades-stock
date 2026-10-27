package models

import (
	"time"
)

// AutoAlphaProposal 记录夜间自动参数搜索与因子进化提案
type AutoAlphaProposal struct {
	ID                   uint      `gorm:"primaryKey" json:"id"`
	StrategyName         string    `gorm:"size:64;index" json:"strategyName"`         // 策略名: ETF_MOMENTUM_ROTATION, DUAL_MA_TREND, VOLATILITY_BREAKOUT
	TargetAsset          string    `gorm:"size:128" json:"targetAsset"`              // 优化目标资产池，如 "科技ETF核心池 (512480, 515050, 159915)"
	OldParams            string    `gorm:"type:text" json:"oldParams"`               // 原始基准参数 (JSON)
	OptimalParams        string    `gorm:"type:text" json:"optimalParams"`           // 经遗传/网格优化后的推荐参数 (JSON)
	OriginalSharpe       float64   `json:"originalSharpe"`                           // 基准夏普比率
	OptimizedSharpe      float64   `json:"optimizedSharpe"`                          // 优化后夏普比率
	ExpectedAnnualReturn float64   `json:"expectedAnnualReturn"`                     // 预期年化收益率 (%)
	MaxDrawdownReduction float64   `json:"maxDrawdownReduction"`                     // 最大回撤改善绝对值 (%)
	WinRate              float64   `json:"winRate"`                                  // 回测历史胜率 (%)
	Status               string    `gorm:"size:32;index;default:'PENDING_APPROVAL'"` // PENDING_APPROVAL, ACTIVE, ARCHIVED, REJECTED
	EvaluationSummary    string    `gorm:"type:text" json:"evaluationSummary"`       // 量化评估建议与鲁棒性说明
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

func (AutoAlphaProposal) TableName() string {
	return "auto_alpha_proposals"
}
