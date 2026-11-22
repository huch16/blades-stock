// Package agent provides the AI agent pipeline for stock analysis.
package agent

import (
	"blades-stock/backend/agent/tools"
	"blades-stock/backend/circuitbreaker"
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/bytedance/sonic"
	"github.com/go-kratos/blades"
	bladesmc "github.com/go-kratos/blades/contrib/mcp"
	bladesTools "github.com/go-kratos/blades/tools"
)

type AgentMode string

const (
	AgentModeReact       AgentMode = "react"
	AgentModePlanExecute AgentMode = "plan_execute"
	// AgentModePipeline 五阶流水线专用模式（前端流水线入口显式传入）。
	// 与 React 同样挂载全量工具，但由能力闸门移除外网检索
	// （WebSearch / FetchWebPage）——原先是提示词里的口头纪律。
	AgentModePipeline AgentMode = "pipeline"
	AgentModeDirect   AgentMode = "direct"
)

type AgentInstance struct {
	Agent    blades.Agent
	Mode     AgentMode
	AIConfig data.AIConfig
	AllTools []bladesTools.Tool
}

// PlanTrace represents a single step in an agent execution trace.
type PlanTrace struct {
	ID           uint      `gorm:"primarykey"`
	CreatedAt    time.Time `json:"createdAt"`
	SessionID    string    `json:"sessionId" gorm:"index;size:128"`
	PlanID       string    `json:"planId" gorm:"size:128"`
	StepIndex    int       `json:"stepIndex"`
	ToolName     string    `json:"toolName" gorm:"size:128"`
	InputJSON    string    `json:"inputJson" gorm:"type:text"`
	OutputJSON   string    `json:"outputJson" gorm:"type:text"`
	Success      bool      `json:"success"`
	ErrorMsg     string    `json:"errorMsg" gorm:"type:text"`
	DurationMs   int64     `json:"durationMs"`
	RetryCount   int       `json:"retryCount"`
	AttemptCount int       `json:"attemptCount"`
}

// TableName specifies the table name for PlanTrace.
func (PlanTrace) TableName() string {
	return "agent_plan_traces"
}

// ClassifyComplexityForAudit exports classifyComplexity for audit testing and benchmarks.
// IsIndividualStockQuestionForAudit exports isIndividualStockQuestion for audit testing.
func classifyComplexity(question string) AgentMode {
	// 针对已完整注入客观数据/样本列表的共振推演、排雷诊断与专项研报，直接采用 Direct 模式快速合成，避免漫长无效的外部工具循环与超时
	if strings.Contains(question, "【AI 形态选股列表共振推演与操盘计划】") ||
		strings.Contains(question, "【AI 选股策略共振推演与实战指导报告】") ||
		strings.Contains(question, "【个股量价形态 AI 实战推演】") ||
		strings.Contains(question, "【个股指标筛选 AI 信号推演与有效性诊断】") ||
		strings.Contains(question, "【A股盘前智能前瞻与ETF轮动内参】") ||
		strings.Contains(question, "【A股全景收盘复盘与次日推演报告】") ||
		strings.Contains(question, "【ETF轮动多Agent决策与投委会打分表】") ||
		strings.Contains(question, "【Serenity式产业分析框架】") ||
		strings.Contains(question, "【全市场核心ETF池实时监测】") ||
		strings.Contains(question, "【预采集参考数据】") ||
		strings.Contains(question, "【盘前自选股与持仓综合排雷诊断清单】") ||
		strings.Contains(question, "【盘前自选股与持仓综合排雷诊断】") ||
		strings.Contains(question, "【股票盘中异动归因与排雷排查】") ||
		strings.Contains(question, "排雷诊断清单") ||
		strings.Contains(question, "综合排雷诊断") {
		return AgentModeDirect
	}

	lowerQ := strings.ToLower(question)

	simplePatterns := []string{
		"今天", "当前", "最新", "现在", "实时",
		"查询", "查一下", "帮我查", "告诉我",
		"什么是", "是什么", "多少", "几号",
		"是否", "有没有", "能不能",
		"开盘价", "收盘价", "最高价", "最低价",
		"停牌", "复牌", "上市",
		"代码", "名称", "简称",
	}

	for _, p := range simplePatterns {
		if strings.Contains(lowerQ, p) {
			wordCount := len([]rune(question))
			if wordCount < 30 {
				return AgentModeReact
			}
		}
	}

	complexPatterns := []string{
		"全面分析", "综合分析", "深度分析", "详细分析",
		"多维度", "全方位", "系统分析",
		"投资建议", "操作建议", "买卖建议",
		"对比分析", "比较分析", "横向对比",
		"行业分析", "赛道分析", "产业链",
		"投资组合", "资产配置", "仓位",
		"风险评估", "风险分析",
		"研究报告", "投资报告",
		"宏观", "政策", "周期",
	}

	for _, p := range complexPatterns {
		if strings.Contains(lowerQ, p) {
			return AgentModePlanExecute
		}
	}

	toolGroupCount := 0
	groups := tools.ClassifyQuestion(question)
	for range groups {
		toolGroupCount++
	}
	if toolGroupCount >= 4 {
		return AgentModePlanExecute
	}

	wordCount := len([]rune(question))
	if wordCount > 80 {
		return AgentModePlanExecute
	}

	return AgentModeReact
}

