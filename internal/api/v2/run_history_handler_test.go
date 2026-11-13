package v2_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	apiv2 "github.com/guidewire-oss/fern-platform/internal/api/v2"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
)

type fakeHistorySvc struct {
	gotID    uint
	gotLimit int
	gotSuite string
	gotSpec  string
	hist     *domain.RunHistory
	spec     *domain.SpecRunHistory
	err      error
}

func (f *fakeHistorySvc) SpecHistory(_ context.Context, id uint, limit int, suite, spec string) (*domain.SpecRunHistory, error) {
	f.gotID, f.gotLimit, f.gotSuite, f.gotSpec = id, limit, suite, spec
	return f.spec, f.err
}

func (f *fakeHistorySvc) RunHistory(_ context.Context, id uint, limit int) (*domain.RunHistory, error) {
	f.gotID, f.gotLimit = id, limit
	return f.hist, f.err
}

func newHistoryRouter(t *testing.T, svc apiv2.RunHistoryService, scope apiv2.ProjectScoper) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v2 := r.Group("/api/v2")
	// Register the trends route too: /test-runs/trends and
	// /test-runs/:id/history must coexist on the same group.
	apiv2.NewTestRunTrendsHandler(nil, nil).Register(v2)
	apiv2.NewRunHistoryHandler(svc).WithScope(scope).Register(v2)
	return r
}

func sampleHistory() *domain.RunHistory {
	return &domain.RunHistory{
		ProjectID: "p1",
		Runs:      []domain.HistoryRun{{ID: 1, RunID: "a"}, {ID: 2, RunID: "b"}},
		Suites:    []domain.SuiteHistory{{SuiteName: "S", Results: []*domain.HistoryCell{nil, {Status: "passed"}}}},
		Specs: []domain.SpecHistory{{SuiteName: "S", SpecName: "x",
			Results: []*domain.HistoryCell{{Status: "failed", DurationMs: 3}, {Status: "passed", DurationMs: 4}}}},
	}
}

