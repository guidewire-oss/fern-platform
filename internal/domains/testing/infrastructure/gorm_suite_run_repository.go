package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/guidewire-oss/fern-platform/pkg/database"
	"gorm.io/gorm"
)

// GormSuiteRunRepository implements domain.SuiteRunRepository using GORM
type GormSuiteRunRepository struct {
	db        *gorm.DB
	converter *DatabaseConverter
}

// NewGormSuiteRunRepository creates a new GORM-based suite run repository
func NewGormSuiteRunRepository(db *gorm.DB) *GormSuiteRunRepository {
	return &GormSuiteRunRepository{db: db}
}

// Create creates a new suite run
// This is used when adding suite runs to an existing test run.
// For new test runs, the entire hierarchy is saved via GormTestRunRepository.Create()
func (r *GormSuiteRunRepository) Create(ctx context.Context, suiteRun *domain.SuiteRun) error {
	dbSuiteRun := &database.SuiteRun{
		TestRunID:    suiteRun.TestRunID,
		SuiteName:    suiteRun.Name,
		Status:       suiteRun.Status,
		StartTime:    suiteRun.StartTime,
		EndTime:      suiteRun.EndTime,
		TotalSpecs:   suiteRun.TotalTests,
		PassedSpecs:  suiteRun.PassedTests,
		FailedSpecs:  suiteRun.FailedTests,
		SkippedSpecs: suiteRun.SkippedTests,
		Duration:     int64(suiteRun.Duration / time.Millisecond),
		Tags:         r.converter.ConvertDomainTagsToDatabase(suiteRun.Tags),
	}

	// GORM will handle the association with existing tags by ID
	if err := r.db.WithContext(ctx).Create(dbSuiteRun).Error; err != nil {
		return fmt.Errorf("failed to create suite run: %w", err)
	}

	suiteRun.ID = dbSuiteRun.ID
	return nil
}

// CreateBatch creates multiple suite runs in a batch
func (r *GormSuiteRunRepository) CreateBatch(ctx context.Context, suiteRuns []*domain.SuiteRun) error {
	if len(suiteRuns) == 0 {
		return nil
	}

	dbSuiteRuns := make([]*database.SuiteRun, len(suiteRuns))
	for i, suiteRun := range suiteRuns {
		dbSuiteRuns[i] = &database.SuiteRun{
			TestRunID:    suiteRun.TestRunID,
			SuiteName:    suiteRun.Name,
			Status:       suiteRun.Status,
			StartTime:    suiteRun.StartTime,
			EndTime:      suiteRun.EndTime,
			TotalSpecs:   suiteRun.TotalTests,
			PassedSpecs:  suiteRun.PassedTests,
			FailedSpecs:  suiteRun.FailedTests,
			SkippedSpecs: suiteRun.SkippedTests,
			Duration:     int64(suiteRun.Duration / time.Millisecond),
			Tags:         r.converter.ConvertDomainTagsToDatabase(suiteRun.Tags),
		}
	}

	if err := r.db.WithContext(ctx).CreateInBatches(dbSuiteRuns, 100).Error; err != nil {
		return fmt.Errorf("failed to create suite runs in batch: %w", err)
	}

	// Update the domain objects with generated IDs
	for i, dbSuiteRun := range dbSuiteRuns {
		suiteRuns[i].ID = dbSuiteRun.ID
	}

	return nil
}

// FindByTestRunID finds all suite runs for a test run
func (r *GormSuiteRunRepository) FindByTestRunID(ctx context.Context, testRunID uint) ([]*domain.SuiteRun, error) {
	var dbSuiteRuns []database.SuiteRun
	if err := r.db.WithContext(ctx).Where("test_run_id = ?", testRunID).Find(&dbSuiteRuns).Error; err != nil {
		return nil, fmt.Errorf("failed to find suite runs: %w", err)
	}

	suiteRuns := make([]*domain.SuiteRun, len(dbSuiteRuns))
	for i, dbSuiteRun := range dbSuiteRuns {
		suiteRuns[i] = r.toDomainSuiteRun(&dbSuiteRun)
	}

	return suiteRuns, nil
}

