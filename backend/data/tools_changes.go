package data

func appendChangesTools(tools []Tool) []Tool {
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockChanges",
			Description: "获取股票异动数据，包括火箭发射、快速反弹、大笔买入、封涨停板、加速下跌、高台跳水、大笔卖出、封跌停板等异动类型。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"changeTypes": map[string]any{
						"type":        "string",
						"description": "异动类型，多个用逗号分隔。如：火箭发射,快速反弹,大笔买入,封涨停板,加速下跌,高台跳水,大笔卖出,封跌停板",
					},
					"pageSize": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认20",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockChangeHistoryList",
			Description: "查询股票异动历史记录。可以根据股票代码、异动类型、日期范围等条件筛选历史异动数据。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码筛选，支持模糊匹配",
					},
					"changeType": map[string]any{
						"type":        "integer",
						"description": "异动类型代码",
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
						"description": "每页条数，默认20",
					},
				},
				Required: []string{"startDate", "endDate"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "FinanceSearch",
			Description: "金融资讯搜索(东方财富妙想)。支持自然语言搜索全网最新公告、研报、财经新闻。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "自然语言搜索查询",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "FinancialQA",
			Description: "金融问答。针对金融领域专业问题进行回答，包括股票分析、财务指标解读、投资策略等。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"question": map[string]any{
						"type":        "string",
						"description": "金融相关问题",
					},
				},
				Required: []string{"question"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "HotspotDiscovery",
			Description: "市场热点发现(东方财富妙想)。发现当前A股市场热点板块和题材，包括热点逻辑分析和相关个股。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"question": map[string]any{
						"type":        "string",
						"description": "热点的自然语言描述，如：今日热点、新能源热点、AI概念热点",
					},
				},
				Required: []string{"question"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetUplimitLadder",
			Description: "获取连板梯队数据，包括连板统计和连板梯队详情。适用于分析连板高度、市场情绪、龙头股识别等场景。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式：2026-04-17，默认今天",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetUplimitHotPlates",
			Description: "获取涨停热门板块数据",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式：2026-04-17，默认今天",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetUplimitHotStocks",
			Description: "获取涨停热门个股数据",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式：2026-04-17，默认今天",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetUplimitExplodedStocks",
			Description: "获取炸板(封板失败)个股数据",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式：2026-04-17，默认今天",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetDailyChangeStats",
			Description: "获取近N日每日异动统计趋势，包括每天的上涨异动数、下跌异动数、封涨停数、封跌停数和总异动数。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"days": map[string]any{
						"type":        "integer",
						"description": "查询天数，默认30",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetChangeRank",
			Description: "获取异动排行数据",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"tradeDate": map[string]any{
						"type":        "string",
						"description": "交易日期，格式：YYYY-MM-DD",
					},
					"changeType": map[string]any{
						"type":        "string",
						"description": "异动类型",
					},
				},
				Required: []string{"tradeDate"},
			},
		},
	})

	// FinGenius 式高级分析工具：游资追踪、筹码分析、资金博弈
	// 不需要外部 API Key，直接调用东方财富 F10 免费接口
	for _, def := range []struct {
		name        string
		description string
	}{
		{"AnalyzeHotMoneyActivity", "游资活跃度追踪分析：针对指定股票，综合龙虎榜买入/卖出数据、营业部买卖明细，分析哪些游资营业部在参与、买卖力度、游资风格（一日游/趋势/接力），辅助判断游资动向和资金博弈格局。"},
		{"AnalyzeChipConcentration", "筹码集中度与主力动向分析：针对指定股票，综合股东户数趋势、大宗交易折溢价、资金流向（主力/超大单/大单净额）、融资融券余额变化等多维度数据，判断筹码是集中还是分散，识别主力可能的建仓、洗盘、出货行为。"},
		{"AnalyzeCapitalGame", "资金博弈分析：针对指定股票，综合历史资金流向（主力净额、超大单、大单、中单、小单的净流入/流出）、龙虎榜买卖力度、融资融券余额变化等多维度，分析多空双方力量对比、主力与散户的博弈格局，判断当前主导资金性质。"},
	} {
		tools = append(tools, Tool{
			Type: "function",
			Function: ToolFunction{
				Name:        def.name,
				Description: def.description,
				Parameters: &FunctionParameters{
					Type: "object",
					Properties: map[string]any{
						"stockCode": map[string]any{
							"type":        "string",
							"description": "股票代码，如 600519、000001.SZ",
						},
						"days": map[string]any{
							"type":        "integer",
							"description": "回溯天数，AnalyzeHotMoneyActivity 默认30天，AnalyzeCapitalGame 默认20个交易日",
						},
					},
					Required: []string{"stockCode"},
				},
			},
		})
	}

	return tools
}
