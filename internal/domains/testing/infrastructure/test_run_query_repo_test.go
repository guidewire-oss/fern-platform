package infrastructure_test

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/infrastructure"
	"github.com/guidewire-oss/fern-platform/pkg/database"
)

func openSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	seedCounter = 0
	// `:memory:` is per-connection in SQLite, but the new errgroup-based
	// ComputeFacets fans out four queries concurrently. With the default
	// (unbounded) connection pool, each goroutine could land on a fresh
	// connection and see an empty database. Pinning MaxOpenConns to 1
	// keeps all queries on the same in-memory store, which is what these
	// tests assume.
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
	if err := db.AutoMigrate(&database.TestRun{}, &database.SuiteRun{}, &database.SpecRun{}, &database.Tag{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

var seedCounter int

func seedRuns(t *testing.T, db *gorm.DB, n int, project, status, branch string) {
	t.Helper()
	base := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		seedCounter++
		row := database.TestRun{
			ProjectID: project,
			RunID:     uniqueRunID(),
			Branch:    branch,
			Status:    status,
			StartTime: base.Add(time.Duration(seedCounter) * time.Hour),
		}
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
}

func uniqueRunID() string { return "run-" + itoa(seedCounter) }
func itoa(i int) string   { return fmtInt(i) }
func fmtInt(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}

func TestQuery_EmptyFilterReturnsRecentFirst(t *testing.T) {
	db := openSQLite(t)
	seedRuns(t, db, 3, "p1", "passed", "main")

	repo := infrastructure.NewTestRunQueryRepo(db)
	page, err := repo.Query(context.Background(), domain.TestRunFilter{}, domain.PageArgs{First: 50})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(page.Edges) != 3 {
		t.Fatalf("expected 3 edges, got %d", len(page.Edges))
	}
	// Newest first: each edge's StartTime should be >= the next one's.
	for i := 0; i+1 < len(page.Edges); i++ {
		if page.Edges[i].Node.StartTime.Before(page.Edges[i+1].Node.StartTime) {
			t.Errorf("ORDER BY broken: edge %d (%v) older than edge %d (%v)",
				i, page.Edges[i].Node.StartTime, i+1, page.Edges[i+1].Node.StartTime)
		}
	}
	if page.PageInfo.HasNextPage {
		t.Error("HasNextPage should be false when full page fits")
	}
}

func TestQuery_PaginationAfterCursorAdvancesPage(t *testing.T) {
	// Regression: pre-fix, p.After was ignored and "next page" returned
	// page 1 again. With the keyset predicate it must return strictly
	// older rows than the cursor.
	db := openSQLite(t)
	for i := 0; i < 5; i++ {
		seedRuns(t, db, 1, "p1", "passed", "main")
	}
	repo := infrastructure.NewTestRunQueryRepo(db)
	page1, err := repo.Query(context.Background(), domain.TestRunFilter{}, domain.PageArgs{First: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page1.Edges) != 2 || !page1.PageInfo.HasNextPage {
		t.Fatalf("page1 unexpected: edges=%d hasNext=%v", len(page1.Edges), page1.PageInfo.HasNextPage)
	}
	cursor := page1.PageInfo.EndCursor
	if cursor == "" {
		t.Fatal("page1.EndCursor should be set when HasNextPage=true")
	}
	page2, err := repo.Query(context.Background(), domain.TestRunFilter{}, domain.PageArgs{First: 2, After: cursor})
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(page2.Edges) != 2 {
		t.Fatalf("page2: expected 2 edges, got %d", len(page2.Edges))
	}
	// Cursor row must not reappear, and page2 rows must be strictly older.
	page1Last := page1.Edges[len(page1.Edges)-1].Node
	for _, e := range page2.Edges {
		if e.Node.ID == page1Last.ID {
			t.Errorf("page2 leaked page1 last row id=%d", e.Node.ID)
		}
		if !e.Node.StartTime.Before(page1Last.StartTime) {
			t.Errorf("page2 row %+v not strictly older than %v", e.Node, page1Last.StartTime)
		}
	}
}

func TestQuery_PaginationRejectsMalformedCursor(t *testing.T) {
	db := openSQLite(t)
	seedRuns(t, db, 1, "p1", "passed", "main")
	repo := infrastructure.NewTestRunQueryRepo(db)
	_, err := repo.Query(context.Background(), domain.TestRunFilter{},
		domain.PageArgs{First: 2, After: "not-a-cursor"})
	if err == nil {
		t.Error("malformed cursor should error rather than silently restart")
	}
}

func TestQuery_FiltersByProjectAndStatus(t *testing.T) {
	db := openSQLite(t)
	seedRuns(t, db, 2, "p1", "passed", "main")
	seedRuns(t, db, 2, "p1", "failed", "main")
	seedRuns(t, db, 2, "p2", "failed", "main")

	repo := infrastructure.NewTestRunQueryRepo(db)
	page, err := repo.Query(context.Background(),
		domain.TestRunFilter{ProjectIDs: []string{"p1"}, Status: []string{"failed"}},
		domain.PageArgs{First: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(page.Edges))
	}
	for _, e := range page.Edges {
		if e.Node.ProjectID != "p1" || e.Node.Status != "failed" {
			t.Errorf("unexpected row: %+v", e.Node)
		}
	}
}

func TestQuery_PaginationHasNextPage(t *testing.T) {
	db := openSQLite(t)
	// 5 rows, page size 3 → expect hasNextPage and 3 edges.
	for i := 0; i < 5; i++ {
		seedRuns(t, db, 1, "p1", "passed", "main-"+itoa(i))
	}
	repo := infrastructure.NewTestRunQueryRepo(db)
	page, err := repo.Query(context.Background(), domain.TestRunFilter{}, domain.PageArgs{First: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Edges) != 3 {
		t.Errorf("expected 3 edges, got %d", len(page.Edges))
	}
	if !page.PageInfo.HasNextPage {
		t.Error("expected HasNextPage=true with more rows available")
	}
	if page.PageInfo.EndCursor == "" {
		t.Error("expected non-empty EndCursor when HasNextPage")
	}
}

func TestQuery_NarrowFilterCountsExact(t *testing.T) {
	db := openSQLite(t)
	seedRuns(t, db, 4, "p1", "passed", "main")

	repo := infrastructure.NewTestRunQueryRepo(db)
	page, err := repo.Query(context.Background(),
		domain.TestRunFilter{ProjectIDs: []string{"p1"}},
		domain.PageArgs{First: 50})
	if err != nil {
		t.Fatal(err)
	}
	if page.TotalCount != 4 {
		t.Errorf("TotalCount=%d, want 4", page.TotalCount)
	}
	if page.TotalCountIsEstimate {
		t.Error("narrow filter should return exact count")
	}
}

func TestQuery_ComputeFacets(t *testing.T) {
	db := openSQLite(t)
	seedRuns(t, db, 3, "p1", "passed", "main")
	seedRuns(t, db, 2, "p1", "failed", "main")
	seedRuns(t, db, 1, "p1", "failed", "release")

	repo := infrastructure.NewTestRunQueryRepo(db)
	facets, err := repo.ComputeFacets(context.Background(),
		domain.TestRunFilter{ProjectIDs: []string{"p1"}, Status: []string{"failed"}})
	if err != nil {
		t.Fatal(err)
	}
	// byStatus: scoped to (project=p1) ignoring status filter, so we
	// should see both passed=3 and failed=3.
	got := map[string]int64{}
	for _, fc := range facets.ByStatus {
		got[fc.Value] = fc.Count
	}
	if got["passed"] != 3 || got["failed"] != 3 {
		t.Errorf("byStatus = %+v", got)
	}
	// byBranch: scoped to (project=p1, status=failed) ignoring branch
	// filter, so main=2, release=1.
	got = map[string]int64{}
	for _, fc := range facets.ByBranch {
		got[fc.Value] = fc.Count
	}
	if got["main"] != 2 || got["release"] != 1 {
		t.Errorf("byBranch = %+v", got)
	}
}

func TestQuery_ComputeTagFacet(t *testing.T) {
	db := openSQLite(t)
	// Seed two runs sharing a suite each, tagged differently.
	tagSmoke := database.Tag{Name: "smoke"}
	tagRelease := database.Tag{Name: "release"}
	if err := db.Create(&tagSmoke).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&tagRelease).Error; err != nil {
		t.Fatal(err)
	}

	mkRun := func(tags ...database.Tag) database.TestRun {
		seedCounter++
		run := database.TestRun{
			ProjectID: "p1",
			RunID:     uniqueRunID(),
			Status:    "passed",
			Branch:    "main",
			StartTime: time.Now().Add(time.Duration(seedCounter) * time.Minute),
		}
		if err := db.Create(&run).Error; err != nil {
			t.Fatal(err)
		}
		suite := database.SuiteRun{TestRunID: run.ID, SuiteName: "s", Tags: tags}
		if err := db.Create(&suite).Error; err != nil {
			t.Fatal(err)
		}
		return run
	}
	mkRun(tagSmoke)
	mkRun(tagSmoke, tagRelease)
	mkRun(tagRelease)

	repo := infrastructure.NewTestRunQueryRepo(db)
	facets, err := repo.ComputeFacets(context.Background(),
		domain.TestRunFilter{ProjectIDs: []string{"p1"}, IncludeTagFacet: true})
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int64{}
	for _, fc := range facets.ByTag {
		counts[fc.Value] = fc.Count
	}
	// smoke appears in two runs, release in two.
	if counts["smoke"] != 2 || counts["release"] != 2 {
		t.Errorf("byTag counts wrong: %+v", counts)
	}
}

func TestQuery_DateRangeFilter(t *testing.T) {
	db := openSQLite(t)
	seedRuns(t, db, 5, "p1", "passed", "main")

	from := time.Date(2026, 5, 1, 1, 30, 0, 0, time.UTC)
	to := time.Date(2026, 5, 1, 3, 30, 0, 0, time.UTC)
	repo := infrastructure.NewTestRunQueryRepo(db)
	page, err := repo.Query(context.Background(),
		domain.TestRunFilter{StartedAt: &domain.DateTimeRange{Gte: &from, Lte: &to}},
		domain.PageArgs{First: 50})
	if err != nil {
		t.Fatal(err)
	}
	// Seeds at hour offsets 0..4; range [1:30, 3:30] catches hours 2 and 3.
	if len(page.Edges) != 2 {
		t.Errorf("date range: expected 2 edges, got %d", len(page.Edges))
	}
}
