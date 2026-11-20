package main

import (
	"blades-stock/backend/agent"
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
	"blades-stock/backend/services"

	"context"
	"fmt"
	"strings"
	"time"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/mathutil"
	"github.com/duke-git/lancet/v2/strutil"
	runtime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) SendDingDingMessage(message string, stockCode string) string {
	ttl, _ := a.cache.TTL([]byte(stockCode))
	//logger.SugaredLogger.Infof("stockCode %s ttl:%d", stockCode, ttl)
	if ttl > 0 {
		return ""
	}
	err := a.cache.Set([]byte(stockCode), []byte("1"), 60*5)
	if err != nil {
		logger.SugaredLogger.Errorf("set cache error:%s", err.Error())
		return ""
	}
	return data.NewDingDingAPI().SendDingDingMessage(message)
}

// SendDingDingMessageByType msgType 报警类型: 1 涨跌报警;2 股价报警 3 成本价报警

// SendDingDingMessageByType msgType 报警类型: 1 涨跌报警;2 股价报警 3 成本价报警
func (a *App) SendDingDingMessageByType(message string, stockCode string, msgType int) string {

	if strutil.HasPrefixAny(stockCode, []string{"SZ", "SH", "sh", "sz"}) && (!isTradingTime(time.Now())) {
		return "非A股交易时间"
	}
	if strutil.HasPrefixAny(stockCode, []string{"hk", "HK"}) && (!IsHKTradingTime(time.Now())) {
		return "非港股交易时间"
	}
	if strutil.HasPrefixAny(stockCode, []string{"us", "US", "gb_"}) && (!IsUSTradingTime(time.Now())) {
		return "非美股交易时间"
	}

	ttl, _ := a.cache.TTL([]byte(stockCode))
	if ttl > 0 {
		return ""
	}
	err := a.cache.Set([]byte(stockCode), []byte("1"), getMsgTypeTTL(msgType))
	if err != nil {
		logger.SugaredLogger.Errorf("set cache error:%s", err.Error())
		return ""
	}
	stockInfo := &data.StockInfo{}
	db.Dao.Model(stockInfo).Where("code = ?", stockCode).First(stockInfo)
	go data.NewAlertWindowsApi("Blades Stock消息通知", getMsgTypeName(msgType), GenNotificationMsg(stockInfo), "").SendNotification()

	go runtime.EventsEmit(a.ctx, "newsPush", map[string]any{
		"time":    "📈 " + getMsgTypeName(msgType),
		"isRed":   true,
		"source":  "blades-stock",
		"content": GenNotificationMsg(stockInfo),
	})

	return data.NewDingDingAPI().SendDingDingMessage(message)
}

// preFetchStockAnalysisData collects real-time market data for the given stock
// and returns a formatted reference string for the agent's system prompt.
// isETFRecommendQuestion 判断用户问题是否属于 ETF/基金推荐、轮动、板块资金类问询。
// 这类问题与当前页面个股无关，注入该个股预采集数据会误导模型偏离 ETF 分析框架。

// preFetchStockAnalysisData collects real-time market data for the given stock
// and returns a formatted reference string for the agent's system prompt.
// isETFRecommendQuestion 判断用户问题是否属于 ETF/基金推荐、轮动、板块资金类问询。
// 这类问题与当前页面个股无关，注入该个股预采集数据会误导模型偏离 ETF 分析框架。
func isETFRecommendQuestion(q string) bool {
	if strings.TrimSpace(q) == "" {
		return false
	}
	lower := strings.ToLower(q)
	etfHints := []string{"ETF", "etf", "基金", "板块", "轮动", "北向资金", "资金流向", "热点", "大盘", "指数"}
	for _, hint := range etfHints {
		if strings.Contains(lower, hint) {
			return true
		}
	}
	return false
}

