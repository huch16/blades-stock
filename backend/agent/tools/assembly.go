package tools

import (
	bladesToolsPkg "github.com/go-kratos/blades/tools"
)

// AssembleBuiltinTools 组装本仓库内置的**全部**候选工具。
//
// 这一清单原先散落在 agent 包的 getToolsByQuestion 里（约 30 行硬编码 append），
// 与能力注册表分居两地：漏 append 一个工具，它就根本到不了模型手里，
// 而注册表无法发现。搬到这里后，「组装」与「身份声明」同处一个包，
// capability_consistency_test.go 的 TestEveryAssembledToolIsDeclared 才能
// 真正把两者对上。
//
// 注意：外部 MCP 服务器提供的工具由调用方另行追加（getMCPTools），
// 它们运行时才确定，不参与静态一致性校验。
func AssembleBuiltinTools() []bladesToolsPkg.Tool {
	var allTools []bladesToolsPkg.Tool

	allTools = append(allTools, GetQueryStockCodeInfoTool())
	allTools = append(allTools, GetQueryStockNewsTool())
	allTools = append(allTools, GetQueryBKDictTool())
	allTools = append(allTools, GetWebSearchTool())
	allTools = append(allTools, GetFetchWebPageTool())
	allTools = append(allTools, GetGraphQueryTool()) // P3.3: 代码图谱查询

	allTools = append(allTools, GetAllDataTools()...)

	allTools = append(allTools, GetHolidayTools()...)

	allTools = append(allTools, GetProjectTools()...)

	allTools = append(allTools, GetBacktestStatsTool())
	allTools = append(allTools, GetBacktestDetailTool())
	allTools = append(allTools, GetRunBacktestTool())
	allTools = append(allTools, GetVectorBacktestTool())
	allTools = append(allTools, GetRunVectorizedBacktestTool())
	allTools = append(allTools, GetDiagnoseStockTool())
	allTools = append(allTools, GetVectorGridSearchTool())
	allTools = append(allTools, GetStockThesisTool())
	allTools = append(allTools, SaveStockThesisTool())
	allTools = append(allTools, TriggerPostMortemAuditTool())
	allTools = append(allTools, GetPostMortemAuditHistoryTool())
	allTools = append(allTools, GetPaperPortfolioSummaryTool())
	allTools = append(allTools, ExecutePaperTradeTool())
	allTools = append(allTools, RunAutoAlphaExplorationTool())
	allTools = append(allTools, GetAutoAlphaProposalsTool())
	allTools = append(allTools, QuerySupplyChainCascadeTool())
	allTools = append(allTools, GetIndustrySupplyGraphTool())

	return allTools
}
