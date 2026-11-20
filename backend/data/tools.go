package data

// @Author spark
// @Date 2026/3/7 18:48
// @Desc
//-----------------------------------------------------------------------------------

// toolSchemaStockCodes 可选多只股票，与 code/stockCode 合并解析（去重）；也可仅在 code/stockCode 中用英文逗号分隔。
var toolSchemaStockCodes = map[string]any{
	"type": "array",
	"items": map[string]any{
		"type": "string",
	},
	"description": "可选，多只股票代码列表；与主字段合并后去重。也可仅在主字段中用英文逗号分隔多只。",
}

func Tools(tools []Tool) []Tool {
	tools = appendScreenTools(tools)
	tools = appendFinanceTools(tools)
	tools = appendMarketTools(tools)
	tools = appendAITools(tools)
	tools = appendKlineTools(tools)
	tools = appendFundTools(tools)
	tools = appendHotTools(tools)
	tools = appendChangesTools(tools)
	tools = appendWallstreetcnTdxTools(tools)

	tools = appendAgentParityTools(tools)

	// 根据 API Key 配置过滤工具，未配置对应 Key 的工具不注册
	tools = FilterToolsByApiKey(tools)

	return tools
}
