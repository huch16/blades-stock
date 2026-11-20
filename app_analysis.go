package main

import (
	"blades-stock/backend/agent"
	"blades-stock/backend/models"
	"blades-stock/backend/quant"

	"context"
	"strings"
	"time"
)

func (a *App) DiagnoseStock(stockCode string) (*quant.StockDiagnosisResult, error) {
	return a.quantService.DiagnoseStock(stockCode)
}

// GetStockThesis 查询标的买方认知档案及最新判定

// GetStockThesis 查询标的买方认知档案及最新判定
func (a *App) GetStockThesis(stockCode string) (*models.StockThesisLedger, error) {
	return a.agentService.GetStockThesis(stockCode)
}

// GetStockThesisHistory 获取标的历史认知演化时间线

// GetStockThesisHistory 获取标的历史认知演化时间线
func (a *App) GetStockThesisHistory(stockCode string, limit int) ([]models.StockVerdictHistory, error) {
	return a.agentService.GetStockThesisHistory(stockCode, limit)
}

// SaveStockThesis 手动保存或调整买方认知假说

// SaveStockThesis 手动保存或调整买方认知假说
func (a *App) SaveStockThesis(stockCode, stockName, verdict, verdictText, coreThesis, keyCatalysts, riskFactors, modelName string) error {
	return a.agentService.SaveStockThesis(stockCode, stockName, verdict, verdictText, coreThesis, keyCatalysts, riskFactors, modelName)
}

// DeleteStockThesis 物理删除指定标的的买方认知假说

// DeleteStockThesis 物理删除指定标的的买方认知假说
func (a *App) DeleteStockThesis(stockCode string) error {
	return a.agentService.DeleteStockThesis(stockCode)
}

// GetAllStockTheses 获取全量标的认知状态映射表 (供自选列表极速匹配)

// GetAllStockTheses 获取全量标的认知状态映射表 (供自选列表极速匹配)
func (a *App) GetAllStockTheses() (map[string]models.StockThesisLedger, error) {
	return a.agentService.GetAllStockTheses()
}

// GetStockTheses 别名兼容接口，兼容前端对全量买方认知映射表的调用

// GetStockTheses 别名兼容接口，兼容前端对全量买方认知映射表的调用
func (a *App) GetStockTheses() (map[string]models.StockThesisLedger, error) {
	return a.agentService.GetStockTheses()
}

// ScanHoldingRisk 盘前持仓与自选股确定性多维度风控扫描 (零 Token 纯 Go 计算)

// ScanHoldingRisk 盘前持仓与自选股确定性多维度风控扫描 (零 Token 纯 Go 计算)
func (a *App) ScanHoldingRisk(items []quant.HoldingScanItem) (*quant.HoldingRiskScanResult, error) {
	return a.quantService.ScanHoldingRisk(items)
}

// ScanWatchlistRisk 根据自选分组 ID 执行盘前持仓确定性风控扫描

// ScanWatchlistRisk 根据自选分组 ID 执行盘前持仓确定性风控扫描
func (a *App) ScanWatchlistRisk(groupId int) (*quant.HoldingRiskScanResult, error) {
	return a.quantService.ScanWatchlistRisk(groupId)
}

// TriggerAIReportBacktest 手动或自动触发 AI 报告历史回测与胜率检验

// TriggerAIReportBacktest 手动或自动触发 AI 报告历史回测与胜率检验
func (a *App) TriggerAIReportBacktest(limit int) (*quant.AIReportBacktestResult, error) {
	return a.quantService.TriggerAIReportBacktest(limit)
}

// RunETFMomentumBacktest 触发 ETF 动量轮动策略回测 (21日加权对数回归 + 硬风控状态机)
//
// etf_backtest.go 的 RunETFMomentumBacktest 此前无任何生产调用者，属于「已开发未集成」
// 的集成缺口。本 RPC 将其暴露给前端，用户可一键触发 ETF 动量轮动回测，输出
// 总收益、年化收益、最大回撤、胜率、盈亏比、交易次数等确定性指标。
//
// symbols: 逗号分隔的 ETF 代码列表（如 "510300.SH,510050.SH"），为空则使用默认 26 标的池
// topN: 同时持有 ETF 数量；momentumDays: 动量回看窗口（默认 21）
// initCash: 初始资金；fees: 单边交易成本（‰）；slippage: 单边滑点（‰）

// symbols: 逗号分隔的 ETF 代码列表（如 "510300.SH,510050.SH"），为空则使用默认 26 标的池
// topN: 同时持有 ETF 数量；momentumDays: 动量回看窗口（默认 21）
// initCash: 初始资金；fees: 单边交易成本（‰）；slippage: 单边滑点（‰）
func (a *App) RunETFMomentumBacktest(symbols string, topN int, momentumDays int, initCash float64, fees float64, slippage float64) (*quant.ETFMomentumResult, error) {
	return a.quantService.RunETFMomentumBacktest(symbols, topN, momentumDays, initCash, fees, slippage)
}

// RunSkillABTest runs an A/B test between two skills.

// RunSkillABTest runs an A/B test between two skills.
func (a *App) RunSkillABTest(skillID1, skillID2 uint, limit int) (*quant.SkillABTestResult, error) {
	return a.quantService.RunSkillABTest(skillID1, skillID2, limit)
}

// GetAuditLogList 分页获取审计日志

// GetAuditLogList 分页获取审计日志
func (a *App) GetAuditLogList(query models.AuditLogQuery) (*models.AuditLogPageResp, error) {
	return a.settingsService.GetAuditLogList(query)
}

