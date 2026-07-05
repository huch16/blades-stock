package agent

import (
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"testing"
)

func TestBacktest(t *testing.T) {
	// Initialize database
	db.Init("data/stock.db")

	// Initialize logger
	logger.InitLogger()

	// Create BacktestService
	svc := NewBacktestService()

	// Check if there are reports needing backtest
	needBacktest := svc.NeedBacktest()
	t.Logf("Reports pending backtest: %v", needBacktest)

	// Run backtest
	t.Log("Running backtest...")
	count, err := svc.RunAllBacktests()
	if err != nil {
		t.Errorf("Error running backtest: %v", err)
		return
	}

	t.Logf("Backtest completed: %d reports processed", count)

	// Show recent results
	results := svc.GetLatestBacktestResults(10)
	t.Logf("Recent %d backtest results:", len(results))
	for _, r := range results {
		t.Logf("  ID=%d, Stock=%s(%s), Direction=%s, Accurate=%v, Change=%.2f%%",
			r.ReportID, r.StockCode, r.StockName, r.Direction, r.IsAccurate, r.PriceChange)
	}

	// Show overall stats
	stats := svc.GetOverallStats()
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
