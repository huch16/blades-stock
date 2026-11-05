package tools

import (
	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	"encoding/json"
	"strings"

	bladesTools "github.com/go-kratos/blades/tools"
)

func GetQueryStockPriceInfoTool() bladesTools.Tool {
	return types.NewTool("QueryStockPriceInfo", "批量获取实时股价数据",
		map[string]*types.ParamDef{
			"stockCodes": {Type: "string", Desc: "股票代码,多个,隔开,股票代码必须转化为sh或者sz或者hk开头的形式,例如：sz399001,sh600859", Required: true},
		},
		func(args string) (string, error) {
			if strings.TrimSpace(args) == "" {
				args = "{}"
			}
			parms := map[string]any{}
			err := json.Unmarshal([]byte(args), &parms)
			if err != nil {
				return "参数解析失败", nil
			}
			rawCodes, ok := parms["stockCodes"].(string)
			if !ok || strings.TrimSpace(rawCodes) == "" {
				return "请提供有效的股票代码列表", nil
			}
			stockCodes := strings.Split(rawCodes, ",")
			var codes []string
			for _, code := range stockCodes {
				codes = append(codes, GetStockCode(code))
			}
			realTimeData, err := data.NewStockDataApi().GetStockCodeRealTimeData(codes...)
			if err != nil {
				return "", err
			}
			marshal, err := json.Marshal(realTimeData)
			if err != nil {
				return "", err
			}
			return string(marshal), nil
		},
	)
}
