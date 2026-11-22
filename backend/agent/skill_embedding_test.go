package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"blades-stock/backend/db"
	"blades-stock/backend/models"
)

// =============================================================================
// P4.2: Skill 同义词扩展测试
// =============================================================================

func TestSkillSynonyms_KeywordMatch(t *testing.T) {
	s := &SkillEmbeddingService{}
	matched := s.MatchSkills("今天 ETF 有什么建议", 2)
	if len(matched) == 0 {
		t.Fatal("expected skill matched by synonym ETF")
	}
	if matched[0].ID != 1 {
		t.Errorf("expected skill ID 1, got %d", matched[0].ID)
	}
}

func TestSkillSynonyms_OnlySynonymsMatch(t *testing.T) {
	s := &SkillEmbeddingService{}
	matched := s.MatchSkills("深市 行情", 2)
	if len(matched) == 0 || matched[0].ID != 2 {
		t.Fatalf("expected skill 2 matched by synonym, got %v", matched)
	}
}

func TestSkillSynonyms_EmbeddingIncludesSynonyms(t *testing.T) {
	sk := models.Skill{
		Name:            "测试",
		Description:     "描述",
		TriggerKeywords: "ETF",
		Synonyms:        "轮动,动量",
	}
	text := skillEmbedText(sk)
	if !strings.Contains(text, "轮动") || !strings.Contains(text, "动量") {
		t.Errorf("skillEmbedText should contain synonyms, got: %s", text)
	}
}

func TestSkillSynonyms_EmptySynonymsNoEffect(t *testing.T) {
	s := &SkillEmbeddingService{}
	matched := s.MatchSkills("ETF 动量", 2)
	// 空 synonyms 不应破坏匹配，应至少匹配到有 ETF 关键词的技能
	if len(matched) == 0 {
		t.Errorf("empty synonyms should not break matching, got 0 matches")
	}
}

// =============================================================================
// splitKeywords LRU 缓存行为
// =============================================================================

func TestSplitKeywords_BasicParse(t *testing.T) {
	s := &SkillEmbeddingService{}
	got := s.splitKeywords("ETF, 轮动, 动量")
	want := []string{"ETF", "轮动", "动量"}
	if !equalStringSlices(got, want) {
		t.Errorf("splitKeywords got %v, want %v", got, want)
	}
}

func TestSplitKeywords_DropsEmpty(t *testing.T) {
	s := &SkillEmbeddingService{}
	got := s.splitKeywords(",ETF,,轮动, ,")
	want := []string{"ETF", "轮动"}
	if !equalStringSlices(got, want) {
		t.Errorf("splitKeywords should drop empty entries, got %v", got)
	}
}

func TestSplitKeywords_LRUCache(t *testing.T) {
	s := &SkillEmbeddingService{}
	first := s.splitKeywords("ETF, 轮动")
	second := s.splitKeywords("ETF, 轮动")
	if len(first) != len(second) {
		t.Errorf("cache hit should return same length, got %d vs %d", len(first), len(second))
	}

	third := s.splitKeywords("完全不同的关键词")
	if len(third) != 1 {
		t.Errorf("unexpected length for different input: %d", len(third))
	}
}

func TestSplitKeywords_NoEmptyResult(t *testing.T) {
	s := &SkillEmbeddingService{}
	got := s.splitKeywords(",,,,")
	if len(got) != 0 {
		t.Errorf("empty keywords should yield empty slice, got %v", got)
	}
}

// =============================================================================
// P3.1: skill_embeddings 原子 upsert + LRU 关键词缓存测试
// =============================================================================

func TestUpsertSkillEmbedding_Atomic(t *testing.T) {
	if err := db.Dao.Exec("CREATE TABLE IF NOT EXISTS skill_embeddings (skill_id INTEGER PRIMARY KEY, embedding BLOB)").Error; err != nil {
		t.Fatalf("create table: %v", err)
	}

	if err := db.Dao.Exec("INSERT INTO skill_embeddings (skill_id, embedding) VALUES (?, ?)", 999001, []byte{1, 2, 3}).Error; err != nil {
		t.Fatalf("first insert: %v", err)
	}

	if err := db.Dao.Exec(`INSERT INTO skill_embeddings (skill_id, embedding) VALUES (?, ?)
		ON CONFLICT(skill_id) DO UPDATE SET embedding = excluded.embedding`, 999001, []byte{4, 5, 6}).Error; err != nil {
		t.Fatalf("upsert: %v", err)
	}

	var count int64
	if err := db.Dao.Raw("SELECT count(*) FROM skill_embeddings WHERE skill_id = ?", 999001).Scan(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row after upsert, got %d", count)
	}

	var gotRaw string
	if err := db.Dao.Raw("SELECT HEX(embedding) FROM skill_embeddings WHERE skill_id = ?", 999001).Scan(&gotRaw).Error; err != nil {
		t.Fatalf("select: %v", err)
	}
	wantHex := "040506"
	if gotRaw != wantHex {
		t.Errorf("expected embedding hex %s, got %s", wantHex, gotRaw)
	}

	db.Dao.Exec("DELETE FROM skill_embeddings WHERE skill_id = ?", 999001)
}

