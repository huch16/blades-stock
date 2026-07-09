package main

import (
	"context"
	"fmt"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/quant"
	"os"
)

func main() {
	// Initialize database
	db.Init("data/stock.db")

	// Initialize logger
	logger.InitLogger()

	fmt.Println("=== Backtest Test Tool ===")
	fmt.Println()

	// Create BacktestManager
	mgr := quant.NewBacktestManager()

	// Check if there are reports needing backtest
	needBacktest := mgr.NeedBacktest()
	fmt.Printf("Reports pending backtest: %v\n", needBacktest)

	if !needBacktest {
		fmt.Println("No reports need backtest. Checking existing results...")
		// Show existing backtest results
		results := mgr.GetLatestResults(10)
		if len(results) == 0 {
			fmt.Println("No backtest results found.")
		} else {
			fmt.Printf("\nRecent %d backtest results:\n", len(results))
			for _, r := range results {
				fmt.Printf("  ID=%d, Stock=%s(%s), Direction=%s, Accurate=%v, Change=%.2f%%\n",
					r.ReportID, r.StockCode, r.StockName, r.Direction, r.IsAccurate, r.PriceChange)
			}
		}

		// Show overall stats
		stats := mgr.GetOverallStats()
		fmt.Printf("\n=== Overall Backtest Statistics ===\n")
		fmt.Printf("Total reports: %d\n", stats.TotalReports)
		fmt.Printf("Verified reports: %d\n", stats.VerifiedCount)
		fmt.Printf("Accurate predictions: %d\n", stats.AccurateCount)
		fmt.Printf("Accuracy rate: %.2f%%\n", stats.AccuracyRate)
		fmt.Printf("Average confidence: %.2f\n", stats.AvgConfidence)

		os.Exit(0)
	}

	// Run backtest
	fmt.Println("\nRunning backtest...")
	result, err := mgr.RunAIReportBacktest(context.Background(), 50)
	if err != nil {
		fmt.Printf("Error running backtest: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Backtest completed: total=%d, processed=%d, skipped=%d, accurate=%d (%.1f%%)\n",
		result.Total, result.Processed, result.Skipped, result.Accurate, result.AccuracyRate*100)

	// Show results
	results := mgr.GetLatestResults(10)
	fmt.Printf("\nRecent %d backtest results:\n", len(results))
	for _, r := range results {
		fmt.Printf("  ID=%d, Stock=%s(%s), Direction=%s, Accurate=%v, Change=%.2f%%\n",
			r.ReportID, r.StockCode, r.StockName, r.Direction, r.IsAccurate, r.PriceChange)
	}

	// Show overall stats
	stats := mgr.GetOverallStats()
	fmt.Printf("\n=== Overall Backtest Statistics ===\n")
	fmt.Printf("Total reports: %d\n", stats.TotalReports)
	fmt.Printf("Verified reports: %d\n", stats.VerifiedCount)
	fmt.Printf("Accurate predictions: %d\n", stats.AccurateCount)
	fmt.Printf("Accuracy rate: %.2f%%\n", stats.AccuracyRate)
	fmt.Printf("Average confidence: %.2f\n", stats.AvgConfidence)

	// Show direction stats
	if len(stats.DirectionStats) > 0 {
		fmt.Printf("\nDirection breakdown:\n")
		for dir, ds := range stats.DirectionStats {
			fmt.Printf("  %s: %d/%d (%.2f%%)\n", dir, ds.Accurate, ds.Count, ds.Rate)
		}
	}

	fmt.Println("\n=== Backtest Test Completed ===")
}
