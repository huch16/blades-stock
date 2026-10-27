package quant

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
)

var (
	autoAlphaMu sync.Mutex
)

// AutoAlphaExplorer 负责夜间非交易时间的无监督参数网格优化与策略因子自进化
type AutoAlphaExplorer struct{}

func NewAutoAlphaExplorer() *AutoAlphaExplorer {
	return &AutoAlphaExplorer{}
}

// ParamGridResult 记录一组参数在历史回测中的表现
type ParamGridResult struct {
	LookbackDays     int     `json:"lookbackDays"`
	StopLoss         float64 `json:"stopLoss"`
	AnnualizedReturn float64 `json:"annualizedReturn"`
	MaxDrawdown      float64 `json:"maxDrawdown"`
	SharpeRatio      float64 `json:"sharpeRatio"`
	WinRate          float64 `json:"winRate"`
	TotalTrades      int     `json:"totalTrades"`
}

// ExploreETFMomentumParams 对指定 ETF 池进行多周期参数网格遍历回测与最优参数自发现
func (e *AutoAlphaExplorer) ExploreETFMomentumParams(targetCodes []string) (*models.AutoAlphaProposal, error) {
	if len(targetCodes) == 0 {
		targetCodes = []string{"512480", "159915", "515050", "588000"} // 默认科技成长核心池
	}

	autoAlphaMu.Lock()
	defer autoAlphaMu.Unlock()

	// 1. 获取所有标的的 K 线收盘价历史 (近 120 个交易日)
	klineDataMap := make(map[string][]float64)
	for _, code := range targetCodes {
		res := data.FetchKLineWithFallback(code, "", "101", 120, "")
		if res != nil && res.Data != nil && len(*res.Data) >= 40 {
			var closes []float64
			for _, k := range *res.Data {
				c, _ := strconv.ParseFloat(k.Close, 64)
				if c > 0 {
					closes = append(closes, c)
				}
			}
			if len(closes) >= 40 {
				klineDataMap[code] = closes
			}
		}
	}

	if len(klineDataMap) == 0 {
		return nil, fmt.Errorf("未能获取足够的有效 ETF K线历史数据进行回测")
	}

	// 2. 参数网格候选空间
	lookbacks := []int{7, 10, 13, 21, 30}
	stopLosses := []float64{0.03, 0.05, 0.08}

	var allResults []ParamGridResult
	var baselineResult *ParamGridResult

	for _, w := range lookbacks {
		for _, sl := range stopLosses {
			res := simulateETFMomentumBacktest(klineDataMap, w, sl)
			allResults = append(allResults, res)
			if w == 21 && sl == 0.05 {
				baselineResult = &res
			}
		}
	}

	if baselineResult == nil && len(allResults) > 0 {
		baselineResult = &allResults[0]
	}

	// 3. 寻找最优参数组 (Sharpe 优先，同时要求 MaxDrawdown 不劣于基准 + 5%)
	best := *baselineResult
	for _, r := range allResults {
		if r.SharpeRatio > best.SharpeRatio && r.MaxDrawdown <= baselineResult.MaxDrawdown+5.0 {
			best = r
		}
	}

	// 4. 构建优化提案
	oldParamJSON, _ := json.Marshal(map[string]interface{}{
		"lookbackDays": 21,
		"stopLoss":     0.05,
	})
	optimalParamJSON, _ := json.Marshal(map[string]interface{}{
		"lookbackDays": best.LookbackDays,
		"stopLoss":     best.StopLoss,
	})

	assetPoolDesc := fmt.Sprintf("科技ETF核心池 (%s)", formatCodesString(targetCodes))
	ddImprovement := baselineResult.MaxDrawdown - best.MaxDrawdown
	if ddImprovement < 0 {
		ddImprovement = 0
	}

	summary := fmt.Sprintf("在近120个交易日历史走势中，通过 %d 组参数网格检验：\n"+
		"• 基准参数 (21日动量+5%%止损)：年化收益率 %.2f%%，夏普比率 %.2f，最大回撤 %.2f%%\n"+
		"• 最优推荐参数 (%d日动量+%.1f%%止损)：年化收益率提升至 %.2f%%，夏普比率由 %.2f 提升至 %.2f，最大回撤收敛 %.2f%%\n"+
		"• 鲁棒性建议：在当前盘面风格切换频繁时期，缩短动量回看窗口至 %d 日能更敏锐捕捉板块轮动先机。",
		len(allResults),
		baselineResult.AnnualizedReturn, baselineResult.SharpeRatio, baselineResult.MaxDrawdown,
		best.LookbackDays, best.StopLoss*100, best.AnnualizedReturn, baselineResult.SharpeRatio, best.SharpeRatio, ddImprovement,
		best.LookbackDays)

	proposal := models.AutoAlphaProposal{
		StrategyName:         "ETF_MOMENTUM_ROTATION",
		TargetAsset:          assetPoolDesc,
		OldParams:            string(oldParamJSON),
		OptimalParams:        string(optimalParamJSON),
		OriginalSharpe:       baselineResult.SharpeRatio,
		OptimizedSharpe:      best.SharpeRatio,
		ExpectedAnnualReturn: best.AnnualizedReturn,
		MaxDrawdownReduction: ddImprovement,
		WinRate:              best.WinRate,
		Status:               "PENDING_APPROVAL",
		EvaluationSummary:    summary,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}

	if db.Dao != nil {
		_ = db.Dao.Create(&proposal).Error
	}

	logger.SugaredLogger.Infof("[AutoAlpha] 完成 ETF 动量策略参数夜间自进化探索: %s -> 最优参数: Lookback=%d天, SL=%.1f%% (夏普: %.2f -> %.2f)",
		assetPoolDesc, best.LookbackDays, best.StopLoss*100, baselineResult.SharpeRatio, best.SharpeRatio)

	return &proposal, nil
}

