package evaluator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewEvaluator(t *testing.T) {
	// Create a temporary golden file
	tmpDir := t.TempDir()
	goldenPath := filepath.Join(tmpDir, "golden.jsonl")
	
	// Write a simple golden case (JSONL format: one JSON object per line)
	goldenContent := `{"id":"test1","question":"test question","expected_tools":["TestTool"],"expected_summary":"test summary"}
`
	if err := os.WriteFile(goldenPath, []byte(goldenContent), 0644); err != nil {
		t.Fatalf("Failed to create golden file: %v", err)
	}

	// Create evaluator
	ev, err := NewEvaluator(goldenPath, 0.8)
	if err != nil {
		t.Fatalf("NewEvaluator failed: %v", err)
	}
	if ev == nil {
		t.Fatal("NewEvaluator returned nil evaluator")
	}
	// Use the evaluator to make sure it's valid
	_ = ev
}

func TestLoadGoldenCases(t *testing.T) {
	// Create a temporary golden file
	tmpDir := t.TempDir()
	goldenPath := filepath.Join(tmpDir, "golden.jsonl")
	
	// Write golden cases (JSONL format: one JSON object per line)
	goldenContent := `{"id":"test1","question":"test question 1","expected_tools":["Tool1"],"expected_summary":"summary 1"}
{"id":"test2","question":"test question 2","expected_tools":["Tool2"],"expected_summary":"summary 2"}
`
	if err := os.WriteFile(goldenPath, []byte(goldenContent), 0644); err != nil {
		t.Fatalf("Failed to create golden file: %v", err)
	}

	// Load golden cases
	cases, err := loadGoldenCases(goldenPath)
	if err != nil {
		t.Fatalf("loadGoldenCases failed: %v", err)
	}
	if len(cases) != 2 {
		t.Errorf("Expected 2 cases, got %d", len(cases))
	}
	if cases[0].ID != "test1" {
		t.Errorf("Expected first case ID 'test1', got '%s'", cases[0].ID)
	}
	if cases[1].ID != "test2" {
		t.Errorf("Expected second case ID 'test2', got '%s'", cases[1].ID)
	}
	if len(cases[0].ExpectedTools) != 1 || cases[0].ExpectedTools[0] != "Tool1" {
		t.Errorf("Expected first case to have Tool1, got %v", cases[0].ExpectedTools)
	}
	if len(cases[1].ExpectedTools) != 1 || cases[1].ExpectedTools[0] != "Tool2" {
		t.Errorf("Expected second case to have Tool2, got %v", cases[1].ExpectedTools)
	}
}