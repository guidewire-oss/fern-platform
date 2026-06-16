package integrations_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
)

// jiraVersionsResponse matches the JIRA REST API /rest/api/3/project/{key}/versions response.
type jiraVersionsResponse []struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Released    bool   `json:"released"`
	ReleaseDate string `json:"releaseDate,omitempty"`
}

// jiraSearchResponse matches the JIRA REST API GET /rest/api/3/search/jql response.
type jiraSearchResponse struct {
	NextPageToken string            `json:"nextPageToken,omitempty"`
	Issues        []jiraSearchIssue `json:"issues"`
}

type jiraSearchIssue struct {
	Key    string           `json:"key"`
	Fields jiraIssueFields  `json:"fields"`
}

type jiraIssueFields struct {
	Summary   string           `json:"summary"`
	Status    jiraStatus       `json:"status"`
	IssueType jiraIssueType    `json:"issuetype"`
	Parent    *jiraSearchIssue `json:"parent,omitempty"`
}

type jiraStatus struct {
	Name string `json:"name"`
}

type jiraIssueType struct {
	Name    string `json:"name"`
	Subtask bool   `json:"subtask,omitempty"`
}

func TestDefaultJiraClient_GetVersions(t *testing.T) {
	ctx := context.Background()
	client := integrations.NewDefaultJiraClient()

	t.Run("returns parsed released and unreleased versions", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			versions := jiraVersionsResponse{
				{ID: "10001", Name: "1.0.0", Released: true, ReleaseDate: "2024-01-15"},
				{ID: "10002", Name: "2.0.0", Released: false},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(versions)
		}))
		defer srv.Close()

		result, err := client.GetVersions(ctx, srv.URL, "PROJ", "user@example.com", "token", integrations.AuthTypeAPIToken)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Fatalf("expected 2 versions, got %d", len(result))
		}

		var v1, v2 integrations.JiraVersion
		for _, v := range result {
			switch v.Name {
			case "1.0.0":
				v1 = v
			case "2.0.0":
				v2 = v
			}
		}
		if v1.ID != "10001" || !v1.Released || v1.ReleaseDate != "2024-01-15" {
			t.Errorf("unexpected v1: %+v", v1)
		}
		if v2.ID != "10002" || v2.Released || v2.ReleaseDate != "" {
			t.Errorf("unexpected v2: %+v", v2)
		}
	})

	t.Run("requests the correct path for the given project key", func(t *testing.T) {
		var capturedPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedPath = r.URL.Path
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(jiraVersionsResponse{})
		}))
		defer srv.Close()

		_, err := client.GetVersions(ctx, srv.URL, "MYPROJECT", "u", "t", integrations.AuthTypeAPIToken)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		expected := "/rest/api/3/project/MYPROJECT/versions"
		if capturedPath != expected {
			t.Errorf("expected path %q, got %q", expected, capturedPath)
		}
	})

	t.Run("returns error on non-200 status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()

		_, err := client.GetVersions(ctx, srv.URL, "PROJ", "u", "bad", integrations.AuthTypeAPIToken)
		if err == nil {
			t.Error("expected error for HTTP 401, got nil")
		}
	})

	t.Run("returns empty slice when project has no versions", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(jiraVersionsResponse{})
		}))
		defer srv.Close()

		result, err := client.GetVersions(ctx, srv.URL, "PROJ", "u", "t", integrations.AuthTypeAPIToken)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("expected empty slice, got %d items", len(result))
		}
	})
}

