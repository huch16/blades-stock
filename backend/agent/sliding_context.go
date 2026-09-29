// Package agent 的滑动上下文压缩模块（P2.2）。
//
// 设计目标：解决多轮对话（特别是 plan_execute 模式）下 context 无限膨胀的问题。
// 旧实现（compressor）：每轮重算整个历史的压缩版本 — O(N²) token cost。
// 新实现（SlidingContext）：保留最近 N 轮完整 turn + 早期 turn 压成单段摘要 — O(N) growth。
//
// 行为契约：
//   - 保留最近 KeepFullN（默认 3）轮完整 turn
//   - 更早的 turn 被折叠进 OlderSummary 段
//   - OlderSummary 增长有上限（MaxSummaryTokens），超限则再次压缩（保留决策点）
//   - Turn 的 Importance 字段：4-5 的 turn 永不压缩（哪怕是早期）
//   - IsDecision=true 的 turn 永不压缩
//
// 用法：
//   sc := NewSlidingContext(nil)
//   for _, turn := range turns { sc.Add(turn) }
//   prompt := sc.Render()
package agent

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Turn 是对话历史中的一个最小单元。
// 字段重要性：Importance >= 4 或 IsDecision=true 的 turn 永不压缩。
type Turn struct {
	Role       string // "user" | "assistant" | "system"
	Content    string
	Timestamp  time.Time
	Importance int  // 1-5；未设置时默认 3
	IsDecision bool // 包含 "结论/建议/操作" 等决策性内容
}

// SlidingContextConfig 配置 SlidingContext 的关键参数。
type SlidingContextConfig struct {
	// KeepFullN 保留最近多少轮完整 turn。0 或负值 → 默认 3。
	KeepFullN int

	// MaxSummaryTokens 早期 turn 摘要的最大 token 数。0 → 默认 500。
	MaxSummaryTokens int

	// Now 用于测试时注入假时钟。nil → time.Now。
	Now func() time.Time
}

func (c SlidingContextConfig) withDefaults() SlidingContextConfig {
	if c.KeepFullN <= 0 {
		c.KeepFullN = 3
	}
	if c.MaxSummaryTokens <= 0 {
		c.MaxSummaryTokens = 500
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	return c
}

// SlidingContext 是线程安全的滑动上下文窗口。
// 内部状态：
//   - fullTurns：最近 keepFullN 轮完整 turn（FIFO）
//   - olderSummary：早期 turn 折叠后的摘要
//   - locked：摘要段也包含 "lock" 列表，记录决策点永不丢
type SlidingContext struct {
	mu sync.Mutex

	cfg           SlidingContextConfig
	fullTurns     []Turn
	olderSummary  string
	lockedPoints  []string // 决策点（永不压缩）
	totalAdded    int
	compressedOut int // 被压缩掉的 turn 数（用于 metrics）
}

// NewSlidingContext 构造一个默认配置的 SlidingContext。
func NewSlidingContext(cfg *SlidingContextConfig) *SlidingContext {
	c := SlidingContextConfig{}
	if cfg != nil {
		c = *cfg
	}
	return &SlidingContext{
		cfg:          c.withDefaults(),
		fullTurns:    make([]Turn, 0, c.withDefaults().KeepFullN+1),
		lockedPoints: make([]string, 0),
	}
}

// Add 推入一轮对话。
// 行为：
//   - 若 Turn.Importance >= 4 或 IsDecision=true → 加入 fullTurns 但永不压缩（"locked"）
//   - 否则 → 加入 fullTurns；若超过 keepFullN，最早一轮（除 locked 外）被 fold 进 olderSummary
//
// 设计要点：即使 turn 数量超 keepFullN，locked turn 始终保留在 fullTurns 末尾。
// 这是为"用户给出关键决策后让 LLM 能立即看到"设计的。
func (sc *SlidingContext) Add(turn Turn) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.totalAdded++

	// Importance/IsDecision 自动填充默认值
	if turn.Importance == 0 {
		turn.Importance = 3
	}
	if turn.Timestamp.IsZero() {
		turn.Timestamp = sc.cfg.Now()
	}

	// locked turn：保留在 fullTurns 且不被压缩
	isLocked := turn.Importance >= 4 || turn.IsDecision

	sc.fullTurns = append(sc.fullTurns, turn)
	if isLocked {
		sc.lockedPoints = append(sc.lockedPoints, summarizeTurnForLock(turn))
	}

	// 裁剪：弹出最早的非 locked turn 并 fold 进 summary
	for len(sc.fullTurns) > sc.cfg.KeepFullN {
		// 检查最早一轮是否是 locked
		oldest := sc.fullTurns[0]
		oldestIsLocked := oldest.Importance >= 4 || oldest.IsDecision
		if oldestIsLocked {
			// 跳过 locked turn 不压缩（这种情况说明 keepFullN 不够大）
			break
		}
		sc.fullTurns = sc.fullTurns[1:]
		sc.olderSummary = sc.foldTurn(sc.olderSummary, oldest)
		sc.compressedOut++
	}

	// 摘要也超 token 预算时再次压缩
	for EncodeCount(sc.olderSummary) > sc.cfg.MaxSummaryTokens {
		sc.olderSummary = sc.shrinkSummary(sc.olderSummary)
	}
}

