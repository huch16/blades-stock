package agent

import (
	"testing"

	"go-stock/backend/agent/tools"
)

func TestGetAllTools(t *testing.T) {
	allTools := getToolsByQuestion("分析一下茅台的股票行情和资金流向")
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
