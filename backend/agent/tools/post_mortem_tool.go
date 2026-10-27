package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"blades-stock/backend/agent/types"
	"blades-stock/backend/quant"

	bladesTools "github.com/go-kratos/blades/tools"
	"github.com/tidwall/gjson"
)

// TriggerPostMortemAuditTool 触发 T+1/T+5/T+20 决策胜率闭环对账与死后剖析
func TriggerPostMortemAuditTool() bladesTools.Tool {
	return types.NewTool(
		"TriggerPostMortemAudit",
		"触发 AI 推荐与买方立论的闭环胜率对账复盘（Self-Audit），自动检测是否击穿止损线，对失败案例执行死后剖析(Post-Mortem)并沉淀经验结晶",
		map[string]*types.ParamDef{
			"targetDate": {
				Type:     "string",
				Desc:     "复盘评估日期 (YYYY-MM-DD)，留空则默认今天",
				Required: false,
			},
		},
		func(args string) (string, error) {
			targetDate := gjson.Get(args, "targetDate").String()
			service := quant.NewPostMortemLoopService()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			sum, err := service.RunDailyAudit(ctx, targetDate)
			if err != nil {
				return fmt.Sprintf("❌ 执行死后剖析对账失败: %v", err), nil
			}

			var sb strings.Builder
			sb.WriteString("### 📊 AI 决策闭环对账与死后剖析 (Post-Mortem) 报表\n\n")
			fmt.Fprintf(&sb, "- **已对账历史样本数**：%d\n", sum.TotalAudited)
			fmt.Fprintf(&sb, "- **胜率表现**：T+1 胜率 `%.1f%%` | T+5 胜率 `%.1f%%` | T+20 胜率 `%.1f%%`\n",
				sum.WinRateT1, sum.WinRateT5, sum.WinRateT20)
			fmt.Fprintf(&sb, "- **达标盈利单**：%d 笔 | **失误/击穿止损单**：%d 笔 (其中硬止损击穿 %d 笔)\n",
				sum.SuccessCount, sum.FailureCount, sum.StopLossHits)
			fmt.Fprintf(&sb, "- **历史平均周期收益率**：%+.2f%%\n", sum.AvgReturn)
			fmt.Fprintf(&sb, "- **自动凝练避险结晶 (Crystals)**：%d 条已写入认知库\n", sum.CrystalsGenerated)

			records, _ := service.GetRecentAuditRecords(5)
			if len(records) > 0 {
				sb.WriteString("\n#### 最近 5 笔重点死后剖析与反思记录\n")
				for _, r := range records {
					statusBadge := "✅ 达标"
					if r.Status == "AUDITED_FAILURE" {
						statusBadge = "❌ 止损击穿"
					}
					fmt.Fprintf(&sb, "- **[%s] %s(%s) T+%d**：%s | 实际收益: %+.2f%% | 最低探底: %.2f元\n",
						statusBadge, r.StockName, r.StockCode, r.HorizonDays, r.InitialDate, r.ActualReturn, r.LowestPrice)
					if r.ReflectionNotes != "" {
						fmt.Fprintf(&sb, "  > 剖析反思: %s\n", r.ReflectionNotes)
					}
				}
			}

			return sb.String(), nil
		},
	)
}

// GetPostMortemAuditHistoryTool 查询历史闭环审计与死后剖析记录
func GetPostMortemAuditHistoryTool() bladesTools.Tool {
	return types.NewTool(
		"GetPostMortemAuditHistory",
		"查询买方决策历史胜率回溯与死后剖析反思记录，用于评估过往研判准确率并汲取避险教训",
		map[string]*types.ParamDef{
			"limit": {
				Type:     "integer",
				Desc:     "查询条数，默认 10，上限 30",
				Required: false,
			},
		},
		func(args string) (string, error) {
			limit := int(gjson.Get(args, "limit").Int())
			if limit <= 0 {
				limit = 10
			}
			service := quant.NewPostMortemLoopService()
			records, err := service.GetRecentAuditRecords(limit)
			if err != nil {
				return fmt.Sprintf("❌ 查询对账历史失败: %v", err), nil
			}

			if len(records) == 0 {
				return "暂无历史闭环审计记录。可调用 TriggerPostMortemAudit 立即执行首期对账。", nil
			}

			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("### 📜 最近 %d 条决策胜率对账与反思档案\n\n", len(records)))
			for i, r := range records {
				tag := "🟢 达标"
				if r.Status == "AUDITED_FAILURE" {
					tag = "🔴 失败"
				}
				fmt.Fprintf(&sb, "%d. **[%s] %s(%s)** (推荐日: %s, 周期: T+%d)\n",
					i+1, tag, r.StockName, r.StockCode, r.InitialDate, r.HorizonDays)
				fmt.Fprintf(&sb, "   - 初始价: %.2f | 止损线: %.2f | 实际收益率: %+.2f%% | 最大回撤: %.2f%%\n",
					r.InitialPrice, r.InitialStopLoss, r.ActualReturn, r.MaxDrawdown)
				if r.ReflectionNotes != "" {
					fmt.Fprintf(&sb, "   - **死后剖析**: %s\n", r.ReflectionNotes)
				}
				if r.LessonsLearned != "" {
					fmt.Fprintf(&sb, "   - **教训提炼**: %s\n", r.LessonsLearned)
				}
			}
			return sb.String(), nil
		},
	)
}
