package tools

import (
	"blades-stock/backend/agent/types"
	"blades-stock/backend/quant"
	"context"
	"fmt"
	"strings"

	bladesTools "github.com/go-kratos/blades/tools"
	"github.com/tidwall/gjson"
)

// GetBacktestStatsTool 获取AI分析回测统计
func GetBacktestStatsTool() bladesTools.Tool {
	return NewDataToolWrapper(
		"GetBacktestStats",
		"获取AI分析回测统计（验证历史预测准确性）",
		map[string]*types.ParamDef{
			"stock_code": {Type: "string", Desc: "股票代码（可选，指定则返回该股票统计）"},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stock_code").String()
			mgr := quant.NewBacktestManager()

			if stockCode != "" {
				stats := mgr.GetStockStats(stockCode)
				return formatStats(stats, stockCode), nil
			}
			stats := mgr.GetOverallStats()
			return formatStats(stats, "全部股票"), nil
		},
	)
}

// GetBacktestDetailTool 获取指定报告的回测详情
func GetBacktestDetailTool() bladesTools.Tool {
	return NewDataToolWrapper(
		"GetBacktestDetail",
		"获取指定报告的回测详情",
		map[string]*types.ParamDef{
			"report_id": {Type: "integer", Desc: "分析报告ID", Required: true},
		},
		func(args string) (string, error) {
			reportID := uint(gjson.Get(args, "report_id").Int())
			if reportID == 0 {
				return "请提供 report_id", nil
			}

			mgr := quant.NewBacktestManager()
			results := mgr.GetLatestResults(100)

			for _, r := range results {
				if r.ReportID == reportID {
					accurateMark := "❌"
					if r.IsAccurate {
						accurateMark = "✅"
					}
					return fmt.Sprintf(`## 回测详情

- 股票: %s(%s)
- 分析日期: %s
- 预测方向: %s
- 目标价: %.2f
- 验证日期: %s
- 验证收盘价: %.2f
- 实际涨跌幅: %.2f%%
- 期间最大涨幅: %.2f%%
- 期间最大回撤: %.2f%%
- 判断准确: %s
- 置信度: %.0f%%
- 摘要: %s`,
						r.StockName, r.StockCode,
						r.AnalysisDate,
						r.Direction,
						r.TargetPrice,
						r.VerifyDate,
						r.VerifyPrice,
						r.PriceChange,
						r.MaxUpside,
						r.MaxDrawdown,
						accurateMark,
						r.Confidence*100,
						r.Summary,
					), nil
				}
			}
			return "该报告暂无回测结果，可使用 RunBacktest 触发回测", nil
		},
	)
}

// GetRunBacktestTool 触发AI分析报告回测
func GetRunBacktestTool() bladesTools.Tool {
	return NewDataToolWrapper(
		"RunBacktest",
		"触发AI分析报告回测（将历史分析与后续真实走势对比验证）",
		map[string]*types.ParamDef{
			"start_date": {Type: "string", Desc: "起始日期（可选，如 2024-01-01）"},
			"end_date":   {Type: "string", Desc: "结束日期（可选，如 2024-12-31）"},
		},
		func(args string) (string, error) {
			mgr := quant.NewBacktestManager()
			startDate := gjson.Get(args, "start_date").String()
			endDate := gjson.Get(args, "end_date").String()

			if startDate != "" || endDate != "" {
				count, err := mgr.RunBacktestOnDateRange(startDate, endDate)
				if err != nil {
					return fmt.Sprintf("回测执行失败: %v", err), nil
				}
				if count == 0 {
					return "指定日期范围内无待回测报告", nil
				}
				stats := mgr.GetOverallStats()
				return fmt.Sprintf("✅ 完成 %d 份分析报告回测！\n当前综合准确率: %.1f%% (已验证 %d 次)", count, stats.AccuracyRate, stats.VerifiedCount), nil
			}

			result, err := mgr.RunAIReportBacktest(context.Background(), 50)
			if err != nil {
				return fmt.Sprintf("回测执行失败: %v", err), nil
			}
			if result.Total == 0 {
				return "所有分析报告已完成回测，无需重复执行", nil
			}
			stats := mgr.GetOverallStats()
			return fmt.Sprintf("✅ 完成 %d 份分析报告回测（跳过 %d 份）！\n当前综合准确率: %.1f%% (已验证 %d 次)",
				result.Processed, result.Skipped, stats.AccuracyRate, stats.VerifiedCount), nil
		},
	)
}