func GetStockAiAgent(ctx context.Context, aiConfig data.AIConfig, question string, agentMode string, matchedSkills []models.Skill, extraTools ...bladesTools.Tool) *AgentInstance {
	logger.SugaredLogger.Infof("GetStockAiAgent aiConfig: %+v", redactAIConfig(aiConfig))

	modelProvider, err := createChatModel(ctx, aiConfig)
	if err != nil {
		logger.SugaredLogger.Error(err.Error())
		return nil
	}

	// 注入 Multi-LLM 容灾 Fallback 能力（若配置了多个可用模型）。
	// 主模型处于全局熔断冷却期时，直接以首个可用备用模型为主模型启动
	// （其余备用顺延），避免每次运行先烧一次主模型调用（首包/超时等待）；
	// 无备用可用时保持现状（仍试主模型）。
	if fallbacks, fbNames := BuildFallbackProvidersFromDB(ctx, aiConfig); len(fallbacks) > 0 {
		primaryName := aiConfig.ModelName
		modelProvider, primaryName, fallbacks, fbNames = promoteCooldownPrimary(modelProvider, primaryName, fallbacks, fbNames)
		modelProvider = NewFallbackModelProvider(modelProvider, primaryName, fallbacks, fbNames)
	}

	mode := resolveAgentMode(agentMode, question)
	allTools := resolveToolsForMode(ctx, mode, question, matchedSkills, extraTools)

	// 兜底：Direct 模式必须零工具（纯研报推演，无需外部工具与 MCP，杜绝额外超时）。
	// resolveToolsForMode 已保证这一点；这里再显式置空一次，
	// 是为了让该不变量**不依赖**任何未来对 resolveToolsForMode 的改动。
	if mode == AgentModeDirect {
		allTools = nil
	}

	// ── 能力闸门（C 期）─────────────────────────────────────────────
	//
	// 把「不要在本流程调用 X」从提示词请求升级为代码级约束：不该出现的工具
	// 根本不在注入列表里。默认影子模式（只记录差异），
	// BLADES_STOCK_GATE_ENFORCE=1 后才真正裁剪，便于先对比真实差异再打开。
	if gate := tools.EffectiveGate(string(mode)); !gate.IsZero() && len(allTools) > 0 {
		kept, removed := tools.ApplyGate(allTools, gate)
		if gate.Active() {
			if len(removed) > 0 {
				logger.SugaredLogger.Infof("[CapabilityGate] mode=%s 已按闸门移除 %d 个能力: %v", mode, len(removed), removed)
			}
			allTools = kept
		} else if len(removed) > 0 {
			logger.SugaredLogger.Warnf("[CapabilityGate] 影子模式：mode=%s 若启用闸门将移除 %d 个能力: %v（设 BLADES_STOCK_GATE_ENFORCE=1 生效）",
				mode, len(removed), removed)
		}
	}

	logger.SugaredLogger.Infof("Agent mode selected: %s (user=%q), question=%q, tools=%d", mode, agentMode, question, len(allTools))

	maxStep := len(allTools)*2 + 10
	if maxStep < 30 {
		maxStep = 30
	}

	agent, err := blades.NewAgent("stock-agent",
		blades.WithModel(modelProvider),
		blades.WithTools(allTools...),
		blades.WithMaxIterations(maxStep),
		blades.WithInstruction(personalityPromptForMode(mode)),
		blades.WithMiddleware(ReasoningMiddleware),
	)
	if err != nil {
		logger.SugaredLogger.Errorf("创建Blades Agent失败: %v", err)
		return nil
	}

	return &AgentInstance{
		Agent:    agent,
		Mode:     mode,
		AIConfig: aiConfig,
		AllTools: allTools,
	}
}

// promoteCooldownPrimary 主模型处于全局熔断冷却期（MarkModelPermanentFailure
// 记录的 30 分钟隔离）且存在可用备用模型时，把首个备用提升为主模型，
// 其余备用顺延；主模型未冷却或无可用备用时原样返回。
//
// 前置条件：BuildFallbackProvidersFromDB 已排除冷却中与同凭证的备用候选，
// 所以这里的 fallbacks 均可直接用作主模型。只剩一个备用时，剩余链为空，
// NewFallbackModelProvider 会直接返回被提升的备用模型本身。
func promoteCooldownPrimary(primary blades.ModelProvider, primaryName string, fallbacks []blades.ModelProvider, fallbackNames []string) (blades.ModelProvider, string, []blades.ModelProvider, []string) {
	if len(fallbacks) == 0 || len(fallbackNames) == 0 {
		return primary, primaryName, fallbacks, fallbackNames
	}
	if !IsModelInCooldown(primaryName) {
		return primary, primaryName, fallbacks, fallbackNames
	}
	logger.SugaredLogger.Warnf("⏭️ [Fallback] 主模型 [%s] 处于熔断冷却期，直接以备用模型 [%s] 为主模型启动", primaryName, fallbackNames[0])
	return fallbacks[0], fallbackNames[0], fallbacks[1:], fallbackNames[1:]
}

func personalityPrompt() string {
	return personalityPromptForMode(AgentModePlanExecute)
}

func personalityPromptForMode(mode AgentMode) string {
	if mode == AgentModeDirect {
		return `你是买方投研执行委员会核心量化分析师兼排雷审计官。

## 核心原则
1. **直接分析**：所有核心客观盘面数据、估值百分位与主要财务指标已由底层系统预先采集并在上下文中完整提供，绝无必要且绝不调用外部工具。
2. **严禁伪造工具调用**：严禁在回答或思考中生成任何类似 "to=functions..."、"<tool_call>" 等代码块或伪造工具调用标签。
3. **专业量化研报结构**：请直接基于提供的真实数据，从技术面、基本面、消息面、资金面四个维度进行深入研判，逻辑严密，语言简练利落，给出明确的多空研判和确定性盈亏比风控提示。`
	}
	base := GetPromptTemplates().Personality
	sensor := GetGlobalMarketSensor()
	if snapshot := sensor.GetSnapshot(); snapshot != nil && snapshot.SummaryPrompt != "" {
		base += "\n\n" + snapshot.SummaryPrompt
	}
	return base
}

