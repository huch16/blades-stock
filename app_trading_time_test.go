package main

// 交易时间边界测试（isTradingTime / IsHKTradingTime / IsUSTradingTime）。
// 全部用例都与是否节假日无关：周末直接返回 false（不调节假日接口）；
// 工作日的用例只取“时间窗口外必 false / 窗口内且为普通工作日必 true”
// 的情形——2026-11-18（周三）是普通交易日，节假日接口不可达时
// isTradingDay 也会默认按交易日处理，所以断言稳定，不依赖外网。
// 注意：现有 app_test.go 里的 TestIsHKTradingTime / TestIsUSTradingTime
// 只有 t.Logf、没有任何断言，本文件补上真正的断言。

import (
	"testing"
	"time"
)

func TestIsTradingTime_WeekendIsNeverTrading(t *testing.T) {
	// 2026-11-21 是周六，2026-11-22 是周日。
	for _, d := range []time.Time{
		time.Date(2026, 11, 21, 10, 0, 0, 0, time.Local),
		time.Date(2026, 11, 22, 10, 0, 0, 0, time.Local),
	} {
		if isTradingTime(d) {
			t.Errorf("isTradingTime(%v) = true, want false (weekend)", d)
		}
	}
}

func TestIsTradingTime_WeekdayBoundaries(t *testing.T) {
	// 2026-11-18（周三）普通交易日。
	wd := func(h, m int) time.Time {
		return time.Date(2026, 11, 18, h, m, 0, 0, time.Local)
	}
	inside := []time.Time{
		wd(9, 15), // 开盘集合竞价起点
		wd(9, 16),
		wd(10, 0),
		wd(11, 30), // 上午收盘
		wd(13, 0),
		wd(14, 59),
		wd(15, 0), // 下午收盘
	}
	for _, d := range inside {
		if !isTradingTime(d) {
			t.Errorf("isTradingTime(%v) = false, want true", d)
		}
	}
	outside := []time.Time{
		wd(3, 0),   // 凌晨
		wd(9, 14),  // 开盘前一分钟
		wd(11, 31), // 午休开始
		wd(12, 0),  // 午休
		wd(12, 59),
		wd(15, 1), // 收盘后一分钟
		wd(16, 0),
		wd(21, 0),
	}
	for _, d := range outside {
		if isTradingTime(d) {
			t.Errorf("isTradingTime(%v) = true, want false", d)
		}
	}
}

func TestIsHKTradingTime_WeekendAndOffHours(t *testing.T) {
	sat := time.Date(2026, 11, 21, 10, 0, 0, 0, time.Local)
	if IsHKTradingTime(sat) {
		t.Error("IsHKTradingTime(Saturday) = true, want false")
	}
	// 工作日凌晨 3 点不在任何港股交易窗口内，与节假日无关。
	off := time.Date(2026, 11, 18, 3, 0, 0, 0, time.Local)
	if IsHKTradingTime(off) {
		t.Error("IsHKTradingTime(weekday 03:00) = true, want false")
	}
	// 午休 12:01 也不在窗口内（12:00 整点被现有实现判为交易中，见下一断言）。
	lunch := time.Date(2026, 11, 18, 12, 1, 0, 0, time.Local)
	if IsHKTradingTime(lunch) {
		t.Error("IsHKTradingTime(weekday 12:01) = true, want false")
	}
	// 09:30、12:00、16:00 在现有实现里判为交易中（窗口边界，钉住行为）。
	for _, d := range []time.Time{
		time.Date(2026, 11, 18, 9, 30, 0, 0, time.Local),
		time.Date(2026, 11, 18, 12, 0, 0, 0, time.Local),
		time.Date(2026, 11, 18, 16, 0, 0, 0, time.Local),
	} {
		if !IsHKTradingTime(d) {
			t.Errorf("IsHKTradingTime(%v) = false, want true (current boundary behavior)", d)
		}
	}
}

func TestIsUSTradingTime_WeekendAndOffHours(t *testing.T) {
	est, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("America/New_York tzdata unavailable: " + err.Error())
	}
	// 2026-11-21 是周六（美东）。
	sat := time.Date(2026, 11, 21, 10, 0, 0, 0, est)
	if IsUSTradingTime(sat) {
		t.Error("IsUSTradingTime(Saturday) = true, want false")
	}
	// 美东周三 09:30 开盘、16:00 收盘、20:00 盘后结束；23:00 不在任何窗口内。
	for _, d := range []time.Time{
		time.Date(2026, 11, 18, 9, 30, 0, 0, est),
		time.Date(2026, 11, 18, 16, 0, 0, 0, est),
		time.Date(2026, 11, 18, 20, 0, 0, 0, est),
	} {
		if !IsUSTradingTime(d) {
			t.Errorf("IsUSTradingTime(%v) = false, want true (current boundary behavior)", d)
		}
	}
	if IsUSTradingTime(time.Date(2026, 11, 18, 23, 0, 0, 0, est)) {
		t.Error("IsUSTradingTime(weekday 23:00 EST) = true, want false")
	}
	if IsUSTradingTime(time.Date(2026, 11, 18, 20, 1, 0, 0, est)) {
		t.Error("IsUSTradingTime(weekday 20:01 EST) = true, want false")
	}
}
