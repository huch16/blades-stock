package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-stock/backend/agent/types"
	"go-stock/backend/data"
	"go-stock/backend/logger"
	"go-stock/backend/models"
	"io"
	"net/http"
	"strings"

	bladesTools "github.com/go-kratos/blades/tools"
)

type learnedSkill struct {
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	TriggerKeywords []string `json:"triggerKeywords"`
	SystemPrompt    string   `json:"systemPrompt"`
	Examples        string   `json:"examples"`
}

func callLLM(systemPrompt, userMessage string) (string, error) {
	settingConfig := data.GetSettingConfig()
	if settingConfig == nil || len(settingConfig.AiConfigs) == 0 {
		return "", fmt.Errorf("无可用 AI 配置")
	}
	var cfg *data.AIConfig
	for _, c := range settingConfig.AiConfigs {
		if c.BaseUrl != "" && c.ApiKey != "" {
			cfg = c
			break
		}
	}
	if cfg == nil {
		return "", fmt.Errorf("无已配置 API Key 的 AI 配置")
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
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.ApiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API 错误 %d: %s", resp.StatusCode, string(respBytes))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("API 返回空结果")
	}
	return result.Choices[0].Message.Content, nil
}

func runLearnSkill(projectName, analysis string) (*learnedSkill, error) {
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

	raw, err := callLLM("你是一个技能提取专家，以 JSON 格式输出。", prompt)
	if err != nil {
		return nil, err
	}

	// extract JSON block if wrapped in markdown
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		raw = raw[start : end+1]
	}

	var skill learnedSkill
	if err := json.Unmarshal([]byte(raw), &skill); err != nil {
		return nil, fmt.Errorf("解析失败: %w, 原始: %s", err, raw)
	}
	if skill.Name == "" {
		return nil, fmt.Errorf("技能名称为空")
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
		return nil, fmt.Errorf("保存技能失败: %w", err)
	}
	return &skill, nil
}

func GetLearnSkillTool() bladesTools.Tool {
	return types.NewTool("LearnSkillFromProject",
		"学习外部项目的方法论并注册为可复用技能。当用户要求学习某个项目/方法论/框架时调用。输入项目分析文本，自动提取技能并保存到数据库，之后匹配关键词时自动激活。",
		map[string]*types.ParamDef{
			"projectName": {Type: "string", Desc: "项目/方法论名称", Required: true},
			"analysis":    {Type: "string", Desc: "对项目的详细分析文本", Required: true},
		},
		func(args string) (string, error) {
			logger.SugaredLogger.Infof("LearnSkillFromProject called with args: %s", args)
			parms := map[string]any{}
			if err := json.Unmarshal([]byte(args), &parms); err != nil {
				return "", err
			}
			projectName, _ := parms["projectName"].(string)
			analysis, _ := parms["analysis"].(string)
			if projectName == "" {
				return "项目名称不能为空", nil
			}
			if analysis == "" {
				return "分析内容不能为空", nil
			}
			skill, err := runLearnSkill(projectName, analysis)
			if err != nil {
				return fmt.Sprintf("学习失败: %v", err), nil
			}
			return fmt.Sprintf("✅ 已成功学习「%s」技能！\n描述: %s\n触发关键词: %s\n下次对话中匹配到关键词时自动激活。",
				skill.Name, skill.Description, strings.Join(skill.TriggerKeywords, ",")), nil
		},
	)
}
