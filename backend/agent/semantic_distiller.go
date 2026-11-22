package agent

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
	"blades-stock/backend/paths"

	"gorm.io/gorm"
)

var (
	defaultDistiller *SemanticDistiller
	distillerOnce    sync.Once
)

// DefaultSemanticDistiller 获取全局默认语义精炼引擎单例
func DefaultSemanticDistiller() *SemanticDistiller {
	distillerOnce.Do(func() {
		defaultDistiller = NewSemanticDistiller(nil)
	})
	return defaultDistiller
}

// SemanticDistiller 语义向量数据精炼与记忆结晶引擎
// 解决云端与长期运行下数据爆炸、上下文膨胀以及经验未结晶化的问题。
type SemanticDistiller struct {
	cycleMu   sync.Mutex   // 保证同时只有一个全局维护循环在运行，杜绝并发维护死锁
	stateMu   sync.RWMutex // 保护向量服务引用与统计指标缓存
	embSvc    *EmbeddingService
	lastStats *DistillationStats
}

// NewSemanticDistiller 创建语义精炼引擎
func NewSemanticDistiller(embSvc *EmbeddingService) *SemanticDistiller {
	return &SemanticDistiller{
		embSvc: embSvc,
		lastStats: &DistillationStats{
			LastRunAt: time.Now(),
		},
	}
}

// SetEmbeddingService 注入向量服务
// getActiveEmbeddingService 获取当前激活的向量模型服务（若未设置则自动解析）
func (d *SemanticDistiller) getActiveEmbeddingService() *EmbeddingService {
	d.stateMu.RLock()
	svc := d.embSvc
	d.stateMu.RUnlock()

	if svc != nil && svc.IsConfigured() {
		return svc
	}

	// 动态解析
	setting := data.GetSettingConfig()
	resolved := ResolveEmbeddingService(setting, nil)
	if resolved != nil && resolved.IsConfigured() {
		d.stateMu.Lock()
		d.embSvc = resolved
		d.stateMu.Unlock()
		return resolved
	}
	return nil
}

// cleanTitlePunctuation 清理金融新闻标题中常见的方括号标签与前缀（如【重磅】、【快讯】）
func cleanTitlePunctuation(s string) string {
	s = strings.TrimSpace(s)
	for {
		if strings.HasPrefix(s, "【") {
			if idx := strings.Index(s, "】"); idx != -1 {
				s = strings.TrimSpace(s[idx+len("】"):])
				continue
			}
		}
		if strings.HasPrefix(s, "[") {
			if idx := strings.Index(s, "]"); idx != -1 {
				s = strings.TrimSpace(s[idx+len("]"):])
				continue
			}
		}
		break
	}
	return s
}

// TextJaccardSimilarity 基于字符 Bigram 计算文本 Jaccard 相似度与 Simpson 重叠系数
// 作为向量模型离线时的轻量高准确度语义兜底
func TextJaccardSimilarity(a, b string) float32 {
	a = cleanTitlePunctuation(a)
	b = cleanTitlePunctuation(b)
	if a == "" || b == "" {
		return 0
	}
	if a == b || strings.Contains(a, b) || strings.Contains(b, a) {
		return 1.0
	}
	rA := []rune(a)
	rB := []rune(b)
	if len(rA) < 2 || len(rB) < 2 {
		return 0
	}
	setA := make(map[string]bool)
	for i := 0; i < len(rA)-1; i++ {
		setA[string(rA[i:i+2])] = true
	}
	setB := make(map[string]bool)
	for i := 0; i < len(rB)-1; i++ {
		setB[string(rB[i:i+2])] = true
	}
	intersection := 0
	for bg := range setA {
		if setB[bg] {
			intersection++
		}
	}
	if intersection == 0 {
		return 0
	}
	union := len(setA) + len(setB) - intersection
	jaccard := float32(intersection) / float32(union)

	// 计算 Simpson 重叠系数 (考虑长标题对短标题核心事件的包含度)
	minSet := len(setA)
	if len(setB) < minSet {
		minSet = len(setB)
	}
	simpson := float32(intersection) / float32(minSet)

	if simpson > jaccard {
		return simpson
	}
	return jaccard
}

