package data

import (
	"fmt"
	"sync"
	"time"

	"blades-stock/backend/circuitbreaker"
	"blades-stock/backend/logger"
)

// ============================================================================
// 交易日历 — 优先使用 AData（8000端口），降级 Baostock（8090端口），最终降级周末判断
// 各数据源由断路器保护：连续失败自动熔断，冷却后自动探测恢复。
// ============================================================================

var (
	tradingDayCache   map[string]bool // dateStr("2006-01-02") → true=交易日
	tradingDayCacheMu sync.RWMutex
	tradingDayInited  bool
)

// InitTradingCalendar 初始化交易日历缓存
// 优先从 AData 获取，失败则尝试 Baostock，最终降级到周末判断
func InitTradingCalendar() {
	// 优先：AData（8000端口，稳定运行）
	if cache := tryTradingCalendarSource("adata", func() (map[string]bool, error) {
		adataClient := NewAdataClient()
		days, err := adataClient.GetTradeCalendarDays()
		if err != nil {
			return nil, err
		}
		cache := make(map[string]bool, len(days))
		for _, day := range days {
			cache[day] = true
		}
		return cache, nil
	}); cache != nil {
		tradingDayCacheMu.Lock()
		tradingDayCache = cache
		tradingDayInited = true
		tradingDayCacheMu.Unlock()
		logger.SugaredLogger.Infof("InitTradingCalendar: AData 已加载交易日历，共 %d 个交易日", len(cache))
		return
	}

	// 降级：Baostock（8090端口）
	year := time.Now().Year()
	if cache := tryTradingCalendarSource("baostock", func() (map[string]bool, error) {
		api := NewBaostockApi()
		resp, err := api.GetTradeCalendarYear(year)
		if err != nil {
			return nil, err
		}
		if resp == nil || len(resp.TradingDays) == 0 {
			return nil, fmt.Errorf("baostock 返回空交易日历")
		}
		cache := make(map[string]bool, len(resp.TradingDays))
		for _, day := range resp.TradingDays {
			cache[day] = true
		}
		// 合并下一年1月
		if resp2, err := api.GetTradeCalendarYear(year + 1); err == nil {
			for _, day := range resp2.TradingDays {
				if len(day) >= 7 && day[:4] == time.Date(year+1, 1, 1, 0, 0, 0, 0, time.Local).Format("2006")[:4] {
					cache[day] = true
				}
			}
		}
		return cache, nil
	}); cache != nil {
		tradingDayCacheMu.Lock()
		tradingDayCache = cache
		tradingDayInited = true
		tradingDayCacheMu.Unlock()
		logger.SugaredLogger.Infof("InitTradingCalendar: Baostock 已加载交易日历，共 %d 个交易日", len(cache))
		return
	}

	tradingDayCacheMu.Lock()
	tradingDayCache = nil
	tradingDayInited = false
	tradingDayCacheMu.Unlock()
}

// tryTradingCalendarSource 带断路器的交易日历数据源探测。
// 返回 nil 表示该源不可用（断路器断开 / 调用失败 / 返回空数据），由调用方继续降级。
func tryTradingCalendarSource(name string, fn func() (map[string]bool, error)) map[string]bool {
	cb := circuitbreaker.Get(name)
	if !cb.Allow() {
		logger.SugaredLogger.Debugf("断路器断开，跳过交易日历数据源 %s", name)
		return nil
	}
	cache, err := fn()
	if err != nil {
		cb.Failure()
		logger.SugaredLogger.Warnf("InitTradingCalendar: %s 获取交易日历失败: %v", name, err)
		return nil
	}
	if len(cache) == 0 {
		logger.SugaredLogger.Warnf("InitTradingCalendar: %s 返回空交易日历", name)
		return nil
	}
	cb.Success()
	return cache
}


// IsTradingDay 判断指定日期是否为交易日
// 优先查交易日历缓存，降级到周末判断
func IsTradingDay(dateStr string) bool {
	var t time.Time
	var err error

	if dateStr == "" {
		t = time.Now()
	} else {
		layouts := []string{"2006-01-02", "20060102"}
		for _, layout := range layouts {
			t, err = time.Parse(layout, dateStr)
			if err == nil {
				break
			}
		}
		if err != nil {
			t = time.Now()
		}
	}

	normalized := t.Format("2006-01-02")

	tradingDayCacheMu.RLock()
	if tradingDayInited && tradingDayCache != nil {
		isTrading, ok := tradingDayCache[normalized]
		tradingDayCacheMu.RUnlock()
		if ok {
			return isTrading
		}
		return false
	} else {
		tradingDayCacheMu.RUnlock()
	}

	// 降级：缓存未初始化，仅周末判断
	wd := t.Weekday()
	return wd != time.Saturday && wd != time.Sunday
}

// IsTodayTradingDay 判断今天是否为交易日
func IsTodayTradingDay() bool {
	return IsTradingDay("")
}

// GetLatestTradingDay 获取最近的交易日（向前查找）
func GetLatestTradingDay(dateStr string) string {
	var t time.Time
	var err error

	if dateStr == "" {
		t = time.Now()
	} else {
		layouts := []string{"2006-01-02", "20060102"}
		for _, layout := range layouts {
			t, err = time.Parse(layout, dateStr)
			if err == nil {
				break
			}
		}
		if err != nil {
			t = time.Now()
		}
	}

	for i := 0; i < 15; i++ {
		d := t.AddDate(0, 0, -i)
		if IsTradingDay(d.Format("2006-01-02")) {
			return d.Format("2006-01-02")
		}
	}

	if dateStr == "" {
		return time.Now().Format("2006-01-02")
	}
	return dateStr
}

// GetTradingDaysBetween 获取两个日期之间的所有交易日（含首尾）
func GetTradingDaysBetween(startDate, endDate string) []string {
	var start, end time.Time
	var err error

	layouts := []string{"2006-01-02", "20060102"}
	for _, layout := range layouts {
		start, err = time.Parse(layout, startDate)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil
	}
	for _, layout := range layouts {
		end, err = time.Parse(layout, endDate)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil
	}

	var days []string
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if IsTradingDay(d.Format("2006-01-02")) {
			days = append(days, d.Format("2006-01-02"))
		}
	}
	return days
}

// GetNextTradingDay 获取下一个交易日（向后查找）
func GetNextTradingDay(dateStr string) string {
	var t time.Time
	var err error

	if dateStr == "" {
		t = time.Now()
	} else {
		layouts := []string{"2006-01-02", "20060102"}
		for _, layout := range layouts {
			t, err = time.Parse(layout, dateStr)
			if err == nil {
				break
			}
		}
		if err != nil {
			t = time.Now()
		}
	}

	for i := 1; i <= 15; i++ {
		d := t.AddDate(0, 0, i)
		if IsTradingDay(d.Format("2006-01-02")) {
			return d.Format("2006-01-02")
		}
	}
	return t.AddDate(0, 0, 1).Format("2006-01-02")
}
