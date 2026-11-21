package quant

// ============================================================================
// blades-stock 集成层 - 将 VectorBT 风格回测引擎接入现有系统
// ============================================================================

import (
	"blades-stock/backend/data"
	"blades-stock/backend/models"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ============================================================================
// Local Types - 向前引用兼容
// ============================================================================

// aiReportTask AI报告回测任务
type aiReportTask struct {
	report models.AIResponseResult
}

// aiReportResult AI报告回测结果
type aiReportResult struct {
	reportID    uint
	isAccurate  bool
	priceChange float64
	maxUpside   float64
	maxDrawdown float64
	direction   string
	summary     string
	skipped     bool
	skipReason  string
}

// stockPriceData 股票价格数据
type stockPriceData struct {
	close  []float64
	high   []float64
	low    []float64
	open   []float64
	volume []float64
	dates  []string
}

// AI报告回测统计
type aiBacktestStats struct {
	Accurate       int
	AccuracyRate   float64
	DirectionStats map[string]*DirStats3
}

type DirStats3 struct {
	Total    int
	Accurate int
	Rate     float64
}

// ============================================================================
// StockDataLoader - 数据加载器（适配 blades-stock 的数据源）
// ============================================================================

// StockDataLoader 从 blades-stock 数据源加载 K 线数据
type StockDataLoader struct{}

// NewStockDataLoader 创建数据加载器
func NewStockDataLoader() *StockDataLoader {
	return &StockDataLoader{}
}

// LoadKLineData 加载 K 线数据
func (l *StockDataLoader) LoadKLineData(stockCode string, days int) (
	close, high, low, open, volume []float64,
	dates []string,
	err error,
) {
	var kLines *[]data.KLineData

	cfg := data.GetSettingConfig()
	if cfg != nil && cfg.Settings != nil && cfg.Settings.DefaultDataSource == "adata" && !strings.Contains(cfg.Settings.ADataServiceUrl, "115.190.72.22") {
		adateClient := data.NewAdataClient()
		adataResult, adataErr := adateClient.GetKLineData(stockCode, "", "")
		if adataErr == nil && len(adataResult) > 0 {
			kl, convErr := adataResultToKLineData(adataResult)
			if convErr == nil {
				kLines = &kl
			}
		}
	}

	if kLines == nil {
		result := data.FetchKLineWithFallback(stockCode, "", "101", days, "")
		if result == nil || result.Data == nil || len(*result.Data) == 0 {
			return nil, nil, nil, nil, nil, nil, fmt.Errorf("no kline data for %s", stockCode)
		}
		kLines = result.Data
	}

	n := len(*kLines)
	close = make([]float64, 0, n)
	high = make([]float64, 0, n)
	low = make([]float64, 0, n)
	open = make([]float64, 0, n)
	volume = make([]float64, 0, n)
	dates = make([]string, 0, n)

	type kline struct {
		day    string
		open   float64
		high   float64
		low    float64
		close  float64
		volume float64
	}
	var sorted []kline
	for _, k := range *kLines {
		o, _ := strconv.ParseFloat(strings.TrimSpace(k.Open), 64)
		h, _ := strconv.ParseFloat(strings.TrimSpace(k.High), 64)
		l, _ := strconv.ParseFloat(strings.TrimSpace(k.Low), 64)
		c, _ := strconv.ParseFloat(strings.TrimSpace(k.Close), 64)
		v, _ := strconv.ParseFloat(strings.TrimSpace(k.Volume), 64)
		sorted = append(sorted, kline{day: k.Day, open: o, high: h, low: l, close: c, volume: v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].day < sorted[j].day
	})

	for _, k := range sorted {
		if k.close > 0 && k.volume > 0 {
			open = append(open, k.open)
			high = append(high, k.high)
			low = append(low, k.low)
			close = append(close, k.close)
			volume = append(volume, k.volume)
			dates = append(dates, k.day)
		}
	}

	return close, high, low, open, volume, dates, nil
}

// ============================================================================
// VectorBacktestService - 向量化回测服务
// ============================================================================

// VectorBacktestService 向量化回测服务
type VectorBacktestService struct {
	loader *StockDataLoader
}

// NewVectorBacktestService 创建向量化回测服务
// RunStockBacktest 对单只股票运行回测
// RunStockGridSearch 对单只股票运行参数网格扫描
// ============================================================================
// AIReportBacktest - AI 分析报告回测（VectorBT 风格）
// ============================================================================

// AIReportBacktest AI 分析报告向量化回测
type AIReportBacktest struct {
	loader *StockDataLoader
}

// NewAIReportBacktest 创建 AI 报告回测器
// BacktestAIReports 回测所有 AI 分析报告（VectorBT 风格）
// AIReportBacktestResult AI 报告回测结果
type AIReportBacktestResult struct {
	Total          int
	Processed      int
	Skipped        int
	Accurate       int
	AccuracyRate   float64
	DirectionStats map[string]*DirStats3
	Duration       time.Duration
	ModelName      string
	SkillUsed      string
}

// ============================================================================
// 辅助函数
// ============================================================================

type aiPred struct {
	Direction  string
	Confidence float64
}

// adataResultToKLineData 将 AData 返回数据转换为 KLineData 切片
func adataResultToKLineData(items []map[string]any) ([]data.KLineData, error) {
	var result []data.KLineData
	for _, item := range items {
		day, _ := item["date"].(string)
		open, _ := item["open"].(string)
		high, _ := item["high"].(string)
		low, _ := item["low"].(string)
		close_, _ := item["close"].(string)
		volume, _ := item["volume"].(string)
		if day == "" {
			continue
		}
		kl := data.KLineData{
			Day:    day,
			Open:   open,
			High:   high,
			Low:    low,
			Close:  close_,
			Volume: volume,
		}
		result = append(result, kl)
	}
	return result, nil
}

// SaveBacktestResultToDB 保存回测结果到数据库
// ============================================================================
// API Handlers - Agent 调用的 API
// ============================================================================

// VectorBTResponse 回测结果响应
type VectorBTResponse struct {
	Code         int
	Message      string
	StrategyName string
	Report       string
	Stats        *Stats
}

// VectorBTHandler VectorBT 风格回测 API
func VectorBTHandler(stockCode string, strategyType string, params map[string]any) (*VectorBTResponse, error) {
	loader := NewStockDataLoader()
	close, high, low, open, volume, dates, err := loader.LoadKLineData(stockCode, 365)
	if err != nil {
		return nil, fmt.Errorf("数据加载失败: %v", err)
	}

	var strategy Strategy
	switch strategyType {
	case "MA":
		fast, slow := 5, 20
		if f, ok := params["fast"].(float64); ok {
			fast = int(f)
		}
		if sl, ok := params["slow"].(float64); ok {
			slow = int(sl)
		}
		strategy = &MAStrategy{FastWindow: fast, SlowWindow: slow}
	case "MACD":
		fast, slow, sig := 12, 26, 9
		if f, ok := params["fast"].(float64); ok {
			fast = int(f)
		}
		if sl, ok := params["slow"].(float64); ok {
			slow = int(sl)
		}
		if sig_, ok := params["signal"].(float64); ok {
			sig = int(sig_)
		}
		strategy = &MACDStrategy{Fast: fast, Slow: slow, Signal: sig}
	case "RSI":
		window := 14
		if w, ok := params["window"].(float64); ok {
			window = int(w)
		}
		strategy = &RSIStrategy{Window: window, Overbought: 70, Oversold: 30}
	case "BOLL":
		window := 20
		numStd := 2.0
		if w, ok := params["window"].(float64); ok {
			window = int(w)
		}
		if ns, ok := params["num_std"].(float64); ok {
			numStd = ns
		}
		strategy = &BollingerStrategy{Window: window, NumStd: numStd}
	case "DualThrust", "DUAL_THRUST":
		k := 0.5
		period := 5
		if kVal, ok := params["k"].(float64); ok {
			k = kVal
		}
		if p, ok := params["period"].(float64); ok {
			period = int(p)
		}
		strategy = &DualThrustStrategy{K: k, Period: period}
	default:
		strategy = &MAStrategy{FastWindow: 5, SlowWindow: 20}
	}

	cfg := DefaultBacktestConfig()
	engine := NewBacktestEngine().
		Data(close, high, low, open, volume, dates).
		Config(cfg).
		Build()

	backtestResult := engine.Run(strategy)
	return &VectorBTResponse{
		Code:         1,
		Message:      "回测完成",
		StrategyName: backtestResult.StrategyName,
		Report:       PrintResult(backtestResult),
		Stats:        backtestResult.Stats,
	}, nil
}

// BatchStockBacktestItem 单标的回测指标汇总
type BatchStockBacktestItem struct {
	StockCode      string    `json:"stock_code"`
	TotalReturnPct float64   `json:"total_return_pct"`
	AnnualReturn   float64   `json:"annual_return"`
	SharpeRatio    float64   `json:"sharpe_ratio"`
	MaxDrawdownPct float64   `json:"max_drawdown_pct"`
	WinRatePct     float64   `json:"win_rate_pct"`
	ProfitFactor   float64   `json:"profit_factor"`
	TotalTrades    int       `json:"total_trades"`
	Dates          []string  `json:"dates,omitempty"`
	EquityCurve    []float64 `json:"equity_curve,omitempty"`
}

// VectorBTMultiResponse 多标的/板块回测综合响应
type VectorBTMultiResponse struct {
	StrategyName       string                   `json:"strategy_name"`
	Params             map[string]any           `json:"params"`
	TotalStocks        int                      `json:"total_stocks"`
	TestedStocks       int                      `json:"tested_stocks"`
	AverageReturnPct   float64                  `json:"avg_return_pct"`
	AverageWinRatePct  float64                  `json:"avg_win_rate_pct"`
	AverageSharpe      float64                  `json:"avg_sharpe"`
	AverageMaxDrawdown float64                  `json:"avg_max_drawdown_pct"`
	ProfitableRatio    float64                  `json:"profitable_ratio"` // 盈利标的比例
	BestStock          BatchStockBacktestItem   `json:"best_stock"`
	WorstStock         BatchStockBacktestItem   `json:"worst_stock"`
	Items              []BatchStockBacktestItem `json:"items"`
	SummaryReport      string                   `json:"summary_report"`
}

// RunVectorizedBacktestMulti 多标的批量向量化回测
func RunVectorizedBacktestMulti(stockCodes []string, strategyType string, params map[string]any, days int) (*VectorBTMultiResponse, error) {
	if len(stockCodes) == 0 {
		return nil, fmt.Errorf("股票代码列表不能为空")
	}
	if days <= 0 {
		days = 365
	}
	if strategyType == "" {
		strategyType = "MA"
	}

	var items []BatchStockBacktestItem
	loader := NewStockDataLoader()

	var totalReturn, totalWinRate, totalSharpe, totalDrawdown float64
	profitableCount := 0

	for _, code := range stockCodes {
		close, high, low, open, volume, dates, err := loader.LoadKLineData(code, days)
		if err != nil || len(close) < 20 {
			continue
		}

		var strategy Strategy
		switch strategyType {
		case "MA":
			fast, slow := 5, 20
			if f, ok := params["fast"].(float64); ok {
				fast = int(f)
			}
			if sl, ok := params["slow"].(float64); ok {
				slow = int(sl)
			}
			strategy = &MAStrategy{FastWindow: fast, SlowWindow: slow}
		case "MACD":
			fast, slow, sig := 12, 26, 9
			if f, ok := params["fast"].(float64); ok {
				fast = int(f)
			}
			if sl, ok := params["slow"].(float64); ok {
				slow = int(sl)
			}
			if sig_, ok := params["signal"].(float64); ok {
				sig = int(sig_)
			}
			strategy = &MACDStrategy{Fast: fast, Slow: slow, Signal: sig}
		case "RSI":
			window := 14
			if w, ok := params["window"].(float64); ok {
				window = int(w)
			}
			strategy = &RSIStrategy{Window: window, Overbought: 70, Oversold: 30}
		case "BOLL":
			window := 20
			numStd := 2.0
			if w, ok := params["window"].(float64); ok {
				window = int(w)
			}
			if ns, ok := params["num_std"].(float64); ok {
				numStd = ns
			}
			strategy = &BollingerStrategy{Window: window, NumStd: numStd}
		default:
			strategy = &MAStrategy{FastWindow: 5, SlowWindow: 20}
		}

		cfg := DefaultBacktestConfig()
		engine := NewBacktestEngine().
			Data(close, high, low, open, volume, dates).
			Config(cfg).
			Build()

		res := engine.Run(strategy)
		if res == nil || res.Stats == nil {
			continue
		}

		item := BatchStockBacktestItem{
			StockCode:      code,
			TotalReturnPct: res.Stats.TotalReturn * 100,
			AnnualReturn:   res.Stats.AnnualReturn * 100,
			SharpeRatio:    res.Stats.SharpeRatio,
			MaxDrawdownPct: res.Stats.MaxDrawdownPct,
			WinRatePct:     res.Stats.WinRate * 100,
			ProfitFactor:   res.Stats.ProfitFactor,
			TotalTrades:    res.Stats.TotalTrades,
		}

		// 仅对单标的或少量标的保留完整净值曲线，避免多标的大报文溢出
		if len(stockCodes) <= 3 {
			item.Dates = res.Dates
			item.EquityCurve = res.Equity
		}

		items = append(items, item)
		totalReturn += item.TotalReturnPct
		totalWinRate += item.WinRatePct
		totalSharpe += item.SharpeRatio
		totalDrawdown += item.MaxDrawdownPct
		if item.TotalReturnPct > 0 {
			profitableCount++
		}
	}

	testedCount := len(items)
	if testedCount == 0 {
		return nil, fmt.Errorf("未能成功获取任何标的的历史数据进行回测")
	}

	avgReturn := totalReturn / float64(testedCount)
	avgWinRate := totalWinRate / float64(testedCount)
	avgSharpe := totalSharpe / float64(testedCount)
	avgDD := totalDrawdown / float64(testedCount)
	profRatio := (float64(profitableCount) / float64(testedCount)) * 100

	// 找出最优和最差标的
	best := items[0]
	worst := items[0]
	for _, it := range items[1:] {
		if it.TotalReturnPct > best.TotalReturnPct {
			best = it
		}
		if it.TotalReturnPct < worst.TotalReturnPct {
			worst = it
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### 📊 VectorBT 向量化回测综合诊断报告\n\n"))
	sb.WriteString(fmt.Sprintf("- **策略名称**: %s\n", strategyType))
	sb.WriteString(fmt.Sprintf("- **参数设置**: %v\n", params))
	sb.WriteString(fmt.Sprintf("- **回测区间**: 最近 %d 天\n", days))
	sb.WriteString(fmt.Sprintf("- **有效回测标的数**: %d / %d\n", testedCount, len(stockCodes)))
	sb.WriteString(fmt.Sprintf("- **平均收益率**: **%.2f%%**\n", avgReturn))
	sb.WriteString(fmt.Sprintf("- **平均胜率**: **%.2f%%** (盈利标的占比: %.1f%%)\n", avgWinRate, profRatio))
	sb.WriteString(fmt.Sprintf("- **平均夏普比率**: **%.2f** | **平均最大回撤**: **%.2f%%**\n\n", avgSharpe, avgDD))
	sb.WriteString(fmt.Sprintf("- 🏆 **最佳标的**: `%s` (收益: **+%.2f%%**, 胜率: %.1f%%, 交易 %d 次)\n",
		best.StockCode, best.TotalReturnPct, best.WinRatePct, best.TotalTrades))
	sb.WriteString(fmt.Sprintf("- ⚠️ **最差标的**: `%s` (收益: **%.2f%%**, 回撤: %.2f%%)\n\n",
		worst.StockCode, worst.TotalReturnPct, worst.MaxDrawdownPct))

	if testedCount > 1 {
		sb.WriteString("| 标的代码 | 总收益率 | 胜率 | 夏普比率 | 最大回撤 | 交易次数 |\n")
		sb.WriteString("| :--- | :--- | :--- | :--- | :--- | :--- |\n")
		showCount := testedCount
		if showCount > 10 {
			showCount = 10
		}
		for i := 0; i < showCount; i++ {
			it := items[i]
			sb.WriteString(fmt.Sprintf("| %s | %.2f%% | %.1f%% | %.2f | %.2f%% | %d |\n",
				it.StockCode, it.TotalReturnPct, it.WinRatePct, it.SharpeRatio, it.MaxDrawdownPct, it.TotalTrades))
		}
		if testedCount > 10 {
			sb.WriteString(fmt.Sprintf("| ... | 更多 %d 只标的已计入统计 | ... | ... | ... | ... |\n", testedCount-10))
		}
	}

	return &VectorBTMultiResponse{
		StrategyName:       strategyType,
		Params:             params,
		TotalStocks:        len(stockCodes),
		TestedStocks:       testedCount,
		AverageReturnPct:   avgReturn,
		AverageWinRatePct:  avgWinRate,
		AverageSharpe:      avgSharpe,
		AverageMaxDrawdown: avgDD,
		ProfitableRatio:    profRatio,
		BestStock:          best,
		WorstStock:         worst,
		Items:              items,
		SummaryReport:      sb.String(),
	}, nil
}

// VectorBTGridSearchResponse 参数扫描结果响应
type VectorBTGridSearchResponse struct {
	Code               int            `json:"code"`
	Message            string         `json:"message"`
	TotalCombinations  int            `json:"total_combinations"`
	BestStrategy       string         `json:"best_strategy"`
	BestParams         map[string]any `json:"best_params"`
	BestStats          *Stats         `json:"best_stats"`
	Duration           string         `json:"duration"`
	InSampleReturn     float64        `json:"in_sample_return"`
	OutSampleReturn    float64        `json:"out_sample_return"`
	DegradationRatio   float64        `json:"degradation_ratio"`
	OverfittingRisk    string         `json:"overfitting_risk"`
	TrainTestSplitDesc string         `json:"train_test_split_desc"`
}

// VectorBTGridSearchHandler 参数网格扫描 API
func VectorBTGridSearchHandler(stockCode string, strategyType string) (*VectorBTGridSearchResponse, error) {
	loader := NewStockDataLoader()
	close, high, low, open, volume, dates, err := loader.LoadKLineData(stockCode, 365)
	if err != nil {
		return nil, fmt.Errorf("数据加载失败: %v", err)
	}

	var strategies []Strategy
	switch strategyType {
	case "MA":
		fastW := []int{3, 5, 7, 10, 15, 20}
		slowW := []int{30, 60, 120, 250}
		strategies = GenerateMAStrategies(fastW, slowW)
	case "MACD":
		fastR := []int{5, 8, 12, 16}
		slowR := []int{18, 24, 30, 50}
		sigR := []int{5, 9, 15}
		strategies = GenerateMACDStrategies(fastR, slowR, sigR)
	case "RSI":
		windows := []int{7, 14, 21, 28}
		strategies = GenerateRSIStrategies(windows, 70, 30)
	case "BOLL":
		for _, w := range []int{10, 20, 30} {
			for _, ns := range []float64{1.5, 2.0, 2.5} {
				strategies = append(strategies, &BollingerStrategy{Window: w, NumStd: ns})
			}
		}
	case "DualThrust", "DUAL_THRUST":
		for _, p := range []int{3, 5, 10} {
			for _, k := range []float64{0.3, 0.5, 0.7} {
				strategies = append(strategies, &DualThrustStrategy{Period: p, K: k})
			}
		}
	default:
		fastW := []int{5, 10, 20}
		slowW := []int{30, 60}
		strategies = GenerateMAStrategies(fastW, slowW)
	}

	cfg := DefaultBacktestConfig()
	totalLen := len(close)
	splitIdx := int(float64(totalLen) * 0.70)
	if splitIdx < 30 || totalLen-splitIdx < 15 {
		// 数据量过少无法有效切分，全量运行
		engine := NewBacktestEngine().
			Data(close, high, low, open, volume, dates).
			Config(cfg).
			Build()
		result := engine.RunGridSearch(strategies)
		return &VectorBTGridSearchResponse{
			Code:               1,
			Message:            fmt.Sprintf("扫描完成，共 %d 种参数组合，耗时 %v", result.TotalCombinations, result.Duration),
			TotalCombinations:  result.TotalCombinations,
			BestStrategy:       result.BestStrategyName,
			BestParams:         result.BestParams,
			BestStats:          result.BestStats,
			Duration:           result.Duration.String(),
			OverfittingRisk:    "样本量较少（全量回测）",
			TrainTestSplitDesc: "未切分",
		}, nil
	}

	// 1. 样本内数据 (In-Sample: 前 70%)
	isClose := close[:splitIdx]
	isHigh := high[:splitIdx]
	isLow := low[:splitIdx]
	isOpen := open[:splitIdx]
	isVolume := volume[:splitIdx]
	isDates := dates[:splitIdx]

	// 2. 样本外数据 (Out-of-Sample: 后 30%)
	oosClose := close[splitIdx:]
	oosHigh := high[splitIdx:]
	oosLow := low[splitIdx:]
	oosOpen := open[splitIdx:]
	oosVolume := volume[splitIdx:]
	oosDates := dates[splitIdx:]

	// 仅在样本内执行网格扫描选出最优参数，杜绝窥探未来数据
	isEngine := NewBacktestEngine().
		Data(isClose, isHigh, isLow, isOpen, isVolume, isDates).
		Config(cfg).
		Build()
	isResult := isEngine.RunGridSearch(strategies)

	// 3. 将样本内最优策略拿到样本外盲测检验泛化能力
	bestIdx := -1
	var bestResult *BacktestResult
	for idx, r := range isResult.Results {
		if r != nil && r.Stats != nil {
			if bestResult == nil || bestResult.Stats == nil || r.Stats.SharpeRatio > bestResult.Stats.SharpeRatio {
				bestResult = r
				bestIdx = idx
			}
		}
	}

	isReturn := 0.0
	oosReturn := 0.0
	if isResult.BestStats != nil {
		isReturn = isResult.BestStats.TotalReturn
	}

	if bestIdx >= 0 && bestIdx < len(strategies) {
		oosEngine := NewBacktestEngine().
			Data(oosClose, oosHigh, oosLow, oosOpen, oosVolume, oosDates).
			Config(cfg).
			Build()
		oosResult := oosEngine.Run(strategies[bestIdx])
		if oosResult != nil && oosResult.Stats != nil {
			oosReturn = oosResult.Stats.TotalReturn
		}
	}

	degradation := 0.0
	if isReturn != 0 {
		degradation = (isReturn - oosReturn) / math.Abs(isReturn) * 100
	}

	overfittingRisk := "稳健泛化 (样本外留存率良好)"
	if oosReturn < 0 && isReturn > 0 {
		overfittingRisk = "高过拟合风险 (样本外收益转负，警惕参数过拟合)"
	} else if degradation > 50 {
		overfittingRisk = "中度衰减 (样本外收益衰减超过 50%)"
	}

	splitDesc := fmt.Sprintf("训练集 %d 天 (%s ~ %s) vs 测试集 %d 天 (%s ~ %s)",
		splitIdx, isDates[0], isDates[len(isDates)-1],
		totalLen-splitIdx, oosDates[0], oosDates[len(oosDates)-1])

	return &VectorBTGridSearchResponse{
		Code:               1,
		Message:            fmt.Sprintf("扫描完成，共 %d 种参数组合，样本内外分离校验完成 (耗时 %v)", isResult.TotalCombinations, isResult.Duration),
		TotalCombinations:  isResult.TotalCombinations,
		BestStrategy:       isResult.BestStrategyName,
		BestParams:         isResult.BestParams,
		BestStats:          isResult.BestStats,
		Duration:           isResult.Duration.String(),
		InSampleReturn:     isReturn,
		OutSampleReturn:    oosReturn,
		DegradationRatio:   degradation,
		OverfittingRisk:    overfittingRisk,
		TrainTestSplitDesc: splitDesc,
	}, nil
}
