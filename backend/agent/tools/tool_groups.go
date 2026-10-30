package tools

import (
	"regexp"
	"strings"

	"github.com/go-kratos/blades/tools"
)

type ToolGroup string

const (
	GroupBase          ToolGroup = "base"
	GroupStockAnalysis ToolGroup = "stock_analysis"
	GroupMarket        ToolGroup = "market"
	GroupScreening     ToolGroup = "screening"
	GroupMoneyFlow     ToolGroup = "money_flow"
	GroupNewsResearch  ToolGroup = "news_research"
	GroupAIAnalysis    ToolGroup = "ai_analysis"
	GroupOperations    ToolGroup = "operations"
	GroupProject       ToolGroup = "project"
)

var toolGroupMap = map[string]ToolGroup{
	"QueryStockCodeInfo":           GroupBase,
	"QueryBKDictInfo":              GroupBase,
	"GetCurrentTime":               GroupBase,
	"GetFollowedStocks":            GroupBase,
	"GetHolidayInfo":               GroupBase,
	"GetHolidayYear":               GroupBase,
	"GetHolidayBatch":              GroupBase,
	"IsTradingDay":                 GroupBase,
	"GetNextTradingDay":            GroupBase,
	"CreateAiRecommendStocks":      GroupBase,
	"BatchCreateAiRecommendStocks": GroupBase,

	"GetStockInfo":            GroupStockAnalysis,
	"GetStockKLine":           GroupStockAnalysis,
	"GetETFMomentumRanking":   GroupStockAnalysis,
	"GetEastMoneyKLine":       GroupStockAnalysis,
	"GetEastMoneyKLineWithMA": GroupStockAnalysis,
	"GetStockMinuteData":      GroupStockAnalysis,
	"GetStockFinancialInfo":   GroupStockAnalysis,
	"GetStockHolderNum":       GroupStockAnalysis,
	"GetStockRZRQInfo":        GroupStockAnalysis,
	"GetStockConceptInfo":     GroupStockAnalysis,

	"GetMarketData":              GroupMarket,
	"GlobalStockIndexesReadable": GroupMarket,
	"GetStockChanges":            GroupMarket,
	"GetStockChangeHistoryList":  GroupMarket,
	"GetDailyChangeStats":        GroupMarket,
	"GetChangeRank":              GroupMarket,
	"GetDailyDimensionStats":     GroupMarket,
	"GetTypeStatsByDate":         GroupMarket,

	"StockEarningsReview":         GroupStockAnalysis,
	"FinancialQA":                 GroupAIAnalysis,
	"GetStockLatestFinance":       GroupStockAnalysis,
	"GetStockQtrMainFinance":      GroupStockAnalysis,
	"GetStockOrgPredict":          GroupStockAnalysis,
	"GetStockPredictSummary":      GroupStockAnalysis,
	"GetStockValuationPercentile": GroupStockAnalysis,
	"GetStockMarginTrading":       GroupStockAnalysis,
	"GetStockBlockTrade":          GroupStockAnalysis,
	"GetStockHolderTrend":         GroupStockAnalysis,
	"GetStockBillboard":           GroupStockAnalysis,
	"GetStockOperationDeptTrade":  GroupStockAnalysis,
	"AnalyzeHotMoneyActivity":    GroupStockAnalysis,
	"AnalyzeChipConcentration":   GroupStockAnalysis,
	"AnalyzeCapitalGame":         GroupStockAnalysis,
	"ComparableCompanyAnalysis":   GroupStockAnalysis,
	"HotspotDiscovery":            GroupMarket,
	"IndustryResearch":            GroupStockAnalysis,
	"TrackingReport":              GroupStockAnalysis,
	"FinanceDataQuery":            GroupStockAnalysis,
	"FinanceSearch":               GroupNewsResearch,
	"FilterStocks":                GroupScreening,
	"SearchStockByIndicators":     GroupScreening,
	"SearchBk":                    GroupScreening,
	"SearchETF":                   GroupScreening,
	"HotStrategyTable":            GroupScreening,
	"HotStockTable":               GroupScreening,
	"ScreenByStrategy":            GroupScreening,
	"GetStockMoneyData":           GroupMoneyFlow,
	"GetMutualTop10Deal":          GroupMoneyFlow,
	"GetStockHistoryMoneyData":    GroupMoneyFlow,
	"GetIndustryMoneyRank":        GroupMoneyFlow,
	"QueryStockNewsTool":          GroupNewsResearch,
	"WebSearch":                   GroupNewsResearch,
	"FetchWebPage":                GroupNewsResearch,
	"GetNewsListData":             GroupNewsResearch,
	"GetStockResearchReport":      GroupNewsResearch,
	"GetIndustryResearchReport":   GroupNewsResearch,
	"GetSecuritiesCompanyOpinion": GroupNewsResearch,
	"StockNotice":                 GroupNewsResearch,
	"GetStockNotice":              GroupNewsResearch,
	"InteractiveAnswer":           GroupNewsResearch,
	"GetInvestCalendar":           GroupNewsResearch,
	"GetLongTigerList":            GroupNewsResearch,
	"GetHotStockList":             GroupNewsResearch,
	"GetHotEventList":             GroupNewsResearch,
	"GetUplimitLadder":            GroupNewsResearch,
	"GetUplimitHotPlates":         GroupNewsResearch,
	"GetUplimitHotStocks":         GroupNewsResearch,
	"GetUplimitExplodedStocks":    GroupNewsResearch,
	"GetUplimitPlateStocks":       GroupNewsResearch,
	"GetWallstreetcnLives":        GroupNewsResearch,
	"GetWallstreetcnMarketReal":   GroupMarket,
	"GetWallstreetcnKline":        GroupMarket,
	"GetWallstreetcnCalendar":     GroupNewsResearch,
	"AiRecommendStocks":           GroupAIAnalysis,
	"GetAIAnalysisHistory":        GroupAIAnalysis,
	"GetAIAnalysisDetail":         GroupAIAnalysis,
	"GetAIAnalysisContent":        GroupAIAnalysis,
	"GetStockThesis":              GroupAIAnalysis,
	"SaveStockThesis":             GroupAIAnalysis,
	"TriggerPostMortemAudit":       GroupAIAnalysis,
	"GetPostMortemAuditHistory":   GroupAIAnalysis,
	"GetPaperPortfolioSummary":    GroupAIAnalysis,
	"ExecutePaperTrade":           GroupAIAnalysis,
	"RunAutoAlphaExploration":     GroupAIAnalysis,
	"GetAutoAlphaProposals":       GroupAIAnalysis,
	"QuerySupplyChainCascade":     GroupStockAnalysis,
	"GetIndustrySupplyGraph":      GroupStockAnalysis,
	"GetBacktestStats":            GroupAIAnalysis,
	"GetBacktestDetail":           GroupAIAnalysis,
	"RunBacktest":                 GroupAIAnalysis,
	"VectorBacktest":              GroupAIAnalysis,
	"RunVectorizedBacktest":       GroupAIAnalysis,
	"DiagnoseStock":               GroupAIAnalysis,
	"VectorGridSearch":            GroupAIAnalysis,
	"SetTradingPrice":             GroupOperations,
	"SendDingDingMessage":         GroupOperations,
	"SendToDingDing":              GroupOperations,
	"SearchFund":                  GroupOperations,
	"GetFundInfo":                 GroupOperations,
	"GetEconomicData":             GroupOperations,

	// AKShare tools
	"GetFuturesSpot":           GroupMarket,
	"GetETFSpot":               GroupMarket,
	"GetETFHistorical":         GroupStockAnalysis,
	"GetOpenFundNetValue":      GroupStockAnalysis,
	"GetBoardConcept":          GroupMarket,
	"GetBoardIndustry":         GroupMarket,
	"GetMacroCPI":              GroupMarket,
	"GetMacroGDP":              GroupMarket,
	"GetMacroPMI":              GroupMarket,
	"GetMacroMoneySupply":      GroupMarket,
	"GetLongHuToday":           GroupNewsResearch,
	"GetConvertibleBondSpot":   GroupMarket,

	// AData tools
	"GetADataMarketCurrent":    GroupMarket,
	"GetADataKLine":            GroupStockAnalysis,
	"GetADataTradeCalendar":    GroupBase,
	"GetADataNorthFlow":        GroupMoneyFlow,

	"ReadProjectFile":    GroupProject,
	"ListProjectDir":     GroupProject,
	"SearchProjectFiles": GroupProject,
	"GitOperation":       GroupProject,
	"LearnProject":       GroupProject,
	"WriteProjectFile":   GroupProject,
	"RunCommand":         GroupProject,
}

