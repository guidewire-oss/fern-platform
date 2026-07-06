package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
	projectsApp "github.com/guidewire-oss/fern-platform/internal/domains/projects/application"
	projectsDomain "github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Fake repos for REST handler tests
// ---------------------------------------------------------------------------

type jiraHandlerTestConnRepo struct{}

func (r *jiraHandlerTestConnRepo) Create(ctx context.Context, c *integrations.JiraConnection) error {
	return nil
}
func (r *jiraHandlerTestConnRepo) Update(ctx context.Context, c *integrations.JiraConnection) error {
	return nil
}
func (r *jiraHandlerTestConnRepo) Delete(ctx context.Context, id string) error { return nil }
func (r *jiraHandlerTestConnRepo) FindByID(ctx context.Context, id string) (*integrations.JiraConnection, error) {
	return nil, nil
}
func (r *jiraHandlerTestConnRepo) FindByProjectID(ctx context.Context, projectID string) ([]*integrations.JiraConnection, error) {
	return nil, nil
}
func (r *jiraHandlerTestConnRepo) FindActiveByProjectID(ctx context.Context, projectID string) ([]*integrations.JiraConnection, error) {
	return nil, nil
}

type jiraHandlerTestJiraClient struct{}

func (c *jiraHandlerTestJiraClient) TestConnection(ctx context.Context, baseURL, username, credential string, authType integrations.AuthenticationType) error {
	return nil
}
func (c *jiraHandlerTestJiraClient) GetProject(ctx context.Context, baseURL, projectKey, username, credential string, authType integrations.AuthenticationType) (*integrations.JiraProject, error) {
	return nil, nil
}
func (c *jiraHandlerTestJiraClient) ListFields(ctx context.Context, baseURL, username, credential string, authType integrations.AuthenticationType) ([]integrations.JiraField, error) {
	return nil, nil
}

type jiraHandlerTestProjectRepo struct{}

func (r *jiraHandlerTestProjectRepo) Save(ctx context.Context, p *projectsDomain.Project) error {
	return nil
}
func (r *jiraHandlerTestProjectRepo) FindByID(ctx context.Context, id uint) (*projectsDomain.Project, error) {
	return nil, projectsDomain.ErrProjectNotFound
}
func (r *jiraHandlerTestProjectRepo) FindByProjectID(ctx context.Context, id projectsDomain.ProjectID) (*projectsDomain.Project, error) {
	if id == "" {
		return nil, projectsDomain.ErrProjectNotFound
	}
	p, err := projectsDomain.NewProject(id, "Test "+string(id), projectsDomain.Team("test-team"))
	if err != nil {
		return nil, err
	}
	return p, nil
}
func (r *jiraHandlerTestProjectRepo) FindByTeam(ctx context.Context, team projectsDomain.Team) ([]*projectsDomain.Project, error) {
	return nil, nil
}
func (r *jiraHandlerTestProjectRepo) FindAll(ctx context.Context, limit, offset int) ([]*projectsDomain.Project, int64, error) {
	return nil, 0, nil
}
func (r *jiraHandlerTestProjectRepo) Update(ctx context.Context, p *projectsDomain.Project) error {
	return nil
}
func (r *jiraHandlerTestProjectRepo) Delete(ctx context.Context, id uint) error { return nil }
func (r *jiraHandlerTestProjectRepo) ExistsByProjectID(ctx context.Context, id projectsDomain.ProjectID) (bool, error) {
	return id != "", nil
}

type jiraHandlerTestPermRepo struct {
	permissions map[string]projectsDomain.PermissionType
}

func newJiraHandlerPermRepo(writeUsers ...string) *jiraHandlerTestPermRepo {
	m := make(map[string]projectsDomain.PermissionType, len(writeUsers))
	for _, u := range writeUsers {
		m[u] = projectsDomain.PermissionWrite
	}
	return &jiraHandlerTestPermRepo{permissions: m}
}

func (r *jiraHandlerTestPermRepo) Save(ctx context.Context, p *projectsDomain.ProjectPermission) error {
	return nil
}
func (r *jiraHandlerTestPermRepo) FindByProjectAndUser(ctx context.Context, projectID projectsDomain.ProjectID, userID string) ([]*projectsDomain.ProjectPermission, error) {
	pt, ok := r.permissions[userID]
	if !ok {
		return nil, nil
	}
	perm, err := projectsDomain.NewProjectPermission(projectID, userID, pt, "test")
	if err != nil {
		return nil, err
	}
	return []*projectsDomain.ProjectPermission{perm}, nil
}
func (r *jiraHandlerTestPermRepo) FindByUser(ctx context.Context, userID string) ([]*projectsDomain.ProjectPermission, error) {
	return nil, nil
}
func (r *jiraHandlerTestPermRepo) FindByProject(ctx context.Context, projectID projectsDomain.ProjectID) ([]*projectsDomain.ProjectPermission, error) {
	return nil, nil
}
func (r *jiraHandlerTestPermRepo) Delete(ctx context.Context, projectID projectsDomain.ProjectID, userID string, p projectsDomain.PermissionType) error {
	return nil
}
func (r *jiraHandlerTestPermRepo) DeleteExpired(ctx context.Context) error { return nil }

