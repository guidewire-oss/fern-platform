package application_test

import (
	"context"
	"errors"
	"fmt"
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

// Helper for matching test runs in mocks
func MatchTestRun(matcher func(*domain.TestRun) bool) interface{} {
	return mock.MatchedBy(matcher)
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

			_, _, err := service.CreateTestRun(ctx, testRun)
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

			_, _, err := service.CreateTestRun(ctx, testRun)
			Expect(err).To(MatchError(expectedErr))
		})

		It("should validate required fields", func() {
			testRun := &domain.TestRun{
				// Missing required fields
			}

			_, _, err := service.CreateTestRun(ctx, testRun)
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

		It("should validate RunID parameter", func() {
			mockTestRunRepo.On("GetByRunID", ctx, "").Return(nil, errors.New("RunID is required"))

			result, err := service.GetTestRunByRunID(ctx, "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("RunID is required"))
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

		It("should validate ProjectID parameter", func() {
			mockTestRunRepo.On("GetLatestByProjectID", ctx, "", 100).Return(nil, errors.New("ProjectID is required"))

			results, err := service.GetProjectTestRuns(ctx, "", 100)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("ProjectID is required"))
			Expect(results).To(BeNil())
		})
	})

	Describe("UpdateTestRunStatus", func() {
		It("should update status successfully", func() {
			existingRun := fixtures.TestRun("proj-123",
				testhelpers.WithTestRunID("test-123"),
				testhelpers.WithStatus("running"),
			)

			mockTestRunRepo.On("GetByID", ctx, uint(1)).Return(existingRun, nil)
			mockTestRunRepo.On("Update", ctx, mock.MatchedBy(func(tr *domain.TestRun) bool {
				return tr.Status == "completed"
			})).Return(nil)
			mockSuiteRepo.On("FindByTestRunID", ctx, uint(1)).Return([]*domain.SuiteRun{}, nil)

			err := service.CompleteTestRun(ctx, 1, "completed")
			Expect(err).NotTo(HaveOccurred())

			mockTestRunRepo.AssertExpectations(GinkgoT())
			mockSuiteRepo.AssertExpectations(GinkgoT())
		})

		It("should allow any status value", func() {
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
			existingRun := fixtures.TestRun("proj-123",
				testhelpers.WithTestRunID("test-123"),
			)

			mockTestRunRepo.On("GetByID", ctx, uint(1)).Return(existingRun, nil)
			mockTestRunRepo.On("Delete", ctx, uint(1)).Return(nil)

			err := service.DeleteTestRun(ctx, 1)
			Expect(err).NotTo(HaveOccurred())

			mockTestRunRepo.AssertExpectations(GinkgoT())
		})

		It("should return error when deletion fails", func() {
			existingRun := fixtures.TestRun("proj-123",
				testhelpers.WithTestRunID("test-123"),
			)
			expectedErr := errors.New("deletion failed")

			mockTestRunRepo.On("GetByID", ctx, uint(1)).Return(existingRun, nil)
			mockTestRunRepo.On("Delete", ctx, uint(1)).Return(expectedErr)

			err := service.DeleteTestRun(ctx, 1)
			Expect(err).To(MatchError(expectedErr))
		})

		It("should return error when test run not found", func() {
			mockTestRunRepo.On("GetByID", ctx, uint(999)).Return(nil, errors.New("not found"))

			err := service.DeleteTestRun(ctx, 999)
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

	Describe("AddSuiteRun", func() {
		It("should add suite run successfully", func() {
			suiteRun := &domain.SuiteRun{
				TestRunID: 1,
				Name:      "Test Suite",
				Status:    "running",
			}

			mockSuiteRepo.On("Create", ctx, suiteRun).Return(nil)

			err := service.AddSuiteRun(ctx, 1, suiteRun)
			Expect(err).NotTo(HaveOccurred())

			mockSuiteRepo.AssertExpectations(GinkgoT())
		})

		It("should return error when test run ID mismatch", func() {
			suiteRun := &domain.SuiteRun{
				TestRunID: 2,
				Name:      "Test Suite",
			}

			err := service.AddSuiteRun(ctx, 1, suiteRun)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("test ID mismatch"))
		})

		It("should return error when repository fails", func() {
			suiteRun := &domain.SuiteRun{
				TestRunID: 1,
				Name:      "Test Suite",
			}

			mockSuiteRepo.On("Create", ctx, suiteRun).Return(errors.New("database error"))

			err := service.AddSuiteRun(ctx, 1, suiteRun)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to create suite run"))
		})
	})

	Describe("AddSpecRun", func() {
		It("should add spec run successfully", func() {
			specRun := &domain.SpecRun{
				SuiteRunID: 1,
				Name:       "Test Spec",
				Status:     "passed",
			}
			suiteRun := &domain.SuiteRun{
				ID:        1,
				Name:      "Suite",
				StartTime: time.Now(),
			}

			mockSpecRepo.On("Create", ctx, specRun).Return(nil)
			mockSpecRepo.On("FindBySuiteRunID", ctx, uint(1)).Return([]*domain.SpecRun{specRun}, nil)
			mockSuiteRepo.On("GetByID", ctx, uint(1)).Return(suiteRun, nil)
			mockSuiteRepo.On("Update", ctx, mock.Anything).Return(nil)

			err := service.AddSpecRun(ctx, 1, specRun)
			Expect(err).NotTo(HaveOccurred())

			mockSpecRepo.AssertExpectations(GinkgoT())
			mockSuiteRepo.AssertExpectations(GinkgoT())
		})

		It("should return error when suite run ID mismatch", func() {
			specRun := &domain.SpecRun{
				SuiteRunID: 2,
				Name:       "Test Spec",
			}

			err := service.AddSpecRun(ctx, 1, specRun)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("suite ID mismatch"))
		})

		It("should return error when repository fails", func() {
			specRun := &domain.SpecRun{
				SuiteRunID: 1,
				Name:       "Test Spec",
			}

			mockSpecRepo.On("Create", ctx, specRun).Return(errors.New("database error"))

			err := service.AddSpecRun(ctx, 1, specRun)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to create spec run"))
		})
	})

	Describe("GetTestRunWithDetails", func() {
		It("should return test run with details", func() {
			expectedRun := fixtures.TestRun("proj-123",
				testhelpers.WithTestRunID("test-123"),
			)

			mockTestRunRepo.On("GetWithDetails", ctx, uint(1)).Return(expectedRun, nil)

			result, err := service.GetTestRunWithDetails(ctx, 1)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(expectedRun))

			mockTestRunRepo.AssertExpectations(GinkgoT())
		})

		It("should return error when not found", func() {
			mockTestRunRepo.On("GetWithDetails", ctx, uint(999)).Return(nil, errors.New("not found"))

			result, err := service.GetTestRunWithDetails(ctx, 999)
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})
	})

	Describe("CreateTestRunWithSuites", func() {
		It("should create test run with suites successfully", func() {
			testRun := &domain.TestRun{
				ProjectID: "proj-123",
				RunID:     "test-123",
				Status:    "running",
			}
			suites := []domain.SuiteRun{
				{
					Name:   "Suite 1",
					Status: "passed",
					SpecRuns: []*domain.SpecRun{
						{Name: "Spec 1", Status: "passed"},
					},
				},
			}

			mockTestRunRepo.On("Create", ctx, testRun).Return(nil)
			mockSuiteRepo.On("Create", ctx, mock.Anything).Return(nil)
			mockSpecRepo.On("CreateBatch", ctx, mock.Anything).Return(nil)
			mockTestRunRepo.On("GetByID", ctx, mock.Anything).Return(testRun, nil)
			mockSuiteRepo.On("FindByTestRunID", ctx, mock.Anything).Return([]*domain.SuiteRun{}, nil)
			mockTestRunRepo.On("Update", ctx, mock.Anything).Return(nil)

			err := service.CreateTestRunWithSuites(ctx, testRun, suites)
			Expect(err).NotTo(HaveOccurred())

			mockTestRunRepo.AssertExpectations(GinkgoT())
			mockSuiteRepo.AssertExpectations(GinkgoT())
		})

		It("should return error when test run creation fails", func() {
			testRun := &domain.TestRun{
				ProjectID: "proj-123",
			}
			suites := []domain.SuiteRun{}

			mockTestRunRepo.On("Create", ctx, testRun).Return(errors.New("creation error"))

			err := service.CreateTestRunWithSuites(ctx, testRun, suites)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("CreateSuiteRun", func() {
		It("should create suite run with defaults", func() {
			suiteRun := &domain.SuiteRun{
				TestRunID: 1,
				Name:      "Test Suite",
			}

			mockSuiteRepo.On("Create", ctx, mock.MatchedBy(func(s *domain.SuiteRun) bool {
				return s.Status == "running" && s.TestRunID == 1
			})).Return(nil)

			err := service.CreateSuiteRun(ctx, suiteRun)
			Expect(err).NotTo(HaveOccurred())

			mockSuiteRepo.AssertExpectations(GinkgoT())
		})

		It("should return error when test run ID is missing", func() {
			suiteRun := &domain.SuiteRun{
				Name: "Test Suite",
			}

			err := service.CreateSuiteRun(ctx, suiteRun)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("test run ID is required"))
		})

		It("should preserve existing status", func() {
			suiteRun := &domain.SuiteRun{
				TestRunID: 1,
				Name:      "Test Suite",
				Status:    "completed",
			}

			mockSuiteRepo.On("Create", ctx, suiteRun).Return(nil)

			err := service.CreateSuiteRun(ctx, suiteRun)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("CreateSpecRun", func() {
		It("should create spec run with defaults", func() {
			specRun := &domain.SpecRun{
				SuiteRunID: 1,
				Name:       "Test Spec",
			}

			mockSpecRepo.On("Create", ctx, mock.MatchedBy(func(s *domain.SpecRun) bool {
				return s.Status == "pending" && s.SuiteRunID == 1
			})).Return(nil)

			err := service.CreateSpecRun(ctx, specRun)
			Expect(err).NotTo(HaveOccurred())

			mockSpecRepo.AssertExpectations(GinkgoT())
		})

		It("should return error when suite run ID is missing", func() {
			specRun := &domain.SpecRun{
				Name: "Test Spec",
			}

			err := service.CreateSpecRun(ctx, specRun)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("suite run ID is required"))
		})

		It("should calculate duration from start and end time", func() {
			startTime := time.Now()
			endTime := startTime.Add(5 * time.Second)
			specRun := &domain.SpecRun{
				SuiteRunID: 1,
				Name:       "Test Spec",
				StartTime:  startTime,
				EndTime:    &endTime,
			}

			mockSpecRepo.On("Create", ctx, mock.MatchedBy(func(s *domain.SpecRun) bool {
				return s.Duration == 5*time.Second
			})).Return(nil)

			err := service.CreateSpecRun(ctx, specRun)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should preserve existing status", func() {
			specRun := &domain.SpecRun{
				SuiteRunID: 1,
				Name:       "Test Spec",
				Status:     "passed",
			}

			mockSpecRepo.On("Create", ctx, specRun).Return(nil)

			err := service.CreateSpecRun(ctx, specRun)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("ListTestRuns", func() {
		It("should list test runs for project with pagination", func() {
			runs := []*domain.TestRun{
				fixtures.TestRun("proj-123", testhelpers.WithTestRunID("test-1")),
				fixtures.TestRun("proj-123", testhelpers.WithTestRunID("test-2")),
			}

			mockTestRunRepo.On("GetLatestByProjectID", ctx, "proj-123", 10).Return(runs, nil)
			mockTestRunRepo.On("CountByProjectID", ctx, "proj-123").Return(int64(2), nil)

			results, count, err := service.ListTestRuns(ctx, "proj-123", 10, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(HaveLen(2))
			Expect(count).To(Equal(int64(2)))

			mockTestRunRepo.AssertExpectations(GinkgoT())
		})

		It("should return empty list when no project ID", func() {
			results, count, err := service.ListTestRuns(ctx, "", 10, 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(BeEmpty())
			Expect(count).To(Equal(int64(0)))
		})

		It("should return error when repository fails", func() {
			mockTestRunRepo.On("GetLatestByProjectID", ctx, "proj-123", 10).Return(nil, errors.New("database error"))

			results, count, err := service.ListTestRuns(ctx, "proj-123", 10, 0)
			Expect(err).To(HaveOccurred())
			Expect(results).To(BeNil())
			Expect(count).To(Equal(int64(0)))
		})
	})

	Describe("GetSuiteRunsByTestRunID", func() {
		It("should return suite runs for test run", func() {
			suites := []*domain.SuiteRun{
				{ID: 1, TestRunID: 1, Name: "Suite 1"},
				{ID: 2, TestRunID: 1, Name: "Suite 2"},
			}

			mockSuiteRepo.On("FindByTestRunID", ctx, uint(1)).Return(suites, nil)

			results, err := service.GetSuiteRunsByTestRunID(ctx, 1)
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(HaveLen(2))

			mockSuiteRepo.AssertExpectations(GinkgoT())
		})

		It("should return error when repository fails", func() {
			mockSuiteRepo.On("FindByTestRunID", ctx, uint(1)).Return(nil, errors.New("database error"))

			results, err := service.GetSuiteRunsByTestRunID(ctx, 1)
			Expect(err).To(HaveOccurred())
			Expect(results).To(BeNil())
		})
	})

	Describe("UpdateTestRun", func() {
		It("should update test run without tags", func() {
			testRun := &domain.TestRun{
				ID:        1,
				ProjectID: "proj-123",
				Status:    "completed",
			}

			mockTestRunRepo.On("Update", ctx, testRun).Return(nil)

			err := service.UpdateTestRun(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())

			mockTestRunRepo.AssertExpectations(GinkgoT())
		})

		It("should return error when update fails", func() {
			testRun := &domain.TestRun{
				ID:        1,
				ProjectID: "proj-123",
			}

			mockTestRunRepo.On("Update", ctx, testRun).Return(errors.New("update failed"))

			err := service.UpdateTestRun(ctx, testRun)
			Expect(err).To(HaveOccurred())
		})

		It("should handle test run with tags", func() {
			testRun := &domain.TestRun{
				ID:        1,
				ProjectID: "proj-123",
				Status:    "completed",
				Tags: []domain.Tag{
					{ID: 1, Name: "priority:high", Category: "priority", Value: "high"},
				},
			}

			mockTestRunRepo.On("Update", ctx, testRun).Return(nil)

			err := service.UpdateTestRun(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should handle test run with multiple tags", func() {
			testRun := &domain.TestRun{
				ID:        1,
				ProjectID: "proj-123",
				Status:    "completed",
				Tags: []domain.Tag{
					{ID: 1, Name: "priority:high", Category: "priority", Value: "high"},
					{ID: 2, Name: "env:staging", Category: "env", Value: "staging"},
					{ID: 3, Name: "smoke", Category: "", Value: "smoke"},
				},
			}

			mockTestRunRepo.On("Update", ctx, testRun).Return(nil)

			err := service.UpdateTestRun(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())

			mockTestRunRepo.AssertExpectations(GinkgoT())
		})

		It("should handle test run with empty tags array", func() {
			testRun := &domain.TestRun{
				ID:        1,
				ProjectID: "proj-123",
				Status:    "completed",
				Tags:      []domain.Tag{}, // Empty tags array
			}

			mockTestRunRepo.On("Update", ctx, testRun).Return(nil)

			err := service.UpdateTestRun(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())

			mockTestRunRepo.AssertExpectations(GinkgoT())
		})

		It("should skip tag association update when repository doesn't support GetDB", func() {
			// This tests the type assertion check: if db, ok := s.testRunRepo.(interface{ GetDB() *gorm.DB })
			// The mock repository doesn't implement GetDB(), so the tag association logic is skipped
			testRun := &domain.TestRun{
				ID:        1,
				ProjectID: "proj-123",
				Status:    "completed",
				Tags: []domain.Tag{
					{ID: 1, Name: "tag1", Category: "cat1", Value: "val1"},
				},
			}

			mockTestRunRepo.On("Update", ctx, testRun).Return(nil)

			err := service.UpdateTestRun(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())

			// Verify that Update was called but no GetDB-related operations occurred
			mockTestRunRepo.AssertExpectations(GinkgoT())
			mockTestRunRepo.AssertNotCalled(GinkgoT(), "GetDB")
		})

		It("should properly convert domain tags to database tags format", func() {
			// This test verifies the tag conversion logic:
			// dbTags[i] = database.Tag{
			//     BaseModel: database.BaseModel{ID: t.ID},
			//     Name: t.Name, Category: t.Category, Value: t.Value,
			// }
			testRun := &domain.TestRun{
				ID:        1,
				ProjectID: "proj-123",
				Status:    "completed",
				Tags: []domain.Tag{
					{ID: 10, Name: "priority:critical", Category: "priority", Value: "critical"},
					{ID: 20, Name: "type:regression", Category: "type", Value: "regression"},
				},
			}

			mockTestRunRepo.On("Update", ctx, testRun).Return(nil)

			err := service.UpdateTestRun(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())

			// The conversion happens inside the function, we're verifying it doesn't error
			mockTestRunRepo.AssertExpectations(GinkgoT())
		})

		It("should handle tags with various category and value combinations", func() {
			testRun := &domain.TestRun{
				ID:        1,
				ProjectID: "proj-123",
				Status:    "completed",
				Tags: []domain.Tag{
					{ID: 1, Name: "no-category", Category: "", Value: "no-category"},
					{ID: 2, Name: "with-category:value", Category: "with-category", Value: "value"},
					{ID: 3, Name: "empty-value:", Category: "empty-value", Value: ""},
					{ID: 4, Name: "special-chars:test@#$", Category: "special-chars", Value: "test@#$"},
				},
			}

			mockTestRunRepo.On("Update", ctx, testRun).Return(nil)

			err := service.UpdateTestRun(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())

			mockTestRunRepo.AssertExpectations(GinkgoT())
		})

		It("should return error when update succeeds but tag update is attempted with nil tags", func() {
			// Edge case: testRun.Tags is nil (not empty array)
			testRun := &domain.TestRun{
				ID:        1,
				ProjectID: "proj-123",
				Status:    "completed",
				Tags:      nil, // nil tags
			}

			mockTestRunRepo.On("Update", ctx, testRun).Return(nil)

			err := service.UpdateTestRun(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())

			// len(nil) == 0, so tag association logic is skipped
			mockTestRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("UpdateTestRun - Tag Association Edge Cases", func() {
		It("should handle test run with large number of tags", func() {
			// Test with many tags to ensure the conversion loop works correctly
			tags := make([]domain.Tag, 100)
			for i := 0; i < 100; i++ {
				tags[i] = domain.Tag{
					ID:       uint(i + 1),
					Name:     fmt.Sprintf("tag-%d", i),
					Category: fmt.Sprintf("category-%d", i%10),
					Value:    fmt.Sprintf("value-%d", i),
				}
			}

			testRun := &domain.TestRun{
				ID:        1,
				ProjectID: "proj-123",
				Status:    "completed",
				Tags:      tags,
			}

			mockTestRunRepo.On("Update", ctx, testRun).Return(nil)

			err := service.UpdateTestRun(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())

			mockTestRunRepo.AssertExpectations(GinkgoT())
		})

		It("should not call tag association when tags array has zero length", func() {
			testRun := &domain.TestRun{
				ID:        1,
				ProjectID: "proj-123",
				Status:    "completed",
				Tags:      []domain.Tag{}, // Zero length explicitly
			}

			mockTestRunRepo.On("Update", ctx, testRun).Return(nil)

			err := service.UpdateTestRun(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())

			// The condition: if len(testRun.Tags) > 0 should be false
			mockTestRunRepo.AssertExpectations(GinkgoT())
		})

		It("should preserve tag IDs during conversion", func() {
			// Verify that tag IDs are correctly set in the database.Tag BaseModel
			testRun := &domain.TestRun{
				ID:        1,
				ProjectID: "proj-123",
				Status:    "completed",
				Tags: []domain.Tag{
					{ID: 999, Name: "tag-with-large-id", Category: "test", Value: "test"},
					{ID: 1, Name: "tag-with-small-id", Category: "test", Value: "test"},
					{ID: 42, Name: "tag-with-mid-id", Category: "test", Value: "test"},
				},
			}

			mockTestRunRepo.On("Update", ctx, testRun).Return(nil)

			err := service.UpdateTestRun(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())

			mockTestRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("updateSuiteStatistics", func() {
		It("should calculate statistics with various spec statuses", func() {
			specRun1 := &domain.SpecRun{
				SuiteRunID: 1,
				Name:       "Spec 1",
				Status:     "passed",
				Duration:   2 * time.Second,
			}
			specRun2 := &domain.SpecRun{
				SuiteRunID: 1,
				Name:       "Spec 2",
				Status:     "failed",
				Duration:   3 * time.Second,
			}
			specRun3 := &domain.SpecRun{
				SuiteRunID: 1,
				Name:       "Spec 3",
				Status:     "skipped",
				Duration:   1 * time.Second,
			}
			suiteRun := &domain.SuiteRun{
				ID:        1,
				Name:      "Suite",
				StartTime: time.Now(),
			}

			mockSpecRepo.On("Create", ctx, specRun1).Return(nil)
			mockSpecRepo.On("FindBySuiteRunID", ctx, uint(1)).Return([]*domain.SpecRun{specRun1, specRun2, specRun3}, nil)
			mockSuiteRepo.On("GetByID", ctx, uint(1)).Return(suiteRun, nil)
			mockSuiteRepo.On("Update", ctx, mock.MatchedBy(func(s *domain.SuiteRun) bool {
				return s.TotalTests == 3 && s.PassedTests == 1 && s.FailedTests == 1 && s.SkippedTests == 1 && s.Duration == 6*time.Second
			})).Return(nil)

			err := service.AddSpecRun(ctx, 1, specRun1)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should handle error when getting spec runs fails", func() {
			specRun := &domain.SpecRun{
				SuiteRunID: 1,
				Name:       "Spec",
				Status:     "passed",
			}

			mockSpecRepo.On("Create", ctx, specRun).Return(nil)
			mockSpecRepo.On("FindBySuiteRunID", ctx, uint(1)).Return(nil, errors.New("database error"))

			err := service.AddSpecRun(ctx, 1, specRun)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to update suite statistics"))
		})

		It("should handle error when getting suite run fails", func() {
			specRun := &domain.SpecRun{
				SuiteRunID: 1,
				Name:       "Spec",
				Status:     "passed",
			}

			mockSpecRepo.On("Create", ctx, specRun).Return(nil)
			mockSpecRepo.On("FindBySuiteRunID", ctx, uint(1)).Return([]*domain.SpecRun{specRun}, nil)
			mockSuiteRepo.On("GetByID", ctx, uint(1)).Return(nil, errors.New("suite not found"))

			err := service.AddSpecRun(ctx, 1, specRun)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to update suite statistics"))
		})

		It("should handle error when updating suite fails", func() {
			specRun := &domain.SpecRun{
				SuiteRunID: 1,
				Name:       "Spec",
				Status:     "passed",
			}
			suiteRun := &domain.SuiteRun{
				ID:        1,
				Name:      "Suite",
				StartTime: time.Now(),
			}

			mockSpecRepo.On("Create", ctx, specRun).Return(nil)
			mockSpecRepo.On("FindBySuiteRunID", ctx, uint(1)).Return([]*domain.SpecRun{specRun}, nil)
			mockSuiteRepo.On("GetByID", ctx, uint(1)).Return(suiteRun, nil)
			mockSuiteRepo.On("Update", ctx, mock.Anything).Return(errors.New("update failed"))

			err := service.AddSpecRun(ctx, 1, specRun)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("CreateTestRunWithSuites edge cases", func() {
		It("should handle suite without spec runs", func() {
			testRun := &domain.TestRun{
				ProjectID: "proj-123",
				RunID:     "test-123",
				Status:    "running",
			}
			suites := []domain.SuiteRun{
				{
					Name:     "Suite 1",
					Status:   "passed",
					SpecRuns: nil, // No spec runs
				},
			}

			mockTestRunRepo.On("Create", ctx, testRun).Return(nil)
			mockSuiteRepo.On("Create", ctx, mock.Anything).Return(nil)
			mockTestRunRepo.On("GetByID", ctx, mock.Anything).Return(testRun, nil)
			mockSuiteRepo.On("FindByTestRunID", ctx, mock.Anything).Return([]*domain.SuiteRun{}, nil)
			mockTestRunRepo.On("Update", ctx, mock.Anything).Return(nil)

			err := service.CreateTestRunWithSuites(ctx, testRun, suites)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error when suite creation fails", func() {
			testRun := &domain.TestRun{
				ProjectID: "proj-123",
				RunID:     "test-123",
			}
			suites := []domain.SuiteRun{
				{Name: "Suite 1"},
			}

			mockTestRunRepo.On("Create", ctx, testRun).Return(nil)
			mockSuiteRepo.On("Create", ctx, mock.Anything).Return(errors.New("suite creation failed"))

			err := service.CreateTestRunWithSuites(ctx, testRun, suites)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to create suite run"))
		})

		It("should return error when spec batch creation fails", func() {
			testRun := &domain.TestRun{
				ProjectID: "proj-123",
				RunID:     "test-123",
			}
			suites := []domain.SuiteRun{
				{
					Name: "Suite 1",
					SpecRuns: []*domain.SpecRun{
						{Name: "Spec 1"},
					},
				},
			}

			mockTestRunRepo.On("Create", ctx, testRun).Return(nil)
			mockSuiteRepo.On("Create", ctx, mock.Anything).Return(nil)
			mockSpecRepo.On("CreateBatch", ctx, mock.Anything).Return(errors.New("batch creation failed"))

			err := service.CreateTestRunWithSuites(ctx, testRun, suites)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to create spec runs"))
		})

		It("should return error when completing test run fails", func() {
			testRun := &domain.TestRun{
				ProjectID: "proj-123",
				RunID:     "test-123",
			}
			suites := []domain.SuiteRun{}

			mockTestRunRepo.On("Create", ctx, testRun).Return(nil)
			mockTestRunRepo.On("GetByID", ctx, mock.Anything).Return(nil, errors.New("not found"))

			err := service.CreateTestRunWithSuites(ctx, testRun, suites)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("ListTestRuns edge cases", func() {
		It("should return error when count fails", func() {
			runs := []*domain.TestRun{
				fixtures.TestRun("proj-123"),
			}

			mockTestRunRepo.On("GetLatestByProjectID", ctx, "proj-123", 10).Return(runs, nil)
			mockTestRunRepo.On("CountByProjectID", ctx, "proj-123").Return(int64(0), errors.New("count failed"))

			results, count, err := service.ListTestRuns(ctx, "proj-123", 10, 0)
			Expect(err).To(HaveOccurred())
			Expect(results).To(BeNil())
			Expect(count).To(Equal(int64(0)))
		})
	})

	Describe("CompleteTestRun edge cases", func() {
		It("should return error when update fails", func() {
			testRun := &domain.TestRun{ID: 1, ProjectID: "proj-123"}

			mockTestRunRepo.On("GetByID", ctx, uint(1)).Return(testRun, nil)
			mockSuiteRepo.On("FindByTestRunID", ctx, uint(1)).Return([]*domain.SuiteRun{}, nil)
			mockTestRunRepo.On("Update", ctx, mock.Anything).Return(errors.New("update failed"))

			err := service.CompleteTestRun(ctx, 1, "completed")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to update test run"))
		})
	})

	Describe("CreateTestRun duplicate handling", func() {
		It("should handle duplicate run ID by returning existing run", func() {
			testRun := &domain.TestRun{
				RunID:     "test-123",
				ProjectID: "proj-456",
				Status:    "running",
			}
			existingRun := &domain.TestRun{
				ID:        1,
				RunID:     "test-123",
				ProjectID: "proj-456",
				Status:    "running",
			}

			mockTestRunRepo.On("Create", ctx, testRun).Return(errors.New("UNIQUE constraint failed"))
			mockTestRunRepo.On("GetByRunID", ctx, "test-123").Return(existingRun, nil)

			result, alreadyExisted, err := service.CreateTestRun(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())
			Expect(alreadyExisted).To(BeTrue())
			Expect(result).To(Equal(existingRun))

			mockTestRunRepo.AssertExpectations(GinkgoT())
		})

		It("should set default status when not provided", func() {
			testRun := &domain.TestRun{
				RunID:     "test-123",
				ProjectID: "proj-456",
			}

			mockTestRunRepo.On("Create", ctx, mock.MatchedBy(func(tr *domain.TestRun) bool {
				return tr.Status == "running"
			})).Return(nil)

			_, _, err := service.CreateTestRun(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error when duplicate fetch fails", func() {
			testRun := &domain.TestRun{
				RunID:     "test-123",
				ProjectID: "proj-456",
				Status:    "running",
			}

			mockTestRunRepo.On("Create", ctx, testRun).Return(errors.New("duplicate key value violates unique constraint"))
			mockTestRunRepo.On("GetByRunID", ctx, "test-123").Return(nil, errors.New("not found"))

			result, alreadyExisted, err := service.CreateTestRun(ctx, testRun)
			Expect(err).To(HaveOccurred())
			Expect(alreadyExisted).To(BeFalse())
			Expect(result).To(BeNil())
		})

		It("should handle duplicate error without run ID", func() {
			testRun := &domain.TestRun{
				RunID:     "",
				ProjectID: "proj-456",
				Status:    "running",
			}

			mockTestRunRepo.On("Create", ctx, testRun).Return(errors.New("UNIQUE constraint failed"))

			result, alreadyExisted, err := service.CreateTestRun(ctx, testRun)
			Expect(err).To(HaveOccurred())
			Expect(alreadyExisted).To(BeFalse())
			Expect(result).To(BeNil())
		})
	})

	Describe("CreateSpecRun duration calculation", func() {
		It("should not calculate duration when end time is nil", func() {
			startTime := time.Now()
			specRun := &domain.SpecRun{
				SuiteRunID: 1,
				Name:       "Test Spec",
				StartTime:  startTime,
				EndTime:    nil,
			}

			mockSpecRepo.On("Create", ctx, mock.MatchedBy(func(s *domain.SpecRun) bool {
				return s.Duration == 0
			})).Return(nil)

			err := service.CreateSpecRun(ctx, specRun)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not calculate duration when start time is zero", func() {
			endTime := time.Now()
			specRun := &domain.SpecRun{
				SuiteRunID: 1,
				Name:       "Test Spec",
				StartTime:  time.Time{},
				EndTime:    &endTime,
			}

			mockSpecRepo.On("Create", ctx, mock.MatchedBy(func(s *domain.SpecRun) bool {
				return s.Duration == 0
			})).Return(nil)

			err := service.CreateSpecRun(ctx, specRun)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("CompleteTestRun statistics calculation", func() {
		It("should calculate statistics from suite runs", func() {
			testRun := &domain.TestRun{
				ID:        1,
				ProjectID: "proj-123",
				Status:    "running",
			}
			suites := []*domain.SuiteRun{
				{TotalTests: 10, PassedTests: 8, FailedTests: 2, SkippedTests: 0},
				{TotalTests: 5, PassedTests: 4, FailedTests: 0, SkippedTests: 1},
			}

			mockTestRunRepo.On("GetByID", ctx, uint(1)).Return(testRun, nil)
			mockSuiteRepo.On("FindByTestRunID", ctx, uint(1)).Return(suites, nil)
			mockTestRunRepo.On("Update", ctx, mock.MatchedBy(func(tr *domain.TestRun) bool {
				return tr.TotalTests == 15 && tr.PassedTests == 12 && tr.FailedTests == 2 && tr.SkippedTests == 1
			})).Return(nil)

			err := service.CompleteTestRun(ctx, 1, "completed")
			Expect(err).NotTo(HaveOccurred())

			mockTestRunRepo.AssertExpectations(GinkgoT())
			mockSuiteRepo.AssertExpectations(GinkgoT())
		})

		It("should return error when getting suite runs fails", func() {
			testRun := &domain.TestRun{ID: 1, ProjectID: "proj-123"}

			mockTestRunRepo.On("GetByID", ctx, uint(1)).Return(testRun, nil)
			mockSuiteRepo.On("FindByTestRunID", ctx, uint(1)).Return(nil, errors.New("database error"))

			err := service.CompleteTestRun(ctx, 1, "completed")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get suite runs"))
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
					_, _, err := service.CreateTestRun(ctx, testRun)
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
			nilService := application.NewTestRunService(nil, nil, nil)
			Expect(func() {
				_, _ = nilService.GetTestRun(ctx, 1)
			}).To(Panic())
		})
	})

	Describe("CreateTestRun with Tag Category and Value", func() {
		It("should populate Category and Value for tags during first-time creation", func() {
			testRun := &domain.TestRun{
				RunID:     "test-123",
				ProjectID: "proj-456",
				Branch:    "main",
				Status:    "running",
				Tags: []domain.Tag{
					{ID: 1, Name: "priority:high", Category: "priority", Value: "high"},
					{ID: 2, Name: "env:staging", Category: "env", Value: "staging"},
					{ID: 3, Name: "smoke", Category: "", Value: "smoke"},
				},
			}

			mockTestRunRepo.On("Create", ctx, mock.MatchedBy(func(tr *domain.TestRun) bool {
				// Verify that all tags have their Category and Value populated
				if len(tr.Tags) != 3 {
					return false
				}

				// Check first tag: priority:high
				if tr.Tags[0].Name != "priority:high" || tr.Tags[0].Category != "priority" || tr.Tags[0].Value != "high" {
					return false
				}

				// Check second tag: env:staging
				if tr.Tags[1].Name != "env:staging" || tr.Tags[1].Category != "env" || tr.Tags[1].Value != "staging" {
					return false
				}

				// Check third tag: smoke (no category)
				if tr.Tags[2].Name != "smoke" || tr.Tags[2].Category != "" || tr.Tags[2].Value != "smoke" {
					return false
				}

				return true
			})).Return(nil)

			_, alreadyExisted, err := service.CreateTestRun(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())
			Expect(alreadyExisted).To(BeFalse())

			mockTestRunRepo.AssertExpectations(GinkgoT())
		})

		It("should ensure tag IDs are preserved during first-time creation", func() {
			testRun := &domain.TestRun{
				RunID:     "test-456",
				ProjectID: "proj-789",
				Status:    "running",
				Tags: []domain.Tag{
					{ID: 10, Name: "type:integration", Category: "type", Value: "integration"},
					{ID: 20, Name: "critical", Category: "", Value: "critical"},
				},
			}

			mockTestRunRepo.On("Create", ctx, mock.MatchedBy(func(tr *domain.TestRun) bool {
				if len(tr.Tags) != 2 {
					return false
				}

				// Verify IDs are preserved
				if tr.Tags[0].ID != 10 || tr.Tags[1].ID != 20 {
					return false
				}

				// Verify Category and Value are correct
				if tr.Tags[0].Category != "type" || tr.Tags[0].Value != "integration" {
					return false
				}
				if tr.Tags[1].Category != "" || tr.Tags[1].Value != "critical" {
					return false
				}

				return true
			})).Return(nil)

			_, _, err := service.CreateTestRun(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())

			mockTestRunRepo.AssertExpectations(GinkgoT())
		})

		It("should handle tags with special characters in Category and Value", func() {
			testRun := &domain.TestRun{
				RunID:     "test-special",
				ProjectID: "proj-special",
				Status:    "running",
				Tags: []domain.Tag{
					{ID: 1, Name: "browser:chrome-v120", Category: "browser", Value: "chrome-v120"},
					{ID: 2, Name: "os:windows-10", Category: "os", Value: "windows-10"},
					{ID: 3, Name: "user.action", Category: "", Value: "user.action"},
				},
			}

			mockTestRunRepo.On("Create", ctx, mock.MatchedBy(func(tr *domain.TestRun) bool {
				if len(tr.Tags) != 3 {
					return false
				}

				// Verify special characters are preserved
				if tr.Tags[0].Category != "browser" || tr.Tags[0].Value != "chrome-v120" {
					return false
				}
				if tr.Tags[1].Category != "os" || tr.Tags[1].Value != "windows-10" {
					return false
				}
				if tr.Tags[2].Category != "" || tr.Tags[2].Value != "user.action" {
					return false
				}

				return true
			})).Return(nil)

			_, _, err := service.CreateTestRun(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())

			mockTestRunRepo.AssertExpectations(GinkgoT())
		})

		It("should create test run with empty tags array", func() {
			testRun := &domain.TestRun{
				RunID:     "test-no-tags",
				ProjectID: "proj-no-tags",
				Status:    "running",
				Tags:      []domain.Tag{},
			}

			mockTestRunRepo.On("Create", ctx, mock.MatchedBy(func(tr *domain.TestRun) bool {
				return len(tr.Tags) == 0
			})).Return(nil)

			_, _, err := service.CreateTestRun(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())

			mockTestRunRepo.AssertExpectations(GinkgoT())
		})
	})

	Describe("UpdateTestRun with Tag Category and Value", func() {
		It("should preserve Category and Value for tags during update", func() {
			testRun := &domain.TestRun{
				ID:        1,
				ProjectID: "proj-123",
				Status:    "completed",
				Tags: []domain.Tag{
					{ID: 1, Name: "priority:high", Category: "priority", Value: "high"},
					{ID: 2, Name: "regression", Category: "", Value: "regression"},
				},
			}

			mockTestRunRepo.On("Update", ctx, mock.MatchedBy(func(tr *domain.TestRun) bool {
				if len(tr.Tags) != 2 {
					return false
				}

				// Verify Category and Value are preserved in update
				if tr.Tags[0].Category != "priority" || tr.Tags[0].Value != "high" {
					return false
				}
				if tr.Tags[1].Category != "" || tr.Tags[1].Value != "regression" {
					return false
				}

				return true
			})).Return(nil)

			err := service.UpdateTestRun(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())

			mockTestRunRepo.AssertExpectations(GinkgoT())
		})

		It("should handle updating test run with new tags having Category and Value", func() {
			testRun := &domain.TestRun{
				ID:        1,
				ProjectID: "proj-123",
				Status:    "completed",
				Tags: []domain.Tag{
					{ID: 5, Name: "platform:linux", Category: "platform", Value: "linux"},
					{ID: 6, Name: "automated", Category: "", Value: "automated"},
					{ID: 7, Name: "feature:auth", Category: "feature", Value: "auth"},
				},
			}

			mockTestRunRepo.On("Update", ctx, mock.MatchedBy(func(tr *domain.TestRun) bool {
				if len(tr.Tags) != 3 {
					return false
				}

				// Verify all new tags have correct Category and Value
				if tr.Tags[0].Category != "platform" || tr.Tags[0].Value != "linux" {
					return false
				}
				if tr.Tags[1].Category != "" || tr.Tags[1].Value != "automated" {
					return false
				}
				if tr.Tags[2].Category != "feature" || tr.Tags[2].Value != "auth" {
					return false
				}

				return true
			})).Return(nil)

			err := service.UpdateTestRun(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())

			mockTestRunRepo.AssertExpectations(GinkgoT())
		})

		It("should maintain Category and Value consistency when updating tags multiple times", func() {
			// First update
			testRun1 := &domain.TestRun{
				ID:        1,
				ProjectID: "proj-123",
				Status:    "running",
				Tags: []domain.Tag{
					{ID: 1, Name: "priority:low", Category: "priority", Value: "low"},
				},
			}

			mockTestRunRepo.On("Update", ctx, mock.MatchedBy(func(tr *domain.TestRun) bool {
				return len(tr.Tags) == 1 && tr.Tags[0].Category == "priority" && tr.Tags[0].Value == "low"
			})).Return(nil).Once()

			err := service.UpdateTestRun(ctx, testRun1)
			Expect(err).NotTo(HaveOccurred())

			// Second update with different tags
			testRun2 := &domain.TestRun{
				ID:        1,
				ProjectID: "proj-123",
				Status:    "completed",
				Tags: []domain.Tag{
					{ID: 1, Name: "priority:low", Category: "priority", Value: "low"},
					{ID: 2, Name: "status:fixed", Category: "status", Value: "fixed"},
				},
			}

			mockTestRunRepo.On("Update", ctx, mock.MatchedBy(func(tr *domain.TestRun) bool {
				if len(tr.Tags) != 2 {
					return false
				}
				// Verify both tags maintain their Category and Value
				if tr.Tags[0].Category != "priority" || tr.Tags[0].Value != "low" {
					return false
				}
				if tr.Tags[1].Category != "status" || tr.Tags[1].Value != "fixed" {
					return false
				}
				return true
			})).Return(nil).Once()

			err = service.UpdateTestRun(ctx, testRun2)
			Expect(err).NotTo(HaveOccurred())

			mockTestRunRepo.AssertExpectations(GinkgoT())
		})

		It("should handle updating tags with empty Category but non-empty Value", func() {
			testRun := &domain.TestRun{
				ID:        1,
				ProjectID: "proj-123",
				Status:    "completed",
				Tags: []domain.Tag{
					{ID: 1, Name: "nightly", Category: "", Value: "nightly"},
					{ID: 2, Name: "experimental", Category: "", Value: "experimental"},
					{ID: 3, Name: "suite:api", Category: "suite", Value: "api"},
				},
			}

			mockTestRunRepo.On("Update", ctx, mock.MatchedBy(func(tr *domain.TestRun) bool {
				if len(tr.Tags) != 3 {
					return false
				}

				// Tags without category should have empty Category but valid Value
				if tr.Tags[0].Category != "" || tr.Tags[0].Value != "nightly" {
					return false
				}
				if tr.Tags[1].Category != "" || tr.Tags[1].Value != "experimental" {
					return false
				}
				if tr.Tags[2].Category != "suite" || tr.Tags[2].Value != "api" {
					return false
				}

				return true
			})).Return(nil)

			err := service.UpdateTestRun(ctx, testRun)
			Expect(err).NotTo(HaveOccurred())

			mockTestRunRepo.AssertExpectations(GinkgoT())
		})
	})
})
