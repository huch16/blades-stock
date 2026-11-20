package data

import (
	"blades-stock/backend/datasource"
)

// NewBaostockApi 按环境变量解析服务地址后构造客户端
// （R-24c：BaostockApi 本体已搬入 datasource，本函数只做 data 侧转发，
// trading_day.go 的 NewBaostockApi() 调用点零改动）。
func NewBaostockApi() *datasource.BaostockApi {
	return datasource.NewBaostockApi(BaostockServiceURL())
}
