package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"blades-stock/backend/db"
)

// ---------------------------------------------------------------------------
// 反思打分 golden 对照（第五轮）：确定性打分 vs LLM 打分，
// 看谁更能抓住「编造数据」的回答。
//
// 对照集：backend/agent/testdata/reflection_golden.json（4 条 fabricated +
// 4 条 quality），说明与部署后真实对照步骤见同目录 reflection_golden_README.md。
//
// 本机无可用的模型 API Key，所以这里用 mockGroundingScorer（一个会核对
// 数字的规则裁判）跑通对照框架。mock 证明的是「框架能测出分辨力差异」
// 和「LLM 裁判在给定证据时应有的行为」，不是真实模型的实测结果；
// 真实对照必须部署后用生产模型跑 TestReflectionGoldenComparison_Real。
// 绝不要把 mock 的结果写成「真实对照结果」。
// ---------------------------------------------------------------------------

type goldenCase struct {
	ID              string   `json:"id"`
	Kind            string   `json:"kind"` // fabricated | quality
	Question        string   `json:"question"`
	ToolsUsed       string   `json:"toolsUsed"`
	Answer          string   `json:"answer"`
	FabricatedFacts []string `json:"fabricatedFacts"`
}

type goldenSet struct {
	Version int          `json:"version"`
	Cases   []goldenCase `json:"cases"`
}

func loadReflectionGoldenCases(t *testing.T) []goldenCase {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "testdata", "reflection_golden.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden set: %v", err)
	}
	var set goldenSet
	if err := json.Unmarshal(raw, &set); err != nil {
		t.Fatalf("parse golden set: %v", err)
	}
	if len(set.Cases) == 0 {
		t.Fatal("golden set is empty")
	}
	return set.Cases
}

var goldenNumberRe = regexp.MustCompile(`\d[\d,]*\.?\d*`)

// extractNumbers 提取文本里的全部数字（去千分位逗号，仅比数字本身，
// 不比单位：回答写 "2.4万亿"、证据写 "1.82万亿" 就是两个不同的数）。
func extractNumbers(s string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, m := range goldenNumberRe.FindAllString(s, -1) {
		out[strings.ReplaceAll(m, ",", "")] = struct{}{}
	}
	return out
}

// sectionBetween 截取 prompt 里两个 "## " 标题之间的正文。
func sectionBetween(text, start, end string) string {
	i := strings.Index(text, start)
	if i < 0 {
		return ""
	}
	rest := text[i+len(start):]
	if j := strings.Index(rest, end); j >= 0 {
		rest = rest[:j]
	}
	return strings.TrimSpace(rest)
}

// mockGroundingScorer 模拟一个「会核对数字」的 LLM 裁判：只从 prompt 里
// 读出「工具证据」和「回答」两节，检查回答里的每个数字是否出现在
// 证据或问题中；每多一个无依据的数字，accuracy 扣 3 分。
// 它不知道 golden 集的答案 key——判定只来自 prompt 内容，
// 与真实 LLM 裁判看到的信息完全一致。
func mockGroundingScorer(systemPrompt, userMessage string) (string, error) {
	evidence := sectionBetween(userMessage, "## 智能体本次调用的工具", "## 智能体的回答")
	answer := sectionBetween(userMessage, "## 智能体的回答", "## 请输出 JSON")
	question := sectionBetween(userMessage, "## 原始问题", "## 智能体本次调用的工具")

	grounded := map[string]struct{}{}
	for n := range extractNumbers(evidence) {
		grounded[n] = struct{}{}
	}
	for n := range extractNumbers(question) {
		grounded[n] = struct{}{}
	}
	ungrounded := 0
	for n := range extractNumbers(answer) {
		if _, ok := grounded[n]; !ok {
			ungrounded++
		}
	}

	accuracy := 10 - 3*ungrounded
	if accuracy < 0 {
		accuracy = 0
	}
	score := 0
	relevance, toolUse, completeness := 7, 8, 8
	insight := "回答数字均有工具证据支撑"
	improvement := "保持引用工具数据的习惯"
	if ungrounded > 0 {
		relevance, toolUse, completeness = 5, 4, 6
		insight = fmt.Sprintf("回答含有 %d 个工具证据中不存在的数字，疑似编造数据", ungrounded)
		improvement = "只引用工具返回的数字；没有查到的数据要明说，不要自行补数"
	}
	switch {
	case accuracy <= 4:
		score = -1
	case accuracy >= 8:
		score = 1
	}
	out, _ := json.Marshal(map[string]any{
		"score":        score,
		"accuracy":     accuracy,
		"relevance":    relevance,
		"toolUse":      toolUse,
		"completeness": completeness,
		"insight":      insight,
		"improvement":  improvement,
	})
	return string(out), nil
}

