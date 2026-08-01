package data

import (
	"os"
	"strings"
)

// 服务地址默认值。优先从环境变量读取（BLADES_STOCK_*），否则使用默认值。
const (
	defaultStaticDataServiceURL = "http://8.134.249.145:18080"
	defaultADataServiceURL      = "http://115.190.72.22:8000"
	defaultAkshareServiceURL    = "http://115.190.72.22:8089"
	defaultBaostockServiceURL   = "http://115.190.72.22:8090"
)

// getEnvOr returns the environment variable value if set and non-empty,
// otherwise the given default.
func getEnvOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

// StaticDataServiceURL 静态数据服务地址（A股/港股/美股基础信息 JSON）。
// 可用环境变量 BLADES_STOCK_STATIC_DATA_URL 覆盖。
func StaticDataServiceURL() string {
	return getEnvOr("BLADES_STOCK_STATIC_DATA_URL", defaultStaticDataServiceURL)
}

// ADataServiceURL AData 数据服务地址。
// 可用环境变量 BLADES_STOCK_ADATA_URL 覆盖。
func ADataServiceURL() string {
	return getEnvOr("BLADES_STOCK_ADATA_URL", defaultADataServiceURL)
}

// AkshareServiceURL AKShare 数据服务地址。
// 可用环境变量 BLADES_STOCK_AKSHARE_URL 覆盖。
func AkshareServiceURL() string {
	return getEnvOr("BLADES_STOCK_AKSHARE_URL", defaultAkshareServiceURL)
}

// BaostockServiceURL Baostock 数据服务地址。
// 可用环境变量 BLADES_STOCK_BAOSTOCK_URL 覆盖。
func BaostockServiceURL() string {
	return getEnvOr("BLADES_STOCK_BAOSTOCK_URL", defaultBaostockServiceURL)
}
