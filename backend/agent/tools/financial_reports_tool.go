package tools

import (
	"encoding/json"
	"go-stock/backend/agent/types"
	"go-stock/backend/data"

	bladesTools "github.com/go-kratos/blades/tools"
	"github.com/tidwall/gjson"
)

func GetStockFinancialInfoTool() bladesTools.Tool {
	return types.NewTool("GetStockFinancialInfo", "查询股票财务数据",
		map[string]*types.ParamDef{
			"stockCode": {Type: "string", Desc: "股票代码", Required: true},
		},
		func(args string) (string, error) {
			stockCode := gjson.Get(args, "stockCode").String()
			resp := data.NewStockDataApi().GetStockFinancialInfo(stockCode)
			jsonBytes, _ := json.Marshal(resp)
			return string(jsonBytes), nil
		},
	)
}
