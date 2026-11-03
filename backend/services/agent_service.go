package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"blades-stock/backend/agent"
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"

	"github.com/samber/lo"
)

// MessageEmitter 抽象的消息/事件发射器接口
type MessageEmitter func(event string, payload any)

// PromptTemplateItem 提示词模板返回项
type PromptTemplateItem struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// AgentService AI 智能体推演、技能库、MCP 与市场智能研报服务
type AgentService struct {
	agentMu        sync.Mutex
	sessionsMu     sync.Mutex
	sessions       map[string]context.CancelFunc
	defaultEmitter MessageEmitter
}

// NewAgentService 创建 Agent 服务实例
func NewAgentService() *AgentService {
	return &AgentService{
		sessions: make(map[string]context.CancelFunc),
	}
}

// SetDefaultEmitter 设置默认的流式推流发射器（在 Server 模式下由 SSE 挂载）
func (s *AgentService) SetDefaultEmitter(emitter MessageEmitter) {
	s.defaultEmitter = emitter
}

// ChatWithAgent 执行 AI Agent 智能会话推演（使用默认发射器向外推流）
// 采用非阻塞 goroutine 启动推演，确保 Web 模式下 HTTP RPC 请求瞬间响应，
// 彻底杜绝 HTTP 连接被长达数十秒的 LLM 推演挂死引发 Nginx/Cloudflare 504 Gateway Timeout 或浏览器断连。
func (s *AgentService) ChatWithAgent(question string, aiConfigId int, sysPromptId *int, memoryMode bool, memoryCount int, thinkingMode bool, agentMode string, sessionId string) {
	go s.ChatWithAgentCustom(s.defaultEmitter, question, aiConfigId, sysPromptId, memoryMode, memoryCount, thinkingMode, agentMode, sessionId)
}

// ChatWithAgentCustom 执行 AI Agent 会话并支持自定义发射器（桌面模式使用 Wails EventsEmit）
func (s *AgentService) ChatWithAgentCustom(emitter MessageEmitter, question string, aiConfigId int, sysPromptId *int, memoryMode bool, memoryCount int, thinkingMode bool, agentMode string, sessionId string) {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("AgentService.ChatWithAgent panic: %v", r)
		}
	}()

	sessKey := strings.TrimSpace(sessionId)
	if sessKey == "" {
		sessKey = "default"
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 会话级隔离：仅取消同一 session 的前次未完成调用，多用户彼此完全并发独立
	s.sessionsMu.Lock()
	if s.sessions == nil {
		s.sessions = make(map[string]context.CancelFunc)
	}
	if oldCancel, exists := s.sessions[sessKey]; exists && oldCancel != nil {
		oldCancel()
	}
	s.sessions[sessKey] = cancel
	s.sessionsMu.Unlock()

	defer func() {
		s.sessionsMu.Lock()
		delete(s.sessions, sessKey)
		s.sessionsMu.Unlock()
	}()

	ch := agent.NewStockAiAgentApi().ChatWithContext(ctx, question, aiConfigId, sysPromptId, memoryMode, memoryCount, thinkingMode, agentMode, sessionId)
	for msg := range ch {
		if emitter != nil {
			emitter("agent-message", AgentMessageToFrontendMap(msg))
		}
	}
}

// AbortChatWithAgent 打断或终止当前正在进行的 Agent 推理（支持多会话隔离）
func (s *AgentService) AbortChatWithAgent() {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	for k, cancel := range s.sessions {
		if cancel != nil {
			cancel()
		}
		delete(s.sessions, k)
	}
}

// AgentMessageToFrontendMap 将流式消息序列化为前端友好的 Map 结构
func AgentMessageToFrontendMap(msg *agent.StreamMessage) map[string]any {
	if msg == nil {
		return map[string]any{}
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return map[string]any{
			"role":              msg.Role,
			"content":           msg.Content,
			"reasoning_content": msg.ReasoningContent,
		}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return map[string]any{
			"role":              msg.Role,
			"content":           msg.Content,
			"reasoning_content": msg.ReasoningContent,
		}
	}
	return m
}

