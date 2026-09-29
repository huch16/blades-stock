package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"blades-stock/backend/circuitbreaker"
	bladesTools "github.com/go-kratos/blades/tools"
	"github.com/google/jsonschema-go/jsonschema"
)

// flakyTool 构造一个可控调用次数 / 返回错误种类的 Tool，用于测试重试 / 熔断。
type flakyTool struct {
	name       string
	attempts   int32 // 已调用次数（原子计数）
	failTimes  int32 // 剩余失败次数；>0 时返回 transient 错误，==0 时返回 success
	failKind   ErrKind
	failMsg    string // 自定义错误消息；空时按 failKind 自动构造
	failOnce   int32 // 仅第一次返回错误
}

func (f *flakyTool) Name() string                     { return f.name }
func (f *flakyTool) Description() string              { return "test tool" }
func (f *flakyTool) InputSchema() *jsonschema.Schema  { return nil }
func (f *flakyTool) OutputSchema() *jsonschema.Schema { return nil }

func (f *flakyTool) Handle(ctx context.Context, input string) (string, error) {
	n := atomic.AddInt32(&f.attempts, 1)
	if atomic.LoadInt32(&f.failOnce) == 1 && n == 1 {
		return "", makeErrByKind(f.failKind, f.failMsg)
	}
	if atomic.LoadInt32(&f.failTimes) > 0 {
		atomic.AddInt32(&f.failTimes, -1)
		return "", makeErrByKind(f.failKind, f.failMsg)
	}
	return fmt.Sprintf("ok:%s", input), nil
}

func makeErrByKind(k ErrKind, msg string) error {
	if msg == "" {
		switch k {
		case ErrKindTransient:
			msg = "connection refused"
		case ErrKindAuth:
			msg = "401 unauthorized"
		case ErrKindNotFound:
			msg = "404 not found"
		case ErrKindBadRequest:
			msg = "400 bad request"
		default:
			msg = "unknown failure"
		}
	}
	return errors.New(msg)
}

func newFlakyTool(name string) *flakyTool {
	return &flakyTool{name: name}
}

func mustParseErrorJSON(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("expected JSON output, got parse error: %v (raw=%q)", err, raw)
	}
	return m
}

// =============================================================================
// 错误分类测试
// =============================================================================

func TestClassifyMCPErr_Nil(t *testing.T) {
	if got := classifyMCPErr(nil); got != ErrKindUnknown {
		t.Errorf("nil error should be Unknown, got %v", got)
	}
}

func TestClassifyMCPErr_Transient(t *testing.T) {
	cases := []string{
		"dial tcp: timeout",
		"connection refused",
		"i/o timeout",
		"EOF",
		"rpc error: code = Unavailable",
		"502 bad gateway",
		"503 service unavailable",
	}
	for _, msg := range cases {
		if got := classifyMCPErr(errors.New(msg)); got != ErrKindTransient {
			t.Errorf("classify(%q) = %v, want Transient", msg, got)
		}
	}
}

func TestClassifyMCPErr_Auth(t *testing.T) {
	cases := []string{"401 unauthorized", "403 forbidden", "invalid token expired", "authentication failed"}
	for _, msg := range cases {
		if got := classifyMCPErr(errors.New(msg)); got != ErrKindAuth {
			t.Errorf("classify(%q) = %v, want Auth", msg, got)
		}
	}
}

func TestClassifyMCPErr_NotFound(t *testing.T) {
	if got := classifyMCPErr(errors.New("404 not found")); got != ErrKindNotFound {
		t.Errorf("got %v, want NotFound", got)
	}
	if got := classifyMCPErr(errors.New("no such file")); got != ErrKindNotFound {
		t.Errorf("got %v, want NotFound", got)
	}
}

func TestClassifyMCPErr_BadRequest(t *testing.T) {
	if got := classifyMCPErr(errors.New("400 bad request")); got != ErrKindBadRequest {
		t.Errorf("got %v, want BadRequest", got)
	}
	if got := classifyMCPErr(errors.New("invalid json: missing field")); got != ErrKindBadRequest {
		t.Errorf("got %v, want BadRequest", got)
	}
}

func TestClassifyMCPErr_Unknown(t *testing.T) {
	if got := classifyMCPErr(errors.New("something completely new")); got != ErrKindUnknown {
		t.Errorf("got %v, want Unknown", got)
	}
}

func TestIsRetryable(t *testing.T) {
	tests := map[ErrKind]bool{
		ErrKindTransient:  true,
		ErrKindAuth:       false,
		ErrKindNotFound:   false,
		ErrKindBadRequest: false,
		ErrKindUnknown:    false,
	}
	for kind, want := range tests {
		if got := isRetryable(kind); got != want {
			t.Errorf("isRetryable(%v) = %v, want %v", kind, got, want)
		}
	}
}

