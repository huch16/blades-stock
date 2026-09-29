package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/logger"
)

// SummaryTurn 表示对话轮次，用于摘要输入
type SummaryTurn struct {
	Role    string `json:"role"`    // user / assistant
	Content string `json:"content"` // 消息内容
}

// LLMConfig 简化的 LLM 配置，仅用于摘要
type LLMConfig struct {
	BaseURL     string
	APIKey      string
	Model       string
	MaxTokens   int
	Temperature float64
}

// SummaryResult LLM 返回的摘要结果
type SummaryResult struct {
	Summary       string   `json:"summary"`
	KeyPoints     []string `json:"key_points,omitempty"`
	DecisionPoint string   `json:"decision_point,omitempty"`
	RiskWarning   string   `json:"risk_warning,omitempty"`
}

// SummaryPromptData 构建给 LLM 的 prompt 数据
type SummaryPromptData struct {
	SystemPrompt string       `json:"system_prompt"`
	Turns        []SummaryTurn `json:"turns"`
	MaxTokens    int          `json:"max_tokens"`
}

// defaultSummaryModel 默认摘要模型（可被 settings 覆盖）
const defaultSummaryModel = "deepseek-chat"

// summaryPromptTemplate 系统提示词模板
const summaryPromptTemplate = `你是专业的金融对话摘要助手。请将用户与助手的多轮对话压缩为一段 ≤ 200 tokens 的结构化摘要。

【核心要求】
1. 必须保留：决策点、关键数字（价格、比率、金额）、具体结论、风险提示
2. 去除：客套话、重复确认、无实质内容的过渡语
3. 格式：单段话，中文，≤ 200 tokens

【输出结构（JSON）】
{
  "summary": "核心摘要（≤200 tokens，保留决策点/数字/结论/风险）",
  "key_points": ["关键点1", "关键点2", ...],
  "decision_point": "最终决策/建议（如：买入/卖出/观望 + 理由）",
  "risk_warning": "主要风险提示（如：流动性风险/政策风险/估值风险）"
}

【示例输入】
user: 159915 现在 1.05 值得买吗？
assistant: 159915 当前 1.05，溢价率 0.8%，近期溢价率中位数 1.2%，建议观望等回调
user: 那 512660 呢？
assistant: 512660 溢价率 -0.5%，折价买入较安全，但需关注流动性

【示例输出】
{
  "summary": "对比 159915(溢价0.8%) 与 512660(折价-0.5%)，前者溢价风险较大，后者折价买入相对安全但需关注流动性",
  "key_points": ["159915溢价0.8%高于中位数", "512660折价-0.5%低估", "流动性风险需关注"],
  "decision_point": "建议观望159915等回调，可考虑512660分批买入但控制仓位",
  "risk_warning": "ETF流动性风险、溢价回归风险、市场系统性风险"
}`

// LLMSummarize 调用 LLM 对多轮对话进行二次摘要
// 仅在 summaryTokens > 800 时触发；失败/未配置 → 降级到 shrinkSummary
func LLMSummarize(ctx context.Context, turns []Turn, summaryModel string) (string, error) {
	if len(turns) == 0 {
		return "", nil
	}

	// 统计当前 summary token 估算
	var totalTokens int
	for _, t := range turns {
		totalTokens += len(t.Content) / 2 // 粗略估算
	}
	if totalTokens <= 800 {
		// 不需要摘要，直接拼接
		return concatenateTurns(turns), nil
	}

	// 获取摘要模型配置
	cfg := getSummaryModelConfig(summaryModel)
	if cfg == nil {
		logger.SugaredLogger.Warn("摘要模型未配置，降级到保尾策略")
		return shrinkSummaryFallback(turns), nil
	}

	// 构建请求
	promptData := buildSummaryPrompt(turns)
	reqBody, _ := json.Marshal(promptData)

	// 调用 LLM
	resp, err := callSummaryLLM(ctx, cfg, reqBody)
	if err != nil {
		logger.SugaredLogger.Warnf("LLM 摘要调用失败，降级保尾: %v", err)
		return shrinkSummaryFallback(turns), nil
	}

	// 解析响应
	var result SummaryResult
	if err := json.Unmarshal(resp, &result); err != nil {
		logger.SugaredLogger.Warnf("LLM 摘要解析失败，降级保尾: %v", err)
		return shrinkSummaryFallback(turns), nil
	}

	// 验证结果
	if result.Summary == "" {
		logger.SugaredLogger.Warn("LLM 返回空摘要，降级保尾")
		return shrinkSummaryFallback(turns), nil
	}

	// 构建最终摘要字符串
	return formatSummaryResult(result), nil
}

