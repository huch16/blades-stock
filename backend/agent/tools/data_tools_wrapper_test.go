package tools

import (
	"testing"
)

func TestGetAllDataTools(t *testing.T) {
	toolList := GetAllDataTools()
	t.Logf("Total tools count: %d", len(toolList))

	toolNames := make(map[string]int)
	for i, tool := range toolList {
		name := tool.Name()
		desc := tool.Description()
		t.Logf("Tool %d: %s - %s", i+1, name, desc)

		if count, exists := toolNames[name]; exists {
			t.Errorf("Duplicate tool name found: %s (count: %d)", name, count+1)
		}
		toolNames[name]++
	}

	t.Log("\n=== Tool List ===")
	for name, count := range toolNames {
		if count > 1 {
			t.Errorf("Duplicate tool: %s (count: %d)", name, count)
		}
	}
}
