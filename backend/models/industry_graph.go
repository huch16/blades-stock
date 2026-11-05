package models

import (
	"time"
)

// IndustryNode 产业链知识图谱节点 (实体：海外巨头、技术赛道、细分产业环节、A股核心标的)
type IndustryNode struct {
	ID          string    `gorm:"primaryKey;size:64" json:"id"`             // 唯一标识 (如 NVDA, AAPL, OPTICAL_MODULE, 300308)
	Name        string    `gorm:"size:128;not null" json:"name"`            // 名称 (如 英伟达, 800G/1.6T高速光模块, 中际旭创)
	Category    string    `gorm:"size:32;index" json:"category"`            // OVERSEAS_GIANT, TECH_CATALYST, DOMESTIC_INDUSTRY, A_STOCK
	StockCode   string    `gorm:"size:32;index" json:"stockCode,omitempty"` // 若为 A 股标的，记录标准代码 (如 300308)
	MarketCap   string    `gorm:"size:32" json:"marketCap,omitempty"`       // 市值规模描述 (如 "2500亿", "500亿")
	Description string    `gorm:"type:text" json:"description,omitempty"`   // 核心技术特征与业务概括
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (IndustryNode) TableName() string {
	return "industry_nodes"
}

// IndustryEdge 产业链传导关系边
type IndustryEdge struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SourceID  string    `gorm:"size:64;index;not null" json:"sourceId"` // 源节点 ID
	TargetID  string    `gorm:"size:64;index;not null" json:"targetId"` // 目标节点 ID
	Relation  string    `gorm:"size:64;not null" json:"relation"`       // SUPPLIES_TO, UPSTREAM_OF, DIRECT_BENEFICIARY, CORE_OEM
	Weight    float64   `json:"weight"`                                 // 传导敏感度/业务暴露度权重 (0.0 ~ 1.0)
	Notes     string    `gorm:"type:text" json:"notes"`                 // 传导逻辑与供货份额说明
	CreatedAt time.Time `json:"createdAt"`
}

func (IndustryEdge) TableName() string {
	return "industry_edges"
}

// CascadeStockCandidate 级联推演挖掘出的候选受益标的
type CascadeStockCandidate struct {
	StockCode       string  `json:"stockCode"`
	StockName       string  `json:"stockName"`
	SupplyRelation  string  `json:"supplyRelation"`  // 供货角色与产业链环节
	RevenueExposure string  `json:"revenueExposure"` // 业务收入占比或订单暴露度估计
	CurrentPrice    float64 `json:"currentPrice"`    // 最新价格
	ChangePercent   float64 `json:"changePercent"`   // 当日/最新涨跌幅 (%)
	PETTM           float64 `json:"peTtm"`           // 市盈率 (TTM)
	TechnicalTrend  string  `json:"technicalTrend"`  // 技术面多空形态 (如: "MA5>MA20多头排列", "平台放量突破")
	ConfidenceScore float64 `json:"confidenceScore"` // 级联传导置信度评分 (0~100)
	ActionPlan      string  `json:"actionPlan"`      // 建议买方行动剧本 (如: "次日竞价关注高开幅度，逢回踩支撑布局")
}

// CascadeEventResult 跨市场与产业链事件级联推演研判结果
type CascadeEventResult struct {
	EventSource       string                  `json:"eventSource"`       // 事件源头 (如 "英伟达 (NVDA)")
	Headline          string                  `json:"headline"`          // 核心催化事件
	ImpactLevel       string                  `json:"impactLevel"`       // 冲击力度: HIGH, MEDIUM, LOW
	AffectedSectors   []string                `json:"affectedSectors"`   // 直接拉动的A股板块链
	BeneficiaryStocks []CascadeStockCandidate `json:"beneficiaryStocks"` // 核心受益标的梯队
	RadarSummary      string                  `json:"radarSummary"`      // 买方雷达研报正文 (Markdown)
	CreatedAt         time.Time               `json:"createdAt"`
}
