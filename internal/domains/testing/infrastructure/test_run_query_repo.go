package infrastructure

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

	"github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"
	"github.com/guidewire-oss/fern-platform/pkg/database"
)

// TestRunQueryRepo executes filtered, paginated, faceted test-run
// queries for the v2 API. It is a read-only repository — writes still
// go through GormTestRunRepository.
//
// Design choices:
//   - Pagination uses keyset (start_time DESC, id DESC) with a Relay-
//     style "fetch first+1, drop the extra to detect hasNextPage".
//   - Filter→SQL is delegated to BuildTestRunWhere so the translator
//     can be unit-tested without a DB.
//   - Count exact vs estimate is chosen by ChooseCountStrategy.
//   - Facet queries run sequentially today; parallelizing is a
//     follow-up once we have load numbers from staging.
type TestRunQueryRepo struct {
	db *gorm.DB
}

// NewTestRunQueryRepo builds a query repo on top of an existing GORM handle.
func NewTestRunQueryRepo(db *gorm.DB) *TestRunQueryRepo {
	return &TestRunQueryRepo{db: db}
}

// Query satisfies the application-layer TestRunQueryService interface
// expected by the v2 HTTP handler.
func (r *TestRunQueryRepo) Query(ctx context.Context, filter domain.TestRunFilter, page domain.PageArgs) (domain.TestRunPage, error) {
	page.Normalize()
	if err := filter.Validate(); err != nil {
		return domain.TestRunPage{}, err
	}

	rows, hasMore, err := r.fetchPage(ctx, filter, page)
	if err != nil {
		return domain.TestRunPage{}, fmt.Errorf("fetch page: %w", err)
	}

	total, isEstimate, err := r.count(ctx, filter)
	if err != nil {
		// Counting is best-effort; a failure should not blank the page.
		// Returning 0 + estimate=true lets the UI render with a "≈"
		// rather than erroring on the whole list response.
		total = int64(len(rows))
		isEstimate = true
	}

	edges := make([]domain.TestRunEdge, 0, len(rows))
	for _, row := range rows {
		node := toDomain(row)
		edges = append(edges, domain.TestRunEdge{
			Cursor: encodeCursorForRow(row),
			Node:   node,
		})
	}

	pageInfo := domain.PageInfo{HasNextPage: hasMore}
	if hasMore && len(edges) > 0 {
		pageInfo.EndCursor = edges[len(edges)-1].Cursor
	}

	return domain.TestRunPage{
		Edges:                edges,
		PageInfo:             pageInfo,
		TotalCount:           total,
		TotalCountIsEstimate: isEstimate,
		Facets:               domain.TestRunFacets{}, // facets wired by application service via cache
	}, nil
}

// ComputeFacets returns counts grouped by status, branch, project,
// and tag, scoped to the filter (minus the field being faceted, so a
// user can broaden a single dimension without other facets vanishing).
//
// The four queries run concurrently via errgroup — they hit different
// indexes and do not share intermediate state, so they parallelize
// cleanly. On a cold facet cache this drops total time from sum to
// max of the four.
//
// The tag facet is the most expensive (it joins through suite_runs and
// suite_run_tags); IncludeTags controls whether it runs at all. The
// HTTP handler turns this off for the default test-runs list and turns
// it on when the user opens the Tag facet section.
func (r *TestRunQueryRepo) ComputeFacets(ctx context.Context, f domain.TestRunFilter) (domain.TestRunFacets, error) {
	var out domain.TestRunFacets
	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		v, err := r.facet(gctx, f, "status", facetExcludeStatus)
		out.ByStatus = v
		return err
	})
	g.Go(func() error {
		v, err := r.facet(gctx, f, "branch", facetExcludeBranch)
		out.ByBranch = v
		return err
	})
	g.Go(func() error {
		v, err := r.facet(gctx, f, "project_id", facetExcludeProject)
		out.ByProject = v
		return err
	})
	if f.IncludeTagFacet {
		g.Go(func() error {
			v, err := r.tagFacet(gctx, f)
			out.ByTag = v
			return err
		})
	}

	if err := g.Wait(); err != nil {
		return domain.TestRunFacets{}, err
	}
	return out, nil
}

