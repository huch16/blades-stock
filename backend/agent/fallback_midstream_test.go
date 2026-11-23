// ── 中途断流降级回归测试（2026-11-22）─────────────────────────────────────
//
// 事故复现：DeepSeek-V4.1-Flash 经 TokenLab 中继跑 Serenity 产业深度拆解，
// 生成中途流被重置（HTTP/2 stream error，85 秒，0 字），备用链未触发。
// 根因：blades 的 OpenAI 适配层对每个 SSE chunk 都 yield 一条消息，首包
// role chunk 与 reasoning-only chunk 都是零 part 的空消息——旧判定
// `!yieldedAny` 在模型还没出一个正文词时就已被满足，于是中途出错走了
// “已吐内容，直接透传”分支。
//
// 新规则：能否降级看“是否已产出实质正文”（hasSubstantiveContent），
// 而不是“是否 yield 过任何 chunk”。
package agent

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-kratos/blades"
)

// collectFallbackStream 消费一次 FallbackModelProvider 流式调用，
// 返回累计正文（Message.Text() 拼接）与流错误。
func collectFallbackStream(gen blades.Generator[*blades.ModelResponse, error]) (string, error) {
	var sb strings.Builder
	var streamErr error
	gen(func(resp *blades.ModelResponse, err error) bool {
		if err != nil {
			streamErr = err
			return false
		}
		if resp != nil && resp.Message != nil {
			sb.WriteString(resp.Message.Text())
		}
		return true
	})
	return sb.String(), streamErr
}

// TestFallbackMidStreamErrorAfterEmptyChunks 事故本体重现：
// 主模型只 yield 过空消息（首包 role chunk + reasoning delta）后流重置，
// 必须降级到备用模型，而不是把错误直接透传。
func TestFallbackMidStreamErrorAfterEmptyChunks(t *testing.T) {
	streamErr := errors.New("stream error: stream ID 1; INTERNAL_ERROR")
	primary := NewScriptedStreamProvider("deepseek-v4.1-flash", []MockStreamStep{
		{Kind: MockStreamEmpty}, // 首包 role chunk
		{Kind: MockStreamEmpty}, // reasoning delta（blades 不映射 reasoning_content）
		{Kind: MockStreamEmpty},
		{Kind: MockStreamError, Err: streamErr},
	})
	fallback := &successMockProvider{name: "nemotron-3", respText: "fallback report"}
	var notifiedCur, notifiedNext string
	var notifiedErr error
	ctx := WithFallbackNotifier(context.Background(), func(cur, next string, err error) {
		notifiedCur, notifiedNext, notifiedErr = cur, next, err
	})
	fb := NewFallbackModelProvider(primary, "deepseek-v4.1-flash",
		[]blades.ModelProvider{fallback}, []string{"nemotron-3"})

	text, err := collectFallbackStream(fb.NewStreaming(ctx, &blades.ModelRequest{
		Messages: []*blades.Message{blades.UserMessage("test")},
	}))
	if err != nil {
		t.Fatalf("空 chunk 后中途断流应降级成功，实际仍返回错误: %v", err)
	}
	if text != "fallback report" {
		t.Fatalf("期望备用模型内容，实际: %q", text)
	}
	if atomic.LoadInt32(&fallback.callCount) != 1 {
		t.Fatalf("备用模型应被调用 1 次，实际 %d 次", fallback.callCount)
	}
	if notifiedCur != "deepseek-v4.1-flash" || notifiedNext != "nemotron-3" {
		t.Fatalf("降级通知错误: cur=%s next=%s", notifiedCur, notifiedNext)
	}
	if notifiedErr == nil || !strings.Contains(notifiedErr.Error(), "stream error") {
		t.Fatalf("降级通知应携带原始流错误，实际: %v", notifiedErr)
	}
}

// TestFallbackMidStreamErrorAfterThinkOnly 只有思考内容（<think> 块）
// 后断流：思考对用户不可见，不算实质正文，必须降级。
func TestFallbackMidStreamErrorAfterThinkOnly(t *testing.T) {
	streamErr := errors.New("stream error: stream ID 1; INTERNAL_ERROR")
	primary := NewScriptedStreamProvider("deepseek-v4.1-flash", []MockStreamStep{
		{Kind: MockStreamThink, Text: "先分析产业链八层结构，再识别稀缺环节……"},
		{Kind: MockStreamThink, Text: "继续思考供需格局与验证周期……"},
		{Kind: MockStreamError, Err: streamErr},
	})
	fallback := &successMockProvider{name: "nemotron-3", respText: "fallback report"}
	ctx := context.Background()
	fb := NewFallbackModelProvider(primary, "deepseek-v4.1-flash",
		[]blades.ModelProvider{fallback}, []string{"nemotron-3"})

	text, err := collectFallbackStream(fb.NewStreaming(ctx, &blades.ModelRequest{
		Messages: []*blades.Message{blades.UserMessage("test")},
	}))
	if err != nil {
		t.Fatalf("仅思考内容后断流应降级成功，实际仍返回错误: %v", err)
	}
	if !strings.Contains(text, "fallback report") {
		t.Fatalf("期望备用模型内容，实际: %q", text)
	}
	if atomic.LoadInt32(&fallback.callCount) != 1 {
		t.Fatalf("备用模型应被调用 1 次，实际 %d 次", fallback.callCount)
	}
}

