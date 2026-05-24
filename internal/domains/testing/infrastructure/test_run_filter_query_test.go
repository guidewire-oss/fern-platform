package infrastructure_test

import (
	"strings"
	"testing"
	"time"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/guidewire-oss/fern-platform/internal/domains/testing/infrastructure"
)

func TestBuildTestRunWhere_Empty(t *testing.T) {
	clauses, args := infrastructure.BuildTestRunWhere(domain.TestRunFilter{})
	if len(clauses) != 0 {
		t.Errorf("empty filter should produce no clauses, got %v", clauses)
	}
	if len(args) != 0 {
		t.Errorf("empty filter should produce no args, got %v", args)
	}
}

func TestBuildTestRunWhere_ProjectAndStatus(t *testing.T) {
	f := domain.TestRunFilter{
		ProjectIDs: []string{"p1", "p2"},
		Status:     []string{"failed"},
	}
	clauses, args := infrastructure.BuildTestRunWhere(f)

	joined := strings.Join(clauses, " AND ")
	if !strings.Contains(joined, "project_id IN") {
		t.Errorf("expected project_id IN clause, got %q", joined)
	}
	if !strings.Contains(joined, "status IN") {
		t.Errorf("expected status IN clause, got %q", joined)
	}

	// IN ? takes a slice as a single GORM arg, so projects + status = 2.
	if len(args) != 2 {
		t.Fatalf("expected 2 args (slice + slice), got %d: %v", len(args), args)
	}
}

func TestBuildTestRunWhere_DateRange(t *testing.T) {
	from := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC)
	f := domain.TestRunFilter{
		StartedAt: &domain.DateTimeRange{Gte: &from, Lte: &to},
	}
	clauses, args := infrastructure.BuildTestRunWhere(f)

	joined := strings.Join(clauses, " AND ")
	if !strings.Contains(joined, "start_time >=") {
		t.Errorf("expected start_time >= clause, got %q", joined)
	}
	if !strings.Contains(joined, "start_time <=") {
		t.Errorf("expected start_time <= clause, got %q", joined)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %v", args)
	}
}

func TestBuildTestRunWhere_DurationRangeLowerOnly(t *testing.T) {
	gte := 1000
	f := domain.TestRunFilter{
		DurationMs: &domain.IntRange{Gte: &gte},
	}
	clauses, args := infrastructure.BuildTestRunWhere(f)

	if len(clauses) != 1 {
		t.Fatalf("expected 1 clause, got %v", clauses)
	}
	if !strings.Contains(clauses[0], "duration_ms >=") {
		t.Errorf("got %q", clauses[0])
	}
	if len(args) != 1 || args[0] != gte {
		t.Errorf("args = %v", args)
	}
}

func TestBuildTestRunWhere_GitCommit(t *testing.T) {
	c := "abcdef0"
	f := domain.TestRunFilter{GitCommit: &c}
	clauses, args := infrastructure.BuildTestRunWhere(f)
	if len(clauses) != 1 || !strings.Contains(clauses[0], "commit_sha =") {
		t.Errorf("got %v", clauses)
	}
	if len(args) != 1 || args[0] != c {
		t.Errorf("args = %v", args)
	}
}

func TestBuildTestRunWhere_BranchesIn(t *testing.T) {
	f := domain.TestRunFilter{Branches: []string{"main", "release"}}
	clauses, args := infrastructure.BuildTestRunWhere(f)
	if len(clauses) != 1 || !strings.Contains(clauses[0], "branch IN") {
		t.Errorf("got %v", clauses)
	}
	if len(args) != 1 {
		t.Errorf("branch IN takes a single slice arg; got %v", args)
	}
}

func TestBuildTestRunWhere_SearchAddsExistsSubquery(t *testing.T) {
	q := "oauth redirect"
	f := domain.TestRunFilter{Search: &q}
	clauses, args := infrastructure.BuildTestRunWhere(f)

	if len(clauses) != 1 {
		t.Fatalf("expected 1 clause, got %v", clauses)
	}
	if !strings.Contains(clauses[0], "EXISTS") || !strings.Contains(clauses[0], "ILIKE") {
		t.Errorf("search clause should be an ILIKE EXISTS subquery, got %q", clauses[0])
	}
	if len(args) != 1 || args[0] != "%"+q+"%" {
		t.Errorf("expected ILIKE pattern wrapping the query, got %v", args)
	}
}

func TestBuildTestRunWhere_TagsOrMode(t *testing.T) {
	f := domain.TestRunFilter{Tags: []string{"smoke", "release"}, TagMode: domain.LogicOr}
	clauses, args := infrastructure.BuildTestRunWhere(f)
	joined := strings.Join(clauses, " AND ")
	if !strings.Contains(joined, "EXISTS") {
		t.Errorf("OR-mode tags should use a single EXISTS, got %q", joined)
	}
	if len(args) != 1 {
		t.Errorf("OR-mode tags takes a single slice arg; got %v", args)
	}
}

func TestBuildTestRunWhere_TagsAndModeUsesMultipleExists(t *testing.T) {
	f := domain.TestRunFilter{Tags: []string{"smoke", "release"}, TagMode: domain.LogicAnd}
	clauses, _ := infrastructure.BuildTestRunWhere(f)
	if len(clauses) != 2 {
		t.Fatalf("AND-mode should produce one EXISTS per tag (got %d)", len(clauses))
	}
}

func TestBuildTestRunOrderBy(t *testing.T) {
	got := infrastructure.BuildTestRunOrderBy()
	if !strings.Contains(got, "start_time DESC") || !strings.Contains(got, "id DESC") {
		t.Errorf("ORDER BY should be deterministic keyset: %q", got)
	}
}
