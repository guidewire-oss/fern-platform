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
