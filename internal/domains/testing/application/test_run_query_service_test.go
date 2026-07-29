package application_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"slices"
	"testing"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
)

type stubRepo struct {
	page       domain.TestRunPage
	calls      int
	gotFilter  domain.TestRunFilter
	facetCalls int
	facets     domain.TestRunFacets
}

func (s *stubRepo) Query(_ context.Context, f domain.TestRunFilter, _ domain.PageArgs) (domain.TestRunPage, error) {
	s.calls++
	s.gotFilter = f
	return s.page, nil
}
func (s *stubRepo) ComputeFacets(_ context.Context, _ domain.TestRunFilter) (domain.TestRunFacets, error) {
	s.facetCalls++
	return s.facets, nil
}

func TestQueryService_PopulatesFacetsFromRepo(t *testing.T) {
	repo := &stubRepo{
		page:   domain.TestRunPage{TotalCount: 1},
		facets: domain.TestRunFacets{ByStatus: []domain.FacetCount{{Value: "failed", Count: 3}}},
	}
	svc := application.NewTestRunQueryService(repo, application.NoopFacetCache{})

	res, err := svc.Query(context.Background(), domain.TestRunFilter{}, domain.PageArgs{First: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Facets.ByStatus) != 1 || res.Facets.ByStatus[0].Value != "failed" {
		t.Errorf("facets not populated: %+v", res.Facets)
	}
	if repo.facetCalls != 1 {
		t.Errorf("expected one facet computation, got %d", repo.facetCalls)
	}
}

func TestQueryService_CacheServesSubsequentRequests(t *testing.T) {
	repo := &stubRepo{
		facets: domain.TestRunFacets{ByStatus: []domain.FacetCount{{Value: "passed", Count: 7}}},
	}
	cache := application.NewMemoryFacetCache(60_000_000_000) // 60s
	svc := application.NewTestRunQueryService(repo, cache)

	_, err := svc.Query(context.Background(), domain.TestRunFilter{ProjectIDs: []string{"p1"}}, domain.PageArgs{First: 10})
	if err != nil {
		t.Fatal(err)
	}
	// Second call with the same filter must not recompute facets.
	_, err = svc.Query(context.Background(), domain.TestRunFilter{ProjectIDs: []string{"p1"}}, domain.PageArgs{First: 10})
	if err != nil {
		t.Fatal(err)
	}
	if repo.facetCalls != 1 {
		t.Errorf("second call should be cache hit; got %d facet calls", repo.facetCalls)
	}
}

func TestQueryService_CacheKeyIsFilterShape(t *testing.T) {
	// Different filters must produce different cache keys.
	a := application.CacheKeyForFilter(domain.TestRunFilter{ProjectIDs: []string{"p1"}})
	b := application.CacheKeyForFilter(domain.TestRunFilter{ProjectIDs: []string{"p2"}})
	if a == b {
		t.Errorf("distinct filters produced identical cache key: %q", a)
	}
	// Same filter must be stable across calls (no map iteration noise).
	c := application.CacheKeyForFilter(domain.TestRunFilter{ProjectIDs: []string{"p1"}})
	if a != c {
		t.Errorf("same filter produced different keys: %q vs %q", a, c)
	}
}

func TestCacheKey_IsAHashNotPlaintext(t *testing.T) {
	k := application.CacheKeyForFilter(domain.TestRunFilter{ProjectIDs: []string{"sensitive-internal"}})
	// Sanity: the project ID itself should not appear in the key.
	if containsString(k, "sensitive-internal") {
		t.Error("cache key should be hashed, not contain raw filter values")
	}
	// And the key should look like a hex digest (sha256 prefix length).
	if _, err := hex.DecodeString(k); err != nil {
		t.Errorf("cache key %q should be hex-encoded: %v", k, err)
	}
	if len(k) != 2*sha256.Size {
		t.Errorf("cache key length=%d, want %d (sha256 hex)", len(k), 2*sha256.Size)
	}
}

func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (indexOf(haystack, needle) >= 0)
}
func indexOf(h, n string) int {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}

// --- project name enrichment -----------------------------------------

