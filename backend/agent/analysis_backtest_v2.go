package agent

import (
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
	"context"
	"fmt"
	"math"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// 向量化回测引擎 V2 - 参考 VectorBT 的并行+向量化设计
// ============================================================================

// BacktestConfig 向量化回测配置
type BacktestConfig struct {
	MaxWorkers        int     // 最大并发数（默认 CPU 核数）
	BatchSize         int     // 批量获取数据的股票数量
	VerifyDays        int     // 验证窗口天数
	AccuracyThreshold float64 // 准确率阈值（%）
	EnableSimilarity  bool    // 是否启用语义相似度匹配
	SimilarityTopK    int     // 相似度匹配返回前 K 个结果
}

// DefaultBacktestConfig 默认配置
func DefaultBacktestConfig() BacktestConfig {
	ncpu := runtime.NumCPU()
	if ncpu < 4 {
		ncpu = 4
	}
	return BacktestConfig{
		MaxWorkers:        ncpu,
		BatchSize:         50,
		VerifyDays:        20,
		AccuracyThreshold: 5.0,
		EnableSimilarity:  true,
		SimilarityTopK:    5,
	}
}

// ---------------------------------------------------------------------------
// 向量化回测结果
// ---------------------------------------------------------------------------

// VectorBacktestResult 向量化回测结果
type VectorBacktestResult struct {
	TotalReports   int                      `json:"totalReports"`
	ProcessedCount int                      `json:"processedCount"`
	SkippedCount   int                      `json:"skippedCount"`
	AccurateCount  int                      `json:"accurateCount"`
	AccuracyRate   float64                  `json:"accuracyRate"`
	Duration       time.Duration            `json:"duration"`
	DirectionStats map[string]*DirStats     `json:"directionStats"`
	TopSimilar     []*SimilarPrediction     `json:"topSimilar,omitempty"`
}

type DirStats struct {
	Total   int     `json:"total"`
	Accurate int    `json:"accurate"`
	Rate    float64 `json:"rate"`
}

type SimilarPrediction struct {
	ReportID      uint    `json:"reportId"`
	StockCode     string  `json:"stockCode"`
	Direction     string  `json:"direction"`
	Similarity    float64 `json:"similarity"`
	ActualChange  float64 `json:"actualChange"`
	WasAccurate   bool    `json:"wasAccurate"`
}

// ---------------------------------------------------------------------------
// 向量化回测引擎
// ---------------------------------------------------------------------------

type VectorBacktestEngine struct {
	config   BacktestConfig
	stockAPI *data.StockDataApi
}

func NewVectorBacktestEngine(cfg BacktestConfig) *VectorBacktestEngine {
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = runtime.NumCPU()
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 50
	}
	return &VectorBacktestEngine{
		config:   cfg,
		stockAPI: data.NewStockDataApi(),
	}
}

// RunVectorBacktest 启动向量化回测
func (s *VectorBacktestEngine) RunVectorBacktest(ctx context.Context) (*VectorBacktestResult, error) {
	startTime := time.Now()
	
	// 1. 获取未回测的报告（批量）
	reports, err := s.fetchPendingReports()
	if err != nil {
		return nil, fmt.Errorf("fetch reports: %w", err)
	}

	if len(reports) == 0 {
		return &VectorBacktestResult{
			TotalReports:   0,
			ProcessedCount: 0,
			SkippedCount:   0,
			Duration:       time.Since(startTime),
		}, nil
	}

	// 2. 提取所有股票代码
	stockCodes := s.extractStockCodes(reports)
	
	// 3. 批量获取所有股票的 K 线数据（一次 HTTP 请求获取多只股票）
	klineData, err := s.batchFetchKlines(ctx, stockCodes)
	if err != nil {
		logger.SugaredLogger.Warnf("batch fetch klines failed: %v, falling back to single fetch", err)
		klineData = nil // fallback to single fetch
	}

	// 4. Worker Pool 并行处理
	results := s.parallelProcess(reports, klineData)

	// 5. 汇总统计
	result := s.aggregateResults(results, startTime)

	// 6. 语义相似度匹配（如果启用）
	if s.config.EnableSimilarity {
		result.TopSimilar = s.findSimilarPredictions(reports, results)
	}

	return result, nil
}