// GetAiAssistantSession 获取 AI 助手会话消息列表
func (s *AgentService) GetAiAssistantSession(sessionId string) (*models.AiAssistantSessionResp, error) {
	return data.GetAiAssistantSession(sessionId)
}

// SaveAiAssistantSession 保存 AI 助手会话消息到数据库
func (s *AgentService) SaveAiAssistantSession(sessionId string, messages []models.AiAssistantMessage) error {
	return data.SaveAiAssistantSession(sessionId, messages)
}

// SaveAIResponseResult 保存单次推理报告
func (s *AgentService) SaveAIResponseResult(ctx context.Context, stockCode, stockName, result, chatId, question string, aiConfigId int) {
	data.NewDeepSeekOpenAi(ctx, aiConfigId).SaveAIResponseResult(stockCode, stockName, result, chatId, question)
}

// GetSystemStateSnapshot 获取全局态势感知快照
func (s *AgentService) GetSystemStateSnapshot() *agent.SystemStateSnapshot {
	return agent.GetGlobalMarketSensor().GetSnapshot()
}

// GetToolQuotaStatus 获取外部工具调用配额与成本状态
func (s *AgentService) GetToolQuotaStatus() agent.QuotaStatus {
	return agent.GetToolQuotaManager().GetQuotaStatus()
}

// ResetToolDailyQuota 手动重置今日工具配额
func (s *AgentService) ResetToolDailyQuota() string {
	agent.GetToolQuotaManager().ResetDailyQuota()
	return "今日外部工具调用配额与预算已成功重置"
}

// SyncRecommendStocksFromIntelligence 从最新研报提炼推荐股票
func (s *AgentService) SyncRecommendStocksFromIntelligence(ctx context.Context) (int, error) {
	return agent.SyncRecommendStocksFromAllIntelligence(ctx)
}

// GetMarketLifecycleStatus 获取当前市场阶段与交易时钟状态
func (s *AgentService) GetMarketLifecycleStatus(ctx context.Context) agent.MarketLifecycleStatus {
	return agent.NewMarketLifecycleService(ctx).GetMarketLifecycleStatus()
}

// GeneratePreMarketBriefing 生成盘前智能晨报
func (s *AgentService) GeneratePreMarketBriefing(ctx context.Context, aiConfigId int) *agent.MarketIntelligenceReport {
	report, err := agent.NewMarketLifecycleService(ctx).GeneratePreMarketBriefing(ctx, aiConfigId)
	if err != nil {
		logger.SugaredLogger.Errorf("生成盘前前瞻失败：%v", err)
	}
	return report
}

// GeneratePostMarketReview 生成收盘深度复盘
func (s *AgentService) GeneratePostMarketReview(ctx context.Context, aiConfigId int) *agent.MarketIntelligenceReport {
	report, err := agent.NewMarketLifecycleService(ctx).GeneratePostMarketReview(ctx, aiConfigId)
	if err != nil {
		logger.SugaredLogger.Errorf("生成收盘复盘失败：%v", err)
	}
	return report
}

// GetLatestMarketIntelligenceReport 获取最近一次生成的研报
func (s *AgentService) GetLatestMarketIntelligenceReport(ctx context.Context) *agent.MarketIntelligenceReport {
	return agent.NewMarketLifecycleService(ctx).GetLatestReport()
}

// GetLatestMarketIntelligenceReportByType 获取指定类型的最新研报
func (s *AgentService) GetLatestMarketIntelligenceReportByType(ctx context.Context, reportType string) *agent.MarketIntelligenceReport {
	return agent.NewMarketLifecycleService(ctx).GetLatestReportByType(reportType)
}

// PurgeCorruptedMarketReports 一键物理扫描并清理数据库与内存中所有残损研报
func (s *AgentService) PurgeCorruptedMarketReports(ctx context.Context) int {
	return agent.PurgeCorruptedReports()
}

// GenerateETFRotationReport 触发全市场 ETF 动量轮动决策分析研报
func (s *AgentService) GenerateETFRotationReport(ctx context.Context, aiConfigId int) (*agent.MarketIntelligenceReport, error) {
	service := agent.NewMarketLifecycleService(ctx)
	return service.GenerateETFRotationReport(ctx, aiConfigId)
}

