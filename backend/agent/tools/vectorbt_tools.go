package tools

import (
	"blades-stock/backend/quant"
	"blades-stock/backend/agent/types"
	"fmt"
	"strings"

	bladesTools "github.com/go-kratos/blades/tools"
	"github.com/tidwall/gjson"
)

func init() {
	// 注册 VectorBT Handler
}

// GetVectorBacktestTool VectorBT 向量化回测工具
func GetVectorBacktestTool() bladesTools.Tool {
	return NewDataToolWrapper(
		"VectorBacktest",
		"【VectorBT向量化回测】对单只股票运行策略回测。支持MA均线交叉、MACD、RSI、布林带等多种策略，返回夏普比率、最大回撤、胜率等完整指标。适用场景：测试策略参数、对比不同策略表现、寻找最优参数组合。",
		map[string]*types.ParamDef{
			"stock_code": {Type: "string", Desc: "股票代码，如 000001（平安银行）、600519（贵州茅台）", Required: true},
			"strategy": {Type: "string", Desc: "策略类型：MA（均线交叉）、MACD、RSI、BOLL（布林带）、DualThrust", Required: false},
			"fast": {Type: "number", Desc: "快线周期（MA/MACD用），如 5", Required: false},
			"slow": {Type: "number", Desc: "慢线周期（MA/MACD用），如 20", Required: false},
			"signal": {Type: "number", Desc: "信号周期（MACD用），如 9", Required: false},
			"window": {Type: "number", Desc: "窗口周期（RSI/布林带用），如 14", Required: false},
			"init_cash": {Type: "number", Desc: "初始资金，默认 100000", Required: false},
			"stop_loss": {Type: "number", Desc: "止损比例，如 0.05 表示 5%", Required: false},
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
			b.WriteString(fmt.Sprintf("\n**详细数据**: 总收益%.2f%% | 夏普%.4f | 最大回撤%.2f%%\n",
				resp.Stats.TotalReturn*100, resp.Stats.SharpeRatio, resp.Stats.MaxDrawdownPct))

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
			"stock_code": {Type: "string", Desc: "股票代码，如 000001、600519", Required: true},
			"strategy": {Type: "string", Desc: "策略类型：MA（均线）、MACD、RSI", Required: false},
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

