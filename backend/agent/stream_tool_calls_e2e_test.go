package agent

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"blades-stock/backend/agent/types"
)

// tool_calls 端到端流式测试（mock provider 接缝，见 run_context.go
// WithMockAgent 与 mock_model_provider.go ToolCallStreamProvider）。
//
// 背景：第三轮只把 tool_calls 字段接到了 StreamMessage（转换 +
// 序列化单测见 stream_tool_calls_test.go），端到端一直缺接缝——
// ChatWithContext 里的 blades Agent 只能由真实模型驱动，
// “mock 模型 → 工具执行 → 流里出现 tool_calls”无从在单测里验证。
//
// 本测试用 WithMockAgent 把脚本化 provider + 桩工具注入一次真实的
// ChatWithContext 全流程：runOnce 流循环、[STEP] 文本、tool_calls
// 结构化消息、终态护栏、运行记录全部走生产路径；
// 唯一被替换的是模型本身（mock）与工具集（桩工具），
// 全程不碰网络、真实模型与生产数据库（临时库见 testing_init_test.go）。
//
// 生产形态口径：首轮流以终态 RoleTool/StatusCompleted + FinishReason
// "tool_calls" 结束（与 blades/contrib/openai 的 choiceToResponse 一致），
// blades 跳过该终态的透传、改由 executeTools 真正执行工具后再 yield
// 带 Response 的工具消息——因此流里应**恰好出现一次** tool_calls，
// 且桩工具**恰好被执行一次**；若 mock 形态不对（如预填 Response 的
// 零状态工具消息），同一调用会被透传与执行各发一次，
// 本测试的“恰好一次”断言会立刻抓住。

