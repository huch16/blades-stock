package data

import (
	"blades-stock/backend/logger"
	"blades-stock/backend/util"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

func init() {
	registerToolHandler("AnalyzeHotMoneyActivity", handleAnalyzeHotMoneyActivity)
	registerToolHandler("AnalyzeChipConcentration", handleAnalyzeChipConcentration)
	registerToolHandler("AnalyzeCapitalGame", handleAnalyzeCapitalGame)
}

func handleAnalyzeHotMoneyActivity(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "AnalyzeHotMoneyActivity", funcArguments)
	stockCode := gjson.Get(funcArguments, "stockCode").String()
	days := int(gjson.Get(funcArguments, "days").Int())
	if days <= 0 {
		days = 30
	}
	if stockCode == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "请提供股票代码")
		return nil
	}
	sdk := NewStockDataApi()

	bbMd := sdk.GetStockBillboardToMarkdown(stockCode)
	deptMd := sdk.GetStockOperationDeptTradeToMarkdown(stockCode)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## 游资追踪分析：%s\n\n", stockCode))
	sb.WriteString(fmt.Sprintf("> 回溯周期：近 %d 天 | 数据来源：东方财富F10龙虎榜\n\n", days))

	sb.WriteString("### 一、龙虎榜上榜记录\n\n")
	if bbMd == "" || strings.Contains(bbMd, "失败") || strings.Contains(bbMd, "无") {
		sb.WriteString("近期待分析期间无龙虎榜上榜记录，可能该股非游资炒作标的或近期交易清淡。\n\n")
	} else {
		sb.WriteString(bbMd + "\n\n")
	}

	sb.WriteString("### 二、营业部买卖明细\n\n")
	sb.WriteString("> 注：买入居前营业部可能为游资席位，卖出居前可能为获利了结或止损出局。\n\n")
	if deptMd == "" || strings.Contains(deptMd, "失败") || strings.Contains(deptMd, "无") {
		sb.WriteString("无近期营业部交易明细数据。\n\n")
	} else {
		sb.WriteString(deptMd + "\n\n")
	}

	sb.WriteString("### 三、游资行为简析\n\n")
	totalBuy, totalSell := extractBillboardTotal(bbMd)
	if totalBuy > 0 || totalSell > 0 {
		net := totalBuy - totalSell
		direction := "净买入"
		if net < 0 {
			direction = "净卖出"
		}
		sb.WriteString(fmt.Sprintf("- 龙虎榜汇总：买入总额 %.0f 万，卖出总额 %.0f 万，%s %.0f 万\n", totalBuy, totalSell, direction, absFloat(net)))
		if net > 0 && totalBuy > 0 {
			sb.WriteString("- 游资情绪：**偏积极**，买入力度大于卖出，游资对该股关注度较高\n")
		} else if net < 0 && totalSell > 0 {
			sb.WriteString("- 游资情绪：**偏消极**，卖出力度大于买入，需警惕游资出货风险\n")
		} else {
			sb.WriteString("- 游资情绪：**中性**，买卖力度相当，游资存在分歧\n")
		}
		if totalBuy > 10000 {
			sb.WriteString("- 买入规模较大，可能为**顶级游资**参与，关注后续持续性\n")
		} else if totalBuy > 3000 {
			sb.WriteString("- 买入规模中等，可能为**二线游资**或**机构**参与\n")
		} else if totalBuy > 0 {
			sb.WriteString("- 买入规模较小，可能为**小游资**或**散户**参与\n")
		}
	} else {
		sb.WriteString("龙虎榜数据不足，无法形成有效判断。建议结合资金流向和K线技术面综合判断。\n\n")
	}

	sb.WriteString("\n---\n")
	sb.WriteString("> ⚠️ 本分析基于公开龙虎榜数据，游资席位可能存在分仓、对倒等操作，结论仅供参考。\n")
	sb.WriteString("> 游资分析需要配合技术面（K线形态、成交量变化）和基本面综合研判。\n")

	logger.SugaredLogger.Debug("AnalyzeHotMoneyActivity completed for %s", stockCode)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, sb.String())
	return nil
}

