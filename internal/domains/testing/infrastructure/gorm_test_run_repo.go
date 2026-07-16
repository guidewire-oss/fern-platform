package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	testingSQL "github.com/guidewire-oss/fern-platform/internal/domains/testing/sql"
	"github.com/guidewire-oss/fern-platform/pkg/database"
	"gorm.io/gorm"
)

// GormTestRunRepository implements domain.TestRunRepository using GORM
type GormTestRunRepository struct {
	db        *gorm.DB
	converter *DatabaseConverter
}

// NewGormTestRunRepository creates a new GORM-based test run repository
func NewGormTestRunRepository(db *gorm.DB) *GormTestRunRepository {
	return &GormTestRunRepository{db: db,
		converter: NewDatabaseConverter()}
}

// Create creates a new test run
func (r *GormTestRunRepository) Create(ctx context.Context, testRun *domain.TestRun) error {
	// Convert domain SuiteRuns to database SuiteRuns
	dbTestRun := r.converter.ConvertTestRunToDatabase(testRun)

	// Use FullSaveAssociations to ensure nested suites, specs, and tags are all saved
	if err := r.db.WithContext(ctx).Session(&gorm.Session{FullSaveAssociations: true}).Create(dbTestRun).Error; err != nil {
		return fmt.Errorf("failed to create test run: %w", err)
	}

	testRun.ID = dbTestRun.ID
	return nil
}

// Update updates an existing test run
func (r *GormTestRunRepository) Update(ctx context.Context, testRun *domain.TestRun) error {
	updates := map[string]interface{}{
		"status":        testRun.Status,
		"end_time":      testRun.EndTime,
		"duration_ms":   int64(testRun.Duration / time.Millisecond),
		"total_tests":   testRun.TotalTests,
		"passed_tests":  testRun.PassedTests,
		"failed_tests":  testRun.FailedTests,
		"skipped_tests": testRun.SkippedTests,
		"updated_at":    time.Now(),
	}

	// ✅ Only include metadata if it’s non-nil and convertible
	if testRun.Metadata != nil {
		switch m := any(testRun.Metadata).(type) {
		case map[string]interface{}:
			updates["metadata"] = database.JSONMap(m)
		case *map[string]interface{}:
			if m != nil {
				updates["metadata"] = database.JSONMap(*m)
			}
		}
	}

	result := r.db.WithContext(ctx).Model(&database.TestRun{}).Where("id = ?", testRun.ID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update test run: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("test run not found")
	}

	return nil
}

// GetByID retrieves a test run by ID
func (r *GormTestRunRepository) GetByID(ctx context.Context, id uint) (*domain.TestRun, error) {
	var dbTestRun database.TestRun
	if err := r.db.WithContext(ctx).
		Preload("Tags").
		Preload("SuiteRuns").
		Preload("SuiteRuns.Tags").
		Preload("SuiteRuns.SpecRuns").
		Preload("SuiteRuns.SpecRuns.Tags").
		First(&dbTestRun, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("test run not found")
		}
		return nil, fmt.Errorf("failed to get test run: %w", err)
	}

	return r.converter.ConvertTestRunToDomain(&dbTestRun), nil
}

// GetByRunID retrieves a test run by run ID (string)
func (r *GormTestRunRepository) GetByRunID(ctx context.Context, runID string) (*domain.TestRun, error) {
	var dbTestRun database.TestRun
	if err := r.db.WithContext(ctx).
		Preload("Tags").
		Preload("SuiteRuns").
		Preload("SuiteRuns.Tags").
		Preload("SuiteRuns.SpecRuns").
		Preload("SuiteRuns.SpecRuns.Tags").
		Where("run_id = ?", runID).First(&dbTestRun).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("test run not found")
		}
		return nil, fmt.Errorf("failed to get test run: %w", err)
	}

	return r.converter.ConvertTestRunToDomain(&dbTestRun), nil
}

