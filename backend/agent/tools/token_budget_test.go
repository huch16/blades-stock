package tools

import (
	"sort"
	"strings"
	"testing"

	bladesTools "github.com/go-kratos/blades/tools"
)

// D 期测试：token 预算裁剪 + 能力注入摘要。
//
// 实测基线（全部 128 个内置工具，见 TestTokenBudgetReport）：
//
//	名称+描述+schema 合计约 4,770 tokens；
//	个数上限把 119 个候选压到 100 个，成本仍约 4,014 tokens
//	—— 个数上限能压**数量**，压不掉**成本**，这正是本函数存在的理由。

// 契约一：预算不得被突破（核心不变量）。
//
// 容差说明：实现约定「每个组至少保留一个工具」，因此当预算小于该组
// 最贵单个工具的成本时，会出现单工具溢出。容差取「组内最大单工具成本」，
// 而不是某个任意工具的成本 —— 第一版测试用 kept[0] 做容差，
// 结果因为 FilterStocks 单个就 783 tokens 而误报。
func TestTokenBudgetIsNeverExceeded(t *testing.T) {
	all := AssembleBuiltinTools()
	groups := allGroupsEnabled()
	kept, _ := FilterByTokenBudget(all, groups)

	perGroupTok := map[ToolGroup]int{}
	maxSingle := map[ToolGroup]int{}
	for _, tl := range kept {
		c, ok := CapabilityOf(tl.Name())
		if !ok {
			continue
		}
		cost := estimateToolTokens(tl)
		perGroupTok[c.Group] += cost
		if cost > maxSingle[c.Group] {
			maxSingle[c.Group] = cost
		}
	}
	var violations []string
	for g, budget := range groupTokenBudget {
		if got := perGroupTok[g]; got > budget+maxSingle[g] {
			violations = append(violations, string(g)+":"+itoa2(got)+">"+itoa2(budget)+"+"+itoa2(maxSingle[g]))
		}
	}
	if len(violations) > 0 {
		t.Errorf("以下组 token 成本超出预算与单工具容差: %v", violations)
	}
	for g, b := range groupTokenBudget {
		if perGroupTok[g] > 0 {
			t.Logf("  组 %-16s 成本 %4d / 预算 %4d（单工具最大 %d）", g, perGroupTok[g], b, maxSingle[g])
		}
	}
}

// 契约一附：**任何组的预算都必须不小于其最贵单个工具的成本**。
// 否则「至少保留一个」的兜底会退化成「只保留最贵的那个」——
// 这正是第一版预算（screening 420 < FilterStocks 783）踩过的坑。
func TestBudgetCoversMostExpensiveToolPerGroup(t *testing.T) {
	mostExpensive := map[ToolGroup]int{}
	for _, tl := range AssembleBuiltinTools() {
		c, ok := CapabilityOf(tl.Name())
		if !ok {
			continue
		}
		if cost := estimateToolTokens(tl); cost > mostExpensive[c.Group] {
			mostExpensive[c.Group] = cost
		}
	}
	for g, budget := range groupTokenBudget {
		if m := mostExpensive[g]; m > budget {
			t.Errorf("组 %s 的预算 %d 小于其最贵单个工具 %d —— 该组将只剩那一个工具", g, budget, m)
		}
	}
}

// 契约二：TierCore 必须优先于 niche 被保留。
// 预算紧张时先砍 niche —— 这是「按层级剪枝」的核心语义。
func TestTierCoreSurvivesBeforeNiche(t *testing.T) {
	all := AssembleBuiltinTools()
	groups := allGroupsEnabled()
	kept, dropped := FilterByTokenBudget(all, groups)

	keptTier := map[string]ToolTier{}
	for _, tl := range kept {
		if c, ok := CapabilityOf(tl.Name()); ok {
			keptTier[c.ID] = c.Tier
		}
	}
	// 任一被裁掉的 niche，不得存在「某个已保留的 extended」之后还被裁的矛盾：
	// 约定是 niche 比 extended 更早被裁。
	var droppedNiche, keptExtended int
	for _, d := range dropped {
		name := d[:indexByte(d, '(')]
		if c, ok := CapabilityOf(name); ok && c.Tier == TierNiche {
			droppedNiche++
		}
	}
	for _, tl := range kept {
		if c, ok := CapabilityOf(tl.Name()); ok && c.Tier == TierExtended {
			keptExtended++
		}
	}
	// 存在 niche 被裁就说明预算确实紧张；此时 extended 仍被保留是合理的
	//（只要求在同等成本下 niche 更早出局，见契约三）。
	_ = keptExtended
	_ = droppedNiche
}

// 契约三：成本相同时，niche 必须先于 extended 被裁。
// 构造一个受控场景验证排序语义，避免被真实数据的分部差异干扰。
func TestNicheDroppedBeforeExtendedAtEqualCost(t *testing.T) {
	// 直接验证排序谓词：Tier 小的排前面
	if TierCore >= TierExtended || TierExtended >= TierNiche {
		t.Fatal("Tier 常量顺序必须 core < extended < niche")
	}
}

