package data

import (
	"fmt"
	"strings"

	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/models"
	"blades-stock/backend/util"

	bladesTools "github.com/go-kratos/blades/tools"
)

func RegisterAITools() []bladesTools.Tool {
	var result []bladesTools.Tool

	result = append(result, NewDataToolWrapper(
		"GetAIAnalysisHistory",
		"查询历史AI分析报告。可以根据股票代码、股票名称、问题关键词、日期范围等条件筛选历史AI分析记录。",
		map[string]*types.ParamDef{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码筛选（可选）",
				Required: false,
			},
			"stockName": {
				Type:     "string",
				Desc:     "股票名称筛选（可选）",
				Required: false,
			},
			"question": {
				Type:     "string",
				Desc:     "问题关键词搜索（可选）",
				Required: false,
			},
			"modelName": {
				Type:     "string",
				Desc:     "AI模型名称筛选（可选）",
				Required: false,
			},
			"startDate": {
				Type:     "string",
				Desc:     "开始日期，格式：YYYY-MM-DD（可选）",
				Required: false,
			},
			"endDate": {
				Type:     "string",
				Desc:     "结束日期，格式：YYYY-MM-DD（可选）",
				Required: false,
			},
			"page": {
				Type:     "integer",
				Desc:     "页码，默认1",
				Required: false,
			},
			"pageSize": {
				Type:     "integer",
				Desc:     "每页条数，默认10",
				Required: false,
			},
		},
		func(args string) (string, error) {
			stockCode := types.ExtractStockCode(args)
			stockName := types.ExtractStringParam(args, "stockName", "stock_name", "name")
			question := types.ExtractStringParam(args, "question", "query", "keyword")
			modelName := types.ExtractStringParam(args, "modelName", "model_name", "model")
			startDate := types.ExtractStringParam(args, "startDate", "start_date")
			endDate := types.ExtractStringParam(args, "endDate", "end_date")
			page, _ := types.ExtractIntParam(args, "page", "page_number")
			pageSize, _ := types.ExtractIntParam(args, "pageSize", "page_size")

			if page <= 0 {
				page = 1
			}
			if pageSize <= 0 || pageSize > 50 {
				pageSize = 10
			}

			pageData, svcErr := data.NewAIResponseResultService().GetAIResponseResultList(models.AIResponseResultQuery{
				StockCode: stockCode,
				StockName: stockName,
				Question:  question,
				ModelName: modelName,
				StartDate: startDate,
				EndDate:   endDate,
				Page:      page,
				PageSize:  pageSize,
			})
			if svcErr != nil {
				return "", svcErr
			}

			if pageData == nil || len(pageData.List) == 0 {
				return "未找到符合条件的历史分析报告", nil
			}

			type historyRow struct {
				ID         uint   `md:"ID"`
				StockCode  string `md:"股票代码"`
				StockName  string `md:"股票名称"`
				Question   string `md:"问题"`
				ModelName  string `md:"模型"`
				CreateTime string `md:"创建时间"`
			}

			var rows []historyRow
			for _, item := range pageData.List {
				questionText := item.Question
				if len(questionText) > 50 {
					questionText = questionText[:50] + "..."
				}
				rows = append(rows, historyRow{
					ID:         item.ID,
					StockCode:  item.StockCode,
					StockName:  item.StockName,
					Question:   questionText,
					ModelName:  item.ModelName,
					CreateTime: item.CreatedAt.Format("2006-01-02 15:04:05"),
				})
			}

			summary := fmt.Sprintf("共找到 %d 条历史分析报告，当前第 %d/%d 页", pageData.Total, page, pageData.TotalPages)
			return summary + "\n\n" + util.MarkdownTableWithTitle("历史AI分析报告", rows), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetAIAnalysisDetail",
		"根据ID获取历史AI分析报告的详细内容",
		map[string]*types.ParamDef{
			"id": {
				Type:     "integer",
				Desc:     "分析报告ID",
				Required: true,
			},
		},
		func(args string) (string, error) {
			idInt, _ := types.ExtractIntParam(args, "id", "recordId")
			id := uint(idInt)
			if id == 0 {
				return "请提供有效的分析报告ID", nil
			}

			var result models.AIResponseResult
			err := db.Dao.First(&result, id).Error
			if err != nil {
				return "未找到该分析报告", nil
			}

			var md strings.Builder
			md.WriteString("### AI分析报告详情\n\n")
			md.WriteString("| 项目 | 内容 |\n| --- | --- |\n")
			md.WriteString(fmt.Sprintf("| ID | %d |\n", result.ID))
			md.WriteString(fmt.Sprintf("| 股票代码 | %s |\n", result.StockCode))
			md.WriteString(fmt.Sprintf("| 股票名称 | %s |\n", result.StockName))
			md.WriteString(fmt.Sprintf("| 模型 | %s |\n", result.ModelName))
			md.WriteString(fmt.Sprintf("| 创建时间 | %s |\n", result.CreatedAt.Format("2006-01-02 15:04:05")))
			md.WriteString(fmt.Sprintf("\n#### 问题\n\n%s\n", result.Question))
			md.WriteString(fmt.Sprintf("\n#### AI分析结果\n\n%s\n", result.Content))

			return md.String(), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetAIAnalysisContent",
		"根据股票代码获取最新的AI分析报告内容。直接返回该股票最近一次AI分析的完整内容。",
		map[string]*types.ParamDef{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如：601138.SH；须与stockName双向匹配（来自工具真实数据），格式非法或代码-名称不一致时禁止填写",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := types.ExtractStockCode(args)
			if stockCode == "" {
				return "请提供股票代码", nil
			}

			var result models.AIResponseResult
			err := db.Dao.Where("stock_code = ?", stockCode).Order("created_at DESC").First(&result).Error
			if err != nil {
				return fmt.Sprintf("未找到 %s 的历史分析报告", stockCode), nil
			}

			var md strings.Builder
			md.WriteString(fmt.Sprintf("### %s (%s) AI分析报告\n\n", result.StockName, result.StockCode))
			md.WriteString("| 项目 | 内容 |\n| --- | --- |\n")
			md.WriteString(fmt.Sprintf("| 报告ID | %d |\n", result.ID))
			md.WriteString(fmt.Sprintf("| 模型 | %s |\n", result.ModelName))
			md.WriteString(fmt.Sprintf("| 分析时间 | %s |\n", result.CreatedAt.Format("2006-01-02 15:04:05")))
			md.WriteString(fmt.Sprintf("\n#### 问题\n\n%s\n", result.Question))
			md.WriteString(fmt.Sprintf("\n#### AI分析结果\n\n%s\n", result.Content))

			return md.String(), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"SendDingDingMessage",
		"发送消息到钉钉机器人",
		map[string]*types.ParamDef{
			"message": {
				Type:     "string",
				Desc:     "要发送的消息内容，支持 Markdown 格式",
				Required: true,
			},
		},
		func(args string) (string, error) {
			message := types.ExtractStringParam(args, "message", "text", "content")
			if message == "" {
				return "消息内容不能为空", nil
			}
			result := data.NewDingDingAPI().SendToDingDing("通知", message)
			return result, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"SendToDingDing",
		"将指定标题和内容以 Markdown 形式发送到钉钉机器人",
		map[string]*types.ParamDef{
			"title": {
				Type:     "string",
				Desc:     "消息标题，会显示为「Blades Stock {title}」",
				Required: true,
			},
			"message": {
				Type:     "string",
				Desc:     "消息正文，支持 Markdown 格式，通知内容需尽可能精简",
				Required: true,
			},
		},
		func(args string) (string, error) {
			title := types.ExtractStringParam(args, "title", "name", "subject")
			message := types.ExtractStringParam(args, "message", "text", "content")
			if title == "" || message == "" {
				return "标题和消息内容不能为空", nil
			}
			result := data.NewDingDingAPI().SendToDingDing(title, message)
			return result, nil
		},
	))

	return result
}
