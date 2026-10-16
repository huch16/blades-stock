package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"blades-stock/backend/agent"
	"blades-stock/backend/data"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
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
	agentCancel    context.CancelFunc
	defaultEmitter MessageEmitter
}

// NewAgentService 创建 Agent 服务实例
func NewAgentService() *AgentService {
	return &AgentService{}
}

// SetDefaultEmitter 设置默认的流式推流发射器（在 Server 模式下由 SSE 挂载）
func (s *AgentService) SetDefaultEmitter(emitter MessageEmitter) {
	s.defaultEmitter = emitter
}

// ChatWithAgent 执行 AI Agent 智能会话推演（使用默认发射器向外推流）
func (s *AgentService) ChatWithAgent(question string, aiConfigId int, sysPromptId *int, memoryMode bool, memoryCount int, thinkingMode bool, agentMode string, sessionId string) {
	s.ChatWithAgentCustom(s.defaultEmitter, question, aiConfigId, sysPromptId, memoryMode, memoryCount, thinkingMode, agentMode, sessionId)
}

// ChatWithAgentCustom 执行 AI Agent 会话并支持自定义发射器（桌面模式使用 Wails EventsEmit）
func (s *AgentService) ChatWithAgentCustom(emitter MessageEmitter, question string, aiConfigId int, sysPromptId *int, memoryMode bool, memoryCount int, thinkingMode bool, agentMode string, sessionId string) {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("AgentService.ChatWithAgent panic: %v", r)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	s.agentMu.Lock()
	if s.agentCancel != nil {
		s.agentCancel()
	}
	s.agentCancel = cancel
	s.agentMu.Unlock()

	defer func() {
		s.agentMu.Lock()
		s.agentCancel = nil
		s.agentMu.Unlock()
	}()

	ch := agent.NewStockAiAgentApi().ChatWithContext(ctx, question, aiConfigId, sysPromptId, memoryMode, memoryCount, thinkingMode, agentMode, sessionId)
	for msg := range ch {
		if emitter != nil {
			emitter("agent-message", AgentMessageToFrontendMap(msg))
		}
	}
}

// AbortChatWithAgent 打断或终止当前正在进行的 Agent 推理
func (s *AgentService) AbortChatWithAgent() {
	s.agentMu.Lock()
	defer s.agentMu.Unlock()
	if s.agentCancel != nil {
		s.agentCancel()
		s.agentCancel = nil
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
