package application_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
)

func sampleFacets() domain.TestRunFacets {
	return domain.TestRunFacets{
		ByStatus: []domain.FacetCount{{Value: "failed", Count: 3}},
	}
}

func TestMemoryCache_GetMissThenHit(t *testing.T) {
	c := application.NewMemoryFacetCache(time.Minute)
	ctx := context.Background()
	key := "k1"

	if _, ok := c.Get(ctx, key); ok {
		t.Fatal("expected cache miss on empty cache")
	}
	c.Set(ctx, key, sampleFacets())
	got, ok := c.Get(ctx, key)
	if !ok {
		t.Fatal("expected cache hit after Set")
	}
	if len(got.ByStatus) != 1 || got.ByStatus[0].Value != "failed" {
		t.Errorf("cached payload mangled: %+v", got)
	}
}

func TestMemoryCache_TTLExpiry(t *testing.T) {
	c := application.NewMemoryFacetCache(20 * time.Millisecond)
	ctx := context.Background()
	c.Set(ctx, "k", sampleFacets())
	time.Sleep(40 * time.Millisecond)
	if _, ok := c.Get(ctx, "k"); ok {
		t.Fatal("expected entry to expire")
	}
}

func TestMemoryCache_OverwriteResetsTTL(t *testing.T) {
	c := application.NewMemoryFacetCache(50 * time.Millisecond)
	ctx := context.Background()
	c.Set(ctx, "k", sampleFacets())
	time.Sleep(30 * time.Millisecond)
	c.Set(ctx, "k", sampleFacets()) // resets ttl
	time.Sleep(30 * time.Millisecond)
	if _, ok := c.Get(ctx, "k"); !ok {
		t.Fatal("entry should still be alive after overwrite")
	}
}

func TestMemoryCache_ConcurrentReadsSafe(t *testing.T) {
	c := application.NewMemoryFacetCache(time.Minute)
	ctx := context.Background()
	c.Set(ctx, "k", sampleFacets())

	var hits int64
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok := c.Get(ctx, "k"); ok {
				atomic.AddInt64(&hits, 1)
			}
		}()
	}
	wg.Wait()
	if hits != 100 {
		t.Errorf("expected 100 concurrent hits, got %d", hits)
	}
}

func TestNoopCache_AlwaysMisses(t *testing.T) {
	c := application.NoopFacetCache{}
	ctx := context.Background()
	c.Set(ctx, "k", sampleFacets())
	if _, ok := c.Get(ctx, "k"); ok {
		t.Fatal("noop cache must never report a hit")
	}
}
