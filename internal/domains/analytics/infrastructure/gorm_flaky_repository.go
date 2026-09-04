package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/analytics/domain"
	"github.com/guidewire-oss/fern-platform/pkg/database"
	"gorm.io/gorm"
)

// GormFlakyDetectionRepository implements FlakyDetectionRepository using GORM
type GormFlakyDetectionRepository struct {
	db *gorm.DB
}

// NewGormFlakyDetectionRepository creates a new GORM-based flaky detection repository
func NewGormFlakyDetectionRepository(db *gorm.DB) *GormFlakyDetectionRepository {
	return &GormFlakyDetectionRepository{db: db}
}

// SaveFlakyTest saves or updates a flaky test record
func (r *GormFlakyDetectionRepository) SaveFlakyTest(ctx context.Context, flaky *domain.FlakyTest) error {
	// Convert domain model to database model
	dbFlaky := &database.FlakyTest{
		ProjectID: flaky.ProjectID,
		TestName:  flaky.TestName,
		SuiteName: flaky.SuiteName,
		// flake_rate is DECIMAL(5,4): a 0–1 fraction, as cmd/seed writes.
		// Scaling to 0–100 overflowed it for any score >= 0.1.
		FlakeRate:        flaky.FlakeScore,
		TotalExecutions:  flaky.TotalRuns,
		FlakyExecutions:  flaky.FailureCount,
		FirstSeenAt:      flaky.FirstSeen,
		LastSeenAt:       flaky.LastSeen,
		Status:           string(flaky.Status),
		Severity:         calculateSeverity(flaky.FlakeScore),
		LastErrorMessage: getLastErrorMessage(flaky.Metadata),
	}

	// Without the row ID, Save inserts and re-analysis trips
	// UNIQUE(project_id, test_name, suite_name).
	dbFlaky.ID = flaky.ID

	tx := r.db.WithContext(ctx)
	if dbFlaky.ID != 0 {
		// Save writes every column and we have no created_at to write back.
		tx = tx.Omit("created_at")
	}

	result := tx.Save(dbFlaky)
	if result.Error != nil {
		return fmt.Errorf("failed to save flaky test: %w", result.Error)
	}

	flaky.ID = dbFlaky.ID

	return nil
}

// GetFlakyTestByName retrieves a flaky test by its natural key.
func (r *GormFlakyDetectionRepository) GetFlakyTestByName(ctx context.Context, projectID string, testName string, suiteName string) (*domain.FlakyTest, error) {
	var dbFlaky database.FlakyTest
	if err := r.db.WithContext(ctx).
		// COALESCE because suite_name is nullable: an older row stored as NULL
		// would miss a plain equality against the empty string analysis
		// supplies, and the insert that followed would duplicate it.
		Where("project_id = ? AND test_name = ? AND COALESCE(suite_name, '') = ?", projectID, testName, suiteName).
		First(&dbFlaky).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrFlakyTestNotFound
		}
		return nil, fmt.Errorf("failed to get flaky test: %w", err)
	}

	return r.toDomainFlakyTest(&dbFlaky)
}

// FindFlakyTestsByProject finds all flaky tests for a project with a specific status
func (r *GormFlakyDetectionRepository) FindFlakyTestsByProject(ctx context.Context, projectID string, status domain.FlakyTestStatus) ([]*domain.FlakyTest, error) {
	var dbFlakyTests []database.FlakyTest

	query := r.db.WithContext(ctx).Where("project_id = ?", projectID)
	if status != "" {
		query = query.Where("status = ?", string(status))
	}

	// There is no flake_score column; ordering by it failed the query.
	if err := query.Order("flake_rate DESC").Find(&dbFlakyTests).Error; err != nil {
		return nil, fmt.Errorf("failed to find flaky tests: %w", err)
	}

	// Appended, not indexed: a skipped row would leave a nil hole.
	flakyTests := make([]*domain.FlakyTest, 0, len(dbFlakyTests))
	for i := range dbFlakyTests {
		flaky, err := r.toDomainFlakyTest(&dbFlakyTests[i])
		if err != nil {
			continue
		}
		flakyTests = append(flakyTests, flaky)
	}

	return flakyTests, nil
}

