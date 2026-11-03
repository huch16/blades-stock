package quant

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"

	"blades-stock/backend/data"
	"blades-stock/backend/models"
)

// ETFMomentumScoreResult ETF 21日加权动量计算结果 (纯数学动量矩阵)
type ETFMomentumScoreResult struct {
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	Close        float64 `json:"close"`
	Annualized   float64 `json:"annualized"`   // 年化收益率 (基于斜率)
	R2           float64 `json:"r2"`           // 线性回归拟合优度 R^2
	Score        float64 `json:"score"`        // 动量综合得分 = Annualized * R2
	Pullback5D   float64 `json:"pullback_5d"`  // 近5日高点回撤百分比
	AboveMA5     bool    `json:"above_ma5"`    // 现价是否收复 MA5
	CooldownGate bool    `json:"cooldown_gate"`// 是否触发回撤冷却拦截 (近5日回撤 >= 5% 且未收复 MA5)
	Rank         int     `json:"rank"`
}

// CalculateETFMomentum 计算单个标的的21日加权对数动量与回撤指标
// 严格对齐 multi-agents-etf-trade-strategy 的数学公式：
// y = log(close[-21:])
// weights = linspace(1, 2, 21)
// slope, intercept = polyfit(x, y, 1, w=weights)
// annualized = exp(slope * 250) - 1
// R^2 = 1 - sum(w * (y - y_hat)^2) / sum(w * (y - y_bar)^2)
// score = annualized * R^2
func CalculateETFMomentum(code, name string, closes []float64) (*ETFMomentumScoreResult, error) {
	n := len(closes)
	if n < 21 {
		return nil, fmt.Errorf("数据不足21天 (实际 %d 天)", n)
	}

	window := closes[n-21:]
	currentClose := window[20]

	// 1. 近 5 日高点回撤与 MA5
	max5D := window[16]
	sum5D := 0.0
	for i := 16; i <= 20; i++ {
		if window[i] > max5D {
			max5D = window[i]
		}
		sum5D += window[i]
	}
	ma5 := sum5D / 5.0
	pullback5D := 0.0
	if max5D > 0 {
		pullback5D = (max5D - currentClose) / max5D * 100.0
	}
	aboveMA5 := currentClose >= ma5
	// 回撤冷却闸门：5日回撤 >= 5% 且 未站上 MA5
	cooldownGate := pullback5D >= 5.0 && !aboveMA5

	// 2. 21日加权线性回归 (对数收盘价)
	y := make([]float64, 21)
	w := make([]float64, 21)
	sumW := 0.0
	sumWY := 0.0
	for i := 0; i < 21; i++ {
		if window[i] <= 0 {
			return nil, fmt.Errorf("收盘价存在非正数: %f", window[i])
		}
		y[i] = math.Log(window[i])
		// weights = linspace(1, 2, 21)
		w[i] = 1.0 + float64(i)*(1.0/20.0)
		sumW += w[i]
		sumWY += w[i] * y[i]
	}
	yBar := sumWY / sumW

	// 加权最小二乘法求斜率 slope
	// x = [0..20]
	sumWX := 0.0
	for i := 0; i < 21; i++ {
		sumWX += w[i] * float64(i)
	}
	xBar := sumWX / sumW

	var num, den float64
	for i := 0; i < 21; i++ {
		xi := float64(i)
		num += w[i] * (xi - xBar) * (y[i] - yBar)
		den += w[i] * math.Pow(xi-xBar, 2)
	}
	if den == 0 {
		return nil, fmt.Errorf("分母为0，回归计算失败")
	}
	slope := num / den
	intercept := yBar - slope*xBar

	// 年化收益率 = exp(slope * 250) - 1 (防范极端斜率导致 Exp 溢出为 +Inf / NaN)
	annualized := 0.0
	slopeExponent := slope * 250.0
	if slopeExponent > 20.0 { // exp(20) 约 4.85亿，超过此值强制钳制防范数值爆炸
		annualized = 50.0 // 限制年化收益上限为 +5000%
	} else if slopeExponent < -20.0 {
		annualized = -1.0
	} else {
		annualized = math.Exp(slopeExponent) - 1.0
	}
	if math.IsNaN(annualized) || math.IsInf(annualized, 0) {
		annualized = 0.0
	}

	// 3. 计算加权 R^2
	var ssRes, ssTot float64
	for i := 0; i < 21; i++ {
		xi := float64(i)
		yHat := slope*xi + intercept
		ssRes += w[i] * math.Pow(y[i]-yHat, 2)
		ssTot += w[i] * math.Pow(y[i]-yBar, 2)
	}

	r2 := 0.0
	if ssTot > 1e-12 {
		r2 = 1.0 - (ssRes / ssTot)
	}
	if math.IsNaN(r2) || math.IsInf(r2, 0) || r2 < 0 {
		r2 = 0
	} else if r2 > 1.0 {
		r2 = 1.0
	}

	score := annualized * r2
	if math.IsNaN(score) || math.IsInf(score, 0) {
		score = 0.0
	}
	if math.IsNaN(pullback5D) || math.IsInf(pullback5D, 0) {
		pullback5D = 0.0
	}

	return &ETFMomentumScoreResult{
		Code:         code,
		Name:         name,
		Close:        math.Round(currentClose*1000) / 1000,
		Annualized:   math.Round(annualized*10000) / 100, // 百分比
		R2:           math.Round(r2*1000) / 1000,
		Score:        math.Round(score*1000) / 1000,
		Pullback5D:   math.Round(pullback5D*100) / 100,
		AboveMA5:     aboveMA5,
		CooldownGate: cooldownGate,
	}, nil
}