// tagFacet computes tag counts via the suite_run_tags many-to-many.
// The filter's own Tags constraint is excluded so the UI can broaden
// or switch tag selection without all other tags vanishing.
func (r *TestRunQueryRepo) tagFacet(ctx context.Context, f domain.TestRunFilter) ([]domain.FacetCount, error) {
	scoped := f
	scoped.Tags = nil
	scoped.TagMode = ""
	clauses, args := BuildTestRunWhere(scoped)

	q := r.db.WithContext(ctx).Table("test_runs").
		Select("tags.name AS value, COUNT(DISTINCT test_runs.id) AS count").
		Joins("JOIN suite_runs sr ON sr.test_run_id = test_runs.id").
		Joins("JOIN suite_run_tags srt ON srt.suite_run_id = sr.id").
		Joins("JOIN tags ON tags.id = srt.tag_id").
		Group("tags.name").
		Order("count DESC")
	for i, c := range clauses {
		q = q.Where(c, sliceArg(args[i])...)
	}
	type row struct {
		Value string
		Count int64
	}
	var rows []row
	// Scan, not Find: same reasoning as facet() above.
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.FacetCount, 0, len(rows))
	for _, x := range rows {
		if x.Value == "" {
			continue
		}
		out = append(out, domain.FacetCount{Value: x.Value, Count: x.Count})
	}
	return out, nil
}

type facetExclude int

const (
	facetExcludeStatus facetExclude = iota
	facetExcludeBranch
	facetExcludeProject
)

// facet runs a single faceted aggregation, leaving the named filter
// field unconstrained so the user can see all its possible values.
func (r *TestRunQueryRepo) facet(ctx context.Context, f domain.TestRunFilter, col string, exclude facetExclude) ([]domain.FacetCount, error) {
	scoped := excludeField(f, exclude)
	clauses, args := BuildTestRunWhere(scoped)
	q := r.db.WithContext(ctx).Table("test_runs").
		Select(col + " AS value, COUNT(*) AS count").
		Group(col).
		Order("count DESC")
	for i, c := range clauses {
		q = q.Where(c, sliceArg(args[i])...)
	}
	type row struct {
		Value string
		Count int64
	}
	var rows []row
	// Scan, not Find: Find adds the model's soft-delete predicate and
	// tries to infer column names from the embedded BaseModel, which
	// the SELECT alias of "value/count" doesn't match. Scan respects
	// the explicit SELECT exactly.
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.FacetCount, 0, len(rows))
	for _, x := range rows {
		if x.Value == "" {
			continue
		}
		out = append(out, domain.FacetCount{Value: x.Value, Count: x.Count})
	}
	return out, nil
}

// excludeField returns a copy of f with the facet-target field zeroed
// out so the GROUP BY shows all values, not just the selected ones.
func excludeField(f domain.TestRunFilter, e facetExclude) domain.TestRunFilter {
	switch e {
	case facetExcludeStatus:
		f.Status = nil
	case facetExcludeBranch:
		f.Branches = nil
	case facetExcludeProject:
		f.ProjectIDs = nil
	}
	return f
}

// fetchPage runs the filtered, ordered, limited query and returns up
// to page.First+1 rows so the caller can detect hasNextPage.
//
// When p.After is set we add a keyset predicate
//   (start_time, id) < (cursorTs, cursorId)
// which combined with ORDER BY start_time DESC, id DESC gives the
// strictly-older next page, and lets Postgres use the
// (start_time DESC, id DESC) index for both the filter and the sort.
func (r *TestRunQueryRepo) fetchPage(ctx context.Context, f domain.TestRunFilter, p domain.PageArgs) ([]database.TestRun, bool, error) {
	clauses, args := BuildTestRunWhere(f)
	q := r.db.WithContext(ctx).
		Model(&database.TestRun{}).
		Table("test_runs")
	for i, c := range clauses {
		q = q.Where(c, sliceArg(args[i])...)
	}
	if p.After != "" {
		ts, id, err := decodeCursor(p.After)
		if err != nil {
			return nil, false, fmt.Errorf("after: %w", err)
		}
		q = q.Where(
			"(test_runs.start_time, test_runs.id) < (?, ?)",
			ts, id,
		)
	}
	q = q.Order(BuildTestRunOrderBy()).Limit(p.First + 1)

	var rows []database.TestRun
	if err := q.Find(&rows).Error; err != nil {
		return nil, false, err
	}
	hasMore := len(rows) > p.First
	if hasMore {
		rows = rows[:p.First]
	}
	return rows, hasMore, nil
}