// Render 把当前状态渲染成可直接拼到 prompt 的 markdown。
// 格式：
//
//	<previous_conversation_summary>
//	[...olderSummary...]
//	</previous_conversation_summary>
//
//	<current_session>
//	[...fullTurns...]
//	</current_session>
//
// 返回空字符串当且仅当 fullTurns 与 olderSummary 都为空。
func (sc *SlidingContext) Render() string {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	var sb strings.Builder
	if sc.olderSummary != "" {
		sb.WriteString("<previous_conversation_summary>\n")
		sb.WriteString(sc.olderSummary)
		sb.WriteString("\n</previous_conversation_summary>\n\n")
	}
	if len(sc.fullTurns) > 0 {
		sb.WriteString("<current_session>\n")
		for _, t := range sc.fullTurns {
			ts := t.Timestamp.Format("15:04")
			marker := ""
			if t.IsDecision || t.Importance >= 4 {
				marker = " ★"
			}
			fmt.Fprintf(&sb, "[%s] %s%s: %s\n", ts, t.Role, marker, t.Content)
		}
		sb.WriteString("</current_session>")
	}
	return sb.String()
}

// TokenCount 返回当前 Render() 的总 token 数（基于 EncodeCount）。
//
// 实现要点：Render() 内部会 sc.mu.Lock()，本函数不能再锁（sync.Mutex 不可重入）。
// 所以先 Render 拿到字符串，再在外层 EncodeCount（EncodeCount 本身无锁）。
func (sc *SlidingContext) TokenCount() int {
	// Render 内部已经加锁，返回后锁已释放
	rendered := sc.Render()
	return EncodeCount(rendered)
}

// Stats 返回诊断信息。
func (sc *SlidingContext) Stats() (full int, summaryTokens int, locked int, totalAdded int, compressed int) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	summaryTokens = EncodeCount(sc.olderSummary) // EncodeCount 本身无锁，锁住也没事
	return len(sc.fullTurns), summaryTokens, len(sc.lockedPoints), sc.totalAdded, sc.compressedOut
}

// Reset 清空所有状态（用于测试 / 多 session 复用）。
func (sc *SlidingContext) Reset() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.fullTurns = sc.fullTurns[:0]
	sc.olderSummary = ""
	sc.lockedPoints = sc.lockedPoints[:0]
	sc.totalAdded = 0
	sc.compressedOut = 0
}

// foldTurn 把单轮 turn 折进已有 summary。
// 旧 summary + 新 turn 行 + 截断到 MaxSummaryTokens。
func (sc *SlidingContext) foldTurn(existingSummary string, t Turn) string {
	line := fmt.Sprintf("[%s] %s: %s", t.Timestamp.Format("15:04"), t.Role, oneLineSummary(t.Content))
	combined := existingSummary
	if combined != "" {
		combined += "\n"
	}
	combined += line

	// 超预算则截断（保留尾部 = 最近的内容）
	maxTokens := sc.cfg.MaxSummaryTokens
	if EncodeCount(combined) > maxTokens {
		return EncodeTrimToTokens(combined, maxTokens)
	}
	return combined
}

// shrinkSummary 在摘要超长时再压缩一次，保留决策点。
// 简单策略：从尾部往前逐行移除，直到 EncodeCount ≤ MaxSummaryTokens。
// （更复杂的策略可留待 P3：让 LLM 二次摘要）
func (sc *SlidingContext) shrinkSummary(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= 1 {
		return EncodeTrimToTokens(s, sc.cfg.MaxSummaryTokens)
	}
	// 保留尾部（最近的内容），丢头部
	kept := make([]string, 0, len(lines))
	used := 0
	for i := len(lines) - 1; i >= 0; i-- {
		need := EncodeCount(lines[i]) + 1
		if used+need > sc.cfg.MaxSummaryTokens && len(kept) > 0 {
			break
		}
		kept = append([]string{lines[i]}, kept...)
		used += need
	}
	if len(kept) == 0 {
		return EncodeTrimToTokens(s, sc.cfg.MaxSummaryTokens)
	}
	return strings.Join(kept, "\n")
}

// summarizeTurnForLock 把 locked turn 压成单行（用于统计）
func summarizeTurnForLock(t Turn) string {
	return fmt.Sprintf("[%s] %s: %s", t.Timestamp.Format("15:04"), t.Role, oneLineSummary(t.Content))
}

// oneLineSummary 把多行内容压成单行（折叠换行 / 多余空白）
func oneLineSummary(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	// 多空格合一
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return strings.TrimSpace(s)
}