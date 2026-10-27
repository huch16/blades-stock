package models

import (
	"time"
)

// PaperPortfolio 智能体专属虚拟模拟基金账户
type PaperPortfolio struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	Name            string    `gorm:"size:128;uniqueIndex" json:"name"` // 基金名称，如 "财智1号买方量化先锋基金"
	InitialCash     float64   `json:"initialCash"`                      // 初始本金 (默认 10,000,000.0 元)
	CashBalance     float64   `json:"cashBalance"`                      // 当前可用现金余额
	TotalAssetValue float64   `json:"totalAssetValue"`                  // 当前基金总资产净值
	CurrentNAV      float64   `json:"currentNav"`                       // 累计单位净值 (初始 1.0000)
	PeakNAV         float64   `json:"peakNav"`                          // 历史最高单位净值
	MaxDrawdown     float64   `json:"maxDrawdown"`                      // 历史最大动态回撤 (%)
	TotalReturnRate float64   `json:"totalReturnRate"`                  // 历史累计总收益率 (%)
	SharpeRatio     float64   `json:"sharpeRatio"`                      // 年化夏普比率
	WinRate         float64   `json:"winRate"`                          // 交易胜率 (%)
	TotalTrades     int       `json:"totalTrades"`                      // 累计平仓交易次数
	WinningTrades   int       `json:"winningTrades"`                    // 盈利交易次数
	LosingTrades    int       `json:"losingTrades"`                     // 亏损交易次数
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (PaperPortfolio) TableName() string {
	return "paper_portfolios"
}

// PaperPosition 虚拟持仓明细
type PaperPosition struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	PortfolioID     uint      `gorm:"index;not null" json:"portfolioId"`
	StockCode       string    `gorm:"size:32;index;not null" json:"stockCode"`
	StockName       string    `gorm:"size:64" json:"stockName"`
	Shares          int       `json:"shares"`          // 持有股数 (100股整数倍)
	AvgCostPrice    float64   `json:"avgCostPrice"`    // 平均持仓成本价
	CurrentPrice    float64   `json:"currentPrice"`    // 最新市价
	MarketValue     float64   `json:"marketValue"`     // 最新持仓市值
	FloatingPnL     float64   `json:"floatingPnl"`     // 浮动盈亏额 (元)
	FloatingPnLRate float64   `json:"floatingPnlRate"` // 浮动盈亏比例 (%)
	StopLossPrice   float64   `json:"stopLossPrice"`   // 挂钩的主动止损线
	TakeProfitPrice float64   `json:"takeProfitPrice"` // 挂钩的预期止盈线
	Weight          float64   `json:"weight"`          // 仓位占总资产比例 (%)
	InitialBuyTime  time.Time `json:"initialBuyTime"`  // 首次建仓时间
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (PaperPosition) TableName() string {
	return "paper_positions"
}

// PaperTradeOrder 虚拟交易成交订单明细
type PaperTradeOrder struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	PortfolioID uint      `gorm:"index;not null" json:"portfolioId"`
	StockCode   string    `gorm:"size:32;index;not null" json:"stockCode"`
	StockName   string    `gorm:"size:64" json:"stockName"`
	Side        string    `gorm:"size:16;not null" json:"side"` // BUY (买入开仓/加仓), SELL (卖出平仓/减仓)
	OrderPrice  float64   `json:"orderPrice"`                   // 委托基准价
	FilledPrice float64   `json:"filledPrice"`                  // 考虑滑点后的实际撮合成交价
	Slippage    float64   `json:"slippage"`                     // 模拟滑点 (默认千分之一)
	Shares      int       `json:"shares"`                       // 成交股数
	Amount      float64   `json:"amount"`                       // 成交金额
	Fee         float64   `json:"fee"`                          // 模拟交易佣金与印花税
	RealizedPnL float64   `json:"realizedPnl"`                  // 平仓已实现盈亏 (买入为 0)
	Reason      string    `gorm:"size:64" json:"reason"`        // 成交动因: AI_RECOMMEND, STOP_LOSS, TAKE_PROFIT, THESIS_BROKEN
	OrderTime   time.Time `gorm:"index" json:"orderTime"`
}

func (PaperTradeOrder) TableName() string {
	return "paper_trade_orders"
}

// PaperDailyNAV 每日净值快照记录 (用于绘制基金收益曲线与夏普计算)
type PaperDailyNAV struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	PortfolioID     uint      `gorm:"index;not null" json:"portfolioId"`
	TradeDate       string    `gorm:"size:32;index;not null" json:"tradeDate"` // YYYY-MM-DD
	NAV             float64   `json:"nav"`                                     // 当日单位净值
	TotalAsset      float64   `json:"totalAsset"`                              // 总资产
	Cash            float64   `json:"cash"`                                    // 现金
	PositionValue   float64   `json:"positionValue"`                           // 持仓总市值
	DailyReturnRate float64   `json:"dailyReturnRate"`                         // 当日涨跌幅 (%)
	DrawdownRate    float64   `json:"drawdownRate"`                            // 相对峰值回撤率 (%)
	CreatedAt       time.Time `json:"createdAt"`
}

func (PaperDailyNAV) TableName() string {
	return "paper_daily_navs"
}

// PaperPortfolioStats 模拟盘综合报表输出
type PaperPortfolioStats struct {
	Portfolio PaperPortfolio    `json:"portfolio"`
	Positions []PaperPosition   `json:"positions"`
	RecentOrders []PaperTradeOrder `json:"recentOrders"`
	NAVHistory []PaperDailyNAV  `json:"navHistory"`
}
