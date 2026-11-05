package agent

import (
	"strings"
	"testing"
	"time"
)

// =============================================================================
// 基础行为
// =============================================================================

func TestSlidingContext_EmptyRender(t *testing.T) {
	sc := NewSlidingContext(nil)
	if got := sc.Render(); got != "" {
		t.Errorf("empty context should render empty, got %q", got)
	}
}

func TestSlidingContext_OnlyFullTurns(t *testing.T) {
	sc := NewSlidingContext(&SlidingContextConfig{KeepFullN: 3})
	for i := 0; i < 2; i++ {
		sc.Add(Turn{Role: "user", Content: "问题", Timestamp: time.Unix(int64(i), 0)})
	}
	rendered := sc.Render()
	if !strings.Contains(rendered, "<current_session>") {
		t.Error("rendered should have current_session tag")
	}
	if strings.Contains(rendered, "<previous_conversation_summary>") {
		t.Error("no turns should mean no summary tag")
	}
}

// =============================================================================
// 滑动行为：超 KeepFullN 时旧 turn 进 summary
// =============================================================================

func TestSlidingContext_OverflowFoldsToSummary(t *testing.T) {
	sc := NewSlidingContext(&SlidingContextConfig{KeepFullN: 2})
	// 推 5 轮：keepFullN=2 → 应有 3 轮被 fold 进 summary
	for i := 0; i < 5; i++ {
		sc.Add(Turn{
			Role:      "user",
			Content:   "消息内容含中文以验证 token 计算准确性",
			Timestamp: time.Unix(int64(i), 0),
		})
	}
	rendered := sc.Render()
	if !strings.Contains(rendered, "<previous_conversation_summary>") {
		t.Error("expected summary tag after overflow")
	}
	if !strings.Contains(rendered, "<current_session>") {
		t.Error("expected current_session tag")
	}

	// 验证统计：3 turn 被压缩
	full, _, _, total, compressed := sc.Stats()
	if full != 2 {
		t.Errorf("expected 2 full turns, got %d", full)
	}
	if total != 5 {
		t.Errorf("expected totalAdded=5, got %d", total)
	}
	if compressed != 3 {
		t.Errorf("expected compressed=3, got %d", compressed)
	}
}

// =============================================================================
// 锁定的 turn 永不压缩
// =============================================================================

func TestSlidingContext_LockedTurnsNeverCompressed(t *testing.T) {
	sc := NewSlidingContext(&SlidingContextConfig{KeepFullN: 2})
	// 第 1 轮：低 importance
	sc.Add(Turn{Role: "user", Content: "普通消息 A", Timestamp: time.Unix(0, 0), Importance: 1})
	// 第 2 轮：高 importance（locked）
	sc.Add(Turn{Role: "assistant", Content: "关键决策 B", Timestamp: time.Unix(1, 0), Importance: 5})
	// 第 3 轮：低 importance
	sc.Add(Turn{Role: "user", Content: "普通消息 C", Timestamp: time.Unix(2, 0), Importance: 1})
	// 第 4 轮：触发折叠 — 最早的低 importance 应被压
	sc.Add(Turn{Role: "user", Content: "普通消息 D", Timestamp: time.Unix(3, 0), Importance: 1})

	rendered := sc.Render()
	if !strings.Contains(rendered, "关键决策 B") {
		t.Errorf("locked turn should still be in current_session: %q", rendered)
	}
	if !strings.Contains(rendered, "普通消息 A") && !strings.Contains(rendered, "普通消息 C") && !strings.Contains(rendered, "普通消息 D") {
		t.Error("at least one of the non-locked turns should still be visible")
	}
}

func TestSlidingContext_IsDecisionFlagLocks(t *testing.T) {
	sc := NewSlidingContext(&SlidingContextConfig{KeepFullN: 1})
	sc.Add(Turn{Role: "user", Content: "普通消息", Importance: 1})
	sc.Add(Turn{Role: "assistant", Content: "操作建议：买入", IsDecision: true})
	sc.Add(Turn{Role: "user", Content: "再聊点别的", Importance: 1})
	sc.Add(Turn{Role: "user", Content: "又一条", Importance: 1})

	rendered := sc.Render()
	if !strings.Contains(rendered, "操作建议") {
		t.Errorf("decision turn should be in fullTurns: %q", rendered)
	}
}

// =============================================================================
// Token 增长有上界
// =============================================================================

func TestSlidingContext_TokenGrowthBounded(t *testing.T) {
	// 验证推 50 轮后 token 数 < 推 50 轮未压缩时的 50%
	sc := NewSlidingContext(&SlidingContextConfig{KeepFullN: 3, MaxSummaryTokens: 300})

	// 推 50 轮 — 每轮内容约 80 tokens
	for i := 0; i < 50; i++ {
		content := strings.Repeat("测试消息内容用于填充 token 预算。", 3)
		sc.Add(Turn{Role: "user", Content: content, Timestamp: time.Unix(int64(i), 0)})
	}

	finalTokens := sc.TokenCount()

	// 推同样 50 轮但 keepFullN=50 的对比（理论上界）
	sc2 := NewSlidingContext(&SlidingContextConfig{KeepFullN: 50, MaxSummaryTokens: 5000})
	for i := 0; i < 50; i++ {
		content := strings.Repeat("测试消息内容用于填充 token 预算。", 3)
		sc2.Add(Turn{Role: "user", Content: content, Timestamp: time.Unix(int64(i), 0)})
	}
	uncompressedTokens := sc2.TokenCount()

	if finalTokens >= uncompressedTokens {
		t.Errorf("compressed (%d) should be smaller than uncompressed (%d)", finalTokens, uncompressedTokens)
	}
	// 期望：压缩后 < 50% 未压缩
	if float64(finalTokens)/float64(uncompressedTokens) > 0.5 {
		t.Errorf("compression ratio too low: compressed=%d uncompressed=%d ratio=%.2f",
			finalTokens, uncompressedTokens, float64(finalTokens)/float64(uncompressedTokens))
	}
}

