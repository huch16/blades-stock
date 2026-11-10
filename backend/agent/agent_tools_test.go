package agent

import (
	"context"
	"testing"

	"blades-stock/backend/agent/tools"
	"blades-stock/backend/models"
)

func TestGetAllTools(t *testing.T) {
	allTools := getToolsByQuestion(context.Background(), "分析一下茅台的股票行情和资金流向", nil)
	t.Logf("Total tools count: %d", len(allTools))

	toolNames := make(map[string]int)
	for i, tool := range allTools {
		name := tool.Name()
		desc := tool.Description()
		t.Logf("Tool %d: %s - %s", i+1, name, desc)

		if count, exists := toolNames[name]; exists {
			t.Errorf("Duplicate tool name found: %s (previous count: %d)", name, count)
		}
		toolNames[name]++
	}

	t.Log("\n=== Checking for duplicates ===")
	duplicates := []string{}
	for name, count := range toolNames {
		if count > 1 {
			duplicates = append(duplicates, name)
			t.Errorf("Duplicate tool: %s (count: %d)", name, count)
		}
	}

	if len(duplicates) == 0 {
		t.Log("No duplicate tools found!")
	}
}

func TestAgentGoTools(t *testing.T) {
	t.Log("Testing agent.go tools:")
	agentToolsList := []string{
		"GetQueryEconomicDataTool",
		"GetQueryStockPriceInfoTool",
		"GetQueryStockCodeInfoTool",
		"GetQueryMarketNewsTool",
		"GetChoiceStockByIndicatorsTool",
		"GetStockKLineTool",
		"GetInteractiveAnswerDataTool",
		"GetFinancialReportTool",
		"GetQueryStockNewsTool",
		"GetIndustryResearchReportTool",
		"GetQueryBKDictTool",
	}

	for _, name := range agentToolsList {
		t.Logf("Tool: %s exists", name)
	}

	dataTools := tools.GetAllDataTools()
	t.Logf("\nData tools count: %d", len(dataTools))

	for i, tool := range dataTools {
		t.Logf("Data tool %d: %s", i+1, tool.Name())
	}
}

func TestMergeSkillToolGroups(t *testing.T) {
	groups := map[tools.ToolGroup]bool{tools.GroupBase: true}
	skills := []models.Skill{
		{ToolGroups: "money_flow, news_research , stock_analysis, bogus_group"},
		{ToolGroups: "market"},
		{ToolGroups: ""},
	}
	added := mergeSkillToolGroups(groups, skills)

	want := []tools.ToolGroup{
		tools.GroupMoneyFlow, tools.GroupNewsResearch, tools.GroupStockAnalysis, tools.GroupMarket,
	}
	for _, g := range want {
		if !groups[g] {
			t.Errorf("group %s not merged", g)
		}
	}
	if groups[tools.GroupScreening] || groups[tools.GroupOperations] {
		t.Errorf("unexpected groups merged: %v", groups)
	}
	if len(added) != len(want) {
		t.Errorf("added = %v, want %d skill-contributed groups", added, len(want))
	}
}

func TestMergeSkillToolGroupsNilSkills(t *testing.T) {
	groups := map[tools.ToolGroup]bool{tools.GroupBase: true}
	if added := mergeSkillToolGroups(groups, nil); len(added) != 0 {
		t.Errorf("nil skills should add nothing, got %v", added)
	}
	if !groups[tools.GroupBase] {
		t.Error("base group should remain")
	}
}
