package main

import (
	"fmt"
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
)

func main() {
	db.Init("data/stock.db")
	logger.InitLogger()

	// 1. 初始化交易日历
	fmt.Println("=== InitTradingCalendar ===")
	data.InitTradingCalendar()

	// 2. 测试关键日期
	tests := []struct {
		date    string
		expect  string
		desc    string
	}{
		{"2026-07-06", "true", "周一（应交易日）"},
		{"2026-07-04", "false", "周六"},
		{"2026-07-05", "false", "周日"},
		{"2026-01-01", "false", "元旦"},
		{"2026-02-16", "false", "春节初一起"},
		{"2026-02-23", "false", "春节末"},
		{"2026-02-24", "true", "春节后首个交易日"},
		{"2026-10-01", "false", "国庆"},
		{"2026-10-08", "true", "国庆后首个交易日"},
		{"2026-05-01", "false", "劳动节"},
	}

	fmt.Println("\n=== Trading Day Tests ===")
	allPass := true
	for _, t := range tests {
		result := data.IsTradingDay(t.date)
		pass := (result && t.expect == "true") || (!result && t.expect == "false")
		status := "PASS"
		if !pass {
			status = "FAIL"
			allPass = false
		}
		fmt.Printf("  [%s] %s (%s): expected=%s, got=%v\n", status, t.date, t.desc, t.expect, result)
	}

	// 3. 测试 GetLatestTradingDay
	fmt.Println("\n=== GetLatestTradingDay ===")
	fmt.Printf("  Today: %s\n", data.GetLatestTradingDay(""))

	// 从周日 2026-07-05 往前找
	fmt.Printf("  From 2026-07-05 (Sun): %s\n", data.GetLatestTradingDay("2026-07-05"))
	// 从国庆假期 2026-10-03 往前找
	fmt.Printf("  From 2026-10-03 (National Day): %s\n", data.GetLatestTradingDay("2026-10-03"))

	// 4. 测试 GetNextTradingDay
	fmt.Println("\n=== GetNextTradingDay ===")
	fmt.Printf("  From 2026-07-06 (Mon): %s\n", data.GetNextTradingDay("2026-07-06"))
	// 从国庆前 2026-09-30 找下一个
	fmt.Printf("  From 2026-09-30: %s\n", data.GetNextTradingDay("2026-09-30"))

	// 5. 测试 Baostock 健康检查
	fmt.Println("\n=== Baostock Health Check ===")
	api := data.NewBaostockApi()
	fmt.Printf("  Health: %v\n", api.HealthCheck())

	if allPass {
		fmt.Println("\n✅ ALL TESTS PASSED")
	} else {
		fmt.Println("\n❌ SOME TESTS FAILED")
	}
}
