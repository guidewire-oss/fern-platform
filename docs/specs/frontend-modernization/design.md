# Spec: Frontend Modernization — Design

**Status:** Draft
**Related:** [requirements.md](./requirements.md), [tasks.md](./tasks.md), [RFC-004](../../rfc/rfc-004-frontend-modernization.md)

This document specifies the technical design that satisfies the requirements.
RFC-004 is the proposal and rationale; this is the implementation contract.

## 1. Architecture overview

```
┌─────────────────────────────────────────────────────────────────────┐
│ Browser                                                              │
│                                                                      │
│  React 18 SPA (TypeScript strict, Tailwind, lucide-react)           │
│   ├── Router (TanStack Router)         — type-safe, code-split      │
│   ├── Feature modules                                                │
│   │     ├── projects/                                                │
│   │     ├── test-runs/                                               │
│   │     ├── flaky/                                                   │
│   │     └── tags/                                                    │
│   ├── components/                                                    │
│   │     ├── ui/        (shadcn primitives)                           │
│   │     ├── filters/   (FilterBar, FacetSelect, ...)                 │
│   │     ├── charts/    (Treemap, TrendLine — wrap D3)                │
│   │     └── layout/                                                  │
│   ├── gql/             (codegen'd types + typed hooks)               │
│   ├── lib/             (pure helpers; no React)                      │
│   └── TanStack Query   (server cache, prefetch, dedup)               │
│                                                                      │
└─────────────┬────────────────────────────────────────────────────────┘
              │  /graphql, /api/v2/*, /api/v1/*, /auth/*
              ▼
┌─────────────────────────────────────────────────────────────────────┐
│ Go binary (cmd/fern-platform)                                        │
│   ├── internal/api/v1/   (frozen — for client libraries)            │
│   ├── internal/api/v2/   (new — paginated, filtered, faceted)       │
│   ├── internal/reporter/graphql/  (gqlgen, additive evolution)      │
│   ├── internal/domains/  (DDD core — unchanged by this spec)        │
│   └── internal/web/embed.go   (//go:embed web/dist; SPA fallback)   │
└─────────────────────────────────────────────────────────────────────┘
```

## 2. Frontend stack

| Concern | Choice | Pinned version |
|---|---|---|
| Framework | React | 18.x |
| Language | TypeScript (strict) | 5.x |
| Build | Vite | 5.x |
| Router | TanStack Router (default) or React Router 6 (fallback) | latest |
| Server-state | TanStack Query | 5.x |
| GraphQL transport | `graphql-request` | 6.x |
| GraphQL codegen | `@graphql-codegen/cli` + typescript-react-query | latest |
| Styling | Tailwind CSS + shadcn/ui primitives | 3.x |
| Icons | lucide-react | latest |
| Charts | d3 (kept), wrapped in React | 7.x |
| Virtualization | `@tanstack/react-virtual` (lists), `react-arborist` (trees) | latest |
| Date | date-fns | 3.x |
| Forms | react-hook-form + zod | latest |
| Unit tests | Vitest + React Testing Library + MSW | latest |
| E2E tests | Playwright | latest |
| Lint/format | eslint, prettier | latest |
| Bundle budgets | `size-limit` | latest |
| Package manager | pnpm | 9.x |

## 3. Repository layout (deltas)

