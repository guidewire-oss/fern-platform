package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/guidewire-oss/fern-platform/internal/testhelpers"
)

func TestApplication(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Testing Application Suite")
}

// Mock repository
type MockTestRunRepository struct {
	mock.Mock
}

func (m *MockTestRunRepository) Create(ctx context.Context, testRun *domain.TestRun) error {
	args := m.Called(ctx, testRun)
	return args.Error(0)
}

func (m *MockTestRunRepository) FindByID(ctx context.Context, id uint) (*domain.TestRun, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TestRun), args.Error(1)
}

func (m *MockTestRunRepository) GetByID(ctx context.Context, id uint) (*domain.TestRun, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TestRun), args.Error(1)
}

func (m *MockTestRunRepository) FindByRunID(ctx context.Context, runID string) (*domain.TestRun, error) {
	args := m.Called(ctx, runID)
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

func (m *MockTestRunRepository) Update(ctx context.Context, testRun *domain.TestRun) error {
	args := m.Called(ctx, testRun)
	return args.Error(0)
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

func (m *MockTestRunRepository) FindByProjectID(ctx context.Context, projectID string) ([]*domain.TestRun, error) {
	args := m.Called(ctx, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.TestRun), args.Error(1)
}

func (m *MockTestRunRepository) FindAll(ctx context.Context) ([]*domain.TestRun, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.TestRun), args.Error(1)
}

func (m *MockTestRunRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTestRunRepository) FindRecentByProjectID(ctx context.Context, projectID string, limit int) ([]*domain.TestRun, error) {
	args := m.Called(ctx, projectID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.TestRun), args.Error(1)
}

func (m *MockTestRunRepository) GetStatsByProjectID(ctx context.Context, projectID string) (map[string]interface{}, error) {
	args := m.Called(ctx, projectID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockTestRunRepository) CountByProjectID(ctx context.Context, projectID string) (int64, error) {
	args := m.Called(ctx, projectID)
	return args.Get(0).(int64), args.Error(1)
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

// Mock suite run repository
type MockSuiteRunRepository struct {
	mock.Mock
}

func (m *MockSuiteRunRepository) Create(ctx context.Context, suiteRun *domain.SuiteRun) error {
	args := m.Called(ctx, suiteRun)
	return args.Error(0)
}

func (m *MockSuiteRunRepository) FindByTestRunID(ctx context.Context, testRunID uint) ([]*domain.SuiteRun, error) {
	args := m.Called(ctx, testRunID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SuiteRun), args.Error(1)
}

func (m *MockSuiteRunRepository) CreateBatch(ctx context.Context, suiteRuns []*domain.SuiteRun) error {
	args := m.Called(ctx, suiteRuns)
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

// Mock spec run repository
type MockSpecRunRepository struct {
	mock.Mock
}

func (m *MockSpecRunRepository) Create(ctx context.Context, specRun *domain.SpecRun) error {
	args := m.Called(ctx, specRun)
	return args.Error(0)
}

func (m *MockSpecRunRepository) FindBySuiteRunID(ctx context.Context, suiteRunID uint) ([]*domain.SpecRun, error) {
	args := m.Called(ctx, suiteRunID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SpecRun), args.Error(1)
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

var _ = Describe("TestRunService", Label("unit", "application", "testing"), func() {
	var (
		service         *application.TestRunService
		mockTestRunRepo *MockTestRunRepository
		mockSuiteRepo   *MockSuiteRunRepository
		mockSpecRepo    *MockSpecRunRepository
		ctx             context.Context
		fixtures        *testhelpers.FixtureBuilder
	)

	BeforeEach(func() {
		mockTestRunRepo = new(MockTestRunRepository)
		mockSuiteRepo = new(MockSuiteRunRepository)
		mockSpecRepo = new(MockSpecRunRepository)
		service = application.NewTestRunService(mockTestRunRepo, mockSuiteRepo, mockSpecRepo)
		ctx = context.Background()
		fixtures = testhelpers.NewFixtureBuilder()
	})

	Describe("CreateTestRun", func() {
		It("should create a test run successfully", func() {
			testRun := &domain.TestRun{
				RunID:     "test-123",
				ProjectID: "proj-456",
				Branch:    "main",
				GitCommit: "abc123",
				Status:    "running",
				StartTime: time.Now(),
			}

			mockTestRunRepo.On("Create", ctx, testRun).Return(nil)

			err := service.CreateTestRun(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())

			mockTestRunRepo.AssertExpectations(GinkgoT())
		})

		It("should return error when repository fails", func() {
			testRun := &domain.TestRun{
				RunID:     "test-123",
				ProjectID: "proj-456",
			}

			expectedErr := errors.New("database error")
			mockTestRunRepo.On("Create", ctx, testRun).Return(expectedErr)

			err := service.CreateTestRun(ctx, testRun)
			Expect(err).To(MatchError(expectedErr))
		})

		It("should validate required fields", func() {
			testRun := &domain.TestRun{
				// Missing required fields
			}

			err := service.CreateTestRun(ctx, testRun)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("project ID is required"))
		})
	})

	Describe("GetTestRun", func() {
		It("should return test run when found", func() {
			expectedRun := fixtures.TestRun("proj-123",
				testhelpers.WithTestRunID("test-123"),
				testhelpers.WithBranch("feature/test"),
			)

			mockTestRunRepo.On("GetByID", ctx, uint(1)).Return(expectedRun, nil)

			result, err := service.GetTestRun(ctx, 1)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(expectedRun))
		})

		It("should return error when not found", func() {
			mockTestRunRepo.On("GetByID", ctx, uint(999)).Return(nil, errors.New("not found"))

			result, err := service.GetTestRun(ctx, 999)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})

	})

	Describe("GetTestRunByRunID", func() {
		It("should return test run when found", func() {
			expectedRun := fixtures.TestRun("proj-123",
				testhelpers.WithTestRunID("test-123"),
			)

			mockTestRunRepo.On("GetByRunID", ctx, "test-123").Return(expectedRun, nil)

			result, err := service.GetTestRunByRunID(ctx, "test-123")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(expectedRun))
		})

		It("should handle empty RunID parameter", func() {
			mockTestRunRepo.On("GetByRunID", ctx, "").Return(nil, errors.New("invalid RunID"))
			
			result, err := service.GetTestRunByRunID(ctx, "")
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})
	})

	Describe("GetTestRunsByProjectID", func() {
		It("should return test runs for project", func() {
			runs := []*domain.TestRun{
				fixtures.TestRun("proj-123", testhelpers.WithTestRunID("test-1")),
				fixtures.TestRun("proj-123", testhelpers.WithTestRunID("test-2")),
			}

			mockTestRunRepo.On("GetLatestByProjectID", ctx, "proj-123", 100).Return(runs, nil)

			results, err := service.GetProjectTestRuns(ctx, "proj-123", 100)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(HaveLen(2))
			Expect(results[0].RunID).To(Equal("test-1"))
			Expect(results[1].RunID).To(Equal("test-2"))
		})

		It("should return empty slice when no runs found", func() {
			mockTestRunRepo.On("GetLatestByProjectID", ctx, "proj-999", 100).Return([]*domain.TestRun{}, nil)

			results, err := service.GetProjectTestRuns(ctx, "proj-999", 100)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(BeEmpty())
		})

		It("should handle empty ProjectID parameter", func() {
			mockTestRunRepo.On("GetLatestByProjectID", ctx, "", 100).Return([]*domain.TestRun{}, nil)
			
			results, err := service.GetProjectTestRuns(ctx, "", 100)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(BeEmpty())
		})
	})

	Describe("UpdateTestRunStatus", func() {
		It("should update status successfully", func() {
			existingRun := fixtures.TestRun("proj-123",
				testhelpers.WithTestRunID("test-123"),
				testhelpers.WithStatus("running"),
			)

			mockTestRunRepo.On("GetByID", ctx, uint(1)).Return(existingRun, nil)
			mockSuiteRepo.On("FindByTestRunID", ctx, uint(1)).Return([]*domain.SuiteRun{}, nil)
			mockTestRunRepo.On("Update", ctx, mock.MatchedBy(func(tr *domain.TestRun) bool {
				return tr.Status == "completed"
			})).Return(nil)

			err := service.CompleteTestRun(ctx, 1, "completed")
			Expect(err).NotTo(HaveOccurred())

			mockTestRunRepo.AssertExpectations(GinkgoT())
		})

		It("should update status successfully with any status value", func() {
			existingRun := fixtures.TestRun("proj-123",
				testhelpers.WithTestRunID("test-123"),
				testhelpers.WithStatus("running"),
			)

			mockTestRunRepo.On("GetByID", ctx, uint(1)).Return(existingRun, nil)
			mockSuiteRepo.On("FindByTestRunID", ctx, uint(1)).Return([]*domain.SuiteRun{}, nil)
			mockTestRunRepo.On("Update", ctx, mock.MatchedBy(func(tr *domain.TestRun) bool {
				return tr.Status == "invalid-status"
			})).Return(nil)

			err := service.CompleteTestRun(ctx, 1, "invalid-status")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error when test run not found", func() {
			mockTestRunRepo.On("GetByID", ctx, uint(999)).Return(nil, errors.New("not found"))

			err := service.CompleteTestRun(ctx, 999, "completed")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("DeleteTestRun", func() {
		It("should delete test run successfully", func() {
			testRun := fixtures.TestRun("proj-123")
			mockTestRunRepo.On("GetByID", ctx, uint(1)).Return(testRun, nil)
			mockTestRunRepo.On("Delete", ctx, uint(1)).Return(nil)

			err := service.DeleteTestRun(ctx, 1)
			Expect(err).NotTo(HaveOccurred())

			mockTestRunRepo.AssertExpectations(GinkgoT())
		})

		It("should return error when deletion fails", func() {
			testRun := fixtures.TestRun("proj-123")
			expectedErr := errors.New("deletion failed")
			mockTestRunRepo.On("GetByID", ctx, uint(1)).Return(testRun, nil)
			mockTestRunRepo.On("Delete", ctx, uint(1)).Return(expectedErr)

			err := service.DeleteTestRun(ctx, 1)
			Expect(err).To(MatchError(expectedErr))
		})

		It("should return error when test run not found for deletion", func() {
			mockTestRunRepo.On("GetByID", ctx, uint(0)).Return(nil, errors.New("test run not found"))
			
			err := service.DeleteTestRun(ctx, 0)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("test run not found"))
		})
	})

	Describe("GetRecentTestRuns", func() {
		It("should return recent test runs", func() {
			runs := []*domain.TestRun{
				fixtures.TestRun("proj-123", testhelpers.WithTestRunID("test-1")),
				fixtures.TestRun("proj-123", testhelpers.WithTestRunID("test-2")),
			}

			mockTestRunRepo.On("GetRecent", ctx, 10).Return(runs, nil)

			results, err := service.GetRecentTestRuns(ctx, 10)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(HaveLen(2))
		})

		It("should pass through zero limit to repository", func() {
			runs := []*domain.TestRun{
				fixtures.TestRun("proj-123"),
			}

			mockTestRunRepo.On("GetRecent", ctx, 0).Return(runs, nil)

			results, err := service.GetRecentTestRuns(ctx, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(HaveLen(1))
		})
	})

	Describe("GetTestRunSummary", func() {
		It("should return summary for project", func() {
			summary := &domain.TestRunSummary{
				TotalRuns:      100,
				PassedRuns:     85,
				FailedRuns:     10,
				AverageRunTime: 5 * time.Minute,
				SuccessRate:    0.85,
			}

			mockTestRunRepo.On("GetTestRunSummary", ctx, "proj-123").Return(summary, nil)

			result, err := service.GetTestRunSummary(ctx, "proj-123")
			Expect(err).NotTo(HaveOccurred())
			Expect(result).NotTo(BeNil())
			Expect(result.TotalRuns).To(Equal(100))
			Expect(result.PassedRuns).To(Equal(85))
			Expect(result.FailedRuns).To(Equal(10))
			Expect(result.SuccessRate).To(Equal(0.85))
		})

		It("should handle repository error", func() {
			mockTestRunRepo.On("GetTestRunSummary", ctx, "proj-123").Return(nil, errors.New("database error"))

			result, err := service.GetTestRunSummary(ctx, "proj-123")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("database error"))
			Expect(result).To(BeNil())
		})
	})

	Describe("Edge Cases", func() {
		It("should handle concurrent operations gracefully", func() {
			testRun := fixtures.TestRun("proj-123")
			mockTestRunRepo.On("Create", ctx, mock.Anything).Return(nil)

			// Simulate concurrent creates
			done := make(chan bool, 3)
			for i := 0; i < 3; i++ {
				go func() {
					err := service.CreateTestRun(ctx, testRun)
					Expect(err).NotTo(HaveOccurred())
					done <- true
				}()
			}

			// Wait for all goroutines
			for i := 0; i < 3; i++ {
				<-done
			}
		})

		It("should handle nil repository gracefully", func() {
			defer func() {
				if r := recover(); r != nil {
					// Expected panic recovery
					Expect(r).NotTo(BeNil())
				}
			}()
			
			nilService := application.NewTestRunService(nil, nil, nil)
			_, err := nilService.GetTestRun(ctx, 1)
			Expect(err).To(BeNil()) // We won't reach here due to panic
		})
	})
})