func TestDefaultJiraClient_SearchIssues(t *testing.T) {
	ctx := context.Background()
	client := integrations.NewDefaultJiraClient()

	t.Run("returns all issues from a single page", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := jiraSearchResponse{
				Issues: []jiraSearchIssue{
					{Key: "PROJ-1", Fields: jiraIssueFields{Summary: "Story one", Status: jiraStatus{Name: "To Do"}, IssueType: jiraIssueType{Name: "Story"}}},
					{Key: "PROJ-2", Fields: jiraIssueFields{Summary: "Story two", Status: jiraStatus{Name: "Done"}, IssueType: jiraIssueType{Name: "Story"}}},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		result, err := client.SearchIssues(ctx, srv.URL, "u", "t", integrations.AuthTypeAPIToken, `fixVersion = "1.0"`, []string{"summary", "status", "issuetype", "parent"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Fatalf("expected 2 issues, got %d", len(result))
		}
		if result[0].Key != "PROJ-1" || result[1].Key != "PROJ-2" {
			t.Errorf("unexpected keys: %v, %v", result[0].Key, result[1].Key)
		}
	})

	t.Run("paginates using nextPageToken until exhausted", func(t *testing.T) {
		var capturedTokens []string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.URL.Query().Get("nextPageToken")
			capturedTokens = append(capturedTokens, token)

			var resp jiraSearchResponse
			if token == "" {
				resp = jiraSearchResponse{
					NextPageToken: "page2",
					Issues: []jiraSearchIssue{
						{Key: "PROJ-1", Fields: jiraIssueFields{Summary: "S1", Status: jiraStatus{Name: "To Do"}, IssueType: jiraIssueType{Name: "Story"}}},
						{Key: "PROJ-2", Fields: jiraIssueFields{Summary: "S2", Status: jiraStatus{Name: "To Do"}, IssueType: jiraIssueType{Name: "Story"}}},
					},
				}
			} else {
				resp = jiraSearchResponse{
					Issues: []jiraSearchIssue{
						{Key: "PROJ-3", Fields: jiraIssueFields{Summary: "S3", Status: jiraStatus{Name: "Done"}, IssueType: jiraIssueType{Name: "Story"}}},
					},
				}
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		result, err := client.SearchIssues(ctx, srv.URL, "u", "t", integrations.AuthTypeAPIToken, `fixVersion = "1.0"`, []string{"summary"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 3 {
			t.Fatalf("expected 3 issues (2 pages), got %d", len(result))
		}
		if len(capturedTokens) != 2 {
			t.Errorf("expected 2 HTTP requests, got %d", len(capturedTokens))
		}
		if capturedTokens[0] != "" || capturedTokens[1] != "page2" {
			t.Errorf("unexpected nextPageToken values: %v", capturedTokens)
		}
	})

	t.Run("sends correct JQL and fields as query params", func(t *testing.T) {
		var capturedJQL, capturedFields, capturedMaxResults string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedJQL = r.URL.Query().Get("jql")
			capturedFields = r.URL.Query().Get("fields")
			capturedMaxResults = r.URL.Query().Get("maxResults")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(jiraSearchResponse{Issues: []jiraSearchIssue{}})
		}))
		defer srv.Close()

		wantJQL := `fixVersion = "v1.2.3" ORDER BY issuetype`
		wantFields := []string{"summary", "status", "issuetype", "parent"}
		_, err := client.SearchIssues(ctx, srv.URL, "u", "t", integrations.AuthTypeAPIToken, wantJQL, wantFields)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if capturedJQL != wantJQL {
			t.Errorf("expected JQL %q, got %q", wantJQL, capturedJQL)
		}
		if capturedFields != strings.Join(wantFields, ",") {
			t.Errorf("expected fields %q, got %q", strings.Join(wantFields, ","), capturedFields)
		}
		maxResults, _ := strconv.Atoi(capturedMaxResults)
		if maxResults <= 0 {
			t.Errorf("expected maxResults > 0, got %q", capturedMaxResults)
		}
	})

	t.Run("parses parent field on issues that have one", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := jiraSearchResponse{
				Issues: []jiraSearchIssue{
					{
						Key: "PROJ-10",
						Fields: jiraIssueFields{
							Summary:   "Child story",
							Status:    jiraStatus{Name: "In Progress"},
							IssueType: jiraIssueType{Name: "Story"},
							Parent: &jiraSearchIssue{
								Key:    "PROJ-1",
								Fields: jiraIssueFields{IssueType: jiraIssueType{Name: "Epic"}},
							},
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		result, err := client.SearchIssues(ctx, srv.URL, "u", "t", integrations.AuthTypeAPIToken, `fixVersion = "1.0"`, []string{"summary", "parent"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 1 {
			t.Fatalf("expected 1 issue, got %d", len(result))
		}
		issue := result[0]
		if issue.Parent == nil {
			t.Fatal("expected parent to be set, got nil")
		}
		if issue.Parent.Key != "PROJ-1" {
			t.Errorf("expected parent key PROJ-1, got %q", issue.Parent.Key)
		}
		if issue.Parent.IssueType != "Epic" {
			t.Errorf("expected parent issue type Epic, got %q", issue.Parent.IssueType)
		}
	})

	t.Run("returns error on non-200 status", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer srv.Close()

		_, err := client.SearchIssues(ctx, srv.URL, "u", "t", integrations.AuthTypeAPIToken, `fixVersion = "1.0"`, []string{"summary"})
		if err == nil {
			t.Error("expected error for HTTP 503, got nil")
		}
	})

	t.Run("parses issuetype.subtask boolean flag", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := jiraSearchResponse{
				Issues: []jiraSearchIssue{
					{
						Key: "PROJ-100",
						Fields: jiraIssueFields{
							Summary:   "A Dev Task (sub-task with custom name)",
							Status:    jiraStatus{Name: "In Progress"},
							IssueType: jiraIssueType{Name: "Dev Task", Subtask: true},
						},
					},
					{
						Key: "PROJ-101",
						Fields: jiraIssueFields{
							Summary:   "A regular story",
							Status:    jiraStatus{Name: "To Do"},
							IssueType: jiraIssueType{Name: "Story", Subtask: false},
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer srv.Close()

		result, err := client.SearchIssues(ctx, srv.URL, "u", "t", integrations.AuthTypeAPIToken, `fixVersion = "v1.0"`, []string{"summary", "issuetype"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Fatalf("expected 2 issues, got %d", len(result))
		}
		if !result[0].Subtask {
			t.Errorf("expected PROJ-100 (Dev Task) to have Subtask=true, got false")
		}
		if result[0].IssueType != "Dev Task" {
			t.Errorf("expected IssueType 'Dev Task', got %q", result[0].IssueType)
		}
		if result[1].Subtask {
			t.Errorf("expected PROJ-101 (Story) to have Subtask=false, got true")
		}
	})
}
