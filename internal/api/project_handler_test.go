package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	projectsApp "github.com/guidewire-oss/fern-platform/internal/domains/projects/application"
	projectsDomain "github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
)

// MockProjectRepository mocks domain.ProjectRepository
type MockProjectRepository struct {
	mock.Mock
}

func (m *MockProjectRepository) Save(ctx context.Context, project *projectsDomain.Project) error {
	return m.Called(ctx, project).Error(0)
}

func (m *MockProjectRepository) FindByID(ctx context.Context, id uint) (*projectsDomain.Project, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*projectsDomain.Project), args.Error(1)
}

func (m *MockProjectRepository) FindByProjectID(ctx context.Context, projectID projectsDomain.ProjectID) (*projectsDomain.Project, error) {
	args := m.Called(ctx, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*projectsDomain.Project), args.Error(1)
}

func (m *MockProjectRepository) FindByTeam(ctx context.Context, team projectsDomain.Team) ([]*projectsDomain.Project, error) {
	args := m.Called(ctx, team)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*projectsDomain.Project), args.Error(1)
}

func (m *MockProjectRepository) FindAll(ctx context.Context, limit, offset int) ([]*projectsDomain.Project, int64, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*projectsDomain.Project), args.Get(1).(int64), args.Error(2)
}

func (m *MockProjectRepository) Update(ctx context.Context, project *projectsDomain.Project) error {
	return m.Called(ctx, project).Error(0)
}

func (m *MockProjectRepository) Delete(ctx context.Context, id uint) error {
	return m.Called(ctx, id).Error(0)
}

func (m *MockProjectRepository) ExistsByProjectID(ctx context.Context, projectID projectsDomain.ProjectID) (bool, error) {
	args := m.Called(ctx, projectID)
	return args.Bool(0), args.Error(1)
}

// MockProjectPermissionRepository mocks domain.ProjectPermissionRepository
type MockProjectPermissionRepository struct {
	mock.Mock
}

func (m *MockProjectPermissionRepository) Save(ctx context.Context, permission *projectsDomain.ProjectPermission) error {
	return m.Called(ctx, permission).Error(0)
}

func (m *MockProjectPermissionRepository) FindByProjectAndUser(ctx context.Context, projectID projectsDomain.ProjectID, userID string) ([]*projectsDomain.ProjectPermission, error) {
	args := m.Called(ctx, projectID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*projectsDomain.ProjectPermission), args.Error(1)
}

func (m *MockProjectPermissionRepository) FindByUser(ctx context.Context, userID string) ([]*projectsDomain.ProjectPermission, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*projectsDomain.ProjectPermission), args.Error(1)
}

func (m *MockProjectPermissionRepository) FindByProject(ctx context.Context, projectID projectsDomain.ProjectID) ([]*projectsDomain.ProjectPermission, error) {
	args := m.Called(ctx, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*projectsDomain.ProjectPermission), args.Error(1)
}

func (m *MockProjectPermissionRepository) Delete(ctx context.Context, projectID projectsDomain.ProjectID, userID string, permission projectsDomain.PermissionType) error {
	return m.Called(ctx, projectID, userID, permission).Error(0)
}