// TestChatWithContextToolCallsEndToEnd 断言：一次带工具调用的完整流里，
// tool_calls 结构化消息恰好出现一次，且名字/参数/状态/结果摘要
// 与桩工具的真实执行结果一致，随后是最终正文与终态。
func TestChatWithContextToolCallsEndToEnd(t *testing.T) {
	// 哑卡片：mock 接缝使 createChatModel 永不执行，BaseUrl 指向哪里都
	// 不会真的建连；复用 agent_stream_terminal_test.go 的种子 helper。
	cfgID := unreachableConfig(t)

	var stubCalls int32
	stub := types.NewTool("MockEchoTool",
		"测试桩工具：返回固定行情 JSON，不碰网络/DB",
		map[string]*types.ParamDef{
			"code": {Type: "string", Desc: "股票代码", Required: true},
		},
		func(args string) (string, error) {
			atomic.AddInt32(&stubCalls, 1)
			return `{"code":"000001","price":12.22}`, nil
		},
	)

	provider := NewToolCallStreamProvider(
		"mock-toolcall",
		"MockEchoTool",
		"call_e2e_001",
		`{"code":"000001"}`,
		"正在查询行情…",
		"根据工具返回的数据，000001 平安银行现价 12.22 元。",
	)

	ctx, cancel := context.WithTimeout(
		WithMockAgent(context.Background(), provider, stub), 60*time.Second)
	defer cancel()

	// 问题内容对脚本化 provider 没有意义；用 "ping" 让问题分类只剩
	// base 组，并行预取（需 ≥2 个非 base 组）保持关闭。
	ch := NewStockAiAgentApi().ChatWithContext(ctx, "ping", int(cfgID), nil, false, 0, false, "react")

	received, closed := drainUntilClosed(t, ch, 50*time.Second)
	if !closed {
		t.Fatalf("流在 50s 内未关闭，已收到 %d 条消息", len(received))
	}
	if len(received) == 0 {
		t.Fatal("未收到任何消息")
	}

	// ── 1. tool_calls 恰好出现一次，且内容与真实执行一致 ──
	var toolCallMsgs []*StreamMessage
	var toolCallIdx = -1
	for i, m := range received {
		if m != nil && len(m.ToolCalls) > 0 {
			toolCallMsgs = append(toolCallMsgs, m)
			if toolCallIdx == -1 {
				toolCallIdx = i
			}
		}
	}
	if len(toolCallMsgs) != 1 {
		t.Fatalf("tool_calls 消息应恰好 1 条，实际 %d 条（形态不对时透传+执行会各发一次）", len(toolCallMsgs))
	}
	tc := toolCallMsgs[0].ToolCalls[0]
	if len(toolCallMsgs[0].ToolCalls) != 1 {
		t.Fatalf("tool_calls 应只含 1 次调用，实际 %d 次", len(toolCallMsgs[0].ToolCalls))
	}
	if tc.Function.Name != "MockEchoTool" {
		t.Errorf("tool_calls function.name 应为 MockEchoTool，实际 %q", tc.Function.Name)
	}
	if tc.Function.Arguments != `{"code":"000001"}` {
		t.Errorf("tool_calls function.arguments 应为请求原样，实际 %q", tc.Function.Arguments)
	}
	if tc.Status != "success" {
		t.Errorf("桩工具返回正常结果，tool_calls status 应为 success，实际 %q", tc.Status)
	}
	if !strings.Contains(tc.Result, "12.22") {
		t.Errorf("tool_calls result 应含工具返回的结果摘要，实际 %q", tc.Result)
	}

	// ── 2. 桩工具恰好被执行一次；provider 恰好跑了两轮 ──
	if got := atomic.LoadInt32(&stubCalls); got != 1 {
		t.Errorf("桩工具应恰好被执行 1 次，实际 %d 次", got)
	}
	if got := provider.Calls(); got != 2 {
		t.Errorf("provider 流式调用应为 2 次（工具轮 + 正文轮），实际 %d 次", got)
	}

	// ── 3. [STEP] 文本与 tool_calls 是同一调用的两面 ──
	var sawStep bool
	for _, m := range received {
		if m != nil && strings.Contains(m.ReasoningContent, "[STEP]🔧 调用工具：MockEchoTool") {
			sawStep = true
			break
		}
	}
	if !sawStep {
		t.Error("流里缺少 [STEP] 调用工具文本——tool_calls 与思考区过程记录应同时出现")
	}

	// ── 4. 顺序：tool_calls → 最终正文 → 终态；终态指标记 1 次调用 ──
	answerIdx := -1
	for i, m := range received {
		if m != nil && strings.Contains(m.Content, "平安银行现价 12.22") {
			answerIdx = i
			break
		}
	}
	if answerIdx == -1 {
		t.Fatalf("流里缺少最终正文（应含“平安银行现价 12.22”）")
	}
	if !(toolCallIdx < answerIdx) {
		t.Errorf("tool_calls（下标 %d）应在最终正文（下标 %d）之前", toolCallIdx, answerIdx)
	}
	last := received[len(received)-1]
	if last.FinishReason != "stop" || last.Content != "agent-DONE" {
		t.Fatalf("最后一条应为终态 agent-DONE/stop，实际 content=%q finish_reason=%q",
			last.Content, last.FinishReason)
	}
	if !(answerIdx < len(received)-1) {
		t.Errorf("最终正文应在终态之前")
	}
	if last.Metrics == nil {
		t.Error("终态消息应带 Metrics")
	} else {
		if last.Metrics.ToolCallCount != 1 {
			t.Errorf("终态 Metrics.ToolCallCount 应为 1，实际 %d", last.Metrics.ToolCallCount)
		}
		if last.Metrics.FinalStatus != "success" {
			t.Errorf("终态 Metrics.FinalStatus 应为 success，实际 %q", last.Metrics.FinalStatus)
		}
	}

	// ── 5. 全程无错误正文 ──
	for _, m := range received {
		if m != nil && strings.Contains(m.Content, "❌") {
			t.Errorf("流里不应出现错误正文，实际 %q", m.Content)
		}
	}
}
