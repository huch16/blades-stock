package agent

import (
	"encoding/json"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"sort"
)

type ChatMemoryService struct {
	sessionID        string
	maxMemory        int
	embeddingService *EmbeddingService
}

func NewChatMemoryService(sessionID string, maxMemory int) *ChatMemoryService {
	if maxMemory <= 0 {
		maxMemory = 20
	}
	return &ChatMemoryService{
		sessionID: sessionID,
		maxMemory: maxMemory * 2,
	}
}

func (s *ChatMemoryService) SetEmbeddingService(es *EmbeddingService) {
	s.embeddingService = es
}

func (s *ChatMemoryService) AddUserMessage(content string) error {
	memory := &db.ChatMemory{
		SessionID: s.sessionID,
		Role:      "user",
		Content:   content,
	}
	if s.embeddingService != nil {
		if vec, err := s.embeddingService.Embed(content); err != nil {
			logger.SugaredLogger.Warnf("embed user message failed: %v", err)
		} else {
			memory.Embedding, _ = json.Marshal(vec)
		}
	}
	return memory.Save()
}

func (s *ChatMemoryService) AddAssistantMessage(content string) error {
	memory := &db.ChatMemory{
		SessionID: s.sessionID,
		Role:      "assistant",
		Content:   content,
	}
	if s.embeddingService != nil {
		if vec, err := s.embeddingService.Embed(content); err != nil {
			logger.SugaredLogger.Warnf("embed assistant message failed: %v", err)
		} else {
			memory.Embedding, _ = json.Marshal(vec)
		}
	}
	return memory.Save()
}

func (s *ChatMemoryService) GetHistoryMessages(query ...string) ([]map[string]string, error) {
	memories, err := db.GetRecentChatMemory(s.sessionID, s.maxMemory)
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
	memories, err := db.GetRecentChatMemory(s.sessionID, s.maxMemory)
	if err != nil {
		logger.SugaredLogger.Errorf("GetChatMemoryList failed: %v", err)
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

func (s *ChatMemoryService) Clear() error {
	return db.ClearChatMemory(s.sessionID)
}