// getSummaryModelConfig 获取摘要模型配置（从 settings 读取）
func getSummaryModelConfig(modelName string) *LLMConfig {
	if modelName == "" {
		modelName = defaultSummaryModel
	}
	setting := data.GetSettingConfig()
	for _, cfg := range setting.AiConfigs {
		if cfg != nil && (cfg.Name == modelName || cfg.ModelName == modelName) {
			return &LLMConfig{
				BaseURL:     cfg.BaseUrl,
				APIKey:      cfg.ApiKey,
				Model:       cfg.ModelName,
				MaxTokens:   500,
				Temperature: 0.1,
			}
		}
	}
	// 兜底：使用第一个可用配置
	for _, cfg := range setting.AiConfigs {
		if cfg != nil && cfg.BaseUrl != "" && cfg.ApiKey != "" {
			return &LLMConfig{
				BaseURL:     cfg.BaseUrl,
				APIKey:      cfg.ApiKey,
				Model:       cfg.ModelName,
				MaxTokens:   500,
				Temperature: 0.1,
			}
		}
	}
	return nil
}

// buildSummaryPrompt 构建给 LLM 的完整 prompt
func buildSummaryPrompt(turns []Turn) SummaryPromptData {
	var turnsData []SummaryTurn
	for _, t := range turns {
		turnsData = append(turnsData, SummaryTurn{
			Role:    t.Role,
			Content: t.Content,
		})
	}
	return SummaryPromptData{
		SystemPrompt: summaryPromptTemplate,
		Turns:        turnsData,
		MaxTokens:    200,
	}
}

// callSummaryLLM 调用 LLM API（OpenAI 兼容格式）
func callSummaryLLM(ctx context.Context, cfg *LLMConfig, promptBody []byte) ([]byte, error) {
	if cfg == nil || cfg.BaseURL == "" || cfg.APIKey == "" {
		return nil, fmt.Errorf("invalid LLM config for summarizer")
	}

	var pData SummaryPromptData
	if err := json.Unmarshal(promptBody, &pData); err != nil {
		return nil, fmt.Errorf("unmarshal prompt data failed: %w", err)
	}

	messages := []map[string]string{
		{"role": "system", "content": pData.SystemPrompt},
	}
	for _, turn := range pData.Turns {
		messages = append(messages, map[string]string{
			"role":    turn.Role,
			"content": turn.Content,
		})
	}

	reqPayload := map[string]interface{}{
		"model":       cfg.Model,
		"messages":    messages,
		"temperature": cfg.Temperature,
		"max_tokens":  cfg.MaxTokens,
		"response_format": map[string]string{
			"type": "json_object",
		},
	}

	jsonBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("marshal summary request payload failed: %w", err)
	}

	endpoint := strings.TrimSuffix(cfg.BaseURL, "/")
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint += "/chat/completions"
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("create summary http request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	client := &http.Client{
		Timeout: 12 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do summary http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("summary http request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read summary http response body failed: %w", err)
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
		return nil, fmt.Errorf("unmarshal summary chat completion failed: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("summary response contains 0 choices")
	}

	return []byte(chatResp.Choices[0].Message.Content), nil
}

// concatenateTurns 简单拼接轮次（不摘要时）
func concatenateTurns(turns []Turn) string {
	var result string
	for i, t := range turns {
		if i > 0 {
			result += "\n"
		}
		result += t.Role + ": " + t.Content
	}
	return result
}

// shrinkSummaryFallback 降级保尾策略（复用现有逻辑）
func shrinkSummaryFallback(turns []Turn) string {
	const keepFull = 3
	if len(turns) <= keepFull {
		return concatenateTurns(turns)
	}
	var result string
	// 早期轮次压缩
	for i := 0; i < len(turns)-keepFull; i++ {
		t := turns[i]
		result += fmt.Sprintf("[%s] %s: %s\n", "摘要", t.Role, truncateForSummary(t.Content, 100))
	}
	// 保留最近轮次完整
	for i := len(turns) - keepFull; i < len(turns); i++ {
		t := turns[i]
		result += fmt.Sprintf("[%s] %s: %s\n", "完整", t.Role, t.Content)
	}
	return result
}

// truncateForSummary 截断用于摘要的文本
func truncateForSummary(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...(省略)"
}

// formatSummaryResult 格式化最终摘要输出
func formatSummaryResult(r SummaryResult) string {
	var result string
	if r.Summary != "" {
		result += r.Summary
	}
	if len(r.KeyPoints) > 0 {
		result += "\n关键点: " + joinKeyPoints(r.KeyPoints)
	}
	if r.DecisionPoint != "" {
		result += "\n决策: " + r.DecisionPoint
	}
	if r.RiskWarning != "" {
		result += "\n风险: " + r.RiskWarning
	}
	return result
}

func joinKeyPoints(points []string) string {
	if len(points) == 0 {
		return ""
	}
	result := points[0]
	for i := 1; i < len(points); i++ {
		result += "; " + points[i]
	}
	return result
}