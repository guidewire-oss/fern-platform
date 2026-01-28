package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/guidewire-oss/fern-platform/pkg/database"
	"gorm.io/gorm"
)

// GormSpecRunRepository implements domain.SpecRunRepository using GORM
type GormSpecRunRepository struct {
	db        *gorm.DB
	converter *DatabaseConverter
}

// NewGormSpecRunRepository creates a new GORM-based spec run repository
func NewGormSpecRunRepository(db *gorm.DB) *GormSpecRunRepository {
	return &GormSpecRunRepository{db: db}
}

// Create creates a new spec run
// This is used when adding spec runs to an existing suite run.
// For new test runs, the entire hierarchy is saved via GormTestRunRepository.Create()
func (r *GormSpecRunRepository) Create(ctx context.Context, specRun *domain.SpecRun) error {
	dbSpecRun := &database.SpecRun{
		SuiteRunID:   specRun.SuiteRunID,
		SpecName:     specRun.Name,
		Status:       specRun.Status,
		StartTime:    specRun.StartTime,
		EndTime:      specRun.EndTime,
		Duration:     int64(specRun.Duration / time.Millisecond),
		ErrorMessage: specRun.ErrorMessage,
		StackTrace:   specRun.StackTrace,
		RetryCount:   specRun.RetryCount,
		IsFlaky:      specRun.IsFlaky,
		Tags:         r.converter.ConvertDomainTagsToDatabase(specRun.Tags),
	}

	if err := r.db.WithContext(ctx).Create(dbSpecRun).Error; err != nil {
		return fmt.Errorf("failed to create spec run: %w", err)
	}

	specRun.ID = dbSpecRun.ID
	return nil
}

// CreateBatch creates multiple spec runs in a batch
func (r *GormSpecRunRepository) CreateBatch(ctx context.Context, specRuns []*domain.SpecRun) error {
	if len(specRuns) == 0 {
		return nil
	}

	dbSpecRuns := make([]*database.SpecRun, len(specRuns))
	for i, specRun := range specRuns {
		dbSpecRuns[i] = &database.SpecRun{
			SuiteRunID:   specRun.SuiteRunID,
			SpecName:     specRun.Name,
			Status:       specRun.Status,
			StartTime:    specRun.StartTime,
			EndTime:      specRun.EndTime,
			Duration:     int64(specRun.Duration / time.Millisecond),
			ErrorMessage: specRun.ErrorMessage,
			StackTrace:   specRun.StackTrace,
			RetryCount:   specRun.RetryCount,
			IsFlaky:      specRun.IsFlaky,
			Tags:         r.converter.ConvertDomainTagsToDatabase(specRun.Tags),
		}
	}

	if err := r.db.WithContext(ctx).CreateInBatches(dbSpecRuns, 100).Error; err != nil {
		return fmt.Errorf("failed to create spec runs in batch: %w", err)
	}

	// Update the domain objects with generated IDs
	for i, dbSpecRun := range dbSpecRuns {
		specRuns[i].ID = dbSpecRun.ID
	}

	return nil
}

// FindBySuiteRunID finds all spec runs for a suite run
func (r *GormSpecRunRepository) FindBySuiteRunID(ctx context.Context, suiteRunID uint) ([]*domain.SpecRun, error) {
	var dbSpecRuns []database.SpecRun
	if err := r.db.WithContext(ctx).Where("suite_run_id = ?", suiteRunID).Find(&dbSpecRuns).Error; err != nil {
		return nil, fmt.Errorf("failed to find spec runs: %w", err)
	}

	specRuns := make([]*domain.SpecRun, len(dbSpecRuns))
	for i, dbSpecRun := range dbSpecRuns {
		specRuns[i] = r.toDomainSpecRun(&dbSpecRun)
	}

	return specRuns, nil
}

// Update updates an existing spec run
func (r *GormSpecRunRepository) Update(ctx context.Context, specRun *domain.SpecRun) error {
	// Not implemented - not used in the application
	return fmt.Errorf("Update not implemented")
}

// GetByID retrieves a spec run by ID
func (r *GormSpecRunRepository) GetByID(ctx context.Context, id uint) (*domain.SpecRun, error) {
	var dbSpecRun database.SpecRun
	if err := r.db.WithContext(ctx).First(&dbSpecRun, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("spec run not found")
		}
		return nil, fmt.Errorf("failed to get spec run: %w", err)
	}

	return r.toDomainSpecRun(&dbSpecRun), nil
}

// CountByStatus counts spec runs by status for a suite run
func (r *GormSpecRunRepository) CountByStatus(ctx context.Context, suiteRunID uint) (map[string]int, error) {
	var results []struct {
		Status string
		Count  int
	}

	err := r.db.WithContext(ctx).Model(&database.SpecRun{}).
		Select("status, COUNT(*) as count").
		Where("suite_run_id = ?", suiteRunID).
		Group("status").
		Scan(&results).Error

	if err != nil {
		return nil, fmt.Errorf("failed to count spec runs by status: %w", err)
	}

	counts := make(map[string]int)
	for _, result := range results {
		counts[result.Status] = result.Count
	}

	return counts, nil
}

// Helper method to convert database model to domain model
func (r *GormSpecRunRepository) toDomainSpecRun(dbSpecRun *database.SpecRun) *domain.SpecRun {
	return &domain.SpecRun{
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
