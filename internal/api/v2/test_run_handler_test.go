package v2_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	apiv2 "github.com/guidewire-oss/fern-platform/internal/api/v2"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
)

// fakeQuerySvc captures the filter/page the handler computed and
// returns a canned response. It is intentionally trivial — we are
// testing handler decoding and encoding, not query semantics.
type fakeQuerySvc struct {
	gotFilter domain.TestRunFilter
	gotPage   domain.PageArgs
	page      domain.TestRunPage
	err       error
}

func (f *fakeQuerySvc) Query(_ context.Context, filter domain.TestRunFilter, page domain.PageArgs) (domain.TestRunPage, error) {
	f.gotFilter = filter
	f.gotPage = page
	return f.page, f.err
}

func newRouter(t *testing.T, svc apiv2.TestRunQueryService) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := apiv2.NewTestRunHandler(svc)
	v2 := r.Group("/api/v2")
	h.Register(v2)
	return r
}

func TestList_HappyPath(t *testing.T) {
	svc := &fakeQuerySvc{
		page: domain.TestRunPage{
			Edges:      []domain.TestRunEdge{{Cursor: "c1", Node: &domain.TestRun{RunID: "r1"}}},
			PageInfo:   domain.PageInfo{HasNextPage: false, EndCursor: ""},
			TotalCount: 1,
		},
	}
	r := newRouter(t, svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/test-runs?project=p1&status=failed&first=25", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := body["edges"]; !ok {
		t.Errorf("response missing edges: %s", w.Body.String())
	}
	if _, ok := body["pageInfo"]; !ok {
		t.Errorf("response missing pageInfo")
	}
	if _, ok := body["totalCount"]; !ok {
		t.Errorf("response missing totalCount")
	}

	if !equalStrings(svc.gotFilter.ProjectIDs, []string{"p1"}) {
		t.Errorf("filter.ProjectIDs = %v", svc.gotFilter.ProjectIDs)
	}
	if !equalStrings(svc.gotFilter.Status, []string{"failed"}) {
		t.Errorf("filter.Status = %v", svc.gotFilter.Status)
	}
	if svc.gotPage.First != 25 {
		t.Errorf("page.First = %d", svc.gotPage.First)
	}
}

