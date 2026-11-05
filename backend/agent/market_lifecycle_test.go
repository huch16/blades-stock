package agent

import (
	"testing"
	"time"
)

func TestGetCurrentMarketPhase(t *testing.T) {
	cases := []struct {
		name string
		hm   int
		want MarketPhase
	}{
		{"pre_market", 900, PhasePreMarket},
		{"call_auction", 920, PhaseCallAuction},
		{"morning_trading", 1000, PhaseMorningTrading},
		{"noon_break", 1200, PhaseNoonBreak},
		{"afternoon_trading", 1400, PhaseAfternoonTrading},
		{"post_market", 1530, PhasePostMarket},
		{"closed_night", 2200, PhaseClosed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Date(2026, 7, 7, tc.hm/100, tc.hm%100, 0, 0, time.Local)
			got, _ := GetCurrentMarketPhase(now)
			if got != tc.want {
				t.Errorf("expected %s, got %s", tc.want, got)
			}
		})
	}
}
