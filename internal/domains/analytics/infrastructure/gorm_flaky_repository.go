package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/analytics/domain"
	"github.com/guidewire-oss/fern-platform/pkg/database"
	"gorm.io/gorm"
)

// maxFlakyTestsPerProject caps FindFlakyTestsByProject
const maxFlakyTestsPerProject = 500
// maxHistoryRowsPerTest caps GetTestRunHistory. Separate from
// maxFlakyTestsPerProject since the two bound different things and
// share a value today.
const maxHistoryRowsPerTest = 500

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
		ProjectID:        flaky.ProjectID,
		TestName:         flaky.TestName,
		SuiteName:        flaky.SuiteName,
		FlakeRate:        flaky.FlakeScore * 100, // Convert to percentage
		TotalExecutions:  flaky.TotalRuns,
		FlakyExecutions:  flaky.FailureCount,
		FirstSeenAt:      flaky.FirstSeen,
		LastSeenAt:       flaky.LastSeen,
		Status:           string(flaky.Status),
		Severity:         calculateSeverity(flaky.FlakeScore),
		LastErrorMessage: getLastErrorMessage(flaky.Metadata),
	}

	// Use upsert pattern
	result := r.db.WithContext(ctx).Save(dbFlaky)
	if result.Error != nil {
		return fmt.Errorf("failed to save flaky test: %w", result.Error)
	}

	return nil
}

// GetFlakyTest retrieves a flaky test by ID
func (r *GormFlakyDetectionRepository) GetFlakyTest(ctx context.Context, testID string) (*domain.FlakyTest, error) {
	var dbFlaky database.FlakyTest
	if err := r.db.WithContext(ctx).Where("test_id = ?", testID).First(&dbFlaky).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("flaky test not found")
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

	// capped so an active project's flake list can't grow
	// unbounded; ordering by flake_score DESC keeps the worst offenders.
	if err := query.Order("flake_score DESC").Limit(maxFlakyTestsPerProject).Find(&dbFlakyTests).Error; err != nil {
		return nil, fmt.Errorf("failed to find flaky tests: %w", err)
	}

	flakyTests := make([]*domain.FlakyTest, len(dbFlakyTests))
	for i, dbFlaky := range dbFlakyTests {
		flaky, err := r.toDomainFlakyTest(&dbFlaky)
		if err != nil {
			// Log error but continue
			continue
		}
		flakyTests[i] = flaky
	}

	return flakyTests, nil
}

// UpdateFlakyTestStatus updates the status of a flaky test
func (r *GormFlakyDetectionRepository) UpdateFlakyTestStatus(ctx context.Context, testID string, status domain.FlakyTestStatus) error {
	result := r.db.WithContext(ctx).Model(&database.FlakyTest{}).
		Where("test_id = ?", testID).
		Update("status", string(status))

	if result.Error != nil {
		return fmt.Errorf("failed to update flaky test status: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("flaky test not found")
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
func (r *GormFlakyDetectionRepository) GetTestRunHistory(ctx context.Context, projectID string, testName string, since time.Time) ([]domain.TestExecutionResult, error) {
	// Use a raw query to get all the needed data in one query
	query := fmt.Sprintf(`
		SELECT
			sr.id as spec_run_id,
			sr.name as test_name,
			sr.status,
			sr.duration,
			sr.failure_message,
			sr.created_at,
			sur.name as suite_name,
			tr.id as test_run_id,
			tr.git_branch,
			tr.git_commit
		FROM spec_runs sr
		JOIN suite_runs sur ON sur.id = sr.suite_run_id
		JOIN test_runs tr ON tr.id = sur.test_run_id
		WHERE tr.project_id = ? AND sr.name = ? AND tr.created_at >= ?
		ORDER BY tr.created_at DESC
		LIMIT %d
	`, maxHistoryRowsPerTest)

	rows, err := r.db.WithContext(ctx).Raw(query, projectID, testName, since).Rows()
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

// GetUniqueTestNames returns all unique test names for a project since a given time
func (r *GormFlakyDetectionRepository) GetUniqueTestNames(ctx context.Context, projectID string, since time.Time) ([]string, error) {
	var testNames []string

	query := `
		SELECT DISTINCT sr.name
		FROM spec_runs sr
		JOIN suite_runs sur ON sur.id = sr.suite_run_id
		JOIN test_runs tr ON tr.id = sur.test_run_id
		WHERE tr.project_id = ? AND tr.created_at >= ?
		ORDER BY sr.name
	`

	err := r.db.WithContext(ctx).Raw(query, projectID, since).Pluck("name", &testNames).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get unique test names: %w", err)
	}

	return testNames, nil
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

	// Generate TestID from project and test name
	testID := fmt.Sprintf("%s:%s", dbFlaky.ProjectID, dbFlaky.TestName)

	return &domain.FlakyTest{
		TestID:       testID,
		ProjectID:    dbFlaky.ProjectID,
		TestName:     dbFlaky.TestName,
		SuiteName:    dbFlaky.SuiteName,
		PackageName:  "", // Not stored in database model
		FirstSeen:    dbFlaky.FirstSeenAt,
		LastSeen:     dbFlaky.LastSeenAt,
		TotalRuns:    dbFlaky.TotalExecutions,
		FailureCount: dbFlaky.FlakyExecutions,
		FlakeScore:   dbFlaky.FlakeRate / 100, // Convert from percentage
		Status:       domain.FlakyTestStatus(dbFlaky.Status),
		Metadata:     metadata,
	}, nil
}
