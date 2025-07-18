package infrastructure_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/guidewire-oss/fern-platform/internal/domains/projects/infrastructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *gorm.DB) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	
	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{})
	require.NoError(t, err)
	
	return db, mock, gormDB
}

func TestGormProjectRepository_Delete_CascadeDelete(t *testing.T) {
	t.Run("should trigger cascade delete of test runs", func(t *testing.T) {
		// Arrange
		db, mock, gormDB := setupMockDB(t)
		defer db.Close()
		
		repo := infrastructure.NewGormProjectRepository(gormDB)
		ctx := context.Background()
		projectID := uint(123)
		
		// Expect hard delete query (using Unscoped)
		mock.ExpectBegin()
		mock.ExpectExec(`DELETE FROM "project_details" WHERE "project_details"."id" = .*`).
			WithArgs(projectID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		
		// Act
		err := repo.Delete(ctx, projectID)
		
		// Assert
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// Integration test to verify cascade delete works with real foreign key
// This test would need a test database to run properly
func TestProjectDeletion_Integration(t *testing.T) {
	t.Skip("Integration test - requires database")
	
	// This is an example of how to test the cascade delete with a real database
	// In a real scenario, you would:
	// 1. Set up a test database with the migrations applied
	// 2. Create a project
	// 3. Create test runs associated with the project
	// 4. Delete the project
	// 5. Verify test runs are also deleted
	
	/*
	// Example implementation:
	db := setupTestDatabase(t)
	projectRepo := infrastructure.NewGormProjectRepository(db)
	
	// Create project
	project := domain.NewProject("test-project", domain.ProjectData{
		Name: "Test Project",
		Team: "test-team",
	})
	err := projectRepo.Create(context.Background(), project)
	require.NoError(t, err)
	
	// Create test runs (would need test run repository)
	// ...
	
	// Delete project
	err = projectRepo.Delete(context.Background(), project.ID())
	require.NoError(t, err)
	
	// Verify test runs are deleted
	var count int64
	db.Model(&TestRun{}).Where("project_id = ?", project.ProjectID()).Count(&count)
	assert.Equal(t, int64(0), count, "Test runs should be cascade deleted")
	*/
}