```
web/                              # REPLACED
├── src/
│   ├── app/                      # routes (file-based)
│   ├── features/                 # domain-scoped composites + hooks
│   ├── components/
│   │   ├── ui/                   # shadcn
│   │   ├── filters/              # FilterBar, FacetSelect, DateRangePicker, ...
│   │   ├── charts/               # Treemap, TrendLine
│   │   └── layout/
│   ├── gql/
│   │   ├── schema.graphql        # mirrored from server (make sync-schema)
│   │   ├── operations/           # hand-authored *.graphql
│   │   └── generated.ts          # codegen'd, committed
│   ├── lib/                      # pure helpers
│   └── main.tsx
├── public/
├── tests/                        # Playwright
├── index.html                    # ~20 lines
├── package.json
├── pnpm-lock.yaml
├── tsconfig.json
├── vite.config.ts
├── tailwind.config.ts
├── codegen.ts
└── perf-budgets.json

internal/
├── api/
│   ├── v1/                       # MOVED here (existing handlers)
│   └── v2/                       # NEW (paginated, filtered)
└── web/
    └── embed.go                  # NEW

migrations/
└── NNN_filter_indexes.up.sql     # NEW (composite + GIN indexes)
                                   # + matching .down.sql

Dockerfile                        # multi-stage: node → go (consolidated)
Dockerfile.simple                 # DELETED
Dockerfile.goreleaser             # DELETED
```

## 4. Filter and pagination contract

### 4.1 GraphQL

Existing list queries are extended with optional `filter` and `page` inputs.
Backwards compatibility: omitting both yields existing behavior with default
pagination.

```graphql
enum LogicMode { AND, OR }

input IntRange      { gte: Int,    lte: Int }
input DateTimeRange { gte: Time,   lte: Time }

input TestRunFilter {
  projectIds: [ID!]
  status:     [TestRunStatus!]
  branches:   [String!]
  tags:       [String!]
  tagMode:    LogicMode = OR
  gitCommit:  String
  authors:    [String!]
  durationMs: IntRange
  startedAt:  DateTimeRange
  search:     String                # FTS over name + error_message
}

input ConnectionArgs {
  first: Int = 50                   # max 200; server clamps
  after: String                     # opaque base64 cursor
}

type TestRunFacets {
  byStatus:  [FacetCount!]!
  byBranch:  [FacetCount!]!
  byTag:     [FacetCount!]!
  byProject: [FacetCount!]!
}
type FacetCount { value: String!, count: Int! }

type TestRunConnection {
  edges:      [TestRunEdge!]!
  pageInfo:   PageInfo!
  totalCount: Int!                  # estimated for broad filters
  facets:     TestRunFacets!
}

extend type Query {
  testRuns(filter: TestRunFilter, page: ConnectionArgs): TestRunConnection!
}
```

Deprecation: the legacy `testRuns(projectId, first)` signature is preserved
via overloaded resolver behavior — passing `projectId` is equivalent to
`filter: { projectIds: [projectId] }`. The old args are marked
`@deprecated(reason: "use filter")`.

### 4.2 REST v2

```
GET /api/v2/test-runs
  ?project=<id>&project=<id>            # repeatable = OR within field
  &status=failed&status=flaky
  &branch=main
  &tag=smoke&tag=release&tagMode=AND
  &from=2026-05-01T00:00:00Z
  &to=2026-05-14T23:59:59Z
  &q=oauth%20redirect
  &first=50
  &after=eyJpZCI6...                    # opaque cursor
```

Response shape mirrors the GraphQL connection:

```json
{
  "edges": [
    { "cursor": "eyJpZCI6...", "node": { "id": "...", "status": "failed", ... } }
  ],
  "pageInfo": {
    "hasNextPage": true,
    "endCursor": "eyJpZCI6..."
  },
  "totalCount": 12448,
  "totalCountIsEstimate": true,
  "facets": {
    "byStatus":  [{ "value": "failed", "count": 412 }, ...],
    "byBranch":  [...],
    "byTag":     [...],
    "byProject": [...]
  }
}
```

v2 list endpoints (initial set):

```
GET /api/v2/projects
GET /api/v2/test-runs
GET /api/v2/test-runs/{id}
GET /api/v2/test-runs/{id}/specs
GET /api/v2/flaky-tests
GET /api/v2/tags
GET /api/v2/me/saved-views
POST /api/v2/me/saved-views
DELETE /api/v2/me/saved-views/{id}
```

### 4.3 Cursors

Cursors are opaque base64-encoded JSON: `{"id": "<uuid>", "ts": "<RFC3339>"}`.
The repo uses `WHERE (started_at, id) < (cursor.ts, cursor.id) ORDER BY
started_at DESC, id DESC LIMIT first+1` for stable pagination under inserts.
Clients **must not** parse cursors.

