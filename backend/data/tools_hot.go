package data

func appendHotTools(tools []Tool) []Tool {
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockConceptInfo",
			Description: "获取股票所属概念详细信息。支持一次查询多只，将并行请求后合并结果。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"code": map[string]any{
						"type":        "string",
						"description": "股票代码,如：601138.SH。注意 上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，港股股票以.HK结尾，北交所股票以.BJ结尾。多只时可用英文逗号分隔。",
					},
					"stockCodes": toolSchemaStockCodes,
				},
				Required: []string{"code"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockInfo",
			Description: "获取股票详细信息，包括实时行情、基本数据等。支持一次查询多只，将并行请求后合并结果。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码（A股：sh,sz开头;港股hk开头,美股：us开头）。多只时可用英文逗号分隔。",
					},
					"stockCodes": toolSchemaStockCodes,
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockMinuteData",
			Description: "获取股票分时数据（当日分钟级成交量和价格）",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如：600519.SH",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetFollowedStocks",
			Description: "获取用户关注/自选的股票列表",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"groupId": map[string]any{
						"type":        "integer",
						"description": "股票分组ID，不传则返回所有关注/自选的股票",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetHotStockList",
			Description: "获取雪球热门股票排行榜",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"marketType": map[string]any{
						"type":        "string",
						"description": "市场类型：全球(10)、沪深(12)、港股(13)、美股(11)，默认10",
					},
					"size": map[string]any{
						"type":        "integer",
						"description": "返回条数，默认20",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetHotEventList",
			Description: "获取雪球热门话题/事件",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"size": map[string]any{
						"type":        "integer",
						"description": "返回条数，默认20",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetIndustryMoneyRank",
			Description: "获取行业资金流向排名（按行业分类）",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"fenlei": map[string]any{
						"type":        "string",
						"description": "行业分类：0=所有行业,1=行业分类,2=概念板块,3=地域板块，默认1",
					},
					"sort": map[string]any{
						"type":        "string",
						"description": "排序字段：netamount=净流入,netbuy=主力净流入,change=涨跌幅，默认netamount",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "返回条数，默认20",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetLongTigerList",
			Description: "获取龙虎榜数据（营业部排行榜）",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式：2026-03-28，默认今天",
					},
				},
				Required: []string{"date"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetEconomicData",
			Description: "获取宏观经济数据，包括GDP、CPI、PPI、PMI等",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"dataType": map[string]any{
						"type":        "string",
						"description": "数据类型：gdp=国内生产总值,cpi=居民消费价格指数,ppi=工业生产者出厂价格指数,pmi=采购经理指数",
					},
				},
				Required: []string{"dataType"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetInvestCalendar",
			Description: "获取投资日历，包括财报发布、股东大会、IPO等重要日期事件",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"yearMonth": map[string]any{
						"type":        "string",
						"description": "年月，格式：2026-03，不传则查询当月",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockNotice",
			Description: "获取个股公告信息",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCodes": map[string]any{
						"type":        "string",
						"description": "股票代码列表，逗号分隔，如：600519,000001",
					},
				},
				Required: []string{"stockCodes"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetHolidayInfo",
			Description: "查询指定日期的节假日信息。返回该日期是否为节假日、节假日名称、是否需要补班等信息。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式：YYYY-MM-DD。不传则查询今天",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "IsTradingDay",
			Description: "判断指定日期是否为A股交易日",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式：YYYY-MM-DD。不传则查询今天",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetNextTradingDay",
			Description: "获取下一个A股交易日",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"startDate": map[string]any{
						"type":        "string",
						"description": "起始日期，格式：YYYY-MM-DD。不传则从今天开始",
					},
					"days": map[string]any{
						"type":        "integer",
						"description": "获取N个交易日后的日期",
					},
				},
			},
		},
	})

	return tools
}
