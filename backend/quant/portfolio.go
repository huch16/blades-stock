package quant

// ============================================================================
// Portfolio Simulation Engine - VectorBT Portfolio.from_signals() 的 Go 实现
// 核心：矩阵运算驱动的组合模拟
// ============================================================================

import "math"

// ============================================================================
// PortfolioConfig - 组合配置
// ============================================================================

// PortfolioConfig 组合配置
type PortfolioConfig struct {
	InitCash     float64 // 初始资金
	Fees         float64 // 手续费率（‰）
	Slippage     float64 // 滑点率（‰）
	StopLoss     float64 // 止损率（%，0表示不禁用）
	TakeProfit    float64 // 止盈率（%，0表示不禁用）
	TrailStop    float64 // 移动止损率（%，0表示不使用）
	ShortEnabled  bool    // 是否允许做空
	Size         float64 // 每笔交易金额比例（0-1，1表示全仓）
	SizeType     SizeType // 仓位计算方式
	UseExitClose bool    // 退出时使用收盘价
	AllowSizeScaling bool // 允许仓位缩放
}

// SizeType 仓位计算方式
type SizeType int

const (
	SizeTypeValue   SizeType = iota // 固定金额
	SizeTypePercent                 // 资金比例
	SizeTypePercentTotal            // 总资金比例
)

// DefaultPortfolioConfig 默认配置（参考 VectorBT）
func DefaultPortfolioConfig() *PortfolioConfig {
	return &PortfolioConfig{
		InitCash:    100000,       // 10万初始资金
		Fees:        1.5,          // 1.5‰ 手续费（单边约0.15%）
		Slippage:    1.0,          // 1‰ 滑点
		StopLoss:    0.05,         // 5% 止损
		TakeProfit:  0.10,         // 10% 止盈
		TrailStop:   0,            // 不使用移动止损
		ShortEnabled: false,       // 默认不做空
		Size:        1.0,          // 全仓
		SizeType:    SizeTypePercent,
		UseExitClose: true,
	}
}

// ============================================================================
// Trade - 交易记录
// ============================================================================

// TradeType 交易类型
type TradeType int

const (
	TradeTypeLong TradeType = iota
	TradeTypeShort
	TradeTypeLongExit
	TradeTypeShortExit
)

// Trade 交易记录
type Trade struct {
	ID         int
	EntryIndex int    // 入场索引
	ExitIndex  int    // 出场索引
	EntryPrice float64 // 入场价格
	ExitPrice  float64 // 出场价格
	EntryCash  float64 // 入场时现金
	Size       float64 // 持仓数量
	TradeType  TradeType
	PnL        float64 // 盈亏金额（含手续费）
	Return     float64 // 收益率
	ExitReason string  // 出场原因
}

// ============================================================================
// Position - 持仓状态
// ============================================================================

// Position 持仓状态
type Position struct {
	EntryIndex  int     // 入场索引
	EntryPrice  float64
	Size        float64
	TradeType   TradeType
	EntryCash   float64
	StopLoss    float64 // 止损价
	TakeProfit  float64 // 止盈价
	HighestPrice float64 // 跟踪最高价（用于移动止损）
	LowestPrice  float64 // 跟踪最低价（用于做空）
}

// ============================================================================
// Portfolio - 组合模拟器（参考 VectorBT Portfolio.from_signals）
// ============================================================================

// Portfolio 向量化组合模拟器
type Portfolio struct {
	config  *PortfolioConfig
	shares  []float64   // 每期持仓数量
	cash    []float64   // 每期现金
	equity  []float64   // 每期总权益
	value   []float64   // 每期组合价值（权益+现金）
	trades  []Trade
	status  []int       // 每期状态: 0=空仓, 1=持仓
	entries []bool      // 入场信号
	exits   []bool      // 出场信号
	close   *Vector     // 收盘价
	tradeID int

	// 向量化运算结果
	returns     []float64 // 每日收益率
	cumReturns  []float64 // 累计收益率
	drawdowns  []float64 // 回撤序列
}

