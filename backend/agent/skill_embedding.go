package agent

import (
	"context"
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
	// refreshMu 串行化 RefreshEmbeddings：同一服务并发刷新时，后一次
	// 应等待前一次完成再基于最新快照计算，而不是交错读写 results。
	refreshMu sync.Mutex
	// skillID → embedding vector
	embeddings map[uint][]float32
	// skillID → content hash
	contentHashes map[uint]string
}

var (
	skillEmbedSvc *SkillEmbeddingService
	skillEmbedMu  sync.Mutex
)

// GetSkillEmbeddingService 返回单例技能语义匹配服务。
//
// retr 是为未来 RAG 检索器预留的扩展缝，当前恒传 nil（唯一调用方
// agent.go:377）且有意忽略：仓库内 Retriever 没有任何实现，
// EmbeddingService 也只有 EmbedText 而无 Search/IndexDocuments 等
// 方法，matchSkillsViaRetriever 及其三处回退分支在生产上不可达，
// 已于第四轮删除。签名保留 retr 是因为 agent.go 只读不改；将来
// 真接通检索器时再把参数接回结构体。retriever.go 的 Retriever
// 接口与 Document 类型作为预留扩展缝保留，见其文件头注释。
func GetSkillEmbeddingService(es *EmbeddingService, retr Retriever) *SkillEmbeddingService {
	_ = retr // 预留参数，当前无实现，不参与匹配
	enabled := es != nil && es.IsConfigured()

	skillEmbedMu.Lock()
	defer skillEmbedMu.Unlock()

	// 单例复用的条件除 enabled 外还必须端点一致：调用方每次传入的
	// *EmbeddingService 都是新解析的实例（指针必不同），只比指针会
	// 在模型/地址变化时继续返回绑定旧端点的服务，导致语义匹配
	// 永远走旧模型；反过来只比 enabled 又会在端点变化时不刷新。
	// 按端点身份（URL+模型）比较，相同则复用，不同则重建。
	if skillEmbedSvc != nil && skillEmbedSvc.enabled == enabled &&
		sameEmbeddingEndpoint(skillEmbedSvc.es, es) {
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
			// 捕获本次创建的实例而非全局 skillEmbedSvc：goroutine
			// 启动后全局可能被下一次调用或测试重建，读全局是竞态。
			svc := skillEmbedSvc
			go func() {
				defer func() {
					if r := recover(); r != nil {
						logger.SugaredLogger.Errorf("RefreshEmbeddings panic: %v", r)
					}
				}()
				_ = svc.RefreshEmbeddings()
			}()
		} else if skillEmbedSvc.embeddingCount() == 0 {
			logger.SugaredLogger.Info("skill_embeddings 为空，后台异步初始化计算")
			// 捕获本次创建的实例而非全局 skillEmbedSvc：goroutine
			// 启动后全局可能被下一次调用或测试重建，读全局是竞态。
			svc := skillEmbedSvc
			go func() {
				defer func() {
					if r := recover(); r != nil {
						logger.SugaredLogger.Errorf("RefreshEmbeddings panic: %v", r)
					}
				}()
				_ = svc.RefreshEmbeddings()
			}()
		} else {
			// 本地DB已存在向量，极速启动！后台增量对比刷新
			// 捕获本次创建的实例而非全局 skillEmbedSvc：goroutine
			// 启动后全局可能被下一次调用或测试重建，读全局是竞态。
			svc := skillEmbedSvc
			go func() {
				defer func() {
					if r := recover(); r != nil {
						logger.SugaredLogger.Errorf("RefreshEmbeddings panic: %v", r)
					}
				}()
				_ = svc.RefreshEmbeddings()
			}()
		}
	} else {
		logger.SugaredLogger.Warn("EmbeddingService 未配置，SkillEmbeddingService 降级为关键词匹配")
	}
	return skillEmbedSvc
}