func ReasoningMiddleware(next blades.Handler) blades.Handler {
	return blades.HandleFunc(func(ctx context.Context, inv *blades.Invocation) blades.Generator[*blades.Message, error] {
		question := inv.Message.Text()

		mode, _ := inv.Message.Metadata["agent_mode"].(string)
		availableTools, _ := inv.Message.Metadata["available_tools"].([]string)

		var thinkingPrompt string
		tpls := GetPromptTemplates()
		if mode == string(AgentModePlanExecute) {
			thinkingPrompt = tpls.PlanExecuteThinking
			thinkingPrompt = renderAvailableTools(thinkingPrompt, availableTools)
			thinkingPrompt = renderTaskTemplate(thinkingPrompt, question)
		} else if mode == string(AgentModeDirect) {
			thinkingPrompt = "请直接根据输入中详实的一手数据进行专业深度量化分析，严格按照结构输出高质量 Markdown 研报，严禁生成任何工具调用代码或伪造标签。"
		} else {
			thinkingPrompt = tpls.ReactThinking
		}

		// 语义向量经验结晶召回 (让 Agent 随着时间推移积累历史高胜率经验与避险规律，越来越聪明)
		if mode != string(AgentModeDirect) {
			crystals, _ := DefaultSemanticDistiller().SearchKnowledgeCrystals(ctx, question, "", 2)
			if len(crystals) > 0 {
				thinkingPrompt += "\n" + DefaultSemanticDistiller().FormatCrystalsForPrompt(crystals)
			}
		}

		newMsg := blades.UserMessage(question + "\n\n" + thinkingPrompt)
		newMsg.Metadata = inv.Message.Metadata
		inv.Message = newMsg

		return next.Handle(ctx, inv)
	})
}

// renderAvailableTools replaces the {{AVAILABLE_TOOLS}} placeholder in a
// thinking prompt with the actual tools injected for this invocation, so
// plan_execute guidance references the real inventory instead of hardcoded
// tool names the model may not actually have.
func renderAvailableTools(tpl string, toolNames []string) string {
	if !strings.Contains(tpl, "{{AVAILABLE_TOOLS}}") {
		return tpl
	}
	inventory := "（本会话已注入对应工具，按需调用）"
	if len(toolNames) > 0 {
		inventory = strings.Join(toolNames, "、")
	}
	return strings.Replace(tpl, "{{AVAILABLE_TOOLS}}", inventory, 1)
}

// individualStockCodeRe 严格匹配 A 股个股代码：沪 60xxxx/688xxx，深 0xxxxx/30xxxx。
// 明确排除 5 开头沪基金/ETF（510/560/588 等）与 1 开头深基金/ETF（159/16x 等）等非个股代码。
var individualStockCodeRe = regexp.MustCompile(`(?:sh|SH|sz|SZ)?(?:60\d{4}|688\d{3}|0\d{4}|30\d{3})`)

// individualStockNameRe 匹配 2-4 个汉字紧邻个股分析语境词。
var individualStockNameRe = regexp.MustCompile(`[\p{Han}]{2,4}的?(?:股票|股价|行情|走势|代码|公司|投资价值|基本面|估值|财务|业绩|财报|研报|深度分析|个股)`)

// nonIndividualStockKeywords 命中即判定为“非单只个股”问题（ETF/基金/板块/市场/资讯/对比/配置等），
// 避免把 ETF 推荐等问题误套进个股深度分析模板。
var nonIndividualStockKeywords = []string{
	"etf", "基金", "板块", "行业", "指数", "市场", "大盘", "轮动", "配置", "组合",
	"资讯", "新闻", "总结", "宏观", "对比", "复盘", "热点", "排行",
}

// isIndividualStockQuestion 判断用户问题是否针对单只个股。
// 先做非个股关键词排除（ETF/基金/板块等），再按个股代码或个股名称语境识别。
func isIndividualStockQuestion(question string) bool {
	lower := strings.ToLower(question)
	for _, kw := range nonIndividualStockKeywords {
		if strings.Contains(lower, kw) {
			return false
		}
	}
	if individualStockCodeRe.MatchString(question) {
		return true
	}
	return individualStockNameRe.MatchString(question)
}

// renderTaskTemplate replaces the {{TASK_TEMPLATE}} placeholder in the
// plan_execute thinking prompt with the template matching the question type:
// individual-stock questions get the deep stock-analysis template, while
// ETF/板块/市场/资讯/对比等 questions get the generic task framework.
func renderTaskTemplate(tpl, question string) string {
	if !strings.Contains(tpl, "{{TASK_TEMPLATE}}") {
		return tpl
	}
	templates := GetPromptTemplates()
	task := templates.GenericTaskTemplate
	if isIndividualStockQuestion(question) {
		task = templates.StockAnalysisTemplate
	}
	return strings.Replace(tpl, "{{TASK_TEMPLATE}}", task, 1)
}

// buildSkillPrompt uses bge-m3 semantic matching (via EmbeddingService) to select relevant skills.
// When EmbeddingService is nil or embedding fails, falls back to keyword matching.
// Target prompt size: ~800 chars per skill (Directive mode), ~2000+ chars (full SystemPrompt mode).
func buildSkillPrompt(question string, embedSvc *EmbeddingService) string {
	matched := matchSkills(question, embedSvc)
	if len(matched) == 0 {
		return ""
	}
	return BuildShortSkillPrompt(matched)
}

// matchSkills returns up to 2 enabled skills relevant to the question. It is
// the single source of truth used both for building the skill prompt and for
// merging each matched skill's required tool groups into the agent's tool set.
func matchSkills(question string, embedSvc *EmbeddingService) []models.Skill {
	if len(data.NewSkillApi().GetEnabledSkills()) == 0 {
		return nil
	}
	skillEmbedSvc := GetSkillEmbeddingService(embedSvc, nil)
	matched := skillEmbedSvc.MatchSkills(question, 2)
	if len(matched) == 0 {
		return nil
	}
	// Apply sandbox filtering: for sandbox skills, only allow a percentage of requests.
	var result []models.Skill
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	for _, skill := range matched {
		if !skill.Sandbox {
			result = append(result, skill)
			continue
		}
		if skill.SandboxPercentage <= 0 {
			// treat as disabled
			continue
		}
		if rnd.Intn(100) < skill.SandboxPercentage {
			result = append(result, skill)
		}
	}
	return result
}

