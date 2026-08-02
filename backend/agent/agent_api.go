package agent

import (
	"context"
	"fmt"
	"blades-stock/backend/data"
	"blades-stock/backend/logger"
	"blades-stock/backend/quant"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-kratos/blades"
	bladesTools "github.com/go-kratos/blades/tools"
	"github.com/samber/lo"
)

type StreamMessage struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content"`
}

type StockAiAgent struct {
	instance   *AgentInstance
	sessionID  string
	aiConfigId int
}

func NewStockAiAgentApi() *StockAiAgent {
	return &StockAiAgent{}
}

func (receiver StockAiAgent) newStockAiAgent(ctx *context.Context, aiConfigId int, thinkingMode bool, question string, agentMode string, extraTools ...bladesTools.Tool) *StockAiAgent {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("panic in newStockAiAgent: %v", r)
		}
	}()

	settingConfig := data.GetSettingConfig()
	if settingConfig == nil {
		logger.SugaredLogger.Errorf("settingConfig is nil")
		return nil
	}

	if aiConfigId <= 0 {
		aiConfigId = int(settingConfig.AiConfigID)
	}

	aiConfig, ok := lo.Find(settingConfig.AiConfigs, func(item *data.AIConfig) bool {
		return item != nil && uint(aiConfigId) == item.ID
	})
	if !ok {
		for _, cfg := range settingConfig.AiConfigs {
			if cfg != nil {
				aiConfig = cfg
				aiConfigId = int(cfg.ID)
				ok = true
				logger.SugaredLogger.Infof("ai config %d not found, fallback to first non-nil: %d (%s)", aiConfigId, cfg.ID, cfg.Name)
				break
			}
		}
	}
	if !ok {
		logger.SugaredLogger.Errorf("no ai config available")
		return nil
	}
	if aiConfig == nil {
		logger.SugaredLogger.Errorf("aiConfig is nil for id: %d", aiConfigId)
		return nil
	}

	aiConfig.Thinking = thinkingMode
	sessionID := aiConfig.SessionId
	if sessionID == "" {
		sessionID = fmt.Sprintf("ai-config-%d", aiConfig.ID)
	}

	agentInstance := GetStockAiAgent(*ctx, *aiConfig, question, agentMode, extraTools...)
	if agentInstance == nil {
		logger.SugaredLogger.Errorf("failed to create agent for config id: %d", aiConfigId)
		return nil
	}

	return &StockAiAgent{
		instance:   agentInstance,
		sessionID:  sessionID,
		aiConfigId: aiConfigId,
	}
}

func (receiver StockAiAgent) Chat(question string, aiConfigId int, sysPromptId *int) chan *StreamMessage {
	return receiver.ChatWithContext(context.Background(), question, aiConfigId, sysPromptId, true, 20, false, "")
}

