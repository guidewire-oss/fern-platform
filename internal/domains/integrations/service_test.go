package integrations_test

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Mock repository ---

type MockJiraConnectionRepository struct {
	mock.Mock
}

func (m *MockJiraConnectionRepository) Create(ctx context.Context, connection *integrations.JiraConnection) error {
	args := m.Called(ctx, connection)
	return args.Error(0)
}

func (m *MockJiraConnectionRepository) Update(ctx context.Context, connection *integrations.JiraConnection) error {
	args := m.Called(ctx, connection)
	return args.Error(0)
}

func (m *MockJiraConnectionRepository) Delete(ctx context.Context, connectionID string) error {
	args := m.Called(ctx, connectionID)
	return args.Error(0)
}

func (m *MockJiraConnectionRepository) FindByID(ctx context.Context, connectionID string) (*integrations.JiraConnection, error) {
	args := m.Called(ctx, connectionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*integrations.JiraConnection), args.Error(1)
}

func (m *MockJiraConnectionRepository) FindByProjectID(ctx context.Context, projectID string) ([]*integrations.JiraConnection, error) {
	args := m.Called(ctx, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*integrations.JiraConnection), args.Error(1)
}

func (m *MockJiraConnectionRepository) FindActiveByProjectID(ctx context.Context, projectID string) ([]*integrations.JiraConnection, error) {
	args := m.Called(ctx, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*integrations.JiraConnection), args.Error(1)
}

// --- Mock JIRA client ---

type MockJiraClientForService struct {
	mock.Mock
}

func (m *MockJiraClientForService) TestConnection(ctx context.Context, url, username, credential string, authType integrations.AuthenticationType) error {
	args := m.Called(ctx, url, username, credential, authType)
	return args.Error(0)
}

func (m *MockJiraClientForService) GetProject(ctx context.Context, url, projectKey, username, credential string, authType integrations.AuthenticationType) (*integrations.JiraProject, error) {
	args := m.Called(ctx, url, projectKey, username, credential, authType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*integrations.JiraProject), args.Error(1)
}

// --- Helper to encrypt a credential the same way the production code does ---

func encryptCredential(plaintext string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	plaintextBytes := []byte(plaintext)
	ciphertextBytes := make([]byte, aes.BlockSize+len(plaintextBytes))
	iv := ciphertextBytes[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertextBytes[aes.BlockSize:], plaintextBytes)
	return base64.StdEncoding.EncodeToString(ciphertextBytes), nil
}

// Helper to build a reconstructed connection with an encrypted credential
func buildEncryptedConnection(id, projectID, name, jiraURL string, authType integrations.AuthenticationType, projectKey, username, plainCredential string, key []byte) *integrations.JiraConnection {
	encrypted, _ := encryptCredential(plainCredential, key)
	return integrations.ReconstructJiraConnection(
		id, projectID, name, jiraURL,
		authType, projectKey, username, encrypted,
		integrations.ConnectionStatusPending,
		false,
		nil,
		time.Now(), time.Now(),
	)
}

var testEncryptionKey = []byte("test-encryption-key-32-bytes-lon") // 32 bytes for AES-256

// ===== CreateConnection tests =====

func TestJiraConnectionService_CreateConnection_Success(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJiraConnectionRepository)
	mockClient := new(MockJiraClientForService)
	service := integrations.NewJiraConnectionService(mockRepo, mockClient, testEncryptionKey)

	mockRepo.On("FindByProjectID", ctx, "proj-1").Return([]*integrations.JiraConnection{}, nil)
	mockRepo.On("Create", ctx, mock.AnythingOfType("*integrations.JiraConnection")).Return(nil)

	conn, err := service.CreateConnection(ctx, "proj-1", "My JIRA", "https://jira.example.com", integrations.AuthTypeAPIToken, "PROJ", "user@example.com", "my-token")

	require.NoError(t, err)
	require.NotNil(t, conn)
	assert.Equal(t, "My JIRA", conn.Name())
	assert.Equal(t, "proj-1", conn.ProjectID())
	assert.Equal(t, "https://jira.example.com", conn.JiraURL())
	assert.Equal(t, "PROJ", conn.ProjectKey())
	mockRepo.AssertExpectations(t)
}

