package v2_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	apiv2 "github.com/guidewire-oss/fern-platform/internal/api/v2"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
)

// fakeScope is a canned ProjectScoper for handler tests.
type fakeScope struct {
	allowed      map[string]struct{}
	unrestricted bool
	err          error
}

func (f fakeScope) AccessibleProjects(_ *gin.Context) (map[string]struct{}, bool, error) {
	return f.allowed, f.unrestricted, f.err
}

func allow(ids ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		m[id] = struct{}{}
	}
	return m
}

// trackingSvc records whether Query ran and with what filter.
type trackingSvc struct {
	called bool
	filter domain.TestRunFilter
	page   domain.TestRunPage
}

func (s *trackingSvc) Query(_ context.Context, f domain.TestRunFilter, _ domain.PageArgs) (domain.TestRunPage, error) {
	s.called = true
	s.filter = f
	return s.page, nil
}

func scopedRunsRouter(t *testing.T, svc apiv2.TestRunQueryService, scope apiv2.ProjectScoper) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	apiv2.NewTestRunHandler(svc).WithScope(scope).Register(r.Group("/api/v2"))
	return r
}

func edgesLen(t *testing.T, body []byte) int {
	t.Helper()
	var resp struct {
		Edges []json.RawMessage `json:"edges"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, string(body))
	}
	return len(resp.Edges)
}

// A non-admin who asks only for a project they cannot access must get an
// empty result — and the query service must never be asked for it.
func TestScope_RunsForeignProjectReturnsEmpty(t *testing.T) {
	svc := &trackingSvc{page: domain.TestRunPage{
		Edges:      []domain.TestRunEdge{{Cursor: "c1", Node: &domain.TestRun{RunID: "r1"}}},
		TotalCount: 1,
	}}
	r := scopedRunsRouter(t, svc, fakeScope{allowed: allow("mine")})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v2/test-runs?project=other", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if n := edgesLen(t, w.Body.Bytes()); n != 0 {
		t.Errorf("expected 0 edges for forbidden project, got %d: %s", n, w.Body.String())
	}
	if svc.called {
		t.Error("query service must NOT run for a fully-forbidden request")
	}
}

// A non-admin with no project filter is scoped to exactly their accessible
// projects — never an unfiltered (all-teams) query.
func TestScope_RunsNoFilterDefaultsToAccessible(t *testing.T) {
	svc := &trackingSvc{}
	r := scopedRunsRouter(t, svc, fakeScope{allowed: allow("a", "b")})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v2/test-runs", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	if !svc.called {
		t.Fatal("query service should run")
	}
	got := append([]string(nil), svc.filter.ProjectIDs...)
	sort.Strings(got)
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("expected filter scoped to [a b], got %v", got)
	}
}

// A mix of accessible and forbidden project ids intersects down to only
// the accessible ones.
func TestScope_RunsMixedIntersects(t *testing.T) {
	svc := &trackingSvc{}
	r := scopedRunsRouter(t, svc, fakeScope{allowed: allow("mine")})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v2/test-runs?project=mine&project=other", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	if len(svc.filter.ProjectIDs) != 1 || svc.filter.ProjectIDs[0] != "mine" {
		t.Errorf("expected intersection [mine], got %v", svc.filter.ProjectIDs)
	}
}

// Admins are unrestricted: the filter passes through untouched.
func TestScope_RunsAdminPassthrough(t *testing.T) {
	svc := &trackingSvc{}
	r := scopedRunsRouter(t, svc, fakeScope{unrestricted: true})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v2/test-runs?project=any", nil))

	if !svc.called || len(svc.filter.ProjectIDs) != 1 || svc.filter.ProjectIDs[0] != "any" {
		t.Errorf("admin passthrough failed: called=%v ids=%v", svc.called, svc.filter.ProjectIDs)
	}
}

// A scoper error fails closed: no data, 5xx, and no query.
func TestScope_RunsErrorFailsClosed(t *testing.T) {
	svc := &trackingSvc{}
	r := scopedRunsRouter(t, svc, fakeScope{err: context.DeadlineExceeded})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v2/test-runs?project=mine", nil))

	if w.Code == http.StatusOK {
		t.Errorf("expected non-200 on scoper error, got 200: %s", w.Body.String())
	}
	if svc.called {
		t.Error("query service must NOT run when authorization fails")
	}
}

// --- trends -------------------------------------------------------------

type trackingTrendsSvc struct {
	called     bool
	projectIDs []string
	from, to   time.Time
}

func (s *trackingTrendsSvc) AggregateDailyByProjects(_ context.Context, ids []string, from time.Time, to time.Time) ([]*domain.DailyProjectAggregate, error) {
	s.called = true
	s.projectIDs = ids
	s.from, s.to = from, to
	return nil, nil
}

func scopedTrendsRouter(t *testing.T, svc apiv2.TestRunTrendsService, scope apiv2.ProjectScoper) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	apiv2.NewTestRunTrendsHandler(svc, func(*gin.Context) string { return "u1" }).
		WithScope(scope).Register(r.Group("/api/v2"))
	return r
}

// Trends must not aggregate a foreign project for a non-admin.
func TestScope_TrendsForeignProjectReturnsEmpty(t *testing.T) {
	svc := &trackingTrendsSvc{}
	r := scopedTrendsRouter(t, svc, fakeScope{allowed: allow("mine")})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v2/test-runs/trends?project=other", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if svc.called {
		t.Error("trends service must NOT aggregate a forbidden project")
	}
	var resp struct {
		Buckets map[string]json.RawMessage `json:"buckets"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Buckets) != 0 {
		t.Errorf("expected empty buckets, got %v", resp.Buckets)
	}
}

