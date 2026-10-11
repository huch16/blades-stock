package agent

import (
	"context"
	"errors"
	"time"

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
	// Config with low rate limit: 1 per second, burst 1
	config := &data.AIConfig{
		RateLimitPerSecond: 1,
		Burst:              1,
		DailyRequestQuota:  0, // unlimited
		DailyCostQuota:     0,
	}
	wrapped := NewModelProviderWrapper(mock, config)

	ctx := context.Background()
	// First request should succeed
	resp1, err1 := wrapped.Generate(ctx, &blades.ModelRequest{
		Messages: []*blades.Message{
			{
				Role: blades.RoleUser,
				Parts: []blades.Part{
					blades.TextPart{Text: "test"},
				},
			},
		},
	})
	require.NoError(t, err1)
	require.NotNil(t, resp1)

	// Second request immediately should be rate limited
	resp2, err2 := wrapped.Generate(ctx, &blades.ModelRequest{
		Messages: []*blades.Message{
			{
				Role: blades.RoleUser,
				Parts: []blades.Part{
					blades.TextPart{Text: "test2"},
				},
			},
		},
	})
	require.Error(t, err2)
	require.ErrorIs(t, err2, ErrRateLimited)
	require.Nil(t, resp2)

	// Wait a bit for the limiter to replenish (token bucket adds 1 token per second)
	time.Sleep(time.Second + 100*time.Millisecond)

	// Third request should succeed again
	resp3, err3 := wrapped.Generate(ctx, &blades.ModelRequest{
		Messages: []*blades.Message{
			{
				Role: blades.RoleUser,
				Parts: []blades.Part{
					blades.TextPart{Text: "test3"},
				},
			},
		},
	})
	require.NoError(t, err3)
	require.NotNil(t, resp3)
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
		DailyRequestQuota:  2, // allow only 2 per day
		DailyCostQuota:     0,
	}
	wrapped := NewModelProviderWrapper(mock, config)

	ctx := context.Background()
	// First two requests should succeed
	for i := 0; i < 2; i++ {
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
		require.NoError(t, err)
		require.NotNil(t, resp)
	}
	// Third request should be quota exceeded
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
	require.ErrorIs(t, err, ErrRateLimited)
	require.Nil(t, resp)
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