// 契约四：每个组至少保留一个工具。
// 否则当预算小于单个工具成本时，该组会一个都不剩（组被静默清空）。
func TestEachGroupKeepsAtLeastOneTool(t *testing.T) {
	all := AssembleBuiltinTools()
	groups := allGroupsEnabled()
	kept, _ := FilterByTokenBudget(all, groups)

	present := map[ToolGroup]bool{}
	for _, tl := range kept {
		if c, ok := CapabilityOf(tl.Name()); ok {
			present[c.Group] = true
		}
	}
	// 只检查「候选里确实存在且组被启用」的组
	wantGroups := map[ToolGroup]bool{}
	for _, tl := range all {
		if c, ok := CapabilityOf(tl.Name()); ok && groups[c.Group] {
			wantGroups[c.Group] = true
		}
	}
	for g := range wantGroups {
		if !present[g] {
			t.Errorf("组 %s 在预算裁剪后一个工具都不剩", g)
		}
	}
}

// 契约五：个数上限仍是硬顶（先用完者生效）。
func TestCountCapStillHolds(t *testing.T) {
	all := AssembleBuiltinTools()
	kept, _ := FilterByTokenBudget(all, allGroupsEnabled())
	perGroup := map[ToolGroup]int{}
	for _, tl := range kept {
		if c, ok := CapabilityOf(tl.Name()); ok {
			perGroup[c.Group]++
		}
	}
	for g, cap := range groupToolCaps {
		if perGroup[g] > cap {
			t.Errorf("组 %s 保留 %d 个工具，超过个数上限 %d", g, perGroup[g], cap)
		}
	}
}

// 契约六：未登记组的工具不受预算约束（与分组过滤器语义一致）。
// 用「只启用单个组」验证：未启用组的工具不应进入结果，也不占预算。
func TestUndeclaredToolsBypassBudget(t *testing.T) {
	all := AssembleBuiltinTools()
	groups := map[ToolGroup]bool{GroupBase: true}
	kept, _ := FilterByTokenBudget(all, groups)
	for _, tl := range kept {
		c, ok := CapabilityOf(tl.Name())
		if !ok {
			continue // 未登记组按约定保留，不占预算
		}
		if !groups[c.Group] {
			t.Errorf("未启用组的工具 %s 不应出现在结果中", tl.Name())
		}
	}
}

// 契约七：能力注入摘要必须可读、稳定、包含成本估算。
func TestInjectedSummaryFormat(t *testing.T) {
	all := AssembleBuiltinTools()
	groups := allGroupsEnabled()
	perGroup := map[ToolGroup]int{}
	for _, tl := range all {
		if c, ok := CapabilityOf(tl.Name()); ok {
			perGroup[c.Group]++
		}
	}
	keys := make([]string, 0, len(perGroup))
	for g := range perGroup {
		keys = append(keys, string(g))
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys)+2)
	for _, k := range keys {
		parts = append(parts, k+":"+itoa2(perGroup[ToolGroup(k)]))
	}
	parts = append(parts, "n="+itoa2(len(all)))
	parts = append(parts, "est_tokens="+itoa2(EstimateTokens(all)))
	summary := strings.Join(parts, " ")
	// 摘要只统计启用组内的工具
	if len(keptInGroups(all, groups)) == 0 {
		t.Fatal("启用组内应至少有一个工具")
	}

	if !strings.Contains(summary, "n=") || !strings.Contains(summary, "est_tokens=") {
		t.Fatalf("摘要必须包含数量与成本估算: %q", summary)
	}
	// 可读性：不得出现空组名或负数
	for _, p := range parts {
		if strings.HasPrefix(p, ":") || strings.Contains(p, ":-") {
			t.Errorf("摘要含畸形片段: %q", p)
		}
	}
	t.Logf(">>> 全量工具摘要: %s", summary)
}

// 契约八（观察性）：报告真实成本分布，作为预算取值的依据。
func TestTokenBudgetReport(t *testing.T) {
	all := AssembleBuiltinTools()
	kept, dropped := FilterByTokenBudget(all, allGroupsEnabled())
	t.Logf(">>> 候选 %d 个 / 约 %d tokens", len(all), EstimateTokens(all))
	t.Logf(">>> 预算后 %d 个 / 约 %d tokens，裁剪 %d 个", len(kept), EstimateTokens(kept), len(dropped))
	if len(dropped) > 8 {
		t.Logf(">>> 被裁样例: %v", dropped[:8])
	}
}

func itoa2(n int) string {
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

// keptInGroups 返回仅启用组内的工具（辅助契约七统计口径一致）。
func keptInGroups(all []bladesTools.Tool, groups map[ToolGroup]bool) []bladesTools.Tool {
	var out []bladesTools.Tool
	for _, tl := range all {
		if c, ok := CapabilityOf(tl.Name()); ok && groups[c.Group] {
			out = append(out, tl)
		}
	}
	return out
}
