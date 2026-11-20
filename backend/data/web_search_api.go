package data

import (
	"blades-stock/backend/datasource"
)

// NewWebSearchApi 转发到 datasource（R-25g：实现已搬入 datasource）。
// 三个设置提供函数由 data 注入，每次搜索时读最新设置；
// tools/services 的全部 data.NewWebSearchApi(30) 调用点零改动。
func NewWebSearchApi(timeout int) *datasource.WebSearchApi {
	return datasource.NewWebSearchApi(timeout,
		func() bool { return GetSettingConfig().DisableBrowserSearch },
		func() string { return GetSettingConfig().BrowserPath },
		func() int64 { return GetSettingConfig().CrawlTimeOut })
}

// GetBrowserManager 转发到 datasource（R-25g；集成测试在用）。
func GetBrowserManager() *datasource.BrowserManager {
	return datasource.GetBrowserManager()
}

// 网络搜索类型别名（R-25g：定义已搬入 datasource）。
type WebSearchApi = datasource.WebSearchApi
type WebSearchResult = datasource.WebSearchResult
type BrowserManager = datasource.BrowserManager
type BrowserInstance = datasource.BrowserInstance
