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

func setupJiraConnectionMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *gorm.DB) {
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

// A freshly constructed connection carries a UUID as its domain ID (see
// integrations.NewJiraConnection). This particular value starts with a
// decimal digit run before the first hex letter, which is exactly the shape
// that fmt.Sscanf(id, "%d", ...) silently parses as a small integer.
const uuidLikeDomainID = "3fa85f64-5717-4562-b3fc-2c963f66afa6"

func makeNewJiraConnection(t *testing.T) *integrations.JiraConnection {
	t.Helper()
	return integrations.ReconstructJiraConnection(
		uuidLikeDomainID,
		"proj-abc",
		"My Connection",
		"https://jira.example.com",
		integrations.AuthTypeAPIToken,
		"PROJ",
		"user@example.com",
		"encrypted-credential",
		integrations.ConnectionStatusPending,
		false,
		"",
		nil,
		time.Now(),
		time.Now(),
	)
}

func TestGormJiraConnectionRepository_Create(t *testing.T) {
	t.Run("does not force the domain's UUID onto the primary key column", func(t *testing.T) {
		sqlDB, mock, gormDB := setupJiraConnectionMockDB(t)
		defer sqlDB.Close()

		repo := repositories.NewGormJiraConnectionRepository(gormDB)
		conn := makeNewJiraConnection(t)

		// Exactly 14 columns, none of them "id" -- the database must assign
		// the primary key. If the buggy code parses the leading digits of the
		// UUID into model.ID, GORM adds an extra "id" argument here and this
		// expectation (14 AnyArg()s) fails to match.
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "jira_connections"`)).
			WithArgs(
				sqlmock.AnyArg(), // created_at
				sqlmock.AnyArg(), // updated_at
				sqlmock.AnyArg(), // deleted_at
				sqlmock.AnyArg(), // project_id
				sqlmock.AnyArg(), // name
				sqlmock.AnyArg(), // jira_url
				sqlmock.AnyArg(), // authentication_type
				sqlmock.AnyArg(), // project_key
				sqlmock.AnyArg(), // username
				sqlmock.AnyArg(), // encrypted_credential
				sqlmock.AnyArg(), // status
				sqlmock.AnyArg(), // is_active
				sqlmock.AnyArg(), // last_tested_at
				sqlmock.AnyArg(), // version_filter
			).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
		mock.ExpectCommit()

		err := repo.Create(context.Background(), conn)

		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("propagates the database-assigned ID back onto the domain object", func(t *testing.T) {
		sqlDB, mock, gormDB := setupJiraConnectionMockDB(t)
		defer sqlDB.Close()

		repo := repositories.NewGormJiraConnectionRepository(gormDB)
		conn := makeNewJiraConnection(t)

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "jira_connections"`)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
		mock.ExpectCommit()

		err := repo.Create(context.Background(), conn)

		require.NoError(t, err)
		// Subsequent lookups (test/update/delete) key off conn.ID(), which is
		// compared against a uint primary key column -- it must reflect the
		// real row id, not the throwaway UUID minted at construction time.
		assert.Equal(t, "42", conn.ID())
	})
}
