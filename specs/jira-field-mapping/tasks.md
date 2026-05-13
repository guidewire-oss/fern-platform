# Tasks: JIRA Field Mapping

All tasks follow the project's TDD red-green-refactor discipline: write the
failing test first, then the minimum implementation, then refactor. Each task
references the requirement(s) and design sections it satisfies.

## Implementation Tasks

### Phase 1: Domain Foundations

- [x] **Task 1.1**: Add `FernField` and `ReductionStrategy` enums plus
  validation errors
  - **ID**: `task-1.1`
  - **BlockedBy**: `none`
  - **Agent**: `general-purpose`
  - **File**: `internal/domains/integrations/types.go`
  - **Change**: Extend the existing `types.go` with the eight `FernField`
    constants, the three `ReductionStrategy` constants, and a sentinel
    error set (`ErrRequiredFieldUnmapped`, `ErrDuplicateJiraField`,
    `ErrMissingReductionStrategy`, `ErrNoJiraConnection`,
    `ErrNoFieldMapping`).
  - **Outcome**: Compile-time constants available; `go test ./...`
    continues to pass.
  - **Context**: Design "Domain types" section. Required Fern fields are
    `requirement_id` and `requirement_title` (Requirement 1 ac1, Req 2
    ac1). Errors are surfaced through GraphQL per the Error Handling
    table.

- [x] **Task 1.2**: Author migration `000022_create_jira_field_mappings_table`
  - **ID**: `task-1.2`
  - **BlockedBy**: `none`
  - **Agent**: `general-purpose`
  - **File**: `migrations/000022_create_jira_field_mappings_table.up.sql`
    (plus the corresponding `.down.sql`)
  - **Change**: Create the table per Design "Migration" — `entries JSONB`,
    `UNIQUE(project_id)`, FK cascade to `project_details`, indices on
    `project_id` and `deleted_at`. Add `OWNER TO app` and grants matching
    migration 000015. (Note: 000017–000021 are already taken; latest is
    000021_add_test_runs_composite_index.)
  - **Outcome**: `make deploy-all` runs the new migration cleanly; rollback
    drops the table.
  - **Context**: Design decisions 1 and 2 (JSONB; unique per project).
    Follow naming and grants from `000015_create_jira_connections_table`.

- [x] **Task 1.3**: Write failing tests for `JiraFieldMapping` aggregate
  - **ID**: `task-1.3`
  - **BlockedBy**: `task-1.1`
  - **Agent**: `general-purpose`
  - **File**: `internal/domains/integrations/jira_field_mapping_test.go`
  - **Change**: Ginkgo + Gomega tests covering: constructor rejects
    duplicate `JiraFieldID` across entries; rejects empty `JiraFieldID`
    on required Fern fields; rejects missing reduction strategy for a
    single-value Fern field mapped to a multi-value JIRA field;
    round-trips through `UpdateEntries`; allows unmapped optional fields;
    allows multi-value Fern field (`tags`) without reduction.
  - **Outcome**: Tests compile and fail with "undefined: JiraFieldMapping"
    or similar.
  - **Context**: Requirements 1 ac5, 2 ac1, 3 ac1, 4 ac1, 4 ac3. Use the
    Ginkgo Describe/Context/It pattern from `jira_connection_test.go`.

- [x] **Task 1.4**: Implement `JiraFieldMapping` aggregate to make Task 1.3
  pass
  - **ID**: `task-1.4`
  - **BlockedBy**: `task-1.3`
  - **Agent**: `general-purpose`
  - **File**: `internal/domains/integrations/jira_field_mapping.go`
  - **Change**: Implement `JiraFieldMapping` struct, `FieldMappingEntry`
    struct, `NewJiraFieldMapping(projectID string, entries []FieldMappingEntry, updatedBy string)`,
    `UpdateEntries(entries []FieldMappingEntry, updatedBy string)`,
    accessors, and a `Snapshot()` method matching the pattern in
    `jira_connection.go`. Enforce invariants in the constructor and
    `UpdateEntries`.
  - **Outcome**: Tests from Task 1.3 pass; `go vet ./...` clean.
  - **Context**: Mirror the private-field + accessor + `Snapshot()` style
    used by `JiraConnection`. Aggregate invariants per Design "Domain
    types" bullet list.