func TestJiraConnectionService_CreateConnection_DuplicateProject(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJiraConnectionRepository)
	mockClient := new(MockJiraClientForService)
	service := integrations.NewJiraConnectionService(mockRepo, mockClient, testEncryptionKey)

	existing := integrations.ReconstructJiraConnection(
		"existing-id", "proj-1", "Existing", "https://jira.example.com",
		integrations.AuthTypeAPIToken, "PROJ", "user", "cred",
		integrations.ConnectionStatusConnected, true, nil,
		time.Now(), time.Now(),
	)
	mockRepo.On("FindByProjectID", ctx, "proj-1").Return([]*integrations.JiraConnection{existing}, nil)

	conn, err := service.CreateConnection(ctx, "proj-1", "My JIRA", "https://jira.example.com", integrations.AuthTypeAPIToken, "PROJ", "user@example.com", "my-token")

	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "already has a JIRA connection")
	mockRepo.AssertExpectations(t)
}

func TestJiraConnectionService_CreateConnection_RepoCheckError(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJiraConnectionRepository)
	mockClient := new(MockJiraClientForService)
	service := integrations.NewJiraConnectionService(mockRepo, mockClient, testEncryptionKey)

	mockRepo.On("FindByProjectID", ctx, "proj-1").Return(nil, errors.New("db error"))

	conn, err := service.CreateConnection(ctx, "proj-1", "My JIRA", "https://jira.example.com", integrations.AuthTypeAPIToken, "PROJ", "user@example.com", "my-token")

	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "failed to check existing connections")
	mockRepo.AssertExpectations(t)
}

func TestJiraConnectionService_CreateConnection_InvalidInput(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJiraConnectionRepository)
	mockClient := new(MockJiraClientForService)
	service := integrations.NewJiraConnectionService(mockRepo, mockClient, testEncryptionKey)

	mockRepo.On("FindByProjectID", ctx, "proj-1").Return([]*integrations.JiraConnection{}, nil)

	// Empty name should fail at NewJiraConnection
	conn, err := service.CreateConnection(ctx, "proj-1", "", "https://jira.example.com", integrations.AuthTypeAPIToken, "PROJ", "user@example.com", "my-token")

	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "failed to create connection")
	mockRepo.AssertExpectations(t)
}

func TestJiraConnectionService_CreateConnection_SaveError(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJiraConnectionRepository)
	mockClient := new(MockJiraClientForService)
	service := integrations.NewJiraConnectionService(mockRepo, mockClient, testEncryptionKey)

	mockRepo.On("FindByProjectID", ctx, "proj-1").Return([]*integrations.JiraConnection{}, nil)
	mockRepo.On("Create", ctx, mock.AnythingOfType("*integrations.JiraConnection")).Return(errors.New("save failed"))

	conn, err := service.CreateConnection(ctx, "proj-1", "My JIRA", "https://jira.example.com", integrations.AuthTypeAPIToken, "PROJ", "user@example.com", "my-token")

	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "failed to save connection")
	mockRepo.AssertExpectations(t)
}

// ===== GetConnection tests =====

func TestJiraConnectionService_GetConnection_Found(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJiraConnectionRepository)
	mockClient := new(MockJiraClientForService)
	service := integrations.NewJiraConnectionService(mockRepo, mockClient, testEncryptionKey)

	expected := integrations.ReconstructJiraConnection(
		"conn-1", "proj-1", "My JIRA", "https://jira.example.com",
		integrations.AuthTypeAPIToken, "PROJ", "user", "cred",
		integrations.ConnectionStatusConnected, true, nil,
		time.Now(), time.Now(),
	)
	mockRepo.On("FindByID", ctx, "conn-1").Return(expected, nil)

	conn, err := service.GetConnection(ctx, "conn-1")

	require.NoError(t, err)
	assert.Equal(t, "conn-1", conn.ID())
	assert.Equal(t, "My JIRA", conn.Name())
	mockRepo.AssertExpectations(t)
}

func TestJiraConnectionService_GetConnection_NotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJiraConnectionRepository)
	mockClient := new(MockJiraClientForService)
	service := integrations.NewJiraConnectionService(mockRepo, mockClient, testEncryptionKey)

	mockRepo.On("FindByID", ctx, "nonexistent").Return(nil, errors.New("not found"))

	conn, err := service.GetConnection(ctx, "nonexistent")

	assert.Error(t, err)
	assert.Nil(t, conn)
	mockRepo.AssertExpectations(t)
}

// ===== UpdateConnection tests =====

