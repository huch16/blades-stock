package agent

import (
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/util"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type ChatMemoryService struct {
	sessionID        string
	maxMemory        int
	embeddingService *EmbeddingService

	mu         sync.RWMutex
	cache      []db.ChatMemory
	cacheReady bool
	msgCount   int
}

func NewChatMemoryService(sessionID string, maxMemory int) *ChatMemoryService {
	if maxMemory <= 0 {
		maxMemory = 20
	}
	return &ChatMemoryService{
		sessionID: sessionID,
		maxMemory: maxMemory * 2,
		cache:     make([]db.ChatMemory, 0, maxMemory*2),
	}
}

func (s *ChatMemoryService) SetEmbeddingService(es *EmbeddingService) {
	s.embeddingService = es
}

func (s *ChatMemoryService) addMessage(role, content string, skipEmbedding bool) error {
	memory := &db.ChatMemory{
		SessionID: s.sessionID,
		Role:      role,
		Content:   content,
	}
	if err := memory.Save(); err != nil {
		return err
	}

	s.mu.Lock()
	s.msgCount++
	s.cache = append(s.cache, *memory)
	if len(s.cache) > s.maxMemory {
		s.cache = s.cache[len(s.cache)-s.maxMemory:]
	}
	s.mu.Unlock()

	// Embedding is offloaded to a background worker so the chat request path
	// never blocks on the embedding HTTP call (which can take seconds).
	if !skipEmbedding && s.embeddingService != nil {
		enqueueEmbedding(s, memory.ID, content)
	}
	return nil
}

type chatMemoryEmbedTask struct {
	svc      *ChatMemoryService
	memoryID uint
	content  string
}

var (
	chatMemoryEmbedCh     chan chatMemoryEmbedTask
	chatMemoryEmbedWorker sync.Once
)

// enqueueEmbedding schedules an async embedding update for a saved chat
// message. The worker persists the vector to DB and refreshes the in-memory
// cache so subsequent semantic retrieval in the same session can use it.
func enqueueEmbedding(s *ChatMemoryService, memoryID uint, content string) {
	chatMemoryEmbedWorker.Do(func() {
		chatMemoryEmbedCh = make(chan chatMemoryEmbedTask, 256)
		go chatMemoryEmbedLoop()
	})
	select {
	case chatMemoryEmbedCh <- chatMemoryEmbedTask{svc: s, memoryID: memoryID, content: content}:
	default:
		logger.SugaredLogger.Warn("chat memory embedding queue full, dropping async embed")
	}
}

func chatMemoryEmbedLoop() {
	for task := range chatMemoryEmbedCh {
		svc := task.svc
		if svc == nil || svc.embeddingService == nil {
			continue
		}
		vec, err := svc.embeddingService.Embed(task.content)
		if err != nil {
			logger.SugaredLogger.Warnf("async embed chat memory %d failed: %v", task.memoryID, err)
			continue
		}
		raw, err := json.Marshal(vec)
		if err != nil {
			continue
		}
		if err := db.Dao.Model(&db.ChatMemory{}).Where("id = ?", task.memoryID).UpdateColumn("embedding", raw).Error; err != nil {
			logger.SugaredLogger.Warnf("update chat_memory embedding failed (id=%d): %v", task.memoryID, err)
		}
		svc.mu.Lock()
		for i := range svc.cache {
			if svc.cache[i].ID == task.memoryID {
				svc.cache[i].Embedding = raw
				break
			}
		}
		svc.mu.Unlock()
	}
}

func (s *ChatMemoryService) AddUserMessage(content string) error {
	return s.addMessage("user", content, false)
}

func (s *ChatMemoryService) AddAssistantMessage(content string) error {
	return s.addMessage("assistant", content, false)
}