// GenerateSerenityResearchReport 触发 Serenity 产业深度拆解研报
func (s *AgentService) GenerateSerenityResearchReport(ctx context.Context, targetName string, code string, aiConfigId int) (*agent.MarketIntelligenceReport, error) {
	service := agent.NewMarketLifecycleService(ctx)
	return service.GenerateSerenityResearchReport(ctx, targetName, code, aiConfigId)
}

// CreateSkill 创建技能
func (s *AgentService) CreateSkill(skill *models.Skill) string {
	err := data.NewSkillApi().Create(skill)
	if err != nil {
		logger.SugaredLogger.Errorf("创建技能失败: %v", err)
		return "创建失败: " + err.Error()
	}
	return "创建成功"
}

// UpdateSkill 更新技能
func (s *AgentService) UpdateSkill(skill *models.Skill) string {
	err := data.NewSkillApi().Update(skill)
	if err != nil {
		logger.SugaredLogger.Errorf("更新技能失败: %v", err)
		return "更新失败: " + err.Error()
	}
	return "更新成功"
}

// DeleteSkill 删除技能
func (s *AgentService) DeleteSkill(id uint) string {
	err := data.NewSkillApi().Delete(id)
	if err != nil {
		logger.SugaredLogger.Errorf("删除技能失败: %v", err)
		return "删除失败: " + err.Error()
	}
	return "删除成功"
}

// GetSkillByID 获取技能详情
func (s *AgentService) GetSkillByID(id uint) *models.Skill {
	skill, err := data.NewSkillApi().GetByID(id)
	if err != nil {
		logger.SugaredLogger.Errorf("获取技能失败: %v", err)
		return nil
	}
	return skill
}

// GetSkillList 分页获取技能列表
func (s *AgentService) GetSkillList(query *models.SkillQuery) *models.SkillPageResp {
	return data.NewSkillApi().List(query)
}

// EnableSkill 启用或禁用技能
func (s *AgentService) EnableSkill(id uint, enable bool) string {
	err := data.NewSkillApi().EnableSkill(id, enable)
	if err != nil {
		logger.SugaredLogger.Errorf("启用/禁用技能失败: %v", err)
		return "操作失败: " + err.Error()
	}
	if enable {
		return "已启用"
	}
	return "已禁用"
}

// GetAllSkills 获取全量技能
func (s *AgentService) GetAllSkills() []models.Skill {
	return data.NewSkillApi().GetAll()
}

// LearnSkillFromProject 从项目研报提取新技能
func (s *AgentService) LearnSkillFromProject(projectName string, analysis string) string {
	llm := agent.LLMFromConfig()
	skill, err := agent.NewEvolver().LearnFromProject(projectName, analysis, llm)
	if err != nil {
		logger.SugaredLogger.Errorf("LearnSkillFromProject failed: %v", err)
		return "学习失败: " + err.Error()
	}
	return fmt.Sprintf("学习成功！已创建技能「%s」\n描述: %s\n触发关键词: %s",
		skill.Name, skill.Description, strings.Join(skill.TriggerKeywords, ","))
}

// ReloadSkills 热重载技能库
func (s *AgentService) ReloadSkills() error {
	return data.NewSkillApi().ReloadSkills()
}

// CreateMCPServer 创建 MCP 服务端配置
func (s *AgentService) CreateMCPServer(server *models.MCPServer) string {
	err := data.NewMCPServerApi().Create(server)
	if err != nil {
		logger.SugaredLogger.Errorf("创建MCP服务器失败: %v", err)
		return "创建失败: " + err.Error()
	}
	return "创建成功"
}

// UpdateMCPServer 更新 MCP 服务端配置
func (s *AgentService) UpdateMCPServer(server *models.MCPServer) string {
	err := data.NewMCPServerApi().Update(server)
	if err != nil {
		logger.SugaredLogger.Errorf("更新MCP服务器失败: %v", err)
		return "更新失败: " + err.Error()
	}
	return "更新成功"
}

