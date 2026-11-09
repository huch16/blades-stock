package tools

import (
	"sort"
	"testing"
)

// C 期：能力闸门测试。
//
// 三条纪律的**真实状态**（与方案假设不同，已用证据核实）：
//
//	① AgentModeDirect：GetStockAiAgent 的 switch 已令 allTools = []，
//	   零工具 → WebSearch/MCP 早已不可达。此处补回归测试**锁死**它，
//	   防止将来有人给 Direct 加工具时悄悄破坏该不变量。
//	② 盘前/盘后/ETF/Serenity 四个研报生成器：全部传入 AgentModeDirect，
//	   同样零工具 → 「禁外部搜索」早已成立。
//	③ 五阶流水线（前端默认 auto 模式）：**工具确实在列表里**，
//	   WebSearch 仅在命中 news_research 关键词组时出现，
//	   「严禁调用 WebSearch」目前**仅靠提示词约束**——这是唯一真正活着的缺口。
//
// 因此 C 期交付 = 闸门基础设施（默认影子模式）+ 锁死 ①② + ③ 留待确认。

func sortedRemoved(removed []string) []string {
	out := append([]string{}, removed...)
	sort.Strings(out)
	return out
}

// 契约一：空闸门不得改动任何东西（保证影子模式/默认路径零影响）。
func TestEmptyGateIsIdentity(t *testing.T) {
	all := assembleForTest()
	kept, removed := ApplyGate(all, Gate{})
	if len(kept) != len(all) {
		t.Fatalf("空闸门应原样返回：got %d want %d", len(kept), len(all))
	}
	if len(removed) != 0 {
		t.Fatalf("空闸门不应移除任何工具: %v", removed)
	}
}

// 契约二：禁组必须整组移除，且一个不漏。
func TestGateForbidsWholeGroup(t *testing.T) {
	all := assembleForTest()
	gate := Gate{ForbidGroups: []ToolGroup{GroupMCPServer}}
	kept, removed := ApplyGate(all, gate)

	// 被移除的必须恰好是 mcp_server 组的全部内置工具
	wantRemoved := map[string]bool{}
	for _, c := range AllCapabilities() {
		if c.Group == GroupMCPServer && c.Source == SourceBuiltin {
			wantRemoved[c.ID] = true
		}
	}
	if len(removed) != len(wantRemoved) {
		t.Fatalf("应移除 %d 个 mcp_server 工具，实际 %d: %v", len(wantRemoved), len(removed), removed)
	}
	for _, r := range removed {
		name := r[:indexByte(r, '(')]
		if !wantRemoved[name] {
			t.Errorf("移除了非目标工具: %q", r)
		}
	}
	// 保留的不得含目标组
	for _, tl := range kept {
		if c, ok := CapabilityOf(tl.Name()); ok && c.Group == GroupMCPServer {
			t.Errorf("目标组工具 %s 仍被保留", tl.Name())
		}
	}
}

// 契约三：禁单个工具必须精确命中，不波及同组其他工具。
func TestGateForbidsSingleTool(t *testing.T) {
	all := assembleForTest()
	gate := Gate{ForbidTools: []string{"GetStockInfo"}}
	kept, removed := ApplyGate(all, gate)
	if len(removed) != 1 || removed[0] != "GetStockInfo(tool)" {
		t.Fatalf("应只移除 GetStockInfo，实际 %v", removed)
	}
	// 同组其他工具（如 GetStockKLine）必须保留
	var hasKLine bool
	for _, tl := range kept {
		if tl.Name() == "GetStockKLine" {
			hasKLine = true
		}
	}
	if !hasKLine {
		t.Error("禁单个工具时误伤了同组其他工具")
	}
}

