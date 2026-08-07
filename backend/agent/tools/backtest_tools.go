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
			"stock_code":   {Type: "string", Desc: "股票代码，如 000001（平安银行）、600519（贵州茅台）", Required: true},
			"strategy":     {Type: "string", Desc: "策略类型：MA（均线交叉）、MACD、RSI、BOLL（布林带）、DualThrust", Required: false},
			"fast":         {Type: "number", Desc: "快线周期（MA/MACD用），如 5", Required: false},
			"slow":         {Type: "number", Desc: "慢线周期（MA/MACD用），如 20", Required: false},
			"signal":       {Type: "number", Desc: "信号周期（MACD用），如 9", Required: false},
			"window":       {Type: "number", Desc: "窗口周期（RSI/布林带用），如 14", Required: false},
			"init_cash":    {Type: "number", Desc: "初始资金，默认 100000", Required: false},
			"stop_loss":    {Type: "number", Desc: "止损比例，如 0.05 表示 5%", Required: false},
			"take_profit":  {Type: "number", Desc: "止盈比例，如 0.10 表示 10%", Required: false},
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
