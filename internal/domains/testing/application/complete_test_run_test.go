package application_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
)

type mockTestRunRepo struct{ mock.Mock }

func (m *mockTestRunRepo) Create(ctx context.Context, testRun *domain.TestRun) error { return nil }
func (m *mockTestRunRepo) GetByID(ctx context.Context, id uint) (*domain.TestRun, error) {
	return nil, nil
}
func (m *mockTestRunRepo) GetWithDetails(ctx context.Context, id uint) (*domain.TestRun, error) {
	return nil, nil
}
func (m *mockTestRunRepo) GetLatestByProjectID(ctx context.Context, projectID string, limit int) ([]*domain.TestRun, error) {
	return nil, nil
}
func (m *mockTestRunRepo) GetTestRunSummary(ctx context.Context, projectID string) (*domain.TestRunSummary, error) {
	return nil, nil
}
func (m *mockTestRunRepo) Delete(ctx context.Context, id uint) error { return nil }
func (m *mockTestRunRepo) CountByProjectID(ctx context.Context, projectID string) (int64, error) {
	return 0, nil
}
func (m *mockTestRunRepo) GetRecent(ctx context.Context, limit int) ([]*domain.TestRun, error) {
	return nil, nil
}
func (m *mockTestRunRepo) GetByRunID(ctx context.Context, runID string) (*domain.TestRun, error) {
	args := m.Called(ctx, runID)
	return args.Get(0).(*domain.TestRun), args.Error(1)
}
func (m *mockTestRunRepo) Update(ctx context.Context, tr *domain.TestRun) error {
	args := m.Called(ctx, tr)
	return args.Error(0)
}

type mockFlakyRepo struct{ mock.Mock }

func (m *mockFlakyRepo) Save(ctx context.Context, flakyTest *domain.FlakyTest) error { return nil }
func (m *mockFlakyRepo) FindByProject(ctx context.Context, projectID string) ([]*domain.FlakyTest, error) {
	return nil, nil
}
func (m *mockFlakyRepo) FindByTestName(ctx context.Context, projectID, testName string) (*domain.FlakyTest, error) {
	return nil, nil
}
func (m *mockFlakyRepo) Update(ctx context.Context, flakyTest *domain.FlakyTest) error { return nil }

// Note: Suite entry point defined in test_run_service_test.go (TestApplication)

var _ = Describe("CompleteTestRunHandler", Label("unit", "application"), func() {
	var (
		mockRepo  *mockTestRunRepo
		mockFlaky *mockFlakyRepo
		handler   *application.CompleteTestRunHandler
	)

	BeforeEach(func() {
		mockRepo = new(mockTestRunRepo)
		mockFlaky = new(mockFlakyRepo)
		handler = application.NewCompleteTestRunHandler(mockRepo, mockFlaky)
	})

	Describe("Handle", func() {
		Context("when test run exists", func() {
			It("should complete the test run successfully", func() {
				tr := &domain.TestRun{RunID: "run-1", StartTime: time.Now().Add(-time.Hour)}
				mockRepo.On("GetByRunID", mock.Anything, "run-1").Return(tr, nil)
				mockRepo.On("Update", mock.Anything, tr).Return(nil)

				err := handler.Handle(context.Background(), application.CompleteTestRunCommand{RunID: "run-1"})

				Expect(err).ToNot(HaveOccurred())
				Expect(tr.Status).To(Equal("completed"))
				Expect(tr.EndTime).ToNot(BeNil())
				Expect(tr.Duration).To(BeNumerically(">", 0))
			})
		})

		Context("when test run is not found", func() {
			It("should return an error", func() {
				mockRepo.On("GetByRunID", mock.Anything, "run-x").Return((*domain.TestRun)(nil), nil)

				err := handler.Handle(context.Background(), application.CompleteTestRunCommand{RunID: "run-x"})

				Expect(err).To(HaveOccurred())
			})
		})

		Context("when run ID is empty", func() {
			It("should return an error", func() {
				err := handler.Handle(context.Background(), application.CompleteTestRunCommand{RunID: ""})

				Expect(err).To(HaveOccurred())
			})
		})
	})
})
