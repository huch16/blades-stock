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

	ch     chan workingDBEntry
	stopCh chan struct{}
	wg     sync.WaitGroup
}

type workingEntry struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Time    int64  `json:"time"`
}

type workingDBEntry struct {
	SessionID string
	Role      string
	Content   string
}

func NewMemoryManager(maxWorking int) *MemoryManager {
	if maxWorking <= 0 {
		maxWorking = 20
	}
	m := &MemoryManager{
		workingMem: make([]workingEntry, 0, maxWorking),
		maxWorking: maxWorking,
		ch:         make(chan workingDBEntry, 64),
		stopCh:     make(chan struct{}),
	}
	m.wg.Add(1)
	go m.flushLoop()
	return m
}

func (m *MemoryManager) flushLoop() {
	defer m.wg.Done()
	batch := make([]AgentMemory, 0, 10)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case entry, ok := <-m.ch:
			if !ok {
				m.flushBatch(batch)
				return
			}
			batch = append(batch, AgentMemory{
				SessionID: entry.SessionID,
				Role:      entry.Role,
				Content:   entry.Content,
				Category:  "working",
				CreatedAt: time.Now(),
			})
			if len(batch) >= 10 {
				m.flushBatch(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				m.flushBatch(batch)
				batch = batch[:0]
			}
		case <-m.stopCh:
			m.flushBatch(batch)
			return
		}
	}
}

func (m *MemoryManager) flushBatch(batch []AgentMemory) {
	if len(batch) == 0 {
		return
	}
	if err := db.Dao.CreateInBatches(batch, 10).Error; err != nil {
		logger.SugaredLogger.Errorf("flush agent memories batch: %v", err)
	}
}

func (m *MemoryManager) Add(sessionID, role, content string) {
	m.mu.Lock()
	m.workingMem = append(m.workingMem, workingEntry{
		Role:    role,
		Content: content,
		Time:    time.Now().UnixMilli(),
	})
	if len(m.workingMem) > m.maxWorking {
		m.workingMem = m.workingMem[len(m.workingMem)-m.maxWorking:]
	}
	m.mu.Unlock()

	select {
	case m.ch <- workingDBEntry{SessionID: sessionID, Role: role, Content: content}:
	default:
		logger.SugaredLogger.Warn("memory write channel full, dropping entry")
	}
}

func (m *MemoryManager) Close() {
	close(m.stopCh)
	m.wg.Wait()
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

// StoreReflection persists a reflection conclusion as long-term memory
// (category "reflection"). High/low quality samples flow back through here to
// close the learn-from-experience loop.
func (m *MemoryManager) StoreReflection(sessionID, content string) error {
	if sessionID == "" || strings.TrimSpace(content) == "" {
		return nil
	}
	mem := &AgentMemory{
		SessionID: sessionID,
		Role:      "reflection",
		Content:   content,
		Category:  "reflection",
		CreatedAt: time.Now(),
	}
	if err := db.Dao.Create(mem).Error; err != nil {
		return fmt.Errorf("store reflection: %w", err)
	}
	return nil
}

// RetrieveReflections returns recent reflection memories for a session.
func (m *MemoryManager) RetrieveReflections(sessionID string, limit int) []Fact {
	if limit <= 0 {
		limit = 5
	}
	var mems []AgentMemory
	db.Dao.Where("session_id = ? AND category = ?", sessionID, "reflection").
		Order("id desc").Limit(limit).Find(&mems)
	refs := make([]Fact, 0, len(mems))
	for _, mem := range mems {
		refs = append(refs, Fact{
			ID:        mem.ID,
			SessionID: mem.SessionID,
			Content:   mem.Content,
			CreatedAt: mem.CreatedAt,
		})
	}
	return refs
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
	return m.DeleteMemory(id)
}

// DeleteMemory removes any agent memory row by id (facts, reflections, or
// evolution advice), used by the reflection dedup path.
func (m *MemoryManager) DeleteMemory(id uint) error {
	return db.Dao.Delete(&AgentMemory{}, id).Error
}

// RetrieveEvolutions returns recent evolution prompt-advice memories.
func (m *MemoryManager) RetrieveEvolutions(sessionID string, limit int) []Fact {
	if limit <= 0 {
		limit = 1
	}
	var mems []AgentMemory
	db.Dao.Where("session_id = ? AND category = ?", sessionID, "evolution").
		Order("id desc").Limit(limit).Find(&mems)
	evos := make([]Fact, 0, len(mems))
	for _, mem := range mems {
		evos = append(evos, Fact{
			ID:        mem.ID,
			SessionID: mem.SessionID,
			Content:   mem.Content,
			CreatedAt: mem.CreatedAt,
		})
	}
	return evos
}

func (m *MemoryManager) MemoryPrompt(sessionID string) string {
	facts := m.RetrieveFacts(sessionID, 10)
	refs := m.RetrieveReflections(sessionID, 5)
	evos := m.RetrieveEvolutions(sessionID, 1)
	if len(facts) == 0 && len(refs) == 0 && len(evos) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("【长期记忆】\n")
	for i, f := range facts {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, f.Content))
	}
	if len(refs) > 0 {
		b.WriteString("【历史反思教训】\n")
		for i, r := range refs {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, r.Content))
		}
	}
	if len(evos) > 0 {
		b.WriteString("【进化建议】请结合以下提示词优化建议改进回答风格（如与本次问题无关可忽略）：\n")
		for _, e := range evos {
			b.WriteString(e.Content + "\n")
		}
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
