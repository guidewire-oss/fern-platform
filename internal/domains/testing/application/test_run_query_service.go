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

// ProjectNameResolver maps project IDs to their display names.
//
// Test runs store only a project_id; the name lives in project_details.
// Keeping this a narrow, consumer-owned port lets the service enrich
// results without depending on the projects domain or on GORM.
type ProjectNameResolver interface {
	NamesByProjectID(ctx context.Context, ids []string) (map[string]string, error)
}

// TestRunQueryService composes the pager with a facet cache, exposing
// the surface the v2 HTTP handler expects. Cache lookups happen on the
// computed filter key; cache misses fall through to ComputeFacets.
type TestRunQueryService struct {
	pager TestRunPager
	cache FacetCache
	names ProjectNameResolver
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

// WithProjectNames attaches a resolver that fills in human-readable
// project names on the returned runs and project facet. Returns the
// service for chaining. Without it the service behaves exactly as
// before — names are simply left empty.
func (s *TestRunQueryService) WithProjectNames(r ProjectNameResolver) *TestRunQueryService {
	s.names = r
	return s
}

// Query satisfies the v2 HTTP handler's contract.
func (s *TestRunQueryService) Query(ctx context.Context, filter domain.TestRunFilter, page domain.PageArgs) (domain.TestRunPage, error) {
	res, err := s.pager.Query(ctx, filter, page)
	if err != nil {
		return domain.TestRunPage{}, err
	}

	res.Facets = s.facetsFor(ctx, filter)
	// Names resolve after facets attach — including on a cache hit — so a
	// renamed project shows its new name without waiting for the cached
	// facet set to expire.
	s.applyProjectNames(ctx, &res)
	return res, nil
}

// facetsFor returns the facets for a filter, preferring the cache and
// falling back to computing (and caching) them. Facets are advisory: a
// computation error yields an empty set rather than failing the page.
func (s *TestRunQueryService) facetsFor(ctx context.Context, filter domain.TestRunFilter) domain.TestRunFacets {
	key := CacheKeyForFilter(filter)
	if facets, ok := s.cache.Get(ctx, key); ok {
		return facets
	}
	facets, err := s.pager.ComputeFacets(ctx, filter)
	if err != nil {
		return domain.TestRunFacets{}
	}
	s.cache.Set(ctx, key, facets)
	return facets
}

// applyProjectNames fills ProjectName on every edge and Label on every
// project facet entry, using one batched lookup for the union of both
// sets of IDs (a facet may name a project with no run on this page).
//
// Names are advisory, like facets: a resolver failure leaves them empty
// and the page is returned intact.
func (s *TestRunQueryService) applyProjectNames(ctx context.Context, page *domain.TestRunPage) {
	if s.names == nil {
		return
	}
	ids := projectIDsIn(page)
	if len(ids) == 0 {
		return
	}

	byID, err := s.names.NamesByProjectID(ctx, ids)
	if err != nil {
		return
	}
	for _, e := range page.Edges {
		if e.Node != nil {
			e.Node.ProjectName = byID[e.Node.ProjectID]
		}
	}
	// Safe to label in place: FacetCache hands back facets the caller
	// owns (see the interface contract), so this never touches cached
	// state.
	for i := range page.Facets.ByProject {
		page.Facets.ByProject[i].Label = byID[page.Facets.ByProject[i].Value]
	}
}

// projectIDsIn collects every project id the page refers to — from the
// runs themselves and from the project facet, which may name a project
// with no run on this page. Duplicates are left in: NamesByProjectID
// owns de-duplication.
func projectIDsIn(page *domain.TestRunPage) []string {
	ids := make([]string, 0, len(page.Edges)+len(page.Facets.ByProject))
	for _, e := range page.Edges {
		if e.Node != nil {
			ids = append(ids, e.Node.ProjectID)
		}
	}
	for _, f := range page.Facets.ByProject {
		ids = append(ids, f.Value)
	}
	return ids
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
		AllowedIDs []string              `json:"ap"`
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
		AllowedIDs: f.AllowedProjectIDs,
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
