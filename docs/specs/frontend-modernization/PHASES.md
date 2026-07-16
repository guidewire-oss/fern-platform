# Frontend Modernization — Phase Tracker

Running log of what has shipped (across the six local feature branches),
what is parked, and where to pick up. Kept in the spec directory so the
next session can resume without archaeology.

**Branches (local only, none pushed):**

| Branch | Head |
|---|---|
| `feat/frontend-modernization-phase-0` | scaffold + Phase 2 backend stubs |
| `feat/frontend-modernization-phase-2-backend` | Phase 2 backend completion + `main.go` wiring |
| `feat/frontend-modernization-phase-3` | Phase 2 closure + perf/CI infrastructure |
| `feat/frontend-modernization-phase-4-observability` | metrics + CSP + Pact scaffold |
| `feat/frontend-modernization-phase-5-observability-deep` | Prom text + OTel + OpenAPI + k8s health + slow-query + saved-views pagination |
| `feat/frontend-modernization-phase-7-real-pages` | **current head** — real React pages, filters/pagination/favorites, treemap perf, search, settings, perf seeder |

Each later branch is forked from the previous one and includes everything below.

---

## Shipped

### Spec & docs
- [x] RFC-004 (Frontend Modernization)
- [x] Spec triplet — requirements / design / tasks / migration-guide
- [x] CONTRIBUTOR_GUIDE.md
- [x] OpenAPI 3.1 spec for `/api/v2/*` at `docs/api/v2-openapi.yaml`

