package evaluator

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"blades-stock/backend/logger"
)

// GoldenCase represents a test case in the golden set
type GoldenCase struct {
	ID              string   `json:"id"`
	Question        string   `json:"question"`
	ExpectedTools   []string `json:"expected_tools"`
	ExpectedSummary string   `json:"expected_summary"`
	Tags            []string `json:"tags,omitempty"`
}

// EvaluatorReport represents the evaluation report
type EvaluatorReport struct {
	ID               uint      `json:"id" gorm:"primarykey"`
	CreatedAt        time.Time `json:"createdAt"`
	RunAt            time.Time `json:"runAt" gorm:"index"`
	PeriodStart      time.Time `json:"periodStart" gorm:"index"`
	PeriodEnd        time.Time `json:"periodEnd" gorm:"index"`
	TotalCases       int       `json:"totalCases"`
	MatchedCases     int       `json:"matchedCases"`
	ToolCorrect      int       `json:"toolCorrect"`
	ToolAccuracy     float64   `json:"toolAccuracy"`
	AvgLatencyMs     float64   `json:"avgLatencyMs"`
	RetrieverHitRate float64   `json:"retrieverHitRate"`
	DetailsJSON      string    `json:"detailsJson" gorm:"type:text"`
}

func (EvaluatorReport) TableName() string {
	return "evaluator_reports"
}

// CaseResult represents the result of evaluating a single golden case
type CaseResult struct {
	GoldenID      string   `json:"golden_id"`
	PlanID        string   `json:"plan_id,omitempty"`
	Found         bool     `json:"found"`
	ExpectedTools []string `json:"expected_tools"`
	ActualTools   []string `json:"actual_tools"`
	ToolMatch     bool     `json:"tool_match"`
	LatencyMs     int64    `json:"latency_ms"`
	Error         string   `json:"error,omitempty"`
}

// Evaluator handles the evaluation of agent traces against golden cases
type Evaluator struct {
	goldenCases []GoldenCase
	threshold   float64
}

// NewEvaluator creates a new evaluator instance
func NewEvaluator(goldenPath string, threshold float64) (*Evaluator, error) {
	cases, err := loadGoldenCases(goldenPath)
	if err != nil {
		return nil, fmt.Errorf("load golden cases: %w", err)
	}
	if threshold <= 0 {
		threshold = 0.9
	}
	return &Evaluator{
		goldenCases: cases,
		threshold:   threshold,
	}, nil
}

// loadGoldenCases loads golden cases from a JSONL file
func loadGoldenCases(path string) ([]GoldenCase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cases []GoldenCase
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var gc GoldenCase
		if err := json.Unmarshal([]byte(line), &gc); err != nil {
			logger.SugaredLogger.Warnf("Failed to parse golden case line: %v", err)
			continue
		}
		cases = append(cases, gc)
	}
	return cases, nil
}

// Run evaluates agent traces against golden cases for a given period
// loadTraces loads agent_plan_traces for the given period
// evaluateCase evaluates a single golden case against traces
// calculateRetrieverHitRate calculates the retriever hit rate from traces
// saveCSVReport saves the evaluation results to a CSV file
// AutoRun runs the evaluator for the last 24 hours (for cron/scheduled runs)
// createDefaultGolden creates a default golden set file