// GetByProjectID retrieves all test runs for a project
func (r *GormTestRunRepository) GetByProjectID(ctx context.Context, projectID string) ([]*domain.TestRun, error) {
	var dbTestRuns []database.TestRun
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("start_time DESC").Find(&dbTestRuns).Error; err != nil {
		return nil, fmt.Errorf("failed to get test runs: %w", err)
	}

	testRuns := make([]*domain.TestRun, len(dbTestRuns))
	for i, dbTestRun := range dbTestRuns {
		testRuns[i] = r.converter.ConvertTestRunToDomain(&dbTestRun)
	}

	return testRuns, nil
}

// GetLatestByProjectID retrieves the latest test runs for a project.
// Eagerly loads all associations for the project detail view.
func (r *GormTestRunRepository) GetLatestByProjectID(ctx context.Context, projectID string, limit int) ([]*domain.TestRun, error) {
	if limit < 0 {
		return nil, fmt.Errorf("limit must be non-negative")
	}

	var dbTestRuns []database.TestRun
	query := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Preload("Tags").
		Preload("SuiteRuns").
		Preload("SuiteRuns.Tags").
		Preload("SuiteRuns.SpecRuns").
		Preload("SuiteRuns.SpecRuns.Tags").
		// Order by start_time (when tests actually ran), not created_at (insertion time).
		Order("start_time DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&dbTestRuns).Error; err != nil {
		return nil, fmt.Errorf("failed to get latest test runs: %w", err)
	}

	testRuns := make([]*domain.TestRun, len(dbTestRuns))
	for i, dbTestRun := range dbTestRuns {
		testRuns[i] = r.converter.ConvertTestRunToDomain(&dbTestRun)
	}

	return testRuns, nil
}

// GetLatestByProjectIDTagsOnly retrieves the latest test runs for a project loading only Tags.
// SuiteRuns and SpecRuns are intentionally omitted — use for the lazy-load chart path only.
func (r *GormTestRunRepository) GetLatestByProjectIDTagsOnly(ctx context.Context, projectID string, limit int) ([]*domain.TestRun, error) {
	if limit < 0 {
		return nil, fmt.Errorf("limit must be non-negative")
	}

	var dbTestRuns []database.TestRun
	query := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Preload("Tags").
		Order("start_time DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&dbTestRuns).Error; err != nil {
		return nil, fmt.Errorf("failed to get latest test runs summary: %w", err)
	}

	testRuns := make([]*domain.TestRun, len(dbTestRuns))
	for i, dbTestRun := range dbTestRuns {
		testRuns[i] = r.converter.ConvertTestRunToDomain(&dbTestRun)
	}

	return testRuns, nil
}

// GetWithDetails retrieves a test run with all its suites and specs
func (r *GormTestRunRepository) GetWithDetails(ctx context.Context, id uint) (*domain.TestRun, error) {
	var dbTestRun database.TestRun
	if err := r.db.WithContext(ctx).
		Preload("Tags").
		Preload("SuiteRuns").
		Preload("SuiteRuns.Tags").
		Preload("SuiteRuns.SpecRuns").
		Preload("SuiteRuns.SpecRuns.Tags").
		First(&dbTestRun, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("test run not found")
		}
		return nil, fmt.Errorf("failed to get test run with details: %w", err)
	}

	return r.converter.ConvertTestRunToDomain(&dbTestRun), nil
}

// FindByDateRange finds test runs within a date range.
// Filters on start_time (when the run actually occurred) to match FindByDateRangeForProjects.
func (r *GormTestRunRepository) FindByDateRange(ctx context.Context, projectID string, startDate, endDate time.Time) ([]*domain.TestRun, error) {
	var dbTestRuns []database.TestRun
	query := r.db.WithContext(ctx).Where("project_id = ? AND start_time >= ? AND start_time <= ?", projectID, startDate, endDate).Order("start_time DESC")

	if err := query.Find(&dbTestRuns).Error; err != nil {
		return nil, fmt.Errorf("failed to find test runs by date range: %w", err)
	}

	testRuns := make([]*domain.TestRun, len(dbTestRuns))
	for i, dbTestRun := range dbTestRuns {
		testRuns[i] = r.converter.ConvertTestRunToDomain(&dbTestRun)
	}

	return testRuns, nil
}

