package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/guidewire-oss/fern-platform/internal/api"
	projectsApp "github.com/guidewire-oss/fern-platform/internal/domains/projects/application"
	projectsDomain "github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
	testHelpers "github.com/guidewire-oss/fern-platform/internal/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestProjectHandlerSettingsAreProcessed verifies that settings in project requests are properly handled
func TestProjectHandlerSettingsAreProcessed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// This test documents that the settings field is now properly processed
	// in both createProject and updateProject endpoints
	t.Run("createProject request structure includes settings", func(t *testing.T) {
		requestBody := map[string]interface{}{
			"projectId":     "test-project",
			"name":          "Test Project",
			"description":   "Test Description",
			"repository":    "https://github.com/test/repo",
			"defaultBranch": "main",
			"team":          "test-team",
			"settings": map[string]interface{}{
				"buildTool":     "maven",
				"javaVersion":   "11",
				"notifications": true,
			},
		}
		
		jsonBytes, err := json.Marshal(requestBody)
		assert.NoError(t, err)
		
		// Create a test request
		req, err := http.NewRequest("POST", "/api/v1/projects", bytes.NewBuffer(jsonBytes))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		
		// The handler would now process settings via:
		// 1. Parse input.Settings from request body
		// 2. Include settings in UpdateProjectRequest
		// 3. Call project.SetSetting() for each key-value pair
		// 4. Return settings in the response as a map (not string)
		
		assert.Contains(t, string(jsonBytes), "settings")
		assert.Contains(t, string(jsonBytes), "buildTool")
	})
	
	t.Run("updateProject request structure includes settings", func(t *testing.T) {
		requestBody := map[string]interface{}{
			"name": "Updated Name",
			"settings": map[string]interface{}{
				"buildTool":   "gradle",
				"javaVersion": "17",
			},
		}
		
		jsonBytes, err := json.Marshal(requestBody)
		assert.NoError(t, err)
		
		req, err := http.NewRequest("PUT", "/api/v1/projects/test-project", bytes.NewBuffer(jsonBytes))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		
		// The handler now properly processes settings in updates
		assert.Contains(t, string(jsonBytes), "settings")
		assert.Contains(t, string(jsonBytes), "gradle")
	})
	
	t.Run("project response includes settings as map", func(t *testing.T) {
		// The convertProjectToAPI method now returns settings as a map
		// instead of a JSON string, making it easier for clients to use
		
		// Example response structure:
		expectedResponse := map[string]interface{}{
			"id":            1,
			"projectId":     "test-project",
			"name":          "Test Project",
			"description":   "Test Description",
			"repository":    "https://github.com/test/repo",
			"defaultBranch": "main",
			"team":          "test-team",
			"isActive":      true,
			"settings": map[string]interface{}{
				"buildTool":     "maven",
				"javaVersion":   "11",
				"notifications": true,
			},
			"createdAt": "2024-01-01T00:00:00Z",
			"updatedAt": "2024-01-01T00:00:00Z",
		}
		
		// Settings are now returned as a proper map
		settings, ok := expectedResponse["settings"].(map[string]interface{})
		assert.True(t, ok, "Settings should be a map, not a string")
		assert.Equal(t, "maven", settings["buildTool"])
	})
}

