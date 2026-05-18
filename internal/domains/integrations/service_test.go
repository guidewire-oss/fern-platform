package integrations_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Minimal mock implementations ---

type mockConnectionRepo struct {
	stored      map[string]*integrations.JiraConnection
	updateFn    func(*integrations.JiraConnection)
	updateCount int
}

func newMockConnectionRepo() *mockConnectionRepo {
	return &mockConnectionRepo{stored: make(map[string]*integrations.JiraConnection)}
}

func (r *mockConnectionRepo) Create(_ context.Context, c *integrations.JiraConnection) error {
	r.stored[c.ID()] = c
	return nil
}

func (r *mockConnectionRepo) Update(_ context.Context, c *integrations.JiraConnection) error {
	r.stored[c.ID()] = c
	r.updateCount++
	if r.updateFn != nil {
		r.updateFn(c)
	}
	return nil
}

func (r *mockConnectionRepo) Delete(_ context.Context, id string) error {
	delete(r.stored, id)
	return nil
}

func (r *mockConnectionRepo) FindByID(_ context.Context, id string) (*integrations.JiraConnection, error) {
	c, ok := r.stored[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return c, nil
}

func (r *mockConnectionRepo) FindByProjectID(_ context.Context, projectID string) ([]*integrations.JiraConnection, error) {
	var out []*integrations.JiraConnection
	for _, c := range r.stored {
		if c.ProjectID() == projectID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (r *mockConnectionRepo) FindActiveByProjectID(_ context.Context, projectID string) ([]*integrations.JiraConnection, error) {
	var out []*integrations.JiraConnection
	for _, c := range r.stored {
		if c.ProjectID() == projectID && c.IsActive() {
			out = append(out, c)
		}
	}
	return out, nil
}

type mockJiraClientSvc struct {
	err    error
	fields []integrations.JiraField
}

func (m *mockJiraClientSvc) TestConnection(_ context.Context, _, _, _ string, _ integrations.AuthenticationType) error {
	return m.err
}

func (m *mockJiraClientSvc) GetProject(_ context.Context, _, _, _, _ string, _ integrations.AuthenticationType) (*integrations.JiraProject, error) {
	return nil, nil
}

func (m *mockJiraClientSvc) ListFields(_ context.Context, _, _, _ string, _ integrations.AuthenticationType) ([]integrations.JiraField, error) {
	return m.fields, m.err
}

// buildStoredConnection creates a connection with a properly-encrypted credential
// ready to be returned by the mock repo (mimicking a record read from the DB).
func buildStoredConnection(t *testing.T, key []byte) *integrations.JiraConnection {
	t.Helper()
	conn, err := integrations.NewJiraConnection(
		"proj-1", "Test", "https://jira.example.com",
		integrations.AuthTypeAPIToken, "PROJ", "user@example.com", "plaintext-token",
	)
	require.NoError(t, err)

	encrypted, err := conn.GetEncryptedCredential(key)
	require.NoError(t, err)

	now := time.Now()
	return integrations.ReconstructJiraConnection(
		conn.ID(), conn.ProjectID(), conn.Name(), conn.JiraURL(),
		conn.AuthenticationType(), conn.ProjectKey(), conn.Username(),
		encrypted,
		integrations.ConnectionStatusPending,
		false, nil, now, now,
	)
}

// --- Tests ---

func TestJiraConnectionService_TestConnection_ActivatesOnSuccess(t *testing.T) {
	key := make([]byte, 32) // zero key is fine for unit tests
	repo := newMockConnectionRepo()
	client := &mockJiraClientSvc{}
	svc := integrations.NewJiraConnectionService(repo, client, key)

	stored := buildStoredConnection(t, key)
	repo.stored[stored.ID()] = stored

	err := svc.TestConnection(context.Background(), stored.ID())
	require.NoError(t, err)

	assert.Equal(t, 1, repo.updateCount, "service must persist the updated connection")
	saved := repo.stored[stored.ID()]
	assert.True(t, saved.IsActive(), "connection must be activated after a successful test")
	assert.Equal(t, integrations.ConnectionStatusConnected, saved.Status())
	assert.NotNil(t, saved.LastTestedAt())
}

func TestJiraConnectionService_TestConnection_NotActivatedOnFailure(t *testing.T) {
	key := make([]byte, 32)
	repo := newMockConnectionRepo()
	client := &mockJiraClientSvc{err: errors.New("auth failed")}
	svc := integrations.NewJiraConnectionService(repo, client, key)

	stored := buildStoredConnection(t, key)
	repo.stored[stored.ID()] = stored

	err := svc.TestConnection(context.Background(), stored.ID())
	assert.Error(t, err)

	assert.Equal(t, 1, repo.updateCount, "service must persist the updated connection")
	saved := repo.stored[stored.ID()]
	assert.False(t, saved.IsActive(), "connection must not be activated after a failed test")
	assert.Equal(t, integrations.ConnectionStatusFailed, saved.Status())
}

func TestJiraConnectionService_ListJiraFields(t *testing.T) {
	key := make([]byte, 32)

	t.Run("decrypts credential and returns fields from client", func(t *testing.T) {
		repo := newMockConnectionRepo()
		wantFields := []integrations.JiraField{
			{ID: "summary", Name: "Summary", Custom: false},
			{ID: "status", Name: "Status", Custom: false},
		}
		client := &mockJiraClientSvc{fields: wantFields}
		svc := integrations.NewJiraConnectionService(repo, client, key)

		stored := buildStoredConnection(t, key)
		repo.stored[stored.ID()] = stored

		fields, err := svc.ListJiraFields(context.Background(), stored.ID())
		require.NoError(t, err)
		assert.Equal(t, wantFields, fields)
	})

	t.Run("returns error when connection not found", func(t *testing.T) {
		repo := newMockConnectionRepo()
		client := &mockJiraClientSvc{}
		svc := integrations.NewJiraConnectionService(repo, client, key)

		_, err := svc.ListJiraFields(context.Background(), "nonexistent-id")
		require.Error(t, err)
	})

	t.Run("propagates client error", func(t *testing.T) {
		repo := newMockConnectionRepo()
		clientErr := errors.New("JIRA unavailable")
		client := &mockJiraClientSvc{err: clientErr}
		svc := integrations.NewJiraConnectionService(repo, client, key)

		stored := buildStoredConnection(t, key)
		repo.stored[stored.ID()] = stored

		_, err := svc.ListJiraFields(context.Background(), stored.ID())
		require.Error(t, err)
		assert.ErrorIs(t, err, clientErr)
	})
}

func TestJiraConnectionService_TestConnection_EncryptedCredentialPreservedAfterTest(t *testing.T) {
	key := make([]byte, 32)
	repo := newMockConnectionRepo()
	client := &mockJiraClientSvc{}
	svc := integrations.NewJiraConnectionService(repo, client, key)

	stored := buildStoredConnection(t, key)
	originalEncrypted := stored.GetEncryptedCredentialDirect()
	repo.stored[stored.ID()] = stored

	require.NoError(t, svc.TestConnection(context.Background(), stored.ID()))

	saved := repo.stored[stored.ID()]
	assert.Equal(t, originalEncrypted, saved.GetEncryptedCredentialDirect(),
		"encrypted credential must not be replaced with plaintext after the test")
}