func TestList_RepeatedParamsAreOR(t *testing.T) {
	svc := &fakeQuerySvc{}
	r := newRouter(t, svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/test-runs?project=p1&project=p2&status=failed&status=flaky", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}
	if !equalStrings(svc.gotFilter.ProjectIDs, []string{"p1", "p2"}) {
		t.Errorf("ProjectIDs = %v", svc.gotFilter.ProjectIDs)
	}
	if !equalStrings(svc.gotFilter.Status, []string{"failed", "flaky"}) {
		t.Errorf("Status = %v", svc.gotFilter.Status)
	}
}

func TestList_ClampsFirstToMax(t *testing.T) {
	svc := &fakeQuerySvc{}
	r := newRouter(t, svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/test-runs?first=99999", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if svc.gotPage.First != domain.MaxPageSize {
		t.Errorf("first should clamp to %d, got %d", domain.MaxPageSize, svc.gotPage.First)
	}
}

func TestList_RejectsInvertedDateRange(t *testing.T) {
	svc := &fakeQuerySvc{}
	r := newRouter(t, svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/api/v2/test-runs?from=2026-05-14T00:00:00Z&to=2026-05-01T00:00:00Z", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for inverted range, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestList_RejectsBadFirst(t *testing.T) {
	svc := &fakeQuerySvc{}
	r := newRouter(t, svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v2/test-runs?first=not-a-number", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad first, got %d", w.Code)
	}
}

func TestList_DateRangeParsed(t *testing.T) {
	svc := &fakeQuerySvc{}
	r := newRouter(t, svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/api/v2/test-runs?from=2026-05-01T00:00:00Z&to=2026-05-14T23:59:59Z", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if svc.gotFilter.StartedAt == nil || svc.gotFilter.StartedAt.Gte == nil || svc.gotFilter.StartedAt.Lte == nil {
		t.Fatalf("StartedAt not populated: %+v", svc.gotFilter.StartedAt)
	}
}

func TestList_TagsAndMode(t *testing.T) {
	svc := &fakeQuerySvc{}
	r := newRouter(t, svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/api/v2/test-runs?tag=smoke&tag=release&tagMode=AND", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !equalStrings(svc.gotFilter.Tags, []string{"smoke", "release"}) {
		t.Errorf("Tags = %v", svc.gotFilter.Tags)
	}
	if svc.gotFilter.TagMode != domain.LogicAnd {
		t.Errorf("TagMode = %q", svc.gotFilter.TagMode)
	}
}

func TestList_ResponseShapeIsConnection(t *testing.T) {
	svc := &fakeQuerySvc{
		page: domain.TestRunPage{
			Edges: []domain.TestRunEdge{
				{Cursor: "cur-1", Node: &domain.TestRun{RunID: "r1"}},
			},
			PageInfo:             domain.PageInfo{HasNextPage: true, EndCursor: "cur-1"},
			TotalCount:           42,
			TotalCountIsEstimate: true,
		},
	}
	r := newRouter(t, svc)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v2/test-runs", nil))

	body := w.Body.String()
	for _, key := range []string{`"edges"`, `"pageInfo"`, `"totalCount"`, `"totalCountIsEstimate"`, `"facets"`} {
		if !strings.Contains(body, key) {
			t.Errorf("response missing %s key: %s", key, body)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Requirement 1.1: the node payload carries the resolved project name
// alongside the ID the client filters and links on.
func TestList_NodeIncludesProjectName(t *testing.T) {
	svc := &fakeQuerySvc{
		page: domain.TestRunPage{
			Edges: []domain.TestRunEdge{{Cursor: "c1", Node: &domain.TestRun{
				RunID:       "r1",
				ProjectID:   "proj-a",
				ProjectName: "Alpha Service",
			}}},
			TotalCount: 1,
		},
	}
	r := newRouter(t, svc)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v2/test-runs", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Edges []struct {
			Node struct {
				ProjectID   string `json:"project_id"`
				ProjectName string `json:"project_name"`
			} `json:"node"`
		} `json:"edges"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := body.Edges[0].Node.ProjectName; got != "Alpha Service" {
		t.Errorf("project_name = %q, want %q", got, "Alpha Service")
	}
	if got := body.Edges[0].Node.ProjectID; got != "proj-a" {
		t.Errorf("project_id = %q, want proj-a", got)
	}
}

// Requirements 2.1/2.3/2.4: the project facet serialises a label; facets
// without one omit the key entirely.
func TestList_ProjectFacetCarriesLabel(t *testing.T) {
	svc := &fakeQuerySvc{
		page: domain.TestRunPage{
			Facets: domain.TestRunFacets{
				ByProject: []domain.FacetCount{
					{Value: "proj-a", Count: 4, Label: "Alpha Service"},
					{Value: "proj-unnamed", Count: 1},
				},
				ByStatus: []domain.FacetCount{{Value: "failed", Count: 4}},
			},
		},
	}
	r := newRouter(t, svc)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v2/test-runs", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Facets struct {
			ByProject []map[string]any `json:"byProject"`
			ByStatus  []map[string]any `json:"byStatus"`
		} `json:"facets"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := body.Facets.ByProject[0]["label"]; got != "Alpha Service" {
		t.Errorf("byProject[0].label = %v, want Alpha Service", got)
	}
	if got := body.Facets.ByProject[0]["value"]; got != "proj-a" {
		t.Errorf("byProject[0].value = %v, want proj-a", got)
	}
	if _, ok := body.Facets.ByProject[1]["label"]; ok {
		t.Error("an unnamed project should omit the label key")
	}
	if _, ok := body.Facets.ByStatus[0]["label"]; ok {
		t.Error("the status facet should never carry a label")
	}
}

// The list UI renders a run's wall-clock time. Deriving it from
// end_time - start_time breaks for runs with no end_time, so the
// stored duration ships explicitly as milliseconds. (domain.TestRun's
// own `duration` field marshals as nanoseconds, which is not what the
// client wants to read.)
func TestList_NodeIncludesDurationMs(t *testing.T) {
	svc := &fakeQuerySvc{
		page: domain.TestRunPage{
			Edges: []domain.TestRunEdge{{Cursor: "c1", Node: &domain.TestRun{
				RunID:    "r1",
				Duration: 90 * time.Second,
			}}},
			TotalCount: 1,
		},
	}
	r := newRouter(t, svc)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v2/test-runs", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Edges []struct {
			Node struct {
				RunID      string `json:"run_id"`
				DurationMs int64  `json:"duration_ms"`
			} `json:"node"`
		} `json:"edges"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := body.Edges[0].Node.DurationMs; got != 90_000 {
		t.Errorf("duration_ms = %d, want 90000", got)
	}
	// The rest of the node must still serialise.
	if got := body.Edges[0].Node.RunID; got != "r1" {
		t.Errorf("run_id = %q, want r1", got)
	}
}

// /projects/:projectId/test-runs is /test-runs pinned to the path's
// project: the path wins over any ?project=, other filters pass through.
func TestListForProject_PinsProjectFromPath(t *testing.T) {
	svc := &fakeQuerySvc{}
	r := newRouter(t, svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/api/v2/projects/p1/test-runs?project=p2&project=p3&tag=jira:GWCP-1&tag=smoke&tagMode=AND&status=failed&allTime=1&first=10", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !equalStrings(svc.gotFilter.ProjectIDs, []string{"p1"}) {
		t.Errorf("ProjectIDs = %v, want [p1]", svc.gotFilter.ProjectIDs)
	}
	if !equalStrings(svc.gotFilter.Tags, []string{"jira:GWCP-1", "smoke"}) || svc.gotFilter.TagMode != domain.LogicAnd {
		t.Errorf("Tags = %v mode %q", svc.gotFilter.Tags, svc.gotFilter.TagMode)
	}
	if !equalStrings(svc.gotFilter.Status, []string{"failed"}) {
		t.Errorf("Status = %v", svc.gotFilter.Status)
	}
	if svc.gotFilter.StartedAt != nil {
		t.Errorf("allTime=1 should leave StartedAt unset, got %+v", svc.gotFilter.StartedAt)
	}
	if svc.gotPage.First != 10 {
		t.Errorf("First = %d, want 10", svc.gotPage.First)
	}
}

func TestListForProject_DefaultsToLast7Days(t *testing.T) {
	svc := &fakeQuerySvc{}
	r := newRouter(t, svc)
	before := time.Now().UTC()
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v2/projects/p1/test-runs", nil))
	after := time.Now().UTC()
	if svc.gotFilter.StartedAt == nil || svc.gotFilter.StartedAt.Gte == nil {
		t.Fatal("expected default 7-day window")
	}
	const week = 7 * 24 * time.Hour
	if gte := *svc.gotFilter.StartedAt.Gte; gte.Before(before.Add(-week)) || gte.After(after.Add(-week)) {
		t.Errorf("StartedAt.Gte = %v, want now-7d (between %v and %v)", gte, before.Add(-week), after.Add(-week))
	}
	if svc.gotFilter.StartedAt.Lte != nil {
		t.Errorf("StartedAt.Lte = %v, want unset", *svc.gotFilter.StartedAt.Lte)
	}
}

func TestListForProject_RespectsScope(t *testing.T) {
	cases := []struct {
		name        string
		scope       fakeScope
		wantCalled  bool
		wantProject []string
	}{
		{"readable project", fakeScope{allowed: allow("p1", "p9")}, true, []string{"p1"}},
		{"unreadable project returns empty", fakeScope{allowed: allow("p9")}, false, nil},
		{"unrestricted", fakeScope{unrestricted: true}, true, []string{"p1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeQuerySvc{}
			gin.SetMode(gin.TestMode)
			r := gin.New()
			apiv2.NewTestRunHandler(svc).WithScope(tc.scope).Register(r.Group("/api/v2"))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v2/projects/p1/test-runs?tag=jira:GWCP-1", nil))

			if w.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			called := svc.gotFilter.Tags != nil
			if called != tc.wantCalled {
				t.Fatalf("query called = %v, want %v", called, tc.wantCalled)
			}
			if called && !equalStrings(svc.gotFilter.ProjectIDs, tc.wantProject) {
				t.Errorf("ProjectIDs = %v, want %v", svc.gotFilter.ProjectIDs, tc.wantProject)
			}
			if !called && !strings.Contains(w.Body.String(), `"edges":[]`) {
				t.Errorf("expected empty connection, got %s", w.Body.String())
			}
		})
	}
}
