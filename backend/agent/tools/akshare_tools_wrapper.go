package tools

import (
	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	"blades-stock/backend/util"
	"fmt"

	bladesTools "github.com/go-kratos/blades/tools"
	"github.com/tidwall/gjson"
)

func GetAkshareTools() []bladesTools.Tool {
	var result []bladesTools.Tool

	result = append(result, NewDataToolWrapper(
		"GetFuturesSpot",
		"通过 AKShare 数据源获取期货行情数据，支持上期所(SHFE)、大商所(DCE)、郑商所(CZCE)、中金所(CFFEX)、广期所(GFEX)等。",
		map[string]*types.ParamDef{
			"market": {
				Type:     "string",
				Desc:     "期货交易所，如：中金所、上期所、大商所、郑商所、广期所",
				Required: true,
			},
		},
		func(args string) (string, error) {
			market := gjson.Get(args, "market").String()
			client := data.NewAkshareClient()
			result, err := client.FuturesSpot(market)
			if err != nil {
				return "", err
			}
			return util.MarkdownTableWithTitle(fmt.Sprintf("期货行情(%s)", market), result), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetETFSpot",
		"通过 AKShare 数据源获取ETF实时行情数据。",
		map[string]*types.ParamDef{},
		func(args string) (string, error) {
			client := data.NewAkshareClient()
			result, err := client.ETFSpot()
			if err != nil {
				return "", err
			}
			return util.MarkdownTableWithTitle("ETF实时行情", result), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetETFHistorical",
		"通过 AKShare 数据源获取ETF历史K线数据。",
		map[string]*types.ParamDef{
			"symbol": {
				Type:     "string",
				Desc:     "ETF代码，如 159915",
				Required: true,
			},
			"period": {
				Type:     "string",
				Desc:     "周期：daily/weekly/monthly",
				Required: false,
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
			"adjust": {
				Type:     "string",
				Desc:     "复权类型：qfq=前复权，hfq=后复权，空=不复权",
				Required: false,
			},
		},
		func(args string) (string, error) {
			symbol := gjson.Get(args, "symbol").String()
			period := gjson.Get(args, "period").String()
			startDate := gjson.Get(args, "startDate").String()
			endDate := gjson.Get(args, "endDate").String()
			adjust := gjson.Get(args, "adjust").String()
			if period == "" {
				period = "daily"
			}
			client := data.NewAkshareClient()
			result, err := client.ETFHistorical(symbol, period, startDate, endDate, adjust)
			if err != nil {
				return "", err
			}
			return util.MarkdownTableWithTitle(fmt.Sprintf("ETF历史K线(%s)", symbol), result), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetOpenFundNetValue",
		"通过 AKShare 数据源获取开放式基金净值数据。",
		map[string]*types.ParamDef{
			"symbol": {
				Type:     "string",
				Desc:     "基金代码",
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
			symbol := gjson.Get(args, "symbol").String()
			startDate := gjson.Get(args, "startDate").String()
			endDate := gjson.Get(args, "endDate").String()
			client := data.NewAkshareClient()
			result, err := client.OpenFundNetValue(symbol, startDate, endDate)
			if err != nil {
				return "", err
			}
			return util.MarkdownTableWithTitle(fmt.Sprintf("基金净值(%s)", symbol), result), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetBoardConcept",
		"通过 AKShare 数据源获取板块概念列表。",
		map[string]*types.ParamDef{},
		func(args string) (string, error) {
			client := data.NewAkshareClient()
			result, err := client.BoardConcept()
			if err != nil {
				return "", err
			}
			return util.MarkdownTableWithTitle("板块概念列表", result), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetBoardIndustry",
		"通过 AKShare 数据源获取行业板块列表。",
		map[string]*types.ParamDef{},
		func(args string) (string, error) {
			client := data.NewAkshareClient()
			result, err := client.BoardIndustry()
			if err != nil {
				return "", err
			}
			return util.MarkdownTableWithTitle("行业板块列表", result), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetMacroCPI",
		"通过 AKShare 数据源获取中国CPI（居民消费价格指数）数据。",
		map[string]*types.ParamDef{},
		func(args string) (string, error) {
			client := data.NewAkshareClient()
			result, err := client.MacroCPI()
			if err != nil {
				return "", err
			}
			return util.MarkdownTableWithTitle("CPI数据", result), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetMacroGDP",
		"通过 AKShare 数据源获取中国GDP数据。",
		map[string]*types.ParamDef{},
		func(args string) (string, error) {
			client := data.NewAkshareClient()
			result, err := client.MacroGDP()
			if err != nil {
				return "", err
			}
			return util.MarkdownTableWithTitle("GDP数据", result), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetMacroPMI",
		"通过 AKShare 数据源获取中国PMI（采购经理人指数）数据。",
		map[string]*types.ParamDef{},
		func(args string) (string, error) {
			client := data.NewAkshareClient()
			result, err := client.MacroPMI()
			if err != nil {
				return "", err
			}
			return util.MarkdownTableWithTitle("PMI数据", result), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetMacroMoneySupply",
		"通过 AKShare 数据源获取中国货币供应量（M2等）数据。",
		map[string]*types.ParamDef{},
		func(args string) (string, error) {
			client := data.NewAkshareClient()
			result, err := client.MacroMoneySupply()
			if err != nil {
				return "", err
			}
			return util.MarkdownTableWithTitle("货币供应量数据", result), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetLongHuToday",
		"通过 AKShare 数据源获取今日龙虎榜数据。",
		map[string]*types.ParamDef{},
		func(args string) (string, error) {
			client := data.NewAkshareClient()
			result, err := client.LongHuToday()
			if err != nil {
				return "", err
			}
			return util.MarkdownTableWithTitle("龙虎榜数据", result), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetConvertibleBondSpot",
		"通过 AKShare 数据源获取可转债行情数据。",
		map[string]*types.ParamDef{},
		func(args string) (string, error) {
			client := data.NewAkshareClient()
			result, err := client.ConvertibleBondSpot()
			if err != nil {
				return "", err
			}
			return util.MarkdownTableWithTitle("可转债行情", result), nil
		},
	))

	return result
}
