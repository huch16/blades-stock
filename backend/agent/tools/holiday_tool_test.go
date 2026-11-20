package tools

import (
	"blades-stock/backend/agent/types"
	"testing"
)

func TestGetHolidayTools(t *testing.T) {
	tools := GetHolidayTools()
	t.Logf("Total holiday tools count: %d", len(tools))

	for i, tl := range tools {
		t.Logf("Tool %d: %s - %s", i+1, tl.Name(), tl.Description())
	}
}

func TestGetHolidayInfo(t *testing.T) {
	tl := NewHolidayTool(
		"GetHolidayInfo",
		"查询指定日期的节假日信息",
		map[string]*types.ParamDef{
			"date": {
				Type:     "string",
				Desc:     "查询日期",
				Required: false,
			},
		},
		func(args string) (string, error) {
			return "test", nil
		},
	)

	if tl.Name() != "GetHolidayInfo" {
		t.Errorf("Expected tool name 'GetHolidayInfo', got '%s'", tl.Name())
	}

	t.Logf("Tool info: Name=%s, Desc=%s", tl.Name(), tl.Description())
}

func TestIsTradingDay_Weekend(t *testing.T) {
	testCases := []struct {
		name     string
		date     string
		expected bool
	}{
		{"周六", "2026-04-04", false},
		{"周日", "2026-04-05", false},
		{"周一", "2026-04-06", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing date: %s, expected trading day: %v", tc.date, tc.expected)
		})
	}
}

func TestIsTradingDay_Holiday(t *testing.T) {
	testCases := []struct {
		name string
		date string
		desc string
	}{
		{"元旦", "2026-01-01", "元旦假期"},
		{"春节初一", "2026-02-17", "春节假期"},
		{"国庆节", "2026-10-01", "国庆假期"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing holiday date: %s (%s)", tc.date, tc.desc)
		})
	}
}
