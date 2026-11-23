package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	"blades-stock/backend/models"

	bladesTools "github.com/go-kratos/blades/tools"
)

// GetStockThesisTool 查询标的当前买方认知假说与最新判定
func GetStockThesisTool() bladesTools.Tool {
	return types.NewTool(
		"GetStockThesis",
		"查询单只股票当前的买方认知假说(Thesis)、最新定性判定(逻辑强化/新论点/维持现状/逻辑削弱/逻辑证伪)、重点跟踪催化及证伪条件",
		map[string]*types.ParamDef{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如 600519, 000001, sh600519",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := types.ExtractStockCode(args)
			if stockCode == "" {
				return "❌ 股票代码不能为空", nil
			}

			api := data.NewStockThesisApi()
			thesis, err := api.GetStockThesis(stockCode)
			if err != nil {
				return fmt.Sprintf("❌ 查询标的认知档案失败: %v", err), nil
			}
			if thesis == nil || thesis.ID == 0 {
				return fmt.Sprintf("标的 %s 暂无已沉淀的买方认知档案。建议在分析后按照标准买方框架（判定结果、核心假说、验证催化、证伪条件）建立首期档案。", stockCode), nil
			}

			sb := strings.Builder{}
			sb.WriteString(fmt.Sprintf("### 🎯 %s (%s) 买方认知档案\n", thesis.StockName, thesis.StockCode))
			sb.WriteString(fmt.Sprintf("- **当前判定**：%s (%s)\n", thesis.VerdictText, thesis.Verdict))
			sb.WriteString(fmt.Sprintf("- **核心投资假说**：%s\n", thesis.CoreThesis))
			if thesis.KeyCatalysts != "" {
				sb.WriteString(fmt.Sprintf("- **重点跟踪催化**：%s\n", thesis.KeyCatalysts))
			}
			if thesis.RiskFactors != "" {
				sb.WriteString(fmt.Sprintf("- **关键证伪条件**：%s\n", thesis.RiskFactors))
			}
			sb.WriteString(fmt.Sprintf("- **更新时间**：%s\n", thesis.UpdatedAt.Format("2006-01-02 15:04:05")))
			if thesis.ModelName != "" {
				sb.WriteString(fmt.Sprintf("- **推演模型**：%s\n", thesis.ModelName))
			}

			return sb.String(), nil
		},
	)
}

// SaveStockThesisTool 保存或修正标的买方认知假说
func SaveStockThesisTool() bladesTools.Tool {
	return types.NewTool(
		"SaveStockThesis",
		"保存、修正或确认单只股票的买方投资假说(Thesis)与定性判定，并自动沉淀至认知演化时间线",
		map[string]*types.ParamDef{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如 600519, 000001",
				Required: true,
			},
			"stockName": {
				Type:     "string",
				Desc:     "股票名称，如 贵州茅台, 平安银行",
				Required: false,
			},
			"verdict": {
				Type:     "string",
				Desc:     "定性判定：逻辑强化 / 维持现状 / 逻辑削弱 / 逻辑证伪 / 新论点",
				Required: true,
			},
			"coreThesis": {
				Type:     "string",
				Desc:     "核心投资逻辑与假说（1-2句话精炼概括）",
				Required: true,
			},
			"keyCatalysts": {
				Type:     "string",
				Desc:     "下一阶段重点跟踪的验证催化事件或指标",
				Required: false,
			},
			"riskFactors": {
				Type:     "string",
				Desc:     "投资假说的打破或证伪条件",
				Required: false,
			},
		},
		func(args string) (string, error) {
			stockCode := types.ExtractStockCode(args)
			if stockCode == "" {
				return "❌ 股票代码不能为空", nil
			}

			stockName := types.ExtractStringParam(args, "stockName", "name", "stock_name")
			verdictRaw := types.ExtractStringParam(args, "verdict")
			coreThesis := types.ExtractStringParam(args, "coreThesis")
			keyCatalysts := types.ExtractStringParam(args, "keyCatalysts")
			riskFactors := types.ExtractStringParam(args, "riskFactors")

			api := data.NewStockThesisApi()
			_, _, _, _, _ = api.ExtractThesisFromContent(verdictRaw)
			// 标准化 verdict
			var verdict, verdictText string
			switch {
			case strings.Contains(verdictRaw, "强化"):
				verdict, verdictText = models.VerdictThesisStrengthen, "逻辑强化"
			case strings.Contains(verdictRaw, "削弱"):
				verdict, verdictText = models.VerdictThesisWeaken, "逻辑削弱"
			case strings.Contains(verdictRaw, "证伪"):
				verdict, verdictText = models.VerdictThesisBroken, "逻辑证伪"
			case strings.Contains(verdictRaw, "新论点") || strings.Contains(verdictRaw, "新假说") || strings.Contains(verdictRaw, "首次"):
				verdict, verdictText = models.VerdictNewThesis, "新论点"
			default:
				verdict, verdictText = models.VerdictNoChange, "维持现状"
			}

			item := &models.StockThesisLedger{
				StockCode:    stockCode,
				StockName:    stockName,
				Verdict:      verdict,
				VerdictText:  verdictText,
				CoreThesis:   coreThesis,
				KeyCatalysts: keyCatalysts,
				RiskFactors:  riskFactors,
				ModelName:    "Agent交互沉淀",
			}

			if err := api.SaveStockThesis(item); err != nil {
				return fmt.Sprintf("❌ 保存买方认知档案失败: %v", err), nil
			}

			resJSON, _ := json.MarshalIndent(map[string]any{
				"status":       "success",
				"stockCode":    stockCode,
				"verdict":      verdictText,
				"coreThesis":   coreThesis,
				"keyCatalysts": keyCatalysts,
				"riskFactors":  riskFactors,
			}, "", "  ")

			return fmt.Sprintf("✅ 标的 %s (%s) 买方认知档案已成功保存并沉淀到演化时间线：\n```json\n%s\n```",
				stockCode, stockName, string(resJSON)), nil
		},
	)
}
