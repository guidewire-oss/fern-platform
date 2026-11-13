# Design notes

## Decisions

### Filter UI: in-table chips vs. the Test runs `FilterSidebar`

The v2 UI now has two filter patterns:

| Screen | Filter UI | What it filters |
|---|---|---|
| Test runs (global list) | `FilterSidebar` | Runs across projects, server-side and faceted with counts: status, branch, tag, project, date range, favorites |
| Project detail, run history table | `FilterChips` bar above the table | Status only, server-side |
| Run detail, suites and specs tables | `RowFilterBar` (search, status and tag chips, "Hide skipped") | Rows of one run, client-side |

**Run detail uses chips, on purpose.** Each of the two tables has its own filter state, and
a bar on top of each table makes clear which one it applies to; one sidebar would not. The
options come from the run's own rows, not from server facets, and most sidebar facets
(project, branch, date, favorites) describe runs, not suites or specs. A sidebar would also
take width from the spec name and error columns.

**Project run history uses chips for now.** Same data type as the global list, so the
sidebar is a reasonable expectation there. Chips are fine while status is the only filter,
and a bar on the table keeps it clear that the history chart above stays unfiltered
(requirement 4).

**When to revisit:** if the project run table gains more filters (branch, tag, date), reuse
`FilterSidebar` with the project facet hidden instead of extending the chip bar, so the same
data doesn't end up with two filter systems. Keep the chart visibly outside the filter's
scope.

### Spec and suite history across runs

Run detail shows results vertically (every spec in one run). Requirement 8 adds the horizontal
view (one spec across runs), in two levels:

1. **Level 1 (this branch): a history strip** per row in the run-detail specs and suites tables.
2. **Level 2 (later): a spec history page** reached from the strip: summary (pass rate, flips,
   median duration, last failure), a scrollable timeline filterable by branch and environment,
   and a table of runs linking back to each run's detail.

Decisions (user, 2026-11-13):

- **Level 1 first**; level 2 is a later change.
- **20 runs** in the strip to start with.
- **Same branch as the viewed run.** Stated for the level 2 page; the strip uses the same rule
  so both views agree.
- **The window ends at the viewed run**, so an old run shows the 20 runs leading up to it.
- **Identity is (project, suite name, spec name).** Ginkgo's full text includes the container
  descriptions, so it is unique within a suite; a rename starts a new history.

The flaky detector's `GetTestRunHistory` (`analytics/infrastructure/gorm_flaky_repository.go`)
queries one spec at a time and filters on `sr.name`; the strip needs all specs of a run in one
query, so it gets its own repository method.

### Spec history panel (requirement 9)

Clicking a spec's strip opens a panel under the row, the same pattern as expanding an error to
its stack trace. It shows a duration bar per run and the errors of the failed runs.

Decisions (user, 2026-11-13):

- **Per-run bars, not a histogram.** Bars line up with the strip and show trends over time,
  which a distribution histogram would hide.
- **Specs only to start.** A suite row already drills into its specs on click, so a suite
  panel would need its own control.
- **Panel first, page later if needed.** This may cover the "level 2" spec history page; decide
  after trying it.

Error messages and stack traces load on demand from a per-spec endpoint. Adding them to the
strip's response would ship error text for every spec on every page load, most of it never
viewed. The per-spec endpoint reuses the strip's window query, so both always cover the same runs.
