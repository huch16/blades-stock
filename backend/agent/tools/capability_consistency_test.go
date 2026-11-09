package tools

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	bladesTools "github.com/go-kratos/blades/tools"
)

// 能力注册表一致性校验（A 期核心交付）。
//
// 要消灭的正是改造前那类**静默**缺陷：
//
//	① 工具写好了却没登记能力组 → 在 FilterToolsByGroupsCapped 中走 !exists
//	   分支，永久全量注入、不受分组上限约束（治理前实测有 20 个工具如此）；
//	② 工具从未被 append 进组装列表 → 根本到不了模型手里；
//	③ 同一个工具被注入两次 → schema 重复下发，浪费 token 且让模型困惑；
//	④ 核心白名单里写了不存在的工具名，或与注册表分组不一致 → 静默失效，
//	   剪枝行为与意图不符（治理前 GetStockBillboard 就是如此）；
//	⑤ 过滤器超发 → cap 名义存在、实际失效。
//
// 这些错误此前没有任何测试能发现。以下用例让它们全部变成红灯。

// assembleForTest 复现「本次请求实际注入给模型的工具列表」。
//
// 与 agent.go getToolsByQuestion 的组合逻辑一致；单独抽出是为了让一致性测试
// 能在不启动 Agent 的情况下枚举全部候选工具。
// 若未来组装逻辑变化，这里必须同步（TestAssembledToolsHaveNoDuplicates 会先报警）。
func assembleForTest() []bladesTools.Tool {
	var all []bladesTools.Tool
	all = append(all, GetQueryStockCodeInfoTool())
	all = append(all, GetQueryStockNewsTool())
	all = append(all, GetQueryBKDictTool())
	all = append(all, GetWebSearchTool())
	all = append(all, GetFetchWebPageTool())
	all = append(all, GetGraphQueryTool())
	all = append(all, GetAllDataTools()...)
	all = append(all, GetHolidayTools()...)
	all = append(all, GetProjectTools()...)
	all = append(all, GetMCPServerTools()...)
	all = append(all, GetBacktestStatsTool())
	all = append(all, GetBacktestDetailTool())
	all = append(all, GetRunBacktestTool())
	all = append(all, GetVectorBacktestTool())
	all = append(all, GetRunVectorizedBacktestTool())
	all = append(all, GetDiagnoseStockTool())
	all = append(all, GetVectorGridSearchTool())
	all = append(all, GetStockThesisTool())
	all = append(all, SaveStockThesisTool())
	all = append(all, TriggerPostMortemAuditTool())
	all = append(all, GetPostMortemAuditHistoryTool())
	all = append(all, GetPaperPortfolioSummaryTool())
	all = append(all, ExecutePaperTradeTool())
	all = append(all, RunAutoAlphaExplorationTool())
	all = append(all, GetAutoAlphaProposalsTool())
	all = append(all, QuerySupplyChainCascadeTool())
	all = append(all, GetIndustrySupplyGraphTool())
	return all
}

func allGroupsEnabled() map[ToolGroup]bool {
	return map[ToolGroup]bool{
		GroupBase: true, GroupStockAnalysis: true, GroupMarket: true,
		GroupScreening: true, GroupMoneyFlow: true, GroupNewsResearch: true,
		GroupAIAnalysis: true, GroupOperations: true, GroupProject: true,
		GroupMCPServer: true,
	}
}

// 契约一：**每一个**会被注入的工具都必须已登记能力。
// 这是本次治理的首要目标：消灭「未登记 → 绕过 cap 永久全量注入」。
func TestEveryAssembledToolIsDeclared(t *testing.T) {
	undeclared := UndeclaredBuiltinTools(assembleForTest())
	if len(undeclared) != 0 {
		t.Fatalf("有 %d 个工具未登记能力组 —— 它们会走 !exists 分支永久全量注入、不受 cap 约束。\n请在 capability_registry.go 的 builtinToolGroups 中登记：\n  %v",
			len(undeclared), undeclared)
	}
}

// 契约二：组装列表不得出现重名工具（否则 schema 重复下发，浪费 token 且让模型困惑）。
func TestAssembledToolsHaveNoDuplicates(t *testing.T) {
	seen := map[string]int{}
	for _, tl := range assembleForTest() {
		seen[tl.Name()]++
	}
	var dups []string
	for n, c := range seen {
		if c > 1 {
			dups = append(dups, n)
		}
	}
	sort.Strings(dups)
	if len(dups) != 0 {
		t.Fatalf("以下工具被重复注入，请检查组装列表: %v", dups)
	}
}