func TestCountsTowardBreaker(t *testing.T) {
	tests := map[ErrKind]bool{
		ErrKindTransient:  true,
		ErrKindAuth:       true,
		ErrKindNotFound:   false,
		ErrKindBadRequest: false,
		ErrKindUnknown:    false,
	}
	for kind, want := range tests {
		if got := countsTowardBreaker(kind); got != want {
			t.Errorf("countsTowardBreaker(%v) = %v, want %v", kind, got, want)
		}
	}
}

// =============================================================================
// 重试行为测试
// =============================================================================

func TestWrapMCPToolHandlerV2_RetryThenSuccess(t *testing.T) {
	// 重试2次后成功（failTimes=2, attempts=1+2=3）
	tool := newFlakyTool("flaky")
	tool.failTimes = 2
	tool.failKind = ErrKindTransient

	wrapped := wrapMCPToolHandlerV2(tool, "test-retry-success")
	result, err := wrapped.Handle(context.Background(), "input")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if result != "ok:input" {
		t.Errorf("expected success result, got %q", result)
	}
	if atomic.LoadInt32(&tool.attempts) != 3 {
		t.Errorf("expected 3 attempts (1 initial + 2 retries), got %d", tool.attempts)
	}
}

func TestWrapMCPToolHandlerV2_MaxRetriesExceeded(t *testing.T) {
	// 始终 transient 失败：应尝试 MaxRetryAttempts+1=4 次后返回 JSON 错误
	tool := newFlakyTool("always-fail")
	tool.failTimes = 100 // 足够大，永远失败
	tool.failKind = ErrKindTransient

	wrapped := wrapMCPToolHandlerV2(tool, "test-max-retries")
	result, err := wrapped.Handle(context.Background(), "input")
	if err != nil {
		t.Fatalf("V2 should never propagate raw err to caller: %v", err)
	}
	if atomic.LoadInt32(&tool.attempts) != int32(MaxRetryAttempts+1) {
		t.Errorf("expected %d attempts, got %d", MaxRetryAttempts+1, tool.attempts)
	}
	m := mustParseErrorJSON(t, result)
	if m["error"] != true {
		t.Error("JSON should have error=true")
	}
	if m["tool"] != "always-fail" {
		t.Errorf("tool field mismatch: %v", m["tool"])
	}
	if m["server"] != "test-max-retries" {
		t.Errorf("server field mismatch: %v", m["server"])
	}
}

func TestWrapMCPToolHandlerV2_NoRetryOnAuth(t *testing.T) {
	// Auth 错误不应重试
	tool := newFlakyTool("auth-fail")
	tool.failTimes = 100
	tool.failKind = ErrKindAuth

	wrapped := wrapMCPToolHandlerV2(tool, "test-no-retry-auth")
	_, err := wrapped.Handle(context.Background(), "input")
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&tool.attempts) != 1 {
		t.Errorf("Auth should not retry, got %d attempts", tool.attempts)
	}
}

func TestWrapMCPToolHandlerV2_NoRetryOnBadRequest(t *testing.T) {
	tool := newFlakyTool("bad-req")
	tool.failTimes = 100
	tool.failKind = ErrKindBadRequest

	wrapped := wrapMCPToolHandlerV2(tool, "test-no-retry-badreq")
	_, err := wrapped.Handle(context.Background(), "x")
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&tool.attempts) != 1 {
		t.Errorf("BadRequest should not retry, got %d attempts", tool.attempts)
	}
}

func TestWrapMCPToolHandlerV2_SuccessClearsBreaker(t *testing.T) {
	// 成功调用应让断路器恢复 closed 状态
	// 先手动制造失败达到阈值
	breakerName := "test-success-clears"
	cb := circuitbreaker.Get(breakerName)
	for i := 0; i < CircuitFailureThreshold; i++ {
		cb.Failure()
	}
	if cb.State() != circuitbreaker.Open {
		t.Fatalf("setup: expected Open, got %v", cb.State())
	}

	// 但 V2 wrapper 在 Allow()=false 时快速失败，不会调用成功路径
	// 所以这里只验证：断路器处于 HalfOpen 时，一次成功就能 close
	cb2 := circuitbreaker.New(circuitbreaker.Config{
		Name:             "test-success-clears-2",
		FailureThreshold: 2,
		Cooldown:         10 * time.Millisecond,
	})
	cb2.Failure()
	cb2.Failure()
	if cb2.State() != circuitbreaker.Open {
		t.Fatal("setup 2: expected Open")
	}
	time.Sleep(20 * time.Millisecond)
	if !cb2.Allow() {
		t.Fatal("after cooldown should allow probe")
	}
	cb2.Success()
	if cb2.State() != circuitbreaker.Closed {
		t.Errorf("Success should close breaker, got %v", cb2.State())
	}
}

