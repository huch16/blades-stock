package agent

import (
	"context"
	"errors"
	"sync"
	"time"

	"blades-stock/backend/data"
	"github.com/go-kratos/blades"
	"golang.org/x/time/rate"
)

// ErrRateLimited is returned when a model provider's rate limit or quota is exceeded.
var ErrRateLimited = errors.New("model provider rate limit or quota exceeded")

// ModelProviderWrapper wraps a blades.ModelProvider with token bucket rate limiting and quota management.
type ModelProviderWrapper struct {
	blades.ModelProvider
	limiter      *rate.Limiter // token bucket for rate limiting
	quotaMu      sync.Mutex    // protects dailyQuota and lastResetDate
	dailyQuota   int           // requests used today
	lastResetDate string        // date string (YYYY-MM-DD) of last reset
	config       *data.AIConfig
}

// NewModelProviderWrapper returns a new wrapper around the given provider.
func NewModelProviderWrapper(provider blades.ModelProvider, config *data.AIConfig) blades.ModelProvider {
	if provider == nil {
		return nil
	}
	w := &ModelProviderWrapper{
		ModelProvider: provider,
		config:        config,
	}
	// Set up rate limiter if configured
	if config.RateLimitPerSecond > 0 {
		burst := config.Burst
		if burst <= 0 {
			burst = 1 // default burst
		}
		w.limiter = rate.NewLimiter(rate.Limit(config.RateLimitPerSecond), burst)
	}
	// Initialize quota tracking
	w.lastResetDate = time.Now().UTC().Format("2006-01-02")
	return w
}

// Generate wraps the provider's Generate method with rate limit and quota checks.
func (w *ModelProviderWrapper) Generate(ctx context.Context, req *blades.ModelRequest) (*blades.ModelResponse, error) {
	// Check rate limiter
	if w.limiter != nil {
		if !w.limiter.Allow() {
			return nil, ErrRateLimited
		}
	}
	// Check daily request quota
	if w.config.DailyRequestQuota > 0 {
		w.quotaMu.Lock()
		today := time.Now().UTC().Format("2006-01-02")
		if today != w.lastResetDate {
			// New day, reset quota
			w.dailyQuota = 0
			w.lastResetDate = today
		}
		if w.dailyQuota >= w.config.DailyRequestQuota {
			w.quotaMu.Unlock()
			return nil, ErrRateLimited
		}
		w.dailyQuota++
		w.quotaMu.Unlock()
	}
	// TODO: Implement daily cost quota tracking if needed.
	// For now, we skip cost tracking.

	// Call the underlying provider
	resp, err := w.ModelProvider.Generate(ctx, req)
	if err != nil {
		// If the underlying provider returns an error, we do not refund the quota.
		// This is conservative: we consider the request as attempted.
		return nil, err
	}
	// On success, quota has already been incremented above.
	return resp, nil
}