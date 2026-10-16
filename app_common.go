package main

import (
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/models"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var ShanghaiTimezone = time.FixedZone("CST", 8*60*60)

func GetShanghaiTime() time.Time {
	return time.Now().In(ShanghaiTimezone)
}

func FormatShanghaiTime(t time.Time) string {
	return t.In(ShanghaiTimezone).Format("2006-01-02 15:04:05")
}

func (a *App) GetTimezone() map[string]any {
	return a.marketService.GetTimezone()
}

func (a *App) LongTigerRank(date string) *[]models.LongTigerRankData {
	return a.marketService.LongTigerRank(date)
}

func (a *App) StockResearchReport(stockCode string) []any {
	return a.marketService.StockResearchReport(stockCode)
}

func (a *App) StockNotice(stockCode string) []any {
	return a.marketService.StockNotice(stockCode)
}

func (a *App) IndustryResearchReport(industryCode string) []any {
	return a.marketService.IndustryResearchReport(industryCode)
}

func (a *App) EMDictCode(code string) []any {
	return a.marketService.EMDictCode(code)
}

func (a *App) AnalyzeSentiment(text string) models.SentimentResult {
	return a.marketService.AnalyzeSentiment(text)
}

func (a *App) HotStock(marketType string) *[]models.HotItem {
	return a.marketService.HotStock(marketType)
}

func (a *App) HotEvent(size int) *[]models.HotEvent {
	return a.marketService.HotEvent(size)
}

func (a *App) HotTopic(size int) []any {
	return a.marketService.HotTopic(size)
}

func (a *App) InvestCalendarTimeLine(yearMonth string) []any {
	return a.marketService.InvestCalendarTimeLine(yearMonth)
}

func (a *App) ClsCalendar() []any {
	return a.marketService.ClsCalendar()
}

func (a *App) GetUplimitHot(date string, limit int) map[string]any {
	return a.marketService.GetUplimitHot(date, limit)
}

func (a *App) IsTradingTime() bool {
	return a.marketService.IsTradingTime()
}

func (a *App) IsHKTradingTime() bool {
	return a.marketService.IsHKTradingTime()
}

func (a *App) IsUSTradingTime() bool {
	return a.marketService.IsUSTradingTime()
}

func (a *App) IsTradingDay(date string) bool {
	return a.marketService.IsTradingDay(date)
}

func (a *App) GetLatestTradingDay() string {
	return a.marketService.GetLatestTradingDay()
}

func (a *App) SearchStock(words string) map[string]any {
	return a.marketService.SearchStock(words)
}

func (a *App) GetHotStrategy() map[string]any {
	return a.marketService.GetHotStrategy()
}

func (a *App) GetCustomStrategyList(query models.CustomStrategyQuery) *models.CustomStrategyPageData {
	return a.marketService.GetCustomStrategyList(query)
}

func (a *App) GetAllCustomStrategies() *[]models.CustomStrategy {
	return a.marketService.GetAllCustomStrategies()
}

func (a *App) SaveCustomStrategy(strategy models.CustomStrategy) string {
	return a.marketService.SaveCustomStrategy(strategy)
}

func (a *App) DeleteCustomStrategy(id uint) string {
	return a.marketService.DeleteCustomStrategy(id)
}

func (a *App) GetAllStocks(page int, pageSize int, name string, technicalIndicators models.TechnicalIndicators) *models.AllStocksResp {
	return a.marketService.GetAllStocks(page, pageSize, name, technicalIndicators)
}

// ChatWithAgent 执行 AI 智能体对话（桌面端通过 Wails EventsEmit 实时推流）
func (a *App) ChatWithAgent(question string, aiConfigId int, sysPromptId *int, memoryMode bool, memoryCount int, thinkingMode bool, agentMode string, sessionId string) {
	a.agentService.ChatWithAgentCustom(func(event string, payload any) {
		runtime.EventsEmit(a.ctx, event, payload)
	}, question, aiConfigId, sysPromptId, memoryMode, memoryCount, thinkingMode, agentMode, sessionId)
}

// AbortChatWithAgent 打断或终止当前正在进行的 Agent 推理
func (a *App) AbortChatWithAgent() {
	a.agentService.AbortChatWithAgent()
}

func (a *App) AnalyzeSentimentWithFreqWeight(text string) map[string]any {
	return a.marketService.AnalyzeSentimentWithFreqWeight(text)
}

func (a *App) GetAIResponseResultList(query models.AIResponseResultQuery) *models.AIResponseResultPageData {
	return a.marketService.GetAIResponseResultList(query)
}

func (a *App) DeleteAIResponseResult(id uint) string {
	return a.marketService.DeleteAIResponseResult(id)
}

func (a *App) BatchDeleteAIResponseResult(ids []uint) string {
	return a.marketService.BatchDeleteAIResponseResult(ids)
}

func (a *App) GetStockChanges(changeTypes []int, pageIndex, pageSize int) *data.StockChangesResponse {
	return a.marketService.GetStockChanges(changeTypes, pageIndex, pageSize)
}

func (a *App) GetAllStockChangesWithPaging(pageSize int) *data.StockChangesResponse {
	return a.marketService.GetAllStockChangesWithPaging(pageSize)
}

func (a *App) GetStockChangeHistory(query models.StockChangeHistoryQuery) *models.StockChangeHistoryPageData {
	return a.marketService.GetStockChangeHistory(query)
}

func (a *App) SaveStockChangesToHistory(changeTypes []int) string {
	return a.marketService.SaveStockChangesToHistory(changeTypes)
}

func (a *App) DeleteStockChangeHistory(days int) string {
	return a.marketService.DeleteStockChangeHistory(days)
}

func (a *App) GetDailyChangeStats(days int) []data.DailyChangeStats {
	return a.marketService.GetDailyChangeStats(days)
}

func (a *App) GetChangeTypeDailyStats(days int) []data.ChangeTypeDailyStats {
	return a.marketService.GetChangeTypeDailyStats(days)
}

func (a *App) GetChangeRank(days int, topN int) *data.ChangeRankResult {
	return a.marketService.GetChangeRank(days, topN)
}

func (a *App) GetDailyDimensionStats(dimension string, name string, days int) []data.DailyDimensionStats {
	return a.marketService.GetDailyDimensionStats(dimension, name, days)
}

func (a *App) GetTypeStatsByDate(date string) []data.TypeCountStats {
	return a.marketService.GetTypeStatsByDate(date)
}

func (a *App) GetAiRecommendStocksList(query models.AiRecommendStocksQuery) *models.AiRecommendStocksPageData {
	return a.marketService.GetAiRecommendStocksList(query)
}

func (a *App) DeleteAiRecommendStocks(id uint) string {
	return a.marketService.DeleteAiRecommendStocks(id)
}

func (a *App) UpdateAiRecommendStocksAlert(id uint, enableAlert bool) string {
	return a.marketService.UpdateAiRecommendStocksAlert(id, enableAlert)
}

func (a *App) GetAllStockInfoList(query data.AllStockInfoQuery) *data.AllStockInfoPageData {
	return a.marketService.GetAllStockInfoList(query)
}

func (a *App) GetAllStockInfoById(id uint) *models.AllStockInfo {
	return a.marketService.GetAllStockInfoById(id)
}

func (a *App) AddAllStockInfo(stock models.AllStockInfo) string {
	return a.marketService.AddAllStockInfo(stock)
}

func (a *App) DeleteAllStockInfo(id uint) string {
	return a.marketService.DeleteAllStockInfo(id)
}

func (a *App) BatchDeleteAllStockInfo(ids []uint) string {
	return a.marketService.BatchDeleteAllStockInfo(ids)
}

func (a *App) GetAllMarkets() []string {
	return a.marketService.GetAllMarkets()
}

func (a *App) GetAllIndustries() []string {
	return a.marketService.GetAllIndustries()
}

func (a *App) GetAllConcepts() []string {
	return a.marketService.GetAllConcepts()
}

func (a *App) GetStockRealTimePrice(stockCode string) map[string]any {
	return a.marketService.GetStockRealTimePrice(stockCode)
}

func (a *App) GetBKFundFlowList(code string, limit int) []models.BKFundFlowPoint {
	return a.marketService.GetBKFundFlowList(code, limit)
}

func (a *App) GetBKFundFlowListByDate(code string, date string) []models.BKFundFlowPoint {
	return a.marketService.GetBKFundFlowListByDate(code, date)
}

func (a *App) GetBKFundFlowTopList(topN int) []models.BKFundFlow {
	return a.marketService.GetBKFundFlowTopList(topN)
}

func (a *App) GetBKFundFlowTopListByDate(date string, topN int) []models.BKFundFlow {
	return a.marketService.GetBKFundFlowTopListByDate(date, topN)
}

func (a *App) GetAllBKCodes() []map[string]string {
	return a.marketService.GetAllBKCodes()
}

func (a *App) GetConceptFundFlowList(code string, limit int) []models.ConceptFundFlowPoint {
	return a.marketService.GetConceptFundFlowList(code, limit)
}

func (a *App) GetConceptFundFlowListByDate(code string, date string) []models.ConceptFundFlowPoint {
	return a.marketService.GetConceptFundFlowListByDate(code, date)
}

func (a *App) GetConceptFundFlowTopList(topN int) []models.ConceptFundFlow {
	return a.marketService.GetConceptFundFlowTopList(topN)
}

func (a *App) GetConceptFundFlowTopListByDate(date string, topN int) []models.ConceptFundFlow {
	return a.marketService.GetConceptFundFlowTopListByDate(date, topN)
}

func (a *App) GetAllConceptCodes() []map[string]string {
	return a.marketService.GetAllConceptCodes()
}
