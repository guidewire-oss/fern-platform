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
	validTokens map[string]bool
	projects    map[string]JiraProject
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
	}

	mux := http.NewServeMux()
	mock.setupRoutes(mux)
	mock.Server = httptest.NewServer(mux)
	return mock
}

func (m *MockJiraServer) setupRoutes(mux *http.ServeMux) {
	// Authentication endpoint
	mux.HandleFunc("/rest/api/2/myself", m.handleMyself)
	
	// Project endpoints
	mux.HandleFunc("/rest/api/2/project/", m.handleProject)
	
	// Field configuration endpoint
	mux.HandleFunc("/rest/api/2/field", m.handleFields)
	
	// Issue type endpoint
	mux.HandleFunc("/rest/api/2/issuetype", m.handleIssueTypes)
	
	// Server info endpoint (for JIRA version detection)
	mux.HandleFunc("/rest/api/2/serverInfo", m.handleServerInfo)
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