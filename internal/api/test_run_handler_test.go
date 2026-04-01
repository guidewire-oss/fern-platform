package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/gin-gonic/gin"
	tagsApp "github.com/guidewire-oss/fern-platform/internal/domains/tags/application"
	projectsApp "github.com/guidewire-oss/fern-platform/internal/domains/projects/application"
	projectsDomain "github.com/guidewire-oss/fern-platform/internal/domains/projects/domain"
	testMocks "github.com/guidewire-oss/fern-platform/internal/testhelpers"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
)

// MockTestRunRepository provides a mock implementation of TestRunRepository
type MockTestRunRepository struct {
	mock.Mock
}

func (m *MockTestRunRepository) Create(ctx context.Context, testRun *domain.TestRun) error {
	args := m.Called(ctx, testRun)
	if args.Get(0) != nil {
		return args.Error(0)
	}
	// Simulate database auto-increment ID
	if testRun.ID == 0 {
		testRun.ID = 1
	}
	// Set timestamps
	if testRun.StartTime.IsZero() {
		testRun.StartTime = time.Now()
	}
	return nil
}

func (m *MockTestRunRepository) Update(ctx context.Context, testRun *domain.TestRun) error {
	args := m.Called(ctx, testRun)
	return args.Error(0)
}

func (m *MockTestRunRepository) GetByID(ctx context.Context, id uint) (*domain.TestRun, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TestRun), args.Error(1)
}

func (m *MockTestRunRepository) GetByRunID(ctx context.Context, runID string) (*domain.TestRun, error) {
	args := m.Called(ctx, runID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TestRun), args.Error(1)
}

func (m *MockTestRunRepository) GetWithDetails(ctx context.Context, id uint) (*domain.TestRun, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TestRun), args.Error(1)
}

func (m *MockTestRunRepository) GetLatestByProjectID(ctx context.Context, projectID string, limit int) ([]*domain.TestRun, error) {
	args := m.Called(ctx, projectID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.TestRun), args.Error(1)
}

func (m *MockTestRunRepository) GetRecent(ctx context.Context, limit int) ([]*domain.TestRun, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.TestRun), args.Error(1)
}

func (m *MockTestRunRepository) GetTestRunSummary(ctx context.Context, projectID string) (*domain.TestRunSummary, error) {
	args := m.Called(ctx, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TestRunSummary), args.Error(1)
}

func (m *MockTestRunRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTestRunRepository) GetAll(ctx context.Context, limit, offset int) ([]*domain.TestRun, int64, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*domain.TestRun), args.Get(1).(int64), args.Error(2)
}

func (m *MockTestRunRepository) CountByProjectID(ctx context.Context, projectID string) (int64, error) {
	args := m.Called(ctx, projectID)
	return args.Get(0).(int64), args.Error(1)
}

// MockSuiteRunRepository provides a mock implementation of SuiteRunRepository
type MockSuiteRunRepository struct {
	mock.Mock
}

func (m *MockSuiteRunRepository) Create(ctx context.Context, suiteRun *domain.SuiteRun) error {
	args := m.Called(ctx, suiteRun)
	return args.Error(0)
}

func (m *MockSuiteRunRepository) Update(ctx context.Context, suiteRun *domain.SuiteRun) error {
	args := m.Called(ctx, suiteRun)
	return args.Error(0)
}

func (m *MockSuiteRunRepository) GetByID(ctx context.Context, id uint) (*domain.SuiteRun, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SuiteRun), args.Error(1)
}

func (m *MockSuiteRunRepository) FindByTestRunID(ctx context.Context, testRunID uint) ([]*domain.SuiteRun, error) {
	args := m.Called(ctx, testRunID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SuiteRun), args.Error(1)
}

func (m *MockSuiteRunRepository) GetWithSpecRuns(ctx context.Context, id uint) (*domain.SuiteRun, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SuiteRun), args.Error(1)
}

func (m *MockSuiteRunRepository) CreateBatch(ctx context.Context, suiteRuns []*domain.SuiteRun) error {
	args := m.Called(ctx, suiteRuns)
	return args.Error(0)
}

// MockSpecRunRepository provides a mock implementation of SpecRunRepository
type MockSpecRunRepository struct {
	mock.Mock
}

func (m *MockSpecRunRepository) Create(ctx context.Context, specRun *domain.SpecRun) error {
	args := m.Called(ctx, specRun)
	return args.Error(0)
}

func (m *MockSpecRunRepository) CreateBatch(ctx context.Context, specRuns []*domain.SpecRun) error {
	args := m.Called(ctx, specRuns)
	return args.Error(0)
}

func (m *MockSpecRunRepository) Update(ctx context.Context, specRun *domain.SpecRun) error {
	args := m.Called(ctx, specRun)
	return args.Error(0)
}

func (m *MockSpecRunRepository) GetByID(ctx context.Context, id uint) (*domain.SpecRun, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SpecRun), args.Error(1)
}

func (m *MockSpecRunRepository) FindBySuiteRunID(ctx context.Context, suiteRunID uint) ([]*domain.SpecRun, error) {
	args := m.Called(ctx, suiteRunID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SpecRun), args.Error(1)
}

func (m *MockSpecRunRepository) GetFailedByTestRunID(ctx context.Context, testRunID uint, limit int) ([]*domain.SpecRun, error) {
	args := m.Called(ctx, testRunID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SpecRun), args.Error(1)
}

