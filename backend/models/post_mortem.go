package models

import (
	"time"
)

// PostMortemAuditRecord 记录 AI 推荐与买方立论的事后复盘与胜率回溯审计
type PostMortemAuditRecord struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	SourceRef          string    `gorm:"size:64;index" json:"sourceRef"`       // 关联源 (如 ai_rec_123, thesis_sh600584)
	StockCode          string    `gorm:"size:32;index;not null" json:"stockCode"`
	StockName          string    `gorm:"size:64" json:"stockName"`
	InitialRating      string    `gorm:"size:32" json:"initialRating"`         // 初始评级 (买入, 强烈推荐, 逻辑强化)
	InitialPrice       float64   `json:"initialPrice"`                         // 推荐/立论时基准价
	InitialStopLoss    float64   `json:"initialStopLoss"`                      // 设定的防守止损价
	InitialTargetPrice float64   `json:"initialTargetPrice"`                   // 设定的目标止盈价
	InitialDate        string    `gorm:"size:32;index" json:"initialDate"`     // 初始推荐日期 (YYYY-MM-DD)
	HorizonDays        int       `json:"horizonDays"`                          // 跟踪周期 (1, 5, 20)
	EvaluatedAt        time.Time `json:"evaluatedAt"`                          // 复盘执行时间
	ActualReturn       float64   `json:"actualReturn"`                         // 周期内实际收益率 (%)
	MaxDrawdown        float64   `json:"maxDrawdown"`                          // 周期内最大回撤 (%)
	LowestPrice        float64   `json:"lowestPrice"`                          // 周期内最低探底价格
	PeakPrice          float64   `json:"peakPrice"`                            // 周期内最高冲顶价格
	StopLossBreached   bool      `gorm:"index" json:"stopLossBreached"`        // 是否击穿止损线
	Status             string    `gorm:"size:32;index" json:"status"`          // AUDITED_SUCCESS, AUDITED_FAILURE, AUDITED_NEUTRAL
	ReflectionNotes    string    `gorm:"type:text" json:"reflectionNotes"`     // 智能体死后剖析 (Post-Mortem) 深度反思
	LessonsLearned     string    `gorm:"type:text" json:"lessonsLearned"`      // 提炼的核心避险教训
	CrystalID          uint      `gorm:"index" json:"crystalId"`               // 自动沉淀的经验结晶 ID
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

func (PostMortemAuditRecord) TableName() string {
	return "post_mortem_audit_records"
}

// PostMortemAuditSummary 统计汇总
type PostMortemAuditSummary struct {
	TotalAudited     int     `json:"totalAudited"`
	SuccessCount     int     `json:"successCount"`
	FailureCount     int     `json:"failureCount"`
	StopLossHits     int     `json:"stopLossHits"`
	WinRateT1        float64 `json:"winRateT1"`
	WinRateT5        float64 `json:"winRateT5"`
	WinRateT20       float64 `json:"winRateT20"`
	AvgReturn        float64 `json:"avgReturn"`
	CrystalsGenerated int    `json:"crystalsGenerated"`
}
