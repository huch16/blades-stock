package data

import (
	"blades-stock/backend/db"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"blades-stock/backend/logger"
	"blades-stock/backend/models"

	"github.com/duke-git/lancet/v2/convertor"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/go-resty/resty/v2"
)

type THSTokenResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

type AiResponse struct {
	Id          string `json:"id"`
	Object      string `json:"object"`
	Created     int    `json:"created"`
	Model       string `json:"model"`
	ServiceTier string `json:"service_tier"`
	Choices     []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		Logprobs     interface{} `json:"logprobs"`
		FinishReason string      `json:"finish_reason"`
		Delta        struct {
			Content   string `json:"content"`
			Role      string `json:"role"`
			ToolCalls []struct {
				Function struct {
					Arguments string `json:"arguments"`
					Name      string `json:"name"`
				} `json:"function"`
				Id    string `json:"id"`
				Index int    `json:"index"`
				Type  string `json:"type"`
			} `json:"tool_calls"`
		} `json:"delta"`
	} `json:"choices"`
	Usage struct {
		PromptTokens          int `json:"prompt_tokens"`
		CompletionTokens      int `json:"completion_tokens"`
		TotalTokens           int `json:"total_tokens"`
		PromptCacheHitTokens  int `json:"prompt_cache_hit_tokens"`
		PromptCacheMissTokens int `json:"prompt_cache_miss_tokens"`
	} `json:"usage"`
	SystemFingerprint string `json:"system_fingerprint"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type FunctionParameters struct {
	Type                 string         `json:"type"`
	Properties           map[string]any `json:"properties"`
	Required             []string       `json:"required,omitempty"`
	AdditionalProperties bool           `json:"additionalProperties"`
}

type ToolFunction struct {
	Name        string              `json:"name"`
	Strict      bool                `json:"strict,omitempty"`
	Description string              `json:"description"`
	Parameters  *FunctionParameters `json:"parameters,omitempty"`
}

// appendToolMessages 统一向 messages 追加一次工具调用的 assistant/tool 两条消息
func appendToolMessages(
	messages *[]map[string]any,
	currentAIContent, reasoningContent, callID, funcName, funcArgs, toolContent string,
) {
	assistantMsg := map[string]any{
		"role":    "assistant",
		"content": currentAIContent,
		"tool_calls": []map[string]any{
			{
				"id": callID,
				//"tool_call_id": callID,
				"type": "function",
				"function": map[string]string{
					"name":      funcName,
					"arguments": funcArgs,
					//"parameters": funcArgs,
				},
			},
		},
	}
	if reasoningContent != "" {
		assistantMsg["reasoning_content"] = reasoningContent
	}
	*messages = append(*messages, assistantMsg)

	*messages = append(*messages, map[string]any{
		"role":         "tool",
		"content":      toolContent,
		"tool_call_id": callID,
	})
}

// thsResultToMarkdown 将通联/同花顺搜索结果统一转换为 markdown 表格
func thsResultToMarkdown(res map[string]any, title string) string {
	if convertor.ToString(res["code"]) != "100" {
		return "无符合条件的数据"
	}

	resData, ok := res["data"].(map[string]any)
	if !ok {
		return "无符合条件的数据"
	}
	result, ok := resData["result"].(map[string]any)
	if !ok {
		return "无符合条件的数据"
	}

	dataList, ok := result["dataList"].([]any)
	if !ok {
		return "无符合条件的数据"
	}
	columns, ok := result["columns"].([]any)
	if !ok {
		return "无符合条件的数据"
	}

	headers := map[string]string{}
	for _, v := range columns {
		d := v.(map[string]any)
		colTitle := convertor.ToString(d["title"])
		if dm := convertor.ToString(d["dateMsg"]); dm != "" {
			colTitle += "[" + dm + "]"
		}
		if u := convertor.ToString(d["unit"]); u != "" {
			colTitle += "(" + u + ")"
		}
		headers[d["key"].(string)] = colTitle
	}

	table := &[]map[string]any{}
	for _, v := range dataList {
		d := v.(map[string]any)
		row := map[string]any{}
		for key, colTitle := range headers {
			row[colTitle] = convertor.ToString(d[key])
		}
		*table = append(*table, row)
	}

	jsonData, _ := json.Marshal(*table)
	markdownTable, _ := JSONToMarkdownTable(jsonData)
	return "\r\n### " + title + "：\r\n" + markdownTable + "\r\n"
}

func stripReasoningContent(messages []map[string]interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(messages))
	for _, msg := range messages {
		newMsg := make(map[string]interface{})
		for k, v := range msg {
			if k == "reasoning_content" {
				continue
			}
			newMsg[k] = v
		}
		result = append(result, newMsg)
	}
	return result
}

func hasReasoningContent(messages []map[string]interface{}) bool {
	for _, msg := range messages {
		if _, ok := msg["reasoning_content"]; ok {
			return true
		}
	}
	return false
}

func stripToolMessages(messages []map[string]interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(messages))
	for _, message := range messages {
		if message["role"] == "tool" {
			continue
		}
		if _, ok := message["tool_calls"]; ok {
			continue
		}
		result = append(result, message)
	}
	return result
}

func openAIHTTPErrorMessage(statusCode int, bodyBytes []byte) string {
	bodyText := strings.TrimSpace(string(bodyBytes))
	if bodyText == "" {
		return fmt.Sprintf("HTTP %d", statusCode)
	}

	res := &models.Resp{}
	if err := json.Unmarshal(bodyBytes, res); err == nil {
		if res.Error.Message != "" {
			return fmt.Sprintf("HTTP %d: %s", statusCode, res.Error.Message)
		}
		if res.Message != "" {
			return fmt.Sprintf("HTTP %d: %s", statusCode, res.Message)
		}
	}
	return fmt.Sprintf("HTTP %d: %s", statusCode, bodyText)
}

func isUnsupportedThinkingError(msg string) bool {
	lower := strings.ToLower(msg)
	if !strings.Contains(lower, "thinking") {
		return false
	}
	return strings.Contains(lower, "unsupported") ||
		strings.Contains(lower, "not support") ||
		strings.Contains(lower, "not_supported") ||
		strings.Contains(lower, "invalid") ||
		strings.Contains(lower, "unknown") ||
		strings.Contains(lower, "unrecognized") ||
		strings.Contains(lower, "extra")
}

func isUnsupportedReasoningContentError(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "reasoning_content") ||
		strings.Contains(lower, "reasoning content")
}

func isUnsupportedToolError(msg string) bool {
	lower := strings.ToLower(msg)
	if !strings.Contains(lower, "tool") && !strings.Contains(lower, "function") {
		return false
	}
	return strings.Contains(lower, "unsupported") ||
		strings.Contains(lower, "not support") ||
		strings.Contains(lower, "not_supported") ||
		strings.Contains(lower, "invalid parameter") ||
		strings.Contains(lower, "invalid params") || // MiniMax: "invalid params, function name or parameters is empty (2013)"
		strings.Contains(lower, "unknown parameter") ||
		strings.Contains(lower, "unrecognized") ||
		strings.Contains(lower, "extra") ||
		strings.Contains(lower, "empty") || // MiniMax: "function name or parameters is empty"
		strings.Contains(lower, "2013") // MiniMax error code for empty function name/parameters
}

func openAIChatEndpoint(baseURL string) (string, string) {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return base, "/chat/completions"
	}

	if u, err := url.Parse(base); err == nil && u.Path != "" {
		path := strings.TrimRight(u.Path, "/")
		if strings.HasSuffix(path, "/chat/completions") {
			u.Path = strings.TrimSuffix(path, "/chat/completions")
			if u.Path == "" {
				u.Path = "/"
			}
			return strings.TrimRight(u.String(), "/"), "/chat/completions"
		}
	}

	if strings.HasSuffix(base, "/chat/completions") {
		return strings.TrimSuffix(base, "/chat/completions"), "/chat/completions"
	}
	return base, "/chat/completions"
}

func shouldHandleToolCalls(finishReason string) bool {
	switch strings.ToLower(strings.TrimSpace(finishReason)) {
	case "tool_calls", "tool_call", "function_call", "function_calls":
		return true
	default:
		return false
	}
}

func AskAi(o *OpenAi, _ error, messages []map[string]interface{}, ch chan map[string]any, question string, think bool) {
	if o.TimeOut <= 0 {
		o.TimeOut = 300
	}
	if strings.TrimSpace(o.BaseUrl) == "" {
		ch <- map[string]any{"code": 0, "question": question, "content": "AI 配置错误：Base URL 为空，请先在设置中配置 AI 服务地址"}
		return
	}
	if strings.TrimSpace(o.ApiKey) == "" {
		ch <- map[string]any{"code": 0, "question": question, "content": "AI 配置错误：API Key 为空，请先在设置中配置 API Key"}
		return
	}
	timeout := time.Duration(o.TimeOut) * time.Second

	var client *resty.Client
	if o.HttpProxyEnabled && o.HttpProxy != "" {
		client = createHTTPClientWithProxy(o.HttpProxy, o.TimeOut)
	} else {
		client = CreateHTTPClientWithTimeout(timeout)
	}

	baseURL, chatPath := openAIChatEndpoint(o.BaseUrl)
	client.SetBaseURL(baseURL)

	thinking := "disabled"
	if think {
		thinking = "enabled"
	}

	if !think {
		messages = stripReasoningContent(messages)
	}

	bodyMap := map[string]interface{}{
		"model":    o.Model,
		"stream":   true,
		"messages": messages,
	}
	if o.Temperature > 0 {
		bodyMap["temperature"] = o.Temperature
	}
	if o.MaxTokens > 0 {
		bodyMap["max_tokens"] = o.MaxTokens
	}
	if think {
		bodyMap["thinking"] = map[string]any{
			"type": thinking,
		}
	}

	req := client.R().
		SetDoNotParseResponse(true).
		SetHeader("Authorization", "Bearer "+o.ApiKey).
		SetHeader("Content-Type", "application/json").
		SetBody(bodyMap)
	if o.ctx != nil {
		req = req.SetContext(o.ctx)
	}
	resp, err := req.Post(chatPath)

	if err != nil {
		logger.SugaredLogger.Errorf("Stream error: %s", err.Error())
		ch <- map[string]any{
			"code":     0,
			"question": question,
			"content":  err.Error(),
		}
		return
	}

	body := resp.RawBody()
	defer body.Close()

	if resp.StatusCode() != 200 {
		bodyBytes, _ := io.ReadAll(body)
		errMsg := openAIHTTPErrorMessage(resp.StatusCode(), bodyBytes)
		logger.SugaredLogger.Errorf("Stream HTTP error %d: %s", resp.StatusCode(), errMsg)
		if think && isUnsupportedThinkingError(errMsg) {
			logger.SugaredLogger.Warnf("Thinking is not supported by model %s, retrying without thinking", o.Model)
			AskAi(o, err, stripReasoningContent(messages), ch, question, false)
			return
		}
		if hasReasoningContent(messages) && isUnsupportedReasoningContentError(errMsg) {
			logger.SugaredLogger.Warnf("reasoning_content is not supported by model %s, retrying without reasoning_content", o.Model)
			AskAi(o, err, stripReasoningContent(messages), ch, question, think)
			return
		}
		if resp.StatusCode() == 400 && think {
			logger.SugaredLogger.Warnf("HTTP 400 with model %s, retrying without thinking", o.Model)
			AskAi(o, err, stripReasoningContent(messages), ch, question, false)
			return
		}
		ch <- map[string]any{
			"code":     0,
			"question": question,
			"content":  errMsg,
		}
		return
	}

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			data := strutil.Trim(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				return
			}
			if data == "" {
				continue
			}

			var streamResponse struct {
				Id      string `json:"id"`
				Model   string `json:"model"`
				Choices []struct {
					Delta struct {
						Content          string `json:"content"`
						ReasoningContent string `json:"reasoning_content"`
					} `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
			}

			if err := json.Unmarshal([]byte(data), &streamResponse); err == nil {
				for _, choice := range streamResponse.Choices {
					if content := choice.Delta.Content; content != "" {
						// 质量校验：丢弃 <unk> 乱码 chunk，避免垃圾内容流到前端。
						if IsDegenerateGarbage(content) {
							logger.SugaredLogger.Warnf("AskAi dropped degenerate <unk> chunk (model=%s)", streamResponse.Model)
							continue
						}
						if content == "###" || content == "##" || content == "#" {
							ch <- map[string]any{
								"code":     1,
								"question": question,
								"chatId":   streamResponse.Id,
								"model":    streamResponse.Model,
								"content":  "\r\n" + content,
								"time":     time.Now().Format(time.DateTime),
							}
						} else {
							ch <- map[string]any{
								"code":     1,
								"question": question,
								"chatId":   streamResponse.Id,
								"model":    streamResponse.Model,
								"content":  content,
								"time":     time.Now().Format(time.DateTime),
							}
						}
					}
					if reasoningContent := choice.Delta.ReasoningContent; reasoningContent != "" {
						ch <- map[string]any{
							"code":              1,
							"question":          question,
							"chatId":            streamResponse.Id,
							"model":             streamResponse.Model,
							"reasoning_content": reasoningContent,
							"time":              time.Now().Format(time.DateTime),
						}
					}
					if choice.FinishReason == "stop" {
						return
					}
				}
			} else {
				logger.SugaredLogger.Warnf("Stream data parse error: %s", err.Error())
				ch <- map[string]any{
					"code":     0,
					"question": question,
					"content":  err.Error(),
				}
			}
		} else {
			if strutil.RemoveNonPrintable(line) != "" {
				logger.SugaredLogger.Warnf("Stream line error: %s", line)
				res := &models.Resp{}
				if err := json.Unmarshal([]byte(line), res); err == nil {
					msg := res.Message
					if res.Error.Message != "" {
						msg = res.Error.Message
					}
					if think && isUnsupportedThinkingError(msg) {
						logger.SugaredLogger.Warnf("Thinking is not supported by model %s, retrying without thinking", o.Model)
						AskAi(o, err, stripReasoningContent(messages), ch, question, false)
						return
					}
					if hasReasoningContent(messages) && isUnsupportedReasoningContentError(msg) {
						logger.SugaredLogger.Warnf("reasoning_content is not supported by model %s, retrying without reasoning_content", o.Model)
						AskAi(o, err, stripReasoningContent(messages), ch, question, think)
						return
					}
					ch <- map[string]any{
						"code":     0,
						"question": question,
						"content":  msg,
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		logger.SugaredLogger.Errorf("Stream scanner error: %s", err.Error())
		ch <- map[string]any{
			"code":     0,
			"question": question,
			"content":  err.Error(),
		}
	}
}

func AskAiWithTools(o *OpenAi, err error, messages []map[string]interface{}, ch chan map[string]any, question string, tools []Tool, thinkingMode bool) {
	if len(tools) == 0 {
		AskAi(o, err, messages, ch, question, thinkingMode)
		return
	}
	AskAiWithToolsDepth(o, err, messages, ch, question, tools, thinkingMode, 0)
}

func AskAiWithToolsDepth(o *OpenAi, err error, messages []map[string]interface{}, ch chan map[string]any, question string, tools []Tool, thinkingMode bool, depth int) {
	const maxDepth = 200
	if depth > maxDepth {
		logger.SugaredLogger.Warnf("AskAiWithTools max depth exceeded: %d", depth)
		ch <- map[string]any{
			"code":     0,
			"question": question,
			"content":  "工具调用次数过多，已终止",
		}
		return
	}

	if strings.TrimSpace(o.BaseUrl) == "" {
		ch <- map[string]any{"code": 0, "question": question, "content": "AI 配置错误：Base URL 为空，请先在设置中配置 AI 服务地址"}
		return
	}
	if strings.TrimSpace(o.ApiKey) == "" {
		ch <- map[string]any{"code": 0, "question": question, "content": "AI 配置错误：API Key 为空，请先在设置中配置 API Key"}
		return
	}
	if o.TimeOut <= 0 {
		o.TimeOut = 300
	}
	timeout := time.Duration(o.TimeOut) * time.Second

	var client *resty.Client
	if o.HttpProxyEnabled && o.HttpProxy != "" {
		client = createHTTPClientWithProxy(o.HttpProxy, o.TimeOut)
	} else {
		client = CreateHTTPClientWithTimeout(timeout)
	}

	baseURL, chatPath := openAIChatEndpoint(o.BaseUrl)
	client.SetBaseURL(baseURL)

	if !thinkingMode {
		messages = stripReasoningContent(messages)
	}

	bodyMap := map[string]interface{}{
		"model":    o.Model,
		"stream":   true,
		"messages": messages,
		"tools":    tools,
	}
	if o.Temperature > 0 {
		bodyMap["temperature"] = o.Temperature
	}
	if o.MaxTokens > 0 {
		bodyMap["max_tokens"] = o.MaxTokens
	}
	if thinkingMode {
		bodyMap["thinking"] = map[string]any{
			"type": "enabled",
		}
	}

	reqBody, _ := json.Marshal(bodyMap)
	if len(reqBody) > 100000 {
		logger.SugaredLogger.Warnf("Request body too large: %d bytes, may cause API error", len(reqBody))
	}

	req := client.R().
		SetDoNotParseResponse(true).
		SetHeader("Authorization", "Bearer "+o.ApiKey).
		SetHeader("Content-Type", "application/json").
		SetBody(bodyMap)
	if o.ctx != nil {
		req = req.SetContext(o.ctx)
	}
	resp, err := req.Post(chatPath)

	if err != nil {
		logger.SugaredLogger.Errorf("Stream error: %s", err.Error())
		ch <- map[string]any{
			"code":     0,
			"question": question,
			"content":  err.Error(),
		}
		return
	}

	body := resp.RawBody()
	defer body.Close()

	if resp.StatusCode() != 200 {
		bodyBytes, _ := io.ReadAll(body)
		errMsg := openAIHTTPErrorMessage(resp.StatusCode(), bodyBytes)
		logger.SugaredLogger.Errorf("Stream HTTP error %d: %s", resp.StatusCode(), errMsg)

		if thinkingMode && isUnsupportedThinkingError(errMsg) {
			logger.SugaredLogger.Warnf("Thinking is not supported by model %s, retrying tools request without thinking", o.Model)
			AskAiWithToolsDepth(o, err, stripReasoningContent(messages), ch, question, tools, false, depth)
			return
		}
		if hasReasoningContent(messages) && isUnsupportedReasoningContentError(errMsg) {
			logger.SugaredLogger.Warnf("reasoning_content is not supported by model %s, retrying tools request without reasoning_content", o.Model)
			AskAiWithToolsDepth(o, err, stripReasoningContent(messages), ch, question, tools, thinkingMode, depth)
			return
		}
		if isUnsupportedToolError(errMsg) {
			logger.SugaredLogger.Warnf("Tools are not supported by model %s, retrying without tools", o.Model)
			AskAi(o, err, stripToolMessages(messages), ch, question, thinkingMode)
			return
		}

		if resp.StatusCode() == 400 {
			if thinkingMode {
				logger.SugaredLogger.Warnf("HTTP 400 with model %s, retrying without thinking (depth=%d)", o.Model, depth)
				AskAiWithToolsDepth(o, err, stripReasoningContent(messages), ch, question, tools, false, depth)
				return
			}
			if len(tools) > 0 {
				logger.SugaredLogger.Warnf("HTTP 400 with model %s, retrying without tools (depth=%d)", o.Model, depth)
				AskAi(o, err, stripToolMessages(messages), ch, question, false)
				return
			}
		}

		ch <- map[string]any{
			"code":     0,
			"question": question,
			"content":  errMsg,
		}
		return
	}

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
	type pendingToolCall struct {
		ID        string
		Name      string
		Arguments strings.Builder
	}
	toolCalls := map[int]*pendingToolCall{}
	toolCallOrder := make([]int, 0)
	var currentAIContent strings.Builder
	var reasoningContentText strings.Builder
	var contentText strings.Builder
	handledToolCalls := false
	lastStreamResponseID := ""
	lastModel := ""

	handlePendingToolCalls := func(streamResponseID, model string) bool {
		if handledToolCalls || len(toolCallOrder) == 0 {
			return false
		}
		if streamResponseID == "" {
			streamResponseID = lastStreamResponseID
		}
		if model == "" {
			model = lastModel
		}
		handledToolCalls = true
		for _, idx := range toolCallOrder {
			call := toolCalls[idx]
			if call == nil || call.Name == "" {
				continue
			}
			if call.ID == "" {
				call.ID = fmt.Sprintf("call_%d", idx)
			}
			funcArguments := call.Arguments.String()
			if handler, ok := toolHandlers[call.Name]; ok {
				if hErr := handler(o, funcArguments, &ToolContext{
					Question:             question,
					Messages:             &messages,
					CurrentAIContent:     &currentAIContent,
					ReasoningContentText: &reasoningContentText,
					CurrentCallID:        call.ID,
					FuncName:             call.Name,
					Ch:                   ch,
					StreamResponseID:     streamResponseID,
					Model:                model,
					Source:               o.ChatSource,
				}); hErr != nil {
					logger.SugaredLogger.Errorf("tool %s error: %s", call.Name, hErr.Error())
					ch <- map[string]any{
						"code":     0,
						"question": question,
						"content":  hErr.Error(),
					}
				}
				continue
			}
			logger.SugaredLogger.Warnf("tool handler not found: %s", call.Name)
			appendToolMessages(
				&messages,
				currentAIContent.String(),
				reasoningContentText.String(),
				call.ID,
				call.Name,
				funcArguments,
				fmt.Sprintf("工具 %s 不存在或未注册", call.Name),
			)
		}
		AskAiWithToolsDepth(o, err, messages, ch, question, tools, thinkingMode, depth+1)
		return true
	}

	for scanner.Scan() {
		line := scanner.Text()
		//logger.SugaredLogger.Infof("Received data: %s", line)
		if strings.HasPrefix(line, "data:") {
			data := strutil.Trim(strings.TrimPrefix(line, "data:"))
			if data == "[DONE]" {
				if handlePendingToolCalls("", "") {
					return
				}
				return
			}
			if data == "" {
				continue
			}

			var streamResponse struct {
				Id      string `json:"id"`
				Model   string `json:"model"`
				Choices []struct {
					Delta struct {
						Content          string `json:"content"`
						ReasoningContent string `json:"reasoning_content"`
						Role             string `json:"role"`
						ToolCalls        []struct {
							Function struct {
								Arguments string `json:"arguments"`
								Name      string `json:"name"`
							} `json:"function"`
							Id    string `json:"id"`
							Index int    `json:"index"`
							Type  string `json:"type"`
						} `json:"tool_calls"`
					} `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
			}

			if err := json.Unmarshal([]byte(data), &streamResponse); err == nil {
				if streamResponse.Id != "" {
					lastStreamResponseID = streamResponse.Id
				}
				if streamResponse.Model != "" {
					lastModel = streamResponse.Model
				}
				for _, choice := range streamResponse.Choices {
					if content := choice.Delta.Content; content != "" {
						// 质量校验：丢弃 <unk> 乱码 chunk，避免垃圾内容流到前端。
						if IsDegenerateGarbage(content) {
							logger.SugaredLogger.Warnf("AskAiWithTools dropped degenerate <unk> chunk (model=%s)", streamResponse.Model)
							continue
						}
						contentText.WriteString(content)

						if content == "###" || content == "##" || content == "#" {
							currentAIContent.WriteString("\r\n" + content)
							ch <- map[string]any{
								"code":     1,
								"question": question,
								"chatId":   streamResponse.Id,
								"model":    streamResponse.Model,
								"content":  "\r\n" + content,
								"time":     time.Now().Format(time.DateTime),
							}
						} else {
							currentAIContent.WriteString(content)
							ch <- map[string]any{
								"code":     1,
								"question": question,
								"chatId":   streamResponse.Id,
								"model":    streamResponse.Model,
								"content":  content,
								"time":     time.Now().Format(time.DateTime),
							}
						}
					}
					if reasoningContent := choice.Delta.ReasoningContent; reasoningContent != "" {
						reasoningContentText.WriteString(reasoningContent)
						ch <- map[string]any{
							"code":              1,
							"question":          question,
							"chatId":            streamResponse.Id,
							"model":             streamResponse.Model,
							"reasoning_content": reasoningContent,
							"time":              time.Now().Format(time.DateTime),
						}
					}
					if len(choice.Delta.ToolCalls) > 0 {
						for _, call := range choice.Delta.ToolCalls {
							idx := call.Index
							tc, ok := toolCalls[idx]
							if !ok {
								tc = &pendingToolCall{}
								toolCalls[idx] = tc
								toolCallOrder = append(toolCallOrder, idx)
							}
							if call.Function.Name != "" {
								tc.Name = call.Function.Name
							}
							if call.Id != "" {
								tc.ID = call.Id
							}
							if call.Function.Arguments != "" {
								tc.Arguments.WriteString(call.Function.Arguments)
							}
						}
					}

					if shouldHandleToolCalls(choice.FinishReason) {
						handlePendingToolCalls(streamResponse.Id, streamResponse.Model)
						return
					}

					if choice.FinishReason == "stop" {
						return
					}
				}
			} else {
				logger.SugaredLogger.Warnf("Stream data parse error: %s", err.Error())
				ch <- map[string]any{
					"code":     0,
					"question": question,
					"content":  err.Error(),
				}
			}
		} else {
			if strutil.RemoveNonPrintable(line) != "" {
				logger.SugaredLogger.Warnf("Stream line error: %s", line)
				res := &models.Resp{}
				if err := json.Unmarshal([]byte(line), res); err == nil {
					msg := res.Message
					if res.Error.Message != "" {
						msg = res.Error.Message
					}

					if thinkingMode && isUnsupportedThinkingError(msg) {
						logger.SugaredLogger.Warnf("Thinking is not supported by model %s, retrying tools request without thinking", o.Model)
						AskAiWithToolsDepth(o, err, stripReasoningContent(messages), ch, question, tools, false, depth)
					} else if hasReasoningContent(messages) && isUnsupportedReasoningContentError(msg) {
						logger.SugaredLogger.Warnf("reasoning_content is not supported by model %s, retrying tools request without reasoning_content", o.Model)
						AskAiWithToolsDepth(o, err, stripReasoningContent(messages), ch, question, tools, thinkingMode, depth)
					} else if isUnsupportedToolError(msg) {
						logger.SugaredLogger.Warnf("Tools are not supported by model %s, retrying without tools", o.Model)
						AskAi(o, err, stripToolMessages(messages), ch, question, thinkingMode)
					} else {
						ch <- map[string]any{
							"code":     0,
							"question": question,
							"content":  msg,
						}
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		logger.SugaredLogger.Errorf("Stream scanner error: %s", err.Error())
		ch <- map[string]any{
			"code":     0,
			"question": question,
			"content":  err.Error(),
		}
	}
}

// deduplicateCheck 同 stock_code + 同 question 近 4h 内已生成则返回 true（去重/跳过）
func deduplicateCheck(stockCode, question string) bool {
	if question == "" {
		return false
	}
	var count int64
	fourHoursAgo := time.Now().Add(-4 * time.Hour)
	err := db.Dao.Model(&models.AIResponseResult{}).
		Where("stock_code = ? AND question = ? AND created_at > ? AND is_del = 0", stockCode, question, fourHoursAgo).
		Count(&count).Error
	if err != nil {
		logger.SugaredLogger.Errorf("deduplicateCheck query error: %v", err)
		return false
	}
	return count > 0
}

func (o *OpenAi) SaveAIResponseResult(stockCode, stockName, result, chatId, question string) {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("panic in SaveAIResponseResult: %v", r)
		}
	}()

	// D. 去重：同 stock_code + 同 question 近 4h 内已生成则跳过保存
	if deduplicateCheck(stockCode, question) {
		logger.SugaredLogger.Infof("SaveAIResponseResult: 重复生成，跳过保存 code=%q question=%q", stockCode, question)
		return
	}

	// C. 事后硬校验（仅对泛化报告）：抽取代码校验存在性/行情，标注市值未核验
	corrected := verifyAndCorrectContent(result, stockCode, question)
	if corrected != result {
		logger.SugaredLogger.Infof("SaveAIResponseResult: 内容经校验修正 code=%q", stockCode)
		result = corrected
	}

	// 非个股代码（"市场分析"/"agent"/空串等）不会进入回测准确度追踪，
	// 提前打点便于发现是否误把泛化分析当成个股报告保存。
	if !models.IsStockReportCode(stockCode) {
		logger.SugaredLogger.Warnf("SaveAIResponseResult: 非个股代码，不计入回测追踪: code=%q name=%q question=%q", stockCode, stockName, question)
	}
	reportRecord := models.AIResponseResult{
		StockCode: stockCode,
		StockName: stockName,
		ModelName: o.Model,
		Content:   result,
		ChatId:    chatId,
		Question:  question,
	}
	err := db.Dao.Create(&reportRecord).Error
	if err != nil {
		logger.SugaredLogger.Errorf("failed to save ai response result: %v", err)
	} else if models.IsStockReportCode(stockCode) {
		// 异步沉淀买方认知假说与历史演进轨迹
		go func(code, name, content, model string, repId uint) {
			defer func() {
				if r := recover(); r != nil {
					logger.SugaredLogger.Errorf("recover in ExtractAndUpdateThesisFromReport: %v", r)
				}
			}()
			NewStockThesisApi().ExtractAndUpdateThesisFromReport(code, name, content, model, repId)
		}(stockCode, stockName, result, o.Model, reportRecord.ID)
	}
}

func (o *OpenAi) GetAIResponseResult(stock string) *models.AIResponseResult {
	var result models.AIResponseResult
	db.Dao.Where("stock_code = ?", stock).Order("id desc").Limit(1).Find(&result)
	return &result
}

func createHTTPClientWithProxy(proxyURL string, timeout int) *resty.Client {
	transport := &http.Transport{
		Proxy: http.ProxyURL(parseProxyURL(proxyURL)),
	}

	httpClient := &http.Client{
		Transport: transport,
	}

	if timeout <= 0 {
		timeout = 300
	}
	httpClient.Timeout = time.Duration(timeout) * time.Second

	return resty.NewWithClient(httpClient).
		SetTimeout(time.Duration(timeout) * time.Second).
		SetRetryCount(0)
}
