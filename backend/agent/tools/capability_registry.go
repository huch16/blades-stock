package tools

import (
	"sort"
	"strings"
	"sync"

	bladesTools "github.com/go-kratos/blades/tools"
)

// ── 工具能力注册表（单一事实源）─────────────────────────────────────────
//
// 背景（改造前的问题）：
//
//	同一个工具的身份信息被复制在多处：`getToolsByQuestion` 里一份硬编码
//	append 清单、`toolGroupMap` 里一份工具名→组映射、`coreToolByGroup` 里
//	一份优先级白名单。三处必须手工保持一致，漏一处就产生两类静默缺陷：
//	  ① 工具漏登记组 → 在 FilterToolsByGroupsCapped 中走 `!exists` 分支，
//	     永久全量注入、完全不受分组上限约束（实测有 20 个工具处于此状态）；
//	  ② 工具从未被 append → 写好了却根本到不了模型手里。
//
// 本文件把「工具 → 组 / 层级 / 来源」收敛成**一份声明**，
// 并成为唯一查询入口（GetToolGroup / CapabilityOf 均读这里）。
//
// 分层含义（用于按预算剪枝，见 FilterToolsByGroupsCapped）：
//
//	TierCore      组内高频核心工具：预算不足时优先保留
//	TierExtended  组内常用工具：有余量则注入
//	TierNiche     低频/边缘工具：仅当同组远未达预算时才注入
//
// Source 用于区分「本仓库内置」与「外部 MCP 服务器动态提供」：
// 后者数量与内容运行时才确定，不参与静态预算与静态一致性校验。

// ToolTier 工具在所属组内的优先级层级。
type ToolTier int

const (
	TierCore ToolTier = iota
	TierExtended
	TierNiche
)

func (t ToolTier) String() string {
	switch t {
	case TierCore:
		return "core"
	case TierExtended:
		return "extended"
	case TierNiche:
		return "niche"
	default:
		return "unknown"
	}
}

// ToolSource 工具提供方。
type ToolSource int

const (
	SourceBuiltin ToolSource = iota // 本仓库内置
	SourceMCP                       // 外部 MCP 服务器动态提供
)

func (s ToolSource) String() string {
	switch s {
	case SourceBuiltin:
		return "builtin"
	case SourceMCP:
		return "mcp"
	default:
		return "unknown"
	}
}

// Capability 一个工具能力的完整身份声明。
type Capability struct {
	ID    string    // 工具名，全局唯一
	Group ToolGroup // 归属能力组
	Tier  ToolTier  // 组内层级
	// Source 提供方；外部的 MCP 工具由运行时单独注册，不写入 builtinCapabilities。
	Source ToolSource
}

var (
	capabilityMu sync.RWMutex
	// capabilities 以工具名为键。后声明覆盖先声明（便于运行时覆盖/修正）。
	capabilities = map[string]Capability{}
)

// DeclareCapabilities 登记一批能力声明。
//
// 幂等且并发安全：重复登记同名能力时，优先级按 Source 处理 ——
// 外部 MCP 的动态登记不得覆盖内置声明。
func DeclareCapabilities(caps ...Capability) {
	if len(caps) == 0 {
		return
	}
	capabilityMu.Lock()
	defer capabilityMu.Unlock()
	for _, c := range caps {
		id := strings.TrimSpace(c.ID)
		if id == "" {
			continue
		}
		c.ID = id
		if existing, ok := capabilities[id]; ok && existing.Source != c.Source {
			// 内置声明优先于外部动态登记，避免 MCP 同名工具覆盖内置能力
			if existing.Source == SourceBuiltin && c.Source == SourceMCP {
				continue
			}
		}
		capabilities[id] = c
	}
}

// CapabilityOf 查询某个工具名的能力声明。
func CapabilityOf(name string) (Capability, bool) {
	capabilityMu.RLock()
	defer capabilityMu.RUnlock()
	c, ok := capabilities[name]
	return c, ok
}

