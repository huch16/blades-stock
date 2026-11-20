package data

import (
	"blades-stock/backend/datasource"
)

// NewTickFlowClient 从设置读取 TickFlow API 密钥后构造客户端
// （R-24d：TickFlowClient 本体已搬入 datasource，本函数只做 data 侧转发，
// agent/tools 的 5 处 data.NewTickFlowClient() 调用点零改动）。
func NewTickFlowClient() *datasource.TickFlowClient {
	cfg := GetSettingConfig()
	return datasource.NewTickFlowClient(cfg.Settings.TickFlowAPIKey)
}
