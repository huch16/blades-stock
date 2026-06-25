package agent

import (
	"context"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"regexp"
	"strings"

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

	aiConfig, ok := lo.Find(settingConfig.AiConfigs, func(item *data.AIConfig) bool {
		return uint(aiConfigId) == item.ID
	})
	if !ok {
		logger.SugaredLogger.Errorf("ai config not found for id: %d", aiConfigId)
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

		var memoryService *ChatMemoryService
		var historyMessages []map[string]string
		if memoryMode && stockAiAgent.sessionID != "" {
			memoryService = NewChatMemoryService(stockAiAgent.sessionID, memoryCount)

			if stockAiAgent.instance.AIConfig.EmbeddingEnabled && stockAiAgent.instance.AIConfig.EmbeddingModelName != "" {
				es := NewEmbeddingService(
					stockAiAgent.instance.AIConfig.BaseUrl,
					stockAiAgent.instance.AIConfig.ApiKey,
					stockAiAgent.instance.AIConfig.EmbeddingModelName,
				)
				memoryService.SetEmbeddingService(es)
			}

			var err error
			historyMessages, err = memoryService.GetHistoryMessages(question)
			if err != nil {
				logger.SugaredLogger.Errorf("failed to get history messages: %v", err)
				historyMessages = nil
			}
		}

		sysPrompt := ""
		if sysPromptOverride != "" {
			sysPrompt = sysPromptOverride
		} else if sysPromptId == nil || *sysPromptId == 0 {
			sysPrompt = `你现在扮演一位拥有20年实战经验的顶级股票投资大师，精通价值投资、趋势交易、量化分析等多种策略。你擅长结合宏观经济、行业周期和企业基本面进行全方位、精准的多维分析，尤其对A股、港股、美股市场有深刻理解，始终秉持"风险控制第一"的原则，善于用通俗易懂的方式传授投资智慧。`
		} else {
			sysPrompt = data.NewPromptTemplateApi().GetPromptTemplateByID(*sysPromptId)
		}

		sysPrompt += `

【强制规则】你必须通过工具调用获取实时数据，严禁凭记忆编造或使用过时数据。以下场景必须调用工具：
1. 股票/指数行情数据（价格、涨跌幅、成交量等）——必须调用工具获取最新实时数据
2. 财务数据（营收、利润、市盈率等）——必须调用工具获取最新财报数据
3. 新闻资讯——必须调用工具获取最新新闻
4. 宏观经济数据——必须调用工具获取最新数据
任何涉及具体数字的回答，都必须先通过工具查询确认，不得使用训练数据中的过时信息。如果你没有获取到最新数据，必须明确告知用户"当前未能获取到最新数据"，绝不能编造数据。`

		if stockAiAgent.instance.Mode == AgentModePlanExecute {
			sysPrompt += `

【深度分析模式】
对于复杂分析任务，请遵循以下步骤：
1. 拆解问题，分步骤获取所需数据
2. 逐步分析每个维度
3. 综合所有信息给出完整分析结论
`
		}

		skillPrompt := buildSkillPrompt(question)
		if skillPrompt != "" {
			sysPrompt += "\n\n" + skillPrompt
		}

		settingConfig := data.GetSettingConfig()
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

		if memoryService != nil {
			if err := memoryService.AddUserMessage(question); err != nil {
				logger.SugaredLogger.Errorf("failed to save user message: %v", err)
			}
		}

		invocation := &blades.Invocation{
			Message:     blades.UserMessage(question),
			Instruction: blades.SystemMessage(sysPrompt),
			History:     bladeHistory,
			Streamable:  true,
		}

		var fullResponse strings.Builder
		for msg, err := range stockAiAgent.instance.Agent.Run(ctx, invocation) {
			if err != nil {
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
					fullResponse.WriteString(text)
					ch <- &StreamMessage{
						Role:    "assistant",
						Content: text,
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
					logger.SugaredLogger.Infof("[ToolCall] %s(%s)", tp.Name, tp.Request)
					ch <- &StreamMessage{
						Role:             "assistant",
						Content:          "",
						ReasoningContent: fmt.Sprintf("[STEP]🔧 调用工具：%s(%s)\n", tp.Name, tp.Request),
					}
				}
			}
		}

		if fullResponse.Len() != 0 && memoryService != nil {
			if err := memoryService.AddAssistantMessage(fullResponse.String()); err != nil {
				logger.SugaredLogger.Errorf("failed to save assistant message: %v", err)
			}
		}

		ch <- &StreamMessage{
			Role:    "assistant",
			Content: "agent-DONE",
		}
		close(ch)
	}()

	return ch
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func safeSend(ch chan *StreamMessage, msg *StreamMessage) {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("panic when sending to channel: %v", r)
		}
	}()
	select {
	case ch <- msg:
	default:
		logger.SugaredLogger.Warnf("channel full, message dropped")
	}
}

func stripPlanJSON(content string) string {
	if !strings.Contains(content, `"steps"`) {
		return content
	}
	var b strings.Builder
	b.Grow(len(content))
	inCodeBlock := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
		}
		if inCodeBlock && strings.Contains(trimmed, `"steps"`) && strings.Contains(trimmed, "[") {
			continue
		}
		if !inCodeBlock && (strings.HasPrefix(trimmed, `{"steps":`) || strings.HasPrefix(trimmed, `{"steps" :`)) {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(line)
	}
	result := strings.TrimRight(b.String(), "\n ")
	if result == "" {
		return ""
	}
	lines := strings.Split(result, "\n")
	cleaned := make([]string, 0, len(lines))
	skipEmpty := true
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			if !skipEmpty {
				cleaned = append(cleaned, l)
			}
			skipEmpty = true
			continue
		}
		skipEmpty = false
		cleaned = append(cleaned, l)
	}
	return strings.Join(cleaned, "\n")
}

func formatMarkdown(content string) string {
	if content == "" {
		return content
	}

	inCodeBlock := false
	lines := strings.Split(content, "\n")
	var result []string

	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")

		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			if !inCodeBlock {
				result = append(result, trimmed)
				continue
			}
		}

		if inCodeBlock {
			result = append(result, line)
			continue
		}

		if trimmed != line && trimmed != "" {
			line = trimmed
		}

		if i > 0 && isBlockElement(trimmed) {
			prev := ""
			if len(result) > 0 {
				prev = result[len(result)-1]
			}
			if prev != "" && !isBlockElement(strings.TrimLeft(prev, " \t")) {
				result = append(result, "")
			}
		}

		line = splitInlineHeading(line)

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

var headingRe = regexp.MustCompile(`(#{1,6}\s+\S)`)

func splitInlineHeading(line string) string {
	idx := headingRe.FindStringIndex(line)
	if idx == nil {
		return line
	}
	if idx[0] == 0 {
		return line
	}
	prefix := line[:idx[0]]
	if strings.TrimSpace(prefix) == "" {
		return line
	}
	return prefix + "\n\n" + line[idx[0]:]
}

func isBlockElement(line string) bool {
	if len(line) == 0 {
		return false
	}
	if line[0] == '#' {
		return true
	}
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "+ ") {
		return true
	}
	if strings.HasPrefix(line, "```") {
		return true
	}
	if strings.HasPrefix(line, "> ") {
		return true
	}
	if len(line) >= 2 && (line[0] >= '1' && line[0] <= '9') && line[1] == '.' {
		return true
	}
	if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "***") || strings.HasPrefix(line, "___") {
		return true
	}
	if strings.HasPrefix(line, "|") {
		return true
	}
	return false
}