func (a *App) preFetchStockAnalysisData(stockCode string) string {
	if stockCode == "" {
		return ""
	}
	var parts []string

	// Real-time quote
	if info, err := a.marketService.GetStockCodeRealTimeData(stockCode); err == nil && info != nil && len(*info) > 0 {
		s := (*info)[0]
		changePercent := s.ChangePercent
		changePrice := s.ChangePrice
		parts = append(parts, fmt.Sprintf("【实时行情】%s(%s) 当前价:%s 涨跌幅:%.2f%% 涨跌额:%.2f 开盘:%s 最高:%s 最低:%s 昨收:%s 成交量:%s 成交额:%s",
			s.Name, s.Code, s.Price, changePercent, changePrice, s.Open, s.High, s.Low, s.PreClose, s.Volume, s.Amount))
	}

	// Latest financial data
	financeMd := a.marketService.GetStockLatestFinanceToMarkdown(stockCode)
	if financeMd != "" && !strings.HasPrefix(financeMd, "获取") {
		parts = append(parts, financeMd)
	}

	// Quarterly financial indicators (ROE, gross margin, etc.)
	qtrMd := a.marketService.GetStockQtrMainFinanceToMarkdown(stockCode)
	if qtrMd != "" && !strings.HasPrefix(qtrMd, "获取") {
		parts = append(parts, qtrMd)
	}

	// Valuation percentile (PE/PB percentile rank)
	valMd := a.marketService.GetStockValuationPercentileToMarkdown(stockCode)
	if valMd != "" && !strings.HasPrefix(valMd, "获取") {
		parts = append(parts, valMd)
	}

	// Institutional prediction summary
	predMd := a.marketService.GetStockPredictSummaryToMarkdown(stockCode)
	if predMd != "" && !strings.HasPrefix(predMd, "获取") {
		parts = append(parts, predMd)
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n\n")
}

func (a *App) NewChatStream(stock, stockCode, question string, aiConfigId int, sysPromptId *int, enableTools bool, think bool) {
	defer func() {
		if err := recover(); err != nil {
			logger.SugaredLogger.Errorf("NewChatStream panic: %v", err)
			runtime.EventsEmit(a.ctx, "newChatStream", map[string]any{
				"code":    0,
				"content": fmt.Sprintf("AI分析异常: %v", err),
			})
			runtime.EventsEmit(a.ctx, "newChatStream", "DONE")
		}
	}()
	analysisQuestion := strings.TrimSpace(question)

	// 当用户问题涉及 ETF/基金推荐等非个股问询时，不注入当前页面的个股预采集数据，
	// 否则模型会被强制引导到当前股票（如 TCL科技）的个股研判，而不是执行 ETF 轮动 skill。
	isETFOriented := isETFRecommendQuestion(analysisQuestion)

	// Pre-fetch real-time stock data as reference context for the agent
	var preFetchCtx string
	if !isETFOriented {
		preFetchCtx = a.preFetchStockAnalysisData(stockCode)
	}

	if analysisQuestion == "" {
		analysisQuestion = fmt.Sprintf("请对%s(%s)进行全方位深度分析，从技术面、基本面、消息面、资金面四个维度综合研判，给出明确的多空判断和操作建议。", stock, stockCode)
	}
	if preFetchCtx != "" {
		analysisQuestion += "\n\n【预采集参考数据】以下数据由系统预先采集，是真实数据，可直接引用分析：\n" + preFetchCtx + "\n\n【重要】严禁编造或推算PE/PB/ROE/毛利率/营收/利润/资金流向/均线等任何数字指标。所有数据须来自预采集数据或工具返回结果。不确定的指标请明确说'数据不可得'，不要自行估算。"
	}

	agentMode := string(agent.AgentModePlanExecute)
	if !enableTools || (strings.TrimSpace(question) == "" && preFetchCtx != "") {
		agentMode = string(agent.AgentModeDirect)
	}

	sysPrompt := ""
	if agentMode == string(agent.AgentModeDirect) {
		sysPrompt = "你现在是顶级证券投资顾问与量化宏观专家“财智”。所有核心客观盘面数据、估值百分位与详尽财务指标已在用户输入中完整提供，无需额外调用外部工具。请直接基于输入中的真实数据，从技术面、基本面、消息面、资金面四个维度输出结构清晰、多空判断明确的深度研报，严禁在正文中生成任何伪造的工具调用标签。"
	}

	for msg := range a.agentService.ChatWithAgentStream(a.ctx, analysisQuestion, aiConfigId, sysPromptId, true, 5, think, agentMode, sysPrompt) {
		if msg == nil {
			continue
		}
		if msg.Content == "agent-DONE" {
			continue
		}
		runtime.EventsEmit(a.ctx, "newChatStream", map[string]any{
			"content":           msg.Content,
			"reasoning_content": msg.ReasoningContent,
		})
	}
	runtime.EventsEmit(a.ctx, "newChatStream", "DONE")
}

func (a *App) SaveAIResponseResult(stockCode, stockName, result, chatId, question string, aiConfigId int) {
	a.agentService.SaveAIResponseResult(a.ctx, stockCode, stockName, result, chatId, question, aiConfigId)
}

// GetSystemStateSnapshot 获取当前系统的全局态势感知快照

// GetSystemStateSnapshot 获取当前系统的全局态势感知快照
func (a *App) GetSystemStateSnapshot() *agent.SystemStateSnapshot {
	return a.agentService.GetSystemStateSnapshot()
}

// GetToolQuotaStatus 获取大模型工具调用成本预算与配额使用状态

// GetToolQuotaStatus 获取大模型工具调用成本预算与配额使用状态
func (a *App) GetToolQuotaStatus() agent.QuotaStatus {
	return a.agentService.GetToolQuotaStatus()
}

// ResetToolDailyQuota 手动重置今日工具调用配额与成本计数

// ResetToolDailyQuota 手动重置今日工具调用配额与成本计数
func (a *App) ResetToolDailyQuota() string {
	return a.agentService.ResetToolDailyQuota()
}

// SyncRecommendStocksFromIntelligence 从最新研报、ETF动量池及历史深度分析中一键提炼推荐股票

// SyncRecommendStocksFromIntelligence 从最新研报、ETF动量池及历史深度分析中一键提炼推荐股票
func (a *App) SyncRecommendStocksFromIntelligence() (int, error) {
	return a.agentService.SyncRecommendStocksFromIntelligence(a.ctx)
}

func GenNotificationMsg(stockInfo *data.StockInfo) string {
	Price, err := convertor.ToFloat(stockInfo.Price)
	if err != nil {
		Price = 0
	}
	PreClose, err := convertor.ToFloat(stockInfo.PreClose)
	if err != nil {
		PreClose = 0
	}
	var RF float64
	if PreClose > 0 {
		RF = mathutil.RoundToFloat(((Price-PreClose)/PreClose)*100, 2)
	}

	return "[" + stockInfo.Name + "] " + stockInfo.Price + " " + convertor.ToString(RF) + "% " + stockInfo.Date + " " + stockInfo.Time
}

// msgType : 1 涨跌报警(5分钟);2 股价报警(30分钟) 3 成本价报警(30分钟) 4 止盈报警(5分钟) 5 止损报警(5分钟)

// msgType : 1 涨跌报警(5分钟);2 股价报警(30分钟) 3 成本价报警(30分钟) 4 止盈报警(5分钟) 5 止损报警(5分钟)
func getMsgTypeTTL(msgType int) int {
	switch msgType {
	case 1:
		return 60 * 5
	case 2:
		return 60 * 30
	case 3:
		return 60 * 30
	case 4:
		return 60 * 5
	case 5:
		return 60 * 5
	default:
		return 60 * 5
	}
}

func getMsgTypeName(msgType int) string {
	switch msgType {
	case 1:
		return "涨跌报警"
	case 2:
		return "股价报警"
	case 3:
		return "成本价报警"
	case 4:
		return "止盈报警"
	case 5:
		return "止损报警"
	default:
		return "未知类型"
	}
}

func (a *App) SummaryStockNews(question string, aiConfigId int, sysPromptId *int, enableTools bool, think bool, eventName string, historyJSON string) {
	ctx, cancel := context.WithCancel(a.ctx)

	a.summaryMu.Lock()
	if a.summaryCancel != nil {
		a.summaryCancel()
	}
	a.summaryCancel = cancel
	a.summaryMu.Unlock()

	if strings.TrimSpace(eventName) == "" {
		eventName = "summaryStockNews"
	}

	// Pre-fetch market index data as reference context
	preFetchCtx := a.preFetchMarketSummaryData()
	if preFetchCtx != "" {
		preFetchCtx = "\n\n【预采集参考数据】以下为当前市场概况，可供直接引用：\n" + preFetchCtx
	}

	q := strings.TrimSpace(question)
	if q == "" {
		q = `请对当前A股市场进行全面的AI资讯总结和分析。

## 执行步骤
### 第1步：获取市场整体数据
- 用 GetMarketData 获取大盘指数行情、涨跌家数、涨跌分布
- 用 GlobalStockIndexesReadable 获取全球主要指数表现
- 用 GetStockChanges 获取今日异动数据（火箭发射、大笔买入等）

### 第2步：捕捉市场热点
- 用 GetUplimitHotPlates 获取今日涨停板块热度排名
- 用 HotStockTable 获取热门股票排名
- 用 GetUplimitHotStocks 获取涨停个股列表
- 用 HotspotDiscovery 发现当前市场热点题材

### 第3步：消息面分析
- 用 GetNewsListData 获取当天重要财经新闻
- 用 GetWallstreetcnLives 获取华尔街见闻7x24快讯
- 用 WebSearch 搜索最新的重要政策/行业消息

### 第4步：资金面分析
- 用 GetStockMoneyData 查看个股资金流入排名
- 用 GetIndustryMoneyRank 查看行业资金流向
- 用 GetMutualTop10Deal 查看北向资金动向

### 第5步：综合分析
- 当前市场整体趋势判断（多头/空头/震荡）
- 主线热点板块和持续性分析
- 资金偏好方向（大盘/小盘，价值/成长）
- 潜在风险和关注点
- 建议关注的方向/板块/个股

注意：每一步都必须调用对应的工具获取最新数据，严禁凭记忆编造。即使某些工具调用失败，也要基于已成功获取的数据进行分析，不要反复追问用户补充信息。` + preFetchCtx
	} else {
		q = q + preFetchCtx
	}

	for msg := range a.agentService.ChatWithAgentStream(ctx, q, aiConfigId, sysPromptId, false, 0, think, "plan_execute") {
		if msg == nil {
			continue
		}
		if msg.Content == "agent-DONE" {
			continue
		}
		runtime.EventsEmit(a.ctx, eventName, map[string]any{
			"content":           msg.Content,
			"reasoning_content": msg.ReasoningContent,
		})
	}

	a.summaryMu.Lock()
	a.summaryCancel = nil
	a.summaryMu.Unlock()

	runtime.EventsEmit(a.ctx, eventName, "DONE")
}

// preFetchMarketSummaryData collects macro-level market data for the AI market summary feature.

// preFetchMarketSummaryData collects macro-level market data for the AI market summary feature.
func (a *App) preFetchMarketSummaryData() string {
	var parts []string

	// Global indexes
	indexes := a.marketService.GlobalStockIndexesReadable()
	if indexes != "" {
		parts = append(parts, indexes)
	}

	return strings.Join(parts, "\n\n")
}

// @Description: // 获取 AiConfig 列表
// @receiver a
// @return error
func (a *App) GetAiConfigs() []*data.AIConfig {
	return a.settingsService.GetAiConfigs()
}

// GetAiAssistantSession 获取 AI 助手会话消息列表，sessionId 为空时获取最新的

// GetAiAssistantSession 获取 AI 助手会话消息列表，sessionId 为空时获取最新的
func (a *App) GetAiAssistantSession(sessionId string) (*models.AiAssistantSessionResp, error) {
	return a.agentService.GetAiAssistantSession(sessionId)
}

// SaveAiAssistantSession 保存 AI 助手会话消息到数据库

// SaveAiAssistantSession 保存 AI 助手会话消息到数据库
func (a *App) SaveAiAssistantSession(sessionId string, messages []models.AiAssistantMessage) error {
	return a.agentService.SaveAiAssistantSession(sessionId, messages)
}

// FetchAiModels 根据接口地址与 apiKey 自动获取支持的模型列表

// FetchAiModels 根据接口地址与 apiKey 自动获取支持的模型列表
func (a *App) FetchAiModels(baseUrl, apiKey string) []string {
	return a.settingsService.FetchAiModels(baseUrl, apiKey)
}

// TestAiConnection 真实调用 /chat/completions 验证 AI 配置连通性

// TestAiConnection 真实调用 /chat/completions 验证 AI 配置连通性
func (a *App) TestAiConnection(baseUrl, apiKey, modelName string) string {
	return a.settingsService.TestAiConnection(baseUrl, apiKey, modelName)
}

type AiModelInfo = services.AiModelInfo

func (a *App) FetchAiModelInfo(baseUrl, apiKey, modelName string) *AiModelInfo {
	info := a.settingsService.FetchAiModelInfo(baseUrl, apiKey, modelName)
	if info == nil {
		return nil
	}
	if info.MaxTokens == 0 {
		if maxTokens := getBuiltinModelMaxTokens(modelName); maxTokens > 0 {
			info.MaxTokens = maxTokens
			info.Source = "builtin"
		}
	}
	return info
}

func getBuiltinModelMaxTokens(modelName string) int {
	modelTokenMap := map[string]int{
		"deepseek-chat":        65536,
		"deepseek-reasoner":    65536,
		"deepseek-coder":       16384,
		"deepseek-v3":          65536,
		"deepseek-r1":          65536,
		"gpt-4o":               16384,
		"gpt-4o-mini":          16384,
		"gpt-4o-2024-05-13":    4096,
		"gpt-4-turbo":          4096,
		"gpt-4-turbo-preview":  4096,
		"gpt-4":                8192,
		"gpt-4-32k":            32768,
		"gpt-3.5-turbo":        4096,
		"gpt-3.5-turbo-16k":    16384,
		"gpt-4.1":              32768,
		"gpt-4.1-mini":         32768,
		"gpt-4.1-nano":         32768,
		"o1":                   100000,
		"o1-mini":              65536,
		"o1-preview":           32768,
		"o3-mini":              100000,
		"o4-mini":              100000,
		"claude-3-5-sonnet":    8192,
		"claude-3-5-haiku":     8192,
		"claude-3-opus":        4096,
		"claude-3-sonnet":      4096,
		"claude-3-haiku":       4096,
		"glm-4":                8192,
		"glm-4-plus":           4096,
		"glm-4-air":            4096,
		"glm-4-flash":          4096,
		"glm-4-long":           4096,
		"chatglm-turbo":        4096,
		"moonshot-v1-8k":       8192,
		"moonshot-v1-32k":      32768,
		"moonshot-v1-128k":     131072,
		"qwen-turbo":           8192,
		"qwen-plus":            131072,
		"qwen-max":             8192,
		"qwen-long":            65536,
		"qwen2.5-72b-instruct": 32768,
		"hunyuan-lite":         4096,
		"hunyuan-standard":     4096,
		"hunyuan-pro":          4096,
		"hunyuan-turbo":        4096,
		"spark-lite":           4096,
		"spark-pro":            4096,
		"spark-max":            4096,
		"spark-4.0-ultra":      4096,
		"yi-light":             16384,
		"yi-large":             16384,
		"yi-medium":            16384,
		"yi-spark":             16384,
		"yi-vision":            16384,
		"abab6.5-chat":         8192,
		"abab6.5s-chat":        8192,
		"abab5.5-chat":         4096,
		"baichuan2-turbo":      4096,
		"baichuan2-53b":        4096,
		"ernie-4.0":            4096,
		"ernie-3.5":            4096,
		"ernie-speed":          4096,
		"ernie-lite":           4096,
	}

	if maxTokens, ok := modelTokenMap[modelName]; ok {
		return maxTokens
	}

	for prefix, maxTokens := range map[string]int{
		"deepseek":      65536,
		"gpt-4o":        16384,
		"gpt-4-turbo":   4096,
		"gpt-4-":        8192,
		"gpt-3.5":       4096,
		"gpt-4.1":       32768,
		"o1-":           65536,
		"o3-":           100000,
		"o4-":           100000,
		"claude-3":      8192,
		"glm-4":         8192,
		"chatglm":       4096,
		"moonshot-v1":   8192,
		"qwen-":         8192,
		"qwen2":         32768,
		"hunyuan-":      4096,
		"spark-":        4096,
		"yi-":           16384,
		"abab":          8192,
		"baichuan":      4096,
		"ernie-":        4096,
		"llama-3":       8192,
		"llama3":        8192,
		"mistral-":      8192,
		"mixtral-":      32768,
		"codestral-":    32768,
		"gemini-1.5":    8192,
		"gemini-2":      8192,
		"command-r":     4096,
		"Qwen/Qwen":     32768,
		"deepseek-ai/":  65536,
		"meta-llama/":   8192,
		"mistralai/":    32768,
		"Pro/deepseek-": 65536,
		"Pro/qwen-":     32768,
	} {
		if strings.HasPrefix(modelName, prefix) {
			return maxTokens
		}
	}

	return 0
}

// InitCronTasks 在应用启动时，自动为启用状态的定时任务创建调度

// AbortSummaryStockNews 取消当前进行中的 SummaryStockNews 流式回答
func (a *App) AbortSummaryStockNews() {
	a.summaryMu.Lock()
	defer a.summaryMu.Unlock()
	if a.summaryCancel != nil {
		a.summaryCancel()
		a.summaryCancel = nil
	}
}

// CreateCronTask
//
//	@Description: 创建定时任务
//	@receiver a
//	@param task 定时任务信息
//	@return string 操作结果

// @Description: 获取市场生命周期状态（盘前/集合竞价/早盘/午休/下午盘/盘后/休市）
// @receiver a
// @return agent.MarketLifecycleStatus
func (a *App) GetMarketLifecycleStatus() agent.MarketLifecycleStatus {
	return a.agentService.GetMarketLifecycleStatus(a.ctx)
}

// GeneratePreMarketBriefing 生成盘前智能前瞻晨报

// GeneratePreMarketBriefing 生成盘前智能前瞻晨报
func (a *App) GeneratePreMarketBriefing(aiConfigId int) *agent.MarketIntelligenceReport {
	return a.agentService.GeneratePreMarketBriefing(a.ctx, aiConfigId)
}

// GeneratePostMarketReview 生成盘后收盘深度复盘

// GeneratePostMarketReview 生成盘后收盘深度复盘
func (a *App) GeneratePostMarketReview(aiConfigId int) *agent.MarketIntelligenceReport {
	return a.agentService.GeneratePostMarketReview(a.ctx, aiConfigId)
}

// GetLatestMarketIntelligenceReport 获取最近一次盘前/盘后智能研报

// GetLatestMarketIntelligenceReport 获取最近一次盘前/盘后智能研报
func (a *App) GetLatestMarketIntelligenceReport() *agent.MarketIntelligenceReport {
	return a.agentService.GetLatestMarketIntelligenceReport(a.ctx)
}

// GetLatestMarketIntelligenceReportByType 获取指定类型的最新研报

// GetLatestMarketIntelligenceReportByType 获取指定类型的最新研报
func (a *App) GetLatestMarketIntelligenceReportByType(reportType string) *agent.MarketIntelligenceReport {
	return a.agentService.GetLatestMarketIntelligenceReportByType(a.ctx, reportType)
}

// PurgeCorruptedMarketReports 清理数据库与内存中所有残损异常研报

// PurgeCorruptedMarketReports 清理数据库与内存中所有残损异常研报
func (a *App) PurgeCorruptedMarketReports() int {
	return a.agentService.PurgeCorruptedMarketReports(a.ctx)
}

// ClearMarketIntelligenceReport 清除指定类型的市场智能研报（内存缓存 + 归档记录）。
// reportType 为空字符串表示清除全部类型。
// 这是用户明确要求补齐的能力：此前研报只活在进程内存里，既无 API 也无 UI 可以删除，
// 只能重启容器。

// ClearMarketIntelligenceReport 清除指定类型的市场智能研报（内存缓存 + 归档记录）。
// reportType 为空字符串表示清除全部类型。
// 这是用户明确要求补齐的能力：此前研报只活在进程内存里，既无 API 也无 UI 可以删除，
// 只能重启容器。
func (a *App) ClearMarketIntelligenceReport(reportType string) string {
	return a.agentService.ClearMarketIntelligenceReport(a.ctx, reportType)
}

// ClearAllMarketIntelligenceReports 清除全部市场智能研报与对应归档记录

// ClearAllMarketIntelligenceReports 清除全部市场智能研报与对应归档记录
func (a *App) ClearAllMarketIntelligenceReports() string {
	return a.agentService.ClearAllMarketIntelligenceReports(a.ctx)
}

// GetRecentAgentRuns 返回最近的 Agent / 研报运行记录（倒序）。
// 每条记录同时包含“请求的模型”与“实际使用的模型”及解析依据，
// 用于解决“到底哪个模型在跑”的问题。

// GetRecentAgentRuns 返回最近的 Agent / 研报运行记录（倒序）。
// 每条记录同时包含“请求的模型”与“实际使用的模型”及解析依据，
// 用于解决“到底哪个模型在跑”的问题。
func (a *App) GetRecentAgentRuns(limit int) []models.AgentRunRecord {
	return a.agentService.GetRecentAgentRuns(limit)
}

// ClearAgentRuns 清空运行记录，返回删除条数

// ClearAgentRuns 清空运行记录，返回删除条数
func (a *App) ClearAgentRuns() int64 {
	return a.agentService.ClearAgentRuns()
}

// SearchCronTasks
//
//	@Description: 搜索定时任务
//	@receiver a
//	@param keyword 搜索关键词
//	@return []models.CronTask 搜索结果

func (a *App) CreateMCPServer(server *models.MCPServer) string {
	return a.agentService.CreateMCPServer(server)
}

func (a *App) UpdateMCPServer(server *models.MCPServer) string {
	return a.agentService.UpdateMCPServer(server)
}

func (a *App) DeleteMCPServer(id uint) string {
	return a.agentService.DeleteMCPServer(id)
}

func (a *App) GetMCPServerByID(id uint) *models.MCPServer {
	return a.agentService.GetMCPServerByID(id)
}

func (a *App) GetMCPServerList(query *models.MCPServerQuery) *models.MCPServerPageResp {
	return a.agentService.GetMCPServerList(query)
}

func (a *App) EnableMCPServer(id uint, enable bool) string {
	return a.agentService.EnableMCPServer(id, enable)
}

func (a *App) TestMCPServer(id uint) string {
	return a.agentService.TestMCPServer(id)
}

func (a *App) CreateSkill(skill *models.Skill) string {
	return a.agentService.CreateSkill(skill)
}

func (a *App) UpdateSkill(skill *models.Skill) string {
	return a.agentService.UpdateSkill(skill)
}

func (a *App) DeleteSkill(id uint) string {
	return a.agentService.DeleteSkill(id)
}

func (a *App) GetSkillByID(id uint) *models.Skill {
	return a.agentService.GetSkillByID(id)
}

func (a *App) GetSkillList(query *models.SkillQuery) *models.SkillPageResp {
	return a.agentService.GetSkillList(query)
}

func (a *App) EnableSkill(id uint, enable bool) string {
	return a.agentService.EnableSkill(id, enable)
}

func (a *App) GetAllSkills() []models.Skill {
	return a.agentService.GetAllSkills()
}

func (a *App) LearnSkillFromProject(projectName string, analysis string) string {
	return a.agentService.LearnSkillFromProject(projectName, analysis)
}

func (a *App) GetMCPToolsByServerID(serverID uint) []models.MCPServerTool {
	return a.agentService.GetMCPToolsByServerID(serverID)
}

func (a *App) EnableMCPTool(id uint, enable bool) string {
	return a.agentService.EnableMCPTool(id, enable)
}

func (a *App) GetAllMCPTools() []models.MCPServerTool {
	return a.agentService.GetAllMCPTools()
}

type PromptTemplateItem = services.PromptTemplateItem

func (a *App) GetPromptTemplates(name, promptType string) []PromptTemplateItem {
	return a.agentService.GetPromptTemplates(name, promptType)
}

// ============================================================================
// VectorBT 向量化回测 API - Wails 暴露接口
// ============================================================================

// VectorBTResult 向量化回测结果

// ReloadSkills 热重载技能定义，无需重启应用
func (a *App) ReloadSkills() error {
	return a.agentService.ReloadSkills()
}

// GetAIResponseResult 获取指定标的最新AI分析结果

// GetAIResponseResult 获取指定标的最新AI分析结果
func (a *App) GetAIResponseResult(stock string) *models.AIResponseResult {
	return data.NewDeepSeekOpenAi(a.ctx, 0).GetAIResponseResult(stock)
}

// FetchAuditLogs 前端兼容别名：分页获取审计日志

// FetchSkillChanges 查询技能审核与变更流水
func (a *App) FetchSkillChanges(query any) (any, error) {
	return a.agentService.FetchSkillChanges(query)
}

// ApproveSkillChange 批准技能发布/更新

// ApproveSkillChange 批准技能发布/更新
func (a *App) ApproveSkillChange(id any) (any, error) {
	return a.agentService.ApproveSkillChange(id)
}

// RejectSkillChange 驳回技能发布/更新

// RejectSkillChange 驳回技能发布/更新
func (a *App) RejectSkillChange(id any) (any, error) {
	return a.agentService.RejectSkillChange(id)
}

// BatchApproveSkillChanges 批量批准技能发布

// BatchApproveSkillChanges 批量批准技能发布
func (a *App) BatchApproveSkillChanges(ids any) (any, error) {
	return a.agentService.BatchApproveSkillChanges(ids)
}

// BatchRejectSkillChanges 批量驳回技能发布

// BatchRejectSkillChanges 批量驳回技能发布
func (a *App) BatchRejectSkillChanges(ids any) (any, error) {
	return a.agentService.BatchRejectSkillChanges(ids)
}

// ============================================================================
// AI 语义向量精炼与知识结晶 (Semantic Distillation & Knowledge Crystallization)
// ============================================================================

// TriggerSemanticDistillation 手动或一键触发全局语义精炼、知识结晶与存储自愈

// TriggerSemanticDistillation 手动或一键触发全局语义精炼、知识结晶与存储自愈
func (a *App) TriggerSemanticDistillation() (*agent.DistillationStats, error) {
	return a.agentService.TriggerSemanticDistillation(a.ctx)
}

// GetDistillationStats 获取当前语义精炼、结晶总数与瘦身指标

// GetDistillationStats 获取当前语义精炼、结晶总数与瘦身指标
func (a *App) GetDistillationStats() (*agent.DistillationStats, error) {
	return a.agentService.GetDistillationStats(a.ctx)
}

// GetKnowledgeCrystals 获取平台长期沉淀凝练的高胜率经验结晶库

// GetKnowledgeCrystals 获取平台长期沉淀凝练的高胜率经验结晶库
func (a *App) GetKnowledgeCrystals(category string, limit int) ([]agent.KnowledgeCrystal, error) {
	return a.agentService.GetKnowledgeCrystals(a.ctx, category, limit)
}

// ============================================================================
// 自主智能体闭环：死后剖析对账、模拟盘问责、策略自进化与产业链图谱
// ============================================================================

// GetPaperPortfolioStats 获取智能体专属模拟基金账户报表（净值NAV、持仓、夏普、回撤与胜率）
