package data

import (
	"context"
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

// crawlTimeoutContext 返回带爬取超时的请求上下文，供各 K 线方法对共享
// SharedHTTPClient 发请求时使用：原来每次请求调 client.SetTimeout 会改写
// 共享 http.Client.Timeout，并发拉取时既是数据竞态（-race 可复现）、又会
// 让某次请求用上别人的超时值。改为请求级 context 超时后，共享客户端本身
// 不再被改动，超时语义不变（timeoutSec<=0 时与原来 SetTimeout(0) 一致：
// 不限时，由共享客户端自身的 300s 上限兜底）。
func crawlTimeoutContext(timeoutSec int64) (context.Context, context.CancelFunc) {
	if timeoutSec <= 0 {
		return context.Background(), func() {}
	}
	return context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
}

// CreateHTTPClientWithTimeout 转发到 datasource（services 经 data 调用）。
func CreateHTTPClientWithTimeout(timeout time.Duration) *resty.Client {
	return datasource.CreateHTTPClientWithTimeout(timeout)
}