// BatchCalculateETFMomentum 并发批量拉取核心 ETF 池历史K线，计算纯数学动量排名与硬风控指标
func BatchCalculateETFMomentum(codes []string) ([]*ETFMomentumScoreResult, error) {
	loader := NewStockDataLoader()
	results := make([]*ETFMomentumScoreResult, 0, len(codes))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, code := range codes {
		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			clean := models.CleanStockCode(c)
			closes, _, _, _, _, _, err := loader.LoadKLineData(clean, 40)
			if err != nil || len(closes) < 21 {
				return
			}

			name := c
			quotes, err := data.NewStockDataApi().GetStockCodeRealTimeData(c)
			if err == nil && quotes != nil && len(*quotes) > 0 {
				name = (*quotes)[0].Name
			}

			res, err := CalculateETFMomentum(c, name, closes)
			if err == nil && res != nil {
				mu.Lock()
				results = append(results, res)
				mu.Unlock()
			}
		}(code)
	}
	wg.Wait()

	// 按动量 Score 降序排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	for i := range results {
		results[i].Rank = i + 1
	}

	return results, nil
}

// FormatETFMomentumSummary 将纯 Go 计算出的真实动量矩阵格式化为硬核 Markdown 表格，注入 Agent
func FormatETFMomentumSummary(list []*ETFMomentumScoreResult) string {
	if len(list) == 0 {
		return "（未能获取到足够K线数据计算动量）\n"
	}

	var sb strings.Builder
	sb.WriteString("| 排名 | ETF名称 (代码) | 现价 | 年化动量(%) | 线性R² | 动量得分 | 5日回撤 | MA5状态 | 风控状态 |\n")
	sb.WriteString("| :---: | :--- | :---: | :---: | :---: | :---: | :---: | :---: | :--- |\n")
	for _, item := range list {
		status := "🟢 正常"
		if item.CooldownGate {
			status = "🚨 回撤冷却拦截(禁止新开仓)"
		} else if !item.AboveMA5 {
			status = "⚠️ MA5下方(警惕)"
		}
		ma5Str := "已站上"
		if !item.AboveMA5 {
			ma5Str = "失守"
		}
		sb.WriteString(fmt.Sprintf("| %d | %s (%s) | %.3f | %+.2f%% | %.3f | **%.3f** | %.2f%% | %s | %s |\n",
			item.Rank, item.Name, item.Code, item.Close, item.Annualized, item.R2, item.Score, item.Pullback5D, ma5Str, status))
	}
	return sb.String()
}

// ETFRegimeStatus 510300 沪深300ETF 硬风控大盘状态机 (严格对齐 stevia0829 JoinQuant 原作规范)
type ETFRegimeStatus struct {
	BenchmarkCode   string  `json:"benchmarkCode"`   // "510300.SH"
	BenchmarkName   string  `json:"benchmarkName"`   // "沪深300ETF"
	CurrentPrice    float64 `json:"currentPrice"`    // 现价
	MA120           float64 `json:"ma120"`           // 120日均线
	Peak60D         float64 `json:"peak60d"`         // 过去60日最高价
	Drawdown60D     float64 `json:"drawdown60d"`     // 过去60日从高点回撤百分比
	State           string  `json:"state"`           // "OFFENSE" 或 "DEFENSE"
	StateDesc       string  `json:"stateDesc"`       // 中文状态说明
	Reason          string  `json:"reason"`          // 判定原因
	SuggestedAction string  `json:"suggestedAction"` // 建议操作
}

