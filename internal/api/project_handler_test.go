package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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

// MockProjectRepository and MockProjectPermissionRepository are declared in
// domain_handler_v1_test.go and shared across the api package tests.

// newTestProject constructs a domain Project via domain.NewProject for use in tests.
func newTestProject(projectID, name, team string) *projectsDomain.Project {
	p, err := projectsDomain.NewProject(
		projectsDomain.ProjectID(projectID),
		name,
		projectsDomain.Team(team),
	)
	if err != nil {
		panic(fmt.Sprintf("newTestProject: %v", err))
	}
	return p
}

var _ = Describe("ProjectHandler", func() {
	var (
		handler        *ProjectHandler
		router         *gin.Engine
		projectRepo    *MockProjectRepository
		permissionRepo *MockProjectPermissionRepository
		userGroup      *gin.RouterGroup
		managerGroup   *gin.RouterGroup
		adminGroup     *gin.RouterGroup
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)

		loggingConfig := &config.LoggingConfig{Level: "info", Format: "json"}
		logger, err := logging.NewLogger(loggingConfig)
		Expect(err).NotTo(HaveOccurred())

		projectRepo = new(MockProjectRepository)
		permissionRepo = new(MockProjectPermissionRepository)

		projectService := projectsApp.NewProjectService(projectRepo, permissionRepo)
		handler = NewProjectHandler(projectService, logger)

		router = gin.New()
		userGroup = router.Group("/api/v1")
		managerGroup = router.Group("/api/v1/manager")
		adminGroup = router.Group("/api/v1/admin")

		handler.RegisterRoutes(userGroup, managerGroup, adminGroup)
	})

	Describe("RegisterRoutes", func() {
		It("registers all expected routes", func() {
			routes := router.Routes()

			expectedRoutes := []string{
				"GET /api/v1/projects",
				"GET /api/v1/projects/:projectId",
				"GET /api/v1/projects/by-project-id/:projectId",
				"GET /api/v1/projects/stats/:projectId",
				"POST /api/v1/manager/projects",
				"PUT /api/v1/manager/projects/:projectId",
				"DELETE /api/v1/manager/projects/:projectId",
				"POST /api/v1/manager/projects/:projectId/activate",
				"POST /api/v1/manager/projects/:projectId/deactivate",
				"POST /api/v1/admin/projects/:projectId/users/:userId/access",
				"DELETE /api/v1/admin/projects/:projectId/users/:userId/access",
				"GET /api/v1/admin/projects/:projectId/users",
			}

			for _, expected := range expectedRoutes {
				found := false
				for _, route := range routes {
					if fmt.Sprintf("%s %s", route.Method, route.Path) == expected {
						found = true
						break
					}
				}
				Expect(found).To(BeTrue(), fmt.Sprintf("route %s not registered", expected))
			}
		})
	})

	Describe("getProject", func() {
		It("returns 501 when projectId is numeric", func() {
			req := httptest.NewRequest("GET", "/api/v1/projects/42", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotImplemented))

			var body map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body["error"]).To(ContainSubstring("not yet implemented"))
		})

		It("returns 200 with project data when found by string ID", func() {
			project := newTestProject("my-project", "My Project", "team-alpha")
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("my-project")).
				Return(project, nil).Once()

			req := httptest.NewRequest("GET", "/api/v1/projects/my-project", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var body map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body["projectId"]).To(Equal("my-project"))
			Expect(body["name"]).To(Equal("My Project"))

			projectRepo.AssertExpectations(GinkgoT())
		})

		It("returns 404 when project is not found", func() {
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("missing")).
				Return(nil, errors.New("not found")).Once()

			req := httptest.NewRequest("GET", "/api/v1/projects/missing", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
			projectRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("getProjectByProjectID", func() {
		It("returns 200 with the project when found", func() {
			project := newTestProject("proj-abc", "Project ABC", "team-beta")
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("proj-abc")).
				Return(project, nil).Once()

			req := httptest.NewRequest("GET", "/api/v1/projects/by-project-id/proj-abc", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var body map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body["projectId"]).To(Equal("proj-abc"))

			projectRepo.AssertExpectations(GinkgoT())
		})

		It("returns 404 when the project does not exist", func() {
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("no-such-proj")).
				Return(nil, errors.New("not found")).Once()

			req := httptest.NewRequest("GET", "/api/v1/projects/by-project-id/no-such-proj", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
			projectRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("updateProject", func() {
		It("returns 200 with updated project on success", func() {
			updated := newTestProject("proj-1", "Updated Name", "team-x")

			// UpdateProject internally calls FindByProjectID then Update
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("proj-1")).
				Return(newTestProject("proj-1", "Original Name", "team-x"), nil).Once()
			projectRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Once()
			// After update, GetProject is called to return the fresh project
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("proj-1")).
				Return(updated, nil).Once()

			body := map[string]interface{}{"name": "Updated Name"}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("PUT", "/api/v1/manager/projects/proj-1", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var resp map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &resp)).To(Succeed())
			Expect(resp["projectId"]).To(Equal("proj-1"))
			Expect(resp["name"]).To(Equal("Updated Name"))

			projectRepo.AssertExpectations(GinkgoT())
		})

		It("returns 400 when request body is invalid JSON", func() {
			req := httptest.NewRequest("PUT", "/api/v1/manager/projects/proj-1", bytes.NewBufferString("not-json"))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("returns 500 when UpdateProject fails", func() {
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("proj-1")).
				Return(newTestProject("proj-1", "Name", "team-x"), nil).Once()
			projectRepo.On("Update", mock.Anything, mock.Anything).
				Return(errors.New("db failure")).Once()

			body := map[string]interface{}{"name": "New Name"}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("PUT", "/api/v1/manager/projects/proj-1", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
			projectRepo.AssertExpectations(GinkgoT())
		})

		It("returns 500 when GetProject after update fails", func() {
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("proj-1")).
				Return(newTestProject("proj-1", "Name", "team-x"), nil).Once()
			projectRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Once()
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("proj-1")).
				Return(nil, errors.New("not found after update")).Once()

			body := map[string]interface{}{"name": "New Name"}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("PUT", "/api/v1/manager/projects/proj-1", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
			projectRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("deleteProject", func() {
		It("returns 200 with success message when project is deleted", func() {
			project := newTestProject("proj-del", "To Delete", "team-z")
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("proj-del")).
				Return(project, nil).Once()
			projectRepo.On("Delete", mock.Anything, project.ID()).Return(nil).Once()

			req := httptest.NewRequest("DELETE", "/api/v1/manager/projects/proj-del", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var body map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body["message"]).To(Equal("Project deleted successfully"))

			projectRepo.AssertExpectations(GinkgoT())
		})

		It("returns 500 when DeleteProject fails", func() {
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("proj-del")).
				Return(nil, errors.New("project not found")).Once()

			req := httptest.NewRequest("DELETE", "/api/v1/manager/projects/proj-del", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
			projectRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("activateProject", func() {
		It("returns 200 with activated message on success", func() {
			project := newTestProject("proj-act", "Activate Me", "team-a")
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("proj-act")).
				Return(project, nil).Once()
			projectRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Once()

			req := httptest.NewRequest("POST", "/api/v1/manager/projects/proj-act/activate", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var body map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body["message"]).To(Equal("Project activated"))

			projectRepo.AssertExpectations(GinkgoT())
		})

		It("returns 500 when ActivateProject fails", func() {
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("proj-act")).
				Return(nil, errors.New("not found")).Once()

			req := httptest.NewRequest("POST", "/api/v1/manager/projects/proj-act/activate", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
			projectRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("deactivateProject", func() {
		It("returns 200 with deactivated message on success", func() {
			project := newTestProject("proj-deact", "Deactivate Me", "team-b")
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("proj-deact")).
				Return(project, nil).Once()
			projectRepo.On("Update", mock.Anything, mock.Anything).Return(nil).Once()

			req := httptest.NewRequest("POST", "/api/v1/manager/projects/proj-deact/deactivate", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var body map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body["message"]).To(Equal("Project deactivated"))

			projectRepo.AssertExpectations(GinkgoT())
		})

		It("returns 500 when DeactivateProject fails", func() {
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("proj-deact")).
				Return(nil, errors.New("not found")).Once()

			req := httptest.NewRequest("POST", "/api/v1/manager/projects/proj-deact/deactivate", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
			projectRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("getProjectStats", func() {
		It("returns 200 with hardcoded zero stats for any project ID", func() {
			req := httptest.NewRequest("GET", "/api/v1/projects/stats/proj-stats", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var body map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body["projectId"]).To(Equal("proj-stats"))
			Expect(body["totalTestRuns"]).To(BeNumerically("==", 0))
			Expect(body["passedRuns"]).To(BeNumerically("==", 0))
			Expect(body["failedRuns"]).To(BeNumerically("==", 0))
			Expect(body["successRate"]).To(BeNumerically("==", 0))
			Expect(body["avgDuration"]).To(BeNumerically("==", 0))
			Expect(body["lastRun"]).To(BeNil())
		})
	})

	Describe("grantProjectAccess", func() {
		It("returns 501 not implemented", func() {
			req := httptest.NewRequest("POST", "/api/v1/admin/projects/proj-1/users/user-1/access", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotImplemented))

			var body map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body["error"]).To(ContainSubstring("not yet implemented"))
		})
	})

	Describe("revokeProjectAccess", func() {
		It("returns 501 not implemented", func() {
			req := httptest.NewRequest("DELETE", "/api/v1/admin/projects/proj-1/users/user-1/access", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotImplemented))

			var body map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body["error"]).To(ContainSubstring("not yet implemented"))
		})
	})

	Describe("getProjectUsers", func() {
		It("returns 200 with an empty users list", func() {
			req := httptest.NewRequest("GET", "/api/v1/admin/projects/proj-1/users", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var body map[string]interface{}
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body["users"]).To(HaveLen(0))
		})
	})
})