func TestJiraConnectionService_UpdateConnection_Success(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJiraConnectionRepository)
	mockClient := new(MockJiraClientForService)
	service := integrations.NewJiraConnectionService(mockRepo, mockClient, testEncryptionKey)

	existing := integrations.ReconstructJiraConnection(
		"conn-1", "proj-1", "Old Name", "https://old.atlassian.net",
		integrations.AuthTypeAPIToken, "OLD", "user", "cred",
		integrations.ConnectionStatusConnected, true, nil,
		time.Now(), time.Now(),
	)
	mockRepo.On("FindByID", ctx, "conn-1").Return(existing, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*integrations.JiraConnection")).Return(nil)

	conn, err := service.UpdateConnection(ctx, "conn-1", "New Name", "https://new.atlassian.net", "NEW")

	require.NoError(t, err)
	assert.Equal(t, "New Name", conn.Name())
	assert.Equal(t, "https://new.atlassian.net", conn.JiraURL())
	assert.Equal(t, "NEW", conn.ProjectKey())
	mockRepo.AssertExpectations(t)
}

func TestJiraConnectionService_UpdateConnection_NotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJiraConnectionRepository)
	mockClient := new(MockJiraClientForService)
	service := integrations.NewJiraConnectionService(mockRepo, mockClient, testEncryptionKey)

	mockRepo.On("FindByID", ctx, "nonexistent").Return(nil, errors.New("not found"))

	conn, err := service.UpdateConnection(ctx, "nonexistent", "Name", "https://jira.example.com", "PROJ")

	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "failed to find connection")
	mockRepo.AssertExpectations(t)
}

func TestJiraConnectionService_UpdateConnection_InvalidInput(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJiraConnectionRepository)
	mockClient := new(MockJiraClientForService)
	service := integrations.NewJiraConnectionService(mockRepo, mockClient, testEncryptionKey)

	existing := integrations.ReconstructJiraConnection(
		"conn-1", "proj-1", "Old Name", "https://old.atlassian.net",
		integrations.AuthTypeAPIToken, "OLD", "user", "cred",
		integrations.ConnectionStatusConnected, true, nil,
		time.Now(), time.Now(),
	)
	mockRepo.On("FindByID", ctx, "conn-1").Return(existing, nil)

	// Empty name should fail validation
	conn, err := service.UpdateConnection(ctx, "conn-1", "", "https://new.atlassian.net", "NEW")

	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "failed to update connection info")
	mockRepo.AssertExpectations(t)
}

func TestJiraConnectionService_UpdateConnection_SaveError(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJiraConnectionRepository)
	mockClient := new(MockJiraClientForService)
	service := integrations.NewJiraConnectionService(mockRepo, mockClient, testEncryptionKey)

	existing := integrations.ReconstructJiraConnection(
		"conn-1", "proj-1", "Old Name", "https://old.atlassian.net",
		integrations.AuthTypeAPIToken, "OLD", "user", "cred",
		integrations.ConnectionStatusConnected, true, nil,
		time.Now(), time.Now(),
	)
	mockRepo.On("FindByID", ctx, "conn-1").Return(existing, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*integrations.JiraConnection")).Return(errors.New("db error"))

	conn, err := service.UpdateConnection(ctx, "conn-1", "New Name", "https://new.atlassian.net", "NEW")

	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "failed to save connection")
	mockRepo.AssertExpectations(t)
}

// ===== UpdateCredentials tests =====

func TestJiraConnectionService_UpdateCredentials_Success(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJiraConnectionRepository)
	mockClient := new(MockJiraClientForService)
	service := integrations.NewJiraConnectionService(mockRepo, mockClient, testEncryptionKey)

	existing := integrations.ReconstructJiraConnection(
		"conn-1", "proj-1", "My JIRA", "https://jira.example.com",
		integrations.AuthTypeAPIToken, "PROJ", "old-user", "old-cred",
		integrations.ConnectionStatusConnected, true, nil,
		time.Now(), time.Now(),
	)
	mockRepo.On("FindByID", ctx, "conn-1").Return(existing, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*integrations.JiraConnection")).Return(nil)

	conn, err := service.UpdateCredentials(ctx, "conn-1", integrations.AuthTypePersonalAccessToken, "new-user", "new-token")

	require.NoError(t, err)
	assert.Equal(t, integrations.AuthTypePersonalAccessToken, conn.AuthenticationType())
	assert.Equal(t, "new-user", conn.Username())
	// Status should be reset to pending after credential update
	assert.Equal(t, integrations.ConnectionStatusPending, conn.Status())
	mockRepo.AssertExpectations(t)
}

