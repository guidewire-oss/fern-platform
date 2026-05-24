# Spec: Filters, Pagination, Favorites Selector

**Status:** Shipped (items 0–5). Item 6 (URL state for filters) parked.
**Owner:** TBD
**Related:** [requirements.md](./requirements.md), [design.md](./design.md), [PHASES.md](./PHASES.md)

User report:
> "There are no filters in projects and test runs. In the test run page,
> there is next page but we do not know where we are. We have favorite
> bookmark but we do not have an option to select favorite bookmarks."

This spec addresses all three.

## 1. Problem statement

| Surface | Today | Gap |
|---|---|---|
| `/v2/projects` | Read-only card grid. No search, no filter, no sort. | 100s of seeded projects → impossible to scan. |
| `/v2/test-runs` filter sidebar | Status / branch / project facets work. Tag facet computed but not rendered. No date range, no duration. | Common slice ("failed runs on `main` in the last 24h, smoke tag") needs 3 of these. |
| `/v2/test-runs` pagination | Single `Next page →` button. No page indicator, no Previous, no jump-to-page. | User loses position; back-navigation impossible. |
| Favorites | Star button works, persists. No way to filter by them. | Saves are useless without a recall path. |

## 2. Requirements (EARS)

### Projects page

- **FR-FP-1** The system **shall** show a search field at the top of `/v2/projects` that filters by project name OR project ID OR team, case-insensitive, debounced 250 ms.
- **FR-FP-2** The system **shall** show a team multi-select that filters the card grid to projects whose `team` field matches any selected value.
- **FR-FP-3** The system **shall** show a category multi-select (java, infra, flux, helm, web) that filters by the project-ID prefix.
- **FR-FP-4** The system **shall** show a "Favorites only" toggle that, when active, hides non-favorited projects.
- **FR-FP-5** The system **shall** show a sort selector with options: Name / Runs / Pass rate / Last activity (and direction toggle).
- **FR-FP-6** Filter state **shall** be reflected in the URL query string so it survives reload and is shareable.

### Test runs filter

- **FR-FR-1** The filter sidebar **shall** render the byTag facet that the API already returns. Tag pills work like the existing project/branch facets (multi-select, with counts).
- **FR-FR-2** The system **shall** show a date-range control with presets (24h, 7d, 30d, custom) that maps to the `from` / `to` REST params.
- **FR-FR-3** The system **shall** show a duration filter (slider or numeric inputs) mapped to `durationGte` / `durationLte`.
- **FR-FR-4** The system **shall** show a "Favorites only" toggle that, when active, sets `project=<favorite projectId>` for each favorited project.
- **FR-FR-5** The active filter state **shall** be reflected in the URL query string, parallel to the REST params the API expects.

### Test runs pagination

- **FR-PG-1** The system **shall** display "Showing X – Y of N" where N is `totalCount` (or `≈ N` when `totalCountIsEstimate` is true).
- **FR-PG-2** The system **shall** display "Page A of B" where B is `ceil(totalCount / first)`. When the count is estimated, B is suffixed with `~`.
- **FR-PG-3** The system **shall** provide a `Previous` button that returns to the previous page (via a cursor history stack).
- **FR-PG-4** The system **shall** provide a `First` button that resets the cursor to the start.
- **FR-PG-5** The system **shall** provide a page-size selector with values 25 / 50 / 100 / 200, clamped server-side.
- **FR-PG-6** Changing any filter or page size **shall** reset the cursor history and return to page 1.

### Non-functional

- **NFR-1** Filter changes **shall** keep the previous result rendered (via `placeholderData: keepPreviousData`) so the table doesn't blank during refetch.
- **NFR-2** All filter inputs **shall** be keyboard-accessible and labelled per WCAG 2.1 AA.
- **NFR-3** URL query parameters **shall** use stable, short names so links don't break across releases.

## 3. Out of scope

- Server-side sort on `/v2/test-runs` — current API doesn't support arbitrary order; default `start_time DESC` stays. Sort selector applies only to projects (client-side).
- Saving a filter combination as a "saved view" — already shipped at `/v2/saved-views`.
- Full free-text search on projects via GraphQL filter — current `projects(filter: { search })` works; client-side narrowing is enough for the seeded volume.

## 4. Design

### Projects filter bar

```
+-------------------------------------------------------------------+
|  🔍 Search projects…           Team ▾  Category ▾  ⭐ Favorites  |
|                                                  Sort: Name ↑     |
+-------------------------------------------------------------------+
| (card grid below)                                                 |
+-------------------------------------------------------------------+
```

All filtering happens client-side against the already-fetched projects list (we fetch 100 by default). For 1000+ projects we'd push to server-side, but that's day-2 work — the seed config does not produce more than a few hundred projects in typical use.

URL params:
- `?q=<search>` — free-text
- `?team=a&team=b` — repeatable
- `?cat=java&cat=infra` — repeatable
- `?fav=1` — favorites-only flag
- `?sort=runs:desc` — sort field + direction (single field)

### Test runs sidebar (additions)