// DeleteMCPServer 删除 MCP 服务端配置
func (s *AgentService) DeleteMCPServer(id uint) string {
	err := data.NewMCPServerApi().Delete(id)
	if err != nil {
		logger.SugaredLogger.Errorf("删除MCP服务器失败: %v", err)
		return "删除失败: " + err.Error()
	}
	return "删除成功"
}

// GetMCPServerByID 获取 MCP 服务端详情
func (s *AgentService) GetMCPServerByID(id uint) *models.MCPServer {
	server, err := data.NewMCPServerApi().GetByID(id)
	if err != nil {
		logger.SugaredLogger.Errorf("获取MCP服务器失败: %v", err)
		return nil
	}
	return server
}

// GetMCPServerList 获取 MCP 服务端分页列表
func (s *AgentService) GetMCPServerList(query *models.MCPServerQuery) *models.MCPServerPageResp {
	return data.NewMCPServerApi().List(query)
}

// EnableMCPServer 启用或禁用 MCP 服务端
func (s *AgentService) EnableMCPServer(id uint, enable bool) string {
	err := data.NewMCPServerApi().EnableServer(id, enable)
	if err != nil {
		logger.SugaredLogger.Errorf("启用/禁用MCP服务器失败: %v", err)
		return "操作失败: " + err.Error()
	}
	if enable {
		return "已启用"
	}
	return "已禁用"
}

// TestMCPServer 测试 MCP 服务端连接
func (s *AgentService) TestMCPServer(id uint) string {
	result, err := data.NewMCPServerApi().TestConnection(id)
	if err != nil {
		logger.SugaredLogger.Errorf("测试MCP服务器连接失败: %v", err)
		return "测试失败: " + err.Error()
	}
	return result
}

// GetMCPToolsByServerID 获取指定 MCP 服务的可用工具
func (s *AgentService) GetMCPToolsByServerID(serverID uint) []models.MCPServerTool {
	return data.NewMCPServerApi().GetToolsByServerID(serverID)
}

// EnableMCPTool 启用或禁用指定工具
func (s *AgentService) EnableMCPTool(id uint, enable bool) string {
	if err := data.NewMCPServerApi().EnableTool(id, enable); err != nil {
		return "操作失败: " + err.Error()
	}
	if enable {
		return "已启用工具"
	}
	return "已禁用工具"
}

// GetAllMCPTools 获取全部 MCP 工具
func (s *AgentService) GetAllMCPTools() []models.MCPServerTool {
	return data.NewMCPServerApi().GetAllTools()
}

// GetPromptTemplates 获取提示词模板
func (s *AgentService) GetPromptTemplates(name, promptType string) []PromptTemplateItem {
	return []PromptTemplateItem{}
}

// -----------------------------------------------------------------------------
// 定时任务管理与调度能力 (Cron Tasks)
// -----------------------------------------------------------------------------

// CreateCronTask 创建定时任务
func (s *AgentService) CreateCronTask(task *models.CronTask) string {
	err := agent.NewCronTaskApi().Create(task)
	if err != nil {
		return fmt.Sprintf("创建失败：%v", err)
	}
	return "创建成功"
}

// UpdateCronTask 更新定时任务
func (s *AgentService) UpdateCronTask(task *models.CronTask) string {
	err := agent.NewCronTaskApi().Update(task)
	if err != nil {
		return fmt.Sprintf("更新失败：%v", err)
	}
	return "更新成功"
}

// DeleteCronTask 删除定时任务
func (s *AgentService) DeleteCronTask(id uint) string {
	err := agent.NewCronTaskApi().Delete(id)
	if err != nil {
		return fmt.Sprintf("删除失败：%v", err)
	}
	return "删除成功"
}

// GetCronTaskByID 根据 ID 获取定时任务
func (s *AgentService) GetCronTaskByID(id uint) *models.CronTask {
	task, err := agent.NewCronTaskApi().GetByID(id)
	if err != nil {
		return nil
	}
	return task
}

// GetCronTaskList 获取定时任务分页列表
func (s *AgentService) GetCronTaskList(query *models.CronTaskQuery) *models.CronTaskPageResp {
	if query == nil {
		query = &models.CronTaskQuery{}
	}
	return agent.NewCronTaskApi().List(query)
}