// 契约四：Direct 模式的默认闸门必须覆盖新闻组与 MCP 管理组。
func TestGateForDirectMode(t *testing.T) {
	g := GateForMode("direct")
	if g.IsZero() {
		t.Fatal("Direct 模式的闸门不应为空")
	}
	fg := g.forbidGroupSet()
	if !fg[GroupNewsResearch] {
		t.Error("Direct 闸门应禁用 news_research（外部搜索/网页抓取）")
	}
	if !fg[GroupMCPServer] {
		t.Error("Direct 闸门应禁用 mcp_server（运维面）")
	}
	// 未知/auto 模式默认不加约束（避免误伤）
	if !GateForMode("").IsZero() {
		t.Error("空 mode（auto）默认不应施加约束")
	}
	if !GateForMode("react").IsZero() {
		t.Error("react 模式默认不应施加约束")
	}
}

// 契约五：EffectiveGate 必须合并内置策略与覆盖配置。
func TestEffectiveGateMergesOverride(t *testing.T) {
	t.Setenv("BLADES_STOCK_GATE_OVERRIDE", `{"forbidTools":["SomeTool"]}`)
	g := EffectiveGate("direct")
	if !g.forbidToolSet()["SomeTool"] {
		t.Error("覆盖配置中的 forbidTools 未合并进来")
	}
	if !g.forbidGroupSet()[GroupNewsResearch] {
		t.Error("合并后丢失了内置策略的组约束")
	}
}

// 契约六：影子模式默认关闭，必须显式打开（保证默认零行为变化）。
func TestGateEnforceDefaultsToShadow(t *testing.T) {
	// 注意：t.Setenv 在此测试内不影响其他测试的读取顺序
	t.Setenv("BLADES_STOCK_GATE_ENFORCE", "")
	if GateEnforceEnabled() {
		t.Error("未显式开启时闸门必须是影子模式（不实际裁剪）")
	}
	t.Setenv("BLADES_STOCK_GATE_ENFORCE", "1")
	if !GateEnforceEnabled() {
		t.Error("显式设为 1 后闸门应生效")
	}
	t.Setenv("BLADES_STOCK_GATE_ENFORCE", "true")
	if !GateEnforceEnabled() {
		t.Error("true 也应生效")
	}
}

// 契约七：Direct 模式下闸门确实把新闻组工具清空（防止将来 Direct 被加工具）。
// 这说明即便有人给 Direct 加了工具，闸门仍能兜住「禁外部搜索」的不变量。
func TestDirectGateWouldStripNewsTools(t *testing.T) {
	all := assembleForTest()
	gate := EffectiveGate("direct")
	kept, removed := ApplyGate(all, gate)

	var newsLeft int
	for _, tl := range kept {
		if c, ok := CapabilityOf(tl.Name()); ok && c.Group == GroupNewsResearch {
			newsLeft++
		}
	}
	if newsLeft != 0 {
		t.Errorf("Direct 闸门后 news_research 仍剩 %d 个工具", newsLeft)
	}
	var mcpLeft int
	for _, tl := range kept {
		if c, ok := CapabilityOf(tl.Name()); ok && c.Group == GroupMCPServer {
			mcpLeft++
		}
	}
	if mcpLeft != 0 {
		t.Errorf("Direct 闸门后 mcp_server 仍剩 %d 个工具", mcpLeft)
	}
	t.Logf("Direct 闸门：%d → %d 个工具，移除 %v", len(all), len(kept), sortedRemoved(removed))
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return len(s)
}

// ── pipeline 模式（用户 2026-11-10 选定方案 A）─────────────────────────
//
// 前端流水线入口显式传 pipeline，后端据此把「严禁调用 WebSearch」
// 从提示词口头纪律升级为代码级闸门。

