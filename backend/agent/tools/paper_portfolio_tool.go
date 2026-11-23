package tools

import (
	"fmt"
	"strings"

	"blades-stock/backend/agent/types"
	"blades-stock/backend/quant"

	bladesTools "github.com/go-kratos/blades/tools"
)

// GetPaperPortfolioSummaryTool 查询智能体专属模拟盘账户资产、净值与持仓问责报表
func GetPaperPortfolioSummaryTool() bladesTools.Tool {
	return types.NewTool(
		"GetPaperPortfolioSummary",
		"查询智能体专属模拟基金账户（财智1号买方量化先锋）的实时单位净值(NAV)、累计收益率、夏普比率、最大回撤、可用现金、持仓明细与胜率问责数据",
		map[string]*types.ParamDef{},
		func(args string) (string, error) {
			mgr := quant.NewPaperPortfolioManager()
			// 先刷新一次净值
			_, _ = mgr.UpdateDailyNAV()
			stats, err := mgr.GetPortfolioStats()
			if err != nil {
				return fmt.Sprintf("❌ 查询模拟基金账户失败: %v", err), nil
			}

			p := stats.Portfolio
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("### 🏆 %s 收益率问责与模拟盘净值简报\n\n", p.Name))
			fmt.Fprintf(&sb, "- **单位净值 (NAV)**：`%.4f` (初始: 1.0000)\n", p.CurrentNAV)
			fmt.Fprintf(&sb, "- **累计总收益率**：`%+.2f%%`\n", p.TotalReturnRate)
			fmt.Fprintf(&sb, "- **年化夏普比率 (Sharpe)**：`%.2f`\n", p.SharpeRatio)
			fmt.Fprintf(&sb, "- **历史最大动态回撤**：`%.2f%%` (历史最高NAV: %.4f)\n", p.MaxDrawdown, p.PeakNAV)
			fmt.Fprintf(&sb, "- **实盘交易胜率**：`%.1f%%` (总平仓 %d 笔: 盈利 %d 笔 / 亏损 %d 笔)\n",
				p.WinRate, p.TotalTrades, p.WinningTrades, p.LosingTrades)
			fmt.Fprintf(&sb, "- **账户资产总览**：总资产 `%.2f` 万元 | 可用现金 `%.2f` 万元 (本金: %.2f 万元)\n\n",
				p.TotalAssetValue/10000.0, p.CashBalance/10000.0, p.InitialCash/10000.0)

			sb.WriteString("#### 📦 当前真实持仓组合\n")
			if len(stats.Positions) == 0 {
				sb.WriteString("*(当前空仓，全部为现金防御状态)*\n")
			} else {
				sb.WriteString("| 标的代码 | 股票名称 | 持股数 | 成本均价 | 最新市价 | 市值(万元) | 浮动盈亏 | 仓位占比 | 止损线 |\n")
				sb.WriteString("| :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- | :--- |\n")
				for _, pos := range stats.Positions {
					fmt.Fprintf(&sb, "| `%s` | **%s** | %d股 | %.2f | %.2f | %.2f | %+.2f%% | %.1f%% | %.2f |\n",
						pos.StockCode, pos.StockName, pos.Shares, pos.AvgCostPrice, pos.CurrentPrice,
						pos.MarketValue/10000.0, pos.FloatingPnLRate, pos.Weight, pos.StopLossPrice)
				}
			}

			if len(stats.RecentOrders) > 0 {
				sb.WriteString("\n#### ⏱️ 最近成交交易记录\n")
				limit := 5
				if len(stats.RecentOrders) < limit {
					limit = len(stats.RecentOrders)
				}
				for i := 0; i < limit; i++ {
					o := stats.RecentOrders[i]
					action := "买入建仓"
					if o.Side == "SELL" {
						action = fmt.Sprintf("卖出平仓 (实现盈亏 %+.2f元)", o.RealizedPnL)
					}
					fmt.Fprintf(&sb, "- `%s` %s %s(%s) %d股, 成交价: %.2f元, 动因: %s\n",
						o.OrderTime.Format("01-02 15:04"), action, o.StockName, o.StockCode, o.Shares, o.FilledPrice, o.Reason)
				}
			}

			return sb.String(), nil
		},
	)
}

