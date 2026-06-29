package tools

import (
	"encoding/json"
	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	"time"

	bladesTools "github.com/go-kratos/blades/tools"
	"github.com/tidwall/gjson"
)

func GetNewsListDataTool() bladesTools.Tool {
	return types.NewTool("GetNewsListData", "获取大盘/市场新闻资讯列表",
		map[string]*types.ParamDef{
			"keyword": {Type: "string", Desc: "搜索关键词", Required: true},
			"date":    {Type: "string", Desc: "日期（格式：yyyy-MM-dd）"},
		},
		func(args string) (string, error) {
			keyword := gjson.Get(args, "keyword").String()
			dateStr := gjson.Get(args, "date").String()
			var startTime time.Time
			if dateStr != "" {
				startTime, _ = time.Parse("2006-01-02", dateStr)
			}
			newsList, _ := data.NewMarketNewsApi().GetNewsListData(keyword, startTime, 1, 20)
			jsonBytes, _ := json.Marshal(newsList)
			return string(jsonBytes), nil
		},
	)
}
