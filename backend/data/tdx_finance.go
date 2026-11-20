package data

import (
	"fmt"
)

func (t *TdxKLineApi) GetFinanceInfo(stockCode string) *TdxFinanceInfo {
	bundle := t.GetF10Data(stockCode)
	if bundle == nil || bundle.Finance == nil {
		return nil
	}
	return bundle.Finance
}

func (t *TdxKLineApi) GetXDXRInfo(stockCode string) *[]TdxXDXRItem {
	bundle := t.GetF10Data(stockCode)
	if bundle == nil {
		return &[]TdxXDXRItem{}
	}
	return &bundle.XDXR
}

func tdxDateToString(d uint32) string {
	if d == 0 {
		return ""
	}
	year := int(d / 10000)
	month := int((d % 10000) / 100)
	day := int(d % 100)
	if year < 1900 || month < 1 || month > 12 || day < 1 || day > 31 {
		return fmt.Sprintf("%d", d)
	}
	return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
}
