package data

import (
	"blades-stock/backend/datasource"
)

// NewWallstreetcnApi 转发到 datasource（R-25e：实现已搬入 datasource）。
// 本客户端无任何设置依赖，构造器无参数；
// tools/data 的全部 data.NewWallstreetcnApi() 调用点零改动。
func NewWallstreetcnApi() *datasource.WallstreetcnApi {
	return datasource.NewWallstreetcnApi()
}

// 华尔街见闻客户端类型别名（R-25e：定义已搬入 datasource）。
type WallstreetcnApi = datasource.WallstreetcnApi
type WSCNLivesRequest = datasource.WSCNLivesRequest
type WSCNLiveItem = datasource.WSCNLiveItem
type WSCNLivesResponse = datasource.WSCNLivesResponse
type WSCNMarketRealResponse = datasource.WSCNMarketRealResponse
type WSCNKlineResponse = datasource.WSCNKlineResponse
type WSCNCalendarResponse = datasource.WSCNCalendarResponse
type WSCNCalendarItem = datasource.WSCNCalendarItem