### 4.4 Total-count strategy

```
if filter is narrow (any of: project, branch, status, tag, search):
    SELECT COUNT(*) exact; totalCountIsEstimate = false
else:
    SELECT reltuples::bigint FROM pg_class WHERE relname='test_runs';
    totalCountIsEstimate = true
```

Threshold for "narrow" is heuristic; encoded in
`internal/domains/testing/infrastructure/count_strategy.go`.

### 4.5 Facet counts

One grouped query per facet, executed in parallel via `errgroup` goroutines,
scoped to the same `WHERE` clause minus the facet's own field. Results
cached in-process with key `facets:test-runs:<sha256(filter-without-facet)>`,
TTL 5 min. The shared `RedisLike` interface lets a Redis-backed adapter
plug in for multi-replica deploys; the default in-memory adapter is
sufficient for single-replica installs.

`byStatus`, `byBranch`, and `byProject` always run. `byTag` is opt-in via
the filter's `IncludeTagFacet` flag (REST: `?facets=tag`) because the
suite_runs join is expensive at scale — see §4.6.

### 4.6 Operational notes

These behaviours are not directly visible in the request/response shape
but are load-bearing for the v2 deployment story.

**Default 7-day window on unscoped queries.** When `GET /api/v2/test-runs`
is called without `from`/`to`, the handler clamps `StartedAt.Gte` to
`now - 7d`. At 1000 projects × 100 runs/day, this drops the working set
for `COUNT(*)` + facets + the keyset scan from ~18M rows to ~700k. The
window matches typical triage scope (the last few days of activity is
what users actually scroll through from the dashboard). Wider views
are explicit — UI presets jump to 30d/90d, or clients can opt out
entirely with `?allTime=1`.

**Trends aggregate endpoint.** The Test Summaries page renders one
trend card per visible project. The original implementation made one
`/api/v2/test-runs?project=…` call per card — at 100 visible projects
that's 100 parallel requests, each driving COUNT(\*) + facets + keyset.
The new `GET /api/v2/test-runs/trends?project=…&project=…&days=N`
endpoint returns per-(project, day) sums in a single response,
backed by a single `GROUP BY (project_id, date_trunc('day', start_time))`
SQL aggregate. Response shape:

```json
{
  "from": "...", "to": "...", "days": 7,
  "buckets": {
    "<projectId>": [
      { "day": "YYYY-MM-DD", "totalRuns": N, "totalTests": N,
        "passedTests": N, "failedTests": N, "skippedTests": N,
        "durationMs": N }
    ]
  }
}
```

A 60-second in-process cache keyed by `(userID, sorted projectIDs, days)`
sits in front of it, same shape as the treemap cache. The cache hides
rapid navigation back to the page.

**Test Summaries v1 parity (FR-17a/b/c/d).** The per-card stat triplet on
`/v2/test-summaries` aligns with v1: **Total tests / Passed / Failed**,
summed client-side from the same `trends.buckets[projectId]` rows already
on hand. No second fetch per card. A **View history** link on each card
deep-links into the project detail view's history tab for the same window,
reusing existing routes. A page-level **Card view ↔ Tree view** toggle
switches between the trend-card grid and the existing `<Treemap>`
component; tree mode hits `/treemap` (already cached for 60s) and shares
the page's window selector. The chosen mode persists in
`localStorage` (`fern-v2.summaries.viewMode`); a follow-up will roundtrip
it via `userPreferences.preferences` so it travels across devices.
No new endpoint, no new aggregate. The whole feature is purely a frontend recomposition of data
the server already returns.

**Tag-facet lazy loading.** `byTag` requires joining `test_runs →
suite_runs → suite_run_tags → tags` and GROUP-BYing across all matched
suite_runs. On a populated DB this is 10-15 seconds cold. The default
list response skips it; the `<FilterSidebar>` renders a "Load tag facet"
button that sets `?facets=tag` on demand. When the user already has
tags selected, the front-end auto-opts in so the visible counts stay
consistent.

