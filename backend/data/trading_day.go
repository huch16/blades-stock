package data

import (
	"sync"
	"time"

	"blades-stock/backend/logger"
)

// ============================================================================
// 交易日历 — 支持 Baostock 精确交易日历（含法定节假日）
// 启动时从 Baostock 获取全年交易日历，缓存到内存。
// 降级策略：Baostock 不可用时回退到周末判断。
// ============================================================================

var (
	tradingDayCache   map[string]bool // dateStr("2006-01-02") → true=交易日
	tradingDayCacheMu sync.RWMutex
	tradingDayInited  bool
)

// InitTradingCalendar 初始化交易日历缓存
// 从 Baostock 获取当年交易日历，失败则降级到周末判断
func InitTradingCalendar() {
	year := time.Now().Year()
	api := NewBaostockApi()

	resp, err := api.GetTradeCalendarYear(year)
	if err != nil {
		logger.SugaredLogger.Warnf("InitTradingCalendar: baostock 获取交易日历失败，降级到周末判断: %v", err)
		tradingDayCache = nil
		tradingDayInited = false
		return
	}

	cache := make(map[string]bool, len(resp.TradingDays))
	for _, day := range resp.TradingDays {
		// baostock 返回格式 "2026-07-03"
		cache[day] = true
	}

	// 同时获取下一年的1月（避免年底跨年问题）
	nextYear := year + 1
	if resp2, err := api.GetTradeCalendarYear(nextYear); err == nil {
		for _, day := range resp2.TradingDays {
			// 只缓存1月的数据
			if len(day) >= 7 && day[:4] == time.Date(nextYear, 1, 1, 0, 0, 0, 0, time.Local).Format("2006")[:4] {
				cache[day] = true
			}
		}
	}

	tradingDayCacheMu.Lock()
	tradingDayCache = cache
	tradingDayInited = true
	tradingDayCacheMu.Unlock()

	logger.SugaredLogger.Infof("InitTradingCalendar: 已加载 %d 年交易日历，共 %d 个交易日", year, resp.TradingCount)
}

// IsTradingDay 判断指定日期是否为交易日
// 优先查 Baostock 交易日历缓存，降级到周末判断
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

	// 统一日期格式
	normalized := t.Format("2006-01-02")

	// 尝试从缓存查询
	tradingDayCacheMu.RLock()
	if tradingDayInited && tradingDayCache != nil {
		isTrading, ok := tradingDayCache[normalized]
		tradingDayCacheMu.RUnlock()
		if ok {
			return isTrading // 缓存命中（交易日=true，非交易日不在map中所以ok=false）
		}
		// 缓存已初始化但该日期不在交易日列表中 → 非交易日（节假日/周末）
		return false
	} else {
		tradingDayCacheMu.RUnlock()
	}

	// 降级：缓存未初始化，仅周末判断
	wd := t.Weekday()
	if wd == time.Saturday || wd == time.Sunday {
		return false
	}
	return true
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

	// 向前查找最多 15 天（覆盖春节+国庆长假）
	for i := 0; i < 15; i++ {
		d := t.AddDate(0, 0, -i)
		if IsTradingDay(d.Format("2006-01-02")) {
			return d.Format("2006-01-02")
		}
	}

	// 兜底：返回原日期
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
