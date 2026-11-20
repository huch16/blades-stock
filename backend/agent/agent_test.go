package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	blades "github.com/go-kratos/blades"
	bladesTools "github.com/go-kratos/blades/tools"
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

func TestClassifyComplexity(t *testing.T) {
	tests := []struct {
		question string
		expected AgentMode
	}{
		{"今天茅台股价多少", AgentModeReact},
		{"查询一下平安银行的代码", AgentModeReact},
		{"全面分析贵州茅台的投资价值与未来成长空间和财务指标", AgentModePlanExecute},
		{"深度分析新能源汽车产业链投资机会，包括上游锂矿、中游电池、下游整车的竞争格局和投资建议", AgentModePlanExecute},
		{"【盘前自选股与持仓综合排雷诊断清单】\n自选分组：短线波段（共 5 只标的）", AgentModeDirect},
		{"【股票盘中异动归因与排雷排查】\n标的：华阳集团(002906)", AgentModeDirect},
		{"【AI 形态选股列表共振推演与操盘计划】", AgentModeDirect},
		{"【A股盘前智能前瞻与ETF轮动内参】", AgentModeDirect},
	}

	for _, tt := range tests {
		result := classifyComplexity(tt.question)
		if result != tt.expected {
			t.Errorf("question=%q expected=%s got=%s", tt.question, tt.expected, result)
		}
	}
}

// TestAgentWithMockProvider tests that the agent can be created and run with a mock model provider.
func TestAgentWithMockProvider(t *testing.T) {
	// Create a mock model provider that returns a fixed response.
	mockResp := "This is a mock response from the model."
	mockProvider := NewMockModelProvider("mock-provider", mockResp)

	// Create a simple tool (optional, but we can pass an empty slice).
	var tools []bladesTools.Tool

	// Create a blades agent with the mock provider.
	agent, err := blades.NewAgent("test-agent",
		blades.WithModel(mockProvider),
		blades.WithTools(tools...),
		blades.WithInstruction("You are a helpful assistant."),
	)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	// Create an invocation with a simple user message.
	userMsg := &blades.Message{
		Role: blades.RoleUser,
		Parts: []blades.Part{
			blades.TextPart{Text: "Hello, how are you?"},
		},
	}
	invocation := &blades.Invocation{
		ID:      "test-invocation",
		Message: userMsg,
		// We don't need to set other fields for this test.
		Streamable: false,
	}

	// Run the agent and collect the response.
	ctx := context.Background()
	msgs := make([]*blades.Message, 0)
	var runErr error
	for msg, e := range agent.Run(ctx, invocation) {
		if e != nil {
			runErr = e
			break
		}
		if msg != nil {
			msgs = append(msgs, msg)
		}
	}
	if runErr != nil {
		t.Fatalf("agent run returned an error: %v", runErr)
	}
	if len(msgs) == 0 {
		t.Fatalf("agent did not return any messages")
	}
	// Check that the last message contains the mock response.
	lastMsg := msgs[len(msgs)-1]
	if !strings.Contains(lastMsg.Text(), mockResp) {
		t.Fatalf("expected mock response '%s' in agent response, got: %s", mockResp, lastMsg.Text())
	}
}
