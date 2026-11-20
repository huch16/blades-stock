package data

import (
	"encoding/json"
	"fmt"
	"strings"

	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	"blades-stock/backend/models"

	"github.com/duke-git/lancet/v2/convertor"
	bladesTools "github.com/go-kratos/blades/tools"
	"github.com/tidwall/gjson"
)

func RegisterKlineTools() []bladesTools.Tool {
	var result []bladesTools.Tool

	result = append(result, NewDataToolWrapper(
		"GetStockKLine",
		"获取股票日K线数据。支持一次查询多只。数据源优先级：通达信MAC→东方财富→新浪→腾讯→通达信。",
		map[string]*types.ParamDef{
			"days": {
				Type:     "string",
				Desc:     "日K数据条数",
				Required: true,
			},
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码（A股：sh,sz开头;港股hk开头,美股：us开头）",
				Required: true,
			},
			"stockCodes": {
				Type:     "array",
				Desc:     "可选，多只股票代码列表",
				Required: false,
			},
		},
		func(args string) (string, error) {
			days := gjson.Get(args, "days").String()
			codes := parseStockCodesFromArgs(args, "stockCode")
			toIntDay := 90
			if days != "" {
				if d, err := parseInt(days); err == nil {
					toIntDay = d
				}
			}
			var allResults []map[string]any
			for _, code := range codes {
				var klineData *[]data.KLineData
				if strings.HasPrefix(code, "hk") || strings.HasPrefix(code, "us") || strings.HasPrefix(code, "gb_") {
					api := data.NewStockDataApi()
					klineData = api.GetHK_KLineData(code, "day", int64(toIntDay))
				} else {
					// A股优先使用 FetchKLineWithFallback（MAC→东方财富→新浪→腾讯→通达信）
					fallbackResult := data.FetchKLineWithFallback(code, "", "101", toIntDay, "")
					if fallbackResult.Data != nil && len(*fallbackResult.Data) > 0 {
						klineData = fallbackResult.Data
					}
				}
				if klineData != nil {
					for _, k := range *klineData {
						allResults = append(allResults, map[string]any{
							"stockCode": code,
							"date":      k.Day,
							"open":      k.Open,
							"high":      k.High,
							"low":       k.Low,
							"close":     k.Close,
							"volume":    k.Volume,
						})
					}
				}
			}
			if len(allResults) == 0 {
				return "未获取到 K 线数据", nil
			}
			jsonData, _ := json.Marshal(allResults)
			markdownTable, _ := data.JSONToMarkdownTable(jsonData)
			return markdownTable, nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetEastMoneyKLine",
		"获取股票 K 线数据。支持日/周/月/季/年及1/5/15/30/60分钟线，可选前/后复权。A股代码格式 000001.SZ、600000.SH，港股 00700.HK。支持多只。",
		map[string]*types.ParamDef{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码。A股如 000001.SZ、600000.SH；港股如 00700.HK。多只时可用英文逗号分隔。",
				Required: true,
			},
			"stockCodes": {
				Type:     "array",
				Desc:     "可选，多只股票代码列表",
				Required: false,
			},
			"kLineType": {
				Type:     "string",
				Desc:     "K 线类型：day/日/101=日K，week/周/102=周K，month/月/103=月K，quarter/季/104=季K，halfYear/半年/105=半年K，year/年/106=年K；分钟线：1/5/15/30/60/120",
				Required: true,
			},
			"adjustFlag": {
				Type:     "string",
				Desc:     "复权类型，仅日K有效：空=不复权，qfq=前复权，hfq=后复权",
				Required: false,
			},
			"limit": {
				Type:     "integer",
				Desc:     "获取 K 线根数",
				Required: false,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			kLineType := gjson.Get(args, "kLineType").String()
			adjustFlag := gjson.Get(args, "adjustFlag").String()
			limit := int(gjson.Get(args, "limit").Int())
			if limit <= 0 {
				limit = 60
			}
			codes := parseStockCodesFromArgs(args, "stockCode")
			if stockCode != "" {
				codes = append(codes, stockCode)
			}
			if len(codes) == 0 {
				return "参数 stockCode 或 stockCodes 不能为空", nil
			}
			kType := data.NormalizeKLineType(kLineType)
			var results []string
			for _, code := range codes {
				if code == "" {
					continue
				}
				// A股优先使用 FetchKLineWithFallback（MAC→东方财富→新浪→腾讯→通达信）
				if data.IsAStockCode(code) {
					res := data.FetchKLineWithFallbackAsSection(code, kType, limit)
					results = append(results, res)
				} else {
					api := data.NewEastMoneyKLineApi(data.GetSettingConfig())
					res := data.EastMoneyKLineSection(api, code, kLineType, adjustFlag, limit)
					results = append(results, res)
				}
			}
			return strings.Join(results, "\n"), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"GetEastMoneyKLineWithMA",
		"获取股票 K 线数据并带多条均线（SMA，按收盘价计算）。用于技术分析时同时查看 K 线与均线。A股数据源优先级：通达信MAC→东方财富→新浪→腾讯→通达信。",
		map[string]*types.ParamDef{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码。A股如 000001.SZ、600000.SH；港股如 00700.HK。多只时可用英文逗号分隔。",
				Required: true,
			},
			"stockCodes": {
				Type:     "array",
				Desc:     "可选，多只股票代码列表",
				Required: false,
			},
			"kLineType": {
				Type:     "string",
				Desc:     "K 线类型：day/日/101=日K，week/周/102=周K，month/月/103=月K；分钟线：1/5/15/30/60/120",
				Required: true,
			},
			"limit": {
				Type:     "integer",
				Desc:     "获取 K 线根数",
				Required: false,
			},
			"maPeriods": {
				Type:     "string",
				Desc:     "均线周期，逗号分隔，如 5,10,20,60。不传则默认 5,10,20,60,120",
				Required: false,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			kLineType := gjson.Get(args, "kLineType").String()
			limit := int(gjson.Get(args, "limit").Int())
			maPeriodsStr := gjson.Get(args, "maPeriods").String()
			if limit <= 0 {
				limit = 60
			}
			codes := parseStockCodesFromArgs(args, "stockCode")
			if stockCode != "" {
				codes = append(codes, stockCode)
			}
			if len(codes) == 0 {
				return "参数 stockCode 或 stockCodes 不能为空", nil
			}
			kType := data.NormalizeKLineType(kLineType)
			var results []string
			for _, code := range codes {
				if code == "" {
					continue
				}
				// A股优先使用 FetchKLineWithFallback + 均线计算
				if data.IsAStockCode(code) {
					res := data.FetchKLineWithMASection(code, kType, limit, maPeriodsStr)
					results = append(results, res)
				} else {
					api := data.NewEastMoneyKLineApi(data.GetSettingConfig())
					res := data.EastMoneyKLineWithMASection(api, code, kLineType, limit, maPeriodsStr)
					results = append(results, res)
				}
			}
			return strings.Join(results, "\n"), nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"CreateAiRecommendStocks",
		"创建/保存AI推荐股票记录",
		map[string]*types.ParamDef{
			"modelName": {
				Type:     "string",
				Desc:     "模型名称",
				Required: true,
			},
			"rating": {
				Type:     "string",
				Desc:     "评级(买入/增持/中性/减持/卖出)",
				Required: true,
			},
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如：601138.SH",
				Required: true,
			},
			"stockName": {
				Type:     "string",
				Desc:     "股票名称",
				Required: true,
			},
			"bkName": {
				Type:     "string",
				Desc:     "板块/概念/行业名称",
				Required: true,
			},
			"stockPrice": {
				Type:     "string",
				Desc:     "推荐时股票价格",
				Required: true,
			},
			"recommendReason": {
				Type:     "string",
				Desc:     "推荐理由/驱动因素/逻辑",
				Required: true,
			},
			"recommendBuyPrice": {
				Type:     "string",
				Desc:     "ai建议买入价区间最低价和最高价之间用`-`分隔",
				Required: true,
			},
			"recommendBuyPriceMin": {
				Type:     "number",
				Desc:     "ai建议最低买入价",
				Required: true,
			},
			"recommendBuyPriceMax": {
				Type:     "number",
				Desc:     "ai建议最高买入价",
				Required: true,
			},
			"recommendStopProfitPrice": {
				Type:     "string",
				Desc:     "ai建议止盈价区间最低价和最高价之间用`-`分隔",
				Required: true,
			},
			"recommendStopProfitPriceMin": {
				Type:     "number",
				Desc:     "ai建议最低止盈价",
				Required: true,
			},
			"recommendStopProfitPriceMax": {
				Type:     "number",
				Desc:     "ai建议最高止盈价",
				Required: true,
			},
			"recommendStopLossPrice": {
				Type:     "string",
				Desc:     "ai建议止损价",
				Required: true,
			},
			"riskRemarks": {
				Type:     "string",
				Desc:     "风险提示",
				Required: true,
			},
			"remarks": {
				Type:     "string",
				Desc:     "操作总结/备注",
				Required: true,
			},
		},
		func(args string) (string, error) {
			var recommend models.AiRecommendStocks
			if err := json.Unmarshal([]byte(args), &recommend); err != nil {
				return "", err
			}
			if err := data.NewAiRecommendStocksService().CreateAiRecommendStocks(&recommend); err != nil {
				return "保存股票推荐失败: " + err.Error(), nil
			}
			return "保存股票推荐成功", nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"BatchCreateAiRecommendStocks",
		"批量创建/保存AI推荐股票记录，建议每次批量保存5条记录",
		map[string]*types.ParamDef{
			"stocks": {
				Type:     "array",
				Desc:     "股票推荐列表",
				Required: true,
				ElemInfo: &types.ParamDef{
					Type:     "object",
					Required: true,
					SubParams: map[string]*types.ParamDef{
						"modelName": {
							Type:     "string",
							Desc:     "模型名称",
							Required: true,
						},
						"rating": {
							Type:     "string",
							Desc:     "评级(买入/增持/中性/减持/卖出)",
							Required: true,
						},
						"stockCode": {
							Type:     "string",
							Desc:     "股票代码，如：601138.SH；须与stockName双向匹配（来自工具真实数据），格式非法或代码-名称不一致时禁止填写",
							Required: true,
						},
						"stockName": {
							Type:     "string",
							Desc:     "股票名称",
							Required: true,
						},
						"bkName": {
							Type:     "string",
							Desc:     "板块/概念/行业名称",
							Required: true,
						},
						"stockPrice": {
							Type:     "string",
							Desc:     "推荐时股票价格",
							Required: true,
						},
						"recommendReason": {
							Type:     "string",
							Desc:     "推荐理由/驱动因素/逻辑",
							Required: true,
						},
						"recommendBuyPrice": {
							Type:     "string",
							Desc:     "ai建议买入价区间最低价和最高价之间用`-`分隔",
							Required: true,
						},
						"recommendBuyPriceMin": {
							Type:     "number",
							Desc:     "ai建议最低买入价",
							Required: true,
						},
						"recommendBuyPriceMax": {
							Type:     "number",
							Desc:     "ai建议最高买入价",
							Required: true,
						},
						"recommendStopProfitPrice": {
							Type:     "string",
							Desc:     "ai建议止盈价区间最低价和最高价之间用`-`分隔",
							Required: true,
						},
						"recommendStopProfitPriceMin": {
							Type:     "number",
							Desc:     "ai建议最低止盈价",
							Required: true,
						},
						"recommendStopProfitPriceMax": {
							Type:     "number",
							Desc:     "ai建议最高止盈价",
							Required: true,
						},
						"recommendStopLossPrice": {
							Type:     "string",
							Desc:     "ai建议止损价",
							Required: true,
						},
						"riskRemarks": {
							Type:     "string",
							Desc:     "风险提示",
							Required: true,
						},
						"remarks": {
							Type:     "string",
							Desc:     "操作总结/备注",
							Required: true,
						},
					},
				},
			},
		},
		func(args string) (string, error) {
			stocks := gjson.Get(args, "stocks").String()
			var recommends []*models.AiRecommendStocks
			if err := json.Unmarshal([]byte(stocks), &recommends); err != nil {
				return "", err
			}
			if err := data.NewAiRecommendStocksService().BatchCreateAiRecommendStocks(recommends); err != nil {
				return "批量保存股票推荐失败: " + err.Error(), nil
			}
			return "批量保存股票推荐成功，共保存 " + convertor.ToString(len(recommends)) + " 条记录", nil
		},
	))

	result = append(result, NewDataToolWrapper(
		"SetTradingPrice",
		"设置股票的预警价位线（开仓价、止盈价、止损价），用于设置股票的买入价格和风险控制参数",
		map[string]*types.ParamDef{
			"stockCode": {
				Type:     "string",
				Desc:     "股票代码，如 000001.SZ、600000.SH",
				Required: true,
			},
			"entryPrice": {
				Type:     "number",
				Desc:     "开仓价/买入价（目标买入价格），0 表示不设置",
				Required: true,
			},
			"takeProfitPrice": {
				Type:     "number",
				Desc:     "止盈价（预期卖出价格），0 表示不设置",
				Required: true,
			},
			"stopLossPrice": {
				Type:     "number",
				Desc:     "止损价（亏损止损价格），0 表示不设置",
				Required: true,
			},
			"costPrice": {
				Type:     "number",
				Desc:     "成本价（持仓成本价格），0 表示不设置",
				Required: true,
			},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			entryPrice := gjson.Get(args, "entryPrice").Float()
			takeProfitPrice := gjson.Get(args, "takeProfitPrice").Float()
			stopLossPrice := gjson.Get(args, "stopLossPrice").Float()
			costPrice := gjson.Get(args, "costPrice").Float()
			result := data.NewStockDataApi().SetTradingPrice(entryPrice, takeProfitPrice, stopLossPrice, costPrice, stockCode)
			if result == "设置成功" {
				return fmt.Sprintf("✅ 价位线设置成功！\n\n📈 %s\n💰 开仓价：%.2f\n🎯 止盈价：%.2f\n🛑 止损价：%.2f\n💵 成本价：%.2f", stockCode, entryPrice, takeProfitPrice, stopLossPrice, costPrice), nil
			}
			return "❌ 价位线设置失败：" + result, nil
		},
	))

	return result
}