type goldenRunResult struct {
	Case         goldenCase
	Score        int
	Accuracy     int
	Relevance    int
	ToolUse      int
	Completeness int
}

// flaggedAsBad 表示打分器把该回答判为低质量（综合负分或 accuracy 很低）。
func (r goldenRunResult) flaggedAsBad() bool {
	return r.Score < 0 || r.Accuracy <= 4
}

// runGoldenComparison 用同一套 golden 集、同一条 GenerateReflection 全路径
// 跑一遍对照。llm == nil 时走确定性打分；传 mock 或真实模型时走 LLM 打分。
// 返回每条的打分结果，并在 t.Log 输出对照表。
func runGoldenComparison(t *testing.T, label string, llm LLMReflectionFunc) []goldenRunResult {
	t.Helper()
	if err := db.Dao.AutoMigrate(&AgentTrace{}, &AgentMemory{}); err != nil {
		t.Fatalf("migrate reflection tables: %v", err)
	}
	cases := loadReflectionGoldenCases(t)
	e := NewEvaluator()
	results := make([]goldenRunResult, 0, len(cases))
	var sb strings.Builder
	fmt.Fprintf(&sb, "\n=== 反思打分 golden 对照 [%s] ===\n", label)
	fmt.Fprintf(&sb, "%-4s %-11s %5s %8s\n", "ID", "kind", "score", "accuracy")
	for _, c := range cases {
		sessionID := fmt.Sprintf("golden-%s-%s", label, c.ID)
		got, err := e.GenerateReflection(sessionID, c.Question, c.Answer, 100, llm, c.ToolsUsed)
		if err != nil {
			t.Fatalf("GenerateReflection %s: %v", c.ID, err)
		}
		r := goldenRunResult{Case: c, Score: got.Score, Accuracy: got.Accuracy, Relevance: got.Relevance, ToolUse: got.ToolUse, Completeness: got.Completeness}
		results = append(results, r)
		fmt.Fprintf(&sb, "%-4s %-11s %5d %8d\n", c.ID, c.Kind, got.Score, got.Accuracy)
	}
	t.Log(sb.String())
	return results
}