// FindByDateRangeForProjects fetches test runs for multiple projects within a date range.
// Preloads SuiteRuns (names, counts, durations) but not SuiteRuns.Tags or SpecRuns — optimized for
// aggregate treemap views. SpecRuns are not preloaded; callers requiring full hydration should use
// a different method.
//
// At scale (e.g. 1000 projects × 6 months) the matching test_run row count can run into the
// millions. GORM's default `Preload` would issue follow-up queries of the form
// `WHERE test_run_id IN ($1, $2, …, $N)` over every matched id — easily exceeding Postgres'
// 65535-parameter hard limit on the extended protocol. We hand-chunk the preload so each
// follow-up query stays well under that ceiling.
const preloadChunkSize = 5000

func (r *GormTestRunRepository) FindByDateRangeForProjects(ctx context.Context, projectIDs []string, startDate, endDate time.Time) ([]*domain.TestRun, error) {
	if len(projectIDs) == 0 {
		return nil, nil
	}

	// 1. Main query — no Preload, just the test_run rows themselves.
	var dbTestRuns []database.TestRun
	if err := r.db.WithContext(ctx).
		Where("project_id IN ? AND start_time >= ? AND start_time <= ?", projectIDs, startDate, endDate).
		Order("start_time DESC").
		Find(&dbTestRuns).Error; err != nil {
		return nil, fmt.Errorf("failed to find test runs by date range: %w", err)
	}
	if len(dbTestRuns) == 0 {
		return nil, nil
	}

	// 2. Build the id list and an index for attaching preloaded data.
	ids := make([]uint, len(dbTestRuns))
	indexByID := make(map[uint]int, len(dbTestRuns))
	for i, run := range dbTestRuns {
		ids[i] = run.ID
		indexByID[run.ID] = i
	}

	// 3. Chunked preload of SuiteRuns — bucket by test_run_id and assign.
	for start := 0; start < len(ids); start += preloadChunkSize {
		end := start + preloadChunkSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]
		var suites []database.SuiteRun
		if err := r.db.WithContext(ctx).
			Where("test_run_id IN ?", chunk).
			Find(&suites).Error; err != nil {
			return nil, fmt.Errorf("failed to preload suite_runs: %w", err)
		}
		for _, s := range suites {
			if idx, ok := indexByID[s.TestRunID]; ok {
				dbTestRuns[idx].SuiteRuns = append(dbTestRuns[idx].SuiteRuns, s)
			}
		}
	}

	// 4. Chunked preload of Tags via the test_run_tags junction.
	for start := 0; start < len(ids); start += preloadChunkSize {
		end := start + preloadChunkSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]
		type tagJoinRow struct {
			database.Tag
			TestRunID uint `gorm:"column:test_run_id"`
		}
		var rows []tagJoinRow
		if err := r.db.WithContext(ctx).
			Table("tags").
			Select("tags.*, test_run_tags.test_run_id AS test_run_id").
			Joins("JOIN test_run_tags ON test_run_tags.tag_id = tags.id").
			Where("test_run_tags.test_run_id IN ?", chunk).
			Scan(&rows).Error; err != nil {
			return nil, fmt.Errorf("failed to preload test_run tags: %w", err)
		}
		for _, t := range rows {
			if idx, ok := indexByID[t.TestRunID]; ok {
				dbTestRuns[idx].Tags = append(dbTestRuns[idx].Tags, t.Tag)
			}
		}
	}

	// 5. Convert to domain.
	testRuns := make([]*domain.TestRun, len(dbTestRuns))
	for i, dbTestRun := range dbTestRuns {
		testRuns[i] = r.converter.ConvertTestRunToDomain(&dbTestRun)
	}
	return testRuns, nil
}

