package data

import (
	"blades-stock/backend/datasource"
)

// NewTdxKLineApi 转发到 datasource（R-25b：实现已搬入 datasource）。
// 超时由 data 从设置注入：提供函数每次新建连接时读最新设置，
// 与原 newClient 每次调 GetSettingConfig() 的行为一致。
// tools/services/data 内的全部 data.NewTdxKLineApi() 调用点零改动。
func NewTdxKLineApi() *datasource.TdxKLineApi {
	return datasource.NewTdxKLineApi(func() int64 {
		return GetSettingConfig().CrawlTimeOut
	})
}

// TDX 导出类型别名（R-25b：定义已搬入 datasource，
// services/根包的 data.Tdx... 引用零改动）。
type TdxKLineApi = datasource.TdxKLineApi
type TdxCallAuctionData = datasource.TdxCallAuctionData
type TdxCompanyInfoSection = datasource.TdxCompanyInfoSection
type TdxFinanceInfo = datasource.TdxFinanceInfo
type TdxXDXRItem = datasource.TdxXDXRItem
type TdxCompanyInfoBundle = datasource.TdxCompanyInfoBundle
type TdxCompanyCategory = datasource.TdxCompanyCategory
type MACBelongBoardItem = datasource.MACBelongBoardItem
type TdxStockBasic = datasource.TdxStockBasic
type TdxExStockBasic = datasource.TdxExStockBasic
