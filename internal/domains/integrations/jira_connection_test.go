package integrations_test

import (
	"context"
	"errors"
	"testing"

	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewJiraConnection(t *testing.T) {
	tests := []struct {
		name        string
		projectID   string
		connName    string
		jiraURL     string
		authType    integrations.AuthenticationType
		projectKey  string
		username    string
		credential  string
		wantErr     bool
		errContains string
	}{
		{
			name:       "valid connection with API token",
			projectID:  "proj-123",
			connName:   "Production JIRA",
			jiraURL:    "https://example.atlassian.net",
			authType:   integrations.AuthTypeAPIToken,
			projectKey: "PROJ",
			username:   "user@example.com",
			credential: "test-token",
			wantErr:    false,
		},
		{
			name:       "valid connection with OAuth",
			projectID:  "proj-123",
			connName:   "Dev JIRA",
			jiraURL:    "https://jira.company.com",
			authType:   integrations.AuthTypeOAuth,
			projectKey: "DEV",
			username:   "oauth-user",
			credential: "oauth-token",
			wantErr:    false,
		},
		{
			name:        "empty project ID",
			projectID:   "",
			connName:    "Test",
			jiraURL:     "https://example.atlassian.net",
			authType:    integrations.AuthTypeAPIToken,
			projectKey:  "TEST",
			username:    "user@example.com",
			credential:  "token",
			wantErr:     true,
			errContains: "project ID is required",
		},
		{
			name:        "empty connection name",
			projectID:   "proj-123",
			connName:    "",
			jiraURL:     "https://example.atlassian.net",
			authType:    integrations.AuthTypeAPIToken,
			projectKey:  "TEST",
			username:    "user@example.com",
			credential:  "token",
			wantErr:     true,
			errContains: "connection name is required",
		},
		{
			name:        "invalid JIRA URL - no protocol",
			projectID:   "proj-123",
			connName:    "Test",
			jiraURL:     "example.atlassian.net",
			authType:    integrations.AuthTypeAPIToken,
			projectKey:  "TEST",
			username:    "user@example.com",
			credential:  "token",
			wantErr:     true,
			errContains: "JIRA URL must start with http:// or https://",
		},
		{
			name:        "invalid JIRA URL - malformed",
			projectID:   "proj-123",
			connName:    "Test",
			jiraURL:     "not-a-url",
			authType:    integrations.AuthTypeAPIToken,
			projectKey:  "TEST",
			username:    "user@example.com",
			credential:  "token",
			wantErr:     true,
			errContains: "JIRA URL must start with http:// or https://",
		},
		{
			name:        "empty project key",
			projectID:   "proj-123",
			connName:    "Test",
			jiraURL:     "https://example.atlassian.net",
			authType:    integrations.AuthTypeAPIToken,
			projectKey:  "",
			username:    "user@example.com",
			credential:  "token",
			wantErr:     true,
			errContains: "project key is required",
		},
		{
			name:        "empty username",
			projectID:   "proj-123",
			connName:    "Test",
			jiraURL:     "https://example.atlassian.net",
			authType:    integrations.AuthTypeAPIToken,
			projectKey:  "TEST",
			username:    "",
			credential:  "token",
			wantErr:     true,
			errContains: "username is required",
		},
		{
			name:        "empty credential",
			projectID:   "proj-123",
			connName:    "Test",
			jiraURL:     "https://example.atlassian.net",
			authType:    integrations.AuthTypeAPIToken,
			projectKey:  "TEST",
			username:    "user@example.com",
			credential:  "",
			wantErr:     true,
			errContains: "credential is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := integrations.NewJiraConnection(
				tt.projectID,
				tt.connName,
				tt.jiraURL,
				tt.authType,
				tt.projectKey,
				tt.username,
				tt.credential,
			)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, conn)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, conn)
				assert.NotEmpty(t, conn.ID())
				assert.Equal(t, tt.projectID, conn.ProjectID())
				assert.Equal(t, tt.connName, conn.Name())
				assert.Equal(t, tt.jiraURL, conn.JiraURL())
				assert.Equal(t, tt.authType, conn.AuthenticationType())
				assert.Equal(t, tt.projectKey, conn.ProjectKey())
				assert.Equal(t, tt.username, conn.Username())
				assert.False(t, conn.IsActive())
				assert.NotNil(t, conn.CreatedAt())
				assert.NotNil(t, conn.UpdatedAt())
			}
		})
	}
}

func TestJiraConnection_UpdateConnectionInfo(t *testing.T) {
	// Test setup

	tests := []struct {
		name        string
		newName     string
		newURL      string
		newKey      string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid update",
			newName: "Updated Name",
			newURL:  "https://updated.atlassian.net",
			newKey:  "UPD",
			wantErr: false,
		},
		{
			name:        "empty name",
			newName:     "",
			newURL:      "https://updated.atlassian.net",
			newKey:      "UPD",
			wantErr:     true,
			errContains: "connection name is required",
		},
		{
			name:        "invalid URL",
			newName:     "Updated Name",
			newURL:      "not-a-url",
			newKey:      "UPD",
			wantErr:     true,
			errContains: "JIRA URL must start with http:// or https://",
		},
		{
			name:        "empty project key",
			newName:     "Updated Name",
			newURL:      "https://updated.atlassian.net",
			newKey:      "",
			wantErr:     true,
			errContains: "project key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh connection for each test
			testConn, _ := integrations.NewJiraConnection(
				"proj-123",
				"Test Name",
				"https://test.atlassian.net",
				integrations.AuthTypeAPIToken,
				"TEST",
				"test@example.com",
				"test-token",
			)

			err := testConn.UpdateConnectionInfo(tt.newName, tt.newURL, tt.newKey)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.newName, testConn.Name())
				assert.Equal(t, tt.newURL, testConn.JiraURL())
				assert.Equal(t, tt.newKey, testConn.ProjectKey())
			}
		})
	}
}