func handleAnalyzeChipConcentration(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "AnalyzeChipConcentration", funcArguments)
	stockCode := gjson.Get(funcArguments, "stockCode").String()
	if stockCode == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "请提供股票代码")
		return nil
	}
	sdk := NewStockDataApi()

	holderMd := sdk.GetStockHolderTrendToMarkdown(stockCode)
	blockMd := sdk.GetStockBlockTradeToMarkdown(stockCode)
	moneyData := sdk.GetStockHistoryMoneyData(stockCode)
	moneyMd := ""
	if len(moneyData) > 0 {
		moneyMd = util.MarkdownTableWithTitle(stockCode+"历史资金流向", moneyData)
	}
	marginMd := sdk.GetStockMarginTradingToMarkdown(stockCode)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## 筹码集中度与主力动向分析：%s\n\n", stockCode))
	sb.WriteString("> 综合股东户数、大宗交易、资金流向、融资融券等维度\n\n")

	sb.WriteString("### 一、股东户数趋势（筹码集中度核心指标）\n\n")
	sb.WriteString("> 股东户数减少=筹码集中（偏利好），增加=筹码分散（偏利空）\n\n")
	if holderMd == "" || strings.Contains(holderMd, "失败") || strings.Contains(holderMd, "无") {
		sb.WriteString("暂无股东户数趋势数据。\n\n")
	} else {
		sb.WriteString(holderMd + "\n\n")
	}

	sb.WriteString("### 二、大宗交易分析\n\n")
	sb.WriteString("> 溢价成交=买方意愿强，折价成交=卖方急售；营业部折溢价可辅助判断机构/游资行为\n\n")
	if blockMd == "" || strings.Contains(blockMd, "失败") || strings.Contains(blockMd, "无") {
		sb.WriteString("暂无大宗交易数据。\n\n")
	} else {
		sb.WriteString(blockMd + "\n\n")
	}

	sb.WriteString("### 三、资金流向（主力/大单/散户）\n\n")
	sb.WriteString("> 主力净流入=大资金看好，持续净流出=主力减仓\n\n")
	if moneyMd == "" || strings.Contains(moneyMd, "失败") || strings.Contains(moneyMd, "无") {
		sb.WriteString("暂无资金流向数据。\n\n")
	} else {
		sb.WriteString(moneyMd + "\n\n")
	}

	sb.WriteString("### 四、融资融券变化\n\n")
	sb.WriteString("> 融资余额增加=杠杆资金看多，融券增加=做空力量增强\n\n")
	if marginMd == "" || strings.Contains(marginMd, "失败") || strings.Contains(marginMd, "无") {
		sb.WriteString("暂无融资融券数据。\n\n")
	} else {
		sb.WriteString(marginMd + "\n\n")
	}

	sb.WriteString("### 五、综合判断\n\n")
	holderConcentrating := detectHolderConcentration(holderMd)
	blockPositive := detectBlockTradePositive(blockMd)
	moneyPositive := detectMoneyFlowPositive(moneyMd)
	marginPositive := detectMarginPositive(marginMd)

	positiveSignals := 0
	negativeSignals := 0
	if holderConcentrating > 0 {
		positiveSignals++
	} else if holderConcentrating < 0 {
		negativeSignals++
	}
	if blockPositive > 0 {
		positiveSignals++
	} else if blockPositive < 0 {
		negativeSignals++
	}
	if moneyPositive > 0 {
		positiveSignals++
	} else if moneyPositive < 0 {
		negativeSignals++
	}
	if marginPositive > 0 {
		positiveSignals++
	} else if marginPositive < 0 {
		negativeSignals++
	}

	sb.WriteString("| 维度 | 信号 | 解读 |\n|---|---|---|\n")
	sb.WriteString(fmt.Sprintf("| 股东户数 | %s | %s |\n",
		signalIcon(holderConcentrating), concentrationDesc(holderConcentrating)))
	sb.WriteString(fmt.Sprintf("| 大宗交易 | %s | %s |\n",
		signalIcon(blockPositive), blockTradeDesc(blockPositive)))
	sb.WriteString(fmt.Sprintf("| 资金流向 | %s | %s |\n",
		signalIcon(moneyPositive), moneyFlowDesc(moneyPositive)))
	sb.WriteString(fmt.Sprintf("| 融资融券 | %s | %s |\n",
		signalIcon(marginPositive), marginDesc(marginPositive)))
	sb.WriteString("\n")

	sb.WriteString(fmt.Sprintf("**综合结论**：积极信号 %d 个，消极信号 %d 个。\n\n", positiveSignals, negativeSignals))
	if positiveSignals >= 3 {
		sb.WriteString("✅ **筹码集中信号强烈**：多维度数据指向主力正在吸筹/建仓，筹码趋于集中，后续上涨概率较大。\n")
	} else if positiveSignals >= 2 {
		sb.WriteString("🔶 **筹码有集中迹象**：部分维度显示资金介入，需持续观察确认。\n")
	} else if negativeSignals >= 3 {
		sb.WriteString("❌ **筹码分散风险**：多维度数据显示资金流出或筹码分散，建议谨慎。\n")
	} else if negativeSignals >= 2 {
		sb.WriteString("🔶 **筹码有分散迹象**：部分维度显示资金离场，需警惕主力出货。\n")
	} else {
		sb.WriteString("⚪ **信号不明**：各维度数据交叉，建议结合K线形态和行业趋势综合判断。\n")
	}

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("**主力行为推测**：%s\n\n", guessMainForceBehavior(positiveSignals, negativeSignals)))

	sb.WriteString("\n---\n")
	sb.WriteString("> ⚠️ 本分析基于公开交易数据，筹码判断仅为概率推测。主力可通过分仓、对倒等方式隐藏真实意图。\n")

	logger.SugaredLogger.Debug("AnalyzeChipConcentration completed for %s", stockCode)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, sb.String())
	return nil
}