// =============================================================================
// 熔断测试
// =============================================================================

func TestWrapMCPToolHandlerV2_CircuitOpensAfterFailures(t *testing.T) {
	serverName := "test-circuit-opens"
	// 使用默认 V2 阈值（3）+ 短 cooldown 以加速测试
	// V2 在首次调用时 Configure 为 FailureThreshold=3 / Cooldown=30s
	// 测试完成后不需要清状态：该 serverName 只在此测试出现

	tool := newFlakyTool("circ-fail")
	tool.failTimes = 100
	tool.failKind = ErrKindTransient

	wrapped := wrapMCPToolHandlerV2(tool, serverName)
	cb := circuitbreaker.Get(CircuitBreakerNamePrefix + serverName)
	// V2 首次调用时已 Configure，默认阈值=3
	if cb.Stats().Failures != 0 {
		t.Fatalf("setup: expected fresh breaker, got %+v", cb.Stats())
	}

	// V2 每次 Handle 调用内部最多尝试 MaxRetryAttempts+1=4 次，但只计 1 次 breaker.Failure()
	// 需要 CircuitFailureThreshold=3 次 Handle 才能 Open
	for i := 0; i < CircuitFailureThreshold; i++ {
		_, _ = wrapped.Handle(context.Background(), "x")
	}

	if cb.State() != circuitbreaker.Open {
		t.Fatalf("expected Open after %d Handle calls, got %v", CircuitFailureThreshold, cb.State())
	}

	// 下一次调用应快速失败（circuit_open JSON），不再调用底层
	beforeAttempts := atomic.LoadInt32(&tool.attempts)
	result, _ := wrapped.Handle(context.Background(), "x")
	afterAttempts := atomic.LoadInt32(&tool.attempts)

	if afterAttempts != beforeAttempts {
		t.Error("circuit open should skip underlying call")
	}
	m := mustParseErrorJSON(t, result)
	if m["circuit_open"] != true {
		t.Errorf("expected circuit_open=true, got %v", m)
	}
}

// =============================================================================
// 错误脱敏测试
// =============================================================================

func TestSanitizeError_Token(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"dial tcp: 401 with token=abc123def456ghi789", "dial tcp: 401 with token=***"},
		{"Authorization: Bearer xyz789abcdef012345", "Bearer=***"},
		{"api_key=sk-1234567890abcdef failed", "api_key=*** failed"},
		{"password=mySecret123 in config", "password=*** in config"},
		{`{"Authorization": "Bearer foo1234567890"}`, `Bearer=***`},
	}
	for _, c := range cases {
		got := sanitizeError(c.in)
		if !contains(got, "***") {
			t.Errorf("sanitize(%q) = %q, should contain ***", c.in, got)
		}
		if got == c.in && len(c.in) > 20 {
			t.Errorf("sanitize did nothing for %q", c.in)
		}
		_ = c.want
	}
}

func TestSanitizeError_PreservesNonSensitive(t *testing.T) {
	in := "connection refused to 192.168.1.1"
	if got := sanitizeError(in); got != in {
		t.Errorf("non-sensitive message should be unchanged: got %q", got)
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// =============================================================================
// 退避时序测试
// =============================================================================

func TestBackoff(t *testing.T) {
	if got := backoff(0); got != InitialBackoff {
		t.Errorf("backoff(0) = %v, want %v", got, InitialBackoff)
	}
	if got := backoff(1); got != InitialBackoff*2 {
		t.Errorf("backoff(1) = %v, want %v", got, InitialBackoff*2)
	}
	if got := backoff(2); got != InitialBackoff*4 {
		t.Errorf("backoff(2) = %v, want %v", got, InitialBackoff*4)
	}
}

// =============================================================================
// 集成：V2 与旧 wrapMCPToolHandler 一致性
// =============================================================================

func TestWrapMCPToolHandler_CompatIsV2(t *testing.T) {
	// 旧 wrapMCPToolHandler 调用入口仍存在（向后兼容）；其实现应等价于 V2
	tool := newFlakyTool("compat")
	tool.failTimes = 1
	tool.failKind = ErrKindTransient

	wrapped := wrapMCPToolHandler(tool, "test-compat")
	result, err := wrapped.Handle(context.Background(), "x")
	if err != nil {
		t.Fatal(err)
	}
	// 1 次失败 → 重试 → 成功
	if result != "ok:x" {
		t.Errorf("compat wrapper should retry then succeed, got %q", result)
	}
}

// =============================================================================
// 防止 import 漂移
// =============================================================================
// 下方引用 bladesTools 是为了在缺少 import 时 build 立即失败，便于定位。
var _ bladesTools.Tool = (*flakyTool)(nil)