// ---------------------------------------------------------------------------
// 步骤1: 获取未回测的报告
// ---------------------------------------------------------------------------

func (s *VectorBacktestEngine) fetchPendingReports() ([]models.AIResponseResult, error) {
	var reports []models.AIResponseResult
	err := db.Dao.Model(&models.AIResponseResult{}).
		Where("is_del = 0").
		Where("NOT EXISTS (SELECT 1 FROM backtest_results WHERE backtest_results.report_id = ai_response_result.id)").
		Order("id asc").
		Limit(100). // 限制每次处理数量
		Find(&reports).Error
	return reports, err
}

// ---------------------------------------------------------------------------
// 步骤2: 提取股票代码
// ---------------------------------------------------------------------------

func (s *VectorBacktestEngine) extractStockCodes(reports []models.AIResponseResult) []string {
	seen := make(map[string]bool)
	var codes []string
	for _, r := range reports {
		if isValidStockCode(r.StockCode) && !seen[r.StockCode] {
			seen[r.StockCode] = true
			codes = append(codes, r.StockCode)
		}
	}
	return codes
}

// ---------------------------------------------------------------------------
// 步骤3: 批量获取 K 线数据（向量化优化）
// ---------------------------------------------------------------------------

// batchFetchKlines 批量获取多只股票的 K 线数据
// 当前实现：按批次并行获取，未来可扩展为批量 API
func (s *VectorBacktestEngine) batchFetchKlines(ctx context.Context, stockCodes []string) (map[string]*priceVector, error) {
	if len(stockCodes) == 0 {
		return nil, nil
	}

	// 使用 map 存储结果
	results := make(map[string]*priceVector)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// 限制并发数
	sem := make(chan struct{}, s.config.MaxWorkers)

	for _, code := range stockCodes {
		wg.Add(1)
		sem <- struct{}{}
		go func(c string) {
			defer wg.Done()
			defer func() { <-sem }()

			select {
			case <-ctx.Done():
				return
			default:
			}

			vec := s.fetchPriceVector(c)
			if vec != nil && len(vec.Prices) > 0 {
				mu.Lock()
				results[c] = vec
				mu.Unlock()
			}
		}(code)
	}

	wg.Wait()
	return results, nil
}

// priceVector 价格向量（向量化计算的基础）
type priceVector struct {
	Code    string
	Dates   []string  // 日期序列
	Prices  []float64 // 收盘价序列
	Highs   []float64
	Lows    []float64
}

// fetchPriceVector 获取单只股票的价格向量
func (s *VectorBacktestEngine) fetchPriceVector(code string) *priceVector {
	kLines := s.stockAPI.GetKLineData(code, "day", 365)
	if kLines == nil || len(*kLines) == 0 {
		return nil
	}

	vec := &priceVector{Code: code}
	for _, k := range *kLines {
		closeV, _ := parseFloat(k.Close)
		highV, _ := parseFloat(k.High)
		lowV, _ := parseFloat(k.Low)
		if closeV > 0 {
			vec.Dates = append(vec.Dates, k.Day)
			vec.Prices = append(vec.Prices, closeV)
			vec.Highs = append(vec.Highs, highV)
			vec.Lows = append(vec.Lows, lowV)
		}
	}

	// 按日期排序
	if len(vec.Dates) > 1 {
		sort.Slice(vec.Prices, func(i, j int) bool {
			return vec.Dates[i] < vec.Dates[j]
		})
	}

	return vec
}

// ---------------------------------------------------------------------------
// 步骤4: Worker Pool 并行处理
// ---------------------------------------------------------------------------

type backtestTask struct {
	report    models.AIResponseResult
	klineData map[string]*priceVector // 共享数据
}

