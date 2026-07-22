# Spec: Frontend Modernization — Tasks

**Status:** Phases 0-3 substantially shipped; Phase 4 (cutover) pending
**Related:** [requirements.md](./requirements.md), [design.md](./design.md), [PHASES.md](./PHASES.md), [feature-filters-pagination-favorites.md](./feature-filters-pagination-favorites.md)

> **Reconciliation (2026-05-17).** This task list was written before
> implementation began. The single source of truth for "what is
> shipped" is now [`PHASES.md`](./PHASES.md); use that for stand-up
> reporting. This file is preserved as the original TDD plan with
> per-task status markers below. Where the implementation deviated
> from the original task (e.g., trigram search replacing FTS, single
> consolidated migration replacing three, the new
> filters/pagination/favorites work that postdates this file), the
> deviation is captured in RFC-004 Appendix B and in the linked
> feature spec.

### Status snapshot (2026-05-17)

| Task block                                              | Status | Notes                                                                                              |
| ------------------------------------------------------- | ------ | -------------------------------------------------------------------------------------------------- |
| T0.1 – T0.6 (Phase 0, foundation)                       | ✅      | Scaffold, embed, deprecation middleware, codegen all shipped                                       |
| T1.1 – T1.3 (Phase 1, shared layer)                     | ✅      | Tokens, layout chrome, ported utilities                                                            |
| T1.4 (`useMe` + `<AuthGate>`)                           | ⏸      | DevAuth no-op covers local; gate needed before any auth-required rollout                          |
| T2.1 (filter indexes + `saved_views`)                   | ✅      | **Consolidated into `000022_v2_schema`** with T2.3's index changes. See RFC-004 Appendix B.5.      |
| T2.2 (domain `TestRunFilter`, `PageArgs`, `TestRunPage`) | ✅      | Includes `IncludeTagFacet` opt-in (RFC-004 Appendix B.3)                                          |
| T2.3 (filtered/paginated/faceted repo)                  | ✅      | Plus `AggregateProjectsInRange` / `AggregateSuitesInRange` for treemap (RFC-004 Appendix B.4)     |
| T2.4 (cursor codec)                                     | ✅      | Includes the `fetchPage` cursor-decoder fix (was silently dropping `p.After`)                      |
| T2.5 (count strategy)                                   | ✅      |                                                                                                    |
| T2.6 (facet cache)                                      | ✅      | In-memory; `RedisLike` adapter ready. **errgroup-parallelized**; tag facet opt-in.                |
| T2.7 (GraphQL schema additions)                         | ⏸      | REST surface covers all current UI needs; revisit when an external GraphQL consumer asks           |
| T2.8 (`GET /api/v2/test-runs`)                          | ✅      | + `?allTime=1`, `?facets=tag`, default 30-day window                                              |
| T2.9 (`<FilterBar>` + `useUrlFilter`)                   | 🟡      | FilterBar surface shipped (per-page filter bars on Test Runs, Projects, Summaries). `useUrlFilter` parked (parked item #1) |
| T2.10 (saved views API + `<SavedViewMenu>`)             | ✅      | Domain, REST CRUD, `SavedViewsBar` on Test Runs                                                    |
| T3.1 (Projects page)                                    | ✅      | Card grid + filter bar (search, team, category, favorites, sort)                                  |
| T3.2 (Project detail + settings)                        | ✅      | 4-tab Settings page (General / Integrations / Team / Notifications). Team + Notifications tabs are UI-only; see PHASES.md parked items 13-14. |
| T3.3 (Test Runs list)                                   | ✅      | Filter sidebar, saved views, pagination with history stack. **v1-parity columns (FR-15b)** — Project, Run ID, Branch, Test Results (P/F/S triple), Status, Duration, Started. |
| T3.4 (Test Run detail)                                  | ✅      | **v1-parity drill (FR-17)** — table-based two-view drill (suites table → specs table) with back-link; run header always visible with tags + metadata panel. Replaces the original virtualized `react-arborist` tree (RFC-004 Appendix B.11). Stack traces, retry counts, flaky markers inline on spec rows. |
| T3.5 (Tag management)                                   | ⏸      |                                                                                                    |
| T3.6 (Flaky dashboard)                                  | ⏸      |                                                                                                    |
| T3.7 (Treemap — standalone /v2/treemap page)            | ❌      | **Superseded by T3.7d.** The standalone route was removed; the same `TreemapView` component is now embedded inside the Summaries page's Tree view as the only entry point. |
| T3.7d (Treemap project → suite → spec drill, in Summaries) | ✅   | FR-16 extended — three drill levels backed by new `AggregateSpecsForSuiteInRange` repo method (capped at 500 rows ORDER BY duration DESC). GraphQL `treemapData(projectId, suiteName, days)`. Standalone `/v2/treemap` route and sidebar nav entry removed. |
| T4.* (Phase 4 cutover)                                  | ⏸      | Awaiting GA decision                                                                               |
| T5.* (Phase 5 removal)                                  | ⏸      | Post-cutover                                                                                       |
| TX.1 – TX.2 (Playwright + axe)                          | ⏸      | Smoke tests via `make docker-test-curl` cover the basics                                          |
| TX.3 (bundle-size CI)                                   | ✅      | `size-limit` configured                                                                            |
| TX.4 (perf budget CI)                                   | ✅      | Lighthouse CI + k6 scenarios                                                                       |
| TX.5 (CSP header)                                       | ✅      |                                                                                                    |
| TX.6 (Web Vitals telemetry)                             | ✅      | `POST /api/v2/telemetry/vitals`                                                                    |
| TBC.1 (v1 golden contracts)                             | ⏸      |                                                                                                    |
| TBC.2 (schema-diff gate)                                | ✅      | Inline `diff` step inside the `web-v2` job on `ci.yml`; fails the build if `internal/reporter/graphql/schema.graphql` and `web-v2/src/gql/schema.graphql` diverge. |
| TBC.3 (config compatibility test)                       | ⏸      |                                                                                                    |
| TBC.4 (migration safety lints)                          | ⏸      |                                                                                                    |
| TBC.5 (`migration-guide.md`)                            | ✅      | Updated 2026-05-17 with cost table + recovery procedure                                            |
| TBC.6 (`fern migrate check` CLI)                        | ⏸      |                                                                                                    |
| TBC.7 (dual-run banner + telemetry)                     | ⏸      | Awaiting Phase 4                                                                                   |
| TBC.8 (redirect map)                                    | ⏸      | Awaiting Phase 4                                                                                   |
| T3.7a (Test Summaries v1 parity)                        | ✅      | FR-17a/b/c/d — Total/Passed/Failed counters, View-history link per card, Card/Tree view toggle on `/v2/test-summaries`. Reuses `/test-runs/trends` + `/treemap`; view-mode persisted via `localStorage` (key `fern-v2.summaries.viewMode`) pending a server-pref roundtrip. |
| T3.7b (Project history stacked-area chart)              | ✅      | FR-17e — v1's `TestHistoryChart` ported to `/v2/projects/:id`. Stacked passed/failed/skipped over the last 20 runs, total-tests reference line, hover tooltips. Reuses the page's existing `/api/v2/test-runs?first=20` payload. |
| T3.1b (Projects pagination — Projects/Summaries/Dashboard) | ✅   | FR-15a — `useProjects` now uses `useInfiniteQuery` and auto-pages through `projects(first, after)` up to 500 (matches FR-16 treemap cap). Eliminates the silent `first: 100` cap that hid projects 101+ on Projects/Summaries/Dashboard while Treemap still showed them. Truncation banner on Projects when cap is hit. Consolidates three duplicated GraphQL queries into one hook. |
| T3.7c (Manager dashboard route + role-gated nav)        | ✅      | FR-17f/g/h — `/v2/manager-dashboard` restored as a thin redirect to `/v2/summaries?view=tree&favoritesOnly=true`, role-gated to manager / admin. Summaries honors `view` + `favoritesOnly` URL params on first mount. Sidebar filters items by role: Manager dashboard shows for manager/admin; Users + Admin overview hidden from non-admins. No new view component — pure routing + preset. |
| T1.4a (TopBar wiring + Sign in/out)                     | ✅      | FR-24a/b/e — `useCurrentUser` hook, real name/avatar/initials in the TopBar pill, role badge ([[admin]]/manager/user color treatment), and v1-style dropdown menu (toggle, outside-click + Escape to close) housing Profile / Admin Panel / Sign-out. Sign-out mirrors v1's `POST /auth/logout` → follow `logout_url`. Sign-in button when `currentUser` is null. Synthetic `dev-admin` keeps the "Local dev" banner and hides Sign-out. Does **not** force a redirect on 401 — that's still T1.4 (`<AuthGate>`, parked). |
| T3.2a (Profile page schema + timezone fixes)            | ✅      | FR-24c/d — dropped the `emailVerified` selection (no such field on `User`), surfaced `groups`, and replaced the 80-zone slice on the timezone picker with the full IANA list (browser zone first, UTC second, rest alphabetical). |

**New work added since this spec (not in the original list):**

- **Bulk perf seeder** (`cmd/fern-platform-seed/perf`) — 1000 projects × 6 months × 100 runs/day across Java / infra / FluxCD / Helm / Node.js categories, with realistic spec names and error messages.
- **Gradient seed mode** (`SEED_HEALTH_BANDS=true`, `make docker-test-seed-gradient`) — 5 evenly-distributed pass-rate bands (≈10/30/60/85/98%) so the treemap and summary-card color scales render across the full red→green range. The default seeder clusters every project at ~75% pass and never exercises the lower half of the gradient. Suite-level `failed_specs` uses the same band-driven cap as run-level `failed_tests`, so the treemap shows consistent colors when drilling from project → suite (previously a red project tile drilled into green suite tiles).
- **Extended seed coverage** — the `seed` binary now populates users, user_groups, user_preferences, user_scopes, project_permissions, jira_connections, flaky_tests, and saved_views in addition to test_runs/suite_runs/spec_runs/tags. Sample fleet of 8 users across admin/manager/user roles with realistic group memberships; flaky records anchored inside the seed time window; ~30% of projects get a Jira connection. Single source of truth for end-to-end demo data.
- **Spec-run mix realism** — `spec_runs` now seeds both **passed** and **failed** rows per suite_run, proportional to the suite's `failed_specs / total_specs` ratio (capped at `SEED_SPECS_PER_FAILED_SUITE` rows per suite_run). The legacy behavior — failures-only — caused the project tile to read 57% while every spec inside drilled to 0% pass and rendered red. The new mix keeps the three drill levels (project / suite / spec) telling a consistent color story. Volume impact: ~3× more `spec_runs` rows at default settings.
- **Substring search** — `pg_trgm` GIN index + `ILIKE` predicate replacing the proposed English FTS path (RFC-004 Appendix B.1).
- **Treemap perf rewrite** — SQL aggregates + 60s in-memory cache (RFC-004 Appendix B.4).
- **DevAuth user upsert** — `main.go` ensures `dev-admin` exists in `users` so `saved_views.user_id` FK is satisfied when auth is off.
- **Per-project Settings page** with 4 tabs and routing under `/v2/projects/:id/settings`. Team and Notifications tabs are UI-only today; parked items 13-14 in PHASES.md cover the backend.
- **Pre-flight index script** — `scripts/v2-preflight-indexes.sh` for `CREATE INDEX CONCURRENTLY` on large prod DBs.

---

**Per-task status legend (used in the table above):**
- ✅ shipped
- 🟡 partially shipped
- ⏸ parked
- ❌ superseded (see notes)

Tasks are grouped by phase. Each task is sized to fit in a single PR. Every
implementation task carries a TDD note (R = red/test, G = green/impl,
F = refactor). Acceptance criteria mirror the requirement IDs.

Execution order **shall** follow phase order. Within a phase, tasks may run
in parallel where dependencies allow (noted).

---

## Phase 0 — Foundation

Goal: scaffold the new pipeline behind `/v2/`, ship a "hello world"
embedded in the Go binary. **No existing functionality changes.**

### T0.1 — Move v1 handlers under `internal/api/v1/`
**Satisfies:** FR-19, FR-22
**Steps:**
1. R: Add a contract test that asserts all v1 route paths and response
   shapes via golden files.
2. G: Move existing `internal/api/*.go` → `internal/api/v1/`. Update imports
   in `main.go`. No behavior change.
3. F: Verify golden contract tests still pass byte-for-byte.

**Acceptance:** existing acceptance suite for v1 passes unmodified.

---

### T0.2 — Add deprecation middleware (no-op until v2 ships)
**Satisfies:** FR-20
**Steps:**
1. R: Test that asserts headers are absent when `FERN_V2_UI_ENABLED=false`
   and present when true.
2. G: Implement `deprecationHeaders(sunset, link)` middleware in
   `pkg/middleware/`.
3. F: Wire to v1 group, gated on env flag.

**Acceptance:** `curl /api/v1/health` returns `Deprecation` and `Sunset`
headers when flag is on.

---

### T0.3 — Scaffold `web/` project
**Satisfies:** FR-1, FR-5, FR-7, FR-8
**Steps:**
1. `pnpm create vite web -- --template react-ts`
2. Add Tailwind, eslint, prettier, vitest, playwright, size-limit configs.
3. Configure `tsconfig.json` with `strict: true`, path alias `@/*`.
4. Add `web/index.html` shell, `web/src/main.tsx`, blank `App.tsx`.
5. Add `pnpm` scripts: `dev`, `build`, `typecheck`, `lint`, `test`,
   `codegen`, `playwright`.

**Acceptance:** `pnpm build` produces `web/dist/index.html` and hashed
assets; `pnpm typecheck` and `pnpm lint` pass with zero output.

---

### T0.4 — Embed `web/dist` in Go binary, mount at `/v2/*`
**Satisfies:** FR-2, FR-3
**Steps:**
1. R: Test that `GET /v2/anything` returns the embedded `index.html` and
   `GET /api/...` is not intercepted.
2. G: Implement `internal/web/embed.go` per design §8.
3. F: Wire into `main.go` behind `FERN_V2_UI_ENABLED`.

**Acceptance:** built binary serves the new SPA at `/v2/`; legacy `/` is
unchanged; API and `/graphql` routes are unaffected.

---

### T0.5 — Make targets + CI integration
**Satisfies:** FR-4, NFR-3, NFR-9
**Steps:**
1. Add Makefile targets per design §12.1.
2. Add CI job (`.github/workflows/web.yml`) per design §12.2.
3. Add `size-limit` config with budgets from requirements §4 / design §11.
4. Block PR merge on typecheck, lint, unit, build, size-limit.

**Acceptance:** PR template runs the new pipeline; first PR's status checks
include `web-typecheck`, `web-lint`, `web-test`, `web-build`, `size-limit`.

---

### T0.6 — Sync GraphQL schema + codegen setup
**Satisfies:** FR-6
**Steps:**
1. Add `make sync-schema` (copies server schema to `web/src/gql/schema.graphql`).
2. Add `codegen.ts` per design §4 of RFC and design §3.
3. Add a single placeholder `web/src/gql/operations/health.graphql`.
4. Commit `web/src/gql/generated.ts`; CI re-runs `pnpm codegen` and fails on
   diff.

**Acceptance:** `useHealthQuery()` hook is typed and compiles; CI fails if
generated file is out of date.

**Phase 0 exit criterion:** new SPA reachable at `/v2/hello` showing
"Hello", CI green end-to-end, legacy UI unaffected.

---

## Phase 1 — Shared layer

Goal: design system, shared lib, and auth gate in place. No business
features yet.

### T1.1 — Design tokens + Tailwind config
**Satisfies:** NFR-5
**Steps:**
1. Define color, spacing, typography tokens as CSS variables on `:root`
   (light only for now; dark-ready).
2. Configure Tailwind theme to consume them.

**Acceptance:** Storybook (or a `/v2/_styleguide` route) shows the palette.

---

### T1.2 — Layout chrome: `<AppShell>`, `<Sidebar>`, `<TopBar>`
**Satisfies:** U7 (parity)
**Steps:**
1. R: Visual snapshot test (Playwright) against the layout.
2. G: Implement using shadcn primitives.
3. F: Match legacy navigation structure (do not redesign).

---

### T1.3 — Port `lib/` utilities to TypeScript
**Satisfies:** NFR-1
**Steps:**
1. R: Port `duration-utils-test.js` to `web/src/lib/duration.test.ts` using
   Vitest; add edge cases.
2. G: Port `duration-utils.js` to `duration.ts` typed.
3. R: Same for timestamp formatting.
4. G: Same.

**Acceptance:** Vitest coverage ≥ 95 % on `lib/`.

---

### T1.4 — `useMe` query + `<AuthGate>`
**Satisfies:** FR-23, FR-24
**Steps:**
1. R: Playwright test that an unauthenticated user is redirected to
   `/auth/login` without seeing an error banner first.
2. G: Implement `<AuthGate>` per design §11; add `useMeQuery` via codegen.
3. R: Vitest test that `<AuthGate>` renders the skeleton while loading.

**Acceptance:** issue #164 reproduction case passes; manual repro shows no
error flash.

**Phase 1 exit criterion:** new UI shows header, sidebar, and login state
matching the legacy UI.

---

## Phase 2 — Filter infrastructure

Goal: the reusable filter/pagination/facet machinery, exercised on the
first page that needs it. **This is the heart of the user's "front is
heavy" concern.**

### T2.1 — DB migration for filter indexes + `saved_views`
**Satisfies:** FR-12, FR-15
**Steps:**
1. R: Repository tests that assert query plans use the new indexes on a
   seeded dataset (10K rows).
2. G: Author `migrations/NNN_filter_indexes.up.sql` per design §5.
3. G: Author matching `.down.sql`.
4. F: Verify `CREATE INDEX CONCURRENTLY` is used; rollback tested.

**Acceptance:** EXPLAIN ANALYZE shows index usage for representative queries
listed in design §5.

---

### T2.2 — Domain `TestRunFilter`, `PageArgs`, `TestRunPage` types
**Satisfies:** FR-9, FR-10
**Steps:**
1. R: Unit tests for filter validation (e.g., clamp `first` ≤ 200, reject
   inverted date ranges).
2. G: Implement in `internal/domains/testing/domain/`.

---

### T2.3 — Repository: filtered, paginated, faceted query
**Satisfies:** FR-9, FR-10, FR-14
**Steps:**
1. R: Integration test with seeded PG (testcontainers) covering each filter
   field, AND/OR tag mode, cursor stability under concurrent inserts.
2. G: Implement `infrastructure/test_run_repo_v2.go`.
3. F: Verify facet queries run in parallel.

**Acceptance:** P75 query latency on 1 M rows ≤ 250 ms (measured in test).

---

### T2.4 — Cursor codec
**Satisfies:** FR-10
**Steps:**
1. R: Property test: encode → decode → equal; tamper → reject.
2. G: HMAC-signed base64 JSON codec in `pkg/cursor/`.

---

### T2.5 — Total-count strategy
**Satisfies:** FR-14 (totalCount)
**Steps:**
1. R: Test that broad filters return estimate and narrow filters return exact.
2. G: Implement `count_strategy.go` per design §4.4.

---

### T2.6 — Redis facet cache
**Satisfies:** NFR (performance)
**Steps:**
1. R: Test cache hit / miss / TTL.
2. G: Implement keyed by `sha256(filter-without-facet)`, TTL 60 s.

---

### T2.7 — GraphQL schema additions
**Satisfies:** FR-21
**Steps:**
1. R: Schema-diff CI step (`graphql-inspector`) that allows additive
   changes only.
2. G: Add `TestRunFilter`, `ConnectionArgs`, `TestRunConnection`, deprecate
   old args per design §4.1.
3. G: Resolver wires to the new query service.

---

### T2.8 — REST v2 `/api/v2/test-runs` handler
**Satisfies:** FR-18
**Steps:**
1. R: Handler test: each filter param parsed, validation errors return 400
   with structured body.
2. G: Implement per design §6.
3. F: Add OpenAPI 3.1 spec entry to `docs/developers/api-reference.md`.

---

### T2.9 — `<FilterBar>` and `useUrlFilter` hook
**Satisfies:** FR-11
**Steps:**
1. R: Vitest test: schema → URL params → state round-trip.
2. R: Playwright: change a filter, copy URL, paste in new tab, identical
   view loads.
3. G: Implement per design §9. Ship the schema-driven primitives.

**Acceptance:** changing any filter updates URL; reloading reproduces view.

---

### T2.10 — `<SavedViewMenu>` + saved-views API
**Satisfies:** FR-15
**Steps:**
1. R: API tests for `GET/POST/DELETE /api/v2/me/saved-views`.
2. G: Implement handlers + repo using `saved_views` table.
3. R: Playwright: save → log out → log in → view present.
4. G: Implement `<SavedViewMenu>` UI.

**Phase 2 exit criterion:** Test-Run list page on `/v2/test-runs` works
end-to-end with all filter primitives, virtualization, and saved views.

---

## Phase 3 — Page migrations (one PR each)

Each page follows the same pattern: design parity, Playwright golden path,
v2 API used, virtualized rendering where applicable.

### T3.1 — Project list (`/v2/projects`)
**Satisfies:** U1, U7
**Acceptance:** parity with legacy `/projects`; Playwright passes; bundle
budget passes.

### T3.2 — Project detail & settings (`/v2/projects/:id`)

### T3.3 — Test run list (`/v2/test-runs`)
Already partially built in T2.9; finalize columns, sorting, empty state.
Columns match v1 (FR-15b): Project, Run ID, Branch, Test Results
(passed/failed/skipped), Status, Duration, Started.
**Satisfies:** FR-15b.

### T3.4 — Test run detail (`/v2/test-runs/:id`)
Two-view drill (FR-17): the page lands on a *Suites* table with v1's
column set; clicking a suite row swaps the table for a *Specs* table
with v1's spec column set. A back-link returns to suites. The run-level
header is always visible above both views. Error messages are
inline-truncated with a click-to-expand stack-trace block. Originally
planned as a virtualized `react-arborist` tree; the table-drill model
was preferred for v1 parity (see RFC-004 Appendix B.11).
**Satisfies:** FR-17, U5.

### T3.5 — Tag management (`/v2/tags`)

### T3.6 — Flaky test dashboard (`/v2/flaky`)

### T3.7 — Treemap (`/v2/treemap`)
Drill-down per design §10. Highest visual-fidelity bar — visual snapshot
tests required.
**Satisfies:** FR-16.

### T3.7a — Test Summaries v1 parity (stats, history link, view toggle)

The v2 Test Summaries page (`web-v2/src/features/summaries/TestSummaries.tsx`)
currently shows per-project sparklines with Runs / Pass rate / Avg-per-run
stats. v1 surfaced **Total tests / Passed / Failed** counters, a **View
history** button per card, and a **Card view ↔ Tree view** toggle backed by
the treemap. This task brings those three affordances into v2 without
introducing new aggregate paths.

**Satisfies:** FR-17a, FR-17b, FR-17c, FR-17d.

**Steps:**

1. **R** — Add a `TestSummaries.test.tsx` Vitest case that mounts the page
   with a stubbed `/api/v2/test-runs/trends` response and asserts the three
   labelled counters (`Total tests`, `Passed`, `Failed`) render with the
   summed values. Stub `useUserPreferences` so `defaultTestSummariesView`
   round-trips.
2. **G** — In `ProjectTrendCard`, replace the existing `Runs / Pass rate /
   Avg s/run` triplet with `Total tests / Passed / Failed`. Sum from the
   already-loaded `buckets` array — do **not** add a new fetch. Keep the
   sparkline below, unchanged. Render `Skipped` inline only when non-zero.
3. **R** — Add a test asserting the card exposes a **View history** link
   that points at the project's history view for the same `days` window.
4. **G** — Add a `<Link to="/projects/$projectId" search={{ tab: 'history',
   days }}>` style affordance (use the project-detail route already in
   place — confirm tab/search params at impl time). Visible label, not
   icon-only.
5. **R** — Add a test asserting the page renders a `Card view / Tree view`
   toggle, and that selecting `Tree view` swaps the grid for a treemap
   component fed by the existing `/treemap` query. Persisting the choice
   should call the user-preferences mutation with
   `defaultTestSummariesView: 'treemap'`.
6. **G** — Implement the toggle. Reuse the existing v2 `<Treemap>`
   component (Phase 3 T3.7) and the existing `/treemap` aggregate
   endpoint — **no new server work**. Load the persisted preference on
   mount; default to `card`. Tree view shares the same window selector.
7. **F** — Pull the counter triplet, the history link, and the view toggle
   into small subcomponents inside the file (or sibling files under
   `features/summaries/`) if any block grows past ~40 lines. Keep the
   trends fetch in one place.

**Acceptance (FR-17a/b/c/d):**
- Each card shows Total / Passed / Failed counters that match the trends
  aggregate for the selected window.
- `View history` is a single-click affordance and lands on the project's
  filtered history.
- Toggling Card ↔ Tree view persists across reloads via user preferences.
- No new GraphQL or REST endpoints; only the existing
  `/api/v2/test-runs/trends` and `/treemap` resolvers are called.
- Lighthouse + bundle-size budgets (NFR-3, NFR-9) remain green.

**Phase 3 exit criterion:** every legacy page has a v2 equivalent with
parity and Playwright coverage.

---

## Phase 4 — Cutover

### T4.1 — Two-week dual-run on `main` branch
**Steps:**
1. Internal users default to v2; external users see a banner with opt-in.
2. Telemetry compares Web Vitals between legacy and v2.

**Acceptance:** v2 P75 LCP and INP equal or better than legacy.

---

### T4.2 — Flip the default
Legacy `/` redirects to `/v2/`. Banner removed.

---

### T4.3 — Sunset announcement for `/api/v1`
**Satisfies:** FR-20
**Steps:**
1. Publish migration guide at `docs/developers/migrate-v1-to-v2.md`.
2. Announce sunset date (12 months from this PR) to client-lib repos.

---

## Phase 5 — Removal

### T5.1 — Delete legacy frontend
**Satisfies:** acceptance criterion 1
**Steps:**
1. Delete `web/index.html` (legacy), `web/js/*.js`, `web/css/font-awesome.min.css`.
2. Remove FontAwesome → emoji fallback CSS, the inline `formatDuration`
   fallback, all `?v=N` cache-busters.
3. Move `/v2/*` routes to `/*` in the Go server.

### T5.2 — Consolidate Dockerfiles
**Steps:**
1. Delete `Dockerfile.simple` and `Dockerfile.goreleaser`.
2. Replace `Dockerfile` with the multi-stage version from design §13.

### T5.3 — Final docs sweep
**Steps:**
1. Update `README.md` install section.
2. Update `CONTRIBUTING.md` with new dev workflow.
3. Update `docs/ARCHITECTURE.md` to reflect the new frontend layout.
4. Archive `docs/specs/frontend-modernization/` to `docs/specs/archive/`.

**Phase 5 exit criterion:** repository contains no references to the legacy
frontend; CI is green; deploy artifact is a single binary.

---

## Cross-cutting non-functional tasks

These run in parallel with feature work; each is its own PR.

### TX.1 — Playwright suite scaffold + CI
**Satisfies:** NFR-2
Run against `make deploy-all` stack in CI.

### TX.2 — `@axe-core/playwright` accessibility assertions
**Satisfies:** NFR-5
Fail CI on critical violations.

### TX.3 — Bundle-size CI gate
**Satisfies:** NFR-3
Per design §12.

### TX.4 — Performance budget CI gate
**Satisfies:** requirements §4 budgets
Lighthouse + custom Playwright timings on seeded dataset.

### TX.5 — CSP header
**Satisfies:** NFR-7
Add to middleware; verify with `securityheaders.com`-style test.

### TX.6 — Web Vitals telemetry
**Satisfies:** observability §14
`web-vitals` lib → `/api/v2/telemetry/vitals` (sampled 10 %).

---

## Backwards-compatibility tasks (run continuously, gate every phase)

These tasks enforce the [compatibility principle](./requirements.md#1a-compatibility-principle-non-negotiable).
They are not optional and not deferrable. Each phase exit requires the
boxes in §11 of [migration-guide.md](./migration-guide.md) to be checked.

### TBC.1 — v1 contract golden tests
**Satisfies:** FR-19, FR-22, FR-27
**Steps:**
1. R: For each v1 endpoint, capture a response golden file from `main` at
   the start of this spec; commit to `internal/api/v1/testdata/`.
2. G: Add a CI test that replays the requests and diffs the responses
   byte-for-byte. Diff = failure.
3. F: Mark the test required for merge to `main`.

**Acceptance:** any PR that changes a v1 response shape fails CI without
the test author explicitly updating goldens *and* a `compat-review` label
from a designated reviewer.

---

### TBC.2 — GraphQL schema-diff gate
**Satisfies:** FR-21
**Steps:**
1. G: Add `graphql-inspector` (or equivalent) to CI; baseline = schema on
   `main` at start of spec.
2. G: Configure rules: additive ≥ ok; field removal / retype = block;
   `@deprecated` addition = ok.
3. F: Document the override flow for genuinely-needed breaking changes
   (requires major version bump + 1 release of deprecation).

**Acceptance:** PRs that remove or retype a non-deprecated field fail CI.

---

### TBC.3 — Config compatibility test
**Satisfies:** FR-30
**Steps:**
1. R: Test that boots the binary with the pre-spec config (committed as a
   fixture) and asserts no missing-required-key errors.
2. R: Test that `FERN_V2_UI_ENABLED=false` returns the legacy UI at `/`
   and that no `/v2/*` route is registered.
3. G: Adjust defaults so both tests pass.

**Acceptance:** existing operators can upgrade without touching their
config.

---

### TBC.4 — DB-migration safety lints
**Satisfies:** FR-29
**Steps:**
1. G: Lint each `migrations/*.up.sql` for: `DROP COLUMN`, `RENAME COLUMN`,
   `ALTER TYPE` (narrowing), `DROP TABLE`, missing `CONCURRENTLY` on index
   creation. Any match fails CI.
2. G: Require a matching `.down.sql` for every `.up.sql`.
3. F: Document the override flow (rare, requires explicit sign-off).

**Acceptance:** the lint runs on every PR; rejects unsafe migrations.

---

### TBC.5 — Publish `migration-guide.md` and link it everywhere
**Satisfies:** FR-28, acceptance criterion (migration guide)
**Steps:**
1. G: Copy [migration-guide.md](./migration-guide.md) to
   `docs/developers/migrate-v1-to-v2.md` (user-facing location).
2. G: Add link from `README.md`, `CONTRIBUTING.md`, and the deprecation
   `Link` header.
3. G: Add a release-notes template that requires a "Migration impact"
   section pointing at the guide.
4. F: Update the guide whenever a phase ships a contract change; commit
   the update in the same PR as the change.

**Acceptance:** the link is reachable; release notes for every v2-related
release reference the guide.

---

### TBC.6 — `fern migrate check` CLI
**Satisfies:** FR-28, acceptance criterion (CLI)
**Steps:**
1. R: Unit tests for the log-parser (Nginx/Caddy/access-log formats) and
   the v1→v2 mapping table.
2. G: Implement `fern-platform migrate check` subcommand:
   - reads access logs from stdin, file, or a Loki/CloudWatch source,
   - groups by endpoint and User-Agent,
   - prints suggested v2 calls,
   - exits non-zero if any v1 traffic was seen in the window.
3. F: Package into the standard binary; document in the guide.

**Acceptance:** operators can run the command in CI before v1 sunset.

---

### TBC.7 — Dual-run telemetry & opt-in/opt-out banner
**Satisfies:** FR-25, U7
**Steps:**
1. G: Implement a per-user `ui_pref` cookie (`legacy` | `v2`).
2. G: Banner on legacy UI: "Try the new UI" → sets cookie, redirects to
   `/v2/*`.
3. G: Banner on v2 UI: "Revert to legacy" (only during dual-run window).
4. G: Emit metrics `fern.ui.session` tagged with `version` for adoption
   tracking.

**Acceptance:** users can opt in or out without operator intervention.

---

### TBC.8 — Redirect map at cutover
**Satisfies:** FR-25, FR-27 (bookmarks keep working)
**Steps:**
1. R: Playwright test: every legacy URL pattern hit after cutover lands
   on the equivalent v2 page (HTTP 302 + final 200).
2. G: Implement `internal/web/redirects.go` with the full table.
3. F: Remove the table one release after `/v2/*` is moved to `/*`.

**Acceptance:** zero broken bookmarks reported in user feedback during
the redirect window.

---

## Definition of Done (whole spec)

- All acceptance criteria from [requirements.md §6](./requirements.md#6-acceptance-criteria) check.
- All open questions in requirements §7 have decisions recorded in
  [design.md §17](./design.md#17-decision-log).
- Spec archived to `docs/specs/archive/frontend-modernization/`.
- One blog/changelog post announces the change and the v1 sunset timeline.

---

## Estimate

| Phase | Engineer-weeks (1 FE), best case | (2 FE), realistic |
|---|---:|---:|
| 0 | 1.0 | 0.5 |
| 1 | 1.0 | 0.5 |
| 2 | 2.5 | 1.5 |
| 3 | 4.0 | 2.5 |
| 4 | 1.0 | 0.5 |
| 5 | 0.5 | 0.5 |
| Cross-cutting | 1.0 | 0.5 |
| **Total** | **~11 weeks** | **~6.5 weeks** |
