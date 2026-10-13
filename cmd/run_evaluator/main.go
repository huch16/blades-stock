package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"blades-stock/backend/agent/evaluator"
	"blades-stock/backend/db"
)

func main() {
	var (
		fromStr     = flag.String("from", "", "Start date (YYYY-MM-DD), default: 24h ago")
		toStr       = flag.String("to", "", "End date (YYYY-MM-DD), default: now")
		goldenPath  = flag.String("golden", "tests/golden/golden.jsonl", "Path to golden set JSONL file")
		threshold   = flag.Float64("threshold", 0.9, "Tool accuracy threshold (0.0-1.0)")
		autoRun     = flag.Bool("auto", false, "Run auto mode (last 24h)")
	)
	flag.Parse()

	ctx := context.Background()

	// Initialize DB
	db.Init("")
	
	if *autoRun {
		// Auto mode: run for last 24 hours
		if err := evaluator.AutoRun(ctx, *threshold); err != nil {
			fmt.Fprintf(os.Stderr, "Auto run failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Auto evaluation completed")
		return
	}

	// Parse time range
	var start, end time.Time
	var err error
	
	if *fromStr == "" {
		start = time.Now().Add(-24 * time.Hour)
	} else {
		start, err = time.Parse("2006-01-02", *fromStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid from date: %v\n", err)
			os.Exit(1)
		}
	}
	
	if *toStr == "" {
		end = time.Now()
	} else {
		end, err = time.Parse("2006-01-02", *toStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid to date: %v\n", err)
			os.Exit(1)
		}
	}

	// Run evaluator
	e, err := evaluator.NewEvaluator(*goldenPath, *threshold)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create evaluator: %v\n", err)
		os.Exit(1)
	}

	report, results, err := e.Run(ctx, start, end)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Evaluation failed: %v\n", err)
		os.Exit(1)
	}

	// Print summary
	fmt.Printf("\n=== Evaluation Report ===\n")
	fmt.Printf("Run Time: %s\n", report.RunAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Period: %s to %s\n", start.Format("2006-01-02"), end.Format("2006-01-02"))
	fmt.Printf("Total Cases: %d\n", report.TotalCases)
	fmt.Printf("Matched Cases: %d\n", report.MatchedCases)
	fmt.Printf("Tool Correct: %d\n", report.ToolCorrect)
	fmt.Printf("Tool Accuracy: %.2f%%\n", report.ToolAccuracy*100)
	fmt.Printf("Avg Latency: %.2f ms\n", report.AvgLatencyMs)
	fmt.Printf("Retriever Hit Rate: %.2f%%\n", report.RetrieverHitRate*100)
	fmt.Printf("Threshold: %.2f%%\n", *threshold*100)
	
	if report.ToolAccuracy < *threshold {
		fmt.Printf("⚠️  ALERT: Accuracy below threshold!\n")
	}
	
	fmt.Printf("\n=== Case Results ===\n")
	for _, r := range results {
		status := "✓"
		if !r.ToolMatch {
			status = "✗"
		}
		fmt.Printf("%s [%s] Expected: %v, Actual: %v, Latency: %dms\n", 
			status, r.GoldenID, r.ExpectedTools, r.ActualTools, r.LatencyMs)
	}
}