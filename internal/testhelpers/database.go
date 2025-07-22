package testhelpers

import (
	"fmt"
	"os"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/guidewire-oss/fern-platform/pkg/database"
)

// TestDatabase provides a test database instance
type TestDatabase struct {
	DB       *gorm.DB
	tempFile string
}

// NewTestDatabase creates a new test database
func NewTestDatabase(t *testing.T) *TestDatabase {
	// Create a temporary SQLite database for testing
	tempFile := fmt.Sprintf("/tmp/fern_test_%s.db", t.Name())
	
	// Open SQLite database
	db, err := gorm.Open(sqlite.Open(tempFile), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	
	// Auto-migrate all models
	err = db.AutoMigrate(
		&database.ProjectPermission{},
		&database.User{},
		&database.TestRun{},
		&database.SuiteRun{},
		&database.SpecRun{},
		&database.Tag{},
		&database.TestRunTag{},
	)
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}
	
	return &TestDatabase{
		DB:       db,
		tempFile: tempFile,
	}
}

// Cleanup removes the test database
func (td *TestDatabase) Cleanup() {
	sqlDB, _ := td.DB.DB()
	if sqlDB != nil {
		sqlDB.Close()
	}
	os.Remove(td.tempFile)
}

// Clear removes all data from tables
func (td *TestDatabase) Clear() {
	// Clear in reverse order of dependencies
	td.DB.Exec("DELETE FROM test_run_tags")
	td.DB.Exec("DELETE FROM spec_runs")
	td.DB.Exec("DELETE FROM suite_runs")
	td.DB.Exec("DELETE FROM test_runs")
	td.DB.Exec("DELETE FROM project_permissions")
	td.DB.Exec("DELETE FROM projects")
	td.DB.Exec("DELETE FROM users")
	td.DB.Exec("DELETE FROM tags")
}


// SeedUsers adds test users to the database
func (td *TestDatabase) SeedUsers(users ...*database.User) error {
	for _, user := range users {
		if err := td.DB.Create(user).Error; err != nil {
			return err
		}
	}
	return nil
}

// SeedTestRuns adds test runs to the database
func (td *TestDatabase) SeedTestRuns(testRuns ...*database.TestRun) error {
	for _, testRun := range testRuns {
		if err := td.DB.Create(testRun).Error; err != nil {
			return err
		}
	}
	return nil
}

// SeedSuiteRuns adds suite runs to the database
func (td *TestDatabase) SeedSuiteRuns(suiteRuns ...*database.SuiteRun) error {
	for _, suiteRun := range suiteRuns {
		if err := td.DB.Create(suiteRun).Error; err != nil {
			return err
		}
	}
	return nil
}

// SeedTags adds tags to the database
func (td *TestDatabase) SeedTags(tags ...*database.Tag) error {
	for _, tag := range tags {
		if err := td.DB.Create(tag).Error; err != nil {
			return err
		}
	}
	return nil
}

// AssertCount asserts the count of records in a table
func (td *TestDatabase) AssertCount(t *testing.T, model interface{}, expected int64) {
	var count int64
	td.DB.Model(model).Count(&count)
	if count != expected {
		t.Errorf("Expected %d records, got %d", expected, count)
	}
}

// TransactionTest runs a test in a transaction and rolls it back
func (td *TestDatabase) TransactionTest(t *testing.T, fn func(*gorm.DB)) {
	tx := td.DB.Begin()
	defer tx.Rollback()
	
	fn(tx)
}