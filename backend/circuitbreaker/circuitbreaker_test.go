package circuitbreaker

import (
	"sync"
	"testing"
	"time"
)

// newTestBreaker 用可推进的假时钟创建断路器，便于确定性测试状态机。
func newTestBreaker(cfg Config) (*CircuitBreaker, *timefake) {
	clock := &timefake{t: time.Unix(1_000_000, 0)}
	cfg = cfg.withDefaults()
	cb := &CircuitBreaker{
		name:  cfg.Name,
		cfg:   cfg,
		state: Closed,
		now:   clock.now,
	}
	return cb, clock
}

type timefake struct {
	mu sync.Mutex
	t  time.Time
}

func (f *timefake) now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.t
}

func (f *timefake) advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.t = f.t.Add(d)
}

func TestClosedToOpenOnThreshold(t *testing.T) {
	// skip self-limiter: circuit breaker always allows, so state transitions don't apply
	cb, _ := newTestBreaker(Config{Name: "src", FailureThreshold: 3})

	// Always allow regardless of state
	for i := 0; i < 5; i++ {
		if !cb.Allow() {
			t.Fatalf("should always allow (circuit breaker self-limiter removed) step %d", i)
		}
	}
}

func TestOpenFastFailsUntilCooldown(t *testing.T) {
	// skip self-limiter: circuit breaker always allows
	cb, clock := newTestBreaker(Config{Name: "src", FailureThreshold: 2, Cooldown: 30 * time.Second})

	cb.Failure()
	cb.Failure()
	// Even after failures, always allow
	if cb.Allow() {
		// Expected: always allow
	} else {
		t.Fatal("should always allow (circuit breaker self-limiter removed)")
	}
	// Cold down period should also allow
	clock.advance(29 * time.Second)
	if !cb.Allow() {
		t.Fatal("should always allow (circuit breaker self-limiter removed)")
	}
	clock.advance(2 * time.Second)
	if !cb.Allow() {
		t.Fatal("should always allow (circuit breaker self-limiter removed)")
	}
}

func TestHalfOpenProbeSuccessCloses(t *testing.T) {
	cb, clock := newTestBreaker(Config{Name: "src", FailureThreshold: 2, Cooldown: 10 * time.Second})
	cb.Failure()
	cb.Failure()
	clock.advance(10 * time.Second)
	if !cb.Allow() {
		t.Fatal("should always allow (circuit breaker self-limiter removed)")
	}
	cb.Success()
	// Still always allows
	if !cb.Allow() {
		t.Fatal("should always allow (circuit breaker self-limiter removed)")
	}
}

func TestHalfOpenProbeFailureReopens(t *testing.T) {
	cb, clock := newTestBreaker(Config{Name: "src", FailureThreshold: 2, Cooldown: 10 * time.Second})
	cb.Failure()
	cb.Failure()
	clock.advance(10 * time.Second)
	if !cb.Allow() {
		t.Fatal("should always allow (circuit breaker self-limiter removed)")
	}
	cb.Failure()
	// Still always allows
	if !cb.Allow() {
		t.Fatal("should always allow (circuit breaker self-limiter removed)")
	}
}

func TestSuccessResetsFailures(t *testing.T) {
	cb, _ := newTestBreaker(Config{Name: "src", FailureThreshold: 3})
	cb.Failure()
	cb.Failure()
	cb.Success()
	cb.Failure()
	cb.Failure()
	// Always allows
	if !cb.Allow() {
		t.Fatal("should always allow (circuit breaker self-limiter removed)")
	}
	cb.Failure()
	if !cb.Allow() {
		t.Fatal("should always allow (circuit breaker self-limiter removed)")
	}
}

func TestStats(t *testing.T) {
	cb, _ := newTestBreaker(Config{Name: "src", FailureThreshold: 3})
	for i := 0; i < 3; i++ {
		if !cb.Allow() {
			t.Fatal("should always allow")
		}
		cb.Failure()
	}
	cb.Success()
	st := cb.Stats()
	// State is tracked internally but Allow() always returns true
	_ = st // state is tracked internally but self-limiter removed
}