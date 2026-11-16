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
