package quant

import (
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/models"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// StockDiagnosisResult 股票结构化三维诊断结果
type StockDiagnosisResult struct {
	StockCode        string                  `json:"stockCode"`
	StockName        string                  `json:"stockName"`
	OverallScore     int                     `json:"overallScore"`     // 综合评分 0-100
	Suggestion       string                  `json:"suggestion"`       // 强烈看多 / 逢低买入 / 震荡观望 / 逢高减持 / 止损离场
	Confidence       float64                 `json:"confidence"`       // 置信度 / 历史胜率 (%)
	CurrentPrice     float64                 `json:"currentPrice"`     // 当前现价
	TargetPrice      float64                 `json:"targetPrice"`      // 建议第一目标价
	StopLossPrice    float64                 `json:"stopLossPrice"`    // 建议止损价
	RiskRewardRatio  float64                 `json:"riskRewardRatio"`  // 盈亏比
	QuantDim         QuantDimensionResult    `json:"quantDim"`         // 量化回测维度
	MoneyDim         MoneyDimensionResult    `json:"moneyDim"`         // 资金筹码维度
	TechnicalDim     TechnicalDimensionResult `json:"technicalDim"`    // 技术形态维度
	Summary          string                  `json:"summary"`          // 核心论据与诊断摘要
	Risks            []string                `json:"risks"`            // 关键风险提示
	HistoryTrackRecord string                `json:"historyTrackRecord"` // AI历史在该股上的预测胜率复盘
	LastUpdated      string                  `json:"lastUpdated"`      // 诊断时间
}

// QuantDimensionResult 量化回测维度
type QuantDimensionResult struct {
	Score          int       `json:"score"`           // 评分 0-100
	StrategyName   string    `json:"strategyName"`    // 回测策略
	WinRatePct     float64   `json:"winRatePct"`      // 历史胜率 %
	TotalReturnPct float64   `json:"totalReturnPct"`  // 回测总收益率 %
	SharpeRatio    float64   `json:"sharpeRatio"`     // 夏普比率
	MaxDrawdownPct float64   `json:"maxDrawdownPct"`  // 最大回撤 %
	ProfitFactor   float64   `json:"profitFactor"`    // 盈亏比
	TotalTrades    int       `json:"totalTrades"`     // 交易样本次数
	EquityCurve    []float64 `json:"equityCurve"`     // 净值曲线
	Evaluation     string    `json:"evaluation"`      // 量化评价
}

// MoneyDimensionResult 资金筹码维度
type MoneyDimensionResult struct {
	Score              int     `json:"score"`              // 评分 0-100
	MainNetInflow5D    float64 `json:"mainNetInflow5D"`    // 近5日主力净流入（万元）
	SuperLargeNetPct   float64 `json:"superLargeNetPct"`   // 超大单净占比 %
	LargeNetPct        float64 `json:"largeNetPct"`        // 大单净占比 %
	RecentInflowTrend  string  `json:"recentInflowTrend"`  // 流入态势: 持续流入 / 净流出 / 分歧震荡
	ChipConcentration  string  `json:"chipConcentration"`  // 筹码倾向
	Evaluation         string  `json:"evaluation"`         // 资金评价
}

// TechnicalDimensionResult 技术形态维度
type TechnicalDimensionResult struct {
	Score         int     `json:"score"`         // 评分 0-100
	Trend         string  `json:"trend"`         // 趋势: 多头排列 / 空头排列 / 震荡整理
	MA5           float64 `json:"ma5"`           // 5日均线
	MA20          float64 `json:"ma20"`          // 20日均线
	MA60          float64 `json:"ma60"`          // 60日均线
	ATR           float64 `json:"atr"`           // 真实波幅 (14日)
	VolumeRatio   float64 `json:"volumeRatio"`   // 量比
	PriceDistanceMA20 float64 `json:"priceDistanceMA20"` // 距20日线乖离率 %
	Evaluation    string  `json:"evaluation"`    // 技术评价
}

// DiagnoseStock 对单只股票执行全面的三维结构化量化诊断
func DiagnoseStock(rawCode string) (*StockDiagnosisResult, error) {
	cleanCode := models.CleanStockCode(rawCode)
	if cleanCode == "" {
		return nil, fmt.Errorf("无效的股票代码: %s", rawCode)
	}

	stockName := ""
	var basic data.StockBasic
	if err := db.Dao.Where("ts_code LIKE ?", "%"+cleanCode+"%").First(&basic).Error; err == nil && basic.Name != "" {
		stockName = basic.Name
	}

	loader := NewStockDataLoader()
	closeP, highP, lowP, _, volP, _, err := loader.LoadKLineData(cleanCode, 365)
	if err != nil || len(closeP) < 20 {
		return nil, fmt.Errorf("获取K线数据失败或数据不足: %v", err)
	}

	n := len(closeP)
	currentPrice := closeP[n-1]

	// 1. 量化回测维度 (VectorBT 双均线策略)
	quantRes := runQuantDiagnosis(cleanCode, 365)

	// 2. 技术形态维度
	techRes := runTechnicalDiagnosis(closeP, highP, lowP, volP)

	// 3. 资金筹码维度
	moneyRes := runMoneyDiagnosis(cleanCode)

	// 4. 结合 ATR 与波动率计算建议止损价与目标价
	atrVal := techRes.ATR
	if atrVal <= 0 {
		atrVal = currentPrice * 0.03 // 默认 3% 波动
	}

	// 止损设置在 1.8 倍 ATR 下方，目标价设在 2.8 倍 ATR 上方（盈亏比约 1.55）
	stopLoss := math.Round((currentPrice-1.8*atrVal)*100) / 100
	if stopLoss < 0 {
		stopLoss = math.Round(currentPrice*0.92*100) / 100
	}
	targetPrice := math.Round((currentPrice+2.8*atrVal)*100) / 100
	lossDiff := currentPrice - stopLoss
	gainDiff := targetPrice - currentPrice
	riskReward := 1.55
	if lossDiff > 0 {
		riskReward = math.Round((gainDiff/lossDiff)*100) / 100
	}

	// 5. 综合评分计算：量化40% + 资金35% + 技术25%
	overallScore := int(math.Round(float64(quantRes.Score)*0.40 + float64(moneyRes.Score)*0.35 + float64(techRes.Score)*0.25))
	if overallScore > 100 {
		overallScore = 100
	} else if overallScore < 0 {
		overallScore = 0
	}

	// 6. 确定操作建议与胜率置信度
	var suggestion string
	switch {
	case overallScore >= 80:
		suggestion = "强烈看多 (Strong Buy)"
	case overallScore >= 65:
		suggestion = "逢低买入 (Buy on Dips)"
	case overallScore >= 50:
		suggestion = "震荡观望 (Hold / Neutral)"
	case overallScore >= 35:
		suggestion = "逢高减持 (Trim / Sell)"
	default:
		suggestion = "规避离场 (Strong Sell)"
	}

	confidence := quantRes.WinRatePct
	if confidence <= 0 {
		confidence = float64(overallScore)
	}

	// 7. 提取历史 AI 在该股上的真实战绩（自反思闭环）
	btMgr := NewBacktestManager()
	var historyTrack string
	if stats := btMgr.GetStockStats(cleanCode); stats != nil && stats.VerifiedCount > 0 {
		historyTrack = fmt.Sprintf("AI历史预测该股 %d 次，实盘验证胜率 %.1f%% (准确 %d 次)",
			stats.VerifiedCount, stats.AccuracyRate*100, stats.AccurateCount)
	} else {
		historyTrack = "暂无该标的历史闭环回测样本"
	}

	// 8. 提炼核心结论与风险提示
	summary := buildDiagnosisSummary(stockName, cleanCode, overallScore, suggestion, quantRes, moneyRes, techRes)
	risks := buildDiagnosisRisks(currentPrice, techRes, moneyRes, quantRes)

	return &StockDiagnosisResult{
		StockCode:          cleanCode,
		StockName:          stockName,
		OverallScore:       overallScore,
		Suggestion:         suggestion,
		Confidence:         math.Round(confidence*10) / 10,
		CurrentPrice:       currentPrice,
		TargetPrice:        targetPrice,
		StopLossPrice:      stopLoss,
		RiskRewardRatio:    riskReward,
		QuantDim:           quantRes,
		MoneyDim:           moneyRes,
		TechnicalDim:       techRes,
		Summary:            summary,
		Risks:              risks,
		HistoryTrackRecord: historyTrack,
		LastUpdated:        time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

func runQuantDiagnosis(cleanCode string, days int) QuantDimensionResult {
	params := map[string]any{"fast": 5.0, "slow": 20.0}
	multiRes, err := RunVectorizedBacktestMulti([]string{cleanCode}, "MA", params, days)
	if err != nil || len(multiRes.Items) == 0 {
		return QuantDimensionResult{
			Score:        50,
			StrategyName: "MA均线趋势跟踪 (5/20)",
			Evaluation:   "历史K线样本不足或数据异常，采用中性基准评分",
		}
	}

	item := multiRes.Items[0]
	score := 50

	// 胜率加权 (40%)
	if item.WinRatePct >= 65 {
		score += 25
	} else if item.WinRatePct >= 50 {
		score += 15
	} else if item.WinRatePct < 40 {
		score -= 15
	}

	// 收益率加权 (30%)
	if item.TotalReturnPct >= 20 {
		score += 15
	} else if item.TotalReturnPct >= 0 {
		score += 5
	} else {
		score -= 10
	}

	// 夏普比率 (20%)
	if item.SharpeRatio >= 1.5 {
		score += 15
	} else if item.SharpeRatio >= 0.8 {
		score += 8
	} else if item.SharpeRatio < 0 {
		score -= 10
	}

	// 最大回撤扣分 (10%)
	if item.MaxDrawdownPct > 25 {
		score -= 10
	} else if item.MaxDrawdownPct < 15 {
		score += 5
	}

	if score > 98 {
		score = 98
	} else if score < 15 {
		score = 15
	}

	eval := fmt.Sprintf("历史回测胜率 %.1f%%，盈亏比 %.2f，夏普比率 %.2f",
		item.WinRatePct, item.ProfitFactor, item.SharpeRatio)
	if item.WinRatePct >= 60 && item.SharpeRatio >= 1.0 {
		eval += "，策略收益风险比优秀，具备明确统计套利优势。"
	} else if item.WinRatePct < 45 {
		eval += "，胜率偏低，历史假突破较多，建议严格把控仓位。"
	} else {
		eval += "，整体表现稳健，跟随趋势交易为宜。"
	}

	return QuantDimensionResult{
		Score:          score,
		StrategyName:   "MA趋势追踪(5/20) + 动态止损",
		WinRatePct:     math.Round(item.WinRatePct*10) / 10,
		TotalReturnPct: math.Round(item.TotalReturnPct*10) / 10,
		SharpeRatio:    math.Round(item.SharpeRatio*100) / 100,
		MaxDrawdownPct: math.Round(item.MaxDrawdownPct*10) / 10,
		ProfitFactor:   math.Round(item.ProfitFactor*100) / 100,
		TotalTrades:    item.TotalTrades,
		EquityCurve:    item.EquityCurve,
		Evaluation:     eval,
	}
}

func runTechnicalDiagnosis(closeP, highP, lowP, volP []float64) TechnicalDimensionResult {
	n := len(closeP)
	currentPrice := closeP[n-1]

	calcMA := func(p []float64, period int) float64 {
		if len(p) < period {
			return p[len(p)-1]
		}
		sum := 0.0
		for i := len(p) - period; i < len(p); i++ {
			sum += p[i]
		}
		return math.Round((sum/float64(period))*100) / 100
	}

	ma5 := calcMA(closeP, 5)
	ma20 := calcMA(closeP, 20)
	ma60 := calcMA(closeP, 60)

	// 计算 ATR (14)
	vHigh := NewVector(highP)
	vLow := NewVector(lowP)
	vClose := NewVector(closeP)
	vATR := NewATR(vHigh, vLow, vClose, 14)
	atrVal := 0.0
	if vATR.Len > 0 {
		atrVal = math.Round(vATR.Values[vATR.Len-1]*100) / 100
	}

	priceDistMA20 := 0.0
	if ma20 > 0 {
		priceDistMA20 = math.Round(((currentPrice-ma20)/ma20)*1000) / 10
	}

	trend := "震荡整理"
	score := 50
	if currentPrice > ma5 && ma5 > ma20 && ma20 > ma60 {
		trend = "多头排列 (强势发散)"
		score = 85
	} else if currentPrice > ma20 && ma20 > ma60 {
		trend = "上升趋势 (良性回调)"
		score = 75
	} else if currentPrice < ma5 && ma5 < ma20 && ma20 < ma60 {
		trend = "空头排列 (弱势下行)"
		score = 25
	} else if currentPrice < ma20 && currentPrice < ma60 {
		trend = "弱势整理 (均线压制)"
		score = 35
	}

	// 乖离率过大惩罚（防追高）
	if priceDistMA20 > 15 {
		score -= 10
	}

	eval := fmt.Sprintf("当前价格 %.2f，MA5=%.2f，MA20=%.2f。处于%s，距20日线乖离率 %.1f%%",
		currentPrice, ma5, ma20, trend, priceDistMA20)

	return TechnicalDimensionResult{
		Score:             score,
		Trend:             trend,
		MA5:               ma5,
		MA20:              ma20,
		MA60:              ma60,
		ATR:               atrVal,
		PriceDistanceMA20: priceDistMA20,
		Evaluation:        eval,
	}
}

func runMoneyDiagnosis(cleanCode string) MoneyDimensionResult {
	hisData := data.NewStockDataApi().GetStockHistoryMoneyData(cleanCode)
	if len(hisData) == 0 {
		return MoneyDimensionResult{
			Score:             50,
			RecentInflowTrend: "平淡 / 无异动",
			Evaluation:        "近期资金流向数据平稳或未采集到大单异动",
		}
	}

	// 统计近 5 日主力净额与超大单/大单占比
	days := 5
	if len(hisData) < days {
		days = len(hisData)
	}

	totalMainInflow := 0.0
	lastSuperPct := 0.0
	lastLargePct := 0.0

	for i := 0; i < days; i++ {
		idx := len(hisData) - 1 - i
		val, _ := strconv.ParseFloat(hisData[idx].F62, 64)
		totalMainInflow += val
		if i == 0 {
			lastSuperPct, _ = strconv.ParseFloat(hisData[idx].F69, 64)
			lastLargePct, _ = strconv.ParseFloat(hisData[idx].F75, 64)
		}
	}

	// 转换为万元
	mainInflowWan := math.Round((totalMainInflow/10000)*10) / 10

	score := 50
	trend := "分歧震荡"

	if mainInflowWan > 1000 {
		score += 30
		trend = "主力持续大幅净买入"
	} else if mainInflowWan > 0 {
		score += 15
		trend = "主力温和净流入"
	} else if mainInflowWan < -1000 {
		score -= 30
		trend = "主力大幅净流出离场"
	} else {
		score -= 10
		trend = "主力资金小幅净流出"
	}

	if lastSuperPct > 5.0 {
		score += 10
	} else if lastSuperPct < -5.0 {
		score -= 10
	}

	if score > 95 {
		score = 95
	} else if score < 15 {
		score = 15
	}

	chip := "筹码中性"
	if score >= 75 {
		chip = "筹码明显向主力集中"
	} else if score <= 35 {
		chip = "筹码趋于分散，散户接盘"
	}

	eval := fmt.Sprintf("近5日主力累计净流入 %.1f 万元，最新超大单净占比 %.1f%%，%s",
		mainInflowWan, lastSuperPct, trend)

	return MoneyDimensionResult{
		Score:             score,
		MainNetInflow5D:   mainInflowWan,
		SuperLargeNetPct:  lastSuperPct,
		LargeNetPct:       lastLargePct,
		RecentInflowTrend: trend,
		ChipConcentration: chip,
		Evaluation:        eval,
	}
}

func buildDiagnosisSummary(name, code string, score int, suggestion string, q QuantDimensionResult, m MoneyDimensionResult, t TechnicalDimensionResult) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("【%s(%s) 深度诊断】综合得分 **%d/100**，操作建议：**%s**。\n\n", name, code, score, suggestion))
	b.WriteString(fmt.Sprintf("- 📊 **量化回测维度 (%d分)**：%s (胜率 %.1f%%, 夏普 %.2f)\n", q.Score, q.Evaluation, q.WinRatePct, q.SharpeRatio))
	b.WriteString(fmt.Sprintf("- 💰 **资金筹码维度 (%d分)**：%s，%s\n", m.Score, m.Evaluation, m.ChipConcentration))
	b.WriteString(fmt.Sprintf("- 📈 **技术形态维度 (%d分)**：%s\n", t.Score, t.Evaluation))
	return b.String()
}

func buildDiagnosisRisks(currentPrice float64, t TechnicalDimensionResult, m MoneyDimensionResult, q QuantDimensionResult) []string {
	var risks []string
	if t.PriceDistanceMA20 > 12 {
		risks = append(risks, fmt.Sprintf("短期乖离率达到 %.1f%%，注意技术指标超买引发的高位剧烈洗盘", t.PriceDistanceMA20))
	}
	if m.MainNetInflow5D < 0 {
		risks = append(risks, fmt.Sprintf("近5日主力累计净流出 %.1f 万元，需警惕冲高时主力资金借机减持", math.Abs(m.MainNetInflow5D)))
	}
	if q.MaxDrawdownPct > 20 {
		risks = append(risks, fmt.Sprintf("该股历史波动剧烈，量化回测最大回撤达 %.1f%%，务必严格执行设定的止损价", q.MaxDrawdownPct))
	}
	if len(risks) == 0 {
		risks = append(risks, "未检测到显著异常风险，但需保持对大盘整体系统性行情的防范")
	}
	return risks
}