func TestJiraConnection_UpdateCredentials(t *testing.T) {
	// Test setup

	tests := []struct {
		name        string
		authType    integrations.AuthenticationType
		username    string
		credential  string
		wantErr     bool
		errContains string
	}{
		{
			name:       "valid credential update",
			authType:   integrations.AuthTypePersonalAccessToken,
			username:   "new-user@example.com",
			credential: "new-token",
			wantErr:    false,
		},
		{
			name:        "empty username",
			authType:    integrations.AuthTypeAPIToken,
			username:    "",
			credential:  "new-token",
			wantErr:     true,
			errContains: "username is required",
		},
		{
			name:        "empty credential",
			authType:    integrations.AuthTypeAPIToken,
			username:    "user@example.com",
			credential:  "",
			wantErr:     true,
			errContains: "credential is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh connection for each test
			testConn, _ := integrations.NewJiraConnection(
				"proj-123",
				"Test Name",
				"https://test.atlassian.net",
				integrations.AuthTypeAPIToken,
				"TEST",
				"test@example.com",
				"test-token",
			)

			err := testConn.UpdateCredentials(tt.authType, tt.username, tt.credential)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.authType, testConn.AuthenticationType())
				assert.Equal(t, tt.username, testConn.Username())
			}
		})
	}
}

func TestJiraConnection_TestConnection(t *testing.T) {
	conn, err := integrations.NewJiraConnection(
		"proj-123",
		"Test Connection",
		"https://test.atlassian.net",
		integrations.AuthTypeAPIToken,
		"TEST",
		"test@example.com",
		"test-token",
	)
	require.NoError(t, err)

	ctx := context.Background()

	// Test with mock client that succeeds
	mockClient := &mockJiraClient{shouldSucceed: true}
	err = conn.TestConnection(ctx, mockClient)
	assert.NoError(t, err)
	assert.NotNil(t, conn.LastTestedAt())
	assert.Equal(t, integrations.ConnectionStatusConnected, conn.Status())

	// Test with mock client that fails
	mockClient.shouldSucceed = false
	mockClient.errorMsg = "authentication failed"
	err = conn.TestConnection(ctx, mockClient)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authentication failed")
	assert.Equal(t, integrations.ConnectionStatusFailed, conn.Status())
}

func TestJiraConnection_Activate_Deactivate(t *testing.T) {
	conn, err := integrations.NewJiraConnection(
		"proj-123",
		"Test Connection",
		"https://test.atlassian.net",
		integrations.AuthTypeAPIToken,
		"TEST",
		"test@example.com",
		"test-token",
	)
	require.NoError(t, err)

	// Initially inactive
	assert.False(t, conn.IsActive())

	// Activate
	conn.Activate()
	assert.True(t, conn.IsActive())

	// Deactivate
	conn.Deactivate()
	assert.False(t, conn.IsActive())
}

func TestJiraConnection_EncryptDecryptCredential(t *testing.T) {
	conn, err := integrations.NewJiraConnection(
		"proj-123",
		"Test Connection",
		"https://test.atlassian.net",
		integrations.AuthTypeAPIToken,
		"TEST",
		"test@example.com",
		"test-token",
	)
	require.NoError(t, err)

	// Test encryption
	encryptionKey := []byte("test-encryption-key-32-bytes-lon")
	encrypted, err := conn.GetEncryptedCredential(encryptionKey)
	assert.NoError(t, err)
	assert.NotEmpty(t, encrypted)
	assert.NotEqual(t, "test-token", encrypted)

	// Test decryption
	decrypted, err := integrations.DecryptCredential(encrypted, encryptionKey)
	assert.NoError(t, err)
	assert.Equal(t, "test-token", decrypted)
}

// Mock JIRA client for testing
type mockJiraClient struct {
	shouldSucceed bool
	errorMsg      string
}

func (m *mockJiraClient) TestConnection(ctx context.Context, url, username, credential string, authType integrations.AuthenticationType) error {
	if !m.shouldSucceed {
		if m.errorMsg != "" {
			return errors.New(m.errorMsg)
		}
		return assert.AnError
	}
	return nil
}

func (m *mockJiraClient) GetProject(ctx context.Context, url, projectKey, username, credential string, authType integrations.AuthenticationType) (*integrations.JiraProject, error) {
	if !m.shouldSucceed {
		return nil, assert.AnError
	}
	return &integrations.JiraProject{
		ID:   "10000",
		Key:  projectKey,
		Name: "Test Project",
	}, nil
}