// AllCapabilities 返回全部已登记能力（按 ID 排序，保证可预期的输出）。
func AllCapabilities() []Capability {
	capabilityMu.RLock()
	defer capabilityMu.RUnlock()
	out := make([]Capability, 0, len(capabilities))
	for _, c := range capabilities {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// UndeclaredBuiltinTools 返回「已组装但未登记能力」的工具名，供一致性测试使用。
// 这些工具在 FilterToolsByGroupsCapped 中会走 `!exists` 分支而绕过分组上限，
// 是本次治理要消灭的首要缺陷。
func UndeclaredBuiltinTools(all []bladesTools.Tool) []string {
	capabilityMu.RLock()
	defer capabilityMu.RUnlock()
	var out []string
	for _, t := range all {
		if _, ok := capabilities[t.Name()]; !ok {
			out = append(out, t.Name())
		}
	}
	sort.Strings(out)
	return out
}

// builtinToolGroups 内置工具的能力组归属。
//
// 这是「工具 → 组」的唯一事实源（原 toolGroupMap 已迁移至此）。
// 组名常量见 tool_groups.go 的 ToolGroup 定义。
var builtinToolGroups = map[string]ToolGroup{
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
	"AnalyzeHotMoneyActivity":     GroupStockAnalysis,
	"AnalyzeChipConcentration":    GroupStockAnalysis,
	"AnalyzeCapitalGame":          GroupStockAnalysis,
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
	"TriggerPostMortemAudit":      GroupAIAnalysis,
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
	"GetFuturesSpot":         GroupMarket,
	"GetETFSpot":             GroupMarket,
	"GetETFHistorical":       GroupStockAnalysis,
	"GetOpenFundNetValue":    GroupStockAnalysis,
	"GetBoardConcept":        GroupMarket,
	"GetBoardIndustry":       GroupMarket,
	"GetMacroCPI":            GroupMarket,
	"GetMacroGDP":            GroupMarket,
	"GetMacroPMI":            GroupMarket,
	"GetMacroMoneySupply":    GroupMarket,
	"GetLongHuToday":         GroupNewsResearch,
	"GetConvertibleBondSpot": GroupMarket,

	// ── B 期：20 个「未登记」工具的策略声明（用户已明确决策）──────────────
	//
	// 治理前它们全部未登记组，走 FilterToolsByGroupsCapped 的 !exists 分支
	// 而**永久全量注入、不受任何分组上限约束**（实测 20 个，占组装总量的 1/6）。
	//
	// 用户决策（2026-11-09）：
	//   · 基金 K 线、TDX 系列   → 在意模型能调用，故归入个股分析组并给足层级；
	//   · MCP 管理类工具        → 不在意。已集中到 GroupMCPServer，
	//     C 期能力闸门会把整组从模型可见列表中裁掉（仅保留给运维/前端调用）。
	//
	// 基金系列（用户明确在意）：归入个股/基金分析组。
	"GetFundKLine":           GroupStockAnalysis,
	"GetFundHistoryNetValue": GroupStockAnalysis,
	"GetFundTop10Holdings":   GroupStockAnalysis,
	// TDX 系列（用户明确在意）：本地通达信数据源，归入个股分析组。
	"GetTdxCompanyCategory":   GroupStockAnalysis,
	"GetTdxCompanyInfo":       GroupStockAnalysis,
	"GetTdxFinanceInfo":       GroupStockAnalysis,
	"GetTdxSymbolBelongBoard": GroupStockAnalysis,
	"GetTdxXDXRInfo":          GroupStockAnalysis,
	"GetIndustryValuation":    GroupStockAnalysis,
	// 代码图谱查询：属项目级能力。
	"GraphQuery": GroupProject,
	// TickFlow 实时行情：属市场行情。
	"TickFlowRealtimeQuotes": GroupMarket,
	// MCP 服务器**管理类**：与上方业务工具无关，属运维/配置面。
	// 单独归组以便 C 期整组闸门裁剪。
	"ListMCPServers":     GroupMCPServer,
	"GetMCPServerDetail": GroupMCPServer,
	"CreateMCPServer":    GroupMCPServer,
	"UpdateMCPServer":    GroupMCPServer,
	"DeleteMCPServer":    GroupMCPServer,
	"EnableMCPServer":    GroupMCPServer,
	"TestMCPServer":      GroupMCPServer,
	"ListMCPServerTools": GroupMCPServer,
	"GetMCPToolDetail":   GroupMCPServer,

	// AData tools
	"GetADataMarketCurrent": GroupMarket,
	"GetADataKLine":         GroupStockAnalysis,
	"GetADataTradeCalendar": GroupBase,
	"GetADataNorthFlow":     GroupMoneyFlow,

	"ReadProjectFile":    GroupProject,
	"ListProjectDir":     GroupProject,
	"SearchProjectFiles": GroupProject,
	"GitOperation":       GroupProject,
	"LearnProject":       GroupProject,
	"WriteProjectFile":   GroupProject,
	"RunCommand":         GroupProject,
}

func init() {
	// 从 coreToolByGroup 反推 Tier，保证与改造前的剪枝行为**逐字节等价**
	// （coreToolByGroup 仍被 FilterToolsByGroupsCapped 使用）。
	cores := map[string]bool{}
	for _, names := range coreToolByGroup {
		for _, n := range names {
			cores[n] = true
		}
	}
	caps := make([]Capability, 0, len(builtinToolGroups))
	for name, group := range builtinToolGroups {
		tier := TierExtended
		if cores[name] {
			tier = TierCore
		}
		caps = append(caps, Capability{ID: name, Group: group, Tier: tier, Source: SourceBuiltin})
	}
	DeclareCapabilities(caps...)
}
