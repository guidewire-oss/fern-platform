# v2 Time-Range & Sort — Tasks

TDD: comparator + range-mapping helpers get failing tests first, then the
minimum code, then wire the UI. `tsc` + `eslint` + `vitest` per step;
verify end-to-end in the running app before marking a phase done.

## Phase 1 — Runs sort
- [x] T1.1 (test-first) `sortProjects(edges, sortKey, sortDir, runsByProject?)`
      pure comparator: name/runs/rate/last, both dirs, tie→name,
      missing→0. Extract from ProjectsList.
- [x] T1.2 ProjectsList uses the helper; add name tiebreaker.
- [x] T1.3 Summaries: build `runsByProject` from the trends buckets
      (window `totalRuns`); sort by it with stats/name fallback. Lifted
      the trends fetch into `useWindowTrends` so cards + sort share one
      call.
- [ ] T1.4 End-to-end: Runs reorders in both directions on Summaries +
      Projects.

## Phase 2 — 180-day window
- [x] T2.1 Summaries `WINDOWS` += 180.
- [x] T2.2 Test-runs `DATE_PRESETS` += 180d.
- [x] T2.3 Audit TestHistoryChart / manager dashboard. TestHistoryChart is
      run-count based (last N runs, not a time window) → 180d N/A. Manager
      dashboard is a redirect to Summaries → covered by T2.1. No other
      time selectors exist.
- [ ] T2.4 End-to-end: 180d loads on Summaries + Test runs.

## Phase 3 — Custom date range
- [x] T3.1 (test-first) range→query mapping (`from`/`to` RFC3339 for
      test-runs; `days` for summaries-custom).
- [x] T3.2 Test-runs FilterSidebar: from/to date inputs → filter.from/to;
      active-range highlight + clear; preset⇄custom mutually clear.
- [x] T3.3 Summaries: from/to date-range picker (anchored range), mirroring
      the test-runs date filter. Replaced the interim "Custom days" input.
- [ ] T3.4 End-to-end: custom test-runs range filters correctly; custom
      summaries range works.

## Cross-cutting
- [x] C.1 tsc + eslint (0 warnings) + full web-v2 vitest green.
- [x] C.2 No silently-broken controls (omit 180d where unsupported, note it).
- [x] C.3 Extended `/api/v2/test-runs/trends` to accept `from`/`to` (RFC3339,
      both required, ≤366d) for a true anchored range in Summaries; the
      service already took explicit bounds, so only the handler + cache key
      changed. Front-end `useWindowTrends` sends from/to and zero-fills the
      sparklines anchored at the range end.

## Follow-up (saved views — separate from the time-range spec)
- [x] SV.1 Delete is idempotent (404 → success) so a row already deleted
      elsewhere / on a double-click still clears from the list.
- [x] SV.2 Mutations invalidate the root `['saved-views']` key with
      refetchType:'all', so the test-runs "Views" bar and the management
      page both refresh after any create/delete (prefix mismatch previously
      left the other list stale).
- [x] SV.3 Clarified roles: create from Test runs (captures the live
      filter) — Saved views page is manage-only (list + delete) with a
      pointer to Test runs. Delete removed from the Test runs Views bar so
      deletion happens only on the Saved views page.
- [x] SV.4 Test runs Views bar: apply a view (with active-view highlight
      via a canonical filter compare) and a "Clear" action that resets all
      active filters; "Save view" captures the current filter.
- [ ] C.3 Stretch (only if asked): extend `/api/v2/test-runs/trends` to
      accept `from`/`to` for a true anchored custom range in Summaries.