// 契约三：**过滤后**每个组实际注入的工具数不得超过该组 cap。
//
// 语义说明：cap 是「注入上限」而非「组内工具数上限」。组内工具总量超过 cap
// 是**正常且预期**的——超出部分按 Tier 降级为 Niche，预算不足时被裁掉。
// 真正的不变量是过滤器不得超发，否则 cap 名义存在、实际失效。
func TestGroupCapsAreRespected(t *testing.T) {
	filtered := FilterToolsByGroupsCapped(assembleForTest(), allGroupsEnabled())

	perGroup := map[ToolGroup]int{}
	for _, tl := range filtered {
		g, ok := lookupCapability(tl)
		if !ok {
			t.Fatalf("工具 %s 未登记能力组（见 TestEveryAssembledToolIsDeclared）", tl.Name())
		}
		perGroup[g]++
	}
	for group, cap := range groupToolCaps {
		if perGroup[group] > cap {
			t.Errorf("组 %s 过滤后实际注入 %d 个工具，超过声明上限 %d —— cap 失效",
				group, perGroup[group], cap)
		}
	}
}

// 契约四：coreToolByGroup 里的名字必须真实存在于该组，且层级与注册表一致。
// 白名单写错名字原本会静默失效；现在必须被发现。
// （治理前 GetStockBillboard 被列在 news_research 白名单里，而它实际属于
//
//	stock_analysis —— 该条目永远不会命中，属于典型静默失效。）
func TestCoreWhitelistMatchesRegistry(t *testing.T) {
	for group, names := range coreToolByGroup {
		for _, name := range names {
			c, ok := CapabilityOf(name)
			if !ok {
				t.Errorf("核心白名单中的 %q（组 %s）未登记到能力注册表 —— 该名字写错后剪枝会静默失效", name, group)
				continue
			}
			if c.Group != group {
				t.Errorf("核心白名单 %q 归属组 %s，但注册表中它是 %s —— 两处不一致", name, group, c.Group)
			}
			if c.Tier != TierCore {
				t.Errorf("核心白名单 %q 在注册表中的层级是 %s，应为 %s", name, c.Tier, TierCore)
			}
		}
	}
}

// 契约五：「谁是核心工具」不得有两个答案。
// coreToolByGroup 与注册表 Tier==Core 必须完全等价，否则剪枝行为不可预测。
func TestCoreSemanticsHaveSingleSource(t *testing.T) {
	fromWhitelist := map[string]bool{}
	for _, names := range coreToolByGroup {
		for _, n := range names {
			fromWhitelist[n] = true
		}
	}
	fromRegistry := map[string]bool{}
	for _, c := range AllCapabilities() {
		if c.Tier == TierCore {
			fromRegistry[c.ID] = true
		}
	}
	var onlyWhitelist, onlyRegistry []string
	for n := range fromWhitelist {
		if !fromRegistry[n] {
			onlyWhitelist = append(onlyWhitelist, n)
		}
	}
	for n := range fromRegistry {
		if !fromWhitelist[n] {
			onlyRegistry = append(onlyRegistry, n)
		}
	}
	sort.Strings(onlyWhitelist)
	sort.Strings(onlyRegistry)
	if len(onlyWhitelist) != 0 || len(onlyRegistry) != 0 {
		t.Fatalf("「核心工具」存在两个答案：\n  只在白名单: %v\n  只在注册表: %v", onlyWhitelist, onlyRegistry)
	}
}

// 契约六：能力组必须是已定义的 ToolGroup 常量，防止拼写错误导致 cap 静默失效。
func TestDeclaredGroupsAreKnown(t *testing.T) {
	valid := map[ToolGroup]bool{
		GroupBase: true, GroupStockAnalysis: true, GroupMarket: true,
		GroupScreening: true, GroupMoneyFlow: true, GroupNewsResearch: true,
		GroupAIAnalysis: true, GroupOperations: true, GroupProject: true,
		GroupMCPServer: true,
	}
	for _, c := range AllCapabilities() {
		if !valid[c.Group] {
			t.Errorf("工具 %q 登记的组 %q 不是已定义的 ToolGroup 常量 —— cap 与白名单都会对它失效", c.ID, c.Group)
		}
	}
}

// 契约七：工具名是能力主键，也是去重依据，不得为空或含首尾空白。
func TestCapabilityIDsAreWellFormed(t *testing.T) {
	for _, c := range AllCapabilities() {
		if strings.TrimSpace(c.ID) != c.ID || c.ID == "" {
			t.Errorf("能力 ID %q 含首尾空白或为空，主键/去重会失准", c.ID)
		}
		if c.Source == SourceBuiltin && c.Group == "" {
			t.Errorf("内置能力 %q 未声明组（空组会被当通用基础工具无限量注入）", c.ID)
		}
	}
}