type stubNameResolver struct {
	names  map[string]string
	err    error
	calls  int
	gotIDs []string
}

func (s *stubNameResolver) NamesByProjectID(_ context.Context, ids []string) (map[string]string, error) {
	s.calls++
	s.gotIDs = append([]string(nil), ids...)
	if s.err != nil {
		return nil, s.err
	}
	return s.names, nil
}

func pageWithRuns(projectIDs ...string) domain.TestRunPage {
	edges := make([]domain.TestRunEdge, 0, len(projectIDs))
	for _, p := range projectIDs {
		edges = append(edges, domain.TestRunEdge{Node: &domain.TestRun{ProjectID: p}})
	}
	return domain.TestRunPage{Edges: edges, TotalCount: int64(len(edges))}
}

// Requirement 1.1/1.2: every node carries the resolved display name.
func TestQueryService_EnrichesNodesWithProjectName(t *testing.T) {
	repo := &stubRepo{page: pageWithRuns("proj-a", "proj-b")}
	res := &stubNameResolver{names: map[string]string{"proj-a": "Alpha", "proj-b": "Beta"}}
	svc := application.NewTestRunQueryService(repo, application.NoopFacetCache{}).
		WithProjectNames(res)

	out, err := svc.Query(context.Background(), domain.TestRunFilter{}, domain.PageArgs{First: 50})
	if err != nil {
		t.Fatal(err)
	}
	if got := out.Edges[0].Node.ProjectName; got != "Alpha" {
		t.Errorf("edge 0 ProjectName = %q, want Alpha", got)
	}
	if got := out.Edges[1].Node.ProjectName; got != "Beta" {
		t.Errorf("edge 1 ProjectName = %q, want Beta", got)
	}
}

// Requirement 1.3: an unresolved ID leaves the name empty so the UI can
// fall back to the raw project_id.
func TestQueryService_LeavesUnknownProjectNameEmpty(t *testing.T) {
	repo := &stubRepo{page: pageWithRuns("proj-a", "proj-missing")}
	res := &stubNameResolver{names: map[string]string{"proj-a": "Alpha"}}
	svc := application.NewTestRunQueryService(repo, application.NoopFacetCache{}).
		WithProjectNames(res)

	out, err := svc.Query(context.Background(), domain.TestRunFilter{}, domain.PageArgs{First: 50})
	if err != nil {
		t.Fatal(err)
	}
	if got := out.Edges[1].Node.ProjectName; got != "" {
		t.Errorf("unknown project name = %q, want empty", got)
	}
	if got := out.Edges[1].Node.ProjectID; got != "proj-missing" {
		t.Errorf("ProjectID mutated: %q", got)
	}
}

// Requirement 2.1/2.2: the project facet gains a label while its value
// stays the project_id that the filter round-trips.
func TestQueryService_LabelsProjectFacet(t *testing.T) {
	repo := &stubRepo{
		page: pageWithRuns("proj-a"),
		facets: domain.TestRunFacets{
			ByProject: []domain.FacetCount{{Value: "proj-a", Count: 4}},
			ByStatus:  []domain.FacetCount{{Value: "failed", Count: 4}},
			ByBranch:  []domain.FacetCount{{Value: "main", Count: 4}},
		},
	}
	res := &stubNameResolver{names: map[string]string{"proj-a": "Alpha"}}
	svc := application.NewTestRunQueryService(repo, application.NoopFacetCache{}).
		WithProjectNames(res)

	out, err := svc.Query(context.Background(), domain.TestRunFilter{}, domain.PageArgs{First: 50})
	if err != nil {
		t.Fatal(err)
	}
	pf := out.Facets.ByProject[0]
	if pf.Value != "proj-a" {
		t.Errorf("facet value = %q, want proj-a (filters key on the ID)", pf.Value)
	}
	if pf.Label != "Alpha" {
		t.Errorf("facet label = %q, want Alpha", pf.Label)
	}
	// Requirement 2.4: other facets are untouched.
	if out.Facets.ByStatus[0].Label != "" {
		t.Errorf("status facet should have no label, got %q", out.Facets.ByStatus[0].Label)
	}
	if out.Facets.ByBranch[0].Label != "" {
		t.Errorf("branch facet should have no label, got %q", out.Facets.ByBranch[0].Label)
	}
}

