package data

import (
	"time"

	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
)

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
