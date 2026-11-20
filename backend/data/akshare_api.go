package data

import (
	"strings"

	"blades-stock/backend/datasource"
)

// NewAkshareClient 按“DB 设置 → 环境变量/默认值”解析服务地址后构造客户端
// （R-24b：AkshareClient 本体已搬入 datasource，本函数只做 data 侧转发，
// app_settings.go 的 data.NewAkshareClient().Ping() 调用点零改动）。
func NewAkshareClient() *datasource.AkshareClient {
	u := ""
	if cfg := GetSettingConfig(); cfg != nil && cfg.Settings != nil {
		u = strings.TrimSpace(cfg.Settings.AkshareServiceUrl)
	}
	if u == "" {
		u = AkshareServiceURL()
	}
	return datasource.NewAkshareClient(u)
}
