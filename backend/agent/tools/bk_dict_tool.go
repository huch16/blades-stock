package tools

import (
	"encoding/json"
	"go-stock/backend/agent/types"
	"go-stock/backend/data"

	"github.com/coocood/freecache"
	bladesTools "github.com/go-kratos/blades/tools"
)

func GetQueryBKDictTool() bladesTools.Tool {
	return types.NewTool("QueryBKDictInfo", "获取所有板块/行业名称或者代码(bkCode,bkName)",
		nil,
		func(args string) (string, error) {
			resp := data.NewMarketNewsApi().EMDictCode("016", freecache.NewCache(100))
			bytes, err := json.Marshal(resp)
			return string(bytes), err
		},
	)
}
