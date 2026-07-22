package application

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
)

// FacetCacheKeyPrefix namespaces all facet-cache entries so they
// can be discovered, monitored, or flushed independently of other
// data in the same Redis instance.
const FacetCacheKeyPrefix = "fern:v2:facets:"

// ErrCacheMiss is the sentinel a RedisLike implementation should
// return from Get to indicate a key was not present. We surface it
// here so adapters built against a Redis client can call this
// package's sentinel rather than redefining one.
var ErrCacheMiss = errors.New("cache: miss")

// RedisLike is the minimal Redis surface the facet cache needs. The
// indirection lets the cache adapter be wired to whatever Redis
// client the platform standardizes on (go-redis, rueidis, etc.)
// without pulling that client's dependency tree into this package.
//
// Implementations must:
//   - Return ErrCacheMiss from Get for absent keys.
//   - Honor TTL on Set (Redis SETEX / SET EX semantics).
//   - Be safe for concurrent use.
type RedisLike interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

// RedisFacetCache stores TestRunFacets in Redis under
// FacetCacheKeyPrefix+<key>. Marshalling is JSON for human-readable
// inspection in redis-cli and to keep the payload portable.
//
// All Redis errors are swallowed: Get failures degrade to a cache
// miss (the application service then recomputes), and Set failures
// are silently ignored (the next Get will recompute and re-cache).
// This matches the "caching is best-effort" principle from RFC §15.
type RedisFacetCache struct {
	client RedisLike
	ttl    time.Duration
}

// NewRedisFacetCache wraps a RedisLike client with the FacetCache
// contract. Pass a ttl matching the data's freshness budget; the
// design recommends 60 s for facet counts on append-mostly data.
func NewRedisFacetCache(client RedisLike, ttl time.Duration) *RedisFacetCache {
	return &RedisFacetCache{client: client, ttl: ttl}
}

func (c *RedisFacetCache) Get(ctx context.Context, key string) (domain.TestRunFacets, bool) {
	raw, err := c.client.Get(ctx, FacetCacheKeyPrefix+key)
	if err != nil {
		return domain.TestRunFacets{}, false
	}
	var out domain.TestRunFacets
	if err := json.Unmarshal(raw, &out); err != nil {
		return domain.TestRunFacets{}, false
	}
	return out, true
}

func (c *RedisFacetCache) Set(ctx context.Context, key string, facets domain.TestRunFacets) {
	payload, err := json.Marshal(facets)
	if err != nil {
		return
	}
	_ = c.client.Set(ctx, FacetCacheKeyPrefix+key, payload, c.ttl)
}