func TestJiraConnectionService_UpdateCredentials_NotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJiraConnectionRepository)
	mockClient := new(MockJiraClientForService)
	service := integrations.NewJiraConnectionService(mockRepo, mockClient, testEncryptionKey)

	mockRepo.On("FindByID", ctx, "nonexistent").Return(nil, errors.New("not found"))

	conn, err := service.UpdateCredentials(ctx, "nonexistent", integrations.AuthTypeAPIToken, "user", "token")

	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "failed to find connection")
	mockRepo.AssertExpectations(t)
}

func TestJiraConnectionService_UpdateCredentials_InvalidInput(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJiraConnectionRepository)
	mockClient := new(MockJiraClientForService)
	service := integrations.NewJiraConnectionService(mockRepo, mockClient, testEncryptionKey)

	existing := integrations.ReconstructJiraConnection(
		"conn-1", "proj-1", "My JIRA", "https://jira.example.com",
		integrations.AuthTypeAPIToken, "PROJ", "user", "cred",
		integrations.ConnectionStatusConnected, true, nil,
		time.Now(), time.Now(),
	)
	mockRepo.On("FindByID", ctx, "conn-1").Return(existing, nil)

	// Empty username
	conn, err := service.UpdateCredentials(ctx, "conn-1", integrations.AuthTypeAPIToken, "", "token")

	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "failed to update credentials")
	mockRepo.AssertExpectations(t)
}

// ===== TestConnection tests =====

func TestJiraConnectionService_TestConnection_Success(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJiraConnectionRepository)
	mockClient := new(MockJiraClientForService)
	service := integrations.NewJiraConnectionService(mockRepo, mockClient, testEncryptionKey)

	conn := buildEncryptedConnection("conn-1", "proj-1", "My JIRA", "https://jira.example.com",
		integrations.AuthTypeAPIToken, "PROJ", "user@example.com", "my-token", testEncryptionKey)

	mockRepo.On("FindByID", ctx, "conn-1").Return(conn, nil)
	mockClient.On("TestConnection", ctx, "https://jira.example.com", "user@example.com", "my-token", integrations.AuthTypeAPIToken).Return(nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*integrations.JiraConnection")).Return(nil)

	err := service.TestConnection(ctx, "conn-1")

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
	mockClient.AssertExpectations(t)
}

func TestJiraConnectionService_TestConnection_Failure(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJiraConnectionRepository)
	mockClient := new(MockJiraClientForService)
	service := integrations.NewJiraConnectionService(mockRepo, mockClient, testEncryptionKey)

	conn := buildEncryptedConnection("conn-1", "proj-1", "My JIRA", "https://jira.example.com",
		integrations.AuthTypeAPIToken, "PROJ", "user@example.com", "my-token", testEncryptionKey)

	mockRepo.On("FindByID", ctx, "conn-1").Return(conn, nil)
	mockClient.On("TestConnection", ctx, "https://jira.example.com", "user@example.com", "my-token", integrations.AuthTypeAPIToken).Return(fmt.Errorf("authentication failed"))
	mockRepo.On("Update", ctx, mock.AnythingOfType("*integrations.JiraConnection")).Return(nil)

	err := service.TestConnection(ctx, "conn-1")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "authentication failed")
	mockRepo.AssertExpectations(t)
	mockClient.AssertExpectations(t)
}

func TestJiraConnectionService_TestConnection_NotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJiraConnectionRepository)
	mockClient := new(MockJiraClientForService)
	service := integrations.NewJiraConnectionService(mockRepo, mockClient, testEncryptionKey)

	mockRepo.On("FindByID", ctx, "nonexistent").Return(nil, errors.New("not found"))

	err := service.TestConnection(ctx, "nonexistent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find connection")
	mockRepo.AssertExpectations(t)
}

// ===== DeleteConnection tests =====

func TestJiraConnectionService_DeleteConnection_Success(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJiraConnectionRepository)
	mockClient := new(MockJiraClientForService)
	service := integrations.NewJiraConnectionService(mockRepo, mockClient, testEncryptionKey)

	mockRepo.On("Delete", ctx, "conn-1").Return(nil)

	err := service.DeleteConnection(ctx, "conn-1")

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestJiraConnectionService_DeleteConnection_Error(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJiraConnectionRepository)
	mockClient := new(MockJiraClientForService)
	service := integrations.NewJiraConnectionService(mockRepo, mockClient, testEncryptionKey)

	mockRepo.On("Delete", ctx, "conn-1").Return(errors.New("db error"))

	err := service.DeleteConnection(ctx, "conn-1")

	assert.Error(t, err)
	mockRepo.AssertExpectations(t)
}