// RunNightlyAutoAlphaExploration 批量执行全资产池夜间参数进化
func (e *AutoAlphaExplorer) RunNightlyAutoAlphaExploration() ([]models.AutoAlphaProposal, error) {
	var proposals []models.AutoAlphaProposal

	pools := []struct {
		Name  string
		Codes []string
	}{
		{"科技成长ETF池", []string{"512480", "159915", "515050", "588000"}},
		{"大盘价值ETF池", []string{"510300", "510050", "510500"}},
		{"顺周期制造池", []string{"512880", "512660", "512400"}},
	}

	for _, p := range pools {
		prop, err := e.ExploreETFMomentumParams(p.Codes)
		if err == nil && prop != nil {
			proposals = append(proposals, *prop)
		}
	}

	return proposals, nil
}

// GetActiveProposals 获取最近的策略优化提案列表
func (e *AutoAlphaExplorer) GetActiveProposals(limit int) ([]models.AutoAlphaProposal, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	var list []models.AutoAlphaProposal
	err := db.Dao.Order("id desc").Limit(limit).Find(&list).Error
	return list, err
}

// simulateETFMomentumBacktest 向量化简易回测单个参数组
func simulateETFMomentumBacktest(klineDataMap map[string][]float64, lookback int, stopLoss float64) ParamGridResult {
	// 找到所有标的对齐的最短长度
	minLen := 9999
	for _, closes := range klineDataMap {
		if len(closes) < minLen {
			minLen = len(closes)
		}
	}

	if minLen <= lookback+5 {
		return ParamGridResult{}
	}

	totalReturn := 1.0
	peakReturn := 1.0
	maxDrawdown := 0.0
	totalTrades := 0
	winTrades := 0

	var dailyReturns []float64

	currentHold := ""
	entryPrice := 0.0

	for t := lookback; t < minLen-1; t++ {
		// 计算各标的在 t 时刻基于 lookback 窗口的动量得分
		bestCode := ""
		bestScore := -999.0

		for code, closes := range klineDataMap {
			window := closes[t-lookback : t]
			score := calcSimpleMomentum(window)
			if score > bestScore {
				bestScore = score
				bestCode = code
			}
		}

		// 检查持仓出场与切换
		todayRet := 0.0
		if currentHold != "" {
			todayPrice := klineDataMap[currentHold][t]
			nextPrice := klineDataMap[currentHold][t+1]
			ret := (nextPrice - todayPrice) / todayPrice

			// 止损触发检查
			if (todayPrice-entryPrice)/entryPrice <= -stopLoss {
				// 止损离场
				ret = -stopLoss
				currentHold = ""
				totalTrades++
			} else if bestCode != currentHold && bestScore > 0 {
				// 轮动换仓
				if (todayPrice - entryPrice) > 0 {
					winTrades++
				}
				totalTrades++
				currentHold = bestCode
				entryPrice = todayPrice
			}
			todayRet = ret
		} else if bestScore > 0 {
			// 建仓
			currentHold = bestCode
			entryPrice = klineDataMap[bestCode][t]
			nextPrice := klineDataMap[bestCode][t+1]
			todayRet = (nextPrice - entryPrice) / entryPrice
		}

		dailyReturns = append(dailyReturns, todayRet)
		totalReturn *= (1.0 + todayRet)
		if totalReturn > peakReturn {
			peakReturn = totalReturn
		}
		dd := (peakReturn - totalReturn) / peakReturn * 100.0
		if dd > maxDrawdown {
			maxDrawdown = dd
		}
	}

	days := float64(len(dailyReturns))
	annualized := (math.Pow(totalReturn, 250.0/days) - 1.0) * 100.0
	if math.IsNaN(annualized) || math.IsInf(annualized, 0) {
		annualized = 0.0
	}

	sharpe := calcSharpeFromDaily(dailyReturns)
	winRate := 0.0
	if totalTrades > 0 {
		winRate = (float64(winTrades) / float64(totalTrades)) * 100.0
	}

	return ParamGridResult{
		LookbackDays:     lookback,
		StopLoss:         stopLoss,
		AnnualizedReturn: math.Round(annualized*100.0) / 100.0,
		MaxDrawdown:      math.Round(maxDrawdown*100.0) / 100.0,
		SharpeRatio:      math.Round(sharpe*100.0) / 100.0,
		WinRate:          math.Round(winRate*100.0) / 100.0,
		TotalTrades:      totalTrades,
	}
}

func calcSimpleMomentum(prices []float64) float64 {
	n := len(prices)
	if n < 2 {
		return 0
	}
	// 对数收益率斜率
	first := prices[0]
	last := prices[n-1]
	if first <= 0 || last <= 0 {
		return 0
	}
	return (last - first) / first
}

func calcSharpeFromDaily(returns []float64) float64 {
	n := float64(len(returns))
	if n < 5 {
		return 0
	}
	sum := 0.0
	for _, r := range returns {
		sum += r
	}
	mean := sum / n

	varSum := 0.0
	for _, r := range returns {
		varSum += math.Pow(r-mean, 2)
	}
	std := math.Sqrt(varSum / n)
	if std <= 0.000001 {
		return 0
	}

	dailyRf := 0.02 / 250.0
	annualSharpe := ((mean - dailyRf) / std) * math.Sqrt(250.0)
	if math.IsNaN(annualSharpe) || math.IsInf(annualSharpe, 0) {
		return 0
	}
	return annualSharpe
}

func formatCodesString(codes []string) string {
	res := ""
	for i, c := range codes {
		if i > 0 {
			res += ", "
		}
		res += c
	}
	return res
}
