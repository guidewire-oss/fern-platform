package domain

import (
	"context"
	"errors"
	"time"
)

// ErrFlakyTestNotFound tells an absent record apart from a real failure.
var ErrFlakyTestNotFound = errors.New("flaky test not found")

// FlakyDetectionRepository defines the interface for flaky test persistence
type FlakyDetectionRepository interface {
	// Save or update a flaky test record
	SaveFlakyTest(ctx context.Context, flaky *FlakyTest) error

	// GetFlakyTestByName looks up by natural key: flaky_tests has no test_id.
	// All three columns, because the unique key is
	// (project_id, test_name, suite_name) and a name can repeat across suites.
	GetFlakyTestByName(ctx context.Context, projectID string, testName string, suiteName string) (*FlakyTest, error)

	// Find flaky tests for a project
	FindFlakyTestsByProject(ctx context.Context, projectID string, status FlakyTestStatus) ([]*FlakyTest, error)

	// UpdateFlakyTestStatus updates one record by its row ID.
	UpdateFlakyTestStatus(ctx context.Context, id uint, status FlakyTestStatus) error

	// Record a test run analysis
	SaveTestRunAnalysis(ctx context.Context, analysis *TestRunAnalysis) error

	// Get test run history for flaky detection
	GetTestRunHistory(ctx context.Context, projectID string, testName string, since time.Time) ([]TestExecutionResult, error)

	// Get unique test names for a project
	GetUniqueTestNames(ctx context.Context, projectID string, since time.Time) ([]string, error)
}

// TestExecutionResult represents a single test execution result
type TestExecutionResult struct {
	TestRunID   string
	TestName    string
	SuiteName   string
	Status      string // passed, failed, skipped
	Duration    time.Duration
	ExecutedAt  time.Time
	Error       string
	Environment map[string]string
}
