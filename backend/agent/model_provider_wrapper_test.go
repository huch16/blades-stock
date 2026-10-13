package agent

import (
	"context"
	"errors"
	"fmt"

	"blades-stock/backend/data"
	"github.com/go-kratos/blades"
	"github.com/stretchr/testify/require"
	"testing"
)

// mockProvider is a simple blades.ModelProvider implementation for testing.
type mockProvider struct {
	name    string
	resp    *blades.ModelResponse
	err     error
}

func (m *mockProvider) Name() string {
	return m.name
}
func (m *mockProvider) Generate(ctx context.Context, req *blades.ModelRequest) (*blades.ModelResponse, error) {
	return m.resp, m.err
}
func (m *mockProvider) NewStreaming(ctx context.Context, req *blades.ModelRequest) blades.Generator[*blades.ModelResponse, error] {
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

func TestModelProviderWrapper_RateLimit(t *testing.T) {
	// Prepare a mock provider that always succeeds
	mock := &mockProvider{
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
	// Config with rate limit: but now always allow (self-limiter removed)
	config := &data.AIConfig{
		RateLimitPerSecond: 1,
		Burst:              1,
		DailyRequestQuota:  0, // unlimited
		DailyCostQuota:     0,
	}
	wrapped := NewModelProviderWrapper(mock, config)

	ctx := context.Background()
	// All requests should succeed (rate limiter removed)
	for i := 0; i < 5; i++ {
		resp, err := wrapped.Generate(ctx, &blades.ModelRequest{
			Messages: []*blades.Message{
				{
					Role: blades.RoleUser,
					Parts: []blades.Part{
						blades.TextPart{Text: fmt.Sprintf("test%d", i)},
					},
				},
			},
		})
		require.NoError(t, err, "request %d should succeed (rate limiter removed)", i)
		require.NotNil(t, resp)
	}
}

func TestModelProviderWrapper_DailyQuota(t *testing.T) {
	mock := &mockProvider{
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
	config := &data.AIConfig{
		RateLimitPerSecond: 0, // unlimited
		Burst:              0,
		DailyRequestQuota:  2, // previously would allow only 2 per day, now unlimited
		DailyCostQuota:     0,
	}
	wrapped := NewModelProviderWrapper(mock, config)

	ctx := context.Background()
	// All requests should succeed (daily quota limiter removed)
	for i := 0; i < 5; i++ {
		resp, err := wrapped.Generate(ctx, &blades.ModelRequest{
			Messages: []*blades.Message{
				{
					Role: blades.RoleUser,
					Parts: []blades.Part{
						blades.TextPart{Text: "test"},
					},
				},
			},
		})
		require.NoError(t, err, "request %d should succeed (daily quota removed)", i)
		require.NotNil(t, resp)
	}
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
	resp, err := wrapped.Generate(ctx, &blades.ModelRequest{
		Messages: []*blades.Message{
			{
				Role: blades.RoleUser,
				Parts: []blades.Part{
					blades.TextPart{Text: "test"},
				},
			},
		},
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "some error")
	require.Nil(t, resp)
}