// GetRecentByProjectIDs fetches recent test runs across a set of projects in one batched
// query sorted globally by start_time DESC. Tags only — no SuiteRuns/SpecRuns preloaded.
func (r *GormTestRunRepository) GetRecentByProjectIDs(ctx context.Context, projectIDs []string, limit, offset int) ([]*domain.TestRun, int64, error) {
	if len(projectIDs) == 0 {
		return nil, 0, nil
	}
	if offset < 0 {
		offset = 0
	}
	var total int64
	if err := r.db.WithContext(ctx).Model(&database.TestRun{}).
		Where("project_id IN ?", projectIDs).
		Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count test runs by project IDs: %w", err)
	}
	var dbTestRuns []database.TestRun
	if err := r.db.WithContext(ctx).
		Where("project_id IN ?", projectIDs).
		Preload("Tags").
		Order("start_time DESC").
		Limit(limit).Offset(offset).
		Find(&dbTestRuns).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to find recent test runs by project IDs: %w", err)
	}
	runs := make([]*domain.TestRun, len(dbTestRuns))
	for i, dbRun := range dbTestRuns {
		runs[i] = r.converter.ConvertTestRunToDomain(&dbRun)
	}
	return runs, total, nil
}

// GetTestRunSummary retrieves summary statistics for a project
func (r *GormTestRunRepository) GetTestRunSummary(ctx context.Context, projectID string) (*domain.TestRunSummary, error) {
	var summary domain.TestRunSummary

	// Get total runs
	var totalCount int64
	if err := r.db.WithContext(ctx).Model(&database.TestRun{}).Where("project_id = ?", projectID).Count(&totalCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count total runs: %w", err)
	}
	summary.TotalRuns = int(totalCount)

	// Get passed runs
	var passedCount int64
	if err := r.db.WithContext(ctx).Model(&database.TestRun{}).Where("project_id = ? AND status = ?", projectID, "passed").Count(&passedCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count passed runs: %w", err)
	}
	summary.PassedRuns = int(passedCount)

	// Get failed runs
	var failedCount int64
	if err := r.db.WithContext(ctx).Model(&database.TestRun{}).Where("project_id = ? AND status = ?", projectID, "failed").Count(&failedCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count failed runs: %w", err)
	}
	summary.FailedRuns = int(failedCount)

	// Get average duration
	var avgDuration float64
	if err := r.db.WithContext(ctx).Model(&database.TestRun{}).Where("project_id = ?", projectID).Select("COALESCE(AVG(duration_ms), 0)").Scan(&avgDuration).Error; err != nil {
		return nil, fmt.Errorf("failed to get average duration: %w", err)
	}
	summary.AverageRunTime = time.Duration(avgDuration) * time.Millisecond

	// Calculate success rate
	if summary.TotalRuns > 0 {
		summary.SuccessRate = float64(summary.PassedRuns) / float64(summary.TotalRuns)
	}

	return &summary, nil
}

// Delete removes a test run
func (r *GormTestRunRepository) Delete(ctx context.Context, id uint) error {
	return r.db.Delete(&database.TestRun{}, id).Error
}

// CountByProjectID counts test runs for a project
func (r *GormTestRunRepository) CountByProjectID(ctx context.Context, projectID string) (int64, error) {
	var count int64
	err := r.db.Model(&database.TestRun{}).
		Where("project_id = ?", projectID).
		Count(&count).Error
	return count, err
}