**Substring search uses `pg_trgm`, not English FTS.** Test output and
stack traces are camelCase identifiers, not English prose. The English
dictionary tokenizes `DataIntegrityViolationException` as a single
stemmed token (`dataintegrityviolationexcept`), so searches for
`DataIntegrity` returned zero rows. The implementation uses a GIN
`gin_trgm_ops` index over `COALESCE(spec_name,'') || ' ' ||
COALESCE(error_message,'')` and queries via `ILIKE '%<q>%'` — index-backed
substring matching that does what users expect when typing fragments of
a stack trace.

**Treemap aggregates run in SQL, with a 60-second result cache.** The
`/treemap` GraphQL resolver returns project-level (top view) or
suite-level (drill view) sums computed via `AggregateProjectsInRange` /
`AggregateSuitesInRange` repository methods — one `GROUP BY` query each,
not millions of hydrated rows. Results are cached in-process for 60s
keyed by `(userID, drillProjectID, days)` since the treemap is a
glanceable dashboard hit repeatedly. User scoping is required because
access filtering varies between admins and team members.

**Cursor pagination is keyset.** The cursor encodes `ts=<unix_ns>&id=<row>`;
`fetchPage` adds `(start_time, id) < (?, ?)` to the WHERE and orders by
`(start_time DESC, id DESC)`. The composite index added by migration
000022 covers both the predicate and the order. Malformed cursors return
400 (not a silent fallback to page 1, which was the original bug).

## 5. Database changes

All v2 schema changes live in a single migration:
`migrations/000022_v2_schema.up.sql`. Forward-only, idempotent
(`IF NOT EXISTS` everywhere); no column is renamed or dropped from
the v1 schema.

```sql
-- Filtered-list path
CREATE INDEX IF NOT EXISTS idx_test_runs_project_started_desc
    ON test_runs (project_id, start_time DESC);

CREATE INDEX IF NOT EXISTS idx_test_runs_keyset
    ON test_runs (start_time DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_test_runs_failed_started
    ON test_runs (start_time DESC)
    WHERE status IN ('failed', 'flaky');

CREATE INDEX IF NOT EXISTS idx_test_runs_project_branch
    ON test_runs (project_id, branch);

-- Substring search (see §4.6 — pg_trgm, not FTS)
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX IF NOT EXISTS idx_spec_runs_search_trgm
    ON spec_runs
    USING GIN (
        (COALESCE(spec_name, '') || ' ' || COALESCE(error_message, ''))
        gin_trgm_ops
    );

-- Saved views (user-scoped filter presets)
CREATE TABLE IF NOT EXISTS saved_views (
    id          BIGSERIAL PRIMARY KEY,
    user_id     VARCHAR(255) NOT NULL
                REFERENCES users(user_id) ON DELETE CASCADE,
    page        VARCHAR(64)  NOT NULL,
    name        VARCHAR(255) NOT NULL,
    filter_json JSONB        NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT saved_views_unique_per_user UNIQUE (user_id, page, name)
);
CREATE INDEX IF NOT EXISTS idx_saved_views_user_page
    ON saved_views (user_id, page);
```

`CREATE INDEX` is not wrapped in `CONCURRENTLY` here because
golang-migrate wraps each migration in a transaction by default and
`CONCURRENTLY` is incompatible with transactions. Instead, large-DB
operators run `scripts/v2-preflight-indexes.sh` before deploying — it
issues `CREATE INDEX CONCURRENTLY IF NOT EXISTS` against the live
database, and the startup migration then becomes a no-op via the
`IF NOT EXISTS` guards above. See
[`migration-guide.md` §4](./migration-guide.md) for the rollout
procedure and a recovery procedure for dirty migrations.

Matching `.down.sql` drops the table and indexes (the `pg_trgm`
extension is intentionally left in place since other code may depend
on it being installed).

## 6. Server-side handler layout

