package data

import (
	"blades-stock/backend/db"

	"github.com/duke-git/lancet/v2/strutil"
)

func checkIsIndexBasic(stock string) bool {
	count := int64(0)
	db.Dao.Model(&IndexBasic{}).Where("name =  ?", stock).Count(&count)
	return count > 0
}

func GetFinancialReportsByXUEQIU(stockCode string, crawlTimeOut int64) *[]string {
	var data string
	if strutil.HasPrefixAny(stockCode, []string{"sh", "sz", "SH", "SZ", "bj", "BJ"}) {
		data = NewStockDataApi().GetStockQtrMainFinanceToMarkdown(stockCode)
	}
	if data == "" {
		data = NewStockDataApi().GetStockLatestFinanceToMarkdown(stockCode)
	}
	if data == "" {
		return &[]string{}
	}
	return &[]string{"\n## 财务数据（东方财富）：\n" + data}
}
