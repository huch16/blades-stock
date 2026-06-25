package agent

import (
	"context"
	"encoding/json"
	"go-stock/backend/db"
	"go-stock/backend/logger"
	"sort"
	"strings"
	"time"

	"github.com/go-kratos/blades"
	"github.com/go-kratos/blades/memory"
	bladesTools "github.com/go-kratos/blades/tools"
)

type BladesMemoryStore struct {
	sessionID        string
	maxResults       int
	embeddingService *EmbeddingService
}

func NewBladesMemoryStore(sessionID string, maxResults int, es *EmbeddingService) *BladesMemoryStore {
	if maxResults <= 0 {
		maxResults = 20
	}
	return &BladesMemoryStore{
		sessionID:        sessionID,
		maxResults:       maxResults,
		embeddingService: es,
	}
}

func (s *BladesMemoryStore) AddMemory(ctx context.Context, m *memory.Memory) error {
	if m == nil || m.Content == nil {
		return nil
	}
	role := roleToStore(m.Content.Role)
	chatMem := &db.ChatMemory{
		SessionID: s.sessionID,
		Role:      role,
		Content:   m.Content.Text(),
		CreatedAt: time.Now(),
	}
	if s.embeddingService != nil {
		if vec, err := s.embeddingService.Embed(m.Content.Text()); err != nil {
			logger.SugaredLogger.Warnf("embed memory failed: %v", err)
		} else {
			chatMem.Embedding, _ = json.Marshal(vec)
		}
	}
	return chatMem.Save()
}

func (s *BladesMemoryStore) SaveSession(ctx context.Context, session blades.Session) error {
	if session == nil {
		return nil
	}
	for _, msg := range session.History() {
		if msg.Role == blades.RoleUser || msg.Role == blades.RoleAssistant {
			role := roleToStore(msg.Role)
			if role == "" {
				continue
			}
			chatMem := &db.ChatMemory{
				SessionID: s.sessionID,
				Role:      role,
				Content:   msg.Text(),
				CreatedAt: time.Now(),
			}
			if s.embeddingService != nil {
				if vec, err := s.embeddingService.Embed(msg.Text()); err != nil {
					logger.SugaredLogger.Warnf("embed session message failed: %v", err)
				} else {
					chatMem.Embedding, _ = json.Marshal(vec)
				}
			}
			if err := chatMem.Save(); err != nil {
				logger.SugaredLogger.Errorf("save session memory failed: %v", err)
			}
		}
	}
	return nil
}

func (s *BladesMemoryStore) SearchMemory(ctx context.Context, query string) ([]*memory.Memory, error) {
	memories, err := db.GetRecentChatMemory(s.sessionID, s.maxResults*2)
	if err != nil {
		return nil, err
	}

	var scored []scoredMemoryV2
	hasEmbedding := s.embeddingService != nil && query != ""

	var queryVec []float32
	if hasEmbedding {
		queryVec, err = s.embeddingService.Embed(query)
		if err != nil || len(queryVec) == 0 {
			hasEmbedding = false
			logger.SugaredLogger.Warnf("embed search query failed, fallback to keyword: %v", err)
		}
	}

	for _, m := range memories {
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		msg := messageFromRole(m.Role, m.Content)
		if msg == nil {
			continue
		}
		score := float32(0)
		if hasEmbedding && len(m.Embedding) > 0 {
			var vec []float32
			if err := json.Unmarshal(m.Embedding, &vec); err == nil && len(vec) > 0 {
				score = CosineSimilarity(queryVec, vec)
			}
		} else if !hasEmbedding && query != "" {
			score = keywordScore(m.Content, query)
		}
		scored = append(scored, scoredMemoryV2{
			mem:   msg,
			score: score,
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	topN := s.maxResults
	if topN > len(scored) {
		topN = len(scored)
	}

	result := make([]*memory.Memory, 0, topN)
	for i := 0; i < topN; i++ {
		result = append(result, &memory.Memory{
			Content: scored[i].mem,
		})
	}
	return result, nil
}

func (s *BladesMemoryStore) ToTool() (bladesTools.Tool, error) {
	return memory.NewMemoryTool(s)
}

type scoredMemoryV2 struct {
	mem   *blades.Message
	score float32
}

func roleToStore(role blades.Role) string {
	switch role {
	case blades.RoleUser:
		return "user"
	case blades.RoleAssistant:
		return "assistant"
	default:
		return ""
	}
}

func messageFromRole(role, content string) *blades.Message {
	switch role {
	case "user":
		return blades.UserMessage(content)
	case "assistant":
		return blades.AssistantMessage(content)
	default:
		return nil
	}
}

func keywordScore(content, query string) float32 {
	contentLower := strings.ToLower(content)
	queryLower := strings.ToLower(query)
	words := strings.Fields(queryLower)
	if len(words) == 0 {
		return 0
	}
	matches := 0
	for _, word := range words {
		if strings.Contains(contentLower, word) {
			matches++
		}
	}
	return float32(matches) / float32(len(words))
}
