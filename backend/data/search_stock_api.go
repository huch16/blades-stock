package data

import (
	"blades-stock/backend/datasource"
)

// NewSearchStockApi 转发到 datasource（R-25f：实现已搬入 datasource）。
// 东财用户标识 qgqpBId 由 data 从设置传入；
// tools/data 的全部 data.NewSearchStockApi(...) 调用点零改动。
func NewSearchStockApi(words string) *datasource.SearchStockApi {
	return datasource.NewSearchStockApi(words, NewSettingsApi().Config.QgqpBId)
}

// SearchStockApi 类型别名（R-25f：定义已搬入 datasource）。
type SearchStockApi = datasource.SearchStockApi
