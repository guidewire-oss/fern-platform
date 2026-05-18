package repositories_test

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
	"github.com/guidewire-oss/fern-platform/internal/infrastructure/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupFieldMappingMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *gorm.DB) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	return db, mock, gormDB
}

func makeTestMapping(t *testing.T) *integrations.JiraFieldMapping {
	t.Helper()
	entries := []integrations.FieldMappingEntry{
		{
			FernField:             integrations.FernFieldRequirementID,
			JiraFieldID:           "customfield_10001",
			JiraFieldIsMultiValue: false,
			ReductionStrategy:     "",
		},
		{
			FernField:             integrations.FernFieldRequirementTitle,
			JiraFieldID:           "summary",
			JiraFieldIsMultiValue: false,
			ReductionStrategy:     "",
		},
	}
	mapping, err := integrations.NewJiraFieldMapping("proj-abc", entries, "user@example.com")
	require.NoError(t, err)
	return mapping
}

// ----------------------------------------------------------------------------
// Upsert tests
// ----------------------------------------------------------------------------

func TestGormJiraFieldMappingRepository_Upsert(t *testing.T) {
	t.Run("on first call (no existing row), executes an INSERT", func(t *testing.T) {
		sqlDB, mock, gormDB := setupFieldMappingMockDB(t)
		defer sqlDB.Close()

		repo := repositories.NewGormJiraFieldMappingRepository(gormDB)
		ctx := context.Background()
		mapping := makeTestMapping(t)

		// The upsert (INSERT … ON CONFLICT DO UPDATE) is a single exec statement.
		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO "jira_field_mappings".*ON CONFLICT`).
			WithArgs(
				sqlmock.AnyArg(), // project_id
				sqlmock.AnyArg(), // entries (JSONB)
				sqlmock.AnyArg(), // updated_by
				sqlmock.AnyArg(), // created_at
				sqlmock.AnyArg(), // updated_at
				sqlmock.AnyArg(), // deleted_at
			).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		err := repo.Upsert(ctx, mapping)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("on second call (same project_id), executes an UPDATE not a second INSERT", func(t *testing.T) {
		sqlDB, mock, gormDB := setupFieldMappingMockDB(t)
		defer sqlDB.Close()

		repo := repositories.NewGormJiraFieldMappingRepository(gormDB)
		ctx := context.Background()
		mapping := makeTestMapping(t)

		// First upsert
		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO "jira_field_mappings".*ON CONFLICT`).
			WithArgs(
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
			).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		err := repo.Upsert(ctx, mapping)
		require.NoError(t, err)

		// Second upsert for the same project — must use the INSERT … ON CONFLICT DO UPDATE
		// path, not a bare INSERT (which would fail on conflict).
		mock.ExpectBegin()
		mock.ExpectExec(`INSERT INTO "jira_field_mappings".*ON CONFLICT`).
			WithArgs(
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
			).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		err = repo.Upsert(ctx, mapping)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ----------------------------------------------------------------------------
// Get tests
// ----------------------------------------------------------------------------

func TestGormJiraFieldMappingRepository_Get(t *testing.T) {
	t.Run("returns the aggregate when a row exists", func(t *testing.T) {
		sqlDB, mock, gormDB := setupFieldMappingMockDB(t)
		defer sqlDB.Close()

		repo := repositories.NewGormJiraFieldMappingRepository(gormDB)
		ctx := context.Background()
		projectID := "proj-abc"

		now := time.Now().UTC().Truncate(time.Second)
		entriesJSON := []byte(`[{"FernField":"requirement_id","JiraFieldID":"customfield_10001","JiraFieldIsMultiValue":false,"ReductionStrategy":""}]`)

		rows := sqlmock.NewRows([]string{
			"project_id", "entries", "updated_by", "created_at", "updated_at", "deleted_at",
		}).AddRow(
			projectID,
			entriesJSON,
			"user@example.com",
			now,
			now,
			nil, // not deleted
		)

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).
			WithArgs(projectID, sqlmock.AnyArg()).
			WillReturnRows(rows)

		result, err := repo.Get(ctx, projectID)

		assert.NoError(t, err)
		require.NotNil(t, result)
		snap := result.Snapshot()
		assert.Equal(t, projectID, snap.ProjectID)
		assert.Equal(t, "user@example.com", snap.UpdatedBy)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns nil, nil when no row exists for the project", func(t *testing.T) {
		sqlDB, mock, gormDB := setupFieldMappingMockDB(t)
		defer sqlDB.Close()

		repo := repositories.NewGormJiraFieldMappingRepository(gormDB)
		ctx := context.Background()
		projectID := "proj-missing"

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).
			WithArgs(projectID, sqlmock.AnyArg()).
			WillReturnError(gorm.ErrRecordNotFound)

		result, err := repo.Get(ctx, projectID)

		assert.NoError(t, err)
		assert.Nil(t, result)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("excludes soft-deleted rows (deleted_at IS NOT NULL)", func(t *testing.T) {
		sqlDB, mock, gormDB := setupFieldMappingMockDB(t)
		defer sqlDB.Close()

		repo := repositories.NewGormJiraFieldMappingRepository(gormDB)
		ctx := context.Background()
		projectID := "proj-deleted"

		// GORM's soft-delete scope automatically adds "deleted_at IS NULL" to the
		// WHERE clause. Returning ErrRecordNotFound simulates what GORM does when
		// the only matching row has been soft-deleted (the scope filters it out).
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).
			WithArgs(projectID, sqlmock.AnyArg()).
			WillReturnError(gorm.ErrRecordNotFound)

		result, err := repo.Get(ctx, projectID)

		assert.NoError(t, err)
		assert.Nil(t, result)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ----------------------------------------------------------------------------
// Delete tests
// ----------------------------------------------------------------------------

func TestGormJiraFieldMappingRepository_SaveAfterReset(t *testing.T) {
	t.Run("upsert succeeds after delete (full round-trip)", func(t *testing.T) {
		sqlDB, mock, gormDB := setupFieldMappingMockDB(t)
		defer sqlDB.Close()

		repo := repositories.NewGormJiraFieldMappingRepository(gormDB)
		ctx := context.Background()
		projectID := "proj-abc"

		// Step 1: Delete (soft-delete)
		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "jira_field_mappings" SET "deleted_at"`)).
			WithArgs(sqlmock.AnyArg(), projectID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		require.NoError(t, repo.Delete(ctx, projectID))

		// Step 2: Upsert (insert new row after soft-delete)
		mapping := makeTestMapping(t)
		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO "jira_field_mappings"`)).
			WithArgs(
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
			).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		require.NoError(t, repo.Upsert(ctx, mapping))

		// Step 3: Get returns the new row
		now := time.Now().UTC().Truncate(time.Second)
		entriesJSON := []byte(`[{"FernField":"requirement_id","JiraFieldID":"customfield_10001","JiraFieldIsMultiValue":false,"ReductionStrategy":""}]`)
		rows := sqlmock.NewRows([]string{
			"project_id", "entries", "updated_by", "created_at", "updated_at", "deleted_at",
		}).AddRow(projectID, entriesJSON, "user@example.com", now, now, nil)

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT`)).
			WithArgs(projectID, sqlmock.AnyArg()).
			WillReturnRows(rows)

		result, err := repo.Get(ctx, projectID)
		require.NoError(t, err)
		require.NotNil(t, result)
		snap := result.Snapshot()
		assert.Equal(t, projectID, snap.ProjectID)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestGormJiraFieldMappingRepository_Delete(t *testing.T) {
	t.Run("soft-deletes by setting deleted_at (does NOT hard-delete)", func(t *testing.T) {
		sqlDB, mock, gormDB := setupFieldMappingMockDB(t)
		defer sqlDB.Close()

		repo := repositories.NewGormJiraFieldMappingRepository(gormDB)
		ctx := context.Background()
		projectID := "proj-abc"

		// A soft-delete issues an UPDATE that sets deleted_at, not a DELETE statement.
		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "jira_field_mappings" SET "deleted_at"`)).
			WithArgs(
				sqlmock.AnyArg(), // deleted_at timestamp
				projectID,        // WHERE project_id = ?
			).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		err := repo.Delete(ctx, projectID)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
