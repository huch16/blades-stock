package quant

import (
	"sync"
	"testing"
	"time"
)

// TestTokenBucket_Allow 锁定令牌桶的基本兑换行为。
func TestTokenBucket_Allow(t *testing.T) {
	tb := NewTokenBucket(10.0, 10.0) // 每秒 10 次，桶容量 10

	// 桶初始满，前 10 次应全部允许
	for i := 0; i < 10; i++ {
		if !tb.Allow() {
			t.Errorf("第 %d 次 Allow() 应返回 true，桶中仍有令牌", i+1)
		}
	}
	// 第 11 次应被拒绝（桶空，且未等待补充）
	if tb.Allow() {
		t.Error("第 11 次 Allow() 应返回 false，桶中无令牌")
	}
}

// TestTokenBucket_Replenish 锁定令牌随时间补充。
func TestTokenBucket_Replenish(t *testing.T) {
	tb := NewTokenBucket(10.0, 10.0) // 每秒 10 个令牌

	// 把桶抽空
	for i := 0; i < 10; i++ {
		tb.Allow()
	}
	// 等待约 0.3 秒，应补充约 3 个令牌
	time.Sleep(350 * time.Millisecond)
	allowed := 0
	for i := 0; i < 5; i++ {
		if tb.Allow() {
			allowed++
		}
	}
	if allowed < 2 {
		t.Errorf("等待 350ms 后应至少补充 2 个令牌，实际允许 %d 次", allowed)
	}
}

// TestTokenBucket_Concurrent 锁定令牌桶在并发下的安全性。
func TestTokenBucket_Concurrent(t *testing.T) {
	tb := NewTokenBucket(100.0, 100.0)
	var wg sync.WaitGroup
	allowed := 0
	var mu sync.Mutex

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if tb.Allow() {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	// 并发 50 次，桶容量 100，全部应允许
	if allowed != 50 {
		t.Errorf("并发 50 次 Allow() 应全部允许，实际 %d 次", allowed)
	}
}

// TestKlineRateWait 锁定 KlineRateWait 不会 panic（限流器已初始化）。
func TestKlineRateWait(t *testing.T) {
	// 应该不阻塞且不 panic（桶初始有令牌）
	KlineRateWait()
}