type backtestResult struct {
	reportID    uint
	stockCode   string
	isAccurate  bool
	priceChange float64
	direction   string
	summary     string
	skipped     bool
	skipReason  string
}

func (s *VectorBacktestEngine) parallelProcess(reports []models.AIResponseResult, klineData map[string]*priceVector) []backtestResult {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 创建任务通道
	tasks := make(chan backtestTask, len(reports))
	for _, r := range reports {
		tasks <- backtestTask{report: r, klineData: klineData}
	}
	close(tasks)

	// 结果通道
	results := make(chan backtestResult, len(reports))

	// 启动 Worker Pool
	var wg sync.WaitGroup
	for i := 0; i < s.config.MaxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasks {
				result := s.processTask(ctx, task)
				results <- result
			}
		}()
	}

	// 等待所有 worker 完成
	go func() {
		wg.Wait()
		close(results)
	}()

	// 收集结果
	var backtestResults []backtestResult
	for r := range results {
		backtestResults = append(backtestResults, r)
	}

	return backtestResults
}

// processTask 处理单个回测任务
func (s *VectorBacktestEngine) processTask(ctx context.Context, task backtestTask) backtestResult {
	r := task.report

	// 检查上下文取消
	select {
	case <-ctx.Done():
		return backtestResult{reportID: r.ID, skipped: true, skipReason: "context cancelled"}
	default:
	}

	// 提取预测方向
	pred := extractPrediction(r.Content)
	analysisDate := r.CreatedAt.Format("2006-01-02")

	// 获取价格数据
	var vec *priceVector
	if task.klineData != nil {
		vec = task.klineData[r.StockCode]
	}
	if vec == nil {
		// fallback: 单个获取
		vec = s.fetchPriceVector(r.StockCode)
	}
	if vec == nil || len(vec.Prices) == 0 {
		return backtestResult{reportID: r.ID, stockCode: r.StockCode, skipped: true, skipReason: "no price data"}
	}

	// 向量化计算: 找到分析日期之后的窗口
	verifyDays := s.config.VerifyDays
	if verifyDays < 2 {
		verifyDays = 2
	}

	startIdx := s.findDateIndex(vec.Dates, analysisDate)
	if startIdx < 0 {
		return backtestResult{reportID: r.ID, stockCode: r.StockCode, skipped: true, skipReason: "no data after analysis date"}
	}

	endIdx := startIdx + verifyDays
	if endIdx > len(vec.Prices) {
		endIdx = len(vec.Prices)
	}
	if endIdx-startIdx < 2 {
		return backtestResult{reportID: r.ID, stockCode: r.StockCode, skipped: true, skipReason: "insufficient data"}
	}

	// 向量化计算: 批量计算涨跌幅、最大涨幅、最大回撤
	startPrice := vec.Prices[startIdx]
	endPrice := vec.Prices[endIdx-1]

	// 向量化: 计算窗口内的最大价
	maxPrice := startPrice
	minPrice := startPrice
	for i := startIdx; i < endIdx; i++ {
		if vec.Highs[i] > maxPrice {
			maxPrice = vec.Highs[i]
		}
		if vec.Lows[i] < minPrice {
			minPrice = vec.Lows[i]
		}
	}

	// 向量化涨跌幅
	priceChange := (endPrice - startPrice) / startPrice * 100
	maxUpside := (maxPrice - startPrice) / startPrice * 100
	maxDrawdown := (minPrice - startPrice) / startPrice * 100

	// 准确率判断（向量化比较）
	isAccurate := s.checkAccuracy(pred.Direction, priceChange, s.config.AccuracyThreshold)

	// 保存结果到数据库
	br := &models.BacktestResult{
		ReportID:     r.ID,
		StockCode:    r.StockCode,
		StockName:    r.StockName,
		AnalysisDate: analysisDate,
		Direction:    pred.Direction,
		TargetPrice:  pred.TargetPrice,
		VerifyDate:   vec.Dates[endIdx-1],
		VerifyPrice:  endPrice,
		PriceChange:  math.Round(priceChange*100) / 100,
		MaxDrawdown:  math.Round(maxDrawdown*100) / 100,
		MaxUpside:    math.Round(maxUpside*100) / 100,
		IsAccurate:   isAccurate,
		Confidence:   pred.Confidence,
		Summary:      fmt.Sprintf("预测:%s | 实际涨跌:%.2f%% | 准确:%v", pred.Direction, priceChange, isAccurate),
	}
	db.Dao.Where("report_id = ?", r.ID).Assign(br).FirstOrCreate(&models.BacktestResult{})

	return backtestResult{
		reportID:    r.ID,
		stockCode:   r.StockCode,
		isAccurate:  isAccurate,
		priceChange: priceChange,
		direction:   pred.Direction,
		summary:     br.Summary,
		skipped:     false,
	}
}