// GetVectorBacktestTool VectorBT 向量化回测工具
func GetVectorBacktestTool() bladesTools.Tool {
	return NewDataToolWrapper(
		"VectorBacktest",
		"【VectorBT向量化回测】对单只股票运行策略回测。支持MA均线交叉、MACD、RSI、布林带等策略，返回夏普比率、最大回撤、胜率。用于测试策略参数、对比策略、寻优参数。",
		map[string]*types.ParamDef{
			"stock_code":  {Type: "string", Desc: "股票代码，如 000001（平安银行）、600519（贵州茅台）", Required: true},
			"strategy":    {Type: "string", Desc: "策略类型：MA（均线交叉）、MACD、RSI、BOLL（布林带）、DualThrust", Required: false},
			"fast":        {Type: "number", Desc: "快线周期（MA/MACD用），如 5", Required: false},
			"slow":        {Type: "number", Desc: "慢线周期（MA/MACD用），如 20", Required: false},
			"signal":      {Type: "number", Desc: "信号周期（MACD用），如 9", Required: false},
			"window":      {Type: "number", Desc: "窗口周期（RSI/布林带用），如 14", Required: false},
			"init_cash":   {Type: "number", Desc: "初始资金，默认 100000", Required: false},
			"stop_loss":   {Type: "number", Desc: "止损比例，如 0.05 表示 5%", Required: false},
			"take_profit": {Type: "number", Desc: "止盈比例，如 0.10 表示 10%", Required: false},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stock_code").String()
			if stockCode == "" {
				return "错误：缺少必需参数 stock_code", nil
			}

			strategyType := gjson.Get(args, "strategy").String()
			if strategyType == "" {
				strategyType = "MA"
			}

			params := make(map[string]any)
			if gjson.Get(args, "fast").Exists() {
				params["fast"] = gjson.Get(args, "fast").Float()
			}
			if gjson.Get(args, "slow").Exists() {
				params["slow"] = gjson.Get(args, "slow").Float()
			}
			if gjson.Get(args, "signal").Exists() {
				params["signal"] = gjson.Get(args, "signal").Float()
			}
			if gjson.Get(args, "window").Exists() {
				params["window"] = gjson.Get(args, "window").Float()
			}

			resp, err := quant.VectorBTHandler(stockCode, strategyType, params)
			if err != nil {
				return fmt.Sprintf("回测失败: %v", err), nil
			}

			var b strings.Builder
			b.WriteString("## 📊 VectorBT 向量化回测结果\n\n")
			b.WriteString(fmt.Sprintf("**股票**: %s | **策略**: %s | **参数**: %v\n\n", stockCode, strategyType, params))
			b.WriteString(resp.Report)
			if resp.Stats != nil {
				b.WriteString(fmt.Sprintf("\n**详细数据**: 总收益%.2f%% | 夏普%.4f | 最大回撤%.2f%%\n",
					resp.Stats.TotalReturn*100, resp.Stats.SharpeRatio, resp.Stats.MaxDrawdownPct))
			}

			return b.String(), nil
		},
	)
}

