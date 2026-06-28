package main

import (
	"testing"
	"time"
)

// Pure unit tests (no DB, no network, no wails runtime)

func TestIsHKTradingTime(t *testing.T) {
	f := IsHKTradingTime(time.Now())
	t.Logf("IsHKTradingTime: %v", f)
}

func TestIsUSTradingTime(t *testing.T) {
	date := time.Now()
	hour, minute, _ := date.Clock()
	t.Logf("current time: %02d:%02d", hour, minute)
	t.Logf("IsUSTradingTime: %v", IsUSTradingTime(date))
}

func TestCalculateNextRunTime(t *testing.T) {
	// No DB or runtime required for cron expression parsing.
	got := NewApp().CalculateNextRunTime("0 0 0 * * ?")
	if got == "" {
		t.Fatal("expected non-empty next run time")
	}
	t.Logf("next run: %s", got)
}