// BuildSkillPromptForTest exports buildSkillPrompt for testing purposes.
func BuildSkillPromptForTest(question string) string {
	return buildSkillPrompt(question, nil)
}

func GetAllTools() []bladesTools.Tool {
	var allTools []bladesTools.Tool
	allTools = append(allTools, tools.GetQueryStockCodeInfoTool())
	allTools = append(allTools, tools.GetQueryStockNewsTool())
	allTools = append(allTools, tools.GetQueryBKDictTool())
	allTools = append(allTools, tools.GetWebSearchTool())
	allTools = append(allTools, tools.GetFetchWebPageTool())
	allTools = append(allTools, tools.GetGraphQueryTool()) // P3.3: 代码图谱查询

	allTools = append(allTools, tools.GetAllDataTools()...)

	allTools = append(allTools, tools.GetHolidayTools()...)

	allTools = append(allTools, tools.GetProjectTools()...)

	allTools = append(allTools, tools.GetMCPServerTools()...)
	allTools = append(allTools, tools.GetBacktestStatsTool())
	allTools = append(allTools, tools.GetBacktestDetailTool())
	allTools = append(allTools, tools.GetRunBacktestTool())
	allTools = append(allTools, tools.GetVectorBacktestTool())
	allTools = append(allTools, tools.GetRunVectorizedBacktestTool())
	allTools = append(allTools, tools.GetDiagnoseStockTool())
	allTools = append(allTools, tools.GetVectorGridSearchTool())
	allTools = append(allTools, tools.GetStockThesisTool())
	allTools = append(allTools, tools.SaveStockThesisTool())
	allTools = append(allTools, tools.TriggerPostMortemAuditTool())
	allTools = append(allTools, tools.GetPostMortemAuditHistoryTool())
	allTools = append(allTools, tools.GetPaperPortfolioSummaryTool())
	allTools = append(allTools, tools.ExecutePaperTradeTool())
	allTools = append(allTools, tools.RunAutoAlphaExplorationTool())
	allTools = append(allTools, tools.GetAutoAlphaProposalsTool())
	allTools = append(allTools, tools.QuerySupplyChainCascadeTool())
	allTools = append(allTools, tools.GetIndustrySupplyGraphTool())

	mcpTools := getMCPTools()
	if len(mcpTools) > 0 {
		allTools = append(allTools, mcpTools...)
	}

	return WrapToolsWithQuota(allTools)
}

// resolveAgentMode 解析本次运行实际使用的模式。
// 空/未知值交给 classifyComplexity 自动判定（保持既有行为）。
func resolveAgentMode(agentMode string, question string) AgentMode {
	switch AgentMode(agentMode) {
	case AgentModeDirect, AgentModeReact, AgentModePlanExecute, AgentModePipeline:
		return AgentMode(agentMode)
	default:
		return classifyComplexity(question)
	}
}

// resolveToolsForMode 按运行模式决定注入哪些工具（纯函数，可单测）。
//
// 关键不变量：**Direct 模式必须返回空列表**。四个市场智能研报生成器
// （盘前/盘后/ETF/Serenity）都以 Direct 模式运行，其提示词中的
// 「无需额外调用外部工具 / 严禁调用 WebSearch」正是靠这一条得到**代码级**保证。
func resolveToolsForMode(ctx context.Context, mode AgentMode, question string, matchedSkills []models.Skill, extraTools []bladesTools.Tool) []bladesTools.Tool {
	if mode == AgentModeDirect {
		// Direct 零工具：摘要也要如实反映，而不是留空让人以为"没记"
		if sink := injectedSinkFrom(ctx); sink != nil {
			*sink = "direct:0"
		}
		return nil
	}
	// pipeline 与 React 一致：挂载全量工具（是否移除外网检索由能力闸门决定）
	out := getToolsByQuestion(ctx, question, matchedSkills)
	out = append(out, extraTools...)
	return WrapToolsWithQuota(out)
}

// getToolsByQuestion 组装本次请求的工具集：候选工具由 tools 包统一提供
// （AssembleBuiltinTools），再按关键词分类 + 技能 required groups 过滤 + 进化重排。
func getToolsByQuestion(ctx context.Context, question string, matchedSkills []models.Skill) []bladesTools.Tool {
	allTools := tools.AssembleBuiltinTools()
	allTools = append(allTools, getMCPTools()...)

	groups := tools.ClassifyQuestion(question)
	merged := mergeSkillToolGroups(groups, matchedSkills)
	filtered := tools.FilterToolsByGroupsCapped(allTools, groups)

	// ── D 期：按 token 预算二次裁剪（默认影子模式）─────────────────────
	//
	// 实测基线：全部 128 个内置工具的「名称+描述+schema」约 4,770 tokens；
	// 个数上限只能压数量（119→100 个），压不掉成本（仍约 4,014 tokens）。
	// 因此这里再按**实测成本**预算裁一道，Tier 低的优先砍。
	//
	// 默认只记录差异，BLADES_STOCK_GATE_ENFORCE=1 才真正裁剪 ——
	// 与能力闸门同一开关，遵循「先对比真实差异再打开」。
	if tools.TokenBudgetEnabled() {
		if _, dropped := tools.FilterByTokenBudget(filtered, groups); len(dropped) > 0 {
			logger.SugaredLogger.Warnf("[TokenBudget] 建议裁剪 %d 个工具: %v", len(dropped), dropped)
		}
	} else {
		kept, dropped := tools.FilterByTokenBudget(filtered, groups)
		if len(dropped) > 0 {
			logger.SugaredLogger.Warnf("[TokenBudget] 影子模式：若启用预算将裁剪 %d 个工具: %v（设 BLADES_STOCK_GATE_ENFORCE=1 生效）",
				len(dropped), dropped)
		}
		_ = kept
	}

	logger.SugaredLogger.Infof("tool grouping: question=%q, matched_groups=%v, skill_merged_groups=%v, total=%d, filtered=%d, est_tokens=%d",
		question, groupNames(groups), merged, len(allTools), len(filtered), tools.EstimateTokens(filtered))

	// 回写能力摘要（运行记录的 ToolsInjected 字段）
	if sink := injectedSinkFrom(ctx); sink != nil {
		*sink = summarizeInjected(groups, filtered)
	}

	// 进化回灌：按 agent_tool_ranks 实测成功率/延迟重排注入顺序。
	return reorderToolsByRank(filtered)
}