// GetRunVectorizedBacktestTool 高级向量化回测工具（支持单标的/多标的/指数成份股）
func GetRunVectorizedBacktestTool() bladesTools.Tool {
	return NewDataToolWrapper(
		"RunVectorizedBacktest",
		"【VectorBT高级向量化回测】运行极速向量化策略回测。支持单只股票或多只股票（逗号分隔，或沪深300代表性成分股hs300），支持MA（双均线）、MACD、RSI、BOLL（布林带）等策略，输出胜率、夏普比率、最大回撤、交易统计及收益率走势。",
		map[string]*types.ParamDef{
			"stock_codes": {Type: "string", Desc: "股票代码，可为单只（如 600519）、逗号分隔的多只（如 600519,000858,000333）、或输入 hs300/沪深300 自动选取代表性权重股", Required: true},
			"strategy":    {Type: "string", Desc: "策略类型：MA（双均线交叉）、MACD、RSI、BOLL（布林带），默认 MA", Required: false},
			"fast":        {Type: "number", Desc: "快线周期（MA/MACD用，如 5）", Required: false},
			"slow":        {Type: "number", Desc: "慢线周期（MA/MACD用，如 20）", Required: false},
			"signal":      {Type: "number", Desc: "信号周期（MACD用，如 9）", Required: false},
			"window":      {Type: "number", Desc: "窗口周期（RSI/BOLL用，如 14 或 20）", Required: false},
			"days":        {Type: "integer", Desc: "回测历史天数，默认 365（过去一年）", Required: false},
		},
		func(args string) (string, error) {
			rawCodes := gjson.Get(args, "stock_codes").String()
			if rawCodes == "" {
				rawCodes = gjson.Get(args, "stock_code").String()
			}
			if rawCodes == "" {
				return "错误：请提供 stock_codes 参数", nil
			}

			// 解析股票列表
			var stockList []string
			lowerCodes := strings.ToLower(rawCodes)
			if strings.Contains(lowerCodes, "hs300") || strings.Contains(lowerCodes, "300") || strings.Contains(rawCodes, "沪深300") {
				// 选取沪深300各主要行业龙头与代表性权重股
				stockList = []string{
					"600519", "300750", "601318", "600036", "000858",
					"000333", "601899", "601012", "002594", "600900",
				}
			} else {
				parts := strings.Split(rawCodes, ",")
				for _, p := range parts {
					p = strings.TrimSpace(p)
					if p != "" {
						stockList = append(stockList, p)
					}
				}
			}

			if len(stockList) == 0 {
				return "错误：未解析到有效的股票代码", nil
			}

			strategyType := gjson.Get(args, "strategy").String()
			if strategyType == "" {
				strategyType = "MA"
			}

			params := make(map[string]any)
			if gjson.Get(args, "fast").Exists() {
				params["fast"] = gjson.Get(args, "fast").Float()
			}
			if gjson.Get(args, "slow").Exists() {
				params["slow"] = gjson.Get(args, "slow").Float()
			}
			if gjson.Get(args, "signal").Exists() {
				params["signal"] = gjson.Get(args, "signal").Float()
			}
			if gjson.Get(args, "window").Exists() {
				params["window"] = gjson.Get(args, "window").Float()
			}

			days := int(gjson.Get(args, "days").Int())
			if days <= 0 {
				days = 365
			}

			multiRes, err := quant.RunVectorizedBacktestMulti(stockList, strategyType, params, days)
			if err != nil {
				return fmt.Sprintf("向量化回测失败: %v", err), nil
			}

			var b strings.Builder
			b.WriteString(multiRes.SummaryReport)

			// 如果是单只标的，并且有净值序列，输出收益走势概览
			if len(multiRes.Items) == 1 && len(multiRes.Items[0].EquityCurve) > 0 {
				curve := multiRes.Items[0].EquityCurve
				b.WriteString(fmt.Sprintf("\n#### 📈 资产净值走势（初始资金 100,000）：\n"))
				b.WriteString(fmt.Sprintf("- 期初资金: ¥%.2f\n", curve[0]))
				b.WriteString(fmt.Sprintf("- 期末资金: ¥%.2f\n", curve[len(curve)-1]))
				// 采样 5 个关键时点生成简要净值对比
				step := len(curve) / 4
				if step > 0 && len(multiRes.Items[0].Dates) == len(curve) {
					b.WriteString("- 关键净值节点: ")
					for i := 0; i < len(curve); i += step {
						b.WriteString(fmt.Sprintf("[%s: ¥%.0f] ", multiRes.Items[0].Dates[i], curve[i]))
					}
					b.WriteString("\n")
				}
			}

			return b.String(), nil
		},
	)
}

// GetVectorGridSearchTool 参数网格扫描工具
func GetVectorGridSearchTool() bladesTools.Tool {
	return NewDataToolWrapper(
		"VectorGridSearch",
		"【VectorBT参数网格扫描】对单只股票并行测试多种参数组合，自动找出最优参数。适用场景：批量测试策略参数空间、寻找最优参数、策略参数优化。输入股票代码和策略类型即可自动运行。",
		map[string]*types.ParamDef{
			"stock_code":       {Type: "string", Desc: "股票代码，如 000001、600519", Required: true},
			"strategy":         {Type: "string", Desc: "策略类型：MA（均线）、MACD、RSI", Required: false},
			"max_combinations": {Type: "integer", Desc: "最大测试组合数，默认 100", Required: false},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stock_code").String()
			if stockCode == "" {
				return "错误：缺少必需参数 stock_code", nil
			}

			strategyType := gjson.Get(args, "strategy").String()
			if strategyType == "" {
				strategyType = "MA"
			}

			resp, err := quant.VectorBTGridSearchHandler(stockCode, strategyType)
			if err != nil {
				return fmt.Sprintf("扫描失败: %v", err), nil
			}

			var b strings.Builder
			b.WriteString("## 🔍 参数网格扫描结果\n\n")
			b.WriteString(fmt.Sprintf("%s\n\n", resp.Message))
			b.WriteString(fmt.Sprintf("**最优策略**: %s\n", resp.BestStrategy))
			b.WriteString(fmt.Sprintf("**最优参数**: %v\n\n", resp.BestParams))

			if s := resp.BestStats; s != nil {
				b.WriteString("**最优表现**:\n")
				b.WriteString(fmt.Sprintf("- 总收益率: %.2f%%\n", s.TotalReturn*100))
				b.WriteString(fmt.Sprintf("- 年化收益率: %.2f%%\n", s.AnnualReturn*100))
				b.WriteString(fmt.Sprintf("- 夏普比率: %.4f\n", s.SharpeRatio))
				b.WriteString(fmt.Sprintf("- 最大回撤: %.2f%%\n", s.MaxDrawdownPct))
				b.WriteString(fmt.Sprintf("- 胜率: %.1f%%\n", s.WinRate*100))
				b.WriteString(fmt.Sprintf("- 盈亏比: %.2f\n", s.ProfitFactor))
				b.WriteString(fmt.Sprintf("- 交易次数: %d\n", s.TotalTrades))
			}

			return b.String(), nil
		},
	)
}

