package infrastructure_test

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/infrastructure"
	"github.com/guidewire-oss/fern-platform/pkg/database"
)

func openProjectDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&database.ProjectDetails{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// countingLogger tallies every SQL statement GORM executes, which is how
// the tests below assert the "one batched query per call" guarantee
// (Requirement 1.4) without reaching into GORM's callback internals.
type countingLogger struct {
	logger.Interface
	n *int
}

func (c countingLogger) Trace(_ context.Context, _ time.Time, _ func() (string, int64), _ error) {
	*c.n++
}

// countQueries returns a session that tallies statements into n.
func countQueries(db *gorm.DB, n *int) *gorm.DB {
	return db.Session(&gorm.Session{
		Logger: countingLogger{Interface: logger.Default.LogMode(logger.Silent), n: n},
	})
}

func seedProject(t *testing.T, db *gorm.DB, projectID, name string) {
	t.Helper()
	if err := db.Create(&database.ProjectDetails{
		ProjectID: projectID,
		Name:      name,
	}).Error; err != nil {
		t.Fatalf("seed project %s: %v", projectID, err)
	}
}

func TestNamesByProjectIDResolvesKnownProjects(t *testing.T) {
	db := openProjectDB(t)
	seedProject(t, db, "proj-a", "Alpha Service")
	seedProject(t, db, "proj-b", "Beta Service")

	repo := infrastructure.NewProjectNameRepo(db)
	got, err := repo.NamesByProjectID(context.Background(), []string{"proj-a", "proj-b"})
	if err != nil {
		t.Fatalf("NamesByProjectID: %v", err)
	}
	if got["proj-a"] != "Alpha Service" {
		t.Errorf("proj-a = %q, want %q", got["proj-a"], "Alpha Service")
	}
	if got["proj-b"] != "Beta Service" {
		t.Errorf("proj-b = %q, want %q", got["proj-b"], "Beta Service")
	}
}

// Requirement 1.3: an ID with no project_details row must be absent from
// the map rather than producing an error, so the caller can fall back to
// showing the raw ID.
func TestNamesByProjectIDOmitsUnknownAndUnnamedProjects(t *testing.T) {
	db := openProjectDB(t)
	seedProject(t, db, "proj-a", "Alpha Service")
	seedProject(t, db, "proj-blank", "")

	repo := infrastructure.NewProjectNameRepo(db)
	got, err := repo.NamesByProjectID(context.Background(),
		[]string{"proj-a", "proj-blank", "proj-missing"})
	if err != nil {
		t.Fatalf("NamesByProjectID: %v", err)
	}
	if _, ok := got["proj-missing"]; ok {
		t.Error("proj-missing should be absent from the map")
	}
	if _, ok := got["proj-blank"]; ok {
		t.Error("a project with an empty name should be absent from the map")
	}
	if len(got) != 1 {
		t.Errorf("map size = %d, want 1", len(got))
	}
}

// Requirement 1.4: one query per request regardless of page size. A
// duplicated ID list must collapse to a single IN (...) lookup.
func TestNamesByProjectIDDeduplicatesAndQueriesOnce(t *testing.T) {
	db := openProjectDB(t)
	seedProject(t, db, "proj-a", "Alpha Service")

	var queries int
	repo := infrastructure.NewProjectNameRepo(countQueries(db, &queries))
	got, err := repo.NamesByProjectID(context.Background(),
		[]string{"proj-a", "proj-a", "proj-a", "proj-a"})
	if err != nil {
		t.Fatalf("NamesByProjectID: %v", err)
	}
	if got["proj-a"] != "Alpha Service" {
		t.Errorf("proj-a = %q, want %q", got["proj-a"], "Alpha Service")
	}
	if queries != 1 {
		t.Errorf("issued %d queries, want 1", queries)
	}
}

func TestNamesByProjectIDEmptyInputIssuesNoQuery(t *testing.T) {
	db := openProjectDB(t)

	var queries int
	repo := infrastructure.NewProjectNameRepo(countQueries(db, &queries))
	got, err := repo.NamesByProjectID(context.Background(), nil)
	if err != nil {
		t.Fatalf("NamesByProjectID: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("map size = %d, want 0", len(got))
	}
	if queries != 0 {
		t.Errorf("issued %d queries, want 0", queries)
	}
}
