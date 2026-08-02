package data

import (
	"math"
	"testing"
)

func TestParseFloat(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"", 0},
		{"-", 0},
		{"null", 0},
		{"12.34", 12.34},
		{"  56.78  ", 56.78},
		{"abc", 0},
		{"1e3", 1000},
	}
	for _, c := range cases {
		if got := parseFloat(c.in); got != c.want {
			t.Errorf("parseFloat(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParsePercent(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"", 0},
		{"-", 0},
		{"null", 0},
		{"12.34%", 0.1234},
		{"12.34", 0.1234},
		{"  50% ", 0.5},
	}
	for _, c := range cases {
		if got := parsePercent(c.in); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("parsePercent(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestRound(t *testing.T) {
	cases := []struct {
		v    float64
		d    int
		want float64
	}{
		{1.23456, 2, 1.23},
		{1.5, 0, 2},
		{1.4999, 0, 1},
		{123.456, 1, 123.5},
		{1.2345, -1, 1.2345},
		{0, 4, 0},
	}
	for _, c := range cases {
		if got := round(c.v, c.d); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("round(%v,%d) = %v, want %v", c.v, c.d, got, c.want)
		}
	}
}

func TestIsFinite(t *testing.T) {
	if !isFinite(1.0) {
		t.Error("isFinite(1.0) should be true")
	}
	if isFinite(math.NaN()) {
		t.Error("isFinite(NaN) should be false")
	}
	if isFinite(math.Inf(1)) {
		t.Error("isFinite(+Inf) should be false")
	}
	if isFinite(math.Inf(-1)) {
		t.Error("isFinite(-Inf) should be false")
	}
}

func TestClampPrice(t *testing.T) {
	if got := clampPrice(5, 1, 10); got != 5 {
		t.Errorf("clampPrice(5,1,10) = %v, want 5", got)
	}
	if got := clampPrice(0, 1, 10); got != 1 {
		t.Errorf("clampPrice(0,1,10) = %v, want 1", got)
	}
	if got := clampPrice(15, 1, 10); got != 10 {
		t.Errorf("clampPrice(15,1,10) = %v, want 10", got)
	}
}

func TestChipBarCostCenter(t *testing.T) {
	if got := chipBarCostCenter(0, 10, 5, 6, 0, 0); got != 5 {
		t.Errorf("invalid low/high (mid midpoint) = %v, want 5", got)
	}
	// close>0 path: tp = (high+low+close)/3 = 32/3 clamped to [10,11]
	if got := chipBarCostCenter(10, 11, 10, 11, 0, 0); math.Abs(got-32.0/3) > 1e-9 {
		t.Errorf("close-path = %v, want %v", got, 32.0/3)
	}
	if got := chipBarCostCenter(10, 20, 12, 16, 1000, 15000); got != 15 {
		t.Errorf("vwap path = %v, want 15", got)
	}
	if got := chipBarCostCenter(10, 20, 0, 15, 0, 0); got != 15 {
		t.Errorf("tp=(h+l+c)/3 path = %v, want 15", got)
	}
	if got := chipBarCostCenter(10, 20, 12, 0, 0, 0); got != 15 {
		t.Errorf("midpoint fallback = %v, want 15", got)
	}
}

func TestDetectMinMaxPrice(t *testing.T) {
	lines := []KLineData{
		{Low: "10.0", High: "12.0"},
		{Low: "9.5", High: "11.5"},
		{Low: "0", High: "0"},
	}
	minP, maxP := detectMinMaxPrice(lines)
	if minP != 9.5 {
		t.Errorf("minP = %v, want 9.5", minP)
	}
	if maxP != 12.0 {
		t.Errorf("maxP = %v, want 12.0", maxP)
	}
	if _, maxP := detectMinMaxPrice(nil); maxP != 0 {
		t.Errorf("empty input maxP = %v, want 0", maxP)
	}
}

func TestAddChipVolumeKernelDegenerate(t *testing.T) {
	dist := make([]float64, 10)
	width := 1.0
	minP := 0.0
	addChipVolumeKernel(dist, 10, minP, width, 0, 0, 0, 0)
	for i, v := range dist {
		if v != 0 {
			t.Errorf("dist[%d] = %v, want 0 (zero vol/path)", i, v)
		}
	}
	addChipVolumeKernel(dist, 10, minP, width, 5, 5, 100, 7)
	if dist[5] != 100 {
		t.Errorf("one-bar collapse: dist[5] = %v, want 100", dist[5])
	}
}

func TestNewChipDistributionCalculator(t *testing.T) {
	c := NewChipDistributionCalculator()
	if c == nil {
		t.Fatal("NewChipDistributionCalculator returned nil")
	}
}