// diagnoseStockCodeKeys 是 DiagnoseStock 接受的代码参数键名（按优先级）。
// 实测根因：工具只认 stock_code 一个键，但模型在同一会话里见到的键名并不
// 统一——SaveStockThesis/GetStockThesis 与游资/筹码工具用 stockCode，
// RunVectorizedBacktest 用 stock_codes，而 L4 提示词只写 DiagnoseStock(股票代码)
// 从不点名参数。模型一旦用 stockCode/code/stock_codes 传参或漏传，
// 本工具就误判为空并返回“请提供股票或ETF代码”（601318 本身无论字符串、
// JSON 数字、带 .SH/SH 前缀都能正常解析，与该代码无关）。
var diagnoseStockCodeKeys = []string{"stock_code", "stockCode", "code", "stock_codes", "symbol", "ticker"}

// extractDiagnoseStockCode 从工具调用参数里提取股票代码，兼容模型可能
// 输出的多种键名与形态（字符串、JSON 数字、单元素数组）；全部缺失/为空
// 时返回 ""，由调用方返回“请提供股票或ETF代码”。
func extractDiagnoseStockCode(args string) string {
	for _, key := range diagnoseStockCodeKeys {
		res := gjson.Get(args, key)
		if !res.Exists() {
			continue
		}
		if res.IsArray() {
			for _, el := range res.Array() {
				if v := strings.TrimSpace(el.String()); v != "" {
					return v
				}
			}
			continue
		}
		if v := strings.TrimSpace(res.String()); v != "" {
			return v
		}
	}
	return ""
}