`internal/api/v2/test_run_handler.go`:

```go
type TestRunV2Handler struct {
    svc testing.TestRunQueryService     // application service
    log *zap.Logger
}

func (h *TestRunV2Handler) List(c *gin.Context) {
    f, err := decodeFilter(c.Request.URL.Query())   // strict, validated
    if err != nil { /* 400 */ }
    page, err := decodePage(c.Request.URL.Query())  // clamps first ≤ 200
    if err != nil { /* 400 */ }

    res, err := h.svc.Query(c.Request.Context(), f, page)
    if err != nil { /* 500, logged */ }

    c.JSON(http.StatusOK, toConnectionDTO(res))
}
```

Application service signature:

```go
// internal/domains/testing/application/test_run_query_service.go
type TestRunQueryService interface {
    Query(ctx context.Context, f domain.TestRunFilter, p domain.PageArgs) (domain.TestRunPage, error)
}
```

Domain types (filter, page args, page result) live in
`internal/domains/testing/domain/`. Infrastructure layer translates
`domain.TestRunFilter` → SQL.

## 7. v1 freeze policy

Files moved from `internal/api/*.go` → `internal/api/v1/*.go` without
behavior change. Route registration in `main.go`:

```go
v1 := router.Group("/api/v1")
v1.Use(deprecationHeaders("2027-05-14", "https://docs.fern/migrate-v2"))
v1registry.Register(v1, deps)

v2 := router.Group("/api/v2")
v2registry.Register(v2, deps)
```

`deprecationHeaders` middleware emits:

```
Deprecation: true
Sunset: Fri, 14 May 2027 00:00:00 GMT
Link: <https://docs.fern/migrate-v2>; rel="deprecation"
```

The existing acceptance suite for v1 runs unchanged in CI; **any v1
behavioral change is a regression**.

## 8. Embedding into Go binary

`internal/web/embed.go`:

```go
package web

import (
    "embed"
    "io/fs"
    "net/http"
    "strings"

    "github.com/gin-gonic/gin"
)

//go:embed all:dist
var dist embed.FS

func Register(r *gin.Engine) error {
    sub, err := fs.Sub(dist, "dist")
    if err != nil { return err }

    fs := http.FS(sub)
    fileServer := http.FileServer(fs)

    r.GET("/assets/*filepath", gin.WrapH(fileServer))
    r.NoRoute(func(c *gin.Context) {
        // never swallow API/GraphQL/auth routes
        p := c.Request.URL.Path
        if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/graphql") ||
           strings.HasPrefix(p, "/auth/") {
            c.AbortWithStatus(http.StatusNotFound)
            return
        }
        c.FileFromFS("index.html", fs)
    })
    return nil
}
```

A `go:build !dev` tag is used so `make dev` can serve from disk to enable
HMR proxying.

## 9. Component design — `<FilterBar>`

```tsx
type FilterBarProps<F> = {
  schema: FilterSchema<F>;       // declarative: fields + their primitive types
  value: F;
  onChange: (next: F) => void;
  facets?: Facets;               // optional, drives counts
  savedViews?: SavedView<F>[];
};
```

The schema declaration drives both the rendered primitives and the URL
codec, so adding a filter is a one-place change:

```ts
export const testRunFilterSchema: FilterSchema<TestRunFilter> = {
  search:     { kind: 'text',     param: 'q',       debounceMs: 250 },
  projectIds: { kind: 'facet',    param: 'project', multi: true },
  status:     { kind: 'enum',     param: 'status',  options: STATUSES },
  branches:   { kind: 'facet',    param: 'branch',  multi: true },
  tags:       { kind: 'tag',      param: 'tag',     multi: true, modeParam: 'tagMode' },
  startedAt:  { kind: 'dateRange', paramFrom: 'from', paramTo: 'to', default: '7d' },
};
```

`useUrlFilter(schema)` is a hook that:

- reads query params on mount,
- writes them back on change (`router.navigate`, `replace: true`),
- returns `[filter, setFilter]`.

