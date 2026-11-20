package quant

// ============================================================================
// Backtest Engine - VectorBT 风格的向量化回测引擎
// 完整替代 blades-stock 的 event-driven backtest 模块
// ============================================================================

import (
	"context"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// BacktestEngine - 主回测引擎
// ============================================================================

// BacktestEngine 向量化回测引擎
type BacktestEngine struct {
	// 数据
	Close  *Vector
	High   *Vector
	Low    *Vector
	Open   *Vector
	Volume *Vector
	Dates  []string

	// 配置
	Config          *BacktestConfig
	PortfolioConfig *PortfolioConfig

	// 上下文
	ctx    context.Context
	cancel context.CancelFunc
}

// BacktestConfig 回测配置
type BacktestConfig struct {
	Name              string  // 策略名称
	InitCash          float64 // 初始资金
	Fees              float64 // 手续费‰
	Slippage          float64 // 滑点‰
	StopLoss          float64 // 止损%
	TakeProfit        float64 // 止盈%
	TrailStop         float64 // 移动止损%
	VerifyDays        int     // 验证窗口天数
	AccuracyThreshold float64 // 准确率阈值%
	ShortEnabled      bool    // 允许做空
}

// DefaultBacktestConfig 默认配置
func DefaultBacktestConfig() *BacktestConfig {
	return &BacktestConfig{
		Name:              "default",
		InitCash:          100000,
		Fees:              1.5,
		Slippage:          1.0,
		StopLoss:          0.05,
		TakeProfit:        0.10,
		TrailStop:         0,
		VerifyDays:        20,
		AccuracyThreshold: 5.0,
		ShortEnabled:      false,
	}
}

// ToPortfolioConfig 转换为 PortfolioConfig
func (bc *BacktestConfig) ToPortfolioConfig() *PortfolioConfig {
	return &PortfolioConfig{
		InitCash:     bc.InitCash,
		Fees:         bc.Fees,
		Slippage:     bc.Slippage,
		StopLoss:     bc.StopLoss,
		TakeProfit:   bc.TakeProfit,
		TrailStop:    bc.TrailStop,
		ShortEnabled: bc.ShortEnabled,
		Size:         1.0,
		SizeType:     SizeTypePercent,
		UseExitClose: true,
	}
}

// ============================================================================
// 回测结果
// ============================================================================

// Trades 交易记录指针版本
type TradeRef struct {
	ID         int
	EntryIndex int
	ExitIndex  int
	EntryPrice float64
	ExitPrice  float64
	PnL        float64
	Return     float64
	ExitReason string
}

func tradeToRef(t Trade) TradeRef {
	return TradeRef{
		ID:         t.ID,
		EntryIndex: t.EntryIndex,
		ExitIndex:  t.ExitIndex,
		EntryPrice: t.EntryPrice,
		ExitPrice:  t.ExitPrice,
		PnL:        t.PnL,
		Return:     t.Return,
		ExitReason: t.ExitReason,
	}
}

// BacktestResult 单次回测结果
type BacktestResult struct {
	StrategyName string
	Params       map[string]any
	Stats        *Stats
	Trades       []TradeRef
	Dates        []string
	Equity       []float64
	Drawdowns    []float64
	Returns      []float64
	Duration     time.Duration
}

// GridSearchResult 参数扫描结果
type GridSearchResult struct {
	TotalCombinations int
	Results           []*BacktestResult
	BestParams        map[string]any
	BestStats         *Stats
	BestStrategyName  string
	Duration          time.Duration
}

// ============================================================================
// Builder - 链式构建器
// ============================================================================

// Builder 回测构建器
type Builder struct {
	engine *BacktestEngine
}

// NewBacktestEngine 创建回测引擎
func NewBacktestEngine() *Builder {
	ctx, cancel := context.WithCancel(context.Background())
	return &Builder{
		engine: &BacktestEngine{
			ctx:    ctx,
			cancel: cancel,
			Config: DefaultBacktestConfig(),
		},
	}
}

// Data 设置数据
func (b *Builder) Data(close, high, low, open, volume []float64, dates []string) *Builder {
	b.engine.Close = NewVector(close)
	b.engine.High = NewVector(high)
	b.engine.Low = NewVector(low)
	b.engine.Open = NewVector(open)
	b.engine.Volume = NewVector(volume)
	b.engine.Dates = dates
	return b
}

// CloseOnly 仅设置收盘价
func (b *Builder) CloseOnly(close []float64, dates []string) *Builder {
	b.engine.Close = NewVector(close)
	b.engine.Dates = dates
	return b
}

// Config 设置配置
func (b *Builder) Config(cfg *BacktestConfig) *Builder {
	b.engine.Config = cfg
	return b
}

// InitCash 设置初始资金
func (b *Builder) InitCash(cash float64) *Builder {
	b.engine.Config.InitCash = cash
	return b
}

// Fees 设置手续费
func (b *Builder) Fees(fees float64) *Builder {
	b.engine.Config.Fees = fees
	return b
}

// StopLoss 设置止损
func (b *Builder) StopLoss(sl float64) *Builder {
	b.engine.Config.StopLoss = sl
	return b
}

// TakeProfit 设置止盈
func (b *Builder) TakeProfit(tp float64) *Builder {
	b.engine.Config.TakeProfit = tp
	return b
}

// Build 构建引擎
func (b *Builder) Build() *BacktestEngine {
	if b.engine.PortfolioConfig == nil {
		b.engine.PortfolioConfig = b.engine.Config.ToPortfolioConfig()
	}
	return b.engine
}

// ============================================================================
// 策略接口
// ============================================================================

// Strategy 策略接口
type Strategy interface {
	Name() string
	Params() map[string]any
	GenerateSignals(close, high, low, open, volume *Vector) (entries, exits *BoolVector)
}

// ============================================================================
// 预定义策略
// ============================================================================

// MAStrategy 均线交叉策略
type MAStrategy struct {
	FastWindow int
	SlowWindow int
}

func (s *MAStrategy) Name() string {
	return "MA"
}

func (s *MAStrategy) Params() map[string]any {
	return map[string]any{"fast": s.FastWindow, "slow": s.SlowWindow}
}

func (s *MAStrategy) GenerateSignals(close, high, low, open, volume *Vector) (entries, exits *BoolVector) {
	fastMA := NewSMA(close, s.FastWindow)
	slowMA := NewSMA(close, s.SlowWindow)
	entries = fastMA.CrossAboveVec(slowMA)
	exits = fastMA.CrossBelowVec(slowMA)
	return
}

// MACDStrategy MACD 策略
type MACDStrategy struct {
	Fast   int
	Slow   int
	Signal int
}

func (s *MACDStrategy) Name() string {
	return "MACD"
}

func (s *MACDStrategy) Params() map[string]any {
	return map[string]any{"fast": s.Fast, "slow": s.Slow, "signal": s.Signal}
}

func (s *MACDStrategy) GenerateSignals(close, high, low, open, volume *Vector) (entries, exits *BoolVector) {
	macd := NewMACD(close, s.Fast, s.Slow, s.Signal)
	entries = macd.MACDCrossAbove()
	exits = macd.MACDCrossBelow()
	return
}

// RSIStrategy RSI 策略
type RSIStrategy struct {
	Window     int
	Overbought float64
	Oversold   float64
}

func (s *RSIStrategy) Name() string {
	return "RSI"
}

func (s *RSIStrategy) Params() map[string]any {
	return map[string]any{"window": s.Window, "overbought": s.Overbought, "oversold": s.Oversold}
}

func (s *RSIStrategy) GenerateSignals(close, high, low, open, volume *Vector) (entries, exits *BoolVector) {
	rsi := NewRSI(close, s.Window)
	entries = rsi.RSIOversold(s.Oversold)
	exits = rsi.RSIOverbought(s.Overbought)
	return
}

// BollingerStrategy 布林带策略
type BollingerStrategy struct {
	Window int
	NumStd float64
}

func (s *BollingerStrategy) Name() string {
	return "BOLL"
}

func (s *BollingerStrategy) Params() map[string]any {
	return map[string]any{"window": s.Window, "num_std": s.NumStd}
}

func (s *BollingerStrategy) GenerateSignals(close, high, low, open, volume *Vector) (entries, exits *BoolVector) {
	boll := NewBollinger(close, s.Window, s.NumStd)
	// 价格触及下轨买入，上穿上轨卖出
	entries = boll.PriceBelowLower(close)
	exits = boll.PriceAboveUpper(close)
	return
}

// DualThrustStrategy Dual Thrust 策略
type DualThrustStrategy struct {
	K      float64
	Period int
}

func (s *DualThrustStrategy) Name() string {
	return "DualThrust"
}

func (s *DualThrustStrategy) Params() map[string]any {
	return map[string]any{"k": s.K, "period": s.Period}
}

func (s *DualThrustStrategy) GenerateSignals(close, high, low, open, volume *Vector) (entries, exits *BoolVector) {
	n := s.Period
	if n <= 0 || n > close.Len {
		entries = NewBoolZeros(close.Len)
		exits = NewBoolZeros(close.Len)
		return
	}

	hh := high.RollingMax(n)
	hc := close.RollingMax(n)
	lc := close.RollingMin(n)
	ll := low.RollingMin(n)

	// range = max(HH, HC) - min(LC, LL)
	hhHc := hh.Max()
	if hc.Max() > hhHc {
		hhHc = hc.Max()
	}
	lcLl := lc.Min()
	if ll.Min() < lcLl {
		lcLl = ll.Min()
	}
	rangeVal := hhHc - lcLl

	openVals := open.Values
	openShift := NewVector(openVals).Shift(1)

	upper := openShift.AddScalar(s.K * rangeVal).NanToNum(0)
	lower := openShift.SubScalar(s.K * rangeVal).NanToNum(0)

	entries = close.GreaterVec(upper)
	exits = close.LessVec(lower)
	return
}

// ============================================================================
// 回测运行
// ============================================================================

// Run 运行单次回测
func (e *BacktestEngine) Run(strategy Strategy) *BacktestResult {
	start := time.Now()

	close := e.Close
	high := e.High
	low := e.Low
	open := e.Open
	volume := e.Volume

	if close == nil || close.Len == 0 {
		return &BacktestResult{
			StrategyName: strategy.Name(),
			Params:       strategy.Params(),
			Duration:     time.Since(start),
		}
	}

	// 生成信号
	entries, exits := strategy.GenerateSignals(close, high, low, open, volume)

	// 转换为 []bool
	entriesBool := make([]bool, entries.Len)
	exitsBool := make([]bool, exits.Len)
	copy(entriesBool, entries.Values)
	copy(exitsBool, exits.Values)

	// 模拟组合
	pf := FromSignals(close.Values, entriesBool, exitsBool, e.PortfolioConfig)
	stats := pf.GetStats()

	tradeRefs := make([]TradeRef, len(pf.trades))
	for i, t := range pf.trades {
		tradeRefs[i] = tradeToRef(t)
	}

	return &BacktestResult{
		StrategyName: strategy.Name(),
		Params:       strategy.Params(),
		Stats:        stats,
		Trades:       tradeRefs,
		Dates:        e.Dates,
		Equity:       pf.value,
		Drawdowns:    pf.drawdowns,
		Returns:      pf.returns,
		Duration:     time.Since(start),
	}
}

// RunGridSearch 运行参数网格扫描
func (e *BacktestEngine) RunGridSearch(strategies []Strategy) *GridSearchResult {
	start := time.Now()

	if len(strategies) == 0 {
		return &GridSearchResult{Duration: time.Since(start)}
	}

	// 并行处理
	workers := runtime.NumCPU()
	if workers > len(strategies) {
		workers = len(strategies)
	}
	if workers < 1 {
		workers = 1
	}

	results := make([]*BacktestResult, len(strategies))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := workerID; j < len(strategies); j += workers {
				select {
				case <-e.ctx.Done():
					return
				default:
				}
				result := e.Run(strategies[j])
				mu.Lock()
				results[j] = result
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// 找出最佳参数 (防范 results[0] 为 nil 或并发异常引发的空指针崩溃)
	var bestResult *BacktestResult
	for _, r := range results {
		if r != nil && r.Stats != nil {
			if bestResult == nil || bestResult.Stats == nil || r.Stats.SharpeRatio > bestResult.Stats.SharpeRatio {
				bestResult = r
			}
		}
	}

	var bestParams map[string]any
	var bestStats *Stats
	var bestStrategyName string
	if bestResult != nil {
		bestParams = bestResult.Params
		bestStats = bestResult.Stats
		bestStrategyName = bestResult.StrategyName
	}

	return &GridSearchResult{
		TotalCombinations: len(strategies),
		Results:           results,
		BestParams:        bestParams,
		BestStats:         bestStats,
		BestStrategyName:  bestStrategyName,
		Duration:          time.Since(start),
	}
}

// ============================================================================
// 批量策略生成器
// ============================================================================

// GenerateMAStrategies 生成均线策略组合
func GenerateMAStrategies(fastWindows, slowWindows []int) []Strategy {
	var strategies []Strategy
	for _, fast := range fastWindows {
		for _, slow := range slowWindows {
			if fast < slow {
				strategies = append(strategies, &MAStrategy{FastWindow: fast, SlowWindow: slow})
			}
		}
	}
	return strategies
}

// GenerateMACDStrategies 生成 MACD 策略组合
func GenerateMACDStrategies(fastRange, slowRange, signalRange []int) []Strategy {
	var strategies []Strategy
	for _, fast := range fastRange {
		for _, slow := range slowRange {
			if fast >= slow {
				continue
			}
			for _, sig := range signalRange {
				strategies = append(strategies, &MACDStrategy{Fast: fast, Slow: slow, Signal: sig})
			}
		}
	}
	return strategies
}

// GenerateRSIStrategies 生成 RSI 策略组合
func GenerateRSIStrategies(windows []int, overbought, oversold float64) []Strategy {
	var strategies []Strategy
	for _, w := range windows {
		strategies = append(strategies, &RSIStrategy{Window: w, Overbought: overbought, Oversold: oversold})
	}
	return strategies
}

// ============================================================================
// 结果分析
// ============================================================================

// AnalyzeResults 分析回测结果
type ResultAnalysis struct {
	TotalStrategies   int
	WinningStrategies int
	BestStrategy      string
	BestParams        map[string]any
	BestReturn        float64
	BestSharpe        float64
	BestWinRate       float64
	AvgReturn         float64
	AvgSharpe         float64
}

// AnalyzeGridResults 分析网格扫描结果
func AnalyzeGridResults(results []*GridSearchResult) *ResultAnalysis {
	analysis := &ResultAnalysis{}
	var allStats []*Stats
	firstSeen := false

	for _, gr := range results {
		if gr == nil || gr.Results == nil {
			continue
		}
		analysis.TotalStrategies += len(gr.Results)
		for _, r := range gr.Results {
			if r != nil && r.Stats != nil {
				allStats = append(allStats, r.Stats)
				if r.Stats.TotalReturn > 0 {
					analysis.WinningStrategies++
				}
				if !firstSeen {
					analysis.BestReturn = r.Stats.TotalReturn
					analysis.BestParams = r.Params
					analysis.BestStrategy = r.StrategyName
					analysis.BestSharpe = r.Stats.SharpeRatio
					analysis.BestWinRate = r.Stats.WinRate
					firstSeen = true
				} else {
					if r.Stats.TotalReturn > analysis.BestReturn {
						analysis.BestReturn = r.Stats.TotalReturn
						analysis.BestParams = r.Params
						analysis.BestStrategy = r.StrategyName
					}
					if r.Stats.SharpeRatio > analysis.BestSharpe {
						analysis.BestSharpe = r.Stats.SharpeRatio
					}
					if r.Stats.WinRate > analysis.BestWinRate {
						analysis.BestWinRate = r.Stats.WinRate
					}
				}
			}
		}
	}

	if len(allStats) > 0 {
		var totalReturn, totalSharpe float64
		for _, s := range allStats {
			totalReturn += s.TotalReturn
			totalSharpe += s.SharpeRatio
		}
		analysis.AvgReturn = totalReturn / float64(len(allStats))
		analysis.AvgSharpe = totalSharpe / float64(len(allStats))
	}

	return analysis
}

// PrintResult 打印回测结果
func PrintResult(r *BacktestResult) string {
	if r == nil || r.Stats == nil {
		return "No results"
	}
	s := r.Stats
	params := r.Params
	var paramsStr []string
	for k, v := range params {
		paramsStr = append(paramsStr, k+"="+formatFloat(v))
	}

	return strings.Join([]string{
		"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━",
		"📊 策略: " + r.StrategyName,
		"⚙️  参数: " + strings.Join(paramsStr, ", "),
		"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━",
		"💰 总收益率: " + formatPct(s.TotalReturn),
		"📅 年化收益率: " + formatPct(s.AnnualReturn),
		"📉 最大回撤: " + formatPct(s.MaxDrawdownPct),
		"⚡ 夏普比率: " + formatFloat(s.SharpeRatio),
		"📐 索提诺比率: " + formatFloat(s.SortinoRatio),
		"🔥 卡玛比率: " + formatFloat(s.CalmarRatio),
		"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━",
		"🎯 胜率: " + formatPct(s.WinRate),
		"💎 盈亏比: " + formatFloat(s.ProfitFactor),
		"📈 交易次数: " + strconv.Itoa(s.TotalTrades),
		"  ✅ 盈利: " + strconv.Itoa(s.WinTrades) + "  ❌ 亏损: " + strconv.Itoa(s.LoseTrades),
		"  📊 平均持仓: " + formatFloat(s.AvgHoldingDays) + " 天",
		"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━",
		"💵 初始资金: " + formatMoney(s.InitCash),
		"💵 最终资金: " + formatMoney(s.FinalValue),
		"⏱️  回测耗时: " + r.Duration.String(),
		"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━",
	}, "\n")
}

func formatFloat(v any) string {
	switch val := v.(type) {
	case float64:
		return sprintf("%.4f", val)
	case int:
		return strconv.Itoa(val)
	default:
		return sprintf("%v", val)
	}
}

func formatPct(v float64) string {
	return sprintf("%.2f", v*100) + "%"
}

func formatMoney(v float64) string {
	return "¥" + sprintf("%.2f", v)
}

func sprintf(format string, args ...any) string {
	// Simple sprintf for float formatting
	if len(args) == 0 {
		return format
	}
	// Very simple implementation
	switch len(args) {
	case 1:
		switch v := args[0].(type) {
		case float64:
			if contains(format, "%.4f") {
				return sprintfFloat(v, 4)
			}
			if contains(format, "%.2f") {
				return sprintfFloat(v, 2)
			}
			return sprintfFloat(v, 4)
		case int:
			return sprintfInt(v)
		}
	}
	return format
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func sprintfFloat(f float64, decimals int) string {
	return strconv.FormatFloat(f, 'f', decimals, 64)
}

func sprintfInt(i int) string {
	if i == 0 {
		return "0"
	}
	negative := i < 0
	if negative {
		i = -i
	}
	var digits []byte
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	if negative {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

// SortResults 按指标排序回测结果
func SortResults(results []*BacktestResult, metric string) {
	sort.Slice(results, func(i, j int) bool {
		a := results[i]
		b := results[j]
		if a == nil || a.Stats == nil {
			return false
		}
		if b == nil || b.Stats == nil {
			return true
		}
		switch metric {
		case "sharpe":
			return a.Stats.SharpeRatio > b.Stats.SharpeRatio
		case "return":
			return a.Stats.TotalReturn > b.Stats.TotalReturn
		case "winrate":
			return a.Stats.WinRate > b.Stats.WinRate
		case "maxdd":
			return a.Stats.MaxDrawdownPct < b.Stats.MaxDrawdownPct
		case "calmar":
			return a.Stats.CalmarRatio > b.Stats.CalmarRatio
		default:
			return a.Stats.SharpeRatio > b.Stats.SharpeRatio
		}
	})
}
