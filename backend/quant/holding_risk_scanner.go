package quant

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/models"
)

// HoldingScanItem 待扫描的持仓或自选标的输入
type HoldingScanItem struct {
	StockCode     string  `json:"stockCode"`
	StockName     string  `json:"stockName"`
	CurrentPrice  float64 `json:"currentPrice"`
	ChangePercent float64 `json:"changePercent"`
	CostPrice     float64 `json:"costPrice,omitempty"`
	CostVolume    float64 `json:"costVolume,omitempty"`
	ProfitRate    float64 `json:"profitRate,omitempty"`
}

// HoldingRiskItem 单只标的的确定性风控计算明细
type HoldingRiskItem struct {
	StockCode       string  `json:"stockCode"`
	StockName       string  `json:"stockName"`
	CurrentPrice    float64 `json:"currentPrice"`
	ChangePercent   float64 `json:"changePercent"`
	CostPrice       float64 `json:"costPrice,omitempty"`
	CostVolume      float64 `json:"costVolume,omitempty"`
	ProfitRate      float64 `json:"profitRate,omitempty"`
	MA5             float64 `json:"ma5"`
	MA10            float64 `json:"ma10"`
	MA20            float64 `json:"ma20"`
	VolumeRatio     float64 `json:"volumeRatio"`     // 量比
	VolumeDesc      string  `json:"volumeDesc"`      // "缩量", "平量", "放量"
	Alignment       string  `json:"alignment"`       // "多头排列", "空头排列", "均线纠缠"
	DistanceToMA20  float64 `json:"distanceToMA20"`  // (Price - MA20) / MA20 * 100%
	ConsecutiveDays int     `json:"consecutiveDays"` // 连涨或连跌天数
	RecentReturn5D  float64 `json:"recentReturn5D"`  // 5日累计涨跌幅 (%)
	RiskTier        string  `json:"riskTier"`        // "GREEN", "YELLOW", "RED"
	TierPrefix      string  `json:"tierPrefix"`      // "🟢", "🟡", "🔴"
	TierTitle       string  `json:"tierTitle"`       // e.g. "🟢【多头持有】", "🟡【待放量确认】", "🔴【禁止新增仓】"
	DefensePrice    float64 `json:"defensePrice"`    // 关键防守位 (动态计算，如 MA20)
	ResistancePrice float64 `json:"resistancePrice"` // 上方阻力位
	ActionAdvice    string  `json:"actionAdvice"`    // 确定性实操策略
	HasRiskLock     bool    `json:"hasRiskLock"`     // 是否命中跨报告硬风控锁定
	RiskLockReason  string  `json:"riskLockReason"`  // 硬风控锁定原因
}

// HoldingRiskScanResult 盘前持仓风控扫描聚合结果
type HoldingRiskScanResult struct {
	GeneratedAt    string            `json:"generatedAt"`
	DataDate       string            `json:"dataDate"`
	DataSource     string            `json:"dataSource"`
	RegimeState    string            `json:"regimeState"` // "OFFENSE" 或 "DEFENSE"
	TotalCount     int               `json:"totalCount"`
	GreenCount     int               `json:"greenCount"`
	YellowCount    int               `json:"yellowCount"`
	RedCount       int               `json:"redCount"`
	Items          []HoldingRiskItem `json:"items"`
	FocusStocks    []string          `json:"focusStocks"`   // 盯盘主线
	RedLineStocks  []string          `json:"redLineStocks"` // 绝对红线
	MarkdownReport string            `json:"markdownReport"`
}