// UpdateFlakyTestStatus updates the status of a flaky test by row ID.
func (r *GormFlakyDetectionRepository) UpdateFlakyTestStatus(ctx context.Context, id uint, status domain.FlakyTestStatus) error {
	result := r.db.WithContext(ctx).Model(&database.FlakyTest{}).
		Where("id = ?", id).
		Update("status", string(status))

	if result.Error != nil {
		return fmt.Errorf("failed to update flaky test status: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return domain.ErrFlakyTestNotFound
	}

	return nil
}

// SaveTestRunAnalysis saves a test run analysis
func (r *GormFlakyDetectionRepository) SaveTestRunAnalysis(ctx context.Context, analysis *domain.TestRunAnalysis) error {
	// For now, we'll just log this. In a real implementation, we'd have a dedicated table
	// This could be used for tracking analysis history and generating reports
	return nil
}

// GetTestRunHistory retrieves test execution history for a specific test
func (r *GormFlakyDetectionRepository) GetTestRunHistory(ctx context.Context, projectID string, testName string, suiteName string, since time.Time) ([]domain.TestExecutionResult, error) {
	// Use a raw query to get all the needed data in one query
	query := `
		SELECT
			sr.id as spec_run_id,
			sr.spec_name as test_name,
			sr.status,
			sr.duration_ms,
			sr.error_message,
			sr.created_at,
			sur.suite_name as suite_name,
			tr.id as test_run_id,
			COALESCE(tr.branch, '') as branch,
			COALESCE(tr.commit_sha, '') as commit_sha
		FROM spec_runs sr
		JOIN suite_runs sur ON sur.id = sr.suite_run_id
		JOIN test_runs tr ON tr.id = sur.test_run_id
		WHERE tr.project_id = ? AND sr.spec_name = ? AND sur.suite_name = ?
		  AND tr.created_at >= ?
		  AND sr.deleted_at IS NULL AND sur.deleted_at IS NULL AND tr.deleted_at IS NULL
		ORDER BY tr.created_at DESC
	`

	rows, err := r.db.WithContext(ctx).Raw(query, projectID, testName, suiteName, since).Rows()
	if err != nil {
		return nil, fmt.Errorf("failed to get test run history: %w", err)
	}
	defer rows.Close()

	var results []domain.TestExecutionResult
	for rows.Next() {
		var (
			specRunID      uint
			testName       string
			status         string
			duration       int64
			failureMessage *string
			createdAt      time.Time
			suiteName      string
			testRunID      uint
			gitBranch      string
			gitCommit      string
		)

		err := rows.Scan(
			&specRunID,
			&testName,
			&status,
			&duration,
			&failureMessage,
			&createdAt,
			&suiteName,
			&testRunID,
			&gitBranch,
			&gitCommit,
		)
		if err != nil {
			continue
		}

		errorMsg := ""
		if failureMessage != nil {
			errorMsg = *failureMessage
		}

		result := domain.TestExecutionResult{
			TestRunID:  fmt.Sprintf("%d", testRunID),
			TestName:   testName,
			SuiteName:  suiteName,
			Status:     status,
			Duration:   time.Duration(duration) * time.Millisecond,
			ExecutedAt: createdAt,
			Error:      errorMsg,
			Environment: map[string]string{
				"branch": gitBranch,
				"commit": gitCommit,
			},
		}
		results = append(results, result)
	}

	return results, nil
}

// GetUniqueTests returns each (spec, suite) pair a project ran since a given time
func (r *GormFlakyDetectionRepository) GetUniqueTests(ctx context.Context, projectID string, since time.Time) ([]domain.TestIdentity, error) {
	query := `
		SELECT DISTINCT sr.spec_name as test_name, sur.suite_name as suite_name
		FROM spec_runs sr
		JOIN suite_runs sur ON sur.id = sr.suite_run_id
		JOIN test_runs tr ON tr.id = sur.test_run_id
		WHERE tr.project_id = ? AND tr.created_at >= ?
		  AND sr.deleted_at IS NULL AND sur.deleted_at IS NULL AND tr.deleted_at IS NULL
		ORDER BY sur.suite_name, sr.spec_name
	`

	var tests []domain.TestIdentity
	if err := r.db.WithContext(ctx).Raw(query, projectID, since).Scan(&tests).Error; err != nil {
		return nil, fmt.Errorf("failed to get unique tests: %w", err)
	}

	return tests, nil
}

// Helper function to calculate severity based on flake score
func calculateSeverity(flakeScore float64) string {
	if flakeScore < 0.1 {
		return "low"
	} else if flakeScore < 0.3 {
		return "medium"
	} else if flakeScore < 0.6 {
		return "high"
	}
	return "critical"
}

// Helper function to get last error message from metadata
func getLastErrorMessage(metadata domain.FlakyTestMetadata) string {
	if len(metadata.RecentFailures) > 0 {
		return metadata.RecentFailures[len(metadata.RecentFailures)-1].ErrorMessage
	}
	if len(metadata.FailurePatterns) > 0 {
		return metadata.FailurePatterns[0]
	}
	return ""
}

// Helper method to convert database model to domain model
func (r *GormFlakyDetectionRepository) toDomainFlakyTest(dbFlaky *database.FlakyTest) (*domain.FlakyTest, error) {
	// Reconstruct metadata from available fields
	metadata := domain.FlakyTestMetadata{}
	if dbFlaky.LastErrorMessage != "" {
		metadata.FailurePatterns = []string{dbFlaky.LastErrorMessage}
	}

	return &domain.FlakyTest{
		ID:           dbFlaky.ID,
		TestID:       domain.BuildTestID(dbFlaky.ProjectID, dbFlaky.SuiteName, dbFlaky.TestName),
		ProjectID:    dbFlaky.ProjectID,
		TestName:     dbFlaky.TestName,
		SuiteName:    dbFlaky.SuiteName,
		PackageName:  "", // Not stored in database model
		FirstSeen:    dbFlaky.FirstSeenAt,
		LastSeen:     dbFlaky.LastSeenAt,
		TotalRuns:    dbFlaky.TotalExecutions,
		FailureCount: dbFlaky.FlakyExecutions,
		FlakeScore:   dbFlaky.FlakeRate,
		Status:       domain.FlakyTestStatus(dbFlaky.Status),
		Metadata:     metadata,
	}, nil
}