### Backend — v2 surface
- [x] T0.1 v1 grouping (existing in `domain_handler_v2.go`)
- [x] T0.2 Deprecation middleware (`pkg/middleware/deprecation.go`)
- [x] T0.4 `internal/web/embed.go` — SPA fallback, reserved API prefixes
- [x] T0.5 Make targets (`Makefile.web`)
- [x] T2.1 Schema migrations — **consolidated into a single `000022_v2_schema`** (was 22/23/24 during development; squashed before any external operator ran it). Indexes for filter/keyset/facets, `pg_trgm` extension + trigram GIN on `spec_runs`, `saved_views` table.
- [x] T2.2 Domain filter types (`internal/domains/testing/domain/filter.go`) — incl. `IncludeTagFacet` opt-in
- [x] T2.3 Filter→SQL translator + full GORM repository with facets
- [x] T2.4 Cursor codec (`pkg/cursor`) — **cursor decoder fix:** `fetchPage` now applies the `(start_time, id) < (?, ?)` keyset predicate from `p.After` (was silently ignored before, causing "Next" to repeat page 1)
- [x] T2.5 Count strategy (narrow exact vs broad estimate)
- [x] T2.6 Facet cache — in-memory + Redis-adapter via `RedisLike` interface; **errgroup-parallelized** so the four GROUP BY queries run concurrently; **tag facet is opt-in** (`?facets=tag`) because the suite_runs join is expensive at scale
- [x] T2.8 REST handler `GET /api/v2/test-runs` — supports cursor pagination, all filter dimensions, `?facets=tag`, and applies a **default 7-day window** when no `from/to` is supplied (escape hatch: `?allTime=1`)
- [x] **Trends aggregate endpoint** — `GET /api/v2/test-runs/trends?project=…&days=N` returns per-(project, day) sums in one response, backed by a single SQL `GROUP BY`. 60s in-process cache keyed by `(userID, sorted projectIDs, days)`. Replaces the fan-out pattern the Test Summaries page used (1 HTTP request instead of N).
- [x] T2.10 Saved views — domain, GORM repo, REST CRUD, pagination; saved-view rows use `time.Time` columns (not int64) to satisfy pgx timestamptz encoding
- [x] **Substring search** — switched from English-FTS (couldn't match camelCase like `DataIntegrityViolationException`) to `pg_trgm` GIN over `spec_name || error_message` with `ILIKE '%q%'`
- [x] **Treemap perf rewrite** — `AggregateProjectsInRange` / `AggregateSuitesInRange` repository methods do GROUP-BY in SQL; resolver no longer hydrates every test_run + suite_runs. 60s in-memory cache keyed by `(userID, drillProjectID, days)`.
- [x] **Dev-admin user upsert** at startup when auth is disabled — eliminates FK violations on `saved_views.user_id` in DevAuth mode
- [x] **Bulk perf seeder** — `cmd/fern-platform-seed/perf` produces 1000 projects × 6 months × 100 runs/day = ~18M test_runs across realistic categories (Java / infra / FluxCD / Helm / Node.js)
- [x] `POST /api/v2/telemetry/vitals` — Core Web Vitals ingestion
- [x] `/healthz` + `/readyz` (k8s-style probes)
- [x] `/metrics` — Prometheus text exposition (real format)
- [x] `main.go` wiring behind `FERN_V2_UI_ENABLED`
- [x] **Operator tooling** — `scripts/v2-preflight-indexes.sh` for `CREATE INDEX CONCURRENTLY` on large prod DBs; migration-guide.md updated with cost table + recovery procedure for dirty migrations

### Cross-cutting
- [x] Prometheus metrics middleware (in-memory recorder + text exposition)
- [x] OpenTelemetry tracing scaffold (`pkg/tracing`, `NoopTracer`)
- [x] CSP middleware (HTML-scoped)
- [x] Slow-query GORM plugin (`pkg/database/slow_query.go`)
- [x] `perf-budgets.json` single source of truth
- [x] k6 scenarios — load, spike, soak (`tests/perf/`)
- [x] Lighthouse CI config (`web-v2/.lighthouserc.json`)
- [x] CI coverage — `web-v2` job added to the existing `ci.yml` (no new workflow files, per OSS contribution policy), with an inline GraphQL schema-drift check between the backend SDL and `web-v2/src/gql/schema.graphql`
- [x] Pact provider verification scaffold (env-gated)
- [x] `docker-compose.yml` + `make docker-test-up` for local smoke

### Frontend
- [x] `web-v2/` scaffold (Vite + TS strict + Tailwind + TanStack + codegen + size-limit configs)
- [x] `web-v2/src/lib/duration.ts` — ported from legacy `web/js`
- [x] **Real pages** — Dashboard, Projects, Project Detail, Project Settings (4 tabs: General / Integrations / Team / Notifications), Test Runs list, Test Run Detail, Test Summaries, Treemap, Profile, Admin Overview, Users, Jira Connections
- [x] **Test Runs page** — filter sidebar (date range presets, status / branch / project / tag facets, duration range, favorites-only toggle, opt-in tag-facet section), saved-views bar with Save/Load/Delete, cursor pagination with client-side history stack for Previous, page indicator (`X-Y of N · Page A of B`)
- [x] **Projects page** — filter bar (search, team multiselect, category multiselect, favorites toggle, sort by name/runs/rate/last activity, active chips with clear-all)
- [x] **Test Summaries page** — same filter bar surface reused; per-project trend cards with sparklines
- [x] **Treemap** — projects view + suite drill-down; tooltip anchors to the hovered tile with edge-flip; loads via the new SQL aggregate path
- [x] **Per-project Settings page** with 4 tabs and routing under `/v2/projects/:id/settings`
- [x] **Theme switch** (light / dark) — CSS custom properties, `.dark` toggled on `<html>`
- [x] **Shared UI primitives** — `Input` / `Textarea` / `Select` using design tokens (`bg-surface` / `text-foreground`) so dark mode doesn't collapse to white-on-white; `Pagination`, `MultiDropdown` (native `<details>` + outside-click + Escape close), `EmptyState`, `Card`, `Spinner`, `Sparkline`
- [x] **Favorites** — wire `useToggleFavorite` + `useUserPreferences`; favorites star on project cards; favorites scope on Test Runs and Projects filter bars
- [x] **Saved views** — `useTestRunsSavedViews` hook + REST integration; Save dialog captures the current filter; Load drops it back in; Delete confirms
- [x] **Natural-order facet values** — Status by workflow order, Branch with main/master/develop first then natural alpha, Project + Tag by case-insensitive natural alpha
- [x] **Search field** — placeholder `error message, spec name…`; backed by the trigram path so `DataIntegrity` matches `DataIntegrityViolationException` etc.

---

## Parked (resume order recommended)

Numbered roughly by ROI. Pick any in order; later items don't strictly
depend on earlier ones unless noted.

### 1. URL state for filters (`useUrlFilter`)
**What:** Wire current React-state filters into the URL query string so links / refresh / Back button preserve the filter shape. Saved views handle the "remember a preset" case; this handles the "share a link to a filtered view" case.
**Why parked:** Saved views deliver 80% of the same value; URL sync is a refinement.
**Where to pick up:** New `web-v2/src/hooks/useUrlFilter.ts`; bridge to TanStack Router's `search` params on Test Runs and Projects pages.

### 1b. `<AuthGate>` component
**What:** Wrap protected routes; redirect to login when DevAuth is off and no session exists.
**Why parked:** DevAuth currently fabricates a `dev-admin` user, so the gate is a no-op in local development. Needed before any prod-with-auth rollout.
**Where to pick up:** `web-v2/src/components/AuthGate.tsx`; consult the auth domain for current-user state.

### 2. T2.7 GraphQL schema additions
**What:** Extend `internal/reporter/graphql/schema.graphql` with `TestRunFilter` / `ConnectionArgs` / `TestRunConnection`, mark legacy args `@deprecated`, run `go generate ./...`, wire the new resolver to `application.TestRunQueryService`.
**Why parked:** gqlgen regeneration touches many files and must be reviewed carefully (regenerated `generated.go` is large). Own PR for reviewability.
**Where to pick up:** `gqlgen.yml` is configured; the application service is ready (`TestRunQueryService.Query`); just add the schema types and wire one new resolver.

### 3. Prometheus client_golang adapter
**What:** Replace `metrics.InMemoryRecorder` with a `prometheus.Registerer`-backed adapter.
**Why parked:** Avoided pulling the dep until needed. In-memory + text exposition serves the smoke story today; the real client unlocks push-gateway and exemplars.
**Where to pick up:** New file `pkg/metrics/prometheus_client.go`, ~30 lines; satisfies `metrics.Recorder`.

### 4. OpenTelemetry SDK adapter
**What:** Real `Tracer` behind the interface, OTLP exporter env-gated.
**Why parked:** Same dependency-discipline reasoning as #3. `NoopTracer` keeps the middleware mounted.
**Where to pick up:** New file `pkg/tracing/otel.go`. Wire `OTEL_EXPORTER_OTLP_ENDPOINT` etc.

### 5. testcontainers-postgres for repository tests
**What:** Replace SQLite-backed tests in `gorm_test_run_query_repo_test.go` with Postgres via testcontainers, so FTS + partial-index + `pg_class` paths are covered.
**Why parked:** SQLite covers SQL plumbing; Postgres-only paths are reviewed manually. testcontainers is heavy in CI.
**Where to pick up:** New file `..._integration_test.go` with `//go:build integration` tag.

### 6. golden-file contract tests for v1 (TBC.1)
**What:** Per-endpoint response snapshots so PRs cannot regress v1 byte-for-byte.
**Why parked:** Requires running v1 endpoints against fixtures.
**Where to pick up:** `internal/api/v1/testdata/` + a recorder middleware.

### 7. DB-migration safety linter (TBC.4)
**What:** Lint each `migrations/*.up.sql` for `DROP COLUMN`, `RENAME COLUMN`, missing `CONCURRENTLY` on index creation.
**Where to pick up:** Small Go program + new CI step.

### 8. Saved-views update endpoint (PUT)
**What:** Rename / re-filter a saved view without delete+recreate.
**Where to pick up:** `saved_view_handler.go` + repo `Update` method.

### 9. Swagger UI mounting + SDK generation
**What:** Serve `docs/api/v2-openapi.yaml` at `/api/v2/docs`; generate TS/Python/Go SDKs via `openapi-generator`.

### 10. Pact consumer pipeline
**What:** Once the frontend lands, wire `pact-js` recorder in `web-v2`, publish to a broker.

### 11. Tag-facet correctness on the join shape
**What:** Verify the `suite_run_tags` join interacts correctly with the new pagination cursor when a run has many tagged suites — likely needs a `DISTINCT` review.

### 12. Frontend cutover & deprecation rollout
**What:** Phase 4 + Phase 5 of the spec — dual-run window, banner, then legacy removal.

### 13. Per-project Team ACL backend
**What:** Make the Team tab of `/v2/projects/:id/settings` interactive. Today it shows the owning team read-only with a "needs backend wiring" callout.
**What's needed:**
- Add `grantProjectAccess(projectId, team, role)` and `revokeProjectAccess(projectId, team)` GraphQL mutations on the projects domain.
- Application service + GORM repo methods. The `project_access` table already exists (migration 000010).
- Update the Team tab to list teams with their role, add a "Grant access" form, allow revoke.
- Update `userCanAccessProject` in the resolvers to consult the access table, not just the owning-team field.

### 14. Notifications delivery backend
**What:** Make the per-project Notifications tab toggles (failed runs / flaky / slow / first-failure) actually fire alerts. Today they persist to localStorage only.
**What's needed:**
- Decide the wire format: a `notification_subscriptions` table keyed by `(user_id, project_id, event_type)` is the obvious shape.
- Background dispatcher that watches `test_runs` inserts and matches subscriptions.
- Slack adapter (incoming webhook URL configured per-project on the Integrations tab — UI exists but currently marked "coming soon" for Slack).
- Email adapter (SMTP or a transactional provider — separate config).
- Replace the localStorage write in the Notifications tab with a GraphQL mutation against the new table.

---

## How to resume

1. Read this file.
2. Pick an item from "Parked".
3. `git checkout feat/frontend-modernization-phase-5-observability-deep`
4. `git checkout -b feat/frontend-modernization-phase-N-<topic>`
5. Implement; follow `.claude/rules/spec-first-tdd.md`.
6. Append a "Shipped" entry and remove from "Parked" when done.

## Local smoke test (now)

```bash
make docker-test-up        # build + start postgres+redis+fern
make docker-test-curl      # automated probe suite
make docker-test-down      # tear down + purge volumes
```

The compose stack runs on `:8080` (fern), `:55432` (postgres), `:56379` (redis).
