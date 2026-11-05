package evaluator

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"blades-stock/backend/agent"
	"blades-stock/backend/db"
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
func (e *Evaluator) Run(ctx context.Context, periodStart, periodEnd time.Time) (*EvaluatorReport, []CaseResult, error) {
	// Load traces for the period
	traces, err := e.loadTraces(periodStart, periodEnd)
	if err != nil {
		return nil, nil, fmt.Errorf("load traces: %w", err)
	}

	// Build plan map: plan_id -> list of traces
	planTraces := make(map[string][]agent.PlanTrace)
	for _, t := range traces {
		planTraces[t.PlanID] = append(planTraces[t.PlanID], t)
	}

	// Evaluate each golden case
	results := make([]CaseResult, 0, len(e.goldenCases))
	var totalLatency int64
	var matchedCases, toolCorrect int

	for _, gc := range e.goldenCases {
		result := e.evaluateCase(gc, planTraces)
		results = append(results, result)

		if result.Found {
			matchedCases++
			totalLatency += result.LatencyMs
			if result.ToolMatch {
				toolCorrect++
			}
		}
	}

	var avgLatency float64
	if matchedCases > 0 {
		avgLatency = float64(totalLatency) / float64(matchedCases)
	}

	toolAccuracy := 0.0
	if len(e.goldenCases) > 0 {
		toolAccuracy = float64(toolCorrect) / float64(len(e.goldenCases))
	}

	// Calculate retriever hit rate (placeholder)
	retrieverHitRate := e.calculateRetrieverHitRate(traces)

	// Build report
	report := &EvaluatorReport{
		RunAt:            time.Now(),
		PeriodStart:      periodStart,
		PeriodEnd:        periodEnd,
		TotalCases:       len(e.goldenCases),
		MatchedCases:     matchedCases,
		ToolCorrect:      toolCorrect,
		ToolAccuracy:     toolAccuracy,
		AvgLatencyMs:     avgLatency,
		RetrieverHitRate: retrieverHitRate,
	}

	detailsJSON, _ := json.Marshal(results)
	report.DetailsJSON = string(detailsJSON)

	// Save report to DB
	if err := db.Dao.Create(report).Error; err != nil {
		logger.SugaredLogger.Errorf("Failed to save evaluator report: %v", err)
	} else {
		logger.SugaredLogger.Infof("Evaluator report saved with ID: %d", report.ID)
	}

	// Save CSV report
	if err := e.saveCSVReport(report, results); err != nil {
		logger.SugaredLogger.Errorf("Failed to save CSV report: %v", err)
	}

	// Check threshold
	if toolAccuracy < e.threshold && len(e.goldenCases) > 0 {
		logger.SugaredLogger.Warnf("ALERT: Tool accuracy %.2f%% below threshold %.2f%%", toolAccuracy*100, e.threshold*100)
	}

	return report, results, nil
}

// loadTraces loads agent_plan_traces for the given period
func (e *Evaluator) loadTraces(start, end time.Time) ([]agent.PlanTrace, error) {
	var traces []agent.PlanTrace
	err := db.Dao.Where("created_at BETWEEN ? AND ?", start, end).Find(&traces).Error
	return traces, err
}

// evaluateCase evaluates a single golden case against traces
func (e *Evaluator) evaluateCase(gc GoldenCase, planTraces map[string][]agent.PlanTrace) CaseResult {
	// For now, we match by checking if any plan contains the expected tools
	// In a real implementation, you'd match by question similarity or plan_id reference

	var actualTools []string
	var found bool
	var latencyMs int64

	for _, traces := range planTraces {
		for _, t := range traces {
			actualTools = append(actualTools, t.ToolName)
			found = true
			latencyMs += t.DurationMs
		}
	}

	// Check if all expected tools were called
	toolMatch := true
	for _, expected := range gc.ExpectedTools {
		matched := false
		for _, actual := range actualTools {
			if strings.EqualFold(expected, actual) {
				matched = true
				break
			}
		}
		if !matched {
			toolMatch = false
			break
		}
	}

	return CaseResult{
		GoldenID:      gc.ID,
		Found:         found,
		ExpectedTools: gc.ExpectedTools,
		ActualTools:   actualTools,
		ToolMatch:     toolMatch,
		LatencyMs:     latencyMs,
	}
}

// calculateRetrieverHitRate calculates the retriever hit rate from traces
func (e *Evaluator) calculateRetrieverHitRate(traces []agent.PlanTrace) float64 {
	// Placeholder - in a real implementation, you'd check retriever metrics
	return 0.0
}

// saveCSVReport saves the evaluation results to a CSV file
func (e *Evaluator) saveCSVReport(report *EvaluatorReport, results []CaseResult) error {
	reportDir := "reports"
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		return err
	}

	filename := filepath.Join(reportDir, fmt.Sprintf("evaluator_%s.csv", report.RunAt.Format("20060102")))
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{"golden_id", "plan_id", "found", "expected_tools", "actual_tools", "tool_match", "latency_ms", "error"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write results
	for _, r := range results {
		record := []string{
			r.GoldenID,
			r.PlanID,
			fmt.Sprintf("%v", r.Found),
			strings.Join(r.ExpectedTools, ";"),
			strings.Join(r.ActualTools, ";"),
			fmt.Sprintf("%v", r.ToolMatch),
			fmt.Sprintf("%d", r.LatencyMs),
			r.Error,
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	logger.SugaredLogger.Infof("CSV report saved to: %s", filename)
	return nil
}

// AutoRun runs the evaluator for the last 24 hours (for cron/scheduled runs)
func AutoRun(ctx context.Context, threshold float64) error {
	goldenPath := "tests/golden/golden.jsonl"
	if _, err := os.Stat(goldenPath); os.IsNotExist(err) {
		// Create default golden file if not exists
		if err := createDefaultGolden(goldenPath); err != nil {
			return err
		}
	}

	e, err := NewEvaluator(goldenPath, threshold)
	if err != nil {
		return err
	}

	end := time.Now()
	start := end.Add(-24 * time.Hour)

	_, _, err = e.Run(ctx, start, end)
	return err
}

// createDefaultGolden creates a default golden set file
func createDefaultGolden(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	defaultCases := []GoldenCase{
		{
			ID:              "g1",
			Question:        "给我分析腾讯的近期财务表现",
			ExpectedTools:   []string{"QueryStockCodeInfo", "WebSearch"},
			ExpectedSummary: "包含营收、净利、估值结论",
			Tags:            []string{"stock_analysis", "financial"},
		},
		{
			ID:              "g2",
			Question:        "今天ETF开盘前有什么建议？",
			ExpectedTools:   []string{"SearchStockByIndicators", "WebSearch"},
			ExpectedSummary: "包含动量排名、因子分析、最终决策",
			Tags:            []string{"etf", "quant", "daily"},
		},
		{
			ID:              "g3",
			Question:        "分析华阳集团在汽车电子产业链中的位置",
			ExpectedTools:   []string{"QueryStockCodeInfo", "WebSearch", "FetchWebPage"},
			ExpectedSummary: "产业链全景图、稀缺层分析、证伪条件",
			Tags:            []string{"industry", "serenity", "auto"},
		},
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, gc := range defaultCases {
		if err := encoder.Encode(gc); err != nil {
			return err
		}
	}

	logger.SugaredLogger.Infof("Created default golden set at: %s", path)
	return nil
}
