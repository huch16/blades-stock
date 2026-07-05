package tools

import (
	"fmt"
	"strings"

	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	bladesTools "github.com/go-kratos/blades/tools"
	"github.com/tidwall/gjson"
)

// GetTickFlowToolWrappers returns TickFlow tool schemas.
func GetTickFlowToolWrappers() []bladesTools.Tool {
	var result []bladesTools.Tool

	// TickFlowKlines
	result = append(result, NewDataToolWrapper(
		"TickFlowKlines",
		"获取个股/指数历史K线数据（TickFlow API）。支持A股、港股、美股、指数。返回Markdown表格，包含开盘/最高/最低/收盘/成交量/成交额。当需要用结构化API获取K线（而非爬虫）时使用。",
		map[string]*types.ParamDef{
			"symbol": {
				Type:     "string",
				Desc:     "标的代码，如 600000.SH, 000001.SZ, ^SPX（标普500）",
				Required: true,
			},
			"period": {
				Type:     "string",
				Desc:     "周期：1m/5m/15m/30m/1h/1d/1w/1M，默认1d",
				Required: false,
			},
			"count": {
				Type:     "integer",
				Desc:     "K线数量，默认200",
				Required: false,
			},
			"adjust": {
				Type:     "string",
				Desc:     "复权：qfq（前复权）/hfq（后复权）/空（不复权），默认空",
				Required: false,
			},
		},
		func(args string) (string, error) {
			symbol := gjson.Get(args, "symbol").String()
			period := gjson.Get(args, "period").String()
			count := int(gjson.Get(args, "count").Int())
			adjust := gjson.Get(args, "adjust").String()
			if symbol == "" {
				return "请提供symbol参数", nil
			}
			client := data.NewTickFlowClient()
			return client.GetKlinesAsMarkdown(symbol, period, count, adjust), nil
		},
	))

	// TickFlowKlinesBatch
	result = append(result, NewDataToolWrapper(
		"TickFlowKlinesBatch",
		"批量获取多只标的K线数据（TickFlow API）。适合对比多只股票走势。返回各标的最近N根K线的Markdown表格。",
		map[string]*types.ParamDef{
			"symbols": {
				Type:     "string",
				Desc:     "标的代码列表，逗号分隔，如 600000.SH,600519.SH",
				Required: true,
			},
			"period": {
				Type:     "string",
				Desc:     "周期，默认1d",
				Required: false,
			},
			"count": {
				Type:     "integer",
				Desc:     "每只标的K线数量，默认100",
				Required: false,
			},
		},
		func(args string) (string, error) {
			symbolsStr := gjson.Get(args, "symbols").String()
			period := gjson.Get(args, "period").String()
			count := int(gjson.Get(args, "count").Int())
			if symbolsStr == "" {
				return "请提供symbols参数", nil
			}
			symbols := strings.Split(symbolsStr, ",")
			for i, s := range symbols {
				symbols[i] = strings.TrimSpace(s)
			}
			client := data.NewTickFlowClient()
			results := client.GetKlinesBatch(symbols, period, count, "")
			var sb strings.Builder
			for _, sym := range symbols {
				if d, ok := results[sym]; ok && d != nil {
					sb.WriteString(client.GetKlinesAsMarkdown(sym, period, count, ""))
					sb.WriteString("\n\n")
				} else {
					sb.WriteString(fmt.Sprintf("### %s\n获取失败或无数据\n\n", sym))
				}
			}
			return sb.String(), nil
		},
	))

	// TickFlowUniverse
	result = append(result, NewDataToolWrapper(
		"TickFlowUniverse",
		"获取TickFlow标的池列表（如沪A股、深A股、港股、美股等）。返回标的代码列表。当需要获取某市场规模化的标的列表时使用。",
		map[string]*types.ParamDef{
			"name": {
				Type:     "string",
				Desc:     "标的池名称，如 a.sh（沪A）、a.sz（深A）、hk（港股）、us（美股）",
				Required: true,
			},
			"limit": {
				Type:     "integer",
				Desc:     "显示前N只，默认20",
				Required: false,
			},
		},
		func(args string) (string, error) {
			name := gjson.Get(args, "name").String()
			limit := int(gjson.Get(args, "limit").Int())
			if name == "" {
				return "请提供name参数（标的池名称）", nil
			}
			client := data.NewTickFlowClient()
			return client.GetUniverseAsMarkdown(name, limit), nil
		},
	))

	// TickFlowInstrument
	result = append(result, NewDataToolWrapper(
		"TickFlowInstrument",
		"获取单只标的元数据（TickFlow API）。返回标的名称、市场、类型、上市日期、行业。当需要了解某只股票的基本信息时使用。",
		map[string]*types.ParamDef{
			"symbol": {
				Type:     "string",
				Desc:     "标的代码，如 600000.SH",
				Required: true,
			},
		},
		func(args string) (string, error) {
			symbol := gjson.Get(args, "symbol").String()
			if symbol == "" {
				return "请提供symbol参数", nil
			}
			client := data.NewTickFlowClient()
			return client.GetInstrumentAsMarkdown(symbol), nil
		},
	))

	// TickFlowRealtimeQuotes
	result = append(result, NewDataToolWrapper(
		"TickFlowRealtimeQuotes",
		"获取多只标的实时行情（TickFlow API）。返回最新价、涨跌幅、涨跌额、最高、最低、成交量、成交额等。支持A股、港股、美股。当需要获取实时价格（而非历史K线）时使用，可替代 Sina 爬虫。",
		map[string]*types.ParamDef{
			"symbols": {
				Type:     "string",
				Desc:     "标的代码列表，逗号分隔，如 600000.SH,000001.SZ,^SPX",
				Required: true,
			},
		},
		func(args string) (string, error) {
			symbolsStr := gjson.Get(args, "symbols").String()
			if symbolsStr == "" {
				return "请提供symbols参数", nil
			}
			symbols := strings.Split(symbolsStr, ",")
			for i, s := range symbols {
				symbols[i] = strings.TrimSpace(s)
			}
			return data.NewTickFlowClient().GetRealtimeQuotesAsMarkdown(symbols), nil
		},
	))

	return result
}