// FromSignals 从信号创建组合（VectorBT Portfolio.from_signals 等效）
func FromSignals(
	closePrices []float64,
	entries []bool,
	exits []bool,
	config *PortfolioConfig,
) *Portfolio {
	if config == nil {
		config = DefaultPortfolioConfig()
	}

	n := len(closePrices)
	pf := &Portfolio{
		config:  config,
		shares:  make([]float64, n),
		cash:    make([]float64, n),
		equity:  make([]float64, n),
		value:   make([]float64, n),
		trades:  make([]Trade, 0, 100),
		status:  make([]int, n),
		entries: entries,
		exits:   exits,
		close:   NewVector(closePrices),
	}

	if n == 0 {
		return pf
	}

	// 初始化
	pf.cash[0] = config.InitCash
	pf.equity[0] = config.InitCash
	pf.value[0] = config.InitCash
	pf.status[0] = 0

	// SL/TP/exit 缓存
	slSignals := make([]bool, n)
	tpSignals := make([]bool, n)
	exitSignals := make([]bool, n)
	for i := 0; i < n; i++ {
		exitSignals[i] = exits[i]
	}

	// 逐根K线处理（但核心计算向量化）
	pos := (*Position)(nil)

	for i := 1; i < n; i++ {
		pf.cash[i] = pf.cash[i-1]
		pf.shares[i] = pf.shares[i-1]
		pf.status[i] = pf.status[i-1]

		price := closePrices[i]

		// SL/TP 信号生成
		if pos != nil {
			// 止损检查
			if config.StopLoss > 0 {
				if pos.TradeType == TradeTypeLong {
					slPrice := pos.EntryPrice * (1 - config.StopLoss)
					if price <= slPrice {
						slSignals[i] = true
					}
					// 移动止损
					if config.TrailStop > 0 {
						if price > pos.HighestPrice {
							pos.HighestPrice = price
						}
						trailSL := pos.HighestPrice * (1 - config.TrailStop)
						if price <= trailSL {
							slSignals[i] = true
						}
					}
				} else if pos.TradeType == TradeTypeShort && config.ShortEnabled {
					slPrice := pos.EntryPrice * (1 + config.StopLoss)
					if price >= slPrice {
						slSignals[i] = true
					}
				}
			}
			// 止盈检查
			if config.TakeProfit > 0 {
				if pos.TradeType == TradeTypeLong {
					tpPrice := pos.EntryPrice * (1 + config.TakeProfit)
					if price >= tpPrice {
						tpSignals[i] = true
					}
				} else if pos.TradeType == TradeTypeShort && config.ShortEnabled {
					tpPrice := pos.EntryPrice * (1 - config.TakeProfit)
					if price <= tpPrice {
						tpSignals[i] = true
					}
				}
			}
		}

		// 检查出场
		shouldExit := slSignals[i] || tpSignals[i] || exitSignals[i]

		// 如果持仓，检查是否需要出场
		if pos != nil {
			if shouldExit {
				// 平仓
				exitPrice := price
				if pf.config.UseExitClose {
					exitPrice = price
				}

				// 扣手续费+滑点
				exitPrice = applyFees(exitPrice, pf.config.Fees, pf.config.Slippage)

				var exitReason string
				if slSignals[i] {
					exitReason = "stop_loss"
				} else if tpSignals[i] {
					exitReason = "take_profit"
				} else {
					exitReason = "signal"
				}

				pnl := calculatePnL(pos, exitPrice, pf.config)
				pf.cash[i] = pf.cash[i-1] + pos.EntryCash + pnl
				pf.shares[i] = 0
				pf.status[i] = 0

				pf.trades = append(pf.trades, Trade{
					ID:         pf.tradeID,
					EntryIndex: pos.EntryIndex,
					ExitIndex:  i,
					EntryPrice: pos.EntryPrice,
					ExitPrice:  exitPrice,
					EntryCash:  pos.EntryCash,
					Size:       pos.Size,
					TradeType:  pos.TradeType,
					PnL:        pnl,
					Return:     pnl / pos.EntryCash,
					ExitReason: exitReason,
				})
				pf.tradeID++
				pos = nil
			}
		}

		// 如果空仓，检查是否需要入场
		if pos == nil && i < n-1 {
			// 使用下一根K线开盘价入场
			nextPrice := closePrices[i]
			if entries[i] {
				entryPrice := applyFees(nextPrice, pf.config.Fees, pf.config.Slippage)
				// 计算入场金额
				entryValue := pf.cash[i] * pf.config.Size

				var size float64
				var tradeType TradeType
				if pf.config.ShortEnabled {
					// 做多
					size = entryValue / entryPrice
					tradeType = TradeTypeLong
				} else {
					size = entryValue / entryPrice
					tradeType = TradeTypeLong
				}

				pf.cash[i] -= entryPrice * size // 扣除买入成本+手续费
				pf.shares[i] = size
				pf.status[i] = 1
				pos = &Position{
					EntryIndex:  i,
					EntryPrice:  entryPrice,
					Size:        size,
					TradeType:   tradeType,
					EntryCash:   entryValue,
					HighestPrice: entryPrice,
					LowestPrice:  entryPrice,
				}
			}
		}

		// 更新权益
		pf.equity[i] = pf.cash[i] + pf.shares[i]*price
		pf.value[i] = pf.equity[i]
	}

	// 强制平仓（最后一天）
	if pos != nil {
		lastPrice := closePrices[n-1]
		lastPrice = applyFees(lastPrice, pf.config.Fees, pf.config.Slippage)
		pnl := calculatePnL(pos, lastPrice, pf.config)
		pf.cash[n-1] += pos.EntryCash + pnl
		pf.equity[n-1] = pf.cash[n-1]
		pf.value[n-1] = pf.cash[n-1]

		pf.trades = append(pf.trades, Trade{
			ID:         pf.tradeID,
			EntryIndex: pos.EntryIndex,
			ExitIndex:  n - 1,
			EntryPrice: pos.EntryPrice,
			ExitPrice:  lastPrice,
			EntryCash:  pos.EntryCash,
			Size:       pos.Size,
			TradeType:  pos.TradeType,
			PnL:        pnl,
			Return:     pnl / pos.EntryCash,
			ExitReason: "end_of_data",
		})
	}

	// 向量化计算收益率和回撤
	pf.computeReturns()
	pf.computeDrawdowns()

	return pf
}