// ===== ActivateConnection / DeactivateConnection tests =====

func TestJiraConnectionService_ActivateConnection_Success(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJiraConnectionRepository)
	mockClient := new(MockJiraClientForService)
	service := integrations.NewJiraConnectionService(mockRepo, mockClient, testEncryptionKey)

	conn := integrations.ReconstructJiraConnection(
		"conn-1", "proj-1", "My JIRA", "https://jira.example.com",
		integrations.AuthTypeAPIToken, "PROJ", "user", "cred",
		integrations.ConnectionStatusConnected, false, nil,
		time.Now(), time.Now(),
	)
	mockRepo.On("FindByID", ctx, "conn-1").Return(conn, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*integrations.JiraConnection")).Return(nil)

	err := service.ActivateConnection(ctx, "conn-1")

	assert.NoError(t, err)
	assert.True(t, conn.IsActive())
	mockRepo.AssertExpectations(t)
}

func TestJiraConnectionService_ActivateConnection_NotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJiraConnectionRepository)
	mockClient := new(MockJiraClientForService)
	service := integrations.NewJiraConnectionService(mockRepo, mockClient, testEncryptionKey)

	mockRepo.On("FindByID", ctx, "nonexistent").Return(nil, errors.New("not found"))

	err := service.ActivateConnection(ctx, "nonexistent")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find connection")
	mockRepo.AssertExpectations(t)
}

func TestJiraConnectionService_DeactivateConnection_Success(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJiraConnectionRepository)
	mockClient := new(MockJiraClientForService)
	service := integrations.NewJiraConnectionService(mockRepo, mockClient, testEncryptionKey)

	conn := integrations.ReconstructJiraConnection(
		"conn-1", "proj-1", "My JIRA", "https://jira.example.com",
		integrations.AuthTypeAPIToken, "PROJ", "user", "cred",
		integrations.ConnectionStatusConnected, true, nil,
		time.Now(), time.Now(),
	)
	mockRepo.On("FindByID", ctx, "conn-1").Return(conn, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*integrations.JiraConnection")).Return(nil)

	err := service.DeactivateConnection(ctx, "conn-1")

	assert.NoError(t, err)
	assert.False(t, conn.IsActive())
	mockRepo.AssertExpectations(t)
}

// ===== GetProjectConnections / GetActiveProjectConnections tests =====

func TestJiraConnectionService_GetProjectConnections(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJiraConnectionRepository)
	mockClient := new(MockJiraClientForService)
	service := integrations.NewJiraConnectionService(mockRepo, mockClient, testEncryptionKey)

	conns := []*integrations.JiraConnection{
		integrations.ReconstructJiraConnection(
			"conn-1", "proj-1", "JIRA 1", "https://jira1.example.com",
			integrations.AuthTypeAPIToken, "P1", "user", "cred",
			integrations.ConnectionStatusConnected, true, nil,
			time.Now(), time.Now(),
		),
	}
	mockRepo.On("FindByProjectID", ctx, "proj-1").Return(conns, nil)

	result, err := service.GetProjectConnections(ctx, "proj-1")

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "conn-1", result[0].ID())
	mockRepo.AssertExpectations(t)
}

func TestJiraConnectionService_GetActiveProjectConnections(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockJiraConnectionRepository)
	mockClient := new(MockJiraClientForService)
	service := integrations.NewJiraConnectionService(mockRepo, mockClient, testEncryptionKey)

	conns := []*integrations.JiraConnection{
		integrations.ReconstructJiraConnection(
			"conn-1", "proj-1", "JIRA 1", "https://jira1.example.com",
			integrations.AuthTypeAPIToken, "P1", "user", "cred",
			integrations.ConnectionStatusConnected, true, nil,
			time.Now(), time.Now(),
		),
	}
	mockRepo.On("FindActiveByProjectID", ctx, "proj-1").Return(conns, nil)

	result, err := service.GetActiveProjectConnections(ctx, "proj-1")

	require.NoError(t, err)
	assert.Len(t, result, 1)
	mockRepo.AssertExpectations(t)
}
