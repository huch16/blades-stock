package agent

import (
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/quant"
	"context"
	"os"
	"testing"
)

func TestBacktest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	if _, err := os.Stat("data/stock.db"); os.IsNotExist(err) {
		t.Skip("Skipping: data/stock.db not found (run from project root)")
	}
	// Initialize database
	db.Init("data/stock.db")

	// Initialize logger
	logger.InitLogger()

	// Create BacktestManager
	mgr := quant.NewBacktestManager()

	// Check if there are reports needing backtest
	needBacktest := mgr.NeedBacktest()
	t.Logf("Reports pending backtest: %v", needBacktest)

	// Run backtest
	t.Log("Running backtest...")
	result, err := mgr.RunAIReportBacktest(context.Background(), 50)
	if err != nil {
		t.Errorf("Error running backtest: %v", err)
		return
	}

	t.Logf("Backtest completed: total=%d, processed=%d, skipped=%d, accurate=%d (%.1f%%)",
		result.Total, result.Processed, result.Skipped, result.Accurate, result.AccuracyRate*100)

	// Show recent results
	results := mgr.GetLatestResults(10)
	t.Logf("Recent %d backtest results:", len(results))
	for _, r := range results {
		t.Logf("  ID=%d, Stock=%s(%s), Direction=%s, Accurate=%v, Change=%.2f%%",
			r.ReportID, r.StockCode, r.StockName, r.Direction, r.IsAccurate, r.PriceChange)
	}

	// Show overall stats
	stats := mgr.GetOverallStats()
	t.Logf("=== Overall Backtest Statistics ===")
	t.Logf("Total reports: %d", stats.TotalReports)
	t.Logf("Verified reports: %d", stats.VerifiedCount)
	t.Logf("Accurate predictions: %d", stats.AccurateCount)
	t.Logf("Accuracy rate: %.2f%%", stats.AccuracyRate)
	t.Logf("Average confidence: %.2f", stats.AvgConfidence)

	// Show direction stats
	if len(stats.DirectionStats) > 0 {
		t.Log("Direction breakdown:")
		for dir, ds := range stats.DirectionStats {
			t.Logf("  %s: %d/%d (%.2f%%)", dir, ds.Accurate, ds.Count, ds.Rate)
		}
	}
}
