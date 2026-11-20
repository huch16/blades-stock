package data

import (
	"net/url"
	"time"

	"blades-stock/backend/datasource"

	"github.com/go-resty/resty/v2"
)

// SharedHTTPClient 是 datasource.SharedHTTPClient 的同一指针转发：
// 根包、cmd/server、agent/tools、services 等不得直接 import datasource（规则 3），
// 一律经 data 包使用；SetTimeout 等原地修改作用于同一共享客户端。
var SharedHTTPClient = datasource.SharedHTTPClient

// ConfigureFromSettings 按设置同步全局代理与超时（R-24a：实现主体已搬入 datasource，
// 本函数只做 data 侧转发——SettingConfig 是 data 包类型，datasource 不能引用它）。
func ConfigureFromSettings(config *SettingConfig) {
	if config == nil {
		return
	}

	if config.HttpProxyEnabled && config.HttpProxy != "" {
		datasource.UpdateHTTPClientProxy(config.HttpProxy)
	} else {
		datasource.DisableHTTPClientProxy()
	}

	if config.CrawlTimeOut > 0 {
		datasource.UpdateHTTPClientTimeout(time.Duration(config.CrawlTimeOut) * time.Second)
	} else {
		datasource.UpdateHTTPClientTimeout(300 * time.Second)
	}
}

// CreateHTTPClientWithTimeout 转发到 datasource（services 经 data 调用）。
func CreateHTTPClientWithTimeout(timeout time.Duration) *resty.Client {
	return datasource.CreateHTTPClientWithTimeout(timeout)
}

// parseProxyURL 转发到 datasource（openai_tools.go 的 createHTTPClientWithProxy 在用）。
func parseProxyURL(proxyURL string) *url.URL {
	return datasource.ParseProxyURL(proxyURL)
}
