package agent

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"

	lru "github.com/hashicorp/golang-lru/v2/expirable"
	gorm_clause "gorm.io/gorm/clause"
)

type SkillEmbedding struct {
	ID          uint      `gorm:"primaryKey"`
	SkillID     uint      `gorm:"uniqueIndex;not null"`
	ContentHash string    `gorm:"size:64"`
	Embedding   []float32 `gorm:"serializer:json"`
}

func (SkillEmbedding) TableName() string { return "skill_embeddings" }

type SkillEmbeddingService struct {
	es      *EmbeddingService
	enabled bool
	mu      sync.RWMutex
	// skillID → embedding vector
	embeddings map[uint][]float32
	// skillID → content hash
	contentHashes map[uint]string
}

var (
	skillEmbedSvc *SkillEmbeddingService
	skillEmbedMu  sync.Mutex
)

func GetSkillEmbeddingService(es *EmbeddingService) *SkillEmbeddingService {
	enabled := es != nil && es.IsConfigured()

	skillEmbedMu.Lock()
	defer skillEmbedMu.Unlock()

	if skillEmbedSvc != nil && skillEmbedSvc.enabled == enabled {
		return skillEmbedSvc
	}

	skillEmbedSvc = &SkillEmbeddingService{
		es:            es,
		embeddings:    make(map[uint][]float32),
		contentHashes: make(map[uint]string),
	}
	skillEmbedSvc.enabled = enabled
	if enabled {
		logger.SugaredLogger.Info("SkillEmbeddingService 已启用（语义匹配）")
		if err := skillEmbedSvc.loadFromDB(); err != nil {
			logger.SugaredLogger.Warnf("从DB加载skill embedding失败: %v，将异步重新计算", err)
			go func() { _ = skillEmbedSvc.RefreshEmbeddings() }()
		} else if skillEmbedSvc.embeddingCount() == 0 {
			logger.SugaredLogger.Info("skill_embeddings 为空，后台异步初始化计算")
			go func() { _ = skillEmbedSvc.RefreshEmbeddings() }()
		} else {
			// 本地DB已存在向量，极速启动！后台增量对比刷新
			go func() { _ = skillEmbedSvc.RefreshEmbeddings() }()
		}
	} else {
		logger.SugaredLogger.Warn("EmbeddingService 未配置，SkillEmbeddingService 降级为关键词匹配")
	}
	return skillEmbedSvc
}

func (s *SkillEmbeddingService) embeddingCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.embeddings)
}