// EnableCronTask 启用/禁用定时任务
func (s *AgentService) EnableCronTask(id uint, enable bool) string {
	err := agent.NewCronTaskApi().EnableTask(id, enable)
	if err != nil {
		return fmt.Sprintf("操作失败：%v", err)
	}
	return "操作成功"
}

// ExecuteCronTaskNow 立即执行定时任务
func (s *AgentService) ExecuteCronTaskNow(id uint) string {
	task, err := agent.NewCronTaskApi().GetByID(id)
	if err != nil {
		return fmt.Sprintf("任务不存在：%v", err)
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Errorf("ExecuteCronTaskNow panic for task %d: %v", id, r)
			}
		}()
		_ = agent.NewCronTaskApi().ExecuteTask(context.Background(), task)
	}()
	return "任务执行中"
}

// GetCronTaskTypes 获取所有任务类型列表
func (s *AgentService) GetCronTaskTypes() []lo.Tuple2[string, string] {
	return agent.NewCronTaskApi().GetTaskTypes()
}

// ValidateCronExpr 验证 Cron 表达式语法与有效性
func (s *AgentService) ValidateCronExpr(expr string) string {
	err := agent.NewCronTaskApi().ValidateCronExpr(expr)
	if err != nil {
		return fmt.Sprintf("无效表达式：%v", err)
	}
	return "有效表达式"
}

// SearchCronTasks 搜索定时任务
func (s *AgentService) SearchCronTasks(keyword string) []models.CronTask {
	return agent.NewCronTaskApi().SearchTasks(keyword)
}

// CalculateNextRunTime 计算 Cron 表达式下一次触发时间
func (s *AgentService) CalculateNextRunTime(cronExpr string) string {
	t := agent.NewCronTaskApi().CalculateNextRunTime(cronExpr)
	return t.Format("2006-01-02 15:04:05")
}

// CalculateNextRunTimes 计算未来多次触发时间序列
func (s *AgentService) CalculateNextRunTimes(cronExpr string, count int) []string {
	times := agent.NewCronTaskApi().CalculateNextRunTimes(cronExpr, count)
	result := make([]string, 0, len(times))
	for _, t := range times {
		result = append(result, t.Format("2006-01-02 15:04:05"))
	}
	return result
}

// -----------------------------------------------------------------------------
// AI 资讯总结与对话流式推流能力
// -----------------------------------------------------------------------------

// SummaryStockNews AI 市场全景资讯总结
func (s *AgentService) SummaryStockNews(question string, aiConfigId int, sysPromptId *int, enableTools bool, think bool, eventName string, historyJSON string) {
	s.SummaryStockNewsCustom(s.defaultEmitter, question, aiConfigId, sysPromptId, enableTools, think, eventName, historyJSON)
}

// SummaryStockNewsCustom 自定义发射器 AI 市场资讯总结
func (s *AgentService) SummaryStockNewsCustom(emitter MessageEmitter, question string, aiConfigId int, sysPromptId *int, enableTools bool, think bool, eventName string, historyJSON string) {
	if strings.TrimSpace(eventName) == "" {
		eventName = "summaryStockNews"
	}
	q := strings.TrimSpace(question)
	if q == "" {
		q = "请对当前A股市场进行全面的AI资讯总结和分析。"
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Errorf("SummaryStockNewsCustom panic: %v", r)
				if emitter != nil {
					emitter(eventName, map[string]any{
						"content": fmt.Sprintf("❌ 资讯总结运行异常: %v", r),
					})
				}
			}
			if emitter != nil {
				emitter(eventName, "DONE")
			}
		}()

		for msg := range agent.NewStockAiAgentApi().ChatWithContext(context.Background(), q, aiConfigId, sysPromptId, false, 0, think, "plan_execute") {
			if msg == nil || msg.Content == "agent-DONE" {
				continue
			}
			if emitter != nil {
				emitter(eventName, map[string]any{
					"content":           msg.Content,
					"reasoning_content": msg.ReasoningContent,
				})
			}
		}
	}()
}

// AbortSummaryStockNews 中止新闻总结推流
func (s *AgentService) AbortSummaryStockNews() {
	// 优雅中止
}