type groupKeywords struct {
	group    ToolGroup
	keywords []string
}

var groupKeywordsList = []groupKeywords{
	{GroupStockAnalysis, []string{
		"股票", "股价", "行情", "K线", "k线", "日K", "周K", "月K", "分钟线",
		"分时", "实时", "价格", "涨跌", "成交量", "成交额",
		"财务", "报表", "营收", "利润", "ROE", "PE", "PB", "EPS",
		"毛利率", "净利率", "现金流", "负债率",
		"行业估值", "行业排名", "行业盈利",
		"股东", "持股", "融资融券", "融券", "融资", "杠杆",
		"股本结构", "股东户数", "实控人", "前十大股东",
		"概念", "板块归属", "所属概念",
		"业绩预告", "增发", "配股", "质押", "解禁", "调研", "监管函",
		"基本资料", "上市日期", "基金资料", "费率", "合约信息",
		"主营业务", "主要客户", "供应商", "参控股", "股权投资", "重大合同",
		"基金业绩", "基金持仓", "基金风险", "基金评级", "基金获奖",
		"业绩点评", "财报分析", "业绩报告", "营收分析", "利润分析", "季报", "年报", "中报",
		"行业研究", "行业报告", "产业分析", "行业深度", "行业趋势", "市场分析",
		"跟踪报告", "个股跟踪", "行业跟踪", "动态跟踪", "最新动态跟踪",
		"查数", "金融数据查询", "数据查询", "指标查询", "估值数据", "行情数据查询",
		"分析", "诊断", "评估", "估值",
		"技术面", "基本面", "MACD", "KDJ", "RSI", "布林", "BOLL",
		"均线", "MA5", "MA10", "MA20", "MA60", "MA120",
		"前复权", "后复权", "复权",
		"可比公司", "对标公司", "同行对比", "行业对标",
		"机构预测", "券商预测", "目标价", "一致性预期",
		"游资", "龙虎榜", "席位", "营业部", "打板", "接力",
		"筹码", "主力", "建仓", "洗盘", "出货", "吸筹",
		"资金博弈", "多空", "资金流向", "主力资金",
	}},
	{GroupMarket, []string{
		"大盘", "市场", "指数", "行情", "涨跌分布", "涨停", "跌停",
		"上涨家数", "下跌家数", "申购", "新股",
		"异动", "火箭发射", "快速反弹", "大笔买入", "封涨停",
		"加速下跌", "高台跳水", "大笔卖出", "封跌停",
		"全球指数", "道琼斯", "纳斯达克", "标普", "恒生", "日经",
		"异动统计", "异动趋势", "异动排行", "异动排名", "异动次数",
		"利好", "利空", "异动类型", "异动分布",
		"问财", "同花顺", "行情查询", "行情数据",
		"热点", "题材", "市场热点", "热点发现", "热点板块",
		"指数行情", "指数点位", "沪深300", "创业板指", "中证500",
		"宏观", "GDP", "CPI", "PPI", "PMI", "社融", "M2", "LPR",
		"期货", "期权", "波动率", "持仓", "行权",
		"北向资金", "南向资金", "沪深港通", "沪股通", "深股通", "港股通", "AH溢价",
	}},
	{GroupScreening, []string{
		"筛选", "选股", "过滤", "条件选股", "指标选股",
		"智能选股",
		"选板块", "板块排行", "板块筛选",
		"选ETF", "ETF筛选",
		"选基金经理", "基金经理筛选", "基金经理排名",
		"选可转债", "可转债筛选", "可转债溢价率",
		"选基金公司", "基金公司筛选", "基金公司排名",
		"选基金", "基金筛选", "基金排名",
		"选期货期权", "期货筛选", "期权筛选",
		"选港股", "港股筛选", "港股排行",
		"选美股", "美股筛选", "美股排行",
		"形态选股", "MACD金叉", "KDJ金叉", "放量突破",
		"连涨", "连跌", "多头排列", "空头排列",
		"板块", "概念", "ETF",
		"热门策略", "热门股票", "策略",
	}},
	{GroupMoneyFlow, []string{
		"资金", "流入", "流出", "净流入", "净流出",
		"北向", "南向", "沪股通", "深股通", "港股通",
		"主力", "机构", "外资",
		"行业资金", "板块资金",
	}},
	{GroupNewsResearch, []string{
		"新闻", "资讯", "消息", "公告", "研报", "研究报告",
		"最新动态", "政策动态", "行业趋势",
		"券商", "机构观点", "分析师", "评级",
		"投资评级", "目标价", 		"产业分析", "行业分析", "深度分析",
		"ESG", "信用评级", "主体评级", "基金评级", "券商金股", "业绩预测",
		"互动", "问答", "投资者互动",
		"投资者关系", "业绩说明会", "路演", "投资者调研", "分析师会议", "投关活动",
		"公告搜索", "分红公告", "回购公告", "重组公告", "定期报告",
		"资讯搜索", "金融资讯", "舆情监控", "热点捕捉", "研报速览", "公告精读",
		"日历", "财报日", "股东大会", "IPO",
		"龙虎榜", "营业部",
		"涨停", "连板", "梯队", "涨停复盘", "涨停板块", "炸板", "封板",
		"热门板块", "板块热度", "板块轮动", "主线题材", "接力板块",
		"个股热度", "热门个股", "人气股", "关注度",
		"封板失败", "开板", "破板",
		"热门话题", "热点事件", "雪球",
		"华尔街见闻", "见闻快讯", "全球快讯", "7x24",
		"美元指数", "非农", "美联储", "降息", "加息", "通胀数据",
		"财经日历", "经济数据公布", "重要数据",
		"城市", "总部", "所在地", "注册地", "办公地址", "省份", "区域", "地区", "地名", "城市名", "地域",
		"有哪些", "名单", "列表", "汇总", "一览", "全部",
		"搜索", "网上搜索", "联网搜索", "查一下",
	}},
	{GroupAIAnalysis, []string{
		"AI分析", "AI推荐", "历史分析", "分析报告",
		"假说", "认知", "论点", "证伪", "thesis", "档案", "认知档案", "投资假说", "买方档案", "买方认知",
		"推荐股票", "买入评级", "增持", "减持",
		"止盈", "止损", "买入价", "目标价",
		"帮我查", "分析一下", "怎么样", "是什么", "解释一下", "总结一下",
		"深度分析", "深度思考", "详细分析", "仔细想想",
		"金融问答", "智能问答",
		"回测", "准确率", "验证", "预测", "准不准", "胜率", "夏普", "双均线", "均线交叉", "策略回测", "向量化回测", "vectorbt", "VectorBT", "诊断", "量化诊断", "三维诊断", "股票体检",
		"历史回顾", "复盘", "历史预测", "历史判断",
		"track record", "accuracy", "backtest",
	}},
	{GroupOperations, []string{
		"预警", "价位", "开仓", "止盈价", "止损价", "成本价",
		"钉钉", "QQ", "通知", "推送", "发送消息",
		"基金", "基金代码", "基金名称", "净值",
		"GDP", "CPI", "PPI", "PMI", "宏观经济",
	}},
	{GroupProject, []string{
		"项目", "代码", "源码", "仓库", "git", "Git", "github",
		"学习", "吸收", "skill", "Skill", "技能", "知识库",
		"文件读取", "目录结构", "看代码", "读代码", "项目结构",
		"分析项目", "导入项目", "项目文件", "源代码",
		"ReadProjectFile", "ListProjectDir", "GitOperation",
		"WriteProjectFile", "RunCommand", "写文件", "执行命令",
		"编译", "测试", "build", "格式化", "lint", "fmt",
	}},
}

