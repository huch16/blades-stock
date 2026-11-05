package agent

import (
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