// GetProjectStats returns aggregated stats for a project in a single query.
func (r *GormTestRunRepository) GetProjectStats(ctx context.Context, projectID string) (*domain.ProjectStatsResult, error) {
	type aggRow struct {
		TotalRuns      int64
		AvgDurationMs  float64
		PassedRuns     int64
		TotalTests     int64
		PassedTests    int64
		UniqueBranches int64
	}
	var agg aggRow
	err := r.db.WithContext(ctx).
		Model(&database.TestRun{}).
		Where("project_id = ?", projectID).
		Select(`
			COUNT(*) as total_runs,
			COALESCE(AVG(duration_ms), 0) as avg_duration_ms,
			`+testingSQL.PassedRunSumSQL+` as passed_runs,
			COALESCE(SUM(total_tests),  0) as total_tests,
			COALESCE(SUM(passed_tests), 0) as passed_tests,
			COUNT(DISTINCT NULLIF(branch, '')) as unique_branches
		`).
		Scan(&agg).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get project stats: %w", err)
	}

	// Fetch last run time in a separate query so the GORM model scanner handles
	// time parsing correctly — MAX(start_time) in a raw SQL Scan returns a plain
	// string in SQLite, which the driver cannot automatically coerce to time.Time.
	// The resulting inconsistency window (a run inserted between the two queries)
	// is intentionally accepted as an acceptable trade-off over a more complex
	// single-query approach.
	var lastRun database.TestRun
	var lastRunTime *time.Time
	err = r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("start_time DESC").
		Select("start_time").
		First(&lastRun).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to get project stats: %w", err)
	}
	if err == nil {
		lastRunTime = &lastRun.StartTime
	}

	return &domain.ProjectStatsResult{
		TotalRuns:      agg.TotalRuns,
		AvgDurationMs:  agg.AvgDurationMs,
		PassedRuns:     agg.PassedRuns,
		TotalTests:     agg.TotalTests,
		PassedTests:    agg.PassedTests,
		UniqueBranches: agg.UniqueBranches,
		LastRunTime:    lastRunTime,
	}, nil
}

// GetRecent retrieves recent test runs across all projects.
// Intentionally loads only Tags — SuiteRuns/SpecRuns are deferred to field resolvers.
func (r *GormTestRunRepository) GetRecent(ctx context.Context, limit int) ([]*domain.TestRun, error) {
	if limit < 0 {
		return nil, fmt.Errorf("limit must be non-negative")
	}

	var dbTestRuns []database.TestRun
	query := r.db.WithContext(ctx).
		Model(&database.TestRun{}).
		Preload("Tags").
		Order("start_time DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&dbTestRuns).Error; err != nil {
		return nil, fmt.Errorf("failed to get recent test runs: %w", err)
	}

	testRuns := make([]*domain.TestRun, len(dbTestRuns))
	for i, dbTestRun := range dbTestRuns {
		testRuns[i] = r.converter.ConvertTestRunToDomain(&dbTestRun)
	}

	return testRuns, nil
}

// GetDashboardStats returns platform-wide aggregate stats in a single query.
func (r *GormTestRunRepository) GetDashboardStats(ctx context.Context) (*domain.DashboardStatsResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	type aggRow struct {
		TotalTestRuns      int64
		RecentTestRuns     int64
		TotalTestsExecuted int64
		PassedTests        int64
		AvgDurationMs      float64
	}
	cutoff := time.Now().Add(-testingSQL.DefaultDashboardRecentWindowHours * time.Hour)
	var agg aggRow
	err := r.db.WithContext(ctx).
		Model(&database.TestRun{}).
		Select(`
			COUNT(*) as total_test_runs,
			COALESCE(SUM(CASE WHEN start_time >= ? THEN 1 ELSE 0 END), 0) as recent_test_runs,
			COALESCE(SUM(total_tests), 0) as total_tests_executed,
			COALESCE(SUM(passed_tests), 0) as passed_tests,
			COALESCE(AVG(duration_ms), 0) as avg_duration_ms
		`, cutoff).
		Scan(&agg).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get dashboard stats: %w", err)
	}
	return &domain.DashboardStatsResult{
		TotalTestRuns:      agg.TotalTestRuns,
		RecentTestRuns:     agg.RecentTestRuns,
		TotalTestsExecuted: agg.TotalTestsExecuted,
		PassedTests:        agg.PassedTests,
		AvgDurationMs:      agg.AvgDurationMs,
	}, nil
}

// GetDB returns the underlying GORM DB instance.
// This allows higher-level services to perform association updates (e.g., tags).
func (r *GormTestRunRepository) GetDB() *gorm.DB {
	return r.db
}

