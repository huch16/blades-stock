package quant

import (
	"blades-stock/backend/db"
	"testing"
)

func TestDiagnoseStock(t *testing.T) {
	db.Init("../../data/stock.db")

	res, err := DiagnoseStock("600671")
	if err != nil {
		t.Logf("DiagnoseStock returned error: %v", err)
		return
	}

	if res == nil {
		t.Fatalf("expected non-nil diagnosis result")
	}

	t.Logf("Diagnosis for %s (%s):", res.StockName, res.StockCode)
	t.Logf("  Overall Score: %d/100, Suggestion: %s, Confidence: %.1f%%", res.OverallScore, res.Suggestion, res.Confidence)
	t.Logf("  Current: %.2f, Target: %.2f, StopLoss: %.2f (RR: %.2f)", res.CurrentPrice, res.TargetPrice, res.StopLossPrice, res.RiskRewardRatio)
}

func TestDiagnoseETF(t *testing.T) {
	db.Init("../../data/stock.db")

	res, err := DiagnoseStock("510300")
	if err != nil {
		t.Logf("DiagnoseStock ETF returned error: %v", err)
		return
	}

	if res == nil {
		t.Fatalf("expected non-nil diagnosis result for ETF")
	}

	t.Logf("Diagnosis for %s (%s):", res.StockName, res.StockCode)
	t.Logf("  Overall Score: %d/100, Suggestion: %s, Confidence: %.1f%%", res.OverallScore, res.Suggestion, res.Confidence)
	t.Logf("  Current: %.2f, Target: %.2f, StopLoss: %.2f (RR: %.2f)", res.CurrentPrice, res.TargetPrice, res.StopLossPrice, res.RiskRewardRatio)
}
