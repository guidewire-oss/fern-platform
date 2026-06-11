# Tasks: JIRA Requirements Sync

## Implementation Tasks

### Phase 1: Foundation (parallel root tasks)

- [ ] **Task 1.1**: Migration for the four new tables
  - **ID**: `task-1.1`
  - **BlockedBy**: `none`
  - **Agent**: `general-purpose`
  - **File**: `migrations/000023_create_requirements_sync_tables.up.sql` (+ down)
  - **Change**: Single migration creating `requirements`, `spec_run_requirements`, `sync_runs`, `project_jira_sync_settings`. Schemas exactly per design.md "Data Model" section. Include all CHECK constraints, partial unique indexes (especially `sync_runs (project_id, trigger_source) WHERE status='running'` for Req 6.2 concurrency guard), and FK cascade rules. Down migration drops in FK-safe order (`spec_run_requirements` → `requirements` → `sync_runs` → `project_jira_sync_settings`).
  - **Outcome**: `make migrate` applies cleanly; `make migrate-down` cleanly reverts; an empty `requirements` table exists with all indexes.
  - **Context**: Reqs 4–8 (data model + release window + audit). Follow pattern of `migrations/000022_create_jira_field_mappings_table.up.sql`. Never edit a merged migration.

- [ ] **Task 1.2**: Failing tests for `JiraClient.SearchIssues`
  - **ID**: `task-1.2`
  - **BlockedBy**: `none`
  - **Agent**: `general-purpose`
  - **File**: `internal/domains/integrations/jira_client_test.go`
  - **Change**: Add Ginkgo describe blocks for `SearchIssues`. Test cases: single-page result, multi-page pagination (verify `startAt` follows server's reported `total`), 429 with `Retry-After` header (verify backoff respected), 5xx retry chain (max 5 retries cap), 401 fail-fast, malformed JSON response, network timeout. Use `httptest.NewServer` per existing pattern in `jira_client_test.go`.
  - **Outcome**: Test file compiles; all new specs fail (red phase).
  - **Context**: Reqs 1.3, 6.1. TDD per `.claude/rules/spec-first-tdd.md` — write tests before implementation.

- [ ] **Task 1.3**: Extend mock JIRA server for sync endpoints
  - **ID**: `task-1.3`
  - **BlockedBy**: `none`
  - **Agent**: `general-purpose`
  - **File**: `acceptance/helpers/mock_jira_server.go` and `mock-jira/`
  - **Change**: Add handlers for `GET /rest/api/2/search` (params: `jql`, `startAt`, `maxResults`, `fields`). Support a configurable fixture set so acceptance tests can simulate 250 issues across multiple pages with realistic Gherkin in descriptions for at least 3 of them. Honor `Retry-After` headers and rate-limit simulation hooks.
  - **Outcome**: Mock server responds correctly to paginated search; existing acceptance tests still pass.
  - **Context**: Used by Phase 8 acceptance tests; pattern matches existing mock endpoints for `/myself` and `/field`.

- [ ] **Task 1.4**: GraphQL schema definitions (no resolvers yet)
  - **ID**: `task-1.4`
  - **BlockedBy**: `none`
  - **Agent**: `general-purpose`
  - **File**: `internal/reporter/graphql/schema.graphql`
  - **Change**: Add the new enums (`SyncRunStatus`, `SyncTriggerSource`, `RequirementType`, `RequirementSource`, `MappingSource`), types (`Requirement`, `JiraSyncRun`, `ProjectJiraSyncSettings`, `RequirementConnection`), inputs (`StartJiraSyncInput`, `ProjectJiraSyncSettingsInput`), and the new fields on `Query` / `Mutation`. Verbatim per design.md "GraphQL surface" section. Run `make gqlgen` to regenerate.
  - **Outcome**: gqlgen regen succeeds; generated code compiles; resolver stubs are auto-created.
  - **Context**: Req 1.5–1.6 (async polling), Req 11 (inventory query). Resolvers implemented in Phase 6.

### Phase 2: Domain layer

- [ ] **Task 2.1**: `Requirement` aggregate + failing tests
  - **ID**: `task-2.1`
  - **BlockedBy**: `task-1.1`
  - **Agent**: `general-purpose`
  - **File**: `internal/domains/requirements/domain/requirement.go`, `requirement_test.go`
  - **Change**: Implement `Requirement` aggregate per design.md "Domain types". Constructor `NewRequirement(...)` validates (non-empty title, valid type, source matches column constraints, scenario requires parent). `UpdateFromJira(issue)` recomputes `description_hash`, returns `bool` indicating whether description changed (drives reparse decision). `MarkOrphaned()` sets `orphaned=true`. `Snapshot()` returns read-only DTO. `ReconstructRequirement(...)` bypasses validation for repo hydration.
  - **Outcome**: All aggregate tests pass; invariants enforced at construction.
  - **Context**: Req 4. Mirror the pattern from `internal/domains/integrations/jira_field_mapping.go` (PR 188).

- [ ] **Task 2.2**: `SyncRun` aggregate + tests
  - **ID**: `task-2.2`
  - **BlockedBy**: `task-1.1`
  - **Agent**: `general-purpose`
  - **File**: `internal/domains/requirements/domain/sync_run.go`, `sync_run_test.go`
  - **Change**: `SyncRun` aggregate with state-machine helpers: `MarkSucceeded(counts)`, `MarkFailed(errSummary)`, `MarkCancelled()`. Invariants: status transitions only `running → {succeeded, failed, cancelled}`; counts can only increase. Validate `trigger_source` against the enum at construction.
  - **Outcome**: State machine tests pass; illegal transitions rejected.
  - **Context**: Reqs 5, 6.2 (concurrency guard at DB enforced separately).

- [ ] **Task 2.3**: `ProjectJiraSyncSettings` aggregate + tests
  - **ID**: `task-2.3`
  - **BlockedBy**: `task-1.1`
  - **Agent**: `general-purpose`
  - **File**: `internal/domains/requirements/domain/project_sync_settings.go`, test file
  - **Change**: Settings aggregate with defaults applied at construction. Validation: `ReleaseWindowDays ∈ [7, 1825]`; `StalenessThresholdHours > 0`; if `GherkinLLMExtractionEnabled=true` then `LLMProvider` must be one of the valid values. `NewDefault(projectID)` constructor returns a fully-default instance (used when no row exists).
  - **Outcome**: Tests pass; invalid values rejected at construction.
  - **Context**: Reqs 8 (release window), 9.6 (LLM disabled by default).

- [ ] **Task 2.4**: Repository interfaces in domain package
  - **ID**: `task-2.4`
  - **BlockedBy**: `task-2.1`, `task-2.2`, `task-2.3`
  - **Agent**: `general-purpose`
  - **File**: `internal/domains/requirements/domain/repository.go`
  - **Change**: Define the four repository interfaces exactly per design.md "Repository interfaces". Pure Go interfaces; no implementation. Each method documented with its contract (idempotency for `Upsert`, return semantics for `Get*`, etc.).
  - **Outcome**: Compiles; mocks generated via `mockgen` (or hand-written) in Phase 3 tests.
  - **Context**: Pattern from `internal/domains/integrations/repository.go`.

### Phase 3: Repositories (GORM implementations)

- [ ] **Task 3.1**: `gorm_requirement_repository.go` + tests
  - **ID**: `task-3.1`
  - **BlockedBy**: `task-1.1`, `task-2.4`
  - **Agent**: `general-purpose`
  - **File**: `internal/infrastructure/repositories/gorm_requirement_repository.go`, test file
  - **Change**: Implement `RequirementRepository`. `Upsert` uses raw SQL `INSERT … ON CONFLICT (project_id, jira_key) WHERE source='jira_sync' DO UPDATE …` (matches partial unique index). `ListByProject` honors `type`, `status`, `fix_version` filters. `GetKeysWithStaleDescription` compares passed-in `description_hash` map against stored values and returns IDs needing reparse. Use `database/sql.NullString` for nullable JIRA key.
  - **Outcome**: Repo tests pass against an in-memory SQLite or testcontainer Postgres; SQL assertions verify partial-index behavior.
  - **Context**: Pattern from `gorm_jira_field_mapping_repository.go`. Watch out for the soft-delete-tombstone trap PR 188 had (handle Reset → Save by clearing `deleted_at` on upsert).

- [ ] **Task 3.2**: `gorm_sync_run_repository.go` + tests
  - **ID**: `task-3.2`
  - **BlockedBy**: `task-1.1`, `task-2.4`
  - **Agent**: `general-purpose`
  - **File**: `internal/infrastructure/repositories/gorm_sync_run_repository.go`, test file
  - **Change**: Implement `SyncRunRepository`. `Create` honors the partial unique index — if it raises a unique violation, return a typed `ErrSyncAlreadyRunning` (consumed by the GraphQL resolver to return a friendly 409-equivalent). `UpdateStatus` accepts a typed `SyncRunUpdate` struct with optional fields; updates only what's set. `HasRunning` is a fast existence check.
  - **Outcome**: Tests pass; concurrent-create test verifies the second goroutine gets `ErrSyncAlreadyRunning`.
  - **Context**: Req 6.2 concurrency at DB layer.

- [ ] **Task 3.3**: `gorm_spec_run_requirement_repository.go` + tests
  - **ID**: `task-3.3`
  - **BlockedBy**: `task-1.1`, `task-2.4`
  - **Agent**: `general-purpose`
  - **File**: `internal/infrastructure/repositories/gorm_spec_run_requirement_repository.go`, test file
  - **Change**: Implement `BackfillForKeys(projectID, keys, windowDays)` — single SQL `INSERT INTO spec_run_requirements (...) SELECT ... FROM spec_runs sr JOIN spec_run_tags ... JOIN tags t ... WHERE tr.created_at >= NOW() - INTERVAL '<windowDays> days' AND t.name LIKE 'jira:%' AND substring(t.name, 6) = ANY($keys) ON CONFLICT (spec_run_id, requirement_id) DO NOTHING`. Return `(insertedCount, error)`. Resolve scenario vs issue binding inline (look for sibling `scenario:TITLE` tag in the same spec_run).
  - **Outcome**: Tests pass; idempotency test verifies running twice returns 0 second time.
  - **Context**: Req 10. Mapping rules per [`adr-mapping-lifecycle.md`](../../adr/test-correlation/mapping-lifecycle.md).

- [ ] **Task 3.4**: `gorm_project_sync_settings_repository.go` + tests
  - **ID**: `task-3.4`
  - **BlockedBy**: `task-1.1`, `task-2.4`
  - **Agent**: `general-purpose`
  - **File**: `internal/infrastructure/repositories/gorm_project_sync_settings_repository.go`, test file
  - **Change**: `Get` returns a default-populated aggregate when no row exists (uses `NewDefault(projectID)`). `Save` is upsert on `project_id` PK. Standard CRUD; no fancy partial indexes.
  - **Outcome**: Tests pass; verifies default-when-missing behavior.
  - **Context**: Reqs 8, 9.6.

### Phase 4: Gherkin parser (Tier 1)

- [ ] **Task 4.1**: Markdown AST extractor + tests
  - **ID**: `task-4.1`
  - **BlockedBy**: `none`
  - **Agent**: `general-purpose`
  - **File**: `internal/domains/requirements/application/gherkin_parser.go`, test file
  - **Change**: Function `ExtractCodeBlocksWithHeadings(markdown string) []CodeBlock` using `github.com/yuin/goldmark` to walk the AST. Each `CodeBlock` carries: the block content, the nearest preceding heading's text + level, and source line ranges (for error reporting). Handle fenced (```), tilde (~~~), and indented code blocks.
  - **Outcome**: Unit tests pass against golden inputs in `internal/domains/requirements/application/testdata/`.
  - **Context**: Req 9.1, 9.2. Pre-step for the Gherkin parser. Tested via the DE-5249 and GWCP-97108 fixture descriptions.

- [ ] **Task 4.2**: Gherkin parser integration + tests
  - **ID**: `task-4.2`
  - **BlockedBy**: `task-4.1`
  - **Agent**: `general-purpose`
  - **File**: `internal/domains/requirements/application/gherkin_parser.go` (extend), test file (extend)
  - **Change**: Function `ParseScenarios(blocks []CodeBlock) []ParsedScenario` using `github.com/cucumber/gherkin/v32/go`. Each `ParsedScenario` carries: title, gherkin body, parent heading hint, plus `IsOutline bool` and (for outlines) the expanded `Examples` rows. Non-Gherkin code blocks (e.g., a `bash` snippet) are silently skipped — Gherkin parse errors don't abort the whole issue.
  - **Outcome**: Tests pass for: simple scenarios, Scenario Outline with Examples, DocString step args, non-Gherkin code blocks ignored, localized keywords (`Scénario:`).
  - **Context**: Req 9.2, 9.3. Library: `github.com/cucumber/gherkin/v32/go`. Add to `go.mod`.

- [ ] **Task 4.3**: Scenario reconciliation against existing rows + tests
  - **ID**: `task-4.3`
  - **BlockedBy**: `task-3.1`, `task-4.2`
  - **Agent**: `general-purpose`
  - **File**: `internal/domains/requirements/application/gherkin_parser.go` (extend), test file (extend)
  - **Change**: Function `ReconcileScenarios(ctx, parentID, freshScenarios []ParsedScenario, repo RequirementRepository) (added, updated, orphaned int, error)`. Existing scenario rows under `parentID` are matched by title (case-insensitive, whitespace-collapsed); matched rows are updated, unmatched-fresh are inserted, unmatched-existing are marked `orphaned=true` (not deleted; preserves `spec_run_requirements` joins).
  - **Outcome**: Tests pass for: rename detection, scenario removal → orphan, new scenario addition, idempotent re-run (zero changes the second time).
  - **Context**: Req 4.5, 9.7.

- [ ] **Task 4.4**: Tier 2 LLM extractor interface + stub (no real provider yet)
  - **ID**: `task-4.4`
  - **BlockedBy**: `task-4.2`
  - **Agent**: `general-purpose`
  - **File**: `internal/domains/requirements/application/llm_extractor.go`, test file
  - **Change**: Interface `LLMScenarioExtractor.Extract(ctx, description string) ([]ParsedScenario, error)`. Stub implementation `NoopLLMExtractor` that always returns empty — wired in when feature flag is off. Real provider impls deferred to follow-on issue. Document the prompt shape and strict-JSON output expectation in a doc comment.
  - **Outcome**: Tests verify Noop returns empty; interface ready for real implementations.
  - **Context**: Req 9.4–9.6. ADR [`gherkin-parsing-tiers.md`](../../adr/test-correlation/gherkin-parsing-tiers.md).

### Phase 5: Sync orchestration

- [ ] **Task 5.1**: `JiraClient.SearchIssues` implementation
  - **ID**: `task-5.1`
  - **BlockedBy**: `task-1.2`
  - **Agent**: `general-purpose`
  - **File**: `internal/domains/integrations/jira_client.go`
  - **Change**: Implement to make Task 1.2's tests pass. Honor `Retry-After`; cap retries at 5; cap backoff at 30s; respect `JIRA_SYNC_MAX_PAGE` env. Return typed `JiraIssue` slice + `total int` + `nextStartAt int`.
  - **Outcome**: All Task 1.2 tests pass (green phase).
  - **Context**: Req 6.1.

- [ ] **Task 5.2**: `SyncService` skeleton + state machine wiring
  - **ID**: `task-5.2`
  - **BlockedBy**: `task-2.2`, `task-3.1`, `task-3.2`, `task-3.3`, `task-3.4`, `task-5.1`
  - **Agent**: `general-purpose`
  - **File**: `internal/domains/requirements/application/sync_service.go`, test file
  - **Change**: `SyncService.StartSync(ctx, input) (*SyncRun, error)` — synchronous part: validate, persist `sync_runs` row (`status='running'`), kick off goroutine for the actual sync. The goroutine: fetch → parse → upsert → backfill → finalize. Use the repo concurrency guard (Task 3.2) — if another sync is running, return `ErrSyncAlreadyRunning` synchronously without persisting a row.
  - **Outcome**: Tests with mock repos + mock JIRA client verify the five-step pipeline; concurrency test verifies guard rejects second invocation.
  - **Context**: Reqs 1.5, 6.2, 6.3.

- [ ] **Task 5.3**: Pagination, rate-limit backoff, per-issue error tolerance
  - **ID**: `task-5.3`
  - **BlockedBy**: `task-5.2`
  - **Agent**: `general-purpose`
  - **File**: `internal/domains/requirements/application/sync_service.go` (extend), test file (extend)
  - **Change**: Fetch loop pages until JIRA reports no more results. Per-issue: try to upsert, catch errors, increment `failed_count`, continue. Update `sync_runs` progress every page (so UI polling reflects progress). Mark sync `failed` only on auth failure or repeated JIRA unreachability past retry cap.
  - **Outcome**: Tests pass for: 250-issue fixture across pages, single-issue failure doesn't abort sync, JIRA returns 401 → sync fails fast, JIRA flakes for 3 pages then recovers → sync succeeds.
  - **Context**: Reqs 1.3, 1.4, 1.7, 2.4, 6.1.

- [ ] **Task 5.4**: Description-hash-based reparse trigger
  - **ID**: `task-5.4`
  - **BlockedBy**: `task-3.1`, `task-4.3`, `task-5.2`
  - **Agent**: `general-purpose`
  - **File**: `internal/domains/requirements/application/sync_service.go` (extend), test file (extend)
  - **Change**: For each issue, compute SHA-256 of description. If matches stored `description_hash` on the existing row, skip the parser. If differs (or no existing row), run Tier 1 parser + reconciler. Store the new hash on upsert.
  - **Outcome**: Tests pass for: changed description triggers reparse, unchanged description skips parser (verified via parser-not-called assertion).
  - **Context**: Req 4.5, 9.1. Design Decision 3.

- [ ] **Task 5.5**: Path B backfill inline call
  - **ID**: `task-5.5`
  - **BlockedBy**: `task-3.3`, `task-5.2`
  - **Agent**: `general-purpose`
  - **File**: `internal/domains/requirements/application/sync_service.go` (extend), test file (extend)
  - **Change**: After all issues processed, call `SpecRunRequirementRepository.BackfillForKeys(projectID, syncedKeys, releaseWindowDays)`. Store returned count in `sync_runs.mappings_created`. Failures here log a warn but do NOT fail the whole sync.
  - **Outcome**: Tests verify backfill called with the right key set; failure path logs warning and still emits `status=succeeded`.
  - **Context**: Req 10.1, 10.5.

- [ ] **Task 5.6**: Incremental + Staleness trigger variants
  - **ID**: `task-5.6`
  - **BlockedBy**: `task-5.3`
  - **Agent**: `general-purpose`
  - **File**: `internal/domains/requirements/application/sync_service.go` (extend), test file (extend)
  - **Change**: Incremental: prepend `updated > <last_succeeded.started_at>` clause to the JQL. Staleness: same as incremental but scoped to `parent in (<epic_key>, <descendant_keys>)`. Orphan handling: on incremental, any synced-in-prior-runs keys that aren't returned this time are marked `orphaned=true`.
  - **Outcome**: Tests verify the JQL composition and orphan-marking.
  - **Context**: Reqs 2, 3.

### Phase 6: API surface (GraphQL resolvers)

- [ ] **Task 6.1**: `startJiraSync`, `cancelJiraSync` mutations + tests
  - **ID**: `task-6.1`
  - **BlockedBy**: `task-1.4`, `task-5.2`
  - **Agent**: `general-purpose`
  - **File**: `internal/reporter/graphql/domain_resolvers_jira_sync.go`, test file
  - **Change**: Implement the two mutation resolvers. Auth: use shared `authorizeProjectManagement(ctx, projectID)` helper (introduce in this PR if PR #181 didn't already land it). Map `ErrSyncAlreadyRunning` to a clear GraphQL error. `cancelJiraSync` calls `SyncService.Cancel(syncRunID)` (interrupts the goroutine; sets status='cancelled').
  - **Outcome**: Resolver tests pass; concurrent-start tests verify the second mutation gets a friendly error.
  - **Context**: Reqs 1.5, 6.2.

- [ ] **Task 6.2**: `jiraSyncRun`, `jiraSyncRuns` queries + tests
  - **ID**: `task-6.2`
  - **BlockedBy**: `task-1.4`, `task-3.2`
  - **Agent**: `general-purpose`
  - **File**: `internal/reporter/graphql/domain_resolvers_jira_sync.go` (extend), test file (extend)
  - **Change**: Implement read queries. `jiraSyncRun(id)` returns latest status for polling. `jiraSyncRuns(projectId, first)` returns recent history, newest first. Both use the same project-auth check.
  - **Outcome**: Tests pass; auth tests verify cross-project access is rejected.
  - **Context**: Reqs 1.6, 5.

- [ ] **Task 6.3**: `requirements`, `requirement` queries + tests
  - **ID**: `task-6.3`
  - **BlockedBy**: `task-1.4`, `task-3.1`
  - **Agent**: `general-purpose`
  - **File**: `internal/reporter/graphql/domain_resolvers_requirements.go`, test file
  - **Change**: Implement requirements list query with type/parent/cursor filters. Cursor pagination keeps queries bounded; default `first=20`, max `first=100`. `requirement(id)` is a single-row lookup. `scenarioCount` is a computed field — count of children with `type=scenario`.
  - **Outcome**: Tests pass; pagination tests verify cursor semantics and `first` cap.
  - **Context**: Req 11.

- [ ] **Task 6.4**: `projectJiraSyncSettings` query + `saveProjectJiraSyncSettings` mutation + tests
  - **ID**: `task-6.4`
  - **BlockedBy**: `task-1.4`, `task-3.4`
  - **Agent**: `general-purpose`
  - **File**: `internal/reporter/graphql/domain_resolvers_jira_sync.go` (extend), test file (extend)
  - **Change**: Query returns the project's settings (default-populated if no row exists). Mutation validates input (range checks on `releaseWindowDays`, etc.) and upserts. Auth: project-scoped manager.
  - **Outcome**: Tests pass; invalid input rejected with field-specific error messages.
  - **Context**: Reqs 7.4, 8.1, 9.6.

### Phase 7: Inventory page UI

- [ ] **Task 7.1**: GraphQL client extensions
  - **ID**: `task-7.1`
  - **BlockedBy**: `task-6.2`, `task-6.3`, `task-6.4`
  - **Agent**: `general-purpose`
  - **File**: `web/js/graphql-client.js`
  - **Change**: Add the new queries (`GET_REQUIREMENTS`, `GET_REQUIREMENT_DETAIL`, `GET_JIRA_SYNC_RUN`, `GET_JIRA_SYNC_RUNS`, `GET_PROJECT_JIRA_SYNC_SETTINGS`) and mutations (`START_JIRA_SYNC`, `CANCEL_JIRA_SYNC`, `SAVE_PROJECT_JIRA_SYNC_SETTINGS`). Field sets exactly as exposed in `schema.graphql`.
  - **Outcome**: Manual query in GraphiQL succeeds for each; no field-mismatch warnings.
  - **Context**: Req 11.

- [ ] **Task 7.2**: Inventory list rendering with filters
  - **ID**: `task-7.2`
  - **BlockedBy**: `task-7.1`
  - **Agent**: `general-purpose`
  - **File**: `web/index.html` (Integrations tab)
  - **Change**: Add an inventory section under the project's JIRA Integrations tab. Two-column layout: filters on left (type checkboxes, status dropdown, fix-version search), virtualized list on right. Each row: type icon, JIRA key, summary, status, fix version, "synced N ago" timestamp, external-link button. No coverage data shown here — that's #29.
  - **Outcome**: Smoke test: list renders for a project with synced requirements; filters apply client-side without re-querying.
  - **Context**: Req 11. Existing UI pattern: `index.html` projects-list section.

- [ ] **Task 7.3**: Row expansion to show scenarios
  - **ID**: `task-7.3`
  - **BlockedBy**: `task-7.2`
  - **Agent**: `general-purpose`
  - **File**: `web/index.html` (extend)
  - **Change**: Clicking a row expands to show child scenarios. Lazy-load via `requirements(parentId=<row.id>, type=SCENARIO)` query. Each scenario row shows: title, `source` badge (`parsed` / `llm_extracted` — different styling), `confidence` value, orphaned flag if set.
  - **Outcome**: Smoke test: clicking an issue with 3 Gherkin scenarios shows all 3 with correct source badges.
  - **Context**: Req 11.3.

- [ ] **Task 7.4**: Sync configuration dialog + start sync flow
  - **ID**: `task-7.4`
  - **BlockedBy**: `task-7.1`
  - **Agent**: `general-purpose`
  - **File**: `web/index.html` (extend)
  - **Change**: "Configure Sync" button opens dialog with: trigger type (Initial / Incremental), issue type checkboxes, JQL field, release-window-days input. Pre-populate from `projectJiraSyncSettings`. On submit: call `startJiraSync` → modal switches to progress panel polling `jiraSyncRun(id)` every 3s → on terminal status, show summary panel with counts.
  - **Outcome**: Smoke test: full UI flow from button click to completion summary works against the mock JIRA server.
  - **Context**: Reqs 1, 1.4, 1.6, 1.8, 7, 8.

- [ ] **Task 7.5**: Sync history panel
  - **ID**: `task-7.5`
  - **BlockedBy**: `task-7.1`
  - **Agent**: `general-purpose`
  - **File**: `web/index.html` (extend)
  - **Change**: A collapsible "Sync History" panel on the Integrations tab listing the last 20 runs (`jiraSyncRuns` query). Each row: trigger type, initiator, started/completed timestamps, status badge, counts, error summary if failed. Failed-row expansion shows the full error.
  - **Outcome**: Smoke test: panel displays the runs from acceptance fixtures correctly.
  - **Context**: Reqs 5.3, 5.4.

### Phase 8: End-to-end verification

- [ ] **Task 8.1**: Acceptance — full initial sync flow
  - **ID**: `task-8.1`
  - **BlockedBy**: `task-1.3`, `task-5.6`, `task-7.4`
  - **Agent**: `general-purpose`
  - **File**: `acceptance/jira_sync_test.go`
  - **Change**: Ginkgo test against the mock JIRA server with a 250-issue fixture. Trigger initial sync via UI (or directly via GraphQL); assert final `sync_runs` row matches expectations and the inventory page shows all 250.
  - **Outcome**: Acceptance test passes locally and in CI.
  - **Context**: Req 1 end to end.

- [ ] **Task 8.2**: Acceptance — incremental sync preserves test associations
  - **ID**: `task-8.2`
  - **BlockedBy**: `task-8.1`
  - **Agent**: `general-purpose`
  - **File**: `acceptance/jira_sync_test.go` (extend)
  - **Change**: After Task 8.1's initial sync, seed a `spec_run_requirement` row for one of the synced requirements. Trigger incremental sync. Assert the seeded row is untouched.
  - **Outcome**: Test passes.
  - **Context**: Req 2.2.

- [ ] **Task 8.3**: Acceptance — Path B backfill creates mappings
  - **ID**: `task-8.3`
  - **BlockedBy**: `task-5.5`, `task-8.1`
  - **Agent**: `general-purpose`
  - **File**: `acceptance/jira_sync_backfill_test.go`
  - **Change**: Seed `spec_run` rows with `jira:NEW-KEY` tags for a key NOT in the requirements table. Mock JIRA fixture includes `NEW-KEY`. Trigger sync. Assert `spec_run_requirements` rows are created with `source='tag'`, `confidence=1.0`.
  - **Outcome**: Test passes.
  - **Context**: Req 10.

- [ ] **Task 8.4**: Acceptance — Gherkin parser produces scenarios
  - **ID**: `task-8.4`
  - **BlockedBy**: `task-5.4`, `task-8.1`
  - **Agent**: `general-purpose`
  - **File**: `acceptance/jira_sync_test.go` (extend)
  - **Change**: Sync a fixture issue whose description contains 3 Gherkin scenarios. Assert: 3 `requirement` rows with `type=scenario`, correct `parent_id`, `source='parsed'`, `confidence=1.0`. Re-sync with unchanged description; verify the parser was NOT re-invoked (assert via metric or repo call counter).
  - **Outcome**: Test passes.
  - **Context**: Reqs 4.5, 9.1, 9.2.

- [ ] **Task 8.5**: Acceptance — orphan handling on issue deletion
  - **ID**: `task-8.5`
  - **BlockedBy**: `task-5.6`, `task-8.1`
  - **Agent**: `general-purpose`
  - **File**: `acceptance/jira_sync_test.go` (extend)
  - **Change**: After Task 8.1, configure the mock JIRA server to no longer return one specific issue. Trigger incremental sync. Assert the requirement row is marked `orphaned=true`, NOT deleted, and any `spec_run_requirement` rows referencing it remain.
  - **Outcome**: Test passes.
  - **Context**: Req 2.3.

### Phase 9: Cleanup and pre-PR gates

- [ ] **Task 9.1**: Documentation — operator guide entry
  - **ID**: `task-9.1`
  - **BlockedBy**: `task-7.4`, `task-7.5`
  - **Agent**: `general-purpose`
  - **File**: `docs/operations/jira-sync.md` (NEW)
  - **Change**: Operator-facing guide for: enabling sync, configuring filters, interpreting sync history, troubleshooting failures, opting in to Tier 2 LLM extraction. One-pager target.
  - **Outcome**: Doc reviewed in the PR.
  - **Context**: Operational support.

- [ ] **Task 9.2**: Pre-PR gate sequence
  - **ID**: `task-9.2`
  - **BlockedBy**: all Phase 8 tasks
  - **Agent**: `general-purpose`
  - **File**: (no file change)
  - **Change**: Run `make test` (unit + integration); run `make test-acceptance`; run `make lint`; run `make build` to verify the binary compiles cleanly. Address any findings.
  - **Outcome**: All gates green.
  - **Context**: Standard pre-PR sequence per `.claude/rules/pre-pr-checklist.md` (if exists; otherwise follow this task as the checklist).

- [ ] **Task 9.3**: Manual smoke
  - **ID**: `task-9.3`
  - **BlockedBy**: `task-9.2`
  - **Agent**: `general-purpose`
  - **File**: (no file change)
  - **Change**: Run `make deploy-all` to bring up the platform locally; configure a JIRA connection (use the mock JIRA at `localhost:8888` or a real Atlassian sandbox); run an initial sync end-to-end; verify inventory page renders; verify sync history shows the run; verify the manager auth check rejects a non-manager attempt.
  - **Outcome**: Smoke checklist in PR description, all boxes ticked.
  - **Context**: Final pre-merge verification.

## Dependency Diagram

```
Phase 1 (parallel root)
  ├─ task-1.1 (migration)
  ├─ task-1.2 (JIRA client tests)
  ├─ task-1.3 (mock server)
  └─ task-1.4 (GraphQL schema)
            ↓
Phase 2 (domain)
  ├─ task-2.1 (Requirement)      ←─ task-1.1
  ├─ task-2.2 (SyncRun)          ←─ task-1.1
  ├─ task-2.3 (Settings)         ←─ task-1.1
  └─ task-2.4 (repo interfaces)  ←─ task-2.1, 2.2, 2.3
            ↓
Phase 3 (repositories)
  ├─ task-3.1 (RequirementRepo)         ←─ task-1.1, 2.4
  ├─ task-3.2 (SyncRunRepo)             ←─ task-1.1, 2.4
  ├─ task-3.3 (SpecRunRequirementRepo)  ←─ task-1.1, 2.4
  └─ task-3.4 (SettingsRepo)            ←─ task-1.1, 2.4

Phase 4 (parser; parallel to Phase 3)
  ├─ task-4.1 (markdown extractor)
  ├─ task-4.2 (Gherkin parse)    ←─ task-4.1
  ├─ task-4.3 (reconciler)       ←─ task-3.1, 4.2
  └─ task-4.4 (LLM stub)         ←─ task-4.2
            ↓
Phase 5 (orchestration)
  ├─ task-5.1 (SearchIssues impl)        ←─ task-1.2
  ├─ task-5.2 (SyncService skeleton)     ←─ 2.2, 3.1, 3.2, 3.3, 3.4, 5.1
  ├─ task-5.3 (pagination/backoff)       ←─ task-5.2
  ├─ task-5.4 (description-hash reparse) ←─ task-3.1, 4.3, 5.2
  ├─ task-5.5 (Path B backfill call)     ←─ task-3.3, 5.2
  └─ task-5.6 (incremental/staleness)    ←─ task-5.3
            ↓
Phase 6 (GraphQL resolvers)
  ├─ task-6.1 (sync mutations)    ←─ 1.4, 5.2
  ├─ task-6.2 (sync queries)      ←─ 1.4, 3.2
  ├─ task-6.3 (requirements)      ←─ 1.4, 3.1
  └─ task-6.4 (settings)          ←─ 1.4, 3.4
            ↓
Phase 7 (UI)
  ├─ task-7.1 (GraphQL client)    ←─ 6.2, 6.3, 6.4
  ├─ task-7.2 (inventory list)    ←─ task-7.1
  ├─ task-7.3 (scenario expand)   ←─ task-7.2
  ├─ task-7.4 (sync config dialog)←─ task-7.1
  └─ task-7.5 (sync history)      ←─ task-7.1
            ↓
Phase 8 (acceptance)
  ├─ task-8.1 (initial sync)         ←─ 1.3, 5.6, 7.4
  ├─ task-8.2 (incremental preserves)←─ task-8.1
  ├─ task-8.3 (Path B backfill)      ←─ 5.5, 8.1
  ├─ task-8.4 (Gherkin scenarios)    ←─ 5.4, 8.1
  └─ task-8.5 (orphan handling)      ←─ 5.6, 8.1
            ↓
Phase 9 (pre-PR)
  ├─ task-9.1 (docs)        ←─ 7.4, 7.5
  ├─ task-9.2 (CI gates)    ←─ all Phase 8
  └─ task-9.3 (smoke)       ←─ task-9.2
```

## Completion Criteria

1. All tasks above are checked off.
2. All requirements (R1 through R11) are demonstrated by acceptance tests in Phase 8.
3. `make test` and `make test-acceptance` are green on the feature branch.
4. The pre-PR gate sequence (task-9.2) reports no outstanding findings.
5. Migration 000023 applies and reverts cleanly; the app starts with all four new tables created.
6. `startJiraSync` mutation drives a sync to `succeeded` in the manual smoke (task-9.3); inventory page renders correctly.
7. No commits include scratch artifacts (`REVIEW*.md`, `.claude/`-local files, `claude-progress.md` etc.) — per the working-directory-cleanliness rule.
8. Tier 2 (LLM) extraction is wired but disabled-by-default; an operator can opt in by setting both env vars and the per-project flag.
