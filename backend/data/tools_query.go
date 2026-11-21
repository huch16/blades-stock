package data

func appendAgentParityTools(tools []Tool) []Tool {
	for _, def := range []struct {
		name        string
		description string
	}{
		{"SendDingDingMessage", "将指定标题和内容以 Markdown 形式发送到钉钉机器人。等同于 SendToDingDing。"},
	} {
		tools = append(tools, Tool{
			Type: "function",
			Function: ToolFunction{
				Name:        def.name,
				Description: def.description,
				Parameters: &FunctionParameters{
					Type: "object",
					Properties: map[string]any{
						"title": map[string]any{
							"type":        "string",
							"description": "消息标题",
						},
						"message": map[string]any{
							"type":        "string",
							"description": "消息正文，通知内容需尽可能精简",
						},
					},
					Required: []string{"title", "message"},
				},
			},
		})
	}

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetHolidayYear",
			Description: "查询指定年份的所有节假日数据，包括日期、名称、连休天数、补班安排等。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"year": map[string]any{
						"type":        "string",
						"description": "查询年份，格式：YYYY。不传则查询当前年份",
					},
				},
			},
		},
	})
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetHolidayBatch",
			Description: "批量查询多个日期的节假日信息。适合需要一次性查询多个日期是否为节假日的场景。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"dates": map[string]any{
						"type":        "string",
						"description": "查询日期列表，多个日期用逗号分隔，格式：YYYY-MM-DD",
					},
				},
				Required: []string{"dates"},
			},
		},
	})

	for _, def := range []struct {
		name        string
		description string
	}{
		{"QueryBasicInfo", "基本资料查询。查询股票、指数、基金、期货、期权、转债、债券、理财、保险等基础信息、发行主体、机构资料、费率、上市地点、上市日期等。"},
		{"QueryFinance", "财务数据查询。查询营业收入、净利润、毛利率、净利率、ROE、ROA、负债率、现金流、市盈率、市净率等财务指标。"},
		{"QueryIndustry", "行业数据查询。查询行业估值、行业财务指标、行业盈利数据、行业行情数据、板块排名等行业维度数据。"},
		{"QueryFutures", "期货期权数据查询。查询期货期权行情、波动率、库存产销、会员持仓、榜单、行权等数据。"},
		{"SelectETF", "ETF智能筛选。按行情、跟踪指数、估值、费率、规模、份额变化等条件筛选ETF。"},
		{"QueryManagement", "公司股东股本查询。查询股本结构、股权结构、股东户数、前十大股东、实控人、质押、高管等。"},
		{"QueryStockConnect", "沪深港通资金流查询。查询北向资金、南向资金、沪股通、深股通、港股通、北向持股变动、AH溢价等。"},
		{"SelectFundManager", "智能选基金经理。根据历史业绩、管理规模、投资风格、风险控制等维度筛选基金经理。"},
		{"SelectConvertibleBond", "智能选可转债。按转股溢价率、正股表现、评级、剩余期限等条件筛选可转债。"},
		{"SelectFundCompany", "智能选基金公司。根据管理规模、旗下产品业绩、投研实力、风险评级等维度筛选基金公司。"},
		{"SelectFund", "智能选基金。根据基金类型、业绩、基金经理、风险、持仓、资产配置等维度筛选基金。"},
		{"SelectFuturesOption", "智能选期货期权。通过行情、波动率、产销、会员持仓、榜单、行权等条件筛选期货期权。"},
		{"SelectHKStock", "智能选港股。通过行情指标、财务指标、行业概念、陆港通等条件筛选港股。"},
		{"SelectUSStock", "智能选美股。通过行情指标、财务指标、行业概念、业绩预测、研报评级等条件筛选美股。"},
		{"QueryFundFinance", "基金理财查询。对基金做业绩、持仓、风险、评级、获奖、基金经理、基金公司综合分析。"},
		{"QueryBusinessData", "公司经营数据查询。查询主营业务构成、主要客户、供应商、参控股公司、股权投资、重大合同等经营数据。"},
	} {
		tools = append(tools, newQueryTool(def.name, def.description, "query"))
	}

	for _, def := range []struct {
		name        string
		description string
	}{
		{"SearchAnnouncement", "公告搜索。搜索A股、港股、基金、ETF等金融标的公告，包括定期财务报告、分红派息、回购增持、资产重组等。"},
		{"StockEarningsReview", "个股业绩点评。获取上市公司业绩点评报告，包含营收分析、利润分析、财务指标解读等深度内容。"},
		{"IndustryResearch", "行业研究报告生成。根据行业关键词生成深度行业研究报告。"},
		{"TrackingReport", "个股或行业跟踪报告。根据股票或行业关键词生成跟踪报告。"},
		{"FinanceDataQuery", "金融数据查询。基于东方财富数据库，支持自然语言查询A股、港股、美股、基金、债券等结构化金融数据。"},
	} {
		tools = append(tools, newSimpleQueryTool(def.name, def.description, "query"))
	}

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetDailyDimensionStats",
			Description: "按维度查询近N日每日异动趋势，支持按股票、行业、概念、异动类型四个维度查询。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"dimension": map[string]any{
						"type":        "string",
						"description": "查询维度：stock=股票，industry=行业，concept=概念，type=异动类型",
					},
					"name": map[string]any{
						"type":        "string",
						"description": "维度名称，如股票名称/代码、行业名称、概念名称、异动类型名称",
					},
					"days": map[string]any{
						"type":        "integer",
						"description": "查询天数，默认30",
					},
				},
				Required: []string{"dimension", "name"},
			},
		},
	})
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetTypeStatsByDate",
			Description: "查询某一天的异动类型分布统计，返回该天每种异动类型的利好/利空次数和总次数。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式：YYYY-MM-DD",
					},
				},
				Required: []string{"date"},
			},
		},
	})
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetUplimitPlateStocks",
			Description: "获取指定板块的涨停股详情，包括板块内所有涨停股票的代码、名称、连板数、封单比、成交额、市值、概念板块等。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"plate_name": map[string]any{
						"type":        "string",
						"description": "板块名称，如：人工智能、机器人、芯片等",
					},
					"date": map[string]any{
						"type":        "string",
						"description": "查询日期，格式：YYYY-MM-DD，默认今天",
					},
				},
				Required: []string{"plate_name"},
			},
		},
	})

	return tools
}

func newQueryTool(name, description, queryField string) Tool {
	return Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        name,
			Description: description,
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					queryField: map[string]any{
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
				Required: []string{queryField},
			},
		},
	}
}

func newSimpleQueryTool(name, description, queryField string) Tool {
	return Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        name,
			Description: description,
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					queryField: map[string]any{
						"type":        "string",
						"description": "自然语言查询语句或关键词",
					},
					"reportDate": map[string]any{
						"type":        "string",
						"description": "仅 StockEarningsReview 可用：报告期，格式YYYY-MM-DD",
					},
				},
				Required: []string{queryField},
			},
		},
	}
}
