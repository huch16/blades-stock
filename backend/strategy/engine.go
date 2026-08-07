package strategy

import "math"

// ScoreDetail 描述一条命中的加分/扣分规则。
type ScoreDetail struct {
	Kind   string  // "bonus" 或 "penalty"
	Rule   string  // 条件表达式
	Reason string  // 人类可读理由
	Delta  float64 // 实际生效的分数变化（penalty 为负）
}

// StrategyScore 是策略评分结果。
type StrategyScore struct {
	Base    float64 // 基础分
	Score   float64 // 原始累计分（可能超出 0-100）
	Clamped float64 // 收敛到 0-100 后的最终分
	Matched []ScoreDetail
	Missed  []string // 未命中的条件
	Skipped []string // 无法识别而跳过的条件
}

// ScoreStrategy 按策略声明对指标快照打分：
// 以 BaseScore 为起点，依次应用命中 bonuses（加分）与 penalties（扣减绝对分），最后收敛到 0-100。
// 声明中无法识别的条件记录到 Skipped，不会参与计算。
func ScoreStrategy(s *Strategy, ind *Indicators) StrategyScore {
	out := StrategyScore{Base: s.BaseScore, Score: s.BaseScore}

	apply := func(rule Rule, kind string) {
		name, _, err := ParseCondition(rule.When)
		if err != nil || name == "" || !IsKnownCondition(name) {
			out.Skipped = append(out.Skipped, rule.When)
			return
		}
		if evalCondition(rule.When, ind) {
			delta := rule.Score
			if kind == "penalty" {
				delta = -math.Abs(delta)
			}
			out.Score += delta
			out.Matched = append(out.Matched, ScoreDetail{
				Kind: kind, Rule: rule.When, Reason: rule.Desc, Delta: delta,
			})
			return
		}
		out.Missed = append(out.Missed, rule.When)
	}

	for _, r := range s.Bonuses {
		apply(r, "bonus")
	}
	for _, r := range s.Penalties {
		apply(r, "penalty")
	}

	out.Clamped = math.Max(0, math.Min(100, out.Score))
	return out
}