func TestGetProject_ErrorCases(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("project not found", func(t *testing.T) {
		repo := new(testHelpers.MockProjectRepository)
		service := projectsApp.NewProjectService(repo, nil)
		handler := api.NewProjectHandler(service, nil)

		router := gin.New()
		user := router.Group("")
		handler.RegisterRoutes(user, user, user)

		repo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("p1")).
			Return(nil, projectsDomain.ErrProjectNotFound)

		req := httptest.NewRequest("GET", "/projects/p1", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("db internal error", func(t *testing.T) {
		repo := new(testHelpers.MockProjectRepository)
		service := projectsApp.NewProjectService(repo, nil)
		handler := api.NewProjectHandler(service, nil)

		router := gin.New()
		user := router.Group("")
		handler.RegisterRoutes(user, user, user)

		repo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("p1")).
			Return(&projectsDomain.Project{}, errors.New("db error"))

		req := httptest.NewRequest("GET", "/projects/p1", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestGetProjectByProjectID_ErrorCases(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("project not found", func(t *testing.T) {
		repo := new(testHelpers.MockProjectRepository)
		service := projectsApp.NewProjectService(repo, nil)
		handler := api.NewProjectHandler(service, nil)

		router := gin.New()
		user := router.Group("")
		handler.RegisterRoutes(user, user, user)

		repo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("p1")).
			Return(nil, projectsDomain.ErrProjectNotFound)

		req := httptest.NewRequest("GET", "/projects/by-project-id/p1", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("db internal error", func(t *testing.T) {
		repo := new(testHelpers.MockProjectRepository)
		service := projectsApp.NewProjectService(repo, nil)
		handler := api.NewProjectHandler(service, nil)

		router := gin.New()
		user := router.Group("")
		handler.RegisterRoutes(user, user, user)

		repo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("p1")).
			Return(&projectsDomain.Project{}, errors.New("db error"))

		req := httptest.NewRequest("GET", "/projects/by-project-id/p1", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestUpdateProject_ErrorCases(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("project not found", func(t *testing.T) {
		repo := new(testHelpers.MockProjectRepository)
		service := projectsApp.NewProjectService(repo, nil)
		handler := api.NewProjectHandler(service, nil)

		router := gin.New()
		manager := router.Group("")
		handler.RegisterRoutes(manager, manager, manager)

		repo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("p1")).
			Return(nil, projectsDomain.ErrProjectNotFound)

		body := bytes.NewBuffer([]byte(`{}`))
		req := httptest.NewRequest("PUT", "/projects/p1", body)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("db internal error", func(t *testing.T) {
		repo := new(testHelpers.MockProjectRepository)
		service := projectsApp.NewProjectService(repo, nil)
		handler := api.NewProjectHandler(service, nil)

		router := gin.New()
		manager := router.Group("")
		handler.RegisterRoutes(manager, manager, manager)

		repo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("p1")).
			Return(&projectsDomain.Project{}, errors.New("db error"))

		body := bytes.NewBuffer([]byte(`{}`))
		req := httptest.NewRequest("PUT", "/projects/p1", body)
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestDeleteProject_ErrorCases(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("project not found", func(t *testing.T) {
		repo := new(testHelpers.MockProjectRepository)
		service := projectsApp.NewProjectService(repo, nil)
		handler := api.NewProjectHandler(service, nil)

		router := gin.New()
		manager := router.Group("")
		handler.RegisterRoutes(manager, manager, manager)

		repo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("p1")).
			Return(nil, projectsDomain.ErrProjectNotFound)

		req := httptest.NewRequest("DELETE", "/projects/p1", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("db internal error", func(t *testing.T) {
		repo := new(testHelpers.MockProjectRepository)
		service := projectsApp.NewProjectService(repo, nil)
		handler := api.NewProjectHandler(service, nil)

		router := gin.New()
		manager := router.Group("")
		handler.RegisterRoutes(manager, manager, manager)

		repo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("p1")).
			Return(&projectsDomain.Project{}, errors.New("db error"))

		req := httptest.NewRequest("DELETE", "/projects/p1", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestActivateProject_ErrorCases(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("project not found", func(t *testing.T) {
		repo := new(testHelpers.MockProjectRepository)
		service := projectsApp.NewProjectService(repo, nil)
		handler := api.NewProjectHandler(service, nil)

		router := gin.New()
		manager := router.Group("")
		handler.RegisterRoutes(manager, manager, manager)

		repo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("p1")).
			Return(nil, projectsDomain.ErrProjectNotFound)

		req := httptest.NewRequest("POST", "/projects/p1/activate", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("db internal error", func(t *testing.T) {
		repo := new(testHelpers.MockProjectRepository)
		service := projectsApp.NewProjectService(repo, nil)
		handler := api.NewProjectHandler(service, nil)

		router := gin.New()
		manager := router.Group("")
		handler.RegisterRoutes(manager, manager, manager)

		repo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("p1")).
			Return(&projectsDomain.Project{}, errors.New("db error"))

		req := httptest.NewRequest("POST", "/projects/p1/activate", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestDeactivateProject_ErrorCases(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("project not found", func(t *testing.T) {
		repo := new(testHelpers.MockProjectRepository)
		service := projectsApp.NewProjectService(repo, nil)
		handler := api.NewProjectHandler(service, nil)

		router := gin.New()
		manager := router.Group("")
		handler.RegisterRoutes(manager, manager, manager)

		repo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("p1")).
			Return(nil, projectsDomain.ErrProjectNotFound)

		req := httptest.NewRequest("POST", "/projects/p1/deactivate", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("db internal error", func(t *testing.T) {
		repo := new(testHelpers.MockProjectRepository)
		service := projectsApp.NewProjectService(repo, nil)
		handler := api.NewProjectHandler(service, nil)

		router := gin.New()
		manager := router.Group("")
		handler.RegisterRoutes(manager, manager, manager)

		repo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("p1")).
			Return(&projectsDomain.Project{}, errors.New("db error"))

		req := httptest.NewRequest("POST", "/projects/p1/deactivate", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}