// ExportAuditLogCSV 导出审计日志为 CSV 字符串

// ExportAuditLogCSV 导出审计日志为 CSV 字符串
func (a *App) ExportAuditLogCSV(query models.AuditLogQuery) (string, error) {
	return a.settingsService.ExportAuditLogCSV(query)
}

// VectorBTResult 向量化回测结果
type VectorBTResult struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// RunVectorBacktest 运行单次回测

// RunVectorBacktest 运行单次回测
func (a *App) RunVectorBacktest(stockCode, strategyType string, params map[string]any) map[string]any {
	return a.quantService.RunVectorBacktest(stockCode, strategyType, params)
}

// RunVectorGridSearch 运行参数网格扫描

// RunVectorGridSearch 运行参数网格扫描
func (a *App) RunVectorGridSearch(stockCode, strategyType string) map[string]any {
	return a.quantService.RunVectorGridSearch(stockCode, strategyType)
}

// GenerateETFRotationReport 手动或一键触发全市场ETF轮动多Agent决策分析

// GenerateETFRotationReport 手动或一键触发全市场ETF轮动多Agent决策分析
func (a *App) GenerateETFRotationReport(aiConfigId int) (*agent.MarketIntelligenceReport, error) {
	return a.agentService.GenerateETFRotationReport(a.ctx, aiConfigId)
}

// GenerateSerenityResearchReport 手动或一键触发Serenity产业深度拆解

// GenerateSerenityResearchReport 手动或一键触发Serenity产业深度拆解
func (a *App) GenerateSerenityResearchReport(targetName string, code string, aiConfigId int) (*agent.MarketIntelligenceReport, error) {
	return a.agentService.GenerateSerenityResearchReport(a.ctx, targetName, code, aiConfigId)
}

// ReloadSkills 热重载技能定义，无需重启应用

// FetchAuditLogs 前端兼容别名：分页获取审计日志
func (a *App) FetchAuditLogs(query models.AuditLogQuery) (*models.AuditLogPageResp, error) {
	return a.settingsService.FetchAuditLogs(query)
}

// FetchSkillChanges 查询技能审核与变更流水

// GetPaperPortfolioStats 获取智能体专属模拟基金账户报表（净值NAV、持仓、夏普、回撤与胜率）
func (a *App) GetPaperPortfolioStats() (*models.PaperPortfolioStats, error) {
	mgr := quant.NewPaperPortfolioManager()
	_, _ = mgr.UpdateDailyNAV()
	return mgr.GetPortfolioStats()
}

// ExecutePaperTrade 执行模拟盘虚拟买入或卖出指令

// ExecutePaperTrade 执行模拟盘虚拟买入或卖出指令
func (a *App) ExecutePaperTrade(side, stockCode, stockName string, price float64, shares int, stopLoss, takeProfit, targetWeight float64, reason string) (*models.PaperTradeOrder, error) {
	mgr := quant.NewPaperPortfolioManager()
	if strings.ToUpper(side) == "BUY" {
		return mgr.ExecuteVirtualBuy(stockCode, stockName, price, stopLoss, takeProfit, targetWeight, reason)
	}
	return mgr.ExecuteVirtualSell(stockCode, price, shares, reason)
}

// RunDailyPostMortemAudit 触发 AI 推荐与买方立论的闭环胜率对账与死后剖析

// RunDailyPostMortemAudit 触发 AI 推荐与买方立论的闭环胜率对账与死后剖析
func (a *App) RunDailyPostMortemAudit(targetDate string) (*models.PostMortemAuditSummary, error) {
	service := quant.NewPostMortemLoopService()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return service.RunDailyAudit(ctx, targetDate)
}

// GetRecentPostMortemRecords 查询最近的决策胜率对账与死后剖析档案

// GetRecentPostMortemRecords 查询最近的决策胜率对账与死后剖析档案
func (a *App) GetRecentPostMortemRecords(limit int) ([]models.PostMortemAuditRecord, error) {
	service := quant.NewPostMortemLoopService()
	return service.GetRecentAuditRecords(limit)
}

// RunAutoAlphaExploration 触发夜间量化因子与参数空间网格搜索进化

// RunAutoAlphaExploration 触发夜间量化因子与参数空间网格搜索进化
func (a *App) RunAutoAlphaExploration(targetPool string) (*models.AutoAlphaProposal, error) {
	var codes []string
	if targetPool != "" {
		for _, p := range strings.Split(targetPool, ",") {
			c := strings.TrimSpace(p)
			if c != "" {
				codes = append(codes, c)
			}
		}
	}
	explorer := quant.NewAutoAlphaExplorer()
	return explorer.ExploreETFMomentumParams(codes)
}

// GetAutoAlphaProposals 获取历史量化策略自进化提案

// GetAutoAlphaProposals 获取历史量化策略自进化提案
func (a *App) GetAutoAlphaProposals(limit int) ([]models.AutoAlphaProposal, error) {
	explorer := quant.NewAutoAlphaExplorer()
	return explorer.GetActiveProposals(limit)
}

// QuerySupplyChainCascade 执行跨市场与产业链事件级联推演雷达

// QuerySupplyChainCascade 执行跨市场与产业链事件级联推演雷达
func (a *App) QuerySupplyChainCascade(source, headline string) (*models.CascadeEventResult, error) {
	graph := quant.NewIndustryKnowledgeGraph()
	return graph.CascadeEventReasoning(source, headline)
}
