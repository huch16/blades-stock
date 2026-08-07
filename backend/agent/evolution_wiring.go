package agent

import (
	"sort"
	"strings"
	"sync"
	"time"

	"blades-stock/backend/db"
	"blades-stock/backend/logger"

	bladesTools "github.com/go-kratos/blades/tools"
)

// ---------------------------------------------------------------------------
// 进化闭环接线：工具重排
// RecordToolUse 写出的成功率/延迟数据，通过这里回灌到工具注入顺序：
// 实测越可靠的工具越靠前，高失败率的工具沉底。这是"进化 → 行为"的
// 直接一环，不再只是只写不读的统计表。
// ---------------------------------------------------------------------------

// GetToolRanksMap returns the latest tool rank rows keyed by tool name.
func (ev *Evolver) GetToolRanksMap() map[string]ToolRank {
	ranks := ev.RankTools()
	if len(ranks) == 0 {
		return nil
	}
	m := make(map[string]ToolRank, len(ranks))
	for _, r := range ranks {
		m[r.ToolName] = r
	}
	return m
}

// successRate returns the empirical success rate of a tool rank.
func (r ToolRank) successRate() float64 {
	total := r.SuccessCount + r.FailCount
	if total <= 0 {
		return 0
	}
	return float64(r.SuccessCount) / float64(total)
}

var (
	toolRankCacheMu sync.Mutex
	toolRankCache   map[string]ToolRank
	toolRankCacheAt time.Time
)

const toolRankCacheTTL = 60 * time.Second

// getToolRanksCache reads agent_tool_ranks with a short TTL so a per-question
// tool selection does not hit the DB every time.
func getToolRanksCache() map[string]ToolRank {
	toolRankCacheMu.Lock()
	defer toolRankCacheMu.Unlock()
	if toolRankCache != nil && time.Since(toolRankCacheAt) < toolRankCacheTTL {
		return toolRankCache
	}
	toolRankCache = DefaultEvolver().GetToolRanksMap()
	toolRankCacheAt = time.Now()
	return toolRankCache
}

// reorderToolsByRank sorts tools so empirically more reliable ones come first,
// feeding recorded tool usage data back into the injection order. Tools without
// any recorded usage keep their original relative order and are placed after
// tools that have been used. A nil/empty rank table is a no-op.
func reorderToolsByRank(ts []bladesTools.Tool) []bladesTools.Tool {
	return reorderToolsByRankWith(ts, getToolRanksCache())
}

// reorderToolsByRankWith is the pure sorting core, separated from the DB cache
// so it can be unit-tested deterministically.
func reorderToolsByRankWith(ts []bladesTools.Tool, ranks map[string]ToolRank) []bladesTools.Tool {
	if len(ts) < 2 {
		return ts
	}
	if len(ranks) == 0 {
		return ts
	}
	rankOf := func(name string) (ToolRank, bool) {
		r, ok := ranks[name]
		return r, ok
	}
	out := make([]bladesTools.Tool, len(ts))
	copy(out, ts)
	sort.SliceStable(out, func(i, j int) bool {
		ri, oki := rankOf(out[i].Name())
		rj, okj := rankOf(out[j].Name())
		switch {
		case oki && okj:
			if ri.successRate() != rj.successRate() {
				return ri.successRate() > rj.successRate()
			}
			ti, tj := ri.SuccessCount+ri.FailCount, rj.SuccessCount+rj.FailCount
			if ti != tj {
				return ti > tj
			}
			if ri.AvgDurationMs != rj.AvgDurationMs {
				return ri.AvgDurationMs < rj.AvgDurationMs
			}
			return ri.ToolName < rj.ToolName
		case oki:
			return true
		case okj:
			return false
		default:
			return false
		}
	})
	return out
}

// ---------------------------------------------------------------------------
// 进化闭环接线：OptimizePrompt
// 低质量轨迹（score<0）触发异步 Prompt 优化建议，结果写入长期记忆
// （category="evolution"），随 MemoryPrompt 注入后续会话。按会话限频，
// 6 小时内至多生成一条，避免每次低分都付一次 LLM 成本。
// ---------------------------------------------------------------------------

const evolutionAdviceCooldown = 6 * time.Hour

