package data

import (
	"time"
)

// IsTradingDay 判断指定日期是否为交易日
// 当前实现：简单判断周末（周六、周日）
// TODO: 接入 tushare trade_cal 或 AKShare 节假日 API 精确判断
func IsTradingDay(dateStr string) bool {
	var t time.Time
	var err error

	// 尝试解析日期字符串
	if dateStr == "" {
		t = time.Now()
	} else {
		// 支持多种格式: 2006-01-02, 20060102
		layouts := []string{"2006-01-02", "20060102"}
		for _, layout := range layouts {
			t, err = time.Parse(layout, dateStr)
			if err == nil {
				break
			}
		}
		if err != nil {
			// 解析失败，默认按今天处理
			t = time.Now()
		}
	}

	// 简单判断：周末非交易日
	wd := t.Weekday()
	if wd == time.Saturday || wd == time.Sunday {
		return false
	}

	// TODO: 调用节假日 API 判断法定节假日
	// 示例: https://timor.tech/api/holiday/info/2026-01-01
	// 或 tushare trade_cal API

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

	// 向前查找最多 7 天
	for i := 0; i < 7; i++ {
		if IsTradingDay(t.Format("2006-01-02")) {
			return t.Format("2006-01-02")
		}
		t = t.AddDate(0, 0, -1)
	}

	// 兜底：返回原日期
	if dateStr == "" {
		return time.Now().Format("2006-01-02")
	}
	return dateStr
}