// ETFPreOpenCheckResult 09:22 集合竞价跳空复核
type ETFPreOpenCheckResult struct {
	TargetCode    string  `json:"targetCode"`
	TargetName    string  `json:"targetName"`
	PreClose      float64 `json:"preClose"`
	OpenPrice     float64 `json:"openPrice"`
	AuctionGapPct float64 `json:"auctionGapPct"` // 开盘跳空幅度 (%)
	Status        string  `json:"status"`        // "NORMAL", "HIGH_SURGE", "LOW_DUMP"
	Warning       string  `json:"warning"`
}

// CalculateETFRegimeStatus 计算 510300 沪深300ETF 120日均线与 60日回撤硬风控状态机
// stevia0829 铁律：
// 1. 沪深300ETF 处于 120 日均线下方 ➔ 强制 DEFENSE
// 2. 沪深300ETF 过去 60 日内从最高价回撤 >= 8.0% ➔ 强制 DEFENSE
// 3. 只有两者同时满足安全底线（Close >= MA120 且 回撤 < 8.0%）➔ 允许 OFFENSE
func CalculateETFRegimeStatus() (*ETFRegimeStatus, error) {
	loader := NewStockDataLoader()
	cleanCode := models.CleanStockCode("510300.SH")
	closes, highs, _, _, _, _, err := loader.LoadKLineData(cleanCode, 130)
	if err != nil || len(closes) < 60 {
		return &ETFRegimeStatus{
			BenchmarkCode:   "510300.SH",
			BenchmarkName:   "沪深300ETF",
			State:           "DEFENSE",
			StateDesc:       "🛡️ 绝对防守 (数据不足兜底)",
			Reason:          "无法获取足够的 510300 历史K线，触发安全防守兜底",
			SuggestedAction: "保持轻仓或持有货币ETF (511880/511990) 防守",
		}, nil
	}

	n := len(closes)
	currentPrice := closes[n-1]

	// 1. 计算 MA120 (若样本不足120，使用当前全量均值)
	maPeriod := 120
	if n < maPeriod {
		maPeriod = n
	}
	sumMA := 0.0
	for i := n - maPeriod; i < n; i++ {
		sumMA += closes[i]
	}
	ma120 := math.Round((sumMA/float64(maPeriod))*1000) / 1000

	// 2. 计算 60 日高点与当前回撤
	lookback60 := 60
	if n < lookback60 {
		lookback60 = n
	}
	peak60 := highs[n-lookback60]
	for i := n - lookback60; i < n; i++ {
		if highs[i] > peak60 {
			peak60 = highs[i]
		}
	}
	drawdown60 := 0.0
	if peak60 > 0 {
		drawdown60 = math.Round(((peak60-currentPrice)/peak60)*10000) / 100
	}

	state := "OFFENSE"
	stateDesc := "🚀 允许动量进攻 (OFFENSE)"
	reason := fmt.Sprintf("沪深300ETF 现价(%.3f) >= MA120(%.3f)，且60日回撤(%.2f%%) < 8.0%%，处于健康右侧上升通道",
		currentPrice, ma120, drawdown60)
	action := "大盘环境安全，允许按 21 日加权动量排名买入第一或前二优势 ETF"

	if currentPrice < ma120 {
		state = "DEFENSE"
		stateDesc = "🛡️ 绝对防守避险 (DEFENSE)"
		reason = fmt.Sprintf("沪深300ETF 现价(%.3f) < MA120(%.3f)，跌破中长线牛熊生命线，触发硬风控大盘熔断",
			currentPrice, ma120)
		action = "全市场做多进攻一票否决！严禁开仓进攻型行业ETF，强制持有货币ETF (511880银华日利/511990华宝添益) 或空仓持币"
	} else if drawdown60 >= 8.0 {
		state = "DEFENSE"
		stateDesc = "🛡️ 绝对防守避险 (DEFENSE)"
		reason = fmt.Sprintf("沪深300ETF 过去60日从高点(%.3f)回撤达到 %.2f%% (>= 8.0%%)，触发回撤熔断闸门",
			peak60, drawdown60)
		action = "全市场做多进攻一票否决！防范大盘深幅破位，强制持有货币ETF或空仓观望"
	}

	return &ETFRegimeStatus{
		BenchmarkCode:   "510300.SH",
		BenchmarkName:   "沪深300ETF",
		CurrentPrice:    currentPrice,
		MA120:           ma120,
		Peak60D:         peak60,
		Drawdown60D:     drawdown60,
		State:           state,
		StateDesc:       stateDesc,
		Reason:          reason,
		SuggestedAction: action,
	}, nil
}

