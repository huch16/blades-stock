package agent

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/go-kratos/blades"
)

type failMockProvider struct {
	name      string
	callCount int32
	err       error
}

func (f *failMockProvider) Name() string { return f.name }
func (f *failMockProvider) Generate(ctx context.Context, req *blades.ModelRequest) (*blades.ModelResponse, error) {
	atomic.AddInt32(&f.callCount, 1)
	return nil, f.err
}
func (f *failMockProvider) NewStreaming(ctx context.Context, req *blades.ModelRequest) blades.Generator[*blades.ModelResponse, error] {
	return func(yield func(*blades.ModelResponse, error) bool) {
		atomic.AddInt32(&f.callCount, 1)
		yield(nil, f.err)
	}
}

type successMockProvider struct {
	name      string
	callCount int32
	respText  string
}

func (s *successMockProvider) Name() string { return s.name }
func (s *successMockProvider) Generate(ctx context.Context, req *blades.ModelRequest) (*blades.ModelResponse, error) {
	atomic.AddInt32(&s.callCount, 1)
	msg := &blades.Message{
		Role:  blades.RoleAssistant,
		Parts: []blades.Part{blades.TextPart{Text: s.respText}},
	}
	return &blades.ModelResponse{Message: msg}, nil
}
func (s *successMockProvider) NewStreaming(ctx context.Context, req *blades.ModelRequest) blades.Generator[*blades.ModelResponse, error] {
	return func(yield func(*blades.ModelResponse, error) bool) {
		atomic.AddInt32(&s.callCount, 1)
		msg := &blades.Message{
			Role:  blades.RoleAssistant,
			Parts: []blades.Part{blades.TextPart{Text: s.respText}},
		}
		yield(&blades.ModelResponse{Message: msg}, nil)
	}
}

func TestFallbackModelProvider_StickyPromotion(t *testing.T) {
	primary := &failMockProvider{
		name: "gemini-3-flash",
		err:  errors.New("403 Forbidden: token_rejected"),
	}
	fallback := &successMockProvider{
		name:     "MiniMax-M3",
		respText: "fallback success answer",
	}

	fbProvider := NewFallbackModelProvider(primary, "gemini-3-flash", []blades.ModelProvider{fallback}, []string{"MiniMax-M3"})

	var notifiedCur, notifiedNext string
	var notifiedErr error
	notifier := func(cur, next string, err error) {
		notifiedCur = cur
		notifiedNext = next
		notifiedErr = err
	}

	ctx := WithFallbackNotifier(context.Background(), notifier)
	req := &blades.ModelRequest{
		Messages: []*blades.Message{blades.UserMessage("test question")},
	}

	// 1. 第一轮流式调用：主模型失败(403)，降级到备用模型
	gen := fbProvider.NewStreaming(ctx, req)
	var streamText string
	gen(func(resp *blades.ModelResponse, err error) bool {
		if err != nil {
			t.Fatalf("unexpected stream err: %v", err)
		}
		if resp != nil && resp.Message != nil {
			streamText = resp.Message.Text()
		}
		return true
	})

	if streamText != "fallback success answer" {
		t.Fatalf("expected 'fallback success answer', got %q", streamText)
	}
	if notifiedCur != "gemini-3-flash" || notifiedNext != "MiniMax-M3" {
		t.Fatalf("notifier expected cur=gemini-3-flash next=MiniMax-M3, got cur=%s next=%s", notifiedCur, notifiedNext)
	}
	if notifiedErr == nil || !IsPermanentAuthError(notifiedErr) {
		t.Fatalf("expected permanent auth error in notifier, got %v", notifiedErr)
	}
	if atomic.LoadInt32(&primary.callCount) != 1 {
		t.Fatalf("expected primary call count 1, got %d", primary.callCount)
	}
	if atomic.LoadInt32(&fallback.callCount) != 1 {
		t.Fatalf("expected fallback call count 1, got %d", fallback.callCount)
	}

	// 2. 第二轮流式调用：由于粘性提升与黑名单机制，必须直接调用 fallback，绝对不再尝试已死亡的 primary
	gen2 := fbProvider.NewStreaming(ctx, req)
	var streamText2 string
	gen2(func(resp *blades.ModelResponse, err error) bool {
		if resp != nil && resp.Message != nil {
			streamText2 = resp.Message.Text()
		}
		return true
	})

	if streamText2 != "fallback success answer" {
		t.Fatalf("expected 'fallback success answer', got %q", streamText2)
	}
	// primary 调用次数必须依然为 1（没有被再次调用）
	if atomic.LoadInt32(&primary.callCount) != 1 {
		t.Fatalf("expected primary call count still 1, but got %d (sticky promotion failed)", primary.callCount)
	}
	// fallback 调用次数应递增为 2
	if atomic.LoadInt32(&fallback.callCount) != 2 {
		t.Fatalf("expected fallback call count 2, got %d", fallback.callCount)
	}
}

func TestIsPermanentAuthError(t *testing.T) {
	cases := []struct {
		errStr   string
		expected bool
	}{
		{"403 Forbidden", true},
		{"HTTP 401 Unauthorized", true},
		{"token_rejected by gateway", true},
		{"invalid token supplied", true},
		{"invalid_api_key in request", true},
		{"unauthorized access", true},
		{"connection reset by peer", false},
		{"context deadline exceeded", false},
		{"500 Internal Server Error", false},
		{"429 Too Many Requests", false},
	}

	for _, c := range cases {
		err := errors.New(c.errStr)
		res := IsPermanentAuthError(err)
		if res != c.expected {
			t.Errorf("IsPermanentAuthError(%q) = %v, expected %v", c.errStr, res, c.expected)
		}
	}
}

func TestFormatFallbackReason(t *testing.T) {
	cases := []struct {
		errStr   string
		expected string
	}{
		{
			errStr:   `POST "https://aigw.woha.men/v1/chat/completions": 403 Forbidden {"message":"Invalid token. Please check your API key.","type":"token_rejected","code":"token_rejected"}`,
			expected: "API Key 鉴权失败 (HTTP 401/403: Invalid token. Please check your API key.)",
		},
		{
			errStr:   `404 Not Found: {"error":{"message":"The model 'gemini-3.8-flash' does not exist"}}`,
			expected: "模型不存在或网关未映射 (HTTP 404: The model 'gemini-3.8-flash' does not exist)",
		},
		{
			errStr:   `HTTP 400: {"error":{"message":"tools parameter is not supported for this model"}}`,
			expected: "请求参数异常或网关不支持Tools (HTTP 400: tools parameter is not supported for this model)",
		},
		{
			errStr:   `429 Too Many Requests: {"error":{"message":"Rate limit exceeded"}}`,
			expected: "网关限流或额度耗尽 (HTTP 429: Rate limit exceeded)",
		},
		{
			errStr:   `502 Bad Gateway: {"message":"Bad Gateway"}`,
			expected: "网关服务器异常 (HTTP 5xx: Bad Gateway)",
		},
		{
			errStr:   `context deadline exceeded`,
			expected: "模型响应超时 (超过设定时限)",
		},
	}

	for _, c := range cases {
		err := errors.New(c.errStr)
		res := FormatFallbackReason(err)
		if res != c.expected {
			t.Errorf("FormatFallbackReason(%q) = %q, expected %q", c.errStr, res, c.expected)
		}
	}
}