// =============================================================================
// Render 格式
// =============================================================================

func TestSlidingContext_RenderStructure(t *testing.T) {
	sc := NewSlidingContext(&SlidingContextConfig{KeepFullN: 2})
	// 用本地时区无关的时间：直接构造 hour+minute
	ts1 := time.Date(2024, 1, 1, 0, 1, 0, 0, time.UTC)
	ts2 := time.Date(2024, 1, 1, 0, 2, 0, 0, time.UTC)
	ts3 := time.Date(2024, 1, 1, 0, 3, 0, 0, time.UTC)
	sc.Add(Turn{Role: "user", Content: "你好", Timestamp: ts1, Importance: 5})
	sc.Add(Turn{Role: "user", Content: "再问一个", Timestamp: ts2, Importance: 1})
	sc.Add(Turn{Role: "user", Content: "第三个", Timestamp: ts3, Importance: 1})

	rendered := sc.Render()
	// ★ 标记：高 importance 应有星号
	if !strings.Contains(rendered, "★") {
		t.Errorf("locked turn should have ★ marker: %q", rendered)
	}
	// 时间戳格式：00:01 / 00:02 / 00:03（UTC，跨平台一致）
	if !strings.Contains(rendered, "00:01") || !strings.Contains(rendered, "00:02") || !strings.Contains(rendered, "00:03") {
		t.Errorf("timestamps should be in HH:MM format: %q", rendered)
	}
}

// =============================================================================
// Stats 与 Reset
// =============================================================================

func TestSlidingContext_Reset(t *testing.T) {
	sc := NewSlidingContext(&SlidingContextConfig{KeepFullN: 2})
	for i := 0; i < 5; i++ {
		sc.Add(Turn{Role: "user", Content: "msg"})
	}
	sc.Reset()
	if rendered := sc.Render(); rendered != "" {
		t.Errorf("after Reset, Render should be empty, got %q", rendered)
	}
	if tokens := sc.TokenCount(); tokens != 0 {
		t.Errorf("after Reset, TokenCount should be 0, got %d", tokens)
	}
}

func TestSlidingContext_StatsAccurate(t *testing.T) {
	// 先推 normal turn 让它们被折叠，最后推 locked turn 验证锁定行为
	sc := NewSlidingContext(&SlidingContextConfig{KeepFullN: 2})
	sc.Add(Turn{Role: "user", Content: "a"})                // normal, idx 0
	sc.Add(Turn{Role: "user", Content: "b"})                // normal, idx 1
	sc.Add(Turn{Role: "user", Content: "c"})                // normal, idx 2 → 触发 fold a
	sc.Add(Turn{Role: "user", Content: "d", Importance: 5}) // locked

	full, _, locked, total, compressed := sc.Stats()
	if total != 4 {
		t.Errorf("totalAdded=%d, want 4", total)
	}
	if locked != 1 {
		t.Errorf("locked=%d, want 1", locked)
	}
	if compressed < 1 {
		t.Errorf("compressed=%d, want ≥ 1", compressed)
	}
	// 锁定 turn 必须在 fullTurns 里
	if full < 1 {
		t.Errorf("full turns should be ≥ 1 (locked turn), got %d", full)
	}
}

// =============================================================================
// 摘要本身超长时收缩
// =============================================================================

func TestSlidingContext_SummaryShrinksWhenOverBudget(t *testing.T) {
	// MaxSummaryTokens=100，远小于正常 10 轮的累计 token
	sc := NewSlidingContext(&SlidingContextConfig{KeepFullN: 2, MaxSummaryTokens: 100})
	for i := 0; i < 20; i++ {
		sc.Add(Turn{Role: "user", Content: strings.Repeat("大量内容", 20)})
	}
	_, summaryTokens, _, _, _ := sc.Stats()
	// 摘要 token 数应在 MaxSummaryTokens 附近（允许一些偏差）
	if summaryTokens > 200 {
		t.Errorf("summary tokens should respect budget, got %d (budget 100)", summaryTokens)
	}
}

// =============================================================================
// oneLineSummary 工具函数
// =============================================================================

func TestOneLineSummary(t *testing.T) {
	tests := map[string]string{
		"hello":        "hello",
		"hello\nworld": "hello world",
		"a\n\nb\n\nc":  "a b c",
		"  spaced  ":   "spaced",
	}
	for in, want := range tests {
		if got := oneLineSummary(in); got != want {
			t.Errorf("oneLineSummary(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOneLineSummary_LongTruncated(t *testing.T) {
	long := strings.Repeat("a", 300)
	got := oneLineSummary(long)
	if !strings.HasSuffix(got, "…") {
		t.Error("long input should be truncated with …")
	}
	if len(got) > 220 {
		t.Errorf("truncated length should be ≤ ~205 chars, got %d", len(got))
	}
}
