package graphql

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authDomain "github.com/guidewire-oss/fern-platform/internal/domains/auth/domain"
	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
	projectsApp "github.com/guidewire-oss/fern-platform/internal/domains/projects/application"
	projectsDomain "github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
	"github.com/guidewire-oss/fern-platform/internal/reporter/graphql/model"
	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Fakes for JiraFieldMappingService and JiraConnectionService dependencies
// ---------------------------------------------------------------------------

// fakeMappingRepo implements integrations.JiraFieldMappingRepository.
type fakeMappingRepo struct {
	mapping  *integrations.JiraFieldMapping
	getErr   error
	upsertErr error
}

func (f *fakeMappingRepo) Get(ctx context.Context, projectID string) (*integrations.JiraFieldMapping, error) {
	return f.mapping, f.getErr
}

func (f *fakeMappingRepo) Upsert(ctx context.Context, mapping *integrations.JiraFieldMapping) error {
	return f.upsertErr
}

func (f *fakeMappingRepo) Delete(ctx context.Context, projectID string) error {
	return nil
}

// fakeConnRepo implements integrations.JiraConnectionRepository.
type fakeConnRepo struct {
	connections []*integrations.JiraConnection
	err         error
}

func (f *fakeConnRepo) Create(ctx context.Context, c *integrations.JiraConnection) error {
	return f.err
}

func (f *fakeConnRepo) Update(ctx context.Context, c *integrations.JiraConnection) error {
	return f.err
}

func (f *fakeConnRepo) Delete(ctx context.Context, id string) error {
	return f.err
}

func (f *fakeConnRepo) FindByID(ctx context.Context, id string) (*integrations.JiraConnection, error) {
	if f.err != nil {
		return nil, f.err
	}
	for _, c := range f.connections {
		if c.ID() == id {
			return c, nil
		}
	}
	return nil, nil
}

func (f *fakeConnRepo) FindByProjectID(ctx context.Context, projectID string) ([]*integrations.JiraConnection, error) {
	return f.connections, f.err
}

func (f *fakeConnRepo) FindActiveByProjectID(ctx context.Context, projectID string) ([]*integrations.JiraConnection, error) {
	if f.err != nil {
		return nil, f.err
	}
	var active []*integrations.JiraConnection
	for _, c := range f.connections {
		if c.IsActive() {
			active = append(active, c)
		}
	}
	return active, nil
}

// fakeJiraClient implements integrations.JiraClient.
type fakeJiraClient struct {
	fields []integrations.JiraField
	err    error
}

func (f *fakeJiraClient) TestConnection(ctx context.Context, baseURL, username, credential string, authType integrations.AuthenticationType) error {
	return f.err
}

func (f *fakeJiraClient) GetProject(ctx context.Context, baseURL, projectKey, username, credential string, authType integrations.AuthenticationType) (*integrations.JiraProject, error) {
	return nil, f.err
}

