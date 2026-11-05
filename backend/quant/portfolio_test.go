package quant

import (
	"math"
	"testing"
)

func TestFromSignals_BasicLong(t *testing.T) {
	closePrices := []float64{100, 102, 101, 105, 107, 106, 110, 108, 112, 115}
	entries := []bool{false, true, false, false, false, false, false, false, false, false}
	exits := []bool{false, false, false, false, false, true, false, false, false, false}
	cfg := &PortfolioConfig{
		InitCash: 100000,
		Fees:     0,
		Slippage: 0,
		Size:     1.0,
	}

	pf := FromSignals(closePrices, entries, exits, cfg)
	if pf == nil {
		t.Fatal("portfolio should not be nil")
	}
	if len(pf.trades) == 0 {
		t.Fatal("expected at least 1 trade")
	}
	// Verify trade entry
	trade := pf.trades[0]
	if trade.EntryIndex != 1 {
		t.Fatalf("expected entry at index 1, got %d", trade.EntryIndex)
	}
	if trade.ExitIndex != 5 {
		t.Fatalf("expected exit at index 5, got %d", trade.ExitIndex)
	}
	if trade.PnL <= 0 {
		t.Fatalf("expected positive PnL, got %f", trade.PnL)
	}
}

func TestFromSignals_EmptyData(t *testing.T) {
	pf := FromSignals(nil, nil, nil, DefaultPortfolioConfig())
	if pf == nil {
		t.Fatal("portfolio should not be nil")
	}
	stats := pf.GetStats()
	if stats.TotalTrades != 0 {
		t.Fatal("expected 0 trades")
	}
}

func TestFromSignals_NilConfig(t *testing.T) {
	closePrices := []float64{100, 101, 102}
	entries := []bool{false, true, false}
	exits := []bool{false, false, true}
	pf := FromSignals(closePrices, entries, exits, nil)
	if pf == nil {
		t.Fatal("portfolio should not be nil with nil config")
	}
}

func TestFromSignals_BuyHold(t *testing.T) {
	closePrices := []float64{100, 102, 104, 103, 106, 108}
	entries := []bool{false, true, false, false, false, false}
	exits := []bool{false, false, false, false, false, true}
	cfg := &PortfolioConfig{
		InitCash: 100000,
		Fees:     0,
		Slippage: 0,
		Size:     1.0,
	}
	pf := FromSignals(closePrices, entries, exits, cfg)
	stats := pf.GetStats()
	if stats.TotalReturn <= 0 {
		t.Fatalf("expected positive return, got %f", stats.TotalReturn)
	}
}

func TestFromSignals_StopLoss(t *testing.T) {
	closePrices := []float64{100, 98, 96, 94, 92, 90}
	entries := []bool{false, true, false, false, false, false}
	exits := []bool{false, false, false, false, false, true}
	cfg := &PortfolioConfig{
		InitCash: 100000,
		Fees:     0,
		Slippage: 0,
		StopLoss: 0.05,
		Size:     1.0,
	}
	pf := FromSignals(closePrices, entries, exits, cfg)
	_ = pf
}

func TestFromSignals_TakeProfit(t *testing.T) {
	closePrices := []float64{100, 103, 106, 109, 112, 115}
	entries := []bool{false, true, false, false, false, false}
	exits := []bool{false, false, false, false, false, true}
	cfg := &PortfolioConfig{
		InitCash:   100000,
		Fees:       0,
		Slippage:   0,
		TakeProfit: 0.10,
		Size:       1.0,
	}
	pf := FromSignals(closePrices, entries, exits, cfg)
	_ = pf
}

func TestFromSignals_Fees(t *testing.T) {
	closePrices := []float64{100, 110, 120}
	entries := []bool{false, true, false}
	exits := []bool{false, false, true}
	cfg := &PortfolioConfig{
		InitCash: 100000,
		Fees:     30.0,
		Slippage: 0,
		Size:     1.0,
	}
	pf := FromSignals(closePrices, entries, exits, cfg)
	stats := pf.GetStats()
	if stats.TotalTrades != 1 {
		t.Fatalf("expected 1 trade, got %d", stats.TotalTrades)
	}
	cfgNoFee := &PortfolioConfig{
		InitCash: 100000,
		Fees:     0,
		Slippage: 0,
		Size:     1.0,
	}
	pfNoFee := FromSignals(closePrices, entries, exits, cfgNoFee)
	if stats.TotalReturn >= pfNoFee.GetStats().TotalReturn {
		t.Fatal("expected return with fees < return without fees")
	}
}

func TestFromSignals_MultipleTrades(t *testing.T) {
	closePrices := []float64{100, 105, 100, 110, 105, 115, 110}
	entries := []bool{false, true, false, false, true, false, false}
	exits := []bool{false, false, true, false, false, false, true}
	cfg := &PortfolioConfig{
		InitCash: 100000,
		Fees:     0,
		Slippage: 0,
		Size:     1.0,
	}
	pf := FromSignals(closePrices, entries, exits, cfg)
	if len(pf.trades) != 2 {
		t.Fatalf("expected 2 trades, got %d", len(pf.trades))
	}
}