// TestFallbackMidStreamErrorAfterSubstantiveContent 已有实质正文后断流：
// 绝不能静默降级拼接两个模型的部分输出——错误原样上抛，已产出的部分
// 正文必须原样到达调用方，备用模型绝不能被调用。
func TestFallbackMidStreamErrorAfterSubstantiveContent(t *testing.T) {
	streamErr := errors.New("stream error: stream ID 1; INTERNAL_ERROR")
	primary := NewScriptedStreamProvider("deepseek-v4.1-flash", []MockStreamStep{
		{Kind: MockStreamText, Text: "## 产业链拆解\n"},
		{Kind: MockStreamText, Text: "第一部分内容"},
		{Kind: MockStreamError, Err: streamErr},
	})
	fallback := &successMockProvider{name: "nemotron-3", respText: "SHOULD NOT APPEAR"}
	fb := NewFallbackModelProvider(primary, "deepseek-v4.1-flash",
		[]blades.ModelProvider{fallback}, []string{"nemotron-3"})

	text, err := collectFallbackStream(fb.NewStreaming(context.Background(), &blades.ModelRequest{
		Messages: []*blades.Message{blades.UserMessage("test")},
	}))
	if err == nil {
		t.Fatal("已有实质正文后断流应透传错误，实际 err 为 nil")
	}
	if !strings.Contains(err.Error(), "stream error") {
		t.Fatalf("透传的应是原始流错误，实际: %v", err)
	}
	if atomic.LoadInt32(&fallback.callCount) != 0 {
		t.Fatal("已有实质正文时备用模型绝不能被调用（会拼接两段输出）")
	}
	if !strings.Contains(text, "第一部分内容") {
		t.Fatalf("已产出的部分正文必须原样到达调用方，实际: %q", text)
	}
	if strings.Contains(text, "SHOULD NOT APPEAR") {
		t.Fatal("备用模型内容绝不能拼进已输出的部分正文")
	}
}

// TestFallbackMidStreamErrorAfterToolCall 已有工具调用后断流：
// 重跑会重复执行工具（部分工具有副作用），必须透传错误、不降级。
func TestFallbackMidStreamErrorAfterToolCall(t *testing.T) {
	streamErr := errors.New("stream error: stream ID 1; INTERNAL_ERROR")
	primary := NewScriptedStreamProvider("deepseek-v4.1-flash", []MockStreamStep{
		{Kind: MockStreamTool, Name: "GetStockKLine"},
		{Kind: MockStreamError, Err: streamErr},
	})
	fallback := &successMockProvider{name: "nemotron-3", respText: "SHOULD NOT APPEAR"}
	fb := NewFallbackModelProvider(primary, "deepseek-v4.1-flash",
		[]blades.ModelProvider{fallback}, []string{"nemotron-3"})

	_, err := collectFallbackStream(fb.NewStreaming(context.Background(), &blades.ModelRequest{
		Messages: []*blades.Message{blades.UserMessage("test")},
	}))
	if err == nil {
		t.Fatal("工具调用后断流应透传错误，实际 err 为 nil")
	}
	if atomic.LoadInt32(&fallback.callCount) != 0 {
		t.Fatal("工具调用后断流时备用模型绝不能被调用（会重复执行工具）")
	}
}

// TestFallbackEmptyChunksEndNormally 只有空消息、流正常结束：
// 仍是空答，必须降级（覆盖零 part 消息走空答分支的路径）。
func TestFallbackEmptyChunksEndNormally(t *testing.T) {
	primary := NewScriptedStreamProvider("deepseek-v4.1-flash", []MockStreamStep{
		{Kind: MockStreamEmpty},
		{Kind: MockStreamEmpty},
	})
	fallback := &successMockProvider{name: "nemotron-3", respText: "fallback report"}
	fb := NewFallbackModelProvider(primary, "deepseek-v4.1-flash",
		[]blades.ModelProvider{fallback}, []string{"nemotron-3"})

	text, err := collectFallbackStream(fb.NewStreaming(context.Background(), &blades.ModelRequest{
		Messages: []*blades.Message{blades.UserMessage("test")},
	}))
	if err != nil {
		t.Fatalf("空消息正常结束应降级成功，实际返回错误: %v", err)
	}
	if text != "fallback report" {
		t.Fatalf("期望备用模型内容，实际: %q", text)
	}
}