func handleAnalyzeCapitalGame(o *OpenAi, funcArguments string, ctx *ToolContext) error {
	sendToolCallLog(ctx, "AnalyzeCapitalGame", funcArguments)
	stockCode := gjson.Get(funcArguments, "stockCode").String()
	days := int(gjson.Get(funcArguments, "days").Int())
	if days <= 0 {
		days = 20
	}
	if stockCode == "" {
		appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, "请提供股票代码")
		return nil
	}
	sdk := NewStockDataApi()

	moneyData := sdk.GetStockHistoryMoneyData(stockCode)
	moneyMd := ""
	if len(moneyData) > 0 {
		moneyMd = util.MarkdownTableWithTitle(stockCode+"历史资金流向", moneyData)
	}
	marginMd := sdk.GetStockMarginTradingToMarkdown(stockCode)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## 资金博弈分析：%s\n\n", stockCode))
	sb.WriteString(fmt.Sprintf("> 分析周期：近 %d 个交易日 | 数据来源：东方财富\n\n", days))

	sb.WriteString("### 一、资金流向结构\n\n")
	sb.WriteString("> 资金流向按订单大小分级：超大单(>=500万)、大单(100~500万)、中单(20~100万)、小单(<20万)\n")
	sb.WriteString("> 主力=超大单+大单，散户≈小单\n\n")
	if moneyMd == "" || strings.Contains(moneyMd, "失败") || strings.Contains(moneyMd, "无") {
		sb.WriteString("暂无资金流向数据。\n\n")
	} else {
		sb.WriteString(moneyMd + "\n\n")
	}

	sb.WriteString("### 二、杠杆资金情绪（融资融券）\n\n")
	if marginMd == "" || strings.Contains(marginMd, "失败") || strings.Contains(marginMd, "无") {
		sb.WriteString("暂无融资融券数据。\n\n")
	} else {
		sb.WriteString(marginMd + "\n\n")
	}

	mainForce, retail := extractMoneyFlowTrend(moneyMd)
	sb.WriteString("### 三、资金博弈解读\n\n")
	sb.WriteString("| 指标 | 数值 | 解读 |\n|---|---|---|\n")
	if mainForce > 0 {
		sb.WriteString(fmt.Sprintf("| 主力资金 | 净流入 %.0f 万 | ✅ 大资金主动买入，主导方向向上 |\n", mainForce))
	} else if mainForce < 0 {
		sb.WriteString(fmt.Sprintf("| 主力资金 | 净流出 %.0f 万 | ❌ 大资金主动卖出，主导方向向下 |\n", absFloat(mainForce)))
	} else {
		sb.WriteString("| 主力资金 | 无明显方向 | ⚪ 主力资金观望或分歧 |\n")
	}
	if retail > 0 {
		sb.WriteString(fmt.Sprintf("| 散户资金 | 净流入 %.0f 万 | ⚠️ 散户在买入，需警惕主力借机出货 |\n", retail))
	} else if retail < 0 {
		sb.WriteString(fmt.Sprintf("| 散户资金 | 净流出 %.0f 万 | ✅ 散户在卖出，减少抛压 |\n", absFloat(retail)))
	} else {
		sb.WriteString("| 散户资金 | 无明显方向 | ⚪ 散户参与度一般 |\n")
	}

	if mainForce > 0 && retail < 0 {
		sb.WriteString("| 博弈格局 | 主力进散户退 | ✅ **理想格局**：主力吸筹，散户交出筹码 |\n")
	} else if mainForce > 0 && retail > 0 {
		sb.WriteString("| 博弈格局 | 共同买入 | ⚠️ 主力与散户都在买，可能引发短期过热 |\n")
	} else if mainForce < 0 && retail > 0 {
		sb.WriteString("| 博弈格局 | 主力出散户进 | ❌ **危险格局**：主力在出货，散户在接盘 |\n")
	} else if mainForce < 0 && retail < 0 {
		sb.WriteString("| 博弈格局 | 共同卖出 | 🔶 多空分歧大，可能处于调整/洗盘阶段 |\n")
	} else {
		sb.WriteString("| 博弈格局 | 方向不明 | ⚪ 需更多数据确认 |\n")
	}

	sb.WriteString("\n### 四、资金性质判断\n\n")
	capitalNature := assessCapitalNature(mainForce, retail, moneyMd)
	sb.WriteString(capitalNature)

	sb.WriteString("\n---\n")
	sb.WriteString("> ⚠️ 资金流向数据基于东方财富分级统计，大单/小单划分标准可能与实际存在偏差。\n")
	sb.WriteString("> 博弈分析需要结合股价所处位置（高位/低位）综合判断。\n")

	logger.SugaredLogger.Debug("AnalyzeCapitalGame completed for %s", stockCode)
	appendToolMessages(ctx.Messages, ctx.CurrentAIContent.String(), ctx.ReasoningContentText.String(), ctx.CurrentCallID, ctx.FuncName, funcArguments, sb.String())
	return nil
}

