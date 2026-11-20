package data

import (
	"strings"

	"blades-stock/backend/datasource"
)

// NewEmAPI 转发到 datasource（R-25d：实现已搬入 datasource）。
// API 密钥由 data 从设置读取后传入；tools/data 的全部
// data.NewEmAPI() 调用点零改动。
func NewEmAPI() *datasource.EmAPI {
	var key string
	if cfg := GetSettingConfig(); cfg != nil && cfg.Settings != nil {
		key = strings.TrimSpace(cfg.Settings.EmApiKey)
	}
	return datasource.NewEmAPI(key)
}

// EmAPI 类型别名（R-25d：定义已搬入 datasource）。
type EmAPI = datasource.EmAPI
