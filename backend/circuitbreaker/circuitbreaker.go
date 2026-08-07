// Package circuitbreaker 提供一个轻量、线程安全的多数据源断路器：
// 某数据源连续失败达到阈值后自动"断开"（Open），在冷却期内所有请求快速失败并跳过该源，
// 冷却期后进入半开（HalfOpen）状态放行少量探测请求；探测成功则恢复（Closed），失败则再次断开。
//
// 用途：在多数据源容灾设计中吸收到 K 线降级链 / 交易日历等调用链，避免某个源持续故障时
// 每次请求都白白等待该源超时，造成请求雪崩。
package circuitbreaker

import (
	"fmt"
	"sync"
	"time"
)

// State 断路器状态。
type State int

const (
	// Closed 闭合：正常放行请求。
	Closed State = iota
	// Open 断开：快速失败，跳过该数据源。
	Open
	// HalfOpen 半开：冷却期过后放行少量探测请求验证数据源是否恢复。
	HalfOpen
)

func (s State) String() string {
	switch s {
	case Closed:
		return "closed"
	case Open:
		return "open"
	case HalfOpen:
		return "half_open"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// Config 断路器配置。
type Config struct {
	// Name 数据源名称，用于日志与状态展示。
	Name string
	// FailureThreshold 连续失败多少次后断开，默认 3。
	FailureThreshold int
	// Cooldown 断开后冷却时长，冷却结束进入半开，默认 30 秒。
	Cooldown time.Duration
	// HalfOpenMax 半开状态下最多同时放行的探测请求数，默认 1。
	HalfOpenMax int
}

func (c Config) withDefaults() Config {
	if c.FailureThreshold <= 0 {
		c.FailureThreshold = 3
	}
	if c.Cooldown <= 0 {
		c.Cooldown = 30 * time.Second
	}
	if c.HalfOpenMax <= 0 {
		c.HalfOpenMax = 1
	}
	return c
}

// CircuitBreaker 是单个数据源的断路器实例。
type CircuitBreaker struct {
	mu sync.Mutex

	name string
	cfg  Config

	state          State
	failures       int       // 当前连续失败数（Closed 累计）
	openAt         time.Time // 最近一次进入 Open 的时间
	halfOpenProbes int       // 半开状态下已放行的探测请求数
	halfOpenOK     int       // 半开状态下连续成功的探测数

	now func() time.Time // 可注入时钟便于测试
}

// New 创建断路器。
func New(cfg Config) *CircuitBreaker {
	cfg = cfg.withDefaults()
	return &CircuitBreaker{
		name:  cfg.Name,
		cfg:   cfg,
		state: Closed,
		now:   time.Now,
	}
}

// Allow 判断当前是否允许请求该数据源。
// 返回 false 时调用方应直接跳过该数据源（快速失败），且无需再调用 Failure。
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case Closed:
		return true
	case Open:
		if cb.now().Sub(cb.openAt) >= cb.cfg.Cooldown {
			// 冷却结束，转入半开并放行一个探测请求。
			cb.state = HalfOpen
			cb.halfOpenProbes = 0
			cb.halfOpenOK = 0
			cb.halfOpenProbes = 1
			return true
		}
		return false
	case HalfOpen:
		if cb.halfOpenProbes < cb.cfg.HalfOpenMax {
			cb.halfOpenProbes++
			return true
		}
		return false
	default:
		return false
	}
}

// Success 记录一次成功，应在数据源返回有效数据后调用。
func (cb *CircuitBreaker) Success() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case HalfOpen:
		cb.halfOpenOK++
		if cb.halfOpenOK >= 1 {
			cb.state = Closed
			cb.failures = 0
			cb.halfOpenProbes = 0
			cb.halfOpenOK = 0
		}
	case Closed:
		cb.failures = 0
	case Open:
		// 断开状态下通常不会真正发起请求，忽略成功上报。
	}
}

// Failure 记录一次失败，应在数据源超时/返回空/报错后调用。
func (cb *CircuitBreaker) Failure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	switch cb.state {
	case Closed:
		cb.failures++
		if cb.failures >= cb.cfg.FailureThreshold {
			cb.tripLocked()
		}
	case HalfOpen:
		// 半开探测失败，立即再次断开并重启冷却。
		cb.tripLocked()
	case Open:
		// 保持断开，冷却时间从首次断开起算，避免被持续失败反复刷新。
	}
}

// tripLocked 断开断路器（调用方须持有锁）。
func (cb *CircuitBreaker) tripLocked() {
	cb.state = Open
	cb.openAt = cb.now()
	cb.failures = 0
	cb.halfOpenProbes = 0
	cb.halfOpenOK = 0
}

// Reset 手动重置为闭合状态。
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = Closed
	cb.failures = 0
	cb.halfOpenProbes = 0
	cb.halfOpenOK = 0
}

// State 返回当前状态。
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// Stats 返回状态摘要（供日志/监控展示）。
type Stats struct {
	Name       string
	State      State
	Failures   int
	HalfOpenOK int
}

// Stats 返回状态摘要。
func (cb *CircuitBreaker) Stats() Stats {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return Stats{Name: cb.name, State: cb.state, Failures: cb.failures, HalfOpenOK: cb.halfOpenOK}
}

// ============================================================================
// 全局注册表：按数据源名称共享断路器实例
// ============================================================================

var (
	registryMu sync.RWMutex
	registry   = make(map[string]*CircuitBreaker)
)

// Get 返回指定数据源的断路器，首次调用时以默认配置创建。
func Get(name string) *CircuitBreaker {
	registryMu.RLock()
	cb, ok := registry[name]
	registryMu.RUnlock()
	if ok {
		return cb
	}

	registryMu.Lock()
	defer registryMu.Unlock()
	if cb, ok := registry[name]; ok {
		return cb
	}
	cb = New(Config{Name: name})
	registry[name] = cb
	return cb
}

// Configure 为指定数据源设置配置（首次生效；已存在的实例重置为闭合态）。
func Configure(name string, cfg Config) *CircuitBreaker {
	cfg.Name = name
	cfg = cfg.withDefaults()
	cb := New(cfg)

	registryMu.Lock()
	registry[name] = cb
	registryMu.Unlock()
	return cb
}
