package agent

import (
	"fmt"
	"strings"
	"testing"
)

// Pure unit tests for the agent's content-compression and classification
// helpers. They exercise local code only and do not require a DB, network,
// or external AI service.

func TestCollapseLongMarkdownTableRunsIsolation(t *testing.T) {
	run := make([]string, 0, 22)
	for i := 0; i < 22; i++ {
		run = append(run, fmt.Sprintf("| col | %02d |", i))
	}
	out := collapseLongMarkdownTableRuns(run)
	if !strings.Contains(strings.Join(out, "\n"), "省略") {
		t.Fatalf("isolated collapse failed: %q", strings.Join(out, "\n"))
	}
}

func TestCollapseLongMarkdownTableRuns(t *testing.T) {
	var b strings.Builder
	_, _ = b.WriteString("前言\n")
	for i := 0; i < 22; i++ {
		_, _ = fmt.Fprintf(&b, "| col | %02d |\n", i)
	}
	_, _ = b.WriteString("后记\n")
	content := b.String()
	norm := normalizeCompressLines(content)
	if !strings.Contains(strings.Join(norm, "\n"), "省略") {
		t.Fatalf("normalize should collapse table: lines=%d body=%q", len(norm), strings.Join(norm, "\n"))
	}
	out := smartContentCompress(content, 900)
	if !strings.Contains(out, "省略") || !strings.Contains(out, "markdown") {
		t.Fatalf("expected long markdown table collapsed: %q", out)
	}
	if !strings.Contains(out, "| col | 21 |") {
		t.Fatalf("expected tail table rows preserved: %q", out)
	}
}

func TestSupplementaryFactLineKeepsStockDigits(t *testing.T) {
	parts := make([]string, 0, 37)
	for i := 0; i < 18; i++ {
		parts = append(parts, "叙述性占位句子不含百分号。")
	}
	parts = append(parts, "重点观测标的代码 600519 仅供单元测试")
	for i := 0; i < 18; i++ {
		parts = append(parts, "叙述性占位句子不含百分号。")
	}
	content := strings.Join(parts, "\n")
	out := smartContentCompress(content, 420)
	if !strings.Contains(out, "600519") {
		t.Fatalf("expected supplementary fact line retained under tight budget: %q", out)
	}
}

func TestSmartContentCompressDedupeAndTail(t *testing.T) {
	repeat := strings.Repeat("重复行\n", 24)
	tail := "尾部结论：综上所述建议持有\n"
	data := "成交额 123.45 亿元\n"
	content := repeat + data + tail
	out := smartContentCompress(content, 120)
	if strings.Count(out, "重复行") > 1 {
		t.Fatalf("expected consecutive duplicate lines collapsed: %q", out)
	}
	if !strings.Contains(out, "综上所述") {
		t.Fatalf("expected tail summary kept: %q", out)
	}
}

func TestCompressExecutedStepResult(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 100; i++ {
		_, _ = fmt.Fprintf(&b, "第%03d段 ROE=%.1f%% 营收同比说明文字占位\n", i, float64(i%13)+0.5)
	}
	line := b.String()
	old := compressExecutedStepResult(line, 0, 8, false)
	recent := compressExecutedStepResult(line, 7, 8, false)
	if len(old) >= len(recent) {
		t.Fatalf("expected older step (smaller byte budget) more compressed: old=%d recent=%d", len(old), len(recent))
	}
	if len(recent) < 200 {
		t.Fatalf("expected recent step to retain meaningful content: len=%d", len(recent))
	}
}

func TestClassifyComplexity(t *testing.T) {
	tests := []struct {
		question string
		expected AgentMode
	}{
		{"今天茅台股价多少", AgentModeReact},
		{"查询一下平安银行的代码", AgentModeReact},
		{"全面分析贵州茅台的投资价值", AgentModePlanExecute},
		{"综合分析当前市场热点和投资机会", AgentModePlanExecute},
		{"帮我查一下今天大盘行情", AgentModeReact},
		{"深度分析新能源汽车产业链投资机会，包括上游锂矿、中游电池、下游整车的竞争格局和投资建议", AgentModePlanExecute},
	}

	for _, tt := range tests {
		result := classifyComplexity(tt.question)
		status := "✓"
		if result != tt.expected {
			status = "✗"
		}
		t.Logf("%s question=%q expected=%s got=%s", status, tt.question, tt.expected, result)
	}
}
