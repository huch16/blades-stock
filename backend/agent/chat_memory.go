package agent

import (
	"encoding/json"
	"fmt"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"sort"
	"sync"
	"time"
)

type ChatMemoryService struct {
	sessionID        string
	maxMemory        int
	embeddingService *EmbeddingService

	mu           sync.RWMutex
	cache        []db.ChatMemory
	cacheReady   bool
	msgCount     int
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
	if !skipEmbedding && s.embeddingService != nil {
		if vec, err := s.embeddingService.Embed(content); err != nil {
			logger.SugaredLogger.Warnf("embed %s message failed: %v", role, err)
		} else {
			memory.Embedding, _ = json.Marshal(vec)
		}
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
	return nil
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
}

func (s *ChatMemoryService) semanticRetrieve(memories []db.ChatMemory, query string) ([]map[string]string, error) {
	queryEmbedding, err := s.embeddingService.Embed(query)
	if err != nil || len(queryEmbedding) == 0 {
		logger.SugaredLogger.Warnf("embed query failed, fallback to recent: %v", err)
		return s.recentRetrieve(memories)
	}

	var scored []scoredMemory
	for _, m := range memories {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		if len(m.Embedding) == 0 {
			scored = append(scored, scoredMemory{
				msg:   map[string]string{"role": m.Role, "content": m.Content},
				score: 0,
			})
			continue
		}
		var vec []float32
		if err := json.Unmarshal(m.Embedding, &vec); err != nil || len(vec) == 0 {
			scored = append(scored, scoredMemory{
				msg:   map[string]string{"role": m.Role, "content": m.Content},
				score: 0,
			})
			continue
		}
		sim := CosineSimilarity(queryEmbedding, vec)
		scored = append(scored, scoredMemory{
			msg:   map[string]string{"role": m.Role, "content": m.Content},
			score: sim,
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
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

func (s *ChatMemoryService) GetFormattedHistory() (string, error) {
	history, err := s.GetHistory()
	if err != nil {
		return "", err
	}
	if len(history) == 0 {
		return "", nil
	}
	result := "【对话历史】\n"
	for _, h := range history {
		result += h + "\n"
	}
	return result, nil
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

	summaryText := fmt.Sprintf("以下是最初(%d轮)对话的摘要：\n", len(pairs)/2)
	for _, p := range pairs {
		summaryText += p + "\n"
	}

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
