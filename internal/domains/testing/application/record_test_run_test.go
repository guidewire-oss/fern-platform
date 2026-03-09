package application_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
)

// recordMockTestRunRepo implements domain.TestRunRepository for RecordTestRunHandler tests
type recordMockTestRunRepo struct{ mock.Mock }

func (m *recordMockTestRunRepo) Create(ctx context.Context, testRun *domain.TestRun) error {
	args := m.Called(ctx, testRun)
	return args.Error(0)
}
func (m *recordMockTestRunRepo) Update(ctx context.Context, testRun *domain.TestRun) error {
	return nil
}
func (m *recordMockTestRunRepo) GetByID(ctx context.Context, id uint) (*domain.TestRun, error) {
	return nil, nil
}
func (m *recordMockTestRunRepo) GetByRunID(ctx context.Context, runID string) (*domain.TestRun, error) {
	return nil, nil
}
func (m *recordMockTestRunRepo) GetWithDetails(ctx context.Context, id uint) (*domain.TestRun, error) {
	return nil, nil
}
func (m *recordMockTestRunRepo) GetLatestByProjectID(ctx context.Context, projectID string, limit int) ([]*domain.TestRun, error) {
	return nil, nil
}
func (m *recordMockTestRunRepo) GetTestRunSummary(ctx context.Context, projectID string) (*domain.TestRunSummary, error) {
	return nil, nil
}
func (m *recordMockTestRunRepo) Delete(ctx context.Context, id uint) error  { return nil }
func (m *recordMockTestRunRepo) CountByProjectID(ctx context.Context, projectID string) (int64, error) {
	return 0, nil
}
func (m *recordMockTestRunRepo) Count(ctx context.Context) (int64, error) { return 0, nil }
func (m *recordMockTestRunRepo) GetRecent(ctx context.Context, limit int) ([]*domain.TestRun, error) {
	return nil, nil
}
func (m *recordMockTestRunRepo) List(ctx context.Context, limit, offset int) ([]*domain.TestRun, int64, error) {
	return nil, 0, nil
}

// Note: Suite entry point is defined in test_run_service_test.go (TestApplication)

var _ = Describe("RecordTestRunHandler", Label("unit", "application"), func() {
	var (
		mockRepo *recordMockTestRunRepo
		handler  *application.RecordTestRunHandler
	)

	BeforeEach(func() {
		mockRepo = new(recordMockTestRunRepo)
		handler = application.NewRecordTestRunHandler(mockRepo)
	})

	Describe("Handle", func() {
		Context("with a valid command", func() {
			It("should create and return a test run", func() {
				mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.TestRun")).Return(nil)

				cmd := application.RecordTestRunCommand{
					RunID:       "run-123",
					ProjectID:   "proj-456",
					Branch:      "main",
					CommitSHA:   "abc123",
					Environment: "staging",
					Metadata:    map[string]interface{}{"key": "value"},
				}

				result, err := handler.Handle(context.Background(), cmd)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result.RunID).To(Equal("run-123"))
				Expect(result.ProjectID).To(Equal("proj-456"))
				Expect(result.Branch).To(Equal("main"))
				Expect(result.GitCommit).To(Equal("abc123"))
				Expect(result.Environment).To(Equal("staging"))
				Expect(result.Status).To(Equal("running"))
				Expect(result.StartTime).ToNot(BeZero())
				Expect(result.Metadata).To(HaveKeyWithValue("key", "value"))
				mockRepo.AssertExpectations(GinkgoT())
			})
		})

		Context("when RunID is missing", func() {
			It("should return a validation error", func() {
				cmd := application.RecordTestRunCommand{
					ProjectID: "proj-456",
					Branch:    "main",
				}

				result, err := handler.Handle(context.Background(), cmd)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("run ID is required"))
				Expect(result).To(BeNil())
			})
		})

		Context("when ProjectID is missing", func() {
			It("should return a validation error", func() {
				cmd := application.RecordTestRunCommand{
					RunID:  "run-123",
					Branch: "main",
				}

				result, err := handler.Handle(context.Background(), cmd)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("project ID is required"))
				Expect(result).To(BeNil())
			})
		})

		Context("when Branch is missing", func() {
			It("should return a validation error", func() {
				cmd := application.RecordTestRunCommand{
					RunID:     "run-123",
					ProjectID: "proj-456",
				}

				result, err := handler.Handle(context.Background(), cmd)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("branch is required"))
				Expect(result).To(BeNil())
			})
		})

		Context("when repository Create fails", func() {
			It("should return an error", func() {
				mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.TestRun")).
					Return(errors.New("database error"))

				cmd := application.RecordTestRunCommand{
					RunID:     "run-123",
					ProjectID: "proj-456",
					Branch:    "main",
				}

				result, err := handler.Handle(context.Background(), cmd)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to save test run"))
				Expect(result).To(BeNil())
				mockRepo.AssertExpectations(GinkgoT())
			})
		})

		Context("with optional fields omitted", func() {
			It("should still create a test run with defaults", func() {
				mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.TestRun")).Return(nil)

				cmd := application.RecordTestRunCommand{
					RunID:     "run-minimal",
					ProjectID: "proj-1",
					Branch:    "develop",
				}

				result, err := handler.Handle(context.Background(), cmd)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).ToNot(BeNil())
				Expect(result.GitCommit).To(BeEmpty())
				Expect(result.Environment).To(BeEmpty())
				Expect(result.Metadata).To(BeNil())
				Expect(result.Status).To(Equal("running"))
			})
		})
	})

	Describe("NewRecordTestRunHandler", func() {
		It("should create a handler with the provided repo", func() {
			h := application.NewRecordTestRunHandler(mockRepo)
			Expect(h).ToNot(BeNil())
		})
	})
})
