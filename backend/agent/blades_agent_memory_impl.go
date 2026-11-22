package agent

import (
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
	"encoding/json"
	"fmt"
	"sort"
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
	mu         sync.RWMutex
	workingMem []workingEntry
	maxWorking int

	ch        chan workingDBEntry
	stopCh    chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
	embSvc    *EmbeddingService
}

type workingEntry struct {
	SessionID string `json:"sessionId"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Time      int64  `json:"time"`
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

// SetEmbeddingService sets the embedding service for semantic memory search
func (m *MemoryManager) SetEmbeddingService(svc *EmbeddingService) {
	m.embSvc = svc
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
			mem := AgentMemory{
				SessionID: entry.SessionID,
				Role:      entry.Role,
				Content:   entry.Content,
				Category:  "working",
				CreatedAt: time.Now(),
			}
			batch = append(batch, mem)
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
		SessionID: sessionID,
		Role:      role,
		Content:   content,
		Time:      time.Now().UnixMilli(),
	})
	// Per-session sliding window: drop oldest entries across all sessions,
	// then trim each session to maxWorking to prevent one session from
	// starving others.
	if len(m.workingMem) > m.maxWorking*4 {
		m.workingMem = m.workingMem[len(m.workingMem)-m.maxWorking*4:]
	}
	// 旧实现只有上面这行全局裁剪，“再按会话裁剪”并没做：单个会话
	// 可以独占全部 maxWorking*4 个槽位，注释声称要防的 starvation
	// 实际仍会发生。此处补上每会话上限，删该会话最旧的条目。
	m.trimSessionLocked(sessionID)
	m.mu.Unlock()

	select {
	case m.ch <- workingDBEntry{SessionID: sessionID, Role: role, Content: content}:
	default:
		logger.SugaredLogger.Warn("memory write channel full, dropping entry")
	}
}

// trimSessionLocked 删除 sessionID 最旧的条目，直到该会话只剩
// maxWorking 条。调用方必须已持 m.mu（写锁）。
func (m *MemoryManager) trimSessionLocked(sessionID string) {
	for {
		count := 0
		oldest := -1
		for i, e := range m.workingMem {
			if e.SessionID == sessionID {
				count++
				if oldest < 0 {
					oldest = i
				}
			}
		}
		if count <= m.maxWorking || oldest < 0 {
			return
		}
		m.workingMem = append(m.workingMem[:oldest], m.workingMem[oldest+1:]...)
	}
}

func (m *MemoryManager) Close() {
	// 幂等：旧实现直接 close(stopCh)，调两次就 panic。
	// Close 本来就没有调用者，防一手总比炸掉好。
	m.closeOnce.Do(func() {
		close(m.stopCh)
	})
	m.wg.Wait()
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

// RetrieveFacts returns fact memories for a session.
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

// SemanticSearchMemory searches memories by semantic similarity using embeddings
// MemoryPrompt returns a formatted string of long-term memories for a session
func (m *MemoryManager) SemanticSearchMemory(sessionID, query string, category string, limit int) ([]AgentMemory, error) {
	if m.embSvc == nil || !m.embSvc.IsConfigured() {
		return nil, fmt.Errorf("embedding service not configured")
	}
	if limit <= 0 {
		limit = 5
	}

	// Generate embedding for query
	queryEmbedding, err := m.embSvc.Embed(query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// Retrieve memories from DB (filtered by session and category if specified)
	var mems []AgentMemory
	queryDB := db.Dao.Model(&AgentMemory{}).Where("session_id = ?", sessionID)
	if category != "" {
		queryDB = queryDB.Where("category = ?", category)
	}
	if err := queryDB.Order("id desc").Find(&mems).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve memories: %w", err)
	}

	// Score memories by cosine similarity.
	// 优先用库里已存的向量（AgentMemory.Embedding 是 little-endian
	// float32 BLOB，DistillAgentMemories 等写入路径会填充它）：
	// 旧实现对每条记忆都重新调一次 Embed，N 条记忆就是 N 次
	// embedding 调用（缓存未命中时还是 N 次网络请求），既慢又
	// 完全无视了已存向量。只有无存量向量时才回落到实时 Embed。
	type scoredMemory struct {
		mem   AgentMemory
		score float32
	}
	var scored []scoredMemory

	for _, mem := range mems {
		var memEmbedding []float32
		if len(mem.Embedding) > 0 {
			memEmbedding = models.BytesToFloat32sLE(mem.Embedding)
		}
		if len(memEmbedding) == 0 {
			var err error
			memEmbedding, err = m.embSvc.Embed(mem.Content)
			if err != nil {
				logger.SugaredLogger.Warnf("failed to embed memory %d: %v", mem.ID, err)
				continue
			}
		}

		// Compute cosine similarity
		score := CosineSimilarity(queryEmbedding, memEmbedding)
		scored = append(scored, scoredMemory{mem: mem, score: score})
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Return top results
	if limit > len(scored) {
		limit = len(scored)
	}
	result := make([]AgentMemory, 0, limit)
	for i := 0; i < limit; i++ {
		result = append(result, scored[i].mem)
	}
	return result, nil
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

func truncateText(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

var _ = json.Marshal

// InitEmbeddingService initializes the embedding service for the default memory manager
// using settings from the configuration. Should be called during application startup.
func InitEmbeddingService() {
	if setting := data.GetSettingConfig(); setting != nil {
		memoryManager := DefaultMemoryManager()
		if memoryManager != nil && setting.EmbeddingEnabled &&
			strings.TrimSpace(setting.EmbeddingModelName) != "" &&
			strings.TrimSpace(setting.EmbeddingBaseUrl) != "" {

			embSvc := NewEmbeddingService(
				setting.EmbeddingBaseUrl,
				setting.EmbeddingApiKey,
				setting.EmbeddingModelName,
			)
			memoryManager.SetEmbeddingService(embSvc)
			logger.SugaredLogger.Infof("Embedding service initialized for memory manager: %s", embSvc.Describe())
		}
	}
}
