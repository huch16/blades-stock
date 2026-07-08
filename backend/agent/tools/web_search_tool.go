package tools

import (
	"encoding/json"
	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"
	"blades-stock/backend/logger"

	bladesTools "github.com/go-kratos/blades/tools"
)

func GetWebSearchTool() bladesTools.Tool {
	return types.NewTool("WebSearch",
		"SearXNG/百度 通用互联网搜索工具。优先使用 SearXNG JSON API 搜索，仅当 SearXNG 和百度 HTTP 均无结果时回退到浏览器。当用户需要查询实时信息、无法通过本地股票接口直接获得的列表/地域/新闻/非结构化数据时调用。例如：总部位于某城市的上市公司、最新热点、实时资讯等。优先调用此工具而不是直接回答无法查询。",
		map[string]*types.ParamDef{
			"query": {Type: "string", Desc: "搜索关键词，建议简洁且包含关键限定词", Required: true},
			"count": {Type: "integer", Desc: "返回结果条数，默认5，最大20", Required: false},
		},
		func(args string) (string, error) {
			logger.SugaredLogger.Infof("WebSearch called with args: %s", args)
			parms := map[string]any{}
			if err := json.Unmarshal([]byte(args), &parms); err != nil {
				logger.SugaredLogger.Errorf("WebSearch unmarshal error: %v", err)
				return "", err
			}
			query, ok := parms["query"].(string)
			if !ok || query == "" {
				return "搜索关键词不能为空", nil
			}
			count := 5
			if c, ok := parms["count"].(float64); ok {
				count = int(c)
			}
			if count <= 0 {
				count = 5
			}
			if count > 20 {
				count = 20
			}
			result := data.NewWebSearchApi(30).SearchToMarkdown(query, count)
			logger.SugaredLogger.Infof("WebSearch result length: %d", len(result))
			return result, nil
		},
	)
}
