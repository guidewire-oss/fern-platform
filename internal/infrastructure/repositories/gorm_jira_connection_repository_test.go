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

// A freshly constructed connection's domain ID is actually empty (see
// integrations.NewJiraConnection) -- this simulates a mis-constructed
// domain object carrying a non-empty, non-row ID instead, the shape of bug
// a prior version of NewJiraConnection used to produce before it was fixed
// to leave new connections' IDs empty. This particular value starts with a
// decimal digit run before the first hex letter, which is exactly the
// shape that fmt.Sscanf(id, "%d", ...) used to silently parse as a small
// integer.
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
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("errors instead of stamping a zero ID when the driver returns no primary key", func(t *testing.T) {
		sqlDB, mock, gormDB := setupJiraConnectionMockDB(t)
		defer sqlDB.Close()

		repo := repositories.NewGormJiraConnectionRepository(gormDB)
		conn := makeNewJiraConnection(t)

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "jira_connections"`)).
			WillReturnRows(sqlmock.NewRows([]string{"id"}))
		mock.ExpectCommit()

		err := repo.Create(context.Background(), conn)

		require.Error(t, err)
		// The domain object must keep its original (throwaway) ID rather than
		// being stamped with "0", which would collide with every other
		// connection that also failed to get a real PK back.
		assert.Equal(t, uuidLikeDomainID, conn.ID())
	})
}

func makeExistingJiraConnection(t *testing.T, id string) *integrations.JiraConnection {
	t.Helper()
	return integrations.ReconstructJiraConnection(
		id,
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

func TestGormJiraConnectionRepository_Update(t *testing.T) {
	t.Run("updates using the numeric row ID parsed from the domain object", func(t *testing.T) {
		sqlDB, mock, gormDB := setupJiraConnectionMockDB(t)
		defer sqlDB.Close()

		repo := repositories.NewGormJiraConnectionRepository(gormDB)
		conn := makeExistingJiraConnection(t, "42")

		mock.ExpectBegin()
		mock.ExpectExec(regexp.QuoteMeta(`UPDATE "jira_connections" SET`)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		err := repo.Update(context.Background(), conn)

		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns an error instead of silently inserting a duplicate row when the domain ID is not numeric", func(t *testing.T) {
		sqlDB, mock, gormDB := setupJiraConnectionMockDB(t)
		defer sqlDB.Close()

		repo := repositories.NewGormJiraConnectionRepository(gormDB)
		// Same shape of bug the Create fix targets: a non-numeric ID (e.g. a
		// UUID from a mis-reconstructed domain object) must never be
		// silently coerced -- that previously produced a truncated integer
		// via fmt.Sscanf, which Save() could then INSERT as a new row.
		conn := makeExistingJiraConnection(t, uuidLikeDomainID)

		err := repo.Update(context.Background(), conn)

		// Must be rejected by our own validation, not surfaced as a generic
		// "unexpected DB call" error from an un-mocked query -- that would
		// mean the bad ID reached Save() and, in a real DB, could silently
		// insert a duplicate row instead of failing loudly.
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid")
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("returns an error instead of inserting when the domain ID is unset", func(t *testing.T) {
		sqlDB, mock, gormDB := setupJiraConnectionMockDB(t)
		defer sqlDB.Close()

		repo := repositories.NewGormJiraConnectionRepository(gormDB)
		conn := makeExistingJiraConnection(t, "")

		err := repo.Update(context.Background(), conn)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "unset")
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
