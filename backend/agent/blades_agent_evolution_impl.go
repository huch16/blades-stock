package agent

import (
	"encoding/json"
	"fmt"
	"go-stock/backend/db"
	"strings"
	"time"
)

type ToolRank struct {
	ToolName    string  `json:"toolName" gorm:"primaryKey;size:64"`
	SuccessCount int    `json:"successCount" gorm:"default:0"`
	FailCount    int    `json:"failCount" gorm:"default:0"`
	AvgDurationMs float64 `json:"avgDurationMs" gorm:"default:0"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (ToolRank) TableName() string { return "agent_tool_ranks" }

type EvolutionPlan struct {
	WeekStart       string   `json:"weekStart"`
	WeekEnd         string   `json:"weekEnd"`
	HighQualityCount int     `json:"highQualityCount"`
	LowQualityCount  int     `json:"lowQualityCount"`
	TopTools        []ToolRank `json:"topTools"`
	Improvements    []string `json:"improvements"`
	Suggestions     []string `json:"suggestions"`
}

type Evolver struct {
	db          bool
}

func NewEvolver() *Evolver {
	return &Evolver{}
}

func init() {
	db.AutoMigrateList = append(db.AutoMigrateList, &ToolRank{})
}

type LLMOptimizerFunc func(systemPrompt, userMessage string) (string, error)

func (ev *Evolver) OptimizePrompt(trace *AgentTrace, llm LLMOptimizerFunc) (string, error) {
	if llm == nil {
		return "", fmt.Errorf("llm function required for prompt optimization")
	}
	prompt := fmt.Sprintf(`你是一个 Prompt 工程优化专家。根据以下智能体执行痕迹，优化该智能体的系统提示词以提高其表现。

## 问题
%s

## 回答（可能存在质量问题）
%s

## 反思
%s

## 要求
1. 分析当前 Prompt 可能存在的不足
2. 给出优化后的 Prompt 建议
3. 以 JSON 格式输出：{"analysis": "问题分析", "optimizedPrompt": "优化后的Prompt"}

当前系统提示词版本的主要问题可能是：
- 指令不够清晰导致工具选择错误
- 缺少特定场景的处理指引
- 对结果格式要求不够严格
- 缺少错误恢复策略`, trace.Question, truncateText(trace.Answer, 500), trace.Feedback)

	raw, err := llm("你是一个 Prompt 优化助手，只输出 JSON。", prompt)
	if err != nil {
		return "", fmt.Errorf("llm optimize failed: %w", err)
	}
	raw = extractJSON(raw)
	var result struct {
		Analysis       string `json:"analysis"`
		OptimizedPrompt string `json:"optimizedPrompt"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return raw, nil
	}
	return fmt.Sprintf("分析: %s\n\n优化建议:\n%s", result.Analysis, result.OptimizedPrompt), nil
}

func (ev *Evolver) GenerateSkillFromTraces(llm LLMOptimizerFunc) (string, error) {
	if llm == nil {
		return "", fmt.Errorf("llm function required for skill generation")
	}
	highTraces := NewEvaluator().GetHighQualityTraces(10)
	if len(highTraces) == 0 {
		return "当前无足够高质量轨迹用于技能提取", nil
	}

	var examples []string
	for _, t := range highTraces {
		examples = append(examples, fmt.Sprintf("- 问题: %s", t.Question))
	}
	exampleText := strings.Join(examples, "\n")

	prompt := fmt.Sprintf(`你是一个技能提取专家。分析以下智能体成功执行的轨迹，提取出可复用的技能。

## 成功轨迹示例
%s

## 要求
输出 JSON 格式的技能草稿（仅输出 JSON）：
{
  "skills": [
    {
      "name": "技能名称",
      "description": "技能描述",
      "triggerKeywords": ["关键词1", "关键词2"],
      "systemPrompt": "技能提示词（复用成功模式）",
      "examples": "技能适用场景示例"
    }
  ]
}

注意：每个技能应当是从多条轨迹中抽象出的通用模式。`, exampleText)

	raw, err := llm("你是一个技能提取专家，以 JSON 格式输出。", prompt)
	if err != nil {
		return "", fmt.Errorf("llm skill generation failed: %w", err)
	}
	return extractJSON(raw), nil
}

func (ev *Evolver) RankTools() []ToolRank {
	var ranks []ToolRank
	db.Dao.Order("success_count desc").Find(&ranks)
	if len(ranks) == 0 {
		return nil
	}
	return ranks
}

func (ev *Evolver) RecordToolUse(toolName string, success bool, durationMs float64) {
	var rank ToolRank
	err := db.Dao.Where("tool_name = ?", toolName).First(&rank).Error
	if err != nil {
		rank = ToolRank{ToolName: toolName}
	}
	if success {
		rank.SuccessCount++
	} else {
		rank.FailCount++
	}
	total := float64(rank.SuccessCount + rank.FailCount)
	if total > 0 {
		rank.AvgDurationMs = (rank.AvgDurationMs*(total-1) + durationMs) / total
	}
	rank.UpdatedAt = time.Now()
	db.Dao.Save(&rank)
}

func (ev *Evolver) ProposeEvolutionPlan() *EvolutionPlan {
	now := time.Now()
	weekStart := now.AddDate(0, 0, -int(now.Weekday()))
	weekEnd := weekStart.AddDate(0, 0, 6)

	var lowTraces []AgentTrace
	db.Dao.Where("score < 0 AND created_at >= ?", weekStart).Order("score asc").Limit(10).Find(&lowTraces)

	var highTraces []AgentTrace
	db.Dao.Where("score > 0 AND created_at >= ?", weekStart).Order("score desc").Limit(10).Find(&highTraces)

	ranks := ev.RankTools()

	var improvements []string
	for _, t := range lowTraces {
		parts := strings.SplitN(t.Feedback, "|", 2)
		if len(parts) == 2 {
			improvements = append(improvements, strings.TrimSpace(parts[1]))
		}
	}
	if len(improvements) == 0 {
		improvements = append(improvements, "本周无低分轨迹，继续保持")
	}

	var suggestions []string
	for _, t := range highTraces {
		suggestions = append(suggestions, t.Question)
	}
	if len(suggestions) == 0 {
		suggestions = append(suggestions, "继续收集高质量轨迹数据")
	}

	return &EvolutionPlan{
		WeekStart:        weekStart.Format("2006-01-02"),
		WeekEnd:          weekEnd.Format("2006-01-02"),
		HighQualityCount: len(highTraces),
		LowQualityCount:  len(lowTraces),
		TopTools:         ranks,
		Improvements:     improvements,
		Suggestions:      suggestions,
	}
}

func (ev *Evolver) CleanupToolRanks(days int) (int, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	r := db.Dao.Where("updated_at < ?", cutoff).Delete(&ToolRank{})
	return int(r.RowsAffected), r.Error
}

var _ = fmt.Sprintf
