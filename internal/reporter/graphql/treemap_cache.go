package graphql

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/reporter/graphql/model"
)

// TreemapCache abstracts treemap-response caching so the resolver can
// run against either an in-process map (single-replica deploys, local
// dev) or a shared Redis instance (multi-replica production, so a
// user hitting different pods inside the 60s TTL sees consistent
// data). The interface is intentionally narrow: get and set, keyed by
// the resolver-computed cache key.
//
// Both impls swallow internal errors — caching is best-effort. A miss
// or marshalling failure just means the resolver re-aggregates.
type TreemapCache interface {
	get(ctx context.Context, key string) (*model.TreemapData, bool)
	set(ctx context.Context, key string, data *model.TreemapData)
}

// ---- In-memory backing (default) ------------------------------------------

// inMemoryTreemapCache is the original tiny TTL'd in-memory map. Kept
// for single-replica deploys and the test/dev path where Redis isn't
// available. Same TTL semantics as the previous treemapCache type.
//
// Keys are scoped by (drill_project_id, suite_name, days, user_id) by
// the resolver — access filtering depends on which projects the
// caller can see, so two users with different team memberships must
// not share entries.
type inMemoryTreemapCache struct {
	mu  sync.Mutex
	m   map[string]treemapEntry
	ttl time.Duration
}

type treemapEntry struct {
	data *model.TreemapData
	exp  time.Time
}

func newInMemoryTreemapCache(ttl time.Duration) *inMemoryTreemapCache {
	return &inMemoryTreemapCache{m: map[string]treemapEntry{}, ttl: ttl}
}

func (c *inMemoryTreemapCache) get(_ context.Context, key string) (*model.TreemapData, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok || time.Now().After(e.exp) {
		if ok {
			delete(c.m, key)
		}
		return nil, false
	}
	return e.data, true
}

func (c *inMemoryTreemapCache) set(_ context.Context, key string, v *model.TreemapData) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = treemapEntry{data: v, exp: time.Now().Add(c.ttl)}
}

// ---- Redis backing (multi-replica) ----------------------------------------

// TreemapCacheKeyPrefix namespaces all treemap-cache entries so they
// can be discovered, monitored, or flushed independently of other
// data sharing the Redis instance.
const TreemapCacheKeyPrefix = "fern:v2:treemap:"

// errTreemapCacheMiss is the sentinel a RedisLike implementation
// should return from Get to indicate a key was not present.
var errTreemapCacheMiss = errors.New("treemap cache: miss")

// TreemapRedisClient is the minimal Redis surface the treemap cache
// needs. Same shape as the facet cache's RedisLike — duplicated here
// to keep package boundaries clean (the graphql package doesn't
// depend on the testing/application package).
type TreemapRedisClient interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

// redisTreemapCache stores TreemapData in Redis under
// TreemapCacheKeyPrefix+<key>. Marshalling is JSON for human-readable
// inspection in redis-cli and to keep the payload portable.
//
// All Redis errors are swallowed: Get failures degrade to a cache
// miss (the resolver then recomputes), and Set failures are silently
// ignored (the next Get will recompute and re-cache).
type redisTreemapCache struct {
	client TreemapRedisClient
	ttl    time.Duration
}

func newRedisTreemapCache(client TreemapRedisClient, ttl time.Duration) *redisTreemapCache {
	return &redisTreemapCache{client: client, ttl: ttl}
}

func (c *redisTreemapCache) get(ctx context.Context, key string) (*model.TreemapData, bool) {
	raw, err := c.client.Get(ctx, TreemapCacheKeyPrefix+key)
	if err != nil {
		return nil, false
	}
	var out model.TreemapData
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, false
	}
	return &out, true
}

func (c *redisTreemapCache) set(ctx context.Context, key string, v *model.TreemapData) {
	if v == nil {
		return
	}
	payload, err := json.Marshal(v)
	if err != nil {
		return
	}
	_ = c.client.Set(ctx, TreemapCacheKeyPrefix+key, payload, c.ttl)
}
