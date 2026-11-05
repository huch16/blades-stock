package agent

import (
	"strings"
	"testing"
	"time"
)

func TestSCManager_GetOrCreate(t *testing.T) {
	mgr := getSCManager()
	id := "test-session-1"
	mgr.Remove(id) // 确保干净起点

	ss1 := mgr.GetOrCreate(id)
	if ss1 == nil {
		t.Fatal("GetOrCreate returned nil for valid sessionID")
	}
	if ss1.SessionID != id {
		t.Errorf("SessionID mismatch: got %q, want %q", ss1.SessionID, id)
	}
	if ss1.Ctx == nil {
		t.Error("Ctx should not be nil")
	}

	ss2 := mgr.GetOrCreate(id)
	if ss2 != ss1 {
		t.Error("GetOrCreate should return same instance for same sessionID")
	}
}

func TestSCManager_EmptySessionID(t *testing.T) {
	if got := getSCManager().GetOrCreate(""); got != nil {
		t.Error("empty sessionID should return nil")
	}
}

func TestSCManager_Remove(t *testing.T) {
	mgr := getSCManager()
	id := "test-remove-session"
	mgr.GetOrCreate(id)
	mgr.Remove(id)
	if _, _, _, _, _, ok := SessionStats(id); ok {
		t.Error("session should be removed")
	}
}

func TestSCManager_AddTurnAndRender(t *testing.T) {
	mgr := getSCManager()
	id := "test-add-turn"
	mgr.Remove(id)

	// AddTurn 每次加 1 个 user turn + 1 个 assistant turn
	AddTurn(id, "今天 159915 表现如何", "今天大涨 3%", 3, false)
	AddTurn(id, "明天怎么看", "建议持有", 4, true)

	rendered := RenderSession(id)
	if rendered == "" {
		t.Error("RenderSession should not be empty after AddTurn")
	}
	if !strings.Contains(rendered, "159915") {
		t.Errorf("rendered should contain user content: %q", rendered)
	}
	if !strings.Contains(rendered, "建议持有") {
		t.Errorf("rendered should contain assistant content: %q", rendered)
	}

	// 2 次 AddTurn = 4 turns total (2 user + 2 assistant)
	// 第二轮 importance=4 且 isDecision=true → user + assistant 都锁
	full, _, locked, total, _, ok := SessionStats(id)
	if !ok {
		t.Fatal("SessionStats should return ok=true")
	}
	if total != 4 {
		t.Errorf("totalAdded=%d, want 4", total)
	}
	if locked != 2 {
		t.Errorf("locked=%d, want 2 (user+assistant both locked by importance=4)", locked)
	}
	if full < 2 {
		t.Errorf("full turns should be ≥ 2 (locked), got %d", full)
	}
}

func TestSCManager_EmptySessionRendersEmpty(t *testing.T) {
	if got := RenderSession(""); got != "" {
		t.Errorf("RenderSession(\"\") should be empty, got %q", got)
	}
	if _, _, _, _, _, ok := SessionStats(""); ok {
		t.Error("SessionStats(\"\") should return ok=false")
	}
}

func TestSCManager_ResetSession(t *testing.T) {
	id := "test-reset-session"
	AddTurn(id, "first message", "first reply", 3, false)

	ResetSession(id)

	// 重置后再 Render 应为空
	rendered := RenderSession(id)
	// 注意：RenderSession 通过 GetOrCreate 创建新的 context，所以 rendered 是新的空 context
	if rendered != "" {
		t.Errorf("after Reset, RenderSession should be empty, got %q", rendered)
	}
	if _, _, _, _, _, ok := SessionStats(id); !ok {
		t.Error("SessionStats should return ok=true (new session created)")
	}
}

func TestSCManager_GC(t *testing.T) {
	mgr := getSCManager()
	id := "test-gc-session"
	mgr.Remove(id)
	mgr.GetOrCreate(id)

	// maxIdle=0 → 不清理
	if got := mgr.GC(0); got != 0 {
		t.Errorf("GC(0) should return 0, got %d", got)
	}
	if _, _, _, _, _, ok := SessionStats(id); !ok {
		t.Error("session should still exist")
	}

	// 手动设置 session 为“很久以前”以验证清理逻辑
	mgr.mu.Lock()
	if ss, ok := mgr.items[id]; ok {
		ss.LastUsedAt = time.Now().Add(-2 * time.Hour)
	}
	mgr.mu.Unlock()

	if got := mgr.GC(1 * time.Hour); got < 1 {
		t.Errorf("GC(1h) should remove ≥ 1 session, got %d", got)
	}
	if _, _, _, _, _, ok := SessionStats(id); ok {
		t.Error("session should be removed after GC(1h)")
	}
}

// =============================================================================
// 与真实 ChatWithContext 流程的轻量集成测试
// =============================================================================

func TestSCManager_TenTurnsKeepsThree(t *testing.T) {
	mgr := getSCManager()
	id := "test-10-turns"
	mgr.Remove(id)

	// 每次 AddTurn 加 2 turn（user + assistant）；10 次 = 20 turn
	for i := 0; i < 10; i++ {
		AddTurn(id, "用户问题 "+itoa(i), "助手回答 "+itoa(i), 3, false)
	}

	full, _, _, total, compressed, _ := SessionStats(id)
	if total != 20 {
		t.Errorf("totalAdded=%d, want 20 (10 AddTurn × 2 turns)", total)
	}
	// 保留最近 3 turn 完整；压 17 turn
	if full != 3 {
		t.Errorf("full=%d, want 3", full)
	}
	if compressed != 17 {
		t.Errorf("compressed=%d, want 17", compressed)
	}
}
