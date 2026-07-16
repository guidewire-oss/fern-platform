## Description

Mounts a new Vite + React + TypeScript SPA at `/v2/*` alongside the legacy v1 UI at `/`, plus the backend surface and operational plumbing that make it work end-to-end. Implements [RFC-004 Frontend Modernization](docs/rfc/rfc-004-frontend-modernization.md) with the table-drill UX for v1 parity (see Appendix B.11 of the RFC for the deviation from the originally-planned virtualized tree).

The runtime rollout model is unchanged: v1 keeps serving at `/`, v2 mounts at `/v2/*` behind `FERN_V2_UI_ENABLED` (default off). Operators flip the flag when ready; v1 sunsets on a 12-month schedule.

**Scope:** 286 files, +35,741 / -396. **v1 deletions:** 0 — strangler pattern keeps v1 reachable until sunset.

## Type of Change

- [x] New feature (non-breaking)
- [x] Performance improvement
- [x] Documentation update
- [x] Refactoring (backend domain layer, treemap cache extraction)
- [ ] Bug fix
- [ ] Breaking change

## Migration / Compatibility

| What | Status |
|---|---|
| `/api/v1/*` request/response shapes | **unchanged** — 12-month sunset window |
| `web/` (v1 SPA) | unchanged, still serves at `/` |
| v1 ingestion clients (`fern-ginkgo-client`) | unaffected |
| Auth flow (Keycloak OAuth/OIDC) | unchanged for both UIs |
| Existing migrations (0001-0021) | untouched |
| New migration `000022_v2_schema` | additive, idempotent (`IF [NOT] EXISTS` everywhere) |
| Environment variables added | `FERN_V2_UI_ENABLED`, `FERN_V1_DEPRECATED`, `FERN_V1_SUNSET_DATE`, `FERN_V1_MIGRATION_GUIDE_URL`, `JIRA_ENCRYPTION_KEY` (**required at startup**) |

For deployment instructions, including the pre-flight index script for large databases, see [`docs/specs/frontend-modernization/migration-guide.md`](docs/specs/frontend-modernization/migration-guide.md).

## Testing

- [x] Unit tests added (Go: +5 cmd/seed tests, web-v2: 65 Vitest tests across 9 files)
- [x] Manual testing completed (docker-compose stack, gradient seed, all v2 pages exercised)
- [ ] Acceptance tests added/updated *(deferred to follow-up — needs k3d cluster context)*

| Check | Status |
|---|---|
| `go test ./...` (28 packages) | ✅ all pass |
| `go vet ./...` | ✅ |
| `govulncheck ./...` | ✅ 0 reachable vulnerabilities |
| `web-v2`: typecheck / lint / tests / build | ✅ all green |
| Web-v2 production CVE audit (`pnpm audit --prod`) | ✅ 0 vulnerabilities |
| GraphQL schema codegen drift | ✅ none |
| Secret scan on diff | ✅ no real secrets introduced |
| Embedded `internal/web/dist/` matches fresh build | ✅ |

## Checklist

- [x] Code follows style guidelines (`.golangci.yml` + web-v2 ESLint)
- [x] Self-review completed
- [x] Documentation updated (RFC-004 Appendix B has 11 deviation entries; migration guide rewritten)
- [x] Tests pass locally
- [x] No production-shipping CVEs (5 dev-tooling advisories in vite/esbuild/lodash transitive — documented, not blocking)
- [x] Conventional commit format
- [x] Rebased on latest `main`

---

## What's in this PR

<details>
<summary><b>1. Frontend modernization — the headline change</b></summary>

### `web-v2/**` — new React+TypeScript SPA (69 files)

Vite + TypeScript + Tailwind + TanStack Router + TanStack Query, mounted at `/v2/*`. Pages:

