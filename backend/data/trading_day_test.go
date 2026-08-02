package data

import (
	"testing"
	"time"
)

// Note: these tests rely on the weekend-heuristic fallback only; they never
// call InitTradingCalendar (which would hit the network), so the global cache
// stays uninitialized during the package run.

func TestIsTradingDayWeekendHeuristic(t *testing.T) {
	cases := []struct {
		date string
		want bool
	}{
		{"2026-01-03", false}, // Saturday
		{"2026-01-04", false}, // Sunday
		{"2026-01-05", true},  // Monday
		{"2026-01-02", true},  // Friday
	}
	for _, c := range cases {
		if got := IsTradingDay(c.date); got != c.want {
			t.Errorf("IsTradingDay(%q) = %v, want %v", c.date, got, c.want)
		}
	}
}

func TestIsTradingDayCompactFormat(t *testing.T) {
	if got := IsTradingDay("20260103"); got != false {
		t.Errorf("IsTradingDay(\"20260103\") = %v, want false (Saturday)", got)
	}
	if got := IsTradingDay("20260105"); got != true {
		t.Errorf("IsTradingDay(\"20260105\") = %v, want true (Monday)", got)
	}
}

func TestIsTradingDayFallbackToNow(t *testing.T) {
	wd := time.Now().Weekday()
	want := wd != time.Saturday && wd != time.Sunday
	for _, date := range []string{"", "not-a-date"} {
		if got := IsTradingDay(date); got != want {
			t.Errorf("IsTradingDay(%q) = %v, want %v (today weekday=%v)", date, got, want, wd)
		}
	}
}

func TestIsTodayTradingDay(t *testing.T) {
	wd := time.Now().Weekday()
	want := wd != time.Saturday && wd != time.Sunday
	if got := IsTodayTradingDay(); got != want {
		t.Errorf("IsTodayTradingDay() = %v, want %v", got, want)
	}
}

func TestGetLatestTradingDay(t *testing.T) {
	cases := []struct {
		date string
		want string
	}{
		{"2026-01-04", "2026-01-02"}, // Sunday -> Friday
		{"2026-01-03", "2026-01-02"}, // Saturday -> Friday
		{"2026-01-05", "2026-01-05"}, // Monday -> itself
		{"20260102", "2026-01-02"},   // compact format, Friday -> itself
	}
	for _, c := range cases {
		if got := GetLatestTradingDay(c.date); got != c.want {
			t.Errorf("GetLatestTradingDay(%q) = %q, want %q", c.date, got, c.want)
		}
	}
}

func TestGetLatestTradingDayFallback(t *testing.T) {
	for _, date := range []string{"", "garbage"} {
		got := GetLatestTradingDay(date)
		if got == "" {
			t.Errorf("GetLatestTradingDay(%q) = empty", date)
		}
		if !IsTradingDay(got) {
			t.Errorf("GetLatestTradingDay(%q) = %q, not a trading day", date, got)
		}
	}
}

func TestGetNextTradingDay(t *testing.T) {
	cases := []struct {
		date string
		want string
	}{
		{"2026-01-02", "2026-01-05"}, // Friday -> Monday
		{"2026-01-03", "2026-01-05"}, // Saturday -> Monday
		{"2026-01-05", "2026-01-06"}, // Monday -> Tuesday
	}
	for _, c := range cases {
		if got := GetNextTradingDay(c.date); got != c.want {
			t.Errorf("GetNextTradingDay(%q) = %q, want %q", c.date, got, c.want)
		}
	}
}

func TestGetNextTradingDayFallback(t *testing.T) {
	for _, date := range []string{"", "garbage"} {
		got := GetNextTradingDay(date)
		if got == "" {
			t.Errorf("GetNextTradingDay(%q) = empty", date)
		}
		if !IsTradingDay(got) {
			t.Errorf("GetNextTradingDay(%q) = %q, not a trading day", date, got)
		}
	}
}

func TestGetTradingDaysBetween(t *testing.T) {
	got := GetTradingDaysBetween("2026-01-02", "2026-01-06")
	want := []string{"2026-01-02", "2026-01-05", "2026-01-06"}
	if len(got) != len(want) {
		t.Fatalf("GetTradingDaysBetween len = %d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("GetTradingDaysBetween[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestGetTradingDaysBetweenCompactFormat(t *testing.T) {
	got := GetTradingDaysBetween("20260102", "20260106")
	if len(got) != 3 {
		t.Errorf("compact format days = %v, want 3 entries", got)
	}
}

func TestGetTradingDaysBetweenInvalid(t *testing.T) {
	if got := GetTradingDaysBetween("bad", "2026-01-06"); got != nil {
		t.Errorf("bad start = %v, want nil", got)
	}
	if got := GetTradingDaysBetween("2026-01-06", "bad"); got != nil {
		t.Errorf("bad end = %v, want nil", got)
	}
	if got := GetTradingDaysBetween("2026-01-06", "2026-01-02"); len(got) != 0 {
		t.Errorf("reversed range = %v, want empty", got)
	}
}
