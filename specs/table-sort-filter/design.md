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