// ScanHoldingRisk 对输入的自选/持仓标的执行确定性多维度风控扫描
// 严格践行零 Token 确定性规则：指标由纯 Go 计算，禁止 LLM 幻觉判定！
func ScanHoldingRisk(items []HoldingScanItem) (*HoldingRiskScanResult, error) {
	if len(items) == 0 {
		return &HoldingRiskScanResult{
			GeneratedAt:    time.Now().Format("2006-01-02 15:04:05"),
			DataSource:     "东方财富/通达信前复权(QFQ)日K",
			MarkdownReport: "自选观察池无标的，请先添加自选股。",
		}, nil
	}

	// 1. 跨报告大盘宏观状态机判定 (stevia0829 核心风控门禁)
	regimeState := "DEFENSE"
	if reg, err := CalculateETFRegimeStatus(); err == nil && reg != nil {
		regimeState = reg.State
	}

	thesisApi := data.NewStockThesisApi()
	var processedItems []HoldingRiskItem
	latestDataDate := ""

	for _, it := range items {
		cleanCode := models.CleanStockCode(it.StockCode)
		if cleanCode == "" {
			continue
		}

		// 获取 60 根日K数据 (带多级降级链与缓存)
		kres := data.FetchKLineWithFallback(cleanCode, it.StockName, "101", 60, "")
		if kres == nil || kres.Data == nil || len(*kres.Data) < 20 {
			// 数据不足兜底
			processedItems = append(processedItems, fallbackItem(it))
			continue
		}

		klist := *kres.Data
		n := len(klist)
		lastBar := klist[n-1]
		if latestDataDate == "" || lastBar.Day > latestDataDate {
			latestDataDate = lastBar.Day
		}

		// 解析最新收盘价与量价序列
		closes := make([]float64, n)
		volumes := make([]float64, n)
		for i, bar := range klist {
			c, _ := strconv.ParseFloat(bar.Close, 64)
			v, _ := strconv.ParseFloat(bar.Volume, 64)
			closes[i] = c
			volumes[i] = v
		}

		curPrice := closes[n-1]
		if it.CurrentPrice > 0 {
			curPrice = it.CurrentPrice
		}

		// 计算均线 MA5, MA10, MA20
		ma5 := calcPeriodSMA(closes, 5)
		ma10 := calcPeriodSMA(closes, 10)
		ma20 := calcPeriodSMA(closes, 20)

		// 计算量比 (当日成交量 / 过去5日平均成交量)
		volRatio := 1.0
		if lastBar.VolumeRatio != "" {
			if vr, err := strconv.ParseFloat(lastBar.VolumeRatio, 64); err == nil && vr > 0 {
				volRatio = vr
			}
		} else if n >= 6 {
			sumPastVol := 0.0
			for j := n - 6; j < n-1; j++ {
				sumPastVol += volumes[j]
			}
			avgPastVol := sumPastVol / 5.0
			if avgPastVol > 0 {
				volRatio = volumes[n-1] / avgPastVol
			}
		}

		volDesc := "平量"
		if volRatio > 1.25 {
			volDesc = "放量"
		} else if volRatio < 0.75 {
			volDesc = "缩量"
		}

		// 均线排列判断
		alignment := "均线纠缠"
		if ma5 > ma10 && ma10 > ma20 {
			alignment = "多头排列"
		} else if ma5 < ma10 && ma10 < ma20 {
			alignment = "空头排列"
		}

		// 距 MA20 偏离度
		distMA20 := 0.0
		if ma20 > 0 {
			distMA20 = (curPrice - ma20) / ma20 * 100.0
		}

		// 5日区间累计涨跌幅与连涨连跌统计
		return5D := 0.0
		if n >= 6 && closes[n-6] > 0 {
			return5D = (closes[n-1] - closes[n-6]) / closes[n-6] * 100.0
		}

		consecDays := 0
		if n >= 2 {
			isUp := closes[n-1] >= closes[n-2]
			for k := n - 1; k >= 1; k-- {
				if (closes[k] >= closes[k-1]) == isUp {
					consecDays++
				} else {
					break
				}
			}
		}

		// 2. 跨报告结论一致性校验 (硬风控门禁与回撤冷却拦截)
		hasRiskLock := false
		riskLockReason := ""

		// (1) 宏观大盘 DEFENSE 状态下对权益类 ETF 的硬风控限制
		isEquityETF := strings.HasPrefix(cleanCode, "51") || strings.HasPrefix(cleanCode, "15") || strings.HasPrefix(cleanCode, "58")
		if isEquityETF && regimeState == "DEFENSE" && (distMA20 < -5.0 || return5D < -5.0) {
			hasRiskLock = true
			riskLockReason = "大盘宏观处于DEFENSE绝对防守状态，标的破位处于回撤冷却期"
		}

		// (2) 5日重挫破位硬风控拦截
		if return5D <= -8.0 && curPrice < ma5 && curPrice < ma20 {
			hasRiskLock = true
			riskLockReason = fmt.Sprintf("5日重挫%.1f%%破位，触发回撤冷却拦截", return5D)
		}

		// (3) 买方认知账本 (Thesis Ledger) 历史判定继承
		if thesis, err := thesisApi.GetStockThesis(cleanCode); err == nil && thesis != nil && thesis.ID > 0 {
			if thesis.Verdict == "AVOID" || strings.Contains(thesis.VerdictText, "回避") || strings.Contains(thesis.VerdictText, "止损") || strings.Contains(thesis.VerdictText, "冷却") {
				hasRiskLock = true
				if riskLockReason == "" {
					riskLockReason = "买方认知账本前期判定为【" + thesis.VerdictText + "】，处于风险防范期"
				}
			}
		}

		// 3. 确定性分级矩阵 (代码决定，禁止 LLM 篡改结论)
		riskTier := "YELLOW"
		tierPrefix := "🟡"
		tierTitle := ""
		defensePrice := ma20
		resistancePrice := ma20
		actionAdvice := ""

		if hasRiskLock {
			// 触发硬风控锁定，强制进入 🔴 拦截档
			riskTier = "RED"
			tierPrefix = "🔴"
			tierTitle = "🔴【禁止新增仓】"
			defensePrice = ma5
			resistancePrice = ma20
			actionAdvice = fmt.Sprintf("MA5<MA10<MA20空头排列，价%.3f仍破MA5、较MA20低%.1f%%，5日%.1f%%。趋势未修复，禁止新增。",
				curPrice, math.Abs(distMA20), return5D)
		} else if alignment == "空头排列" && curPrice < ma20 && (distMA20 < -1.0 || return5D < -5.0) {
			// 典型空头破位
			riskTier = "RED"
			tierPrefix = "🔴"
			tierTitle = "🔴【空头禁抄底】"
			defensePrice = ma20
			resistancePrice = ma20
			actionAdvice = fmt.Sprintf("MA5<MA10<MA20空头排列，MA20=%.2f压制，近期弱势。不抄底，反弹%.2f-%.2f减仓。",
				ma20, ma10, ma20)
		} else if it.CostPrice > 0 && it.ProfitRate < -7.0 && curPrice < ma20 {
			// 真实持仓严重浮亏
			riskTier = "RED"
			tierPrefix = "🔴"
			tierTitle = "🔴【破位止损】"
			defensePrice = ma20
			actionAdvice = fmt.Sprintf("浮亏扩大至%.1f%%且跌破MA20=%.2f，触发风控红线，果断止损。", it.ProfitRate, ma20)
		} else if alignment == "多头排列" && curPrice >= ma20 {
			// 确定性多头排列且站上 MA20
			riskTier = "GREEN"
			tierPrefix = "🟢"
			defensePrice = ma20
			if it.ChangePercent <= 0 || volRatio < 1.0 {
				tierTitle = "🟢【多头持有】"
				actionAdvice = fmt.Sprintf("MA5>MA10>MA20多头排列，价高于MA20约%.1f%%，缩量回调属强势整理。持有，防守MA20=%.2f。",
					distMA20, ma20)
			} else if consecDays >= 2 {
				tierTitle = "🟢【多头趋势】"
				actionAdvice = fmt.Sprintf("MA5>MA10>MA20多头排列，站上MA20约%.1f%%，连涨%d日。持有，破MA20=%.2f离场。",
					distMA20, consecDays, ma20)
			} else {
				tierTitle = "🟢【多头进攻】"
				defensePrice = ma5
				actionAdvice = fmt.Sprintf("多头排列完好，价高于MA20约%.1f%%。依托MA5=%.2f持有跟踪，破MA20=%.2f防守。",
					distMA20, ma5, ma20)
			}
		} else {
			// 中间态 / 震荡 / 纠缠 / 破位反弹
			riskTier = "YELLOW"
			tierPrefix = "🟡"
			if curPrice >= ma20 && alignment != "多头排列" {
				tierTitle = "🟡【待放量确认】"
				defensePrice = ma20
				actionAdvice = fmt.Sprintf("价在MA5/MA20上，但MA10<MA20纠缠、量比仅%.2f。持有不加仓，量比>1.5且放量再进攻，破MA20=%.2f防守。",
					volRatio, ma20)
			} else if curPrice < ma20 && consecDays >= 3 {
				tierTitle = "🟡【MA20下方】"
				defensePrice = ma5
				resistancePrice = ma20
				actionAdvice = fmt.Sprintf("跌破MA20=%.2f，量比%.2f。连涨%d日不追高，回补%.2f再转多，破MA5=%.2f减仓。",
					ma20, volRatio, consecDays, ma20, ma5)
			} else if alignment == "空头排列" {
				tierTitle = "🟡【空头反弹减仓】"
				defensePrice = ma5
				resistancePrice = ma20
				actionAdvice = fmt.Sprintf("MA5<MA10<MA20，破MA20=%.3f，量比%.2f走弱。反弹%.2f-%.2f减仓，防守%.2f。",
					ma20, volRatio, ma20, ma20*1.015, ma5)
			} else {
				tierTitle = "🟡【防守观望】"
				defensePrice = ma20
				actionAdvice = fmt.Sprintf("均线纠缠震荡，量比%.2f。多空方向不明，保持防守观望，破MA20=%.2f减仓。",
					volRatio, ma20)
			}
		}

		processedItems = append(processedItems, HoldingRiskItem{
			StockCode:       cleanCode,
			StockName:       it.StockName,
			CurrentPrice:    curPrice,
			ChangePercent:   it.ChangePercent,
			CostPrice:       it.CostPrice,
			CostVolume:      it.CostVolume,
			ProfitRate:      it.ProfitRate,
			MA5:             math.Round(ma5*1000) / 1000,
			MA10:            math.Round(ma10*1000) / 1000,
			MA20:            math.Round(ma20*1000) / 1000,
			VolumeRatio:     math.Round(volRatio*100) / 100,
			VolumeDesc:      volDesc,
			Alignment:       alignment,
			DistanceToMA20:  math.Round(distMA20*10) / 10,
			ConsecutiveDays: consecDays,
			RecentReturn5D:  math.Round(return5D*10) / 10,
			RiskTier:        riskTier,
			TierPrefix:      tierPrefix,
			TierTitle:       tierTitle,
			DefensePrice:    math.Round(defensePrice*100) / 100,
			ResistancePrice: math.Round(resistancePrice*100) / 100,
			ActionAdvice:    actionAdvice,
			HasRiskLock:     hasRiskLock,
			RiskLockReason:  riskLockReason,
		})
	}

	// 4. 统计分级与构建结构化报告
	greenItems := []HoldingRiskItem{}
	yellowItems := []HoldingRiskItem{}
	redItems := []HoldingRiskItem{}

	for _, p := range processedItems {
		switch p.RiskTier {
		case "GREEN":
			greenItems = append(greenItems, p)
		case "YELLOW":
			yellowItems = append(yellowItems, p)
		case "RED":
			redItems = append(redItems, p)
		}
	}

	// 盯盘主线与红线
	focusList := []string{}
	redLineList := []string{}

	if len(greenItems) > 0 {
		var names []string
		var defs []string
		for _, g := range greenItems {
			names = append(names, g.StockName)
			defs = append(defs, fmt.Sprintf("%.1f", g.DefensePrice))
		}
		focusList = append(focusList, fmt.Sprintf("%s 的多头结构能否守住MA20(%s)，决定守仓还是撤。",
			strings.Join(names, "与"), strings.Join(defs, "/")))
	} else {
		focusList = append(focusList, "全市场弱势震荡，暂无稳健多头标的，保持空仓/防守为主。")
	}

	if len(redItems) > 0 {
		for _, r := range redItems {
			redLineList = append(redLineList, fmt.Sprintf("禁止新增%s(%s)，空头排列下反弹均为减仓窗口", r.StockName, r.StockCode))
		}
	}
	for _, y := range yellowItems {
		if y.ConsecutiveDays >= 3 || y.VolumeRatio < 1.2 {
			redLineList = append(redLineList, fmt.Sprintf("禁止因单日异动追高%s", y.StockName))
			break
		}
	}

	// 5. 组装纯 Go 确定性 Markdown 研报 (严格控制在 250 CJK 字以内)
	var sb strings.Builder
	sb.WriteString("1. 风险分级清单\n")

	// 依次写入 🟢、🟡、🔴
	for _, it := range greenItems {
		sb.WriteString(fmt.Sprintf("%s%s(%s)：%s\n", it.TierTitle, it.StockName, it.StockCode, it.ActionAdvice))
	}
	for _, it := range yellowItems {
		sb.WriteString(fmt.Sprintf("%s%s(%s)：%s\n", it.TierTitle, it.StockName, it.StockCode, it.ActionAdvice))
	}
	for _, it := range redItems {
		sb.WriteString(fmt.Sprintf("%s%s(%s)：%s\n", it.TierTitle, it.StockName, it.StockCode, it.ActionAdvice))
	}

	sb.WriteString("2. 盯盘主线与红线\n")
	sb.WriteString(fmt.Sprintf("盯盘：%s\n", strings.Join(focusList, "；")))
	sb.WriteString(fmt.Sprintf("红线：%s。\n", strings.Join(redLineList, "；")))
	if latestDataDate == "" {
		latestDataDate = time.Now().Format("2006-01-02")
	}
	sb.WriteString(fmt.Sprintf("【数据溯源】：%s 收盘 | 东方财富/通达信前复权(QFQ) | 指标口径: MA(5/10/20) + 5日均量比", latestDataDate))

	return &HoldingRiskScanResult{
		GeneratedAt:    time.Now().Format("2006-01-02 15:04:05"),
		DataDate:       latestDataDate,
		DataSource:     "东方财富/通达信前复权(QFQ)日K",
		RegimeState:    regimeState,
		TotalCount:     len(processedItems),
		GreenCount:     len(greenItems),
		YellowCount:    len(yellowItems),
		RedCount:       len(redItems),
		Items:          processedItems,
		FocusStocks:    focusList,
		RedLineStocks:  redLineList,
		MarkdownReport: sb.String(),
	}, nil
}