// A facet can reference a project that has no run on the current page;
// its label must still resolve, and it must come from the same single
// lookup as the page's own IDs.
func TestQueryService_ResolvesFacetOnlyProjectsInOneLookup(t *testing.T) {
	repo := &stubRepo{
		page: pageWithRuns("proj-a", "proj-a"),
		facets: domain.TestRunFacets{
			ByProject: []domain.FacetCount{
				{Value: "proj-a", Count: 2},
				{Value: "proj-elsewhere", Count: 9},
			},
		},
	}
	res := &stubNameResolver{names: map[string]string{
		"proj-a":         "Alpha",
		"proj-elsewhere": "Elsewhere",
	}}
	svc := application.NewTestRunQueryService(repo, application.NoopFacetCache{}).
		WithProjectNames(res)

	out, err := svc.Query(context.Background(), domain.TestRunFilter{}, domain.PageArgs{First: 50})
	if err != nil {
		t.Fatal(err)
	}
	if out.Facets.ByProject[1].Label != "Elsewhere" {
		t.Errorf("facet-only label = %q, want Elsewhere", out.Facets.ByProject[1].Label)
	}
	if res.calls != 1 {
		t.Errorf("resolver called %d times, want 1", res.calls)
	}
	// The service collects ids plainly and lets the resolver de-duplicate
	// (it owns that contract); what matters here is that the facet-only
	// project reached it in the same single call.
	if !slices.Contains(res.gotIDs, "proj-elsewhere") {
		t.Errorf("resolver got %v, want it to include the facet-only id", res.gotIDs)
	}
}

// Requirement 1.5: names are advisory. A resolver failure must not fail
// the page.
func TestQueryService_ToleratesNameResolverError(t *testing.T) {
	repo := &stubRepo{page: pageWithRuns("proj-a")}
	res := &stubNameResolver{err: errors.New("db down")}
	svc := application.NewTestRunQueryService(repo, application.NoopFacetCache{}).
		WithProjectNames(res)

	out, err := svc.Query(context.Background(), domain.TestRunFilter{}, domain.PageArgs{First: 50})
	if err != nil {
		t.Fatalf("resolver failure should not fail the query: %v", err)
	}
	if len(out.Edges) != 1 {
		t.Fatalf("expected the page to survive, got %d edges", len(out.Edges))
	}
	if out.Edges[0].Node.ProjectName != "" {
		t.Errorf("ProjectName = %q, want empty", out.Edges[0].Node.ProjectName)
	}
}

// A service with no resolver keeps its pre-feature behaviour.
func TestQueryService_WithoutResolverLeavesNamesEmpty(t *testing.T) {
	repo := &stubRepo{
		page:   pageWithRuns("proj-a"),
		facets: domain.TestRunFacets{ByProject: []domain.FacetCount{{Value: "proj-a", Count: 1}}},
	}
	svc := application.NewTestRunQueryService(repo, application.NoopFacetCache{})

	out, err := svc.Query(context.Background(), domain.TestRunFilter{}, domain.PageArgs{First: 50})
	if err != nil {
		t.Fatal(err)
	}
	if out.Edges[0].Node.ProjectName != "" {
		t.Errorf("ProjectName = %q, want empty", out.Edges[0].Node.ProjectName)
	}
	if out.Facets.ByProject[0].Label != "" {
		t.Errorf("Label = %q, want empty", out.Facets.ByProject[0].Label)
	}
}