// GetDiagnoseStockTool 对指定股票或ETF执行三维量化深度诊断（VectorBT矩阵回测+资金筹码流向+技术形态+点位测算）
func GetDiagnoseStockTool() bladesTools.Tool {
	return NewDataToolWrapper(
		"DiagnoseStock",
		"【三维量化深度诊断】全面融合 VectorBT 向量化回测、资金筹码流向与技术形态，输出量化评分、操作建议、目标价/止损价与置信胜率。输入股票或ETF代码（如 600671, 510300, 159915）即可一键生成全方位诊断报告。",
		map[string]*types.ParamDef{
			"stock_code": {Type: "string", Desc: "股票或ETF代码，如 600671、510300、159915、000001", Required: true},
		},
		func(args string) (string, error) {
			rawCode := extractDiagnoseStockCode(args)
			if rawCode == "" {
				return "请提供股票或ETF代码", nil
			}
			diag, err := quant.DiagnoseStock(rawCode)
			if err != nil {
				return fmt.Sprintf("三维量化诊断执行失败: %v", err), nil
			}
			if diag == nil {
				return "三维量化诊断返回空数据", nil
			}

			var b strings.Builder
			b.WriteString(fmt.Sprintf("## 🎯 【%s (%s)】三维量化深度诊断报告\n\n", diag.StockName, diag.StockCode))
			b.WriteString(fmt.Sprintf("- **综合量化评分**: `%d / 100`\n", diag.OverallScore))
			b.WriteString(fmt.Sprintf("- **行动建议**: **%s**\n", diag.Suggestion))
			b.WriteString(fmt.Sprintf("- **策略置信胜率**: `%.1f%%`\n\n", diag.Confidence))

			b.WriteString("### 📌 关键点位测算\n\n")
			b.WriteString(fmt.Sprintf("- **当前最新价**: ¥%.2f\n", diag.CurrentPrice))
			if diag.CurrentPrice > 0 {
				b.WriteString(fmt.Sprintf("- **第一目标价**: ¥%.2f (预期空间: +%.1f%%)\n", diag.TargetPrice, ((diag.TargetPrice-diag.CurrentPrice)/diag.CurrentPrice)*100))
				b.WriteString(fmt.Sprintf("- **建议止损价**: ¥%.2f (防守空间: %.1f%%)\n", diag.StopLossPrice, ((diag.StopLossPrice-diag.CurrentPrice)/diag.CurrentPrice)*100))
			} else {
				b.WriteString(fmt.Sprintf("- **第一目标价**: ¥%.2f\n", diag.TargetPrice))
				b.WriteString(fmt.Sprintf("- **建议止损价**: ¥%.2f\n", diag.StopLossPrice))
			}
			b.WriteString(fmt.Sprintf("- **理论盈亏比**: `%.2f : 1`\n\n", diag.RiskRewardRatio))

			b.WriteString("### 📊 三维量化拆解\n\n")
			b.WriteString(fmt.Sprintf("1. **量化回测维度** (`%d/100`): 策略 **%s**，胜率 `%.1f%%`，总收益率 `%.1f%%`，夏普 `%.2f`，最大回撤 `%.1f%%`（%s）。\n",
				diag.QuantDim.Score, diag.QuantDim.StrategyName, diag.QuantDim.WinRatePct, diag.QuantDim.TotalReturnPct, diag.QuantDim.SharpeRatio, diag.QuantDim.MaxDrawdownPct, diag.QuantDim.Evaluation))
			b.WriteString(fmt.Sprintf("2. **资金筹码维度** (`%d/100`): 近5日主力净流入 **%.2f 万元**（超大单占比 `%.1f%%`，态势: %s）。评价: %s\n",
				diag.MoneyDim.Score, diag.MoneyDim.MainNetInflow5D, diag.MoneyDim.SuperLargeNetPct, diag.MoneyDim.RecentInflowTrend, diag.MoneyDim.Evaluation))
			b.WriteString(fmt.Sprintf("3. **技术形态维度** (`%d/100`): 趋势 **%s**，MA20=¥%.2f，ATR(14)=%.2f。评价: %s\n\n",
				diag.TechnicalDim.Score, diag.TechnicalDim.Trend, diag.TechnicalDim.MA20, diag.TechnicalDim.ATR, diag.TechnicalDim.Evaluation))

			if diag.Summary != "" {
				b.WriteString("### 📈 核心诊断摘要\n\n")
				b.WriteString(diag.Summary + "\n\n")
			}
			if len(diag.Risks) > 0 {
				b.WriteString("### ⚠️ 潜在风险警示\n\n")
				for _, rk := range diag.Risks {
					b.WriteString(fmt.Sprintf("- %s\n", rk))
				}
				b.WriteString("\n")
			}
			b.WriteString(fmt.Sprintf("*历史闭环检验: %s*\n", diag.HistoryTrackRecord))

			return b.String(), nil
		},
	)
}

// formatStats 格式化统计结果
func formatStats(stats *quant.AccuracyStats, scope string) string {
	if stats.VerifiedCount == 0 {
		return fmt.Sprintf("## 📊 回测统计（%s）\n\n暂无回测数据。可使用 RunBacktest 触发回测后再查。", scope)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("## 📊 回测统计（%s）\n\n", scope))
	b.WriteString(fmt.Sprintf("- 已验证报告: %d 份\n", stats.VerifiedCount))
	b.WriteString(fmt.Sprintf("- 预测准确: %d 次\n", stats.AccurateCount))
	b.WriteString(fmt.Sprintf("- 综合准确率: **%.1f%%**\n\n", stats.AccuracyRate))
	b.WriteString("### 分方向统计\n\n")
	b.WriteString("| 方向 | 次数 | 准确 | 准确率 |\n")
	b.WriteString("|------|------|------|--------|\n")
	for _, dir := range []string{"看涨", "看跌", "震荡", "中性"} {
		if ds, ok := stats.DirectionStats[dir]; ok && ds.Count > 0 {
			b.WriteString(fmt.Sprintf("| %s | %d | %d | %.1f%% |\n", dir, ds.Count, ds.Accurate, ds.Rate))
		}
	}
	return b.String()
}