| Page | Route | Notes |
|---|---|---|
| Dashboard | `/v2/` | Project tiles + recent activity |
| Projects list | `/v2/projects` | Filter bar, infinite-scroll pagination (up to 500), favorites |
| Project detail | `/v2/projects/:id` | Stacked-area test-history chart ported from v1 |
| Project settings | `/v2/projects/:id/settings` | 4 tabs: General / Integrations / Team / Notifications |
| Test runs list | `/v2/test-runs` | Filter sidebar, saved views, keyset pagination, **v1-parity columns** (Project, Run ID, Branch, Test Results P/F/S, Status, Duration, Started) |
| Test run detail | `/v2/test-runs/:id` | **v1-parity two-view drill**: suites table → click → specs table → back. Run header always visible with tags, metadata, stat cards, stack-trace expand on error cells |
| Test summaries | `/v2/test-summaries` | Card view (sparklines, P/F counters, View History link) + Tree view (treemap embedded) toggle, 7-day default window, persists to localStorage |
| Manager dashboard | `/v2/manager-dashboard` | Thin redirect to `/v2/test-summaries?view=tree&favoritesOnly=true`, role-gated |
| Saved views | `/v2/saved-views` | CRUD page for filter presets |
| Tags | `/v2/tags` | Real usage counts |
| Users (admin) | `/v2/users` | Suspend / activate / delete / role change |
| Settings | `/v2/settings` | User-level notification prefs |
| Admin overview | `/v2/admin` | System health summary |
| Jira integrations | `/v2/jira` | Connection list + CRUD |
| Profile | `/v2/profile` | Read-only user info, role, sign-out |

Shared infrastructure: `AppShell` (TopBar dropdown + role-filtered Sidebar), `ErrorBoundary`, `RoleGuard`, 9 reusable UI primitives (Button / Card / Input / Modal / Pagination / Sparkline / Spinner / StatusBadge / Table).

### `internal/web/dist/**` — embedded built artifacts (83 files)

The v2 SPA's `vite build` output, embedded via `//go:embed all:dist` so the final container ships as one artifact. Checked in so the Go build doesn't require pnpm.

</details>

<details>
<summary><b>2. Backend — REST + GraphQL surface</b></summary>

### `internal/api/v2/**` — new REST namespace (10 files)

A `/api/v2/*` REST API for endpoints whose request/response shape changed. Co-existing with `/api/v1/*` which stays unchanged.

- `test_run_handler.go` — `GET /api/v2/test-runs` with cursor pagination + facets + 7-day default window + opt-in tag facet (`?facets=tag`)
- `test_run_trends_handler.go` — `GET /api/v2/test-runs/trends` aggregates per-(project, day) sums for sparklines via one `GROUP BY` instead of N parallel requests
- `saved_view_handler.go` — CRUD for saved filter views
- `telemetry_handler.go` — `POST /api/v2/telemetry/vitals` for client Web Vitals
- `health_handler.go` — k8s-friendly readyz/livez with explicit DB ping

### `internal/domains/testing/**` — query / page / facet engine (24 files)

DDD-layered:

