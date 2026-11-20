package data

import (
	"fmt"
	"strings"

	"blades-stock/backend/datasource"
)

// InitTradingCalendar 初始化交易日历缓存（R-24e：实现主体已搬入 datasource，
// 本函数只负责解析服务地址并注入两个数据源的取数闭包——AData 客户端与
// 设置表仍在 data 包，datasource 不碰它们；main.go / app_settings.go 的
// data.InitTradingCalendar() 调用点零改动）。
func InitTradingCalendar() {
	// 优先从 AData 获取，失败则尝试 Baostock，最终降级到周末判断
	adataUrl := ""
	if cfg := GetSettingConfig(); cfg != nil && cfg.Settings != nil {
		adataUrl = strings.TrimSpace(cfg.Settings.ADataServiceUrl)
	}
	if adataUrl == "" {
		adataUrl = ADataServiceURL()
	}

	// 若指向已知离线服务器 115.190.72.22，直接跳过，避免网络超时挂起
	var adataDays func() ([]string, error)
	if !strings.Contains(adataUrl, "115.190.72.22") {
		adataDays = func() ([]string, error) {
			adataClient := NewAdataClient()
			return adataClient.GetTradeCalendarDays()
		}
	}

	baostockDays := func(year int) ([]string, error) {
		api := NewBaostockApi()
		resp, err := api.GetTradeCalendarYear(year)
		if err != nil {
			return nil, err
		}
		if resp == nil {
			return nil, fmt.Errorf("baostock 返回空交易日历")
		}
		return resp.TradingDays, nil
	}

	datasource.InitTradingCalendar(adataDays, baostockDays)
}

// IsTradingDay 判断指定日期是否为交易日（转发到 datasource；缓存未初始化时降级周末判断）。
func IsTradingDay(dateStr string) bool {
	return datasource.IsTradingDay(dateStr)
}

// IsTodayTradingDay 判断今天是否为交易日。
func IsTodayTradingDay() bool {
	return datasource.IsTodayTradingDay()
}

// GetLatestTradingDay 获取最近的交易日（向前查找）。
func GetLatestTradingDay(dateStr string) string {
	return datasource.GetLatestTradingDay(dateStr)
}

// GetTradingDaysBetween 获取两个日期之间的所有交易日（含首尾）。
func GetTradingDaysBetween(startDate, endDate string) []string {
	return datasource.GetTradingDaysBetween(startDate, endDate)
}

// GetNextTradingDay 获取下一个交易日（向后查找）。
func GetNextTradingDay(dateStr string) string {
	return datasource.GetNextTradingDay(dateStr)
}
