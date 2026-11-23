package tools

import (
	"fmt"
	"strings"

	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"

	bladesTools "github.com/go-kratos/blades/tools"
)

// GetStrategyScreenTool 返回策略评分选股工具。
// 策略通过 YAML 声明（backend/strategy/strategies/*.yaml），评分基于真实 K 线指标计算。
func GetStrategyScreenTool() bladesTools.Tool {
	return types.NewTool(
		"ScreenByStrategy",
		"按策略评分选股：基于真实K线技术指标（均线/MACD/RSI/量比等）对股票池打分排序。支持策略名称或别名，策略包括：趋势跟随（均线多头）、MACD金叉、放量突破、均线回踩、超跌反弹。可选传入股票池代码列表；不传则使用默认股票池。",
		map[string]*types.ParamDef{
			"strategyName": {
				Type:     "string",
				Desc:     "策略名称或别名，如 趋势跟随 / macd_golden_cross / 放量突破",
				Required: true,
			},
			"codeList": {
				Type:     "string",
				Desc:     "待筛选股票代码，逗号分隔（如 600519,000001）；为空则筛选默认股票池",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "返回条数，默认10，最大50",
				Required: false,
			},
		},
		func(args string) (string, error) {
			name := types.ExtractStringParam(args, "strategyName", "strategy", "name")
			if strings.TrimSpace(name) == "" {
				return "请指定策略名称 strategyName", nil
			}
			codes := types.ExtractStockCodes(args)
			limit, _ := types.ExtractIntParam(args, "limit", "count")

			outcome, err := data.ScreenByStrategy(name, codes, limit)
			if err != nil {
				return fmt.Sprintf("策略选股失败: %v", err), nil
			}
			return renderStrategyOutcome(outcome), nil
		},
	)
}

func renderStrategyOutcome(o *data.StrategyScreenOutcome) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\r\n### 策略「%s」评分选股结果（共扫描 %d 只，基础分 %.0f，风险 %s）\r\n",
		o.StrategyName, o.ScannedCount, o.BaseScore, o.RiskLevel))
	b.WriteString(fmt.Sprintf("策略说明：%s\r\n", o.Criteria))

	if len(o.Results) == 0 {
		b.WriteString("未筛选出符合条件的股票（可能因 K 线数据获取失败或无匹配）。")
		return b.String()
	}

	b.WriteString("\r\n| 排名 | 代码 | 名称 | 评分 | 现价 | 涨跌幅 | 量比 | 触发理由 |\r\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- |\r\n")
	for i, r := range o.Results {
		reason := r.Reason
		if reason == "" {
			reason = "仅基础分"
		}
		if len([]rune(reason)) > 80 {
			runes := []rune(reason)
			reason = string(runes[:80]) + "…"
		}
		b.WriteString(fmt.Sprintf("| %d | %s | %s | %.1f | %s | %s | %s | %s |\r\n",
			i+1, r.Code, r.Name, r.Score, r.Close, r.ChangePct, r.VolumeRatio, reason))
	}
	return b.String()
}