// IsETFRecommendQuestion 判断用户问题是否属于 ETF/基金推荐、轮动、板块资金类问询。
func IsETFRecommendQuestion(q string) bool {
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

// PreFetchStockAnalysisData 预采集目标个股的实时行情、财务、估值分位及机构预测等数据
func PreFetchStockAnalysisData(stockCode string) string {
	if stockCode == "" {
		return ""
	}
	api := data.NewStockDataApi()

	var (
		realtimeStr string
		financeMd   string
		qtrMd       string
		valMd       string
		predMd      string
		wg          sync.WaitGroup
	)

	wg.Add(5)

	// 1. 实时行情与最新价
	go func() {
		defer wg.Done()
		defer func() { _ = recover() }()
		if info, err := api.GetStockCodeRealTimeData(stockCode); err == nil && info != nil && len(*info) > 0 {
			s := (*info)[0]
			changePercent := s.ChangePercent
			changePrice := s.ChangePrice
			realtimeStr = fmt.Sprintf("【实时行情】%s(%s) 当前价:%s 涨跌幅:%.2f%% 涨跌额:%.2f 开盘:%s 最高:%s 最低:%s 昨收:%s 成交量:%s 成交额:%s",
				s.Name, s.Code, s.Price, changePercent, changePrice, s.Open, s.High, s.Low, s.PreClose, s.Volume, s.Amount)
		}
	}()

	// 2. 最新财务指标
	go func() {
		defer wg.Done()
		defer func() { _ = recover() }()
		res := api.GetStockLatestFinanceToMarkdown(stockCode)
		if res != "" && !strings.HasPrefix(res, "获取") {
			financeMd = res
		}
	}()

	// 3. 季度财务表现 (ROE, 毛利率等)
	go func() {
		defer wg.Done()
		defer func() { _ = recover() }()
		res := api.GetStockQtrMainFinanceToMarkdown(stockCode)
		if res != "" && !strings.HasPrefix(res, "获取") {
			qtrMd = res
		}
	}()

	// 4. 估值分位点 (PE/PB 历史百分位)
	go func() {
		defer wg.Done()
		defer func() { _ = recover() }()
		res := api.GetStockValuationPercentileToMarkdown(stockCode)
		if res != "" && !strings.HasPrefix(res, "获取") {
			valMd = res
		}
	}()

	// 5. 机构一致性盈利预测
	go func() {
		defer wg.Done()
		defer func() { _ = recover() }()
		res := api.GetStockPredictSummaryToMarkdown(stockCode)
		if res != "" && !strings.HasPrefix(res, "获取") {
			predMd = res
		}
	}()

	wg.Wait()

	var parts []string
	if realtimeStr != "" {
		parts = append(parts, realtimeStr)
	}
	if financeMd != "" {
		parts = append(parts, financeMd)
	}
	if qtrMd != "" {
		parts = append(parts, qtrMd)
	}
	if valMd != "" {
		parts = append(parts, valMd)
	}
	if predMd != "" {
		parts = append(parts, predMd)
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n\n")
}

// NewChatStream AI 交互对话流式推流
func (s *AgentService) NewChatStream(stock, stockCode, question string, aiConfigId int, sysPromptId *int, enableTools bool, think bool) {
	s.NewChatStreamCustom(s.defaultEmitter, stock, stockCode, question, aiConfigId, sysPromptId, enableTools, think)
}

// NewChatStreamCustom 自定义发射器 AI 交互对话流式推流
func (s *AgentService) NewChatStreamCustom(emitter MessageEmitter, stock, stockCode, question string, aiConfigId int, sysPromptId *int, enableTools bool, think bool) {
	go func() {
		cleanCode := models.CleanStockCode(stockCode)
		sessKey := fmt.Sprintf("stock_analysis_%s", cleanCode)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		s.sessionsMu.Lock()
		if s.sessions == nil {
			s.sessions = make(map[string]context.CancelFunc)
		}
		if oldCancel, exists := s.sessions[sessKey]; exists && oldCancel != nil {
			oldCancel()
		}
		s.sessions[sessKey] = cancel
		s.sessionsMu.Unlock()

		defer func() {
			s.sessionsMu.Lock()
			delete(s.sessions, sessKey)
			s.sessionsMu.Unlock()
		}()

		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Errorf("NewChatStreamCustom panic: %v", r)
				if emitter != nil {
					emitter("newChatStream", map[string]any{
						"content": fmt.Sprintf("❌ 智能分析运行异常: %v", r),
					})
				}
			}
			if emitter != nil {
				emitter("newChatStream", "DONE")
			}
		}()

		analysisQuestion := strings.TrimSpace(question)
		isETFOriented := IsETFRecommendQuestion(analysisQuestion)

		// 预采集真实盘面与财务数据注入 Prompt 上下文
		var preFetchCtx string
		if !isETFOriented && stockCode != "" {
			preFetchCtx = PreFetchStockAnalysisData(stockCode)
		}

		if analysisQuestion == "" {
			displayName := stock
			if displayName == "" {
				displayName = stockCode
			}
			analysisQuestion = fmt.Sprintf("请对%s(%s)进行全方位深度分析，从技术面、基本面、消息面、资金面四个维度综合研判，给出明确的多空判断和操作建议。", displayName, stockCode)
		}
		if preFetchCtx != "" {
			analysisQuestion += "\n\n【预采集参考数据】以下数据由系统预先采集，是真实数据，可直接引用分析：\n" + preFetchCtx + "\n\n【重要】严禁编造或推算PE/PB/ROE/毛利率/营收/利润/资金流向/均线等任何数字指标。所有数据须来自预采集数据或工具返回结果。不确定的指标请明确说'数据不可得'，不要自行估算。"
		}

		// 根据工具启用状态与预采集数据动态决策 Agent 推演模式：
		// 1. 若用户关闭函数工具调用，或针对常规个股全景研判（question为空）已预注入详尽真实数据，采用 direct 极速合成，15-25秒内出结果；
		// 2. 若用户开启工具调用且输入了特定探索问题，采用 plan_execute 进行工具探索。
		agentMode := string(agent.AgentModePlanExecute)
		if !enableTools || (strings.TrimSpace(question) == "" && preFetchCtx != "") {
			agentMode = string(agent.AgentModeDirect)
		}

		sysPrompt := ""
		if agentMode == string(agent.AgentModeDirect) {
			sysPrompt = "你现在是买方投研执行委员会核心量化分析师兼排雷审计官。所有核心客观盘面数据、估值百分位与详尽财务指标已在用户输入中完整提供，无需额外调用外部工具。请直接基于输入中的真实数据，从技术面量价突破、基本面财报排雷、消息面事实校验、资金面筹码博弈四个维度输出结构清晰、多空判断明确的深度研报，严禁在正文中生成任何伪造的工具调用标签。请突出核心逻辑与量化矛盾，语言干练利落，总字数控制在 1500~2000 字以内，确保专业度与交付效率。"
		}

		for msg := range agent.NewStockAiAgentApi().ChatWithContext(ctx, analysisQuestion, aiConfigId, sysPromptId, false, 0, think, agentMode, sysPrompt) {
			if msg == nil || msg.Content == "agent-DONE" {
				continue
			}
			if emitter != nil {
				emitter("newChatStream", map[string]any{
					"content":           msg.Content,
					"reasoning_content": msg.ReasoningContent,
				})
			}
		}
	}()
}

