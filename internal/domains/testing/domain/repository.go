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

	// GetRecentByProjectIDs fetches recent test runs across a set of projects in one batched
	// query, sorted globally by start_time DESC. Tags only — no SuiteRuns/SpecRuns preloaded.
	// Returns the page of runs and the total count across all supplied projects.
	GetRecentByProjectIDs(ctx context.Context, projectIDs []string, limit, offset int) ([]*TestRun, int64, error)

	// GetDashboardStats returns platform-wide aggregate stats in a single query.
	GetDashboardStats(ctx context.Context) (*DashboardStatsResult, error)

	// AggregateProjectsInRange returns one row per project_id with summed
	// counts and run-count for the time window. Used by the treemap top
	// view, which only needs totals — no individual runs, no suites.
	AggregateProjectsInRange(ctx context.Context, projectIDs []string, startDate, endDate time.Time) ([]*ProjectAggregate, error)

	// AggregateSuitesInRange returns one row per suite_name for a single
	// project in the window. Used by the treemap drill-down view.
	AggregateSuitesInRange(ctx context.Context, projectID string, startDate, endDate time.Time) ([]*SuiteAggregate, error)

	// AggregateSpecsForSuiteInRange returns one row per spec_name inside
	// a single (project, suite) pair for the window. Used by the treemap
	// third-level drill (project → suite → specs). Results capped at 500
	// rows by area (DurationMs DESC) to honor the treemap node budget.
	AggregateSpecsForSuiteInRange(ctx context.Context, projectID, suiteName string, startDate, endDate time.Time) ([]*SpecAggregate, error)

	// AggregateDailyByProjects returns one row per (project_id, day) for
	// the window. Used by the Test Summaries page's trend cards, which
	// previously fanned out N parallel /api/v2/test-runs requests — one
	// per project — to compute the same sparkline data client-side.
	// This collapses to a single SQL GROUP BY.
	AggregateDailyByProjects(ctx context.Context, projectIDs []string, startDate, endDate time.Time) ([]*DailyProjectAggregate, error)
}

// ProjectAggregate is the per-project row returned by
// AggregateProjectsInRange. Values are sums over the window — there is
// no individual run-level data here by design.
type ProjectAggregate struct {
	ProjectID    string
	TotalRuns    int
	TotalTests   int
	PassedTests  int
	FailedTests  int
	SkippedTests int
	DurationMs   int64
}

// SuiteAggregate is the per-suite row returned by
// AggregateSuitesInRange.
type SuiteAggregate struct {
	SuiteName    string
	TotalTests   int
	PassedTests  int
	FailedTests  int
	SkippedTests int
	DurationMs   int64
}

// SpecAggregate is the per-spec row returned by
// AggregateSpecsForSuiteInRange. PassRate is computed as
// PassedRuns / TotalRuns when the caller needs it; for the treemap
// the boolean IsFlaky is also surfaced so flaky specs can be styled
// distinctly from steady-failing ones.
type SpecAggregate struct {
	SpecName    string
	TotalRuns   int
	PassedRuns  int
	FailedRuns  int
	SkippedRuns int
	IsFlaky     bool
	DurationMs  int64
}

// DailyProjectAggregate is the per-(project_id, day) row returned by
// AggregateDailyByProjects. `Day` is the UTC-truncated day boundary
// (00:00:00 UTC). Buckets are sparse — days with no runs are absent
// from the result; callers fill gaps client-side.
type DailyProjectAggregate struct {
	ProjectID    string
	Day          time.Time
	TotalRuns    int
	TotalTests   int
	PassedTests  int
	FailedTests  int
	SkippedTests int
	DurationMs   int64
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
