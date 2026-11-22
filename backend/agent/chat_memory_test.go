package agent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"blades-stock/backend/db"
)

// fakeEmbedServer returns a fixed query embedding for every request so the
// semantic scoring path can be tested without a real embedding backend.
func fakeEmbedServer(t *testing.T, vec []float32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data":  []map[string]interface{}{{"embedding": vec}},
			"usage": map[string]int{"total_tokens": 3},
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func embedBytes(t *testing.T, vec []float32) []byte {
	t.Helper()
	b, err := json.Marshal(vec)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestSemanticRetrieveKeepsRecentUnembedded 验证语义召回对刚写入（尚无向量）的
// 消息不公平的修复：最新的无嵌入消息因新鲜度基线而保留，语义相关的旧消息仍排最前。
func TestSemanticRetrieveKeepsRecentUnembedded(t *testing.T) {
	srv := fakeEmbedServer(t, []float32{1, 0, 0, 0})
	es := NewEmbeddingService(srv.URL, "", "test-model")

	// memories 按时间升序（旧→新）。
	memories := []db.ChatMemory{
		{ID: 1, Role: "user", Content: "你好", Embedding: embedBytes(t, []float32{0, 1, 0, 0})},
		{ID: 2, Role: "user", Content: "茅台最近走势", Embedding: embedBytes(t, []float32{1, 0, 0, 0})},
		{ID: 3, Role: "assistant", Content: "茅台今日收于xx", Embedding: embedBytes(t, []float32{1, 0, 0, 0})},
		{ID: 4, Role: "user", Content: "谢谢", Embedding: nil},
		{ID: 5, Role: "assistant", Content: "不客气", Embedding: nil},
		{ID: 6, Role: "user", Content: "帮我看看宁德时代", Embedding: nil},
	}

	svc := NewChatMemoryService("test-session", 5) // s.maxMemory=10 -> topN=5
	svc.SetEmbeddingService(es)
	result, err := svc.semanticRetrieve(memories, "茅台最近走势")
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 5 {
		t.Fatalf("topN=5, got %d: %v", len(result), result)
	}
	got := ""
	for _, m := range result {
		got += m["content"] + "|"
	}
	for _, want := range []string{"帮我看看宁德时代", "不客气", "谢谢"} {
		if !strings.Contains(got, want) {
			t.Errorf("recent unembedded message %q dropped: %s", want, got)
		}
	}
	if strings.Contains(got, "你好") {
		t.Errorf("oldest irrelevant message should be dropped: %s", got)
	}
	if !strings.HasPrefix(got, "茅台今日收于xx|茅台最近走势|") {
		t.Errorf("semantically relevant embedded messages should rank first: %s", got)
	}
}

func TestBuildRollingSummary(t *testing.T) {
	prev := "上一轮摘要：讨论了某股基本面。"
	pairs := []string{"用户: 你好", "助手: 你好，请问需要什么帮助？", "用户: 查一下茅台", "助手: 茅台现价上涨"}
	out := buildRollingSummary(prev, pairs)
	for _, want := range []string{"上一轮摘要", "早期对话要点", "查一下茅台"} {
		if !strings.Contains(out, want) {
			t.Errorf("rolling summary missing %q: %s", want, out)
		}
	}

	big := make([]string, 60)
	for i := range big {
		big[i] = fmt.Sprintf("用户: 第%d轮内容", i)
	}
	out = buildRollingSummary("", big)
	if !strings.Contains(out, "已省略") {
		t.Errorf("expected truncation marker: %s", out)
	}
	if lines := strings.Count(out, "\n"); lines > 40 {
		t.Errorf("summary too large: %d lines", lines)
	}
}

// TestCompressHistoryWritesRollingSummary 验证压缩路径：旧消息删除、滚动摘要落库，
// 且保留的最近消息数量符合预期（异步 LLM 摘要无配置时静默跳过）。
func TestCompressHistoryWritesRollingSummary(t *testing.T) {
	sessionID := "test-compress-session"
	svc := NewChatMemoryService(sessionID, 20)
	base := time.Now()
	for i := 0; i < 40; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		if err := db.Dao.Create(&db.ChatMemory{
			SessionID: sessionID,
			Role:      role,
			Content:   fmt.Sprintf("msg-%d", i),
			CreatedAt: base.Add(time.Duration(i) * time.Second),
		}).Error; err != nil {
			t.Fatal(err)
		}
	}

	latest, err := svc.CompressHistory(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(latest) == 0 || !strings.Contains(latest[0]["content"], "早期对话要点") {
		t.Fatalf("summary message missing: %v", latest)
	}
	count, err := db.CountChatMemory(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 20 {
		t.Errorf("expected 20 messages remaining, got %d", count)
	}
	summary, err := db.GetChatMemorySummary(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(summary.Summary, "msg-0") {
		t.Errorf("summary should mention early content: %s", summary.Summary)
	}
}

// TestTruncateBytesToRuneBoundary 验证字节截断不切断多字节汉字：
// 旧实现 raw[:8000] 可能正好落在汉字中间，产生非法 UTF-8 污染 LLM prompt。
func TestTruncateBytesToRuneBoundary(t *testing.T) {
	// "茅" 3 字节：上限 5 字节时切在第二个"茅"中间，必须回退到 3 字节。
	s := "茅茅茅茅"
	got := truncateBytesToRuneBoundary(s, 5)
	if got != "茅" {
		t.Errorf("expected single char, got %q", got)
	}
	if !utf8.ValidString(got) {
		t.Errorf("result is not valid UTF-8: %q", got)
	}
	// 未超限时原样返回；空串与 0 上限返回空串。
	if got := truncateBytesToRuneBoundary("你好", 100); got != "你好" {
		t.Errorf("short string should be unchanged, got %q", got)
	}
	if got := truncateBytesToRuneBoundary("你好", 0); got != "" {
		t.Errorf("zero budget should return empty, got %q", got)
	}
	// 混合 ASCII 与汉字：切在边界上不丢合法字符。
	mixed := "ab茅c"
	if got := truncateBytesToRuneBoundary(mixed, 5); got != "ab茅" {
		t.Errorf("expected %q, got %q", "ab茅", got)
	}
}