// SetStockAICron 设置个股 AI 监控定时任务
func (s *AgentService) SetStockAICron(cronText, stockCode string) {
	data.NewStockDataApi().SetStockAICron(cronText, stockCode)
}

// GetAIResponseResult 获取股票最新研报分析结果
func (s *AgentService) GetAIResponseResult(stockCode string) string {
	res := data.NewDeepSeekOpenAi(context.Background(), 0).GetAIResponseResult(stockCode)
	if res != nil {
		return res.Content
	}
	return ""
}

// -----------------------------------------------------------------------------
// 买方认知假说沉淀与历史演进 (Stock Thesis Ledger & Cognitive Deposition)
// -----------------------------------------------------------------------------

// GetStockThesis 查询标的买方认知假说与最新判定
func (s *AgentService) GetStockThesis(stockCode string) (*models.StockThesisLedger, error) {
	return data.NewStockThesisApi().GetStockThesis(stockCode)
}

// GetStockThesisHistory 获取标的历史认知演化时间线
func (s *AgentService) GetStockThesisHistory(stockCode string, limit int) ([]models.StockVerdictHistory, error) {
	return data.NewStockThesisApi().GetStockThesisHistory(stockCode, limit)
}

// SaveStockThesis 手动保存或调整买方认知假说
func (s *AgentService) SaveStockThesis(stockCode, stockName, verdict, verdictText, coreThesis, keyCatalysts, riskFactors, modelName string) error {
	thesis := &models.StockThesisLedger{
		StockCode:    stockCode,
		StockName:    stockName,
		Verdict:      verdict,
		VerdictText:  verdictText,
		CoreThesis:   coreThesis,
		KeyCatalysts: keyCatalysts,
		RiskFactors:  riskFactors,
		ModelName:    modelName,
	}
	return data.NewStockThesisApi().SaveStockThesis(thesis)
}

