package tools

import (
	"encoding/json"
	"fmt"

	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	bladesTools "github.com/go-kratos/blades/tools"
)

// GetAkshareTools returns all AKShare-related tools.
func GetAkshareTools() []bladesTools.Tool {
	var result []bladesTools.Tool

	// ── 商品期货 ──────────────────────────────────
	result = append(result, NewDataToolWrapper(
		"GetAkshareFuturesSpot",
		"获取商品期货实时行情（现货价、主力合约、近月合约、基差等）。覆盖SHFE(上期所)/DCE(大商所)/CZCE(郑商所)/GFEX(广期所)/INE。不支持CFFEX。",
		map[string]*types.ParamDef{
			"market": {
				Type:     "string",
				Desc:     "交易所代码：SHFE(上期所)/DCE(大商所)/CZCE(郑商所)/GFEX(广期所)/INE(上海国际能源交易中心)。不支持CFFEX",
				Required: true,
			},
		},
		func(args string) (string, error) {
			var params struct {
				Market string `json:"market"`
			}
			if err := json.Unmarshal([]byte(args), &params); err != nil {
				return "", fmt.Errorf("参数解析失败: %w", err)
			}
			client := data.NewAkshareClient()
			data, err := client.FuturesSpot(params.Market)
			if err != nil {
				return "", err
			}
			jsonData, _ := json.Marshal(data)
			return string(jsonData), nil
		},
	))

	// ── ETF 实时行情 ──────────────────────────────
	result = append(result, NewDataToolWrapper(
		"GetAkshareETFSpot",
		"获取 ETF 实时行情（含折溢价率）。",
		map[string]*types.ParamDef{},
		func(args string) (string, error) {
			client := data.NewAkshareClient()
			data, err := client.ETFSpot()
			if err != nil {
				return "", err
			}
			jsonData, _ := json.Marshal(data)
			return string(jsonData), nil
		},
	))

	// ── 板块/概念 ─────────────────────────────────
	result = append(result, NewDataToolWrapper(
		"GetAkshareBoardConcept",
		"获取 A 股概念板块列表（含板块代码、名称、成分股数量等）。",
		map[string]*types.ParamDef{},
		func(args string) (string, error) {
			client := data.NewAkshareClient()
			data, err := client.BoardConcept()
			if err != nil {
				return "", err
			}
			jsonData, _ := json.Marshal(data)
			return string(jsonData), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetAkshareBoardIndustry",
		"获取 A 股行业板块列表（含板块代码、名称、成分股数量等）。",
		map[string]*types.ParamDef{},
		func(args string) (string, error) {
			client := data.NewAkshareClient()
			data, err := client.BoardIndustry()
			if err != nil {
				return "", err
			}
			jsonData, _ := json.Marshal(data)
			return string(jsonData), nil
		},
	))

	// ── 宏观经济 ──────────────────────────────────
	result = append(result, NewDataToolWrapper(
		"GetAkshareMacroCPI",
		"获取消费者物价指数（CPI）数据。",
		map[string]*types.ParamDef{},
		func(args string) (string, error) {
			client := data.NewAkshareClient()
			data, err := client.MacroCPI()
			if err != nil {
				return "", err
			}
			jsonData, _ := json.Marshal(data)
			return string(jsonData), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetAkshareMacroGDP",
		"获取国内生产总值（GDP）数据。",
		map[string]*types.ParamDef{},
		func(args string) (string, error) {
			client := data.NewAkshareClient()
			data, err := client.MacroGDP()
			if err != nil {
				return "", err
			}
			jsonData, _ := json.Marshal(data)
			return string(jsonData), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetAkshareMacroPMI",
		"获取采购经理人指数（PMI）数据。",
		map[string]*types.ParamDef{},
		func(args string) (string, error) {
			client := data.NewAkshareClient()
			data, err := client.MacroPMI()
			if err != nil {
				return "", err
			}
			jsonData, _ := json.Marshal(data)
			return string(jsonData), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetAkshareMacroMoneySupply",
		"获取货币供应数据（M2/M1/M0）。",
		map[string]*types.ParamDef{},
		func(args string) (string, error) {
			client := data.NewAkshareClient()
			data, err := client.MacroMoneySupply()
			if err != nil {
				return "", err
			}
			jsonData, _ := json.Marshal(data)
			return string(jsonData), nil
		},
	))

	// ── 龙虎榜 ────────────────────────────────────
	result = append(result, NewDataToolWrapper(
		"GetAkshareLongHuToday",
		"获取今日龙虎榜数据（异动股票、上榜原因、买卖双方营业部等）。",
		map[string]*types.ParamDef{},
		func(args string) (string, error) {
			client := data.NewAkshareClient()
			data, err := client.LongHuToday()
			if err != nil {
				return "", err
			}
			jsonData, _ := json.Marshal(data)
			return string(jsonData), nil
		},
	))

	// ── 可转债 ────────────────────────────────────
	result = append(result, NewDataToolWrapper(
		"GetAkshareConvertibleBondSpot",
		"获取可转债实时行情（含价格、溢价率、换手率等）。",
		map[string]*types.ParamDef{},
		func(args string) (string, error) {
			client := data.NewAkshareClient()
			data, err := client.ConvertibleBondSpot()
			if err != nil {
				return "", err
			}
			jsonData, _ := json.Marshal(data)
			return string(jsonData), nil
		},
	))

	// ── 通用调用 ──────────────────────────────────
	result = append(result, NewDataToolWrapper(
		"CallAkshareFunction",
		"通用 AKShare 函数调用。可以调用任意 AKShare 支持的接口（通过 AKShare Python 包）。常用函数名：stock_zh_a_spot_em（A股实时行情）、stock_hk_spot_em（港股实时行情）、bond_cb_jsl（可转债实时行情）、fund_etf_hist_em（ETF历史行情）等。",
		map[string]*types.ParamDef{
			"function": {
				Type:     "string",
				Desc:     "AKShare 函数名，如 stock_zh_a_spot_em、bond_cb_jsl 等",
				Required: true,
			},
			"kwargs": {
				Type:     "object",
				Desc:     "可选参数字典（JSON 对象），如 {\"symbol\": \"600000\", \"start_date\": \"20240101\"}",
				Required: false,
			},
		},
		func(args string) (string, error) {
			var params struct {
				Function string                 `json:"function"`
				Kwargs   map[string]interface{} `json:"kwargs"`
			}
			if err := json.Unmarshal([]byte(args), &params); err != nil {
				return "", fmt.Errorf("参数解析失败: %w", err)
			}
			client := data.NewAkshareClient()
			data, err := client.Call(params.Function, params.Kwargs)
			if err != nil {
				return "", err
			}
			jsonData, _ := json.Marshal(data)
			return string(jsonData), nil
		},
	))

	return result
}
