package types

import (
	"encoding/json"
	"regexp"
	"strings"
)

// AgentActionType 定义 Agent 对整个系统的控制指令类型
type AgentActionType string

const (
	ActionNavigateView    AgentActionType = "NAVIGATE_VIEW"
	ActionOpenDiagnosis   AgentActionType = "OPEN_DIAGNOSIS"
	ActionOpenMarketIntel AgentActionType = "OPEN_MARKET_INTELLIGENCE"
	ActionShowKline       AgentActionType = "SHOW_KLINE"
	ActionDisciplineAlert AgentActionType = "DISCIPLINE_ALERT"
	ActionFilterWatchlist AgentActionType = "FILTER_WATCHLIST"
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
// 提取过程中会：
//  1. 过滤掉包含占位符（如 <<STOCK_CODE>>、<<STOCK_NAME>>）的示例动作；
//  2. 去除同一次响应中重复出现的相同动作（基于 Type + Payload 的 JSON 序列化）。
func ParseAndExtractActions(rawText string) (string, []AgentAction) {
	if rawText == "" {
		return "", nil
	}

	var rawActions []AgentAction
	cleanedText := actionBlockRegex.ReplaceAllStringFunc(rawText, func(match string) string {
		subMatches := actionBlockRegex.FindStringSubmatch(match)
		if len(subMatches) > 1 {
			jsonBlock := strings.TrimSpace(subMatches[1])
			// 尝试解析为单动作或者动作数组
			var singleAction AgentAction
			if err := json.Unmarshal([]byte(jsonBlock), &singleAction); err == nil && singleAction.Type != "" {
				rawActions = append(rawActions, singleAction)
			} else {
				var actionList []AgentAction
				if err := json.Unmarshal([]byte(jsonBlock), &actionList); err == nil {
					for _, a := range actionList {
						if a.Type != "" {
							rawActions = append(rawActions, a)
						}
					}
				}
			}
		}
		return "" // 将动作指令块从主文本中移除，避免污染用户界面
	})

	// 过滤占位符示例 & 去重
	seen := make(map[string]struct{})
	var actions []AgentAction
	for _, a := range rawActions {
		if hasPlaceholder(a) {
			continue
		}
		key, err := actionKey(a)
		if err != nil {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		actions = append(actions, a)
	}

	return strings.TrimSpace(cleanedText), actions
}

// hasPlaceholder 检测动作的 payload 中是否包含 <<...>> 占位符（示例）
func hasPlaceholder(a AgentAction) bool {
	for _, v := range a.Payload {
		switch val := v.(type) {
		case string:
			if strings.Contains(val, "<<") && strings.Contains(val, ">>") {
				return true
			}
		}
	}
	return false
}

// actionKey 序列化动作（Type+Payload）为字符串，用作去重 key
func actionKey(a AgentAction) (string, error) {
	payloadBytes, err := json.Marshal(a.Payload)
	if err != nil {
		return "", err
	}
	return string(a.Type) + "|" + string(payloadBytes), nil
}