// =============================================================================
// 辅助
// =============================================================================

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestMatchSkills_KeywordTopKEnforced 验证关键词命中也要遵守 topK：
// 旧实现只对无关键词的通用技能截断，关键词命中全部返回，
// topK=2 时 3 个技能同时命中会注入 3 份 Directive。
// （测试技能 1 与 3 都含 ETF 相关关键词，见 testing_init_test.go。）
func TestMatchSkills_KeywordTopKEnforced(t *testing.T) {
	s := &SkillEmbeddingService{}
	matched := s.MatchSkills("今天 ETF 有什么建议", 1)
	if len(matched) != 1 {
		t.Fatalf("keyword matches must be truncated to topK=1, got %d: %v", len(matched), matched)
	}
	matched2 := s.MatchSkills("今天 ETF 有什么建议", 2)
	if len(matched2) != 2 {
		t.Fatalf("expected 2 keyword matches with topK=2, got %d: %v", len(matched2), matched2)
	}
}

// TestRefreshEmbeddings_KeepsOldVectorOnFailure 验证刷新时单个技能
// 向量化失败不丢弃其旧向量：旧实现用全新 results map 整体替换，
// 失败技能直接从 s.embeddings 里消失，语义匹配永久跳过它。
// 同时验证旧哈希被保留（下次刷新仍会重试），而不是被新哈希覆盖。
func TestRefreshEmbeddings_KeepsOldVectorOnFailure(t *testing.T) {
	// 同包的 TestUpsertSkillEmbedding_Atomic 会先建一个只有
	// (skill_id, embedding) 两列的最小表，AutoMigrate 在其上加
	// id 主键列会失败：此时删表重建（该测试已跑完，无依赖）。
	if err := db.Dao.AutoMigrate(&SkillEmbedding{}); err != nil {
		if err := db.Dao.Exec("DROP TABLE IF EXISTS skill_embeddings").Error; err != nil {
			t.Fatalf("drop minimal skill_embeddings table: %v", err)
		}
		if err := db.Dao.AutoMigrate(&SkillEmbedding{}); err != nil {
			t.Fatalf("migrate skill_embeddings: %v", err)
		}
	}
	// 只有名字含“技能B”的技能 2 向量化失败，其它成功。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Input string `json:"input"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if strings.Contains(req.Input, "技能B") {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("embedding failed"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data":  []map[string]interface{}{{"embedding": []float32{0.5, 0.5}}},
			"usage": map[string]int{"total_tokens": 3},
		})
	}))
	t.Cleanup(srv.Close)

	es := NewEmbeddingService(srv.URL, "", "test-model-refresh")
	oldVec := []float32{0.9, 0.1}
	svc := &SkillEmbeddingService{
		es:            es,
		enabled:       true,
		embeddings:    map[uint][]float32{2: oldVec},
		contentHashes: map[uint]string{2: "stale-hash"},
	}
	if err := svc.RefreshEmbeddings(); err != nil {
		t.Fatalf("RefreshEmbeddings failed: %v", err)
	}
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	if got := svc.embeddings[2]; len(got) != 2 || got[0] != 0.9 {
		t.Errorf("failed skill must keep its old vector, got %v", got)
	}
	if svc.contentHashes[2] != "stale-hash" {
		t.Errorf("failed skill must keep its old hash for retry, got %q", svc.contentHashes[2])
	}
	if len(svc.embeddings[1]) == 0 || len(svc.embeddings[3]) == 0 {
		t.Errorf("healthy skills must be refreshed, got embeddings keys: %v", svc.embeddings)
	}
}

// TestSameEmbeddingEndpoint 验证单例复用按端点身份（URL+模型）判断，
// 而不是指针：调用方每次传入的都是新解析的实例，指针比较永远不等。
func TestSameEmbeddingEndpoint(t *testing.T) {
	a := NewEmbeddingService("https://api.openai.com/v1", "", "text-embedding-3-small")
	b := NewEmbeddingService("https://api.openai.com/v1/", "", "text-embedding-3-small")
	if !sameEmbeddingEndpoint(a, b) {
		t.Errorf("same URL+model should be the same endpoint")
	}
	c := NewEmbeddingService("https://api.openai.com/v1", "", "text-embedding-3-large")
	if sameEmbeddingEndpoint(a, c) {
		t.Errorf("different model should be a different endpoint")
	}
	if sameEmbeddingEndpoint(a, nil) || sameEmbeddingEndpoint(nil, a) {
		t.Errorf("nil vs non-nil should differ")
	}
	if !sameEmbeddingEndpoint(nil, nil) {
		t.Errorf("nil vs nil should be equal")
	}
}

// TestGetSkillEmbeddingService_EndpointChange 验证端点变化时单例重建，
// 而不是继续返回绑定旧端点的服务。
func TestGetSkillEmbeddingService_EndpointChange(t *testing.T) {
	skillEmbedMu.Lock()
	prev := skillEmbedSvc
	skillEmbedSvc = nil
	skillEmbedMu.Unlock()
	t.Cleanup(func() {
		skillEmbedMu.Lock()
		skillEmbedSvc = prev
		skillEmbedMu.Unlock()
	})

	es1 := NewEmbeddingService("https://api.openai.com/v1", "", "model-one")
	es2 := NewEmbeddingService("https://api.openai.com/v1", "", "model-two")
	s1 := GetSkillEmbeddingService(es1, nil)
	s2 := GetSkillEmbeddingService(es1, nil)
	if s1 != s2 {
		t.Fatalf("same endpoint should reuse the singleton")
	}
	s3 := GetSkillEmbeddingService(es2, nil)
	if s3 == s1 {
		t.Fatalf("changed model should rebuild the singleton")
	}
	if s3.es == nil || !strings.Contains(s3.es.Describe(), "model-two") {
		t.Fatalf("rebuilt service should bind the new endpoint, got %v", s3.es)
	}
}
