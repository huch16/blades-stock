package tools

import (
	"encoding/json"
	"go-stock/backend/agent/types"
	"go-stock/backend/data"
	"go-stock/backend/logger"

	bladesTools "github.com/go-kratos/blades/tools"
)

func GetQueryStockCodeInfoTool() bladesTools.Tool {
	return types.NewTool("QueryStockCodeInfo", "查询股票/指数信息(股票/指数名称,股票/指数代码,股票/指数拼音,股票/指数拼音首字母,股票/指数交易所等",
		map[string]*types.ParamDef{
			"searchWord": {Type: "string", Desc: "股票搜索关键词", Required: true},
		},
		func(args string) (string, error) {
			logger.SugaredLogger.Infof("QueryStockCodeInfo called with args: %s", args)
			parms := map[string]any{}
			err := json.Unmarshal([]byte(args), &parms)
			if err != nil {
				logger.SugaredLogger.Errorf("QueryStockCodeInfo unmarshal error: %v", err)
				return "", err
			}
			searchWord, ok := parms["searchWord"].(string)
			if !ok {
				logger.SugaredLogger.Errorf("QueryStockCodeInfo searchWord not found in args")
				return "未找到股票信息", nil
			}
			logger.SugaredLogger.Infof("QueryStockCodeInfo searching for: %s", searchWord)
			stockList := data.NewStockDataApi().GetStockList(searchWord)
			marshal, err := json.Marshal(stockList)
			if err != nil {
				logger.SugaredLogger.Errorf("QueryStockCodeInfo marshal error: %v", err)
				return "", err
			}
			logger.SugaredLogger.Infof("QueryStockCodeInfo result length: %d", len(marshal))
			return string(marshal), nil
		},
	)
}
