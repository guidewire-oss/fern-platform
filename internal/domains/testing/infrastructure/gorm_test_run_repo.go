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

	if err := r.db.WithContext(ctx).Create(dbTestRun).Error; err != nil {
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
		"metadata":      database.JSONMap(testRun.Metadata),
		"updated_at":    time.Now(),
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
	if err := r.db.WithContext(ctx).First(&dbTestRun, id).Error; err != nil {
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
	if err := r.db.WithContext(ctx).Where("run_id = ?", runID).First(&dbTestRun).Error; err != nil {
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
	query := r.db.WithContext(ctx).Where("project_id = ?", projectID).Preload("SuiteRuns.SpecRuns").Order("created_at DESC")

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
	if err := r.db.WithContext(ctx).Preload("SuiteRuns.SpecRuns").First(&dbTestRun, id).Error; err != nil {
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

// GetRecent retrieves recent test runs across all projects
func (r *GormTestRunRepository) GetRecent(ctx context.Context, limit int) ([]*domain.TestRun, error) {
	var dbTestRuns []database.TestRun
	query := r.db.WithContext(ctx).Model(&database.TestRun{}).Preload("SuiteRuns.SpecRuns").Order("created_at DESC")

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