// Trends aggregates only the accessible subset of the requested projects.
func TestScope_TrendsIntersects(t *testing.T) {
	svc := &trackingTrendsSvc{}
	r := scopedTrendsRouter(t, svc, fakeScope{allowed: allow("mine")})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v2/test-runs/trends?project=mine&project=other", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	if !svc.called {
		t.Fatal("trends service should run for an accessible project")
	}
	if len(svc.projectIDs) != 1 || svc.projectIDs[0] != "mine" {
		t.Errorf("expected trends scoped to [mine], got %v", svc.projectIDs)
	}
}

// An explicit from/to range is forwarded verbatim to the service instead
// of the rolling days window.
func TestTrends_FromToRangeForwarded(t *testing.T) {
	svc := &trackingTrendsSvc{}
	r := scopedTrendsRouter(t, svc, fakeScope{unrestricted: true})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet,
		"/api/v2/test-runs/trends?project=mine&from=2026-01-05T00:00:00Z&to=2026-02-10T23:59:59Z", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !svc.called {
		t.Fatal("trends service should run")
	}
	if got := svc.from.Format(time.RFC3339); got != "2026-01-05T00:00:00Z" {
		t.Errorf("from = %s, want 2026-01-05T00:00:00Z", got)
	}
	if got := svc.to.Format(time.RFC3339); got != "2026-02-10T23:59:59Z" {
		t.Errorf("to = %s, want 2026-02-10T23:59:59Z", got)
	}
}

// One-sided or malformed ranges are rejected.
func TestTrends_RangeValidation(t *testing.T) {
	cases := []string{
		"project=mine&from=2026-01-05T00:00:00Z",                         // missing to
		"project=mine&to=2026-01-05T00:00:00Z",                           // missing from
		"project=mine&from=nope&to=2026-01-05T00:00:00Z",                 // bad from
		"project=mine&from=2026-02-10T00:00:00Z&to=2026-01-05T00:00:00Z", // to <= from
	}
	for _, qs := range cases {
		svc := &trackingTrendsSvc{}
		r := scopedTrendsRouter(t, svc, fakeScope{unrestricted: true})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v2/test-runs/trends?"+qs, nil))
		if w.Code != http.StatusBadRequest {
			t.Errorf("query %q: status=%d, want 400", qs, w.Code)
		}
		if svc.called {
			t.Errorf("query %q: service must not run on invalid range", qs)
		}
	}
}

// The handler must record the caller's full readable set separately from
// their own project selection. Facet computation clears the selection;
// if the authorization scope rode along in the same field, the project
// facet would enumerate every project in the system.
func TestList_NonAdminGetsAuthorizationScopeSeparately(t *testing.T) {
	svc := &trackingSvc{}
	r := scopedRunsRouter(t, svc, fakeScope{allowed: allow("p1", "p2")})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v2/test-runs?project=p1", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	// Selection stays the caller's own choice.
	if got := svc.filter.ProjectIDs; len(got) != 1 || got[0] != "p1" {
		t.Errorf("ProjectIDs = %v, want just the selected p1", got)
	}
	// Boundary is the full readable set, so the facet can offer p2.
	got := append([]string(nil), svc.filter.AllowedProjectIDs...)
	sort.Strings(got)
	if len(got) != 2 || got[0] != "p1" || got[1] != "p2" {
		t.Errorf("AllowedProjectIDs = %v, want the full allowed set [p1 p2]", got)
	}
}

// An admin is unrestricted: no boundary is imposed.
func TestList_AdminGetsNoAuthorizationScope(t *testing.T) {
	svc := &trackingSvc{}
	r := scopedRunsRouter(t, svc, fakeScope{unrestricted: true})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v2/test-runs", nil))

	if len(svc.filter.AllowedProjectIDs) != 0 {
		t.Errorf("AllowedProjectIDs = %v, want empty for an admin", svc.filter.AllowedProjectIDs)
	}
}
