package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
	projectsApp "github.com/guidewire-oss/fern-platform/internal/domains/projects/application"
	projectsDomain "github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

// --- Mock JiraConnectionRepository ---

type mockJiraConnectionRepo struct {
	mu          sync.Mutex
	connections map[string]*integrations.JiraConnection
	byProject   map[string][]*integrations.JiraConnection
	failCreate  bool
	failFind    bool
	failUpdate  bool
	failDelete  bool
}

func newMockJiraConnectionRepo() *mockJiraConnectionRepo {
	return &mockJiraConnectionRepo{
		connections: make(map[string]*integrations.JiraConnection),
		byProject:   make(map[string][]*integrations.JiraConnection),
	}
}

func (r *mockJiraConnectionRepo) Create(_ context.Context, conn *integrations.JiraConnection) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failCreate {
		return fmt.Errorf("create failed")
	}
	r.connections[conn.ID()] = conn
	r.byProject[conn.ProjectID()] = append(r.byProject[conn.ProjectID()], conn)
	return nil
}

func (r *mockJiraConnectionRepo) Update(_ context.Context, conn *integrations.JiraConnection) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failUpdate {
		return fmt.Errorf("update failed")
	}
	r.connections[conn.ID()] = conn
	return nil
}

func (r *mockJiraConnectionRepo) Delete(_ context.Context, connectionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failDelete {
		return fmt.Errorf("delete failed")
	}
	conn, ok := r.connections[connectionID]
	if !ok {
		return fmt.Errorf("connection not found")
	}
	delete(r.connections, connectionID)
	// Remove from byProject
	projConns := r.byProject[conn.ProjectID()]
	for i, c := range projConns {
		if c.ID() == connectionID {
			r.byProject[conn.ProjectID()] = append(projConns[:i], projConns[i+1:]...)
			break
		}
	}
	return nil
}

func (r *mockJiraConnectionRepo) FindByID(_ context.Context, connectionID string) (*integrations.JiraConnection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failFind {
		return nil, fmt.Errorf("find failed")
	}
	conn, ok := r.connections[connectionID]
	if !ok {
		return nil, fmt.Errorf("connection not found")
	}
	return conn, nil
}

func (r *mockJiraConnectionRepo) FindByProjectID(_ context.Context, projectID string) ([]*integrations.JiraConnection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failFind {
		return nil, fmt.Errorf("find failed")
	}
	conns := r.byProject[projectID]
	if conns == nil {
		return []*integrations.JiraConnection{}, nil
	}
	return conns, nil
}

func (r *mockJiraConnectionRepo) FindActiveByProjectID(_ context.Context, projectID string) ([]*integrations.JiraConnection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failFind {
		return nil, fmt.Errorf("find failed")
	}
	var active []*integrations.JiraConnection
	for _, c := range r.byProject[projectID] {
		if c.IsActive() {
			active = append(active, c)
		}
	}
	return active, nil
}

func (r *mockJiraConnectionRepo) addConnection(conn *integrations.JiraConnection) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.connections[conn.ID()] = conn
	r.byProject[conn.ProjectID()] = append(r.byProject[conn.ProjectID()], conn)
}

// --- Mock JiraClient ---

type mockJiraClient struct {
	failTest bool
}

func (c *mockJiraClient) TestConnection(_ context.Context, _, _, _ string, _ integrations.AuthenticationType) error {
	if c.failTest {
		return fmt.Errorf("connection test failed")
	}
	return nil
}

func (c *mockJiraClient) GetProject(_ context.Context, _, _, _, _ string, _ integrations.AuthenticationType) (*integrations.JiraProject, error) {
	return &integrations.JiraProject{ID: "1", Key: "TEST", Name: "Test Project"}, nil
}

// --- Mock ProjectRepository ---

type mockProjectRepo struct{}

