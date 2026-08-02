package agent

import (
	"strings"
	"testing"

	"blades-stock/backend/agent/tools"
)

func TestParseMCPArgs(t *testing.T) {
	if got := parseMCPArgs(""); got != nil {
		t.Errorf("parseMCPArgs(\"\") = %v, want nil", got)
	}
	got := parseMCPArgs(`["a","b"]`)
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("parseMCPArgs valid = %v, want [a b]", got)
	}
	if got := parseMCPArgs("{invalid"); got != nil {
		t.Errorf("parseMCPArgs invalid = %v, want nil", got)
	}
}

func TestParseMCPEnv(t *testing.T) {
	if got := parseMCPEnv(""); got != nil {
		t.Errorf("parseMCPEnv(\"\") = %v, want nil", got)
	}
	got := parseMCPEnv(`{"K":"V"}`)
	if got["K"] != "V" {
		t.Errorf("parseMCPEnv valid = %v, want map[K:V]", got)
	}
	if got := parseMCPEnv("[1,2]"); got != nil {
		t.Errorf("parseMCPEnv wrong type = %v, want nil", got)
	}
}

func TestParseMCPHeaders(t *testing.T) {
	if got := parseMCPHeaders(""); got != nil {
		t.Errorf("parseMCPHeaders(\"\") = %v, want nil", got)
	}
	got := parseMCPHeaders(`{"Authorization":"Bearer x"}`)
	if got["Authorization"] != "Bearer x" {
		t.Errorf("parseMCPHeaders valid = %v", got)
	}
	if got := parseMCPHeaders("nope"); got != nil {
		t.Errorf("parseMCPHeaders invalid = %v, want nil", got)
	}
}

func TestSafeTruncateString(t *testing.T) {
	if got := safeTruncateString("hello", 100); got != "hello" {
		t.Errorf("safeTruncateString short = %q", got)
	}
	if got := safeTruncateString("hello world", 5); !strings.HasPrefix(got, "hello") || !strings.Contains(got, "已截断") {
		t.Errorf("safeTruncateString long = %q, want truncated marker", got)
	}
	// UTF-8 mid-rune boundary: truncate right after a multi-byte rune start
	cjk := "中文内容占位"
	got := safeTruncateString(cjk, 6)
	if !strings.Contains(got, "已截断") {
		t.Errorf("safeTruncateString mid-rune = %q, want truncated marker", got)
	}
}

func TestIsMarkdownTableRow(t *testing.T) {
	if !isMarkdownTableRow("| a | b |") {
		t.Error("isMarkdownTableRow(| a | b |) = false, want true")
	}
	if isMarkdownTableRow("just text") {
		t.Error("isMarkdownTableRow(plain) = true, want false")
	}
	if isMarkdownTableRow("|") {
		t.Error("isMarkdownTableRow(single pipe) = true, want false")
	}
}

func TestIsASCIIDigit(t *testing.T) {
	if !isASCIIDigit('5') {
		t.Error("isASCIIDigit('5') = false")
	}
	if isASCIIDigit('x') {
		t.Error("isASCIIDigit('x') = true")
	}
}

func TestContainsApproxISODate(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		{"报告日期 2026-01-05 发布", true},
		{"日期 20260105 无分隔符", false},
		{"没有日期信息", false},
	}
	for _, c := range cases {
		if got := containsApproxISODate(c.s); got != c.want {
			t.Errorf("containsApproxISODate(%q) = %v, want %v", c.s, got, c.want)
		}
	}
}

func TestMaxConsecutiveDigitRun(t *testing.T) {
	cases := []struct {
		s    string
		want int
	}{
		{"", 0},
		{"abc", 0},
		{"600519", 6},
		{"abc12345xyz", 5},
		{"1a22a333", 3},
	}
	for _, c := range cases {
		if got := maxConsecutiveDigitRun(c.s); got != c.want {
			t.Errorf("maxConsecutiveDigitRun(%q) = %d, want %d", c.s, got, c.want)
		}
	}
}

