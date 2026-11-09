package tools

import (
	"testing"

	"blades-stock/backend/agent/types"
	bladesTools "github.com/go-kratos/blades/tools"
)

// buildAllFakeTools 依据真实 builtinToolGroups（能力注册表）构造同组映射的假工具，验证裁剪逻辑。
func buildAllFakeTools() []bladesTools.Tool {
	names := map[string]bool{}
	for name := range builtinToolGroups {
		names[name] = true
	}
	// 未映射工具（如 WebSearch）也应存在
	names["CustomUnmappedTool"] = true
	var out []bladesTools.Tool
	for n := range names {
		out = append(out, types.NewTool(n, "desc", map[string]*types.ParamDef{}, func(string) (string, error) { return "", nil }))
	}
	return out
}

func groupNamesOf(ts []bladesTools.Tool) map[string]int {
	m := map[string]int{}
	for _, t := range ts {
		m[t.Name()]++
	}
	return m
}

func TestFilterToolsByGroupsCapped_StockAnalysisCap(t *testing.T) {
	all := buildAllFakeTools()
	groups := map[ToolGroup]bool{GroupBase: true, GroupStockAnalysis: true}
	out := FilterToolsByGroupsCapped(all, groups)
	got := groupNamesOf(out)
	cap := groupToolCaps[GroupStockAnalysis]
	if got["GetStockInfo"] != 1 {
		t.Errorf("core GetStockInfo dropped, got map=%v", got)
	}
	// 白名单内核心工具应全部保留
	for _, n := range coreToolByGroup[GroupStockAnalysis] {
		if got[n] != 1 {
			t.Errorf("core tool %s dropped, got map=%v", n, got)
		}
	}
	// 组内工具总数不得超过上限
	count := 0
	for n, g := range builtinToolGroups {
		if g != GroupStockAnalysis {
			continue
		}
		count += got[n]
	}
	if count > cap {
		t.Errorf("stock_analysis injected %d tools > cap %d", count, cap)
	}
}

func TestFilterToolsByGroupsCapped_UncappedAndUnmapped(t *testing.T) {
	all := buildAllFakeTools()
	groups := map[ToolGroup]bool{GroupBase: true}
	out := FilterToolsByGroupsCapped(all, groups)
	got := groupNamesOf(out)
	// 未映射工具始终注入
	if got["CustomUnmappedTool"] != 1 {
		t.Errorf("unmapped CustomUnmappedTool should always be injected, got=%v", got)
	}
	// base 无上限，全部注入
	for n, g := range builtinToolGroups {
		if g != GroupBase {
			continue
		}
		if got[n] != 1 {
			t.Errorf("base tool %s dropped", n)
		}
	}
}

func TestFilterToolsByGroupsCapped_FillUpToCap(t *testing.T) {
	all := buildAllFakeTools()
	groups := map[ToolGroup]bool{GroupBase: true, GroupScreening: true}
	out := FilterToolsByGroupsCapped(all, groups)
	got := groupNamesOf(out)
	cap := groupToolCaps[GroupScreening]
	count := 0
	for name, g := range builtinToolGroups {
		if g != GroupScreening {
			continue
		}
		count += got[name]
	}
	if count > cap {
		t.Errorf("screening injected %d > cap %d", count, cap)
	}
	// 白名单核心必须都在
	for _, n := range coreToolByGroup[GroupScreening] {
		if got[n] != 1 {
			t.Errorf("core screening tool %s dropped", n)
		}
	}
}

func TestFilterToolsByGroupsCapped_ExcludedGroup(t *testing.T) {
	all := buildAllFakeTools()
	groups := map[ToolGroup]bool{GroupBase: true}
	out := FilterToolsByGroupsCapped(all, groups)
	got := groupNamesOf(out)
	if got["GetMarketData"] != 0 {
		t.Errorf("market tool leaked when market group excluded: %v", got)
	}
}

func TestFilterToolsByGroupsCapped_CorePriorityOverFill(t *testing.T) {
	// 构造一个组内全部工具都命中的场景：筛选组白名单 6 个 + 其余 11 个，
	// 上限 10，应优先保留白名单 6 个再补 4 个非核心，且不超上限。
	all := buildAllFakeTools()
	groups := map[ToolGroup]bool{GroupBase: true, GroupScreening: true}
	out := FilterToolsByGroupsCapped(all, groups)
	got := groupNamesOf(out)
	cap := groupToolCaps[GroupScreening]
	count := 0
	for name, g := range builtinToolGroups {
		if g != GroupScreening {
			continue
		}
		count += got[name]
	}
	if count > cap {
		t.Fatalf("screening injected %d > cap %d", count, cap)
	}
	// 即使非核心被填充，核心工具也不得被挤出
	for _, n := range coreToolByGroup[GroupScreening] {
		if got[n] != 1 {
			t.Errorf("core %s got squeezed out", n)
		}
	}
}

// marketSummaryDefaultQuestion 对应 app.go SummaryStockNews 默认提示词中显式要求
// 调用的工具清单（非严格全文，覆盖每个被点名工具所在行）。
const marketSummaryDefaultQuestion = `请对当前A股市场进行全面的AI资讯总结和分析。
第1步：用 GetMarketData 获取大盘指数行情、用 GlobalStockIndexesReadable 获取全球主要指数表现、用 GetStockChanges 获取今日异动数据
第2步：用 GetUplimitHotPlates 获取今日涨停板块热度排名、用 HotStockTable 获取热门股票排名、用 GetUplimitHotStocks 获取涨停个股列表、用 HotspotDiscovery 发现当前市场热点题材
第3步：用 GetNewsListData 获取当天重要财经新闻、用 GetWallstreetcnLives 获取华尔街见闻7x24快讯、用 WebSearch 搜索最新政策消息
第4步：用 GetStockMoneyData 查看个股资金流入排名、用 GetIndustryMoneyRank 查看行业资金流向、用 GetMutualTop10Deal 查看北向资金动向
第5步：综合分析，每一步都必须调用对应工具获取最新数据，严禁凭记忆编造。`

// TestFilterToolsByGroupsCapped_MarketSummaryContract 回归测试：市场资讯总结默认
// 提示词点名的工具在分类+裁剪后必须全部存活，否则模型调用会报 "tool not found"。
func TestFilterToolsByGroupsCapped_MarketSummaryContract(t *testing.T) {
	all := buildAllFakeTools()
	groups := ClassifyQuestion(marketSummaryDefaultQuestion)
	if !groups[GroupNewsResearch] {
		t.Fatalf("question should match news_research group, got %v", groups)
	}
	out := FilterToolsByGroupsCapped(all, groups)
	got := groupNamesOf(out)
	required := []string{
		"GetMarketData", "GlobalStockIndexesReadable", "GetStockChanges",
		"GetUplimitHotPlates", "HotStockTable", "GetUplimitHotStocks", "HotspotDiscovery",
		"GetNewsListData", "GetWallstreetcnLives", "WebSearch",
		"GetStockMoneyData", "GetIndustryMoneyRank", "GetMutualTop10Deal",
	}
	for _, n := range required {
		if got[n] != 1 {
			t.Errorf("market summary contract tool %s dropped after cap filter (groups=%v)", n, groups)
		}
	}
}