func (r *mockProjectRepo) Save(_ context.Context, _ *projectsDomain.Project) error {
	return nil
}
func (r *mockProjectRepo) FindByID(_ context.Context, _ uint) (*projectsDomain.Project, error) {
	return nil, fmt.Errorf("not found")
}
func (r *mockProjectRepo) FindByProjectID(_ context.Context, _ projectsDomain.ProjectID) (*projectsDomain.Project, error) {
	return nil, fmt.Errorf("not found")
}
func (r *mockProjectRepo) FindByTeam(_ context.Context, _ projectsDomain.Team) ([]*projectsDomain.Project, error) {
	return nil, nil
}
func (r *mockProjectRepo) FindAll(_ context.Context, _, _ int) ([]*projectsDomain.Project, int64, error) {
	return nil, 0, nil
}
func (r *mockProjectRepo) Update(_ context.Context, _ *projectsDomain.Project) error {
	return nil
}
func (r *mockProjectRepo) Delete(_ context.Context, _ uint) error {
	return nil
}
func (r *mockProjectRepo) ExistsByProjectID(_ context.Context, _ projectsDomain.ProjectID) (bool, error) {
	return false, nil
}

// --- Mock ProjectPermissionRepository ---

type mockPermissionRepo struct {
	permissions map[string][]*projectsDomain.ProjectPermission
	failFind    bool
}

func newMockPermissionRepo() *mockPermissionRepo {
	return &mockPermissionRepo{
		permissions: make(map[string][]*projectsDomain.ProjectPermission),
	}
}

func (r *mockPermissionRepo) Save(_ context.Context, perm *projectsDomain.ProjectPermission) error {
	key := string(perm.ProjectID()) + ":" + perm.UserID()
	r.permissions[key] = append(r.permissions[key], perm)
	return nil
}

func (r *mockPermissionRepo) FindByProjectAndUser(_ context.Context, projectID projectsDomain.ProjectID, userID string) ([]*projectsDomain.ProjectPermission, error) {
	if r.failFind {
		return nil, fmt.Errorf("permission lookup failed")
	}
	key := string(projectID) + ":" + userID
	perms := r.permissions[key]
	if perms == nil {
		return []*projectsDomain.ProjectPermission{}, nil
	}
	return perms, nil
}

func (r *mockPermissionRepo) FindByUser(_ context.Context, _ string) ([]*projectsDomain.ProjectPermission, error) {
	return nil, nil
}

func (r *mockPermissionRepo) FindByProject(_ context.Context, _ projectsDomain.ProjectID) ([]*projectsDomain.ProjectPermission, error) {
	return nil, nil
}

func (r *mockPermissionRepo) Delete(_ context.Context, _ projectsDomain.ProjectID, _ string, _ projectsDomain.PermissionType) error {
	return nil
}

func (r *mockPermissionRepo) DeleteExpired(_ context.Context) error {
	return nil
}

func (r *mockPermissionRepo) grantPermission(projectID projectsDomain.ProjectID, userID string, permType projectsDomain.PermissionType) {
	perm, _ := projectsDomain.NewProjectPermission(projectID, userID, permType, "system")
	key := string(projectID) + ":" + userID
	r.permissions[key] = append(r.permissions[key], perm)
}

// --- Helper to create a test connection ---

func createTestConnection(id, projectID string) *integrations.JiraConnection {
	now := time.Now()
	return integrations.ReconstructJiraConnection(
		id, projectID, "Test Connection", "https://jira.example.com",
		integrations.AuthTypeAPIToken, "TEST", "user@example.com", "encrypted-cred",
		integrations.ConnectionStatusPending, false, nil, now, now,
	)
}

// --- Tests ---

