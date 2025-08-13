package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// Atlassian Cloud JIRA API structures
type JiraProject struct {
	Expand          string          `json:"expand"`
	Self            string          `json:"self"`
	ID              string          `json:"id"`
	Key             string          `json:"key"`
	Name            string          `json:"name"`
	AvatarUrls      AvatarUrls      `json:"avatarUrls"`
	ProjectTypeKey  string          `json:"projectTypeKey"`
	ProjectCategory ProjectCategory `json:"projectCategory,omitempty"`
	IssueTypes      []IssueType     `json:"issueTypes,omitempty"`
}

type AvatarUrls struct {
	Size48x48 string `json:"48x48"`
	Size24x24 string `json:"24x24"`
	Size16x16 string `json:"16x16"`
	Size32x32 string `json:"32x32"`
}

type ProjectCategory struct {
	Self        string `json:"self"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type JiraUser struct {
	Self         string     `json:"self"`
	AccountID    string     `json:"accountId"`
	AccountType  string     `json:"accountType"`
	EmailAddress string     `json:"emailAddress"`
	AvatarUrls   AvatarUrls `json:"avatarUrls"`
	DisplayName  string     `json:"displayName"`
	Active       bool       `json:"active"`
	TimeZone     string     `json:"timeZone"`
	Locale       string     `json:"locale"`
	Groups       Groups     `json:"groups"`
	Expand       string     `json:"expand"`
}

type Groups struct {
	Size  int         `json:"size"`
	Items []GroupItem `json:"items"`
}

type GroupItem struct {
	Name string `json:"name"`
	Self string `json:"self"`
}

type IssueType struct {
	Self        string `json:"self"`
	ID          string `json:"id"`
	Description string `json:"description"`
	IconUrl     string `json:"iconUrl"`
	Name        string `json:"name"`
	Subtask     bool   `json:"subtask"`
	AvatarID    int    `json:"avatarId"`
}

type JiraField struct {
	ID             string         `json:"id"`
	Key            string         `json:"key"`
	Name           string         `json:"name"`
	Custom         bool           `json:"custom"`
	Orderable      bool           `json:"orderable"`
	Navigable      bool           `json:"navigable"`
	Searchable     bool           `json:"searchable"`
	ClauseNames    []string       `json:"clauseNames"`
	Schema         FieldSchema    `json:"schema"`
}

type FieldSchema struct {
	Type     string `json:"type"`
	Items    string `json:"items,omitempty"`
	System   string `json:"system,omitempty"`
	Custom   string `json:"custom,omitempty"`
	CustomID int    `json:"customId,omitempty"`
}

type ServerInfo struct {
	BaseUrl         string   `json:"baseUrl"`
	Version         string   `json:"version"`
	VersionNumbers  []int    `json:"versionNumbers"`
	DeploymentType  string   `json:"deploymentType"`
	BuildNumber     int      `json:"buildNumber"`
	BuildDate       string   `json:"buildDate"`
	ServerTime      string   `json:"serverTime"`
	ScmInfo         string   `json:"scmInfo"`
	ServerTitle     string   `json:"serverTitle"`
	HealthChecks    []Health `json:"healthChecks"`
}

type Health struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Passed      bool   `json:"passed"`
}

type ErrorResponse struct {
	ErrorMessages []string          `json:"errorMessages"`
	Errors        map[string]string `json:"errors"`
}

var validTokens = map[string]bool{
	"test-api-token-123": true,
	"valid-token":        true,
	"demo-token":         true,
}

var projects = map[string]JiraProject{
	"FERN": {
		Expand: "description,lead,issueTypes,url,projectKeys,permissions,insight",
		Self:   "https://fern-platform.atlassian.net/rest/api/2/project/10000",
		ID:     "10000",
		Key:    "FERN",
		Name:   "Fern Platform",
		AvatarUrls: AvatarUrls{
			Size48x48: "https://fern-platform.atlassian.net/rest/api/2/universal_avatar/view/type/project/avatar/10200",
			Size24x24: "https://fern-platform.atlassian.net/rest/api/2/universal_avatar/view/type/project/avatar/10200?size=small",
			Size16x16: "https://fern-platform.atlassian.net/rest/api/2/universal_avatar/view/type/project/avatar/10200?size=xsmall",
			Size32x32: "https://fern-platform.atlassian.net/rest/api/2/universal_avatar/view/type/project/avatar/10200?size=medium",
		},
		ProjectTypeKey: "software",
		ProjectCategory: ProjectCategory{
			Self:        "https://fern-platform.atlassian.net/rest/api/2/projectCategory/10000",
			ID:          "10000",
			Name:        "Development",
			Description: "Development projects",
		},
	},
	"TEST": {
		Expand: "description,lead,issueTypes,url,projectKeys,permissions,insight",
		Self:   "https://fern-platform.atlassian.net/rest/api/2/project/10001",
		ID:     "10001",
		Key:    "TEST",
		Name:   "Test Project",
		AvatarUrls: AvatarUrls{
			Size48x48: "https://fern-platform.atlassian.net/rest/api/2/universal_avatar/view/type/project/avatar/10201",
			Size24x24: "https://fern-platform.atlassian.net/rest/api/2/universal_avatar/view/type/project/avatar/10201?size=small",
			Size16x16: "https://fern-platform.atlassian.net/rest/api/2/universal_avatar/view/type/project/avatar/10201?size=xsmall",
			Size32x32: "https://fern-platform.atlassian.net/rest/api/2/universal_avatar/view/type/project/avatar/10201?size=medium",
		},
		ProjectTypeKey: "software",
	},
	"DEMO": {
		Expand: "description,lead,issueTypes,url,projectKeys,permissions,insight",
		Self:   "https://fern-platform.atlassian.net/rest/api/2/project/10002",
		ID:     "10002",
		Key:    "DEMO",
		Name:   "Demo Project",
		AvatarUrls: AvatarUrls{
			Size48x48: "https://fern-platform.atlassian.net/rest/api/2/universal_avatar/view/type/project/avatar/10202",
			Size24x24: "https://fern-platform.atlassian.net/rest/api/2/universal_avatar/view/type/project/avatar/10202?size=small",
			Size16x16: "https://fern-platform.atlassian.net/rest/api/2/universal_avatar/view/type/project/avatar/10202?size=xsmall",
			Size32x32: "https://fern-platform.atlassian.net/rest/api/2/universal_avatar/view/type/project/avatar/10202?size=medium",
		},
		ProjectTypeKey: "business",
		ProjectCategory: ProjectCategory{
			Self:        "https://fern-platform.atlassian.net/rest/api/2/projectCategory/10001",
			ID:          "10001",
			Name:        "Business",
			Description: "Business projects",
		},
	},
}

func authenticate(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return false
	}

	if strings.HasPrefix(auth, "Basic ") {
		// For Basic auth, check if it's properly formatted
		return len(auth) > 6
	} else if strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimPrefix(auth, "Bearer ")
		return validTokens[token]
	}

	return false
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		ErrorMessages: []string{message},
		Errors:        make(map[string]string),
	})
}

func handleMyself(w http.ResponseWriter, r *http.Request) {
	if !authenticate(r) {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	user := JiraUser{
		Self:         "https://fern-platform.atlassian.net/rest/api/2/user?accountId=5b10a2844c20165700ede21g",
		AccountID:    "5b10a2844c20165700ede21g",
		AccountType:  "atlassian",
		EmailAddress: "test@fern.com",
		AvatarUrls: AvatarUrls{
			Size48x48: "https://avatar-management--avatars.us-west-2.prod.public.atl-paas.net/5b10a2844c20165700ede21g/48",
			Size24x24: "https://avatar-management--avatars.us-west-2.prod.public.atl-paas.net/5b10a2844c20165700ede21g/24",
			Size16x16: "https://avatar-management--avatars.us-west-2.prod.public.atl-paas.net/5b10a2844c20165700ede21g/16",
			Size32x32: "https://avatar-management--avatars.us-west-2.prod.public.atl-paas.net/5b10a2844c20165700ede21g/32",
		},
		DisplayName: "Test User",
		Active:      true,
		TimeZone:    "America/Los_Angeles",
		Locale:      "en_US",
		Groups: Groups{
			Size: 2,
			Items: []GroupItem{
				{Name: "jira-administrators", Self: "https://fern-platform.atlassian.net/rest/api/2/group?groupname=jira-administrators"},
				{Name: "jira-software-users", Self: "https://fern-platform.atlassian.net/rest/api/2/group?groupname=jira-software-users"},
			},
		},
		Expand: "groups,applicationRoles",
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-AUSERNAME", "test@fern.com")
	json.NewEncoder(w).Encode(user)
}

func handleProject(w http.ResponseWriter, r *http.Request) {
	if !authenticate(r) {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 {
		writeError(w, http.StatusBadRequest, "Invalid request")
		return
	}
	projectKey := parts[4]

	project, exists := projects[projectKey]
	if !exists {
		writeError(w, http.StatusNotFound, fmt.Sprintf("No project could be found with key '%s'.", projectKey))
		return
	}

	// Add issue types if requested
	if strings.Contains(r.URL.Query().Get("expand"), "issueTypes") {
		project.IssueTypes = getIssueTypes()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(project)
}

func handleProjects(w http.ResponseWriter, r *http.Request) {
	if !authenticate(r) {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	projectList := make([]JiraProject, 0, len(projects))
	for _, p := range projects {
		projectList = append(projectList, p)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(projectList)
}

func getIssueTypes() []IssueType {
	return []IssueType{
		{
			Self:        "https://fern-platform.atlassian.net/rest/api/2/issuetype/10000",
			ID:          "10000",
			Description: "A big user story that needs to be broken down. Created by Jira Software - do not edit or delete.",
			IconUrl:     "https://fern-platform.atlassian.net/images/icons/issuetypes/epic.svg",
			Name:        "Epic",
			Subtask:     false,
			AvatarID:    10307,
		},
		{
			Self:        "https://fern-platform.atlassian.net/rest/api/2/issuetype/10001",
			ID:          "10001",
			Description: "Functionality or a feature expressed as a user goal.",
			IconUrl:     "https://fern-platform.atlassian.net/rest/api/2/universal_avatar/view/type/issuetype/avatar/10315",
			Name:        "Story",
			Subtask:     false,
			AvatarID:    10315,
		},
		{
			Self:        "https://fern-platform.atlassian.net/rest/api/2/issuetype/10002",
			ID:          "10002",
			Description: "A small piece of work that's part of a larger task.",
			IconUrl:     "https://fern-platform.atlassian.net/rest/api/2/universal_avatar/view/type/issuetype/avatar/10318",
			Name:        "Task",
			Subtask:     false,
			AvatarID:    10318,
		},
		{
			Self:        "https://fern-platform.atlassian.net/rest/api/2/issuetype/10003",
			ID:          "10003",
			Description: "A problem or error.",
			IconUrl:     "https://fern-platform.atlassian.net/rest/api/2/universal_avatar/view/type/issuetype/avatar/10303",
			Name:        "Bug",
			Subtask:     false,
			AvatarID:    10303,
		},
	}
}

func handleFields(w http.ResponseWriter, r *http.Request) {
	if !authenticate(r) {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	fields := []JiraField{
		{
			ID:         "summary",
			Key:        "summary",
			Name:       "Summary",
			Custom:     false,
			Orderable:  true,
			Navigable:  true,
			Searchable: true,
			ClauseNames: []string{"summary"},
			Schema:     FieldSchema{Type: "string", System: "summary"},
		},
		{
			ID:         "issuetype",
			Key:        "issuetype",
			Name:       "Issue Type",
			Custom:     false,
			Orderable:  true,
			Navigable:  true,
			Searchable: true,
			ClauseNames: []string{"issuetype", "type"},
			Schema:     FieldSchema{Type: "issuetype"},
		},
		{
			ID:         "project",
			Key:        "project",
			Name:       "Project",
			Custom:     false,
			Orderable:  false,
			Navigable:  true,
			Searchable: true,
			ClauseNames: []string{"project"},
			Schema:     FieldSchema{Type: "project"},
		},
		{
			ID:         "customfield_10000",
			Key:        "customfield_10000",
			Name:       "Epic Link",
			Custom:     true,
			Orderable:  true,
			Navigable:  true,
			Searchable: true,
			ClauseNames: []string{"cf[10000]", "Epic Link"},
			Schema:     FieldSchema{Type: "string", Custom: "com.pyxis.greenhopper.jira:gh-epic-link", CustomID: 10000},
		},
		{
			ID:         "customfield_10001",
			Key:        "customfield_10001",
			Name:       "Story Points",
			Custom:     true,
			Orderable:  true,
			Navigable:  true,
			Searchable: true,
			ClauseNames: []string{"cf[10001]", "Story Points"},
			Schema:     FieldSchema{Type: "number", Custom: "com.atlassian.jira.plugin.system.customfieldtypes:float", CustomID: 10001},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fields)
}

func handleIssueTypes(w http.ResponseWriter, r *http.Request) {
	if !authenticate(r) {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(getIssueTypes())
}

func handleServerInfo(w http.ResponseWriter, r *http.Request) {
	serverInfo := ServerInfo{
		BaseUrl:        "https://fern-platform.atlassian.net",
		Version:        "1001.0.0-SNAPSHOT",
		VersionNumbers: []int{1001, 0, 0},
		DeploymentType: "Cloud",
		BuildNumber:    100216,
		BuildDate:      "2025-07-26T05:36:26.000+0000",
		ServerTime:     time.Now().Format("2006-01-02T15:04:05.000-0700"),
		ScmInfo:        "7f31073df3dd9154d360365c3a7d0b0c21126055",
		ServerTitle:    "Fern Platform JIRA",
		HealthChecks: []Health{
			{Name: "db.connection", Description: "Database connection", Passed: true},
			{Name: "db.query", Description: "Database query", Passed: true},
			{Name: "lucene.index", Description: "Lucene index", Passed: true},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(serverInfo)
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"name":        "Mock JIRA Cloud Server",
		"description": "Realistic mock of Atlassian JIRA Cloud API",
		"version":     "1001.0.0-SNAPSHOT",
		"endpoints": []string{
			"/rest/api/2/myself",
			"/rest/api/2/project/{projectKey}",
			"/rest/api/2/project",
			"/rest/api/2/field",
			"/rest/api/2/issuetype",
			"/rest/api/2/serverInfo",
		},
		"authentication": map[string]interface{}{
			"bearer_tokens": []string{"test-api-token-123", "valid-token", "demo-token"},
			"basic_auth":    "any username/password combination",
		},
		"projects": []map[string]string{
			{"key": "FERN", "name": "Fern Platform"},
			{"key": "TEST", "name": "Test Project"},
			{"key": "DEMO", "name": "Demo Project"},
		},
	}
	json.NewEncoder(w).Encode(response)
}

func enableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next(w, r)
	}
}

func main() {
	mux := http.NewServeMux()

	// Root handler
	mux.HandleFunc("/", enableCORS(handleRoot))

	// JIRA API endpoints
	mux.HandleFunc("/rest/api/2/myself", enableCORS(handleMyself))
	mux.HandleFunc("/rest/api/2/project/", enableCORS(handleProject))
	mux.HandleFunc("/rest/api/2/project", enableCORS(handleProjects))
	mux.HandleFunc("/rest/api/2/field", enableCORS(handleFields))
	mux.HandleFunc("/rest/api/2/issuetype", enableCORS(handleIssueTypes))
	mux.HandleFunc("/rest/api/2/serverInfo", enableCORS(handleServerInfo))

	log.Println("Mock JIRA Cloud Server starting on :8080")
	log.Println("Visit http://localhost:8080 for API information")

	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}