// ParseHoldingScanItemsFromPrompt 从前端 Prompt 文本中反解析股票代码与持仓信息
func ParseHoldingScanItemsFromPrompt(prompt string) []HoldingScanItem {
	var items []HoldingScanItem
	// 匹配形如: 贵州茅台(600519): 现价1466.54元, 涨幅-0.19%
	re := regexp.MustCompile(`([\p{Han}A-Za-z0-9_]+)\s*\((00\d{4}|30\d{4}|60\d{4}|68\d{4}|51\d{4}|58\d{4}|15\d{4})\)[^\n\r]*`)
	matches := re.FindAllStringSubmatch(prompt, -1)

	rePrice := regexp.MustCompile(`现价\s*([0-9.]+)`)
	reChg := regexp.MustCompile(`涨幅\s*([+-]?[0-9.]+)`)
	reCost := regexp.MustCompile(`持仓成本\s*([0-9.]+)`)
	reProfit := regexp.MustCompile(`\(([+-]?[0-9.]+)%\)`)

	seen := make(map[string]bool)
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		name := strings.TrimSpace(m[1])
		code := strings.TrimSpace(m[2])
		if seen[code] {
			continue
		}
		seen[code] = true

		fullLine := m[0]
		price := 0.0
		if pm := rePrice.FindStringSubmatch(fullLine); len(pm) >= 2 {
			price, _ = strconv.ParseFloat(pm[1], 64)
		}
		chg := 0.0
		if cm := reChg.FindStringSubmatch(fullLine); len(cm) >= 2 {
			chg, _ = strconv.ParseFloat(cm[1], 64)
		}
		cost := 0.0
		if csm := reCost.FindStringSubmatch(fullLine); len(csm) >= 2 {
			cost, _ = strconv.ParseFloat(csm[1], 64)
		}
		profit := 0.0
		if pfm := reProfit.FindStringSubmatch(fullLine); len(pfm) >= 2 {
			profit, _ = strconv.ParseFloat(pfm[1], 64)
		}

		items = append(items, HoldingScanItem{
			StockCode:     code,
			StockName:     name,
			CurrentPrice:  price,
			ChangePercent: chg,
			CostPrice:     cost,
			ProfitRate:    profit,
		})
	}

	return items
}

