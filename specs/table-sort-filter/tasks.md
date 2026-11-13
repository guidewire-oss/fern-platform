# Tasks

- [x] 1. Backend: `SortSpec` on `PageArgs`, `?sort=&order=` parsing, sort-aware ORDER BY + keyset + cursor codec (tests: handler, filter query, repo pagination under sort)
- [x] 2. Shared UI: `lib/tableSort.ts` (cycle + sortRows), `SortableTH`, `FilterChips`, `lib/tableFilter.ts`
- [x] 3. Test runs list: server sort via filter (persisted, saved with views, resets pagination)
- [x] 4. Project detail: server sort + status chips; chart stays unfiltered
- [x] 5. Run detail: suites/specs sort, search, status/tag chips, hide skipped; state lifted to survive drill-in
- [x] 6. Users, Tags, Saved views
- [x] 7. E2E: local backend + Playwright against seeded Postgres (all checks pass)

## Follow-ups (not done)
- Vite dev proxy lacks `/query` (GraphQL path), so run detail fails under `vite` dev; works when served by the backend.
- Filter sidebar status facet omits `completed` (pre-existing).

## Spec/suite history strip (requirement 8, level 1)
- [x] 8. Backend: one query returning, for every suite and spec in a run, its status in the project's last 20 same-branch runs up to the viewed run (tests: window ends at the viewed run, branch filter, absent specs, one query per run)
  - `GET /api/v2/test-runs/:id/history?limit=` (default 20, max 100), project-scoped like the other v2 reads; a run in a project the caller can't read returns 404, same as a missing run.
  - `domain.BuildRunHistory` (pure, unit-tested), `infrastructure.RunHistoryRepo` (four queries per request whatever the run size; SQLite-tested), `v2.RunHistoryHandler`.
- [x] 9. Run detail: history strip column in the specs and suites tables, viewed run marked, hover details (tests: rendering, absent cells, marker)
  - `useRunHistory` (REST, separate from the GraphQL run query, never blocks the page; a dash until it loads or if it fails), `HistoryStrip`, `runHistory.ts` (lookup by suite/spec name, accessible summary).
- [ ] 10. E2E against seeded Postgres

## Spec history panel (requirement 9)
- [x] 11. Backend: `GET /api/v2/test-runs/:id/history/spec?suite=&spec=` — the strip's window plus, per run, status, duration, error message and stack trace for one spec (tests: window shared with the strip, messages only on that spec, unknown spec, scope)
  - `RunHistoryRepo.loadWindow` is shared by both endpoints; `SpecHistory` adds one query. An unknown spec returns every cell absent, not 404. The handler shares id/limit parsing and the 404-on-no-access check between routes.
- [x] 12. Run detail: strip toggles a panel under the spec row with a per-run duration bar chart (status colors, absent gaps, viewed marker, median) and the failed runs' errors (tests: toggle, bars, failures list, loading/error states)
  - `SpecHistoryPanel` (fetches via `useSpecHistory` only while open), `specHistory.ts` (pure bar layout, median, column path, failed runs), `statusColors.ts` (bg/fill maps shared with the strip). Bars follow the dataviz mark specs: 14 wide, 2 gap, 4px rounded top, square base, zero baseline; hover via SVG `<title>` on a full-slot hit target.
