# v2 Time-Range & Sort — Requirements

## Context

Three related gaps in the v2 SPA, reported while exercising Summaries and
Test runs:

1. **Sort by "Runs" appears to do nothing.** In Summaries (and the
   Projects grid), toggling the sort direction on Name / Pass rate / Last
   activity reorders the list, but choosing **Runs** produces no visible
   change.
2. **Time windows top out at 90 days.** Summaries offers 7 / 30 / 90-day
   windows; Test runs offers 24h / 7d / 30d / 90d date presets. Users
   want a **180-day** option (and it should be added everywhere a
   fixed-window picker exists).
3. **No custom date range.** There's no way to pick an arbitrary
   from/to range; only fixed presets.

## Requirements (EARS)

### FR-1 — Sort by Runs works
- WHEN a user sorts by **Runs** and toggles direction, the list SHALL
  reorder by the run count, ascending/descending.
- The sort SHALL key off a value that actually varies per project and
  matches what the surface displays:
  - Summaries: the **selected window's** total runs (the number the card
    reflects), so the order matches the visible data.
  - Projects grid: the project's run-count stat (`stats.totalTestRuns`).
- IF the sort value is missing, it SHALL be treated as 0 (stable, not a
  crash), and ties SHALL fall back to name for a deterministic order.

### FR-2 — 180-day window everywhere applicable
- The Summaries window picker SHALL include **180 days** (the trends API
  already accepts `days` 1..365).
- The Test-runs date presets SHALL include **180d**.
- Any other fixed-window picker that maps to a supported API window SHALL
  gain 180d (audit: project history chart, manager dashboard).
- WHERE a surface's backing API cannot express 180 days, the option is
  omitted and noted (no silently-broken control).

### FR-3 — Custom date range
- WHERE the backing API accepts an explicit range (`/api/v2/test-runs`
  supports `from`/`to`), the UI SHALL offer a **custom from/to** picker
  in addition to the presets.
- WHEN a custom range is set, the list SHALL query that exact range and
  the active range SHALL be visible/clearable.
- Summaries trends currently accept only `days` (relative to now). Custom
  range there SHALL either (a) offer a "custom N days" entry, or (b) be
  enabled by extending the trends endpoint to accept `from`/`to` — the
  design picks one; option (a) requires no backend change.

## Non-goals
- No change to how coverage releases are fetched (that window is a fixed
  52-week epic scope, not user-facing).
- No new charting; only the range/sort controls and the queries they drive.

## Acceptance
- Sorting by Runs visibly reorders both Summaries and Projects, in both
  directions, against real data.
- 180-day option present and functional on Summaries + Test runs.
- A custom from/to range works on Test runs end-to-end (verified in the
  running app), and the chosen Summaries approach works there too.
- tsc + eslint clean; unit tests for the sort comparator and any range→
  query mapping; existing suites green.
