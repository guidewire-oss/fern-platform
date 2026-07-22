package application

import (
	"context"
	"sync"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
)

// FacetCache caches the (often expensive) facet counts that accompany
// a test-run list query. The default TTL is short (~60 s) because the
// underlying data is append-mostly and users want fresh counts.
//
// Implementations must be safe for concurrent use.
type FacetCache interface {
	Get(ctx context.Context, key string) (domain.TestRunFacets, bool)
	Set(ctx context.Context, key string, facets domain.TestRunFacets)
}

// NoopFacetCache disables caching. Useful in tests, in single-instance
// development, or when Redis is intentionally not wired up.
type NoopFacetCache struct{}

func (NoopFacetCache) Get(context.Context, string) (domain.TestRunFacets, bool) {
	return domain.TestRunFacets{}, false
}
func (NoopFacetCache) Set(context.Context, string, domain.TestRunFacets) {}

// MemoryFacetCache is a process-local TTL cache. Suitable for a single
// fern-platform replica or as a fallback when Redis is unavailable.
// For multi-replica deployments wire a Redis-backed implementation
// behind the same interface so cache hits cross replicas.
type MemoryFacetCache struct {
	ttl     time.Duration
	mu      sync.RWMutex
	entries map[string]memEntry
}

type memEntry struct {
	facets domain.TestRunFacets
	exp    time.Time
}

// NewMemoryFacetCache returns a cache that expires entries after ttl.
func NewMemoryFacetCache(ttl time.Duration) *MemoryFacetCache {
	return &MemoryFacetCache{
		ttl:     ttl,
		entries: make(map[string]memEntry),
	}
}

func (c *MemoryFacetCache) Get(_ context.Context, key string) (domain.TestRunFacets, bool) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.exp) {
		return domain.TestRunFacets{}, false
	}
	return e.facets, true
}

func (c *MemoryFacetCache) Set(_ context.Context, key string, facets domain.TestRunFacets) {
	c.mu.Lock()
	c.entries[key] = memEntry{facets: facets, exp: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}
