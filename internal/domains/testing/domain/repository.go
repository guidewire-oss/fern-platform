package domain

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned by repositories when a requested resource is not found
var ErrNotFound = errors.New("resource not found")

// TestRunRepository defines the interface for test run persistence
type TestRunRepository interface {
	// Create persists a new test run
	Create(ctx context.Context, testRun *TestRun) error

	// Update updates an existing test run
	Update(ctx context.Context, testRun *TestRun) error

	// GetByID retrieves a test run by ID
	GetByID(ctx context.Context, id uint) (*TestRun, error)

	// GetByRunID retrieves a test run by run ID (string)
	GetByRunID(ctx context.Context, runID string) (*TestRun, error)

	// GetWithDetails retrieves a test run with all related data
	GetWithDetails(ctx context.Context, id uint) (*TestRun, error)

	// GetLatestByProjectID retrieves the latest test runs for a project with full association preloading.
	// Use GetLatestByProjectIDTagsOnly for the chart/list path that only needs top-level fields.
	GetLatestByProjectID(ctx context.Context, projectID string, limit int) ([]*TestRun, error)

	// GetLatestByProjectIDTagsOnly retrieves the latest test runs for a project with Tags only (no SuiteRuns/SpecRuns).
	// Use this for the lazy-load chart path where only top-level run fields are needed.
	// SuiteRuns and SpecRuns will be empty slices — callers must not assume full hydration.
	GetLatestByProjectIDTagsOnly(ctx context.Context, projectID string, limit int) ([]*TestRun, error)

	// FindByDateRangeForProjects fetches test runs across multiple projects within a date range in one query.
	FindByDateRangeForProjects(ctx context.Context, projectIDs []string, startDate, endDate time.Time) ([]*TestRun, error)

	// GetTestRunSummary retrieves summary statistics for a project
	GetTestRunSummary(ctx context.Context, projectID string) (*TestRunSummary, error)

	// Delete removes a test run
	Delete(ctx context.Context, id uint) error

	// CountByProjectID counts test runs for a project
	CountByProjectID(ctx context.Context, projectID string) (int64, error)

	// GetProjectStats returns aggregated stats for a project in one query.
	GetProjectStats(ctx context.Context, projectID string) (*ProjectStatsResult, error)

	// GetRecent retrieves recent test runs across all projects
	GetRecent(ctx context.Context, limit int) ([]*TestRun, error)

	// GetDashboardStats returns platform-wide aggregate stats in a single query.
	GetDashboardStats(ctx context.Context) (*DashboardStatsResult, error)
}

// SuiteRunRepository defines the interface for suite run persistence
type SuiteRunRepository interface {
	// Create persists a new suite run
	Create(ctx context.Context, suiteRun *SuiteRun) error

	// CreateBatch creates multiple suite runs
	CreateBatch(ctx context.Context, suiteRuns []*SuiteRun) error

	// Update updates an existing suite run
	Update(ctx context.Context, suiteRun *SuiteRun) error

	// GetByID retrieves a suite run by ID
	GetByID(ctx context.Context, id uint) (*SuiteRun, error)

	// FindByTestRunID retrieves all suite runs for a test run
	FindByTestRunID(ctx context.Context, testRunID uint) ([]*SuiteRun, error)
}

// SpecRunRepository defines the interface for spec run persistence
type SpecRunRepository interface {
	// Create persists a new spec run
	Create(ctx context.Context, specRun *SpecRun) error

	// CreateBatch creates multiple spec runs
	CreateBatch(ctx context.Context, specRuns []*SpecRun) error

	// Update updates an existing spec run
	Update(ctx context.Context, specRun *SpecRun) error

	// GetByID retrieves a spec run by ID
	GetByID(ctx context.Context, id uint) (*SpecRun, error)

	// FindBySuiteRunID retrieves all spec runs for a suite
	FindBySuiteRunID(ctx context.Context, suiteRunID uint) ([]*SpecRun, error)
}

// FlakyTestRepository defines the interface for flaky test persistence
type FlakyTestRepository interface {
	// Save persists flaky test data
	Save(ctx context.Context, flakyTest *FlakyTest) error

	// FindByProject retrieves flaky tests for a project
	FindByProject(ctx context.Context, projectID string) ([]*FlakyTest, error)

	// FindByTestName retrieves flaky test history
	FindByTestName(ctx context.Context, projectID, testName string) (*FlakyTest, error)

	// Update updates flaky test statistics
	Update(ctx context.Context, flakyTest *FlakyTest) error
}