func (s *ChatMemoryService) ensureCache() error {
	s.mu.RLock()
	if s.cacheReady {
		s.mu.RUnlock()
		return nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cacheReady {
		return nil
	}

	memories, err := db.GetRecentChatMemory(s.sessionID, s.maxMemory)
	if err != nil {
		return err
	}
	s.cache = memories
	if len(s.cache) > s.maxMemory {
		s.cache = s.cache[len(s.cache)-s.maxMemory:]
	}
	s.cacheReady = true
	return nil
}

func (s *ChatMemoryService) loadMessages() ([]db.ChatMemory, error) {
	if err := s.ensureCache(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	out := make([]db.ChatMemory, len(s.cache))
	copy(out, s.cache)
	s.mu.RUnlock()
	return out, nil
}

func (s *ChatMemoryService) GetHistoryMessages(query ...string) ([]map[string]string, error) {
	memories, err := s.loadMessages()
	if err != nil {
		logger.SugaredLogger.Errorf("GetChatMemoryList failed: %v", err)
		return nil, err
	}

	if s.embeddingService != nil && len(query) > 0 && query[0] != "" {
		return s.semanticRetrieve(memories, query[0])
	}

	return s.recentRetrieve(memories)
}

func (s *ChatMemoryService) recentRetrieve(memories []db.ChatMemory) ([]map[string]string, error) {
	var messages []map[string]string
	for _, m := range memories {
		if m.Role == "user" || m.Role == "assistant" {
			messages = append(messages, map[string]string{
				"role":    m.Role,
				"content": m.Content,
			})
		}
	}
	return messages, nil
}

type scoredMemory struct {
	msg   map[string]string
	score float32
	idx   int
}

// 语义召回 = 语义相似度 + 新鲜度加权。刚写入的消息嵌入尚未异步生成（无向量），
// 若只按相似度打分会把它们全压到 0 分而被旧消息挤出，这里给它们一条新鲜度基线，
// 使最近的对话尾部始终可见，同时语义更相关的旧消息仍能胜出。
const (
	semanticWeight = float32(0.7)
	recencyWeight  = float32(0.3)
)

func (s *ChatMemoryService) semanticRetrieve(memories []db.ChatMemory, query string) ([]map[string]string, error) {
	queryEmbedding, err := s.embeddingService.Embed(query)
	if err != nil || len(queryEmbedding) == 0 {
		logger.SugaredLogger.Warnf("embed query failed, fallback to recent: %v", err)
		return s.recentRetrieve(memories)
	}

	n := float32(len(memories))
	var scored []scoredMemory
	for i, m := range memories {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		// memories 按时间升序（旧→新），新鲜度按位置线性归一。
		recency := float32(1.0)
		if n > 1 {
			recency = float32(i+1) / n
		}
		sim := float32(0)
		if len(m.Embedding) > 0 {
			var vec []float32
			if err := json.Unmarshal(m.Embedding, &vec); err == nil && len(vec) > 0 {
				sim = CosineSimilarity(queryEmbedding, vec)
				if sim < 0 {
					sim = 0
				}
			}
		}
		score := semanticWeight*sim + recencyWeight*recency
		scored = append(scored, scoredMemory{
			msg:   map[string]string{"role": m.Role, "content": m.Content},
			score: score,
			idx:   i,
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].idx > scored[j].idx
	})

	topN := s.maxMemory / 2
	if topN > len(scored) {
		topN = len(scored)
	}

	result := make([]map[string]string, 0, topN)
	for i := 0; i < topN; i++ {
		result = append(result, scored[i].msg)
	}

	if len(result) == 0 {
		return s.recentRetrieve(memories)
	}
	return result, nil
}

func (s *ChatMemoryService) GetHistory() ([]string, error) {
	memories, err := s.loadMessages()
	if err != nil {
		logger.SugaredLogger.Errorf("GetHistory failed: %v", err)
		return nil, err
	}

	var history []string
	for _, m := range memories {
		role := m.Role
		if role == "user" {
			history = append(history, "用户: "+m.Content)
		} else if role == "assistant" {
			history = append(history, "助手: "+m.Content)
		}
	}
	return history, nil
}

func (s *ChatMemoryService) GetSummary() (string, error) {
	summary, err := db.GetChatMemorySummary(s.sessionID)
	if err != nil {
		return "", nil
	}
	return summary.Summary, nil
}

func (s *ChatMemoryService) NeedsCompress() bool {
	s.mu.RLock()
	count := s.msgCount
	s.mu.RUnlock()
	if count > 30 {
		return true
	}
	dbCount, err := db.CountChatMemory(s.sessionID)
	if err != nil {
		logger.SugaredLogger.Warnf("count chat memory failed: %v", err)
		return false
	}
	s.mu.Lock()
	if dbCount > s.msgCount {
		s.msgCount = dbCount
	}
	s.mu.Unlock()
	return dbCount > 30
}

func (s *ChatMemoryService) CompressHistory(latestMessages []map[string]string) ([]map[string]string, error) {
	allMessages, err := db.GetRecentChatMemory(s.sessionID, 200)
	if err != nil {
		return latestMessages, err
	}

	targetCount := len(allMessages) / 2
	if targetCount < 5 {
		targetCount = 5
	}

	compressEnd := len(allMessages) - targetCount
	if compressEnd <= 0 {
		return latestMessages, nil
	}

	compressible := allMessages[:compressEnd]

	var pairs []string
	for _, m := range compressible {
		if m.Role == "user" {
			pairs = append(pairs, "用户: "+m.Content)
		} else if m.Role == "assistant" {
			pairs = append(pairs, "助手: "+m.Content)
		}
	}

	if len(pairs) == 0 {
		return latestMessages, nil
	}

	// 滚动摘要：基于上一轮摘要 + 本次被压缩对话的关键要点拼接，而不是把全部
	// 历史截断拼成一个伪"摘要"。真正的 LLM 摘要随后异步生成并覆盖本占位版本，
	// 下一轮请求通过 GetSummary 即可读到自然语言摘要。
	prevSummary, _ := s.GetSummary()
	summaryText := buildRollingSummary(prevSummary, pairs)

	for i := 0; i < compressEnd; i++ {
		if err := allMessages[i].Delete(); err != nil {
			logger.SugaredLogger.Errorf("delete chat memory failed: %v", err)
		}
	}

	if err := db.DeleteChatMemorySummary(s.sessionID); err != nil {
		logger.SugaredLogger.Errorf("delete chat memory summary failed: %v", err)
	}
	_ = (&db.ChatMemorySummary{
		SessionID: s.sessionID,
		Summary:   summaryText,
		CreatedAt: time.Now(),
	}).Save()

	// 异步 LLM 升级为真正的自然语言摘要（不阻塞请求路径）。
	s.summarizeHistoryAsync(s.sessionID, pairs)

	logger.SugaredLogger.Infof("compressed %d messages into summary for session %s", len(compressible), s.sessionID)

	s.mu.Lock()
	s.cache = nil
	s.cacheReady = false
	s.msgCount = 0
	s.mu.Unlock()

	summaryMsg := []map[string]string{{
		"role":    "system",
		"content": summaryText,
	}}
	return append(summaryMsg, latestMessages...), nil
}

// buildRollingSummary compacts the previous summary plus this round's
// compressible messages into a bounded key-points block. It is the cheap
// synchronous fallback until the async LLM summary lands.
func buildRollingSummary(prevSummary string, pairs []string) string {
	var b strings.Builder
	if prev := strings.TrimSpace(prevSummary); prev != "" {
		b.WriteString(prev)
		b.WriteString("\n")
	}
	rounds := len(pairs) / 2
	if rounds < 1 {
		rounds = 1
	}
	b.WriteString(fmt.Sprintf("早期对话要点(%d轮，已压缩)：\n", rounds))
	kept := 0
	for _, p := range pairs {
		if kept >= 24 {
			b.WriteString("...(更早对话要点已省略)\n")
			break
		}
		b.WriteString(util.TruncateString(p, 120, truncateSuffix) + "\n")
		kept++
	}
	return b.String()
}

// summaryGenMu serializes async LLM summary generation so concurrent
// CompressHistory calls don't interleave LLM writes.
var summaryGenMu sync.Mutex

// summarizeHistoryAsync uses the configured LLM to turn the compressed round
// into a true prose summary, replacing the placeholder written by the sync
// path. It runs in the background and is best-effort: on any failure the
// rolling key-points summary stays in place.
func (s *ChatMemoryService) summarizeHistoryAsync(sessionID string, pairs []string) {
	defer func() {
		if r := recover(); r != nil {
			logger.SugaredLogger.Errorf("panic in summarizeHistoryAsync: %v", r)
		}
	}()
	if len(pairs) == 0 {
		return
	}
	llm := defaultLLMReflectionFunc()
	if llm == nil {
		return
	}

	var sb strings.Builder
	for i, p := range pairs {
		if i >= 40 {
			sb.WriteString("...(更早对话内容省略)\n")
			break
		}
		sb.WriteString(p)
		sb.WriteString("\n")
	}
	raw := sb.String()
	if len(raw) > 8000 {
		raw = raw[:8000] + "\n...(内容截断)"
	}

	summaryGenMu.Lock()
	defer summaryGenMu.Unlock()

	resp, err := llm(
		"你是一个严谨的对话摘要助手，只输出中文摘要，不输出任何其他内容。",
		"请把以下多轮对话压缩为一段精炼的中文摘要（保留关键事实、结论、股票/代码/具体数字、已明确的用户偏好与风险提示，丢弃寒暄与重复），控制在 300 字以内：\n\n"+raw,
	)
	if err != nil || strings.TrimSpace(resp) == "" {
		return
	}
	if err := db.DeleteChatMemorySummary(sessionID); err != nil {
		logger.SugaredLogger.Errorf("delete summary before async write failed: %v", err)
	}
	_ = (&db.ChatMemorySummary{
		SessionID: sessionID,
		Summary:   util.TruncateString(strings.TrimSpace(resp), 2000, truncateSuffix),
		CreatedAt: time.Now(),
	}).Save()
	logger.SugaredLogger.Debugf("async LLM summary stored for session %s", sessionID)
}

func (s *ChatMemoryService) Clear() error {
	if err := db.DeleteChatMemorySummary(s.sessionID); err != nil {
		logger.SugaredLogger.Errorf("delete chat memory summary failed in Clear: %v", err)
	}
	err := db.ClearChatMemory(s.sessionID)
	s.mu.Lock()
	s.cache = nil
	s.cacheReady = false
	s.msgCount = 0
	s.mu.Unlock()
	return err
}