func (m *MockProjectPermissionRepository) DeleteExpired(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

// newTestProject is a helper that builds a domain project for use in tests
func newTestProject(projectID, name, team string) *projectsDomain.Project {
	p, err := projectsDomain.NewProject(projectsDomain.ProjectID(projectID), name, projectsDomain.Team(team))
	if err != nil {
		panic(err)
	}
	return p
}

var _ = Describe("ProjectHandler", func() {
	var (
		handler     *ProjectHandler
		router      *gin.Engine
		projectRepo *MockProjectRepository
		permRepo    *MockProjectPermissionRepository
		service     *projectsApp.ProjectService
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		loggingConfig := &config.LoggingConfig{Level: "info", Format: "json"}
		logger, err := logging.NewLogger(loggingConfig)
		Expect(err).NotTo(HaveOccurred())

		projectRepo = new(MockProjectRepository)
		permRepo = new(MockProjectPermissionRepository)
		service = projectsApp.NewProjectService(projectRepo, permRepo)
		handler = NewProjectHandler(service, logger)

		router = gin.New()
		router.Use(authContextMiddleware("user-1", "Alice", "alice@example.com", "manager", "team-1", "Team A"))

		userGroup := router.Group("/api/v1")
		managerGroup := router.Group("/api/v1")
		adminGroup := router.Group("/api/v1/admin")
		handler.RegisterRoutes(userGroup, managerGroup, adminGroup)
	})

	Describe("createProject", func() {
		It("should create a project successfully", func() {
			projectRepo.On("ExistsByProjectID", mock.Anything, projectsDomain.ProjectID("proj-1")).Return(false, nil).Once()
			projectRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Once()
			permRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Once()

			body, _ := json.Marshal(map[string]interface{}{
				"projectId": "proj-1",
				"name":      "My Project",
				"team":      "platform",
			})
			req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))
			var resp map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp["projectId"]).To(Equal("proj-1"))
			Expect(resp["name"]).To(Equal("My Project"))
			Expect(resp["team"]).To(Equal("platform"))
		})

		It("should generate a project ID when not provided", func() {
			projectRepo.On("ExistsByProjectID", mock.Anything, mock.Anything).Return(false, nil).Once()
			projectRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Once()
			permRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Once()

			body, _ := json.Marshal(map[string]interface{}{
				"name": "Auto ID Project",
				"team": "platform",
			})
			req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))
			var resp map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp["projectId"]).NotTo(BeEmpty())
		})

		It("should update additional fields after creation", func() {
			projectRepo.On("ExistsByProjectID", mock.Anything, mock.Anything).Return(false, nil).Once()
			projectRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Once()
			permRepo.On("Save", mock.Anything, mock.Anything).Return(nil).Once()
			projectRepo.On("FindByProjectID", mock.Anything, mock.Anything).Return(newTestProject("proj-2", "Repo Project", "platform"), nil).Once()
			projectRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Once()

			body, _ := json.Marshal(map[string]interface{}{
				"projectId":   "proj-2",
				"name":        "Repo Project",
				"team":        "platform",
				"description": "A project with extras",
				"repository":  "https://github.com/org/repo",
			})
			req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))
		})

		It("should return 400 for missing required fields", func() {
			body, _ := json.Marshal(map[string]interface{}{"name": "No Team"})
			req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return 500 when service fails", func() {
			projectRepo.On("ExistsByProjectID", mock.Anything, mock.Anything).Return(false, nil).Once()
			projectRepo.On("Save", mock.Anything, mock.Anything).Return(errors.New("db error")).Once()

			body, _ := json.Marshal(map[string]interface{}{"name": "Fail Project", "team": "platform"})
			req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Describe("getProject", func() {
		It("should return a project by string project ID", func() {
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("my-project")).
				Return(newTestProject("my-project", "My Project", "platform"), nil).Once()

			req := httptest.NewRequest("GET", "/api/v1/projects/my-project", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			var resp map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp["projectId"]).To(Equal("my-project"))
		})

		It("should return 501 for a numeric ID", func() {
			req := httptest.NewRequest("GET", "/api/v1/projects/42", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotImplemented))
		})

		It("should return 404 when project not found", func() {
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("missing")).
				Return(nil, projectsDomain.ErrProjectNotFound).Once()

			req := httptest.NewRequest("GET", "/api/v1/projects/missing", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("getProjectByProjectID", func() {
		It("should return a project", func() {
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("proj-abc")).
				Return(newTestProject("proj-abc", "ABC Project", "team-x"), nil).Once()

			req := httptest.NewRequest("GET", "/api/v1/projects/by-project-id/proj-abc", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			var resp map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp["projectId"]).To(Equal("proj-abc"))
		})

		It("should return 404 when project not found", func() {
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("missing")).
				Return(nil, projectsDomain.ErrProjectNotFound).Once()

			req := httptest.NewRequest("GET", "/api/v1/projects/by-project-id/missing", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("updateProject", func() {
		It("should update a project successfully", func() {
			p := newTestProject("proj-1", "Old Name", "platform")
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("proj-1")).Return(p, nil).Twice()
			projectRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Once()

			body, _ := json.Marshal(map[string]interface{}{"name": "New Name"})
			req := httptest.NewRequest("PUT", "/api/v1/projects/proj-1", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			var resp map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp["name"]).To(Equal("New Name"))
		})

		It("should return 500 when update service fails", func() {
			p := newTestProject("proj-1", "Old Name", "platform")
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("proj-1")).Return(p, nil).Once()
			projectRepo.On("Update", mock.Anything, mock.Anything).Return(errors.New("db error")).Once()

			body, _ := json.Marshal(map[string]interface{}{"name": "New Name"})
			req := httptest.NewRequest("PUT", "/api/v1/projects/proj-1", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})

		It("should return 500 when get-after-update fails", func() {
			p := newTestProject("proj-1", "Old Name", "platform")
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("proj-1")).Return(p, nil).Once()
			projectRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Once()
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("proj-1")).Return(nil, errors.New("db error")).Once()

			body, _ := json.Marshal(map[string]interface{}{"name": "New Name"})
			req := httptest.NewRequest("PUT", "/api/v1/projects/proj-1", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Describe("deleteProject", func() {
		It("should delete a project successfully", func() {
			p := newTestProject("proj-1", "My Project", "platform")
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("proj-1")).Return(p, nil).Once()
			projectRepo.On("Delete", mock.Anything, mock.Anything).Return(nil).Once()

			req := httptest.NewRequest("DELETE", "/api/v1/projects/proj-1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			var resp map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp["message"]).To(ContainSubstring("deleted"))
		})

		It("should return 500 when delete fails", func() {
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("proj-1")).Return(nil, errors.New("not found")).Once()

			req := httptest.NewRequest("DELETE", "/api/v1/projects/proj-1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Describe("listProjects", func() {
		It("should list projects with default pagination", func() {
			projects := []*projectsDomain.Project{
				newTestProject("proj-1", "Project 1", "team-a"),
				newTestProject("proj-2", "Project 2", "team-b"),
			}
			projectRepo.On("FindAll", mock.Anything, 20, 0).Return(projects, int64(2), nil).Once()

			req := httptest.NewRequest("GET", "/api/v1/projects", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("X-Total-Count")).To(Equal("2"))
			var resp []interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp).To(HaveLen(2))
		})

		It("should respect limit and offset query params", func() {
			projectRepo.On("FindAll", mock.Anything, 5, 10).Return([]*projectsDomain.Project{}, int64(0), nil).Once()

			req := httptest.NewRequest("GET", "/api/v1/projects?limit=5&offset=10", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			projectRepo.AssertExpectations(GinkgoT())
		})

		It("should return 500 when service fails", func() {
			projectRepo.On("FindAll", mock.Anything, mock.Anything, mock.Anything).Return(nil, int64(0), errors.New("db error")).Once()

			req := httptest.NewRequest("GET", "/api/v1/projects", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Describe("activateProject", func() {
		It("should activate a project", func() {
			p := newTestProject("proj-1", "My Project", "platform")
			p.Deactivate()
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("proj-1")).Return(p, nil).Once()
			projectRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Once()

			req := httptest.NewRequest("POST", "/api/v1/projects/proj-1/activate", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			var resp map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp["message"]).To(ContainSubstring("activated"))
		})

		It("should return 500 when activate fails", func() {
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("proj-1")).Return(nil, errors.New("not found")).Once()

			req := httptest.NewRequest("POST", "/api/v1/projects/proj-1/activate", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Describe("deactivateProject", func() {
		It("should deactivate a project", func() {
			p := newTestProject("proj-1", "My Project", "platform")
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("proj-1")).Return(p, nil).Once()
			projectRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Once()

			req := httptest.NewRequest("POST", "/api/v1/projects/proj-1/deactivate", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			var resp map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp["message"]).To(ContainSubstring("deactivated"))
		})

		It("should return 500 when deactivate fails", func() {
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("proj-1")).Return(nil, errors.New("not found")).Once()

			req := httptest.NewRequest("POST", "/api/v1/projects/proj-1/deactivate", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Describe("getProjectStats", func() {
		It("should return placeholder stats for a project", func() {
			req := httptest.NewRequest("GET", "/api/v1/projects/stats/proj-1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			var resp map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp["projectId"]).To(Equal("proj-1"))
			Expect(resp["totalTestRuns"]).To(BeNumerically("==", 0))
		})
	})

	Describe("admin endpoints", func() {
		It("grantProjectAccess should return 501", func() {
			req := httptest.NewRequest("POST", "/api/v1/admin/projects/proj-1/users/user-1/access", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotImplemented))
		})

		It("revokeProjectAccess should return 501", func() {
			req := httptest.NewRequest("DELETE", "/api/v1/admin/projects/proj-1/users/user-1/access", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotImplemented))
		})

		It("getProjectUsers should return empty list", func() {
			req := httptest.NewRequest("GET", "/api/v1/admin/projects/proj-1/users", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			var resp map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp["users"]).To(BeEmpty())
		})
	})
})
