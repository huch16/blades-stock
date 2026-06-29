package tools

import (
	"encoding/json"
	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	"strings"

	bladesTools "github.com/go-kratos/blades/tools"
	"github.com/tidwall/gjson"
)

func GetEconomicDataTool() bladesTools.Tool {
	return types.NewTool("GetEconomicData", "获取宏观经济指标数据",
		map[string]*types.ParamDef{
			"indicator": {Type: "string", Desc: "指标名称：GDP/CPI/PPI/PMI", Required: true},
		},
		func(args string) (string, error) {
			indicator := gjson.Get(args, "indicator").String()
			api := data.NewMarketNewsApi()
			var result any
			switch strings.ToLower(indicator) {
			case "gdp":
				result = api.GetGDP()
			case "cpi":
				result = api.GetCPI()
			case "ppi":
				result = api.GetPPI()
			case "pmi":
				result = api.GetPMI()
			default:
				return "未知指标: " + indicator + "，支持：GDP/CPI/PPI/PMI", nil
			}
			jsonBytes, _ := json.Marshal(result)
			return string(jsonBytes), nil
		},
	)
}
