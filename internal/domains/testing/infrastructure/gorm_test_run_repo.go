package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
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
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("created_at DESC").Find(&dbTestRuns).Error; err != nil {
		return nil, fmt.Errorf("failed to get test runs: %w", err)
	}

	testRuns := make([]*domain.TestRun, len(dbTestRuns))
	for i, dbTestRun := range dbTestRuns {
		testRuns[i] = r.converter.ConvertTestRunToDomain(&dbTestRun)
	}

	return testRuns, nil
}

// GetLatestByProjectID retrieves the latest test runs for a project
func (r *GormTestRunRepository) GetLatestByProjectID(ctx context.Context, projectID string, limit int) ([]*domain.TestRun, error) {
	var dbTestRuns []database.TestRun
	query := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Preload("Tags").
		Preload("SuiteRuns").
		Preload("SuiteRuns.Tags").
		Preload("SuiteRuns.SpecRuns").
		Preload("SuiteRuns.SpecRuns.Tags").
		Order("created_at DESC")

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

// FindByDateRange finds test runs within a date range
func (r *GormTestRunRepository) FindByDateRange(ctx context.Context, projectID string, startDate, endDate time.Time) ([]*domain.TestRun, error) {
	var dbTestRuns []database.TestRun
	query := r.db.WithContext(ctx).Where("project_id = ? AND created_at >= ? AND created_at <= ?", projectID, startDate, endDate).Order("created_at DESC")

	if err := query.Find(&dbTestRuns).Error; err != nil {
		return nil, fmt.Errorf("failed to find test runs by date range: %w", err)
	}

	testRuns := make([]*domain.TestRun, len(dbTestRuns))
	for i, dbTestRun := range dbTestRuns {
		testRuns[i] = r.converter.ConvertTestRunToDomain(&dbTestRun)
	}

	return testRuns, nil
}

// GetTestRunSummary retrieves summary statistics for a project
func (r *GormTestRunRepository) GetTestRunSummary(ctx context.Context, projectID string) (*domain.TestRunSummary, error) {
	var row struct {
		Total       int64
		Passed      int64
		Failed      int64
		AvgDuration float64
	}
	if err := r.db.WithContext(ctx).
		Model(&database.TestRun{}).
		Where("project_id = ?", projectID).
		Select("COUNT(*) as total, " +
			"COALESCE(SUM(CASE WHEN status='passed' THEN 1 ELSE 0 END), 0) as passed, " +
			"COALESCE(SUM(CASE WHEN status='failed' THEN 1 ELSE 0 END), 0) as failed, " +
			"COALESCE(AVG(duration_ms), 0) as avg_duration").
		Scan(&row).Error; err != nil {
		return nil, fmt.Errorf("failed to get test run summary: %w", err)
	}

	summary := &domain.TestRunSummary{
		TotalRuns:      int(row.Total),
		PassedRuns:     int(row.Passed),
		FailedRuns:     int(row.Failed),
		AverageRunTime: time.Duration(row.AvgDuration) * time.Millisecond,
	}
	if summary.TotalRuns > 0 {
		summary.SuccessRate = float64(summary.PassedRuns) / float64(summary.TotalRuns)
	}
	return summary, nil
}

// Delete removes a test run
func (r *GormTestRunRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&database.TestRun{}, id).Error
}

// CountByProjectID counts test runs for a project
func (r *GormTestRunRepository) CountByProjectID(ctx context.Context, projectID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&database.TestRun{}).
		Where("project_id = ?", projectID).
		Count(&count).Error
	return count, err
}

// Count counts all test runs
func (r *GormTestRunRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&database.TestRun{}).Count(&count).Error
	return count, err
}

// GetRecent retrieves recent test runs across all projects
func (r *GormTestRunRepository) GetRecent(ctx context.Context, limit int) ([]*domain.TestRun, error) {
	var dbTestRuns []database.TestRun
	query := r.db.WithContext(ctx).
		Model(&database.TestRun{}).
		Preload("Tags").
		Preload("SuiteRuns").
		Preload("SuiteRuns.Tags").
		Preload("SuiteRuns.SpecRuns").
		Preload("SuiteRuns.SpecRuns.Tags").
		Order("created_at DESC")

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

// List retrieves test runs with pagination across all projects
func (r *GormTestRunRepository) List(ctx context.Context, limit, offset int) ([]*domain.TestRun, int64, error) {
	var dbTestRuns []database.TestRun
	var total int64

	if err := r.db.WithContext(ctx).Model(&database.TestRun{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count test runs: %w", err)
	}

	query := r.db.WithContext(ctx).
		Model(&database.TestRun{}).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset)

	if err := query.Find(&dbTestRuns).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list test runs: %w", err)
	}

	testRuns := make([]*domain.TestRun, len(dbTestRuns))
	for i, dbTestRun := range dbTestRuns {
		testRuns[i] = r.converter.ConvertTestRunToDomain(&dbTestRun)
	}

	return testRuns, total, nil
}

// GetDB returns the underlying GORM DB instance.
// This allows higher-level services to perform association updates (e.g., tags).
func (r *GormTestRunRepository) GetDB() *gorm.DB {
	return r.db
}
