package agent

import (
	"strings"
	"testing"
)

// randomBlock builds a non-periodic block of n chars using an LCG, so exact
// consecutive duplicates only exist where we deliberately create them.
func randomBlock(n int) string {
	var b strings.Builder
	x := uint32(123456789)
	for i := 0; i < n; i++ {
		x = x*1664525 + 1013904223
		b.WriteByte(byte('a' + x%26))
	}
	return b.String()
}

func TestThinkStreamFilter_SplitAcrossDeltas(t *testing.T) {
	var f thinkStreamFilter
	chunks := []string{
		"国家推行“6张网”",
		"计划，哪些公司最受益？\n\n",
		"<think>我需要先搜索相关政策",
		"再筛选受益公司</think>\n",
		"## 结论\n\n只有1只值得关注",
	}
	var out strings.Builder
	for _, c := range chunks {
		out.WriteString(f.Clean(c))
	}
	want := "国家推行“6张网”计划，哪些公司最受益？\n\n\n## 结论\n\n只有1只值得关注"
	if got := out.String(); got != want {
		t.Fatalf("split-across-deltas mismatch:\ngot  %q\nwant %q", got, want)
	}
}

func TestThinkStreamFilter_Nested(t *testing.T) {
	var f thinkStreamFilter
	in := "<think>A<think>B<think>C</think>正文开始"
	got := f.Clean(in)
	if want := "正文开始"; got != want {
		t.Fatalf("nested mismatch: got %q want %q", got, want)
	}
}

func TestThinkStreamFilter_Unclosed(t *testing.T) {
	var f thinkStreamFilter
	got := f.Clean("<think>永远不会关闭的思考")
	if got != "" {
		t.Fatalf("unclosed should drop everything, got %q", got)
	}
	got2 := f.Clean("后续内容")
	if got2 != "" {
		t.Fatalf("still inside block after unclosed open, got %q", got2)
	}
}

func TestThinkStreamFilter_ThinkingVariant(t *testing.T) {
	var f thinkStreamFilter
	got := f.Clean("前<thinking>思考</thinking>后")
	if want := "前后"; got != want {
		t.Fatalf("thinking variant mismatch: got %q want %q", got, want)
	}
}

func TestCleanFinalContent_CollapsesDuplicateAnswer(t *testing.T) {
	answer := randomBlock(1620)
	raw := "<think>思考过程</think>" + answer + "<think>再次确认</think>" + answer
	got := cleanFinalContent(raw)
	if got != answer {
		t.Fatalf("duplicate not collapsed: got len=%d want len=%d", len(got), len(answer))
	}
}

func TestCleanFinalContent_NormalProseUntouched(t *testing.T) {
	prose := "一段正常文字，"
	got := cleanFinalContent(prose)
	if got != prose {
		t.Fatalf("normal prose changed: got %q", got)
	}
	big := randomBlock(1242)
	got2 := cleanFinalContent(big)
	if got2 != big {
		t.Fatalf("large non-duplicate prose changed: got len=%d want len=%d", len(got2), len(big))
	}
}

func TestCleanFinalContent_MixedRealistic(t *testing.T) {
	answer := randomBlock(900)
	raw := "## 分析\n\n" + answer + "<think>我确认一下数据</think>" + answer + "\n"
	want := "## 分析\n\n" + answer + "\n"
	got := cleanFinalContent(raw)
	if got != want {
		t.Fatalf("realistic mixed mismatch: got len=%d want len=%d", len(got), len(want))
	}
}

func TestCleanFinalContent_FirstCopyHasTrailingNewlines(t *testing.T) {
	// id=32 regression: MiniMax-M3 emitted the whole article twice, with the
	// first copy carrying a trailing "\n\n" that the second copy lacked.
	article := randomBlock(2310)
	raw := article + "\n\n" + article
	want := article
	got := cleanFinalContent(raw)
	if got != want {
		t.Fatalf("trailing-newline copy not collapsed: got len=%d want len=%d", len(got), len(want))
	}
}

func TestCleanFinalContent_DuplicateWithSeparatorWhitespace(t *testing.T) {
	answer := randomBlock(900)
	raw := answer + "\n\n\n" + answer
	want := answer
	got := cleanFinalContent(raw)
	if got != want {
		t.Fatalf("separator whitespace not collapsed: got len=%d want len=%d", len(got), len(want))
	}
}

func TestCleanFinalContent_DuplicateAfterThinkingPreambleAndHr(t *testing.T) {
	// id=34 regression: MiniMax-M3 emitted thinking sentences, then the report,
	// then repeated the last thinking sentence plus a "---" separator, then the
	// whole report again. The two copies sit behind separator junk that is not
	// whitespace, which the v2 (trim-tail) matcher failed to collapse.
	report := randomBlock(2350)
	preamble := "\n\n我需要先弄清楚\"六张网\"计划的具体内容，再针对性地筛选受益公司。\n\n我已了解\"六张网\"的基本框架。\n\n继续深挖细分龙头候选池：数据已基本充分，我现在直接生成最终完整分析报告。"
	sep := "\n\n---\n\n"
	junk := "数据已基本充分，我现在直接生成最终完整分析报告。" + sep
	raw := preamble + sep + report + junk + report
	want := preamble + sep + report
	got := cleanFinalContent(raw)
	if got != want {
		tail := func(s string, n int) string {
			if len(s) > n {
				s = s[len(s)-n:]
			}
			return s
		}
		t.Fatalf("preamble+hr duplicate not collapsed:\ngot  len=%d\nwant len=%d\ngot-tail  %q\nwant-tail %q", len(got), len(want), tail(got, 60), tail(want, 60))
	}
}

func TestCleanFinalContent_NonJunkGapNotCollapsed(t *testing.T) {
	// The gap between the two copies must be separator-like; real differing
	// content must never be deleted.
	answer := randomBlock(1000)
	gap := "这是中间一段完全不同的实质内容，不是思考句残渣。" + "\n\n" + randomBlock(200)
	raw := answer + gap + answer
	got := cleanFinalContent(raw)
	if got != raw {
		t.Fatalf("non-junk gap was collapsed: got len=%d want len=%d", len(got), len(raw))
	}
}
