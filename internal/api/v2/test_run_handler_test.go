package v2_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
