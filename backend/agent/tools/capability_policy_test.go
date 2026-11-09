package tools

import (
	"sort"
	"strings"
	"testing"
)

// B 期策略回归测试：把用户 2026-11-09 的明确决策固化成可执行断言。
//
// 决策原文：
//
//	B 期那 20 个未映射工具，逐个判定后声明式保留；
//	在意模型能调用基金 K 线、TDX 系列，MCP 管理类工具不在意。
//
// 没有这些用例，下面的问题会在某次重构后悄悄回来且无人发现：
//   · 用户在意的能力被分组上限挤出，模型永远调不到；
//   · 用户不在意的运维工具又被全量塞回提示词。

// caredCapabilities 用户明确在意义表能够调用的能力（含所属组）。
var caredCapabilities = []string{
	// 基金 K 线系列
	"GetFundKLine", "GetFundHistoryNetValue", "GetFundTop10Holdings",
	// TDX 系列（本地通达信数据源）
	"GetTdxCompanyCategory", "GetTdxCompanyInfo", "GetTdxFinanceInfo",
	"GetTdxSymbolBelongBoard", "GetTdxXDXRInfo",
}

// mcpAdminCapabilities 用户明确不在意义表调用的 MCP 管理类能力。
// 它们与业务能力无关，属运维/配置面，C 期将整组闸门裁剪。
var mcpAdminCapabilities = []string{
	"ListMCPServers", "GetMCPServerDetail", "CreateMCPServer", "UpdateMCPServer",
	"DeleteMCPServer", "EnableMCPServer", "TestMCPServer",
	"ListMCPServerTools", "GetMCPToolDetail",
}

// 契约一：用户在意 → 必须归入 stock_analysis 且**不得是 Niche**。
// （Extended 也不行：非核心白名单成员会被两轮填充挤出，见契约二。）
func TestCaredCapabilitiesAreProtected(t *testing.T) {
	for _, name := range caredCapabilities {
		c, ok := CapabilityOf(name)
		if !ok {
			t.Fatalf("用户在意 %s，但它未登记能力组", name)
		}
		if c.Group != GroupStockAnalysis {
			t.Errorf("用户在意 %s：期望 stock_analysis，实际 %s", name, c.Group)
		}
		if c.Tier == TierNiche {
			t.Errorf("用户在意 %s：不得为 niche（预算不足时最先被裁）", name)
		}
	}
}

// 契约二（关键）：用户在意 → 必须在**真实问题**下确实注入给模型。
//
// 这条用例抓住过一个真问题：这些工具虽归入 stock_analysis，但既非核心白名单
// 成员、排序又靠后，于是在「白名单优先 + 按序补足到 cap」的两轮填充里被挤出，
// 结果用户说「在意」，模型却永远调不到。现通过提为核心白名单 + 相应上调 cap 保证。
func TestCaredCapabilitiesAreReachable(t *testing.T) {
	all := assembleForTest()
	questions := []string{
		"帮我看看贵州茅台600519的基金K线",
		"查一下通达信的财务数据",
		"分析这只股票的基本面",
		"600519",
		"基金持仓和净值走势帮我看看",
	}
	for _, q := range questions {
		groups := ClassifyQuestion(q)
		filtered := FilterToolsByGroupsCapped(all, groups)
		present := map[string]bool{}
		for _, tl := range filtered {
			present[tl.Name()] = true
		}
		if !present["GetFundKLine"] {
			t.Errorf("问题 %q 未注入 GetFundKLine（用户在意，命中组=%v）", q, sortedGroupNames(groups))
		}
		if !present["GetStockInfo"] {
			t.Errorf("问题 %q 未注入基础工具 GetStockInfo", q)
		}
	}
}

// 契约三：用户不在意的 MCP 管理类 → 必须集中在 GroupMCPServer，一个都不能漏。
// 漏一个就会被当成通用业务工具全量注入（正是治理前的状态）。
func TestMCPAdminCapabilitiesAreIsolated(t *testing.T) {
	for _, name := range mcpAdminCapabilities {
		c, ok := CapabilityOf(name)
		if !ok {
			t.Fatalf("MCP 管理类 %s 未登记能力组", name)
		}
		if c.Group != GroupMCPServer {
			t.Errorf("MCP 管理类 %s 应集中在 GroupMCPServer，实际 %s —— 漏网者会被当业务工具注入", name, c.Group)
		}
		if c.Source != SourceBuiltin {
			t.Errorf("%s 是本仓库内置工具，Source 应为 builtin，实际 %s", name, c.Source)
		}
	}
	// 反向：不得有别的工具被误放进该组（否则会被连带裁掉）
	allowed := map[string]bool{}
	for _, n := range mcpAdminCapabilities {
		allowed[n] = true
	}
	for _, c := range AllCapabilities() {
		if c.Group == GroupMCPServer && !allowed[c.ID] {
			t.Errorf("工具 %s 被误归入 mcp_server 组，会被 C 期闸门连带裁掉", c.ID)
		}
	}
}

