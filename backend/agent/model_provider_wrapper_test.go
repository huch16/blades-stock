package agent

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"blades-stock/backend/data"
	"github.com/go-kratos/blades"
	"github.com/stretchr/testify/require"
	"testing"
)

// mockProvider is a simple blades.ModelProvider implementation for testing.
type mockProvider struct {
	name string
	resp *blades.ModelResponse
	err  error

	mu            sync.Mutex
	generateCalls int
	streamCalls   int
}

func (m *mockProvider) Name() string {
	return m.name
}
func (m *mockProvider) Generate(ctx context.Context, req *blades.ModelRequest) (*blades.ModelResponse, error) {
	m.mu.Lock()
	m.generateCalls++
	m.mu.Unlock()
	return m.resp, m.err
}
func (m *mockProvider) NewStreaming(ctx context.Context, req *blades.ModelRequest) blades.Generator[*blades.ModelResponse, error] {
	m.mu.Lock()
	m.streamCalls++
	m.mu.Unlock()
	return func(yield func(*blades.ModelResponse, error) bool) {
		if m.err != nil {
			yield(nil, m.err)
			return
		}
		if m.resp == nil {
			yield(nil, nil)
			return
		}
		yield(m.resp, nil)
	}
}
func (m *mockProvider) calls() (generate, stream int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.generateCalls, m.streamCalls
}

func testModelRequest(i int) *blades.ModelRequest {
	return &blades.ModelRequest{
		Messages: []*blades.Message{
			{
				Role: blades.RoleUser,
				Parts: []blades.Part{
					blades.TextPart{Text: fmt.Sprintf("test%d", i)},
				},
			},
		},
	}
}

func successMock() *mockProvider {
	return &mockProvider{
		name: "mock",
		resp: &blades.ModelResponse{
			Message: &blades.Message{
				Role: blades.RoleAssistant,
				Parts: []blades.Part{
					blades.TextPart{Text: "hello"},
				},
			},
		},
		err: nil,
	}
}

// drainStream runs a generator to completion and returns the first response
// and the first error seen (the wrapper yields at most one terminal error).
func drainStream(t *testing.T, gen blades.Generator[*blades.ModelResponse, error]) (*blades.ModelResponse, error) {
	t.Helper()
	var gotResp *blades.ModelResponse
	var gotErr error
	gen(func(resp *blades.ModelResponse, err error) bool {
		if err != nil && gotErr == nil {
			gotErr = err
		}
		if resp != nil && gotResp == nil {
			gotResp = resp
		}
		return true
	})
	return gotResp, gotErr
}

func TestModelProviderWrapper_RateLimit(t *testing.T) {
	mock := successMock()
	// Config with rate limit: 1 req/s, burst 1 — the 2nd immediate call
	// must be rejected with an explicit error, and must never reach the
	// underlying provider.
	config := &data.AIConfig{
		RateLimitPerSecond: 1,
		Burst:              1,
		DailyRequestQuota:  0, // unlimited
		DailyCostQuota:     0,
	}
	wrapped := NewModelProviderWrapper(mock, config)

	ctx := context.Background()
	resp, err := wrapped.Generate(ctx, testModelRequest(0))
	require.NoError(t, err, "first request within the limit should succeed")
	require.NotNil(t, resp)

	resp, err = wrapped.Generate(ctx, testModelRequest(1))
	require.ErrorIs(t, err, ErrRateLimited, "second immediate request must be rate limited")
	require.Nil(t, resp)

	genCalls, _ := mock.calls()
	require.Equal(t, 1, genCalls, "rejected request must not reach the underlying provider")
}

func TestModelProviderWrapper_DailyQuota(t *testing.T) {
	mock := successMock()
	config := &data.AIConfig{
		RateLimitPerSecond: 0, // unlimited
		Burst:              0,
		DailyRequestQuota:  2, // only 2 requests per day
		DailyCostQuota:     0,
	}
	wrapped := NewModelProviderWrapper(mock, config)

	ctx := context.Background()
	for i := 0; i < 2; i++ {
		resp, err := wrapped.Generate(ctx, testModelRequest(i))
		require.NoError(t, err, "request %d within the daily quota should succeed", i)
		require.NotNil(t, resp)
	}
	resp, err := wrapped.Generate(ctx, testModelRequest(2))
	require.ErrorIs(t, err, ErrRateLimited, "3rd request over the daily quota must be rejected")
	require.Nil(t, resp)

	genCalls, _ := mock.calls()
	require.Equal(t, 2, genCalls, "rejected request must not reach the underlying provider")
}