func (receiver StockAiAgent) ChatWithContext(ctx context.Context, question string, aiConfigId int, sysPromptId *int, memoryMode bool, memoryCount int, thinkingMode bool, agentMode string, optsOverride ...string) chan *StreamMessage {
	ch := make(chan *StreamMessage, 1024)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Errorf("panic in ChatWithContext: %v", r)
				ch <- &StreamMessage{
					Role:    "assistant",
					Content: fmt.Sprintf("❌ 内部错误: %v", r),
				}
				close(ch)
			}
		}()

		var sessionIDOverride string
		var sysPromptOverride string
		if len(optsOverride) > 0 && optsOverride[0] != "" {
			sysPromptOverride = optsOverride[0]
		}
		if len(optsOverride) > 1 && optsOverride[1] != "" {
			sessionIDOverride = optsOverride[1]
		}

		settingConfig := data.GetSettingConfig()

		// Build embedding service. Priority:
		// 1) Global standalone embedding config (vector model) from settings.
		// 2) Selected chat config's own embedding settings.
		// 3) Scan all configs for an embedding-enabled one (e.g. bge-m3)
		//    so that chat models without embedding support still get semantic matching.
		var embedService *EmbeddingService
		originalAiConfig, _ := lo.Find(settingConfig.AiConfigs, func(item *data.AIConfig) bool {
			return uint(aiConfigId) == item.ID
		})
		embedEnabled := settingConfig.EmbeddingEnabled && settingConfig.EmbeddingModelName != ""
		if embedEnabled {
			embedBaseUrl := settingConfig.EmbeddingBaseUrl
			if embedBaseUrl == "" && originalAiConfig != nil {
				embedBaseUrl = originalAiConfig.BaseUrl
			}
			embedService = NewEmbeddingService(
				embedBaseUrl,
				settingConfig.EmbeddingApiKey,
				settingConfig.EmbeddingModelName,
			)
		} else {
			embedCfg := originalAiConfig
			if embedCfg == nil || !embedCfg.EmbeddingEnabled || embedCfg.EmbeddingModelName == "" {
				for _, cfg := range settingConfig.AiConfigs {
					if cfg.EmbeddingEnabled && cfg.EmbeddingModelName != "" {
						embedCfg = cfg
						break
					}
				}
			}
			if embedCfg != nil && embedCfg.EmbeddingEnabled && embedCfg.EmbeddingModelName != "" {
				embedBaseUrl := embedCfg.EmbeddingBaseUrl
				if embedBaseUrl == "" {
					embedBaseUrl = embedCfg.BaseUrl
				}
				embedService = NewEmbeddingService(
					embedBaseUrl,
					embedCfg.ApiKey,
					embedCfg.EmbeddingModelName,
				)
			}
		}

		// Detect skill before creating agent, so we can log skill usage
		skillPrompt := buildSkillPrompt(question, embedService)

		stockAiAgent := receiver.newStockAiAgent(&ctx, aiConfigId, thinkingMode, question, agentMode)
		if stockAiAgent == nil {
			logger.SugaredLogger.Errorf("stockAiAgent is nil")
			ch <- &StreamMessage{
				Role:    "assistant",
				Content: "❌ AI 配置不存在或无效，请检查 AI 配置",
			}
			close(ch)
			return
		}

		if sessionIDOverride != "" {
			stockAiAgent.sessionID = sessionIDOverride
		}

		// Apply timeout from AI config to the context, so the LLM HTTP call
		// respects the configured timeout instead of hanging indefinitely.
		// Default to 300s (5 min) if not configured, matching the frontend timeout.
		if stockAiAgent.instance != nil {
			timeOutSec := stockAiAgent.instance.AIConfig.TimeOut
			if timeOutSec <= 0 {
				timeOutSec = 300
			}
			toCtx, cancel := context.WithTimeout(ctx, time.Duration(timeOutSec)*time.Second)
			ctx = toCtx
			defer cancel()
		}

		var memoryService *ChatMemoryService
		var historyMessages []map[string]string
		if memoryMode && stockAiAgent.sessionID != "" {
			memoryService = NewChatMemoryService(stockAiAgent.sessionID, memoryCount)
			if embedService != nil {
				memoryService.SetEmbeddingService(embedService)
			}

			var err error
			historyMessages, err = memoryService.GetHistoryMessages(question)
			if err != nil {
				logger.SugaredLogger.Errorf("failed to get history messages: %v", err)
				historyMessages = nil
			}
		}

		sysPrompt := sysPromptOverride
		if sysPrompt == "" {
			if settingConfig.Prompt != "" {
				sysPrompt = settingConfig.Prompt + "\n\n"
			}
			if skillPrompt != "" {
				sysPrompt += skillPrompt + "\n\n"
			}
			sysPrompt += `【强制规则】你必须通过工具调用获取实时数据，严禁凭记忆编造或使用过时数据。以下场景必须调用工具：
1. 股票/指数行情数据（价格、涨跌幅、成交量等）——必须调用工具获取最新实时数据
2. 财务数据（营收、利润、市盈率PE、市净率PB、ROE、毛利率、净利率等）——必须调用工具获取最新财报数据
3. 资金流向数据（主力净流入/流出、大单、超大单、散户）——必须调用工具获取
4. 技术指标（均线MA、MACD、KDJ、RSI等）——必须调用工具或基于实时行情推算
5. 新闻资讯——必须调用工具获取最新新闻
6. 宏观经济数据——必须调用工具获取最新数据

【数据真实性红线】这是最高优先级规则：
- 严禁凭空编造PE、PB、ROE、营收、利润、毛利率、净利率、资金流向金额、成交额、股东人数、换手率、均线价格等任何数字指标
- 如果你没有通过工具获取到某个指标的确切数值，必须明确说"数据不可得"或"未查询到"，绝不能编造一个数字充数
- 如果你不确定某个信息的来源，不要声称它来自"公告"、"公司披露"或"机构调研"
- 任何涉及具体数字的回答，都必须先通过工具查询确认，不得使用训练数据中的过时信息
- 如果你没有获取到最新数据，必须明确告知用户"当前未能获取到最新数据"`
		}

		aiConfig, _ := lo.Find(settingConfig.AiConfigs, func(item *data.AIConfig) bool {
			return uint(aiConfigId) == item.ID
		})
		maxInputTokens := 0
		if aiConfig != nil {
			maxInputTokens = getMaxInputTokens(aiConfig.MaxTokens)
		}

		sysPromptTokens := estimateTokens(sysPrompt)
		questionTokens := estimateTokens(question)
		historyBudget := maxInputTokens - sysPromptTokens - questionTokens
		if historyBudget < 0 {
			historyBudget = 0
		}
		if len(historyMessages) > 0 && historyBudget > 0 {
			historyMessages = trimHistoryMessages(historyMessages, historyBudget)
		}

		// auto-compress history when too long
		if memoryService != nil && memoryService.NeedsCompress() {
			historyMessages, _ = memoryService.CompressHistory(historyMessages)
		}

		var bladeHistory []*blades.Message
		for _, hm := range historyMessages {
			role := hm["role"]
			content := hm["content"]
			switch role {
			case "user":
				bladeHistory = append(bladeHistory, blades.UserMessage(content))
			case "assistant":
				bladeHistory = append(bladeHistory, blades.AssistantMessage(content))
			case "system":
				bladeHistory = append(bladeHistory, blades.SystemMessage(content))
			}
		}

		// inject conversation summary if available
		if memoryService != nil {
			if summary, err := memoryService.GetSummary(); err == nil && summary != "" {
				bladeHistory = append([]*blades.Message{blades.SystemMessage("【对话摘要】" + summary)}, bladeHistory...)
			}
		}

		// inject historical analysis memory (relevant past AI reports)
		stockCode, stockName := extractStockInfo(question)
		analysisMem := NewAnalysisMemoryService().BuildAutoInjectContext(stockCode, stockName)
		if analysisMem != "" {
			bladeHistory = append([]*blades.Message{blades.SystemMessage(analysisMem)}, bladeHistory...)
		}

		// inject backtest track record
		btMgr := quant.NewBacktestManager()
		if stockCode != "" {
			if stats := btMgr.GetStockStats(stockCode); stats.VerifiedCount > 0 {
				trackRecord := fmt.Sprintf("【回测记录】你在 %s 上的历史分析准确率: %.1f%% (已验证 %d 次)", stockCode, stats.AccuracyRate, stats.VerifiedCount)
				sysPrompt += "\n\n" + trackRecord
			}
		} else if overall := btMgr.GetOverallStats(); overall.VerifiedCount > 0 {
			trackRecord := fmt.Sprintf("【回测记录】你的历史分析总体准确率: %.1f%% (已验证 %d 次, 共 %d 份报告)", overall.AccuracyRate, overall.VerifiedCount, overall.TotalReports)
				sysPrompt += "\n\n" + trackRecord
		}

		if memoryService != nil {
			if err := memoryService.AddUserMessage(question); err != nil {
				logger.SugaredLogger.Errorf("failed to save user message: %v", err)
			}
		}

		// parallel pre-fetch: for multi-group questions, run sub-agents
		// concurrently to gather data before the main agent loop.
		if prefetched := preFetchInParallel(ctx, question, stockAiAgent.instance.AllTools, stockAiAgent.instance.AIConfig); prefetched != nil {
			parallelCtx := buildParallelContext(prefetched)
			if parallelCtx != "" {
				sysPrompt = parallelCtx + "\n\n" + sysPrompt
			}
		}

		ch <- &StreamMessage{
			Role:             "assistant",
			Content:          "",
			ReasoningContent: "[STATUS]🤔 正在理解问题并制定分析计划...",
		}

		invocation := &blades.Invocation{
			Message:     blades.UserMessage(question),
			Instruction: blades.SystemMessage(sysPrompt),
			History:     bladeHistory,
			Streamable:  true,
		}
		invocation.Message.Metadata = map[string]any{
			"agent_mode": string(stockAiAgent.instance.Mode),
		}

		var fullResponse strings.Builder
		var toolNames []string
		var agentErr error
		for msg, err := range stockAiAgent.instance.Agent.Run(ctx, invocation) {
			if err != nil {
				agentErr = err
				logger.SugaredLogger.Errorf("agent run error: %v", err)

				errMsg := fmt.Sprintf("❌ Agent 调用失败：%v", err)
				if strings.Contains(err.Error(), "exceeds max iterations") {
					errMsg += "\n\n**可能原因**：模型在执行过程中进行了过多轮工具调用仍无法收敛，可能陷入了循环。\n\n**解决方案**：\n1. 尝试更精确地描述你的问题，减少模糊性\n2. 切换到支持更长上下文或更强推理能力的模型\n3. 简化查询条件"
				} else if strings.Contains(err.Error(), "reasoning") || strings.Contains(err.Error(), "thinking") {
					errMsg += "\n\n**可能原因**：当前模型开启了 thinking/reasoning 模式，但该模式与 Agent 工具调用不兼容。\n\n**解决方案**：请在 AI 配置中关闭 thinking 模式，或切换到支持工具调用的模型（如 deepseek-chat、gpt-4o 等）。"
				}

				ch <- &StreamMessage{
					Role:    "assistant",
					Content: errMsg,
				}
				break
			}

			if msg == nil {
				continue
			}

			if msg.Role == blades.RoleAssistant {
				text := msg.Text()
				if text != "" {
					// Skip if this is the final accumulated message (openai stream
					// yields both delta chunks and the final full text — appending
					// the full text again causes duplication in the frontend).
					if fullResponse.Len() > 0 && text == fullResponse.String() {
						// final accumulated message, already streamed as deltas
					} else {
						fullResponse.WriteString(text)
						ch <- &StreamMessage{
							Role:    "assistant",
							Content: text,
						}
					}
				}

				if msg.Metadata != nil {
					if rc, ok := msg.Metadata["reasoning_content"].(string); ok && rc != "" {
						ch <- &StreamMessage{
							Role:             "assistant",
							Content:          "",
							ReasoningContent: rc,
						}
					}
				}
			}

			for _, part := range msg.Parts {
				if tp, ok := part.(*blades.ToolPart); ok {
					toolNames = append(toolNames, tp.Name)
					logger.SugaredLogger.Infof("[ToolCall] %s(%s)", tp.Name, tp.Request)
					ch <- &StreamMessage{
						Role:             "assistant",
						Content:          "",
						ReasoningContent: fmt.Sprintf("[STEP]🔧 调用工具：%s(%s)\n", tp.Name, tp.Request),
					}
					if tp.Response != "" {
						resultLen := len(tp.Response)
						status := "✅"
						if resultLen == 0 || strings.Contains(tp.Response, "error") || strings.Contains(tp.Response, "失败") {
							status = "⚠️"
						}
						summary := tp.Response
						if len(summary) > 80 {
							summary = summary[:80] + "..."
						}
						summary = strings.ReplaceAll(summary, "\n", " ")
						ch <- &StreamMessage{
							Role:             "assistant",
							Content:          "",
							ReasoningContent: fmt.Sprintf("[STEP]%s 工具 %s 返回数据 (%d 字符): %s\n", status, tp.Name, resultLen, summary),
						}
					}
				}
			}
		}

		// If agent exited early due to error, don't send status/DONE signals — just close.
		if agentErr != nil {
			close(ch)
			return
		}

		ch <- &StreamMessage{
			Role:             "assistant",
			Content:          "",
			ReasoningContent: "[STATUS]✍️ 正在根据数据生成回答...",
		}

		if fullResponse.Len() != 0 && memoryService != nil {
			if err := memoryService.AddAssistantMessage(fullResponse.String()); err != nil {
				logger.SugaredLogger.Errorf("failed to save assistant message: %v", err)
			}
		}

		// fire-and-forget post-processing: memory, evaluation, tool ranking
		if fullResponse.Len() > 0 && stockAiAgent != nil && stockAiAgent.sessionID != "" {
			go postProcessResponse(stockAiAgent.sessionID, question, fullResponse.String(), toolNames)
		}

		ch <- &StreamMessage{
			Role:    "assistant",
			Content: "agent-DONE",
		}
		close(ch)
	}()

	return ch
}