// summarizeInjected 生成可读的能力注入摘要，形如
// "base:12 stock_analysis:26 n=41 est_tokens=1834"。
// 写入运行记录后，「模型当时能看到什么」可以直接查询，不必翻日志。
func summarizeInjected(groups map[tools.ToolGroup]bool, injected []bladesTools.Tool) string {
	perGroup := map[tools.ToolGroup]int{}
	for _, tl := range injected {
		if c, ok := tools.CapabilityOf(tl.Name()); ok {
			perGroup[c.Group]++
		}
	}
	keys := make([]string, 0, len(perGroup))
	for g := range perGroup {
		keys = append(keys, string(g))
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys)+2)
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s:%d", k, perGroup[tools.ToolGroup(k)]))
	}
	parts = append(parts, fmt.Sprintf("n=%d", len(injected)))
	parts = append(parts, fmt.Sprintf("est_tokens=%d", tools.EstimateTokens(injected)))
	return strings.Join(parts, " ")
}

// mergeSkillToolGroups force-enables each matched skill's required tool groups
// (Skill.ToolGroups) in addition to keyword classification. Without this, a
// matched skill's directive can never execute because the tools it depends on
// (e.g. money_flow/news_research for the ETF skill) were filtered out.
// Returns the list of group names contributed by skills (for logging).
func mergeSkillToolGroups(groups map[tools.ToolGroup]bool, matchedSkills []models.Skill) []string {
	var skillGroups []string
	for _, skill := range matchedSkills {
		if skill.ToolGroups == "" {
			continue
		}
		for g := range tools.ParseToolGroups(skill.ToolGroups) {
			if !groups[g] {
				skillGroups = append(skillGroups, string(g))
			}
			groups[g] = true
		}
	}
	return skillGroups
}

func groupNames(groups map[tools.ToolGroup]bool) []string {
	var names []string
	for g := range groups {
		names = append(names, string(g))
	}
	return names
}

type mcpServerResult struct {
	tools []bladesTools.Tool
	name  string
}

var (
	mcpToolsCache     []bladesTools.Tool
	mcpToolsCacheTime time.Time
	mcpToolsCacheMu   sync.Mutex
)

func getMCPTools() []bladesTools.Tool {
	mcpToolsCacheMu.Lock()
	if mcpToolsCache != nil && time.Since(mcpToolsCacheTime) < 5*time.Minute {
		cached := mcpToolsCache
		mcpToolsCacheMu.Unlock()
		return cached
	}
	mcpToolsCacheMu.Unlock()
	var servers []models.MCPServer
	err := db.Dao.Where("enable = ?", true).Find(&servers).Error
	if err != nil {
		logger.SugaredLogger.Debugf("获取MCP服务器列表失败（表不存在是正常现象，首次运行会自动建表）: %v", err)
		return nil
	}

	skillServerIDs := getSkillMCPServerIDs()
	for _, id := range skillServerIDs {
		found := false
		for _, s := range servers {
			if s.ID == id {
				found = true
				break
			}
		}
		if !found {
			var server models.MCPServer
			if err := db.Dao.Where("id = ? AND enable = ?", id, true).First(&server).Error; err == nil {
				servers = append(servers, server)
			}
		}
	}

	if len(servers) == 0 {
		return nil
	}

	results := make(chan mcpServerResult, len(servers))
	var wg sync.WaitGroup

	for _, server := range servers {
		wg.Add(1)
		go func(srv models.MCPServer) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					logger.SugaredLogger.Errorf("panic connecting MCP server [%s]: %v", srv.Name, r)
					results <- mcpServerResult{tools: nil, name: srv.Name}
				}
			}()
			tools := connectMCPServer(srv)
			results <- mcpServerResult{tools: tools, name: srv.Name}
		}(server)
	}

	wg.Wait()
	close(results)

	var mcpTools []bladesTools.Tool
	for r := range results {
		if len(r.tools) > 0 {
			mcpTools = append(mcpTools, r.tools...)
		}
	}
	mcpToolsCacheMu.Lock()
	mcpToolsCache = mcpTools
	mcpToolsCacheTime = time.Now()
	mcpToolsCacheMu.Unlock()
	return mcpTools
}

