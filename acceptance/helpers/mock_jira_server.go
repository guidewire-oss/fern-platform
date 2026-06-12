package helpers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
)

// MockJiraServer provides a mock JIRA API for testing
type MockJiraServer struct {
	*httptest.Server
	validTokens       map[string]bool
	projects          map[string]JiraProject
	versions          map[string][]MockVersion // projectKey → versions
	issuesByVersion   map[string][]MockIssue   // fixVersionName → issues
	issuesByKey       map[string]MockIssue     // issueKey → issue (for issueKey IN queries)
	unavailable       bool                     // when true, all /rest/api/3/ endpoints return 503
}

// MockVersion is a configurable JIRA fix version for test fixtures.
type MockVersion struct {
	ID          string
	Name        string
	Released    bool
	ReleaseDate string // ISO date, empty if unreleased
}

// MockIssueParent is the parent reference on a MockIssue.
type MockIssueParent struct {
	Key       string
	IssueType string
}

// MockIssue is a configurable JIRA issue for test fixtures.
type MockIssue struct {
	Key       string
	Summary   string
	Status    string
	IssueType string
	Parent    *MockIssueParent
}

// JiraProject represents a JIRA project
type JiraProject struct {
	ID             string `json:"id"`
	Key            string `json:"key"`
	Name           string `json:"name"`
	ProjectTypeKey string `json:"projectTypeKey"`
}

// JiraUser represents a JIRA user
type JiraUser struct {
	AccountID    string `json:"accountId"`
	EmailAddress string `json:"emailAddress"`
	DisplayName  string `json:"displayName"`
}

// JiraField represents a JIRA field
type JiraField struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Custom        bool     `json:"custom"`
	Navigable     bool     `json:"navigable"`
	Searchable    bool     `json:"searchable"`
	ClauseNames   []string `json:"clauseNames"`
	Schema        Schema   `json:"schema"`
}

// Schema represents field schema
type Schema struct {
	Type   string `json:"type"`
	Items  string `json:"items,omitempty"`
	System string `json:"system,omitempty"`
}

// NewMockJiraServer creates a new mock JIRA server
func NewMockJiraServer() *MockJiraServer {
	mock := &MockJiraServer{
		validTokens: map[string]bool{
			"test-api-token-123": true,
			"valid-token":        true,
		},
		projects: map[string]JiraProject{
			"FERN": {
				ID:             "10000",
				Key:            "FERN",
				Name:           "Fern Platform",
				ProjectTypeKey: "software",
			},
			"TEST": {
				ID:             "10001",
				Key:            "TEST",
				Name:           "Test Project",
				ProjectTypeKey: "software",
			},
		},
		versions:        make(map[string][]MockVersion),
		issuesByVersion: make(map[string][]MockIssue),
		issuesByKey:     make(map[string]MockIssue),
	}

	mux := http.NewServeMux()
	mock.setupRoutes(mux)
	mock.Server = httptest.NewServer(mux)
	return mock
}

func (m *MockJiraServer) setupRoutes(mux *http.ServeMux) {
	// Authentication endpoint
	mux.HandleFunc("/rest/api/2/myself", m.handleMyself)

	// Project endpoints (v2)
	mux.HandleFunc("/rest/api/2/project/", m.handleProject)

	// Field configuration endpoint
	mux.HandleFunc("/rest/api/2/field", m.handleFields)

	// Issue type endpoint
	mux.HandleFunc("/rest/api/2/issuetype", m.handleIssueTypes)

	// Server info endpoint (for JIRA version detection)
	mux.HandleFunc("/rest/api/2/serverInfo", m.handleServerInfo)

	// Coverage endpoints
	mux.HandleFunc("/rest/api/3/project/", m.handleProjectV3)
	mux.HandleFunc("/rest/api/3/search/jql", m.handleIssueSearch)
}

