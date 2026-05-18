# Technology Stack

## Runtime & Language
- **Backend:** Go 1.24 (toolchain min 1.23)
- **Frontend:** Static HTML/CSS/JS in `web/` (no React build pipeline)
- **Database:** PostgreSQL 14+ via GORM
- **Cache/queue:** Redis 6+

## Key Dependencies
| Package | Purpose |
|---------|---------|
| `gin-gonic/gin` | HTTP web framework |
| `99designs/gqlgen` + `vektah/gqlparser` | GraphQL server (schema-first, generated resolvers) |
| `graph-gophers/dataloader/v7` | N+1 query batching for GraphQL |
| `gorm.io/gorm` + `gorm.io/driver/postgres` | ORM and Postgres driver |
| `golang-migrate/migrate/v4` | Schema migrations (files in `migrations/`) |
| `spf13/viper` | Config loading (`config/config.yaml`) |
| `golang-jwt/jwt/v5` | JWT handling for OAuth/SSO |
| `onsi/ginkgo/v2` + `onsi/gomega` | BDD test framework (unit + acceptance) |
| `stretchr/testify` | Assertion library used alongside Ginkgo for unit tests |
| `DATA-DOG/go-sqlmock` | DB mocking for repository tests |
| `gorilla/websocket` | Real-time updates |
| `sirupsen/logrus` | Structured logging |

## Build & Development
- `make build` — build binary into `bin/fern-platform`
- `make dev` — run with live reload
- `make test` / `make test-unit` — run unit tests (Ginkgo)
- `make test-acceptance` — run e2e tests (Ginkgo, requires deployed stack)
- `make test-playwright` — Playwright UI tests via Dagger
- `make test-performance` — GraphQL performance regression
- `make lint` / `make vet` / `make fmt` — Go linting / static analysis / formatting
- `make deploy-all` — full local k3d + KubeVela deployment (~15 min)

## Conventions
- **GraphQL schema-first** — edit `internal/reporter/graphql/schema.graphql`, then `go generate` for resolvers
- **Domain-driven layout** — code organized by domain (`internal/domains/<domain>/`) with `application/`, `domain/`, `infrastructure/` sub-packages
- **Repository pattern** — interfaces in `domain/`, GORM implementations in `infrastructure/`
- **TDD with Ginkgo** — `Describe`/`Context`/`It` blocks; one `_suite_test.go` per package wires Ginkgo
- **Migrations** — additive, never edit a merged migration; add a new file under `migrations/`
- **Mock JIRA server** — `mock-jira/` and `acceptance/helpers/mock_jira_server.go` for integration tests
- **Spec-first development** — new features get a spec under `specs/<name>/` (requirements, design, tasks)