func connectMCPServer(server models.MCPServer) []bladesTools.Tool {
	isHTTP := server.URL != "" && server.Command == ""
	if isHTTP {
		circuitbreaker.Configure("mcp:"+server.Name, circuitbreaker.Config{
			Name:             "mcp:" + server.Name,
			FailureThreshold: 1,
			Cooldown:         10 * time.Minute,
			HalfOpenMax:      1,
		})
	}

	breaker := circuitbreaker.Get("mcp:" + server.Name)
	if !breaker.Allow() {
		logger.SugaredLogger.Warnf("MCP服务 [%s] 断路器处于开启(熔断)状态，直接跳过连接", server.Name)
		return nil
	}

	var transport bladesmc.TransportType
	var endpoint string
	var command string
	var args []string
	var env map[string]string
	var headers map[string]string

	switch {
	case server.Command != "":
		transport = bladesmc.TransportStdio
		command = server.Command
		args = parseMCPArgs(server.Args)
		env = parseMCPEnv(server.Env)
	case server.URL != "":
		transport = bladesmc.TransportHTTP
		endpoint = server.URL
		headers = parseMCPHeaders(server.Headers)
	default:
		logger.SugaredLogger.Warnf("MCP服务器 [%s] 未配置URL或Command，跳过", server.Name)
		return nil
	}

	// 超时驱动：优先环境变量 MCP_TIMEOUT_SECONDS，再读取 SettingConfig.McpTimeout，兜底 8s。
	// 对于远程 HTTP MCP，连接超时严格限制在 1.5s 内，且一旦失败立即触发 10 分钟断路熔断，避免不可达的远程服务阻塞 Agent 会话。
	clientTimeout := data.GetSettingConfig().GetMcpTimeout()
	if transport == bladesmc.TransportHTTP {
		if clientTimeout > 1500*time.Millisecond {
			clientTimeout = 1500 * time.Millisecond
		}
	}

	mcpClient, err := bladesmc.NewClient(bladesmc.ClientConfig{
		Name:      server.Name,
		Transport: transport,
		Endpoint:  endpoint,
		Command:   command,
		Args:      args,
		Env:       env,
		Headers:   headers,
		Timeout:   clientTimeout,
	})
	if err != nil {
		logger.SugaredLogger.Errorf("创建MCP客户端失败 [%s]: %v", server.Name, err)
		breaker.Failure()
		return nil
	}

	connStart := time.Now()
	var connectErr error
	maxConnAttempts := 2
	if transport == bladesmc.TransportHTTP {
		maxConnAttempts = 1
	}
	for connAttempt := 1; connAttempt <= maxConnAttempts; connAttempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), clientTimeout)
		connectErr = mcpClient.Connect(ctx)
		cancel()
		if connectErr == nil {
			break
		}
		if connAttempt < maxConnAttempts {
			logger.SugaredLogger.Warnf("MCP服务 [%s] 首次连接失败: %v, 300ms后进行重试...", server.Name, connectErr)
			time.Sleep(300 * time.Millisecond)
		}
	}
	RecordMCPConnect(context.Background(), server.Name, connectErr == nil, time.Since(connStart))

	// Audit log for MCP connection
	mcpAuditEntry := models.AuditLog{
		ToolName:   "MCP_CONNECT",
		ServerName: server.Name,
		Success:    connectErr == nil,
		DurationMs: int64(time.Since(connStart).Milliseconds()),
	}
	if connectErr != nil {
		mcpAuditEntry.ErrorMsg = connectErr.Error()
	}
	go RecordAuditLog(mcpAuditEntry)
	if connectErr != nil {
		breaker.Failure()
		logger.SugaredLogger.Errorf("MCP连接最终失败 [%s]: %v", server.Name, connectErr)
		return nil
	}
	breaker.Success()

	// Resolve 工具列表
	var toolList []bladesTools.Tool
	var resolveErr error
	for resolveAttempt := 1; resolveAttempt <= 2; resolveAttempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), clientTimeout)
		toolList, resolveErr = mcpClient.Resolve(ctx)
		cancel()
		if resolveErr == nil {
			break
		}
		if resolveAttempt < 2 {
			logger.SugaredLogger.Warnf("MCP服务 [%s] 获取工具列表首次失败: %v, 500ms后重试...", server.Name, resolveErr)
			time.Sleep(500 * time.Millisecond)
		}
	}
	if resolveErr != nil {
		logger.SugaredLogger.Errorf("获取MCP工具列表最终失败 [%s]: %v", server.Name, resolveErr)
		return nil
	}

	if len(toolList) == 0 {
		return nil
	}

	// 根据 DB 中工具启用状态过滤
	enabledTools := getEnabledToolNames(server.ID)
	if len(enabledTools) > 0 {
		filtered := make([]bladesTools.Tool, 0, len(toolList))
		for _, t := range toolList {
			if enabledTools[t.Name()] {
				filtered = append(filtered, t)
			}
		}
		logger.SugaredLogger.Infof("从MCP服务器 [%s] 使用工具白名单过滤后加载了 %d/%d 个工具", server.Name, len(filtered), len(toolList))
		toolList = filtered
	} else {
		logger.SugaredLogger.Infof("从MCP服务器 [%s] 加载了 %d 个工具（未配置白名单）", server.Name, len(toolList))
	}

	for i := range toolList {
		// P1.2: 用 V2 版本接入重试 / 熔断 / 错误分类；旧 wrapMCPToolHandler 仍存在作为 deprecated 兼容入口。
		toolList[i] = wrapMCPToolHandlerV2(toolList[i], server.Name)
	}
	return toolList
}

func getEnabledToolNames(serverID uint) map[string]bool {
	tools := data.NewMCPServerApi().GetEnabledToolsByServerID(serverID)
	if len(tools) == 0 {
		return nil
	}
	m := make(map[string]bool, len(tools))
	for _, t := range tools {
		m[t.ToolName] = true
	}
	return m
}

// wrapMCPToolHandler wraps an MCP tool's handler to return call errors as structured JSON
// responses instead of propagating them as Go errors. This prevents a single bad MCP tool call
// from killing the entire sub-agent (the LLM sees the error text and can retry or switch tools).
//
// Deprecated: P1.2 起应直接调用 wrapMCPToolHandlerV2，享受重试 / 熔断 / 错误分类 / 脱敏全特性。
// 本函数保留仅为维持外部引用兼容，实现等价于 V2。
func wrapMCPToolHandler(original bladesTools.Tool, serverName string) bladesTools.Tool {
	return wrapMCPToolHandlerV2(original, serverName)
}

