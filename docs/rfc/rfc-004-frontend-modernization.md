# RFC-004: Frontend Modernization

**Status:** Implemented (with deviations — see Appendix B)
**Author:** (proposer)
**Created:** 2026-05-14
**Last Updated:** 2026-05-17
**Tracking issue:** TBD

## Abstract

The Fern Platform UI is currently delivered as a single 8,787-line `web/index.html`
file that loads React, ReactDOM, and Babel-standalone from a CDN and transpiles
JSX in the browser at runtime. There is no build step, no type system, no module
bundler, and no component-level test harness. This RFC proposes migrating to a
modern frontend stack — **React 18 + TypeScript + Vite + GraphQL Codegen +
TanStack Query + Tailwind + Playwright** — embedded into the Go binary via
`embed.FS`, with an incremental "strangler" migration that keeps the legacy UI
shippable until parity is reached.

## Table of Contents

1. [Background and Current State](#background-and-current-state)
2. [Problem Statement](#problem-statement)
3. [Goals and Non-Goals](#goals-and-non-goals)
4. [Proposal](#proposal)
5. [Detailed Design](#detailed-design)
6. [Migration Strategy](#migration-strategy)
7. [Build, Release, and Serving](#build-release-and-serving)
8. [Testing Strategy](#testing-strategy)
9. [Alternatives Considered](#alternatives-considered)
10. [Risks and Mitigation](#risks-and-mitigation)
11. [Success Metrics](#success-metrics)
12. [Implementation Plan](#implementation-plan)
13. [Open Questions](#open-questions)

---

## Background and Current State

### What we have today

```
web/
├── index.html              8,787 lines — markup, inline styles, inline React/JSX
├── css/font-awesome.min.css
├── fonts/
├── images/
└── js/
    ├── graphql-client.js          528 lines — hand-rolled fetch wrapper
    ├── graphql-client-test.js     124 lines
    ├── duration-utils.js           83 lines
    ├── duration-utils-test.js     198 lines
    └── timestamp-component.js     288 lines
```

Key observations from reading `web/index.html`:

- React 18, ReactDOM, and `@babel/standalone` are loaded from `unpkg.com` at
  runtime. JSX is transpiled in the user's browser on every page load.
- D3 v7 is loaded the same way for the treemap visualization.
- There is no module system; scripts are global and ordering-dependent
  (`?v=5`, `?v=2`, `?v=1` cache-busters appear in the HTML).
- A FontAwesome failure fallback hardcodes Unicode emoji substitutions inline.
- A duplicate `formatDuration` implementation is inlined as a "fallback" in
  case `duration-utils.js` fails to load.
- Component state, GraphQL queries, routing, and styles are all interleaved
  in one file.

### How it is served

The Go binary serves `web/` as static files from disk (see `cmd/fern-platform/main.go`
and `pkg/middleware/`). The directory is not embedded — production deployments
must ship the `web/` tree alongside the binary.

### Why this hurts

Recent UI-related fixes give the texture of the problem:

- `#144` ("load a single tab at a time to improve UI perf") — perf regression.
- `#154` / `#156` ("make summary tag display more meaningful data") — display bug.
- `#164` / `#165` ("suppress error flash before SSO redirect") — race condition.
- `#170` ("prevent proxy from caching the OAuth login redirect").

Every one of these required hand-editing the 8.7 KLOC monolith with no type
checker, no component isolation, and no automated regression coverage.

---

## Problem Statement

1. **No type safety.** GraphQL responses are `any`; field renames silently break
   the UI and are only caught by users.
2. **No build-time validation.** JSX errors surface as blank screens in
   production.
3. **Runtime Babel transpilation** adds ~150–300 ms to first paint and pulls
   ~3 MB of JS through `unpkg.com` on a cold cache.
4. **Third-party CDN dependency** is an availability and supply-chain risk
   (unpkg outage = product outage; a compromised package = XSS in every
   browser).
5. **No componentization.** Reuse happens by copy-paste; bug fixes must be
   applied in multiple places.
6. **No frontend test harness.** `*-test.js` files exist but only cover the
   two utility modules; none of the UI is covered.
7. **Onboarding cliff.** New contributors must read an 8.7 KLOC HTML file to
   change a button.
8. **Accessibility and i18n are unaddressed** — no semantic component layer
   to anchor improvements.

---

## Goals and Non-Goals

### Goals

- **G1.** Replace runtime-transpiled JSX with a real build pipeline producing
  hashed, minified, tree-shaken bundles.
- **G2.** Introduce TypeScript end-to-end with codegen-driven types from the
  GraphQL schema.
- **G3.** Decompose the monolith into reusable, individually-testable
  components.
- **G4.** Eliminate runtime CDN dependencies for application code.
- **G5.** Ship the built frontend embedded in the Go binary so deploy remains
  a single artifact.
- **G6.** Establish a Playwright-based E2E suite that runs in CI and gates
  releases.
- **G7.** Do the migration **incrementally** — at no point is the platform
  un-shippable for more than the length of a single PR.

### Non-Goals

- Rewriting the GraphQL schema or REST API.
- Changing the visual design (we keep parity first, redesign second).
- Server-side rendering, micro-frontends, or framework pluralism.
- Mobile-native clients.

---

## Proposal

### Stack

| Concern | Choice | Why |
|---|---|---|
| Framework | **React 18** | Already in use; keep cognitive load low. |
| Language | **TypeScript (strict)** | Catch GraphQL drift and refactors at compile time. |
| Build | **Vite 5** | Fast HMR, ESM-native, sane defaults, small config. |
| Routing | **TanStack Router** (or React Router 6) | Type-safe routes, code-split per route. |
| Data | **TanStack Query + graphql-request** | Cache, retries, devtools; pairs cleanly with codegen. |
| Codegen | **GraphQL Code Generator** (`@graphql-codegen/*`) | Types + typed hooks from `gqlgen.yml` schema. |
| Styling | **Tailwind CSS** + **shadcn/ui** primitives | Utility-first; no CSS-in-JS runtime; accessible primitives. |
| Charts | **D3** (kept) wrapped in React components | Treemap already invested in D3. |
| Icons | **lucide-react** | Tree-shakable; replaces FontAwesome + emoji fallback hack. |
| Tests | **Vitest** (unit), **Playwright** (E2E) | Vite-native; Playwright already implied by `acceptance/`. |
| Lint/format | **eslint** + **prettier** + **tsc --noEmit** in CI | Standard. |
| Package manager | **pnpm** | Fast, deterministic, monorepo-friendly if we ever split. |

### Repository layout

```
fern-platform/
├── web/                          # NEW frontend project (replaces existing web/)
│   ├── src/
│   │   ├── app/                  # Route components (file-based or table)
│   │   │   ├── dashboard/
│   │   │   ├── projects/
│   │   │   ├── test-runs/
│   │   │   └── flaky/
│   │   ├── components/           # Reusable UI primitives + composites
│   │   │   ├── ui/               # shadcn/ui primitives (Button, Dialog, ...)
│   │   │   ├── charts/           # Treemap, TrendLine, ...
│   │   │   └── layout/
│   │   ├── features/             # Feature-scoped components + hooks
│   │   │   ├── test-runs/
│   │   │   ├── flaky-detection/
│   │   │   └── tags/
│   │   ├── gql/                  # CODEGEN OUTPUT — do not hand-edit
│   │   │   ├── schema.graphql    # mirrored from server
│   │   │   ├── operations/       # .graphql files (queries/mutations)
│   │   │   └── generated.ts      # types + typed hooks
│   │   ├── lib/                  # Cross-cutting helpers (auth, time, utils)
│   │   ├── styles/               # tailwind.css + globals
│   │   └── main.tsx              # entry
│   ├── public/                   # Static assets (favicon, images)
│   ├── tests/                    # Playwright specs
│   ├── index.html                # 20-line shell, not 8,787
│   ├── package.json
│   ├── pnpm-lock.yaml
│   ├── tsconfig.json
│   ├── vite.config.ts
│   ├── tailwind.config.ts
│   └── codegen.ts                # graphql-codegen config
│
├── web/dist/                     # Vite build output (gitignored)
└── internal/web/
    └── embed.go                  # //go:embed web/dist + Gin handler
```

### High-level architecture

```
┌─────────────────────────────────────────────────────────────────┐
│ Browser                                                          │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ React 18 SPA (TypeScript, Tailwind)                       │  │
│  │  ┌────────────┐  ┌─────────────────┐  ┌────────────────┐ │  │
│  │  │ Router     │→ │ Feature modules │→ │ UI primitives  │ │  │
│  │  └────────────┘  └────────┬────────┘  └────────────────┘ │  │
│  │                           ▼                                │  │
│  │                  ┌────────────────────┐                    │  │
│  │                  │ TanStack Query +   │                    │  │
│  │                  │ generated GraphQL  │ ◀── codegen ───┐   │  │
│  │                  │ typed hooks        │                │   │  │
│  │                  └─────────┬──────────┘                │   │  │
│  └────────────────────────────┼────────────────────────────┼──┘  │
└─────────────────────────────── │ ───────────────────────────┼─────┘
                                 ▼                            │
                  ┌──────────────────────────────┐            │
                  │ Go binary (cmd/fern-platform)│            │
                  │  /graphql  /api/v1/*  /*     │ ◀──schema──┘
                  │  serves embedded web/dist    │
                  └──────────────────────────────┘
```

---

## Detailed Design

### 1. Build pipeline

`web/vite.config.ts`:

```ts
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'node:path';

export default defineConfig({
  plugins: [react()],
  resolve: { alias: { '@': path.resolve(__dirname, 'src') } },
  build: {
    outDir: 'dist',
    sourcemap: true,
    rollupOptions: {
      output: {
        manualChunks: {
          react: ['react', 'react-dom'],
          d3: ['d3'],
        },
      },
    },
  },
  server: {
    proxy: {
      '/api':     'http://localhost:8080',
      '/graphql': 'http://localhost:8080',
      '/auth':    'http://localhost:8080',
    },
  },
});
```

**Outcome:** `pnpm dev` runs Vite at `:5173` and proxies API calls to the Go
server on `:8080`. `pnpm build` produces hashed assets in `web/dist/`.

### 2. GraphQL codegen

The Go server defines the schema (`gqlgen.yml`). The frontend mirrors the
schema file into `web/src/gql/schema.graphql` via a `make sync-schema` target,
then runs codegen:

`web/codegen.ts`:

```ts
import type { CodegenConfig } from '@graphql-codegen/cli';

const config: CodegenConfig = {
  schema: 'src/gql/schema.graphql',
  documents: 'src/gql/operations/**/*.graphql',
  generates: {
    'src/gql/generated.ts': {
      plugins: [
        'typescript',
        'typescript-operations',
        'typescript-react-query',
      ],
      config: {
        fetcher: { endpoint: '/graphql' },
        exposeFetcher: true,
      },
    },
  },
};
export default config;
```

Each `.graphql` operation file produces a typed React Query hook:

```ts
// usage in a component
const { data, isLoading } = useGetTestRunsQuery({ projectId, first: 50 });
//                              ▲ fully typed: variables AND result
```

A schema change at the server forces a recompile failure in the frontend — the
class of bug that produced `#154` becomes a build error.

### 3. Component architecture

Three tiers, strict dependency direction (inner → outer is forbidden):

```
features/*          ← business logic, feature-scoped
   │
   ▼
components/charts, components/layout, components/ui (shadcn)
   │
   ▼
lib/* (pure helpers, no React)
```

Rules:

- `features/` may import from `components/` and `lib/`, never from another
  feature directory.
- `components/` is feature-agnostic. No GraphQL hooks here.
- All data fetching happens in `features/<x>/hooks/` and is passed in via
  props — components do not call hooks for data.

### 4. State management

- **Server state:** TanStack Query (caching, retries, optimistic updates).
- **URL state:** route params + search params (router-owned).
- **Ephemeral UI state:** `useState` / `useReducer`.
- **Cross-cutting client state** (current user, theme): React Context, one
  provider per concern. **No Redux, no Zustand** unless a specific need arises.

### 5. Auth integration

OAuth/Keycloak flow stays server-side. The SPA reads the current user from a
`/api/v1/me` endpoint on mount; 401 triggers redirect to `/auth/login` (the
existing server route). The `error flash before SSO redirect` bug (#164) is
prevented by a top-level `<AuthGate>` that renders a skeleton until auth state
resolves.

### 6. Embedding into the Go binary

`internal/web/embed.go`:

```go
package web

import (
    "embed"
    "io/fs"
    "net/http"

    "github.com/gin-gonic/gin"
)

//go:embed all:dist
var distFS embed.FS

func Register(r *gin.Engine) {
    sub, _ := fs.Sub(distFS, "dist")
    fileServer := http.FileServer(http.FS(sub))

    // Serve hashed assets directly.
    r.GET("/assets/*filepath", gin.WrapH(fileServer))

    // SPA fallback: anything not matched by API routes returns index.html.
    r.NoRoute(func(c *gin.Context) {
        c.FileFromFS("index.html", http.FS(sub))
    })
}
```

**Result:** the binary is still a single artifact. No `web/` directory needs
to be shipped alongside it. `Dockerfile` simplifies.

### 7. Dev server workflow

```
Terminal 1:  make dev              # Go server on :8080 (live reload)
Terminal 2:  cd web && pnpm dev    # Vite on :5173, proxies /graphql + /api → :8080
```

Developer opens `http://localhost:5173`. HMR < 100 ms.

In production, only `:8080` is exposed and the Go binary serves the embedded
bundle.

---

## Filtering, Pagination, and Heavy-Data Handling

The current UI is **data-heavy**: test-run lists run to thousands of rows,
spec trees can have tens of thousands of leaves, and the treemap renders
every suite at once. The legacy implementation loads full datasets eagerly
and filters in JavaScript, which is the root cause of `#144` ("load a single
tab at a time to improve UI perf"). This section is a first-class part of
the design, not an optimization afterthought.

### Principles

1. **Filter on the server, render on the client.** The browser must never
   receive rows it will not display.
2. **Every list view is paginated.** No infinite scroll without virtualization.
3. **Filters are part of the URL.** Sharing a link reproduces the view.
4. **Empty filter state must be cheap** — the default landing query for any
   page must return ≤ 100 rows.

### Filter model

Each list page (Projects, Test Runs, Specs, Flaky Tests, Tags) gets a
**`<FilterBar>`** with a uniform composition:

```
┌─────────────────────────────────────────────────────────────────────┐
│ [Search ⌕]  [Project ▾] [Branch ▾] [Status ▾] [Tags ▾] [Date 📅]    │
│ [More filters…]                              [Clear] [Save view ★]  │
└─────────────────────────────────────────────────────────────────────┘
   Active: project=fern · status=failed · last 7 days        12,448 ↓
```

Components (all from the shared `components/filters/` library):

| Primitive | Use |
|---|---|
| `<SearchInput>` | Free-text, debounced 250 ms, server-side ILIKE / full-text |
| `<FacetSelect>` | Multi-select with counts from the server (e.g. branches with run counts) |
| `<DateRangePicker>` | Presets: 24 h / 7 d / 30 d / custom; default 7 d |
| `<DurationFilter>` | Slider for "specs slower than N seconds" |
| `<StatusToggle>` | passed / failed / skipped / flaky pills |
| `<TagFilter>` | Multi-select with `AND` / `OR` mode toggle |
| `<SavedViewMenu>` | User-named filter presets stored in the backend |

All filters update **a single URL query string** owned by the router. Example:

```
/v2/test-runs?project=fern&status=failed&branch=main&from=2026-05-01&q=oauth
```

The page component reads search params, hands them to a typed GraphQL query,
and re-fetches via TanStack Query. Back/forward, copy-link, and reload all
just work.

### Server-side filter contract

Filters require a coordinated schema change. We extend list queries with a
typed `filter` input:

```graphql
input TestRunFilter {
  projectIds:   [ID!]
  status:       [TestRunStatus!]
  branches:     [String!]
  tags:         [String!]
  tagMode:      LogicMode = OR        # AND | OR
  gitCommit:    String
  authors:      [String!]
  durationMs:   IntRange              # { gte, lte }
  startedAt:    DateTimeRange         # { gte, lte }
  search:       String                # full-text on name + error message
}

input ConnectionArgs {
  first:  Int = 50
  after:  String                      # opaque cursor
}

type Query {
  testRuns(filter: TestRunFilter, page: ConnectionArgs): TestRunConnection!
}

type TestRunConnection {
  edges:      [TestRunEdge!]!
  pageInfo:   PageInfo!
  totalCount: Int!                    # for "12,448 ↓" affordance
  facets:     TestRunFacets!          # counts per branch / status / tag
}
```

Notes:

- **Cursor-based pagination** (Relay-style) — stable under inserts, unlike
  offset pagination, which matters because test runs are written continuously.
- **`totalCount` is optional and cached** — computed as a count-estimate
  (`pg_class.reltuples` or `EXPLAIN`-derived) when the filter is broad, exact
  when narrow. This avoids the `SELECT COUNT(*)` trap on 10M-row tables.
- **`facets` returns counts per facet value scoped to the current filter** —
  this is what powers the "23" badge next to a status pill. Implemented as a
  single grouped query, not N round-trips.

Server-side, each filter field becomes a composable `WHERE` clause in the
GORM repository under `internal/domains/testing/infrastructure/`. Indexes:

```sql
-- migrations/NNN_test_run_filter_indexes.up.sql
CREATE INDEX CONCURRENTLY idx_test_runs_project_started
  ON test_runs (project_id, started_at DESC);
CREATE INDEX CONCURRENTLY idx_test_runs_status
  ON test_runs (status) WHERE status IN ('failed','flaky');
CREATE INDEX CONCURRENTLY idx_test_runs_branch
  ON test_runs (project_id, branch);
CREATE INDEX CONCURRENTLY idx_test_runs_fts
  ON test_runs USING GIN (to_tsvector('english', name || ' ' || coalesce(error_message,'')));
```

A separate migration adds indexes; rollout uses `CREATE INDEX CONCURRENTLY`
so it does not block writes.

### Rendering large result sets

Even with server-side filtering, individual results can be large (a single
failed run may have 5,000 specs). The UI must virtualize:

| View | Strategy |
|---|---|
| Test-run list | `@tanstack/react-virtual` — only rendered rows are in the DOM |
| Spec tree (run detail) | Virtualized tree (`react-arborist` or custom on top of `react-virtual`) |
| Error log viewer | `react-window` + lazy-load full text on expand |
| Treemap | D3 with `quadtree`-based hit-testing; **drill-down**, not "draw 50k rects" — top-level shows projects/suites, click to descend |
| Tag cloud | Cap at top N by frequency; "show more" pages the rest |

Rule: **no list component renders > 100 DOM rows at a time, regardless of
result size.**

### Query performance budgets

Targets enforced via a Playwright + Lighthouse check in CI on a seeded
~1 M-row dataset:

| Page | P75 server response | P75 time-to-interactive |
|---|---:|---:|
| Project list | < 100 ms | < 800 ms |
| Test-run list (default filter) | < 250 ms | < 1.2 s |
| Test-run detail (5 K specs) | < 400 ms | < 1.5 s |
| Treemap (top level) | < 300 ms | < 1.5 s |
| Flaky dashboard | < 500 ms | < 1.5 s |

If any query exceeds budget, the PR is blocked. The budget file lives at
`web/perf-budgets.json` and is reviewed each release.

### Caching and prefetching

- TanStack Query caches by `[queryKey, filter]`. Default `staleTime` 30 s for
  lists, 5 min for static metadata (projects, tags).
- On hover of a test-run row, **prefetch** the detail query — perceived
  navigation latency drops to near zero without changing the backend.
- Facet counts are cached server-side in Redis (key = filter hash, TTL 60 s).
  Most users issue the same default filter; one cache hit serves them all.

### Saved views

Users hit the same filter repeatedly ("failed runs on `main` in the last 24h
for project X"). A small `saved_views` table per user stores `{name, page,
filter_json}`. The `<SavedViewMenu>` lists them and applies with one click.
Implementation is server-side because saved views need to follow the user
across browsers and machines.

### Frontend bundle weight

Because the frontend is "heavy" today (3 MB from unpkg, runtime Babel), the
new build also commits to a bundle budget:

| Bundle | Budget (gzipped) | Enforced by |
|---|---:|---|
| Initial route (`/v2/`) | ≤ 150 KB | `size-limit` in CI |
| Per-route async chunks | ≤ 80 KB | `size-limit` in CI |
| Total app (all routes) | ≤ 600 KB | `size-limit` in CI |

Techniques to stay within budget:

- **Route-level code splitting** — D3 (~120 KB) ships only on routes that
  render charts.
- **`manualChunks`** in Vite isolates `react`/`react-dom` so it caches across
  releases.
- **Tree-shaken icons** via `lucide-react` (only used icons ship).
- **No moment.js / no lodash full import** — use `date-fns` and per-function
  lodash imports.
- **Tailwind JIT** — only utilities that appear in source ship in the CSS.

---

## Migration Strategy

We use the **strangler pattern** — the new SPA is mounted at `/v2/` while the
legacy `index.html` remains the default. Pages migrate one at a time. The
flip happens only when parity is verified.

> **Delivery note.** The phased plan below was the *development* plan as
> written. In practice the code landed in a single PR rather than a PR
> per phase — see [Appendix B.10](#b10-single-pr-delivery). The runtime
> rollout (flag-gated coexistence, cutover, sunset window) is unchanged.

### qse 0 — Foundation (1 sprint)

- Scaffold `web/` with Vite + TS + Tailwind + codegen.
- Stand up `internal/web/embed.go`, mounted at `/v2/*`.
- Add `make web-build`, `make web-dev`, `make web-test` Make targets.
- Wire CI: `pnpm install`, `pnpm typecheck`, `pnpm lint`, `pnpm test`,
  `pnpm build`. Build artifact is the input to the Go build job.
- Ship empty `/v2/` route showing "Hello from new UI" behind a feature flag.

**Exit criterion:** new UI scaffold builds, embeds, deploys, and is reachable.
Legacy UI unaffected.

### Phase 1 — Shared layer (1 sprint)

- Port `graphql-client.js`, `duration-utils.js`, `timestamp-component.js` to
  TS in `src/lib/` with full unit tests in Vitest.
- Build the design system: layout, navbar, sidebar, theme provider, auth gate.
- Build first three shadcn/ui primitives needed: Button, Card, Table.

**Exit criterion:** style and layout parity with legacy header/nav.

### Phase 2 — Migrate pages, one per PR

Suggested order (lowest risk → highest value):

1. Project list  → most static, easiest port.
2. Project detail / settings.
3. Test run list.
4. Test run detail (spec tree, error display).
5. Tag management.
6. Flaky test dashboard.
7. **Treemap** — D3 component wrapped in React; complex, highest user-visible
   value, migrate last.

Each PR:

- Ports one page to `/v2/<page>`.
- Adds Playwright spec covering the page's golden path.
- Adds a redirect: when a user with the `v2-ui` flag visits the legacy URL,
  they land on `/v2/<page>`.

### Phase 3 — Cutover

- All pages migrated. Run both UIs side-by-side for two weeks behind a flag.
- Internal users on `v2` by default; external users opt-in via a banner.
- Flip the default. Legacy URL `/*` becomes a redirect to `/v2/*`.

### Phase 4 — Removal

- After one release with v2-default and no rollback, delete legacy `web/index.html`,
  legacy `web/js/*.js`, the FontAwesome fallback, and the `?v=N` cache-busting.
- Move `/v2/*` → `/*`. Bundle size drops; deploy simplifies.

**Total estimate:** 6–8 sprints for a single engineer, 3–4 sprints with two.

---

## Build, Release, and Serving

### Local dev

```bash
make dev              # Go server with live reload
make web-dev          # Vite dev server with HMR
```

### Production build

The frontend build is a prerequisite of the Go build:

```makefile
# Makefile.core (new targets)
web-deps:
	cd web && pnpm install --frozen-lockfile

web-build: web-deps
	cd web && pnpm build

build: web-build
	go build -o bin/fern-platform ./cmd/fern-platform
```

### CI pipeline

```
┌─────────────┐    ┌──────────────┐    ┌─────────────┐    ┌─────────────┐
│ pnpm install│ →  │ typecheck    │ →  │ vitest      │ →  │ pnpm build  │
└─────────────┘    │ eslint       │    │             │    └─────┬───────┘
                   └──────────────┘    └─────────────┘          ▼
                                                          ┌─────────────┐
                                                          │ go build    │ →  Playwright E2E  →  image
                                                          │ (embeds it) │
                                                          └─────────────┘
```

### Dockerfile

Multi-stage with a Node stage producing `dist/`, then the existing Go stage
copies it before `go build`:

```dockerfile
FROM node:20-alpine AS web
WORKDIR /web
COPY web/package.json web/pnpm-lock.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

FROM golang:1.25 AS server
WORKDIR /src
COPY . .
COPY --from=web /web/dist ./web/dist
RUN go build -o /out/fern-platform ./cmd/fern-platform

FROM gcr.io/distroless/base-debian12
COPY --from=server /out/fern-platform /fern-platform
ENTRYPOINT ["/fern-platform"]
```

This collapses `Dockerfile`, `Dockerfile.simple`, and `Dockerfile.goreleaser`
into one.

---

## Testing Strategy

This section defines a test strategy benchmarked against industry standards:
the **test pyramid** (Cohn), **practical test pyramid** (Fowler/Vocke),
**Google testing pyramid**, and **ISO/IEC 25010** quality characteristics
(functional suitability, performance efficiency, compatibility, usability,
reliability, security, maintainability, portability).

### The pyramid we target

```
                         ┌─────┐
                        / E2E   \           ~5 %   ~25 specs
                       /─────────\
                      / Integration\        ~15 %  ~150 tests
                     /─────────────\
                    / Component / API \     ~30 %  ~400 tests
                   /───────────────────\
                  /        Unit         \   ~50 %  ~1500 tests
                 /───────────────────────\
                Static (tsc, lint, schema-diff) — runs first, costs least
```

Numbers are illustrative targets, not quotas. The rule is **shape, not
headcount**: feedback time grows by an order of magnitude per layer up,
so the cheap layers must outnumber the expensive ones.

### Test types and tools

| Layer | Tool | Scope | Speed budget | Where it runs |
|---|---|---|---:|---|
| Static type | `tsc --noEmit` (strict) | TS soundness, schema drift | ≤ 30 s | pre-commit + CI |
| Lint | ESLint, Prettier, `eslint-plugin-jsx-a11y` | Style + a11y smells | ≤ 20 s | pre-commit + CI |
| Schema contract | `graphql-inspector`, REST golden files | API surface freeze | ≤ 30 s | CI |
| Unit (FE) | Vitest | Pure functions, hooks | ≤ 60 s | CI |
| Component | Vitest + RTL + MSW | Components against mocked GraphQL | ≤ 90 s | CI |
| Visual regression | Playwright + `toHaveScreenshot` (or Chromatic) | Design system, charts | ≤ 3 min | CI nightly + on demand |
| Unit (BE) | Go `testing` + `testify` | Pure Go logic | ≤ 60 s | CI |
| Repository | Go + testcontainers-postgres | SQL queries, plans, transactions | ≤ 3 min | CI |
| Contract (consumer) | Pact (frontend → backend) | Each FE GraphQL/REST query is contractually frozen | ≤ 60 s | CI |
| Contract (provider) | Pact verifier on Go server | Backend honors recorded contracts | ≤ 90 s | CI |
| Integration | Ginkgo (existing `acceptance/`) | Cross-domain Go behaviors | ≤ 5 min | CI |
| E2E | Playwright on deployed stack | User journeys, auth, real DB | ≤ 10 min | CI on PR (smoke) + nightly (full) |
| A11y | `@axe-core/playwright` | WCAG 2.1 AA on every E2E page | piggy-backs E2E | CI |
| Mutation | Stryker (TS), `go-mutesting` (Go) | Effectiveness of unit tests | weekly cron | scheduled |
| Fuzz | Go 1.18+ `testing.F` | Cursor codec, filter parser, FTS escaper | nightly cron | scheduled |
| Security | `gosec`, `govulncheck`, `pnpm audit`, `trivy`, OWASP ZAP baseline | Vulns, supply chain, web | ≤ 5 min | CI + nightly |

### Coverage thresholds (enforced in CI)

Industry guidance (Google, Microsoft, Atlassian engineering blogs) converges
on **70–80 % line coverage as a healthy floor for application code**, with
**hot paths approaching 90 %**. We use **statement + branch + function**
coverage, not just lines, because line coverage hides untested branches.

| Scope | Min statement | Min branch | Min function | Tool |
|---|---:|---:|---:|---|
| `internal/domains/**` (business core) | 90 % | 85 % | 95 % | `go test -coverpkg`, `go tool cover` |
| `internal/api/v2/**` | 85 % | 80 % | 90 % | same |
| `internal/api/v1/**` (frozen) | 100 % via golden files | n/a | n/a | golden replay |
| `pkg/**` (shared libs) | 85 % | 80 % | 90 % | same |
| `web/src/lib/**` | 90 % | 85 % | 95 % | Vitest `--coverage` (v8) |
| `web/src/features/**/hooks/**` | 80 % | 70 % | 85 % | Vitest |
| `web/src/components/ui/**` (shadcn) | 70 % | 60 % | 80 % | Vitest |
| `web/src/app/**` (routes) | E2E covers | — | — | Playwright |
| Migration scripts | 100 % up + down round-tripped | n/a | n/a | repository tests |

**Coverage is a smell detector, not a goal.** A PR that adds code without
moving coverage downward is fine; a PR that ships green tests with no
assertions is not. We pair coverage with **mutation testing** (below) to
detect assertion-free tests.

#### Diff coverage rule (Google style)

`diff-coverage ≥ 80 %` on every PR, enforced via `diff-cover`. Lifting the
absolute number is a long-term effort; refusing to *lower* it is the
day-to-day discipline.

### Mutation testing

Coverage tells you tests *executed* the code. Mutation testing tells you
they *would catch a bug*. We run mutation testing on the highest-leverage
modules:

| Module | Tool | Target kill rate | Schedule |
|---|---|---:|---|
| `internal/domains/testing/domain` | `go-mutesting` | ≥ 75 % | weekly |
| `internal/domains/testing/application` | `go-mutesting` | ≥ 70 % | weekly |
| `web/src/lib` | StrykerJS | ≥ 80 % | weekly |
| `pkg/cursor`, `pkg/middleware` | `go-mutesting` | ≥ 80 % | weekly |

Kill rates below threshold open a tracked issue, not a build failure —
mutation testing is too noisy for blocking gates but valuable as a
recurring health signal.

### Contract testing (Pact)

The single biggest source of UI bugs in the legacy codebase is **silent
schema drift**. Codegen catches *type* drift; Pact catches *behavioral*
drift — null vs. empty array, ordering guarantees, error codes.

```
frontend (consumer)                       backend (provider)
─────────────────────────                 ──────────────────────────
useGetTestRunsQuery({...})  ──pact.json──►  Pact verifier runs against
records expected interaction               real Go handler + testcontainers
                                           ✓ matches  →  PR can merge
```

Pact contracts live in `web/tests/pacts/` (consumer side) and are
published to a Pact broker (or `webhook → S3` if a broker is overkill).
The Go CI job downloads them and runs `pact-go verify` against the
running server.

This means a frontend PR that adds a new GraphQL query **automatically
gates the backend's next release** on satisfying it — and vice versa.

### Flake control

The legacy `acceptance/` tests show classic flake patterns (timing-based
waits, real-clock dependencies). Industry standard practices we adopt:

- **No `sleep` in tests.** Use Playwright auto-wait, `waitFor`, RTL's
  `findBy*`. CI fails any test that imports `sleep` in test files.
- **Deterministic time.** Tests inject a `Clock` interface; production
  uses `time.Now`, tests use `clockwork`/`fakeclock`. Frontend uses
  `vi.useFakeTimers()`.
- **Test data isolation.** Every test owns its data; database is reset
  per test via a transactional rollback wrapper or schema-per-test.
- **Flake retry budget.** Playwright `retries: 1` in CI; *more than one
  retry needed for a green PR* opens a `flaky-test` issue automatically
  via a CI bot.
- **Flake dashboard.** A weekly report (`fern migrate check` analog,
  reading CI history) ranks tests by flake rate and assigns owners.

### Accessibility testing

`@axe-core/playwright` runs on every E2E test, asserting **zero critical
or serious violations**. WCAG 2.1 AA contrast and keyboard navigation
are checked on the design-system pages explicitly. This is FR-NFR-5
made executable.

### Security testing

Layered, all gated in CI:

| Concern | Tool | Where it runs |
|---|---|---|
| Go dependency vulns | `govulncheck` | CI on PR |
| Go static analysis | `gosec` | CI on PR |
| Node dependency vulns | `pnpm audit --prod` | CI on PR |
| Container image vulns | `trivy image` | CI on image build |
| Secret leaks | `gitleaks` | pre-commit + CI |
| Dynamic web scan | OWASP ZAP baseline (passive) | nightly against deployed stack |
| SBOM | `syft` | CI on release; signed with `cosign` |
| License compliance | `go-licenses`, `license-checker` | CI on PR |

### Test data strategy

A versioned fixture dataset (`testdata/seed-v1.sql`, ~1 M test_runs across
~50 projects, ~6 mo of history) is checked in as a `pg_restore` dump.
All performance tests and E2E run against it. The size and shape are
modeled on production telemetry from existing Fern installations
(approximate p99 of customer cardinality).

### CI orchestration

```
┌──────────────────────────────────────────────────────────────────┐
│ PR pipeline (target wall-clock: ≤ 12 min)                        │
│                                                                  │
│  static  ─►  unit  ─►  component  ─┐                             │
│   30 s     60 s        90 s         ├─►  build  ─►  contract     │
│                                     │     3 min      90 s        │
│  go-unit ─►  go-repo ─►  go-vet  ──┘                  │          │
│   60 s      3 min        20 s                          ▼          │
│                                                  e2e smoke (6) ──►│
│                                                     8 min        │
└──────────────────────────────────────────────────────────────────┘
┌──────────────────────────────────────────────────────────────────┐
│ Nightly (cron)                                                   │
│  full Playwright suite, visual snapshots, OWASP ZAP, soak test,  │
│  mutation testing (weekly), license/SBOM diff                    │
└──────────────────────────────────────────────────────────────────┘
```

Each job emits JUnit XML, which we **dogfood**: Fern Platform ingests its
own test results via its own ingestion endpoint. Eating our own dog food
catches client-library regressions immediately.

---

## Performance Testing

Industry standard for a data-heavy SaaS-like product is to test
performance across four dimensions: **load**, **stress**, **soak**, and
**spike** — with results tied back to **SLOs** and **error budgets**
(Google SRE Book, *The Site Reliability Workbook*).

### SLOs

Public SLOs for the user-facing surface. Error budgets derived from these
drive release / rollback decisions.

| SLI | SLO (28-day rolling) | Error budget |
|---|---|---|
| Availability (HTTP 2xx/3xx on `/api/v2/*`) | 99.9 % | 43 min / month |
| Latency: list endpoints, P95 | < 500 ms | 1 % over budget |
| Latency: detail endpoints, P95 | < 300 ms | 1 % over budget |
| Latency: ingestion `POST /api/v1/test-runs`, P95 | < 250 ms | 1 % over budget |
| Frontend LCP, P75 (RUM) | < 2.5 s ("Good", Core Web Vitals) | 5 % degradation |
| Frontend INP, P75 (RUM) | < 200 ms ("Good", CWV) | 5 % degradation |
| Frontend CLS, P75 (RUM) | < 0.1 ("Good", CWV) | 5 % degradation |
| GraphQL error rate | < 0.5 % | 1 % over budget |

CWV thresholds match Google's published "Good" classification (Web Vitals
2024 update). LCP and INP supersede FID; we adopt INP as the standard.

### Test types

| Test | Purpose | Tool | Frequency |
|---|---|---|---|
| **Load** | Verify SLOs under expected peak | k6 | Each release candidate |
| **Stress** | Find the breaking point | k6 | Each release candidate |
| **Soak** | Detect memory leaks, FD leaks, slow degradation | k6 | Weekly, 8 h run |
| **Spike** | Survive sudden traffic surges (CI mass-upload) | k6 | Each release candidate |
| **Frontend lab** | Catch regressions before deploy | Lighthouse CI | Every PR |
| **Frontend field (RUM)** | Real-user truth | `web-vitals` → backend → Grafana | Continuous |
| **Bundle size** | Catch JS weight regressions | `size-limit` | Every PR |
| **Query profiling** | Catch missing indexes, N+1 | `pgBadger`, `pg_stat_statements` | Continuous in staging |
| **Chaos** | Verify graceful degradation | `litmus`, `chaos-mesh` | Monthly in staging |

### Load model

Modeled on the seeded 1 M-row dataset and realistic mix:

```
Scenario                       Weight   Method/Endpoint
─────────────────────────────  ──────   ──────────────────────────────────
Browse test-run list (filtered)  35 %    GET /api/v2/test-runs?...
Open test-run detail             20 %    GET /api/v2/test-runs/:id
Open spec details                15 %    GET /api/v2/test-runs/:id/specs
Treemap top-level                 5 %    GraphQL treemap query
Flaky dashboard                   5 %    GET /api/v2/flaky-tests
Ingest test run (CI traffic)     15 %    POST /api/v1/test-runs
Search                            5 %    GET /api/v2/test-runs?q=...
```

Target ramp for the load test:

- **0–2 min:** ramp to 50 VUs (virtual users).
- **2–10 min:** hold at 50 VUs (expected peak).
- **10–12 min:** ramp to 200 VUs (stress).
- **12–15 min:** hold at 200 VUs.
- **Pass criteria:** at 50 VUs, every SLO above is met. At 200 VUs, the
  service does not crash, error rate stays < 5 %, and latency degrades
  gracefully (no cliff). Recovery to nominal latency within 2 min after
  load drops.

### k6 example

`tests/perf/test-runs-list.js`:

```js
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend, Rate } from 'k6/metrics';

const latency = new Trend('list_latency', true);
const errors  = new Rate('errors');

export const options = {
  scenarios: {
    steady: { executor: 'constant-vus', vus: 50, duration: '10m' },
  },
  thresholds: {
    'list_latency': ['p(95)<500', 'p(99)<900'],   // SLO
    'errors':       ['rate<0.005'],
    'http_req_failed': ['rate<0.01'],
  },
};

export default function () {
  const r = http.get(`${__ENV.FERN_URL}/api/v2/test-runs?status=failed&first=50`,
    { headers: { Authorization: `Bearer ${__ENV.TOKEN}` } });
  latency.add(r.timings.duration);
  errors.add(r.status !== 200);
  check(r, { 'has connection': (res) => res.json('edges') !== undefined });
  sleep(1);
}
```

Run from CI on every release candidate against a deployed staging that
matches production sizing. Results are published to a Grafana dashboard;
the build fails if any threshold is exceeded.

### Soak

Weekly 8-hour run at 30 VUs. Pass criteria:

- RSS memory growth < 5 % over the run after warm-up.
- Goroutine count stable (within ±10 % of baseline).
- DB connection-pool usage flat (no leak).
- No deadlocks (`pg_stat_activity` for `idle in transaction` > 60 s
  triggers alert).

### Spike (CI burst)

Simulate a "monorepo CI just uploaded 5,000 test runs in 30 s" event:

- 0–30 s: 500 RPS ingestion to `POST /api/v1/test-runs`.
- 30–90 s: cool down.
- Pass criteria: ingestion error rate < 1 %, no impact on read latency
  (P95 stays under SLO), no DB lock contention errors logged.

### Frontend performance — lab vs. field

**Lab (Lighthouse CI):**

`.lighthouserc.json`:

```json
{
  "ci": {
    "collect": {
      "url": [
        "http://localhost:8080/v2/",
        "http://localhost:8080/v2/test-runs",
        "http://localhost:8080/v2/test-runs/seeded-id",
        "http://localhost:8080/v2/treemap"
      ],
      "numberOfRuns": 5
    },
    "assert": {
      "assertions": {
        "categories:performance":   ["error", { "minScore": 0.9 }],
        "categories:accessibility": ["error", { "minScore": 0.95 }],
        "categories:best-practices":["error", { "minScore": 0.95 }],
        "largest-contentful-paint": ["error", { "maxNumericValue": 2500 }],
        "cumulative-layout-shift":  ["error", { "maxNumericValue": 0.1 }],
        "total-blocking-time":      ["error", { "maxNumericValue": 200 }]
      }
    }
  }
}
```

**Field (RUM):**

`web-vitals` library reports LCP, INP, CLS, FCP, TTFB on a sampled 10 %
of sessions to `POST /api/v2/telemetry/vitals`. Backend aggregates per
route and emits Prometheus histograms. Grafana panels show P75 by route,
by browser, by geo. Regressions surface within hours, not weeks.

### Bundle-size enforcement

`size-limit` config (`.size-limit.json`):

```json
[
  { "name": "initial",     "path": "dist/assets/index-*.js",   "limit": "150 KB", "gzip": true },
  { "name": "route-chart", "path": "dist/assets/chart-*.js",   "limit": " 80 KB", "gzip": true },
  { "name": "route-runs",  "path": "dist/assets/runs-*.js",    "limit": " 60 KB", "gzip": true },
  { "name": "total app",   "path": "dist/assets/*.js",         "limit": "600 KB", "gzip": true }
]
```

PR fails on regression. The PR comment is auto-posted with a delta:
`+12.4 KB (+8.6 %) — over budget by 4.1 KB`.

### Database performance regression detection

A nightly job in staging runs **EXPLAIN ANALYZE** on a curated set of
queries (`tests/perf/queries.sql`) and diffs the **plan node types**
against a baseline. Switching from `Index Scan` to `Seq Scan` on a
1 M-row table is the kind of regression that breaks production at 3 AM;
catching it in staging is cheap.

### Performance budgets summary

A single source of truth file `perf-budgets.json` is consumed by Lighthouse
CI, size-limit, k6 thresholds, and the README so they cannot drift:

```json
{
  "frontend": {
    "lcp_p75_ms":          { "good": 2500, "warning": 4000 },
    "inp_p75_ms":          { "good": 200,  "warning": 500 },
    "cls_p75":             { "good": 0.1,  "warning": 0.25 },
    "tti_p75_ms":          { "good": 1500, "warning": 3000 },
    "initial_bundle_kb":   150,
    "total_bundle_kb":     600
  },
  "backend": {
    "list_p95_ms":         500,
    "list_p99_ms":         900,
    "detail_p95_ms":       300,
    "ingest_p95_ms":       250,
    "graphql_error_rate":  0.005,
    "availability":        0.999
  }
}
```

### Chaos and resilience (optional, recommended)

Run monthly in staging:

- **Pod kill** (random `fern-platform` pod) — verify K8s rescheduling and
  no in-flight ingestion losses (idempotency on `POST /test-runs`).
- **DB failover** — CloudNativePG primary failover; verify reconnect logic
  and connection-pool reset.
- **Redis outage** — verify facet cache degrades to compute-on-request
  rather than returning empty facets.
- **Network partition** — verify reasonable timeouts (no indefinite
  hangs) and circuit breakers where applicable.

### Observability for performance

- **Prometheus metrics:** per-endpoint histogram with `le` buckets aligned
  to the SLOs (250 ms, 500 ms, 1 s, 2 s, 5 s).
- **OpenTelemetry traces:** each request emits a span; DB queries are
  child spans. Service map auto-generated in Grafana Tempo / Jaeger.
- **Slow-query log:** any query > 500 ms is logged with the (parsed)
  filter JSON to support post-hoc analysis.
- **Burn-rate alerts:** alert when error budget is consumed > 2× normal
  rate for 1 h or > 14× for 5 min (Google SRE multi-window multi-burn
  pattern).

---

## Industry-Standard Test Strategy Mapping

To make it explicit what we are aligning with:

| Standard / Framework | How this RFC complies |
|---|---|
| **ISO/IEC 25010** (product quality) | All 8 characteristics addressed (see test types) |
| **Test Pyramid** (Cohn) | Shape enforced; unit-heavy, E2E-light |
| **Practical Test Pyramid** (Vocke) | Contract tests separate the trust boundary |
| **Google Testing on the Toilet** practices | Hermetic tests, deterministic clocks, no test interdependence |
| **Google SRE SLO model** | Public SLOs, error budgets, multi-burn-rate alerts |
| **Core Web Vitals (2024)** | LCP/INP/CLS budgets in lab + field |
| **OWASP ASVS L2** (web app) | ZAP baseline, SBOM, dep scanning, gosec, gitleaks |
| **WCAG 2.1 AA** | axe-core in every E2E |
| **Twelve-Factor** (config) | New config keys are env vars with safe defaults |
| **C4 model** | RFC includes container and component diagrams |
| **CNCF observability** | Prometheus + OTel + structured logs |
| **DORA metrics** | CI optimizes for lead time + change-failure rate via fast PR pipeline + golden contract tests |

---

## Alternatives Considered

### A1. HTMX + Alpine + Tailwind (server-rendered)

- **Pros:** No build step; minimal JS; backend devs can edit UI; perfect fit
  for CRUD pages.
- **Cons:** Treemap and live-updating dashboards are awkward; we lose the
  rich client cache; D3 integration is bespoke; less attractive to frontend
  hires.
- **Verdict:** Rejected. Fern is a *dashboard* product, not a CRUD app.
  Charts and cross-page state cost more than HTMX saves.

### A2. SvelteKit / SolidStart

- **Pros:** Smaller bundles, simpler reactivity.
- **Cons:** Smaller hiring pool; the team already knows React; D3 ecosystem
  bindings are React-first.
- **Verdict:** Rejected on team-fit grounds.

### A3. Keep React but add only Vite + TS (no codegen, no Tailwind)

- **Pros:** Smallest delta.
- **Cons:** Leaves the `any`-typed GraphQL drift, leaves the styling chaos.
  Half a migration is the worst outcome.
- **Verdict:** Rejected.

### A4. Next.js / Remix (SSR)

- **Pros:** SSR, file-based routing, ecosystem.
- **Cons:** Adds Node to the runtime, complicates the single-binary story,
  and SSR offers no real benefit for an authenticated internal dashboard.
- **Verdict:** Rejected. We want the binary to remain self-contained.

### A5. Big-bang rewrite

- **Pros:** Faster end-state.
- **Cons:** Months without shippable UI changes; high risk of stalling.
- **Verdict:** Rejected. Strangler pattern preferred.

---

## Risks and Mitigation

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Migration stalls midway, leaving two UIs forever | Medium | High | Hard deadline per phase; remove legacy in Phase 4 within one release of cutover |
| Treemap port loses visual fidelity | Medium | Medium | Port last, after design system is stable; snapshot tests |
| Bundle size grows | Low | Medium | Bundle-size CI check (`size-limit`); code-split per route |
| Codegen produces churn in PRs | Medium | Low | Commit generated file; CI verifies it matches schema |
| Developers unfamiliar with TS/Vite | Medium | Low | Pairing on first three PRs; doc in `docs/developers/` |
| `pnpm` adds a new tool to install | Low | Low | Doc + `corepack enable` is enough |
| Embedded `dist/` bloats binary | Low | Low | gzip-compressed serving; measure — usually < 2 MB total |

---

## Success Metrics

Baseline (today) vs. target (after Phase 4):

| Metric | Today | Target |
|---|---:|---:|
| `web/index.html` size | 8,787 lines | 0 (deleted) |
| Time-to-interactive (cold cache, p75) | ~3.5 s | ≤ 1.5 s |
| Runtime JS pulled from `unpkg.com` | ~3 MB | 0 |
| Type errors caught at build time | 0 | All |
| GraphQL operations with typed hooks | 0 % | 100 % |
| Playwright tests covering golden paths | 0 | ≥ 12 |
| Build artifact count to deploy | 2 (binary + web/) | 1 (binary) |
| Time to add a new dashboard widget (rough) | ~2 days | ≤ 2 hours |
| Test-run list P75 server response (1 M rows) | unmeasured | < 250 ms |
| Test-run list P75 TTI (1 M rows) | > 5 s (anecdotal) | < 1.2 s |
| Initial bundle, gzipped | ~3 MB (CDN React+Babel) | ≤ 150 KB |
| DOM rows for a 5 K-spec run | 5,000 | ≤ 100 (virtualized) |
| URL-restorable filter state | no | yes (every list page) |
| **Test coverage — domain core (statements)** | unmeasured | ≥ 90 % |
| **Test coverage — frontend lib (statements)** | minimal | ≥ 90 % |
| **Diff coverage on PRs** | not enforced | ≥ 80 % |
| **Mutation kill rate — domain core** | unmeasured | ≥ 75 % |
| **Playwright golden-path coverage** | 0 | ≥ 12 scenarios |
| **PR pipeline wall-clock** | varies | ≤ 12 min |
| **Test flakiness — auto-retried passes** | unmeasured | < 1 % |
| **SLO: list endpoint P95** | unmeasured | < 500 ms |
| **SLO: availability** | unmeasured | 99.9 % |
| **CWV LCP P75 (RUM)** | unmeasured | < 2.5 s |
| **CWV INP P75 (RUM)** | unmeasured | < 200 ms |
| **CWV CLS P75 (RUM)** | unmeasured | < 0.1 |
| **Lighthouse perf score (lab)** | unmeasured | ≥ 0.90 |
| **Lighthouse a11y score (lab)** | unmeasured | ≥ 0.95 |

---

## Implementation Plan

| Phase | Scope | Owner | Duration | Exit |
|---|---|---|---:|---|
| 0 | Scaffold, embed, CI, `/v2/hello` | FE lead | 1 sprint | New UI mounts, legacy unaffected |
| 1 | Design system, shared lib, auth gate | FE lead | 1 sprint | Header/nav parity |
| 2.1 | Project list + detail | FE | 1 sprint | Parity + Playwright passing |
| 2.2 | Test run list + detail | FE | 1 sprint | Parity + Playwright passing |
| 2.3 | Tags + flaky dashboard | FE | 1 sprint | Parity + Playwright passing |
| 2.4 | Treemap | FE + viz | 1 sprint | Visual parity verified |
| 3 | Cutover, dual-run, default flip | FE | 1 sprint | v2 default on |
| 4 | Delete legacy code, simplify Dockerfile, single bundle | FE | 0.5 sprint | Single binary, single Dockerfile |

Cross-cutting (runs in parallel with the above):

| Track | Scope | Owner | Duration | Exit |
|---|---|---|---:|---|
| T-cov | Coverage thresholds, mutation testing schedule, diff-coverage gate | FE+BE | 0.5 sprint | CI enforces thresholds |
| T-pact | Pact consumer + provider tests, broker (or S3) wiring | FE+BE | 1 sprint | Consumer PR can break provider build and vice versa |
| T-perf | k6 scenarios, Lighthouse CI, size-limit, perf-budgets.json | SRE+FE | 1 sprint | PR pipeline gates on budgets |
| T-obs | Prometheus histograms, OTel traces, RUM endpoint, dashboards | SRE | 1 sprint | Burn-rate alerts firing in staging |
| T-sec | gosec, govulncheck, gitleaks, trivy, ZAP nightly, SBOM signing | Sec+SRE | 0.5 sprint | All gates green on `main` |
| T-flake | Flake-retry budget enforcement, weekly flake dashboard | QA | 0.5 sprint | Reports landing in shared channel |

---

## Open Questions

1. **Router choice — TanStack Router or React Router 6?** TanStack gives
   typed routes but is younger; React Router is the safe pick. Lean TanStack
   to extend the type-safety story; revisit at end of Phase 0.
2. **Component library — shadcn/ui or Radix-only?** shadcn = copy-in
   components (no runtime dep); Radix = lib. Default shadcn for ownership.
3. **Do we want dark mode in the first cut?** Recommend: design tokens
   ready for it (CSS vars on `:root`), implementation deferred to post-cutover.
4. **Translations / i18n?** Out of scope for this RFC; the new structure
   makes it straightforward to add `react-i18next` later.
5. **Schema sync mechanism** — `make sync-schema` copying the file, or a
   build-time `gqlgen` postprocess? Recommend the former; explicit beats
   magic.
6. **Where does GraphQL playground live now?** Keep the Go-side
   `/graphql` GET handler; the new UI does not need to host it.
7. **Are we committing to `pnpm` org-wide,** or is `npm` mandated? Confirm
   before Phase 0.

---

## Appendix A — File diff summary as delivered

> **Status note.** An earlier version of this appendix projected
> deletions of the legacy `web/` SPA "at end of Phase 4". Those
> deletions did not land in this PR — v1 coexists at `/` by design
> until the post-cutover sunset window completes. Appendix A below
> reflects what was actually shipped; the legacy-removal diff is
> deferred to a follow-up PR after v2 GA.

```
Additions (v2 source + tooling):
+ web-v2/{src,index.html,package.json,pnpm-lock.yaml,tsconfig.json,
  vite.config.ts, tailwind.config.ts, postcss.config.js,
  codegen.ts, .eslintrc.cjs, .lighthouserc.json}
+ web-v2/src/{components,features,lib,gql,styles}        (TS source)
+ web-v2/src/test-setup.ts + per-file *.test.{ts,tsx}    (Vitest)

Additions (Go backend support):
+ internal/web/{embed.go,embed_test.go}                  (~70 lines + tests)
+ internal/web/dist/**                                   (built artifacts, //go:embed)
+ internal/api/v2/{routes,health,saved_view,telemetry,
  test_run,test_run_trends}.go + *_test.go              (REST namespace)
+ internal/domains/testing/{application,domain,infrastructure}/
  {filter,saved_view,facet_cache,test_run_query_*,
  count_strategy}.go                                     (filter/page/facet pipeline)
+ internal/reporter/graphql/treemap_cache.go             (treemap cache interface)
+ migrations/000022_v2_schema.{up,down}.sql              (single consolidated)
+ scripts/v2-preflight-indexes.sh                        (non-blocking index build)
+ pkg/{cursor,metrics,tracing,middleware/{csp,deprecation,devauth}}

Additions (CI + deploy):
+ (new web-v2 job added to existing .github/workflows/ci.yml,
   including GraphQL schema-drift check; no new workflow files)
+ docker-compose.yml + docker-compose.local.yaml.example
+ Makefile.web, Makefile.docker (new targets, incl. `make jira-key`)
+ perf-budgets.json + tests/perf/*.js (k6) + tests/contract/*.go (Pact-style)

Modifications (v1 untouched where possible):
~ cmd/fern-platform/main.go         (mount v2 SPA, deprecation headers,
                                     read FERN_V2_UI_ENABLED + JIRA key)
~ internal/api/{auth,project,tag}_handler.go    (real wiring to repo
                                     for suspend/activate/delete users,
                                     project access management, real
                                     tag usage counts)
~ internal/reporter/graphql/{schema,resolver,domain_resolvers}.*
                                    (projects pagination, treemap drill,
                                     request-scoped dataloaders)
~ pkg/database/db.go                (dirty-flag fail-fast)
~ pkg/middleware/oauth.go           (centralized SameSite policy)
~ deployments/fern-platform-kubevela.yaml  (JIRA_ENCRYPTION_KEY secretRef,
                                     FERN_V2_UI_ENABLED env)

Deferred (will land post-cutover, not in this PR):
- web/index.html                    (8,787 lines)
- web/js/{graphql-client,duration-utils,timestamp-component}.js
- web/css/font-awesome.min.css
- The inline formatDuration fallback + FontAwesome → emoji shim
```

Net delivered in this PR: ~66,000 lines added (most are v2 SPA source,
embedded built artifacts, GraphQL generated code, and pnpm-lock); ~400
lines modified on the v1 side for safe coexistence. No v1 source
deletions — legacy SPA stays reachable at `/` until sunset.

## Appendix B — Delivered features vs. proposal

This RFC was authored against a clean-room mental model of v2. Real
implementation hit four design points that warranted deviating from
the original proposal. Each is captured here so future readers know
the implementation has drifted from the RFC text on purpose.

### B.1 Search uses `pg_trgm`, not English FTS

**Proposed (§5 — Search):** `to_tsvector('english', spec_name || error_message)` with a GIN index; queries via `plainto_tsquery`.

**Delivered:** `pg_trgm` extension + GIN `gin_trgm_ops` index on the same expression; queries are `ILIKE '%<q>%'`.

**Why:** Test output is not English prose. The English dictionary tokenized identifiers like `DataIntegrityViolationException` as a single stemmed token (`dataintegrityviolationexcept`), so a search for `DataIntegrity` returned zero rows even though 24M `spec_runs` contained that exact substring. Trigrams give index-backed substring matching, which is what users actually want when typing a piece of a stack trace.

**Where:** migration `000022_v2_schema` (extension + index); `BuildTestRunWhere` in `internal/domains/testing/infrastructure/test_run_filter_query.go`.

### B.2 Default 7-day window on unscoped list queries

**Proposed:** `GET /api/v2/test-runs` returns results across all time when no `from`/`to` is supplied.

**Delivered:** The handler clamps `StartedAt.Gte` to `now - 7d` when neither `from` nor `to` is passed. The Test Summaries page's initial window selector also defaults to 7 days so the two surfaces agree on the same scope. Clients that want the full history can opt out with `?allTime=1`.

**Why:** At 1000 projects × 100 runs/day × 6 months the unbounded query had to count and facet ~18M rows on every page load. 7 days drops the working set ~26× while matching the triage scope users actually scroll through from the dashboard. (An earlier internal iteration shipped with 30 days; user testing showed the typical dashboard pass is well under a week, and the 30-day default left the page noticeably slower than v1 on the same hardware. We tightened it to 7.) Wider windows are explicit — UI presets jump to 30d/90d, and `?allTime=1` removes the clamp entirely.

**Where:** `internal/api/v2/test_run_handler.go` `list()`; `web-v2/src/features/summaries/TestSummaries.tsx`.

### B.3 Tag facet is opt-in

**Proposed (§4 — Facets):** Every list response includes `byStatus`, `byBranch`, `byProject`, and `byTag`.

**Delivered:** The first three always run (parallel via `errgroup`). `byTag` is computed only when the filter sets `IncludeTagFacet`, which the handler turns on via `?facets=tag` and the front-end's `<FilterSidebar>` triggers when the user opens the Tag section or has tags already selected.

**Why:** The tag facet joins `test_runs → suite_runs → suite_run_tags → tags` and GROUP-BYs across 900k+ suite_runs on the seeded dataset — measured at 15 seconds in the container log on a cold cache. Skipping it for the default landing case turns a slow page into a fast one without losing any information; expanding the section pays the cost intentionally.

**Where:** `internal/domains/testing/domain/filter.go` (the `IncludeTagFacet` field); `TestRunQueryRepo.ComputeFacets`; `?facets=tag` parsing in the v2 handler.

### B.4 Caches are in-process, not Redis-only

**Proposed (§4 — Facet caching):** Redis-backed cache with 5-minute TTL.

**Delivered:** Two distinct in-process caches, both fronting a `RedisLike` interface so a Redis adapter can be plugged in later:

| Cache             | Where                                                   | TTL  | Key shape                                |
| ----------------- | ------------------------------------------------------- | ---- | ---------------------------------------- |
| Facet cache       | `internal/domains/testing/application/facet_cache.go`   | 5min | SHA-256 of the filter projection         |
| Treemap cache     | `internal/reporter/graphql/treemap_cache.go`            | 60s  | `userID \| drillProjectID \| days`       |

**Why:** Single-replica deploys (the only mode v2 supports today) don't need cross-pod cache coherence, and adding Redis to the required-dependencies list was a barrier the team didn't want for v2 GA. The `RedisLike` interface keeps the upgrade path clean: a multi-replica deploy can supply a Redis-backed adapter without changing call sites.

### B.5 Migrations consolidated before GA

**Proposed:** v2 schema changes ship in their own migration files alongside whatever order they were authored.

**Delivered:** During development the schema landed in three migrations (000022 / 000023 / 000024). Before any external operator ran the code, those were squashed into a single `000022_v2_schema` and the intermediates deleted. The dead FTS GIN index (`idx_spec_runs_fts`) is also dropped as part of this consolidation since the search path migrated to trigrams (see B.1).

**Why:** Operators see a clean "v2 schema" entry in `schema_migrations` instead of three layered patches that reflected our internal iteration order.

**Where:** `migrations/000024_v2_schema.{up,down}.sql`; `scripts/v2-preflight-indexes.sh` for non-blocking deploys on large databases; `docs/specs/frontend-modernization/migration-guide.md` §4 for operator-facing rollout and recovery procedures.

### B.6 Dedicated trends endpoint instead of client-side fan-out

**Proposed:** The Test Summaries page builds per-project sparklines by calling `/api/v2/test-runs` once per project and bucketing the responses in the browser.

**Delivered:** The page calls `GET /api/v2/test-runs/trends?project=…&project=…&days=N` exactly once, getting back a `{ buckets: { projectId: [...daily rows] } }` shape that's already aggregated server-side. Daily rows come from a single `GROUP BY (project_id, date_trunc('day', start_time))` SQL aggregate. A 60-second in-process cache (mirror of the treemap cache) sits in front of the endpoint, keyed by `(userID, sorted projectIDs, days)`. The per-card render then defers its sparkline SVG paint via IntersectionObserver so off-screen cards don't pay layout cost.

**Why:** At 100 visible projects, the original fan-out fired ~100 parallel HTTP requests on every page mount, each driving COUNT(\*) + four facet GROUP BYs + the keyset list query — roughly ~400 SQL queries per page load. v1 never had this page at all (its closest equivalent, the treemap, is a single aggregate resolver), so users perceived v2 as dramatically slower on a surface that has no v1 counterpart. Collapsing to one endpoint + one SQL query closes the gap.

**Where:** `internal/api/v2/test_run_trends_handler.go`; `AggregateDailyByProjects` on `TestRunRepository`; `web-v2/src/features/summaries/TestSummaries.tsx` (`<TrendCardsGrid>` plus the `useInViewOnce` hook for lazy sparkline render).

### B.7 Treemap is embedded in Summaries, not a standalone page

**Proposed (§"Detailed Design" — Treemap):** A dedicated `/v2/treemap` route that mirrors v1's treemap-as-its-own-page model, with a two-level drill (project → suite).

**Delivered:** No standalone `/v2/treemap` route. The treemap component lives inside the Test Summaries page's Tree view (toggled from the Card view via a view-mode switch). The drill is now three levels — project → suite → spec — backed by a new `AggregateSpecsForSuiteInRange` repo method, capped at 500 spec rows ordered by total duration DESC so the deepest view stays bounded. Spec tiles use a continuous red-to-green gradient driven by per-spec `passRate` so a suite tile's color reads consistently with the spec tiles it contains.

**Why:** During UX review the standalone page felt redundant — the treemap and the summary cards answered the same "what's the health of my projects?" question from slightly different angles, so users had to remember which entry point to use. Folding the treemap into the Summaries page made it one decision (Card view vs Tree view) with one filter bar that scopes both. Three levels of drill came from user feedback that "I can see the suite is red, but which spec?" was a constant follow-up click.

**Where:** `web-v2/src/features/treemap/Treemap.tsx` (now consumed by `TestSummaries.tsx`'s `<TreeView>`); GraphQL `treemapData(projectId, suiteName, days)` in `internal/reporter/graphql/schema.graphql`; `AggregateSpecsForSuiteInRange` on `TestRunRepository`.

### B.8 Manager Dashboard is a thin redirect, not a dedicated page

**Proposed (§"Detailed Design" — Pages):** A v2 equivalent of v1's `/#/manager-dashboard` page with its own layout and data fetching.

**Delivered:** `/v2/manager-dashboard` is a 30-line route component that redirects to `/v2/summaries?view=tree&favoritesOnly=true` with role gating (manager / admin). The Summaries page reads `view` and `favoritesOnly` from URL search params on first mount.

**Why:** v1's manager dashboard was — once we looked at the actual rendered UI — "the summary page, but filtered to my favorites, in tree view". Building a parallel page would have duplicated state, query keys, and styling for behavior we already had. A URL preset gave manager/admin users the same landing experience for zero new components and one new redirect file. The role gate still belongs in v2 because non-managers should not see the entry point in the sidebar nav.

**Where:** `web-v2/src/features/summaries/ManagerDashboardRedirect.tsx`; `canSeeNavItem` predicate in `web-v2/src/components/layout/AppShell.tsx`; URL-param handling in `TestSummaries.tsx`.

### B.9 `JIRA_ENCRYPTION_KEY` is required, not optional

**Proposed (§5 — Auth integration):** Jira credentials are encrypted at rest using a key sourced from configuration. No explicit guidance on key handling.

**Delivered:** The application fails to start unless `JIRA_ENCRYPTION_KEY` is present and decodes to exactly 32 bytes (base64). No hardcoded fallback exists anywhere in the repo. A `make jira-key` Makefile target writes a per-developer key into `~/.fern/jira-key` for local development; production deploys mount the value via a Kubernetes `Secret` (`deployments/fern-platform-kubevela.yaml` uses `valueFrom.secretKeyRef`).

**Why:** A development-time fallback (`[]byte("your-32-byte-encryption-key-here")`) was discovered in code review. Even gated behind "only used in dev" the literal-in-source string is an unacceptable security posture for an OSS project — copy-paste deployments would silently inherit it. Failing fast at startup makes the misconfiguration loud instead of latent. Per-developer key generation keeps the dev experience one command away.

**Where:** `internal/domains/factory.go` (`loadJiraEncryptionKey`); `Makefile.docker` (`jira-key` target); `docker-compose.yml` (uses the fail-fast `${VAR:?...}` syntax); `deployments/fern-platform-kubevela.yaml` (Secret ref); `docs/specs/frontend-modernization/migration-guide.md` §8 configuration table.

### B.10 Single-PR delivery

**Proposed (§"Migration Strategy"):** Phased delivery — Phase 0 (foundation), Phase 1 (shared layer), Phase 2 (one PR per page), Phase 3 (cutover), Phase 4 (removal). Each phase a separate PR with exit criteria.

**Delivered:** All development phases (0 through 3) collapsed into a single PR. The intermediate phase branches exist in the repo's branch history but were squashed before opening the upstream PR. The **runtime** rollout model is unchanged — code is dark-launched behind `FERN_V2_UI_ENABLED=false`, flipped on after smoke-validation, and v1 sunsets on a 12-month schedule.

**Why:** Phased PRs assumed reviewers could meaningfully gate each phase. In practice the phases interleaved (Phase 2's backend query work only made sense once Phase 3's pages started consuming it; Phase 1's `<UserPill>` design only stabilized after Phase 3 pages exercised it) so phase-shaped PRs would have either shipped broken intermediate states or required cross-phase backtracking. Squashing on the way out gave reviewers one coherent change to evaluate against the RFC. The post-merge milestones (cutover, sunset) are unchanged and remain operator-controlled flag flips.

**Where:** This PR's single commit on top of `origin/main`; commit history preserved on the backup branch `feat/frontend-modernization-phase-7-real-pages-backup-pre-rebase`. The Migration Strategy section above carries a delivery note pointing here.

### B.11 Test-run detail is a table drill, not a virtualized tree

**Proposed (§"Detailed Design" / T3.4 in tasks.md):** Render the spec list with `react-arborist`-style virtualized tree control, lazy-loading children so a 5,000-spec run renders the visible viewport in ≤ 1.5 s. Treats the run as a single nested data structure (run → suites → specs) and folds the drill into a single tree control.

**Delivered:** Two-view drill that mirrors v1 exactly. Landing on `/v2/test-runs/:id` shows a *Suites* table with v1's columns (Suite Name, Test Results P/F/S triple, Status, Duration, Tags). Clicking a suite row swaps the table for a *Specs* table with v1's spec columns (Test Name, Status, Duration, Error Message, Tags, Started). A back-link returns to the suites view. Run-level header (project, runId, branch, status, environment, commit, tags, four stat cards, collapsible metadata panel) is always visible above both views. Error messages are inline-truncated with click-to-expand stack-trace blocks. Stack traces, retry counts, and flaky markers surface inline on spec rows.

**Why:** v1 has trained users on this exact navigation pattern (table → click → table → back), and a side-by-side comparison made it clear that the tree control — while technically equivalent — broke users' muscle memory and forced them to scan deep nesting to find what `<th>Status</th>` immediately exposes in v1. The virtualization rationale (5,000-spec runs) turned out not to bite at the data volumes we measured: the largest seeded suites cap around 200-400 specs, and the browser handles that table fine without virtualization. If a future use case actually produces a 5,000-spec suite we can swap in row virtualization within the specs table without changing the surrounding navigation.

**Where:** `web-v2/src/features/test-runs/TestRunDetail.tsx` (the table-drill renderer); `web-v2/src/features/test-runs/TestRunsList.tsx` (the matching v1-parity column set on the runs list, satisfying FR-15b); `web-v2/src/features/test-runs/TestRunDetail.test.tsx` (covers both views, the drill transition, and the back-link).
