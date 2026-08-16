package infrastructure_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/guidewire-oss/fern-platform/internal/domains/analytics/domain"
	"github.com/guidewire-oss/fern-platform/internal/domains/analytics/infrastructure"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupFlakyMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *gorm.DB) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{})
	require.NoError(t, err)

	return db, mock, gormDB
}

// GetTestRunHistory's raw query must not load a test's entire execution
// history unbounded.
func TestGormFlakyDetectionRepository_GetTestRunHistory_HasLimit(t *testing.T) {
	db, mock, gormDB := setupFlakyMockDB(t)
	defer db.Close()

	repo := infrastructure.NewGormFlakyDetectionRepository(gormDB)

	rows := sqlmock.NewRows([]string{
		"spec_run_id", "test_name", "status", "duration", "failure_message",
		"created_at", "suite_name", "test_run_id", "git_branch", "git_commit",
	})
	mock.ExpectQuery(`LIMIT 500`).
		WithArgs("project-123", "TestFoo", sqlmock.AnyArg()).
		WillReturnRows(rows)

	_, err := repo.GetTestRunHistory(context.Background(), "project-123", "TestFoo", time.Now())
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// FindFlakyTestsByProject must not load every flaky test for a project
// unbounded.
func TestGormFlakyDetectionRepository_FindFlakyTestsByProject_HasLimit(t *testing.T) {
	db, mock, gormDB := setupFlakyMockDB(t)
	defer db.Close()

	repo := infrastructure.NewGormFlakyDetectionRepository(gormDB)

	rows := sqlmock.NewRows([]string{"id", "project_id", "status", "flake_rate"})
	mock.ExpectQuery(`SELECT \* FROM "flaky_tests" WHERE project_id = \$1 AND status = \$2 AND "flaky_tests"\."deleted_at" IS NULL ORDER BY flake_score DESC LIMIT \$3`).
		WillReturnRows(rows)

	_, err := repo.FindFlakyTestsByProject(context.Background(), "project-123", domain.StatusActive)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