func (m *MockJiraServer) handleMyself(w http.ResponseWriter, r *http.Request) {
	if !m.authenticate(r) {
		http.Error(w, `{"errorMessages":["Unauthorized"],"errors":{}}`, http.StatusUnauthorized)
		return
	}

	user := JiraUser{
		AccountID:    "123",
		EmailAddress: "test@fern.com",
		DisplayName:  "Test User",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func (m *MockJiraServer) handleProject(w http.ResponseWriter, r *http.Request) {
	if !m.authenticate(r) {
		http.Error(w, `{"errorMessages":["Unauthorized"],"errors":{}}`, http.StatusUnauthorized)
		return
	}

	// Extract project key from path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		http.Error(w, `{"errorMessages":["Invalid request"],"errors":{}}`, http.StatusBadRequest)
		return
	}
	projectKey := parts[4]

	project, exists := m.projects[projectKey]
	if !exists {
		http.Error(w, fmt.Sprintf(`{"errorMessages":["No project could be found with key '%s'."],"errors":{}}`, projectKey), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(project)
}

func (m *MockJiraServer) handleFields(w http.ResponseWriter, r *http.Request) {
	if !m.authenticate(r) {
		http.Error(w, `{"errorMessages":["Unauthorized"],"errors":{}}`, http.StatusUnauthorized)
		return
	}

	fields := []JiraField{
		{
			ID:          "summary",
			Name:        "Summary",
			Custom:      false,
			Navigable:   true,
			Searchable:  true,
			ClauseNames: []string{"summary"},
			Schema:      Schema{Type: "string"},
		},
		{
			ID:          "issuetype",
			Name:        "Issue Type",
			Custom:      false,
			Navigable:   true,
			Searchable:  true,
			ClauseNames: []string{"issuetype", "type"},
			Schema:      Schema{Type: "issuetype"},
		},
		{
			ID:          "customfield_10000",
			Name:        "Epic Link",
			Custom:      true,
			Navigable:   true,
			Searchable:  true,
			ClauseNames: []string{"cf[10000]", "Epic Link"},
			Schema:      Schema{Type: "string"},
		},
		{
			ID:          "customfield_10001",
			Name:        "Story Points",
			Custom:      true,
			Navigable:   true,
			Searchable:  true,
			ClauseNames: []string{"cf[10001]", "Story Points"},
			Schema:      Schema{Type: "number"},
		},
		{
			ID:          "fixVersions",
			Name:        "Fix Version/s",
			Custom:      false,
			Navigable:   true,
			Searchable:  true,
			ClauseNames: []string{"fixVersion"},
			Schema:      Schema{Type: "array", Items: "version"},
		},
		{
			ID:          "components",
			Name:        "Component/s",
			Custom:      false,
			Navigable:   true,
			Searchable:  true,
			ClauseNames: []string{"component"},
			Schema:      Schema{Type: "array", Items: "component"},
		},
		{
			ID:          "labels",
			Name:        "Labels",
			Custom:      false,
			Navigable:   true,
			Searchable:  true,
			ClauseNames: []string{"labels"},
			Schema:      Schema{Type: "array", Items: "string"},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fields)
}

func (m *MockJiraServer) handleIssueTypes(w http.ResponseWriter, r *http.Request) {
	if !m.authenticate(r) {
		http.Error(w, `{"errorMessages":["Unauthorized"],"errors":{}}`, http.StatusUnauthorized)
		return
	}

	issueTypes := []map[string]interface{}{
		{
			"id":          "10000",
			"name":        "Epic",
			"description": "A big user story that needs to be broken down.",
			"iconUrl":     m.URL + "/images/icons/issuetypes/epic.svg",
			"subtask":     false,
		},
		{
			"id":          "10001",
			"name":        "Story",
			"description": "A user story.",
			"iconUrl":     m.URL + "/images/icons/issuetypes/story.svg",
			"subtask":     false,
		},
		{
			"id":          "10002",
			"name":        "Task",
			"description": "A task that needs to be done.",
			"iconUrl":     m.URL + "/images/icons/issuetypes/task.svg",
			"subtask":     false,
		},
		{
			"id":          "10003",
			"name":        "Bug",
			"description": "A problem which impairs or prevents the functions of the product.",
			"iconUrl":     m.URL + "/images/icons/issuetypes/bug.svg",
			"subtask":     false,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(issueTypes)
}

func (m *MockJiraServer) handleServerInfo(w http.ResponseWriter, r *http.Request) {
	// No auth required for server info
	serverInfo := map[string]interface{}{
		"baseUrl":        m.URL,
		"version":        "8.20.10",
		"versionNumbers": []int{8, 20, 10},
		"deploymentType": "Server",
		"buildNumber":    820010,
		"serverTitle":    "Mock JIRA Server",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(serverInfo)
}

func (m *MockJiraServer) authenticate(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return false
	}

	// Support both Basic auth and Bearer token
	if strings.HasPrefix(auth, "Basic ") {
		// For simplicity, we accept any Basic auth where the token part is valid
		return true
	} else if strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimPrefix(auth, "Bearer ")
		return m.validTokens[token]
	}

	return false
}

// AddValidToken adds a valid token for testing
func (m *MockJiraServer) AddValidToken(token string) {
	m.validTokens[token] = true
}

// AddProject adds a project for testing
func (m *MockJiraServer) AddProject(key string, project JiraProject) {
	m.projects[key] = project
}

// SetVersions configures the fix versions returned for a project key.
func (m *MockJiraServer) SetVersions(projectKey string, versions []MockVersion) {
	m.versions[projectKey] = versions
}

// SetIssuesForVersion configures the issues returned when searching by fix version name.
func (m *MockJiraServer) SetIssuesForVersion(fixVersionName string, issues []MockIssue) {
	m.issuesByVersion[fixVersionName] = issues
	for _, issue := range issues {
		m.issuesByKey[issue.Key] = issue
	}
}

// AddIssueByKey registers an issue that can be retrieved via issueKey IN (...) JQL.
// Use this for parent epics that are not themselves in any fix version.
func (m *MockJiraServer) AddIssueByKey(issue MockIssue) {
	m.issuesByKey[issue.Key] = issue
}

// SimulateUnavailable makes coverage endpoints (versions and issue search) return 503.
func (m *MockJiraServer) SimulateUnavailable(on bool) {
	m.unavailable = on
}

// --- v3 handlers ---

func (m *MockJiraServer) handleProjectV3(w http.ResponseWriter, r *http.Request) {
	if m.unavailable {
		http.Error(w, `{"errorMessages":["Service unavailable"]}`, http.StatusServiceUnavailable)
		return
	}
	if !m.authenticate(r) {
		http.Error(w, `{"errorMessages":["Unauthorized"],"errors":{}}`, http.StatusUnauthorized)
		return
	}

	// Expect path: /rest/api/3/project/{key}/versions
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 5 || parts[4] != "versions" {
		http.Error(w, `{"errorMessages":["Not found"]}`, http.StatusNotFound)
		return
	}
	projectKey := parts[3]

	type versionResponse struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Released    bool   `json:"released"`
		ReleaseDate string `json:"releaseDate,omitempty"`
	}

	versions := m.versions[projectKey]
	resp := make([]versionResponse, len(versions))
	for i, v := range versions {
		resp[i] = versionResponse{ID: v.ID, Name: v.Name, Released: v.Released, ReleaseDate: v.ReleaseDate}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (m *MockJiraServer) handleIssueSearch(w http.ResponseWriter, r *http.Request) {
	if m.unavailable {
		http.Error(w, `{"errorMessages":["Service unavailable"]}`, http.StatusServiceUnavailable)
		return
	}
	if !m.authenticate(r) {
		http.Error(w, `{"errorMessages":["Unauthorized"],"errors":{}}`, http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, `{"errorMessages":["Method not allowed"]}`, http.StatusMethodNotAllowed)
		return
	}

	jql := r.URL.Query().Get("jql")
	issues := m.resolveJQL(jql)

	type issueStatus struct {
		Name string `json:"name"`
	}
	type issueType struct {
		Name string `json:"name"`
	}
	type parentFields struct {
		IssueType issueType `json:"issuetype"`
	}
	type issueParent struct {
		Key    string       `json:"key"`
		Fields parentFields `json:"fields"`
	}
	type issueFields struct {
		Summary   string       `json:"summary"`
		Status    issueStatus  `json:"status"`
		IssueType issueType    `json:"issuetype"`
		Parent    *issueParent `json:"parent,omitempty"`
	}
	type issueResponse struct {
		Key    string      `json:"key"`
		Fields issueFields `json:"fields"`
	}
	type searchResponse struct {
		NextPageToken string          `json:"nextPageToken,omitempty"`
		Issues        []issueResponse `json:"issues"`
	}

	issueResponses := make([]issueResponse, len(issues))
	for i, issue := range issues {
		fields := issueFields{
			Summary:   issue.Summary,
			Status:    issueStatus{Name: issue.Status},
			IssueType: issueType{Name: issue.IssueType},
		}
		if issue.Parent != nil {
			fields.Parent = &issueParent{
				Key:    issue.Parent.Key,
				Fields: parentFields{IssueType: issueType{Name: issue.Parent.IssueType}},
			}
		}
		issueResponses[i] = issueResponse{Key: issue.Key, Fields: fields}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(searchResponse{Issues: issueResponses})
}

// resolveJQL returns issues matching a simple JQL expression.
// Supports: fixVersion = "name"  and  issueKey IN (KEY-1,KEY-2,...)
func (m *MockJiraServer) resolveJQL(jql string) []MockIssue {
	jql = strings.TrimSpace(jql)

	if strings.Contains(jql, "fixVersion") {
		// Extract version name from: fixVersion = "Atmos vNext" ORDER BY ...
		start := strings.Index(jql, `"`)
		end := strings.LastIndex(jql, `"`)
		if start >= 0 && end > start {
			versionName := jql[start+1 : end]
			return m.issuesByVersion[versionName]
		}
	}

	if strings.Contains(jql, "issueKey IN") || strings.Contains(jql, "issuekey in") {
		// Extract keys from: issueKey IN (KEY-1, KEY-2, ...)
		open := strings.Index(jql, "(")
		close := strings.Index(jql, ")")
		if open >= 0 && close > open {
			keyPart := jql[open+1 : close]
			var result []MockIssue
			for _, raw := range strings.Split(keyPart, ",") {
				key := strings.TrimSpace(raw)
				if issue, ok := m.issuesByKey[key]; ok {
					result = append(result, issue)
				}
			}
			return result
		}
	}

	return nil
}