package agent

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"blades-stock/backend/agent/types"
	"blades-stock/backend/db"
	bladesTools "github.com/go-kratos/blades/tools"
)

func fakeTool(name string) bladesTools.Tool {
	return types.NewTool(name, "desc", map[string]*types.ParamDef{}, func(string) (string, error) { return "", nil })
}

func TestReorderToolsByRankWith(t *testing.T) {
	ts := []bladesTools.Tool{fakeTool("A"), fakeTool("B"), fakeTool("C"), fakeTool("D")}
	ranks := map[string]ToolRank{
		"B": {ToolName: "B", SuccessCount: 10, FailCount: 0},
		"A": {ToolName: "A", SuccessCount: 5, FailCount: 5},
		"C": {ToolName: "C", SuccessCount: 0, FailCount: 10},
	}
	out := reorderToolsByRankWith(ts, ranks)
	got := []string{out[0].Name(), out[1].Name(), out[2].Name(), out[3].Name()}
	want := []string{"B", "A", "C", "D"} // D 无记录，沉底且保持顺序
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
	// 原始切片不得被修改
	if ts[0].Name() != "A" {
		t.Fatalf("input slice was mutated: first=%s", ts[0].Name())
	}
}

func TestReorderToolsByRankWithEmpty(t *testing.T) {
	ts := []bladesTools.Tool{fakeTool("A"), fakeTool("B")}
	out := reorderToolsByRankWith(ts, nil)
	if len(out) != 2 || out[0].Name() != "A" {
		t.Fatalf("nil ranks should be no-op, got %v", toolNamesOf(out))
	}
}

func toolNamesOf(ts []bladesTools.Tool) []string {
	names := make([]string, 0, len(ts))
	for _, t := range ts {
		names = append(names, t.Name())
	}
	return names
}

func TestToolCallSucceeded(t *testing.T) {
	cases := []struct {
		resp string
		want bool
	}{
		{"", false},
		{" | 代码 | 名称 |", true},
		{"error: timeout", false},
		{"调用API失败: 500", false},
		{"获取失败，请稍后重试", false},
		{"数据返回正常，共 120 条", true},
	}
	for _, c := range cases {
		if got := toolCallSucceeded(c.resp); got != c.want {
			t.Errorf("toolCallSucceeded(%q) = %v, want %v", c.resp, got, c.want)
		}
	}
}

func TestShouldRunLLMReflection(t *testing.T) {
	// 第五轮新契约（约 1/10 采样 + 每日上限，见 reflection_llm_sampling.go）：
	// 旧的 ~1/3 采样 + 长回答/负分必打已被取代，负分不再走旁路。
	long := strings.Repeat("分析结论：今日行情正常，数据完整。", 40) // >500 runes
	mid := strings.Repeat("今日行情正常，数据完整。", 30)       // 150~500
	short := "今天涨了"
	if shouldRunLLMReflection(short) {
		t.Error("short answer should skip LLM reflection")
	}
	// 同一回答采样结果必须确定（无随机性）。
	a := mid + "特定后缀"
	if shouldRunLLMReflection(a) != shouldRunLLMReflection(a) {
		t.Error("sampling must be deterministic per answer")
	}
	// 中等长度答案仅按约 1/10 采样：500 条里既不能为 0 也不能全中。
	runs := 0
	for i := 0; i < 500; i++ {
		s := fmt.Sprintf("问题%d 的数据分析结果：%s", i, mid)
		if shouldRunLLMReflection(s) {
			runs++
		}
	}
	if runs < 20 || runs > 100 {
		t.Fatalf("sampling should be ~1/10, got %d/500", runs)
	}
	// 长回答也不再必打：500 条长回答里必有未入选的。
	longRuns := 0
	for i := 0; i < 500; i++ {
		s := fmt.Sprintf("长回答%d：%s", i, long)
		if shouldRunLLMReflection(s) {
			longRuns++
		}
	}
	if longRuns == 0 || longRuns >= 500 {
		t.Fatalf("long answers should also be sampled (~1/10), got %d/500", longRuns)
	}
}