```
+----------------------------+
| Filters             [Reset]|
+----------------------------+
| Search [           ]       |
|                            |
| Status                     |
|   ☐ passed         (1551)  |
|   ☑ failed          (488)  |
|   ☐ flaky           (261)  |
|                            |
| Date range                 |
|   ( ) Last 24h             |
|   ( ) Last 7d              |
|   (•) Last 30d             |
|   ( ) Custom… [from][to]   |
|                            |
| Duration (ms)              |
|   ≥ [   ]   ≤ [   ]        |
|                            |
| ⭐ Favorites only [   ]    |
|                            |
| Branch                     |
|   ☑ main           (1700)  |
|   ☐ develop         (340)  |
|                            |
| Project                    |
|   …                        |
|                            |
| Tags                       |
|   ☑ smoke           (412)  |
|   ☐ regression      (302)  |
|   …                        |
+----------------------------+
```

### Pagination bar (test runs)

```
+----------------------------------------------------------+
|  Showing 51–100 of 12,448      Page size [50 ▾]          |
|                                                          |
|  [⏮ First]  [◀ Previous]   Page 2 of 249   [Next ▶]      |
+----------------------------------------------------------+
```

Implementation note: the v2 API is cursor-paginated (forward-only). To support "Previous", the UI keeps an in-memory stack of previously-used `after` cursors. Pushing a new cursor when clicking Next; popping when clicking Previous. Resetting the stack on any filter change.

`Page A of B` is computed locally as `floor(seenCount / first) + 1` and `ceil(totalCount / first)`. When `totalCountIsEstimate=true`, the denominator is rendered with a `~` suffix.

## 5. Tasks

1. **Projects filter bar** — `ProjectsList.tsx` gains a `<ProjectsFilterBar>` component with search + team + category + favorites + sort. Filters apply client-side to the existing query result.
2. **URL state for projects** — read/write search/team/cat/fav/sort from the query string via `useSearch` / `useNavigate` from TanStack Router.
3. **Test runs sidebar — render byTag** — render the existing `facets.byTag` data the same way as the other facets.
4. **Test runs sidebar — date range** — preset buttons + custom inputs; map to `from`/`to` ISO timestamps.
5. **Test runs sidebar — duration** — two numeric inputs; map to `durationGte` / `durationLte`.
6. **Test runs sidebar — favorites toggle** — when active, set `filter.project` to the user's favorites array.
7. **Pagination component** — `<Pagination>` showing Showing / Page X of Y / page-size / First / Prev / Next. Cursor history kept in component state.
8. **URL state for test runs** — same pattern as projects.
9. **Verify against seeded data** — at least filtered+paginated query returns correct totals.

## 6. Acceptance

- [x] Pagination shows current page (`X-Y of N · Page A of B`) and Previous works via client-side cursor history stack.
- [x] Changing any filter resets the cursor stack to page 1.
- [x] All inputs are keyboard-reachable; native `<details>` dropdowns close on outside-click and Escape.
- [ ] `/v2/projects?q=java&fav=1` URL-driven filters — covered only by parked item 6 (URL state).
- [ ] `/v2/test-runs?status=failed&tag=smoke&from=…` URL-driven filters — same; UI works but URL is not the source of truth yet.

## 7. Implementation order

0. **Fix the input contrast bug.** ✅ `web-v2/src/components/ui/Input.tsx` — shared `Input` / `Textarea` / `Select` using `bg-surface text-foreground`; global `input { color: inherit; }` in `globals.css` so legacy `bg-white` inputs no longer collapse to white-on-white in dark mode.
1. **Saved views integration.** ✅ `web-v2/src/features/saved-views/integration.ts` exposes `useTestRunsSavedViews`, `useCreateSavedView`, `useDeleteSavedView`. `TestRunsList.tsx` renders a `SavedViewsBar` that captures the live filter on Save and applies it on Load. Required two backend fixes: saved-view rows use `time.Time` (was int64 — pgx couldn't encode timestamptz), and `main.go` upserts the synthetic `dev-admin` user at startup when auth is disabled, so the `saved_views.user_id` FK is satisfied in DevAuth mode.
2. **Pagination on test runs.** ✅ `Pagination.tsx` shows "X-Y of N" (`≈` for estimates), "Page A of B", First / Previous / Next, page-size selector (25 / 50 / 100 / 200). `TestRunsList.tsx` holds a `cursorStackRef` for back-navigation and resets it on any filter change. Required a server-side fix: `fetchPage` was ignoring `p.After` entirely — it now decodes the `ts=&id=` cursor and applies `(start_time, id) < (?, ?)` so "Next" actually advances.
3. **Test-runs sidebar additions.** ✅ Date-range presets (24h / 7d / 30d) → ISO `from`/`to`; duration inputs → `durationGte` / `durationLte` (UI labels them "Run time" in seconds, converted to ms before the request); favorites-only toggle that sets `project` to the user's starred projects; tag facet renders behind a "Load tag facet" button (opt-in via `?facets=tag`, since the suite-runs join is expensive at scale).
4. **Projects filter bar.** ✅ `ProjectsFilterBar.tsx` — search across name/projectId/team, team multiselect, category multiselect (java / infra / flux / helm / web by project-ID prefix), favorites-only toggle, sort by name / runs / pass rate / last activity with asc/desc, active filter chips with per-chip clear and "Clear all".
5. **Test summaries — same filter affordances.** ✅ `TestSummaries.tsx` reuses `ProjectsFilterBar` directly so the per-project trend cards scope to the same filter shape as the Projects page.
6. **URL state for all surfaces.** Parked. Saved views cover the "remember a preset" case; URL state is the "share a link to this exact view" refinement. See PHASES.md parked item #1 for resumption.
