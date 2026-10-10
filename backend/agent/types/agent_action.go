package types

import (
	"encoding/json"
	"regexp"
	"strings"
)

// AgentActionType 定义 Agent 对整个系统的控制指令类型
type AgentActionType string

const (
	ActionNavigateView       AgentActionType = "NAVIGATE_VIEW"
	ActionOpenDiagnosis      AgentActionType = "OPEN_DIAGNOSIS"
	ActionOpenMarketIntel    AgentActionType = "OPEN_MARKET_INTELLIGENCE"
	ActionShowKline          AgentActionType = "SHOW_KLINE"
	ActionDisciplineAlert    AgentActionType = "DISCIPLINE_ALERT"
	ActionFilterWatchlist    AgentActionType = "FILTER_WATCHLIST"
)

// AgentAction 表示具体的控制指令
type AgentAction struct {
	Type    AgentActionType        `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

// AgentResponseWithActions 包装 Agent 文本输出与伴随动作指令
type AgentResponseWithActions struct {
	Text    string        `json:"text"`
	Actions []AgentAction `json:"actions"`
}

var actionBlockRegex = regexp.MustCompile("(?s)```agent-action\\s*([\\s\\S]*?)\\s*```")

// ParseAndExtractActions 从 LLM 输出的 Markdown/文本中自动提取动作代码块，并返回净化后的文本和动作列表
func ParseAndExtractActions(rawText string) (string, []AgentAction) {
	if rawText == "" {
		return "", nil
	}

	var actions []AgentAction
	cleanedText := actionBlockRegex.ReplaceAllStringFunc(rawText, func(match string) string {
		subMatches := actionBlockRegex.FindStringSubmatch(match)
		if len(subMatches) > 1 {
			jsonBlock := strings.TrimSpace(subMatches[1])
			// 尝试解析为单动作或者动作数组
			var singleAction AgentAction
			if err := json.Unmarshal([]byte(jsonBlock), &singleAction); err == nil && singleAction.Type != "" {
				actions = append(actions, singleAction)
			} else {
				var actionList []AgentAction
				if err := json.Unmarshal([]byte(jsonBlock), &actionList); err == nil {
					for _, a := range actionList {
						if a.Type != "" {
							actions = append(actions, a)
						}
					}
				}
			}
		}
		return "" // 将动作指令块从主文本中移除，避免污染用户界面
	})

	return strings.TrimSpace(cleanedText), actions
}
