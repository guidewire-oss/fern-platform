package application

import (
	"context"
	"testing"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

func TestCompleteTestRunHandler_Handle(t *testing.T) {
	mockRepo := new(mockTestRunRepo)
	mockFlaky := new(mockFlakyRepo)
	h := NewCompleteTestRunHandler(mockRepo, mockFlaky)

	tr := &domain.TestRun{RunID: "run-1", StartTime: time.Now().Add(-time.Hour)}
	mockRepo.On("GetByRunID", mock.Anything, "run-1").Return(tr, nil)
	mockRepo.On("Update", mock.Anything, tr).Return(nil)

	err := h.Handle(context.Background(), CompleteTestRunCommand{RunID: "run-1"})
	assert.NoError(t, err)
	assert.Equal(t, "completed", tr.Status)
	assert.NotNil(t, tr.EndTime)
	assert.True(t, tr.Duration > 0)
}

func TestCompleteTestRunHandler_Handle_NotFound(t *testing.T) {
	mockRepo := new(mockTestRunRepo)
	mockFlaky := new(mockFlakyRepo)
	h := NewCompleteTestRunHandler(mockRepo, mockFlaky)
	mockRepo.On("GetByRunID", mock.Anything, "run-x").Return((*domain.TestRun)(nil), nil)

	err := h.Handle(context.Background(), CompleteTestRunCommand{RunID: "run-x"})
	assert.Error(t, err)
}

func TestCompleteTestRunHandler_Handle_EmptyRunID(t *testing.T) {
	mockRepo := new(mockTestRunRepo)
	mockFlaky := new(mockFlakyRepo)
	h := NewCompleteTestRunHandler(mockRepo, mockFlaky)

	err := h.Handle(context.Background(), CompleteTestRunCommand{RunID: ""})
	assert.Error(t, err)
}
