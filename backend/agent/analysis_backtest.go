package agent

import (
	"fmt"
	"blades-stock/backend/db"
	"blades-stock/backend/models"
	"math"
	"sort"
	"strconv"
	"strings"

	"blades-stock/backend/data"
	"blades-stock/backend/logger"
)

// BacktestService 负责将 AI 分析报告与市场真实走势对比 → 回测验证
type BacktestService struct{}

func NewBacktestService() *BacktestService {
	return &BacktestService{}
}

// ---------------------------------------------------------------------------
// 预测提取（从 Markdown 报告中解析方向与目标价）
// ---------------------------------------------------------------------------

type prediction struct {
	Direction   string  // 看涨 / 看跌 / 中性 / 震荡
	TargetPrice float64 // 目标价（0 表示未给出）
	Confidence  float64 // 0-1
}

func extractPrediction(content string) prediction {
	p := prediction{Direction: "中性", Confidence: 0.5}

	// --- 方向判断 ---
	bullish := countKeywords(content, []string{"看涨", "看好", "上涨", "反弹", "乐观", "做多", "买入", "加仓", "牛市", "多头", "强势", "突破"})
	bearish := countKeywords(content, []string{"看跌", "看空", "下跌", "回落", "悲观", "做空", "卖出", "减仓", "回调", "熊市", "空头", "弱势", "破位"})
	neutral := countKeywords(content, []string{"震荡", "盘整", "中性", "观望", "横盘", "整理", "徘徊", "均衡"})

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

	// --- 概率修正 ---
	if prob := extractProbability(content); prob > 0 {
		p.Confidence = prob
	}

	// --- 目标价 ---
	p.TargetPrice = extractTargetPrice(content)

	if p.Confidence > 1 {
		p.Confidence = 1
	}
	return p
}