// 契约八：pipeline 闸门必须且只能禁掉外网检索工具。
func TestPipelineGateForbidsOnlyExternalSearch(t *testing.T) {
	g := GateForMode(AgentModePipeline)
	if g.IsZero() {
		t.Fatal("pipeline 模式必须有闸门（否则流水线纪律仍只靠提示词）")
	}
	ft := g.forbidToolSet()
	for _, must := range []string{"WebSearch", "FetchWebPage"} {
		if !ft[must] {
			t.Errorf("pipeline 闸门应禁用 %s", must)
		}
	}
	// 不得禁用一手内置数据源（组内其他工具仍应可用）
	for _, mustNot := range []string{
		"QueryStockNewsTool", "GetNewsListData", "StockNotice", "GetStockNotice",
		"GetStockInfo", "GetMarketData", "ScreenByStrategy",
	} {
		if ft[mustNot] {
			t.Errorf("pipeline 闸门不应禁用一手内置工具 %s", mustNot)
		}
	}
	// 不得整组禁用 news_research（那会连带砍掉一手新闻工具）
	if g.forbidGroupSet()[GroupNewsResearch] {
		t.Error("pipeline 闸门不应整组禁用 news_research —— 会误伤一手内置数据源")
	}
}

// 契约九：pipeline 闸门默认**立即生效**（非 Deferred）。
// 它是新建 mode，只影响流水线自身，不改变任何既有调用方。
func TestPipelineGateIsNotDeferred(t *testing.T) {
	g := GateForMode(AgentModePipeline)
	if g.Deferred {
		t.Error("pipeline 闸门不应标为 Deferred —— 那是给「迁移已有纪律」用的")
	}
	if !g.Active() {
		t.Error("pipeline 闸门应默认生效（即使未设 BLADES_STOCK_GATE_ENFORCE）")
	}
	// 对比：direct 闸门标为 Deferred，默认只影子观察
	d := GateForMode("direct")
	if !d.Deferred {
		t.Error("direct 闸门应标为 Deferred（其行为现状已满足约束，先看差异）")
	}
}

// 契约十：pipeline 闸门确实把 WebSearch/FetchWebPage 移除，其余保留。
func TestPipelineGateStripsExternalSearchOnly(t *testing.T) {
	all := AssembleBuiltinTools()
	kept, removed := ApplyGate(all, EffectiveGate(AgentModePipeline))

	removedSet := map[string]bool{}
	for _, r := range removed {
		removedSet[r[:indexByte(r, '(')]] = true
	}
	if !removedSet["WebSearch"] || !removedSet["FetchWebPage"] {
		t.Fatalf("应移除 WebSearch 与 FetchWebPage，实际 %v", removed)
	}
	if len(removed) != 2 {
		t.Errorf("pipeline 闸门应只移除 2 个外网检索工具，实际 %d: %v", len(removed), removed)
	}
	// 一手新闻工具必须保留
	var hasNewsTool, hasInfo bool
	for _, tl := range kept {
		switch tl.Name() {
		case "QueryStockNewsTool", "GetNewsListData":
			hasNewsTool = true
		case "GetStockInfo":
			hasInfo = true
		}
	}
	if !hasNewsTool || !hasInfo {
		t.Errorf("pipeline 闸门误伤了一手工具（newsTool=%v info=%v）", hasNewsTool, hasInfo)
	}
}

// 契约十一：环境变量覆盖必须保持 Deferred（运维旋钮，先看差异再收紧）。
func TestOverrideGateStaysDeferred(t *testing.T) {
	t.Setenv("BLADES_STOCK_GATE_OVERRIDE", `{"forbidTools":["GetStockKLine"]}`)
	g := EffectiveGate("react")
	if !g.Deferred {
		t.Error("环境变量覆盖的闸门应标为 Deferred")
	}
	if !g.forbidToolSet()["GetStockKLine"] {
		t.Error("覆盖配置的 forbidTools 未合并")
	}
	if g.Active() {
		t.Error("Deferred 闸门在未开启 ENFORCE 时不应生效")
	}
	t.Setenv("BLADES_STOCK_GATE_ENFORCE", "1")
	if !EffectiveGate("react").Active() {
		t.Error("显式开启 ENFORCE 后 Deferred 闸门应生效")
	}
}
