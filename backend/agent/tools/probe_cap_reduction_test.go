//go:build probe

package tools

import (
	"testing"

	bladesTools "github.com/go-kratos/blades/tools"
)

// buildRealToolSet 模拟 agent.go getToolsByQuestion 的组装顺序。
func buildRealToolSet() []bladesTools.Tool {
	var all []bladesTools.Tool
	all = append(all, GetQueryStockCodeInfoTool())
	all = append(all, GetQueryStockNewsTool())
	all = append(all, GetQueryBKDictTool())
	all = append(all, GetWebSearchTool())
	all = append(all, GetFetchWebPageTool())
	all = append(all, GetAllDataTools()...)
	all = append(all, GetHolidayTools()...)
	all = append(all, GetProjectTools()...)
	all = append(all, GetMCPServerTools()...)
	all = append(all, GetBacktestStatsTool())
	all = append(all, GetBacktestDetailTool())
	all = append(all, GetRunBacktestTool())
	all = append(all, GetVectorBacktestTool())
	all = append(all, GetVectorGridSearchTool())
	return all
}

// 估算工具定义注入 token 数：name+description+参数 schema 近似按字符数/2。
func estTokens(ts []bladesTools.Tool) int {
	total := 0
	for _, t := range ts {
		total += len(t.Name()) + len(t.Description())
	}
	return total / 2
}

func TestProbeCapReduction(t *testing.T) {
	all := buildRealToolSet()
	scenarios := []struct {
		question string
		groups   []ToolGroup
	}{
		{"帮我分析一下贵州茅台这只股票的基本面和近期走势", []ToolGroup{GroupStockAnalysis, GroupMarket, GroupMoneyFlow}},
		{"筛选出市盈率低于15且ROE大于20的股票", []ToolGroup{GroupScreening, GroupStockAnalysis}},
		{"推荐3只动量最强的ETF", []ToolGroup{GroupMoneyFlow, GroupMarket, GroupScreening}},
		{"最近市场有哪些热点新闻和涨停板", []ToolGroup{GroupNewsResearch, GroupMarket}},
		{"有没有最新的财经资讯和研究报告", []ToolGroup{GroupNewsResearch}},
		{"测试一下回测策略", []ToolGroup{GroupAIAnalysis}},
	}
	for _, sc := range scenarios {
		g := map[ToolGroup]bool{GroupBase: true}
		for _, gr := range sc.groups {
			g[gr] = true
		}
		out := FilterToolsByGroupsCapped(all, g)
		before := len(all)
		after := len(out)
		t.Logf("[%s]\n  groups=%v before=%d tools (~%d tok) -> after=%d tools (~%d tok), cut=%d%%",
			sc.question, sc.groups, before, estTokens(all), after, estTokens(out), 100-100*after/before)
		// 命中的每个组的核心白名单工具必须全部保留（仅当该工具在输入集中真实存在）
		inAll := map[string]bool{}
		for _, t := range all {
			inAll[t.Name()] = true
		}
		for _, gr := range sc.groups {
			for _, n := range coreToolByGroup[gr] {
				if !inAll[n] {
					continue // 条件注册的工具（如 MCP）未启用，跳过
				}
				found := false
				for _, t := range out {
					if t.Name() == n {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("[%s] group %s core tool %s dropped after cap filter", sc.question, gr, n)
				}
			}
		}
	}
	if len(all) < 100 {
		t.Fatalf("expected >=100 real tools, got %d (tool set too small to be meaningful)", len(all))
	}
}
