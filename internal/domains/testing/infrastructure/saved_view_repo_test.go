package infrastructure_test

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/infrastructure"
)

func openSavedViewsDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// SQLite cannot represent the cross-table FK to users(user_id) used
	// in the production migration; for repo-shape tests we use a stripped
	// schema that exercises the same column names + unique constraint.
	if err := db.Exec(`
		CREATE TABLE saved_views (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id     TEXT NOT NULL,
			page        TEXT NOT NULL,
			name        TEXT NOT NULL,
			filter_json BLOB NOT NULL,
			created_at  DATETIME,
			updated_at  DATETIME,
			UNIQUE (user_id, page, name)
		)
	`).Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func TestSavedView_CreateAndList(t *testing.T) {
	db := openSavedViewsDB(t)
	repo := infrastructure.NewGormSavedViewRepository(db)
	ctx := context.Background()

	v := &domain.SavedView{
		UserID: "u1", Page: "test-runs", Name: "Failures on main",
		FilterJSON: []byte(`{"status":["failed"]}`),
	}
	if err := repo.Create(ctx, v); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if v.ID == 0 {
		t.Error("Create should populate ID")
	}

	got, err := repo.List(ctx, "u1", "test-runs")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "Failures on main" {
		t.Errorf("List = %+v", got)
	}
}

func TestSavedView_DuplicateNameConflicts(t *testing.T) {
	db := openSavedViewsDB(t)
	repo := infrastructure.NewGormSavedViewRepository(db)
	ctx := context.Background()

	v := &domain.SavedView{UserID: "u1", Page: "test-runs", Name: "Dup", FilterJSON: []byte(`{}`)}
	if err := repo.Create(ctx, v); err != nil {
		t.Fatal(err)
	}
	dup := &domain.SavedView{UserID: "u1", Page: "test-runs", Name: "Dup", FilterJSON: []byte(`{}`)}
	err := repo.Create(ctx, dup)
	if !errors.Is(err, domain.ErrSavedViewConflict) {
		t.Errorf("expected conflict, got %v", err)
	}
}

func TestSavedView_ListScopedByUser(t *testing.T) {
	db := openSavedViewsDB(t)
	repo := infrastructure.NewGormSavedViewRepository(db)
	ctx := context.Background()

	mustCreate := func(uid, page, name string) {
		if err := repo.Create(ctx, &domain.SavedView{
			UserID: uid, Page: page, Name: name, FilterJSON: []byte(`{}`),
		}); err != nil {
			t.Fatal(err)
		}
	}
	mustCreate("u1", "test-runs", "A")
	mustCreate("u2", "test-runs", "A")
	mustCreate("u1", "flaky", "B")

	got, err := repo.List(ctx, "u1", "test-runs")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].UserID != "u1" || got[0].Page != "test-runs" {
		t.Errorf("List leaked across users or pages: %+v", got)
	}
}

func TestSavedView_DeleteRespectsOwner(t *testing.T) {
	db := openSavedViewsDB(t)
	repo := infrastructure.NewGormSavedViewRepository(db)
	ctx := context.Background()

	v := &domain.SavedView{UserID: "u1", Page: "p", Name: "n", FilterJSON: []byte(`{}`)}
	if err := repo.Create(ctx, v); err != nil {
		t.Fatal(err)
	}
	// Wrong user: must not delete.
	err := repo.Delete(ctx, "u2", v.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("delete-as-wrong-user should be NotFound, got %v", err)
	}
	// Right user: succeeds.
	if err := repo.Delete(ctx, "u1", v.ID); err != nil {
		t.Errorf("Delete: %v", err)
	}
}
