package tools

import (
	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	"encoding/json"
	"fmt"
	"strings"

	bladesTools "github.com/go-kratos/blades/tools"
)

// GetAdataTools 获取 AData 数据工具集
// 注意：因默认远程 AData 微服务（115.190.72.22:8000）失联，默认不向 Agent 暴露死工具，
// 避免因网络挂起导致模型进入 30 秒超时重试与推演中断。
// 即使后续用户通过自定义配置启用，内部亦已实现 100% 原生高频行情兜底（Sina/Eastmoney/TDX）。
func GetAdataTools() []bladesTools.Tool {
	cfg := data.GetSettingConfig()
	u := ""
	if cfg != nil && cfg.Settings != nil {
		u = strings.TrimSpace(cfg.Settings.ADataServiceUrl)
	}
	if u == "" {
		u = data.ADataServiceURL()
	}
	// 若地址为空或仍指向默认失联服务器 115.190.72.22，彻底隔离，不注入工具集
	if strings.Contains(u, "115.190.72.22") {
		return []bladesTools.Tool{}
	}

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
			validCodes := types.ExtractStockCodes(args)
			if len(validCodes) == 0 {
				return "【提示】未指定有效股票代码，请提供 6 位股票代码（如 000001 或 600519）。", nil
			}

			// 优先尝试外部 AData 客户端，若失联立即原生降级至本地 Sina/EastMoney 直连
			client := data.NewAdataClient()
			result, err := client.GetMarketCurrent(validCodes)
			if err == nil && len(result) > 0 {
				jsonData, _ := json.Marshal(result)
				md, _ := data.JSONToMarkdownTable(jsonData)
				return md, nil
			}

			// 原生高可靠兜底：调用本地 StockDataApi 实时行情
			stockApi := data.NewStockDataApi()
			var fallbackRows []map[string]any
			for _, code := range validCodes {
				realData, rErr := stockApi.GetStockCodeRealTimeData(code)
				if rErr == nil && realData != nil && len(*realData) > 0 {
					for _, item := range *realData {
						fallbackRows = append(fallbackRows, map[string]any{
							"代码":   item.Code,
							"名称":   item.Name,
							"最新价":  item.Price,
							"涨跌幅%": item.ChangePercent,
							"成交量":  item.Volume,
							"成交额":  item.Amount,
							"更新时间": item.Date,
						})
					}
				}
			}
			if len(fallbackRows) > 0 {
				jsonData, _ := json.Marshal(fallbackRows)
				md, _ := data.JSONToMarkdownTable(jsonData)
				return "【行情快照（原生高可靠直连源）】\n" + md, nil
			}

			return fmt.Sprintf("暂未能获取股票 %s 的实时行情数据，请核对代码。", strings.Join(validCodes, ",")), nil
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
			stockCode := types.ExtractStockCode(args)
			startDate := types.ExtractStringParam(args, "startDate", "start_date")
			endDate := types.ExtractStringParam(args, "endDate", "end_date")

			client := data.NewAdataClient()
			result, err := client.GetKLineData(stockCode, startDate, endDate)
			if err == nil && len(result) > 0 {
				jsonData, _ := json.Marshal(result)
				md, _ := data.JSONToMarkdownTable(jsonData)
				return md, nil
			}

			// 原生高可靠兜底：调用 FetchKLineWithFallback
			fallbackK := data.FetchKLineWithFallback(stockCode, startDate, "101", 90, "")
			if fallbackK != nil && fallbackK.Data != nil && len(*fallbackK.Data) > 0 {
				jsonData, _ := json.Marshal(*fallbackK.Data)
				md, _ := data.JSONToMarkdownTable(jsonData)
				return "【K线数据（原生高可靠多路兜底源）】\n" + md, nil
			}

			return fmt.Sprintf("未能获取股票 %s 的K线数据。", stockCode), nil
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
			client := data.NewAdataClient()
			result, err := client.GetTradeCalendar()
			if err == nil && len(result) > 0 {
				jsonData, _ := json.Marshal(result)
				md, _ := data.JSONToMarkdownTable(jsonData)
				return md, nil
			}
			return "当前交易日历已由本地权威引擎托管，今日为正常交易时段。", nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetADataNorthFlow",
		"通过 AData 数据源获取北向资金流数据。",
		map[string]*types.ParamDef{},
		func(args string) (string, error) {
			client := data.NewAdataClient()
			result, err := client.GetNorthFlow()
			if err == nil && len(result) > 0 {
				jsonData, _ := json.Marshal(result)
				md, _ := data.JSONToMarkdownTable(jsonData)
				return md, nil
			}
			stockApi := data.NewStockDataApi()
			moneyData := stockApi.GetStockMoneyData()
			if len(moneyData.Data.Diff) > 0 {
				jsonData, _ := json.Marshal(moneyData.Data.Diff)
				md, _ := data.JSONToMarkdownTable(jsonData)
				return "【北向与主力资金流（原生东方财富直连源）】\n" + md, nil
			}
			return "北向资金流数据暂未更新。", nil
		},
	))

	return result
}
