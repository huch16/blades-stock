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

var (
	defaultMemoryManager *MemoryManager
	memoryManagerOnce    sync.Once
)

func DefaultMemoryManager() *MemoryManager {
	memoryManagerOnce.Do(func() {
		defaultMemoryManager = NewMemoryManager(20)
	})
	return defaultMemoryManager
}

type MemoryManager struct {
	mu            sync.RWMutex
	workingMem    []workingEntry
	maxWorking    int
}

type workingEntry struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Time    int64  `json:"time"`
}

func NewMemoryManager(maxWorking int) *MemoryManager {
	if maxWorking <= 0 {
		maxWorking = 20
	}
	return &MemoryManager{
		workingMem: make([]workingEntry, 0, maxWorking),
		maxWorking: maxWorking,
	}
}

func (m *MemoryManager) Add(sessionID, role, content string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.workingMem = append(m.workingMem, workingEntry{
		Role:    role,
		Content: content,
		Time:    time.Now().UnixMilli(),
	})
	if len(m.workingMem) > m.maxWorking {
		m.workingMem = m.workingMem[len(m.workingMem)-m.maxWorking:]
	}
	mem := &AgentMemory{
		SessionID: sessionID,
		Role:      role,
		Content:   content,
		Category:  "working",
		CreatedAt: time.Now(),
	}
	if err := db.Dao.Create(mem).Error; err != nil {
		logger.SugaredLogger.Errorf("save agent memory: %v", err)
	}
}

func (m *MemoryManager) GetWorking() []workingEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]workingEntry, len(m.workingMem))
	copy(out, m.workingMem)
	return out
}

func (m *MemoryManager) WorkingPrompt() string {
	m.mu.RLock()
	entries := m.workingMem
	m.mu.RUnlock()
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("【本轮工作记忆】\n")
	for _, e := range entries {
		label := "用户"
		if e.Role == "assistant" {
			label = "助手"
		}
		b.WriteString(fmt.Sprintf("[%s] %s\n", label, truncateText(e.Content, 200)))
	}
	return b.String()
}

func (m *MemoryManager) SummarizeAndArchive(sessionID string) (string, error) {
	m.mu.Lock()
	entries := m.workingMem
	m.workingMem = make([]workingEntry, 0, m.maxWorking)
	m.mu.Unlock()

	if len(entries) == 0 {
		return "", nil
	}

	var lines []string
	for _, e := range entries {
		label := "U"
		if e.Role == "assistant" {
			label = "A"
		}
		lines = append(lines, fmt.Sprintf("%s: %s", label, e.Content))
	}
	raw := strings.Join(lines, "\n")

	summary := raw
	if len(summary) > 1000 {
		summary = summary[:1000] + "...(已压缩)"
	}

	summary = fmt.Sprintf("会话摘要 (%d 轮):\n%s", len(entries)/2+1, summary)

	var session AgentSession
	err := db.Dao.Where("session_id = ?", sessionID).First(&session).Error
	if err != nil {
		session = AgentSession{
			SessionID: sessionID,
			Title:     "会话 " + time.Now().Format("2006-01-02 15:04"),
			Summary:   summary,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		db.Dao.Create(&session)
	} else {
		db.Dao.Model(&session).Updates(map[string]interface{}{
			"summary":    summary,
			"updated_at": time.Now(),
		})
	}
	return summary, nil
}

type Fact struct {
	ID        uint      `json:"id"`
	SessionID string    `json:"sessionId"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

func (m *MemoryManager) StoreFact(sessionID, content string) error {
	mem := &AgentMemory{
		SessionID: sessionID,
		Role:      "fact",
		Content:   content,
		Category:  "fact",
		CreatedAt: time.Now(),
	}
	if err := db.Dao.Create(mem).Error; err != nil {
		return fmt.Errorf("store fact: %w", err)
	}
	db.Dao.Model(&AgentSession{}).Where("session_id = ?", sessionID).
		UpdateColumn("fact_count", db.Dao.Raw("fact_count + 1"))
	return nil
}

func (m *MemoryManager) RetrieveFacts(sessionID string, limit int) []Fact {
	if limit <= 0 {
		limit = 10
	}
	var mems []AgentMemory
	db.Dao.Where("session_id = ? AND category = ?", sessionID, "fact").
		Order("id desc").Limit(limit).Find(&mems)
	facts := make([]Fact, 0, len(mems))
	for _, mem := range mems {
		facts = append(facts, Fact{
			ID:        mem.ID,
			SessionID: mem.SessionID,
			Content:   mem.Content,
			CreatedAt: mem.CreatedAt,
		})
	}
	return facts
}

func (m *MemoryManager) DeleteFact(id uint) error {
	return db.Dao.Delete(&AgentMemory{}, id).Error
}

func (m *MemoryManager) MemoryPrompt(sessionID string) string {
	facts := m.RetrieveFacts(sessionID, 10)
	if len(facts) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("【长期记忆】\n")
	for i, f := range facts {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, f.Content))
	}
	return b.String()
}

func (m *MemoryManager) GetAllSessions(limit int) []AgentSession {
	if limit <= 0 {
		limit = 20
	}
	var sessions []AgentSession
	db.Dao.Order("updated_at desc").Limit(limit).Find(&sessions)
	return sessions
}

func (m *MemoryManager) GetSession(sessionID string) *AgentSession {
	var session AgentSession
	err := db.Dao.Where("session_id = ?", sessionID).First(&session).Error
	if err != nil {
		return nil
	}
	return &session
}

func (m *MemoryManager) GetSessionMemories(sessionID string, limit int) []AgentMemory {
	if limit <= 0 {
		limit = 50
	}
	var mems []AgentMemory
	db.Dao.Where("session_id = ?", sessionID).Order("id desc").Limit(limit).Find(&mems)
	return mems
}

func (m *MemoryManager) CleanupOldSessions(days int) (int, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	r := db.Dao.Where("created_at < ?", cutoff).Delete(&AgentSession{})
	if r.Error != nil {
		return 0, r.Error
	}
	db.Dao.Where("created_at < ?", cutoff).Delete(&AgentMemory{})
	return int(r.RowsAffected), nil
}

func truncateText(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

var _ = json.Marshal
