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
	cb, _ := newTestBreaker(Config{Name: "src", FailureThreshold: 3})

	for i := 0; i < 2; i++ {
		if !cb.Allow() {
			t.Fatalf("closed 状态应放行 (step %d)", i)
		}
		cb.Failure()
		if cb.State() != Closed {
			t.Fatalf("step %d: 未达阈值应保持 closed", i)
		}
	}
	if !cb.Allow() {
		t.Fatal("closed 状态应放行")
	}
	cb.Failure()
	if cb.State() != Open {
		t.Fatalf("达到阈值应断开, state=%v", cb.State())
	}
}

func TestOpenFastFailsUntilCooldown(t *testing.T) {
	cb, clock := newTestBreaker(Config{Name: "src", FailureThreshold: 2, Cooldown: 30 * time.Second})

	cb.Failure()
	cb.Failure()
	if cb.State() != Open {
		t.Fatalf("应断开, state=%v", cb.State())
	}

	// 冷却期内：快速失败，不放行。
	if cb.Allow() {
		t.Fatal("冷却期内不应放行")
	}
	// 冷却期内反复 Failure 不应刷新冷却起点（测试无 panic 且仍断开）。
	cb.Failure()
	cb.Failure()

	// 冷却未结束。
	clock.advance(29 * time.Second)
	if cb.Allow() {
		t.Fatal("冷却未结束不应放行")
	}

	// 冷却结束 → 半开放行一次探测。
	clock.advance(2 * time.Second)
	if !cb.Allow() {
		t.Fatal("冷却结束后应放行探测")
	}
	if cb.State() != HalfOpen {
		t.Fatalf("应为 half_open, state=%v", cb.State())
	}
	// 半开单探测：不再放行第二个。
	if cb.Allow() {
		t.Fatal("half_open 单探测不应并发放行")
	}
}

func TestHalfOpenProbeSuccessCloses(t *testing.T) {
	cb, clock := newTestBreaker(Config{Name: "src", FailureThreshold: 2, Cooldown: 10 * time.Second})
	cb.Failure()
	cb.Failure()
	if cb.State() != Open {
		t.Fatalf("应断开: %v", cb.State())
	}

	clock.advance(10 * time.Second)
	if !cb.Allow() {
		t.Fatal("应放行探测")
	}
	cb.Success()
	if cb.State() != Closed {
		t.Fatalf("探测成功应恢复 closed: %v", cb.State())
	}
	// 恢复后正常放行，失败计数已清零。
	if !cb.Allow() {
		t.Fatal("closed 应放行")
	}
}

func TestHalfOpenProbeFailureReopens(t *testing.T) {
	cb, clock := newTestBreaker(Config{Name: "src", FailureThreshold: 2, Cooldown: 10 * time.Second})
	cb.Failure()
	cb.Failure()
	clock.advance(10 * time.Second)
	if !cb.Allow() {
		t.Fatal("应放行探测")
	}
	cb.Failure()
	if cb.State() != Open {
		t.Fatalf("探测失败应立即再次断开: %v", cb.State())
	}
	// 新冷却期内不放行。
	if cb.Allow() {
		t.Fatal("再次断开后冷却期内不应放行")
	}
}

func TestSuccessResetsFailures(t *testing.T) {
	cb, _ := newTestBreaker(Config{Name: "src", FailureThreshold: 3})
	cb.Failure()
	cb.Failure()
	cb.Success()
	cb.Failure()
	cb.Failure()
	if cb.State() != Closed {
		t.Fatalf("成功应清零连续失败, state=%v", cb.State())
	}
	cb.Failure()
	if cb.State() != Open {
		t.Fatalf("再次累计达阈值应断开: %v", cb.State())
	}
}

func TestConcurrentSafety(t *testing.T) {
	cb := Get("race-test")
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if cb.Allow() {
				cb.Success()
			} else {
				cb.Failure()
			}
			_ = cb.State()
			_ = cb.Stats()
		}()
	}
	wg.Wait()
}

func TestRegistrySharedInstance(t *testing.T) {
	a := Get("registry-test")
	b := Get("registry-test")
	if a != b {
		t.Fatal("同名应返回同一实例")
	}
	Configure("registry-test", Config{Cooldown: time.Second})
	c := Get("registry-test")
	if c == a {
		t.Fatal("Configure 后应替换为新实例")
	}
}
