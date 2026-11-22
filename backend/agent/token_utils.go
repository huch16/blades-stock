package agent

import (
	"strings"
	"unicode"
)

const (
	chineseCharsPerToken = 1.5
	englishCharsPerToken = 4.0
	toolsTokenReserve    = 8000
	skillPromptReserve   = 2000 // Directive ~3KB ≈ 2000 tok, full prompt ≤4000 tok
	reactLoopReserve     = 1000 // react 模式 reasoning 中间产物较短
	planExecLoopReserve  = 2500 // plan_execute 模板已去重，2500 覆盖精简后的模板+余量
	safetyMargin         = 0.85
	defaultContextWindow = 32768 // 未知模型保守假设 32K，避免溢出真实上下文
)

// knownModelContextWindows maps model family keywords to their documented
// context window size (tokens). AIConfig.MaxTokens is the *output* cap (sent
// as max_tokens), never the input context window, so the input budget must be
// derived from the model itself.
var knownModelContextWindows = []struct {
	keywords []string
	tokens   int
}{
	{[]string{"gpt-4.1", "gpt-4-1"}, 1047576}, // GPT-4.1 全系 1M
	{[]string{"gemini-2.5"}, 1048576},         // Gemini 2.5 pro/flash 1M
	{[]string{"gemini"}, 1048576},             // Gemini 2.x 1M
	{[]string{"minimax-m3"}, 1048576},         // MiniMax-M3 1M ctx
	{[]string{"minimax"}, 200000},             // M1/M2 200K，取保守值
	{[]string{"gpt-4o"}, 128000},              // 含 gpt-4o-mini
	{[]string{"gpt-4-turbo"}, 128000},
	{[]string{"o4"}, 200000},
	{[]string{"o3"}, 200000},
	{[]string{"o1"}, 200000},
	{[]string{"claude"}, 200000},
	{[]string{"deepseek-v3", "deepseek-chat"}, 128000},
	{[]string{"deepseek-r1", "deepseek-reasoner"}, 64000},
	{[]string{"deepseek"}, 128000},
	{[]string{"qwen"}, 128000},
	{[]string{"kimi", "moonshot"}, 128000},
	{[]string{"glm"}, 128000},
	{[]string{"grok"}, 131072},
	{[]string{"doubao"}, 128000},
	{[]string{"ernie"}, 128000},
	{[]string{"hunyuan"}, 128000},
	{[]string{"llama"}, 128000},
	{[]string{"mistral-large"}, 128000},
	{[]string{"mistral"}, 32768},
	{[]string{"internlm"}, 32768},
}

// estimateModelContextWindow returns a conservative estimate of the model's
// context window. Unknown models fall back to a safe 32K default rather than
// risking context overflow mid-conversation.
func estimateModelContextWindow(modelName string) int {
	lower := strings.ToLower(strings.TrimSpace(modelName))
	if lower == "" {
		return defaultContextWindow
	}
	for _, m := range knownModelContextWindows {
		for _, kw := range m.keywords {
			if strings.Contains(lower, kw) {
				return m.tokens
			}
		}
	}
	return defaultContextWindow
}

func estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	chineseCount := 0
	nonChineseCount := 0
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			chineseCount++
		} else {
			nonChineseCount++
		}
	}
	tokens := float64(chineseCount)/chineseCharsPerToken + float64(nonChineseCount)/englishCharsPerToken
	return int(tokens) + 1
}

// getMaxInputTokens derives the input-context budget from the model's context
// window (not from MaxTokens, which is the output cap). Reserves cover the
// tool definitions, skill prompt, and reasoning framework, leaving the rest
// for system prompt + question + conversation history.
func getMaxInputTokens(modelName string, isPlanExecute bool) int {
	availableTokens := int(float64(estimateModelContextWindow(modelName)) * safetyMargin)
	loopReserve := reactLoopReserve
	if isPlanExecute {
		loopReserve = planExecLoopReserve
	}
	reserved := toolsTokenReserve + skillPromptReserve + loopReserve
	result := availableTokens - reserved
	if result < 4000 {
		result = 4000
	}
	return result
}

func trimHistoryMessages(historyMessages []map[string]string, maxTokens int) []map[string]string {
	if len(historyMessages) == 0 {
		return historyMessages
	}
	currentTokens := 0
	for _, msg := range historyMessages {
		currentTokens += estimateTokens(msg["content"]) + 4
	}
	if currentTokens <= maxTokens {
		return historyMessages
	}
	halfLen := len(historyMessages) / 2
	if halfLen > 0 {
		trimmed := historyMessages[halfLen:]
		trimmedTokens := 0
		for _, msg := range trimmed {
			trimmedTokens += estimateTokens(msg["content"]) + 4
		}
		if trimmedTokens <= maxTokens {
			return trimmed
		}
	}
	result := []map[string]string{}
	tokenSum := 0
	for i := len(historyMessages) - 1; i >= 0; i-- {
		msgTokens := estimateTokens(historyMessages[i]["content"]) + 4
		if tokenSum+msgTokens > maxTokens {
			break
		}
		tokenSum += msgTokens
		result = append([]map[string]string{historyMessages[i]}, result...)
	}
	if len(result) == 0 && len(historyMessages) > 0 {
		lastMsg := historyMessages[len(historyMessages)-1]
		content := lastMsg["content"]
		maxChars := maxTokens * 2
		// 按 rune 截断并保留尾部：直接按字节切片 content[len-maxChars:]
		// 可能切在多字节汉字中间，产生非法 UTF-8。
		runes := []rune(content)
		if len(runes) > maxChars {
			lastMsg = map[string]string{
				"role":    lastMsg["role"],
				"content": string(runes[len(runes)-maxChars:]) + "\n...(更早的内容已省略)",
			}
		}
		result = []map[string]string{lastMsg}
	}
	return result
}
