package quant

import (
	"strconv"
	"strings"
	"testing"
)

func TestDefaultBacktestConfig(t *testing.T) {
	cfg := DefaultBacktestConfig()
	if cfg.InitCash != 100000 {
		t.Fatalf("InitCash should be 100000, got %f", cfg.InitCash)
	}
	if cfg.Fees != 1.5 {
		t.Fatalf("Fees should be 1.5, got %f", cfg.Fees)
	}
	if cfg.Name != "default" {
		t.Fatalf("Name should be default, got %s", cfg.Name)
	}
}

func TestBacktestConfigToPortfolioConfig(t *testing.T) {
	bc := DefaultBacktestConfig()
	bc.InitCash = 50000
	bc.Fees = 2.0
	pc := bc.ToPortfolioConfig()
	if pc.InitCash != 50000 {
		t.Fatalf("InitCash mismatch: %f", pc.InitCash)
	}
	if pc.Fees != 2.0 {
		t.Fatalf("Fees mismatch: %f", pc.Fees)
	}
}

func TestNewBacktestEngineBuilder(t *testing.T) {
	b := NewBacktestEngine()
	if b == nil {
		t.Fatal("builder should not be nil")
	}
}

func TestBuilderData(t *testing.T) {
	close := []float64{100, 101, 102}
	high := []float64{102, 103, 104}
	low := []float64{99, 100, 101}
	open := []float64{100, 101, 102}
	volume := []float64{1000, 1500, 1200}
	dates := []string{"2024-01-01", "2024-01-02", "2024-01-03"}

	e := NewBacktestEngine().Data(close, high, low, open, volume, dates).Build()
	if e.Close.Len != 3 {
		t.Fatalf("expected length 3, got %d", e.Close.Len)
	}
	if e.Close.Values[0] != 100 {
		t.Fatalf("Close[0] should be 100, got %f", e.Close.Values[0])
	}
	if len(e.Dates) != 3 {
		t.Fatalf("expected 3 dates, got %d", len(e.Dates))
	}
}

func TestBuilderCloseOnly(t *testing.T) {
	close := []float64{100, 101, 102}
	dates := []string{"2024-01-01", "2024-01-02", "2024-01-03"}

	e := NewBacktestEngine().CloseOnly(close, dates).Build()
	if e.Close.Len != 3 {
		t.Fatalf("expected length 3, got %d", e.Close.Len)
	}
	if e.High != nil {
		t.Fatal("High should be nil with CloseOnly")
	}
}

func TestBuilderConfig(t *testing.T) {
	cfg := DefaultBacktestConfig()
	cfg.InitCash = 200000
	e := NewBacktestEngine().CloseOnly([]float64{100}, []string{"2024-01-01"}).Config(cfg).Build()
	if e.Config.InitCash != 200000 {
		t.Fatalf("Config override failed: %f", e.Config.InitCash)
	}
}

func TestBuilderChain(t *testing.T) {
	e := NewBacktestEngine().
		InitCash(50000).
		Fees(2.0).
		StopLoss(0.1).
		TakeProfit(0.2).
		CloseOnly([]float64{100}, []string{"2024-01-01"}).
		Build()
	if e.Config.InitCash != 50000 {
		t.Fatal("InitCash chain failed")
	}
	if e.Config.Fees != 2.0 {
		t.Fatal("Fees chain failed")
	}
	if e.Config.StopLoss != 0.1 {
		t.Fatal("StopLoss chain failed")
	}
	if e.Config.TakeProfit != 0.2 {
		t.Fatal("TakeProfit chain failed")
	}
}

func TestEngineRun_NoData(t *testing.T) {
	e := NewBacktestEngine().Build()
	strategy := &MAStrategy{FastWindow: 5, SlowWindow: 20}
	result := e.Run(strategy)
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.Stats != nil {
		t.Fatal("Stats should be nil with no data")
	}
}