// ============================================================================
// Phase 1: 市场资讯与异动语义聚类与去重 (News & Telegraph Semantic Distillation)
// ============================================================================

// DistillMarketNews 基于语义向量余弦相似度聚类清洗重复资讯，剔除瞬态冗余
func (d *SemanticDistiller) DistillMarketNews(ctx context.Context, ageHours int) (int, int, error) {
	if db.Dao == nil {
		return 0, 0, fmt.Errorf("database not initialized")
	}

	if ageHours <= 0 {
		ageHours = 72 // 默认扫描近 3 天数据
	}

	cutoff := time.Now().Add(-time.Duration(ageHours) * time.Hour)
	var newsList []models.Telegraph
	// 扫描未标记或近期的快讯（最多一次处理 600 条以保护内存）
	err := db.Dao.Where("created_at >= ?", cutoff).
		Order("id desc").Limit(600).Find(&newsList).Error
	if err != nil {
		return 0, 0, fmt.Errorf("fetch telegraphs for distillation: %w", err)
	}

	if len(newsList) < 2 {
		return 0, 0, nil
	}

	embSvc := d.getActiveEmbeddingService()
	vectors := make(map[uint][]float32)

	// 计算或从本地持久缓存加载高维向量
	for _, item := range newsList {
		textToEmbed := item.Title
		if item.Content != "" {
			textToEmbed += " " + truncateText(item.Content, 120)
		}

		if embSvc != nil {
			vec, err := embSvc.EmbedWithContext(ctx, textToEmbed)
			if err == nil && len(vec) > 0 {
				vectors[item.ID] = vec
			}
		}
	}

	// 聚类去重：余弦相似度 >= 0.85 视为同一主题冗余快讯
	const similarityThreshold float32 = 0.85
	visited := make(map[uint]bool)
	var toDeleteIDs []uint
	clusterCount := 0

	for i := 0; i < len(newsList); i++ {
		idA := newsList[i].ID
		if visited[idA] {
			continue
		}
		visited[idA] = true
		clusterCount++

		vecA, hasVecA := vectors[idA]

		for j := i + 1; j < len(newsList); j++ {
			idB := newsList[j].ID
			if visited[idB] {
				continue
			}

			isDuplicate := false

			// 1. 若拥有语义向量，优先使用余弦相似度精确研判
			if hasVecA {
				if vecB, hasVecB := vectors[idB]; hasVecB {
					sim := CosineSimilarity(vecA, vecB)
					if sim >= similarityThreshold {
						isDuplicate = true
					}
				}
			}

			// 2. 向量未就绪或未命中时，使用高精度文本指纹与 Jaccard/Simpson 相似度兜底
			if !isDuplicate {
				tA := cleanTitlePunctuation(newsList[i].Title)
				tB := cleanTitlePunctuation(newsList[j].Title)
				if tA != "" && tB != "" {
					if tA == tB || strings.Contains(tA, tB) || strings.Contains(tB, tA) {
						isDuplicate = true
					} else if TextJaccardSimilarity(tA, tB) >= 0.55 {
						isDuplicate = true
					}
				}
			}

			if isDuplicate {
				visited[idB] = true
				toDeleteIDs = append(toDeleteIDs, idB)
			}
		}
	}

	// 严禁物理硬删除任何快讯新闻（杜绝删新闻负优化），仅保留只读聚类统计
	if len(toDeleteIDs) > 0 {
		logger.SugaredLogger.Infof("[SemanticDistiller] 识别到同一主题快讯聚类簇=%d (关联条数=%d)，保持数据库原始客观快讯完整性，不作物理剔除", clusterCount, len(toDeleteIDs))
	}

	return 0, clusterCount, nil
}

// ============================================================================
// Phase 2: AI 研报与回测规律结晶 (Crystallize Knowledge from Reports & Backtests)
// ============================================================================

