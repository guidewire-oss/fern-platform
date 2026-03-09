package interfaces_test

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/interfaces"
	"github.com/guidewire-oss/fern-platform/internal/reporter/service"
)

// --- Mock repositories for building handlers ---

type compatMockTestRunRepo struct{ mock.Mock }

func (m *compatMockTestRunRepo) Create(ctx context.Context, testRun *domain.TestRun) error {
	args := m.Called(ctx, testRun)
	return args.Error(0)
}
func (m *compatMockTestRunRepo) Update(ctx context.Context, testRun *domain.TestRun) error {
	args := m.Called(ctx, testRun)
	return args.Error(0)
}
func (m *compatMockTestRunRepo) GetByID(ctx context.Context, id uint) (*domain.TestRun, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TestRun), args.Error(1)
}
func (m *compatMockTestRunRepo) GetByRunID(ctx context.Context, runID string) (*domain.TestRun, error) {
	args := m.Called(ctx, runID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TestRun), args.Error(1)
}
func (m *compatMockTestRunRepo) GetWithDetails(ctx context.Context, id uint) (*domain.TestRun, error) {
	return nil, nil
}
func (m *compatMockTestRunRepo) GetLatestByProjectID(ctx context.Context, projectID string, limit int) ([]*domain.TestRun, error) {
	return nil, nil
}
func (m *compatMockTestRunRepo) GetTestRunSummary(ctx context.Context, projectID string) (*domain.TestRunSummary, error) {
	return nil, nil
}
func (m *compatMockTestRunRepo) Delete(ctx context.Context, id uint) error { return nil }
func (m *compatMockTestRunRepo) CountByProjectID(ctx context.Context, projectID string) (int64, error) {
	return 0, nil
}
func (m *compatMockTestRunRepo) Count(ctx context.Context) (int64, error) { return 0, nil }
func (m *compatMockTestRunRepo) GetRecent(ctx context.Context, limit int) ([]*domain.TestRun, error) {
	return nil, nil
}
func (m *compatMockTestRunRepo) List(ctx context.Context, limit, offset int) ([]*domain.TestRun, int64, error) {
	return nil, 0, nil
}

type compatMockFlakyRepo struct{ mock.Mock }

func (m *compatMockFlakyRepo) Save(ctx context.Context, flakyTest *domain.FlakyTest) error {
	return nil
}
func (m *compatMockFlakyRepo) FindByProject(ctx context.Context, projectID string) ([]*domain.FlakyTest, error) {
	return nil, nil
}
func (m *compatMockFlakyRepo) FindByTestName(ctx context.Context, projectID, testName string) (*domain.FlakyTest, error) {
	return nil, nil
}
func (m *compatMockFlakyRepo) Update(ctx context.Context, flakyTest *domain.FlakyTest) error {
	return nil
}