### Phase 2: Persistence

- [x] **Task 2.1**: Add `JiraFieldMappingRepository` interface
  - **ID**: `task-2.1`
  - **BlockedBy**: `task-1.4`
  - **Agent**: `general-purpose`
  - **File**: `internal/domains/integrations/repository.go`
  - **Change**: Add the new interface with `Get`, `Upsert`, `Delete`
    methods alongside the existing `JiraConnectionRepository`.
  - **Outcome**: Compiles; no implementation yet.
  - **Context**: Design "Repository interface" section. Requirement 5 ac1,
    5 ac3.

- [x] **Task 2.2**: Write failing tests for `GormJiraFieldMappingRepository`
  - **ID**: `task-2.2`
  - **BlockedBy**: `task-1.2, task-2.1`
  - **Agent**: `general-purpose`
  - **File**: `internal/infrastructure/repositories/gorm_jira_field_mapping_repository_test.go`
  - **Change**: Using `go-sqlmock`, assert insert-path on first upsert,
    update-path on second upsert (same `project_id`), `Get` returns
    aggregate; `Get` returns `nil, nil` when no row; soft-deleted rows
    are excluded from `Get`; `Delete` performs a soft delete via
    `deleted_at`.
  - **Outcome**: Tests fail with "undefined: GormJiraFieldMappingRepository".
  - **Context**: Match the test style of
    `gorm_jira_connection_repository_test.go` if one exists; otherwise
    follow the `go-sqlmock` pattern documented in `go.mod`.

- [x] **Task 2.3**: Implement `GormJiraFieldMappingRepository`
  - **ID**: `task-2.3`
  - **BlockedBy**: `task-2.2`
  - **Agent**: `general-purpose`
  - **File**: `internal/infrastructure/repositories/gorm_jira_field_mapping_repository.go`
  - **Change**: Implement the repository against a new GORM model
    `database.JiraFieldMapping` (add the model alongside the existing
    `database.JiraConnection`). Serialize `entries` to JSONB; deserialize
    on read; reconstruct the domain aggregate via a `Reconstruct...`
    helper added to `jira_field_mapping.go` (analogous to
    `ReconstructJiraConnection`).
  - **Outcome**: Tests from Task 2.2 pass.
  - **Context**: Use the upsert pattern `ON CONFLICT (project_id) DO
    UPDATE`. Wrap reads in `WithContext(ctx)`.

### Phase 3: JIRA Client Extension (parallel with Phases 1-2)

- [x] **Task 3.1**: Write failing tests for `DefaultJiraClient.ListFields`
  - **ID**: `task-3.1`
  - **BlockedBy**: `none`
  - **Agent**: `general-purpose`
  - **File**: `internal/domains/integrations/jira_client_test.go`
  - **Change**: Using `httptest.NewServer`, mock JIRA's
    `/rest/api/2/field` endpoint and assert: the request includes the
    correct auth header for each `AuthenticationType`; the response is
    parsed into `[]JiraField`; entries where `Custom == true` are
    filtered out; the result is sorted by `Name` ascending; HTTP non-200
    surfaces as a wrapped error.
  - **Outcome**: Tests fail at `c.ListFields(...)` (method missing).
  - **Context**: Design decision 5 (filter custom fields server-side).
    Mirror the `TestConnection` HTTP test style from `jira_client.go`.

- [x] **Task 3.2**: Add `ListFields` to `JiraClient` interface and
  `DefaultJiraClient`
  - **ID**: `task-3.2`
  - **BlockedBy**: `task-3.1`
  - **Agent**: `general-purpose`
  - **File**: `internal/domains/integrations/jira_client.go` (and the
    `JiraClient` interface in `jira_connection.go`)
  - **Change**: Add `ListFields(ctx, url, username, credential string,
    authType AuthenticationType) ([]JiraField, error)` to the interface
    and implement it on `DefaultJiraClient`. Filter custom fields and
    sort by name.
  - **Outcome**: Tests from Task 3.1 pass. Any existing fake/mock
    `JiraClient` implementations updated to satisfy the new interface.
  - **Context**: Design "JIRA client extension" snippet. Watch for fakes
    in test helpers and acceptance harness — update them too or the
    build breaks.

