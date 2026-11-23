package agent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"blades-stock/backend/db"
	"blades-stock/backend/models"
)

// TestMemoryManager_AddTrimsPerSession 验证每个会话的工作记忆
// 有独立上限：旧实现只有全局裁剪，单个会话可独占全部槽位，
// 其它会话的记忆会被挤出（starvation，与 Add 的注释不符）。
func TestMemoryManager_AddTrimsPerSession(t *testing.T) {
	if err := db.Dao.AutoMigrate(&AgentMemory{}); err != nil {
		t.Fatalf("migrate agent_memories: %v", err)
	}
	m := NewMemoryManager(3)
	defer m.Close()
	for i := 0; i < 5; i++ {
		m.Add("session-a", "user", "消息 A")
	}
	for i := 0; i < 2; i++ {
		m.Add("session-b", "user", "消息 B")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	countBySession := map[string]int{}
	for _, e := range m.workingMem {
		countBySession[e.SessionID]++
	}
	if n := countBySession["session-a"]; n != 3 {
		t.Errorf("session-a should be trimmed to maxWorking=3, got %d", n)
	}
	if n := countBySession["session-b"]; n != 2 {
		t.Errorf("session-b should be untouched (2 entries), got %d", n)
	}
}

// TestMemoryManager_CloseIdempotent 验证 Close 调两次不 panic：
// 旧实现直接 close(stopCh)，第二次调用会 panic。
func TestMemoryManager_CloseIdempotent(t *testing.T) {
	m := NewMemoryManager(2)
	m.Close()
	m.Close() // must not panic
}

// TestSemanticSearchMemory_UsesStoredVectors 验证语义检索优先用
// 记忆行里已存的向量：旧实现对每条记忆都重新调一次 Embed
// （N 条记忆 = N 次 embedding 调用），且服务抖动时会丢记忆。
// 假服务对记忆正文返回 500，只对查询文本返回向量。
func TestSemanticSearchMemory_UsesStoredVectors(t *testing.T) {
	if err := db.Dao.AutoMigrate(&AgentMemory{}); err != nil {
		t.Fatalf("migrate agent_memories: %v", err)
	}
	const sessionID = "test-sem-search-stored-vec"
	t.Cleanup(func() {
		db.Dao.Where("session_id = ?", sessionID).Delete(&AgentMemory{})
	})
	rows := []AgentMemory{
		{SessionID: sessionID, Role: "user", Content: "茅台股价上涨", Category: "fact",
			Embedding: models.Float32sToBytesLE([]float32{1, 0})},
		{SessionID: sessionID, Role: "user", Content: "债券利率下行", Category: "fact",
			Embedding: models.Float32sToBytesLE([]float32{0, 1})},
	}
	for _, r := range rows {
		if err := db.Dao.Create(&r).Error; err != nil {
			t.Fatalf("insert memory: %v", err)
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Input string `json:"input"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		// 记忆正文的 Embed 一律失败：若实现仍逐条重算，会丢记忆。
		if strings.Contains(req.Input, "茅台") || strings.Contains(req.Input, "债券") {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("embedding failed"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data":  []map[string]interface{}{{"embedding": []float32{1, 0}}},
			"usage": map[string]int{"total_tokens": 1},
		})
	}))
	t.Cleanup(srv.Close)

	m := NewMemoryManager(5)
	defer m.Close()
	m.SetEmbeddingService(NewEmbeddingService(srv.URL, "", "test-model-semsearch"))
	results, err := m.SemanticSearchMemory(sessionID, "白酒龙头走势", "", 2)
	if err != nil {
		t.Fatalf("SemanticSearchMemory failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results from stored vectors, got %d", len(results))
	}
	if results[0].Content != "茅台股价上涨" {
		t.Errorf("most similar memory should rank first, got %q", results[0].Content)
	}
}

// TestSemanticSearchMemory_CandidateCap 回归测试：语义检索一次最多拉
// 200 条候选。旧实现 Find 无上限，250 条无存量向量的记忆会触发
// 251 次 embedding 调用（1 次查问句 + 250 次候选）；修复后应为
// 201 次（1 + 200）。
func TestSemanticSearchMemory_CandidateCap(t *testing.T) {
	if err := db.Dao.AutoMigrate(&AgentMemory{}); err != nil {
		t.Fatalf("migrate agent_memories: %v", err)
	}
	sessionID := "batch8-semantic-cap-session"
	// 清理本会话旧数据（同包其它测试共用临时库）。
	db.Dao.Where("session_id = ?", sessionID).Delete(&AgentMemory{})

	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data":  []map[string]interface{}{{"embedding": []float32{1, 0}}},
			"usage": map[string]int{"total_tokens": 1},
		})
	}))
	t.Cleanup(srv.Close)

	// 250 条无存量向量的记忆，内容各不相同（避免 LRU/DB 缓存命中）。
	rows := make([]AgentMemory, 0, 250)
	for i := 0; i < 250; i++ {
		rows = append(rows, AgentMemory{
			SessionID: sessionID,
			Role:      "user",
			Content:   fmt.Sprintf("batch8-semantic-cap-candidate-%03d", i),
		})
	}
	if err := db.Dao.CreateInBatches(rows, 100).Error; err != nil {
		t.Fatalf("seed memories: %v", err)
	}

	m := NewMemoryManager(20)
	defer m.Close()
	// 唯一模型名，避免命中其它测试留下的缓存向量。
	m.SetEmbeddingService(NewEmbeddingService(srv.URL, "", "batch8-semantic-cap-model"))

	results, err := m.SemanticSearchMemory(sessionID, "batch8-semantic-cap-query", "", 5)
	if err != nil {
		t.Fatalf("SemanticSearchMemory: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("expected top 5 results, got %d", len(results))
	}
	// 1 次查问句 + 200 条候选；旧实现无上限会是 1 + 250 = 251。
	if got := atomic.LoadInt32(&hits); got != 201 {
		t.Errorf("expected 201 embedding calls (1 query + 200 candidates), got %d", got)
	}
}
