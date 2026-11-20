package data

func appendScreenTools(tools []Tool) []Tool {
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SearchStockByIndicators",
			Description: "根据自然语言筛选股票。可以使用K线形态、技术指标、财务指标等条件选股，支持多只股票查询（用,分隔）。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"words": map[string]any{
						"type":        "string",
						"description": "选股条件描述，支持K线形态、技术指标、财务指标等。",
					},
				},
				Required: []string{"words"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SearchBk",
			Description: "根据自然语言查询板块/概念/指数整体数据。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"words": map[string]any{
						"type":        "string",
						"description": "板块/概念/指数查询条件描述。",
					},
				},
				Required: []string{"words"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SearchETF",
			Description: "根据自然语言查询ETF数据。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"words": map[string]any{
						"type":        "string",
						"description": "ETF查询条件描述。",
					},
				},
				Required: []string{"words"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "InteractiveAnswer",
			Description: "获取投资者与上市公司互动问答的数据,反映当前投资者关注的热点问题",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"page": map[string]any{
						"type":        "string",
						"description": "分页号",
					},
					"pageSize": map[string]any{
						"type":        "string",
						"description": "分页大小",
					},
					"keyWord": map[string]any{
						"type":        "string",
						"description": "搜索关键词（可输入股票名称或者当前热门板块/行业/概念/标的/事件等）",
					},
				},
				Required: []string{"page", "pageSize"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "FilterStocks",
			Description: "根据技术指标或者关注排名或者连涨/连跌天数筛选股票。支持MACD金叉、KDJ金叉、均线排列、K线形态，人气，关注排名，连涨/连跌天数等。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"keyword": map[string]any{
						"type":        "string",
						"description": "股票名称或代码关键词搜索",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "页码，默认1",
					},
					"pageSize": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认20",
					},
					"macdGoldenFork": map[string]any{
						"type":        "boolean",
						"description": "MACD金叉",
					},
					"kdjGoldenFork": map[string]any{
						"type":        "boolean",
						"description": "KDJ金叉",
					},
					"breakThrough": map[string]any{
						"type":        "boolean",
						"description": "放量突破",
					},
					"lowFundsInflow": map[string]any{
						"type":        "boolean",
						"description": "低位资金净流入",
					},
					"highFundsOutflow": map[string]any{
						"type":        "boolean",
						"description": "高位资金净流出",
					},
					"breakUpMa5Days": map[string]any{
						"type":        "boolean",
						"description": "向上突破5日均线",
					},
					"longAvgArray": map[string]any{
						"type":        "boolean",
						"description": "均线多头排列",
					},
					"shortAvgArray": map[string]any{
						"type":        "boolean",
						"description": "均线空头排列",
					},
					"upperLargeVolume": map[string]any{
						"type":        "boolean",
						"description": "连涨放量",
					},
					"downNarrowVolume": map[string]any{
						"type":        "boolean",
						"description": "下跌无量",
					},
					"morningStar": map[string]any{
						"type":        "boolean",
						"description": "早晨之星",
					},
					"eveningStar": map[string]any{
						"type":        "boolean",
						"description": "黄昏之星",
					},
					"upNday": map[string]any{
						"type":        "integer",
						"description": "连涨天数：3/5/8天及以上",
					},
					"downNday": map[string]any{
						"type":        "integer",
						"description": "连跌天数：3/5/8/10/14天及以上",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "QueryStockCodeInfo",
			Description: "查询股票/指数信息(名称、代码、拼音、交易所等)",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"searchWord": map[string]any{
						"type":        "string",
						"description": "股票搜索关键词",
					},
				},
				Required: []string{"searchWord"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "QueryIwencai",
			Description: "同花顺问财行情数据查询。支持自然语言查询股票、ETF、指数等实时价格、涨跌幅、成交量、技术指标等行情数据。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "自然语言查询语句，如：同花顺最新价格、主力资金流向、上证指数行情等",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "分页页码，默认1",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认10",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SelectAStock",
			Description: "A股智能选股(同花顺i问财)。通过自然语言查询进行A股股票筛选，支持行情指标、技术形态、财务指标、行业概念等多条件组合筛选。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "自然语言选股条件",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "分页页码，默认1",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认10",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SelectSector",
			Description: "选板块(同花顺i问财)。通过自然语言查询板块/概念信息。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "自然语言查询板块条件",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "分页页码，默认1",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认10",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "QueryMacro",
			Description: "宏观数据查询(同花顺i问财)。查询GDP、CPI、PPI、利率、汇率、社融、M2、PMI等宏观经济指标数据。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "自然语言查询语句",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "分页页码，默认1",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认10",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "QueryZhishu",
			Description: "指数数据查询(同花顺i问财)。查询上证指数、沪深300、创业板指等指数行情数据。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "自然语言查询语句",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "分页页码，默认1",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认10",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "QueryEvent",
			Description: "事件数据查询(同花顺i问财)。查询业绩预告、增发配股、股权质押、限售解禁、机构调研等事件数据。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "自然语言查询语句",
					},
					"page": map[string]any{
						"type":        "integer",
						"description": "分页页码，默认1",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "每页条数，默认10",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	return tools
}