var reStockPattern = regexp.MustCompile(`(?i)(?:\b|\D)\d{6}(?:\b|\D)|(?:sh|sz|bj)\d{6}|\d{6}\.(?:sh|sz|bj)`)

func ClassifyQuestion(question string) map[ToolGroup]bool {
	applyToolGroupConfig()
	matched := map[ToolGroup]bool{
		GroupBase: true,
	}
	lowerQ := strings.ToLower(question)
	for _, gk := range groupKeywordsList {
		for _, kw := range gk.keywords {
			if strings.Contains(lowerQ, strings.ToLower(kw)) {
				matched[gk.group] = true
				break
			}
		}
	}

	// 智能证券代码检测：如果问题包含 6 位股票代码或证券前后缀，秒级自动激活个股量化与 AI 深度诊断工具组
	if reStockPattern.MatchString(question) {
		matched[GroupStockAnalysis] = true
		matched[GroupAIAnalysis] = true
	}

	if len(matched) <= 1 {
		for _, g := range []ToolGroup{
			GroupStockAnalysis, GroupMarket, GroupNewsResearch,
		} {
			matched[g] = true
		}
	}
	return matched
}

// FilterToolsByGroups curates tools per group; groups not in map are excluded.
func FilterToolsByGroups(allTools []tools.Tool, groups map[ToolGroup]bool) []tools.Tool {
	var filtered []tools.Tool
	for _, t := range allTools {
		group, exists := toolGroupMap[t.Name()]
		if !exists || groups[group] {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// groupToolCaps 每组允许注入模型的最大工具数。每注入一个工具，其名称、描述、
// 参数 schema 都会全量发送给模型消耗 tokens；这些上限保证命中多组时不会把
// 上百个工具定义全部塞进提示词。无上限的组（GroupBase/GroupProject 等体量小
// 或必须完整的组）不在此表。
var groupToolCaps = map[ToolGroup]int{
	GroupStockAnalysis: 20,
	GroupMarket:        14,
	GroupScreening:     10,
	GroupMoneyFlow:     4,
	GroupNewsResearch:  14,
	GroupAIAnalysis:    14,
	GroupOperations:    5,
}

// coreToolByGroup 每组的高频核心工具白名单（按优先级排序，越靠前越优先保留）。
// 组内工具超过 groupToolCaps 上限时，只保留白名单内的工具；白名单外的工具丢弃。
var coreToolByGroup = map[ToolGroup][]string{
	GroupStockAnalysis: {
		"GetStockInfo", "GetStockKLine", "GetStockFinancialInfo", "GetStockLatestFinance",
		"GetEastMoneyKLineWithMA", "GetStockConceptInfo", "GetStockValuationPercentile",
		"GetStockRZRQInfo", "GetStockMarginTrading", "GetStockHolderTrend",
		"GetStockBillboard", "AnalyzeHotMoneyActivity", "GetStockQtrMainFinance",
		"GetStockOrgPredict", "GetETFHistorical", "QuerySupplyChainCascade", "GetIndustrySupplyGraph",
	},
	GroupMarket: {
		"GetMarketData", "GlobalStockIndexesReadable", "GetStockChanges", "GetChangeRank",
		"GetBoardConcept", "GetETFSpot", "HotspotDiscovery",
	},
	GroupScreening: {
		"SearchStockByIndicators", "FilterStocks", "SearchBk", "SearchETF",
	},
	GroupMoneyFlow: {
		"GetStockMoneyData", "GetStockHistoryMoneyData", "GetIndustryMoneyRank",
		"GetMutualTop10Deal",
	},
	GroupNewsResearch: {
		"GetNewsListData", "GetStockResearchReport",
		"GetStockNotice", "GetHotStockList", "GetHotEventList",
		"GetUplimitHotStocks", "GetUplimitHotPlates", "GetWallstreetcnLives",
		"WebSearch", "FetchWebPage",
	},
	GroupAIAnalysis: {
		"FinancialQA", "AiRecommendStocks", "GetAIAnalysisHistory", "GetStockThesis", "SaveStockThesis", "GetPaperPortfolioSummary", "ExecutePaperTrade", "TriggerPostMortemAudit", "GetPostMortemAuditHistory", "RunAutoAlphaExploration", "GetAutoAlphaProposals", "RunBacktest", "RunVectorizedBacktest", "DiagnoseStock", "VectorBacktest", "VectorGridSearch",
	},
	GroupOperations: {
		"SetTradingPrice", "SendToDingDing", "GetFundInfo", "SearchFund",
	},
}

// FilterToolsByGroupsCapped 在按组过滤基础上对每组工具数设上限：
// 组内工具超过 caps[group] 时，仅保留 coreToolByGroup 白名单中的工具；
// 未设置上限或未定义白名单的组保持全量。用于削减注入模型的工具定义 token 开销。
func FilterToolsByGroupsCapped(allTools []tools.Tool, groups map[ToolGroup]bool) []tools.Tool {
	var filtered []tools.Tool
	coreSet := make(map[string]bool)
	for _, group := range []ToolGroup{GroupBase, GroupStockAnalysis, GroupMarket, GroupScreening, GroupMoneyFlow, GroupNewsResearch, GroupAIAnalysis, GroupOperations, GroupProject} {
		if !groups[group] {
			continue
		}
		for _, n := range coreToolByGroup[group] {
			coreSet[n] = true
		}
	}

	// 第一遍：未映射组工具 + 命中组内属于白名单的工具 + 无上限组的工具。
	added := make(map[ToolGroup]int)
	for _, t := range allTools {
		group, exists := toolGroupMap[t.Name()]
		if !exists {
			// 未映射到任何特定组的通用基础工具
			filtered = append(filtered, t)
			continue
		}
		if !groups[group] {
			continue
		}
		_, capped := groupToolCaps[group]
		if !capped {
			filtered = append(filtered, t)
			continue
		}
		if coreSet[t.Name()] {
			filtered = append(filtered, t)
			added[group]++
			continue
		}
	}
	// 第二遍：白名单工具数未达上限的组，按原顺序补充剩余工具，直至上限。
	for _, t := range allTools {
		group, exists := toolGroupMap[t.Name()]
		if !exists || !groups[group] {
			continue
		}
		cap, capped := groupToolCaps[group]
		if !capped {
			continue
		}
		if added[group] >= cap {
			continue
		}
		if coreSet[t.Name()] {
			continue // 已注入
		}
		filtered = append(filtered, t)
		added[group]++
	}
	return filtered
}

// ParseToolGroups parses a comma-separated list of tool group names into a set.
// Unknown names are ignored so a typo can never silently drop required tools.
func ParseToolGroups(str string) map[ToolGroup]bool {
	groups := map[ToolGroup]bool{}
	for _, part := range strings.Split(str, ",") {
		name := strings.TrimSpace(part)
		if name == "" || !isValidToolGroup(ToolGroup(name)) {
			continue
		}
		groups[ToolGroup(name)] = true
	}
	return groups
}

func isValidToolGroup(g ToolGroup) bool {
	switch g {
	case GroupBase, GroupStockAnalysis, GroupMarket, GroupScreening,
		GroupMoneyFlow, GroupNewsResearch, GroupAIAnalysis,
		GroupOperations, GroupProject:
		return true
	}
	return false
}

// GetToolGroup returns the ToolGroup assigned to a tool name (used for validation & testing).
func GetToolGroup(toolName string) (ToolGroup, bool) {
	g, ok := toolGroupMap[toolName]
	return g, ok
}