func getHistory(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

func TestRunHistory_HappyPath(t *testing.T) {
	svc := &fakeHistorySvc{hist: sampleHistory()}
	w := getHistory(t, newHistoryRouter(t, svc, nil), "/api/v2/test-runs/2/history")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if svc.gotID != 2 || svc.gotLimit != domain.DefaultRunHistoryLimit {
		t.Errorf("got id=%d limit=%d", svc.gotID, svc.gotLimit)
	}
	var body struct {
		Runs []struct {
			RunID string `json:"runId"`
		} `json:"runs"`
		Suites []struct {
			Results []*struct {
				Status string `json:"status"`
			} `json:"results"`
		} `json:"suites"`
		Specs []struct {
			SpecName string `json:"specName"`
			Results  []*struct {
				Status     string `json:"status"`
				DurationMs int64  `json:"durationMs"`
			} `json:"results"`
		} `json:"specs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Runs) != 2 || body.Runs[1].RunID != "b" {
		t.Errorf("runs = %+v", body.Runs)
	}
	if body.Suites[0].Results[0] != nil {
		t.Errorf("absent suite cell should be null, got %+v", body.Suites[0].Results[0])
	}
	if s := body.Specs[0]; s.SpecName != "x" || s.Results[0].Status != "failed" || s.Results[1].DurationMs != 4 {
		t.Errorf("spec = %+v", s)
	}
}

func TestRunHistory_Limit(t *testing.T) {
	svc := &fakeHistorySvc{hist: sampleHistory()}
	r := newHistoryRouter(t, svc, nil)
	if w := getHistory(t, r, "/api/v2/test-runs/2/history?limit=5"); w.Code != http.StatusOK || svc.gotLimit != 5 {
		t.Errorf("limit=5: status=%d gotLimit=%d", w.Code, svc.gotLimit)
	}
	for _, bad := range []string{"0", "-1", "abc", "101"} {
		if w := getHistory(t, r, "/api/v2/test-runs/2/history?limit="+bad); w.Code != http.StatusBadRequest {
			t.Errorf("limit=%s: status=%d, want 400", bad, w.Code)
		}
	}
}

func TestRunHistory_BadID(t *testing.T) {
	r := newHistoryRouter(t, &fakeHistorySvc{hist: sampleHistory()}, nil)
	for _, bad := range []string{"0", "abc", "-3"} {
		if w := getHistory(t, r, "/api/v2/test-runs/"+bad+"/history"); w.Code != http.StatusBadRequest {
			t.Errorf("id=%s: status=%d, want 400", bad, w.Code)
		}
	}
}

func TestRunHistory_NotFound(t *testing.T) {
	r := newHistoryRouter(t, &fakeHistorySvc{err: domain.ErrNotFound}, nil)
	if w := getHistory(t, r, "/api/v2/test-runs/9/history"); w.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404", w.Code)
	}
}

func TestRunHistory_ServiceError(t *testing.T) {
	r := newHistoryRouter(t, &fakeHistorySvc{err: errors.New("db down")}, nil)
	if w := getHistory(t, r, "/api/v2/test-runs/9/history"); w.Code != http.StatusInternalServerError {
		t.Errorf("status=%d, want 500", w.Code)
	}
}

func TestRunHistory_Scope(t *testing.T) {
	cases := []struct {
		name  string
		scope fakeScope
		want  int
	}{
		{"allowed project", fakeScope{allowed: allow("p1")}, http.StatusOK},
		{"other team's project looks missing", fakeScope{allowed: allow("p2")}, http.StatusNotFound},
		{"no projects", fakeScope{allowed: allow()}, http.StatusNotFound},
		{"admin", fakeScope{unrestricted: true}, http.StatusOK},
		{"scope failure fails closed", fakeScope{err: errors.New("boom")}, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newHistoryRouter(t, &fakeHistorySvc{hist: sampleHistory()}, tc.scope)
			if w := getHistory(t, r, "/api/v2/test-runs/2/history"); w.Code != tc.want {
				t.Errorf("status=%d, want %d", w.Code, tc.want)
			}
		})
	}
}

func sampleSpecHistory() *domain.SpecRunHistory {
	return &domain.SpecRunHistory{
		ProjectID: "p1",
		Runs:      []domain.HistoryRun{{ID: 1, RunID: "a"}, {ID: 2, RunID: "b"}},
		SuiteName: "S",
		SpecName:  "x y",
		Results: []*domain.SpecHistoryCell{
			{Status: "failed", DurationMs: 3, ErrorMessage: "boom", StackTrace: "at 1"},
			{Status: "passed", DurationMs: 4},
		},
	}
}

func TestSpecHistory_HappyPath(t *testing.T) {
	svc := &fakeHistorySvc{spec: sampleSpecHistory()}
	w := getHistory(t, newHistoryRouter(t, svc, nil), "/api/v2/test-runs/2/history/spec?suite=S&spec=x%20y&limit=7")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if svc.gotID != 2 || svc.gotLimit != 7 || svc.gotSuite != "S" || svc.gotSpec != "x y" {
		t.Errorf("got id=%d limit=%d suite=%q spec=%q", svc.gotID, svc.gotLimit, svc.gotSuite, svc.gotSpec)
	}
	var body struct {
		Runs     []struct{ ID uint } `json:"runs"`
		SpecName string              `json:"specName"`
		Results  []*struct {
			Status       string `json:"status"`
			ErrorMessage string `json:"errorMessage"`
			StackTrace   string `json:"stackTrace"`
		} `json:"results"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.SpecName != "x y" || len(body.Runs) != 2 || len(body.Results) != 2 {
		t.Fatalf("body = %+v", body)
	}
	if r := body.Results[0]; r.ErrorMessage != "boom" || r.StackTrace != "at 1" {
		t.Errorf("failure text = %+v", r)
	}
	if !strings.Contains(w.Body.String(), `"durationMs":4}`) {
		t.Errorf("a pass should omit empty error fields: %s", w.Body.String())
	}
}

func TestSpecHistory_RequiresSuiteAndSpec(t *testing.T) {
	r := newHistoryRouter(t, &fakeHistorySvc{spec: sampleSpecHistory()}, nil)
	for _, q := range []string{"", "?suite=S", "?spec=x"} {
		if w := getHistory(t, r, "/api/v2/test-runs/2/history/spec"+q); w.Code != http.StatusBadRequest {
			t.Errorf("query %q: status=%d, want 400", q, w.Code)
		}
	}
}

func TestSpecHistory_NotFoundAndScope(t *testing.T) {
	r := newHistoryRouter(t, &fakeHistorySvc{err: domain.ErrNotFound}, nil)
	if w := getHistory(t, r, "/api/v2/test-runs/9/history/spec?suite=S&spec=x"); w.Code != http.StatusNotFound {
		t.Errorf("missing run: status=%d, want 404", w.Code)
	}
	r = newHistoryRouter(t, &fakeHistorySvc{spec: sampleSpecHistory()}, fakeScope{allowed: allow("p2")})
	if w := getHistory(t, r, "/api/v2/test-runs/2/history/spec?suite=S&spec=x"); w.Code != http.StatusNotFound {
		t.Errorf("other team's project: status=%d, want 404", w.Code)
	}
}
