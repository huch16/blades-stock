package tools

import (
	"fmt"
	"go-stock/backend/agent/types"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"strings"

	bladesTools "github.com/go-kratos/blades/tools"
	"github.com/tidwall/gjson"
)

func init() {
	RegisterBacktestHandler(handleRunBacktest)
}

var runBacktestHandler func() string

func RegisterBacktestHandler(h func() string) {
	runBacktestHandler = h
}

func GetBacktestStatsTool() bladesTools.Tool {
	return NewDataToolWrapper(
		"GetBacktestStats",
		"获取AI分析回测统计（验证历史预测准确性）",
		map[string]*types.ParamDef{
			"stock_code": {Type: "string", Desc: "股票代码（可选，指定则返回该股票统计）"},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stock_code").String()

			var totalVerified int64
			var totalAccurate int64
			q := db.Dao.Model(&models.BacktestResult{})
			if stockCode != "" {
				q = q.Where("stock_code = ?", stockCode)
			}
			q.Count(&totalVerified)

			if totalVerified == 0 {
				return "暂无回测数据。可使用 RunBacktest 触发回测后再查。", nil
			}

			aq := db.Dao.Model(&models.BacktestResult{})
			if stockCode != "" {
				aq = aq.Where("stock_code = ? AND is_accurate = ?", stockCode, true)
			} else {
				aq = aq.Where("is_accurate = ?", true)
			}
			aq.Count(&totalAccurate)

			rate := float64(totalAccurate) / float64(totalVerified) * 100

			var dirStats []struct {
				Direction string
				Count     int
				Accurate  int
			}
			db.Dao.Model(&models.BacktestResult{}).
				Select("direction, COUNT(*) as count, SUM(CASE WHEN is_accurate = 1 THEN 1 ELSE 0 END) as accurate").
				Group("direction").
				Find(&dirStats)

			var b strings.Builder
			scope := "全部股票"
			if stockCode != "" {
				scope = stockCode
			}
			b.WriteString(fmt.Sprintf("## 📊 回测统计（%s）\n\n", scope))
			b.WriteString(fmt.Sprintf("- 已验证报告: %d 份\n", totalVerified))
			b.WriteString(fmt.Sprintf("- 预测准确: %d 次\n", totalAccurate))
			b.WriteString(fmt.Sprintf("- 综合准确率: **%.1f%%**\n\n", rate))
			b.WriteString("### 分方向统计\n\n")
			b.WriteString("| 方向 | 次数 | 准确 | 准确率 |\n")
			b.WriteString("|------|------|------|--------|\n")
			for _, ds := range dirStats {
				dr := float64(0)
				if ds.Count > 0 {
					dr = float64(ds.Accurate) / float64(ds.Count) * 100
				}
				b.WriteString(fmt.Sprintf("| %s | %d | %d | %.1f%% |\n", ds.Direction, ds.Count, ds.Accurate, dr))
			}

			return b.String(), nil
		},
	)
}

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

			var result models.BacktestResult
			err := db.Dao.Where("report_id = ?", reportID).First(&result).Error
			if err != nil {
				return "该报告暂无回测结果，可使用 RunBacktest 触发回测", nil
			}

			accurateMark := "❌"
			if result.IsAccurate {
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
				result.StockName, result.StockCode,
				result.AnalysisDate,
				result.Direction,
				result.TargetPrice,
				result.VerifyDate,
				result.VerifyPrice,
				result.PriceChange,
				result.MaxUpside,
				result.MaxDrawdown,
				accurateMark,
				result.Confidence*100,
				result.Summary,
			), nil
		},
	)
}

func GetRunBacktestTool() bladesTools.Tool {
	return NewDataToolWrapper(
		"RunBacktest",
		"触发AI分析报告回测（将历史分析与后续真实走势对比验证）",
		map[string]*types.ParamDef{
			"start_date": {Type: "string", Desc: "起始日期（可选，如 2024-01-01）"},
			"end_date":   {Type: "string", Desc: "结束日期（可选，如 2024-12-31）"},
		},
		func(args string) (string, error) {
			if runBacktestHandler != nil {
				result := runBacktestHandler()
				return result, nil
			}
			return "回测服务未就绪", nil
		},
	)
}

func handleRunBacktest() string {
	var count int64
	db.Dao.Model(&models.AIResponseResult{}).
		Where("is_del = 0").
		Where("NOT EXISTS (SELECT 1 FROM backtest_results WHERE backtest_results.report_id = ai_response_result.id)").
		Count(&count)

	if count == 0 {
		return "所有分析报告已完成回测，无需重复执行"
	}

	return fmt.Sprintf("发现 %d 份待回测报告，请在AI配置中启用回测功能后重试，或手动运行 RunBacktest", count)
}
