package data

import (
	"testing"
	"time"
)

func TestKlineCacheGetSet(t *testing.T) {
	klineCache = make(map[string]*klineFallbackCacheEntry)
	defer func() { klineCache = make(map[string]*klineFallbackCacheEntry) }()

	if got := klineCacheGet("sh510300", "101", 30); got != nil {
		t.Fatal("expected empty cache on first read")
	}

	rows := []KLineData{{Day: "2026-08-01", Close: "4.5"}, {Day: "2026-07-31", Close: "4.4"}}
	klineCacheSet("sh510300", "101", &KLineSourceResult{Data: &rows, Source: "tdx-mac"}, 30)

	got := klineCacheGet("sh510300", "101", 30)
	if got == nil {
		t.Fatal("expected cache hit")
	}
	if got.Source != "tdx-mac" || len(*got.Data) != 2 || (*got.Data)[0].Close != "4.5" {
		t.Fatalf("unexpected cached result: %+v", got)
	}

	// larger limit than cached must MISS and refetch
	if got := klineCacheGet("sh510300", "101", 90); got != nil {
		t.Fatal("expected miss for larger limit")
	}

	// different code must MISS
	if got := klineCacheGet("sz159920", "101", 30); got != nil {
		t.Fatal("expected miss for different code")
	}

	// expired entry must MISS
	klineCache["sh510300|101"].fetchedAt = time.Now().Add(-klineCacheTTL - time.Second)
	if got := klineCacheGet("sh510300", "101", 30); got != nil {
		t.Fatal("expected miss after TTL expiry")
	}
}

func TestKlineCacheKeepsLargestLimit(t *testing.T) {
	klineCache = make(map[string]*klineFallbackCacheEntry)
	defer func() { klineCache = make(map[string]*klineFallbackCacheEntry) }()

	rows90 := make([]KLineData, 90)
	for i := range rows90 {
		rows90[i] = KLineData{Day: "d"}
	}
	klineCacheSet("sh510300", "101", &KLineSourceResult{Data: &rows90, Source: "sina"}, 90)

	rows30 := make([]KLineData, 30)
	klineCacheSet("sh510300", "101", &KLineSourceResult{Data: &rows30, Source: "tencent"}, 30)

	got := klineCacheGet("sh510300", "101", 90)
	if got == nil || len(*got.Data) != 90 {
		t.Fatalf("expected 90-bar cache retained, got %v", got)
	}
}
