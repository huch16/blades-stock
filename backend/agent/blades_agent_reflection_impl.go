package agent

import (
	"encoding/json"
	"fmt"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"strings"
	"sync"
	"time"
)

type ReflectionResult struct {
	ReflectionID uint   `json:"reflectionId"`
	SessionID    string `json:"sessionId"`
	Question     string `json:"question"`
	Score        int    `json:"score"`
	Insight      string `json:"insight"`
	Improvement  string `json:"improvement"`
	CreatedAt    string `json:"createdAt"`
}

var defaultEvaluator *Evaluator
var evaluatorOnce sync.Once

func DefaultEvaluator() *Evaluator {
	evaluatorOnce.Do(func() {
		defaultEvaluator = NewEvaluator()
	})
	return defaultEvaluator
}

type Evaluator struct{}

func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

func (e *Evaluator) ScoreResult(result string) int {
	if result == "" {
		return 0
	}
	positive := []string{
		"成功", "完成", "获取到", "查到了", "如下",
		"数据", "结果", "输出", "返回", "response",
	}
	negative := []string{
		"失败", "错误", "异常", "无法", "超时",
		"error", "failed", "timeout", "nil", "空",
	}
	lower := strings.ToLower(result)
	score := 0
	for _, kw := range positive {
		if strings.Contains(lower, strings.ToLower(kw)) {
			score++
		}
	}
	for _, kw := range negative {
		if strings.Contains(lower, strings.ToLower(kw)) {
			score--
		}
	}
	if score > 0 {
		return 1
	}
	if score < 0 {
		return -1
	}
	return 0
}

type LLMReflectionFunc func(systemPrompt, userMessage string) (string, error)

func (e *Evaluator) GenerateReflection(sessionID, question, answer string, durationMs int64, llm LLMReflectionFunc, preScore ...int) (*ReflectionResult, error) {
	autoScore := 0
	if len(preScore) > 0 {
		autoScore = preScore[0]
	} else {
		autoScore = e.ScoreResult(answer)
	}
	reflectionPrompt := fmt.Sprintf(`你是一个智能体反思分析器。请分析以下智能体的执行轨迹并给出反思。

## 原始问题
%s

## 智能体的回答
%s

## 请输出 JSON 格式（不要包含其他内容）：
{
  "score": %d,
  "insight": "对该回答质量的一句洞察",
  "improvement": "如果分数低，给出具体改进建议；如果分数高，指出可复用的经验"
}`, question, answer, autoScore)

	var insight, improvement string
	if llm != nil {
		raw, err := llm("你是一个严格的智能体反思分析器，以 JSON 格式输出。", reflectionPrompt)
		if err == nil {
			var parsed struct {
				Score       int    `json:"score"`
				Insight     string `json:"insight"`
				Improvement string `json:"improvement"`
			}
			raw = extractJSON(raw)
			if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
				autoScore = parsed.Score
				insight = parsed.Insight
				improvement = parsed.Improvement
			}
		}
	}
	if insight == "" {
		insight = e.defaultInsight(autoScore)
	}
	if improvement == "" {
		improvement = e.defaultImprovement(autoScore)
	}

	trace := &AgentTrace{
		SessionID:  sessionID,
		Question:   question,
		Answer:     truncateText(answer, 2000),
		Score:      autoScore,
		Feedback:   fmt.Sprintf("insight: %s | improvement: %s", insight, improvement),
		DurationMs: durationMs,
		CreatedAt:  time.Now(),
	}
	if err := db.Dao.Create(trace).Error; err != nil {
		logger.SugaredLogger.Errorf("save agent trace: %v", err)
	}

	return &ReflectionResult{
		SessionID:   sessionID,
		Question:    question,
		Score:       autoScore,
		Insight:     insight,
		Improvement: improvement,
		CreatedAt:   time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

func (e *Evaluator) RecordFeedback(traceID uint, score int, feedback string) error {
	return db.Dao.Model(&AgentTrace{}).Where("id = ?", traceID).Updates(map[string]interface{}{
		"score":    score,
		"feedback": feedback,
	}).Error
}

func (e *Evaluator) GetHighQualityTraces(limit int) []AgentTrace {
	if limit <= 0 {
		limit = 10
	}
	var traces []AgentTrace
	db.Dao.Where("score > 0").Order("score desc, id desc").Limit(limit).Find(&traces)
	return traces
}

func (e *Evaluator) GetLowQualityTraces(limit int) []AgentTrace {
	if limit <= 0 {
		limit = 10
	}
	var traces []AgentTrace
	db.Dao.Where("score < 0").Order("score asc, id desc").Limit(limit).Find(&traces)
	return traces
}

func (e *Evaluator) defaultInsight(score int) string {
	if score > 0 {
		return "智能体成功完成了任务，结果可用"
	}
	if score < 0 {
		return "智能体执行出现问题，需要关注"
	}
	return "智能体执行结果中性，无明显问题也无亮点"
}

func (e *Evaluator) defaultImprovement(score int) string {
	if score > 0 {
		return "保持当前策略，可考虑优化响应速度"
	}
	if score < 0 {
		return "检查工具调用参数和返回结果处理逻辑"
	}
	return "无明显改进方向，建议观察后续表现"
}

func extractJSON(raw string) string {
	start := strings.Index(raw, "{")
	if start == -1 {
		return raw
	}
	end := strings.LastIndex(raw, "}")
	if end == -1 || end < start {
		return raw
	}
	return raw[start : end+1]
}