// TestFallbackPureThinkLongStreamEndNormally 纯 think 长流、正常结束：
// think 体对用户不可见，应判空答降级。回归用例：旧终态检查用
// cleanFinalContent，其“挽救”路径会在原始文本 >100 字时把 think 体
// 当作正文，导致长 think 流被误判为“有正文”而不降级。
func TestFallbackPureThinkLongStreamEndNormally(t *testing.T) {
	longThink := "首先分析产业链的八层结构与稀缺环节，再评估供需格局、验证周期与政策变量，" +
		"接着对照历史可比案例的估值区间，最后综合判断龙头公司的竞争壁垒是否仍然成立。"
	primary := NewScriptedStreamProvider("minimax-m3", []MockStreamStep{
		{Kind: MockStreamThink, Text: longThink},
		{Kind: MockStreamThink, Text: longThink},
		{Kind: MockStreamThink, Text: longThink},
	})
	fallback := &successMockProvider{name: "nemotron-3", respText: "fallback report"}
	fb := NewFallbackModelProvider(primary, "minimax-m3",
		[]blades.ModelProvider{fallback}, []string{"nemotron-3"})

	text, err := collectFallbackStream(fb.NewStreaming(context.Background(), &blades.ModelRequest{
		Messages: []*blades.Message{blades.UserMessage("test")},
	}))
	if err != nil {
		t.Fatalf("纯 think 长流正常结束应降级成功，实际返回错误: %v", err)
	}
	// 注意：主模型已吐出的 think 块在终态判定前已下发给消费方，
	// collectFallbackStream 拼接后会含 think 文本，故只断言备用内容出现。
	if !strings.Contains(text, "fallback report") {
		t.Fatalf("期望备用模型内容，实际: %q", text)
	}
	if atomic.LoadInt32(&fallback.callCount) != 1 {
		t.Fatalf("备用模型应被调用 1 次，实际 %d 次", fallback.callCount)
	}
}

// TestFallbackStreamStallTriggersFallback 流静默挂起（零字节、无错误）：
// 看门狗应在阈值内主动取消本次尝试并降级，而不是等到 300 秒请求超时。
func TestFallbackStreamStallTriggersFallback(t *testing.T) {
	oldFirst, oldStall := fallbackFirstChunkTimeout, fallbackStreamStallTimeout
	fallbackFirstChunkTimeout = 50 * time.Millisecond
	fallbackStreamStallTimeout = 50 * time.Millisecond
	defer func() {
		fallbackFirstChunkTimeout, fallbackStreamStallTimeout = oldFirst, oldStall
	}()

	primary := NewStallStreamProvider("stall-model")
	fallback := &successMockProvider{name: "nemotron-3", respText: "fallback after stall"}
	var notifiedCur, notifiedNext string
	var notifiedErr error
	ctx := WithFallbackNotifier(context.Background(), func(cur, next string, err error) {
		notifiedCur, notifiedNext, notifiedErr = cur, next, err
	})
	fb := NewFallbackModelProvider(primary, "stall-model",
		[]blades.ModelProvider{fallback}, []string{"nemotron-3"})

	start := time.Now()
	text, err := collectFallbackStream(fb.NewStreaming(ctx, &blades.ModelRequest{
		Messages: []*blades.Message{blades.UserMessage("test")},
	}))
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("流停滞应触发降级成功，实际返回错误: %v", err)
	}
	if text != "fallback after stall" {
		t.Fatalf("期望备用模型内容，实际: %q", text)
	}
	if atomic.LoadInt32(&fallback.callCount) != 1 {
		t.Fatalf("备用模型应被调用 1 次，实际 %d 次", fallback.callCount)
	}
	if notifiedCur != "stall-model" || notifiedNext != "nemotron-3" {
		t.Fatalf("降级通知错误: cur=%s next=%s", notifiedCur, notifiedNext)
	}
	if notifiedErr == nil || !strings.Contains(notifiedErr.Error(), "停滞") {
		t.Fatalf("降级通知应携带停滞错误，实际: %v", notifiedErr)
	}
	if elapsed > 10*time.Second {
		t.Fatalf("停滞降级应在阈值附近完成，实际耗时 %v（疑似等到了请求超时）", elapsed)
	}
}

// TestHasSubstantiveContent 锁定“实质正文”的判定边界：
// 思考体（无论闭合与否）与空白都不算正文；think 块之后的真实文本才算。
func TestHasSubstantiveContent(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{"", false},
		{"   \n\t ", false},
		{"<think>先分析产业链八层结构</think>", false},
		{"<think>未闭合的思考", false},
		{"## 产业链拆解\n正文内容", true},
		{"<think>思考</think>\n\n正文内容", true},
		{"```agent-action\n{\"action\":\"x\"}\n```", false},
	}
	for _, c := range cases {
		if got := hasSubstantiveContent(c.raw); got != c.want {
			t.Errorf("hasSubstantiveContent(%q) = %v, want %v", c.raw, got, c.want)
		}
	}
}