var _ = Describe("TestRunHandler", func() {
	var (
		handler        *TestRunHandler
		router         *gin.Engine
		logger         *logging.Logger
		testRunRepo    *MockTestRunRepository
		suiteRunRepo   *MockSuiteRunRepository
		specRunRepo    *MockSpecRunRepository
		projectRepo    *testMocks.MockProjectRepository
		permRepo       *testMocks.MockProjectPermissionRepository
		testingService *application.TestRunService
		projectService *projectsApp.ProjectService
		userGroup      *gin.RouterGroup
		adminGroup     *gin.RouterGroup
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)

		// Initialize logger
		loggingConfig := &config.LoggingConfig{
			Level:  "info",
			Format: "json",
		}
		var err error
		logger, err = logging.NewLogger(loggingConfig)
		Expect(err).NotTo(HaveOccurred())

		// Create mocks
		testRunRepo = new(MockTestRunRepository)
		suiteRunRepo = new(MockSuiteRunRepository)
		specRunRepo = new(MockSpecRunRepository)
		projectRepo = new(testMocks.MockProjectRepository)
		permRepo = new(testMocks.MockProjectPermissionRepository)

		// Create service with mocks
		testingService = application.NewTestRunService(testRunRepo, suiteRunRepo, specRunRepo)
		projectService = projectsApp.NewProjectService(projectRepo, permRepo)

		// Create handler
		handler = NewTestRunHandler(testingService, projectService, logger)

		// Setup router with groups
		router = gin.New()
		userGroup = router.Group("/api/v1")
		adminGroup = router.Group("/api/v1/admin")

		// Register routes
		handler.RegisterRoutes(userGroup, adminGroup)
	})

	Describe("createTestRun", func() {
		It("should create a test run successfully", func() {
			project, err := projectsDomain.NewProject(projectsDomain.ProjectID("project-123"), "Test Project", projectsDomain.Team("team-1"))
			Expect(err).NotTo(HaveOccurred())
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("project-123")).Return(project, nil).Once()

			// Prepare request body
			requestBody := map[string]interface{}{
				"projectId": "project-123",
				"branch":    "main",
				"tags":      []string{"tag1", "tag2"},
			}
			jsonBody, _ := json.Marshal(requestBody)

			// Mock repository expectations
			testRunRepo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()

			// Perform request
			req := httptest.NewRequest("POST", "/api/v1/admin/test-runs", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Assert response
			Expect(w.Code).To(Equal(http.StatusCreated))

			var response map[string]interface{}
			err = json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["projectId"]).To(Equal("project-123"))
			Expect(response["branch"]).To(Equal("main"))
			Expect(response["status"]).To(Equal("running"))
			Expect(response["tags"]).To(Equal([]interface{}{"tag1", "tag2"}))

			testRunRepo.AssertExpectations(GinkgoT())
			projectRepo.AssertExpectations(GinkgoT())
		})

		It("should create a test run with custom ID", func() {
			project, err := projectsDomain.NewProject(projectsDomain.ProjectID("project-123"), "Test Project", projectsDomain.Team("team-1"))
			Expect(err).NotTo(HaveOccurred())
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("project-123")).Return(project, nil).Once()

			requestBody := map[string]interface{}{
				"id":        "custom-id-123",
				"projectId": "project-123",
			}
			jsonBody, _ := json.Marshal(requestBody)

			testRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(tr *domain.TestRun) bool {
				return tr.RunID == "custom-id-123"
			})).Return(nil).Once()

			req := httptest.NewRequest("POST", "/api/v1/admin/test-runs", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))
			testRunRepo.AssertExpectations(GinkgoT())
			projectRepo.AssertExpectations(GinkgoT())
		})

		It("should return bad request for missing required fields", func() {
			requestBody := map[string]interface{}{
				"branch": "main",
			}
			jsonBody, _ := json.Marshal(requestBody)

			req := httptest.NewRequest("POST", "/api/v1/admin/test-runs", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return not found when project doesn't exist", func() {
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("missing-project")).
				Return(nil, projectsDomain.ErrProjectNotFound).Once()

			requestBody := map[string]interface{}{
				"projectId": "missing-project",
				"branch":    "main",
			}
			jsonBody, _ := json.Marshal(requestBody)

			req := httptest.NewRequest("POST", "/api/v1/admin/test-runs", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Equal("Invalid project ID"))

			testRunRepo.AssertNotCalled(GinkgoT(), "Create", mock.Anything, mock.Anything)
			projectRepo.AssertExpectations(GinkgoT())
		})

		It("should return internal server error when project validation fails with non-notfound error", func() {
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("project-123")).
				Return(nil, errors.New("db error")).Once()

			requestBody := map[string]interface{}{
				"projectId": "project-123",
				"branch":    "main",
			}
			jsonBody, _ := json.Marshal(requestBody)

			req := httptest.NewRequest("POST", "/api/v1/admin/test-runs", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["error"]).To(Equal("Failed to validate project"))

			testRunRepo.AssertNotCalled(GinkgoT(), "Create", mock.Anything, mock.Anything)
			projectRepo.AssertExpectations(GinkgoT())
		})

		It("should return internal server error when service fails", func() {
			project, err := projectsDomain.NewProject(projectsDomain.ProjectID("project-123"), "Test Project", projectsDomain.Team("team-1"))
			Expect(err).NotTo(HaveOccurred())
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("project-123")).Return(project, nil).Once()

			requestBody := map[string]interface{}{
				"projectId": "project-123",
			}
			jsonBody, _ := json.Marshal(requestBody)

			testRunRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("database error")).Once()

			req := httptest.NewRequest("POST", "/api/v1/admin/test-runs", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
			testRunRepo.AssertExpectations(GinkgoT())
			projectRepo.AssertExpectations(GinkgoT())
		})

		It("should handle duplicate test run creation", func() {
			project, err := projectsDomain.NewProject(projectsDomain.ProjectID("project-123"), "Test Project", projectsDomain.Team("team-1"))
			Expect(err).NotTo(HaveOccurred())
			projectRepo.On("FindByProjectID", mock.Anything, projectsDomain.ProjectID("project-123")).Return(project, nil).Once()

			requestBody := map[string]interface{}{
				"id":        "duplicate-id",
				"projectId": "project-123",
			}
			jsonBody, _ := json.Marshal(requestBody)

			// First call returns unique constraint violation
			testRunRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("unique constraint violation")).Once()
			// GetByRunID returns existing test run
			existingTestRun := &domain.TestRun{
				ID:        1,
				RunID:     "duplicate-id",
				ProjectID: "project-123",
				Status:    "running",
			}
			testRunRepo.On("GetByRunID", mock.Anything, "duplicate-id").Return(existingTestRun, nil).Once()

			req := httptest.NewRequest("POST", "/api/v1/admin/test-runs", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusCreated))
			testRunRepo.AssertExpectations(GinkgoT())
			projectRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("getTestRun", func() {
		It("should get a test run successfully", func() {
			now := time.Now()
			testRun := &domain.TestRun{
				ID:           1,
				ProjectID:    "project-123",
				RunID:        "run-123",
				Name:         "Test Run",
				Branch:       "main",
				Status:       "passed",
				StartTime:    now,
				TotalTests:   10,
				PassedTests:  9,
				FailedTests:  1,
				SkippedTests: 0,
				Duration:     5 * time.Second,
				Environment:  "test",
			}

			testRunRepo.On("GetByID", mock.Anything, uint(1)).Return(testRun, nil).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["id"]).To(BeNumerically("==", 1))
			Expect(response["projectId"]).To(Equal("project-123"))
			Expect(response["status"]).To(Equal("passed"))

			testRunRepo.AssertExpectations(GinkgoT())
		})

		It("should return bad request for invalid ID", func() {
			req := httptest.NewRequest("GET", "/api/v1/test-runs/invalid", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return not found when test run doesn't exist", func() {
			testRunRepo.On("GetByID", mock.Anything, uint(999)).Return(nil, errors.New("not found")).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs/999", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
			testRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("getTestRunByRunID", func() {
		It("should return test run when found", func() {
			expectedRun := &domain.TestRun{
				ID:        1,
				RunID:     "run-123",
				ProjectID: "project-123",
				Status:    "passed",
			}
			testRunRepo.On("GetByRunID", mock.Anything, "run-123").Return(expectedRun, nil)

			req := httptest.NewRequest("GET", "/api/v1/test-runs/by-run-id/run-123", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should return not found when run ID does not exist", func() {
			testRunRepo.On("GetByRunID", mock.Anything, "nonexistent").Return(nil, errors.New("not found"))

			req := httptest.NewRequest("GET", "/api/v1/test-runs/by-run-id/nonexistent", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("listTestRuns", func() {
		It("should list test runs successfully with default pagination", func() {
			testRuns := []*domain.TestRun{
				{ID: 1, ProjectID: "project-123", Status: "passed"},
				{ID: 2, ProjectID: "project-123", Status: "failed"},
			}

			testRunRepo.On("GetLatestByProjectID", mock.Anything, "project-123", 50).Return(testRuns, nil).Once()
			testRunRepo.On("CountByProjectID", mock.Anything, "project-123").Return(int64(2), nil).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs?project_id=project-123", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("X-Total-Count")).To(Equal("2"))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["total"]).To(BeNumerically("==", 2))
			Expect(response["limit"]).To(BeNumerically("==", 50))
			Expect(response["offset"]).To(BeNumerically("==", 0))

			testRunRepo.AssertExpectations(GinkgoT())
		})

		It("should list test runs with custom pagination", func() {
			testRuns := []*domain.TestRun{
				{ID: 3, ProjectID: "project-123", Status: "passed"},
			}

			testRunRepo.On("GetLatestByProjectID", mock.Anything, "project-123", 10).Return(testRuns, nil).Once()
			testRunRepo.On("CountByProjectID", mock.Anything, "project-123").Return(int64(15), nil).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs?project_id=project-123&limit=10&offset=10", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("X-Total-Count")).To(Equal("15"))

			testRunRepo.AssertExpectations(GinkgoT())
		})

		It("should return bad request for invalid limit", func() {
			req := httptest.NewRequest("GET", "/api/v1/test-runs?limit=0", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return bad request for negative offset", func() {
			req := httptest.NewRequest("GET", "/api/v1/test-runs?offset=-1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should handle list test runs without project ID", func() {
			// When no project ID is provided, the service returns empty list
			// This matches the current implementation in test_run_service.go
			req := httptest.NewRequest("GET", "/api/v1/test-runs", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["total"]).To(BeNumerically("==", 0))
			Expect(response["data"]).To(HaveLen(0))
		})

		It("should return internal server error when service fails", func() {
			testRunRepo.On("GetLatestByProjectID", mock.Anything, "project-123", 50).Return(nil, errors.New("database error")).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs?project_id=project-123", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
			testRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("countTestRuns", func() {
		It("should count test runs successfully", func() {
			testRunRepo.On("GetLatestByProjectID", mock.Anything, "project-123", 0).Return([]*domain.TestRun{}, nil).Once()
			testRunRepo.On("CountByProjectID", mock.Anything, "project-123").Return(int64(42), nil).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs/count?project_id=project-123", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["total"]).To(BeNumerically("==", 42))

			testRunRepo.AssertExpectations(GinkgoT())
		})

		It("should count test runs without project ID", func() {
			// When no project ID is provided, the service returns 0
			// This matches the current implementation in test_run_service.go
			req := httptest.NewRequest("GET", "/api/v1/test-runs/count", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["total"]).To(BeNumerically("==", 0))
		})

		It("should return internal server error when service fails", func() {
			testRunRepo.On("GetLatestByProjectID", mock.Anything, "project-123", 0).Return(nil, errors.New("database error")).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs/count?project_id=project-123", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
			testRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("updateTestRunStatus", func() {
		It("should update status successfully", func() {
			existingRun := &domain.TestRun{
				ID:        1,
				RunID:     "run-123",
				ProjectID: "project-123",
				Status:    "running",
			}
			updatedRun := &domain.TestRun{
				ID:        1,
				RunID:     "run-123",
				ProjectID: "project-123",
				Status:    "completed",
			}

			testRunRepo.On("GetByRunID", mock.Anything, "run-123").Return(existingRun, nil)
			testRunRepo.On("GetByID", mock.Anything, uint(1)).Return(existingRun, nil).Once()
			suiteRunRepo.On("FindByTestRunID", mock.Anything, uint(1)).Return([]*domain.SuiteRun{}, nil)
			testRunRepo.On("Update", mock.Anything, mock.Anything).Return(nil)
			testRunRepo.On("GetByID", mock.Anything, uint(1)).Return(updatedRun, nil).Once()

			requestBody := map[string]interface{}{
				"status": "completed",
			}
			jsonBody, _ := json.Marshal(requestBody)

			req := httptest.NewRequest("PUT", "/api/v1/admin/test-runs/run-123/status", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should return not found when run ID does not exist", func() {
			testRunRepo.On("GetByRunID", mock.Anything, "nonexistent").Return(nil, errors.New("not found"))

			requestBody := map[string]interface{}{
				"status": "completed",
			}
			jsonBody, _ := json.Marshal(requestBody)

			req := httptest.NewRequest("PUT", "/api/v1/admin/test-runs/nonexistent/status", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})

		It("should return bad request for missing status", func() {
			requestBody := map[string]interface{}{}
			jsonBody, _ := json.Marshal(requestBody)

			req := httptest.NewRequest("PUT", "/api/v1/admin/test-runs/run-123/status", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("deleteTestRun", func() {
		It("should delete test run successfully", func() {
			existingRun := &domain.TestRun{
				ID:        1,
				RunID:     "run-123",
				ProjectID: "project-123",
			}
			testRunRepo.On("GetByID", mock.Anything, uint(1)).Return(existingRun, nil)
			testRunRepo.On("Delete", mock.Anything, uint(1)).Return(nil)

			req := httptest.NewRequest("DELETE", "/api/v1/admin/test-runs/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNoContent))
		})

		It("should return not found when test run does not exist", func() {
			testRunRepo.On("GetByID", mock.Anything, uint(999)).Return(nil, errors.New("not found"))

			req := httptest.NewRequest("DELETE", "/api/v1/admin/test-runs/999", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})

		It("should return bad request for invalid ID", func() {
			req := httptest.NewRequest("DELETE", "/api/v1/admin/test-runs/invalid", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("getTestRunStats", func() {
		It("should get test run stats successfully", func() {
			summary := &domain.TestRunSummary{
				TotalRuns:      100,
				PassedRuns:     80,
				FailedRuns:     20,
				AverageRunTime: 30 * time.Second,
				SuccessRate:    80.0,
			}

			testRunRepo.On("GetTestRunSummary", mock.Anything, "project-123").Return(summary, nil).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs/stats?project_id=project-123", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["total"]).To(BeNumerically("==", 100))
			Expect(response["passed"]).To(BeNumerically("==", 80))
			Expect(response["failed"]).To(BeNumerically("==", 20))
			Expect(response["avgDuration"]).To(BeNumerically("==", 30))
			Expect(response["successRate"]).To(BeNumerically("==", 80.0))

			testRunRepo.AssertExpectations(GinkgoT())
		})

		It("should get test run stats with custom days parameter", func() {
			summary := &domain.TestRunSummary{
				TotalRuns:      50,
				PassedRuns:     45,
				FailedRuns:     5,
				AverageRunTime: 25 * time.Second,
				SuccessRate:    90.0,
			}

			testRunRepo.On("GetTestRunSummary", mock.Anything, "project-123").Return(summary, nil).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs/stats?project_id=project-123&days=7", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["days"]).To(BeNumerically("==", 7))

			testRunRepo.AssertExpectations(GinkgoT())
		})

		It("should return internal server error when service fails", func() {
			testRunRepo.On("GetTestRunSummary", mock.Anything, "project-123").Return(nil, errors.New("database error")).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs/stats?project_id=project-123", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
			testRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("getRecentTestRuns", func() {
		It("should get recent test runs successfully", func() {
			testRuns := []*domain.TestRun{
				{ID: 1, ProjectID: "project-123", Status: "passed"},
				{ID: 2, ProjectID: "project-123", Status: "failed"},
			}

			testRunRepo.On("GetLatestByProjectID", mock.Anything, "project-123", 10).Return(testRuns, nil).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs/recent?project_id=project-123", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response []map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(response)).To(Equal(2))

			testRunRepo.AssertExpectations(GinkgoT())
		})

		It("should get recent test runs with custom limit", func() {
			testRuns := []*domain.TestRun{
				{ID: 1, ProjectID: "project-123", Status: "passed"},
			}

			testRunRepo.On("GetLatestByProjectID", mock.Anything, "project-123", 5).Return(testRuns, nil).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs/recent?project_id=project-123&limit=5", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			testRunRepo.AssertExpectations(GinkgoT())
		})

		It("should return bad request for invalid limit", func() {
			req := httptest.NewRequest("GET", "/api/v1/test-runs/recent?limit=0", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return internal server error when service fails", func() {
			testRunRepo.On("GetLatestByProjectID", mock.Anything, "project-123", 10).Return(nil, errors.New("database error")).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs/recent?project_id=project-123", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
			testRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("assignTagsToTestRun", func() {
		It("should assign tags successfully", func() {
			requestBody := map[string]interface{}{
				"tags": []string{"tag1", "tag2", "tag3"},
			}
			jsonBody, _ := json.Marshal(requestBody)

			req := httptest.NewRequest("POST", "/api/v1/test-runs/1/tags", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["message"]).To(Equal("Tags assigned successfully"))
			Expect(response["tags"]).To(Equal([]interface{}{"tag1", "tag2", "tag3"}))
		})

		It("should return bad request for invalid ID", func() {
			requestBody := map[string]interface{}{
				"tags": []string{"tag1"},
			}
			jsonBody, _ := json.Marshal(requestBody)

			req := httptest.NewRequest("POST", "/api/v1/test-runs/invalid/tags", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return bad request for missing tags", func() {
			requestBody := map[string]interface{}{}
			jsonBody, _ := json.Marshal(requestBody)

			req := httptest.NewRequest("POST", "/api/v1/test-runs/1/tags", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("bulkDeleteTestRuns", func() {
		It("should bulk delete test runs successfully", func() {
			for _, id := range []uint{1, 2} {
				existingRun := &domain.TestRun{ID: id, ProjectID: "project-123"}
				testRunRepo.On("GetByID", mock.Anything, id).Return(existingRun, nil)
				testRunRepo.On("Delete", mock.Anything, id).Return(nil)
			}

			requestBody := map[string]interface{}{
				"ids": []uint{1, 2},
			}
			jsonBody, _ := json.Marshal(requestBody)

			req := httptest.NewRequest("POST", "/api/v1/admin/test-runs/bulk-delete", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["deleted"]).To(BeEquivalentTo(2))
		})

		It("should return bad request for missing body", func() {
			req := httptest.NewRequest("POST", "/api/v1/admin/test-runs/bulk-delete", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("convertTestRunToAPI", func() {
		It("should convert test run to API format correctly", func() {
			now := time.Now()
			endTime := now.Add(5 * time.Minute)
			testRun := &domain.TestRun{
				ID:           1,
				ProjectID:    "project-123",
				RunID:        "run-123",
				Name:         "Test Run",
				Branch:       "main",
				GitBranch:    "feature/test",
				GitCommit:    "abc123",
				Status:       "passed",
				StartTime:    now,
				EndTime:      &endTime,
				TotalTests:   10,
				PassedTests:  9,
				FailedTests:  1,
				SkippedTests: 0,
				Duration:     5 * time.Minute,
				Environment:  "test",
				Tags: []domain.Tag{
					{ID: 1, Name: "smoke", Category: "", Value: "smoke"},
					{ID: 2, Name: "priority:high", Category: "priority", Value: "high"},
				},
				Metadata: map[string]interface{}{"key": "value"},
			}

			result := convertTestRunToAPI(testRun)

			Expect(result["id"]).To(Equal(uint(1)))
			Expect(result["projectId"]).To(Equal("project-123"))
			Expect(result["runId"]).To(Equal("run-123"))
			Expect(result["name"]).To(Equal("Test Run"))
			Expect(result["branch"]).To(Equal("main"))
			Expect(result["gitBranch"]).To(Equal("feature/test"))
			Expect(result["gitCommit"]).To(Equal("abc123"))
			Expect(result["status"]).To(Equal("passed"))
			Expect(result["totalTests"]).To(Equal(10))
			Expect(result["passedTests"]).To(Equal(9))
			Expect(result["failedTests"]).To(Equal(1))
			Expect(result["skippedTests"]).To(Equal(0))
			Expect(result["duration"]).To(Equal(int64(300000)))
			Expect(result["environment"]).To(Equal("test"))
			Expect(result["tags"]).To(HaveLen(2))
		})

		It("should handle zero times and empty fields", func() {
			testRun := &domain.TestRun{
				ID:        1,
				ProjectID: "project-123",
				Status:    "running",
				// StartTime will be zero value since it's not a pointer
			}

			result := convertTestRunToAPI(testRun)

			Expect(result["id"]).To(Equal(uint(1)))
			Expect(result["projectId"]).To(Equal("project-123"))
			Expect(result["runId"]).To(Equal(""))
			Expect(result["status"]).To(Equal("running"))
			// StartTime will be the zero value of time.Time
			Expect(result["startTime"]).NotTo(BeNil())
			Expect(result["endTime"]).To(BeNil())
			Expect(result["duration"]).To(Equal(int64(0)))
		})

		It("should handle test run with no tags", func() {
			testRun := &domain.TestRun{
				ID:        1,
				ProjectID: "project-123",
				Status:    "running",
				Tags:      nil,
			}

			result := convertTestRunToAPI(testRun)

			Expect(result["id"]).To(Equal(uint(1)))
			Expect(result["tags"]).To(BeNil())
		})

		It("should handle test run with empty tags array", func() {
			testRun := &domain.TestRun{
				ID:        1,
				ProjectID: "project-123",
				Status:    "running",
				Tags:      []domain.Tag{},
			}

			result := convertTestRunToAPI(testRun)

			Expect(result["id"]).To(Equal(uint(1)))
			Expect(result["tags"]).To(HaveLen(0))
		})
	})

	Describe("RegisterRoutes", func() {
		It("should register all routes correctly", func() {
			routes := router.Routes()

			// Check user routes exist
			userRoutes := []string{
				"GET /api/v1/test-runs",
				"GET /api/v1/test-runs/count",
				"GET /api/v1/test-runs/:id",
				"GET /api/v1/test-runs/by-run-id/:runId",
				"GET /api/v1/test-runs/stats",
				"GET /api/v1/test-runs/recent",
				"POST /api/v1/test-runs/:id/tags",
				"GET /api/v1/test-runs/:id/suite-runs",
				"GET /api/v1/test-runs/:id/suite-runs/:suiteId",
				"GET /api/v1/test-runs/:id/suite-runs/:suiteId/spec-runs",
				"GET /api/v1/test-runs/:id/suite-runs/:suiteId/spec-runs/:specId",
			}

			// Check admin routes exist
			adminRoutes := []string{
				"POST /api/v1/admin/test-runs",
				"PUT /api/v1/admin/test-runs/:runId/status",
				"DELETE /api/v1/admin/test-runs/:id",
				"POST /api/v1/admin/test-runs/bulk-delete",
			}

			allExpectedRoutes := append(userRoutes, adminRoutes...)

			for _, expectedRoute := range allExpectedRoutes {
				found := false
				for _, route := range routes {
					if fmt.Sprintf("%s %s", route.Method, route.Path) == expectedRoute {
						found = true
						break
					}
				}
				Expect(found).To(BeTrue(), fmt.Sprintf("Route %s not found", expectedRoute))
			}
		})
	})

	Describe("getTestRunByRunID", func() {
		It("should handle error when service fails", func() {
			testRunRepo.On("GetByRunID", mock.Anything, "nonexistent").Return(nil, errors.New("not found"))

			req := httptest.NewRequest("GET", "/api/v1/test-runs/by-run-id/nonexistent", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})

	Describe("updateTestRunStatus", func() {
		It("should handle missing status field", func() {
			requestBody := map[string]interface{}{}
			jsonBody, _ := json.Marshal(requestBody)

			req := httptest.NewRequest("PUT", "/api/v1/admin/test-runs/run-123/status", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("bulkDeleteTestRuns", func() {
		It("should handle empty IDs list", func() {
			requestBody := map[string]interface{}{
				"ids": []uint{},
			}
			jsonBody, _ := json.Marshal(requestBody)

			req := httptest.NewRequest("POST", "/api/v1/admin/test-runs/bulk-delete", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("SetTagService", func() {
		It("should set tag service", func() {
			mockTagService := &tagsApp.TagService{}
			handler.SetTagService(mockTagService)
			Expect(handler.tagService).To(Equal(mockTagService))
		})
	})

	Describe("getSuiteRuns", func() {
		It("should get suite runs successfully", func() {
			suiteRuns := []*domain.SuiteRun{
				{ID: 1, TestRunID: 1, Name: "Suite 1"},
				{ID: 2, TestRunID: 1, Name: "Suite 2"},
			}
			suiteRunRepo.On("FindByTestRunID", mock.Anything, uint(1)).Return(suiteRuns, nil).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs/1/suite-runs", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should return bad request for invalid test run ID", func() {
			req := httptest.NewRequest("GET", "/api/v1/test-runs/invalid/suite-runs", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return internal server error when service fails", func() {
			suiteRunRepo.On("FindByTestRunID", mock.Anything, uint(1)).Return(nil, errors.New("db error")).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs/1/suite-runs", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Describe("getSuiteRun", func() {
		It("should get suite run successfully when parent matches", func() {
			suiteRun := &domain.SuiteRun{ID: 2, TestRunID: 1, Name: "Suite 1"}
			suiteRunRepo.On("GetByID", mock.Anything, uint(2)).Return(suiteRun, nil).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs/1/suite-runs/2", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should return bad request for invalid test run ID", func() {
			req := httptest.NewRequest("GET", "/api/v1/test-runs/invalid/suite-runs/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return bad request for invalid suite run ID", func() {
			req := httptest.NewRequest("GET", "/api/v1/test-runs/1/suite-runs/invalid", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return not found when suite run doesn't exist", func() {
			suiteRunRepo.On("GetByID", mock.Anything, uint(1)).Return(nil, domain.ErrNotFound).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs/1/suite-runs/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})

		It("should return not found when suite belongs to different test run", func() {
			suiteRun := &domain.SuiteRun{ID: 1, TestRunID: 2, Name: "Suite 1"}
			suiteRunRepo.On("GetByID", mock.Anything, uint(1)).Return(suiteRun, nil).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs/1/suite-runs/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})

		It("should return internal server error when repository fails unexpectedly", func() {
			suiteRunRepo.On("GetByID", mock.Anything, uint(1)).Return(nil, errors.New("database connection failed")).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs/1/suite-runs/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Describe("getSpecRuns", func() {
		It("should get spec runs successfully when parent matches", func() {
			suiteRun := &domain.SuiteRun{ID: 2, TestRunID: 1, Name: "Suite 1"}
			suiteRunRepo.On("GetByID", mock.Anything, uint(2)).Return(suiteRun, nil).Once()
			specRuns := []*domain.SpecRun{
				{ID: 3, SuiteRunID: 2, Name: "Spec 1"},
				{ID: 4, SuiteRunID: 2, Name: "Spec 2"},
			}
			specRunRepo.On("FindBySuiteRunID", mock.Anything, uint(2)).Return(specRuns, nil).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs/1/suite-runs/2/spec-runs", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should return bad request for invalid test run ID", func() {
			req := httptest.NewRequest("GET", "/api/v1/test-runs/invalid/suite-runs/1/spec-runs", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return bad request for invalid suite run ID", func() {
			req := httptest.NewRequest("GET", "/api/v1/test-runs/1/suite-runs/invalid/spec-runs", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return not found when suite belongs to different test run", func() {
			suiteRun := &domain.SuiteRun{ID: 1, TestRunID: 2, Name: "Suite 1"}
			suiteRunRepo.On("GetByID", mock.Anything, uint(1)).Return(suiteRun, nil).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs/1/suite-runs/1/spec-runs", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})

		It("should return empty list when suite is valid but has no spec runs", func() {
			suiteRun := &domain.SuiteRun{ID: 1, TestRunID: 1, Name: "Suite 1"}
			suiteRunRepo.On("GetByID", mock.Anything, uint(1)).Return(suiteRun, nil).Once()
			specRunRepo.On("FindBySuiteRunID", mock.Anything, uint(1)).Return([]*domain.SpecRun{}, nil).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs/1/suite-runs/1/spec-runs", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			var response []*domain.SpecRun
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response).To(HaveLen(0))
		})

		It("should return internal server error when suite validation fails", func() {
			suiteRunRepo.On("GetByID", mock.Anything, uint(1)).Return(nil, errors.New("db error")).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs/1/suite-runs/1/spec-runs", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})

		It("should return internal server error when spec list lookup fails", func() {
			suiteRun := &domain.SuiteRun{ID: 1, TestRunID: 1, Name: "Suite 1"}
			suiteRunRepo.On("GetByID", mock.Anything, uint(1)).Return(suiteRun, nil).Once()
			specRunRepo.On("FindBySuiteRunID", mock.Anything, uint(1)).Return(nil, errors.New("db error")).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs/1/suite-runs/1/spec-runs", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Describe("getSpecRun", func() {
		It("should get spec run successfully when full parent chain matches", func() {
			specRun := &domain.SpecRun{ID: 3, SuiteRunID: 2, Name: "Spec 1"}
			specRunRepo.On("GetByID", mock.Anything, uint(3)).Return(specRun, nil).Once()
			suiteRun := &domain.SuiteRun{ID: 2, TestRunID: 1, Name: "Suite 1"}
			suiteRunRepo.On("GetByID", mock.Anything, uint(2)).Return(suiteRun, nil).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs/1/suite-runs/2/spec-runs/3", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should return bad request for invalid test run ID", func() {
			req := httptest.NewRequest("GET", "/api/v1/test-runs/invalid/suite-runs/1/spec-runs/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return bad request for invalid suite run ID", func() {
			req := httptest.NewRequest("GET", "/api/v1/test-runs/1/suite-runs/invalid/spec-runs/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return bad request for invalid spec run ID", func() {
			req := httptest.NewRequest("GET", "/api/v1/test-runs/1/suite-runs/1/spec-runs/invalid", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("should return not found when spec run doesn't exist", func() {
			specRunRepo.On("GetByID", mock.Anything, uint(1)).Return(nil, domain.ErrNotFound).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs/1/suite-runs/1/spec-runs/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})

		It("should return not found when spec belongs to different suite", func() {
			specRun := &domain.SpecRun{ID: 1, SuiteRunID: 2, Name: "Spec 1"}
			specRunRepo.On("GetByID", mock.Anything, uint(1)).Return(specRun, nil).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs/1/suite-runs/1/spec-runs/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})

		It("should return not found when parent suite belongs to different test run", func() {
			specRun := &domain.SpecRun{ID: 1, SuiteRunID: 1, Name: "Spec 1"}
			specRunRepo.On("GetByID", mock.Anything, uint(1)).Return(specRun, nil).Once()
			suiteRun := &domain.SuiteRun{ID: 1, TestRunID: 2, Name: "Suite 1"}
			suiteRunRepo.On("GetByID", mock.Anything, uint(1)).Return(suiteRun, nil).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs/1/suite-runs/1/spec-runs/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusNotFound))
		})

		It("should return internal server error when validation chain fails unexpectedly", func() {
			specRun := &domain.SpecRun{ID: 1, SuiteRunID: 1, Name: "Spec 1"}
			specRunRepo.On("GetByID", mock.Anything, uint(1)).Return(specRun, nil).Once()
			suiteRunRepo.On("GetByID", mock.Anything, uint(1)).Return(nil, errors.New("db error")).Once()

			req := httptest.NewRequest("GET", "/api/v1/test-runs/1/suite-runs/1/spec-runs/1", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	Describe("Public API Endpoints", func() {
		var publicRouter *gin.Engine
		var publicGroup *gin.RouterGroup

		BeforeEach(func() {
			publicRouter = gin.New()
			publicGroup = publicRouter.Group("/api/v1")
			handler.RegisterSubmissionRoutes(publicGroup)
		})

		Describe("recordTestRun", func() {
			It("should record a test run successfully", func() {
				req := TestRunRequest{
					TestProjectID: "project-123",
					GitBranch:     "main",
					GitSha:        "abc123",
					Environment:   "test",
					Tags:          []Tag{},
					SuiteRuns: []SuiteRun{
						{
							SuiteName: "Suite 1",
							SpecRuns: []SpecRun{
								{SpecDescription: "Spec 1", Status: "passed"},
							},
						},
					},
				}
				jsonBody, _ := json.Marshal(req)

				testRunRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
				testRunRepo.On("GetByRunID", mock.Anything, mock.Anything).Return(nil, errors.New("not found"))
				suiteRunRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
				specRunRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

				httpReq := httptest.NewRequest("POST", "/api/v1/test-runs", bytes.NewBuffer(jsonBody))
				httpReq.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				publicRouter.ServeHTTP(w, httpReq)

				Expect(w.Code).To(Equal(http.StatusCreated))
			})

			It("should return bad request for empty body", func() {
				httpReq := httptest.NewRequest("POST", "/api/v1/test-runs", nil)
				httpReq.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				publicRouter.ServeHTTP(w, httpReq)

				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})
		})

		Describe("startTestRun", func() {
			It("should start a test run successfully", func() {
				req := map[string]interface{}{
					"projectId": "project-123",
					"branch":    "main",
				}
				jsonBody, _ := json.Marshal(req)

				testRunRepo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()

				httpReq := httptest.NewRequest("POST", "/api/v1/test-runs/start", bytes.NewBuffer(jsonBody))
				httpReq.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				publicRouter.ServeHTTP(w, httpReq)

				Expect(w.Code).To(Equal(http.StatusCreated))

				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				Expect(err).NotTo(HaveOccurred())
				Expect(response).To(HaveKey("id"))
				Expect(response).To(HaveKey("runId"))
			})

			It("should return bad request for missing projectId", func() {
				req := map[string]interface{}{
					"branch": "main",
				}
				jsonBody, _ := json.Marshal(req)

				httpReq := httptest.NewRequest("POST", "/api/v1/test-runs/start", bytes.NewBuffer(jsonBody))
				httpReq.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				publicRouter.ServeHTTP(w, httpReq)

				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})

			It("should generate runId if not provided", func() {
				req := map[string]interface{}{
					"projectId": "project-123",
				}
				jsonBody, _ := json.Marshal(req)

				testRunRepo.On("Create", mock.Anything, mock.MatchedBy(func(tr *domain.TestRun) bool {
					return tr.RunID != ""
				})).Return(nil).Once()

				httpReq := httptest.NewRequest("POST", "/api/v1/test-runs/start", bytes.NewBuffer(jsonBody))
				httpReq.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				publicRouter.ServeHTTP(w, httpReq)

				Expect(w.Code).To(Equal(http.StatusCreated))
			})
		})

		Describe("completeTestRun", func() {
			It("should return not found for nonexistent run", func() {
				req := map[string]interface{}{
					"runId":  "nonexistent",
					"status": "passed",
				}
				jsonBody, _ := json.Marshal(req)

				testRunRepo.On("GetByRunID", mock.Anything, "nonexistent").Return(nil, errors.New("not found")).Once()

				httpReq := httptest.NewRequest("POST", "/api/v1/test-runs/complete", bytes.NewBuffer(jsonBody))
				httpReq.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				publicRouter.ServeHTTP(w, httpReq)

				Expect(w.Code).To(Equal(http.StatusNotFound))
			})

			It("should return bad request for missing runId", func() {
				req := map[string]interface{}{
					"status": "passed",
				}
				jsonBody, _ := json.Marshal(req)

				httpReq := httptest.NewRequest("POST", "/api/v1/test-runs/complete", bytes.NewBuffer(jsonBody))
				httpReq.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				publicRouter.ServeHTTP(w, httpReq)

				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})
		})

		Describe("addSuiteRun", func() {
			It("should add a suite run successfully", func() {
				req := map[string]interface{}{
					"testRunId": "run-123",
					"suiteName": "Suite 1",
					"status":    "passed",
				}
				jsonBody, _ := json.Marshal(req)

				testRunRepo.On("GetByRunID", mock.Anything, "run-123").Return(&domain.TestRun{
					ID:    1,
					RunID: "run-123",
				}, nil).Once()
				suiteRunRepo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()

				httpReq := httptest.NewRequest("POST", "/api/v1/suite-runs", bytes.NewBuffer(jsonBody))
				httpReq.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				publicRouter.ServeHTTP(w, httpReq)

				Expect(w.Code).To(Equal(http.StatusCreated))

				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				Expect(err).NotTo(HaveOccurred())
				Expect(response).To(HaveKey("id"))
				Expect(response).To(HaveKey("suiteName"))
			})

			It("should return not found for nonexistent test run", func() {
				req := map[string]interface{}{
					"testRunId": "nonexistent",
					"suiteName": "Suite 1",
				}
				jsonBody, _ := json.Marshal(req)

				testRunRepo.On("GetByRunID", mock.Anything, "nonexistent").Return(nil, errors.New("not found")).Once()

				httpReq := httptest.NewRequest("POST", "/api/v1/suite-runs", bytes.NewBuffer(jsonBody))
				httpReq.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				publicRouter.ServeHTTP(w, httpReq)

				Expect(w.Code).To(Equal(http.StatusNotFound))
			})

			It("should return bad request for missing required fields", func() {
				req := map[string]interface{}{
					"testRunId": "run-123",
				}
				jsonBody, _ := json.Marshal(req)

				httpReq := httptest.NewRequest("POST", "/api/v1/suite-runs", bytes.NewBuffer(jsonBody))
				httpReq.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				publicRouter.ServeHTTP(w, httpReq)

				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})
		})

		Describe("addSpecRun", func() {
			It("should return bad request for missing required fields", func() {
				req := map[string]interface{}{
					"specName": "Spec 1",
				}
				jsonBody, _ := json.Marshal(req)

				httpReq := httptest.NewRequest("POST", "/api/v1/spec-runs", bytes.NewBuffer(jsonBody))
				httpReq.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				publicRouter.ServeHTTP(w, httpReq)

				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})
		})

		Describe("updateTestRunPublic", func() {
			It("should return not implemented", func() {
				req := map[string]interface{}{
					"status": "passed",
				}
				jsonBody, _ := json.Marshal(req)

				httpReq := httptest.NewRequest("PUT", "/api/v1/test-runs/1", bytes.NewBuffer(jsonBody))
				httpReq.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				publicRouter.ServeHTTP(w, httpReq)

				Expect(w.Code).To(Equal(http.StatusNotImplemented))
			})
		})

		Describe("RegisterPublicRoutes", func() {
			It("should register all public routes correctly", func() {
				routes := publicRouter.Routes()

				publicRoutes := []string{
					"POST /api/v1/test-runs",
					"POST /api/v1/test-runs/start",
					"POST /api/v1/test-runs/complete",
					"POST /api/v1/suite-runs",
					"POST /api/v1/spec-runs",
					"PUT /api/v1/test-runs/:id",
				}

				for _, expectedRoute := range publicRoutes {
					found := false
					for _, route := range routes {
						if fmt.Sprintf("%s %s", route.Method, route.Path) == expectedRoute {
							found = true
							break
						}
					}
					Expect(found).To(BeTrue(), fmt.Sprintf("Route %s not found", expectedRoute))
				}
			})
		})
	})
})