// ---------------------------------------------------------------------------
// Test helper
// ---------------------------------------------------------------------------

func buildJiraConnectionHandler(t *testing.T, permRepo projectsDomain.ProjectPermissionRepository) (*JiraConnectionHandler, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	logger, err := logging.NewLogger(&config.LoggingConfig{Level: "error", Format: "json"})
	require.NoError(t, err)

	key := []byte("0123456789abcdef")
	connSvc := integrations.NewJiraConnectionService(&jiraHandlerTestConnRepo{}, &jiraHandlerTestJiraClient{}, key)
	projSvc := projectsApp.NewProjectService(&jiraHandlerTestProjectRepo{}, permRepo)

	base := NewBaseHandler(logger)
	handler := NewJiraConnectionHandler(base, connSvc, projSvc)
	router := gin.New()
	return handler, router
}

// productionContextMiddleware sets the same gin context keys as the real auth middleware.
// Critically, it uses "user_role" — NOT "role".
func productionContextMiddleware(userID, userRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("user_role", userRole)
		c.Next()
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestJiraConnectionHandler_AdminBypassesPermissionCheck(t *testing.T) {
	// admin-1 has no permission row; the admin bypass must still allow access.
	permRepo := newJiraHandlerPermRepo() // empty — no rows for anyone
	handler, router := buildJiraConnectionHandler(t, permRepo)
	router.Use(productionContextMiddleware("admin-1", "admin"))
	router.GET("/projects/:projectId/jira-connections", handler.GetConnections)

	req := httptest.NewRequest("GET", "/projects/proj-1/jira-connections", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "admin with no permission row must be allowed")
}

func TestJiraConnectionHandler_NonAdminWithoutPermissionsIsForbidden(t *testing.T) {
	permRepo := newJiraHandlerPermRepo() // no rows
	handler, router := buildJiraConnectionHandler(t, permRepo)
	router.Use(productionContextMiddleware("user-1", "user"))
	router.GET("/projects/:projectId/jira-connections", handler.GetConnections)

	req := httptest.NewRequest("GET", "/projects/proj-1/jira-connections", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code, "user with no permission row must be forbidden")
}

func TestJiraConnectionHandler_NonAdminWithWritePermissionCanView(t *testing.T) {
	permRepo := newJiraHandlerPermRepo("user-1")
	handler, router := buildJiraConnectionHandler(t, permRepo)
	router.Use(productionContextMiddleware("user-1", "user"))
	router.GET("/projects/:projectId/jira-connections", handler.GetConnections)

	req := httptest.NewRequest("GET", "/projects/proj-1/jira-connections", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "user with write permission must be able to view connections")
}

func TestJiraConnectionHandler_WrongContextKeyDoesNotBypassAdmin(t *testing.T) {
	// Regression guard: setting "role" (wrong key) instead of "user_role" must NOT
	// grant the admin bypass. Before the P0 fix, getUserRole read "role", so setting
	// only "user_role" would also have been wrong — now "user_role" is the required key.
	permRepo := newJiraHandlerPermRepo() // no rows
	handler, router := buildJiraConnectionHandler(t, permRepo)
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "admin-1")
		c.Set("role", "admin") // old/wrong key — must not trigger bypass
		c.Next()
	})
	router.GET("/projects/:projectId/jira-connections", handler.GetConnections)

	req := httptest.NewRequest("GET", "/projects/proj-1/jira-connections", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code,
		"setting the wrong context key must not grant admin bypass")
}

func TestJiraConnectionHandler_UnauthenticatedUserIsRejected(t *testing.T) {
	permRepo := newJiraHandlerPermRepo()
	handler, router := buildJiraConnectionHandler(t, permRepo)
	// No middleware — user_id is not set
	router.GET("/projects/:projectId/jira-connections", handler.GetConnections)

	req := httptest.NewRequest("GET", "/projects/proj-1/jira-connections", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code, "request with no user_id must be rejected")
}
