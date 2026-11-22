package agent

import (
	"context"
	"errors"
	"sync"
	"time"

	"blades-stock/backend/data"
	"github.com/go-kratos/blades"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"golang.org/x/time/rate"
)

// ErrRateLimited is returned when a model provider's rate limit or quota is exceeded.
var ErrRateLimited = errors.New("model provider rate limit or quota exceeded")

// ModelProviderWrapper wraps a blades.ModelProvider with token bucket rate limiting and quota management.
type ModelProviderWrapper struct {
	blades.ModelProvider
	limiter       *rate.Limiter // token bucket for rate limiting
	quotaMu       sync.Mutex    // protects dailyQuota and lastResetDate
	dailyQuota    int           // requests used today
	dailyCostUSD  float64       // cost used today
	lastResetDate string        // date string (YYYY-MM-DD) of last reset
	config        *data.AIConfig
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
	w.dailyQuota = 0
	w.dailyCostUSD = 0
	w.lastResetDate = time.Now().UTC().Format("2006-01-02")
	return w
}

// estimateLLMCost provides a rough estimate of LLM request cost in USD.
// This is a placeholder implementation - in a production system, this would
// use actual token usage from the LLM response.
func estimateLLMCost(config *data.AIConfig, req *blades.ModelRequest) float64 {
	// Placeholder: return a fixed small cost per request
	// In practice, this should be based on actual token usage and model pricing
	return 0.001
}

// recordLLMQotaMetrics records LLM quota metrics for observability
func recordLLMQotaMetrics(ctx context.Context, dailyQuota int64, dailyCostUSD float64, requestQuota int64, costQuota float64) {
	initMetrics()
	if ctx == nil {
		ctx = context.Background()
	}

	attrs := []attribute.KeyValue{}

	if toolQuotaLimitGauge != nil {
		if requestQuota > 0 {
			toolQuotaLimitGauge.Record(ctx, requestQuota, metric.WithAttributes(append(attrs, attribute.String("type", "llm_request_quota"))...))
		}
		if costQuota > 0 {
			toolQuotaLimitGauge.Record(ctx, int64(costQuota), metric.WithAttributes(append(attrs, attribute.String("type", "llm_cost_quota"))...))
		}
	}

	if toolQuotaUsedGauge != nil {
		toolQuotaUsedGauge.Record(ctx, dailyQuota, metric.WithAttributes(append(attrs, attribute.String("type", "llm_request_used"))...))
		toolQuotaUsedGauge.Record(ctx, int64(dailyCostUSD*1000), metric.WithAttributes(append(attrs, attribute.String("type", "llm_cost_used_milli_usd"))...))
	}
}

// checkLimits enforces the card's configured rate limit and daily quotas.
//
// All checks are no-ops when the card leaves them at 0 (unlimited — the
// default for every card, see data/settings_api.go), so restoring
// enforcement cannot disturb existing normal calls: only cards with an
// explicitly configured limit can ever be rejected.
// A rejection returns ErrRateLimited and records the event for
// observability; callers must surface the error, never silently drop
// the request.
func (w *ModelProviderWrapper) checkLimits(ctx context.Context, req *blades.ModelRequest) error {
	if w.config == nil {
		return nil
	}
	// Check rate limiter
	if w.limiter != nil {
		if !w.limiter.Allow() {
			RecordQuotaExceeded(ctx, w.config.ModelName, "rate_limit")
			return ErrRateLimited
		}
	}
	// Check daily request quota
	if w.config.DailyRequestQuota > 0 {
		w.quotaMu.Lock()
		today := time.Now().UTC().Format("2006-01-02")
		if today != w.lastResetDate {
			// New day, reset quota
			w.dailyQuota = 0
			w.dailyCostUSD = 0
			w.lastResetDate = today
		}
		if w.dailyQuota >= w.config.DailyRequestQuota {
			w.quotaMu.Unlock()
			RecordQuotaExceeded(ctx, w.config.ModelName, "daily_request_quota")
			return ErrRateLimited
		}
		w.dailyQuota++
		// Record LLM quota metrics
		go recordLLMQotaMetrics(ctx, int64(w.dailyQuota), w.dailyCostUSD, int64(w.config.DailyRequestQuota), w.config.DailyCostQuota)
		w.quotaMu.Unlock()
	}
	// Check daily cost quota
	if w.config.DailyCostQuota > 0 {
		w.quotaMu.Lock()
		today := time.Now().UTC().Format("2006-01-02")
		if today != w.lastResetDate {
			// New day, reset cost tracking
			w.dailyQuota = 0
			w.dailyCostUSD = 0
			w.lastResetDate = today
		}
		// Estimate cost for this request (placeholder - in practice would use token usage)
		estimatedCost := estimateLLMCost(w.config, req)
		if w.dailyCostUSD+estimatedCost > w.config.DailyCostQuota {
			w.quotaMu.Unlock()
			RecordQuotaExceeded(ctx, w.config.ModelName, "daily_cost_quota")
			return ErrRateLimited
		}
		w.dailyCostUSD += estimatedCost
		// Record LLM quota metrics
		go recordLLMQotaMetrics(ctx, int64(w.dailyQuota), w.dailyCostUSD, int64(w.config.DailyRequestQuota), w.config.DailyCostQuota)
		w.quotaMu.Unlock()
	}
	return nil
}

// Generate wraps the provider's Generate method with rate limit and quota checks.
func (w *ModelProviderWrapper) Generate(ctx context.Context, req *blades.ModelRequest) (*blades.ModelResponse, error) {
	if err := w.checkLimits(ctx, req); err != nil {
		return nil, err
	}
	// Call the underlying provider
	resp, err := w.ModelProvider.Generate(ctx, req)
	if err == nil && resp != nil && resp.Message != nil {
		// Non-streaming responses carry real token usage (the streaming
		// adapter never populates Message.TokenUsage); record it when
		// present so the token metric is actually produced.
		if u := resp.Message.TokenUsage; u.TotalTokens > 0 {
			modelName := ""
			if w.config != nil {
				modelName = w.config.ModelName
			}
			if modelName == "" {
				modelName = w.Name()
			}
			RecordTokenUsage(ctx, modelName, u.InputTokens, u.OutputTokens)
		}
	}
	return resp, err
}

// NewStreaming mirrors Generate: the same rate-limit/quota gate applies
// before the stream starts, so a limited card cannot be bypassed by
// streaming (previously NewStreaming was not overridden at all and the
// embedded provider was passed through unchecked).
// The check runs once per stream, not per chunk; a rejection yields a
// single terminal error instead of silently dropping the request.
// Token usage is not recorded here: the streaming adapter never populates
// Message.TokenUsage (only the non-streaming path does).
func (w *ModelProviderWrapper) NewStreaming(ctx context.Context, req *blades.ModelRequest) blades.Generator[*blades.ModelResponse, error] {
	if err := w.checkLimits(ctx, req); err != nil {
		return func(yield func(*blades.ModelResponse, error) bool) {
			yield(nil, err)
		}
	}
	return w.ModelProvider.NewStreaming(ctx, req)
}