func parseMCPArgs(argsStr string) []string {
	if argsStr == "" {
		return nil
	}
	var args []string
	if err := sonic.Unmarshal([]byte(argsStr), &args); err != nil {
		logger.SugaredLogger.Warnf("解析MCP Args失败: %v", err)
		return nil
	}
	return args
}

func parseMCPEnv(envStr string) map[string]string {
	if envStr == "" {
		return nil
	}
	var env map[string]string
	if err := sonic.Unmarshal([]byte(envStr), &env); err != nil {
		logger.SugaredLogger.Warnf("解析MCP Env失败: %v", err)
		return nil
	}
	return env
}

// parseMCPHeaders 是 data.ParseMCPHeaders 的薄代理（本包调用点与测试仍用此名）。
func parseMCPHeaders(headersStr string) map[string]string {
	return data.ParseMCPHeaders(headersStr)
}

func getSkillMCPServerIDs() []uint {
	skills := data.NewSkillApi().GetEnabledSkills()
	var ids []uint
	seen := make(map[uint]bool)
	for _, skill := range skills {
		if skill.MCPServerIDs == "" {
			continue
		}
		for _, id := range data.NewSkillApi().GetMCPServerIDs(&skill) {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids
}

func safeTruncateString(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	for i := maxBytes; i > maxBytes-4 && i >= 0; i-- {
		if utf8.ValidString(s[:i]) {
			if i < len(s) {
				return s[:i] + "...(已截断)"
			}
			return s[:i]
		}
	}
	return s[:maxBytes-10] + "...(已截断)"
}

func normalizeCompressLines(content string) []string {
	raw := strings.Split(content, "\n")
	out := make([]string, 0, len(raw))
	var last string
	for _, line := range raw {
		t := strings.TrimSpace(line)
		if t == "" || isSkippableNoiseLine(t) {
			continue
		}
		if t == last {
			continue
		}
		last = t
		out = append(out, t)
	}
	return collapseLongMarkdownTableRuns(out)
}

func isMarkdownTableRow(s string) bool {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "|") {
		return false
	}
	return strings.Count(t, "|") >= 2
}

func collapseLongMarkdownTableRuns(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}
	const head, tail = 5, 5
	const minRun = 16
	res := make([]string, 0, len(lines))
	i := 0
	for i < len(lines) {
		if !isMarkdownTableRow(lines[i]) {
			res = append(res, lines[i])
			i++
			continue
		}
		start := i
		for i < len(lines) && isMarkdownTableRow(lines[i]) {
			i++
		}
		run := lines[start:i]
		if len(run) < minRun || len(run) <= head+tail {
			res = append(res, run...)
			continue
		}
		omit := len(run) - head - tail
		res = append(res, run[:head]...)
		res = append(res, fmt.Sprintf("…（省略 %d 行 markdown 表格）…", omit))
		res = append(res, run[len(run)-tail:]...)
	}
	return res
}

func isSupplementaryFactLine(s string) bool {
	if len(s) > 260 {
		return false
	}
	if containsApproxISODate(s) {
		return true
	}
	if maxConsecutiveDigitRun(s) >= 6 {
		return true
	}
	if strings.Contains(s, "元") && containsNumbers(s) {
		return true
	}
	return false
}

func isASCIIDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func containsApproxISODate(s string) bool {
	for i := 0; i+10 <= len(s); i++ {
		if !isASCIIDigit(s[i]) {
			continue
		}
		if isASCIIDigit(s[i+1]) && isASCIIDigit(s[i+2]) && isASCIIDigit(s[i+3]) &&
			s[i+4] == '-' &&
			isASCIIDigit(s[i+5]) && isASCIIDigit(s[i+6]) &&
			s[i+7] == '-' &&
			isASCIIDigit(s[i+8]) && isASCIIDigit(s[i+9]) {
			return true
		}
	}
	return false
}

func maxConsecutiveDigitRun(s string) int {
	best, cur := 0, 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			cur++
			if cur > best {
				best = cur
			}
		} else {
			cur = 0
		}
	}
	return best
}

func isSkippableNoiseLine(s string) bool {
	if len(s) > 120 {
		return false
	}
	switch {
	case strings.HasPrefix(s, "```"):
		return true
	case strings.HasPrefix(s, "---") && strings.Trim(s, "-") == "":
		return true
	case strings.HasPrefix(s, "***") && strings.Trim(s, "*") == "":
		return true
	case strings.HasPrefix(s, "___") && strings.Trim(s, "_") == "":
		return true
	}
	allSep := true
	for _, r := range s {
		if r != '|' && r != '-' && r != ':' && r != ' ' && r != '\t' {
			allSep = false
			break
		}
	}
	return allSep && strings.Contains(s, "-") && len(s) >= 8
}