// Labels are applied after the facet cache read, so a rename shows up
// without waiting for the cached facet set to expire.
func TestQueryService_LabelsCachedFacets(t *testing.T) {
	repo := &stubRepo{
		page:   pageWithRuns("proj-a"),
		facets: domain.TestRunFacets{ByProject: []domain.FacetCount{{Value: "proj-a", Count: 1}}},
	}
	cache := application.NewMemoryFacetCache(60_000_000_000)
	res := &stubNameResolver{names: map[string]string{"proj-a": "Alpha"}}
	svc := application.NewTestRunQueryService(repo, cache).WithProjectNames(res)

	if _, err := svc.Query(context.Background(), domain.TestRunFilter{}, domain.PageArgs{First: 50}); err != nil {
		t.Fatal(err)
	}
	// Second call is served from the facet cache.
	res.names = map[string]string{"proj-a": "Alpha Renamed"}
	out, err := svc.Query(context.Background(), domain.TestRunFilter{}, domain.PageArgs{First: 50})
	if err != nil {
		t.Fatal(err)
	}
	if repo.facetCalls != 1 {
		t.Fatalf("expected the facet cache to serve call 2, got %d computations", repo.facetCalls)
	}
	if got := out.Facets.ByProject[0].Label; got != "Alpha Renamed" {
		t.Errorf("cached facet label = %q, want the fresh name", got)
	}
}

// Labels must not be written into the slice the facet cache holds: the
// cache hands every caller the same backing array, so mutating it in
// place is a data race between concurrent requests and bakes one
// request's labels into another's cached result.
func TestQueryService_DoesNotMutateCachedFacets(t *testing.T) {
	repo := &stubRepo{
		page:   pageWithRuns("proj-a"),
		facets: domain.TestRunFacets{ByProject: []domain.FacetCount{{Value: "proj-a", Count: 1}}},
	}
	cache := application.NewMemoryFacetCache(60_000_000_000)
	res := &stubNameResolver{names: map[string]string{"proj-a": "Alpha"}}
	svc := application.NewTestRunQueryService(repo, cache).WithProjectNames(res)

	if _, err := svc.Query(context.Background(), domain.TestRunFilter{}, domain.PageArgs{First: 50}); err != nil {
		t.Fatal(err)
	}
	cached, ok := cache.Get(context.Background(), application.CacheKeyForFilter(domain.TestRunFilter{}))
	if !ok {
		t.Fatal("expected a cached facet set")
	}
	if got := cached.ByProject[0].Label; got != "" {
		t.Errorf("cached facet was mutated: Label = %q, want empty", got)
	}
}

// With the resolver failing, a cache hit must not serve labels left
// behind by an earlier successful request.
func TestQueryService_CacheHitWithFailingResolverHasNoStaleLabels(t *testing.T) {
	repo := &stubRepo{
		page:   pageWithRuns("proj-a"),
		facets: domain.TestRunFacets{ByProject: []domain.FacetCount{{Value: "proj-a", Count: 1}}},
	}
	cache := application.NewMemoryFacetCache(60_000_000_000)
	res := &stubNameResolver{names: map[string]string{"proj-a": "Alpha"}}
	svc := application.NewTestRunQueryService(repo, cache).WithProjectNames(res)

	if _, err := svc.Query(context.Background(), domain.TestRunFilter{}, domain.PageArgs{First: 50}); err != nil {
		t.Fatal(err)
	}
	res.err = errors.New("db down")
	out, err := svc.Query(context.Background(), domain.TestRunFilter{}, domain.PageArgs{First: 50})
	if err != nil {
		t.Fatal(err)
	}
	if got := out.Facets.ByProject[0].Label; got != "" {
		t.Errorf("stale label served from cache: %q, want empty", got)
	}
}

// The authorization boundary must be part of the facet cache key.
// Without it, two users with different readable project sets would share
// a cached facet set — one team's project list served to another.
func TestCacheKeyForFilter_VariesByAuthorizationScope(t *testing.T) {
	a := application.CacheKeyForFilter(domain.TestRunFilter{
		AllowedProjectIDs: []string{"p1"},
	})
	b := application.CacheKeyForFilter(domain.TestRunFilter{
		AllowedProjectIDs: []string{"p2"},
	})
	unscoped := application.CacheKeyForFilter(domain.TestRunFilter{})
	if a == b {
		t.Error("callers with different readable projects share a cache key")
	}
	if a == unscoped {
		t.Error("a scoped caller shares the unrestricted (admin) cache key")
	}
}