func (f *fakeJiraClient) ListFields(ctx context.Context, baseURL, username, credential string, authType integrations.AuthenticationType) ([]integrations.JiraField, error) {
	return f.fields, f.err
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func newTestResolverWithJiraMapping(t *testing.T, mappingSvc *integrations.JiraFieldMappingService, connSvc *integrations.JiraConnectionService) *Resolver {
	t.Helper()
	logger, err := logging.NewLogger(&config.LoggingConfig{
		Level:      "error",
		Format:     "json",
		Output:     "stdout",
		Structured: true,
	})
	require.NoError(t, err)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Default permissive project service: any projectID resolves to a project,
	// and the standard test users "admin-1" and "manager-1" have project-scoped
	// permissions on every project. Regular users (e.g. "user-1") have none and
	// are rejected by the new project-scoped auth helper.
	projSvc := projectsApp.NewProjectService(
		newAnyProjectRepo(),
		newSeededPermissionRepo("admin-1", "manager-1"),
	)

	r := NewResolver(nil, projSvc, nil, nil, connSvc, nil, db, logger)
	r.jiraFieldMappingService = mappingSvc
	return r
}

// anyProjectRepo answers FindByProjectID for any non-empty ProjectID with a
// synthetic project. Used by tests that don't care which project exists.
type anyProjectRepo struct{}

func newAnyProjectRepo() *anyProjectRepo { return &anyProjectRepo{} }

func (a *anyProjectRepo) Save(ctx context.Context, p *projectsDomain.Project) error { return nil }
func (a *anyProjectRepo) FindByID(ctx context.Context, id uint) (*projectsDomain.Project, error) {
	return nil, projectsDomain.ErrProjectNotFound
}
func (a *anyProjectRepo) FindByProjectID(ctx context.Context, id projectsDomain.ProjectID) (*projectsDomain.Project, error) {
	if id == "" {
		return nil, projectsDomain.ErrProjectNotFound
	}
	p, err := projectsDomain.NewProject(id, "Synthetic "+string(id), projectsDomain.Team("test-team"))
	if err != nil {
		return nil, err
	}
	return p, nil
}
func (a *anyProjectRepo) FindByTeam(ctx context.Context, team projectsDomain.Team) ([]*projectsDomain.Project, error) {
	return nil, nil
}
func (a *anyProjectRepo) FindAll(ctx context.Context, limit, offset int) ([]*projectsDomain.Project, int64, error) {
	return nil, 0, nil
}
func (a *anyProjectRepo) Update(ctx context.Context, p *projectsDomain.Project) error { return nil }
func (a *anyProjectRepo) Delete(ctx context.Context, id uint) error                   { return nil }
func (a *anyProjectRepo) ExistsByProjectID(ctx context.Context, id projectsDomain.ProjectID) (bool, error) {
	return id != "", nil
}

// seededPermissionRepo grants every listed user a Read permission on any
// project they're queried for. Users not in the list get no permissions.
type seededPermissionRepo struct {
	allowedUsers map[string]struct{}
}

func newSeededPermissionRepo(userIDs ...string) *seededPermissionRepo {
	s := &seededPermissionRepo{allowedUsers: make(map[string]struct{}, len(userIDs))}
	for _, u := range userIDs {
		s.allowedUsers[u] = struct{}{}
	}
	return s
}

func (s *seededPermissionRepo) Save(ctx context.Context, p *projectsDomain.ProjectPermission) error {
	return nil
}
func (s *seededPermissionRepo) FindByProjectAndUser(ctx context.Context, projectID projectsDomain.ProjectID, userID string) ([]*projectsDomain.ProjectPermission, error) {
	if _, ok := s.allowedUsers[userID]; !ok {
		return nil, nil
	}
	perm, err := projectsDomain.NewProjectPermission(projectID, userID, projectsDomain.PermissionRead, "test-seed")
	if err != nil {
		return nil, err
	}
	return []*projectsDomain.ProjectPermission{perm}, nil
}
func (s *seededPermissionRepo) FindByUser(ctx context.Context, userID string) ([]*projectsDomain.ProjectPermission, error) {
	return nil, nil
}
func (s *seededPermissionRepo) FindByProject(ctx context.Context, projectID projectsDomain.ProjectID) ([]*projectsDomain.ProjectPermission, error) {
	return nil, nil
}
func (s *seededPermissionRepo) Delete(ctx context.Context, projectID projectsDomain.ProjectID, userID string, p projectsDomain.PermissionType) error {
	return nil
}
func (s *seededPermissionRepo) DeleteExpired(ctx context.Context) error { return nil }

func adminCtxForMapping() context.Context {
	return context.WithValue(context.Background(), "user", &authDomain.User{
		UserID: "admin-1",
		Role:   authDomain.RoleAdmin,
		Groups: []authDomain.UserGroup{},
	})
}

func managerCtxForMapping() context.Context {
	return context.WithValue(context.Background(), "user", &authDomain.User{
		UserID: "manager-1",
		Role:   authDomain.RoleManager,
		Groups: []authDomain.UserGroup{},
	})
}

func regularUserCtxForMapping() context.Context {
	return context.WithValue(context.Background(), "user", &authDomain.User{
		UserID: "user-1",
		Role:   authDomain.RoleUser,
		Groups: []authDomain.UserGroup{{GroupName: "team-a"}},
	})
}

// testEncryptionKey matches the key used when constructing JiraConnectionService in tests.
var testEncryptionKey = []byte("0123456789abcdef")

func buildActiveConnection(projectID string) *integrations.JiraConnection {
	// Build via the constructor so GetEncryptedCredential encrypts using our test key.
	conn, err := integrations.NewJiraConnection(
		projectID, "My JIRA", "https://jira.example.com",
		integrations.AuthTypeAPIToken, "PROJ", "user@example.com", "test-api-token",
	)
	if err != nil {
		panic("buildActiveConnection: " + err.Error())
	}
	encryptedCred, err := conn.GetEncryptedCredential(testEncryptionKey)
	if err != nil {
		panic("buildActiveConnection: failed to encrypt credential: " + err.Error())
	}
	return integrations.ReconstructJiraConnection(
		"conn-1", projectID, "My JIRA", "https://jira.example.com",
		integrations.AuthTypeAPIToken, "PROJ", "user@example.com", encryptedCred,
		integrations.ConnectionStatusConnected, true,
		nil, time.Now(), time.Now(),
	)
}

func buildDefaultSnapshot(projectID string) *integrations.JiraFieldMappingSnapshot {
	entries := []integrations.FieldMappingEntry{
		{FernField: integrations.FernFieldRequirementID, JiraFieldID: "issuekey"},
		{FernField: integrations.FernFieldRequirementTitle, JiraFieldID: "summary"},
	}
	return &integrations.JiraFieldMappingSnapshot{
		ProjectID: projectID,
		Entries:   entries,
		UpdatedBy: "",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// ---------------------------------------------------------------------------
// TestJiraFieldMappingResolver (query)
// ---------------------------------------------------------------------------

func TestJiraFieldMappingResolver(t *testing.T) {
	t.Run("non-manager user is denied with authorization error", func(t *testing.T) {
		connRepo := &fakeConnRepo{}
		connSvc := integrations.NewJiraConnectionService(connRepo, &fakeJiraClient{}, testEncryptionKey)
		mappingRepo := &fakeMappingRepo{}
		mappingSvc := integrations.NewJiraFieldMappingService(mappingRepo, connRepo)

		resolver := newTestResolverWithJiraMapping(t, mappingSvc, connSvc)
		qr := &queryResolver{resolver}

		result, err := qr.JiraFieldMapping_domain(regularUserCtxForMapping(), "proj-1")

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "forbidden")
	})

	t.Run("manager user gets the mapping from service", func(t *testing.T) {
		connRepo := &fakeConnRepo{}
		connSvc := integrations.NewJiraConnectionService(connRepo, &fakeJiraClient{}, testEncryptionKey)

		snap := buildDefaultSnapshot("proj-1")
		// Build a persisted mapping aggregate
		mapping := integrations.ReconstructJiraFieldMapping("proj-1", snap.Entries, "manager-1", snap.CreatedAt, snap.UpdatedAt)
		mappingRepo := &fakeMappingRepo{mapping: mapping}
		mappingSvc := integrations.NewJiraFieldMappingService(mappingRepo, connRepo)

		resolver := newTestResolverWithJiraMapping(t, mappingSvc, connSvc)
		qr := &queryResolver{resolver}

		result, err := qr.JiraFieldMapping_domain(managerCtxForMapping(), "proj-1")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "proj-1", result.ProjectID)
		assert.NotEmpty(t, result.Entries)
	})

	t.Run("admin user gets the mapping from service", func(t *testing.T) {
		connRepo := &fakeConnRepo{}
		connSvc := integrations.NewJiraConnectionService(connRepo, &fakeJiraClient{}, testEncryptionKey)

		snap := buildDefaultSnapshot("proj-2")
		mapping := integrations.ReconstructJiraFieldMapping("proj-2", snap.Entries, "admin-1", snap.CreatedAt, snap.UpdatedAt)
		mappingRepo := &fakeMappingRepo{mapping: mapping}
		mappingSvc := integrations.NewJiraFieldMappingService(mappingRepo, connRepo)

		resolver := newTestResolverWithJiraMapping(t, mappingSvc, connSvc)
		qr := &queryResolver{resolver}

		result, err := qr.JiraFieldMapping_domain(adminCtxForMapping(), "proj-2")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "proj-2", result.ProjectID)
	})

	t.Run("service error is propagated", func(t *testing.T) {
		connRepo := &fakeConnRepo{}
		connSvc := integrations.NewJiraConnectionService(connRepo, &fakeJiraClient{}, testEncryptionKey)
		mappingRepo := &fakeMappingRepo{getErr: errors.New("database unavailable")}
		mappingSvc := integrations.NewJiraFieldMappingService(mappingRepo, connRepo)

		resolver := newTestResolverWithJiraMapping(t, mappingSvc, connSvc)
		qr := &queryResolver{resolver}

		result, err := qr.JiraFieldMapping_domain(managerCtxForMapping(), "proj-1")

		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("unauthenticated request is denied", func(t *testing.T) {
		connRepo := &fakeConnRepo{}
		connSvc := integrations.NewJiraConnectionService(connRepo, &fakeJiraClient{}, testEncryptionKey)
		mappingRepo := &fakeMappingRepo{}
		mappingSvc := integrations.NewJiraFieldMappingService(mappingRepo, connRepo)

		resolver := newTestResolverWithJiraMapping(t, mappingSvc, connSvc)
		qr := &queryResolver{resolver}

		result, err := qr.JiraFieldMapping_domain(context.Background(), "proj-1")

		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

// ---------------------------------------------------------------------------
// TestJiraFieldsResolver (query)
// ---------------------------------------------------------------------------

func TestJiraFieldsResolver(t *testing.T) {
	t.Run("non-manager user is denied", func(t *testing.T) {
		jiraClient := &fakeJiraClient{}
		// Seed an active connection so the resolver gets past the
		// connection-lookup and reaches the per-project auth check.
		connRepo := &fakeConnRepo{connections: []*integrations.JiraConnection{buildActiveConnection("proj-1")}}
		connSvc := integrations.NewJiraConnectionService(connRepo, jiraClient, testEncryptionKey)
		mappingSvc := integrations.NewJiraFieldMappingService(&fakeMappingRepo{}, connRepo)

		resolver := newTestResolverWithJiraMapping(t, mappingSvc, connSvc)
		qr := &queryResolver{resolver}

		result, err := qr.JiraFields_domain(regularUserCtxForMapping(), "conn-1")

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "forbidden")
	})

	t.Run("manager user gets list of JIRA fields from service", func(t *testing.T) {
		fields := []integrations.JiraField{
			{ID: "summary", Name: "Summary", Custom: false, MultiValue: false},
			{ID: "description", Name: "Description", Custom: false, MultiValue: false},
			{ID: "labels", Name: "Labels", Custom: false, MultiValue: true},
		}
		jiraClient := &fakeJiraClient{fields: fields}

		activeConn := buildActiveConnection("proj-1")
		connRepo := &fakeConnRepo{connections: []*integrations.JiraConnection{activeConn}}
		connSvc := integrations.NewJiraConnectionService(connRepo, jiraClient, testEncryptionKey)
		mappingSvc := integrations.NewJiraFieldMappingService(&fakeMappingRepo{}, connRepo)

		resolver := newTestResolverWithJiraMapping(t, mappingSvc, connSvc)
		qr := &queryResolver{resolver}

		result, err := qr.JiraFields_domain(managerCtxForMapping(), "conn-1")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result, 3)
		assert.Equal(t, "summary", result[0].ID)
		assert.Equal(t, "Summary", result[0].Name)
		assert.False(t, result[0].MultiValue)
		assert.Equal(t, "labels", result[2].ID)
		assert.True(t, result[2].MultiValue)
	})

	t.Run("returns empty slice when no fields available", func(t *testing.T) {
		jiraClient := &fakeJiraClient{fields: []integrations.JiraField{}}

		activeConn := buildActiveConnection("proj-1")
		connRepo := &fakeConnRepo{connections: []*integrations.JiraConnection{activeConn}}
		connSvc := integrations.NewJiraConnectionService(connRepo, jiraClient, testEncryptionKey)
		mappingSvc := integrations.NewJiraFieldMappingService(&fakeMappingRepo{}, connRepo)

		resolver := newTestResolverWithJiraMapping(t, mappingSvc, connSvc)
		qr := &queryResolver{resolver}

		result, err := qr.JiraFields_domain(managerCtxForMapping(), "conn-1")

		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("connection not found returns error", func(t *testing.T) {
		jiraClient := &fakeJiraClient{}
		connRepo := &fakeConnRepo{err: errors.New("not found")}
		connSvc := integrations.NewJiraConnectionService(connRepo, jiraClient, testEncryptionKey)
		mappingSvc := integrations.NewJiraFieldMappingService(&fakeMappingRepo{}, connRepo)

		resolver := newTestResolverWithJiraMapping(t, mappingSvc, connSvc)
		qr := &queryResolver{resolver}

		result, err := qr.JiraFields_domain(managerCtxForMapping(), "conn-missing")

		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

// ---------------------------------------------------------------------------
// TestSaveJiraFieldMappingResolver (mutation)
// ---------------------------------------------------------------------------

func TestSaveJiraFieldMappingResolver(t *testing.T) {
	validInput := model.SaveJiraFieldMappingInput{
		ProjectID: "proj-1",
		Entries: []*model.FieldMappingEntryInput{
			{FernField: model.FernFieldRequirementID, JiraFieldID: "issuekey"},
			{FernField: model.FernFieldRequirementTitle, JiraFieldID: "summary"},
			{FernField: model.FernFieldDescription, JiraFieldID: "description"},
			{FernField: model.FernFieldParentRequirement, JiraFieldID: ""},
			{FernField: model.FernFieldRequirementType, JiraFieldID: "issuetype"},
			{FernField: model.FernFieldReleaseVersion, JiraFieldID: ""},
			{FernField: model.FernFieldRequirementStatus, JiraFieldID: "status"},
			{FernField: model.FernFieldTags, JiraFieldID: "labels"},
		},
	}

	t.Run("non-manager user is denied", func(t *testing.T) {
		connRepo := &fakeConnRepo{}
		connSvc := integrations.NewJiraConnectionService(connRepo, &fakeJiraClient{}, testEncryptionKey)
		mappingSvc := integrations.NewJiraFieldMappingService(&fakeMappingRepo{}, connRepo)

		resolver := newTestResolverWithJiraMapping(t, mappingSvc, connSvc)
		mr := &mutationResolver{resolver}

		result, err := mr.SaveJiraFieldMapping_domain(regularUserCtxForMapping(), validInput)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "forbidden")
	})

	t.Run("valid input saves and returns the mapping", func(t *testing.T) {
		activeConn := buildActiveConnection("proj-1")
		connRepo := &fakeConnRepo{connections: []*integrations.JiraConnection{activeConn}}
		connSvc := integrations.NewJiraConnectionService(connRepo, &fakeJiraClient{}, testEncryptionKey)
		mappingSvc := integrations.NewJiraFieldMappingService(&fakeMappingRepo{}, connRepo)

		resolver := newTestResolverWithJiraMapping(t, mappingSvc, connSvc)
		mr := &mutationResolver{resolver}

		result, err := mr.SaveJiraFieldMapping_domain(managerCtxForMapping(), validInput)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "proj-1", result.ProjectID)
		assert.NotEmpty(t, result.Entries)
	})

	t.Run("ErrNoJiraConnection maps to appropriate GraphQL error", func(t *testing.T) {
		// No active connections → Save should fail with ErrNoJiraConnection
		connRepo := &fakeConnRepo{connections: []*integrations.JiraConnection{}}
		connSvc := integrations.NewJiraConnectionService(connRepo, &fakeJiraClient{}, testEncryptionKey)
		mappingSvc := integrations.NewJiraFieldMappingService(&fakeMappingRepo{}, connRepo)

		resolver := newTestResolverWithJiraMapping(t, mappingSvc, connSvc)
		mr := &mutationResolver{resolver}

		result, err := mr.SaveJiraFieldMapping_domain(managerCtxForMapping(), validInput)

		assert.Error(t, err)
		assert.Nil(t, result)
		// The resolver must surface ErrNoJiraConnection in a way the client can identify
		assert.True(t, errors.Is(err, integrations.ErrNoJiraConnection) || containsErrMsg(err, "no active JIRA connection"),
			"expected ErrNoJiraConnection to be surfaced, got: %v", err)
	})

	t.Run("ErrRequiredFieldUnmapped maps to appropriate GraphQL error", func(t *testing.T) {
		// Active connection present but required field is missing
		activeConn := buildActiveConnection("proj-1")
		connRepo := &fakeConnRepo{connections: []*integrations.JiraConnection{activeConn}}
		connSvc := integrations.NewJiraConnectionService(connRepo, &fakeJiraClient{}, testEncryptionKey)
		mappingSvc := integrations.NewJiraFieldMappingService(&fakeMappingRepo{}, connRepo)

		resolver := newTestResolverWithJiraMapping(t, mappingSvc, connSvc)
		mr := &mutationResolver{resolver}

		// Input with required field unmapped (REQUIREMENT_ID left empty)
		invalidInput := model.SaveJiraFieldMappingInput{
			ProjectID: "proj-1",
			Entries: []*model.FieldMappingEntryInput{
				{FernField: model.FernFieldRequirementID, JiraFieldID: ""},    // required but empty
				{FernField: model.FernFieldRequirementTitle, JiraFieldID: "summary"},
			},
		}

		result, err := mr.SaveJiraFieldMapping_domain(managerCtxForMapping(), invalidInput)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.True(t, errors.Is(err, integrations.ErrRequiredFieldUnmapped) || containsErrMsg(err, "required"),
			"expected ErrRequiredFieldUnmapped to be surfaced, got: %v", err)
	})

	t.Run("unauthenticated request is denied", func(t *testing.T) {
		connRepo := &fakeConnRepo{}
		connSvc := integrations.NewJiraConnectionService(connRepo, &fakeJiraClient{}, testEncryptionKey)
		mappingSvc := integrations.NewJiraFieldMappingService(&fakeMappingRepo{}, connRepo)

		resolver := newTestResolverWithJiraMapping(t, mappingSvc, connSvc)
		mr := &mutationResolver{resolver}

		result, err := mr.SaveJiraFieldMapping_domain(context.Background(), validInput)

		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

// ---------------------------------------------------------------------------
// TestResetJiraFieldMappingResolver (mutation)
// ---------------------------------------------------------------------------

func TestResetJiraFieldMappingResolver(t *testing.T) {
	t.Run("non-manager user is denied", func(t *testing.T) {
		connRepo := &fakeConnRepo{}
		connSvc := integrations.NewJiraConnectionService(connRepo, &fakeJiraClient{}, testEncryptionKey)
		mappingSvc := integrations.NewJiraFieldMappingService(&fakeMappingRepo{}, connRepo)

		resolver := newTestResolverWithJiraMapping(t, mappingSvc, connSvc)
		mr := &mutationResolver{resolver}

		result, err := mr.ResetJiraFieldMapping_domain(regularUserCtxForMapping(), "proj-1")

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "forbidden")
	})

	t.Run("manager resets and gets default mapping back", func(t *testing.T) {
		connRepo := &fakeConnRepo{}
		connSvc := integrations.NewJiraConnectionService(connRepo, &fakeJiraClient{}, testEncryptionKey)
		// Repo has an existing mapping that will be deleted, returning defaults
		existingMapping := integrations.ReconstructJiraFieldMapping(
			"proj-1",
			[]integrations.FieldMappingEntry{
				{FernField: integrations.FernFieldRequirementID, JiraFieldID: "custom-id"},
				{FernField: integrations.FernFieldRequirementTitle, JiraFieldID: "custom-title"},
			},
			"manager-1", time.Now(), time.Now(),
		)
		mappingRepo := &fakeMappingRepo{mapping: existingMapping}
		mappingSvc := integrations.NewJiraFieldMappingService(mappingRepo, connRepo)

		resolver := newTestResolverWithJiraMapping(t, mappingSvc, connSvc)
		mr := &mutationResolver{resolver}

		result, err := mr.ResetJiraFieldMapping_domain(managerCtxForMapping(), "proj-1")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "proj-1", result.ProjectID)
		// After reset the entries should contain the defaults
		assert.NotEmpty(t, result.Entries)
	})

	t.Run("admin resets and returns default mapping", func(t *testing.T) {
		connRepo := &fakeConnRepo{}
		connSvc := integrations.NewJiraConnectionService(connRepo, &fakeJiraClient{}, testEncryptionKey)
		mappingSvc := integrations.NewJiraFieldMappingService(&fakeMappingRepo{}, connRepo)

		resolver := newTestResolverWithJiraMapping(t, mappingSvc, connSvc)
		mr := &mutationResolver{resolver}

		result, err := mr.ResetJiraFieldMapping_domain(adminCtxForMapping(), "proj-2")

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "proj-2", result.ProjectID)
	})

	t.Run("unauthenticated request is denied", func(t *testing.T) {
		connRepo := &fakeConnRepo{}
		connSvc := integrations.NewJiraConnectionService(connRepo, &fakeJiraClient{}, testEncryptionKey)
		mappingSvc := integrations.NewJiraFieldMappingService(&fakeMappingRepo{}, connRepo)

		resolver := newTestResolverWithJiraMapping(t, mappingSvc, connSvc)
		mr := &mutationResolver{resolver}

		result, err := mr.ResetJiraFieldMapping_domain(context.Background(), "proj-1")

		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

// ---------------------------------------------------------------------------
// TestFernFieldModelConverters — round-trip over all domain constants
// ---------------------------------------------------------------------------

func TestFernFieldModelConverters(t *testing.T) {
	t.Run("every FernField survives domain→model→domain round-trip", func(t *testing.T) {
		for _, domainField := range integrations.AllFernFields() {
			modelField, err := fernFieldToModel(domainField)
			require.NoError(t, err, "fernFieldToModel(%q) should not error", domainField)
			roundTripped := modelToFernField(modelField)
			assert.Equal(t, domainField, roundTripped,
				"domain field %q lost in round-trip via model %q", domainField, modelField)
		}
	})

	t.Run("every model.FernField survives model→domain→model round-trip", func(t *testing.T) {
		modelFields := []model.FernField{
			model.FernFieldRequirementID,
			model.FernFieldRequirementTitle,
			model.FernFieldDescription,
			model.FernFieldParentRequirement,
			model.FernFieldRequirementType,
			model.FernFieldReleaseVersion,
			model.FernFieldRequirementStatus,
			model.FernFieldTags,
		}
		for _, mf := range modelFields {
			domainField := modelToFernField(mf)
			roundTripped, err := fernFieldToModel(domainField)
			require.NoError(t, err, "fernFieldToModel(%q) should not error", domainField)
			assert.Equal(t, mf, roundTripped,
				"model field %q lost in round-trip via domain %q", mf, domainField)
		}
	})
}

// ---------------------------------------------------------------------------
// containsErrMsg is a small helper for flexible error message matching
// ---------------------------------------------------------------------------

func containsErrMsg(err error, substr string) bool {
	if err == nil {
		return false
	}
	return len(err.Error()) > 0 && errContains(err.Error(), substr)
}

func errContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	}())
}