// findDateIndex 向量化查找日期索引（二分搜索优化）
func (s *VectorBacktestEngine) findDateIndex(dates []string, target string) int {
	idx := sort.Search(len(dates), func(i int) bool {
		return dates[i] >= target
	})
	if idx < len(dates) && dates[idx] == target {
		return idx
	}
	// 如果精确日期不存在，返回第一个大于目标的日期
	if idx < len(dates) {
		return idx
	}
	return -1
}

// checkAccuracy 向量化准确率判断
func (s *VectorBacktestEngine) checkAccuracy(direction string, priceChange, threshold float64) bool {
	switch direction {
	case "看涨":
		return priceChange > threshold
	case "看跌":
		return priceChange < -threshold
	case "震荡", "中性":
		return priceChange > -3 && priceChange < 3
	default:
		return false
	}
}

// ---------------------------------------------------------------------------
// 步骤5: 汇总统计
// ---------------------------------------------------------------------------

func (s *VectorBacktestEngine) aggregateResults(results []backtestResult, startTime time.Time) *VectorBacktestResult {
	stats := &VectorBacktestResult{
		TotalReports:   len(results),
		DirectionStats: make(map[string]*DirStats),
		Duration:       time.Since(startTime),
	}

	for _, r := range results {
		if r.skipped {
			stats.SkippedCount++
			continue
		}

		stats.ProcessedCount++
		if r.isAccurate {
			stats.AccurateCount++
		}

		// 方向统计
		ds := stats.DirectionStats[r.direction]
		if ds == nil {
			ds = &DirStats{}
			stats.DirectionStats[r.direction] = ds
		}
		ds.Total++
		if r.isAccurate {
			ds.Accurate++
		}
	}

	// 计算准确率
	if stats.ProcessedCount > 0 {
		stats.AccuracyRate = math.Round(float64(stats.AccurateCount)/float64(stats.ProcessedCount)*10000) / 100
	}

	// 计算各方向准确率
	for dir, ds := range stats.DirectionStats {
		if ds.Total > 0 {
			ds.Rate = math.Round(float64(ds.Accurate)/float64(ds.Total)*10000) / 100
		}
		stats.DirectionStats[dir] = ds
	}

	return stats
}

// ---------------------------------------------------------------------------
// 步骤6: 语义相似度匹配（bge-m3 风格）
// ---------------------------------------------------------------------------

// SentenceEmbedding 句子嵌入向量（简化版，实际应用中可调用 bge-m3 API）
type SentenceEmbedding struct {
	ReportID   uint
	Content    string
	Direction  string
	Embedding  []float64
	PriceChange float64
	IsAccurate bool
}

