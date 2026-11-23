package tools

import (
	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	"blades-stock/backend/util"

	bladesTools "github.com/go-kratos/blades/tools"
)

func GetQueryStockNewsTool() bladesTools.Tool {
	return types.NewTool("QueryStockNewsTool", "按关键词搜索相关市场资讯/新闻",
		map[string]*types.ParamDef{
			"searchWords": {Type: "string", Desc: "搜索关键词(多个关键词使用空格分隔)", Required: true},
		},
		func(args string) (string, error) {
			searchWords := types.ExtractStringParam(args, "searchWords", "keyword", "searchWord", "stockName", "query")
			if searchWords == "" {
				searchWords = "市场资讯"
			}
			res := data.NewMarketNewsApi().CailianpressWeb(searchWords)
			if res == nil || len(res.List) == 0 {
				return "暂无相关市场资讯或新闻", nil
			}
			return util.MarkdownTableWithTitle(searchWords+"市场资讯/新闻", res.List), nil
		},
	)
}
