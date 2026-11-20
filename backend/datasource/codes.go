package datasource

import (
	"strings"
)

// IsAStockCode 判断股票代码是否为A股（沪深京市场）。
// R-25b 自 backend/data/tool_eastmoney_kline.go 移入；data 侧留同名转发。
func IsAStockCode(code string) bool {
	return strings.HasSuffix(code, ".SZ") || strings.HasSuffix(code, ".SH") || strings.HasSuffix(code, ".BJ")
}

// IsHKStockCode 判断股票代码是否为港股。
// R-25b 自 backend/data/tool_eastmoney_kline.go 移入；data 侧留同名转发。
func IsHKStockCode(code string) bool {
	upper := strings.ToUpper(code)
	return strings.HasSuffix(upper, ".HK") || strings.HasPrefix(upper, "HK")
}

// IsUSStockCode 判断股票代码是否为美股。
// R-25b 自 backend/data/tool_eastmoney_kline.go 移入；data 侧留同名转发。
func IsUSStockCode(code string) bool {
	upper := strings.ToUpper(code)
	return strings.HasSuffix(upper, ".US") || strings.HasPrefix(upper, "US") || strings.HasPrefix(upper, "GB_")
}
