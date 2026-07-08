package tools

import (
	"encoding/json"
	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	"strings"

	bladesTools "github.com/go-kratos/blades/tools"
	"github.com/tidwall/gjson"
)

func GetAdataTools() []bladesTools.Tool {
	var result []bladesTools.Tool

	result = append(result, NewDataToolWrapper(
		"GetADataMarketCurrent",
		"通过 AData 数据源获取股票实时行情（最新价、涨跌幅、成交量等），支持多只股票。",
		map[string]*types.ParamDef{
			"codeList": {
				Type:     "string",
				Desc:     "股票代码列表，用逗号分隔，如 000001,600519",
				Required: true,
			},
		},
		func(args string) (string, error) {
			codes := gjson.Get(args, "codeList").String()
			client := data.NewAdataClient()
			result, err := client.GetMarketCurrent(strings.Split(codes, ","))
			if err != nil {
				return "", err
			}
			jsonData, _ := json.Marshal(result)
			md, _ := data.JSONToMarkdownTable(jsonData)
			return md, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetADataKLine",
		"通过 AData 数据源获取股票日K线数据。",
		map[string]*types.ParamDef{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如 000001 或 600519",
				Required: true,
			},
			"startDate": {
				Type:     "string",
				Desc:     "开始日期 YYYY-MM-DD",
				Required: false,
			},
			"endDate": {
				Type:     "string",
				Desc:     "结束日期 YYYY-MM-DD",
				Required: false,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			startDate := gjson.Get(args, "startDate").String()
			endDate := gjson.Get(args, "endDate").String()
			client := data.NewAdataClient()
			result, err := client.GetKLineData(stockCode, startDate, endDate)
			if err != nil {
				return "", err
			}
			jsonData, _ := json.Marshal(result)
			md, _ := data.JSONToMarkdownTable(jsonData)
			return md, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetADataTradeCalendar",
		"通过 AData 数据源获取交易日历。",
		map[string]*types.ParamDef{
			"year": {
				Type:     "integer",
				Desc:     "年份，如 2026",
				Required: true,
			},
		},
		func(args string) (string, error) {
			year := int(gjson.Get(args, "year").Int())
			_ = year // AData 返回全年日历，不需要年份参数
			client := data.NewAdataClient()
			result, err := client.GetTradeCalendar()
			if err != nil {
				return "", err
			}
			jsonData, _ := json.Marshal(result)
			md, _ := data.JSONToMarkdownTable(jsonData)
			return md, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetADataNorthFlow",
		"通过 AData 数据源获取北向资金流数据。",
		map[string]*types.ParamDef{},
		func(args string) (string, error) {
			client := data.NewAdataClient()
			result, err := client.GetNorthFlow()
			if err != nil {
				return "", err
			}
			jsonData, _ := json.Marshal(result)
			md, _ := data.JSONToMarkdownTable(jsonData)
			return md, nil
		},
	))

	return result
}
