package data

func appendFinanceTools(tools []Tool) []Tool {
	//tools = append(tools, Tool{
	//	Type: "function",
	//	Function: ToolFunction{
	//		Name:        "QueryBKDictInfo",
	//		Description: "获取所有板块/行业名称或者代码(bkCode,bkName)",
	//	},
	//})

	//tools = append(tools, Tool{
	//	Type: "function",
	//	Function: ToolFunction{
	//		Name:        "GetIndustryResearchReport",
	//		Description: "获取行业/板块研究报告,请先使用QueryBKDictInfo工具获取行业代码，然后输入行业代码调用",
	//		Parameters: FunctionParameters{
	//			Type: "object",
	//			Properties: map[string]any{
	//				"bkCode": map[string]any{
	//					"type":        "string",
	//					"description": "板块/行业代码",
	//				},
	//			},
	//			Required: []string{"bkCode"},
	//		},
	//	},
	//})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockResearchReport",
			Description: "获取市场分析师的股票研究报告。支持一次查询多只，将并行请求后合并结果。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码。多只时可用英文逗号分隔。",
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
			Name:        "HotStrategyTable",
			Description: "获取当前热门选股策略",
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "HotStockTable",
			Description: "当前热门股票排名",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"pageSize": map[string]any{
						"type":        "string",
						"description": "分页大小",
					},
				},
				Required: []string{"pageSize"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockMoneyData",
			Description: "今日股票资金流入排名",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"pageSize": map[string]any{
						"type":        "string",
						"description": "分页大小",
					},
				},
				Required: []string{"pageSize"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name: "GetMutualTop10Deal",
			Description: "获取:北向资金（沪股通、深股通）南向资金（港股通）交易日期对应十大成交股数据（注意：当日数据 17:00–18:00 左右更新）。" +
				"MUTUAL_TYPE=001 表示沪股通十大成交股；" +
				"002 表示港股通(沪)十大成交股；" +
				"003 表示深股通十大成交股；" +
				"004 表示港股通(深)十大成交股。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"mutualType": map[string]any{
						"type": "string",
						"description": "互联互通通道类型：" +
							"001=沪股通十大成交股，" +
							"002=港股通(沪)十大成交股，" +
							"003=深股通十大成交股，" +
							"004=港股通(深)十大成交股",
					},
					"tradeDate": map[string]any{
						"type":        "string",
						"description": "交易日期，格式：YYYY-MM-DD，例如 2026-03-16（注意：当日数据 17:00–18:00 左右更新）",
					},
					"page": map[string]any{
						"type":        "number",
						"description": "页码，从 1 开始，默认 1",
					},
					"pageSize": map[string]any{
						"type":        "number",
						"description": "每页条数，默认 10",
					},
				},
				Required: []string{"mutualType", "tradeDate"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockFinancialInfo",
			Description: "获取股票财务报表信息。支持一次查询多只，将并行请求后合并结果。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码,如：601138.SH。注意 上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，港股股票以.HK结尾，北交所股票以.BJ结尾。多只时可用英文逗号分隔。",
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
			Name:        "GetStockHolderNum",
			Description: "获取股票股东人数信息(股东人数与股价比( 注:股票价格通常与股东人数成反比，股东人数越少代表筹码越集中，股价越有可能上涨))。支持一次查询多只，将并行请求后合并结果。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码,如：601138.SH。注意 上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，港股股票以.HK结尾，北交所股票以.BJ结尾。多只时可用英文逗号分隔。",
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
			Name:        "GetStockHistoryMoneyData",
			Description: "获取股票历史资金流向数据。支持一次查询多只，将并行请求后合并结果。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码,如：601138.SH。注意 上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，港股股票以.HK结尾，北交所股票以.BJ结尾。多只时可用英文逗号分隔。",
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
			Name:        "GetStockRZRQInfo",
			Description: "获取股票融资融券信息，包括融资余额、融券余额、两融余额、融资净买入等。适用于 A 股两融标的。支持一次查询多只，将并行请求后合并结果。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码。如：601138.SH、000001.SZ 或 sh601138、sz000001。多只时可用英文逗号分隔。",
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
			Name:        "GetIndustryValuation",
			Description: "获取行业/板块平均估值和中值（PE,PEG等）",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"bkName": map[string]any{
						"type":        "string",
						"description": "行业/板块名称,如：半导体",
					},
				},
				Required: []string{"bkName"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SearchInvestor",
			Description: "投资者关系活动搜索(同花顺i问财)。搜索上市公司投资者关系活动记录。",
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

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "SearchReport",
			Description: "研报搜索(同花顺i问财)。搜索主流投研机构发布的研究报告。",
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

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "QueryInsResearch",
			Description: "机构研究与评级查询(同花顺i问财)。查询研报评级、业绩预测、ESG评级、券商金股等机构观点数据。",
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
			Name:        "GetStockLatestFinance",
			Description: "获取股票最新财务主要数据，包括每股收益(EPS)、每股净资产(BPS)、净资产收益率(ROE)、营业收入、净利润及同比/环比增速等。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ、600000.SH",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockQtrMainFinance",
			Description: "获取股票季度主要财务指标，包括EPS、BPS、营业收入、净利润、同比增长率、ROE、毛利率等按季度列示。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockOrgPredict",
			Description: "获取股票机构预测数据，包括各券商/机构对未来数年的EPS和PE预测明细。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockPredictSummary",
			Description: "获取股票机构预测汇总，按年度汇总多家机构的EPS预测均值、增长率和PE估值。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockValuationPercentile",
			Description: "获取股票估值百分位数据，展示当前PE在历史30%/50%/70%分位的值，判断估值高低。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockMarginTrading",
			Description: "获取股票融资融券数据，包括融资买入额、融资余额、融券卖出量、融券余额等按日列示。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockBlockTrade",
			Description: "获取股票大宗交易数据，包括成交价、溢价率、成交金额、买方/卖方营业部等。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockHolderTrend",
			Description: "获取股票户均持股趋势数据，展示股东户数和户均持股数量随时间的变化趋势。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockBillboard",
			Description: "获取股票龙虎榜数据，包括上榜日期、上榜原因、买入/卖出总额等。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockOperationDeptTrade",
			Description: "获取股票营业部买卖明细，展示各营业部在龙虎榜上的买入/卖出金额和占比。数据来源于东方财富F10。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 600519、000001.SZ",
					},
				},
				Required: []string{"stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "ComparableCompanyAnalysis",
			Description: "可比公司分析(东方财富妙想)。对指定公司进行可比公司分析，包括财务指标对比和估值对比，帮助判断公司相对估值水平。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "公司名称或股票代码，如：贵州茅台、东方财富",
					},
				},
				Required: []string{"query"},
			},
		},
	})

	return tools
}