func TestModelProviderWrapper_DailyCostQuota(t *testing.T) {
	mock := successMock()
	config := &data.AIConfig{
		RateLimitPerSecond: 0,
		Burst:              0,
		DailyRequestQuota:  0,
		// estimateLLMCost charges 0.001 per request: quota 0.0025 allows
		// 2 requests, the 3rd would cost 0.003 > 0.0025.
		DailyCostQuota: 0.0025,
	}
	wrapped := NewModelProviderWrapper(mock, config)

	ctx := context.Background()
	for i := 0; i < 2; i++ {
		_, err := wrapped.Generate(ctx, testModelRequest(i))
		require.NoError(t, err, "request %d within the daily cost quota should succeed", i)
	}
	_, err := wrapped.Generate(ctx, testModelRequest(2))
	require.ErrorIs(t, err, ErrRateLimited, "request over the daily cost quota must be rejected")
}

func TestModelProviderWrapper_UnlimitedPassesThrough(t *testing.T) {
	// No limits configured (all 0 = unlimited, the default for every
	// card): normal calls must pass through untouched.
	mock := successMock()
	config := &data.AIConfig{
		RateLimitPerSecond: 0,
		Burst:              0,
		DailyRequestQuota:  0,
		DailyCostQuota:     0,
	}
	wrapped := NewModelProviderWrapper(mock, config)

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		resp, err := wrapped.Generate(ctx, testModelRequest(i))
		require.NoError(t, err, "request %d should succeed (no limits configured)", i)
		require.NotNil(t, resp)
	}
	genCalls, _ := mock.calls()
	require.Equal(t, 5, genCalls)
}

func TestModelProviderWrapper_PassThroughError(t *testing.T) {
	mock := &mockProvider{
		name: "mock",
		resp: nil,
		err:  errors.New("some error"),
	}
	config := &data.AIConfig{
		RateLimitPerSecond: 0,
		Burst:              0,
		DailyRequestQuota:  0,
		DailyCostQuota:     0,
	}
	wrapped := NewModelProviderWrapper(mock, config)

	ctx := context.Background()
	resp, err := wrapped.Generate(ctx, testModelRequest(0))
	require.Error(t, err)
	require.ErrorContains(t, err, "some error")
	require.Nil(t, resp)
}

func TestModelProviderWrapper_StreamingUnlimited(t *testing.T) {
	mock := successMock()
	config := &data.AIConfig{
		RateLimitPerSecond: 0,
		Burst:              0,
		DailyRequestQuota:  0,
		DailyCostQuota:     0,
	}
	wrapped := NewModelProviderWrapper(mock, config)

	ctx := context.Background()
	resp, err := drainStream(t, wrapped.NewStreaming(ctx, testModelRequest(0)))
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "hello", resp.Message.Text())
}

func TestModelProviderWrapper_StreamingRateLimited(t *testing.T) {
	// Streaming must go through the same gate: previously NewStreaming
	// was not overridden and bypassed the limiter entirely.
	mock := successMock()
	config := &data.AIConfig{
		RateLimitPerSecond: 1,
		Burst:              1,
		DailyRequestQuota:  0,
		DailyCostQuota:     0,
	}
	wrapped := NewModelProviderWrapper(mock, config)

	ctx := context.Background()
	resp, err := drainStream(t, wrapped.NewStreaming(ctx, testModelRequest(0)))
	require.NoError(t, err, "first stream within the limit should succeed")
	require.NotNil(t, resp)

	resp, err = drainStream(t, wrapped.NewStreaming(ctx, testModelRequest(1)))
	require.ErrorIs(t, err, ErrRateLimited, "second immediate stream must be rate limited")
	require.Nil(t, resp)

	_, streamCalls := mock.calls()
	require.Equal(t, 1, streamCalls, "rejected stream must not reach the underlying provider")
}

func TestModelProviderWrapper_StreamingDailyQuota(t *testing.T) {
	mock := successMock()
	config := &data.AIConfig{
		RateLimitPerSecond: 0,
		Burst:              0,
		DailyRequestQuota:  1,
		DailyCostQuota:     0,
	}
	wrapped := NewModelProviderWrapper(mock, config)

	ctx := context.Background()
	_, err := drainStream(t, wrapped.NewStreaming(ctx, testModelRequest(0)))
	require.NoError(t, err)

	_, err = drainStream(t, wrapped.NewStreaming(ctx, testModelRequest(1)))
	require.ErrorIs(t, err, ErrRateLimited, "stream over the daily quota must be rejected")
}