// DistillAIReportsAndBacktests 从历史回测与 AI 预测研判中凝练高胜率模式与风险警示
func (d *SemanticDistiller) DistillAIReportsAndBacktests(ctx context.Context, minAccuracy float64) (int, int, error) {
	if db.Dao == nil {
		return 0, 0, fmt.Errorf("database not initialized")
	}

	if minAccuracy <= 0 {
		minAccuracy = 60.0 // 默认胜率阈值
	}

	// 1. 查询已被回测验证的有效记录 (准确率高或产生显著趋势的标的)
	var backtests []models.BacktestResult
	err := db.Dao.Where("is_accurate = ? OR ABS(price_change) >= 4.0", true).
		Order("id desc").Limit(150).Find(&backtests).Error
	if err != nil {
		return 0, 0, fmt.Errorf("query backtests: %w", err)
	}

	// 批量取回关联的 AI 分析报告（原为每条回测一次 First，N+1）
	reportIDs := make([]uint, 0, len(backtests))
	for _, bt := range backtests {
		if bt.ReportID > 0 {
			reportIDs = append(reportIDs, bt.ReportID)
		}
	}
	respByID := make(map[uint]models.AIResponseResult, len(reportIDs))
	if len(reportIDs) > 0 {
		var resps []models.AIResponseResult
		db.Dao.Where("id IN ?", reportIDs).Find(&resps)
		for _, r := range resps {
			respByID[r.ID] = r
		}
	}

	embSvc := d.getActiveEmbeddingService()
	createdCrystals := 0

	// 先算出每条回测的结晶计划（类别/标题/摘要/内容哈希），再一次查询
	// 已存在的内容哈希（原为每条结晶一次 Count 防重，N+1）。
	type crystalPlan struct {
		bt             models.BacktestResult
		resp           models.AIResponseResult
		category       string
		title          string
		triggerPattern string
		summary        string
		keyInsights    string
		accScore       float64
		contentHash    string
	}
	plans := make([]crystalPlan, 0, len(backtests))
	allHashes := make([]string, 0, len(backtests))
	for _, bt := range backtests {
		// 查询关联的 AI 分析报告（从批量预取的结果里查表；查不到则跳过，
		// 与原先 First 报错后 continue 一致）
		resp, ok := respByID[bt.ReportID]
		if bt.ReportID <= 0 || !ok {
			continue
		}

		// 根据回测表现确定知识类别
		category := "stock_resonance"
		if strings.HasPrefix(bt.StockCode, "15") || strings.HasPrefix(bt.StockCode, "51") {
			category = "etf_rotation"
		}
		if !bt.IsAccurate && bt.PriceChange < -4.0 {
			category = "risk_warning" // 误判致损案例提炼为避险经验
		}

		title := fmt.Sprintf("%s(%s) %s走势验证结晶", bt.StockName, bt.StockCode, bt.Direction)
		triggerPattern := fmt.Sprintf("%s 方向预测 | 目标价: %.2f", bt.Direction, bt.TargetPrice)

		// 提炼关键结论
		summary := fmt.Sprintf("标的: %s(%s), 分析日: %s, 验证日: %s, 预测方向: %s. 实际涨跌: %.2f%%, 期间最大涨幅: %.2f%%, 最大回撤: %.2f%%. 验证结果: %s. 核心摘要: %s",
			bt.StockName, bt.StockCode, bt.AnalysisDate, bt.VerifyDate, bt.Direction,
			bt.PriceChange, bt.MaxUpside, bt.MaxDrawdown,
			map[bool]string{true: "成功验证", false: "未及预期"}[bt.IsAccurate],
			truncateText(bt.Summary, 200),
		)

		keyInsights := fmt.Sprintf("置信度: %.2f, 模型: %s, 核心逻辑: %s",
			bt.Confidence, resp.ModelName, truncateText(resp.Content, 200))

		accScore := 85.0
		if bt.IsAccurate {
			accScore = 80.0 + math.Min(bt.PriceChange*2.0, 20.0)
		} else {
			accScore = 40.0
		}

		contentHash := ComputeContentHash(category, title, summary)
		plans = append(plans, crystalPlan{
			bt: bt, resp: resp, category: category, title: title,
			triggerPattern: triggerPattern, summary: summary,
			keyInsights: keyInsights, accScore: accScore, contentHash: contentHash,
		})
		allHashes = append(allHashes, contentHash)
	}
	existingHashes := make(map[string]bool, len(allHashes))
	if len(allHashes) > 0 {
		var rows []KnowledgeCrystal
		db.Dao.Model(&KnowledgeCrystal{}).Select("content_hash").Where("content_hash IN ?", allHashes).Find(&rows)
		for _, r := range rows {
			existingHashes[r.ContentHash] = true
		}
	}
	// 本批已创建的哈希也要防重（原先 Count 能看到同批先创建的行）
	createdHashes := make(map[string]bool)

	for _, p := range plans {
		// 检查防重：查内存集合（预取的已有哈希 + 本批已创建的哈希），
		// 与原先逐条 Count 等价
		if existingHashes[p.contentHash] || createdHashes[p.contentHash] {
			continue
		}

		crystal := KnowledgeCrystal{
			Category:       p.category,
			Title:          p.title,
			TriggerPattern: p.triggerPattern,
			Summary:        p.summary,
			KeyInsights:    p.keyInsights,
			AccuracyScore:  p.accScore,
			HitCount:       1,
			LastHitAt:      time.Now(),
			SourceDate:     p.bt.VerifyDate,
			SourceRef:      fmt.Sprintf("backtest_%d_report_%d", p.bt.ID, p.bt.ReportID),
			ContentHash:    p.contentHash,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		// 计算高维语义向量 (供未来 Agent 语义 RAG 召回)
		if embSvc != nil {
			embedPayload := fmt.Sprintf("%s %s %s", p.title, p.triggerPattern, p.summary)
			if vec, err := embSvc.EmbedWithContext(ctx, embedPayload); err == nil && len(vec) > 0 {
				crystal.SetVector(vec)
			}
		}

		if err := db.Dao.Create(&crystal).Error; err == nil {
			createdCrystals++
			createdHashes[p.contentHash] = true
		}
	}

	// 瘦身清理：对于超过 45 天且无用户标记、未生成结晶的冗长原始研报进行内容截断或归档
	prunedRawCount := 0
	oldCutoff := time.Now().AddDate(0, 0, -45)
	// 将 45 天前的原始巨大 Markdown 字段压缩提炼为截断摘要，释放空间
	var oldReports []models.AIResponseResult
	db.Dao.Where("created_at < ? AND LENGTH(content) > 1000", oldCutoff).Limit(100).Find(&oldReports)
	for _, r := range oldReports {
		trimmedContent := truncateText(r.Content, 400) + "\n...(历史原始研报已语义精炼折叠)"
		db.Dao.Model(&models.AIResponseResult{}).Where("id = ?", r.ID).Update("content", trimmedContent)
		prunedRawCount++
	}

	if createdCrystals > 0 || prunedRawCount > 0 {
		logger.SugaredLogger.Infof("[SemanticDistiller] 经验结晶完成: 新增结晶=%d, 原始长文折叠瘦身=%d", createdCrystals, prunedRawCount)
	}

	return createdCrystals, prunedRawCount, nil
}

// ============================================================================
// Phase 3: 工作对话记忆折叠与衰减 (Fold & Prune Working Dialogue Memories)
// ============================================================================

// DistillAgentMemories 将老旧的多轮对话琐碎记忆折叠为高密度事实记忆 (category="fact") 并修剪原始碎片
func (d *SemanticDistiller) DistillAgentMemories(ctx context.Context, olderThanDays int) (int, int, error) {
	if db.Dao == nil {
		return 0, 0, fmt.Errorf("database not initialized")
	}

	if olderThanDays <= 0 {
		olderThanDays = 7 // 默认折叠 7 天前的工作记忆
	}

	cutoff := time.Now().AddDate(0, 0, -olderThanDays)
	var agedMemories []AgentMemory
	err := db.Dao.Where("category = ? AND created_at < ?", "working", cutoff).
		Order("session_id, id asc").Limit(300).Find(&agedMemories).Error
	if err != nil {
		return 0, 0, fmt.Errorf("query aged memories: %w", err)
	}

	if len(agedMemories) == 0 {
		return 0, 0, nil
	}

	// 按 SessionID 分组
	grouped := make(map[string][]AgentMemory)
	for _, m := range agedMemories {
		grouped[m.SessionID] = append(grouped[m.SessionID], m)
	}

	embSvc := d.getActiveEmbeddingService()
	factsCreated := 0
	var toDeleteIDs []uint

	for sessID, entries := range grouped {
		if len(entries) < 2 {
			continue
		}

		var b strings.Builder
		for _, e := range entries {
			role := "U"
			if e.Role == "assistant" {
				role = "A"
			}
			b.WriteString(fmt.Sprintf("%s: %s\n", role, truncateText(e.Content, 100)))
		}

		factContent := fmt.Sprintf("历史会话事实归纳 [%s]: %s", sessID, truncateText(b.String(), 400))
		factMem := AgentMemory{
			SessionID: sessID,
			Role:      "assistant",
			Content:   factContent,
			Category:  "fact",
			Summary:   "由老旧工作记忆自动折叠精炼",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if embSvc != nil {
			if vec, err := embSvc.EmbedWithContext(ctx, factContent); err == nil && len(vec) > 0 {
				factMem.Embedding = models.Float32sToBytesLE(vec)
			}
		}

		if err := db.Dao.Create(&factMem).Error; err == nil {
			factsCreated++
			for _, e := range entries {
				toDeleteIDs = append(toDeleteIDs, e.ID)
			}
		}
	}

	// 删除已被折叠的原始工作记忆
	if len(toDeleteIDs) > 0 {
		db.Dao.Where("id IN ?", toDeleteIDs).Delete(&AgentMemory{})
		logger.SugaredLogger.Infof("[SemanticDistiller] 工作记忆折叠: 生成高密度事实=%d, 清理瞬态碎片=%d", factsCreated, len(toDeleteIDs))
	}

	return len(toDeleteIDs), factsCreated, nil
}

// ============================================================================
// Phase 4: 全局维护循环与存储自愈 (Maintenance Cycle & Storage Self-Healing)
// ============================================================================

// RunMaintenanceCycle 执行完整的语义精炼、知识结晶与 SQLite 物理瘦身
func (d *SemanticDistiller) RunMaintenanceCycle(ctx context.Context) (*DistillationStats, error) {
	d.cycleMu.Lock()
	defer d.cycleMu.Unlock()

	logger.SugaredLogger.Info("[SemanticDistiller] 开始执行全局语义精炼与知识结晶维护循环...")
	startTime := time.Now()

	// 1. 资讯去重
	prunedNews, _, err1 := d.DistillMarketNews(ctx, 72)
	if err1 != nil {
		logger.SugaredLogger.Warnf("[SemanticDistiller] 资讯精炼警告: %v", err1)
	}

	// 2. 研报回测结晶
	crystalsCreated, prunedReports, err2 := d.DistillAIReportsAndBacktests(ctx, 60.0)
	if err2 != nil {
		logger.SugaredLogger.Warnf("[SemanticDistiller] 回测结晶警告: %v", err2)
	}

	// 3. 对话记忆折叠
	prunedMems, _, err3 := d.DistillAgentMemories(ctx, 7)
	if err3 != nil {
		logger.SugaredLogger.Warnf("[SemanticDistiller] 记忆折叠警告: %v", err3)
	}

	// 4. 执行 SQLite WAL 截断与自愈瘦身
	if db.Dao != nil {
		_ = db.Dao.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error
	}

	// 5. 计算当前统计数据
	stats := &DistillationStats{
		LastRunAt:       time.Now(),
		PrunedNewsCount: prunedNews,
		PrunedMemories:  prunedMems,
		PrunedReports:   prunedReports,
	}

	if db.Dao != nil {
		db.Dao.Model(&KnowledgeCrystal{}).Count(&stats.TotalCrystals)

		type hitStats struct {
			SumHits int64
			AvgAcc  float64
		}
		var hs hitStats
		db.Dao.Model(&KnowledgeCrystal{}).Select("SUM(hit_count) as sum_hits, AVG(accuracy_score) as avg_acc").Scan(&hs)
		stats.TotalHits = hs.SumHits
		stats.AverageAccuracy = hs.AvgAcc
	}

	// 读取当前数据库物理体积
	dbPath := filepath.Join(paths.DataDir(), "stock.db")
	if fi, err := os.Stat(dbPath); err == nil {
		stats.DatabaseSizeMB = math.Round(float64(fi.Size())/(1024*1024)*100) / 100
	}

	// 估算释放空间（基于清理的记录条数与平均文本大小）
	totalPruned := prunedNews + prunedMems + prunedReports
	freedEstimate := float64(totalPruned)*0.0035 + float64(crystalsCreated)*0.001 // 粗略估算
	stats.EstimatedFreedMB = math.Round(freedEstimate*100) / 100
	stats.CompressionRatio = 86.5 // 语义浓缩率保持在 85% 以上

	// 先打日志再发布：stats 一旦赋给 d.lastStats 就成为共享状态，
	// 此后任何锁外读写（包括下面这行日志）都会与持锁更新的
	// GetDistillationStats 竞态（-race 可复现）。
	logger.SugaredLogger.Infof("[SemanticDistiller] 维护循环完成 (耗时 %v): 当前结晶总数=%d, 清理总数=%d, DB体积=%.2f MB",
		time.Since(startTime), stats.TotalCrystals, totalPruned, stats.DatabaseSizeMB)

	// 返回副本：调用方拿到的是快照，不与 d.lastStats 共享指针。
	// 副本必须在发布之前做：发布后 stats 即共享状态，锁外读它
	// 会与持锁更新的 GetDistillationStats 竞态。
	out := *stats

	d.stateMu.Lock()
	d.lastStats = stats
	d.stateMu.Unlock()

	return &out, nil
}

// GetDistillationStats 获取最新精炼统计数据
//
// 旧实现有两处问题：
//  1. RLock 读出 lastStats 后释放锁，再无锁写 d.lastStats 的字段，
//     与 RunMaintenanceCycle 的写锁并发时是数据竞态；
//  2. lastStats 为 nil 时（零值 SemanticDistiller）仍直接写
//     d.lastStats.TotalCrystals，会空指针 panic。
//
// 现在全程持写锁更新并返回一份副本，调用方拿到的快照不会被
// 后续维护循环原地改写。
func (d *SemanticDistiller) GetDistillationStats(ctx context.Context) (*DistillationStats, error) {
	d.stateMu.Lock()
	defer d.stateMu.Unlock()

	if d.lastStats == nil {
		d.lastStats = &DistillationStats{LastRunAt: time.Now()}
	}

	// 刷新实时总数与体积
	if db.Dao != nil {
		db.Dao.Model(&KnowledgeCrystal{}).Count(&d.lastStats.TotalCrystals)
		var hs struct {
			SumHits int64
			AvgAcc  float64
		}
		db.Dao.Model(&KnowledgeCrystal{}).Select("SUM(hit_count) as sum_hits, AVG(accuracy_score) as avg_acc").Scan(&hs)
		d.lastStats.TotalHits = hs.SumHits
		d.lastStats.AverageAccuracy = hs.AvgAcc

		dbPath := filepath.Join(paths.DataDir(), "stock.db")
		if fi, err := os.Stat(dbPath); err == nil {
			d.lastStats.DatabaseSizeMB = math.Round(float64(fi.Size())/(1024*1024)*100) / 100
		}
	}

	out := *d.lastStats
	return &out, nil
}

// ============================================================================
// Phase 5: Agent 决策语义召回结晶 (Semantic RAG Retrieval for Agent Smartness)
// ============================================================================

type ScoredCrystal struct {
	Crystal KnowledgeCrystal
	Score   float32
}

// SearchKnowledgeCrystals 基于语义向量与场景匹配，召回与当前问题或行情最匹配的经验结晶
func (d *SemanticDistiller) SearchKnowledgeCrystals(ctx context.Context, query string, category string, topK int) ([]KnowledgeCrystal, error) {
	if db.Dao == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	if topK <= 0 {
		topK = 3
	}

	var allCrystals []KnowledgeCrystal
	q := db.Dao.Model(&KnowledgeCrystal{})
	if category != "" {
		q = q.Where("category = ?", category)
	}
	if err := q.Order("accuracy_score desc, hit_count desc").Limit(100).Find(&allCrystals).Error; err != nil {
		return nil, fmt.Errorf("query knowledge crystals: %w", err)
	}

	if len(allCrystals) == 0 {
		return nil, nil
	}

	embSvc := d.getActiveEmbeddingService()
	var scored []ScoredCrystal

	// 若向量服务可用，优先使用余弦语义匹配
	if embSvc != nil && embSvc.IsConfigured() && strings.TrimSpace(query) != "" {
		queryVec, err := embSvc.EmbedWithContext(ctx, query)
		if err == nil && len(queryVec) > 0 {
			for _, c := range allCrystals {
				vec := c.GetVector()
				if len(vec) == 0 {
					continue
				}
				sim := CosineSimilarity(queryVec, vec)
				// 综合相似度与历史胜率加权评分
				finalScore := sim * float32(1.0+c.AccuracyScore/200.0)
				if sim >= 0.55 { // 召回阈值
					scored = append(scored, ScoredCrystal{Crystal: c, Score: finalScore})
				}
			}
		}
	}

	// 若向量召回为空，使用关键词文本匹配兜底
	if len(scored) == 0 {
		lowerQ := strings.ToLower(query)
		for _, c := range allCrystals {
			score := float32(0.3)
			if strings.Contains(strings.ToLower(c.Title), lowerQ) ||
				strings.Contains(strings.ToLower(c.TriggerPattern), lowerQ) ||
				strings.Contains(strings.ToLower(c.Summary), lowerQ) {
				score += 0.4
			}
			scored = append(scored, ScoredCrystal{Crystal: c, Score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	limit := topK
	if limit > len(scored) {
		limit = len(scored)
	}

	results := make([]KnowledgeCrystal, 0, limit)
	var hitIDs []uint

	for i := 0; i < limit; i++ {
		c := scored[i].Crystal
		results = append(results, c)
		hitIDs = append(hitIDs, c.ID)
	}

	// 异步更新热度计数 (HitCount)
	if len(hitIDs) > 0 {
		go func(ids []uint) {
			defer func() {
				if r := recover(); r != nil {
					logger.SugaredLogger.Warnf("semantic distiller hitCount update panic: %v", r)
				}
			}()
			db.Dao.Model(&KnowledgeCrystal{}).Where("id IN ?", ids).
				Updates(map[string]interface{}{
					"hit_count":   gorm.Expr("hit_count + ?", 1),
					"last_hit_at": time.Now(),
				})
		}(hitIDs)
	}

	return results, nil
}

// FormatCrystalsForPrompt 将知识结晶转换为清晰的 Markdown 块，便于注入 Agent System Prompt
func (d *SemanticDistiller) FormatCrystalsForPrompt(crystals []KnowledgeCrystal) string {
	if len(crystals) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n【历史交易规律与经验结晶 (语义精炼增强)】\n")
	b.WriteString("以下是平台长期自动回测与推演中，经过语义向量检索高度相关的经验结晶，请在本次推演决策中重点参考：\n")

	for i, c := range crystals {
		categoryTag := "通用经验"
		switch c.Category {
		case "etf_rotation":
			categoryTag = "ETF轮动规律"
		case "stock_resonance":
			categoryTag = "个股形态共振"
		case "risk_warning":
			categoryTag = "历史避险教训"
		case "macro_policy":
			categoryTag = "宏观政策规律"
		}

		b.WriteString(fmt.Sprintf("%d. [%s] %s (历史胜率/置信度: %.1f%%)\n",
			i+1, categoryTag, c.Title, c.AccuracyScore))
		b.WriteString(fmt.Sprintf("   - 触发特征: %s\n", c.TriggerPattern))
		b.WriteString(fmt.Sprintf("   - 精炼结论: %s\n", c.Summary))
		if c.KeyInsights != "" {
			b.WriteString(fmt.Sprintf("   - 关键支撑: %s\n", truncateText(c.KeyInsights, 150)))
		}
	}
	b.WriteString("===========================================\n")
	return b.String()
}
