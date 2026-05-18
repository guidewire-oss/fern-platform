# Project Structure

## Directory Layout
```
fern-platform/
├── cmd/fern-platform/          # main.go entrypoint
├── internal/
│   ├── api/                    # REST handlers
│   ├── reporter/
│   │   ├── graphql/            # gqlgen schema, generated code, resolvers
│   │   └── service/            # service layer
│   ├── domains/                # domain-driven packages
│   │   ├── analytics/
│   │   ├── auth/
│   │   ├── integrations/       # PM connectors (JIRA, etc.)
│   │   ├── projects/
│   │   ├── summary/
│   │   ├── tags/
│   │   └── testing/            # test runs, suites, specs
│   ├── infrastructure/         # cross-cutting infra (logging, config)
│   └── testhelpers/            # shared test helpers
├── pkg/                        # exported packages
├── migrations/                 # golang-migrate SQL files
├── config/                     # config.yaml + examples
├── web/                        # static frontend (HTML/CSS/JS)
├── acceptance/                 # Ginkgo e2e tests
│   ├── auth/  projects/  testruns/  testsummaries/  pmconnectors/
│   └── helpers/                # shared e2e helpers (mock servers, navigation)
├── deployments/components/     # KubeVela component manifests
├── mock-jira/                  # local mock JIRA server
├── ci/                         # Dagger CI pipeline
├── specs/                      # SDD specs (this project's specs)
├── steering/                   # SDD steering docs (this directory)
├── ctx/                        # SDD subsystem context docs
└── docs/                       # user-facing documentation
```

## Key Directories
| Path | Purpose |
|------|---------|
| `cmd/fern-platform/main.go` | Application entrypoint |
| `internal/domains/<domain>/` | Business logic per domain (DDD layering) |
| `internal/domains/<d>/domain/` | Entities + repository interfaces |
| `internal/domains/<d>/application/` | Service layer / use cases |
| `internal/domains/<d>/infrastructure/` | GORM repo impls and external clients |
| `internal/reporter/graphql/` | GraphQL schema, generated resolvers, domain_resolvers.go |
| `internal/api/` | REST handler layer (v1 + v2) |
| `migrations/` | Versioned SQL migrations |
| `web/` | Static frontend served by the Go binary |
| `acceptance/` | Ginkgo-based end-to-end tests |
| `deployments/components/` | KubeVela `Application` and component definitions |

## Naming Conventions
- **Go files:** `snake_case.go`, tests as `<name>_test.go`, Ginkgo suite wiring as `<pkg>_suite_test.go`
- **GraphQL resolvers:** `<Type>_domain` suffix in `domain_resolvers*.go` files
- **Migrations:** `NNNNNN_<verb>_<subject>.up.sql` / `.down.sql` (sequential numeric prefix)
- **Domain packages:** lowercase singular noun (`project`, not `project_management`)

## Architecture Notes
Each domain package owns its entities, repository interfaces, and service logic. GORM implementations go in `infrastructure/`. The GraphQL layer (`internal/reporter/graphql/`) is what the web UI talks to; the REST layer (`internal/api/`) is for client libraries submitting test results. Auth is OAuth/OIDC with role/group checks enforced at the resolver level.