// TestReflectionGoldenComparison 是 mock 对照：确定性打分 vs mock grounding 裁判。
//
// 预期结论（断言锁死）：
//   - 确定性打分抓不住编造：4 条 fabricated 全不判低分——它对数字是加分项
//     （含 %/亿/万 会抬 accuracy），编造的数字越多分越高，与优质回答无区分。
//   - mock 裁判能抓住：4/4 条 fabricated 判低分，且 4 条 quality 全不误伤。
//
// 这证明的是「给定证据时，核对数字的裁判比关键词打分更能抓住编造」
// 以及「对照框架本身能测出这种差异」。它不证明真实模型的收益，
// 真实对照见 TestReflectionGoldenComparison_Real（部署后跑）。
func TestReflectionGoldenComparison(t *testing.T) {
	det := runGoldenComparison(t, "deterministic", nil)
	mock := runGoldenComparison(t, "mock-llm", mockGroundingScorer)

	countFlagged := func(rs []goldenRunResult, kind string) int {
		n := 0
		for _, r := range rs {
			if r.Case.Kind == kind && r.flaggedAsBad() {
				n++
			}
		}
		return n
	}
	detFab := countFlagged(det, "fabricated")
	mockFab := countFlagged(mock, "fabricated")
	mockQualityBad := countFlagged(mock, "quality")

	if detFab != 0 {
		t.Errorf("deterministic scorer should flag 0/4 fabricated (it rewards numbers), flagged %d/4", detFab)
	}
	if mockFab != 4 {
		t.Errorf("mock grounding scorer should flag 4/4 fabricated, flagged %d/4", mockFab)
	}
	if mockQualityBad != 0 {
		t.Errorf("mock grounding scorer should flag 0/4 quality answers, flagged %d/4", mockQualityBad)
	}

	// 聚合：mock 在 fabricated 上的 accuracy 应系统性低于 quality。
	avgAcc := func(rs []goldenRunResult, kind string) float64 {
		sum, n := 0, 0
		for _, r := range rs {
			if r.Case.Kind == kind {
				sum += r.Accuracy
				n++
			}
		}
		if n == 0 {
			return 0
		}
		return float64(sum) / float64(n)
	}
	if avgAcc(mock, "fabricated") >= avgAcc(mock, "quality") {
		t.Errorf("mock accuracy should separate fabricated (%.1f) from quality (%.1f)",
			avgAcc(mock, "fabricated"), avgAcc(mock, "quality"))
	}
	if avgAcc(det, "fabricated") < avgAcc(det, "quality") {
		t.Errorf("unexpected: deterministic accuracy separates fabricated (%.1f) from quality (%.1f); golden set may no longer reproduce the blind spot",
			avgAcc(det, "fabricated"), avgAcc(det, "quality"))
	}
}

// TestReflectionGoldenComparison_Real 是部署后的真实对照：同一套 golden 集，
// 用生产选卡（defaultLLMReflectionFunc，优先 DeepSeek）跑一次。
// 默认跳过（单元测试不碰外部 AI）；部署后在服务器上用：
//
//	BLADES_STOCK_REFLECTION_GOLDEN_REAL_LLM=1 go test -run 'TestReflectionGoldenComparison_Real' ./backend/agent/ -v
//
// 判定标准与 mock 对照相同：真实 LLM 应把 F1–F4 打低分且不误伤 Q1–Q4。
// 若未通过，说明真实收益未被证明，保持 1/10 采样 + 每日 20 次的受限状态，
// 不要上调采样率。步骤详见 testdata/reflection_golden_README.md。
func TestReflectionGoldenComparison_Real(t *testing.T) {
	if os.Getenv("BLADES_STOCK_REFLECTION_GOLDEN_REAL_LLM") != "1" {
		t.Skip("real-LLM golden comparison needs BLADES_STOCK_REFLECTION_GOLDEN_REAL_LLM=1 and a production model card (run post-deploy, not in unit tests)")
	}
	llm := defaultLLMReflectionFunc()
	if llm == nil {
		t.Fatal("no usable AI config for real-LLM golden comparison (need a credentialed card, DeepSeek preferred)")
	}
	results := runGoldenComparison(t, "real-llm", llm)
	fab, qualityBad := 0, 0
	for _, r := range results {
		switch r.Case.Kind {
		case "fabricated":
			if r.flaggedAsBad() {
				fab++
			}
		case "quality":
			if r.flaggedAsBad() {
				qualityBad++
			}
		}
	}
	if fab != 4 || qualityBad != 0 {
		t.Errorf("real LLM did NOT prove the benefit: flagged %d/4 fabricated, %d/4 quality as bad; keep sampled/restricted mode, do not raise the sample rate", fab, qualityBad)
	}
}
