package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
)

// fakeRedis is a minimal in-process stand-in for a Redis SET/GET with
// TTL. It satisfies application.RedisLike so the cache adapter can be
// tested without bringing in a real client or miniredis.
type fakeRedis struct {
	mu      sync.Mutex
	values  map[string]fakeEntry
	getErr  error
	setErr  error
	getHits int
	setHits int
}

type fakeEntry struct {
	val []byte
	exp time.Time
}

func newFakeRedis() *fakeRedis {
	return &fakeRedis{values: map[string]fakeEntry{}}
}

func (f *fakeRedis) Get(_ context.Context, key string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getHits++
	if f.getErr != nil {
		return nil, f.getErr
	}
	e, ok := f.values[key]
	if !ok {
		return nil, application.ErrCacheMiss
	}
	if !e.exp.IsZero() && time.Now().After(e.exp) {
		delete(f.values, key)
		return nil, application.ErrCacheMiss
	}
	return e.val, nil
}

func (f *fakeRedis) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setHits++
	if f.setErr != nil {
		return f.setErr
	}
	exp := time.Time{}
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	f.values[key] = fakeEntry{val: value, exp: exp}
	return nil
}

func TestRedisCache_RoundTrip(t *testing.T) {
	r := newFakeRedis()
	c := application.NewRedisFacetCache(r, time.Minute)
	ctx := context.Background()

	if _, ok := c.Get(ctx, "k"); ok {
		t.Fatal("unexpected hit on empty cache")
	}
	c.Set(ctx, "k", sampleFacets())
	got, ok := c.Get(ctx, "k")
	if !ok {
		t.Fatal("expected hit after Set")
	}
	if len(got.ByStatus) != 1 || got.ByStatus[0].Value != "failed" {
		t.Errorf("payload mangled: %+v", got)
	}
}

func TestRedisCache_PrefixesKey(t *testing.T) {
	r := newFakeRedis()
	c := application.NewRedisFacetCache(r, time.Minute)
	c.Set(context.Background(), "abc", sampleFacets())

	r.mu.Lock()
	defer r.mu.Unlock()
	found := false
	for k := range r.values {
		if k == application.FacetCacheKeyPrefix+"abc" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected key with prefix %q, got %+v", application.FacetCacheKeyPrefix, r.values)
	}
}

func TestRedisCache_GetMissOnRedisError(t *testing.T) {
	r := newFakeRedis()
	r.getErr = errors.New("redis down")
	c := application.NewRedisFacetCache(r, time.Minute)

	// Redis being down must not poison the response — caller treats it
	// as a miss and falls back to computing facets directly.
	if _, ok := c.Get(context.Background(), "k"); ok {
		t.Fatal("Redis error should report a miss, not an erroneous hit")
	}
}

func TestRedisCache_SetSwallowsWriteFailure(t *testing.T) {
	r := newFakeRedis()
	r.setErr = errors.New("redis full")
	c := application.NewRedisFacetCache(r, time.Minute)

	// Set must not panic or surface the error — caching is best-effort.
	c.Set(context.Background(), "k", sampleFacets())
	if r.setHits != 1 {
		t.Errorf("expected one Set attempt, got %d", r.setHits)
	}
}

func TestRedisCache_GetWithGarbageReturnsMiss(t *testing.T) {
	r := newFakeRedis()
	// Pre-populate with bad bytes that aren't valid JSON.
	_ = r.Set(context.Background(), application.FacetCacheKeyPrefix+"k", []byte("not-json{"), time.Minute)

	c := application.NewRedisFacetCache(r, time.Minute)
	if _, ok := c.Get(context.Background(), "k"); ok {
		t.Fatal("garbage payload should produce a miss")
	}
}

// Sanity: the cache adapter satisfies the FacetCache interface.
func TestRedisCache_ImplementsInterface(t *testing.T) {
	var _ application.FacetCache = application.NewRedisFacetCache(newFakeRedis(), time.Minute)

	// And the marshalled payload round-trips through JSON cleanly.
	b, err := json.Marshal(domain.TestRunFacets{ByStatus: []domain.FacetCount{{Value: "x", Count: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	var back domain.TestRunFacets
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.ByStatus[0].Value != "x" {
		t.Errorf("round trip mangled facets: %+v", back)
	}
}
