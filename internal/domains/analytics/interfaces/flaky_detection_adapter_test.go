package interfaces_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	"github.com/guidewire-oss/fern-platform/internal/domains/analytics/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/analytics/domain"
	"github.com/guidewire-oss/fern-platform/internal/domains/analytics/interfaces"
	"github.com/guidewire-oss/fern-platform/pkg/config"
	"github.com/guidewire-oss/fern-platform/pkg/logging"
)

func TestAnalyticsInterfaces(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Analytics Interfaces Suite")
}

// MockFlakyDetectionRepository mocks the FlakyDetectionRepository interface
type MockFlakyDetectionRepository struct {
	mock.Mock
}

func (m *MockFlakyDetectionRepository) SaveFlakyTest(ctx context.Context, flaky *domain.FlakyTest) error {
	args := m.Called(ctx, flaky)
	return args.Error(0)
}

func (m *MockFlakyDetectionRepository) GetFlakyTest(ctx context.Context, testID string) (*domain.FlakyTest, error) {
	args := m.Called(ctx, testID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.FlakyTest), args.Error(1)
}

func (m *MockFlakyDetectionRepository) FindFlakyTestsByProject(ctx context.Context, projectID string, status domain.FlakyTestStatus) ([]*domain.FlakyTest, error) {
	args := m.Called(ctx, projectID, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.FlakyTest), args.Error(1)
}

func (m *MockFlakyDetectionRepository) UpdateFlakyTestStatus(ctx context.Context, testID string, status domain.FlakyTestStatus) error {
	args := m.Called(ctx, testID, status)
	return args.Error(0)
}

func (m *MockFlakyDetectionRepository) SaveTestRunAnalysis(ctx context.Context, analysis *domain.TestRunAnalysis) error {
	args := m.Called(ctx, analysis)
	return args.Error(0)
}

func (m *MockFlakyDetectionRepository) GetTestRunHistory(ctx context.Context, projectID string, testName string, since time.Time) ([]domain.TestExecutionResult, error) {
	args := m.Called(ctx, projectID, testName, since)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.TestExecutionResult), args.Error(1)
}

