package data

func appendAITools(tools []Tool) []Tool {
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SendToDingDing",
			Description: "将指定标题和内容以 Markdown 形式发送到钉钉机器人。用于把分析结果、摘要或通知推送到钉钉群。需在设置中开启钉钉推送并配置机器人 Webhook。通知内容需尽可能精简。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"title": map[string]any{
						"type":        "string",
						"description": "消息标题，会显示为「Blades Stock {title}」",
					},
					"message": map[string]any{
						"type":        "string",
						"description": "消息正文，支持 Markdown 格式，通知内容需尽可能精简",
					},
				},
				Required: []string{"title", "message"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetAIAnalysisHistory",
			Description: "查询历史AI分析报告。可以根据股票代码、股票名称、问题关键词、日期范围等条件筛选历史AI分析记录。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码筛选",
					},
					"stockName": map[string]any{
						"type":        "string",
						"description": "股票名称筛选",
					},
					"question": map[string]any{
						"type":        "string",
						"description": "问题关键词搜索",
					},
					"startDate": map[string]any{
						"type":        "string",
						"description": "开始日期，格式：YYYY-MM-DD",
					},
					"endDate": map[string]any{
						"type":        "string",
						"description": "结束日期，格式：YYYY-MM-DD",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "页码，默认1",
					},
					"pageSize": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认10",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetAIAnalysisDetail",
			Description: "根据ID获取历史AI分析报告的详细内容",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"id": map[string]any{
						"type":        "integer",
						"description": "分析报告ID",
					},
				},
				Required: []string{"id"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetAIAnalysisContent",
			Description: "根据股票代码获取最新的AI分析报告内容",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如：600519.SH、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	return tools
}