// CheckETFPreOpenGap 执行 09:22 集合竞价跳空复核
func CheckETFPreOpenGap(code string) (*ETFPreOpenCheckResult, error) {
	quotes, err := data.NewStockDataApi().GetStockCodeRealTimeData(code)
	if err != nil || quotes == nil || len(*quotes) == 0 {
		return &ETFPreOpenCheckResult{
			TargetCode: code,
			TargetName: code,
			Status:     "NORMAL",
			Warning:    "暂无集合竞价实时报价，沿用前收盘价执行",
		}, nil
	}

	q := (*quotes)[0]
	preClose, _ := strconv.ParseFloat(q.PreClose, 64)
	openPrice, _ := strconv.ParseFloat(q.Price, 64)
	if openPrice <= 0 || preClose <= 0 {
		return &ETFPreOpenCheckResult{
			TargetCode: code,
			TargetName: q.Name,
			PreClose:   preClose,
			OpenPrice:  openPrice,
			Status:     "NORMAL",
			Warning:    "未取得有效竞价价格",
		}, nil
	}

	gapPct := math.Round(((openPrice-preClose)/preClose)*10000) / 100
	status := "NORMAL"
	warning := fmt.Sprintf("集合竞价波动平稳 (%+.2f%%)，在安全区间 [-3.0%%, +2.5%%] 内，可正常执行开盘计划", gapPct)

	if gapPct > 2.5 {
		status = "HIGH_SURGE"
		warning = fmt.Sprintf("⚠️ 集合竞价大幅跳空高开 (%+.2f%% > +2.5%%)，警惕高开诱多与盘中回落，严禁追高开仓！建议等待盘中回踩确认", gapPct)
	} else if gapPct < -3.0 {
		status = "LOW_DUMP"
		warning = fmt.Sprintf("🚨 集合竞价大幅跳空低开 (%+.2f%% < -3.0%%)，警惕动量失速与破位踩踏！若持有该ETF建议触发止损或暂缓建仓", gapPct)
	}

	return &ETFPreOpenCheckResult{
		TargetCode:    code,
		TargetName:    q.Name,
		PreClose:      preClose,
		OpenPrice:     openPrice,
		AuctionGapPct: gapPct,
		Status:        status,
		Warning:       warning,
	}, nil
}

// FormatETFRegimeSummary 将 510300 硬风控状态机与 09:22 集合竞价复核格式化为硬核 Markdown 报告
func FormatETFRegimeSummary(regime *ETFRegimeStatus, preOpen *ETFPreOpenCheckResult) string {
	var sb strings.Builder
	sb.WriteString("### 🛡️ 510300 沪深300ETF 大盘硬风控状态机 (stevia0829 原生规范)\n\n")
	sb.WriteString(fmt.Sprintf("- **当前状态**: **%s**\n", regime.StateDesc))
	sb.WriteString(fmt.Sprintf("- **基准价格**: 现价 %.3f | 120日牛熊生命线(MA120): %.3f\n", regime.CurrentPrice, regime.MA120))
	sb.WriteString(fmt.Sprintf("- **60日高点回撤**: 60日峰值 %.3f | 当前回撤 **%.2f%%** (风控阈值: 8.0%%)\n", regime.Peak60D, regime.Drawdown60D))
	sb.WriteString(fmt.Sprintf("- **判定原因**: %s\n", regime.Reason))
	sb.WriteString(fmt.Sprintf("- **风控执行建议**: **%s**\n\n", regime.SuggestedAction))

	if preOpen != nil {
		sb.WriteString("### ⏰ 09:22 集合竞价跳空复核 (Auction Gate)\n\n")
		sb.WriteString(fmt.Sprintf("- **复核标的**: %s (%s)\n", preOpen.TargetName, preOpen.TargetCode))
		sb.WriteString(fmt.Sprintf("- **跳空幅度**: 昨收 %.3f ➔ 现价/竞价 %.3f (跳空 **%+.2f%%**)\n", preOpen.PreClose, preOpen.OpenPrice, preOpen.AuctionGapPct))
		sb.WriteString(fmt.Sprintf("- **竞价结论**: %s\n\n", preOpen.Warning))
	}
	return sb.String()
}
