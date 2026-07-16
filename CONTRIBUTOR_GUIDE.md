# Fern Platform — Contributor Guide

A practical orientation for new contributors. Read this end-to-end before your
first PR. For deeper references, see `docs/ARCHITECTURE.md`, `CONTRIBUTING.md`,
and `docs/all-docs.md`.

---

## 1. What Fern Platform Is

Fern Platform is a **unified test intelligence platform**: a Go monolith that
ingests test results from any CI/CD framework (Jest, pytest, JUnit, Ginkgo,
etc.), stores them in PostgreSQL, and surfaces them through a web UI and
GraphQL/REST APIs.

Think "Datadog/Grafana, but for test runs": aggregation, flaky-test detection,
performance trends, and team-scoped dashboards.

### Core Features

| Feature | What it does | Where it lives |
|---|---|---|
| **Test run ingestion** | REST/GraphQL endpoints accept runs, suites, specs from client libs | `internal/api/test_run_handler.go`, `internal/domains/testing` |
| **Flaky test detection** | Computes flakiness from historical pass/fail patterns | `internal/domains/testing/domain/flaky.go`, `internal/api/flaky_test_handler.go` |
| **Project management** | Manager-created projects, team ownership, permissions | `internal/domains/projects`, `internal/api/project_handler.go` |
| **Treemap / summaries UI** | Single-page web UI for dashboards and drill-down | `web/index.html`, `web/js/` |
| **Tag-based filtering** | Tag specs/runs for filtering and grouping | `internal/domains/tags`, `internal/api/tag_handler.go` |
| **OAuth / RBAC** | Keycloak-backed SSO with group-based permissions | `internal/domains/auth`, `pkg/auth` |
| **Jira integration** | Link test failures to Jira issues | `internal/domains/integrations`, `internal/api/jira_connection_handler.go` |
| **GraphQL API** | Rich querying for analytics | `internal/reporter/graphql`, `gqlgen.yml` |
| **Analytics (in progress)** | Trend detection, AI insights | `internal/domains/analytics` |

### Client libraries (separate repos)

- Go/Ginkgo — `fern-ginkgo-client`
- Java/JUnit — `fern-junit-client`, `fern-junit-gradle-plugin`
- JS/Jest — `fern-jest-client`

These POST test results to the platform's REST endpoints using a `FERN_PROJECT_ID`.

---

## 2. Architecture at a Glance

Fern follows **Domain-Driven Design (DDD)** with hexagonal layering. One Go
binary (`cmd/fern-platform`) wires everything together.

```
cmd/fern-platform/main.go          -- entry point, Gin HTTP server
│
├── internal/api/                  -- HTTP handlers (REST + adapters)
├── internal/domains/              -- DDD business domains
│   ├── testing/                   -- TestRun, SuiteRun, SpecRun, FlakyTest
│   ├── projects/                  -- Project, Permission, Team
│   ├── auth/                      -- User, Session, Group, Scope
│   ├── tags/                      -- Tagging
│   ├── summary/                   -- Run summaries
│   ├── integrations/              -- Jira etc.
│   └── analytics/                 -- (future) trends/AI
├── internal/reporter/graphql/     -- GraphQL schema + resolvers (gqlgen)
├── internal/infrastructure/       -- DB, Redis, external clients
├── pkg/                           -- Reusable: auth, config, db, middleware, logging
├── web/                           -- Static SPA (HTML/JS/CSS, no build step)
├── migrations/                    -- SQL migrations (golang-migrate)
└── deployments/                   -- KubeVela manifests for k3d deploy
```

### Each domain has four layers

```
internal/domains/<name>/
  domain/          # entities, value objects, repository interfaces — pure Go
  application/     # use cases / services — orchestrates domain
  infrastructure/  # GORM repositories, external API clients
  interfaces/      # HTTP/GraphQL adapters
```

**Dependency rule:** interfaces → application → domain. Infrastructure
implements interfaces declared in `domain/`. Never import infrastructure
from `domain/`.