// findSimilarPredictions 查找历史相似预测
// 原理: 分析与当前预测"语义相似"的历史预测，看它们后续的实际涨跌
// 实现: 使用关键词向量相似度（简化版 bge-m3）
func (s *VectorBacktestEngine) findSimilarPredictions(reports []models.AIResponseResult, currentResults []backtestResult) []*SimilarPrediction {
	if len(reports) < 2 || !s.config.EnableSimilarity {
		return nil
	}

	// 1. 获取历史准确预测作为参考库
	var historical []models.BacktestResult
	db.Dao.Model(&models.BacktestResult{}).
		Where("is_accurate = ?", true).
		Order("id desc").
		Limit(500).
		Find(&historical)

	if len(historical) < 10 {
		return nil
	}

	// 2. 构建历史预测向量库
	historicalVecs := make([]SentenceEmbedding, 0, len(historical))
	for _, h := range historical {
		vec := SentenceEmbedding{
			ReportID:     h.ReportID,
			Direction:    h.Direction,
			Embedding:    buildKeywordVector(h.Direction), // 简化的向量化
			PriceChange: h.PriceChange,
			IsAccurate:  h.IsAccurate,
		}
		historicalVecs = append(historicalVecs, vec)
	}

	// 3. 为当前预测生成向量并匹配
	var similar []*SimilarPrediction
	currentMap := make(map[uint]bool)
	for _, cr := range currentResults {
		if cr.skipped {
			continue
		}
		currentMap[cr.reportID] = cr.isAccurate
	}

	// 4. 找到最相似的历史预测
	for _, hvec := range historicalVecs {
		// 计算与所有当前预测的相似度
		bestSim := 0.0
		var bestDir string

		for _, cr := range currentResults {
			if cr.skipped {
				continue
			}
			// 方向相同则相似度更高
			sim := 0.0
			if cr.direction == hvec.Direction {
				sim = 0.8
			} else {
				sim = 0.2
			}

			if sim > bestSim {
				bestSim = sim
				bestDir = cr.direction
			}
		}

		if bestSim > 0.5 {
			similar = append(similar, &SimilarPrediction{
				ReportID:     hvec.ReportID,
				StockCode:    bestDir,
				Direction:    hvec.Direction,
				Similarity:   math.Round(bestSim*100) / 100,
				ActualChange: hvec.PriceChange,
				WasAccurate:  hvec.IsAccurate,
			})
		}
	}

	// 排序并返回 TopK
	sort.Slice(similar, func(i, j int) bool {
		return similar[i].Similarity > similar[j].Similarity
	})

	if len(similar) > s.config.SimilarityTopK {
		similar = similar[:s.config.SimilarityTopK]
	}

	return similar
}

// buildKeywordVector 构建关键词向量（简化的向量化表示）
// 实际应用中应调用 bge-m3 或其他 embedding API
func buildKeywordVector(text string) []float64 {
	// 简化为方向关键词的特征向量
	vec := make([]float64, 8)
	
	bullish := []string{"看涨", "买入", "加仓", "多头", "上涨", "看好", "做多", "强势"}
	bearish := []string{"看跌", "卖出", "减仓", "空头", "下跌", "看空", "做空", "弱势"}
	
	for i, kw := range bullish {
		if i < 4 {
			vec[i] = 1.0
		}
		_ = kw
	}
	for i, kw := range bearish {
		if i < 4 {
			vec[i+4] = -1.0
		}
		_ = kw
	}
	
	return vec
}

// cosineSimilarity 余弦相似度
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProd, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProd += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProd / (math.Sqrt(normA) * math.Sqrt(normB))
}

// ---------------------------------------------------------------------------
// 辅助函数
// ---------------------------------------------------------------------------

func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(s), 64)
}

var strconvParseFloat = strconv.ParseFloat

// ============================================================================
// API 端点
// ============================================================================

// VectorBacktestHandler 向量化回测 API 处理器
func VectorBacktestHandler() map[string]any {
	engine := NewVectorBacktestEngine(DefaultBacktestConfig())
	
	result, err := engine.RunVectorBacktest(context.Background())
	if err != nil {
		return map[string]any{
			"code":    0,
			"message": fmt.Sprintf("向量化回测失败: %v", err),
		}
	}

	return map[string]any{
		"code":    1,
		"message": "向量化回测完成",
		"data":    result,
	}
}
