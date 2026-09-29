package agent

import "strings"

// containsDecisionKeyword 判断内容是否包含"决策性"关键词。
// 用于 P2.2 的 IsDecision 自动推断，避免调用方手动指定每个 turn 的 importance。
//
// 判定标准（任一命中即视为决策）：
//   - "建议" / "结论" / "总结" — 报告型回答必有这些
//   - "买入" / "卖出" / "持有" / "观望" / "加仓" / "减仓" — 交易动作
//   - "操作" + ("建议" 或 "策略") — 操作建议
//   - "建议" + ("买入" 或 "卖出" 或 "持有") — 综合表达
//
// 故意保守：宁可漏判也不要误判（误判会让 turn 永不压缩，长期占用 fullTurns 槽位）。
func containsDecisionKeyword(content string) bool {
	if len(content) < 10 {
		return false
	}
	lower := strings.ToLower(content)

	// 高权重词（任一命中即视为决策）
	strongKeywords := []string{"买入", "卖出", "加仓", "减仓", "止损", "止盈"}
	for _, kw := range strongKeywords {
		if strings.Contains(content, kw) {
			return true
		}
	}

	// 中权重词（与"建议/结论/操作"组合）
	mediumKeywords := []string{"结论", "总结", "最终建议"}
	for _, kw := range mediumKeywords {
		if strings.Contains(content, kw) {
			return true
		}
	}

	// 低权重词（单独不够）
	weakKeywords := []string{"建议", "操作策略"}
	hasWeak := false
	for _, kw := range weakKeywords {
		if strings.Contains(content, kw) {
			hasWeak = true
			break
		}
	}
	if !hasWeak {
		return false
	}

	// 弱权重词 + 强/中权重词共现 → 视为决策
	combinedKeywords := append(strongKeywords, mediumKeywords...)
	for _, kw := range combinedKeywords {
		if strings.Contains(content, kw) {
			return true
		}
	}

	// "建议持有/买入/卖出" 形式
	hintPatterns := []string{"建议持有", "建议买入", "建议卖出", "建议观望"}
	for _, p := range hintPatterns {
		if strings.Contains(content, p) {
			return true
		}
	}

	_ = lower
	return false
}