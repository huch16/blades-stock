package quant

// ============================================================================
// BacktestManager - 统一回测管理器
// 取代 analysis_backtest.go (V1) 和 analysis_backtest_v2.go (V2)
// 职责：AI报告回测 + 策略回测 + 统计查询
// ============================================================================

import (
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// StockDataLoaderInterface defines the interface for loading stock data
// This allows for mocking in tests
type StockDataLoaderInterface interface {
	LoadKLineData(stockCode string, days int) (
		close, high, low, open, volume []float64,
		dates []string,
		err error,
	)
}

// BacktestManager 统一回测管理器
type BacktestManager struct {
	loader StockDataLoaderInterface
}

// NewBacktestManager 创建回测管理器
func NewBacktestManager() *BacktestManager {
	return &BacktestManager{
		loader: NewStockDataLoader(),
	}
}

// ============================================================================
// AI 报告回测
// ============================================================================

// RunAIReportBacktest 对所有未回测的 AI 报告执行回测
func (m *BacktestManager) RunAIReportBacktest(ctx context.Context, limit int) (*AIReportBacktestResult, error) {
	start := time.Now()

	reports, err := m.fetchPendingReports(limit)
	if err != nil {
		return nil, fmt.Errorf("fetch pending reports: %w", err)
	}
	if len(reports) == 0 {
		return &AIReportBacktestResult{Duration: time.Since(start)}, nil
	}

	// 按股票代码批量加载数据
	stockData := m.batchLoadStockData(reports)

	// 并行回测
	results := m.parallelBacktestReports(ctx, reports, stockData)

	// 保存结果
	saved := 0
	for _, br := range results {
		for _, rep := range reports {
			if rep.ID == br.reportID {
				m.saveReportBacktest(rep, br)
				if !br.skipped {
					saved++
				}
				break
			}
		}
	}

	stats := m.computeReportStats(results)
	logger.SugaredLogger.Infof("AI report backtest: total=%d, processed=%d, saved=%d, accurate=%d (%.1f%%), duration=%v",
		len(reports), len(reports)-countSkippedResults(results), saved, stats.Accurate, stats.AccuracyRate*100, time.Since(start))

	// Collect unique model names from reports
	modelNames := make(map[string]bool)
	for _, r := range reports {
		if r.ModelName != "" {
			modelNames[r.ModelName] = true
		}
	}
	modelList := make([]string, 0, len(modelNames))
	for m := range modelNames {
		modelList = append(modelList, m)
	}
	// SkillUsed is not directly available; leave empty for now
	skillUsed := ""

	return &AIReportBacktestResult{
		Total:          len(reports),
		Processed:      len(reports) - countSkippedResults(results),
		Skipped:        countSkippedResults(results),
		Accurate:       stats.Accurate,
		AccuracyRate:   stats.AccuracyRate,
		DirectionStats: stats.DirectionStats,
		Duration:       time.Since(start),
		ModelName:      strings.Join(modelList, ", "),
		SkillUsed:      skillUsed,
	}, nil
}

// RunSingleReportBacktest 对单个 AI 报告回测
func (m *BacktestManager) RunSingleReportBacktest(r models.AIResponseResult) (*models.BacktestResult, error) {
	pureCode := models.CleanStockCode(r.StockCode)
	if pureCode == "" {
		pureCode = r.StockCode
	}
	if !models.IsStockReportCode(pureCode) {
		return nil, fmt.Errorf("非个股报告，不可回测: id=%d code=%q", r.ID, r.StockCode)
	}
	pred := extractPredictionFromReport(r.Content)
	analysisDate := r.CreatedAt.Format("2006-01-02")

	close, high, low, _, _, dates, err := m.loader.LoadKLineData(pureCode, 365)
	if err != nil || len(close) == 0 {
		return nil, fmt.Errorf("no price data for %s: %w", pureCode, err)
	}

	idx := sort.Search(len(dates), func(i int) bool {
		return dates[i] >= analysisDate
	})
	if idx >= len(close) {
		return nil, fmt.Errorf("no price data after %s for %s", analysisDate, r.StockCode)
	}

	// 读取配置
	var settings data.Settings
	db.Dao.First(&settings)
	verifyDays := int(settings.BacktestVerifyDays)
	if verifyDays < 2 {
		verifyDays = 20
	}
	threshold := settings.BacktestAccuracyThreshold
	if threshold == 0 {
		threshold = 5.0
	}

	endIdx := idx + verifyDays
	if endIdx > len(close) {
		endIdx = len(close)
	}
	if endIdx-idx < 2 {
		return nil, fmt.Errorf("insufficient price data after %s", analysisDate)
	}

	startPrice := close[idx]
	endPrice := close[endIdx-1]
	priceChange := (endPrice - startPrice) / startPrice * 100

	maxPrice := startPrice
	minPrice := startPrice
	for i := idx; i < endIdx; i++ {
		if high[i] > maxPrice {
			maxPrice = high[i]
		}
		if low[i] < minPrice {
			minPrice = low[i]
		}
	}
	maxUpside := (maxPrice - startPrice) / startPrice * 100
	maxDrawdown := (minPrice - startPrice) / startPrice * 100

	isAccurate := checkDirectionAccuracy(pred.Direction, priceChange, threshold)

	result := &models.BacktestResult{
		ReportID:     r.ID,
		StockCode:    r.StockCode,
		StockName:    r.StockName,
		AnalysisDate: analysisDate,
		Direction:    pred.Direction,
		TargetPrice:  pred.TargetPrice,
		VerifyDate:   dates[endIdx-1],
		VerifyPrice:  endPrice,
		PriceChange:  math.Round(priceChange*100) / 100,
		MaxDrawdown:  math.Round(maxDrawdown*100) / 100,
		MaxUpside:    math.Round(maxUpside*100) / 100,
		IsAccurate:   isAccurate,
		Confidence:   pred.Confidence,
		Summary:      fmt.Sprintf("预测:%s | 实际涨跌:%.2f%% | 准确:%v", pred.Direction, priceChange, isAccurate),
	}

	if err := db.Dao.Where("report_id = ?", r.ID).Assign(result).FirstOrCreate(&models.BacktestResult{}).Error; err != nil {
		return nil, fmt.Errorf("save backtest result: %w", err)
	}
	return result, nil
}

// ============================================================================
// 策略回测（委托给 VectorBacktestService）
// ============================================================================

// RunStrategyBacktest 对单只股票运行策略回测
func (m *BacktestManager) RunStrategyBacktest(stockCode string, strategy Strategy, days int, initCash float64) (*BacktestResult, error) {
	svc := NewVectorBacktestService()
	return svc.RunStockBacktest(stockCode, strategy, days, initCash)
}

// RunStrategyGridSearch 对单只股票运行参数网格扫描
func (m *BacktestManager) RunStrategyGridSearch(stockCode string, strategies []Strategy, days int, initCash float64) (*GridSearchResult, error) {
	svc := NewVectorBacktestService()
	return svc.RunStockGridSearch(stockCode, strategies, days, initCash)
}

// ============================================================================
// 统计查询
// ============================================================================

// AccuracyStats 回测准确率统计
type AccuracyStats struct {
	TotalReports   int                  `json:"totalReports"`
	VerifiedCount  int                  `json:"verifiedCount"`
	AccurateCount  int                  `json:"accurateCount"`
	AccuracyRate   float64              `json:"accuracyRate"`
	AvgConfidence  float64              `json:"avgConfidence"`
	DirectionStats map[string]DirStat   `json:"directionStats"`
}

// DirStat 方向统计
type DirStat struct {
	Count    int     `json:"count"`
	Accurate int     `json:"accurate"`
	Rate     float64 `json:"rate"`
}

// GetOverallStats 获取总体回测统计
func (m *BacktestManager) GetOverallStats() *AccuracyStats {
	stats := &AccuracyStats{DirectionStats: make(map[string]DirStat)}

	var total int64
	db.Dao.Model(&models.AIResponseResult{}).
		Where("is_del = 0").
		Where(models.StockReportCodeSQL()).
		Count(&total)
	stats.TotalReports = int(total)

	var results []models.BacktestResult
	db.Dao.Find(&results)
	stats.VerifiedCount = len(results)

	for _, r := range results {
		if r.IsAccurate {
			stats.AccurateCount++
		}
		ds := stats.DirectionStats[r.Direction]
		ds.Count++
		if r.IsAccurate {
			ds.Accurate++
		}
		stats.DirectionStats[r.Direction] = ds
		stats.AvgConfidence += r.Confidence
	}

	if stats.VerifiedCount > 0 {
		stats.AccuracyRate = math.Round(float64(stats.AccurateCount)/float64(stats.VerifiedCount)*10000) / 100
		stats.AvgConfidence = math.Round(stats.AvgConfidence/float64(stats.VerifiedCount)*100) / 100
	}

	for dir, ds := range stats.DirectionStats {
		if ds.Count > 0 {
			ds.Rate = math.Round(float64(ds.Accurate)/float64(ds.Count)*10000) / 100
		}
		stats.DirectionStats[dir] = ds
	}

	return stats
}

// GetStockStats 获取指定股票的回测统计
func (m *BacktestManager) GetStockStats(stockCode string) *AccuracyStats {
	stats := &AccuracyStats{DirectionStats: make(map[string]DirStat)}

	var total int64
	db.Dao.Model(&models.AIResponseResult{}).
		Where("is_del = 0 AND stock_code = ?", stockCode).
		Where(models.StockReportCodeSQL()).
		Count(&total)
	stats.TotalReports = int(total)

	var results []models.BacktestResult
	db.Dao.Where("stock_code = ?", stockCode).Find(&results)
	stats.VerifiedCount = len(results)

	for _, r := range results {
		if r.IsAccurate {
			stats.AccurateCount++
		}
		ds := stats.DirectionStats[r.Direction]
		ds.Count++
		if r.IsAccurate {
			ds.Accurate++
		}
		stats.DirectionStats[r.Direction] = ds
		stats.AvgConfidence += r.Confidence
	}

	if stats.VerifiedCount > 0 {
		stats.AccuracyRate = math.Round(float64(stats.AccurateCount)/float64(stats.VerifiedCount)*10000) / 100
		stats.AvgConfidence = math.Round(stats.AvgConfidence/float64(stats.VerifiedCount)*100) / 100
	}

	for dir, ds := range stats.DirectionStats {
		if ds.Count > 0 {
			ds.Rate = math.Round(float64(ds.Accurate)/float64(ds.Count)*10000) / 100
		}
		stats.DirectionStats[dir] = ds
	}

	return stats
}

// NeedBacktest 是否有待回测的报告
func (m *BacktestManager) NeedBacktest() bool {
	var count int64
	db.Dao.Model(&models.AIResponseResult{}).
		Where("is_del = 0").
		Where(models.StockReportCodeSQL()).
		Where("NOT EXISTS (SELECT 1 FROM backtest_results WHERE backtest_results.report_id = ai_response_result.id)").
		Count(&count)
	return count > 0
}

// GetLatestResults 获取最近 N 条回测结果
func (m *BacktestManager) GetLatestResults(limit int) []models.BacktestResult {
	if limit <= 0 {
		limit = 20
	}
	var results []models.BacktestResult
	db.Dao.Order("id desc").Limit(limit).Find(&results)
	return results
}

// RunBacktestOnDateRange 对指定日期范围内的报告批量回测
func (m *BacktestManager) RunBacktestOnDateRange(startDate, endDate string) (int, error) {
	q := db.Dao.Model(&models.AIResponseResult{}).
		Where("is_del = 0").
		Where(models.StockReportCodeSQL())
	if startDate != "" {
		q = q.Where("created_at >= ?", startDate)
	}
	if endDate != "" {
		q = q.Where("created_at <= ?", endDate+" 23:59:59")
	}

	var reports []models.AIResponseResult
	q.Order("id asc").Limit(100).Find(&reports)
	if len(reports) == 0 {
		return 0, nil
	}

	count := 0
	for _, r := range reports {
		if _, err := m.RunSingleReportBacktest(r); err != nil {
			logger.SugaredLogger.Warnf("backtest report %d skipped: %v", r.ID, err)
			continue
		}
		count++
	}
	return count, nil
}

// ============================================================================
// 内部方法
// ============================================================================

func (m *BacktestManager) fetchPendingReports(limit int) ([]models.AIResponseResult, error) {
	if limit <= 0 {
		limit = 50
	}
	var reports []models.AIResponseResult
	err := db.Dao.Model(&models.AIResponseResult{}).
		Where("is_del = 0").
		Where(models.StockReportCodeSQL()).
		Where("NOT EXISTS (SELECT 1 FROM backtest_results WHERE backtest_results.report_id = ai_response_result.id)").
		Order("id asc").
		Limit(limit).
		Find(&reports).Error
	return reports, err
}

func (m *BacktestManager) batchLoadStockData(reports []models.AIResponseResult) map[string]*stockPriceData {
	stockCodes := make(map[string]bool)
	for _, r := range reports {
		stockCodes[r.StockCode] = true
	}

	stockData := make(map[string]*stockPriceData)
	for code := range stockCodes {
		c, h, l, o, v, d, err := m.loader.LoadKLineData(code, 365)
		if err != nil {
			logger.SugaredLogger.Warnf("load kline for %s failed: %v", code, err)
			continue
		}
		stockData[code] = &stockPriceData{close: c, high: h, low: l, open: o, volume: v, dates: d}
	}
	return stockData
}

func (m *BacktestManager) parallelBacktestReports(ctx context.Context, reports []models.AIResponseResult, stockData map[string]*stockPriceData) []aiReportResult {
	tasks := make(chan aiReportTask, len(reports))
	for _, r := range reports {
		tasks <- aiReportTask{report: r}
	}
	close(tasks)

	results := make(chan aiReportResult, len(reports))
	workers := 8
	if workers > len(reports) {
		workers = len(reports)
	}

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
				results <- m.backtestSingleReport(t.report, stockData)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var allResults []aiReportResult
	for r := range results {
		allResults = append(allResults, r)
	}
	return allResults
}

func (m *BacktestManager) backtestSingleReport(r models.AIResponseResult, stockData map[string]*stockPriceData) aiReportResult {
	if !models.IsStockReportCode(r.StockCode) {
		return aiReportResult{reportID: r.ID, skipped: true, skipReason: "非个股报告"}
	}
	pred := extractPredictionFromReport(r.Content)
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

	// 读取配置
	var settings data.Settings
	db.Dao.First(&settings)
	verifyDays := int(settings.BacktestVerifyDays)
	if verifyDays < 2 {
		verifyDays = 20
	}
	threshold := settings.BacktestAccuracyThreshold
	if threshold == 0 {
		threshold = 5.0
	}

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

	maxPrice := startPrice
	minPrice := startPrice
	for i := idx; i < endIdx; i++ {
		if pd.high[i] > maxPrice {
			maxPrice = pd.high[i]
		}
		if pd.low[i] < minPrice {
			minPrice = pd.low[i]
		}
	}
	maxUpside := (maxPrice - startPrice) / startPrice * 100
	maxDrawdown := (minPrice - startPrice) / startPrice * 100

	isAccurate := checkDirectionAccuracy(pred.Direction, priceChange, threshold)

	return aiReportResult{
		reportID:    r.ID,
		isAccurate:  isAccurate,
		priceChange: math.Round(priceChange*100) / 100,
		maxUpside:   math.Round(maxUpside*100) / 100,
		maxDrawdown: math.Round(maxDrawdown*100) / 100,
		direction:   pred.Direction,
		summary:     fmt.Sprintf("预测:%s | 实际涨跌:%.2f%% | 准确:%v", pred.Direction, priceChange, isAccurate),
		skipped:     false,
	}
}

func (m *BacktestManager) saveReportBacktest(r models.AIResponseResult, br aiReportResult) {
	pred := extractPredictionFromReport(r.Content)
	analysisDate := r.CreatedAt.Format("2006-01-02")

	summary := br.summary
	if br.skipped {
		summary = fmt.Sprintf("已跳过: %s", br.skipReason)
	}

	result := &models.BacktestResult{
		ReportID:     r.ID,
		StockCode:    r.StockCode,
		StockName:    r.StockName,
		AnalysisDate: analysisDate,
		Direction:    pred.Direction,
		TargetPrice:  pred.TargetPrice,
		VerifyDate:   "",
		VerifyPrice:  0,
		PriceChange:  br.priceChange,
		MaxDrawdown:  br.maxDrawdown,
		MaxUpside:    br.maxUpside,
		IsAccurate:   br.isAccurate,
		Confidence:   pred.Confidence,
		Summary:      summary,
	}
	db.Dao.Where("report_id = ?", r.ID).Assign(result).FirstOrCreate(&models.BacktestResult{})
}

func (m *BacktestManager) computeReportStats(results []aiReportResult) *aiBacktestStats {
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
	processed := countNotSkippedResults(results)
	if processed > 0 {
		stats.AccuracyRate = float64(stats.Accurate) / float64(processed)
	}
	return stats
}

// ============================================================================
// 预测提取（从 AI 报告 Markdown 中解析方向、目标价、置信度）
// ============================================================================

type prediction struct {
	Direction   string
	TargetPrice float64
	Confidence  float64
}

func extractPredictionFromReport(content string) prediction {
	p := prediction{Direction: "中性", Confidence: 0.5}

	bullish := countReportKeywords(content, []string{"看涨", "看好", "上涨", "反弹", "乐观", "做多", "买入", "加仓", "牛市", "多头", "强势", "突破"})
	bearish := countReportKeywords(content, []string{"看跌", "看空", "下跌", "回落", "悲观", "做空", "卖出", "减仓", "回调", "熊市", "空头", "弱势", "破位"})
	neutral := countReportKeywords(content, []string{"震荡", "盘整", "中性", "观望", "横盘", "整理", "徘徊", "均衡"})

	switch {
	case bullish > bearish && bullish > neutral:
		p.Direction = "看涨"
		p.Confidence = 0.5 + float64(bullish-bearish)*0.08
	case bearish > bullish && bearish > neutral:
		p.Direction = "看跌"
		p.Confidence = 0.5 + float64(bearish-bullish)*0.08
	case neutral >= bullish && neutral >= bearish:
		p.Direction = "震荡"
		p.Confidence = 0.5 + float64(neutral)*0.05
	case bullish == bearish && bullish > 0:
		p.Direction = "中性"
		p.Confidence = 0.5
	}

	if prob := extractProbability(content); prob > 0 {
		p.Confidence = prob
	}

	p.TargetPrice = extractTargetPrice(content)

	if p.Confidence > 1 {
		p.Confidence = 1
	}
	return p
}

func countReportKeywords(text string, keywords []string) int {
	n := 0
	for _, kw := range keywords {
		n += strings.Count(text, kw)
	}
	return n
}

func extractProbability(text string) float64 {
	patterns := []string{"概率", "可能性", "胜率"}
	for _, pat := range patterns {
		idx := strings.Index(text, pat)
		if idx >= 0 {
			before := text[:idx]
			fields := strings.Fields(before)
			for i := len(fields) - 1; i >= 0; i-- {
				cleaned := strings.TrimRight(fields[i], "：:，,。. %")
				if v, err := strconv.ParseFloat(cleaned, 64); err == nil {
					if v >= 0 && v <= 100 {
						return v / 100
					}
				}
			}
		}
	}
	if idx := strings.Index(text, "%-"); idx >= 0 {
		before := text[:idx]
		fields := strings.Fields(before)
		for i := len(fields) - 1; i >= 0; i-- {
			cleaned := strings.TrimRight(fields[i], "：:，,。.")
			if v, err := strconv.ParseFloat(cleaned, 64); err == nil {
				if v >= 0 && v <= 100 {
					return v / 100
				}
			}
		}
	}
	return 0
}

func extractTargetPrice(text string) float64 {
	patterns := []string{"目标价", "目标价位", "目标价格", "上看"}
	for _, pat := range patterns {
		idx := strings.Index(text, pat)
		if idx >= 0 {
			after := text[idx+len([]rune(pat)):]
			fields := strings.Fields(after)
			if len(fields) > 0 {
				cleaned := strings.TrimLeft(fields[0], "：: ")
				if v, err := strconv.ParseFloat(cleaned, 64); err == nil {
					return v
				}
			}
		}
	}
	return 0
}

func checkDirectionAccuracy(direction string, priceChange, threshold float64) bool {
	switch direction {
	case "看涨":
		return priceChange > threshold
	case "看跌":
		return priceChange < -threshold
	case "震荡":
		return priceChange > -3 && priceChange < 3
	case "中性":
		return priceChange > -5 && priceChange < 5
	default:
		return false
	}
}

// ============================================================================
// 辅助
// ============================================================================

func countSkippedResults(results []aiReportResult) int {
	n := 0
	for _, r := range results {
		if r.skipped {
			n++
		}
	}
	return n
}

func countNotSkippedResults(results []aiReportResult) int {
	n := 0
	for _, r := range results {
		if !r.skipped {
			n++
		}
	}
	return n
}

// getReportsForSkill retrieves reports that match the skill's trigger keywords.
func (m *BacktestManager) getReportsForSkill(skill *models.Skill, limit int) ([]models.AIResponseResult, error) {
	if skill == nil {
		return nil, fmt.Errorf("skill is nil")
	}
	keywords := strings.Split(skill.TriggerKeywords, ",")
	var kwList []string
	for _, k := range keywords {
		k = strings.TrimSpace(k)
		if k != "" {
			kwList = append(kwList, k)
		}
	}
	if len(kwList) == 0 {
		return nil, fmt.Errorf("no trigger keywords for skill %d", skill.ID)
	}
	var reports []models.AIResponseResult
	db := db.Dao.Model(&models.AIResponseResult{})
	for i, k := range kwList {
		if i == 0 {
			db = db.Where("question LIKE ?", "%"+k+"%")
		} else {
			db = db.Or("question LIKE ?", "%"+k+"%")
		}
	}
	if limit > 0 {
		db = db.Limit(limit)
	}
	if err := db.Find(&reports).Error; err != nil {
		return nil, fmt.Errorf("query reports: %w", err)
	}
	return reports, nil
}

// RunSkillABTest runs an A/B test between two skills based on trigger keyword matching
// and returns accuracy for each.
func (m *BacktestManager) RunSkillABTest(ctx context.Context, skillID1, skillID2 uint, limit int) (*SkillABTestResult, error) {
	skill1, err := data.NewSkillApi().GetByID(skillID1)
	if err != nil {
		return nil, fmt.Errorf("get skill %d: %w", skillID1, err)
	}
	skill2, err := data.NewSkillApi().GetByID(skillID2)
	if err != nil {
		return nil, fmt.Errorf("get skill %d: %w", skillID2, err)
	}
	if skill1 == nil || skill2 == nil {
		return nil, fmt.Errorf("one or both skills not found")
	}
	// Get reports for each skill
	reports1, err := m.getReportsForSkill(skill1, limit)
	if err != nil {
		return nil, err
	}
	reports2, err := m.getReportsForSkill(skill2, limit)
	if err != nil {
		return nil, err
	}
	// Run backtest for each set
	var acc1, acc2 float64
	var total1, total2 int
	if len(reports1) > 0 {
		var accurate1 int
		for _, r := range reports1 {
			br, err := m.RunSingleReportBacktest(r)
			if err != nil {
				continue
			}
			if br.IsAccurate {
				accurate1++
			}
			total1++
		}
		if total1 > 0 {
			acc1 = float64(accurate1) / float64(total1)
		}
	}
	if len(reports2) > 0 {
		var accurate2 int
		for _, r := range reports2 {
			br, err := m.RunSingleReportBacktest(r)
			if err != nil {
				continue
			}
			if br.IsAccurate {
				accurate2++
			}
			total2++
		}
		if total2 > 0 {
			acc2 = float64(accurate2) / float64(total2)
		}
	}
	result := &SkillABTestResult{
		SkillID1:   skill1.ID,
		SkillID2:   skill2.ID,
		Accuracy1:  acc1,
		Accuracy2:  acc2,
		TotalTested: total1 + total2,
		Details:    map[uint]int{skill1.ID: total1, skill2.ID: total2},
	}
	return result, nil
}

// SkillABTestResult holds the result of an A/B test between two skills.
type SkillABTestResult struct {
	SkillID1     uint
	SkillID2     uint
	Accuracy1    float64
	Accuracy2    float64
	TotalTested  int
	Details      map[uint]int // skillID -> number of reports tested
}
