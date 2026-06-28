package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-stock/backend/data"
	"go-stock/backend/db"
	"go-stock/backend/models"
	"io"
	"net/http"
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

type Evolver struct{}

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

type LearnedSkill struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	TriggerKeywords []string `json:"triggerKeywords"`
	SystemPrompt    string   `json:"systemPrompt"`
	Examples        string   `json:"examples"`
}

func (ev *Evolver) LearnFromProject(projectName, analysis string, llm LLMOptimizerFunc) (*LearnedSkill, error) {
	if llm == nil {
		return nil, fmt.Errorf("llm function required for learning")
	}

	prompt := fmt.Sprintf(`你是一个技能提取专家。根据以下对 "%s" 项目的分析报告，提取出一个可复用的 Agent 技能。

## 项目分析报告
%s

## 要求
输出 JSON 格式（仅输出 JSON，不要包含其他内容）：
{
  "name": "技能名称（简洁，如"Serenity式投研"）",
  "description": "技能描述（一句话说明这个技能做什么）",
  "triggerKeywords": ["关键词1", "关键词2", "关键词3"],
  "systemPrompt": "技能的系统提示词（完整描述该技能的工作流、规则、输出格式要求，直接可注入 Agent）",
  "examples": "使用示例（当用户说类似的话时，Agent 应该如何回应的示例）"
}

注意：systemPrompt 要完整、可直接使用，包含工作流步骤、判断标准、输出规范。`, projectName, analysis)

	raw, err := llm("你是一个技能提取专家，以 JSON 格式输出。", prompt)
	if err != nil {
		return nil, fmt.Errorf("llm learn failed: %w", err)
	}
	raw = extractJSON(raw)

	var skill LearnedSkill
	if err := json.Unmarshal([]byte(raw), &skill); err != nil {
		return nil, fmt.Errorf("parse skill JSON failed: %w, raw: %s", err, raw)
	}
	if skill.Name == "" {
		return nil, fmt.Errorf("learned skill has empty name")
	}

	dbSkill := &models.Skill{
		Name:            skill.Name,
		Description:     skill.Description,
		Category:        "learned",
		SystemPrompt:    skill.SystemPrompt,
		Examples:        skill.Examples,
		TriggerKeywords: strings.Join(skill.TriggerKeywords, ","),
		Enable:          true,
	}
	if err := data.NewSkillApi().Create(dbSkill); err != nil {
		return nil, fmt.Errorf("save skill failed: %w", err)
	}

	return &skill, nil
}

func (ev *Evolver) CleanupToolRanks(days int) (int, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	r := db.Dao.Where("updated_at < ?", cutoff).Delete(&ToolRank{})
	return int(r.RowsAffected), r.Error
}

// LLMFromConfig creates an LLM callback from the first enabled AI config.
func LLMFromConfig() LLMOptimizerFunc {
	return func(systemPrompt, userMessage string) (string, error) {
		settingConfig := data.GetSettingConfig()
		if settingConfig == nil || len(settingConfig.AiConfigs) == 0 {
			return "", fmt.Errorf("no AI configs available")
		}
		var cfg *data.AIConfig
		for _, c := range settingConfig.AiConfigs {
			if c.BaseUrl != "" && c.ApiKey != "" {
				cfg = c
				break
			}
		}
		if cfg == nil {
			return "", fmt.Errorf("no enabled AI config with API key")
		}
		baseURL := strings.TrimSuffix(strings.TrimSpace(cfg.BaseUrl), "/")
		baseURL = strings.TrimSuffix(baseURL, "/chat/completions")
		modelName := cfg.ModelName
		if modelName == "" {
			modelName = "gpt-4o"
		}

		body := map[string]any{
			"model": modelName,
			"messages": []map[string]string{
				{"role": "system", "content": systemPrompt},
				{"role": "user", "content": userMessage},
			},
			"temperature": 0.3,
			"max_tokens":  4096,
		}
		bodyBytes, _ := json.Marshal(body)

		req, err := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
		if err != nil {
			return "", fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+cfg.ApiKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("http call: %w", err)
		}
		defer resp.Body.Close()

		respBytes, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBytes))
		}

		var result struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(respBytes, &result); err != nil {
			return "", fmt.Errorf("parse response: %w", err)
		}
		if len(result.Choices) == 0 {
			return "", fmt.Errorf("no choices in response")
		}
		return result.Choices[0].Message.Content, nil
	}
}

var _ = fmt.Sprintf
