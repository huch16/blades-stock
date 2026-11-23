package data

import (
	"blades-stock/backend/agent/types"

	bladesTools "github.com/go-kratos/blades/tools"
)

// NewDataToolWrapper 构造数据工具（R-26 自父包 backend/agent/tools 移入；
// 父包留同名转发，adata/backtest/mcp_skill/tickflow 等调用点零改动）。
func NewDataToolWrapper(name, description string, params map[string]*types.ParamDef, handler func(args string) (string, error)) bladesTools.Tool {
	return types.NewTool(name, description, params, func(args string) (string, error) {
		result, err := handler(args)
		if err != nil {
			return result, err
		}
		return TruncateToolResult(name, result), nil
	})
}

// MaxToolResultLen 工具返回进模型上下文前的最大字符数。
// K线/资金明细等大返回超限时保留头部（表头上下文）+ 尾部（最新数据），中间截断。
// R-26 自父包移入时导出（父包 test 与 const 别名在用）。
const MaxToolResultLen = 8000

// maxToolResultLen 同包内别名（保持原实现写法不变）。
const maxToolResultLen = MaxToolResultLen

func TruncateToolResult(name, result string) string {
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

func parseInt(s string) (int, error) {
	var result int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result, nil
}