func TestEngineRun_MAStrategy(t *testing.T) {
	close := generateTrendData(100, 0.5)
	dates := generateDates(len(close))

	e := NewBacktestEngine().
		CloseOnly(close, dates).
		Config(DefaultBacktestConfig()).
		Build()

	strategy := &MAStrategy{FastWindow: 5, SlowWindow: 20}
	result := e.Run(strategy)
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.StrategyName != "MA" {
		t.Fatalf("expected MA, got %s", result.StrategyName)
	}
	if result.Stats == nil {
		t.Fatal("stats should not be nil")
	}
}

func TestEngineRun_MACDStrategy(t *testing.T) {
	close := generateTrendData(100, 0.3)
	dates := generateDates(len(close))

	e := NewBacktestEngine().
		CloseOnly(close, dates).
		Build()

	strategy := &MACDStrategy{Fast: 5, Slow: 13, Signal: 5}
	result := e.Run(strategy)
	if result == nil || result.Stats == nil {
		t.Fatal("MACD strategy should produce stats")
	}
}

func TestEngineRun_RSIStrategy(t *testing.T) {
	close := generateTrendData(100, 0.2)
	dates := generateDates(len(close))

	e := NewBacktestEngine().
		CloseOnly(close, dates).
		Build()

	strategy := &RSIStrategy{Window: 14, Overbought: 70, Oversold: 30}
	result := e.Run(strategy)
	if result == nil || result.Stats == nil {
		t.Fatal("RSI strategy should produce stats")
	}
}

func TestEngineRun_BollingerStrategy(t *testing.T) {
	close := generateTrendData(100, 0.3)
	dates := generateDates(len(close))

	e := NewBacktestEngine().
		CloseOnly(close, dates).
		Build()

	strategy := &BollingerStrategy{Window: 20, NumStd: 2}
	result := e.Run(strategy)
	if result == nil || result.Stats == nil {
		t.Fatal("Bollinger strategy should produce stats")
	}
}

func TestEngineRun_DualThrustStrategy(t *testing.T) {
	n := 120
	close := make([]float64, n)
	high := make([]float64, n)
	low := make([]float64, n)
	open := make([]float64, n)
	volume := make([]float64, n)
	dates := make([]string, n)
	for i := 0; i < n; i++ {
		close[i] = 100 + float64(i)*0.3
		high[i] = close[i] + 2
		low[i] = close[i] - 2
		open[i] = close[i]
		volume[i] = 1000
		dates[i] = "2024-01-" + pad2(i+1)
	}

	e := NewBacktestEngine().
		Data(close, high, low, open, volume, dates).
		Build()

	strategy := &DualThrustStrategy{K: 0.5, Period: 10}
	result := e.Run(strategy)
	if result == nil || result.Stats == nil {
		t.Fatal("DualThrust strategy should produce stats")
	}
}

func TestEngineRun_GridSearch(t *testing.T) {
	close := generateTrendData(100, 0.3)
	dates := generateDates(len(close))

	e := NewBacktestEngine().
		CloseOnly(close, dates).
		Build()

	strategies := []Strategy{
		&MAStrategy{FastWindow: 5, SlowWindow: 20},
		&MAStrategy{FastWindow: 10, SlowWindow: 30},
		&MAStrategy{FastWindow: 20, SlowWindow: 60},
	}

	result := e.RunGridSearch(strategies)
	if result == nil {
		t.Fatal("grid search result should not be nil")
	}
	if result.TotalCombinations != 3 {
		t.Fatalf("expected 3 combinations, got %d", result.TotalCombinations)
	}
	if result.BestStats == nil {
		t.Fatal("best stats should not be nil")
	}
}

func TestEngineRun_GridSearchEmpty(t *testing.T) {
	e := NewBacktestEngine().CloseOnly([]float64{100}, []string{"2024-01-01"}).Build()
	result := e.RunGridSearch(nil)
	if result == nil {
		t.Fatal("empty grid search should not be nil")
	}
}

func TestGenerateMAStrategies(t *testing.T) {
	strategies := GenerateMAStrategies([]int{5, 10}, []int{20, 30, 60})
	if len(strategies) != 6 {
		t.Fatalf("expected 6 MA strategies (2x3), got %d", len(strategies))
	}
}

