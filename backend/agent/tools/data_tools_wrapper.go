package tools

import (
	"strings"

	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"

	bladesTools "github.com/go-kratos/blades/tools"
	"github.com/tidwall/gjson"
)

func NewDataToolWrapper(name, description string, params map[string]*types.ParamDef, handler func(args string) (string, error)) bladesTools.Tool {
	return types.NewTool(name, description, params, func(args string) (string, error) {
		result, err := handler(args)
		if err != nil {
			return result, err
		}
		return truncateToolResult(name, result), nil
	})
}

// maxToolResultLen 工具返回进模型上下文前的最大字符数。
// K线/资金明细等大返回超限时保留头部（表头上下文）+ 尾部（最新数据），中间截断。
const maxToolResultLen = 8000

func truncateToolResult(name, result string) string {
	if len(result) <= maxToolResultLen {
		return result
	}
	runes := []rune(result)
	if len(runes) <= maxToolResultLen {
		return result
	}
	head := maxToolResultLen * 2 / 5
	tail := maxToolResultLen - head
	headText := string(runes[:head])
	tailText := string(runes[len(runes)-tail:])
	return headText + "\n...(工具 " + name + " 返回过长，中间已截断，保留最新数据)...\n" + tailText
}

func GetAllDataTools() []bladesTools.Tool {
	var result []bladesTools.Tool

	result = append(result, registerScreenTools()...)
	result = append(result, registerFinanceTools()...)
	result = append(result, registerMarketTools()...)
	result = append(result, registerAITools()...)
	result = append(result, registerKlineTools()...)
	result = append(result, registerFundTools()...)
	result = append(result, registerHotTools()...)
	result = append(result, registerChangesTools()...)
	result = append(result, registerWallstreetcnTdxTools()...)

	// 注册游资追踪、筹码分析、资金博弈等高阶分析工具
	result = append(result, GetHotMoneyChipTools()...)

	// AKShare tools（数据源：VPS :8089 AKShare 代理）
	result = append(result, GetAkshareTools()...)

	// AData tools（数据源：VPS :8000 AData 服务）
	result = append(result, GetAdataTools()...)

	// 根据 API Key 配置过滤工具，未配置对应 Key 的工具不注册
	// TickFlow tools (K线、标池、标的元数据)
	result = append(result, GetTickFlowToolWrappers()...)

	// 策略评分选股（YAML 声明策略 + 真实K线指标打分）
	result = append(result, GetStrategyScreenTool())

	var filtered []bladesTools.Tool
	for _, t := range result {
		if !data.IsToolKeyConfigured(t.Name()) {
			continue
		}
		filtered = append(filtered, t)
	}

	return filtered
}

func parseStockCodesFromArgs(args string, mainField string) []string {
	var codes []string
	mainCode := gjson.Get(args, mainField).String()
	if mainCode != "" {
		for _, c := range strings.Split(mainCode, ",") {
			c = strings.TrimSpace(c)
			if c != "" {
				codes = append(codes, c)
			}
		}
	}
	codesArr := gjson.Get(args, "stockCodes").Array()
	for _, c := range codesArr {
		code := strings.TrimSpace(c.String())
		if code != "" {
			codes = append(codes, code)
		}
	}
	return codes
}

func parseInt(s string) (int, error) {
	var result int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result, nil
}