### Phase 4: Service Layer

- [x] **Task 4.1**: Write failing tests for `JiraFieldMappingService`
  - **ID**: `task-4.1`
  - **BlockedBy**: `task-1.4, task-2.1`
  - **Agent**: `general-purpose`
  - **File**: `internal/domains/integrations/jira_field_mapping_service_test.go`
  - **Change**: Using fake repos for `JiraFieldMappingRepository` and
    `JiraConnectionRepository`, assert: `Get` returns the saved snapshot
    when present; `Get` returns the default snapshot when no row exists
    (defaults per Requirement 1 ac3); `Save` rejects when no active JIRA
    connection exists for the project (`ErrNoJiraConnection`); `Save`
    rejects invalid input with typed errors; `Save` upserts on success
    and records `updated_by` from a passed-through user-id parameter.
  - **Outcome**: Tests fail with "undefined: JiraFieldMappingService".
  - **Context**: Requirements 1 ac3, 2 ac3, 3 ac3, 4 ac1, 5 ac1, 5 ac4,
    8 ac2. Defaults set is fixed; encode it as a package-level
    `defaultFernToJiraMapping` map.

- [x] **Task 4.2**: Implement `JiraFieldMappingService`
  - **ID**: `task-4.2`
  - **BlockedBy**: `task-4.1`
  - **Agent**: `general-purpose`
  - **File**: `internal/domains/integrations/jira_field_mapping_service.go`
  - **Change**: Constructor `NewJiraFieldMappingService(mappingRepo,
    connRepo)`. Methods `Get(ctx, projectID) (*JiraFieldMappingSnapshot,
    error)` and `Save(ctx, projectID string, entries
    []FieldMappingEntry, updatedBy string) (*JiraFieldMappingSnapshot,
    error)`. Emit logrus structured logs for save and validation
    failures.
  - **Outcome**: Tests from Task 4.1 pass.
  - **Context**: Requirement 9 ac5 (structured logs). Validation runs
    inside `JiraFieldMapping.UpdateEntries` so errors propagate from the
    aggregate.

### Phase 5: GraphQL Surface

- [x] **Task 5.1**: Extend GraphQL schema
  - **ID**: `task-5.1`
  - **BlockedBy**: `task-4.2, task-3.2`
  - **Agent**: `general-purpose`
  - **File**: `internal/reporter/graphql/schema.graphql`
  - **Change**: Add `enum FernField`, `enum ReductionStrategy`, `type
    JiraField`, `type FieldMappingEntry`, `type JiraFieldMapping`, input
    types, and `extend type Query { jiraFieldMapping, jiraFields }` +
    `extend type Mutation { saveJiraFieldMapping, resetJiraFieldMapping }`
    per the Design "GraphQL schema additions" snippet.
  - **Outcome**: `go generate ./...` regenerates resolvers without
    errors; the new types appear under `internal/reporter/graphql/generated/`.
  - **Context**: Design decision 4 (GraphQL only). Schema-first; do not
    hand-edit generated files.

- [x] **Task 5.2**: Write failing resolver tests
  - **ID**: `task-5.2`
  - **BlockedBy**: `task-5.1`
  - **Agent**: `general-purpose`
  - **File**: `internal/reporter/graphql/domain_resolvers_jira_mapping_test.go`
  - **Change**: Tests asserting: a non-manager / non-admin caller is
    denied (matches post-PR #156 pattern, see memory note on the auth
    regression); a manager loads the mapping; saving with invalid input
    returns a typed validation error; saving with valid input returns
    the saved snapshot; `jiraFields` resolver calls
    `JiraClient.ListFields` with the credentials of the project's active
    connection.
  - **Outcome**: Tests fail because resolvers are not yet wired.
  - **Context**: Requirement 7 ac1, 7 ac2. Reuse the role-extraction
    helper used in `RecentTestRuns_domain` and `TestRuns_domain` to keep
    auth consistent.

