package domain

import (
	"context"
	"errors"
	"math"
	"strconv"
	"time"
)

// ErrFlakyTestNotFound tells an absent record apart from a real failure.
var ErrFlakyTestNotFound = errors.New("flaky test not found")

// ErrInvalidFlakyTestID rejects an identifier no flaky_tests row could carry.
var ErrInvalidFlakyTestID = errors.New("invalid flaky test ID")

// ParseFlakyTestRowID parses a caller-supplied row ID. 63 bits because the
// column is BIGSERIAL; the MaxUint bound stops a 32-bit uint wrapping a large
// input onto another row.
func ParseFlakyTestRowID(raw string) (uint, error) {
	id, err := strconv.ParseUint(raw, 10, 63)
	if err != nil || id == 0 || id > uint64(math.MaxUint) {
		return 0, ErrInvalidFlakyTestID
	}
	return uint(id), nil
}

// FlakyDetectionRepository defines the interface for flaky test persistence
type FlakyDetectionRepository interface {
	// Save or update a flaky test record
	SaveFlakyTest(ctx context.Context, flaky *FlakyTest) error

	// GetFlakyTestByName looks up by the natural key
	// (project_id, test_name, suite_name); there is no test_id column.
	GetFlakyTestByName(ctx context.Context, projectID string, testName string, suiteName string) (*FlakyTest, error)

	// Find flaky tests for a project
	FindFlakyTestsByProject(ctx context.Context, projectID string, status FlakyTestStatus) ([]*FlakyTest, error)

	// UpdateFlakyTestStatus updates one record by its row ID.
	UpdateFlakyTestStatus(ctx context.Context, id uint, status FlakyTestStatus) error

	// Record a test run analysis
	SaveTestRunAnalysis(ctx context.Context, analysis *TestRunAnalysis) error

	// GetTestRunHistory returns one spec's history within one suite. Suite is
	// part of the scope: each (spec, suite) gets its own flaky_tests row.
	GetTestRunHistory(ctx context.Context, projectID string, testName string, suiteName string, since time.Time) ([]TestExecutionResult, error)

	// GetUniqueTests lists the analysable units for a project.
	GetUniqueTests(ctx context.Context, projectID string, since time.Time) ([]TestIdentity, error)
}

// TestIdentity names one analysable unit: a spec within a suite. Analysis runs
// per pair rather than pooling a name's runs across suites.
type TestIdentity struct {
	TestName  string
	SuiteName string
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
