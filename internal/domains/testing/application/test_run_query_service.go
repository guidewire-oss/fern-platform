package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
)

// TestRunPager is the read-side dependency the application service
// expects from the infrastructure layer. Defined here so the service
// can be wired against a fake in unit tests without importing
// gorm or the database driver.
type TestRunPager interface {
	Query(ctx context.Context, filter domain.TestRunFilter, page domain.PageArgs) (domain.TestRunPage, error)
	ComputeFacets(ctx context.Context, filter domain.TestRunFilter) (domain.TestRunFacets, error)
}

// TestRunQueryService composes the pager with a facet cache, exposing
// the surface the v2 HTTP handler expects. Cache lookups happen on the
// computed filter key; cache misses fall through to ComputeFacets.
type TestRunQueryService struct {
	pager TestRunPager
	cache FacetCache
}

// NewTestRunQueryService wires a pager and a cache.
//
// In production, pager is the GORM-backed TestRunQueryRepo and cache
// is a Redis-backed implementation. In tests, pass NoopFacetCache and
// a stub pager.
func NewTestRunQueryService(p TestRunPager, c FacetCache) *TestRunQueryService {
	if c == nil {
		c = NoopFacetCache{}
	}
	return &TestRunQueryService{pager: p, cache: c}
}

// Query satisfies the v2 HTTP handler's contract.
func (s *TestRunQueryService) Query(ctx context.Context, filter domain.TestRunFilter, page domain.PageArgs) (domain.TestRunPage, error) {
	res, err := s.pager.Query(ctx, filter, page)
	if err != nil {
		return domain.TestRunPage{}, err
	}

	key := CacheKeyForFilter(filter)
	if facets, ok := s.cache.Get(ctx, key); ok {
		res.Facets = facets
		return res, nil
	}
	facets, err := s.pager.ComputeFacets(ctx, filter)
	if err != nil {
		// Facets are advisory; degrade gracefully rather than fail the page.
		return res, nil
	}
	s.cache.Set(ctx, key, facets)
	res.Facets = facets
	return res, nil
}

// CacheKeyForFilter derives a stable, opaque cache key from a filter.
//
// Stability matters: any two filters that produce the same query plan
// must hash to the same key, and changing one filter field must
// produce a different key. We get this by JSON-marshalling the filter
// (Go's encoding/json visits struct fields in declared order) and
// hashing the result.
//
// Opaqueness matters too: raw filter values may include internal IDs
// we do not want to leak via cache keys in logs or metrics, so we
// hash rather than serialize directly.
func CacheKeyForFilter(f domain.TestRunFilter) string {
	b, err := json.Marshal(filterKeyView(f))
	if err != nil {
		// json.Marshal cannot fail on a struct of basic types; fall back
		// to a constant key so a degenerate filter still produces something.
		return hex.EncodeToString(sha256.New().Sum(nil))
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// filterKeyView is a deterministic projection of TestRunFilter for
// keying. Slices are normalized: copied (not sorted, because order
// matters for OR-vs-AND semantics on `tags`); nil pointers serialize
// as null, which is what we want.
//
// We project rather than marshal the filter directly so future
// non-key fields can be added without invalidating cache entries.
func filterKeyView(f domain.TestRunFilter) any {
	return struct {
		ProjectIDs []string              `json:"p"`
		Status     []string              `json:"s"`
		Branches   []string              `json:"b"`
		Tags       []string              `json:"t"`
		TagMode    domain.LogicMode      `json:"tm"`
		GitCommit  *string               `json:"c"`
		Authors    []string              `json:"a"`
		DurationMs *domain.IntRange      `json:"d"`
		StartedAt  *domain.DateTimeRange `json:"st"`
		Search     *string               `json:"q"`
	}{
		ProjectIDs: f.ProjectIDs,
		Status:     f.Status,
		Branches:   f.Branches,
		Tags:       f.Tags,
		TagMode:    f.TagMode,
		GitCommit:  f.GitCommit,
		Authors:    f.Authors,
		DurationMs: f.DurationMs,
		StartedAt:  f.StartedAt,
		Search:     f.Search,
	}
}