- [x] **Task 5.3**: Wire resolvers to the service
  - **ID**: `task-5.3`
  - **BlockedBy**: `task-5.2`
  - **Agent**: `general-purpose`
  - **File**: `internal/reporter/graphql/domain_resolvers.go`
  - **Change**: Implement the four resolvers: `JiraFieldMapping_domain`,
    `JiraFields_domain`, `SaveJiraFieldMapping_domain`,
    `ResetJiraFieldMapping_domain`. Each runs the standard auth check
    first, then delegates to the service. Map domain errors to GraphQL
    errors with stable codes.
  - **Outcome**: Tests from Task 5.2 pass; existing resolver tests
    continue to pass.
  - **Context**: Follow the resolver style elsewhere in
    `domain_resolvers.go`. Inject the new service through the existing
    `factory.go` wiring.

- [x] **Task 5.4**: Wire services into the integrations factory and main
  - **ID**: `task-5.4`
  - **BlockedBy**: `task-4.2, task-2.3`
  - **Agent**: `general-purpose`
  - **File**: `internal/domains/integrations/factory.go`
  - **Change**: Add a constructor for `JiraFieldMappingService` and
    expose it on the integrations factory; update `cmd/fern-platform/
    main.go` to instantiate the GORM repo and pass the service to the
    GraphQL layer.
  - **Outcome**: `make build` succeeds; the server starts and the new
    GraphQL operations respond.
  - **Context**: Match the pattern used to wire `JiraConnectionService`.

### Phase 6: Frontend

- [x] **Task 6.1**: Wire `FieldMappingModal` to real GraphQL data
  - **ID**: `task-6.1`
  - **BlockedBy**: `task-5.3`
  - **Agent**: `general-purpose`
  - **File**: `web/index.html` (`FieldMappingModal` component, line ~7638)
  - **Change**: Replace the hardcoded JIRA field list in `fetchJiraFields`
    with a real `jiraFields(connectionId)` GraphQL query; replace
    `loadExistingMappings` with a real `jiraFieldMapping(projectId)` query;
    uncomment and implement the `SAVE_FIELD_MAPPINGS` mutation call using
    `saveJiraFieldMapping`; wire the Reset button to `resetJiraFieldMapping`.
    Align Fern field IDs with domain constants: rename `title` →
    `requirement_title`, `type` → `requirement_type`, `release` →
    `release_version`, `status` → `requirement_status`. Remove the two
    extra fields (`priority`, `component`) that are not in the spec's
    eight-field set. Add the multi-value reduction strategy prompt when a
    single-value Fern field is mapped to a multi-value JIRA field (Req 4).
  - **Outcome**: Manual smoke test against `make deploy-all` shows the
    modal loads real JIRA fields, save round-trips and survives a reload,
    validation matches Req 2/3/4.
  - **Context**: Requirements 1, 2, 3, 4, 6. Design decision 7. The
    drag-and-drop interaction and SVG lines are already working; this task
    is purely about connecting them to the backend.

- [x] **Task 6.2**: Hide mapping UI / show prompt when no connection
  - **ID**: `task-6.2`
  - **BlockedBy**: `task-6.1`
  - **Agent**: `general-purpose`
  - **File**: `web/index.html` (`FieldMappingModal` component)
  - **Change**: When the `jiraConnection` query returns no active connection,
    hide the drag-and-drop columns and render a short prompt directing the
    user to configure a JIRA connection first, with a scroll-anchor to the
    connection panel above.
  - **Outcome**: Manual test: a project without a JIRA connection shows the
    prompt and no error toast.
  - **Context**: Requirement 6 ac4.

### Phase 7: Acceptance and Wrap-Up