func smartContentCompress(content string, maxTokens int) string {
	if maxTokens <= 0 {
		return content
	}

	lines := normalizeCompressLines(content)
	if len(lines) == 0 {
		return ""
	}

	var normJoined strings.Builder
	for i, ln := range lines {
		if i > 0 {
			normJoined.WriteByte('\n')
		}
		normJoined.WriteString(ln)
	}
	nj := normJoined.String()
	if EncodeCount(nj) <= maxTokens {
		return nj
	}

	var headers, dataLines, summaryLines, otherLines []string
	for _, line := range lines {
		switch {
		case isHeaderLine(line):
			headers = append(headers, line)
		case isDataLine(line) || isSupplementaryFactLine(line):
			dataLines = append(dataLines, line)
		case isSummaryLine(line):
			summaryLines = append(summaryLines, line)
		default:
			otherLines = append(otherLines, line)
		}
	}

	result := make([]string, 0, len(lines))
	used := 0
	tryAdd := func(parts []string) bool {
		for _, p := range parts {
			// 每行额外 +1 token 算换行符（cl100k_base 下换行也是 1 token）
			need := EncodeCount(p) + 1
			if used+need > maxTokens {
				return false
			}
			result = append(result, p)
			used += need
		}
		return true
	}

	headerCap := maxTokens / 5
	if headerCap > 1000 {
		headerCap = 1000
	}
	if headerCap < 120 && len(headers) > 0 {
		headerCap = min(maxTokens/3, 400)
	}
	tryAdd(smartTruncateLines(headers, headerCap))

	remaining := maxTokens - used
	if remaining < 1 {
		return strings.Join(result, "\n")
	}

	summaryBudget := max(1, remaining*35/100)
	if len(summaryLines) > 0 && summaryBudget < 80 {
		summaryBudget = min(remaining, 240)
	}
	tryAdd(smartTruncateLinesFromEnd(summaryLines, summaryBudget))

	remaining = maxTokens - used
	if remaining < 1 {
		out := strings.Join(result, "\n")
		if EncodeCount(out) > maxTokens {
			return EncodeTrimToTokens(out, maxTokens)
		}
		return out
	}

	dataBudget := max(1, remaining*55/100)
	if len(dataLines) > 0 && dataBudget < 80 {
		dataBudget = min(remaining, 320)
	}
	tryAdd(smartTruncateLinesFromEnd(dataLines, dataBudget))

	remaining = maxTokens - used
	if remaining > 48 {
		tryAdd(smartTruncateLinesFromEnd(otherLines, remaining))
	}

	finalContent := strings.Join(result, "\n")
	if EncodeCount(finalContent) > maxTokens {
		finalContent = EncodeTrimToTokens(finalContent, maxTokens)
	}
	return finalContent
}

func isHeaderLine(line string) bool {
	headerKeywords := []string{
		"分析", "结论", "建议", "总结", "评估", "预测", "风险", "机会",
		"价格", "涨跌", "涨幅", "成交量", "市值", "市盈率", "市净率",
		"营收", "利润", "增长率", "ROE", "ROA", "毛利率", "净利率",
	}

	for _, keyword := range headerKeywords {
		if strings.Contains(line, keyword) && len(line) < 100 {
			return true
		}
	}

	return containsKeyMetrics(line)
}

func isDataLine(line string) bool {
	return strings.Contains(line, "%") ||
		strings.Contains(line, "亿元") ||
		strings.Contains(line, "万元") ||
		(strings.Count(line, " ") > 2 && containsNumbers(line))
}

func isSummaryLine(line string) bool {
	summaryKeywords := []string{
		"总体", "整体", "综合", "综上所述", "总体来看", "整体而言",
		"建议", "推荐", "关注", "警惕", "规避",
	}

	for _, keyword := range summaryKeywords {
		if strings.Contains(line, keyword) {
			return true
		}
	}
	return false
}

func containsKeyMetrics(line string) bool {
	metrics := []string{
		"市盈率", "市净率", "ROE", "ROA", "毛利率", "净利率",
		"营收", "利润", "增长率", "股价", "市值", "成交量",
	}

	for _, metric := range metrics {
		if strings.Contains(line, metric) {
			return true
		}
	}
	return false
}

func containsNumbers(line string) bool {
	for _, r := range line {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

func smartTruncateLines(lines []string, maxTokens int) []string {
	if maxTokens <= 0 {
		return nil
	}
	kept, _ := EncodeLinesSplit(lines, maxTokens)
	return kept
}

func smartTruncateLinesFromEnd(lines []string, maxTokens int) []string {
	if maxTokens <= 0 || len(lines) == 0 {
		return nil
	}
	kept, _ := EncodeLinesKeepTail(lines, maxTokens)
	return kept
}

func init() {
	tools.RegisterLearnProjectHandler(func(source, name string) string {
		if name == "" {
			parts := strings.Split(strings.TrimSuffix(source, ".git"), "/")
			name = parts[len(parts)-1]
		}

		localPath := resolveProjectPath(source, name)

		if strings.HasPrefix(source, "http") || strings.HasPrefix(source, "git@") {
			cmd := exec.Command("git", "clone", source, localPath)
			output, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Sprintf("git clone 失败: %s\n输出: %s", err.Error(), string(output))
			}
		}

		es := &EmbeddingService{}
		sg := NewSkillGenerator(es)
		skill, err := sg.FullPipeline(source, localPath, name)
		if err != nil {
			return fmt.Sprintf("学习项目失败: %v", err)
		}

		return fmt.Sprintf("✅ 成功学习项目「%s」并生成了 Skill「%s」！\n以后你可以直接问我关于 %s 项目的问题，我会用已学的知识回答。\n必要时我会使用 ReadProjectFile 等工具查阅最新的代码细节。", name, skill.Name, name)
	})
}

func redactAIConfig(cfg data.AIConfig) data.AIConfig {
	if cfg.ApiKey != "" {
		cfg.ApiKey = "****" + cfg.ApiKey[max(0, len(cfg.ApiKey)-4):]
	}
	if cfg.HttpProxy != "" && len(cfg.HttpProxy) > 8 {
		cfg.HttpProxy = cfg.HttpProxy[:8] + "****"
	}
	return cfg
}

func resolveProjectPath(source, name string) string {
	base := filepath.Join(os.TempDir(), "blades-stock-learned-projects")
	os.MkdirAll(base, 0755)
	// name 来自用户输入或 URL 尾段，必须清洗：filepath.Base("..") 仍是 ".."，
	// 直接 Join 会逃出 base 目录。
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == ".." || name == "" {
		name = "unnamed-project"
	}
	return filepath.Join(base, name)
}