// calculatePnL 计算盈亏
func calculatePnL(pos *Position, exitPrice float64, cfg *PortfolioConfig) float64 {
	if pos.TradeType == TradeTypeLong {
		return (exitPrice - pos.EntryPrice) * pos.Size
	}
	// Short
	return (pos.EntryPrice - exitPrice) * pos.Size
}

// applyFees 应用手续费和滑点
func applyFees(price, fees, slippage float64) float64 {
	// 先扣手续费再扣滑点
	return price * (1 - fees/1000 - slippage/1000)
}

// computeReturns 向量化计算每日收益率
func (pf *Portfolio) computeReturns() {
	n := len(pf.value)
	pf.returns = make([]float64, n)
	pf.cumReturns = make([]float64, n)
	if n < 2 {
		return
	}
	initCash := pf.config.InitCash
	for i := 1; i < n; i++ {
		if pf.value[i-1] > 0 {
			pf.returns[i] = (pf.value[i] - pf.value[i-1]) / pf.value[i-1]
		}
	}
	// 累计收益率
	pf.cumReturns[0] = 0
	for i := 1; i < n; i++ {
		pf.cumReturns[i] = (pf.value[i] - initCash) / initCash
	}
}

// computeDrawdowns 向量化计算回撤序列
func (pf *Portfolio) computeDrawdowns() {
	n := len(pf.value)
	pf.drawdowns = make([]float64, n)
	if n == 0 {
		return
	}
	peak := pf.value[0]
	for i := 0; i < n; i++ {
		if pf.value[i] > peak {
			peak = pf.value[i]
		}
		if peak > 0 {
			pf.drawdowns[i] = (peak - pf.value[i]) / peak
		}
	}
}

// ============================================================================
// Portfolio Metrics - 组合指标
// ============================================================================

// Stats 组合统计指标
type Stats struct {
	TotalReturn      float64 // 总收益率
	AnnualReturn     float64 // 年化收益率
	SharpeRatio      float64 // 夏普比率
	MaxDrawdown      float64 // 最大回撤
	MaxDrawdownPct   float64 // 最大回撤率（%）
	WinRate          float64 // 胜率
	ProfitFactor     float64 // 盈亏比
	TotalTrades     int     // 总交易次数
	WinTrades        int     // 盈利次数
	LoseTrades       int     // 亏损次数
	AvgWin           float64 // 平均盈利
	AvgLose          float64 // 平均亏损
	AvgHoldingDays   float64 // 平均持仓天数
	MaxConsecutiveWin int    // 最大连续盈利
	MaxConsecutiveLose int   // 最大连续亏损
	CalmarRatio      float64 // 卡玛比率
	SortinoRatio     float64 // 索提诺比率
	Volatility       float64 // 波动率
	FinalValue       float64 // 最终资金
	InitCash         float64 // 初始资金
	TotalDays        int     // 总天数
}