var _ = Describe("JiraConnectionHandler", func() {
	var (
		handler        *JiraConnectionHandler
		jiraRepo       *mockJiraConnectionRepo
		jiraClient     *mockJiraClient
		permRepo       *mockPermissionRepo
		jiraService    *integrations.JiraConnectionService
		projectService *projectsApp.ProjectService
		router         *gin.Engine
	)

	// 32-byte encryption key for AES-256
	encryptionKey := []byte("01234567890123456789012345678901")

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		logger, err := logging.NewLogger(&config.LoggingConfig{Level: "info", Format: "json"})
		Expect(err).NotTo(HaveOccurred())

		jiraRepo = newMockJiraConnectionRepo()
		jiraClient = &mockJiraClient{}
		permRepo = newMockPermissionRepo()

		jiraService = integrations.NewJiraConnectionService(jiraRepo, jiraClient, encryptionKey)
		projectService = projectsApp.NewProjectService(&mockProjectRepo{}, permRepo)

		baseHandler := NewBaseHandler(logger)
		handler = NewJiraConnectionHandler(baseHandler, jiraService, projectService)

		router = gin.New()
		projectGroup := router.Group("/api/v1/projects/:projectId/jira")
		projectGroup.Use(func(c *gin.Context) {
			c.Set("user_id", "user-123")
			c.Next()
		})
		projectGroup.POST("/connections", handler.CreateConnection)
		projectGroup.GET("/connections", handler.GetConnections)

		connGroup := router.Group("/api/v1/jira/connections/:connectionId")
		connGroup.Use(func(c *gin.Context) {
			c.Set("user_id", "user-123")
			c.Next()
		})
		connGroup.GET("", handler.GetConnection)
		connGroup.PUT("", handler.UpdateConnection)
		connGroup.PUT("/credentials", handler.UpdateCredentials)
		connGroup.POST("/test", handler.TestConnection)
		connGroup.DELETE("", handler.DeleteConnection)
	})

	Describe("CreateConnection", func() {
		It("returns 201 when connection is created successfully", func() {
			permRepo.grantPermission("proj-1", "user-123", projectsDomain.PermissionWrite)

			payload := CreateJiraConnectionRequest{
				Name:               "My JIRA",
				JiraURL:            "https://jira.example.com",
				AuthenticationType: "api_token",
				ProjectKey:         "TEST",
				Username:           "user@example.com",
				Credential:         "my-api-token",
			}
			b, _ := json.Marshal(payload)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/proj-1/jira/connections", bytes.NewBuffer(b))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))
			var body JiraConnectionResponse
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body.Name).To(Equal("My JIRA"))
			Expect(body.ProjectKey).To(Equal("TEST"))
			Expect(body.Status).To(Equal("pending"))
		})

		It("returns 400 when request body is invalid", func() {
			permRepo.grantPermission("proj-1", "user-123", projectsDomain.PermissionWrite)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/proj-1/jira/connections", bytes.NewBufferString(`{"bad":`))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("returns 400 when required fields are missing", func() {
			permRepo.grantPermission("proj-1", "user-123", projectsDomain.PermissionWrite)

			payload := map[string]string{"name": "test"}
			b, _ := json.Marshal(payload)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/proj-1/jira/connections", bytes.NewBuffer(b))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("returns 403 when user has no write permission", func() {
			permRepo.grantPermission("proj-1", "user-123", projectsDomain.PermissionRead)

			payload := CreateJiraConnectionRequest{
				Name:               "My JIRA",
				JiraURL:            "https://jira.example.com",
				AuthenticationType: "api_token",
				ProjectKey:         "TEST",
				Username:           "user@example.com",
				Credential:         "my-api-token",
			}
			b, _ := json.Marshal(payload)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/proj-1/jira/connections", bytes.NewBuffer(b))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusForbidden))
		})

		It("returns 401 when user_id is empty", func() {
			noAuthRouter := gin.New()
			group := noAuthRouter.Group("/api/v1/projects/:projectId/jira")
			group.Use(func(c *gin.Context) {
				c.Set("user_id", "")
				c.Next()
			})
			group.POST("/connections", handler.CreateConnection)

			payload := CreateJiraConnectionRequest{
				Name:               "My JIRA",
				JiraURL:            "https://jira.example.com",
				AuthenticationType: "api_token",
				ProjectKey:         "TEST",
				Username:           "user@example.com",
				Credential:         "my-api-token",
			}
			b, _ := json.Marshal(payload)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/proj-1/jira/connections", bytes.NewBuffer(b))
			req.Header.Set("Content-Type", "application/json")
			noAuthRouter.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusUnauthorized))
		})

		It("returns 500 when service fails to create connection", func() {
			permRepo.grantPermission("proj-1", "user-123", projectsDomain.PermissionWrite)

			// Add an existing connection so the service returns "already has a connection"
			existing := createTestConnection("existing-1", "proj-1")
			jiraRepo.addConnection(existing)

			payload := CreateJiraConnectionRequest{
				Name:               "My JIRA",
				JiraURL:            "https://jira.example.com",
				AuthenticationType: "api_token",
				ProjectKey:         "TEST",
				Username:           "user@example.com",
				Credential:         "my-api-token",
			}
			b, _ := json.Marshal(payload)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/proj-1/jira/connections", bytes.NewBuffer(b))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Describe("GetConnections", func() {
		It("returns 200 with connections list", func() {
			conn := createTestConnection("conn-1", "proj-1")
			jiraRepo.addConnection(conn)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/proj-1/jira/connections", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			var body []JiraConnectionResponse
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body).To(HaveLen(1))
			Expect(body[0].Name).To(Equal("Test Connection"))
		})

		It("returns 200 with empty list when no connections exist", func() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/proj-empty/jira/connections", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			var body []JiraConnectionResponse
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body).To(BeEmpty())
		})

		It("returns 500 when service fails", func() {
			jiraRepo.failFind = true

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/proj-1/jira/connections", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Describe("GetConnection", func() {
		It("returns 200 with connection details", func() {
			conn := createTestConnection("conn-1", "proj-1")
			jiraRepo.addConnection(conn)
			permRepo.grantPermission("proj-1", "user-123", projectsDomain.PermissionRead)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/jira/connections/conn-1", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			var body JiraConnectionResponse
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body.ID).To(Equal("conn-1"))
			Expect(body.Name).To(Equal("Test Connection"))
		})

		It("returns 404 when connection not found", func() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/jira/connections/nonexistent", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})

		It("returns 403 when user has no read permission", func() {
			conn := createTestConnection("conn-1", "proj-1")
			jiraRepo.addConnection(conn)
			// No permissions granted

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/jira/connections/conn-1", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusForbidden))
		})
	})

	Describe("UpdateConnection", func() {
		It("returns 200 when connection is updated", func() {
			conn := createTestConnection("conn-1", "proj-1")
			jiraRepo.addConnection(conn)
			permRepo.grantPermission("proj-1", "user-123", projectsDomain.PermissionWrite)

			payload := UpdateJiraConnectionRequest{
				Name:       "Updated Name",
				JiraURL:    "https://jira-new.example.com",
				ProjectKey: "NEWKEY",
			}
			b, _ := json.Marshal(payload)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPut, "/api/v1/jira/connections/conn-1", bytes.NewBuffer(b))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			var body JiraConnectionResponse
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body.Name).To(Equal("Updated Name"))
		})

		It("returns 404 when connection not found", func() {
			payload := UpdateJiraConnectionRequest{
				Name:       "Updated Name",
				JiraURL:    "https://jira.example.com",
				ProjectKey: "TEST",
			}
			b, _ := json.Marshal(payload)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPut, "/api/v1/jira/connections/nonexistent", bytes.NewBuffer(b))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})

		It("returns 400 for invalid JSON body", func() {
			conn := createTestConnection("conn-1", "proj-1")
			jiraRepo.addConnection(conn)
			permRepo.grantPermission("proj-1", "user-123", projectsDomain.PermissionWrite)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPut, "/api/v1/jira/connections/conn-1", bytes.NewBufferString("{bad"))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("returns 403 when user has no write permission", func() {
			conn := createTestConnection("conn-1", "proj-1")
			jiraRepo.addConnection(conn)
			permRepo.grantPermission("proj-1", "user-123", projectsDomain.PermissionRead)

			payload := UpdateJiraConnectionRequest{
				Name:       "Updated",
				JiraURL:    "https://jira.example.com",
				ProjectKey: "TEST",
			}
			b, _ := json.Marshal(payload)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPut, "/api/v1/jira/connections/conn-1", bytes.NewBuffer(b))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusForbidden))
		})
	})

	Describe("UpdateCredentials", func() {
		It("returns 200 when credentials are updated", func() {
			conn := createTestConnection("conn-1", "proj-1")
			jiraRepo.addConnection(conn)
			permRepo.grantPermission("proj-1", "user-123", projectsDomain.PermissionWrite)

			payload := UpdateJiraCredentialsRequest{
				AuthenticationType: "api_token",
				Username:           "newuser@example.com",
				Credential:         "new-token",
			}
			b, _ := json.Marshal(payload)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPut, "/api/v1/jira/connections/conn-1/credentials", bytes.NewBuffer(b))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("returns 404 when connection not found", func() {
			payload := UpdateJiraCredentialsRequest{
				AuthenticationType: "api_token",
				Username:           "user@example.com",
				Credential:         "token",
			}
			b, _ := json.Marshal(payload)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPut, "/api/v1/jira/connections/nonexistent/credentials", bytes.NewBuffer(b))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})

		It("returns 400 for invalid request body", func() {
			conn := createTestConnection("conn-1", "proj-1")
			jiraRepo.addConnection(conn)
			permRepo.grantPermission("proj-1", "user-123", projectsDomain.PermissionWrite)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPut, "/api/v1/jira/connections/conn-1/credentials", bytes.NewBufferString("{bad"))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("TestConnection", func() {
		It("returns 404 when connection not found", func() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/jira/connections/nonexistent/test", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})

		It("returns 403 when user has no write permission", func() {
			conn := createTestConnection("conn-1", "proj-1")
			jiraRepo.addConnection(conn)
			permRepo.grantPermission("proj-1", "user-123", projectsDomain.PermissionRead)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/jira/connections/conn-1/test", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusForbidden))
		})
	})

	Describe("DeleteConnection", func() {
		It("returns 204 when connection is deleted", func() {
			conn := createTestConnection("conn-1", "proj-1")
			jiraRepo.addConnection(conn)
			permRepo.grantPermission("proj-1", "user-123", projectsDomain.PermissionWrite)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodDelete, "/api/v1/jira/connections/conn-1", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNoContent))
		})

		It("returns 404 when connection not found", func() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodDelete, "/api/v1/jira/connections/nonexistent", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})

		It("returns 403 when user has no write permission", func() {
			conn := createTestConnection("conn-1", "proj-1")
			jiraRepo.addConnection(conn)
			permRepo.grantPermission("proj-1", "user-123", projectsDomain.PermissionRead)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodDelete, "/api/v1/jira/connections/conn-1", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusForbidden))
		})

		It("returns 500 when delete service fails", func() {
			conn := createTestConnection("conn-1", "proj-1")
			jiraRepo.addConnection(conn)
			permRepo.grantPermission("proj-1", "user-123", projectsDomain.PermissionWrite)
			jiraRepo.failDelete = true

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodDelete, "/api/v1/jira/connections/conn-1", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Describe("convertToResponse", func() {
		It("converts a JiraConnection to response format", func() {
			now := time.Now()
			testedAt := now.Add(-time.Hour)
			conn := integrations.ReconstructJiraConnection(
				"conn-1", "proj-1", "My Connection", "https://jira.example.com",
				integrations.AuthTypeAPIToken, "TEST", "user@example.com", "cred",
				integrations.ConnectionStatusConnected, true, &testedAt, now, now,
			)

			resp := handler.convertToResponse(conn)
			Expect(resp.ID).To(Equal("conn-1"))
			Expect(resp.ProjectID).To(Equal("proj-1"))
			Expect(resp.Name).To(Equal("My Connection"))
			Expect(resp.JiraURL).To(Equal("https://jira.example.com"))
			Expect(resp.AuthenticationType).To(Equal("api_token"))
			Expect(resp.ProjectKey).To(Equal("TEST"))
			Expect(resp.Username).To(Equal("user@example.com"))
			Expect(resp.Status).To(Equal("connected"))
			Expect(resp.IsActive).To(BeTrue())
			Expect(resp.LastTestedAt).NotTo(BeNil())
		})

		It("handles nil LastTestedAt", func() {
			now := time.Now()
			conn := integrations.ReconstructJiraConnection(
				"conn-1", "proj-1", "My Connection", "https://jira.example.com",
				integrations.AuthTypeAPIToken, "TEST", "user@example.com", "cred",
				integrations.ConnectionStatusPending, false, nil, now, now,
			)

			resp := handler.convertToResponse(conn)
			Expect(resp.LastTestedAt).To(BeNil())
		})
	})
})