func calcPeriodSMA(closes []float64, period int) float64 {
	n := len(closes)
	if n == 0 || period <= 0 {
		return 0
	}
	if period > n {
		period = n
	}
	sum := 0.0
	for i := n - period; i < n; i++ {
		sum += closes[i]
	}
	return sum / float64(period)
}

func fallbackItem(it HoldingScanItem) HoldingRiskItem {
	// 无 K 线数据时也必须给出有效防守位（结构性不变量：DefensePrice > 0），
	// 否则下游按防守位减仓的逻辑会拿到 0 值。无均线可用时以现价下 5% 作为
	// 保守防守线；现价未知时无法推导，保持 0 并由调用方按缺失处理。
	defensePrice := 0.0
	if it.CurrentPrice > 0 {
		defensePrice = math.Round(it.CurrentPrice*0.95*100) / 100
	}
	return HoldingRiskItem{
		StockCode:     it.StockCode,
		StockName:     it.StockName,
		CurrentPrice:  it.CurrentPrice,
		ChangePercent: it.ChangePercent,
		RiskTier:      "YELLOW",
		TierPrefix:    "🟡",
		TierTitle:     "🟡【防守观望】",
		DefensePrice:  defensePrice,
		ActionAdvice:  "历史K线样本不足，维持中性观望与仓位控制。",
	}
}
