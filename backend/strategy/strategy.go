// Package strategy implements a declarative, YAML-driven stock screening
// strategy framework: strategies declare base scores, bonus rules and penalty
// rules over real K-line indicators, and the scoring engine evaluates them.
//
// Unlike Stock-dashboard's declaration/execution split (where YAML bonuses and
// penalties were never actually evaluated), this package's scoring engine runs
// the declared rules against indicators computed from real K-line data.
package strategy

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v2"
)

// Rule 是策略中的一个加分或扣分规则。
// When 使用条件 DSL 描述（如 "ma_golden_cross(5,10)"、"volume_ratio_gt(1.5)"）。
type Rule struct {
	When  string  `yaml:"when"`  // 条件表达式
	Score float64 `yaml:"score"` // 分数变化（bonus 为正，penalty 按绝对值扣减）
	Desc  string  `yaml:"desc"`  // 人类可读的理由
}

// Strategy 是一份策略声明，对应一个 YAML 文件。
type Strategy struct {
	Name               string   `yaml:"name"`
	Alias              []string `yaml:"alias"`
	Category           string   `yaml:"category"`
	JudgmentCriteria   string   `yaml:"judgment_criteria"`
	Description        string   `yaml:"description"`
	BaseScore          float64  `yaml:"base_score"`
	Bonuses            []Rule   `yaml:"bonuses"`
	Penalties          []Rule   `yaml:"penalties"`
	RequiredIndicators []string `yaml:"required_indicators"`
	RiskLevel          string   `yaml:"risk_level"`
	HoldPeriod         string   `yaml:"hold_period"`
}

// Parse 解析单个 YAML 策略文档。
func Parse(data []byte) (*Strategy, error) {
	var s Strategy
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("解析策略 YAML 失败: %w", err)
	}
	if strings.TrimSpace(s.Name) == "" {
		return nil, fmt.Errorf("策略缺少 name 字段")
	}
	return &s, nil
}

// Resolve 按名称或别名查找策略，支持不区分大小写的精确匹配，回退到包含匹配。
func Resolve(list []Strategy, nameOrAlias string) (*Strategy, bool) {
	target := strings.ToLower(strings.TrimSpace(nameOrAlias))
	if target == "" {
		return nil, false
	}
	for i := range list {
		if strings.ToLower(list[i].Name) == target {
			return &list[i], true
		}
		for _, a := range list[i].Alias {
			if strings.ToLower(strings.TrimSpace(a)) == target {
				return &list[i], true
			}
		}
	}
	for i := range list {
		if strings.Contains(strings.ToLower(list[i].Name), target) {
			return &list[i], true
		}
	}
	return nil, false
}
