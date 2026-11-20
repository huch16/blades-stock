package data

func appendKlineTools(tools []Tool) []Tool {
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetStockKLine",
			Description: "获取股票日K线数据。支持一次查询多只，将并行请求后合并结果。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"days": map[string]any{
						"type":        "string",
						"description": "日K数据条数",
					},
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码（A股：sh,sz开头;港股hk开头,美股：us开头）。多只时可用英文逗号分隔。",
					},
					"stockCodes": toolSchemaStockCodes,
				},
				Required: []string{"days", "stockCode"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetEastMoneyKLine",
			Description: "获取股票 K 线数据。支持日/周/月/季/年 K 线及 1/5/15/30/60 分钟线，可选前复权(qfq)或后复权(hfq)。股票代码格式：A股 000001.SZ、600000.SH，港股 00700.HK 等。支持一次查询多只，将并行请求后合并结果。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码。A股如 000001.SZ、600000.SH；港股如 00700.HK。多只时可用英文逗号分隔。",
					},
					"stockCodes": toolSchemaStockCodes,
					"kLineType": map[string]any{
						"type":        "string",
						"description": "K 线类型：day/日/101=日K，week/周/102=周K，month/月/103=月K，quarter/季/104=季K，halfYear/半年/105=半年K，year/年/106=年K；分钟线：1/5/15/30/60/120。",
					},
					"adjustFlag": map[string]any{
						"type":        "string",
						"description": "复权类型，仅日K有效：空=不复权，qfq=前复权，hfq=后复权。",
					},
					"limit": map[string]any{
						"type":        "number",
						"description": "获取 K 线根数（日K为天数，周K为周数，月K为月数，分钟为天数内分钟数等）。",
					},
				},
				Required: []string{"stockCode", "kLineType", "limit"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "GetEastMoneyKLineWithMA",
			Description: "获取股票 K 线数据并带多条均线（SMA，按收盘价计算）。用于技术分析时同时查看 K 线与均线。股票代码格式同 GetEastMoneyKLine。支持一次查询多只，将并行请求后合并结果。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码。A股如 000001.SZ、600000.SH；港股如 00700.HK。多只时可用英文逗号分隔。",
					},
					"stockCodes": toolSchemaStockCodes,
					"kLineType": map[string]any{
						"type":        "string",
						"description": "K 线类型：day/日/101=日K，week/周/102=周K，month/月/103=月K；分钟线：1/5/15/30/60/120。",
					},
					"limit": map[string]any{
						"type":        "number",
						"description": "获取 K 线根数（如 60 表示最近 60 根）。",
					},
					"maPeriods": map[string]any{
						"type":        "string",
						"description": "均线周期，逗号分隔，如 \"5,10,20,60\"。不传则默认 5,10,20,60,120。",
					},
				},
				Required: []string{"stockCode", "kLineType", "limit"},
			},
		},
	})

	//CreateAiRecommendStocks
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "CreateAiRecommendStocks",
			Description: "创建/保存AI推荐股票记录",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"modelName": map[string]any{
						"type":        "string",
						"description": "模型名称",
					},
					"rating": map[string]any{
						"type":        "string",
						"description": "评级(买入:强烈看好，预期显著跑赢行业 / 大盘，涨幅空间大。 增持:依然看好，预期跑赢行业 / 大盘，但强度弱于买入。中性:不看多也不看空，预期基本持平市场 / 行业。减持:不看好，预期跑输行业 / 大盘，建议减仓。卖出:强烈看空，预期大幅跑输，建议回避。)",
					},
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码,如：601138.SH。注意 上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，港股股票以.HK结尾，北交所股票以.BJ结尾，",
					},
					"stockName": map[string]any{
						"type":        "string",
						"description": "股票名称",
					},
					"bkCode": map[string]any{
						"type":        "string",
						"description": "板块/行业代码",
					},
					"bkName": map[string]any{
						"type":        "string",
						"description": "板块/概念/行业名称",
					},
					"stockPrice": map[string]any{
						"type":        "string",
						"description": "推荐时股票价格",
					},
					"stockPrePrice": map[string]any{
						"type":        "string",
						"description": "前一交易日股票价格",
					},
					"stockClosePrice": map[string]any{
						"type":        "string",
						"description": "推荐时股票收盘价格",
					},
					"recommendReason": map[string]any{
						"type":        "string",
						"description": "推荐理由/驱动因素/逻辑",
					},
					"recommendBuyPrice": map[string]any{
						"type":        "string",
						"description": "ai建议买入价区间最低价和最高价之间用`-`分隔",
					},
					"recommendBuyPriceMax": map[string]any{
						"type":        "number",
						"description": "ai建议最高买入价",
					},
					"recommendBuyPriceMin": map[string]any{
						"type":        "number",
						"description": "ai建议最低买入价",
					},
					"recommendStopProfitPrice": map[string]any{
						"type":        "string",
						"description": "ai建议止盈价区间最低价和最高价之间用`-`分隔",
					},
					"recommendStopProfitPriceMax": map[string]any{
						"type":        "number",
						"description": "ai建议最高止盈价",
					},
					"recommendStopProfitPriceMin": map[string]any{
						"type":        "number",
						"description": "ai建议最低止盈价",
					},

					"recommendStopLossPrice": map[string]any{
						"type":        "string",
						"description": "ai建议止损价",
					},
					"riskRemarks": map[string]any{
						"type":        "string",
						"description": "风险提示",
					},
					"remarks": map[string]any{
						"type":        "string",
						"description": "操作总结/备注",
					},
				},
				Required: []string{"rating", "stockCode", "stockName", "bkName", "modelName", "recommendReason", "stockPrice"},
			},
		},
	})

	//BatchCreateAiRecommendStocks
	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "BatchCreateAiRecommendStocks",
			Description: "批量创建/保存AI推荐股票记录，建议每次批量保存5条记录",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stocks": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"modelName": map[string]any{
									"type":        "string",
									"description": "模型名称",
								},
								"rating": map[string]any{
									"type":        "string",
									"description": "评级(买入:强烈看好，预期显著跑赢行业 / 大盘，涨幅空间大。 增持:依然看好，预期跑赢行业 / 大盘，但强度弱于买入。中性:不看多也不看空，预期基本持平市场 / 行业。减持:不看好，预期跑输行业 / 大盘，建议减仓。卖出:强烈看空，预期大幅跑输，建议回避。)",
								},
								"stockCode": map[string]any{
									"type":        "string",
									"description": "股票代码,如：601138.SH。上海证券交易所股票以.SH结尾，深圳证券交易所股票以.SZ结尾，港股股票以.HK结尾，北交所股票以.BJ结尾。代码必须是6位数字且与stockName双向匹配（须来自工具返回的真实数据），格式非法或代码-名称不一致时禁止填写，严禁编造代码。",
								},
								"stockName": map[string]any{
									"type":        "string",
									"description": "股票名称",
								},
								"bkCode": map[string]any{
									"type":        "string",
									"description": "板块/行业代码",
								},
								"bkName": map[string]any{
									"type":        "string",
									"description": "板块/概念/行业名称",
								},
								"stockPrice": map[string]any{
									"type":        "string",
									"description": "推荐时股票价格",
								},
								"stockPrePrice": map[string]any{
									"type":        "string",
									"description": "前一交易日股票价格",
								},
								"stockClosePrice": map[string]any{
									"type":        "string",
									"description": "推荐时股票收盘价格",
								},
								"recommendReason": map[string]any{
									"type":        "string",
									"description": "推荐理由/驱动因素/逻辑",
								},
								"recommendBuyPrice": map[string]any{
									"type":        "string",
									"description": "ai建议买入价区间最低价和最高价之间用`-`分隔",
								},
								"recommendBuyPriceMin": map[string]any{
									"type":        "number",
									"description": "ai建议最低买入价",
								},
								"recommendBuyPriceMax": map[string]any{
									"type":        "number",
									"description": "ai建议最高买入价",
								},
								"recommendStopProfitPrice": map[string]any{
									"type":        "string",
									"description": "ai建议止盈价区间最低价和最高价之间用`-`分隔",
								},
								"recommendStopProfitPriceMin": map[string]any{
									"type":        "number",
									"description": "ai建议最低止盈价",
								},
								"recommendStopProfitPriceMax": map[string]any{
									"type":        "number",
									"description": "ai建议最高止盈价",
								},
								"recommendStopLossPrice": map[string]any{
									"type":        "string",
									"description": "ai建议止损价",
								},
								"riskRemarks": map[string]any{
									"type":        "string",
									"description": "风险提示",
								},
								"remarks": map[string]any{
									"type":        "string",
									"description": "操作总结/备注",
								},
							},
						},
					},
				},

				Required: []string{"rating", "stockCode", "stockName", "bkName", "modelName", "recommendReason", "stockPrice"},
			},
		},
	})

	tools = append(tools, Tool{
		Type: "function",
		Function: ToolFunction{
			Name: "SetTradingPrice",
			Description: "设置股票的预警价位线（开仓价、止盈价、止损价），用于设置股票的买入价格和风险控制参数。设置后会同步到行情界面显示。" +
				"开仓价：买入的目标价格；止盈价：预期卖出获利价格；止损价：亏损到该价格时必须卖出止损。" +
				"注意：所有价格参数必须为正数，0 表示不设置该价格。",
			Parameters: &FunctionParameters{
				Type: "object",
				Properties: map[string]any{
					"stockCode": map[string]any{
						"type":        "string",
						"description": "股票代码，如 000001.SZ、600000.SH（沪市）、00700.HK（港股）。注意：上海以.SH结尾，深圳以.SZ结尾，港股以.HK结尾，北交所以.BJ结尾。",
					},
					"entryPrice": map[string]any{
						"type":        "number",
						"description": "开仓价/买入价（目标买入价格），0 表示不设置",
					},
					"takeProfitPrice": map[string]any{
						"type":        "number",
						"description": "止盈价（预期卖出价格），0 表示不设置",
					},
					"stopLossPrice": map[string]any{
						"type":        "number",
						"description": "止损价（亏损止损价格），0 表示不设置",
					},
					"costPrice": map[string]any{
						"type":        "number",
						"description": "成本价（持仓成本价格），0 表示不设置",
					},
				},
				Required: []string{"stockCode", "entryPrice", "takeProfitPrice", "stopLossPrice", "costPrice"},
			},
		},
	})

	return tools
}
