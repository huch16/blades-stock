package data

func appendMarketTools(tools []Tool) []Tool {
	//tools = append(tools, Tool{
	//	Type: "function",
	//	Function: ToolFunction{
	//		Name:        "CailianpressWeb",
	//		Description: "财经新闻资讯搜索",
	//		Parameters: &FunctionParameters{
	//			Type: "object",
	//			Properties: map[string]any{
	//				"searchWords": map[string]any{
	//					"type": "string",
	//					"description": "搜索关键词（不要使用分隔符如空格逗号），为空时返回最新10条新闻资讯" +
	//						"板块/概念名称：半导体\n" +
	//						"股票名称：中科曙光\n" +
	//						"政策：十五五规划\n",
	//				},
	//			},
	//			Required: []string{},
	//		},
	//	},
	//})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetNewsListData",
			Description: "获取新闻资讯",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"keyWord": map[string]any{
						"type":        "string",
						"description": "搜索时的关键词，可为空",
					},
					"startTime": map[string]any{
						"type":        "string",
						"description": "开始时间（如：2026-02-23 00:00:00）",
					},
					"limit": map[string]any{
						"type":        "number",
						"description": "每页条数（未传 page/pageSize 时生效，默认 20）",
					},
					"page": map[string]any{
						"type":        "number",
						"description": "页码，从 1 开始",
					},
					"pageSize": map[string]any{
						"type":        "number",
						"description": "每页条数，与 page 配合使用",
					},
				},
				Required: []string{"startTime"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GlobalStockIndexesReadable",
			Description: "获取全球主要指数概览，并输出为 AI 易读的 Markdown 结构化文本。",
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "AiRecommendStocks",
			Description: "获取近期AI分析/推荐股票明细列表",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"startDate": map[string]any{
						"type":        "string",
						"description": "开始时间（如：2026-02-23 00:00:00）",
					},
					"endDate": map[string]any{
						"type":        "string",
						"description": "结束时间（如：2026-02-26 23:59:59）",
					},
					"page": map[string]any{
						"type":        "string",
						"description": "分页号（如：1）",
					},
					"pageSize": map[string]any{
						"type":        "string",
						"description": "分页大小(如： 1500)",
					},
					"keyWord": map[string]any{
						"type":        "string",
						"description": "搜索关键词",
					},
				},
				Required: []string{"startDate", "endDate", "page", "pageSize"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "StockNotice",
			Description: "获取上市公司公告列表。可查询一只或多只股票的最新公告（如业绩预告、重大事项、募集资金、减持、增持、监管问题、财务异常等），多只股票用英文逗号分隔。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stock_list": map[string]any{
						"type":        "string",
						"description": "股票代码，多只用英文逗号分隔。例如：600584,600900 或 002046,601138",
					},
				},
				Required: []string{"stock_list"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetSecuritiesCompanyOpinion",
			Description: "获取券商/机构的市场分析观点/要点",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"startDate": map[string]any{
						"type":        "string",
						"description": "开始时间（如：2026-02-23）",
					},
					"endDate": map[string]any{
						"type":        "string",
						"description": "结束时间（如：2026-02-26）",
					},
				},
				Required: []string{"startDate", "endDate"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetCurrentTime",
			Description: "获取当前本地时间（格式：YYYY-MM-DD HH:mm:ss）及星期几",
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetMarketData",
			Description: "获取市场行情数据，包括指数行情、涨跌分布和今日申购信息",
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "QueryStockNews",
			Description: "按关键词搜索相关市场资讯/新闻(财联社)",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"searchWords": map[string]any{
						"type":        "string",
						"description": "搜索关键词(多个关键词使用空格分隔)",
					},
				},
				Required: []string{"searchWords"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SearchNews",
			Description: "财经新闻搜索(同花顺i问财)。搜索财经领域新闻资讯，覆盖官媒、主流财经媒体等。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "搜索关键词",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	return tools
}