- **domain/** — `Filter`, `PageArgs`, `TestRunPage`, `SavedView` types
- **application/** — `TestRunQueryService` orchestrates filter + page + facet; `FacetCache` interface with in-memory + Redis impls; opt-in tag facet via `IncludeTagFacet`
- **infrastructure/** — `GormTestRunQueryRepo`, `BuildTestRunWhere` (the SQL filter builder), `CountStrategy` (estimate vs exact), `SavedViewRepo`
- **interfaces/** — `CompatibilityAdapter` now propagates context properly (no more silent `context.Background()`)

### `internal/reporter/graphql/**` — schema + resolvers (14 files)

- New schema entries: `treemapData(projectId, suiteName, days)`, `SuiteTreemapNode`, `SpecTreemapNode`, `ProjectConnection`, `TestRunStats`
- `TreemapCache` interface (in-memory + Redis) with 60s TTL, keyed by `(userID, drillProjectID, days)`
- **Request-scoped dataloaders** (was process-scoped — caused stale data at server startup)
- `domain_resolvers.go` populates per-spec `passRate`, `TotalRuns/PassedRuns/FailedRuns/SkippedRuns` on `SpecTreemapNode`

### `internal/api/*` modifications (9 files)

Previously-stubbed admin endpoints now do real work:

- `auth_handler.go` — `suspendUser`, `activateUser`, `deleteUser` (were returning fake 200s)
- `project_handler.go` — `grantProjectAccess`, `revokeProjectAccess`, `getProjectUsers` against `project_permissions` table
- `tag_handler.go` — real usage counts via new SQL `UsageCounts` aggregate

### Domain additions (8 files)

- `UserRepository` gains `UpdateStatus` + `SoftDelete` methods
- `factory.go` — **`loadJiraEncryptionKey()` reads from env, fails fast at startup** (closed a hardcoded-literal security regression)

### `internal/testhelpers/mocks.go`

Mock impls for the 6 new repo methods.

</details>

<details>
<summary><b>3. Database</b></summary>

### `migrations/000022_v2_schema.{up,down}.sql`

Single consolidated migration (was 3 during dev, squashed before PR):

- 4 indexes on `test_runs` (project+time desc, keyset, partial failed/flaky, project+branch)
- `pg_trgm` extension + GIN trigram index on `spec_runs(spec_name + error_message)` for substring search
- `saved_views` table (user-scoped filter presets)

Idempotent (`IF [NOT] EXISTS` everywhere); zero v1 schema changes — no columns renamed or dropped.

### `pkg/database/db.go` modifications

- **Dirty-schema fail-fast** — `Migrate()` refuses to proceed if `schema_migrations.dirty=true`
- Post-migration version check guards against multi-replica races

</details>

<details>
<summary><b>4. Deployment & operations</b></summary>

### `docker-compose.yml` + `docker-compose.local.yaml.example`

Full local stack (postgres + redis + fern) with healthchecks. The `.local.yaml.example` is an opt-in overlay (gitignored when copied) for per-developer config overrides.

### `deployments/fern-platform-kubevela.yaml`

- `JIRA_ENCRYPTION_KEY` now reads from a k8s Secret (`secretKeyRef`), not inline
- `FERN_V2_UI_ENABLED` env var added (defaults off)

### `Makefile`, `Makefile.docker`, `Makefile.web`

- `make docker-test-up/down/rebuild` — full local stack lifecycle
- `make docker-test-seed-{gradient,perf,extras}` — three seed modes
- `make jira-key` — generates per-developer 32-byte key to `~/.fern/jira-key`
- `make web-v2-{deps,build,fix,stage}` — web-v2 lifecycle

### `cmd/seed/{main,extras,templates}.go` — bulk perf-test seeder

Replaces the old simple seeder. Highlights:

- Uses pgx `CopyFrom` for bulk loading
- Can do 1000 projects × 180 days × 100 runs/day = 18M test_runs
- Realistic categories (Java / infra / Flux / Helm / Web)
- Extras: users, user_groups, user_preferences, user_scopes, project_permissions, jira_connections, flaky_tests, saved_views
- **`SEED_HEALTH_BANDS=true`** mode distributes projects across 5 pass-rate bands for the treemap gradient
- `SEED_SPECS_PER_FAILED_SUITE` controls per-suite spec_runs row count

</details>

<details>
<summary><b>5. CI / observability / testing infrastructure</b></summary>

### `.github/workflows/ci.yml`

A single new **`web-v2`** job added to the existing `ci.yml` (no new workflow files, per OSS contribution policy). Runs in parallel with the Go test job. Steps:

- GraphQL schema-drift check (fails if `internal/reporter/graphql/schema.graphql` and `web-v2/src/gql/schema.graphql` diverge)
- `pnpm typecheck` (TypeScript strict)
- `pnpm lint` (ESLint, `--max-warnings 0`)
- `pnpm test:run` (Vitest, 65 tests)
- `pnpm build` (production bundle)

Go v2 packages are already covered by the existing `test` job's `go test ./...`; v2-specific govulncheck is folded into the existing `vulnerability-check` job.

### `pkg/{cursor,metrics,tracing,middleware/{csp,deprecation,devauth}}` (19 files)

- `cursor` — opaque keyset cursor codec
- `metrics` + `prometheus` — Prometheus exposition
- `tracing` — OpenTelemetry init
- `middleware/csp` — Content Security Policy
- `middleware/deprecation` — RFC 8594 Sunset/Deprecation headers for `/api/v1/*`
- `middleware/devauth` — Local dev-admin bypass when `AUTH_ENABLED=false`
- `pkg/middleware/oauth.go` — centralized SameSite cookie policy

### `tests/`

- `tests/perf/` — k6 scripts (ingest spike, soak, test-runs list)
- `tests/contract/` — Pact-style consumer/provider tests
- `perf-budgets.json` — perf SLO thresholds
- `scripts/v2-preflight-indexes.sh` — non-blocking `CREATE INDEX CONCURRENTLY` for large prod DBs
- `scripts/docker-test-smoke.sh` — quick UI smoke check

</details>

<details>
<summary><b>6. Bug triage closed in this PR</b></summary>

A working punch-list was kept during development. Items resolved in this PR:

**Security / "looks live but isn't":**
- Hardcoded JIRA encryption key removed (env required, `make jira-key` for dev)
- Admin user-management stubs wired to real repo
- Dead REST stubs deleted (`getUserPreferences`, `updateUserPreferences`)
- Tag usage counts now real (was returning 0)
- Project access management wired to `project_permissions`

**v2 correctness:**
- `canManage` cache key now includes role (gear icon shows/hides correctly per role)
- `RoleGuard` for `/users` + `/admin` routes
- AuthGate 401 → `/auth/start` redirect via sentinel
- Manager dashboard denial gets recovery links + Sign in CTA
- Project settings danger-zone confirm: case-insensitive + trimmed

**Shared backend hardening:**
- Treemap cache extracted to interface (in-memory default, Redis available)
- Compatibility adapter context propagation
- SameSite cookie policy centralized
- Migration dirty-flag fail-fast
- Trends endpoint replaces client-side fan-out (~400 SQL queries → 1)

**Seed-data correctness fixes:**
- `computeSpecFailures` — when `perSuite == totalSpecs` (full spec coverage), the body count now matches the suite header's `failedSpecs` exactly (closes the "header says 3 failed, body shows 4 failed" bug)
- `sampleNearTarget` — replaces uniform `1 + rng.Intn(maxFail)` (mean = maxFail/2) with biased sampling clustered near `maxFail` (mean ≥ 85%). Without this, every health band in the gradient seed averaged toward green; band 0 now produces visibly red tiles
- Each has unit tests in `cmd/seed/healthbands_test.go`

</details>

<details>
<summary><b>7. Documentation</b></summary>

- [`docs/rfc/rfc-004-frontend-modernization.md`](docs/rfc/rfc-004-frontend-modernization.md) — the architectural RFC. **Appendix B has 11 entries** documenting where implementation diverged from the original proposal (search uses trigrams not FTS, 7-day default window, opt-in tag facet, in-process caches, consolidated migrations, dedicated trends endpoint, treemap embedded in Summaries, manager dashboard as redirect, encryption key required, single-PR delivery, table-drill instead of virtualized tree)
- [`docs/specs/frontend-modernization/{requirements,design,tasks,migration-guide,PHASES,feature-filters-pagination-favorites}.md`](docs/specs/frontend-modernization/) — full spec triplet plus supporting docs. FR-15b and FR-17 reflect the v1-parity column sets delivered
- [`docs/api/v2-openapi.yaml`](docs/api/v2-openapi.yaml) — OpenAPI 3.1 spec for the new `/api/v2/*` surface
- [`docs/user-guide/permissions.md`](docs/user-guide/permissions.md), [`CONTRIBUTOR_GUIDE.md`](CONTRIBUTOR_GUIDE.md) — supplemental guides

</details>

---

## Local verification

```bash
# Build and start the stack
make docker-test-build
make docker-test-up

# Seed 10 projects across all 5 health bands with full spec coverage
make docker-test-seed \
  SEED_PROJECTS=10 \
  SEED_DAYS=7 \
  SEED_RUNS_PER_DAY=5 \
  SEED_SUITES_PER_RUN=8 \
  SEED_SPECS_PER_FAILED_SUITE=50 \
  SEED_HEALTH_BANDS=true \
  SEED_TRUNCATE=true

# Verify row counts
make docker-test-perf-counts

# Open the v2 SPA
open http://localhost:8080/v2/
```

For the deployed Kubernetes path (k3d / KubeVela), see `docs/specs/frontend-modernization/migration-guide.md` §4.

## Known follow-ups (not blocking this PR)

- 5 dev-tooling CVE advisories (vite 5 → 6, esbuild, transitive lodash) — none ship in the production bundle; addressable in a follow-up PR
- Acceptance test coverage for new `/api/v2/*` endpoints — `tests/perf` + `tests/contract` scaffold present, full suite is follow-up
- Phase 4 (cutover banner + redirect map) and Phase 5 (legacy removal) — operator-controlled flag flips, not code-shipping decisions; tracked in [`docs/specs/frontend-modernization/tasks.md`](docs/specs/frontend-modernization/tasks.md)
