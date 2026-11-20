package data

type dataToolGroup string

const (
	dataToolGroupBase          dataToolGroup = "base"
	dataToolGroupStockAnalysis dataToolGroup = "stock_analysis"
	dataToolGroupMarket        dataToolGroup = "market"
	dataToolGroupScreening     dataToolGroup = "screening"
	dataToolGroupMoneyFlow     dataToolGroup = "money_flow"
	dataToolGroupNewsResearch  dataToolGroup = "news_research"
	dataToolGroupAIAnalysis    dataToolGroup = "ai_analysis"
	dataToolGroupOperations    dataToolGroup = "operations"
)

var dataToolGroupMap = map[string]dataToolGroup{
	"QueryStockCodeInfo": dataToolGroupBase,
	"GetCurrentTime":     dataToolGroupBase,
	"GetHolidayInfo":     dataToolGroupBase,
	"GetHolidayYear":     dataToolGroupBase,
	"GetHolidayBatch":    dataToolGroupBase,
	"IsTradingDay":       dataToolGroupBase,
	"GetNextTradingDay":  dataToolGroupBase,
	"GetFollowedStocks":  dataToolGroupBase,

	"GetStockInfo":                dataToolGroupStockAnalysis,
	"GetStockKLine":               dataToolGroupStockAnalysis,
	"GetEastMoneyKLine":           dataToolGroupStockAnalysis,
	"GetEastMoneyKLineWithMA":     dataToolGroupStockAnalysis,
	"GetStockMinuteData":          dataToolGroupStockAnalysis,
	"GetStockFinancialInfo":       dataToolGroupStockAnalysis,
	"GetStockHolderNum":           dataToolGroupStockAnalysis,
	"GetStockRZRQInfo":            dataToolGroupStockAnalysis,
	"GetStockConceptInfo":         dataToolGroupStockAnalysis,
	"GetIndustryValuation":        dataToolGroupStockAnalysis,
	"GetTdxCompanyInfo":           dataToolGroupStockAnalysis,
	"GetTdxFinanceInfo":           dataToolGroupStockAnalysis,
	"GetTdxXDXRInfo":              dataToolGroupStockAnalysis,
	"GetTdxCompanyCategory":       dataToolGroupStockAnalysis,
	"GetTdxSymbolBelongBoard":     dataToolGroupStockAnalysis,
	"GetStockLatestFinance":       dataToolGroupStockAnalysis,
	"GetStockQtrMainFinance":      dataToolGroupStockAnalysis,
	"GetStockOrgPredict":          dataToolGroupStockAnalysis,
	"GetStockPredictSummary":      dataToolGroupStockAnalysis,
	"GetStockValuationPercentile": dataToolGroupStockAnalysis,
	"GetStockMarginTrading":       dataToolGroupStockAnalysis,
	"GetStockBlockTrade":          dataToolGroupStockAnalysis,
	"GetStockHolderTrend":         dataToolGroupStockAnalysis,
	"GetStockBillboard":           dataToolGroupStockAnalysis,
	"GetStockOperationDeptTrade":  dataToolGroupStockAnalysis,
	"ComparableCompanyAnalysis":   dataToolGroupStockAnalysis,
	"FinancialQA":                 dataToolGroupStockAnalysis,
	"GetAIAnalysisContent":        dataToolGroupStockAnalysis,
	"GetStockResearchReport":      dataToolGroupStockAnalysis,
	"GetIndustryResearchReport":   dataToolGroupStockAnalysis,
	"InteractiveAnswer":           dataToolGroupStockAnalysis,
	"GetSecuritiesCompanyOpinion": dataToolGroupStockAnalysis,
	"StockNotice":                 dataToolGroupStockAnalysis,
	"GetStockNotice":              dataToolGroupStockAnalysis,
	"SearchInvestor":              dataToolGroupStockAnalysis,
	"SearchReport":                dataToolGroupStockAnalysis,
	"QueryInsResearch":            dataToolGroupStockAnalysis,
	"QueryBasicInfo":              dataToolGroupStockAnalysis,
	"QueryFinance":                dataToolGroupStockAnalysis,
	"QueryIndustry":               dataToolGroupStockAnalysis,
	"QueryManagement":             dataToolGroupStockAnalysis,
	"QueryFundFinance":            dataToolGroupStockAnalysis,
	"QueryBusinessData":           dataToolGroupStockAnalysis,
	"StockEarningsReview":         dataToolGroupStockAnalysis,
	"IndustryResearch":            dataToolGroupStockAnalysis,
	"TrackingReport":              dataToolGroupStockAnalysis,
	"FinanceDataQuery":            dataToolGroupStockAnalysis,
	"AnalyzeHotMoneyActivity":     dataToolGroupStockAnalysis,
	"AnalyzeChipConcentration":    dataToolGroupStockAnalysis,
	"AnalyzeCapitalGame":          dataToolGroupStockAnalysis,

	"GetMarketData":              dataToolGroupMarket,
	"GlobalStockIndexesReadable": dataToolGroupMarket,
	"GetStockChanges":            dataToolGroupMarket,
	"GetStockChangeHistoryList":  dataToolGroupMarket,
	"GetDailyChangeStats":        dataToolGroupMarket,
	"GetChangeRank":              dataToolGroupMarket,
	"QueryIwencai":               dataToolGroupMarket,
	"QueryMacro":                 dataToolGroupMarket,
	"QueryZhishu":                dataToolGroupMarket,
	"QueryEvent":                 dataToolGroupMarket,
	"QueryFutures":               dataToolGroupMarket,
	"QueryStockConnect":          dataToolGroupMarket,
	"HotspotDiscovery":           dataToolGroupMarket,
	"GetWallstreetcnMarketReal":  dataToolGroupMarket,
	"GetWallstreetcnKline":       dataToolGroupMarket,
	"GetDailyDimensionStats":     dataToolGroupMarket,
	"GetTypeStatsByDate":         dataToolGroupMarket,

	"SearchStockByIndicators": dataToolGroupScreening,
	"SearchBk":                dataToolGroupScreening,
	"SearchETF":               dataToolGroupScreening,
	"HotStrategyTable":        dataToolGroupScreening,
	"HotStockTable":           dataToolGroupScreening,
	"FilterStocks":            dataToolGroupScreening,
	"SelectAStock":            dataToolGroupScreening,
	"SelectSector":            dataToolGroupScreening,
	"SelectETF":               dataToolGroupScreening,
	"SelectFundManager":       dataToolGroupScreening,
	"SelectConvertibleBond":   dataToolGroupScreening,
	"SelectFundCompany":       dataToolGroupScreening,
	"SelectFund":              dataToolGroupScreening,
	"SelectFuturesOption":     dataToolGroupScreening,
	"SelectHKStock":           dataToolGroupScreening,
	"SelectUSStock":           dataToolGroupScreening,

	"GetStockMoneyData":        dataToolGroupMoneyFlow,
	"GetMutualTop10Deal":       dataToolGroupMoneyFlow,
	"GetStockHistoryMoneyData": dataToolGroupMoneyFlow,
	"GetIndustryMoneyRank":     dataToolGroupMoneyFlow,

	"GetNewsListData":          dataToolGroupNewsResearch,
	"QueryStockNews":           dataToolGroupNewsResearch,
	"GetInvestCalendar":        dataToolGroupNewsResearch,
	"GetLongTigerList":         dataToolGroupNewsResearch,
	"GetHotStockList":          dataToolGroupNewsResearch,
	"GetHotEventList":          dataToolGroupNewsResearch,
	"SearchNews":               dataToolGroupNewsResearch,
	"SearchAnnouncement":       dataToolGroupNewsResearch,
	"FinanceSearch":            dataToolGroupNewsResearch,
	"GetUplimitLadder":         dataToolGroupNewsResearch,
	"GetUplimitHotPlates":      dataToolGroupNewsResearch,
	"GetUplimitHotStocks":      dataToolGroupNewsResearch,
	"GetUplimitExplodedStocks": dataToolGroupNewsResearch,
	"GetUplimitPlateStocks":    dataToolGroupNewsResearch,
	"GetWallstreetcnLives":     dataToolGroupNewsResearch,
	"GetWallstreetcnCalendar":  dataToolGroupNewsResearch,

	"CreateAiRecommendStocks":      dataToolGroupAIAnalysis,
	"BatchCreateAiRecommendStocks": dataToolGroupAIAnalysis,
	"AiRecommendStocks":            dataToolGroupAIAnalysis,
	"GetAIAnalysisHistory":         dataToolGroupAIAnalysis,
	"GetAIAnalysisDetail":          dataToolGroupAIAnalysis,

	"SetTradingPrice":        dataToolGroupOperations,
	"SendDingDingMessage":    dataToolGroupOperations,
	"SendToDingDing":         dataToolGroupOperations,
	"SearchFund":             dataToolGroupOperations,
	"GetFundInfo":            dataToolGroupOperations,
	"GetFundKLine":           dataToolGroupOperations,
	"GetFundHistoryNetValue": dataToolGroupOperations,
	"GetFundTop10Holdings":   dataToolGroupOperations,
	"GetEconomicData":        dataToolGroupOperations,
}