func countKeywords(text string, keywords []string) int {
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
	// also check top/bottom of range like 60%-80%
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
	// 目标价: target price patterns
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

// ---------------------------------------------------------------------------
// 获取实际验证价格
// ---------------------------------------------------------------------------

type pricePoint struct {
	Date  string
	Close float64
	High  float64
	Low   float64
}

func getActualPrices(stockCode string, days int) []pricePoint {
	kLines := data.NewStockDataApi().GetKLineData(stockCode, "day", int64(days))
	if kLines == nil || len(*kLines) == 0 {
		return nil
	}
	var pts []pricePoint
	for _, k := range *kLines {
		closeV, _ := strconv.ParseFloat(k.Close, 64)
		highV, _ := strconv.ParseFloat(k.High, 64)
		lowV, _ := strconv.ParseFloat(k.Low, 64)
		if closeV > 0 {
			pts = append(pts, pricePoint{
				Date:  k.Day,
				Close: closeV,
				High:  highV,
				Low:   lowV,
			})
		}
	}
	// Sort ascending by date
	sort.Slice(pts, func(i, j int) bool {
		return pts[i].Date < pts[j].Date
	})
	return pts
}

// ---------------------------------------------------------------------------
// 核心回测逻辑
// ---------------------------------------------------------------------------

// RunAllBacktests 对所有尚未回测的分析报告执行回测
func (s *BacktestService) RunAllBacktests() (int, error) {
	var reports []models.AIResponseResult
	db.Dao.Model(&models.AIResponseResult{}).
		Where("is_del = 0").
		Where("NOT EXISTS (SELECT 1 FROM backtest_results WHERE backtest_results.report_id = ai_response_result.id)").
		Order("id asc").
		Limit(50).
		Find(&reports)

	if len(reports) == 0 {
		return 0, nil
	}

	count := 0
	for _, r := range reports {
		if _, err := s.RunSingleBacktest(r); err != nil {
			logger.SugaredLogger.Warnf("backtest report %d skipped: %v", r.ID, err)
			continue
		}
		count++
	}
	return count, nil
}

// RunSingleBacktest 对单个报告执行回测并保存结果
func isValidStockCode(code string) bool {
	if len(code) < 6 || len(code) > 12 {
		return false
	}
	// 常见股票代码格式: sh/sz/bj 开头 + 数字, 或纯数字, 或 us/GB 开头
	for _, prefix := range []string{"sh", "sz", "bj", "gb_", "GB_", "us"} {
		if strings.HasPrefix(strings.ToLower(code), prefix) {
			return len(code) > len(prefix)
		}
	}
	// 纯数字（A股代码）
	_, err := strconv.Atoi(code)
	return err == nil
}

func (s *BacktestService) RunSingleBacktest(r models.AIResponseResult) (*models.BacktestResult, error) {
	if !isValidStockCode(r.StockCode) {
		return nil, fmt.Errorf("invalid stock code: %s", r.StockCode)
	}
	pred := extractPrediction(r.Content)
	analysisDate := r.CreatedAt.Format("2006-01-02")

	// 获取分析日期之后的真实价格
	prices := getActualPrices(r.StockCode, 365)
	if len(prices) == 0 {
		return nil, fmt.Errorf("no price data for %s", r.StockCode)
	}

	startIdx := -1
	for i, p := range prices {
		if p.Date >= analysisDate {
			startIdx = i
			break
		}
	}
	if startIdx < 0 {
		return nil, fmt.Errorf("no price data after %s for %s", analysisDate, r.StockCode)
	}

	// 取后续 verifyDays 个交易日 作为验证窗口
	verifyWindow := 20 // 临时使用默认值，TODO: 从配置读取
	// verifyWindow := verifyDays // 暂时注释掉，编译错误
	if verifyWindow < 2 {
		verifyWindow = 2 // 最少 2 个交易日
	}
	endIdx := startIdx + verifyWindow
	if endIdx > len(prices) {
		endIdx = len(prices)
	}
	if endIdx-startIdx < 2 {
		return nil, fmt.Errorf("insufficient price data after %s", analysisDate)
	}

	startPrice := prices[startIdx].Close
	endPrice := prices[endIdx-1].Close

	priceChange := (endPrice - startPrice) / startPrice * 100

	maxPrice := startPrice
	minPrice := startPrice
	for i := startIdx; i < endIdx; i++ {
		if prices[i].High > maxPrice {
			maxPrice = prices[i].High
		}
		if prices[i].Low < minPrice {
			minPrice = prices[i].Low
		}
	}
	maxUpside := (maxPrice - startPrice) / startPrice * 100
	maxDrawdown := (minPrice - startPrice) / startPrice * 100

	// 判断方向准确（使用可配置阈值）
	isAccurate := false
	// 从数据库读取配置
	var settings data.Settings
	db.Dao.First(&settings)
	accuracyThreshold := settings.BacktestAccuracyThreshold
	if accuracyThreshold == 0 {
		accuracyThreshold = 0.0 // 默认 0%
	}
	verifyDays := int(settings.BacktestVerifyDays)
	if verifyDays == 0 {
		verifyDays = 20 // 默认 20 个交易日
	}
	switch pred.Direction {
	case "看涨":
		isAccurate = priceChange > accuracyThreshold
	case "看跌":
		isAccurate = priceChange < -accuracyThreshold
	case "震荡":
		isAccurate = priceChange > -3 && priceChange < 3
	case "中性":
		isAccurate = priceChange > -5 && priceChange < 5
	}

	result := &models.BacktestResult{
		ReportID:     r.ID,
		StockCode:    r.StockCode,
		StockName:    r.StockName,
		AnalysisDate: analysisDate,
		Direction:    pred.Direction,
		TargetPrice:  pred.TargetPrice,
		VerifyDate:   prices[endIdx-1].Date,
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

// ---------------------------------------------------------------------------
// 统计查询
// ---------------------------------------------------------------------------

type AccuracyStats struct {
	TotalReports   int     `json:"totalReports"`
	VerifiedCount  int     `json:"verifiedCount"`
	AccurateCount  int     `json:"accurateCount"`
	AccuracyRate   float64 `json:"accuracyRate"`
	AvgConfidence  float64 `json:"avgConfidence"`
	DirectionStats map[string]DirectionStat
}

type DirectionStat struct {
	Count    int     `json:"count"`
	Accurate int     `json:"accurate"`
	Rate     float64 `json:"rate"`
}

func (s *BacktestService) GetOverallStats() *AccuracyStats {
	stats := &AccuracyStats{
		DirectionStats: make(map[string]DirectionStat),
	}

	var total int64
	db.Dao.Model(&models.AIResponseResult{}).Where("is_del = 0").Count(&total)
	stats.TotalReports = int(total)

	var results []models.BacktestResult
	db.Dao.Find(&results)
	stats.VerifiedCount = len(results)

	for _, r := range results {
		if r.IsAccurate {
			stats.AccurateCount++
		}
		dir := r.Direction
		ds := stats.DirectionStats[dir]
		ds.Count++
		if r.IsAccurate {
			ds.Accurate++
		}
		stats.DirectionStats[dir] = ds
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

func (s *BacktestService) GetStockAccuracy(stockCode string) *AccuracyStats {
	stats := &AccuracyStats{
		DirectionStats: make(map[string]DirectionStat),
	}

	var total int64
	db.Dao.Model(&models.AIResponseResult{}).Where("is_del = 0 AND stock_code = ?", stockCode).Count(&total)
	stats.TotalReports = int(total)

	var results []models.BacktestResult
	db.Dao.Where("stock_code = ?", stockCode).Find(&results)
	stats.VerifiedCount = len(results)

	for _, r := range results {
		if r.IsAccurate {
			stats.AccurateCount++
		}
		dir := r.Direction
		ds := stats.DirectionStats[dir]
		ds.Count++
		if r.IsAccurate {
			ds.Accurate++
		}
		stats.DirectionStats[dir] = ds
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

// RunBacktestOnDateRange 对指定日期范围内的分析报告批量回测
func (s *BacktestService) RunBacktestOnDateRange(startDate, endDate string) (int, error) {
	var reports []models.AIResponseResult
	q := db.Dao.Model(&models.AIResponseResult{}).Where("is_del = 0")

	if startDate != "" {
		q = q.Where("created_at >= ?", startDate)
	}
	if endDate != "" {
		q = q.Where("created_at <= ?", endDate+" 23:59:59")
	}

	q.Order("id asc").Limit(100).Find(&reports)
	if len(reports) == 0 {
		return 0, nil
	}

	count := 0
	for _, r := range reports {
		if _, err := s.RunSingleBacktest(r); err != nil {
			logger.SugaredLogger.Errorf("backtest report %d failed: %v", r.ID, err)
			continue
		}
		count++
	}
	return count, nil
}

// NeedBacktest 检查是否有未回测的报告
func (s *BacktestService) NeedBacktest() bool {
	var count int64
	db.Dao.Model(&models.AIResponseResult{}).
		Where("is_del = 0").
		Where("NOT EXISTS (SELECT 1 FROM backtest_results WHERE backtest_results.report_id = ai_response_result.id)").
		Count(&count)
	return count > 0
}

// GetLatestBacktestResults 获取最近 N 条回测结果
func (s *BacktestService) GetLatestBacktestResults(limit int) []models.BacktestResult {
	if limit <= 0 {
		limit = 20
	}
	var results []models.BacktestResult
	db.Dao.Order("id desc").Limit(limit).Find(&results)
	return results
}
