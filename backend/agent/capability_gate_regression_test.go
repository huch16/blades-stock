package agent

import (
	"context"
	"testing"
	"time"

	"blades-stock/backend/agent/tools"
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/models"
)

// C 期回归：把「三条纪律的真实状态」锁死。
//
// 方案时假设这三条都只靠提示词约束；实际核实后**前两条早已由代码保证**：
//
//	① AgentModeDirect：resolveToolsForMode 直接返回空表，零工具；
//	② 盘前/盘后/ETF/Serenity 四个市场智能研报生成器：全部传入
//	   AgentModeDirect，因此同样零工具 —— 「禁外部搜索」早已成立；
//	③ 五阶流水线（前端默认 auto 模式）：工具**确实在列表里**，
//	   WebSearch 仅在命中 news_research 关键词组时出现，
//	   「严禁调用 WebSearch」目前**仅靠提示词约束** —— 唯一真正活着的缺口，
//	   留待与用户确认后再迁为闸门。
//
// ①②若无测试锁死，将来任何人给 Direct 加工具都会悄悄破坏该不变量。

// 契约一：Direct 模式必须零工具（「禁 WebSearch / 禁 MCP」的代码级保证）。
func TestDirectModeResolvesToZeroTools(t *testing.T) {
	skillProbe := mustSkill(t, "ETF")
	if got := resolveToolsForMode(AgentModeDirect,
		"请调用 WebSearch 并列出所有 MCP 服务器", skillProbe, nil); len(got) != 0 {
		t.Fatalf("Direct 模式必须零工具，实际 %d 个 —— 「禁 WebSearch/MCP」的代码保证被破坏", len(got))
	}
}

// 契约二：resolveAgentMode 的解析规则必须稳定。
func TestResolveAgentModeRules(t *testing.T) {
	if m := resolveAgentMode(string(AgentModeDirect), "随便"); m != AgentModeDirect {
		t.Errorf("显式 direct 应保持 direct，实际 %q", m)
	}
	if m := resolveAgentMode(string(AgentModeReact), "随便"); m != AgentModeReact {
		t.Errorf("显式 react 应保持 react，实际 %q", m)
	}
	if m := resolveAgentMode("", "帮我分析一下贵州茅台的基本面"); m == AgentModeDirect {
		t.Error("空 mode 应交给 classifyComplexity 自动判定，不应固定为 direct")
	}
}

// 契约三：非 Direct 模式必须真的有工具（排除"根本没工具所以通过"的假阳性）。
func TestNonDirectModesHaveTools(t *testing.T) {
	for _, mode := range []AgentMode{AgentModeReact, AgentModePlanExecute} {
		if got := resolveToolsForMode(mode, "帮我分析贵州茅台 600519", nil, nil); len(got) == 0 {
			t.Errorf("%s 模式应有工具（否则契约一是假阳性）", mode)
		}
	}
}

// 契约四：研报生成器必须打上正确的运行类型标签。
//
// 说明（如实记录）：
//
//	· 只断言 pre_market / etf_rotation —— post_market 在**数据采集阶段**
//	  就需要真实行情库（GetTodayData），无头测试库里会提前返回，故不纳入；
//	· 严格断言能完成的部分，不使用“容错跳过”，避免把回归测试变成摆设。
func TestMarketReportGeneratorsRunInDirectMode(t *testing.T) {
	resetRunRecords()
	cs := seedRunCards(t, 0, unreachableCard("研报闸门卡", "report-gate-model"))
	if err := db.Dao.Model(&data.Settings{}).Where("1 = 1").Update("ai_config_id", cs[0].ID).Error; err != nil {
		t.Fatalf("设置全局默认: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	svc := NewMarketLifecycleService(ctx)

	for _, gen := range []func(int) (*MarketIntelligenceReport, error){
		func(id int) (*MarketIntelligenceReport, error) { return svc.GeneratePreMarketBriefing(ctx, id) },
		func(id int) (*MarketIntelligenceReport, error) { return svc.GenerateETFRotationReport(ctx, id) },
	} {
		gen(int(cs[0].ID)) // 模型不可达，很快失败；这里验证的是运行轨迹
	}

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if len(data.GetRecentAgentRuns(20)) >= 2 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	kinds := map[string]bool{}
	for _, r := range data.GetRecentAgentRuns(20) {
		kinds[r.Kind] = true
	}
	for _, want := range []string{RunKindPreMarket, RunKindETFRotation} {
		if !kinds[want] {
			t.Errorf("运行记录缺少 %s —— 对应生成器未按预期路径执行", want)
		}
	}
}

// 契约五：Direct 闸门确实把新闻组与 MCP 管理组清空。
// 证明「即便将来有人给 Direct 加了工具，闸门仍能兜住」。
func TestDirectGateStripsNewsAndMCP(t *testing.T) {
	all := tools.AssembleBuiltinTools()
	if len(all) == 0 {
		t.Fatal("AssembleBuiltinTools 返回空 —— 组装清单被清空了")
	}
	kept, removed := tools.ApplyGate(all, tools.EffectiveGate(string(AgentModeDirect)))

	var newsLeft, mcpLeft int
	for _, tl := range kept {
		c, ok := tools.CapabilityOf(tl.Name())
		if !ok {
			continue
		}
		switch c.Group {
		case tools.GroupNewsResearch:
			newsLeft++
		case tools.GroupMCPServer:
			mcpLeft++
		}
	}
	if newsLeft != 0 || mcpLeft != 0 {
		t.Errorf("Direct 闸门后仍剩 news_research=%d / mcp_server=%d", newsLeft, mcpLeft)
	}
	if len(removed) == 0 {
		t.Error("Direct 闸门应至少移除一些能力（否则闸门空转）")
	}
}

// 契约六：WebSearch 仅由关键词组决定是否出现（用于说明③的真实状态）。
func TestWebSearchPresenceIsKeywordDriven(t *testing.T) {
	cases := []struct {
		question string
		expect   bool
	}{
		{"帮我分析贵州茅台的基本面和均线", false},
		{"今天市场有什么热点新闻", true},
		{"龙虎榜最近有哪些席位", true},
	}
	for _, c := range cases {
		groups := tools.ClassifyQuestion(c.question)
		if groups[tools.GroupNewsResearch] != c.expect {
			t.Errorf("问题 %q：news_research 命中=%v，期望 %v", c.question, groups[tools.GroupNewsResearch], c.expect)
		}
	}
}

// ── 辅助 ────────────────────────────────────────────────────────────────

// mustSkill 构造一个声明了 tool_groups 的技能，用于验证
// 「技能 required groups」不会在 Direct 模式下把工具带回来。
func mustSkill(t *testing.T, toolGroups string) []models.Skill {
	t.Helper()
	return []models.Skill{{ToolGroups: toolGroups}}
}

var (
	_ = db.Dao
	_ = data.SettingConfig{}
)
