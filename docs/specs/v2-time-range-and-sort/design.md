# v2 Time-Range & Sort — Design

## Build reconnaissance

- **Sort control** lives in `web-v2/src/features/projects/ProjectsFilterBar.tsx`
  — `SortKey = 'name' | 'runs' | 'rate' | 'last'`, plus a direction toggle.
  Both `ProjectsList` and `TestSummaries` reuse this bar + `ProjectsFilter`.
- **ProjectsList** sorts by `node.stats.{totalTestRuns,successRate,lastRunTime}`.
- **TestSummaries** now applies the same sort (commit `dc7782e`) but off
  the same all-time `stats` — so **Runs** orders by `stats.totalTestRuns`.
  Root-cause hypothesis for "no change": the seeded/visible projects have
  near-identical `totalTestRuns` (e.g. 450 each), so the order barely
  moves; and it doesn't match the per-window run numbers the cards show.
  The card numbers come from the trends query
  (`sumWindowStats(buckets[projectId])`, field `totalRuns`).
- **Summaries window**: `WINDOWS = [7,30,90]` in `TestSummaries.tsx`,
  drives `days` on `GET /api/v2/test-runs/trends` (server validates
  `days` 1..365).
- **Test-runs dates**: `FilterSidebar.tsx` `DATE_PRESETS = [24h,7d,30d,90d]`
  set `from` on the filter; `GET /api/v2/test-runs` accepts `from`/`to`
  (RFC3339) and, absent them, defaults to the last 7 days.

## FR-1 — Runs sort

- **Projects grid:** keep `stats.totalTestRuns`; add a **name tiebreaker**
  so equal counts still land in a stable, sensible order (this alone makes
  the control feel responsive when counts tie).
- **Summaries:** sort by the **window run totals** the cards display, not
  all-time stats. The trends response is already fetched by
  `TrendCardsGrid`; lift a `projectId → totalRuns(window)` map up (or
  compute in the parent) so `filteredEdges` can sort by it. When trends
  haven't loaded yet, fall back to `stats.totalTestRuns`, then name.
- Extract the comparator into a pure, unit-tested helper
  `sortProjects(edges, sortKey, sortDir, runsByProject?)`.

## FR-2 — 180-day window

- `TestSummaries` `WINDOWS`: add `{ days: 180, label: '180 days' }`
  (optionally `365`). No API change (trends allows ≤365).
- `FilterSidebar` `DATE_PRESETS`: add `{ label: '180d', days: 180 }`.
- Audit `TestHistoryChart` / manager dashboard for fixed windows; add 180d
  where the API supports it, else leave and note.

## FR-3 — Custom date range

- **Test runs (full support):** add a compact custom-range control to
  `FilterSidebar` — two date inputs → set `filter.from`/`filter.to` as
  RFC3339 (start-of-day `from`, end-of-day `to`). `buildQuery` already
  forwards `from`/`to`. Show the active custom range as a clearable chip;
  selecting a preset clears the custom range and vice-versa.
- **Summaries (API is days-only):** ship option (a) — a "Custom…" entry
  that opens a small number-of-days input (1..365) mapped to `days`. This
  needs no backend change. (Deferred stretch: extend the trends handler to
  accept `from`/`to` for a true anchored range — out of scope unless asked.)

## Testing
- Pure `sortProjects` comparator: name/runs/rate/last, both directions,
  tie→name fallback, missing-stat→0. Vitest.
- Range mapping: preset/custom → query params (`days`, or `from`/`to`).
- Light render checks that the 180d option and custom control appear and
  drive the query.
- End-to-end: verify Runs reorders, 180d loads, and a custom test-runs
  range filters, in the running app.

## Risks
- Sorting Summaries by window runs couples the sort to async trends data
  (order settles once trends load); acceptable with the stats/name
  fallback, but note the one-frame reflow.
