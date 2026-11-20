package tools

import (
	"blades-stock/backend/agent/types"
	"blades-stock/backend/data"

	datatools "blades-stock/backend/agent/tools/data"

	bladesTools "github.com/go-kratos/blades/tools"
)

// NewDataToolWrapper 构造数据工具（R-26：实现已搬入子包
// backend/agent/tools/data，本函数只做转发；父包内 adata/backtest/
// mcp_skill/tickflow 等调用点零改动）。
func NewDataToolWrapper(name, description string, params map[string]*types.ParamDef, handler func(args string) (string, error)) bladesTools.Tool {
	return datatools.NewDataToolWrapper(name, description, params, handler)
}

// maxToolResultLen 转发子包的常量值（R-26；tool_result_truncate_test.go 在用）。
const maxToolResultLen = datatools.MaxToolResultLen

// truncateToolResult 转发到子包（R-26；tool_result_truncate_test.go 在用）。
func truncateToolResult(name, result string) string {
	return datatools.TruncateToolResult(name, result)
}

func GetAllDataTools() []bladesTools.Tool {
	var result []bladesTools.Tool

	result = append(result, datatools.RegisterScreenTools()...)
	result = append(result, datatools.RegisterFinanceTools()...)
	result = append(result, datatools.RegisterMarketTools()...)
	result = append(result, datatools.RegisterAITools()...)
	result = append(result, datatools.RegisterKlineTools()...)
	result = append(result, datatools.RegisterFundTools()...)
	result = append(result, datatools.RegisterHotTools()...)
	result = append(result, datatools.RegisterChangesTools()...)
	result = append(result, datatools.RegisterWallstreetcnTdxTools()...)

	// ETF 动量排名工具仍在父包（tool_etf_momentum.go 依赖父包 GetStockCode），
	// 此处补回（原在 registerKlineTools 内聚合），工具总数不变。
	result = append(result, GetETFMomentumRankingTool())

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
