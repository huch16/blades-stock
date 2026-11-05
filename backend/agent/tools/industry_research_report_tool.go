package tools

import (
	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	"encoding/json"

	bladesTools "github.com/go-kratos/blades/tools"
	"github.com/tidwall/gjson"
)

func GetIndustryResearchReportTool() bladesTools.Tool {
	return types.NewTool("GetIndustryResearchReport", "查询行业研究报告（如光伏行业研究、半导体行业研究等）",
		map[string]*types.ParamDef{
			"industryName": {Type: "string", Desc: "行业名称", Required: true},
		},
		func(args string) (string, error) {
			industryName := gjson.Get(args, "industryName").String()
			resp := data.NewMarketNewsApi().IndustryResearchReport(industryName, 7)
			jsonBytes, _ := json.Marshal(resp)
			return string(jsonBytes), nil
		},
	)
}
