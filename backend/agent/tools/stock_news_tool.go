package tools

import (
	"go-stock/backend/agent/types"
	"go-stock/backend/data"
	"go-stock/backend/util"

	bladesTools "github.com/go-kratos/blades/tools"
	"github.com/tidwall/gjson"
)

func GetQueryStockNewsTool() bladesTools.Tool {
	return types.NewTool("QueryStockNewsTool", "按关键词搜索相关市场资讯/新闻",
		map[string]*types.ParamDef{
			"searchWords": {Type: "string", Desc: "搜索关键词(多个关键词使用空格分隔)", Required: true},
		},
		func(args string) (string, error) {
			searchWords := gjson.Get(args, "searchWords").String()
			res := data.NewMarketNewsApi().CailianpressWeb(searchWords)
			return util.MarkdownTableWithTitle(searchWords+"市场资讯/新闻", res.List), nil
		},
	)
}