func TestGenerateMACDStrategies(t *testing.T) {
	strategies := GenerateMACDStrategies([]int{5, 8}, []int{18, 24}, []int{5, 9})
	if len(strategies) == 0 {
		t.Fatal("expected MACD strategies")
	}
}

func TestGenerateRSIStrategies(t *testing.T) {
	strategies := GenerateRSIStrategies([]int{7, 14, 21}, 70, 30)
	if len(strategies) != 3 {
		t.Fatalf("expected 3 RSI strategies, got %d", len(strategies))
	}
}

func TestMAStrategyParams(t *testing.T) {
	s := &MAStrategy{FastWindow: 5, SlowWindow: 20}
	params := s.Params()
	if params["fast"].(int) != 5 || params["slow"].(int) != 20 {
		t.Fatal("MA params mismatch")
	}
}

func TestMACDStrategyName(t *testing.T) {
	s := &MACDStrategy{Fast: 12, Slow: 26, Signal: 9}
	if s.Name() != "MACD" {
		t.Fatalf("expected MACD, got %s", s.Name())
	}
}

func TestRSIStrategyName(t *testing.T) {
	s := &RSIStrategy{Window: 14, Overbought: 70, Oversold: 30}
	if s.Name() != "RSI" {
		t.Fatalf("expected RSI, got %s", s.Name())
	}
}

func TestBollingerStrategyName(t *testing.T) {
	s := &BollingerStrategy{Window: 20, NumStd: 2}
	if s.Name() != "BOLL" {
		t.Fatalf("expected BOLL, got %s", s.Name())
	}
}

func TestDualThrustStrategyName(t *testing.T) {
	s := &DualThrustStrategy{K: 0.5, Period: 10}
	if s.Name() != "DualThrust" {
		t.Fatalf("expected DualThrust, got %s", s.Name())
	}
}

func TestPrintResult_Nil(t *testing.T) {
	s := PrintResult(nil)
	if s != "No results" {
		t.Fatalf("expected 'No results', got '%s'", s)
	}
}

func TestPrintResult_NoStats(t *testing.T) {
	s := PrintResult(&BacktestResult{})
	if s != "No results" {
		t.Fatalf("expected 'No results', got '%s'", s)
	}
}

func TestPrintResult(t *testing.T) {
	close := []float64{100, 102, 104, 103, 106}
	entries := []bool{false, true, false, false, false}
	exits := []bool{false, false, false, true, false}
	pf := FromSignals(close, entries, exits, &PortfolioConfig{InitCash: 100000, Fees: 0, Slippage: 0, Size: 1.0})
	result := &BacktestResult{
		StrategyName: "MA",
		Params:       map[string]any{"fast": 5, "slow": 20},
		Stats:        pf.GetStats(),
		Trades:       []TradeRef{},
	}

	output := PrintResult(result)
	if !strings.Contains(output, "MA") {
		t.Fatal("output should contain strategy name")
	}
	if !strings.Contains(output, "总收益率") {
		t.Fatal("output should contain TotalReturn label")
	}
	if !strings.Contains(output, "夏普比率") {
		t.Fatal("output should contain SharpeRatio label")
	}
}

// --- sprintf / formatter tests ---

func TestSprintfFloat(t *testing.T) {
	tests := []struct {
		input    float64
		decimals int
		expected string
	}{
		{3.14159, 2, "3.14"},
		{3.14159, 4, "3.1416"},
		{0.0, 2, "0.00"},
		{-3.14159, 2, "-3.14"},
		{100.0, 2, "100.00"},
		{0.005, 2, "0.01"},
		{-0.005, 2, "-0.01"},
		{1.0 / 3.0, 4, "0.3333"},
		{123.456789, 0, "123"},
		{99.9999, 2, "100.00"},
	}
	for _, tt := range tests {
		got := sprintfFloat(tt.input, tt.decimals)
		if got != tt.expected {
			t.Errorf("sprintfFloat(%f, %d) = %q, want %q", tt.input, tt.decimals, got, tt.expected)
		}
	}
}

