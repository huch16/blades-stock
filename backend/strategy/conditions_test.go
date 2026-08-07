package strategy

import "testing"

func TestParseCondition(t *testing.T) {
	cases := []struct {
		expr     string
		name     string
		argCount int
		wantErr  bool
	}{
		{"ma_golden_cross(5,10)", "ma_golden_cross", 2, false},
		{"above_ma(20)", "above_ma", 1, false},
		{"macd_golden_cross", "macd_golden_cross", 0, false},
		{"volume_ratio_gt(1.5)", "volume_ratio_gt", 1, false},
		{"pullback_below_ma(20,3)", "pullback_below_ma", 2, false},
		{"bad(abc)", "bad", 0, true},
		{"unclosed(1", "", 0, true},
	}
	for _, c := range cases {
		name, args, err := ParseCondition(c.expr)
		if (err != nil) != c.wantErr {
			t.Fatalf("ParseCondition(%q) err = %v, wantErr = %v", c.expr, err, c.wantErr)
		}
		if err != nil {
			continue
		}
		if name != c.name || len(args) != c.argCount {
			t.Fatalf("ParseCondition(%q) = (%q, %v), want (%q, %d)", c.expr, name, args, c.name, c.argCount)
		}
	}
}

func risingIndicator() *Indicators {
	closes := make([]float64, 70)
	for i := range closes {
		closes[i] = 10 + float64(i)*0.1
	}
	return ComputeIndicators(makeBars(closes, nil))
}

func TestConditionsRising(t *testing.T) {
	ind := risingIndicator()

	tests := []struct {
		expr string
		want bool
	}{
		{"above_ma(5)", true},
		{"above_ma(60)", true},
		{"below_ma(5)", false},
		{"ma_bullish_alignment", true},
		{"ma_bearish_alignment", false},
		{"macd_above_zero", true},
		{"ma_golden_cross(5,10)", false},
		{"ma_death_cross(5,10)", false},
		{"change_pct_gt(0)", true},
		{"change_pct_lt(0)", false},
		{"volume_ratio_gt(0.5)", true},
		{"rsi_gt(50)", true},
		{"rsi_lt(30)", false},
		{"unknown_condition", false},
	}
	for _, tt := range tests {
		if got := evalCondition(tt.expr, ind); got != tt.want {
			t.Fatalf("evalCondition(%q) = %v, want %v", tt.expr, got, tt.want)
		}
	}
}

func TestScoreStrategy(t *testing.T) {
	s := &Strategy{
		Name:      "测试",
		BaseScore: 50,
		Bonuses: []Rule{
			{When: "above_ma(5)", Score: 10, Desc: "站上5日线"},
			{When: "unknown_cond(1)", Score: 5, Desc: "未知条件"},
			{When: "rsi_lt(30)", Score: 20, Desc: "超卖"},
		},
		Penalties: []Rule{
			{When: "below_ma(10)", Score: 30, Desc: "跌破10日线"},
			{When: "ma_bearish_alignment", Score: 25, Desc: "空头排列"},
		},
	}
	sc := ScoreStrategy(s, risingIndicator())

	if sc.Base != 50 {
		t.Fatalf("Base = %v", sc.Base)
	}
	if sc.Score != 60 { // 50 + 10, 未知跳过, 超卖不命中, 两条 penalty 未命中
		t.Fatalf("Score = %v, want 60", sc.Score)
	}
	if sc.Clamped != 60 {
		t.Fatalf("Clamped = %v, want 60", sc.Clamped)
	}
	if len(sc.Matched) != 1 || sc.Matched[0].Delta != 10 {
		t.Fatalf("Matched = %+v", sc.Matched)
	}
	if len(sc.Skipped) != 1 || sc.Skipped[0] != "unknown_cond(1)" {
		t.Fatalf("Skipped = %v", sc.Skipped)
	}
}

func TestScoreStrategyPenalty(t *testing.T) {
	closes := make([]float64, 30)
	for i := range closes {
		closes[i] = 10 - float64(i)*0.1
	}
	s := &Strategy{
		Name:      "测试",
		BaseScore: 50,
		Penalties: []Rule{
			{When: "below_ma(10)", Score: 20, Desc: "跌破10日线"},
		},
	}
	sc := ScoreStrategy(s, ComputeIndicators(makeBars(closes, nil)))
	if sc.Score != 30 {
		t.Fatalf("penalty 后 Score = %v, want 30", sc.Score)
	}
	if sc.Matched[0].Delta != -20 {
		t.Fatalf("penalty Delta = %v, want -20", sc.Matched[0].Delta)
	}
}

func TestScoreStrategyClamp(t *testing.T) {
	s := &Strategy{
		Name:      "测试",
		BaseScore: 95,
		Bonuses:   []Rule{{When: "above_ma(5)", Score: 20, Desc: "加分"}},
		Penalties: []Rule{{When: "below_ma(60)", Score: 100, Desc: "重罚"}},
	}
	sc := ScoreStrategy(s, risingIndicator())
	if sc.Clamped != 100 {
		t.Fatalf("加分超限应收敛到 100: %v", sc.Clamped)
	}

	falling := make([]float64, 70)
	for i := range falling {
		falling[i] = 30 - float64(i)*0.4
	}
	sc2 := ScoreStrategy(s, ComputeIndicators(makeBars(falling, nil)))
	if sc2.Clamped != 0 {
		t.Fatalf("重罚应收敛到 0: %v", sc2.Clamped)
	}
}