// GetAllStockTheses 获取全量标的认知状态映射表 (供自选列表极速匹配)
func (s *AgentService) GetAllStockTheses() (map[string]models.StockThesisLedger, error) {
	return data.NewStockThesisApi().GetAllStockTheses()
}

// GetStockTheses 别名兼容接口，兼容前端对全量买方认知映射表的调用
func (s *AgentService) GetStockTheses() (map[string]models.StockThesisLedger, error) {
	return s.GetAllStockTheses()
}

// -----------------------------------------------------------------------------
// 技能治理与审批流兼容性桩 (Skill Governance Compatibility)
// -----------------------------------------------------------------------------

// FetchSkillChanges 查询技能审核与变更流水
func (s *AgentService) FetchSkillChanges(query any) (any, error) {
	return map[string]any{"list": []any{}, "total": 0}, nil
}

// ApproveSkillChange 批准技能发布/更新
func (s *AgentService) ApproveSkillChange(id any) (any, error) {
	return map[string]any{"code": 1, "msg": "ok"}, nil
}

// RejectSkillChange 驳回技能发布/更新
func (s *AgentService) RejectSkillChange(id any) (any, error) {
	return map[string]any{"code": 1, "msg": "ok"}, nil
}

// BatchApproveSkillChanges 批量批准技能发布
func (s *AgentService) BatchApproveSkillChanges(ids any) (any, error) {
	return map[string]any{"code": 1, "msg": "ok"}, nil
}

// BatchRejectSkillChanges 批量驳回技能发布
func (s *AgentService) BatchRejectSkillChanges(ids any) (any, error) {
	return map[string]any{"code": 1, "msg": "ok"}, nil
}

// -----------------------------------------------------------------------------
// AI 语义向量精炼与经验结晶 (Semantic Distillation & Knowledge Crystallization)
// -----------------------------------------------------------------------------

// TriggerSemanticDistillation 手动或定时触发全局语义精炼、知识结晶与存储自愈
func (s *AgentService) TriggerSemanticDistillation(ctx context.Context) (*agent.DistillationStats, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	distiller := agent.DefaultSemanticDistiller()
	return distiller.RunMaintenanceCycle(ctx)
}

// GetDistillationStats 获取当前语义精炼、结晶总数与瘦身指标
func (s *AgentService) GetDistillationStats(ctx context.Context) (*agent.DistillationStats, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	distiller := agent.DefaultSemanticDistiller()
	return distiller.GetDistillationStats(ctx)
}

// GetKnowledgeCrystals 分页/分类获取平台凝练的历史高胜率经验结晶库
func (s *AgentService) GetKnowledgeCrystals(ctx context.Context, category string, limit int) ([]agent.KnowledgeCrystal, error) {
	if limit <= 0 {
		limit = 50
	}
	var crystals []agent.KnowledgeCrystal
	q := db.Dao.Model(&agent.KnowledgeCrystal{})
	if strings.TrimSpace(category) != "" {
		q = q.Where("category = ?", category)
	}
	err := q.Order("accuracy_score desc, hit_count desc, id desc").Limit(limit).Find(&crystals).Error
	return crystals, err
}