// count returns (total, isEstimate, error). Errors here are non-fatal
// at the call site — see Query.
func (r *TestRunQueryRepo) count(ctx context.Context, f domain.TestRunFilter) (int64, bool, error) {
	switch ChooseCountStrategy(f) {
	case CountExact:
		clauses, args := BuildTestRunWhere(f)
		q := r.db.WithContext(ctx).Model(&database.TestRun{}).Table("test_runs")
		for i, c := range clauses {
			q = q.Where(c, sliceArg(args[i])...)
		}
		var n int64
		if err := q.Count(&n).Error; err != nil {
			return 0, false, err
		}
		return n, false, nil
	default: // CountEstimate
		var n int64
		row := r.db.WithContext(ctx).Raw(EstimateCountSQL()).Row()
		scanErr := row.Scan(&n)
		// Postgres returns reltuples = -1 for a freshly-created table
		// that has never been analyzed (i.e. no ANALYZE has run since
		// rows were inserted). In that case the estimate is useless;
		// fall back to an exact count.
		if scanErr != nil || n < 0 {
			clauses, args := BuildTestRunWhere(f)
			q := r.db.WithContext(ctx).Model(&database.TestRun{}).Table("test_runs")
			for i, c := range clauses {
				q = q.Where(c, sliceArg(args[i])...)
			}
			var exact int64
			if cerr := q.Count(&exact).Error; cerr != nil {
				return 0, true, cerr
			}
			return exact, false, nil
		}
		return n, true, nil
	}
}

// sliceArg unwraps a single-element slice we passed as `any` so GORM
// receives the actual slice (for `IN ?`) or scalar (for `=`, `>=`, etc).
// This mirrors GORM's own dispatch rules.
func sliceArg(a any) []any {
	return []any{a}
}

// decodeCursor reverses encodeCursorForRow's "ts=<unixNs>&id=<rowId>"
// form. Stale or malformed cursors return an error so the caller can
// fail the request rather than silently restart pagination from page 1
// (the symptom we previously had: the cursor was discarded entirely
// and "Next" returned the same first page in a loop).
func decodeCursor(s string) (time.Time, uint, error) {
	v, err := url.ParseQuery(s)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("malformed cursor: %w", err)
	}
	tsStr := v.Get("ts")
	idStr := v.Get("id")
	if tsStr == "" || idStr == "" {
		return time.Time{}, 0, fmt.Errorf("malformed cursor: missing ts/id")
	}
	tsNs, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("malformed cursor ts: %w", err)
	}
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("malformed cursor id: %w", err)
	}
	if id > math.MaxUint {
		return time.Time{}, 0, fmt.Errorf("malformed cursor id: out of range")
	}
	return time.Unix(0, tsNs).UTC(), uint(id), nil
}

// encodeCursorForRow produces a deterministic cursor string for a row.
// Cursors are intentionally opaque to the client; the wire format is
// finalized once pkg/cursor's secret is wired into config.
//
// Until then we emit an unsigned debug string. The TestRunQueryService
// in the application layer wraps the repo and replaces this with a
// signed cursor before the response goes out.
func encodeCursorForRow(r database.TestRun) string {
	return fmt.Sprintf("ts=%d&id=%d", r.StartTime.UnixNano(), r.ID)
}

// toDomain projects a database row into the read-side domain shape.
// Suites/specs are intentionally not hydrated — the list view does
// not need them, and lazy-loading detail keeps queries snappy.
func toDomain(r database.TestRun) *domain.TestRun {
	return &domain.TestRun{
		ID:           r.ID,
		RunID:        r.RunID,
		ProjectID:    r.ProjectID,
		Branch:       r.Branch,
		GitBranch:    r.Branch,
		GitCommit:    r.CommitSHA,
		Status:       r.Status,
		StartTime:    r.StartTime,
		EndTime:      r.EndTime,
		TotalTests:   r.TotalTests,
		PassedTests:  r.PassedTests,
		FailedTests:  r.FailedTests,
		SkippedTests: r.SkippedTests,
		// duration_ms is stored, not derived. Carrying it matters for
		// runs with no end_time (still running, or ended abnormally),
		// where the client cannot compute the elapsed time itself.
		Duration:    time.Duration(r.Duration) * time.Millisecond,
		Environment: r.Environment,
	}
}
