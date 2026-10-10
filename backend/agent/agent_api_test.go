package agent

import (
	"errors"
	"testing"
	"time"
)

func TestIsTransientModelError(t *testing.T) {
	transientCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"HTTP 429 Rate Limit", errors.New("HTTP 429 Too Many Requests"), true},
		{"HTTP 500 Server Error", errors.New("500 internal server error"), true},
		{"HTTP 502 Bad Gateway", errors.New("502 bad gateway"), true},
		{"HTTP 503 Service Unavailable", errors.New("503 Service Unavailable"), true},
		{"HTTP 504 Gateway Timeout", errors.New("504 Gateway Timeout"), true},
		{"Resource Exhausted", errors.New("resourceexhausted: quota exceeded"), true},
		{"Server Busy", errors.New("server busy, please try again later"), true},
		{"Overloaded", errors.New("model is overloaded"), true},
		{"Unexpected end of JSON input", errors.New("unexpected end of JSON input"), true},
		{"Unexpected EOF", errors.New("unexpected EOF"), true},
		{"Plain EOF", errors.New("EOF"), true},
		{"Connection reset by peer", errors.New("read: connection reset by peer"), true},
		{"Broken pipe", errors.New("write: broken pipe"), true},
		{"Windows WSARECV error", errors.New("wsarecv: an existing connection was forcibly closed"), true},
		{"Timeout awaiting response", errors.New("net/http: timeout awaiting response headers"), true},
		{"Context deadline exceeded", errors.New("context deadline exceeded"), true},
		{"Client timeout", errors.New("client.timeout exceeded while awaiting headers"), true},

		// Non-transient errors (should NOT retry)
		{"Authentication Failure", errors.New("invalid api key (401)"), false},
		{"Permission Denied", errors.New("permission denied (403)"), false},
		{"Model Not Found", errors.New("model 'gpt-none' does not exist (404)"), false},
		{"Invalid Request Params", errors.New("invalid params, function parameters is empty (2013)"), false},
		{"Bad Schema", errors.New("schema validation failed"), false},
	}

	for _, tc := range transientCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := isTransientModelError(tc.err)
			if actual != tc.expected {
				t.Errorf("isTransientModelError(%v) = %v; want %v", tc.err, actual, tc.expected)
			}
		})
	}
}

func TestAgentRunMetrics(t *testing.T) {
	start := time.Now().Add(-1500 * time.Millisecond)
	firstChunk := start.Add(300 * time.Millisecond)

	metrics := AgentRunMetrics{
		TotalDuration:     time.Since(start),
		FirstChunkLatency: firstChunk.Sub(start),
		ToolCallCount:     3,
		RetryCount:        1,
		FinalStatus:       "retry_success",
	}

	if metrics.ToolCallCount != 3 {
		t.Errorf("expected 3 tool calls, got %d", metrics.ToolCallCount)
	}
	if metrics.RetryCount != 1 {
		t.Errorf("expected 1 retry, got %d", metrics.RetryCount)
	}
	if metrics.FinalStatus != "retry_success" {
		t.Errorf("expected retry_success status, got %s", metrics.FinalStatus)
	}
	if metrics.FirstChunkLatency < 250*time.Millisecond || metrics.FirstChunkLatency > 350*time.Millisecond {
		t.Errorf("first chunk latency unexpected: %v", metrics.FirstChunkLatency)
	}
}
