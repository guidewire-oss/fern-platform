// Package sql provides shared SQL constants for the testing domain.
package sql

// PassedRunCaseSQL is the canonical CASE expression for determining whether a
// test run counts as "passed" in aggregation queries. Import this constant
// rather than duplicating the expression — divergence would produce different
// pass-rate numbers in different parts of the UI with no compile-time warning.
const PassedRunCaseSQL = `CASE WHEN (status = 'completed' OR status = 'passed') AND failed_tests = 0 AND total_tests > 0 THEN 1 ELSE 0 END`

// PassedRunSumSQL wraps PassedRunCaseSQL in a SUM() aggregate, ready for use
// in a SELECT clause without runtime string concatenation.
const PassedRunSumSQL = "SUM(" + PassedRunCaseSQL + ")"

// DefaultDashboardRecentWindowHours is the default look-back window (in hours)
// used when counting "recent" test runs on the dashboard. Callers should prefer
// passing a configured value over using this default directly.
const DefaultDashboardRecentWindowHours = 24
