package data

import (
	"testing"

	"blades-stock/backend/datasource"
)

func TestNormalizeBaostockCode(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"sh600671", "sh.600671"},
		{"sz000100", "sz.000100"},
		{"bj430047", "bj.430047"},
		{"600671", "sh.600671"},
		{"000100", "sz.000100"},
		{"300750", "sz.300750"},
		{"830799", "bj.830799"},
		{"sh.600671", "sh.600671"},
		{"  600671  ", "sh.600671"},
		{"ab", "ab"},
	}
	for _, c := range cases {
		if got := datasource.NormalizeBaostockCode(c.in); got != c.want {
			t.Errorf("normalizeBaostockCode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestComputeSMA(t *testing.T) {
	closes := []float64{1, 2, 3, 4, 5}
	cases := []struct {
		idx, period int
		want        float64
	}{
		{4, 3, 4},
		{0, 1, 1},
		{1, 2, 1.5},
		{1, 3, -1}, // not enough data
		{0, 2, -1},
	}
	for _, c := range cases {
		if got := computeSMA(closes, c.idx, c.period); got != c.want {
			t.Errorf("computeSMA(closes, %d, %d) = %v, want %v", c.idx, c.period, got, c.want)
		}
	}
}

func TestGetMaxPeriod(t *testing.T) {
	if got := getMaxPeriod([]int{1, 5, 3}); got != 5 {
		t.Errorf("getMaxPeriod = %d, want 5", got)
	}
	if got := getMaxPeriod([]int{}); got != 0 {
		t.Errorf("getMaxPeriod(empty) = %d, want 0", got)
	}
	if got := getMaxPeriod([]int{0, 0}); got != 0 {
		t.Errorf("getMaxPeriod(zeros) = %d, want 0", got)
	}
}

func TestParseFloatToFloat(t *testing.T) {
	if v, err := parseFloatToFloat(""); v != 0 || err != nil {
		t.Errorf("parseFloatToFloat(\"\") = %v, %v", v, err)
	}
	if v, err := parseFloatToFloat("-"); v != 0 || err != nil {
		t.Errorf("parseFloatToFloat(-) = %v, %v", v, err)
	}
	if v, err := parseFloatToFloat("null"); v != 0 || err != nil {
		t.Errorf("parseFloatToFloat(null) = %v, %v", v, err)
	}
	if v, err := parseFloatToFloat("12.5"); v != 12.5 || err != nil {
		t.Errorf("parseFloatToFloat(12.5) = %v, %v", v, err)
	}
	if _, err := parseFloatToFloat("abc"); err == nil {
		t.Error("parseFloatToFloat(abc) should error")
	}
}

func TestAggregateKLineEveryN(t *testing.T) {
	if got := AggregateKLineEveryN(nil, 2); got != nil {
		t.Error("AggregateKLineEveryN(nil) should return nil")
	}
	src := []KLineData{}
	if got := AggregateKLineEveryN(&src, 2); got != &src {
		t.Error("AggregateKLineEveryN(empty) should return src unchanged")
	}
	if got := AggregateKLineEveryN(nil, 1); got != nil {
		t.Error("AggregateKLineEveryN(n=1) should return input unchanged")
	}

	bars := []KLineData{
		{Day: "09:30", Open: "10.00", Close: "10.50", High: "10.60", Low: "9.90", Volume: "100", Amount: "1000"},
		{Day: "09:31", Open: "10.50", Close: "10.80", High: "10.90", Low: "10.40", Volume: "200", Amount: "2100"},
	}
	out := AggregateKLineEveryN(&bars, 2)
	if out == nil || len(*out) != 1 {
		t.Fatalf("aggregate 2->1 bars failed: %+v", out)
	}
	agg := (*out)[0]
	if agg.Day != "09:31" || agg.Open != "10.00" || agg.Close != "10.80" {
		t.Errorf("agg OHLC = day=%s open=%s close=%s, want 09:31/10.00/10.80", agg.Day, agg.Open, agg.Close)
	}
	if agg.High != "10.9000" || agg.Low != "9.9000" {
		t.Errorf("agg high/low = %s/%s, want 10.9000/9.9000", agg.High, agg.Low)
	}
	if agg.Volume != "300" || agg.Amount != "3100.00" {
		t.Errorf("agg volume/amount = %s/%s, want 300/3100.00", agg.Volume, agg.Amount)
	}
}

func TestCalculateChangePercent(t *testing.T) {
	cases := []struct {
		open, close string
		want        float64
	}{
		{"10", "11", 10},
		{"10", "5", -50},
		{"0", "11", 0},
		{"10", "10", 0},
	}
	for _, c := range cases {
		if got := calculateChangePercent(c.open, c.close); got != c.want {
			t.Errorf("calculateChangePercent(%q,%q) = %v, want %v", c.open, c.close, got, c.want)
		}
	}
}

func TestNormalizeF10Code(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"600519", "600519.SH"},
		{"000001", "000001.SZ"},
		{"300750", "300750.SZ"},
		{"830799", "830799.BJ"},
		{"430047", "430047.BJ"},
		{"600519.SH", "600519.SH"},
		{"sh600519", "600519.SH"},
	}
	for _, c := range cases {
		if got := normalizeF10Code(c.in); got != c.want {
			t.Errorf("normalizeF10Code(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestF10FieldNameCN(t *testing.T) {
	if got := f10FieldNameCN("EPSJB"); got != "基本每股收益" {
		t.Errorf("f10FieldNameCN(EPSJB) = %q, want 基本每股收益", got)
	}
	if got := f10FieldNameCN("UNKNOWN_FIELD"); got != "UNKNOWN_FIELD" {
		t.Errorf("f10FieldNameCN(unknown) = %q, want passthrough", got)
	}
}

func TestF10FormatValue(t *testing.T) {
	if got := f10FormatValue("K", nil); got != "-" {
		t.Errorf("f10FormatValue(nil) = %q, want -", got)
	}
	if got := f10FormatValue("EPSJB", "abc"); got != "abc" {
		t.Errorf("f10FormatValue(string) = %q, want abc", got)
	}
	if got := f10FormatValue("PARENT_NETPROFIT", float64(1.5e8)); got != "1.50亿" {
		t.Errorf("f10FormatValue(money) = %q, want 1.50亿", got)
	}
	if got := f10FormatValue("K", 42); got != "42" {
		t.Errorf("f10FormatValue(int) = %q, want 42", got)
	}
}

func TestF10FormatMoney(t *testing.T) {
	cases := []struct {
		val  float64
		want string
	}{
		{2.5e8, "2.50亿"},
		{-1e9, "-10.00亿"},
		{5e4, "5.00万"},
		{123.456, "123.46"},
		{0, "0.00"},
	}
	for _, c := range cases {
		if got := f10FormatMoney(c.val); got != c.want {
			t.Errorf("f10FormatMoney(%v) = %q, want %q", c.val, got, c.want)
		}
	}
}

func TestF10FormatVolume(t *testing.T) {
	cases := []struct {
		val  float64
		want string
	}{
		{3e8, "3.00亿股"},
		{2e5, "20.00万股"},
		{100, "100股"},
		{100.5, "100股"}, // %.0f rounds half-to-even
		{0, "0股"},
	}
	for _, c := range cases {
		if got := f10FormatVolume(c.val); got != c.want {
			t.Errorf("f10FormatVolume(%v) = %q, want %q", c.val, got, c.want)
		}
	}
}

func TestF10FormatNumber(t *testing.T) {
	cases := []struct {
		key  string
		val  float64
		want string
	}{
		{"RANK", 5, "5"},
		{"RANK", 5.7, "6"},
		{"CHANGE_RATE", 12.345, "12.35%"},
		{"DEAL_PRICE", 10.5, "10.50"},
		{"PARENT_NETPROFIT", 1.5e8, "1.50亿"},
		{"TOTAL_SHARE", 2e5, "20.00万股"},
		{"EPSJB", 1.234, "1.23"},
		{"PE", 25.678, "25.68"},
		{"FOO", 5, "5"},
		{"FOO", 5.5, "5.50"},
	}
	for _, c := range cases {
		if got := f10FormatNumber(c.key, c.val); got != c.want {
			t.Errorf("f10FormatNumber(%q, %v) = %q, want %q", c.key, c.val, got, c.want)
		}
	}
}

func TestF10FormatString(t *testing.T) {
	if got := f10FormatString("REPORT_DATE", "2026-01-05 00:00:00"); got != "2026-01-05" {
		t.Errorf("f10FormatString(date) = %q, want 2026-01-05", got)
	}
	if got := f10FormatString("SECURITY_NAME_ABBR", "贵州茅台"); got != "贵州茅台" {
		t.Errorf("f10FormatString(plain) = %q, want passthrough", got)
	}
}
