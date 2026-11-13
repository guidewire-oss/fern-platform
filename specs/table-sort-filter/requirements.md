# Sortable and filterable tables (v2 UI)

## Requirements

1. WHEN a user clicks a sortable column header THE SYSTEM SHALL sort ascending; a second click SHALL sort descending; a third click SHALL restore the table's natural order.
2. THE SYSTEM SHALL mark the sorted column with `aria-sort` and an arrow icon.
3. WHERE a table is server-paginated (global Test runs, project run table) THE SYSTEM SHALL sort on the server across the whole result set, not just the loaded page.
   - `GET /api/v2/test-runs` accepts `sort=started|duration|status|branch|project|run_id|failed` and `order=asc|desc`; unknown values return 400.
   - Keyset pagination SHALL stay correct under any sort (no duplicates or gaps); a cursor issued under one sort SHALL be rejected under another.
   - The default (no `sort`) SHALL keep the existing `start_time DESC` order and cursor format.
4. THE project run table SHALL filter by status (server-side); the history chart SHALL keep showing the unfiltered window.
5. THE run-detail suites and specs tables SHALL support client-side sort, text search, status chips and tag chips; the specs table SHALL offer "Hide skipped".
6. Suite/spec sort and filter state SHALL survive drilling into a suite and back.
7. Users SHALL sort by name/role/status/last login and filter by role and status; Tags SHALL render each category as a sortable table with a category filter; Saved views SHALL sort by name and page.
8. Spec and suite history strip on run detail (level 1):
   - THE run-detail specs table SHALL show, per spec, a strip of that spec's results in the project's last 20 runs on the same branch as the viewed run, ending at the viewed run (not at the latest run), oldest left.
   - Each cell SHALL show the status (passed, failed, skipped, pending) or that the spec was absent from that run; the viewed run's cell SHALL be marked; hovering a cell SHALL show the run's start time, branch and duration.
   - THE suites table SHALL show the same strip per suite, using the suite's status.
   - A spec's identity across runs SHALL be (project, suite name, spec name); a renamed spec starts a new history.
   - THE strip data for all rows of a run SHALL come from one request, not one request per row.
9. Spec history panel on run detail:
   - WHEN a user activates a spec's history strip THE SYSTEM SHALL open a panel under that spec row; activating it again SHALL close it.
   - THE panel SHALL plot the spec's duration in each run of the strip's window as one bar per run, in strip order, colored by status, with absent runs left empty, the viewed run marked, and a median line; hovering a bar SHALL show the run's start time, branch, status and duration.
   - THE panel SHALL list every failed run in the window, newest first, with its start time, branch, a link to that run, and the error message, expandable to the stack trace.
   - Error messages SHALL load only when the panel opens, through a per-spec request, not with the strip.