// ExecutePaperTradeTool 执行虚拟交易下单
func ExecutePaperTradeTool() bladesTools.Tool {
	return types.NewTool(
		"ExecutePaperTrade",
		"在智能体专属模拟基金账户中执行虚拟买入(BUY)或卖出(SELL)指令，具备自动滑点模拟、单票最高15%风控限制与止盈止损线自动绑定",
		map[string]*types.ParamDef{
			"side": {
				Type:     "string",
				Desc:     "交易方向：BUY (买入开仓/加仓), SELL (卖出平仓/减仓)",
				Required: true,
			},
			"stockCode": {
				Type:     "string",
				Desc:     "标的代码，如 002463, 300308, 600519",
				Required: true,
			},
			"stockName": {
				Type:     "string",
				Desc:     "标的名称，如 沪电股份, 中际旭创",
				Required: false,
			},
			"price": {
				Type:     "number",
				Desc:     "委托基准价格 (元)",
				Required: true,
			},
			"shares": {
				Type:     "integer",
				Desc:     "交易股数 (仅卖出时生效，留空或0则全部平仓)",
				Required: false,
			},
			"stopLoss": {
				Type:     "number",
				Desc:     "主动防守止损价 (元)",
				Required: false,
			},
			"takeProfit": {
				Type:     "number",
				Desc:     "目标止盈价 (元)",
				Required: false,
			},
			"targetWeight": {
				Type:     "number",
				Desc:     "买入仓位占比 (0.01~0.15，默认 0.10 即 10% 仓位)",
				Required: false,
			},
			"reason": {
				Type:     "string",
				Desc:     "下单动因: AI_RECOMMEND, STOP_LOSS, TAKE_PROFIT, THESIS_BROKEN",
				Required: false,
			},
		},
		func(args string) (string, error) {
			side := strings.ToUpper(types.ExtractStringParam(args, "side", "direction", "action"))
			stockCode := types.ExtractStockCode(args)
			stockName := types.ExtractStringParam(args, "stockName", "name", "stock_name")
			price, _ := types.ExtractFloatParam(args, "price")
			shares, _ := types.ExtractIntParam(args, "shares", "quantity")
			stopLoss, _ := types.ExtractFloatParam(args, "stopLoss", "stop_loss")
			takeProfit, _ := types.ExtractFloatParam(args, "takeProfit", "take_profit")
			targetWeight, _ := types.ExtractFloatParam(args, "targetWeight", "target_weight")
			reason := types.ExtractStringParam(args, "reason")

			if stockCode == "" {
				return "❌ 股票代码不能为空", nil
			}
			if price <= 0 {
				return "❌ 委托价格必须大于 0", nil
			}

			mgr := quant.NewPaperPortfolioManager()
			if side == "BUY" {
				order, err := mgr.ExecuteVirtualBuy(stockCode, stockName, price, stopLoss, takeProfit, targetWeight, reason)
				if err != nil {
					return fmt.Sprintf("❌ 虚拟买入委托失败: %v", err), nil
				}
				return fmt.Sprintf("✅ 虚拟买入委托撮合成交成功！\n- 标的：%s(%s)\n- 成交股数：%d 股 (成交均价 %.2f 元，滑点千分之一)\n- 占用资金：%.2f 元 (含佣金 %.2f 元)\n- 设定止损线：%.2f 元 | 目标止盈：%.2f 元",
					order.StockName, order.StockCode, order.Shares, order.FilledPrice, order.Amount, order.Fee, stopLoss, takeProfit), nil
			} else if side == "SELL" {
				order, err := mgr.ExecuteVirtualSell(stockCode, price, shares, reason)
				if err != nil {
					return fmt.Sprintf("❌ 虚拟卖出平仓失败: %v", err), nil
				}
				return fmt.Sprintf("✅ 虚拟卖出平仓成功！\n- 标的：%s(%s)\n- 卖出股数：%d 股 (成交价 %.2f 元)\n- 回笼现金：%.2f 元\n- 本笔已实现盈亏：%+.2f 元 (扣减印花税与佣金 %.2f 元)",
					order.StockName, order.StockCode, order.Shares, order.FilledPrice, order.Amount-order.Fee, order.RealizedPnL, order.Fee), nil
			} else {
				return "❌ 无效的交易方向，仅支持 BUY 或 SELL", nil
			}
		},
	)
}