// sameEmbeddingEndpoint 比较两个 embedding 服务是否指向同一端点
// （URL+模型）。Describe 不含密钥，适合做身份比较；指针比较不可用，
// 因为调用方每次传入的都是新解析的实例。
func sameEmbeddingEndpoint(a, b *EmbeddingService) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Describe() == b.Describe()
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
	// 串行化并发刷新，避免两次刷新交错读写同一批 results。
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	// 只在快照/安装两个瞬间持 s.mu：旧实现全程持写锁并在锁内
	// wg.Wait() 等网络 Embed 返回，期间 MatchSkills 的 RLock
	// 全部阻塞，一次刷新可卡住每次问答的技能匹配。
	// 此处先快照旧向量与旧哈希，网络计算全程无锁，最后再上锁安装。
	s.mu.RLock()
	oldEmbeddings := make(map[uint][]float32, len(s.embeddings))
	for id, vec := range s.embeddings {
		oldEmbeddings[id] = vec
	}
	oldHashes := make(map[uint]string, len(s.contentHashes))
	for id, h := range s.contentHashes {
		oldHashes[id] = h
	}
	s.mu.RUnlock()

	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make(map[uint][]float32)
	hashes := make(map[uint]string)
	var dirtyRows []SkillEmbedding
	var failedIDs []uint

	for _, skill := range skills {
		txt := skillEmbedText(skill)
		hash := computeEmbeddingHash(s.es.modelName, txt)
		hashes[skill.ID] = hash

		// 增量检查：如果当前技能内容哈希与DB加载的哈希一致且向量存在，直接复用
		// （results 由 goroutine 并发写，必须同样持 mu，避免与主循环竞态）
		if oldHashes[skill.ID] == hash && len(oldEmbeddings[skill.ID]) > 0 {
			mu.Lock()
			results[skill.ID] = oldEmbeddings[skill.ID]
			mu.Unlock()
			continue
		}

		wg.Add(1)
		go func(sk models.Skill, contentHash, text string) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					logger.SugaredLogger.Errorf("skill %s embedding panic: %v", sk.Name, r)
				}
			}()
			vec, err := s.es.Embed(text)
			if err != nil {
				logger.SugaredLogger.Warnf("skill %s embedding失败: %v", sk.Name, err)
				mu.Lock()
				failedIDs = append(failedIDs, sk.ID)
				mu.Unlock()
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

	// 失败技能保留旧向量与旧哈希：旧实现直接丢弃该技能，换新后
	// s.embeddings 里再无此技能，语义匹配永久跳过它，直到下一次
	// 全量刷新；旧哈希则保证下次刷新仍会重试本次失败的重新计算。
	// 注意：回填在 wg.Wait() 之后由主 goroutine 执行，此时再无
	// 并发写 hashes/results，避免与主循环的写入竞态。
	for _, id := range failedIDs {
		if old, ok := oldEmbeddings[id]; ok && len(old) > 0 {
			results[id] = old
			hashes[id] = oldHashes[id]
		}
	}

	if len(results) == 0 {
		logger.SugaredLogger.Warn("所有skill embedding计算失败，技能触发将降级为关键词匹配")
		return nil
	}

	s.mu.Lock()
	s.embeddings = results
	s.contentHashes = hashes
	s.mu.Unlock()

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

func (s *SkillEmbeddingService) MatchSkills(question string, topK int) (matched []models.Skill) {
	skills := data.NewSkillApi().GetEnabledSkills()
	if len(skills) == 0 {
		return nil
	}

	// Keyword matching first: instant and deterministic. Skill triggering must
	// never depend on a slow or unhealthy embedding endpoint, so the semantic
	// layer is only consulted when no keyword hits.
	if kwMatched := s.keywordMatch(skills, question, topK); len(kwMatched) > 0 {
		// keywordMatch 只对无关键词的通用技能做了 topK 截断，关键词
		// 命中本身未截断：3 个技能同时命中时 topK=2 的约定会被打破，
		// 多余技能的 Directive 全量注入 prompt。此处统一截断。
		if topK > 0 && len(kwMatched) > topK {
			kwMatched = kwMatched[:topK]
		}
		return kwMatched
	}

	// Semantic matching via internal skill embeddings.
	// Retriever telemetry: record hit/miss + latency for the vector search
	// (keyword hits above are deterministic and do not go through
	// retrieval). The defer also covers the "embeddings unavailable /
	// embed failed" early returns — those are genuine retrieval misses.
	semanticStart := time.Now()
	defer func() {
		RecordRetrieverHit(context.Background(), "skill_embedding", len(matched) > 0, time.Since(semanticStart))
	}()
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
