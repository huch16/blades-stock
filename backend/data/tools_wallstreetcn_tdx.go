package data

func appendWallstreetcnTdxTools(tools []Tool) []Tool {
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetTdxCompanyInfo",
			Description: "通过通达信协议获取股票F10公司资料，包括公司简介、股本结构、财务摘要、除权除息等完整信息。当东方财富F10接口不可用或需要补充数据时可使用此工具。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码,如：600519.SH。注意 上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，北交所股票以.BJ结尾。多只时可用英文逗号分隔。",
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
			Name:        "GetTdxFinanceInfo",
			Description: "通过通达信协议获取股票财务信息，包括每股收益、总资产、净资产、营业收入、净利润、股东人数等核心财务指标。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码,如：600519.SH。注意 上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，北交所股票以.BJ结尾。多只时可用英文逗号分隔。",
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
			Name:        "GetTdxXDXRInfo",
			Description: "通过通达信协议获取股票除权除息信息，包括分红、配股、送转股等历史记录及股本变动情况。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码,如：600519.SH。注意 上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，北交所股票以.BJ结尾。多只时可用英文逗号分隔。",
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
			Name:        "GetTdxSymbolBelongBoard",
			Description: "通过通达信MAC接口获取股票所属板块信息，包括行业板块、概念板块等，以及板块涨跌幅、涨停/跌停家数等数据。支持一次查询多只，将并行请求后合并结果。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码,如：600519.SH。注意 上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，北交所股票以.BJ结尾，港股以.HK结尾。多只时可用英文逗号分隔。",
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
			Name:        "GetTdxCompanyCategory",
			Description: "通过通达信协议获取股票F10分类信息。不传category参数时返回所有可用分类名称列表；传入category参数时返回该分类的详细内容。可用分类包括：最新提示、公司概况、财务分析、股本结构、股东研究、机构持股、分红融资、高管治理、资金动向、资本运作、热点题材、公司公告、公司报道、经营分析、行业分析、研报评级。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码,如：600519.SH。注意 上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，北交所股票以.BJ结尾。多只时可用英文逗号分隔。",
					},
					"category": map[string]any{
						"type":        "string",
						"description": "F10分类名称，如：公司概况、财务分析、股本结构、股东研究、机构持股、分红融资、高管治理、资金动向、资本运作、热点题材、公司公告、公司报道、经营分析、行业分析、研报评级、最新提示。不传或为空时返回所有可用分类列表。",
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
			Name:        "GetWallstreetcnLives",
			Description: "获取华尔街见闻实时快讯。支持全球7x24、A股、美股、港股、外汇、商品、黄金、原油、债券、加密货币等频道。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"channel": map[string]any{
						"type":        "string",
						"description": "频道：global-channel=全球7x24, a-stock-channel=A股, us-stock-channel=美股, hk-stock-channel=港股, forex-channel=外汇, commodity-channel=商品, goldc-channel=黄金, oil-channel=原油, bond-channel=债券, crypto-channel=加密货币。默认global-channel",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "条数，默认20，最大50",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetWallstreetcnMarketReal",
			Description: "获取华尔街见闻全球实时行情报价。包含美元指数、欧元/美元、美元/日元、离岸人民币、现货黄金、WTI原油等品种。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"prodCodes": map[string]any{
						"type":        "string",
						"description": "品种代码(逗号分隔)，可选：DXY.OTC=美元指数, EURUSD.OTC=欧元美元, USDJPY.OTC=美元日元, USDCNH.OTC=离岸人民币, XAUUSD.OTC=现货黄金, USCL.OTC=WTI原油。留空返回全部。",
					},
				},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetWallstreetcnKline",
			Description: "获取华尔街见闻K线数据。支持美元指数、外汇、黄金、原油等品种的各周期K线。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"prodCode": map[string]any{
						"type":        "string",
						"description": "品种代码：DXY.OTC=美元指数, EURUSD.OTC=欧元美元, USDJPY.OTC=美元日元, USDCNH.OTC=离岸人民币, XAUUSD.OTC=现货黄金, USCL.OTC=WTI原油",
					},
					"periodType": map[string]any{
						"type":        "integer",
						"description": "K线周期(秒)：60=1分钟, 300=5分钟, 900=15分钟, 1800=30分钟, 3600=1小时, 14400=4小时, 86400=日线。默认300",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "K线条数，默认50",
					},
				},
				Required: []string{"prodCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetWallstreetcnCalendar",
			Description: "获取华尔街见闻财经日历。包含全球重要经济数据公布时间、预期值、前值等。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"days": map[string]any{
						"type":        "integer",
						"description": "查看未来几天内的财经日历，默认3天",
					},
				},
			},
		},
	})

	return tools
}
