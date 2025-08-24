package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	projectsApp "github.com/guidewire-oss/fern-platform/internal/domains/projects/application"
	projectsDomain "github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	// ...existing code...
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

// Mocks

type mockTestRunService struct {
	mock.Mock
}

func (m *mockTestRunService) CreateTestRun(ctx context.Context, tr *domain.TestRun) error {
	args := m.Called(ctx, tr)
	return args.Error(0)
}
func (m *mockTestRunService) GetTestRunByRunID(ctx context.Context, runID string) (*domain.TestRun, error) {
	args := m.Called(ctx, runID)
	return args.Get(0).(*domain.TestRun), args.Error(1)
}
func (m *mockTestRunService) ListTestRuns(ctx context.Context, projectUUID string, limit, offset int) ([]*domain.TestRun, int64, error) {
	args := m.Called(ctx, projectUUID, limit, offset)
	return args.Get(0).([]*domain.TestRun), args.Get(1).(int64), args.Error(2)
}
func (m *mockTestRunService) CreateSuiteRun(ctx context.Context, sr *domain.SuiteRun) error {
	return nil
}
func (m *mockTestRunService) CreateSpecRun(ctx context.Context, sr *domain.SpecRun) error { return nil }

// ...other methods as needed...

type mockProjectService struct {
	mock.Mock
}

func (m *mockProjectService) CreateProject(ctx context.Context, id projectsDomain.ProjectID, name string, team projectsDomain.Team, source string) (*projectsDomain.Project, error) {
	args := m.Called(ctx, id, name, team, source)
	return args.Get(0).(*projectsDomain.Project), args.Error(1)
}
func (m *mockProjectService) GetProject(ctx context.Context, id projectsDomain.ProjectID) (*projectsDomain.Project, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*projectsDomain.Project), args.Error(1)
}
func (m *mockProjectService) UpdateProject(ctx context.Context, id projectsDomain.ProjectID, req projectsApp.UpdateProjectRequest) error {
	return nil
}
func (m *mockProjectService) ListProjects(ctx context.Context, limit, offset int) ([]*projectsDomain.Project, int64, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]*projectsDomain.Project), args.Get(1).(int64), args.Error(2)
}

func TestCreateFernTestReport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New() // Use gin.New() instead of gin.Default() to avoid middleware conflicts
	mockTestRunSvc := new(mockTestRunService)
	mockProjectSvc := new(mockProjectService)
	cfg := &config.LoggingConfig{Level: "debug", Format: "text", Output: "stdout", Structured: false}
	logger, _ := logging.NewLogger(cfg)
	handler := NewFernLegacyHandler(mockTestRunSvc, mockProjectSvc, logger)
	handler.RegisterRoutes(r.Group("/api"))

	project := &projectsDomain.Project{}
	mockProjectSvc.On("GetProject", mock.Anything, projectsDomain.ProjectID("proj-123")).Return(project, nil)
	mockTestRunSvc.On("GetTestRunByRunID", mock.Anything, mock.Anything).Return((*domain.TestRun)(nil), nil)
	mockTestRunSvc.On("CreateTestRun", mock.Anything, mock.Anything).Return(nil)

	body := map[string]interface{}{
		"test_project_id":   "proj-123",
		"test_project_name": "TestProj",
		"test_seed":         42,
		"start_time":        time.Now().Format(time.RFC3339),
		"suite_runs":        []interface{}{},
	}
	b, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/reports/testrun", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestListFernTestReports(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New() // Use gin.New() instead of gin.Default()
	mockTestRunSvc := new(mockTestRunService)
	mockProjectSvc := new(mockProjectService)
	cfg := &config.LoggingConfig{Level: "debug", Format: "text", Output: "stdout", Structured: false}
	logger, _ := logging.NewLogger(cfg)
	handler := NewFernLegacyHandler(mockTestRunSvc, mockProjectSvc, logger)
	handler.RegisterRoutes(r.Group("/api"))

	tr := &domain.TestRun{RunID: "run-1", ProjectID: "proj-1", GitBranch: "main", GitCommit: "abc123", Status: "completed", StartTime: time.Now()}
	mockTestRunSvc.On("ListTestRuns", mock.Anything, "proj-1", 20, 0).Return([]*domain.TestRun{tr}, int64(1), nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/reports/testruns?project_uuid=proj-1", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetFernTestReport_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New() // Use gin.New() instead of gin.Default()
	mockTestRunSvc := new(mockTestRunService)
	mockProjectSvc := new(mockProjectService)
	cfg := &config.LoggingConfig{Level: "debug", Format: "text", Output: "stdout", Structured: false}
	logger, _ := logging.NewLogger(cfg)
	handler := NewFernLegacyHandler(mockTestRunSvc, mockProjectSvc, logger)
	handler.RegisterRoutes(r.Group("/api"))

	mockTestRunSvc.On("GetTestRunByRunID", mock.Anything, "run-404").Return((*domain.TestRun)(nil), assert.AnError)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/reports/testrun/run-404", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
