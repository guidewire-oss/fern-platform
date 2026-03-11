package integrations_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===== TestConnection via HTTP =====

func TestDefaultJiraClient_TestConnection_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rest/api/2/myself", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.Header.Get("Authorization"), "Basic ")
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"displayName": "Test User"})
	}))
	defer server.Close()

	client := integrations.NewDefaultJiraClient()
	err := client.TestConnection(context.Background(), server.URL, "user@example.com", "api-token", integrations.AuthTypeAPIToken)

	assert.NoError(t, err)
}

func TestDefaultJiraClient_TestConnection_AuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"message": "Unauthorized"})
	}))
	defer server.Close()

	client := integrations.NewDefaultJiraClient()
	err := client.TestConnection(context.Background(), server.URL, "user@example.com", "bad-token", integrations.AuthTypeAPIToken)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JIRA authentication failed")
	assert.Contains(t, err.Error(), "401")
}

func TestDefaultJiraClient_TestConnection_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := integrations.NewDefaultJiraClient()
	err := client.TestConnection(context.Background(), server.URL, "user@example.com", "token", integrations.AuthTypeAPIToken)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestDefaultJiraClient_TestConnection_ConnectionRefused(t *testing.T) {
	client := integrations.NewDefaultJiraClient()
	// Use an address that will refuse the connection
	err := client.TestConnection(context.Background(), "http://127.0.0.1:1", "user", "token", integrations.AuthTypeAPIToken)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to JIRA")
}

func TestDefaultJiraClient_TestConnection_CancelledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	client := integrations.NewDefaultJiraClient()
	err := client.TestConnection(ctx, server.URL, "user", "token", integrations.AuthTypeAPIToken)

	assert.Error(t, err)
}

// ===== Auth type header verification =====

func TestDefaultJiraClient_TestConnection_APITokenAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		assert.Contains(t, authHeader, "Basic ")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := integrations.NewDefaultJiraClient()
	err := client.TestConnection(context.Background(), server.URL, "user@example.com", "api-token", integrations.AuthTypeAPIToken)
	assert.NoError(t, err)
}

func TestDefaultJiraClient_TestConnection_OAuthAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		assert.Equal(t, "Bearer oauth-token", authHeader)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := integrations.NewDefaultJiraClient()
	err := client.TestConnection(context.Background(), server.URL, "user", "oauth-token", integrations.AuthTypeOAuth)
	assert.NoError(t, err)
}

func TestDefaultJiraClient_TestConnection_PATAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		assert.Equal(t, "Bearer my-pat", authHeader)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := integrations.NewDefaultJiraClient()
	err := client.TestConnection(context.Background(), server.URL, "user", "my-pat", integrations.AuthTypePersonalAccessToken)
	assert.NoError(t, err)
}

// ===== GetProject tests =====

func TestDefaultJiraClient_GetProject_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rest/api/2/project/PROJ", r.URL.Path)
		assert.Equal(t, "GET", r.Method)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"id":   "10000",
			"key":  "PROJ",
			"name": "My Project",
		})
	}))
	defer server.Close()

	client := integrations.NewDefaultJiraClient()
	project, err := client.GetProject(context.Background(), server.URL, "PROJ", "user@example.com", "token", integrations.AuthTypeAPIToken)

	require.NoError(t, err)
	require.NotNil(t, project)
	assert.Equal(t, "10000", project.ID)
	assert.Equal(t, "PROJ", project.Key)
	assert.Equal(t, "My Project", project.Name)
}

func TestDefaultJiraClient_GetProject_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := integrations.NewDefaultJiraClient()
	project, err := client.GetProject(context.Background(), server.URL, "NONEXIST", "user", "token", integrations.AuthTypeAPIToken)

	assert.Error(t, err)
	assert.Nil(t, project)
	assert.Contains(t, err.Error(), "not found")
}

func TestDefaultJiraClient_GetProject_AuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	client := integrations.NewDefaultJiraClient()
	project, err := client.GetProject(context.Background(), server.URL, "PROJ", "user", "bad-token", integrations.AuthTypeAPIToken)

	assert.Error(t, err)
	assert.Nil(t, project)
	assert.Contains(t, err.Error(), "failed to get project")
}

func TestDefaultJiraClient_GetProject_ConnectionError(t *testing.T) {
	client := integrations.NewDefaultJiraClient()
	project, err := client.GetProject(context.Background(), "http://127.0.0.1:1", "PROJ", "user", "token", integrations.AuthTypeAPIToken)

	assert.Error(t, err)
	assert.Nil(t, project)
	assert.Contains(t, err.Error(), "failed to connect to JIRA")
}

func TestDefaultJiraClient_GetProject_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	client := integrations.NewDefaultJiraClient()
	project, err := client.GetProject(context.Background(), server.URL, "PROJ", "user", "token", integrations.AuthTypeAPIToken)

	assert.Error(t, err)
	assert.Nil(t, project)
	assert.Contains(t, err.Error(), "failed to parse project response")
}

// ===== Common headers =====

func TestDefaultJiraClient_CommonHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		assert.Equal(t, "Fern-Platform/1.0", r.Header.Get("User-Agent"))
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer server.Close()

	client := integrations.NewDefaultJiraClient()
	err := client.TestConnection(context.Background(), server.URL, "user", "token", integrations.AuthTypeAPIToken)
	assert.NoError(t, err)
}