## 10. Virtualization design

- **`<TestRunTable>`** uses `useVirtualizer` from `@tanstack/react-virtual`
  with `overscan: 8` and `estimateSize: () => 56`. Container is the page
  scroll, not nested.
- **`<SpecTree>`** uses `react-arborist`, lazy-loading children at >100 leaves
  per parent.
- **`<Treemap>`** renders ≤ 500 SVG nodes; clicking a tile fetches the next
  level via `useGetTreemapNodeQuery({ parentId })`. Top-level is fetched at
  page load with `staleTime: 60_000`.

## 11. Auth and `<AuthGate>`

```tsx
function AuthGate({ children }: { children: ReactNode }) {
  const { data, isLoading, isError } = useMeQuery({ retry: false });
  if (isLoading) return <FullPageSkeleton />;
  if (isError)   { redirectToLogin(); return <FullPageSkeleton />; }
  return <UserContext.Provider value={data.me}>{children}</UserContext.Provider>;
}
```

The skeleton is rendered server-side via the static `index.html` shell, so
the user never sees the error flash from #164.

**TopBar identity surface (FR-24a/b/e, shipped as T1.4a).** A small
`useCurrentUser` hook in `features/auth/` fronts the same `currentUser`
GraphQL field v1 uses (a separate, minimal projection from the Profile
page's query so the chrome request stays cheap and cache-distinct). The
hook treats 401/403 as "no user" rather than a hard error, so the chrome
shows a Sign-in button instead of a red banner before the user has
logged in. The TopBar pill is a v1-style dropdown menu: avatar → initials
fallback → name → role badge in the trigger; a card with name / email /
role / Profile link / Admin Panel link (admin-only) / Sign-out drops
below on click. The menu toggles on repeat click and closes on outside
click or `Escape`. Sign-out posts to `/auth/logout` and follows the
returned `logout_url` to the IdP's end-session endpoint (mirrors v1's
flow); on any failure it falls back to `/auth/login`. The synthetic
`dev-admin` principal (`AUTH_ENABLED=false`) keeps the amber "Local
dev" banner and hides Sign-out — there's no provider session to
terminate. This is the chrome surface only; the full route-level
`<AuthGate>` (forcing a redirect on 401 for protected pages) is still
parked as T1.4.

**Profile page schema discipline (FR-24c/d).** GraphQL selections must
match the live schema — gqlgen rejects the entire query on an unknown
field, which is how the Profile page silently broke when it asked for
a non-existent `User.emailVerified`. The page now requests only the
fields defined on `type User`, with `groups` surfaced alongside the
role badge for SSO parity with v1. The timezone picker uses the full
`Intl.supportedValuesOf('timeZone')` list with the browser-detected
zone first followed by UTC, replacing the prior arbitrary 80-zone
slice that hid every region from `Asia/*` onward.

**Treemap consolidation + spec-level drill (FR-16 revised, shipped as
T3.7d).** The standalone `/v2/treemap` route was removed — the same
`TreemapView` component now lives only inside the Summaries page's
Tree view toggle. Three drill levels exist: top shows projects (area
= total tests, color = pass rate); clicking a project drills into
suites (area = total specs, color = pass rate); clicking a suite
drills into specs (area = duration, color = pass/fail/skipped status).
The breadcrumb at the top of the treemap (`Projects / <project> /
<suite>`) lets the user jump back to any level. Spec aggregation is
backed by a new `AggregateSpecsForSuiteInRange` repository method
that joins `spec_runs → suite_runs → test_runs` with `ORDER BY
duration DESC LIMIT 500` — the cap matches FR-16's 500-node treemap
budget and biases toward the most expensive specs (the ones worth
seeing on the tile map). Flaky specs carry an `isFlaky` flag through
the resolver so the tile-renderer can dim them distinctly from
steady-failing ones. **Color uses a continuous `passRate`
(`PassedRuns / TotalRuns`) at every drill level**, not a binary
status-string-to-color mapping; a spec that passes 99/100 runs reads
as the same green as its 99% suite, not bright red because of one
failure. The status string (`passed`/`failed`/`skipped`) is
preserved as a majority-outcome label/badge, not as the color
driver. `SpecTreemapNode` therefore exposes both `status` and the
run-count breakdown (`totalRuns`/`passedRuns`/`failedRuns`/
`skippedRuns`/`passRate`).