var _ = Describe("CompatibilityAdapter", func() {
	var (
		mockRepo      *compatMockTestRunRepo
		mockFlaky     *compatMockFlakyRepo
		recordHandler *application.RecordTestRunHandler
		completeHandler *application.CompleteTestRunHandler
		adapter       *interfaces.CompatibilityAdapter
	)

	BeforeEach(func() {
		mockRepo = new(compatMockTestRunRepo)
		mockFlaky = new(compatMockFlakyRepo)
		recordHandler = application.NewRecordTestRunHandler(mockRepo)
		completeHandler = application.NewCompleteTestRunHandler(mockRepo, mockFlaky)
		adapter = interfaces.NewCompatibilityAdapter(recordHandler, completeHandler, mockRepo)
	})

	Describe("NewCompatibilityAdapter", func() {
		It("should create an adapter", func() {
			Expect(adapter).ToNot(BeNil())
		})
	})

	Describe("CreateTestRun", func() {
		Context("when creation succeeds", func() {
			It("should convert input to domain command and return database model", func() {
				mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.TestRun")).Return(nil)

				input := service.CreateTestRunInput{
					RunID:       "run-1",
					ProjectID:   "proj-1",
					Branch:      "main",
					CommitSHA:   "abc123",
					Environment: "staging",
				}

				result, err := adapter.CreateTestRun(input)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result.RunID).To(Equal("run-1"))
				Expect(result.ProjectID).To(Equal("proj-1"))
				Expect(result.Branch).To(Equal("main"))
				Expect(result.CommitSHA).To(Equal("abc123"))
				Expect(result.Environment).To(Equal("staging"))
				Expect(result.Status).To(Equal("running"))
			})
		})

		Context("when record handler fails", func() {
			It("should return an error", func() {
				mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.TestRun")).
					Return(errors.New("db error"))

				input := service.CreateTestRunInput{
					RunID:     "run-1",
					ProjectID: "proj-1",
					Branch:    "main",
				}

				result, err := adapter.CreateTestRun(input)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to create test run"))
				Expect(result).To(BeNil())
			})
		})

		Context("when input has validation errors (missing required fields)", func() {
			It("should return an error for missing RunID", func() {
				input := service.CreateTestRunInput{
					ProjectID: "proj-1",
					Branch:    "main",
				}

				result, err := adapter.CreateTestRun(input)

				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})
		})
	})

	Describe("CompleteTestRun", func() {
		Context("when test run exists", func() {
			It("should complete the test run", func() {
				tr := &domain.TestRun{
					RunID:     "run-1",
					Status:    "running",
					StartTime: time.Now().Add(-time.Hour),
				}
				mockRepo.On("GetByRunID", mock.Anything, "run-1").Return(tr, nil)
				mockRepo.On("Update", mock.Anything, tr).Return(nil)

				err := adapter.CompleteTestRun("run-1")

				Expect(err).ToNot(HaveOccurred())
				Expect(tr.Status).To(Equal("completed"))
				Expect(tr.EndTime).ToNot(BeNil())
			})
		})

		Context("when run ID is empty", func() {
			It("should return an error", func() {
				err := adapter.CompleteTestRun("")

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when test run is not found", func() {
			It("should return an error", func() {
				mockRepo.On("GetByRunID", mock.Anything, "nonexistent").Return((*domain.TestRun)(nil), nil)

				err := adapter.CompleteTestRun("nonexistent")

				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("GetTestRun", func() {
		Context("when test run exists", func() {
			It("should return the database model", func() {
				now := time.Now()
				endTime := now.Add(time.Hour)
				tr := &domain.TestRun{
					RunID:        "run-1",
					ProjectID:    "proj-1",
					Branch:       "main",
					GitCommit:    "abc123",
					Status:       "completed",
					StartTime:    now,
					EndTime:      &endTime,
					TotalTests:   10,
					PassedTests:  8,
					FailedTests:  1,
					SkippedTests: 1,
					Duration:     time.Duration(3600000000000), // 1 hour in ns
					Environment:  "prod",
				}
				mockRepo.On("GetByRunID", mock.Anything, "run-1").Return(tr, nil)

				result, err := adapter.GetTestRun("run-1")

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result.RunID).To(Equal("run-1"))
				Expect(result.ProjectID).To(Equal("proj-1"))
				Expect(result.TotalTests).To(Equal(10))
				Expect(result.PassedTests).To(Equal(8))
				Expect(result.FailedTests).To(Equal(1))
				Expect(result.SkippedTests).To(Equal(1))
				Expect(result.Duration).To(Equal(int64(3600000))) // converted to ms
			})
		})

		Context("when test run is not found", func() {
			It("should return nil", func() {
				mockRepo.On("GetByRunID", mock.Anything, "missing").Return((*domain.TestRun)(nil), nil)

				result, err := adapter.GetTestRun("missing")

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(BeNil())
			})
		})

		Context("when repository returns an error", func() {
			It("should propagate the error", func() {
				mockRepo.On("GetByRunID", mock.Anything, "err-run").Return((*domain.TestRun)(nil), errors.New("db error"))

				result, err := adapter.GetTestRun("err-run")

				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})
		})
	})
})
