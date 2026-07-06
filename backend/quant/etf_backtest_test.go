package quant

import (
	"math"
	"testing"
)

func TestComputeETFScore_Uptrend(t *testing.T) {
	closes := make([]float64, 21)
	for i := 0; i < 21; i++ {
		closes[i] = 1.0 + float64(i)*0.02
	}
	annual, r2, score := computeETFScore(closes, 21)
	if score <= 0 {
		t.Fatalf("uptrend should have positive score, got %f", score)
	}
	if annual <= 0 {
		t.Fatalf("uptrend annualized should be positive, got %f", annual)
	}
	if r2 < 0.8 {
		t.Fatalf("linear uptrend should have high R2, got %f", r2)
	}
}

func TestComputeETFScore_Downtrend(t *testing.T) {
	closes := make([]float64, 21)
	for i := 0; i < 21; i++ {
		closes[i] = 5.0 - float64(i)*0.03
	}
	annual, r2, score := computeETFScore(closes, 21)
	if score >= 0 {
		t.Fatalf("downtrend should have negative score, got %f", score)
	}
	if annual >= 0 {
		t.Fatalf("downtrend annualized should be negative, got %f", annual)
	}
	if r2 < 0.8 {
		t.Fatalf("linear downtrend should have high R2, got %f", r2)
	}
}

func TestComputeETFScore_Flat(t *testing.T) {
	closes := make([]float64, 21)
	for i := 0; i < 21; i++ {
		closes[i] = 10.0
	}
	annual, _, score := computeETFScore(closes, 21)
	if math.Abs(score) > 1e-6 {
		t.Fatalf("flat prices should have near-zero score, got %f", score)
	}
	if math.Abs(annual) > 1e-6 {
		t.Fatalf("flat prices should have near-zero annualized, got %f", annual)
	}
}

func TestComputeETFScore_ShortData(t *testing.T) {
	closes := []float64{1.0, 2.0, 3.0}
	annual, r2, score := computeETFScore(closes, 21)
	if score != 0 || annual != 0 || r2 != 0 {
		t.Fatalf("data < momentumDays should return zeros, got %f %f %f", annual, r2, score)
	}
}

func TestComputeETFScore_ExactWindow(t *testing.T) {
	closes := make([]float64, 30)
	for i := 0; i < 30; i++ {
		closes[i] = 2.0 + float64(i)*0.01
	}
	annual, r2, score := computeETFScore(closes, 21)
	if score <= 0 {
		t.Fatalf("longer data should use last 21, should be positive, got %f", score)
	}
	if r2 < 0.7 {
		t.Fatalf("near-linear trend should have high R2, got %f", r2)
	}
	_ = annual
}

func TestComputeETFScore_AcceleratingUptrend(t *testing.T) {
	closes := make([]float64, 21)
	for i := 0; i < 21; i++ {
		closes[i] = 1.0 + math.Pow(float64(i)/10.0, 2)
	}
	annual, r2, score := computeETFScore(closes, 21)
	if score <= 0 {
		t.Fatalf("accelerating uptrend should still have positive score, got %f", score)
	}
	if annual <= 0 {
		t.Fatalf("accelerating uptrend annualized should be positive, got %f", annual)
	}
	// R2 may be lower for non-linear trend
	_ = r2
}

func TestComputeETFScore_RecentRally(t *testing.T) {
	closes := make([]float64, 21)
	for i := 0; i < 15; i++ {
		closes[i] = 1.0
	}
	for i := 15; i < 21; i++ {
		closes[i] = 1.0 + float64(i-14)*0.02
	}
	score1, _, _ := computeETFScore(closes, 21)
	_ = score1
}

func TestComputeETFScore_RecentDrop(t *testing.T) {
	closes := make([]float64, 21)
	for i := 0; i < 15; i++ {
		closes[i] = 5.0
	}
	for i := 15; i < 21; i++ {
		closes[i] = 5.0 - float64(i-14)*0.03
	}
	score1, _, _ := computeETFScore(closes, 21)
	_ = score1
}

func TestComputeETFScore_VolatileFlat(t *testing.T) {
	closes := make([]float64, 21)
	base := 10.0
	for i := 0; i < 21; i++ {
		closes[i] = base + math.Sin(float64(i)*2.0)
	}
	annual, r2, score := computeETFScore(closes, 21)
	if math.Abs(score) > 5.0 {
		t.Fatalf("oscillating prices should have low absolute score, got %f", score)
	}
	if r2 < -0.5 {
		t.Fatalf("oscillating around flat should not be terrible fit, got %f", r2)
	}
	_ = annual
}

func TestGetETFName_Found(t *testing.T) {
	syms := []string{"510300.SH", "588000.SH"}
	names := []string{"HS300ETF", "KC50ETF"}
	if n := getETFName("588000.SH", syms, names); n != "KC50ETF" {
		t.Fatalf("expected KC50ETF, got %s", n)
	}
}

func TestGetETFName_NotFound(t *testing.T) {
	syms := []string{"510300.SH"}
	names := []string{"HS300ETF"}
	if n := getETFName("999999.SH", syms, names); n != "999999.SH" {
		t.Fatalf("expected symbol fallback, got %s", n)
	}
}

func TestGetETFName_IndexBoundary(t *testing.T) {
	// More names than symbols - should still work
	syms := []string{"510300.SH"}
	names := []string{"HS300ETF", "Extra"}
	if n := getETFName("510300.SH", syms, names); n != "HS300ETF" {
		t.Fatalf("expected HS300ETF, got %s", n)
	}
}

func TestDefaultETFSymbols(t *testing.T) {
	syms := DefaultETFSymbols()
	names := DefaultETFNames()
	if len(syms) != len(names) {
		t.Fatalf("symbols and names length mismatch: %d vs %d", len(syms), len(names))
	}
	if len(syms) == 0 {
		t.Fatal("no default ETF symbols")
	}
}

func TestDefaultETFMomentumConfig(t *testing.T) {
	cfg := DefaultETFMomentumConfig()
	if cfg.TopN != 3 {
		t.Fatalf("default TopN should be 3, got %d", cfg.TopN)
	}
	if cfg.MomentumDays != 21 {
		t.Fatalf("default MomentumDays should be 21, got %d", cfg.MomentumDays)
	}
	if cfg.InitCash != 100000 {
		t.Fatalf("default InitCash should be 100000, got %.0f", cfg.InitCash)
	}
}
