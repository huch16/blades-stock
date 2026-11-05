// Package agent 的 session 级 SlidingContext 管理器（P2.2 接入层）。
//
// 目的：在 ChatWithContext 中按 sessionID 维护一个 SlidingContext，
// 让 prompt 中的"历史对话"部分用 token-aware 的滑动窗口构造，
// 而不是每次重新 trim 整个历史（避免 O(N²) token cost）。
//
// 设计要点：
//   - 全局 registry（sessionID → *SessionSliding）
//   - SessionSliding 持有 *SlidingContext + 上次加入时间（用于过期清理，可选）
//   - 默认不在 ChatWithContext 里强制使用（feature flag 控制），避免破坏现有 memory 路径
package agent

import (
	"sync"
	"time"
)

// SessionSliding 是单个 session 的滑动上下文包装。
type SessionSliding struct {
	SessionID  string
	Ctx        *SlidingContext
	LastUsedAt time.Time
}

// sessionSlidingManager 全局 registry，sync.Map 适配读多写少场景。
type sessionSlidingManager struct {
	mu    sync.Mutex
	items map[string]*SessionSliding
}

var (
	scManager     *sessionSlidingManager
	scManagerOnce sync.Once
	scDefaultConf = SlidingContextConfig{
		KeepFullN:        3,
		MaxSummaryTokens: 500,
	}
)

func getSCManager() *sessionSlidingManager {
	scManagerOnce.Do(func() {
		scManager = &sessionSlidingManager{items: make(map[string]*SessionSliding)}
		// 启动后台定时 GC 协程，每小时清理闲置超过 24 小时的滑动上下文，防止 24/7 常驻服务内存膨胀
		go func() {
			ticker := time.NewTicker(1 * time.Hour)
			defer ticker.Stop()
			for range ticker.C {
				scManager.GC(24 * time.Hour)
			}
		}()
	})
	return scManager
}

// GetOrCreate 返回 sessionID 对应的 SessionSliding，不存在则创建。
func (m *sessionSlidingManager) GetOrCreate(sessionID string) *SessionSliding {
	if sessionID == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.items[sessionID]; ok {
		s.LastUsedAt = time.Now()
		return s
	}
	s := &SessionSliding{
		SessionID:  sessionID,
		Ctx:        NewSlidingContext(&scDefaultConf),
		LastUsedAt: time.Now(),
	}
	m.items[sessionID] = s
	return s
}

// Remove 删除 session（用户主动重置对话时调用）。
func (m *sessionSlidingManager) Remove(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, sessionID)
}

// GC 清理超过 maxIdle 的 session（防止长时间运行内存增长）。
// maxIdle == 0 表示不过期。
func (m *sessionSlidingManager) GC(maxIdle time.Duration) int {
	if maxIdle <= 0 {
		return 0
	}
	cutoff := time.Now().Add(-maxIdle)
	m.mu.Lock()
	defer m.mu.Unlock()
	removed := 0
	for k, v := range m.items {
		if v.LastUsedAt.Before(cutoff) {
			delete(m.items, k)
			removed++
		}
	}
	return removed
}

// AddTurn 是给 ChatWithContext 用的便捷入口：把 user/assistant 一对 turn 加入 session。
func AddTurn(sessionID, userContent, assistantContent string, importance int, isDecision bool) {
	if sessionID == "" {
		return
	}
	ss := getSCManager().GetOrCreate(sessionID)
	now := time.Now()
	if userContent != "" {
		ss.Ctx.Add(Turn{Role: "user", Content: userContent, Timestamp: now, Importance: importance})
	}
	if assistantContent != "" {
		ss.Ctx.Add(Turn{Role: "assistant", Content: assistantContent, Timestamp: now, Importance: importance, IsDecision: isDecision})
	}
}

// RenderSession 返回当前 session 的滑动上下文渲染（用于注入到 LLM prompt）。
// sessionID 为空时返回空字符串（不影响无 session 场景）。
func RenderSession(sessionID string) string {
	if sessionID == "" {
		return ""
	}
	ss := getSCManager().GetOrCreate(sessionID)
	return ss.Ctx.Render()
}

// SessionStats 返回 session 的诊断信息（用于调试 / 测试）。
func SessionStats(sessionID string) (full int, summaryTokens int, locked int, totalAdded int, compressed int, ok bool) {
	if sessionID == "" {
		return 0, 0, 0, 0, 0, false
	}
	m := getSCManager()
	m.mu.Lock()
	defer m.mu.Unlock()
	ss, exists := m.items[sessionID]
	if !exists {
		return 0, 0, 0, 0, 0, false
	}
	full, summaryTokens, locked, totalAdded, compressed = ss.Ctx.Stats()
	return full, summaryTokens, locked, totalAdded, compressed, true
}

// ResetSession 显式重置 session（前端"开始新对话"按钮调）。
func ResetSession(sessionID string) {
	getSCManager().Remove(sessionID)
}