// GetByID retrieves a suite run by ID
func (r *GormSuiteRunRepository) GetByID(ctx context.Context, id uint) (*domain.SuiteRun, error) {
	var dbSuiteRun database.SuiteRun
	if err := r.db.WithContext(ctx).First(&dbSuiteRun, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("suite run not found")
		}
		return nil, fmt.Errorf("failed to get suite run: %w", err)
	}

	return r.toDomainSuiteRun(&dbSuiteRun), nil
}

// Update updates a suite run
func (r *GormSuiteRunRepository) Update(ctx context.Context, suiteRun *domain.SuiteRun) error {
	updates := map[string]interface{}{
		"status":        suiteRun.Status,
		"end_time":      suiteRun.EndTime,
		"duration":      int64(suiteRun.Duration / time.Millisecond),
		"total_specs":   suiteRun.TotalTests,
		"passed_specs":  suiteRun.PassedTests,
		"failed_specs":  suiteRun.FailedTests,
		"skipped_specs": suiteRun.SkippedTests,
		"updated_at":    time.Now(),
	}

	result := r.db.WithContext(ctx).Model(&database.SuiteRun{}).Where("id = ?", suiteRun.ID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update suite run: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("suite run not found")
	}

	return nil
}

// GetWithSpecRuns retrieves a suite run with all its spec runs
func (r *GormSuiteRunRepository) GetWithSpecRuns(ctx context.Context, id uint) (*domain.SuiteRun, error) {
	var dbSuiteRun database.SuiteRun
	if err := r.db.WithContext(ctx).Preload("SpecRuns").First(&dbSuiteRun, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("suite run not found")
		}
		return nil, fmt.Errorf("failed to get suite run with spec runs: %w", err)
	}

	suiteRun := r.toDomainSuiteRun(&dbSuiteRun)

	// Convert spec runs if preloaded
	if len(dbSuiteRun.SpecRuns) > 0 {
		suiteRun.SpecRuns = make([]*domain.SpecRun, len(dbSuiteRun.SpecRuns))
		for i, dbSpecRun := range dbSuiteRun.SpecRuns {
			suiteRun.SpecRuns[i] = &domain.SpecRun{
				ID:             dbSpecRun.ID,
				SuiteRunID:     dbSpecRun.SuiteRunID,
				Name:           dbSpecRun.SpecName,
				ClassName:      "", // Not stored in database
				Status:         dbSpecRun.Status,
				StartTime:      dbSpecRun.StartTime,
				EndTime:        dbSpecRun.EndTime,
				Duration:       time.Duration(dbSpecRun.Duration) * time.Millisecond,
				ErrorMessage:   dbSpecRun.ErrorMessage,
				FailureMessage: dbSpecRun.ErrorMessage, // Use error message
				StackTrace:     dbSpecRun.StackTrace,
				RetryCount:     dbSpecRun.RetryCount,
				IsFlaky:        dbSpecRun.IsFlaky,
			}
		}
	}

	return suiteRun, nil
}

// Helper method to convert database model to domain model
func (r *GormSuiteRunRepository) toDomainSuiteRun(dbSuiteRun *database.SuiteRun) *domain.SuiteRun {
	return &domain.SuiteRun{
		ID:           dbSuiteRun.ID,
		TestRunID:    dbSuiteRun.TestRunID,
		Name:         dbSuiteRun.SuiteName,
		PackageName:  "", // Not stored in database
		ClassName:    "", // Not stored in database
		Status:       dbSuiteRun.Status,
		StartTime:    dbSuiteRun.StartTime,
		EndTime:      dbSuiteRun.EndTime,
		TotalTests:   dbSuiteRun.TotalSpecs,
		PassedTests:  dbSuiteRun.PassedSpecs,
		FailedTests:  dbSuiteRun.FailedSpecs,
		SkippedTests: dbSuiteRun.SkippedSpecs,
		Duration:     time.Duration(dbSuiteRun.Duration) * time.Millisecond,
	}
}