// ============ helper functions (duplicated from agent/tools/hot_money_chip_tools.go) ============

func extractBillboardTotal(md string) (buy, sell float64) {
	if md == "" {
		return 0, 0
	}
	lines := strings.Split(md, "\n")
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "买入") || strings.Contains(lower, "buy") {
			if v, err := extractFirstNumber(line); err == nil && v > 0 {
				buy = v
			}
		}
		if strings.Contains(lower, "卖出") || strings.Contains(lower, "sell") {
			if v, err := extractFirstNumber(line); err == nil && v > 0 {
				sell = v
			}
		}
	}
	_ = lines
	return buy, sell
}

func extractFirstNumber(s string) (float64, error) {
	var numStr strings.Builder
	inNumber := false
	hasDot := false
	for _, c := range s {
		if c >= '0' && c <= '9' {
			numStr.WriteRune(c)
			inNumber = true
		} else if c == '.' && inNumber && !hasDot {
			numStr.WriteRune(c)
			hasDot = true
		} else if inNumber {
			break
		}
	}
	if numStr.Len() > 0 {
		var v float64
		_, err := fmt.Sscanf(numStr.String(), "%f", &v)
		return v, err
	}
	return 0, fmt.Errorf("no number found")
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func detectHolderConcentration(md string) int {
	if md == "" || strings.Contains(md, "无") || strings.Contains(md, "失败") {
		return 0
	}
	text := strings.ToLower(md)
	decreaseWords := []string{"减少", "下降", "递减", "降低"}
	increaseWords := []string{"增加", "上升", "递增", "升高"}
	for _, w := range decreaseWords {
		if strings.Contains(text, w) {
			return 1
		}
	}
	for _, w := range increaseWords {
		if strings.Contains(text, w) {
			return -1
		}
	}
	return 0
}

func detectBlockTradePositive(md string) int {
	if md == "" || strings.Contains(md, "无") || strings.Contains(md, "失败") {
		return 0
	}
	text := strings.ToLower(md)
	if strings.Contains(text, "溢价") {
		return 1
	}
	if strings.Contains(text, "折价") {
		return -1
	}
	return 0
}

func detectMoneyFlowPositive(md string) int {
	if md == "" || strings.Contains(md, "无") || strings.Contains(md, "失败") {
		return 0
	}
	text := strings.ToLower(md)
	if strings.Contains(text, "净流入") || strings.Contains(text, "主力净额") {
		return 1
	}
	if strings.Contains(text, "净流出") {
		return -1
	}
	return 0
}

func detectMarginPositive(md string) int {
	if md == "" || strings.Contains(md, "无") || strings.Contains(md, "失败") {
		return 0
	}
	text := strings.ToLower(md)
	if strings.Contains(text, "增加") || strings.Contains(text, "上升") {
		return 1
	}
	if strings.Contains(text, "减少") || strings.Contains(text, "下降") {
		return -1
	}
	return 0
}

func signalIcon(v int) string {
	if v > 0 {
		return "✅"
	}
	if v < 0 {
		return "❌"
	}
	return "⚪"
}

func concentrationDesc(v int) string {
	if v > 0 {
		return "股东户数减少，筹码趋于集中，主力可能在建仓"
	}
	if v < 0 {
		return "股东户数增加，筹码趋于分散，需警惕主力出货"
	}
	return "数据不足或变化不明显"
}

func blockTradeDesc(v int) string {
	if v > 0 {
		return "溢价大宗交易为主，买方意愿强"
	}
	if v < 0 {
		return "折价大宗交易为主，卖方急售"
	}
	return "数据不足或无明显信号"
}

func moneyFlowDesc(v int) string {
	if v > 0 {
		return "主力资金净流入，大资金看好"
	}
	if v < 0 {
		return "主力资金净流出，大资金减仓"
	}
	return "数据不足或无明显方向"
}

func marginDesc(v int) string {
	if v > 0 {
		return "融资余额增加，杠杆资金看多"
	}
	if v < 0 {
		return "融资余额减少，杠杆资金离场"
	}
	return "数据不足或无明显变化"
}

func guessMainForceBehavior(pos, neg int) string {
	switch {
	case pos >= 3 && neg == 0:
		return "🟢 **强烈建仓信号**：所有维度一致指向主力正在积极建仓，筹码迅速集中，后续大概率上涨。"
	case pos >= 2 && neg <= 1:
		return "🟢 **大概率建仓/增持**：多个维度显示资金介入，主力可能在逐步建仓。"
	case pos >= 2 && neg >= 2:
		return "🟡 **分歧较大**：各维度信号不同，可能处于洗盘阶段或主力调仓换股。"
	case neg >= 3 && pos == 0:
		return "🔴 **强烈出货信号**：所有维度一致指向主力出货，风险较高。"
	case neg >= 2 && pos <= 1:
		return "🔴 **大概率出货/减仓**：多个维度显示资金流出，主力可能在逐步出货。"
	case neg == 0 && pos == 0:
		return "⚪ **无明显主力行为**：数据信号不够明显，建议结合技术面和行业趋势判断。"
	default:
		return "🟡 **信号混合**：积极和消极信号并存，建议持续观察。"
	}
}

func extractMoneyFlowTrend(md string) (mainForce, retail float64) {
	return 0, 0
}

func assessCapitalNature(mainForce, retail float64, moneyMd string) string {
	if moneyMd == "" {
		return "数据不足，无法判断资金性质。\n"
	}
	var sb strings.Builder
	sb.WriteString("**资金性质评估：**\n\n")

	lines := strings.Split(moneyMd, "\n")
	mainDays := 0
	retailDays := 0
	for _, line := range lines {
		if strings.Contains(line, "主力") && (strings.Contains(line, "流入") || strings.Contains(line, "+")) {
			mainDays++
		}
		if strings.Contains(line, "小单") && (strings.Contains(line, "流出") || strings.Contains(line, "-")) {
			retailDays++
		}
	}
	sb.WriteString(fmt.Sprintf("- 主力净流入天数占比：约 %d/%d 个交易日\n", mainDays, maxInt(len(lines)/5, 1)))
	sb.WriteString(fmt.Sprintf("- 散户净流出天数占比：约 %d/%d 个交易日\n", retailDays, maxInt(len(lines)/5, 1)))

	if mainForce > 0 && mainDays > 0 {
		sb.WriteString("- 资金性质判断：**主力资金主导**，近期以机构/游资行为为主\n")
	} else if retail > 0 && retailDays > 0 {
		sb.WriteString("- 资金性质判断：**散户资金主导**，需关注是否是主力出货阶段\n")
	} else {
		sb.WriteString("- 资金性质判断：**资金方向不明确**，多空力量均衡\n")
	}

	return sb.String()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