type dataToolGroupKeywords struct {
	group    dataToolGroup
	keywords []string
}

var dataToolGroupKeywordsList = []dataToolGroupKeywords{
	{dataToolGroupStockAnalysis, []string{
		"股票", "股价", "个股", "行情", "K线", "k线", "日K", "周K", "月K", "分时", "实时", "价格",
		"财务", "报表", "营收", "利润", "ROE", "PE", "PB", "EPS", "现金流", "负债率",
		"股东", "持股", "融资融券", "融券", "融资", "概念", "基本面", "技术面", "估值",
		"基本资料", "上市日期", "股本结构", "股东户数", "实控人", "主营业务", "主要客户",
		"供应商", "经营数据", "业绩点评", "财报分析", "行业研究", "跟踪报告", "金融数据查询",
		"研报", "研究报告", "机构预测", "券商预测", "目标价", "可比公司", "同行对比",
		"分析", "诊断", "评估", "怎么样", "怎么看", "能买吗", "值得买吗",
		"游资", "龙虎榜", "筹码", "建仓", "洗盘", "出货", "资金博弈", "多空", "主力", "资金流向",
	}},
	{dataToolGroupMarket, []string{
		"大盘", "市场", "指数", "涨跌分布", "涨停", "跌停", "上涨家数", "下跌家数", "异动",
		"热点", "题材", "宏观", "GDP", "CPI", "PPI", "PMI", "社融", "M2", "LPR",
		"美元指数", "黄金", "原油", "外汇", "美股", "港股", "A股", "全球",
		"期货", "期权", "波动率", "持仓", "北向资金", "南向资金", "沪深港通", "AH溢价",
	}},
	{dataToolGroupScreening, []string{
		"筛选", "选股", "条件选股", "指标选股", "智能选股", "选板块", "板块排行",
		"选ETF", "ETF", "形态选股", "MACD金叉", "KDJ金叉", "放量突破", "连涨", "连跌",
		"基金经理", "基金公司", "选基金", "基金筛选", "基金排名", "可转债", "转债",
		"选期货", "选期权", "港股筛选", "美股筛选", "选港股", "选美股",
		"多头排列", "空头排列", "热门策略", "热门股票",
	}},
	{dataToolGroupMoneyFlow, []string{
		"资金", "流入", "流出", "净流入", "净流出", "北向", "南向", "沪股通", "深股通",
		"港股通", "主力", "外资", "行业资金", "板块资金",
	}},
	{dataToolGroupNewsResearch, []string{
		"新闻", "资讯", "消息", "公告", "最新动态", "政策", "券商", "机构观点", "评级",
		"互动", "问答", "投资者关系", "调研", "财经日历", "龙虎榜", "连板", "梯队",
		"公告搜索", "分红公告", "回购公告", "重组公告", "涨停股", "涨停明细",
		"炸板", "华尔街见闻", "见闻快讯", "7x24", "非农", "美联储", "降息", "加息",
	}},
	{dataToolGroupAIAnalysis, []string{
		"AI分析", "AI推荐", "历史分析", "分析报告", "推荐股票", "买入评级", "增持", "减持",
		"止盈", "止损", "买入价",
	}},
	{dataToolGroupOperations, []string{
		"预警", "价位", "开仓", "成本价", "钉钉", "QQ", "通知", "推送", "发送消息",
		"基金", "基金代码", "基金名称", "净值",
	}},
}