### Request flow (typical write)

```
client lib  →  POST /api/v1/test-runs                       (HTTP)
            →  internal/api/test_run_handler.go             (handler/adapter)
            →  internal/domains/testing/application/...     (use case)
            →  internal/domains/testing/domain/test_run.go  (entity validation)
            →  internal/domains/testing/infrastructure/...  (GORM repo)
            →  PostgreSQL
```

Read flow is symmetric; the GraphQL resolver in `internal/reporter/graphql`
calls the same application services.

### Storage

- **PostgreSQL** — primary store (run via CloudNativePG operator in k3d, or
  external for Docker). Schema lives in `migrations/`.
- **Redis** — caching and ephemeral state.
- **Keycloak** — OAuth provider (only when running full k8s deploy).

---

## 3. Local Setup

There are two supported paths. Pick based on what you're changing.

### Path A — Full Kubernetes deploy (recommended for first run)

Best when: you need OAuth, you're working on deploy manifests, or you want the
exact prod-like topology.

**Prereqs**

- Docker (with buildx)
- [k3d](https://k3d.io/) ≥ v5
- `kubectl`, `helm`
- [KubeVela CLI](https://kubevela.io/docs/installation/kubernetes#install-vela-cli)
- Go 1.23+ (Makefile uses Go for arch detection)
- 8 GB RAM free

**One-time host setup (needed for Keycloak OAuth)**

```bash
echo "127.0.0.1 fern-platform.local" | sudo tee -a /etc/hosts
echo "127.0.0.1 keycloak"             | sudo tee -a /etc/hosts
```

**Deploy**

```bash
git clone https://github.com/guidewire-oss/fern-platform
cd fern-platform
make deploy-all          # ~15 min: k3d cluster + KubeVela + CNPG + build + deploy
```

When it finishes, open <http://fern-platform.local:8080>. Default login:
`admin@fern.com` / `test123`.

Useful targets:

| Make target | What it does |
|---|---|
| `make deploy-all` | Full bring-up: cluster + prereqs + build + deploy |
| `make teardown` | Tear it all down |
| `make dev` | Run the binary against an existing cluster's DB with live reload |
| `make test` | Unit tests |
| `make help` | List all targets (grouped by category) |

### Path B — Run the Go binary against your own Postgres/Redis

Best when: iterating quickly on Go code without re-pushing images. OAuth/SSO
will be disabled or stubbed.

```bash
# 1. Bring up dependencies
docker run -d --name fern-pg    -p 5432:5432 -e POSTGRES_PASSWORD=postgres postgres:14
docker run -d --name fern-redis -p 6379:6379 redis:7

# 2. Set env (or use config/ files)
export DB_HOST=localhost DB_USER=postgres DB_PASSWORD=postgres DB_NAME=fern_platform
export REDIS_HOST=localhost

# 3. Create DB + run migrations
createdb -h localhost -U postgres fern_platform
# migrations run automatically on first start

# 4. Build & run
make run                 # or: go run ./cmd/fern-platform
```

The server listens on `:8080`. Hit `GET /health` to confirm it's up.

### Path C — Docker (coming soon)

Docker images are not yet published. Use A or B until v0.1.0.

---

## 4. Development Workflow

Fern uses **spec-first TDD** for non-trivial changes (see
`.claude/rules/spec-first-tdd.md` if you're contributing via Claude Code).
For most small bug fixes, the lightweight loop is:

1. **Branch off `main`** — name it `fix/...`, `feat/...`, or `chore/...`.
2. **Write a failing test first** — find the closest test file
   (`*_test.go` in the same package) and add a case before touching impl.
3. **Implement until green** — keep tests green at every commit.
4. **Run the suite** — `make test` for unit, `make test-acceptance` for E2E
   (Ginkgo-based, in `acceptance/`).
5. **Lint & vet** — `make lint vet fmt`.
6. **Verify end-to-end** — if your change touches a user-visible feature,
   start the app (`make dev` or `make deploy-all`) and exercise it through
   the UI or `curl`. Unit tests don't catch codegen and runtime issues.
7. **Open a PR** against `main`. CI runs tests, lint, govulncheck, and a
   build of the image. If your change touches `web-v2/**`, CI also runs
   the v2 SPA's typecheck, lint, unit tests, and production build, plus a
   GraphQL schema-drift check between the backend SDL and the copy
   web-v2's codegen consumes.

### v2 SPA contributors — pre-push checklist

The v2 SPA (`web-v2/`) is embedded into the Go binary via `//go:embed
all:dist`. The CI pipeline catches source breakage, but running the
checks locally before push saves a CI round-trip:

```bash
cd web-v2
pnpm install                       # first time only
pnpm typecheck && pnpm lint && pnpm test:run && pnpm build
```

If you modified the GraphQL schema in
`internal/reporter/graphql/schema.graphql`, also copy it to
`web-v2/src/gql/schema.graphql` and re-run `pnpm codegen` — CI fails
the build if the two files diverge.

### Code conventions

- **Go style:** standard `gofmt` + `goimports`. CI enforces.
- **Errors:** wrap with `fmt.Errorf("...: %w", err)`. Domain errors live in
  `<domain>/domain/errors.go`.
- **Logging:** use `pkg/logging` (structured zap). Don't `fmt.Println`.
- **HTTP handlers:** thin — validate input, call application service, format
  response. No SQL in handlers.
- **Database access:** only from `infrastructure/` layer. Domain talks via
  the repository interface.
- **Tests:** prefer table-driven tests, use `testify/require` for fatal
  asserts and Ginkgo for behavior specs in `acceptance/`.

### Adding a new feature — typical files touched

A new endpoint (say, "list slow tests") usually touches:

```
internal/domains/testing/domain/test_run.go         # new query method on entity / repo iface
internal/domains/testing/application/...            # new use case
internal/domains/testing/infrastructure/...         # GORM impl of the new query
internal/api/test_run_handler.go                    # new route + handler
internal/reporter/graphql/schema.graphql + resolver # if GraphQL
migrations/NNN_xxx.sql                              # if schema change
*_test.go in each of the above                      # the TDD tests
```

If schema changes: add a `migrations/NNN_*.up.sql` and matching `.down.sql`.

---

## 5. Where to Look First

Reading these in order gives you a working mental model in ~30 min:

1. `README.md` — product framing.
2. `docs/ARCHITECTURE.md` — full architecture write-up.
3. `internal/domains/README.md` — DDD layout rationale.
4. `cmd/fern-platform/main.go` — see how everything is wired.
5. `internal/api/test_run_handler.go` — example handler.
6. `internal/domains/testing/domain/test_run.go` — example domain entity.
7. `migrations/` — schema reality.

For UI work, `web/index.html` and `web/js/*.js` are the entire frontend
(no build step, vanilla JS / framework-light).

---

## 6. Common Tasks Cheat Sheet

```bash
make help                      # show all targets
make dev                       # local dev with live reload
make test                      # unit tests
make test-acceptance           # Ginkgo E2E (needs deployed stack)
make lint vet fmt              # static checks
make docker-build              # build image locally
make deploy-all                # full k3d deploy
make teardown                  # remove cluster
go generate ./...              # regenerate gqlgen / mocks
```

GraphQL playground (when running): <http://fern-platform.local:8080/graphql>

---

## 7. Getting Help

- **Bugs / features:** GitHub Issues on `guidewire-oss/fern-platform`.
- **Architecture questions:** `docs/rfc/` has the design proposals.
- **Stuck on local setup:** `docs/troubleshooting/`.
- **Client libraries:** each client lib has its own repo under `guidewire-oss/`.

Welcome aboard — start with a small issue labeled `good-first-issue` if one
exists, otherwise pick a flaky-detection or UI tweak; both areas have
good test coverage to lean on.