func TestSprintfInt(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{123, "123"},
		{-1, "-1"},
		{-123, "-123"},
		{1000000, "1000000"},
	}
	for _, tt := range tests {
		got := sprintfInt(tt.input)
		if got != tt.expected {
			t.Errorf("sprintfInt(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestFormatFloat(t *testing.T) {
	if formatFloat(3.14159) != "3.1416" {
		t.Fatalf("formatFloat(3.14159) = %s", formatFloat(3.14159))
	}
	if formatFloat(42) != "42" {
		t.Fatalf("formatFloat(42) = %s", formatFloat(42))
	}
}

func TestFormatPct(t *testing.T) {
	s := formatPct(0.0567)
	if s != "5.67%" {
		t.Fatalf("formatPct(0.0567) = %s, want 5.67%%", s)
	}
	s = formatPct(-0.05)
	if s != "-5.00%" {
		t.Fatalf("formatPct(-0.05) = %s, want -5.00%%", s)
	}
	// Test negative float formatting with sprintfFloat fix
	f := -0.05
	s = formatPct(f)
	if s != "-5.00%" {
		t.Fatalf("formatPct(-0.05) = %s, want -5.00%%", s)
	}
}

func TestFormatMoney(t *testing.T) {
	s := formatMoney(100000)
	if s != "¥100000.00" {
		t.Fatalf("formatMoney(100000) = %s", s)
	}
}

func TestAnalyzeGridResults(t *testing.T) {
	close := []float64{100, 102, 101, 105, 107, 106, 110}
	entries := []bool{false, true, false, false, false, false, false}
	exits := []bool{false, false, false, false, false, true, false}
	pf := FromSignals(close, entries, exits, &PortfolioConfig{InitCash: 100000, Fees: 0, Slippage: 0, Size: 1.0})
	stats := pf.GetStats()

	gr := &GridSearchResult{
		TotalCombinations: 1,
		Results: []*BacktestResult{
			{
				StrategyName: "MA",
				Params:       map[string]any{"fast": 5},
				Stats:        stats,
			},
		},
	}

	analysis := AnalyzeGridResults([]*GridSearchResult{gr})
	if analysis == nil {
		t.Fatal("analysis should not be nil")
	}
	if analysis.TotalStrategies != 1 {
		t.Fatalf("expected 1 strategy, got %d", analysis.TotalStrategies)
	}
	if analysis.BestStrategy != "MA" {
		t.Fatalf("expected MA, got %s", analysis.BestStrategy)
	}
}

func TestSortResultsBySharpe(t *testing.T) {
	close := []float64{100, 110, 120}
	e := []bool{false, true, false}
	x := []bool{false, false, true}

	pf1 := FromSignals(close, e, x, &PortfolioConfig{InitCash: 100000, Fees: 0, Slippage: 0, Size: 0.5})
	pf2 := FromSignals(close, e, x, &PortfolioConfig{InitCash: 100000, Fees: 0, Slippage: 0, Size: 1.0})

	results := []*BacktestResult{
		{StrategyName: "A", Stats: pf1.GetStats()},
		{StrategyName: "B", Stats: pf2.GetStats()},
	}

	SortResults(results, "sharpe")
	if results[0].StrategyName != "B" {
		t.Fatal("SortResults by sharpe should put B first (higher size = higher return)")
	}
}

// --- helpers ---

func generateTrendData(start float64, slope float64) []float64 {
	n := 200
	data := make([]float64, n)
	for i := 0; i < n; i++ {
		data[i] = start + float64(i)*slope
	}
	return data
}

func generateDates(n int) []string {
	dates := make([]string, n)
	for i := 0; i < n; i++ {
		dates[i] = "2024-01-" + pad2(i+1)
	}
	return dates
}

func pad2(i int) string {
	if i < 10 {
		return "0" + strconv.Itoa(i)
	}
	return strconv.Itoa(i)
}
