package infrastructure

import (
	"fmt"

	"github.com/guidewire-oss/fern-platform/internal/domains/summary/domain"
	"github.com/guidewire-oss/fern-platform/pkg/database"
	"gorm.io/gorm"
)

// GormSummaryRepository implements domain.Repository using GORM
type GormSummaryRepository struct {
	db *gorm.DB
}

// NewGormSummaryRepository creates a new GORM-based summary repository
func NewGormSummaryRepository(db *gorm.DB) *GormSummaryRepository {
	return &GormSummaryRepository{db: db}
}

// GetTestRunsBySeed retrieves all test runs for a project and seed (run_id)
func (r *GormSummaryRepository) GetTestRunsBySeed(projectUUID string, seed string) ([]domain.TestRunData, error) {
	var dbTestRuns []database.TestRun

	// Query test runs by project_id and run_id (seed)
	// The run_id field stores the seed value as a string
	query := r.db.
		Model(&database.TestRun{}).
		Where("project_id = ? AND run_id = ?", projectUUID, seed).
		Preload("SuiteRuns.SpecRuns.Tags").
		Preload("SuiteRuns.Tags").
		Order("start_time")

	if err := query.Find(&dbTestRuns).Error; err != nil {
		return nil, fmt.Errorf("failed to get test runs: %w", err)
	}

	// Convert database models to domain models
	result := make([]domain.TestRunData, len(dbTestRuns))
	for i, dbTestRun := range dbTestRuns {
		result[i] = r.convertTestRunToDomain(&dbTestRun)
	}

	return result, nil
}

// convertTestRunToDomain converts database TestRun to domain TestRunData
func (r *GormSummaryRepository) convertTestRunToDomain(dbTestRun *database.TestRun) domain.TestRunData {
	testRunData := domain.TestRunData{
		GitBranch: dbTestRun.Branch,
		GitSHA:    dbTestRun.CommitSHA,
		StartTime: dbTestRun.StartTime,
		SuiteRuns: make([]domain.SuiteRunData, len(dbTestRun.SuiteRuns)),
	}

	if dbTestRun.EndTime != nil {
		testRunData.EndTime = *dbTestRun.EndTime
	}

	// Convert suite runs
	for i, dbSuite := range dbTestRun.SuiteRuns {
		testRunData.SuiteRuns[i] = domain.SuiteRunData{
			SpecRuns: make([]domain.SpecRunData, len(dbSuite.SpecRuns)),
			Tags:     r.convertTags(dbSuite.Tags),
		}

		// Convert spec runs
		for j, dbSpec := range dbSuite.SpecRuns {
			testRunData.SuiteRuns[i].SpecRuns[j] = domain.SpecRunData{
				Status: dbSpec.Status,
				Tags:   r.convertTags(dbSpec.Tags),
			}
		}
	}

	return testRunData
}

// convertTags converts database tags to domain tags
func (r *GormSummaryRepository) convertTags(dbTags []database.Tag) []domain.TagData {
	tags := make([]domain.TagData, len(dbTags))
	for i, dbTag := range dbTags {
		tags[i] = domain.TagData{
			Category: dbTag.Category,
			Value:    dbTag.Value,
		}
	}
	return tags
}
