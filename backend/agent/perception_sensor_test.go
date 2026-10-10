package agent

import (
	"testing"
	"time"
)

func TestCalculateClockPhase(t *testing.T) {
	// 模拟周一 09:20 竞价
	auctionTime := time.Date(2026, 7, 6, 9, 20, 0, 0, time.Local)
	phase, _ := CalculateClockPhase(auctionTime)
	if phase != PhaseCallAuction {
		t.Errorf("expected PhaseCallAuction, got %s", phase)
	}

	// 模拟周一 10:00 早盘
	openTime := time.Date(2026, 7, 6, 10, 0, 0, 0, time.Local)
	phase, _ = CalculateClockPhase(openTime)
	if phase != PhaseMorningTrading {
		t.Errorf("expected PhaseMorningTrading, got %s", phase)
	}

	// 模拟周日
	weekendTime := time.Date(2026, 7, 5, 10, 0, 0, 0, time.Local)
	phase, _ = CalculateClockPhase(weekendTime)
	if phase != PhaseClosed {
		t.Errorf("expected PhaseClosed, got %s", phase)
	}
}

func TestGetGlobalMarketSensorSnapshot(t *testing.T) {
	sensor := GetGlobalMarketSensor()
	snapshot := sensor.GetSnapshot()
	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.SummaryPrompt == "" {
		t.Error("expected non-empty SummaryPrompt")
	}
}
