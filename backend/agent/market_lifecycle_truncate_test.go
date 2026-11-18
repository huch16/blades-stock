package agent

import (
	"strings"
	"testing"
)

// drainToChan 把一组消息灌进 channel 后关闭，模拟 ChatWithContext 的流。
func drainToChan(msgs ...*StreamMessage) chan *StreamMessage {
	ch := make(chan *StreamMessage, len(msgs))
	for _, m := range msgs {
		ch <- m
	}
	close(ch)
	return ch
}

// TestCollectReportStream_TruncationDetected 锁定核心契约：
// 模型因 maxTokens 耗尽结束（finish_reason=length）时，绝不能被当成
// 「正常生成完成」。这是用户实证「盘后分析等经常运行一段，没有全部完成」
// 的直接根因 —— 旧实现不看 FinishReason，半份研报被当完整研报保存展示。
func TestCollectReportStream_TruncationDetected(t *testing.T) {
	ch := drainToChan(
		&StreamMessage{Role: "assistant", Content: "### 📊 一、全天行情概览\n"},
		&StreamMessage{Role: "assistant", Content: "涨跌家数比 3:1，赚钱效应良好。\n"},
		&StreamMessage{Role: "assistant", Content: "### 🚀 二、核心主线（此处被 maxTokens 截断，后续章节完全缺失）"},
		// 终态：模型真实结束原因是 length（截断），不是 stop
		&StreamMessage{Role: "assistant", Content: "agent-DONE", FinishReason: "length"},
	)

	res := collectReportStream(ch)

	if !res.Truncated {
		t.Fatal("finish_reason=length 必须被判定为截断，否则半份研报会被当成完整研报展示")
	}
	if res.FinishReason != "length" {
		t.Errorf("FinishReason = %q, want \"length\"", res.FinishReason)
	}
	// 截断时正文仍应被汇总（调用方据此给出带上下文的错误）
	if !strings.Contains(res.Content, "全天行情概览") {
		t.Error("截断时已收到的正文仍应被汇总，便于定位丢到哪里")
	}
	if strings.Contains(res.Content, "agent-DONE") {
		t.Error("终态占位串 agent-DONE 绝不能被写进正文")
	}
}

// TestCollectReportStream_NormalStopNotTruncated 反向护栏：
// 正常结束（stop）绝不能误判为截断，否则用户每次都会被提示调大 maxTokens。
func TestCollectReportStream_NormalStopNotTruncated(t *testing.T) {
	ch := drainToChan(
		&StreamMessage{Role: "assistant", Content: "### 📊 一、全天行情概览\n完整内容……\n"},
		&StreamMessage{Role: "assistant", Content: "agent-DONE", FinishReason: "stop"},
	)

	res := collectReportStream(ch)

	if res.Truncated {
		t.Error("finish_reason=stop 是正常结束，不得判定为截断")
	}
	if res.FinishReason != "stop" {
		t.Errorf("FinishReason = %q, want \"stop\"", res.FinishReason)
	}
	if res.StreamErr != "" {
		t.Errorf("正常流不应有错误，实际 %q", res.StreamErr)
	}
}

// TestCollectReportStream_ContentFilterTruncated content_filter 同样是不完整输出。
func TestCollectReportStream_ContentFilterTruncated(t *testing.T) {
	ch := drainToChan(
		&StreamMessage{Role: "assistant", Content: "部分正文……"},
		&StreamMessage{Role: "assistant", Content: "agent-DONE", FinishReason: "content_filter"},
	)

	res := collectReportStream(ch)

	if !res.Truncated {
		t.Error("content_filter 表示正文被安全策略拦截，应判定为截断")
	}
}

// TestCollectReportStream_ErrorCapturedNotMixedIntoContent
// 错误串必须进 StreamErr，绝不能混进正文（否则用户会在研报里看到 ❌）。
func TestCollectReportStream_ErrorCapturedNotMixedIntoContent(t *testing.T) {
	ch := drainToChan(
		&StreamMessage{Role: "assistant", Content: "正常的一段正文\n"},
		&StreamMessage{Role: "assistant", Content: "❌ Agent 调用失败: 模型未产出正文"},
	)

	res := collectReportStream(ch)

	if res.StreamErr == "" {
		t.Fatal("错误消息必须被捕获到 StreamErr")
	}
	if strings.Contains(res.Content, "❌") {
		t.Error("错误串绝不混入正文")
	}
	if !strings.Contains(res.Content, "正常的一段正文") {
		t.Error("错误之前的正常正文应保留")
	}
}

// TestCollectReportStream_ReasoningSeparatedFromContent
// reasoning_content 必须与正文分离，不能污染研报正文。
func TestCollectReportStream_ReasoningSeparatedFromContent(t *testing.T) {
	ch := drainToChan(
		&StreamMessage{Role: "assistant", ReasoningContent: "让我先分析今天的盘面数据……"},
		&StreamMessage{Role: "assistant", ReasoningContent: "[STATUS]🤔 正在理解问题"},
		&StreamMessage{Role: "assistant", Content: "### 最终研报正文"},
	)

	res := collectReportStream(ch)

	if !strings.Contains(res.Reasoning, "让我先分析今天的盘面数据") {
		t.Error("非 STATUS 的 reasoning 应进入 Reasoning")
	}
	if strings.Contains(res.Reasoning, "[STATUS]") {
		t.Error("[STATUS] 遥测行不应进入 Reasoning")
	}
	if strings.Contains(res.Content, "让我先分析") {
		t.Error("reasoning 绝不混入正文")
	}
}

// TestCollectReportStream_NoTerminalNoPanic 流未发终态时也不能 panic
// （外层 ChatWithContext 的 defer 护栏保证一定会发，这里是防御性验证）。
func TestCollectReportStream_NoTerminalNoPanic(t *testing.T) {
	ch := drainToChan(
		&StreamMessage{Role: "assistant", Content: "只有正文，没有终态消息"},
	)

	res := collectReportStream(ch)

	if res.Truncated {
		t.Error("没有终态消息时不应判为截断（无证据）")
	}
	if !strings.Contains(res.Content, "只有正文") {
		t.Error("正文应被正常汇总")
	}
}