- [x] **Task 7.1**: Mock JIRA server `/rest/api/2/field` endpoint *(DONE — issue #25)*
  - **ID**: `task-7.1`
  - **BlockedBy**: `none`
  - **File**: `acceptance/helpers/mock_jira_server.go`
  - **Status**: `handleFields` at line 88 already returns a deterministic
    fixture of standard fields including one custom field. No further work
    needed.

- [x] **Task 7.2**: End-to-end acceptance test
  - **ID**: `task-7.2`
  - **BlockedBy**: `task-5.3, task-6.1, task-7.1`
  - **Agent**: `general-purpose`
  - **File**: `acceptance/jira_field_mapping_test.go` (already exists from
    issue #25 — extend rather than create)
  - **Change**: Add Ginkgo specs covering the backend persistence path:
    (a) save a valid mapping via GraphQL and verify it persists on reload;
    (b) attempt a conflicting save (duplicate JIRA field) and assert the
    typed validation error; (c) attempt a save with a required field
    unmapped and assert the validation error; (d) attempt a read and write
    as a non-manager and assert access denial. The existing Playwright UI
    specs from #25 can remain; add GraphQL-level specs alongside them.
  - **Outcome**: `make test-acceptance` passes including the new specs.
  - **Context**: Build on the existing mock JIRA scaffolding and helpers
    already in the file.

- [x] **Task 7.2a**: Fix: `TestConnection` did not activate the connection
  - **ID**: `task-7.2a`
  - **BlockedBy**: `task-7.2`
  - **File**: `internal/domains/integrations/service.go`,
    `internal/domains/integrations/service_test.go`
  - **Change**: `JiraConnectionService.TestConnection` called `conn.TestConnection`
    (which sets `status=connected`) but never called `conn.Activate()`, so
    `is_active` remained `false` in the database. Fix: call `conn.Activate()`
    before `repo.Update` on the success path. Added three new unit tests in
    `service_test.go` verifying activation on success, no activation on failure,
    and encrypted-credential preservation.
  - **Outcome**: Tests pass; `is_active=true` is now persisted after a successful
    connection test.

- [x] **Task 7.2b**: Fix: `jiraFieldMapping.updatedAt` null violation on default snapshot
  - **ID**: `task-7.2b`
  - **BlockedBy**: `task-7.2`
  - **File**: `internal/reporter/graphql/schema.graphql`,
    `internal/reporter/graphql/domain_resolvers_jira_mapping.go`
  - **Change**: gqlgen's `MarshalTime` returns `graphql.Null` for zero
    `time.Time` values. `defaultSnapshot()` returns a zero `UpdatedAt`, which
    violated the `updatedAt: Time!` schema constraint. Fix: changed `updatedBy`
    and `updatedAt` to nullable (`String` / `Time`) in the schema and regenerated.
    `mappingSnapshotToModel` now conditionally sets the pointer fields only when
    non-zero/non-empty.
  - **Outcome**: `jiraFieldMapping` query succeeds for projects with no saved
    mapping (returns defaults with null `updatedBy`/`updatedAt`).

- [x] **Task 7.2c**: Design fix: allow mapping save without a tested connection
  - **ID**: `task-7.2c`
  - **BlockedBy**: `task-7.2`
  - **File**: `internal/domains/integrations/jira_field_mapping_service.go`,
    `internal/domains/integrations/jira_field_mapping_service_test.go`
  - **Change**: `Save` previously called `FindActiveByProjectID`, blocking saves
    until the connection had been tested. This was overly restrictive — field
    mapping is configuration metadata independent of connection liveness. Changed
    to `FindByProjectID` so saves succeed as long as any connection is configured.
    Updated the corresponding service test.
  - **Outcome**: Users can configure field mappings immediately after creating a
    connection, without needing to test it first.

- [x] **Task 7.3**: End-to-end manual verification and docs
  - **ID**: `task-7.3`
  - **BlockedBy**: `task-7.2, task-6.2`
  - **Agent**: `general-purpose`
  - **File**: `docs/jira-integration.md` (or section in existing
    `docs/integrations.md`)
  - **Change**: Document the Field Mapping configuration steps, the
    defaults, the required fields, and the multi-value reduction
    options. Run `make deploy-all`, hit the screen as a manager and as
    a non-manager, save a mapping, verify it survives a pod restart.
  - **Outcome**: `feature_list.json` (if present) is updated with
    `passes: true` for this feature. Otherwise, manual verification is
    captured in the PR description.
  - **Context**: Spec-first-TDD rule: a task is not complete until the
    feature has been exercised end-to-end. Also satisfies the
    "Verify end-to-end (MANDATORY)" step in the workflow.

- [x] **Task 7.3a**: Fix: `ON CONFLICT` predicate mismatch in upsert
  - **ID**: `task-7.3a`
  - **BlockedBy**: `task-7.3`
  - **File**: `internal/infrastructure/repositories/gorm_jira_field_mapping_repository.go`
  - **Change**: The migration created a partial unique index (`WHERE deleted_at IS NULL`)
    but the upsert used `ON CONFLICT ("project_id")` without the predicate, causing
    `SQLSTATE 42P10`. Fixed by changing the conflict target to
    `ON CONFLICT ("project_id") WHERE "deleted_at" IS NULL`.
  - **Outcome**: `saveJiraFieldMapping` succeeds for projects with no prior saved mapping.

- [x] **Task 7.4**: Refactor and deduplicate pass
  - **ID**: `task-7.4`
  - **BlockedBy**: `task-7.3`
  - **Agent**: `general-purpose`
  - **File**: (multiple — changed files in this spec)
  - **Change**: Scan for duplicated validation logic between the
    aggregate, service, and resolver; extract helpers as needed; ensure
    no function is doing more than one thing. Run `make test` and
    `make lint` after each refactor.
  - **Outcome**: All tests green; no obvious duplication remains.
  - **Context**: Spec-first-TDD "After implementation, before
    verification" pass.

## Dependency Diagram

```
task-1.1 (types) ──┬──▶ task-1.3 (agg tests) ──▶ task-1.4 (agg impl) ──┬──▶ task-2.1 (repo iface) ──┬──▶ task-2.2 (repo tests) ──▶ task-2.3 (repo impl) ──┐
                   │                                                    │                            │                                                    │
task-1.2 (mig)  ───┤                                                    │                            │                                                    │
                   │                                                    │                            ▼                                                    │
                   └────────────────────────────────────────────────────┴──────────────────────────▶ task-4.1 (svc tests) ──▶ task-4.2 (svc impl) ──┐    │
                                                                                                                                                     │    │
task-3.1 (client tests) ──▶ task-3.2 (client impl) ─────────────────────────────────────────────────────────────────────────────────────────────────┤    │
                                                                                                                                                     ▼    ▼
                                                                                                                              task-5.1 (gql schema) ──┬──▶ task-5.4 (factory wiring)
                                                                                                                                                      │
                                                                                                                              task-5.2 (resolver tests) ──▶ task-5.3 (resolver impl)
                                                                                                                                                      │
                                                                                                                                                      ▼
                                                                                                                                         task-6.1 (wire frontend) ──▶ task-6.2 (no-conn prompt)
                                                                                                                                                      │
[task-7.1 DONE] ──────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
                                                                                                                                                      ▼
                                                                                                                                         task-7.2 (acceptance)
                                                                                                                                                      │
                                                                                                                                                      ▼
                                                                                                                                         task-7.3 (manual + docs) ──▶ task-7.4 (refactor)
```

**Parallel opportunities**:
- `task-1.1`, `task-1.2`, `task-3.1` are all root tasks (`BlockedBy: none`) and can run simultaneously.
- After `task-1.4` lands, `task-2.1` (repo interface) and `task-4.1` (service tests) both unblock — the service track and persistence track can progress in parallel.
- `task-3.2` (JIRA client) progresses independently of the persistence track until Phase 5 converges.
- `task-7.1` is already done.

**Critical path**: `task-1.1` → `task-1.3` → `task-1.4` → `task-4.1` → `task-4.2` → `task-5.1` → `task-5.2` → `task-5.3` → `task-6.1` → `task-7.2` → `task-7.3` → `task-7.4` (12 tasks). Shortening Phase 4 or Phase 5 is the highest-leverage place to compress the schedule.

## Completion Criteria

The feature is complete when all of the following hold:

1. All tasks above are checked.
2. `make test` (unit), `make test-acceptance`, and `make lint` pass on
   `feature/26-jira-field-mapping`.
3. The Field Mapping screen renders, persists, and validates as described
   in `requirements.md` against a locally deployed stack
   (`make deploy-all`).
4. A non-manager / non-admin user receives an access-denied response on
   read and write.
5. `make build` produces a binary with the new GraphQL operations
   wired through `internal/domains/integrations/factory.go`.
6. The PR description references issue #26 and links the spec directory.
7. `/spec:verify` reports no gaps against `requirements.md`.
