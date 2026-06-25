package tools

import (
	"encoding/json"
	"go-stock/backend/agent/types"
	"go-stock/backend/data"

	bladesTools "github.com/go-kratos/blades/tools"
	"github.com/tidwall/gjson"
)

func GetInteractiveAnswerTool() bladesTools.Tool {
	return types.NewTool("InteractiveAnswer", "投资者关系互动平台问答搜索",
		map[string]*types.ParamDef{
			"keyword": {Type: "string", Desc: "搜索关键词", Required: true},
		},
		func(args string) (string, error) {
			keyword := gjson.Get(args, "keyword").String()
			resp := data.NewMarketNewsApi().InteractiveAnswer(1, 20, keyword)
			jsonBytes, _ := json.Marshal(resp)
			return string(jsonBytes), nil
		},
	)
}
