package interfaces

import (
	"context"
	"fmt"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/application"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/guidewire-oss/fern-platform/internal/reporter/service"
	"github.com/guidewire-oss/fern-platform/pkg/database"
)

// CompatibilityAdapter adapts the new domain-driven design to the existing service interface
// This ensures backward compatibility while we migrate to DDD
type CompatibilityAdapter struct {
	recordTestRunHandler   *application.RecordTestRunHandler
	completeTestRunHandler *application.CompleteTestRunHandler
	testRunRepo            domain.TestRunRepository
}

// NewCompatibilityAdapter creates a new adapter
func NewCompatibilityAdapter(
	recordHandler *application.RecordTestRunHandler,
	completeHandler *application.CompleteTestRunHandler,
	repo domain.TestRunRepository,
) *CompatibilityAdapter {
	return &CompatibilityAdapter{
		recordTestRunHandler:   recordHandler,
		completeTestRunHandler: completeHandler,
		testRunRepo:            repo,
	}
}

// CreateTestRun adapts the existing service method to use the new
// domain. Takes a context so cancellation, deadlines, and tracing
// propagate end-to-end. Previously used context.Background(), which
// silently dropped all of those — kept as a default in the older
// CreateTestRunSync below for legacy callers.
func (a *CompatibilityAdapter) CreateTestRun(ctx context.Context, input service.CreateTestRunInput) (*database.TestRun, error) {
	// Convert to domain command
	cmd := application.RecordTestRunCommand{
		RunID:       input.RunID,
		ProjectID:   input.ProjectID,
		Branch:      input.Branch,
		CommitSHA:   input.CommitSHA,
		Environment: input.Environment,
		Metadata:    input.Metadata,
	}

	// Execute use case
	snapshot, err := a.recordTestRunHandler.Handle(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to create test run: %w", err)
	}

	// Convert back to database model for compatibility
	return &database.TestRun{
		ProjectID:    snapshot.ProjectID,
		RunID:        snapshot.RunID,
		Branch:       snapshot.Branch,
		CommitSHA:    snapshot.GitCommit,
		Status:       snapshot.Status,
		StartTime:    snapshot.StartTime,
		EndTime:      snapshot.EndTime,
		TotalTests:   snapshot.TotalTests,
		PassedTests:  snapshot.PassedTests,
		FailedTests:  snapshot.FailedTests,
		SkippedTests: snapshot.SkippedTests,
		Duration:     int64(snapshot.Duration / 1000000), // Convert to milliseconds
		Environment:  snapshot.Environment,
		Metadata:     database.JSONMap(snapshot.Metadata),
	}, nil
}

// CompleteTestRun adapts the existing service method.
func (a *CompatibilityAdapter) CompleteTestRun(ctx context.Context, runID string) error {
	cmd := application.CompleteTestRunCommand{
		RunID: runID,
	}

	return a.completeTestRunHandler.Handle(ctx, cmd)
}

// GetTestRun retrieves a test run by ID.
func (a *CompatibilityAdapter) GetTestRun(ctx context.Context, runID string) (*database.TestRun, error) {
	testRun, err := a.testRunRepo.GetByRunID(ctx, runID)
	if err != nil {
		return nil, err
	}
	if testRun == nil {
		return nil, nil
	}

	// Convert to database model
	return &database.TestRun{
		ProjectID:    testRun.ProjectID,
		RunID:        testRun.RunID,
		Branch:       testRun.Branch,
		CommitSHA:    testRun.GitCommit,
		Status:       testRun.Status,
		StartTime:    testRun.StartTime,
		EndTime:      testRun.EndTime,
		TotalTests:   testRun.TotalTests,
		PassedTests:  testRun.PassedTests,
		FailedTests:  testRun.FailedTests,
		SkippedTests: testRun.SkippedTests,
		Duration:     int64(testRun.Duration / 1000000), // Convert to milliseconds
		Environment:  testRun.Environment,
		Metadata:     database.JSONMap(testRun.Metadata),
	}, nil
}
