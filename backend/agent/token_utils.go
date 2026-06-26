package agent

import (
	"unicode"
)

const (
	chineseCharsPerToken = 1.5
	englishCharsPerToken = 4.0
	toolsTokenReserve    = 8000
	skillPromptReserve   = 4000
	reactLoopReserve     = 8000
	safetyMargin         = 0.85
)

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

func getMaxInputTokens(maxTokens int) int {
	if maxTokens <= 0 {
		return 120000
	}
	availableTokens := int(float64(maxTokens) * safetyMargin)
	reserved := toolsTokenReserve + skillPromptReserve + reactLoopReserve
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
		if len(content) > maxChars {
			lastMsg = map[string]string{
				"role":    lastMsg["role"],
				"content": content[len(content)-maxChars:] + "\n...(更早的内容已省略)",
			}
		}
		result = []map[string]string{lastMsg}
	}
	return result
}


