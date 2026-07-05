package agent

import (
	"fmt"
	"strings"
	"sync"

	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
)

type SkillEmbedding struct {
	ID        uint `gorm:"primaryKey"`
	SkillID   uint `gorm:"uniqueIndex;not null"`
	Embedding []float32
}

func (SkillEmbedding) TableName() string { return "skill_embeddings" }

type SkillEmbeddingService struct {
	es      *EmbeddingService
	enabled bool
	mu      sync.RWMutex
	// skillID → embedding vector
	embeddings map[uint][]float32
}

var skillEmbedSvc *SkillEmbeddingService
var skillEmbedOnce sync.Once

func GetSkillEmbeddingService(es *EmbeddingService) *SkillEmbeddingService {
	skillEmbedOnce.Do(func() {
		skillEmbedSvc = &SkillEmbeddingService{
			es:         es,
			embeddings: make(map[uint][]float32),
		}
		skillEmbedSvc.enabled = es != nil && es.IsConfigured()
		if skillEmbedSvc.enabled {
			logger.SugaredLogger.Info("SkillEmbeddingService 已启用（bge-m3语义匹配）")
			if err := skillEmbedSvc.loadFromDB(); err != nil {
				logger.SugaredLogger.Warnf("从DB加载skill embedding失败: %v，将重新计算", err)
				_ = skillEmbedSvc.RefreshEmbeddings()
			}
		} else {
			logger.SugaredLogger.Warn("EmbeddingService 未配置，SkillEmbeddingService 降级为关键词匹配")
		}
	})
	return skillEmbedSvc
}



func (s *SkillEmbeddingService) loadFromDB() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var rows []SkillEmbedding
	if err := db.Dao.Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		s.embeddings[row.SkillID] = row.Embedding
	}
	logger.SugaredLogger.Infof("从DB加载了 %d 个skill embedding", len(rows))
	return nil
}

func (s *SkillEmbeddingService) RefreshEmbeddings() error {
	if !s.enabled {
		return nil
	}
	skills := data.NewSkillApi().GetEnabledSkills()
	if len(skills) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make(map[uint][]float32)

	for _, skill := range skills {
		wg.Add(1)
		go func(sk models.Skill) {
			defer wg.Done()
			// Embed description + system prompt (the meaningful part)
			text := sk.Description + "\n" + sk.SystemPrompt
			vec, err := s.es.Embed(text)
			if err != nil {
				logger.SugaredLogger.Warnf("skill %s embedding失败: %v", sk.Name, err)
				return
			}
			mu.Lock()
			results[sk.ID] = vec
			mu.Unlock()
		}(skill)
	}
	wg.Wait()

	if len(results) == 0 {
		logger.SugaredLogger.Warn("所有skill embedding计算失败")
		return nil
	}

	// Batch upsert to DB
	s.embeddings = results

	if err := db.Dao.Exec("DELETE FROM skill_embeddings").Error; err != nil {
		logger.SugaredLogger.Warnf("清空skill_embeddings表失败: %v", err)
	}

	rows := make([]SkillEmbedding, 0, len(results))
	for skillID, emb := range results {
		rows = append(rows, SkillEmbedding{
			SkillID:   skillID,
			Embedding: emb,
		})
	}
	if err := db.Dao.CreateInBatches(rows, 100).Error; err != nil {
		logger.SugaredLogger.Warnf("批量写入skill_embeddings失败: %v", err)
	}

	logger.SugaredLogger.Infof("已刷新 %d 个skill embedding（bge-m3）", len(results))
	return nil
}

func (s *SkillEmbeddingService) MatchSkills(question string, topK int) []models.Skill {
	skills := data.NewSkillApi().GetEnabledSkills()
	if len(skills) == 0 {
		return nil
	}

	// Fallback: if embedding not enabled, use keyword match
	if !s.enabled || s.es == nil {
		return s.keywordMatch(skills, question, topK)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Embed question
	vec, err := s.es.Embed(question)
	if err != nil {
		logger.SugaredLogger.Warnf("question embedding失败，降级关键词: %v", err)
		return s.keywordMatch(skills, question, topK)
	}

	// Score all skills by cosine similarity
	type scored struct {
		skill models.Skill
		score float32
	}
	var scoredSkills []scored
	for _, skill := range skills {
		emb, ok := s.embeddings[skill.ID]
		if !ok {
			continue
		}
		score := CosineSimilarity(vec, emb)
		if score > 0.3 {
			scoredSkills = append(scoredSkills, scored{skill, score})
		}
	}

	// Sort by score desc
	for i := 0; i < len(scoredSkills)-1; i++ {
		for j := i + 1; j < len(scoredSkills); j++ {
			if scoredSkills[j].score > scoredSkills[i].score {
				scoredSkills[i], scoredSkills[j] = scoredSkills[j], scoredSkills[i]
			}
		}
	}

	if len(scoredSkills) > topK {
		scoredSkills = scoredSkills[:topK]
	}

	var matched []models.Skill
	for _, sc := range scoredSkills {
		matched = append(matched, sc.skill)
	}
	return matched
}

func (s *SkillEmbeddingService) keywordMatch(skills []models.Skill, question string, topK int) []models.Skill {
	var matched []models.Skill
	for _, skill := range skills {
		if skill.TriggerKeywords == "" {
			if len(matched) < topK {
				matched = append(matched, skill)
			}
			continue
		}
		keywords := strings.Split(skill.TriggerKeywords, ",")
		for _, kw := range keywords {
			kw = strings.TrimSpace(kw)
			if kw != "" && strings.Contains(question, kw) {
				matched = append(matched, skill)
				break
			}
		}
	}
	return matched
}

// BuildShortSkillPrompt generates a short directive-only prompt for matched skills.
// The long reference content is stripped; the agent must infer from the directive.
// Target: ~800 chars per skill.
func BuildShortSkillPrompt(matchedSkills []models.Skill) string {
	if len(matchedSkills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n===== 【专业技能 - 必须严格遵守】 =====\n")

	for _, skill := range matchedSkills {
		sb.WriteString(fmt.Sprintf("\n--- %s ---\n", skill.Name))
		if skill.Description != "" {
			sb.WriteString(fmt.Sprintf("描述：%s\n", skill.Description))
		}
		// Inject short directive instead of full SystemPrompt
		if skill.Directive != "" {
			sb.WriteString(fmt.Sprintf("【规则指令】\n%s\n", skill.Directive))
		}
		if skill.TriggerKeywords != "" {
			sb.WriteString(fmt.Sprintf("触发关键词：%s\n", skill.TriggerKeywords))
		}
		if skill.Examples != "" {
			sb.WriteString(fmt.Sprintf("【行为示例】\n%s\n", skill.Examples))
		}
	}
	sb.WriteString("\n===== 专业技能指令结束 =====\n")
	return sb.String()
}

func init() {
	// Auto-migrate skill_embeddings table
	// Skip in test context where db.Dao may be nil
	if db.Dao != nil {
		db.Dao.AutoMigrate(&SkillEmbedding{})
	}
}