// GetStats 获取组合统计（参考 VectorBT stats 属性）
func (pf *Portfolio) GetStats() *Stats {
	n := len(pf.value)
	if n == 0 {
		return &Stats{}
	}

	initCash := pf.config.InitCash
	finalValue := pf.value[n-1]
	totalReturn := (finalValue - initCash) / initCash

	// 年化收益率（假设252交易日）
	totalDays := n
	years := float64(totalDays) / 252.0
	annualReturn := 0.0
	if years > 0 {
		annualReturn = math.Pow(1+totalReturn, 1/years) - 1
	}

	// 最大回撤
	maxDD := 0.0
	for _, dd := range pf.drawdowns {
		if dd > maxDD {
			maxDD = dd
		}
	}

	// 波动率
	var vol float64
	returns := pf.returns
	if len(returns) > 1 {
		annualizedVol := NewVector(returns).Std() * math.Sqrt(252)
		vol = annualizedVol
	}

	// 夏普比率（无风险利率假设为0）
	sharpe := 0.0
	if vol > 0 {
		sharpe = annualReturn / vol
	}

	// 交易统计
	winTrades := 0
	loseTrades := 0
	totalWin := 0.0
	totalLose := 0.0
	var totalHoldingDays float64
	var maxConsecutiveWin, maxConsecutiveLose, currentConsecutiveWin, currentConsecutiveLose int

	for _, t := range pf.trades {
		if t.PnL > 0 {
			winTrades++
			totalWin += t.PnL
			currentConsecutiveWin++
			currentConsecutiveLose = 0
		} else if t.PnL < 0 {
			loseTrades++
			totalLose += math.Abs(t.PnL)
			currentConsecutiveLose++
			currentConsecutiveWin = 0
		}
		totalHoldingDays += float64(t.ExitIndex - t.EntryIndex)

		if currentConsecutiveWin > maxConsecutiveWin {
			maxConsecutiveWin = currentConsecutiveWin
		}
		if currentConsecutiveLose > maxConsecutiveLose {
			maxConsecutiveLose = currentConsecutiveLose
		}
	}

	winRate := 0.0
	if len(pf.trades) > 0 {
		winRate = float64(winTrades) / float64(len(pf.trades))
	}
	avgWin := 0.0
	if winTrades > 0 {
		avgWin = totalWin / float64(winTrades)
	}
	avgLose := 0.0
	if loseTrades > 0 {
		avgLose = totalLose / float64(loseTrades)
	}
	profitFactor := 0.0
	if totalLose > 0 {
		profitFactor = totalWin / totalLose
	}
	avgHoldingDays := 0.0
	if len(pf.trades) > 0 {
		avgHoldingDays = totalHoldingDays / float64(len(pf.trades))
	}

	// 卡玛比率
	calmar := 0.0
	if maxDD > 0 && years > 0 {
		calmar = annualReturn / maxDD
	}

	// 索提诺比率（下跌波动率）
	downsideReturns := make([]float64, 0)
	for _, r := range returns[1:] {
		if r < 0 {
			downsideReturns = append(downsideReturns, r)
		}
	}
	downsideVol := 0.0
	if len(downsideReturns) > 0 {
		downsideVol = NewVector(downsideReturns).Std() * math.Sqrt(252)
	}
	sortino := 0.0
	if downsideVol > 0 {
		sortino = annualReturn / downsideVol
	}

	return &Stats{
		TotalReturn:       totalReturn,
		AnnualReturn:      annualReturn,
		SharpeRatio:       sharpe,
		MaxDrawdown:       maxDD * initCash,
		MaxDrawdownPct:    maxDD * 100,
		WinRate:           winRate,
		ProfitFactor:      profitFactor,
		TotalTrades:       len(pf.trades),
		WinTrades:         winTrades,
		LoseTrades:        loseTrades,
		AvgWin:            avgWin,
		AvgLose:           avgLose,
		AvgHoldingDays:    avgHoldingDays,
		MaxConsecutiveWin: maxConsecutiveWin,
		MaxConsecutiveLose: maxConsecutiveLose,
		CalmarRatio:       calmar,
		SortinoRatio:      sortino,
		Volatility:        vol,
		FinalValue:        finalValue,
		InitCash:          initCash,
		TotalDays:         totalDays,
	}
}

// ============================================================================
// FromHolding - 买入持有基准
// ============================================================================

// FromHolding 买入持有策略（基准对比）
func FromHolding(closePrices []float64, initCash float64) *Portfolio {
	n := len(closePrices)
	if n == 0 {
		return nil
	}
	shares := initCash / closePrices[0]
	equity := make([]float64, n)
	for i := 0; i < n; i++ {
		equity[i] = shares * closePrices[i]
	}
	pf := &Portfolio{
		config:  &PortfolioConfig{InitCash: initCash},
		value:   equity,
		equity:  equity,
		cash:    make([]float64, n),
		shares:  make([]float64, n),
	}
	pf.computeReturns()
	pf.computeDrawdowns()
	return pf
}

// GetHoldingStats 买入持有统计
func (pf *Portfolio) GetHoldingStats() *Stats {
	return pf.GetStats()
}