// 契约八（观察性，不失败）：组内工具总量超出 cap 的情况必须可见。
// 这些溢出工具当前会被白名单逻辑裁掉；C/D 期改为按 token 预算时，
// 这张清单就是精准剪枝的输入。
func TestGroupOverflowIsObservable(t *testing.T) {
	total := map[ToolGroup]int{}
	for _, tl := range assembleForTest() {
		g, ok := lookupCapability(tl)
		if !ok {
			continue
		}
		total[g]++
	}
	var overflow []string
	for group, cap := range groupToolCaps {
		if total[group] > cap {
			overflow = append(overflow, fmt.Sprintf("%s %d/%d(溢出%d)",
				group, total[group], cap, total[group]-cap))
		}
	}
	sort.Strings(overflow)
	t.Logf("组内总量超出 cap 的组（超出部分会被裁掉，C/D 期改 token 预算时的输入）: %v", overflow)
}

// ── B 期策略回归（用户 2026-11-09 明确决策）─────────────────────────────
//
// 这些用例把「用户在意什么」固化成可执行断言，防止后续改动悄悄回退：
//   · 基金 K 线 / TDX 系列：用户在意义模型能调用；
//   · MCP 管理类：用户不在意 → 已独立成组，C 期整组闸门裁剪。

func mustCapability(t *testing.T, name string) Capability {
	t.Helper()
	c, ok := CapabilityOf(name)
	if !ok {
		t.Fatalf("工具 %s 未登记能力（应先修 TestEveryAssembledToolIsDeclared）", name)
	}
	return c
}

// 契约九：用户明确「在意」的能力，不得被登记为 Niche（否则预算不足时最先被裁）。
func TestUserCaredCapabilitiesAreNotNiche(t *testing.T) {
	cared := []string{
		// 基金 K 线系列
		"GetFundKLine", "GetFundHistoryNetValue", "GetFundTop10Holdings",
		// TDX 系列
		"GetTdxCompanyCategory", "GetTdxCompanyInfo", "GetTdxFinanceInfo",
		"GetTdxSymbolBelongBoard", "GetTdxXDXRInfo",
	}
	for _, name := range cared {
		c := mustCapability(t, name)
		if c.Tier == TierNiche {
			t.Errorf("用户明确在意义模型调用 %s，但它被登记为 niche —— 预算不足时会最先被裁掉", name)
		}
		if c.Group != GroupStockAnalysis {
			t.Errorf("用户在意 %s，期望归入 stock_analysis，实际 %s", name, c.Group)
		}
	}
}

// 契约十：用户不在意模型调用的 MCP 管理类工具，必须全部集中在 GroupMCPServer，
// 且**一个都不能漏**（漏一个就会被当成通用工具全量注入）。
func TestMCPServerAdminToolsAreIsolated(t *testing.T) {
	admin := []string{
		"ListMCPServers", "GetMCPServerDetail", "CreateMCPServer", "UpdateMCPServer",
		"DeleteMCPServer", "EnableMCPServer", "TestMCPServer",
		"ListMCPServerTools", "GetMCPToolDetail",
	}
	for _, name := range admin {
		c := mustCapability(t, name)
		if c.Group != GroupMCPServer {
			t.Errorf("MCP 管理类工具 %s 应集中在 GroupMCPServer（便于 C 期整组闸门裁剪），实际 %s", name, c.Group)
		}
		if c.Source != SourceBuiltin {
			t.Errorf("%s 是本仓库内置工具，Source 应为 builtin，实际 %s", name, c.Source)
		}
	}
	// 反向检查：不得有别的工具被误放进该组
	for _, c := range AllCapabilities() {
		if c.Group == GroupMCPServer {
			found := false
			for _, n := range admin {
				if c.ID == n {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("工具 %s 被误归入 mcp_server 组 —— 会被 C 期闸门连带裁掉", c.ID)
			}
		}
	}
}

// 契约十一：MCP 管理类必须真的出现在注入列表里（否则闸门裁了个空组，
// 而这些运维工具仍会全量下发）。
func TestMCPServerAdminToolsAreActuallyAssembled(t *testing.T) {
	present := map[string]bool{}
	for _, tl := range assembleForTest() {
		present[tl.Name()] = true
	}
	admin := []string{
		"ListMCPServers", "GetMCPServerDetail", "CreateMCPServer", "UpdateMCPServer",
		"DeleteMCPServer", "EnableMCPServer", "TestMCPServer",
		"ListMCPServerTools", "GetMCPToolDetail",
	}
	for _, n := range admin {
		if !present[n] {
			t.Errorf("MCP 管理类工具 %s 未被注入列表包含 —— C 期闸门将无法裁掉它（治理失效）", n)
		}
	}
}
