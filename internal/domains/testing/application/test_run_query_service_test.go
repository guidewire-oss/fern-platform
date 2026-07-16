package application_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
