package tools

import (
	"fmt"
	"go-stock/backend/agent/types"
	"go-stock/backend/data"

	bladesTools "github.com/go-kratos/blades/tools"
	"github.com/tidwall/gjson"
)

func GetSearchStockByIndicatorsTool() bladesTools.Tool {
	return types.NewTool("SearchStockByIndicators", "条件选股（按财务指标/估值指标等筛选A股）",
		map[string]*types.ParamDef{
			"query": {Type: "string", Desc: "选股条件（如：市盈率>0,市净率<10）", Required: true},
		},
		func(args string) (string, error) {
			query := gjson.Get(args, "query").String()
			if query == "" {
				query = fmt.Sprintf("选股条件: %s", gjson.Get(args, "query").String())
			}
			resp := data.NewIwencaiAPI().QueryToMarkdown(query, 1, 50)
			return resp, nil
		},
	)
}
