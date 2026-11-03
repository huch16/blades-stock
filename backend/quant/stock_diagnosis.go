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
	TargetPriceNote  string                  `json:"targetPriceNote,omitempty"` // 目标价说明 (例如偏空建议防守时不设做多目标价)
	StopLossPrice    float64                 `json:"stopLossPrice"`    // 建议止损价
	RiskRewardRatio  float64                 `json:"riskRewardRatio"`  // 盈亏比
	WindowBars       int                     `json:"windowBars"`       // 诊断使用的K线样本根数
	StartDate        string                  `json:"startDate"`        // 样本起始日期
	EndDate          string                  `json:"endDate"`          // 样本截止日期
	DataWindowDesc   string                  `json:"dataWindowDesc"`   // 样本时间窗口说明
	DataSource       string                  `json:"dataSource"`       // 原始数据来源
	Adjust           string                  `json:"adjust"`           // 复权类型 (如 qfq)
	CodeVersion      string                  `json:"codeVersion"`      // 计算逻辑版本
	ComputedAt       string                  `json:"computedAt"`       // 确定性计算时间戳
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
	HasData            bool    `json:"hasData"`            // 是否采集到真实有效的资金流向数据
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
	closeP, highP, lowP, openP, volP, dates, err := loader.LoadKLineData(cleanCode, 250)
	if err != nil || len(closeP) < 20 {
		return nil, fmt.Errorf("获取K线数据失败或数据不足: %v", err)
	}

	// 统一严格截取确定性 250 根（约 1 个标准交易年度），保证不可漂移、100% 幂等可复现
	if len(closeP) > 250 {
		startIdx := len(closeP) - 250
		closeP = closeP[startIdx:]
		highP = highP[startIdx:]
		lowP = lowP[startIdx:]
		openP = openP[startIdx:]
		volP = volP[startIdx:]
		dates = dates[startIdx:]
	}

	n := len(closeP)
	currentPrice := closeP[n-1]
	startDate := dates[0]
	endDate := dates[n-1]
	dataWindowDesc := fmt.Sprintf("基于 %d 根日K线样本 (%s ~ %s) 确定性回测", n, startDate, endDate)

	// 1. 量化回测维度 (直接基于同一份确定性 K 线切片运行 VectorBT，彻底消除 365 vs 500 漂移事故)
	quantRes := runQuantDiagnosisOnData(closeP, highP, lowP, openP, volP, dates)

	// 2. 技术形态维度
	techRes := runTechnicalDiagnosis(closeP, highP, lowP, volP)

	// 3. 资金筹码维度
	moneyRes := runMoneyDiagnosis(cleanCode)

	// 4. 结合 ATR 与波动率计算建议止损价与动态目标价
	atrVal := techRes.ATR
	if atrVal <= 0 {
		atrVal = currentPrice * 0.03 // 默认 3% 波动
	}

	// 5. 综合评分计算：
	// 若资金流向数据有效：量化40% + 资金35% + 技术25%
	// 若资金流向数据缺失：动态降级重新加权（量化60% + 技术40%），绝不平白赠送 50 分中性分抬高垃圾股评分！
	overallScore := 0
	if moneyRes.HasData {
		overallScore = int(math.Round(float64(quantRes.Score)*0.40 + float64(moneyRes.Score)*0.35 + float64(techRes.Score)*0.25))
	} else {
		overallScore = int(math.Round(float64(quantRes.Score)*0.60 + float64(techRes.Score)*0.40))
	}
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

	targetPrice := 0.0
	stopLoss := 0.0
	riskReward := 0.0
	targetNote := ""

	// 7. 严密量化约束：若综合评分偏空（<50）或评级为减持/规避，坚决不设做多目标价！杜绝一边建议卖出一边显示+18%目标价的荒谬自相矛盾！
	if overallScore >= 50 {
		targetGain := math.Max(math.Abs(quantRes.TotalReturnPct/100)*currentPrice, 1.8*atrVal)
		targetPrice = math.Round((currentPrice+targetGain)*100) / 100
		stopLoss = math.Round((currentPrice-1.2*atrVal)*100) / 100
		lossDiff := currentPrice - stopLoss
		gainDiff := targetPrice - currentPrice
		if lossDiff > 0 {
			riskReward = math.Round((gainDiff/lossDiff)*100) / 100
		}
		targetNote = fmt.Sprintf("基于 ATR(%.2f) 与策略期望收益动态测算第一做多阻力位", atrVal)
	} else {
		targetPrice = 0
		stopLoss = math.Round((currentPrice-1.0*atrVal)*100) / 100
		riskReward = 0
		targetNote = "当前量化评级偏空，以风控防守为主，不设做多目标价"
	}

	confidence := quantRes.WinRatePct
	if confidence <= 0 {
		confidence = float64(overallScore)
	}

	// 8. 提取历史 AI 在该股上的真实战绩（自反思闭环）
	btMgr := NewBacktestManager()
	var historyTrack string
	if stats := btMgr.GetStockStats(cleanCode); stats != nil && stats.VerifiedCount > 0 {
		historyTrack = fmt.Sprintf("AI历史预测该股 %d 次，实盘验证胜率 %.1f%% (准确 %d 次)",
			stats.VerifiedCount, stats.AccuracyRate*100, stats.AccurateCount)
	} else {
		historyTrack = "暂无该标的历史闭环回测样本"
	}

	// 9. 提炼核心结论与风险提示
	summary := buildDiagnosisSummary(stockName, cleanCode, overallScore, suggestion, quantRes, moneyRes, techRes)
	if !moneyRes.HasData {
		summary += "（资金维度数据暂缺，已采用量化60%+技术40%动态降级加权）"
	}
	risks := buildDiagnosisRisks(currentPrice, techRes, moneyRes, quantRes)

	return &StockDiagnosisResult{
		StockCode:          cleanCode,
		StockName:          stockName,
		OverallScore:       overallScore,
		Suggestion:         suggestion,
		Confidence:         math.Round(confidence*10) / 10,
		CurrentPrice:       currentPrice,
		TargetPrice:        targetPrice,
		TargetPriceNote:    targetNote,
		StopLossPrice:      stopLoss,
		RiskRewardRatio:    riskReward,
		WindowBars:         n,
		StartDate:          startDate,
		EndDate:            endDate,
		DataWindowDesc:     dataWindowDesc,
		DataSource:         "EastMoney / Sina / Tushare (QFQ)",
		Adjust:             "qfq",
		CodeVersion:        "v2.0.0",
		ComputedAt:         time.Now().Format("2006-01-02 15:04:05"),
		QuantDim:           quantRes,
		MoneyDim:           moneyRes,
		TechnicalDim:       techRes,
		Summary:            summary,
		Risks:              risks,
		HistoryTrackRecord: historyTrack,
		LastUpdated:        time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

func runQuantDiagnosisOnData(closeP, highP, lowP, openP, volP []float64, dates []string) QuantDimensionResult {
	cfg := DefaultBacktestConfig()
	engine := NewBacktestEngine().
		Data(closeP, highP, lowP, openP, volP, dates).
		Config(cfg).
		Build()

	strategy := &MAStrategy{FastWindow: 5, SlowWindow: 20}
	res := engine.Run(strategy)
	if res == nil || res.Stats == nil {
		return QuantDimensionResult{
			Score:        50,
			StrategyName: "MA趋势跟踪 (5/20)",
			Evaluation:   "历史K线样本不足或数据异常，采用中性基准评分",
		}
	}

	winRatePct := res.Stats.WinRate * 100
	totalReturnPct := res.Stats.TotalReturn * 100
	sharpeRatio := res.Stats.SharpeRatio
	maxDrawdownPct := res.Stats.MaxDrawdownPct
	profitFactor := res.Stats.ProfitFactor
	totalTrades := res.Stats.TotalTrades

	score := 50

	// 胜率加权 (40%)
	if winRatePct >= 65 {
		score += 25
	} else if winRatePct >= 50 {
		score += 15
	} else if winRatePct < 40 {
		score -= 15
	}

	// 收益率加权 (30%)
	if totalReturnPct >= 20 {
		score += 15
	} else if totalReturnPct >= 0 {
		score += 5
	} else {
		score -= 10
	}

	// 夏普比率 (20%)
	if sharpeRatio >= 1.5 {
		score += 15
	} else if sharpeRatio >= 0.8 {
		score += 8
	} else if sharpeRatio < 0 {
		score -= 10
	}

	// 最大回撤扣分 (10%)
	if maxDrawdownPct > 25 {
		score -= 10
	} else if maxDrawdownPct < 15 {
		score += 5
	}

	if score > 98 {
		score = 98
	} else if score < 15 {
		score = 15
	}

	eval := fmt.Sprintf("历史回测胜率 %.1f%%，盈亏比 %.2f，夏普比率 %.2f",
		winRatePct, profitFactor, sharpeRatio)
	if winRatePct >= 60 && sharpeRatio >= 1.0 {
		eval += "，策略收益风险比优秀，具备明确统计套利优势。"
	} else if winRatePct < 45 {
		eval += "，胜率偏低，历史假突破较多，建议严格把控仓位。"
	} else {
		eval += "，整体表现稳健，跟随趋势交易为宜。"
	}
	if totalTrades < 5 {
		eval += fmt.Sprintf(" — ⚠️ 回测样本过少(仅 %d 次交易)，统计结论仅供参考", totalTrades)
	}

	trimmedEquity := trimEquityCurvePrefix(res.Equity)

	return QuantDimensionResult{
		Score:          score,
		StrategyName:   "MA趋势追踪(5/20) + 动态止损",
		WinRatePct:     math.Round(winRatePct*10) / 10,
		TotalReturnPct: math.Round(totalReturnPct*10) / 10,
		SharpeRatio:    math.Round(sharpeRatio*100) / 100,
		MaxDrawdownPct: math.Round(maxDrawdownPct*10) / 10,
		ProfitFactor:   math.Round(profitFactor*100) / 100,
		TotalTrades:    totalTrades,
		EquityCurve:    trimmedEquity,
		Evaluation:     eval,
	}
}

// trimEquityCurvePrefix 去除回测前期无交易时的平坦合成初始资金前缀
func trimEquityCurvePrefix(equity []float64) []float64 {
	if len(equity) <= 2 {
		return equity
	}
	initialVal := equity[0]
	firstChangeIdx := 0
	for i := 1; i < len(equity); i++ {
		if math.Abs(equity[i]-initialVal) > 1e-6 {
			// 保留变动前的一根作为基准原点
			firstChangeIdx = i - 1
			break
		}
	}
	if firstChangeIdx > 0 && firstChangeIdx < len(equity) {
		return equity[firstChangeIdx:]
	}
	return equity
}

func runQuantDiagnosis(cleanCode string, days int) QuantDimensionResult {
	loader := NewStockDataLoader()
	closeP, highP, lowP, openP, volP, dates, err := loader.LoadKLineData(cleanCode, days)
	if err != nil || len(closeP) < 20 {
		return QuantDimensionResult{
			Score:        0,
			StrategyName: "MA均线趋势跟踪 (5/20)",
			Evaluation:   "历史K线样本不足(<20根)或数据源异常，无法进行量化回测",
		}
	}
	return runQuantDiagnosisOnData(closeP, highP, lowP, openP, volP, dates)
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
			HasData:           false,
			Score:             0,
			RecentInflowTrend: "平淡 / 暂无资金数据",
			Evaluation:        "近期主力大单资金流向数据暂缺，该维度不参与加权评分",
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
		HasData:           true,
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