func TestGetStats(t *testing.T) {
	closePrices := []float64{100, 102, 104, 103, 106, 108, 107, 110}
	entries := []bool{false, true, false, false, false, false, false, false}
	exits := []bool{false, false, false, false, false, true, false, false}
	cfg := &PortfolioConfig{
		InitCash: 100000,
		Fees:     0,
		Slippage: 0,
		Size:     1.0,
	}
	pf := FromSignals(closePrices, entries, exits, cfg)
	stats := pf.GetStats()
	if stats.InitCash != 100000 {
		t.Fatalf("InitCash mismatch: %f", stats.InitCash)
	}
	if stats.FinalValue <= 0 {
		t.Fatalf("FinalValue should be positive: %f", stats.FinalValue)
	}
	if stats.TotalTrades != 1 {
		t.Fatalf("expected 1 trade, got %d", stats.TotalTrades)
	}
	if stats.TotalReturn <= 0 {
		t.Fatalf("expected positive return, got %f", stats.TotalReturn)
	}
	if stats.WinRate != 1.0 {
		t.Fatalf("expected 100%% win rate, got %f", stats.WinRate)
	}
	if stats.MaxDrawdownPct < 0 {
		t.Fatalf("MaxDrawdownPct should be >= 0, got %f", stats.MaxDrawdownPct)
	}
}

func TestGetStats_Shrinking(t *testing.T) {
	closePrices := []float64{100, 95, 90, 85, 80}
	entries := []bool{false, true, false, false, false}
	exits := []bool{false, false, false, false, true}
	cfg := &PortfolioConfig{
		InitCash: 100000,
		Fees:     0,
		Slippage: 0,
		Size:     1.0,
	}
	pf := FromSignals(closePrices, entries, exits, cfg)
	stats := pf.GetStats()
	if stats.TotalReturn >= 0 {
		t.Fatalf("expected negative return for declining prices, got %f", stats.TotalReturn)
	}
	if stats.WinRate != 0 {
		t.Fatalf("expected 0%% win rate, got %f", stats.WinRate)
	}
}

func TestGetStats_Sharpe(t *testing.T) {
	closePrices := []float64{100, 101, 102, 101, 103, 104, 103, 105}
	entries := []bool{false, true, false, false, false, false, false, false}
	exits := []bool{false, false, false, false, false, false, false, true}
	cfg := &PortfolioConfig{
		InitCash: 100000,
		Fees:     0,
		Slippage: 0,
		Size:     1.0,
	}
	pf := FromSignals(closePrices, entries, exits, cfg)
	stats := pf.GetStats()
	if stats.SharpeRatio == 0 {
		t.Fatal("SharpeRatio should not be 0")
	}
	if stats.Volatility <= 0 {
		t.Fatal("Volatility should be > 0")
	}
}

func TestGetStats_NoTrades(t *testing.T) {
	closePrices := []float64{100, 101, 102}
	entries := []bool{false, false, false}
	exits := []bool{false, false, false}
	cfg := &PortfolioConfig{InitCash: 100000}
	pf := FromSignals(closePrices, entries, exits, cfg)
	stats := pf.GetStats()
	if stats.TotalTrades != 0 {
		t.Fatal("expected 0 trades")
	}
	if stats.TotalReturn != 0 {
		t.Fatal("expected 0 return")
	}
	if stats.InitCash != stats.FinalValue {
		t.Fatal("final value should equal initial with no trades")
	}
}

func TestFromHolding(t *testing.T) {
	closePrices := []float64{100, 102, 104, 103, 106}
	pf := FromHolding(closePrices, 100000)
	if pf == nil {
		t.Fatal("FromHolding should not be nil")
	}
	stats := pf.GetHoldingStats()
	if stats.TotalReturn <= 0 {
		t.Fatalf("expected positive return, got %f", stats.TotalReturn)
	}
}

func TestFromHolding_Empty(t *testing.T) {
	pf := FromHolding(nil, 100000)
	if pf != nil {
		t.Fatal("FromHolding of empty should be nil")
	}
}

func TestDefaultPortfolioConfig(t *testing.T) {
	cfg := DefaultPortfolioConfig()
	if cfg.InitCash != 100000 {
		t.Fatalf("InitCash should be 100000, got %f", cfg.InitCash)
	}
	if cfg.Fees != 1.5 {
		t.Fatalf("Fees should be 1.5, got %f", cfg.Fees)
	}
	if cfg.Size != 1.0 {
		t.Fatalf("Size should be 1.0, got %f", cfg.Size)
	}
}

func TestPortfolioReturns(t *testing.T) {
	closePrices := []float64{100, 110, 120}
	entries := []bool{false, true, false}
	exits := []bool{false, false, true}
	cfg := &PortfolioConfig{InitCash: 100000, Fees: 0, Slippage: 0, Size: 1.0}
	pf := FromSignals(closePrices, entries, exits, cfg)
	if len(pf.returns) != 3 {
		t.Fatalf("expected 3 returns, got %d", len(pf.returns))
	}
	initCash := pf.config.InitCash
	cumReturn := (pf.value[2] - initCash) / initCash
	expected := (120.0 - 110.0) / 110.0 // entry at 110, exit at 120
	if math.Abs(cumReturn-expected) > 0.01 {
		t.Fatalf("cumulative return should be ~%f, got %f", expected, cumReturn)
	}
}