// AggregateProjectsInRange returns one row per project_id with summed
// totals over the window. The treemap top view only needs these sums;
// hydrating every test_run + its suite_runs (the legacy path) wasted
// most of its work because suite-level data is discarded until the
// user drills into a single project.
func (r *GormTestRunRepository) AggregateProjectsInRange(
	ctx context.Context, projectIDs []string, startDate, endDate time.Time,
) ([]*domain.ProjectAggregate, error) {
	if len(projectIDs) == 0 {
		return nil, nil
	}
	type row struct {
		ProjectID    string
		TotalRuns    int
		TotalTests   int
		PassedTests  int
		FailedTests  int
		SkippedTests int
		DurationMs   int64
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Table("test_runs").
		Select(`
			project_id            AS project_id,
			COUNT(*)              AS total_runs,
			COALESCE(SUM(total_tests),   0) AS total_tests,
			COALESCE(SUM(passed_tests),  0) AS passed_tests,
			COALESCE(SUM(failed_tests),  0) AS failed_tests,
			COALESCE(SUM(skipped_tests), 0) AS skipped_tests,
			COALESCE(SUM(duration_ms),   0) AS duration_ms
		`).
		Where("project_id IN ?", projectIDs).
		Where("start_time >= ? AND start_time <= ?", startDate, endDate).
		Group("project_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*domain.ProjectAggregate, len(rows))
	for i, r := range rows {
		out[i] = &domain.ProjectAggregate{
			ProjectID:    r.ProjectID,
			TotalRuns:    r.TotalRuns,
			TotalTests:   r.TotalTests,
			PassedTests:  r.PassedTests,
			FailedTests:  r.FailedTests,
			SkippedTests: r.SkippedTests,
			DurationMs:   r.DurationMs,
		}
	}
	return out, nil
}

// AggregateSuitesInRange returns one row per suite_name for a single
// project's drill-down view. Joins suite_runs to test_runs to scope by
// start_time on the parent run (suite_runs has its own start_time but
// the treemap's "last 7 days" is a run-level concept).
func (r *GormTestRunRepository) AggregateSuitesInRange(
	ctx context.Context, projectID string, startDate, endDate time.Time,
) ([]*domain.SuiteAggregate, error) {
	type row struct {
		SuiteName    string
		TotalTests   int
		PassedTests  int
		FailedTests  int
		SkippedTests int
		DurationMs   int64
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Table("suite_runs AS sr").
		Select(`
			sr.suite_name                       AS suite_name,
			COALESCE(SUM(sr.total_specs),   0)  AS total_tests,
			COALESCE(SUM(sr.passed_specs),  0)  AS passed_tests,
			COALESCE(SUM(sr.failed_specs),  0)  AS failed_tests,
			COALESCE(SUM(sr.skipped_specs), 0)  AS skipped_tests,
			COALESCE(SUM(sr.duration_ms),   0)  AS duration_ms
		`).
		Joins("JOIN test_runs tr ON tr.id = sr.test_run_id").
		Where("tr.project_id = ?", projectID).
		Where("tr.start_time >= ? AND tr.start_time <= ?", startDate, endDate).
		Group("sr.suite_name").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*domain.SuiteAggregate, len(rows))
	for i, r := range rows {
		out[i] = &domain.SuiteAggregate{
			SuiteName:    r.SuiteName,
			TotalTests:   r.TotalTests,
			PassedTests:  r.PassedTests,
			FailedTests:  r.FailedTests,
			SkippedTests: r.SkippedTests,
			DurationMs:   r.DurationMs,
		}
	}
	return out, nil
}

// AggregateSpecsForSuiteInRange rolls up spec_runs for a single
// (project, suite) pair in the window. Joins spec_runs → suite_runs →
// test_runs to scope by project + suite name + start_time. Limits to
// 500 rows ordered by total duration so the treemap node budget
// (FR-16) is never blown by a single suite with thousands of specs.
func (r *GormTestRunRepository) AggregateSpecsForSuiteInRange(
	ctx context.Context, projectID, suiteName string, startDate, endDate time.Time,
) ([]*domain.SpecAggregate, error) {
	type row struct {
		SpecName    string
		TotalRuns   int
		PassedRuns  int
		FailedRuns  int
		SkippedRuns int
		IsFlaky     bool
		DurationMs  int64
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Table("spec_runs AS sp").
		Select(`
			sp.spec_name                                                       AS spec_name,
			COUNT(*)                                                           AS total_runs,
			COALESCE(SUM(CASE WHEN sp.status = 'passed'  THEN 1 ELSE 0 END), 0) AS passed_runs,
			COALESCE(SUM(CASE WHEN sp.status = 'failed'  THEN 1 ELSE 0 END), 0) AS failed_runs,
			COALESCE(SUM(CASE WHEN sp.status = 'skipped' THEN 1 ELSE 0 END), 0) AS skipped_runs,
			BOOL_OR(sp.is_flaky)                                                AS is_flaky,
			COALESCE(SUM(sp.duration_ms), 0)                                    AS duration_ms
		`).
		Joins("JOIN suite_runs sr ON sr.id = sp.suite_run_id").
		Joins("JOIN test_runs  tr ON tr.id = sr.test_run_id").
		Where("tr.project_id = ?", projectID).
		Where("sr.suite_name = ?", suiteName).
		Where("tr.start_time >= ? AND tr.start_time <= ?", startDate, endDate).
		Group("sp.spec_name").
		Order("duration_ms DESC").
		Limit(500).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*domain.SpecAggregate, len(rows))
	for i, r := range rows {
		out[i] = &domain.SpecAggregate{
			SpecName:    r.SpecName,
			TotalRuns:   r.TotalRuns,
			PassedRuns:  r.PassedRuns,
			FailedRuns:  r.FailedRuns,
			SkippedRuns: r.SkippedRuns,
			IsFlaky:     r.IsFlaky,
			DurationMs:  r.DurationMs,
		}
	}
	return out, nil
}

// AggregateDailyByProjects rolls up test_runs by (project_id, day) for
// the Test Summaries page. The page previously fanned out N parallel
// requests (one per project) to compute sparkline data; this returns
// the same data in a single SQL GROUP BY. Rows are sparse — days with
// no runs are absent and callers must zero-fill.
func (r *GormTestRunRepository) AggregateDailyByProjects(
	ctx context.Context, projectIDs []string, startDate, endDate time.Time,
) ([]*domain.DailyProjectAggregate, error) {
	if len(projectIDs) == 0 {
		return nil, nil
	}
	type row struct {
		ProjectID    string
		Day          time.Time
		TotalRuns    int
		TotalTests   int
		PassedTests  int
		FailedTests  int
		SkippedTests int
		DurationMs   int64
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Table("test_runs").
		Select(`
			project_id            AS project_id,
			date_trunc('day', start_time) AS day,
			COUNT(*)              AS total_runs,
			COALESCE(SUM(total_tests),   0) AS total_tests,
			COALESCE(SUM(passed_tests),  0) AS passed_tests,
			COALESCE(SUM(failed_tests),  0) AS failed_tests,
			COALESCE(SUM(skipped_tests), 0) AS skipped_tests,
			COALESCE(SUM(duration_ms),   0) AS duration_ms
		`).
		Where("project_id IN ?", projectIDs).
		Where("start_time >= ? AND start_time <= ?", startDate, endDate).
		Group("project_id, day").
		Order("project_id, day").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]*domain.DailyProjectAggregate, len(rows))
	for i, r := range rows {
		out[i] = &domain.DailyProjectAggregate{
			ProjectID:    r.ProjectID,
			Day:          r.Day.UTC(),
			TotalRuns:    r.TotalRuns,
			TotalTests:   r.TotalTests,
			PassedTests:  r.PassedTests,
			FailedTests:  r.FailedTests,
			SkippedTests: r.SkippedTests,
			DurationMs:   r.DurationMs,
		}
	}
	return out, nil
}
