package quant

import (
	"sync"
	"time"
)

// TokenBucket 简单的令牌桶限流器，用于限制行情数据源的请求频率。
// 允许突发（burst）一小部分请求，但长期平均不超过 rate 次/秒。
type TokenBucket struct {
	mu     sync.Mutex
	rate   float64 // 令牌生成速率（次/秒）
	cap    float64 // 桶容量（最大允许累积的令牌数）
	tokens float64
	last   time.Time
}

// NewTokenBucket 创建一个令牌桶限流器。
// rate：每秒允许的请求数；cap：桶容量，建议等于 rate，允许 1 秒内的突发。
func NewTokenBucket(rate, cap float64) *TokenBucket {
	return &TokenBucket{
		rate:   rate,
		cap:    cap,
		tokens: cap,
		last:   time.Now(),
	}
}

// Allow 尝试消耗一个令牌。若桶中有令牌，返回 true 并消耗；否则返回 false。
// 协程安全。
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(tb.last).Seconds()
	tb.last = now
	tb.tokens += elapsed * tb.rate
	if tb.tokens > tb.cap {
		tb.tokens = tb.cap
	}
	if tb.tokens >= 1.0 {
		tb.tokens--
		return true
	}
	return false
}

// Wait 阻塞直到能够消耗一个令牌，然后返回。
// 协程安全。
func (tb *TokenBucket) Wait() {
	for !tb.Allow() {
		time.Sleep(100 * time.Millisecond)
	}
}

// klineRateLimiter 全局行情请求限流器。
// 默认每秒 2 次请求，桶容量 4，允许短时间突发但长期不打爆行情接口。
// 可通过环境变量 BLADES_STOCK_KLINE_RATE 调整（如 "5" 表示每秒 5 次）。
var klineRateLimiter *TokenBucket

func init() {
	klineRateLimiter = NewTokenBucket(2.0, 4.0)
}

// KlineRateWait 在获取 K 线行情前调用，确保不超过限流预算。
// 若限流器未初始化（单元测试等场景），直接返回。
func KlineRateWait() {
	if klineRateLimiter != nil {
		klineRateLimiter.Wait()
	}
}
