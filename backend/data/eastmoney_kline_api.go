package data

import (
	"blades-stock/backend/datasource"
)

// NewEastMoneyKLineApi 转发到 datasource（R-25c：实现已搬入 datasource）。
// 超时与 BrowserPath 由 data 从设置传入；config 为 nil 时取默认值，
// 与原构造器存指针、方法内 nil 检查的行为一致。
// tools/services/data 内的全部 data.NewEastMoneyKLineApi(...) 调用点零改动。
func NewEastMoneyKLineApi(config *SettingConfig) *datasource.EastMoneyKLineApi {
	var timeout int64
	var browserPath string
	if config != nil {
		timeout = config.CrawlTimeOut
		browserPath = config.BrowserPath
	}
	return datasource.NewEastMoneyKLineApi(timeout, browserPath)
}

// 东方财富 K 线类型别名（R-25c：定义已搬入 datasource，
// tool_eastmoney_kline.go 的 EastMoneyKLineSection/WithMASection 参数类型零改动）。
type EastMoneyKLineApi = datasource.EastMoneyKLineApi
type KLineType = datasource.KLineType
type EastMoneyKLineResponse = datasource.EastMoneyKLineResponse
type CallAuctionData = datasource.CallAuctionData

// K 线类型常量别名（R-25c：定义已搬入 datasource，测试文件引用零改动）。
const (
	KLineType1Min     = datasource.KLineType1Min
	KLineType5Min     = datasource.KLineType5Min
	KLineType15Min    = datasource.KLineType15Min
	KLineType30Min    = datasource.KLineType30Min
	KLineType60Min    = datasource.KLineType60Min
	KLineType120Min   = datasource.KLineType120Min
	KLineTypeDay      = datasource.KLineTypeDay
	KLineTypeWeek     = datasource.KLineTypeWeek
	KLineTypeMonth    = datasource.KLineTypeMonth
	KLineTypeQuarter  = datasource.KLineTypeQuarter
	KLineTypeHalfYear = datasource.KLineTypeHalfYear
	KLineTypeYear     = datasource.KLineTypeYear
)
