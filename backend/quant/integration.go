package quant

// ============================================================================
// blades-stock 集成层 - 将 VectorBT 风格回测引擎接入现有系统
// ============================================================================

import (
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
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
	reportID     uint
	isAccurate   bool
	priceChange  float64
	maxUpside    float64
	maxDrawdown  float64
	direction    string
	summary      string
	skipped      bool
	skipReason   string
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
	Total   int
	Accurate int
	Rate    float64
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
	if cfg.Settings.DefaultDataSource == "adata" {
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
func NewVectorBacktestService() *VectorBacktestService {
	return &VectorBacktestService{
		loader: NewStockDataLoader(),
	}
}

// RunStockBacktest 对单只股票运行回测
func (s *VectorBacktestService) RunStockBacktest(
	stockCode string,
	strategy Strategy,
	days int,
	initCash float64,
) (*BacktestResult, error) {
	close, high, low, open, volume, dates, err := s.loader.LoadKLineData(stockCode, days)
	if err != nil {
		return nil, err
	}

	cfg := DefaultBacktestConfig()
	cfg.InitCash = initCash
	cfg.Name = strategy.Name()

	engine := NewBacktestEngine().
		Data(close, high, low, open, volume, dates).
		Config(cfg).
		Build()

	return engine.Run(strategy), nil
}

// RunStockGridSearch 对单只股票运行参数网格扫描
func (s *VectorBacktestService) RunStockGridSearch(
	stockCode string,
	strategies []Strategy,
	days int,
	initCash float64,
) (*GridSearchResult, error) {
	close, high, low, open, volume, dates, err := s.loader.LoadKLineData(stockCode, days)
	if err != nil {
		return nil, err
	}

	cfg := DefaultBacktestConfig()
	cfg.InitCash = initCash
	cfg.Name = "grid_search"

	engine := NewBacktestEngine().
		Data(close, high, low, open, volume, dates).
		Config(cfg).
		Build()

	return engine.RunGridSearch(strategies), nil
}

// ============================================================================
// AIReportBacktest - AI 分析报告回测（VectorBT 风格）
// ============================================================================

// AIReportBacktest AI 分析报告向量化回测
type AIReportBacktest struct {
	loader *StockDataLoader
}

// NewAIReportBacktest 创建 AI 报告回测器
func NewAIReportBacktest() *AIReportBacktest {
	return &AIReportBacktest{
		loader: NewStockDataLoader(),
	}
}

// BacktestAIReports 回测所有 AI 分析报告（VectorBT 风格）
func (s *AIReportBacktest) BacktestAIReports(ctx context.Context, limit int) (*AIReportBacktestResult, error) {
	start := time.Now()

	var reports []models.AIResponseResult
	err := db.Dao.Model(&models.AIResponseResult{}).
		Where("is_del = 0").
		Where("NOT EXISTS (SELECT 1 FROM backtest_results WHERE backtest_results.report_id = ai_response_result.id)").
		Order("id asc").
		Limit(limit).
		Find(&reports).Error
	if err != nil {
		return nil, err
	}

	if len(reports) == 0 {
		return &AIReportBacktestResult{
			Total:   0,
			Duration: time.Since(start),
		}, nil
	}

	// 按股票代码分组加载数据（批量优化）
	stockCodes := make(map[string]bool)
	for _, r := range reports {
		stockCodes[r.StockCode] = true
	}

	stockData := make(map[string]*stockPriceData)
	for code := range stockCodes {
		c, h, l, o, v, d, loadErr := s.loader.LoadKLineData(code, 365)
		if loadErr != nil {
			logger.SugaredLogger.Warnf("load kline for %s failed: %v", code, loadErr)
			continue
		}
		stockData[code] = &stockPriceData{close: c, high: h, low: l, open: o, volume: v, dates: d}
	}

	// Worker Pool 并行回测
	tasks := make(chan aiReportTask, len(reports))
	for _, r := range reports {
		tasks <- aiReportTask{report: r}
	}
	close(tasks)

	results := make(chan aiReportResult, len(reports))
	workers := 8

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range tasks {
				select {
				case <-ctx.Done():
					return
				default:
				}
				r := s.backtestSingleReport(t.report, stockData)
				results <- r
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var backtestResults []aiReportResult
	for r := range results {
		backtestResults = append(backtestResults, r)
	}

	// 保存结果
	for _, br := range backtestResults {
		if br.skipped {
			continue
		}
		for _, rep := range reports {
			if rep.ID == br.reportID {
				SaveBacktestResultToDB(rep, br)
				break
			}
		}
	}

	stats := s.computeStats(backtestResults)
	return &AIReportBacktestResult{
		Total:          len(reports),
		Processed:      len(reports) - countSkipped2(backtestResults),
		Skipped:        countSkipped2(backtestResults),
		Accurate:       stats.Accurate,
		AccuracyRate:   stats.AccuracyRate,
		DirectionStats: stats.DirectionStats,
		Duration:       time.Since(start),
	}, nil
}

func (s *AIReportBacktest) backtestSingleReport(r models.AIResponseResult, stockData map[string]*stockPriceData) aiReportResult {
	pred := extractAIPrediction(r.Content)
	analysisDate := r.CreatedAt.Format("2006-01-02")

	pd, ok := stockData[r.StockCode]
	if !ok || pd == nil || len(pd.close) == 0 {
		return aiReportResult{reportID: r.ID, skipped: true, skipReason: "no data"}
	}

	idx := sort.Search(len(pd.dates), func(i int) bool {
		return pd.dates[i] >= analysisDate
	})
	if idx >= len(pd.close) {
		return aiReportResult{reportID: r.ID, skipped: true, skipReason: "no data after date"}
	}

	verifyDays := 20
	endIdx := idx + verifyDays
	if endIdx > len(pd.close) {
		endIdx = len(pd.close)
	}
	if endIdx-idx < 2 {
		return aiReportResult{reportID: r.ID, skipped: true, skipReason: "insufficient data"}
	}

	startPrice := pd.close[idx]
	endPrice := pd.close[endIdx-1]
	priceChange := (endPrice - startPrice) / startPrice * 100

	isAccurate := checkPredictionAccuracy(pred.Direction, priceChange, 5.0)
	return aiReportResult{
		reportID:    r.ID,
		isAccurate: isAccurate,
		priceChange: mathRound2(priceChange*100) / 100,
		direction:  pred.Direction,
		summary:    fmt.Sprintf("预测:%s | 实际涨跌:%.2f%% | 准确:%v", pred.Direction, priceChange, isAccurate),
		skipped:    false,
	}
}

func (s *AIReportBacktest) computeStats(results []aiReportResult) *aiBacktestStats {
	stats := &aiBacktestStats{
		DirectionStats: make(map[string]*DirStats3),
	}
	for _, r := range results {
		if r.skipped {
			continue
		}
		if r.isAccurate {
			stats.Accurate++
		}
		ds := stats.DirectionStats[r.direction]
		if ds == nil {
			ds = &DirStats3{}
			stats.DirectionStats[r.direction] = ds
		}
		ds.Total++
		if r.isAccurate {
			ds.Accurate++
		}
	}
	processed := countNotSkipped2(results)
	if processed > 0 {
		stats.AccuracyRate = float64(stats.Accurate) / float64(processed)
	}
	return stats
}

// AIReportBacktestResult AI 报告回测结果
type AIReportBacktestResult struct {
	Total          int
	Processed      int
	Skipped        int
	Accurate       int
	AccuracyRate   float64
	DirectionStats map[string]*DirStats3
	Duration       time.Duration
}

// ============================================================================
// 辅助函数
// ============================================================================

type aiPred struct {
	Direction  string
	Confidence float64
}

func extractAIPrediction(content string) aiPred {
	p := aiPred{Direction: "中性", Confidence: 0.5}
	bullish := countKw(content, []string{"看涨", "买入", "加仓", "多头", "上涨", "看好", "强势", "突破", "做多"})
	bearish := countKw(content, []string{"看跌", "卖出", "减仓", "空头", "下跌", "看空", "弱势", "破位", "做空"})
	neutral := countKw(content, []string{"震荡", "盘整", "中性", "观望", "横盘"})

	if bullish > bearish && bullish > neutral {
		p.Direction = "看涨"
		p.Confidence = 0.5 + float64(bullish-bearish)*0.08
	} else if bearish > bullish && bearish > neutral {
		p.Direction = "看跌"
		p.Confidence = 0.5 + float64(bearish-bullish)*0.08
	} else if neutral >= bullish && neutral >= bearish {
		p.Direction = "震荡"
		p.Confidence = 0.5 + float64(neutral)*0.05
	}
	if p.Confidence > 1 {
		p.Confidence = 1
	}
	return p
}

func countKw(text string, keywords []string) int {
	n := 0
	for _, kw := range keywords {
		n += strings.Count(text, kw)
	}
	return n
}

func checkPredictionAccuracy(direction string, priceChange, threshold float64) bool {
	switch direction {
	case "看涨":
		return priceChange > threshold
	case "看跌":
		return priceChange < -threshold
	case "震荡":
		return priceChange > -3 && priceChange < 3
	default:
		return false
	}
}

func mathRound2(x float64) float64 {
	return float64(int(x + 0.5))
}

func countSkipped2(results []aiReportResult) int {
	n := 0
	for _, r := range results {
		if r.skipped {
			n++
		}
	}
	return n
}

func countNotSkipped2(results []aiReportResult) int {
	n := 0
	for _, r := range results {
		if !r.skipped {
			n++
		}
	}
	return n
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
func SaveBacktestResultToDB(r models.AIResponseResult, br aiReportResult) {
	pred := extractAIPrediction(r.Content)
	analysisDate := r.CreatedAt.Format("2006-01-02")

	result := &models.BacktestResult{
		ReportID:     r.ID,
		StockCode:    r.StockCode,
		StockName:    r.StockName,
		AnalysisDate: analysisDate,
		Direction:    pred.Direction,
		Confidence:   pred.Confidence,
		IsAccurate:   br.isAccurate,
		PriceChange:  br.priceChange,
		MaxUpside:    br.maxUpside,
		MaxDrawdown:  br.maxDrawdown,
		Summary:      br.summary,
	}
	db.Dao.Where("report_id = ?", r.ID).Assign(result).FirstOrCreate(&models.BacktestResult{})
}

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

// VectorBTGridSearchResponse 参数扫描结果响应
type VectorBTGridSearchResponse struct {
	Code            int
	Message         string
	TotalCombinations int
	BestStrategy    string
	BestParams      map[string]any
	BestStats       *Stats
	Duration        string
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
	default:
		fastW := []int{5, 10, 20}
		slowW := []int{30, 60}
		strategies = GenerateMAStrategies(fastW, slowW)
	}

	cfg := DefaultBacktestConfig()
	engine := NewBacktestEngine().
		Data(close, high, low, open, volume, dates).
		Config(cfg).
		Build()

	result := engine.RunGridSearch(strategies)

	return &VectorBTGridSearchResponse{
		Code:              1,
		Message:           fmt.Sprintf("扫描完成，共 %d 种参数组合，耗时 %v", result.TotalCombinations, result.Duration),
		TotalCombinations: result.TotalCombinations,
		BestStrategy:      result.BestStrategyName,
		BestParams:        result.BestParams,
		BestStats:         result.BestStats,
		Duration:          result.Duration.String(),
	}, nil
}