**Manager dashboard restoration (FR-17f/g/h, shipped as T3.7c).** v1
had a dedicated `/manager-dashboard` route that bookmarked the
"my team's projects, treemap view" landing page. v2 removed it during
the consolidation, on the theory that filter-driven Summaries covered
the workflow. The bookmark loss was a real regression for managers, so
the route is restored as a **thin redirect**, not a new view: hitting
`/v2/manager-dashboard` checks the user's role; manager/admin land at
`/v2/summaries?view=tree&favoritesOnly=true` (`replace: true` so the
browser back button doesn't bounce). Anyone else sees a "Manager
access only" empty state. The route component
(`ManagerDashboardRedirect.tsx`) holds no rendering logic beyond the
gate. `TestSummaries` honors `view` and `favoritesOnly` URL search
params on first mount, taking precedence over its sessionStorage
persistence — so the deep-link lands in the intended state even if the
user previously left Summaries in Card view. Sidebar items now carry
an optional `requires: Array<'admin' | 'manager'>` field; entries
without it are visible to all signed-in users, and Sidebar drops
sections that filter down to zero items. `canSeeNavItem` is exported
so role-gating is unit-testable without mounting React Query.

**Projects pagination consolidation (FR-15a, shipped as T3.1b).** Three
pages — Projects, Test Summaries, Dashboard — each had their own
`projects(first: 100)` GraphQL query with a hard 100-row cap. The cap
was invisible to users but produced a real inconsistency: the treemap
aggregates run via SQL `GROUP BY project_id` (no cap until FR-16's 500
ceiling), so projects 101+ would appear on `/v2/treemap` but vanish
from `/v2/projects`, `/v2/summaries`, and the Dashboard. A single
`useProjects` hook now uses `useInfiniteQuery` to walk
`projects(first, after)` cursor pages (100 per page) until the server
reports no next page, then exposes a flat `Project[]` for callers to
filter / sort / render. Auto-fetch is bounded by `MAX_PROJECTS = 500`
to match FR-16's treemap node cap, so the two views stay consistent.
`/v2/projects` surfaces a truncation banner when the cap clips the
list — users can still find the rest by narrowing filters (search,
team, favorites). The page-flattening + cap logic lives in
`pagedProjects.ts` so it can be unit-tested without mounting React
Query.

**`language` preference is persisted but currently cosmetic.** The
`UserPreferences.language` GraphQL field, the matching DB column, and
the Profile-page Language dropdown are wired end-to-end (mutation
writes, query reads round-trip). No code in either v1 or v2 actually
consumes the value — there is no i18n library, no message catalogs,
and the Settings → Language panel hardcodes `en-US`. The field is
deliberately retained as scaffolding for a future i18n effort so that
work won't require a schema migration when it lands; for now the
picker is a no-op the user can change without effect. Removing it
would be a breaking change to the user-preferences contract, so it
stays.

## 12. Build, dev, CI

### 12.1 Makefile targets (new)

```makefile
web-deps:
	cd web && pnpm install --frozen-lockfile

web-codegen: web-deps sync-schema
	cd web && pnpm codegen

web-dev: web-deps
	cd web && pnpm dev

web-build: web-deps web-codegen
	cd web && pnpm typecheck && pnpm lint && pnpm test --run && pnpm build

web-test-e2e:
	cd web && pnpm playwright test

sync-schema:
	cp $$(pwd)/internal/reporter/graphql/schema.graphql web/src/gql/schema.graphql

build: web-build
	go build -o bin/fern-platform ./cmd/fern-platform
```

### 12.2 CI pipeline

```
node-job:
  - pnpm install
  - pnpm typecheck
  - pnpm lint
  - pnpm test --run --coverage
  - pnpm build
  - size-limit                    # bundle budget gate
  - upload web/dist artifact

go-job (needs node-job):
  - download web/dist artifact
  - go build, go test, go vet, govulncheck
  - build container image

e2e-job (needs go-job):
  - deploy stack (k3d ephemeral)
  - playwright test
  - axe-core a11y assertions
```

### 12.3 Local dev

```
Terminal 1: make dev          # Go server :8080, live reload
Terminal 2: make web-dev      # Vite :5173, proxies /graphql, /api, /auth → :8080
```

Developer browses `http://localhost:5173` for HMR; auth flow uses the same
Keycloak at `http://keycloak:8080`.

## 13. Dockerfile (consolidated)

```dockerfile
FROM node:20-alpine AS web
WORKDIR /web
RUN corepack enable
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm typecheck && pnpm test --run && pnpm build

FROM golang:1.25 AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /web/dist ./web/dist
RUN CGO_ENABLED=0 go build -o /out/fern-platform ./cmd/fern-platform

FROM gcr.io/distroless/base-debian12
COPY --from=server /out/fern-platform /fern-platform
EXPOSE 8080
ENTRYPOINT ["/fern-platform"]
```

Existing `Dockerfile.simple` and `Dockerfile.goreleaser` are deleted.

## 14. Observability

- Each v2 handler emits a `fern.v2.<endpoint>` metric tagged with
  `status_class`, `filter_kind` (broad/narrow), and duration histogram.
- Slow query log: any list query > 500 ms is logged with the filter JSON
  for diagnosis.
- A `/metrics` Prometheus endpoint exposes counts and histograms.
- The frontend reports Core Web Vitals (`web-vitals` lib) to a
  `/api/v2/telemetry/vitals` sink; sampled 10 %.

## 15. Security

- CSP header on HTML responses:
  ```
  Content-Security-Policy: default-src 'self'; script-src 'self';
    style-src 'self' 'unsafe-inline'; img-src 'self' data:;
    connect-src 'self';
  ```
- No CDNs at runtime — bundled fonts (Inter) shipped in `dist/`.
- Saved views API requires the user's session; `user_id` is server-derived,
  not client-supplied.
- `cursor` payloads are HMAC-signed to prevent enumeration of internal IDs.

## 16. Rollback plan

- The new UI is mounted at `/v2/*` and gated by an env flag
  `FERN_V2_UI_ENABLED`. Setting it false hides v2 routes.
- v2 API endpoints are independent of v1 — disabling them does not affect
  client libraries.
- The new indexes are `CREATE INDEX CONCURRENTLY`; safe to drop without
  blocking writes.

## 17. Decision log

| ID | Decision | Rationale |
|---|---|---|
| D-1 | Stay on React, not switch frameworks | Existing code is React; switching adds risk without payoff |
| D-2 | TanStack Router over React Router 6 | Type-safe routes match the TS-first ethos |
| D-3 | Cursor pagination over offset | Stable under writes; test runs are append-heavy |
| D-4 | New REST `/api/v2`, GraphQL evolves additively | REST clients are external (CI integrations); GraphQL is internal |
| D-5 | Embed frontend in Go binary | Single artifact preserves current deploy story |
| D-6 | Tailwind over CSS-in-JS | No runtime cost; JIT removes unused utilities |
| D-7 | shadcn over Material/Mantine | Components live in our repo; no vendor lock-in |
| D-8 | pnpm over npm | Faster, smaller node_modules, deterministic |
| D-9 | Strangler (`/v2/`) over big-bang | Always shippable; reversible at any phase |

## 18. References

- [Requirements](./requirements.md)
- [Tasks](./tasks.md)
- [RFC-004](../../rfc/rfc-004-frontend-modernization.md)
- [RFC 8594 — Sunset HTTP Header](https://www.rfc-editor.org/rfc/rfc8594)
- TanStack Query, TanStack Router docs
- GraphQL Code Generator docs
