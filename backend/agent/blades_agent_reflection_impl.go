package agent

import (
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type ReflectionResult struct {
	ReflectionID uint   `json:"reflectionId"`
	SessionID    string `json:"sessionId"`
	Question     string `json:"question"`
	Score        int    `json:"score"`
	Accuracy     int    `json:"accuracy"`
	Relevance    int    `json:"relevance"`
	ToolUse      int    `json:"toolUse"`
	Completeness int    `json:"completeness"`
	Insight      string `json:"insight"`
	Improvement  string `json:"improvement"`
	CreatedAt    string `json:"createdAt"`
}

// ReflectionScore carries the four-dimensional quality evaluation of an answer.
type ReflectionScore struct {
	Score        int `json:"score"`        // -1 差 / 0 中性 / 1 好
	Accuracy     int `json:"accuracy"`     // 0-10
	Relevance    int `json:"relevance"`    // 0-10
	ToolUse      int `json:"toolUse"`      // 0-10
	Completeness int `json:"completeness"` // 0-10
}

type Evaluator struct{}

func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

// ScoreResult returns a coarse keyword-based verdict (-1/0/1). It remains the
// deterministic fallback when no LLM reflection is available.
func (e *Evaluator) ScoreResult(result string) int {
	return e.ScoreResultStructured(result).Score
}

// ScoreResultStructured evaluates an answer across four dimensions using
// keyword heuristics. It is cheap, deterministic, and used as the fallback
// whenever LLM reflection fails or is unavailable.
func (e *Evaluator) ScoreResultStructured(result string) ReflectionScore {
	if result == "" {
		return ReflectionScore{}
	}
	positive := []string{
		"成功", "完成", "获取到", "查到了", "如下",
		"数据", "结果", "输出", "返回", "response",
	}
	negative := []string{
		"失败", "错误", "异常", "无法", "超时",
		"error", "failed", "timeout", "nil",
		// 注意：不要用裸 "空"——它会误伤"空间/空头/空白"等正常词。
		// 只认表示"查无结果"的精确短语。
		"为空", "空值", "无数据",
	}
	lower := strings.ToLower(result)
	score := 0
	accuracy := 5
	relevance := 5
	toolUse := 5
	completeness := 5
	for _, kw := range positive {
		if strings.Contains(lower, strings.ToLower(kw)) {
			score++
			completeness++
		}
	}
	for _, kw := range negative {
		if strings.Contains(lower, strings.ToLower(kw)) {
			score--
			accuracy--
		}
	}

	// Data-driven indicators boost accuracy & tool use.
	if strings.Contains(result, "%") || strings.Contains(result, "亿") || strings.Contains(result, "万") {
		accuracy++
		completeness++
	}
	if strings.Contains(result, "截至") || strings.Contains(result, "时间") ||
		strings.Contains(result, "最新") || strings.Contains(result, "今日") {
		toolUse++
		relevance++
	}
	if strings.Contains(result, "建议") || strings.Contains(result, "风险") ||
		strings.Contains(result, "观点") || strings.Contains(result, "结论") {
		completeness++
	}

	clamp := func(v int) int {
		if v < 0 {
			return 0
		}
		if v > 10 {
			return 10
		}
		return v
	}
	accuracy = clamp(accuracy)
	relevance = clamp(relevance)
	toolUse = clamp(toolUse)
	completeness = clamp(completeness)

	composite := 0
	if score > 0 {
		composite = 1
	} else if score < 0 {
		composite = -1
	}
	return ReflectionScore{
		Score:        composite,
		Accuracy:     accuracy,
		Relevance:    relevance,
		ToolUse:      toolUse,
		Completeness: completeness,
	}
}

type LLMReflectionFunc func(systemPrompt, userMessage string) (string, error)

// GenerateReflection 对一次回答做四维度质量评分，写入 AgentTrace 与长期记忆，
// 并在低分时触发 Prompt 进化建议。
//
// 调用点：agent_api.go 的 postProcessResponse（回答收尾的 fire-and-forget
// 步骤 3）。llm == nil 时只走确定性评分（零 Token）；llm != nil 时走 LLM
// 打分，失败则兜底到确定性评分。
//
// 【第五轮已恢复 LLM 打分】按用户批准的条件：约 1/10 采样 + 每日上限约 20 次
// + 最便宜模型，门禁见 reflection_llm_sampling.go，选卡见 reflection_llm.go
// 的 selectReflectionConfig。采样/上限由 postProcessResponse 在调用前判定，
// 本函数只负责“LLM 打分失败 → 确定性兜底”，不做采样决策。
// 另注：低分分支的 maybeGenerateEvolutionAdvice 内部会**真的调 LLM**
// （OptimizePrompt），该分支受 6 小时/会话限频（见 evolution_wiring.go），
// 是独立于打分预算的另一条成本，不要与打分采样混为一谈。
//
// 历史（提交 a158883，strip agent reflection token burn）：
// 唯一调用点曾在 agent_api.go 的回答收尾处被整体删除。那次清理针对的是
// 「反射评估的 LLM 打分会产生 token 消耗」，但被删掉的代码块实际包含
// **两个不同代价**的部分：
//
//  1. LLM 打分（花钱）—— 由 shouldRunLLMReflection 限频后才调，是清理的正当目标。
//  2. 确定性评分 + writeBackToMemory + 写 AgentTrace（**零 token**）——
//     被连带删除。当 llm == nil 时本函数完全自给自足：仍会
//     ScoreResultStructured → defaultInsight/defaultImprovement → 写库。
//
// 第三轮恢复了第 2 部分；第五轮按采样/上限条件恢复了第 1 部分。
func (e *Evaluator) GenerateReflection(sessionID, question, answer string, durationMs int64, llm LLMReflectionFunc, toolsUsed string) (*ReflectionResult, error) {
	fallback := e.ScoreResultStructured(answer)
	autoScore := fallback.Score
	if toolsUsed == "" {
		toolsUsed = "（无工具调用记录）"
	}
	// 反思 prompt 只在真的要调 LLM 时才构造：生产调用传 llm == nil，
	// 每次问答都拼一个 ~2KB 的 prompt 字符串是纯浪费。
	result := ReflectionScore{
		Score:        autoScore,
		Accuracy:     fallback.Accuracy,
		Relevance:    fallback.Relevance,
		ToolUse:      fallback.ToolUse,
		Completeness: fallback.Completeness,
	}
	var insight, improvement string
	if llm != nil {
		// prompt 模板与调用点：采样/上限由调用方（postProcessResponse +
		// reflectionLLMGate）在传 llm 之前判定，本处只负责调用与兜底，
		// 不要在这里再加采样——否则同一回答会被两道门禁各算一次。
		reflectionPrompt := fmt.Sprintf(`你是一个严格的智能体反思评估器。请从四个维度评估以下智能体回答的质量，并输出 JSON。

## 评分维度（每项 0-10 分）
- accuracy: 回答中的数据/事实是否真实可信、有无编造或自相矛盾
- relevance: 回答是否紧扣用户问题，有无答非所问
- toolUse: 是否充分利用了工具返回的数据来支撑结论
- completeness: 是否覆盖问题的关键方面，结论是否完整

## 综合评分（score）
- 1 = 高质量回答（准确、完整、数据充分）
- 0 = 一般（可用但存在明显不足）
- -1 = 低质量（答非所问、数据编造、严重缺失）

## 原始问题
%s

## 智能体本次调用的工具
%s

## 智能体的回答
%s

## 请输出 JSON 格式（不要包含其他内容）：
{
  "score": 0,
  "accuracy": 0,
  "relevance": 0,
  "toolUse": 0,
  "completeness": 0,
  "insight": "对该回答质量的一句洞察",
  "improvement": "如果分数低，给出具体改进建议；如果分数高，指出可复用的经验"
}`, question, toolsUsed, truncateText(answer, 2000))
		raw, err := llm("你是一个严格的智能体反思分析器，以 JSON 格式输出。", reflectionPrompt)
		if err != nil {
			// LLM 打分失败不要静默吞掉：记 Warn 后仍用确定性评分兜底，
			// 否则打分链路全断也无人知道。
			logger.SugaredLogger.Warnf("LLM reflection failed, fallback to keyword scoring: %v", err)
		} else {
			var parsed struct {
				Score        int    `json:"score"`
				Accuracy     int    `json:"accuracy"`
				Relevance    int    `json:"relevance"`
				ToolUse      int    `json:"toolUse"`
				Completeness int    `json:"completeness"`
				Insight      string `json:"insight"`
				Improvement  string `json:"improvement"`
			}
			raw = extractJSONBraces(raw)
			if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
				logger.SugaredLogger.Debugf("LLM reflection response not JSON, fallback to keyword scoring: %v", err)
			} else {
				result.Score = parsed.Score
				result.Accuracy = parsed.Accuracy
				result.Relevance = parsed.Relevance
				result.ToolUse = parsed.ToolUse
				result.Completeness = parsed.Completeness
				insight = parsed.Insight
				improvement = parsed.Improvement
			}
		}
	}
	if insight == "" {
		insight = e.defaultInsight(result.Score)
	}
	if improvement == "" {
		improvement = e.defaultImprovement(result.Score)
	}

	trace := &AgentTrace{
		SessionID:    sessionID,
		Question:     question,
		Answer:       truncateText(answer, 2000),
		ToolsUsed:    toolsUsed,
		Score:        result.Score,
		Accuracy:     result.Accuracy,
		Relevance:    result.Relevance,
		ToolUse:      result.ToolUse,
		Completeness: result.Completeness,
		Feedback:     fmt.Sprintf("insight: %s | improvement: %s", insight, improvement),
		DurationMs:   durationMs,
		CreatedAt:    time.Now(),
	}
	// 轨迹写路径顺带触发过期清理（每小时最多一次），与审计日志同模式。
	maybeCleanupOldAgentTraces()
	if err := db.Dao.Create(trace).Error; err != nil {
		logger.SugaredLogger.Errorf("save agent trace: %v", err)
	}

	// 质量闭环：高分/低分样本回写长期记忆，供后续会话反思参考。
	e.writeBackToMemory(sessionID, question, result, insight, improvement)

	// 进化闭环：低分轨迹异步触发 Prompt 优化建议（限频）。
	if result.Score < 0 {
		DefaultEvolver().maybeGenerateEvolutionAdvice(sessionID, trace)
	}

	return &ReflectionResult{
		SessionID:    sessionID,
		Question:     question,
		Score:        result.Score,
		Accuracy:     result.Accuracy,
		Relevance:    result.Relevance,
		ToolUse:      result.ToolUse,
		Completeness: result.Completeness,
		Insight:      insight,
		Improvement:  improvement,
		CreatedAt:    time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

// writeBackToMemory persists reflection conclusions as long-term memories so
// the agent can learn from past quality issues (category: "reflection"). Before
// writing, it deduplicates: if a reflection about a similar question already
// exists in this session, the old entry is replaced instead of accumulating
// near-identical lessons.
func (e *Evaluator) writeBackToMemory(sessionID, question string, score ReflectionScore, insight, improvement string) {
	if sessionID == "" {
		return
	}
	if score.Score == 0 {
		return
	}
	label := "高质量样本"
	if score.Score < 0 {
		label = "低质量样本"
	}
	content := fmt.Sprintf("[%s] 问题:%s | 综合:%d 准确:%d/10 相关:%d/10 工具:%d/10 完整:%d/10 | 洞察:%s | 改进:%s",
		label, truncateText(question, 120), score.Score,
		score.Accuracy, score.Relevance, score.ToolUse, score.Completeness,
		truncateText(insight, 150), truncateText(improvement, 150))

	// 同类教训合并：同一会话中问题高度相似（字符二元组 Jaccard ≥0.5）的旧反思
	// 删除，只保留最新；完全不同的问题（如不同个股）不受影响。
	key := truncateText(question, 120)
	for _, r := range DefaultMemoryManager().RetrieveReflections(sessionID, 20) {
		existing := extractReflectionQuestion(r.Content)
		if existing != "" && textSimilarity(existing, key) >= 0.5 {
			if err := DefaultMemoryManager().DeleteMemory(r.ID); err != nil {
				logger.SugaredLogger.Warnf("dedup reflection delete failed: %v", err)
			}
			break
		}
	}

	if err := DefaultMemoryManager().StoreReflection(sessionID, content); err != nil {
		logger.SugaredLogger.Warnf("store reflection memory failed: %v", err)
	}
}

// extractReflectionQuestion pulls the question out of a stored reflection
// content line ("[label] 问题:xxx | 综合:..."), used for dedup comparison.
func extractReflectionQuestion(content string) string {
	idx := strings.Index(content, "问题:")
	if idx < 0 {
		return ""
	}
	rest := content[idx+len("问题:"):]
	if end := strings.Index(rest, " | "); end >= 0 {
		return strings.TrimSpace(rest[:end])
	}
	return strings.TrimSpace(rest)
}

// charBigrams splits a string into adjacent character bigrams, used for cheap
// Chinese-friendly text similarity (no tokenizer required).
func charBigrams(s string) []string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) == 0 {
		return nil
	}
	if len(runes) == 1 {
		return []string{string(runes)}
	}
	out := make([]string, 0, len(runes)-1)
	for i := 0; i+1 < len(runes); i++ {
		out = append(out, string(runes[i:i+2]))
	}
	return out
}

// textSimilarity returns the multiset Jaccard overlap of character bigrams
// between two strings (1.0 = identical, 0.0 = no shared bigrams).
func textSimilarity(a, b string) float64 {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == b {
		return 1
	}
	ga, gb := charBigrams(a), charBigrams(b)
	if len(ga) == 0 || len(gb) == 0 {
		return 0
	}
	counts := make(map[string]int, len(ga))
	for _, g := range ga {
		counts[g]++
	}
	inter := 0
	for _, g := range gb {
		if counts[g] > 0 {
			counts[g]--
			inter++
		}
	}
	union := len(ga) + len(gb) - inter
	if union <= 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func (e *Evaluator) GetHighQualityTraces(limit int) []AgentTrace {
	if limit <= 0 {
		limit = 10
	}
	var traces []AgentTrace
	db.Dao.Where("score > 0").Order("score desc, id desc").Limit(limit).Find(&traces)
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

func extractJSONBraces(raw string) string {
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