// postProcessResponse runs memory storage, evaluation, and tool ranking in
// parallel after the agent response is complete. It is fire-and-forget and
// must not block the response stream.
func postProcessResponse(sessionID, question, response string, toolNames []string) {
	recoverLog := func(label string) {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("panic in postProcessResponse [%s]: %v", label, r)
		}
	}
	var wg sync.WaitGroup

	// 1. Store working memory
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer recoverLog("memory")
		DefaultMemoryManager().Add(sessionID, "user", question)
		DefaultMemoryManager().Add(sessionID, "assistant", response)
	}()

	// 2. Score and store reflection
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer recoverLog("reflection")
		score := DefaultEvaluator().ScoreResult(response)
		DefaultEvaluator().GenerateReflection(sessionID, question, response, 0, nil, score)
	}()

	// 3. Record tool usage
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer recoverLog("tool_ranking")
		for _, name := range toolNames {
			DefaultEvolver().RecordToolUse(name, true, 0)
		}
	}()

	wg.Wait()
	logger.SugaredLogger.Debugf("postProcessResponse completed for session %s", sessionID)
}

// extractStockInfo 从用户问题中提取股票代码/名称（简单启发式匹配）
func extractStockInfo(question string) (stockCode, stockName string) {
	// 匹配股票代码: 6 位数字（A股），或 sh/SH/sz/SZ 前缀
	codeRe := regexp.MustCompile(`(?:sh|SH|sz|SZ)?\d{6}`)
	if m := codeRe.FindString(question); m != "" {
		stockCode = m
	}
	// 匹配常见股票名称关键词（中文 2-4 字，出现在特定语境）
	nameRe := regexp.MustCompile(`([\p{Han}]{2,4})(?:股票|股价|行情|走势|代码)`)
	if m := nameRe.FindStringSubmatch(question); len(m) > 1 {
		stockName = m[1]
	}
	return
}
