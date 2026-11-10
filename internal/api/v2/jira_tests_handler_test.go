package v2_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	apiv2 "github.com/guidewire-oss/fern-platform/internal/api/v2"
	tagsdomain "github.com/guidewire-oss/fern-platform/internal/domains/tags/domain"
)

type fakeJiraLookup struct {
	runs   []tagsdomain.CoveredSpecRun
	err    error
	gotKey string // empty until called
}

func (f *fakeJiraLookup) GetSpecRunsByJiraTag(_ context.Context, _, issueKey string) ([]tagsdomain.CoveredSpecRun, error) {
	f.gotKey = issueKey
	return f.runs, f.err
}

type jiraTestsResp struct {
	ProjectID string `json:"projectId"`
	IssueKey  string `json:"issueKey"`
	HasTests  bool   `json:"hasTests"`
}

func TestJiraTestsHandler(t *testing.T) {
	oneRun := []tagsdomain.CoveredSpecRun{{SpecName: "logs in", Status: "passed"}}
	p1 := fakeScope{allowed: allow("p1")}
	cases := []struct {
		name       string
		pid, key   string
		repo       *fakeJiraLookup
		scope      apiv2.ProjectScoper
		wantCode   int
		wantCalled bool
	}{
		{"tagged runs exist", "p1", "GWCP-12345", &fakeJiraLookup{runs: oneRun}, p1, http.StatusOK, true},
		{"no tagged runs", "p1", "GWCP-12345", &fakeJiraLookup{}, p1, http.StatusOK, true},
		{"lowercase key accepted", "p1", "gwcp-12345", &fakeJiraLookup{runs: oneRun}, p1, http.StatusOK, true},
		{"unrestricted caller", "any", "GWCP-1", &fakeJiraLookup{runs: oneRun}, fakeScope{unrestricted: true}, http.StatusOK, true},
		{"nil scope", "any", "GWCP-1", &fakeJiraLookup{}, nil, http.StatusOK, true},
		{"invalid key", "p1", "not-a-key", &fakeJiraLookup{}, p1, http.StatusBadRequest, false},
		{"foreign project", "other", "GWCP-1", &fakeJiraLookup{runs: oneRun}, p1, http.StatusForbidden, false},
		{"scope error", "p1", "GWCP-1", &fakeJiraLookup{}, fakeScope{err: errors.New("boom")}, http.StatusInternalServerError, false},
		{"repo error", "p1", "GWCP-1", &fakeJiraLookup{err: errors.New("db down")}, p1, http.StatusInternalServerError, true},
	}
	gin.SetMode(gin.TestMode)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			apiv2.NewJiraTestsHandler(tc.repo).WithScope(tc.scope).Register(r.Group("/api/v2"))
			w := httptest.NewRecorder()
			path := fmt.Sprintf("/api/v2/projects/%s/jira/%s/tests", tc.pid, tc.key)
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))

			if w.Code != tc.wantCode {
				t.Fatalf("code = %d, want %d; body=%s", w.Code, tc.wantCode, w.Body.String())
			}
			if called := tc.repo.gotKey != ""; called != tc.wantCalled {
				t.Fatalf("repo called = %v, want %v", called, tc.wantCalled)
			}
			if tc.wantCode != http.StatusOK {
				return
			}
			var resp jiraTestsResp
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			want := jiraTestsResp{tc.pid, strings.ToUpper(tc.key), len(tc.repo.runs) > 0}
			if resp != want {
				t.Errorf("response = %+v, want %+v", resp, want)
			}
		})
	}
}
