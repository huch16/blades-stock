package tools

import (
	"encoding/json"
	"fmt"
	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	"blades-stock/backend/logger"
	"strconv"
	"strings"

	bladesTools "github.com/go-kratos/blades/tools"
	"github.com/tidwall/gjson"
)

func GetStockKLineTool() bladesTools.Tool {
	return types.NewTool("GetStockKLine", "查询股票K线数据（支持日、周、月、年、分钟级别）",
		map[string]*types.ParamDef{
			"code":  {Type: "string", Desc: "股票代码", Required: true},
			"type":  {Type: "string", Desc: "K线类型：day/week/month/year/minute", Required: true},
			"count": {Type: "string", Desc: "数量（举例：10）"},
		},
		func(args string) (string, error) {
			code := gjson.Get(args, "code").String()
			kType := gjson.Get(args, "type").String()
			countStr := gjson.Get(args, "count").String()
			code = GetStockCode(code)
			count, err := strconv.ParseInt(countStr, 10, 64)
			if err != nil {
				count = 90
			}
			kline := data.NewStockDataApi().GetKLineData(code, kType, count)
			if kline == nil {
				return fmt.Sprintf("未获取到股票 %s 的K线数据", code), nil
			}
			bytes, err := json.Marshal(kline)
			if err != nil {
				logger.SugaredLogger.Errorf("Failed to marshal KLine data: %v", err)
				return "", err
			}
			return truncateToolResult("GetStockKLine", string(bytes)), nil
		},
	)
}

func GetStockEastMoneyKLineTool() bladesTools.Tool {
	return types.NewTool("GetEastMoneyKLine", "查询东方财富K线数据",
		map[string]*types.ParamDef{
			"code":     {Type: "string", Desc: "股票代码", Required: true},
			"kline_ty": {Type: "string", Desc: "K线类型", Required: true},
			"limit":    {Type: "string", Desc: "返回条数"},
			"adjust":   {Type: "string", Desc: "复权类型：forward/backward/null"},
		},
		func(args string) (string, error) {
			code := gjson.Get(args, "code").String()
			klineTy := gjson.Get(args, "kline_ty").String()
			limit := int(gjson.Get(args, "limit").Int())
			if limit <= 0 {
				limit = 60
			}
			adjust := gjson.Get(args, "adjust").String()
			api := data.NewEastMoneyKLineApi(data.GetSettingConfig())
			result := api.GetKLineData(code, klineTy, adjust, limit)
			if result == nil {
				return fmt.Sprintf("未获取到股票 %s 的东方财富K线数据", code), nil
			}
			bytes, err := json.Marshal(result)
			if err != nil {
				logger.SugaredLogger.Errorf("Failed to marshal EastMoneyKLine data: %v", err)
				return "", err
			}
			return truncateToolResult("GetEastMoneyKLine", string(bytes)), nil
		},
	)
}

func GetEastMoneyKLineWithMATool() bladesTools.Tool {
	return types.NewTool("GetEastMoneyKLineWithMA", "查询东方财富K线数据（带均线MA）",
		map[string]*types.ParamDef{
			"code":       {Type: "string", Desc: "股票代码", Required: true},
			"kline_ty":   {Type: "string", Desc: "K线类型", Required: true},
			"limit":      {Type: "string", Desc: "返回条数"},
			"adjust":     {Type: "string", Desc: "复权类型"},
			"ma_periods": {Type: "string", Desc: "均线周期，逗号分隔（如 5,10,20,60）"},
		},
		func(args string) (string, error) {
			code := gjson.Get(args, "code").String()
			klineTy := gjson.Get(args, "kline_ty").String()
			limit := int(gjson.Get(args, "limit").Int())
			if limit <= 0 {
				limit = 60
			}
			maPeriodsStr := gjson.Get(args, "ma_periods").String()
			var maPeriods []int
			for _, s := range strings.Split(maPeriodsStr, ",") {
				s = strings.TrimSpace(s)
				if p, err := strconv.Atoi(s); err == nil {
					maPeriods = append(maPeriods, p)
				}
			}
			if len(maPeriods) == 0 {
				maPeriods = []int{5, 10, 20, 60}
			}
			api := data.NewEastMoneyKLineApi(data.GetSettingConfig())
			result, err := api.GetKLineWithMA(code, klineTy, limit, maPeriods...)
			if err != nil {
				return fmt.Sprintf("查询东方财富K线数据失败: %v", err), nil
			}
			bytes, err := json.Marshal(result)
			if err != nil {
				logger.SugaredLogger.Errorf("Failed to marshal EastMoneyKLineMA data: %v", err)
				return "", err
			}
			return truncateToolResult("GetEastMoneyKLineWithMA", string(bytes)), nil
		},
	)
}

func GetStockMinuteDataTool() bladesTools.Tool {
	return types.NewTool("GetStockMinuteData", "查询股票分钟数据",
		map[string]*types.ParamDef{
			"code": {Type: "string", Desc: "股票代码", Required: true},
		},
		func(args string) (string, error) {
			code := gjson.Get(args, "code").String()
			code = GetStockCode(code)
			result, desc := data.NewStockDataApi().GetStockMinutePriceData(code)
			if result == nil {
				return fmt.Sprintf("未获取到股票 %s 的分钟数据: %s", code, desc), nil
			}
			bytes, err := json.Marshal(result)
			if err != nil {
				logger.SugaredLogger.Errorf("Failed to marshal minute data: %v", err)
				return "", err
			}
			return truncateToolResult("GetStockMinuteData", string(bytes)), nil
		},
	)
}