func (s *SkillEmbeddingService) loadFromDB() error {
	// Ensure the table exists before querying. The package-level init() runs
	// before db.Dao is connected, so AutoMigrate is performed lazily here.
	if db.Dao != nil {
		if err := db.Dao.AutoMigrate(&SkillEmbedding{}); err != nil {
			logger.SugaredLogger.Warnf("AutoMigrate skill_embeddings 失败: %v", err)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var rows []SkillEmbedding
	if err := db.Dao.Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		s.embeddings[row.SkillID] = row.Embedding
		s.contentHashes[row.SkillID] = row.ContentHash
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
	hashes := make(map[uint]string)
	var dirtyRows []SkillEmbedding

	for _, skill := range skills {
		txt := skillEmbedText(skill)
		hash := computeEmbeddingHash(s.es.modelName, txt)
		hashes[skill.ID] = hash

		// 增量检查：如果当前技能内容哈希与DB加载的哈希一致且向量存在，直接复用
		if s.contentHashes[skill.ID] == hash && len(s.embeddings[skill.ID]) > 0 {
			results[skill.ID] = s.embeddings[skill.ID]
			continue
		}

		wg.Add(1)
		go func(sk models.Skill, contentHash, text string) {
			defer wg.Done()
			vec, err := s.es.Embed(text)
			if err != nil {
				logger.SugaredLogger.Warnf("skill %s embedding失败: %v", sk.Name, err)
				return
			}
			mu.Lock()
			results[sk.ID] = vec
			dirtyRows = append(dirtyRows, SkillEmbedding{
				SkillID:     sk.ID,
				ContentHash: contentHash,
				Embedding:   vec,
			})
			mu.Unlock()
		}(skill, hash, txt)
	}
	wg.Wait()

	if len(results) == 0 {
		logger.SugaredLogger.Warn("所有skill embedding计算失败，技能触发将降级为关键词匹配")
		return nil
	}

	s.embeddings = results
	s.contentHashes = hashes

	// 仅批量更新变动或新增的 rows
	if len(dirtyRows) > 0 && db.Dao != nil {
		if err := db.Dao.Clauses(gorm_clause.OnConflict{
			Columns:   []gorm_clause.Column{{Name: "skill_id"}},
			DoUpdates: gorm_clause.AssignmentColumns([]string{"content_hash", "embedding"}),
		}).CreateInBatches(dirtyRows, 100).Error; err != nil {
			logger.SugaredLogger.Warnf("批量写入变动skill_embeddings失败: %v", err)
		}
		logger.SugaredLogger.Infof("SkillEmbeddingService 增量更新完成: 变动 %d 个, 总计 %d 个", len(dirtyRows), len(results))
	} else {
		logger.SugaredLogger.Debugf("SkillEmbeddingService 全部命中缓存，无需更新 (总计 %d 个)", len(results))
	}
	return nil
}

// skillEmbedText builds a compact embedding source for a skill. The full
// SystemPrompt is intentionally excluded — it can exceed the provider's token
// limit (e.g. nv-embed-v1 max 4096), which would 400 the whole refresh.
func skillEmbedText(sk models.Skill) string {
	var parts []string
	if strings.TrimSpace(sk.Name) != "" {
		parts = append(parts, sk.Name)
	}
	if strings.TrimSpace(sk.Description) != "" {
		parts = append(parts, sk.Description)
	}
	if strings.TrimSpace(sk.TriggerKeywords) != "" {
		parts = append(parts, "触发关键词："+sk.TriggerKeywords)
	}
	// P4.2: synonyms 也拼进 embedding 文本，扩义语义匹配范围。
	if strings.TrimSpace(sk.Synonyms) != "" {
		parts = append(parts, "同义词："+sk.Synonyms)
	}
	text := strings.Join(parts, "\n")
	const maxRunes = 2000
	if r := []rune(text); len(r) > maxRunes {
		text = string(r[:maxRunes])
	}
	return text
}

func (s *SkillEmbeddingService) MatchSkills(question string, topK int) []models.Skill {
	skills := data.NewSkillApi().GetEnabledSkills()
	if len(skills) == 0 {
		return nil
	}

	// Keyword matching first: instant and deterministic. Skill triggering must
	// never depend on a slow or unhealthy embedding endpoint, so the semantic
	// layer is only consulted when no keyword hits.
	if kwMatched := s.keywordMatch(skills, question, topK); len(kwMatched) > 0 {
		return kwMatched
	}

	// Semantic matching only when keywords miss and embeddings are available.
	if !s.enabled || s.es == nil {
		return nil
	}

	s.mu.RLock()
	embCount := len(s.embeddings)
	s.mu.RUnlock()
	if embCount == 0 {
		logger.SugaredLogger.Warnf("question %q 关键词未命中且无skill embedding可用（刷新失败），无技能匹配", truncateQuestion(question))
		return nil
	}

	// Embed question
	vec, err := s.es.Embed(question)
	if err != nil {
		logger.SugaredLogger.Warnf("question embedding失败，关键词未命中，无技能匹配: %v", err)
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

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

	// Sort by score desc (stable: same score keeps DB sort_order order)
	sort.SliceStable(scoredSkills, func(i, j int) bool {
		return scoredSkills[i].score > scoredSkills[j].score
	})

	if len(scoredSkills) > topK {
		scoredSkills = scoredSkills[:topK]
	}

	var matched []models.Skill
	for _, sc := range scoredSkills {
		matched = append(matched, sc.skill)
	}
	return matched
}

func truncateQuestion(q string) string {
	r := []rune(q)
	if len(r) > 40 {
		return string(r[:40]) + "…"
	}
	return q
}

func (s *SkillEmbeddingService) keywordMatch(skills []models.Skill, question string, topK int) []models.Skill {
	var matched, universal []models.Skill
	for _, skill := range skills {
		hasKeywords := skill.TriggerKeywords != "" || skill.Synonyms != ""
		if !hasKeywords {
			universal = append(universal, skill)
			continue
		}
		// P4.2: trigger_keywords ∪ synonyms 都参与匹配
		allKeywords := append(s.splitKeywords(skill.TriggerKeywords), s.splitKeywords(skill.Synonyms)...)
		for _, kw := range allKeywords {
			if kw != "" && strings.Contains(question, kw) {
				matched = append(matched, skill)
				break
			}
		}
	}
	// Universal skills (no keywords) fill remaining slots after keyword matches.
	for _, skill := range universal {
		if len(matched) >= topK {
			break
		}
		matched = append(matched, skill)
	}
	return matched
}

// splitKeywords returns the trimmed, non-empty keywords for a comma-separated
// trigger keyword string, caching results to avoid re-splitting the same
// keyword set on every question (MatchSkills is called per chat request).
//
// P3.1: 用 hashicorp/golang-lru/v2/expirable.LRU 替代无上限 map：
//   - 容量 1024（足够容纳数百个 Skill 的关键词集）
//   - TTL 30 分钟（Skill 列表变化后自动失效，无需主动清缓存）
var splitKwCache = lru.NewLRU[string, []string](1024, nil, 30*time.Minute)

func (s *SkillEmbeddingService) splitKeywords(raw string) []string {
	if cached, ok := splitKwCache.Get(raw); ok {
		return cached
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if out == nil {
		out = []string{}
	}

	splitKwCache.Add(raw, out)
	return out
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
		// Inject short directive if available, otherwise full SystemPrompt
		if skill.Directive != "" {
			sb.WriteString(fmt.Sprintf("【规则指令】\n%s\n", skill.Directive))
		} else if skill.SystemPrompt != "" {
			sb.WriteString(fmt.Sprintf("【规则指令】\n%s\n", skill.SystemPrompt))
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