func TestIsSkippableNoiseLine(t *testing.T) {
	noise := []string{
		"```go",
		"```",
		"---",
		"----------",
		"***",
		"___",
		"|-----|----|",
	}
	for _, s := range noise {
		if !isSkippableNoiseLine(s) {
			t.Errorf("isSkippableNoiseLine(%q) = false, want true", s)
		}
	}
	kept := []string{
		"普通说明文字",
		"| 表格行 | 保留 |",
		"包含 ------ 的文本",
	}
	for _, s := range kept {
		if isSkippableNoiseLine(s) {
			t.Errorf("isSkippableNoiseLine(%q) = true, want false", s)
		}
	}
}

func TestIsHeaderLine(t *testing.T) {
	if !isHeaderLine("二、基本面分析") {
		t.Error("isHeaderLine(基本面分析) = false, want true")
	}
	if !isHeaderLine("总结：该股估值偏高") {
		t.Error("isHeaderLine(总结...) = false, want true")
	}
	if isHeaderLine("普通叙述行") {
		t.Error("isHeaderLine(plain) = true, want false")
	}
}

func TestIsDataLine(t *testing.T) {
	if !isDataLine("净利润同比增长 12.5%") {
		t.Error("isDataLine(percent) = false, want true")
	}
	if !isDataLine("营收 150 亿元") {
		t.Error("isDataLine(亿元) = false, want true")
	}
	if !isDataLine("2018 2019 2020 2021 2022") {
		t.Error("isDataLine(multi-space+numbers) = false, want true")
	}
	if isDataLine("这是一句普通描述") {
		t.Error("isDataLine(plain) = true, want false")
	}
}

func TestIsSummaryLine(t *testing.T) {
	for _, s := range []string{"综上所述，建议关注", "整体来看风险可控", "总体判断：谨慎"} {
		if !isSummaryLine(s) {
			t.Errorf("isSummaryLine(%q) = false, want true", s)
		}
	}
	if isSummaryLine("具体技术指标如下") {
		t.Error("isSummaryLine(plain) = true, want false")
	}
}

func TestContainsNumbers(t *testing.T) {
	if !containsNumbers("abc123") {
		t.Error("containsNumbers(abc123) = false")
	}
	if containsNumbers("无数字") {
		t.Error("containsNumbers(无数字) = true")
	}
}

func TestNormalizeCompressLines(t *testing.T) {
	content := "  首行  \n\n首行\n---\n---\n| a | b |\n| c | d |\n尾行"
	out := normalizeCompressLines(content)
	if len(out) == 0 {
		t.Fatal("normalizeCompressLines returned empty")
	}
	if strings.TrimSpace(out[0]) != "首行" {
		t.Errorf("first line = %q, want 首行", out[0])
	}
	if strings.Contains(strings.Join(out, "\n"), "---") {
		t.Errorf("noise line --- should be dropped: %v", out)
	}
}

func TestGroupNames(t *testing.T) {
	m := map[tools.ToolGroup]bool{
		tools.GroupBase:          true,
		tools.GroupStockAnalysis: true,
	}
	names := groupNames(m)
	if len(names) != 2 {
		t.Errorf("groupNames = %v, want 2 entries", names)
	}
	joined := strings.Join(names, ",")
	if !strings.Contains(joined, string(tools.GroupBase)) ||
		!strings.Contains(joined, string(tools.GroupStockAnalysis)) {
		t.Errorf("groupNames = %v, missing expected groups", names)
	}
}

func TestPersonalityPrompt(t *testing.T) {
	p := personalityPrompt()
	if strings.TrimSpace(p) == "" {
		t.Error("personalityPrompt returned empty string")
	}
}

func TestGetAllToolsNonNil(t *testing.T) {
	tools_ := GetAllTools()
	if tools_ == nil {
		t.Fatal("GetAllTools returned nil")
	}
}

func TestBuildSkillPromptForTestNoPanic(t *testing.T) {
	q := "测试问题：今天大盘怎么样？"
	// No skills exist in the empty test DB, so the result is "" — the invariant
	// here is that the DB-driven path does not panic.
	_ = BuildSkillPromptForTest(q)
}