func (ev *Evolver) maybeGenerateEvolutionAdvice(sessionID string, trace *AgentTrace) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.SugaredLogger.Errorf("panic in maybeGenerateEvolutionAdvice: %v", r)
			}
		}()
		if sessionID == "" || trace == nil {
			return
		}
		var recent int64
		db.Dao.Model(&AgentMemory{}).
			Where("session_id = ? AND category = ? AND created_at >= ?", sessionID, "evolution", time.Now().Add(-evolutionAdviceCooldown)).
			Count(&recent)
		if recent > 0 {
			return
		}
		llm := LLMFromConfig()
		if llm == nil {
			return
		}
		advice, err := ev.OptimizePrompt(trace, llm)
		if err != nil {
			logger.SugaredLogger.Debugf("evolution advice skipped: %v", err)
			return
		}
		mem := &AgentMemory{
			SessionID: sessionID,
			Role:      "evolution",
			Content:   truncateText(advice, 800),
			Category:  "evolution",
			CreatedAt: time.Now(),
		}
		if err := db.Dao.Create(mem).Error; err != nil {
			logger.SugaredLogger.Warnf("store evolution advice: %v", err)
			return
		}
		logger.SugaredLogger.Infof("stored evolution prompt advice for session %s", sessionID)
	}()
}

// ---------------------------------------------------------------------------
// 进化闭环接线：GenerateSkillFromTraces
// 从高质量轨迹批量提取可复用技能草稿。只记录草稿供人工审阅，绝不自动建
// 技能。7 天限频，可在应用 cron 中每天调度执行。
// ---------------------------------------------------------------------------

const skillPassCooldown = 7 * 24 * time.Hour

// RunSkillExtractionPass extracts a skill draft from recent high-quality
// traces and logs it. Guarded by a 7-day throttle persisted in agent_memories
// so a daily cron cannot spam the LLM.
func RunSkillExtractionPass() {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("panic in RunSkillExtractionPass: %v", r)
		}
	}()
	var recent int64
	db.Dao.Model(&AgentMemory{}).
		Where("category = ? AND role = ? AND created_at >= ?", "evolution", "skill_pass", time.Now().Add(-skillPassCooldown)).
		Count(&recent)
	if recent > 0 {
		return
	}
	llm := LLMFromConfig()
	if llm == nil {
		logger.SugaredLogger.Debug("RunSkillExtractionPass skipped: no AI config")
		return
	}
	draft, err := DefaultEvolver().GenerateSkillFromTraces(llm)
	if err != nil {
		logger.SugaredLogger.Warnf("skill extraction pass failed: %v", err)
		return
	}
	logger.SugaredLogger.Infof("skill extraction draft generated:\n%s", draft)
	_ = db.Dao.Create(&AgentMemory{
		SessionID: "__system__",
		Role:      "skill_pass",
		Content:   truncateText(draft, 2000),
		Category:  "evolution",
		CreatedAt: time.Now(),
	}).Error
}

// toolCallSucceeded infers tool-call success from the raw tool response text.
// It feeds failure information into RecordToolUse so agent_tool_ranks reflects
// reality instead of recording everything as a success.
func toolCallSucceeded(resp string) bool {
	if strings.TrimSpace(resp) == "" {
		return false
	}
	lower := strings.ToLower(resp)
	for _, bad := range []string{"error", "failed", "timeout", "失败", "错误", "异常", "无法", "暂无"} {
		if strings.Contains(lower, bad) {
			return false
		}
	}
	return true
}

// shouldRunLLMReflection bounds the LLM reflection cost: only substantive
// answers, negative-scored answers (need a second opinion), and a deterministic
// ~1/3 sample of medium answers get an LLM call. Everything else falls back to
// the cheap keyword scorer.
func shouldRunLLMReflection(answer string, autoScore int) bool {
	n := len([]rune(answer))
	if n < 150 {
		return false
	}
	if autoScore < 0 {
		return true
	}
	if n >= 500 {
		return true
	}
	return sampleHash(answer) == 0
}

// sampleHash returns a stable 0..2 hash bucket for a string, used to sample
// without randomness so the same answer is always sampled the same way.
func sampleHash(s string) int {
	var h uint32 = 2166136261
	for i := 0; i < len(s) && i < 256; i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return int(h % 3)
}