func (m *MockFlakyDetectionRepository) GetUniqueTestNames(ctx context.Context, projectID string, since time.Time) ([]string, error) {
	args := m.Called(ctx, projectID, since)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func newTestLogger() *logging.Logger {
	logger, _ := logging.NewLogger(&config.LoggingConfig{
		Level: "debug",
	})
	return logger
}

var _ = Describe("FlakyDetectionAdapter", func() {
	var (
		adapter  *interfaces.FlakyDetectionAdapter
		mockRepo *MockFlakyDetectionRepository
		logger   *logging.Logger
		router   *gin.Engine
	)

	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
		mockRepo = new(MockFlakyDetectionRepository)
		logger = newTestLogger()
		cfg := domain.DefaultFlakyTestDetectionConfig()
		service := application.NewFlakyDetectionService(mockRepo, cfg)
		adapter = interfaces.NewFlakyDetectionAdapter(service, logger)

		router = gin.New()
		api := router.Group("/api/v1")
		adapter.RegisterRoutes(api)
	})

	Describe("NewFlakyDetectionAdapter", func() {
		It("should create adapter with service and logger", func() {
			Expect(adapter).NotTo(BeNil())
		})
	})

	Describe("GetFlakyTests", func() {
		It("should return flaky tests for a project", func() {
			flakyTests := []*domain.FlakyTest{
				{TestID: "t1", ProjectID: "proj1", TestName: "test1", Status: domain.StatusActive},
				{TestID: "t2", ProjectID: "proj1", TestName: "test2", Status: domain.StatusActive},
			}
			mockRepo.On("FindFlakyTestsByProject", mock.Anything, "proj1", domain.StatusActive).
				Return(flakyTests, nil)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/v1/projects/proj1/flaky-tests", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(200))
			Expect(w.Body.String()).To(ContainSubstring("test1"))
			Expect(w.Body.String()).To(ContainSubstring("test2"))
			Expect(w.Body.String()).To(ContainSubstring(`"total":2`))
		})

		It("should return 400 when project ID is empty", func() {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/v1/projects//flaky-tests", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(400))
			Expect(w.Body.String()).To(ContainSubstring("project ID is required"))
		})

		It("should return 500 when service returns error", func() {
			mockRepo.On("FindFlakyTestsByProject", mock.Anything, "proj1", domain.StatusActive).
				Return(nil, fmt.Errorf("database error"))

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/v1/projects/proj1/flaky-tests", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(500))
			Expect(w.Body.String()).To(ContainSubstring("Failed to get flaky tests"))
		})
	})

	Describe("MarkTestResolved", func() {
		It("should resolve a test successfully", func() {
			mockRepo.On("UpdateFlakyTestStatus", mock.Anything, "test-123", domain.StatusResolved).
				Return(nil)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("PUT", "/api/v1/flaky-tests/test-123/resolve", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(200))
			Expect(w.Body.String()).To(ContainSubstring("Test marked as resolved"))
			mockRepo.AssertExpectations(GinkgoT())
		})

		It("should return 500 when resolve fails", func() {
			mockRepo.On("UpdateFlakyTestStatus", mock.Anything, "test-bad", domain.StatusResolved).
				Return(fmt.Errorf("not found"))

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("PUT", "/api/v1/flaky-tests/test-bad/resolve", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(500))
			Expect(w.Body.String()).To(ContainSubstring("Failed to mark test as resolved"))
		})
	})

	Describe("IgnoreTest", func() {
		It("should ignore a test successfully", func() {
			mockRepo.On("UpdateFlakyTestStatus", mock.Anything, "test-456", domain.StatusIgnored).
				Return(nil)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("PUT", "/api/v1/flaky-tests/test-456/ignore", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(200))
			Expect(w.Body.String()).To(ContainSubstring("Test marked as ignored"))
			mockRepo.AssertExpectations(GinkgoT())
		})

		It("should return 500 when ignore fails", func() {
			mockRepo.On("UpdateFlakyTestStatus", mock.Anything, "test-bad", domain.StatusIgnored).
				Return(fmt.Errorf("db error"))

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("PUT", "/api/v1/flaky-tests/test-bad/ignore", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(500))
			Expect(w.Body.String()).To(ContainSubstring("Failed to ignore test"))
		})
	})

	Describe("AnalyzeTestRun", func() {
		It("should analyze a test run successfully", func() {
			mockRepo.On("GetUniqueTestNames", mock.Anything, "proj1", mock.AnythingOfType("time.Time")).
				Return([]string{}, nil)
			mockRepo.On("SaveTestRunAnalysis", mock.Anything, mock.AnythingOfType("*domain.TestRunAnalysis")).
				Return(nil)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/projects/proj1/test-runs/run1/analyze", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(200))
			Expect(w.Body.String()).To(ContainSubstring("analysis"))
		})

		It("should return 500 when analysis fails", func() {
			mockRepo.On("GetUniqueTestNames", mock.Anything, "proj1", mock.AnythingOfType("time.Time")).
				Return(nil, fmt.Errorf("db error"))

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/api/v1/projects/proj1/test-runs/run1/analyze", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(500))
			Expect(w.Body.String()).To(ContainSubstring("Failed to analyze test run"))
		})
	})

	Describe("GetFlakyTestTrends", func() {
		It("should return trends with default period", func() {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/v1/projects/proj1/flaky-tests/trends", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(200))
			Expect(w.Body.String()).To(ContainSubstring("trends"))
			Expect(w.Body.String()).To(ContainSubstring(`"period":"30d"`))
		})

		It("should accept custom period parameter", func() {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/v1/projects/proj1/flaky-tests/trends?period=7d", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(200))
			Expect(w.Body.String()).To(ContainSubstring(`"period":"7d"`))
		})

		It("should return 400 for invalid period format", func() {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/api/v1/projects/proj1/flaky-tests/trends?period=bad", nil)
			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(400))
			Expect(w.Body.String()).To(ContainSubstring("Invalid period format"))
		})
	})

	Describe("ResolveFlakyTests", func() {
		It("should delegate to service GetFlakyTests", func() {
			expected := []*domain.FlakyTest{
				{TestID: "t1", ProjectID: "proj1"},
			}
			mockRepo.On("FindFlakyTestsByProject", mock.Anything, "proj1", domain.StatusActive).
				Return(expected, nil)

			result, err := adapter.ResolveFlakyTests(context.Background(), "proj1")

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(1))
			Expect(result[0].TestID).To(Equal("t1"))
		})
	})

	Describe("ResolveTestFlakeScore", func() {
		It("should return 0.0 for any test", func() {
			score, err := adapter.ResolveTestFlakeScore(context.Background(), "proj1", "test1")

			Expect(err).NotTo(HaveOccurred())
			Expect(score).To(Equal(0.0))
		})
	})

	Describe("RegisterRoutes", func() {
		It("should register all expected routes", func() {
			testRouter := gin.New()
			group := testRouter.Group("/api/v1")
			adapter.RegisterRoutes(group)

			routes := testRouter.Routes()
			routePaths := make(map[string]string)
			for _, r := range routes {
				routePaths[r.Method+":"+r.Path] = r.Path
			}

			Expect(routePaths).To(HaveKey("POST:/api/v1/projects/:projectId/test-runs/:testRunId/analyze"))
			Expect(routePaths).To(HaveKey("GET:/api/v1/projects/:projectId/flaky-tests"))
			Expect(routePaths).To(HaveKey("GET:/api/v1/projects/:projectId/flaky-tests/trends"))
			Expect(routePaths).To(HaveKey("PUT:/api/v1/flaky-tests/:testId/resolve"))
			Expect(routePaths).To(HaveKey("PUT:/api/v1/flaky-tests/:testId/ignore"))
		})
	})
})
