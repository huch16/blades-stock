package agent

import (
	"context"
	"errors"
	"testing"
	"time"
)

// 第四轮 B 组（运行）回归测试：瞬时错误分类补漏、i/o timeout 归属、
// 流式发送 ctx 感知（防 goroutine 泄漏）。

func TestIsTransientModelError_Round4(t *testing.T) {
	cases := []struct {
		name     string
		errStr   string
		expected bool
	}{
		// 新增：网络层瞬时故障（旧列表只覆盖了 reset/broken pipe，
		// 拨号/DNS/拒连/握手超时等形态会直接跳出重试循环）。
		{"dial refused", "dial tcp 1.2.3.4:443: connect: connection refused", true},
		{"dial timeout", "dial tcp: i/o timeout", true},
		{"dial tcp generic", "dial tcp: lookup api.deepseek.com: no such host", true},
		{"DNS no such host", "no such host", true},
		{"DNS temp failure", "temporary failure in name resolution", true},
		{"network unreachable", "dial tcp: network is unreachable", true},
		{"socket hang up", "socket hang up", true},
		{"connection aborted", "read: connection aborted", true},
		{"connection timed out", "connect: connection timed out", true},
		{"connection reset bare", "read: connection reset", true},
		{"handshake timeout", "net/http: TLS handshake timeout", true},
		{"i/o timeout is transient", "read: i/o timeout", true},
		// 新增：容量/不可用文案（TokenLab at capacity 无 429 时也应重试）。
		{"at capacity without 429", "model is at capacity, please retry", true},
		{"service unavailable bare", "service unavailable", true},
		{"model unavailable", "model temporarily unavailable", true},

		// 护栏：非瞬时错误仍不重试。
		{"auth still not transient", "invalid api key (401)", false},
		{"not found still not transient", "model 'gpt-none' does not exist (404)", false},
		{"bare reset still not transient", "password reset required", false},
		{"bad params still not transient", "invalid params, function parameters is empty (2013)", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isTransientModelError(errors.New(c.errStr)); got != c.expected {
				t.Errorf("isTransientModelError(%q) = %v, want %v", c.errStr, got, c.expected)
			}
		})
	}
}

func TestIsRequestTimeoutError_Round4(t *testing.T) {
	cases := []struct {
		name     string
		errStr   string
		expected bool
	}{
		{"context deadline", "context deadline exceeded", true},
		{"client timeout", "context deadline exceeded (Client.Timeout exceeded while awaiting headers)", true},
		{"awaiting headers", "net/http: timeout awaiting response headers", true},
		{"request timeout", "request timeout after 300s", true},
		// i/o timeout 是 net 层拨号/读写超时，不是我们配置的请求超时：
		// 只应进瞬时重试，不应被报成“调大卡片超时”。
		{"i/o timeout is NOT a config timeout", "read: i/o timeout", false},
		{"plain network error", "connection reset by peer", false},
		{"auth error", "401 Unauthorized", false},
	}
	for _, c := range cases {
		if got := isRequestTimeoutError(errors.New(c.errStr)); got != c.expected {
			t.Errorf("isRequestTimeoutError(%q) = %v, want %v", c.errStr, got, c.expected)
		}
	}
	if isRequestTimeoutError(nil) {
		t.Errorf("isRequestTimeoutError(nil) 应为 false")
	}
}

// TestSendStreamMsg_CancelledConsumer 守卫流式发送的泄漏修复：
// 消费者已退出（ctx 已取消）且无人接收时，发送必须立刻返回 false，
// 而不是像旧的 `ch <- m` 那样永久阻塞（生产 goroutine 泄漏）。
func TestSendStreamMsg_CancelledConsumer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 模拟 AbortChatWithAgent / 前端断连后的取消

	ch := make(chan *StreamMessage) // 无缓冲、无接收方
	start := time.Now()
	ok := sendStreamMsg(ch, ctx, &StreamMessage{Role: "assistant", Content: "x"})
	if ok {
		t.Fatalf("ctx 已取消且无人接收时应返回 false")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("取消后发送应在毫秒级返回，实际 %v（疑似阻塞）", elapsed)
	}
}

// TestSendStreamMsg_LiveConsumer 守卫正常路径：消费者活着时消息必须送达，
// 不得因“防泄漏”而丢正文 chunk。
func TestSendStreamMsg_LiveConsumer(t *testing.T) {
	ch := make(chan *StreamMessage, 1)
	msg := &StreamMessage{Role: "assistant", Content: "hello"}
	if !sendStreamMsg(ch, context.Background(), msg) {
		t.Fatalf("消费者活着时应返回 true")
	}
	if got := <-ch; got != msg {
		t.Fatalf("应原样收到消息，got %+v", got)
	}

	// nil ctx 退化为阻塞发送（调用方从不传 nil，此处只保证不 panic）。
	ch2 := make(chan *StreamMessage, 1)
	if !sendStreamMsg(ch2, nil, msg) {
		t.Fatalf("nil ctx + 有缓冲时应返回 true")
	}
	if got := <-ch2; got != msg {
		t.Fatalf("nil ctx 时应原样收到消息，got %+v", got)
	}
}
