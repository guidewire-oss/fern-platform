package application_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	"github.com/guidewire-oss/fern-platform/internal/domains/analytics/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/analytics/domain"
)

func TestAnalyticsApplication(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Analytics Application Suite")
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

var _ = Describe("FlakyDetectionService", func() {
	var (
		service  *application.FlakyDetectionService
		mockRepo *MockFlakyDetectionRepository
		ctx      context.Context
		config   domain.FlakyTestDetectionConfig
	)

	BeforeEach(func() {
		mockRepo = new(MockFlakyDetectionRepository)
		ctx = context.Background()
		config = domain.DefaultFlakyTestDetectionConfig()
		service = application.NewFlakyDetectionService(mockRepo, config)
	})

	Describe("NewFlakyDetectionService", func() {
		It("should create a service with the given repo and config", func() {
			Expect(service).NotTo(BeNil())
		})
	})

	Describe("GetFlakyTests", func() {
		It("should return active flaky tests for a project", func() {
			expected := []*domain.FlakyTest{
				{TestID: "proj_test1", ProjectID: "proj", TestName: "test1", Status: domain.StatusActive},
				{TestID: "proj_test2", ProjectID: "proj", TestName: "test2", Status: domain.StatusActive},
			}

			mockRepo.On("FindFlakyTestsByProject", ctx, "proj", domain.StatusActive).
				Return(expected, nil)

			result, err := service.GetFlakyTests(ctx, "proj")

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(HaveLen(2))
			Expect(result[0].TestID).To(Equal("proj_test1"))
			Expect(result[1].TestID).To(Equal("proj_test2"))
			mockRepo.AssertExpectations(GinkgoT())
		})

		It("should return empty list when no flaky tests exist", func() {
			mockRepo.On("FindFlakyTestsByProject", ctx, "proj", domain.StatusActive).
				Return([]*domain.FlakyTest{}, nil)

			result, err := service.GetFlakyTests(ctx, "proj")

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(BeEmpty())
			mockRepo.AssertExpectations(GinkgoT())
		})

		It("should propagate repository errors", func() {
			mockRepo.On("FindFlakyTestsByProject", ctx, "proj", domain.StatusActive).
				Return(nil, fmt.Errorf("database connection failed"))

			result, err := service.GetFlakyTests(ctx, "proj")

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("database connection failed"))
			Expect(result).To(BeNil())
		})
	})

	Describe("MarkTestResolved", func() {
		It("should update test status to resolved", func() {
			mockRepo.On("UpdateFlakyTestStatus", ctx, "test-123", domain.StatusResolved).
				Return(nil)

			err := service.MarkTestResolved(ctx, "test-123")

			Expect(err).NotTo(HaveOccurred())
			mockRepo.AssertExpectations(GinkgoT())
		})

		It("should propagate errors from repository", func() {
			mockRepo.On("UpdateFlakyTestStatus", ctx, "nonexistent", domain.StatusResolved).
				Return(fmt.Errorf("test not found"))

			err := service.MarkTestResolved(ctx, "nonexistent")

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("test not found"))
		})
	})

	Describe("IgnoreTest", func() {
		It("should update test status to ignored", func() {
			mockRepo.On("UpdateFlakyTestStatus", ctx, "test-456", domain.StatusIgnored).
				Return(nil)

			err := service.IgnoreTest(ctx, "test-456")

			Expect(err).NotTo(HaveOccurred())
			mockRepo.AssertExpectations(GinkgoT())
		})

		It("should propagate errors from repository", func() {
			mockRepo.On("UpdateFlakyTestStatus", ctx, "bad-id", domain.StatusIgnored).
				Return(fmt.Errorf("update failed"))

			err := service.IgnoreTest(ctx, "bad-id")

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("update failed"))
		})
	})

	Describe("AnalyzeTestRun", func() {
		It("should return error when getting unique test names fails", func() {
			mockRepo.On("GetUniqueTestNames", ctx, "proj", mock.AnythingOfType("time.Time")).
				Return(nil, fmt.Errorf("db error"))

			result, err := service.AnalyzeTestRun(ctx, "proj", "run-1")

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get test names"))
			Expect(result).To(BeNil())
		})

		It("should analyze test run with no tests", func() {
			mockRepo.On("GetUniqueTestNames", ctx, "proj", mock.AnythingOfType("time.Time")).
				Return([]string{}, nil)
			mockRepo.On("SaveTestRunAnalysis", ctx, mock.AnythingOfType("*domain.TestRunAnalysis")).
				Return(nil)

			result, err := service.AnalyzeTestRun(ctx, "proj", "run-1")

			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.TotalTests).To(Equal(0))
			Expect(result.ProjectID).To(Equal("proj"))
			Expect(result.TestRunID).To(Equal("run-1"))
			mockRepo.AssertExpectations(GinkgoT())
		})

		It("should return error when saving analysis fails", func() {
			mockRepo.On("GetUniqueTestNames", ctx, "proj", mock.AnythingOfType("time.Time")).
				Return([]string{}, nil)
			mockRepo.On("SaveTestRunAnalysis", ctx, mock.AnythingOfType("*domain.TestRunAnalysis")).
				Return(fmt.Errorf("save failed"))

			result, err := service.AnalyzeTestRun(ctx, "proj", "run-1")

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to save analysis"))
			Expect(result).To(BeNil())
		})

		It("should detect new flaky test", func() {
			// Build history: 10 runs, 3 failures => 30% failure rate (within 5%-95%)
			history := make([]domain.TestExecutionResult, 0, 10)
			for i := 0; i < 10; i++ {
				status := "passed"
				errMsg := ""
				if i < 3 {
					status = "failed"
					errMsg = "assertion error"
				}
				history = append(history, domain.TestExecutionResult{
					TestRunID:  fmt.Sprintf("run-%d", i),
					TestName:   "testFlaky",
					SuiteName:  "suite1",
					Status:     status,
					Duration:   time.Second,
					ExecutedAt: time.Now().Add(time.Duration(-i) * time.Hour),
					Error:      errMsg,
				})
			}

			mockRepo.On("GetUniqueTestNames", ctx, "proj", mock.AnythingOfType("time.Time")).
				Return([]string{"testFlaky"}, nil)
			mockRepo.On("GetTestRunHistory", ctx, "proj", "testFlaky", mock.AnythingOfType("time.Time")).
				Return(history, nil)
			mockRepo.On("GetFlakyTest", ctx, "proj_testFlaky").
				Return(nil, fmt.Errorf("flaky test not found"))
			mockRepo.On("SaveFlakyTest", ctx, mock.AnythingOfType("*domain.FlakyTest")).
				Return(nil)
			mockRepo.On("SaveTestRunAnalysis", ctx, mock.AnythingOfType("*domain.TestRunAnalysis")).
				Return(nil)

			result, err := service.AnalyzeTestRun(ctx, "proj", "run-main")

			Expect(err).NotTo(HaveOccurred())
			Expect(result.NewFlaky).To(HaveLen(1))
			Expect(result.NewFlaky[0]).To(Equal("proj_testFlaky"))
			mockRepo.AssertExpectations(GinkgoT())
		})

		It("should skip tests with too few runs", func() {
			// Only 3 runs, less than MinimumRuns (10)
			history := []domain.TestExecutionResult{
				{TestRunID: "r1", TestName: "testNew", Status: "failed", ExecutedAt: time.Now()},
				{TestRunID: "r2", TestName: "testNew", Status: "passed", ExecutedAt: time.Now()},
				{TestRunID: "r3", TestName: "testNew", Status: "failed", ExecutedAt: time.Now()},
			}

			mockRepo.On("GetUniqueTestNames", ctx, "proj", mock.AnythingOfType("time.Time")).
				Return([]string{"testNew"}, nil)
			mockRepo.On("GetTestRunHistory", ctx, "proj", "testNew", mock.AnythingOfType("time.Time")).
				Return(history, nil)
			mockRepo.On("SaveTestRunAnalysis", ctx, mock.AnythingOfType("*domain.TestRunAnalysis")).
				Return(nil)

			result, err := service.AnalyzeTestRun(ctx, "proj", "run-2")

			Expect(err).NotTo(HaveOccurred())
			Expect(result.NewFlaky).To(BeEmpty())
			Expect(result.StillFlaky).To(BeEmpty())
			Expect(result.ResolvedFlaky).To(BeEmpty())
		})

		It("should continue analyzing other tests when one fails", func() {
			mockRepo.On("GetUniqueTestNames", ctx, "proj", mock.AnythingOfType("time.Time")).
				Return([]string{"testBad", "testGood"}, nil)
			// First test fails to get history
			mockRepo.On("GetTestRunHistory", ctx, "proj", "testBad", mock.AnythingOfType("time.Time")).
				Return(nil, fmt.Errorf("history error"))
			// Second test has too few runs so it's actionNone
			mockRepo.On("GetTestRunHistory", ctx, "proj", "testGood", mock.AnythingOfType("time.Time")).
				Return([]domain.TestExecutionResult{{Status: "passed"}}, nil)
			mockRepo.On("SaveTestRunAnalysis", ctx, mock.AnythingOfType("*domain.TestRunAnalysis")).
				Return(nil)

			result, err := service.AnalyzeTestRun(ctx, "proj", "run-3")

			Expect(err).NotTo(HaveOccurred())
			Expect(result.TotalTests).To(Equal(2))
		})
	})

	Describe("GetFlakyTestTrends", func() {
		It("should return empty trends", func() {
			trends, err := service.GetFlakyTestTrends(ctx, "proj", 30*24*time.Hour)

			Expect(err).NotTo(HaveOccurred())
			Expect(trends).To(BeEmpty())
		})
	})
})
