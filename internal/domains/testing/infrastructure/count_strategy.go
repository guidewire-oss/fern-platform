package infrastructure

import "github.com/guidewire-oss/fern-platform/internal/domains/testing/domain"

// CountStrategy selects how TestRunPage.TotalCount is computed for a
// given filter. Exact counts are honest but linear in result-set size;
// estimates are O(1) and accurate enough for "12,448 ↓" UI affordances
// when the filter spans most of the table.
type CountStrategy int

const (
	// CountExact runs SELECT COUNT(*) with the filter applied.
	CountExact CountStrategy = iota
	// CountEstimate reads pg_class.reltuples for the table and reports
	// it as an approximate total. totalCountIsEstimate=true in the
	// response so clients can render the "≈" affordance.
	CountEstimate
)

// ChooseCountStrategy picks exact vs estimate based on filter narrowness.
//
// The heuristic mirrors domain.TestRunFilter.IsNarrow(): any filter that
// constrains by project/branch/status/tag/commit/author/search is narrow
// enough to count exactly; date-only or duration-only is broad and gets
// an estimate.
func ChooseCountStrategy(f domain.TestRunFilter) CountStrategy {
	if f.IsNarrow() {
		return CountExact
	}
	return CountEstimate
}

// EstimateCountSQL returns the query for an approximate row count of
// the test_runs table. Postgres-specific; safe to call on any engine
// that supports pg_class (i.e. only Postgres in our deployment).
func EstimateCountSQL() string {
	return `SELECT reltuples::bigint FROM pg_class WHERE relname = 'test_runs'`
}