func TestTextSimilarityAndExtractQuestion(t *testing.T) {
	if textSimilarity("完全相同的问题", "完全相同的问题") != 1 {
		t.Error("identical strings should be 1.0")
	}
	if textSimilarity("天目药业走势分析", "天目药业走势和资金分析") < 0.5 {
		t.Error("near-duplicate questions should be similar")
	}
	if textSimilarity("今天大盘怎么样", "贵州茅台最新财报") != 0 {
		t.Error("unrelated questions should be dissimilar")
	}

	content := "[低质量样本] 问题:天目药业走势分析 | 综合:-1 准确:3/10 | 洞察:x | 改进:y"
	if got := extractReflectionQuestion(content); got != "天目药业走势分析" {
		t.Errorf("extractReflectionQuestion = %q", got)
	}
	if got := extractReflectionQuestion("no label here"); got != "" {
		t.Errorf("extractReflectionQuestion on garbage = %q", got)
	}
}

func TestRenderAvailableTools(t *testing.T) {
	tpl := "可用工具：{{AVAILABLE_TOOLS}}。"
	out := renderAvailableTools(tpl, []string{"GetStockInfo", "GetMarketData"})
	if !strings.Contains(out, "GetStockInfo、GetMarketData") {
		t.Errorf("placeholder not replaced: %q", out)
	}
	out = renderAvailableTools(tpl, nil)
	if !strings.Contains(out, "已注入对应工具") {
		t.Errorf("nil tools should fall back gracefully: %q", out)
	}
	if got := renderAvailableTools("无占位符", nil); got != "无占位符" {
		t.Errorf("no placeholder should be unchanged: %q", got)
	}
}

// TestWriteBackToMemoryDedup 验证同类反思教训合并：相似问题只保留一条最新记录，
// 完全无关的问题互不影响。
func TestWriteBackToMemoryDedup(t *testing.T) {
	sid := "test-dedup-session"
	e := NewEvaluator()
	score := ReflectionScore{Score: -1, Accuracy: 3, Relevance: 4, ToolUse: 2, Completeness: 3}
	e.writeBackToMemory(sid, "天目药业走势分析", score, "insight", "improvement")
	e.writeBackToMemory(sid, "天目药业走势和资金分析", score, "insight2", "improvement2")
	e.writeBackToMemory(sid, "贵州茅台最新财报怎么看", score, "insight3", "improvement3")
	refs := DefaultMemoryManager().RetrieveReflections(sid, 20)
	if len(refs) != 2 {
		t.Fatalf("dedup should keep 2 reflections (2 similar merged + 1 distinct), got %d", len(refs))
	}
	merged := false
	distinct := false
	for _, r := range refs {
		if strings.Contains(r.Content, "insight2") {
			merged = true
		}
		if strings.Contains(r.Content, "insight3") {
			distinct = true
		}
	}
	if !merged || !distinct {
		t.Fatalf("merge/newest/keep-distinct semantics wrong: %v", refs)
	}
}

// TestMemoryPromptEvolutionSection 验证进化建议随长期记忆注入会话。
func TestMemoryPromptEvolutionSection(t *testing.T) {
	sid := "test-evo-session"
	_ = db.Dao.Create(&AgentMemory{
		SessionID: sid, Role: "evolution", Category: "evolution",
		Content:   "建议：多调用 GetMarketData 获取大盘数据后再下结论",
		CreatedAt: time.Now(),
	}).Error
	p := DefaultMemoryManager().MemoryPrompt(sid)
	if !strings.Contains(p, "【进化建议】") || !strings.Contains(p, "GetMarketData") {
		t.Errorf("MemoryPrompt missing evolution section: %q", p)
	}
}

// TestGenerateReflectionToolsUsed 验证工具调用轨迹写入 AgentTrace.ToolsUsed。
func TestGenerateReflectionToolsUsed(t *testing.T) {
	sid := "test-tools-session"
	_, err := NewEvaluator().GenerateReflection(sid, "测试问题", "今日行情正常，数据完整，建议继续关注。", 0, nil, "GetStockInfo、GetMarketData")
	if err != nil {
		t.Fatal(err)
	}
	var trace AgentTrace
	if err := db.Dao.Where("session_id = ?", sid).Order("id desc").First(&trace).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(trace.ToolsUsed, "GetStockInfo") {
		t.Errorf("ToolsUsed not persisted: %q", trace.ToolsUsed)
	}
}