// 契约四（隔离的有效性证明）：MCP 管理类在**任何**问题下都不得注入给模型。
// 这条用例区分「声明上归了组」与「实际上注不进去」——只有后者才是真隔离。
func TestMCPAdminCapabilitiesAreNotInjected(t *testing.T) {
	all := assembleForTest()
	admin := map[string]bool{}
	for _, n := range mcpAdminCapabilities {
		admin[n] = true
	}
	// 覆盖各类提问，包括直接提到 MCP/外部工具的场景
	questions := []string{
		"分析一下贵州茅台", "600519 现在能买吗", "ETF轮动帮我看看", "今天大盘怎么样",
		"帮我查下最近龙虎榜", "MCP服务器怎么配置", "外部工具怎么用", "你好",
		"帮我写份报告", "查询通达信数据", "基金K线看看", "帮我看看代码",
		"复盘一下今天的盘面", "龙虎榜单", "增减持", "F10", "debug",
	}
	for _, q := range questions {
		filtered := FilterToolsByGroupsCapped(all, ClassifyQuestion(q))
		var leaked []string
		for _, tl := range filtered {
			if admin[tl.Name()] {
				leaked = append(leaked, tl.Name())
			}
		}
		if len(leaked) > 0 {
			t.Errorf("问题 %q 仍注入了 MCP 管理类工具（隔离失效）: %v", q, leaked)
		}
	}
}

// 契约五：MCP 管理类必须**确实在组装列表里**。
// 否则闸门裁的是空组，而这些运维工具仍会经别的路径全量下发。
func TestMCPAdminCapabilitiesAreAssembled(t *testing.T) {
	present := map[string]bool{}
	for _, tl := range assembleForTest() {
		present[tl.Name()] = true
	}
	for _, n := range mcpAdminCapabilities {
		if !present[n] {
			t.Errorf("MCP 管理类 %s 未出现在组装列表中 —— 闸门无从裁剪（隔离是假象）", n)
		}
	}
}

// 契约六：解密「cap 上调」的代价必须可见。
// 把基金/TDX 提为核心白名单后，stock_analysis 上限由 20 提到 26，
// 这会让每个命中该组的请求都多发 6 个工具定义。D 期改用 token 预算时，
// 这条日志就是评估依据。
func TestStockAnalysisCapCostIsVisible(t *testing.T) {
	all := assembleForTest()
	groups := ClassifyQuestion("分析贵州茅台600519")
	filtered := FilterToolsByGroupsCapped(all, groups)
	coreCount := len(coreToolByGroup[GroupStockAnalysis])
	t.Logf("stock_analysis: cap=%d, 核心白名单=%d, 本次实际问题(%q)注入=%d 个工具",
		groupToolCaps[GroupStockAnalysis], coreCount, "分析贵州茅台600519", len(filtered))
	if coreCount > groupToolCaps[GroupStockAnalysis] {
		t.Fatalf("核心白名单 %d 超过 cap %d —— 第一轮会超发", coreCount, groupToolCaps[GroupStockAnalysis])
	}
}

// 契约七：溢出清单保持可读（兜底防呆，确保没有组被静默遗忘）。
func TestOverflowReportIsComplete(t *testing.T) {
	total := map[ToolGroup]int{}
	for _, tl := range assembleForTest() {
		if g, ok := lookupCapability(tl); ok {
			total[g]++
		}
	}
	var lines []string
	for group, cap := range groupToolCaps {
		if total[group] > cap {
			lines = append(lines, string(group)+":"+itoa(total[group])+"/"+itoa(cap))
		}
	}
	sort.Strings(lines)
	t.Logf("溢出组（超限部分被裁，D 期改 token 预算的输入）: %s", strings.Join(lines, "  "))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// sortedGroupNames 命中的组名（排序后，便于断言输出稳定）。
func sortedGroupNames(groups map[ToolGroup]bool) []string {
	var out []string
	for g, on := range groups {
		if on {
			out = append(out, string(g))
		}
	}
	sort.Strings(out)
	return out
}
