# 29-jira-coverage-hierarchy - Tasks

## Status: In Progress (6/13 complete)

TDD discipline: every RED task writes failing tests that define the expected behaviour.
Every GREEN task writes the minimum implementation to make those tests pass.
No implementation code is written without a failing test first.

---

## Implementation Tasks

### Phase 1: Foundations (all parallel — non-TDD setup required before tests can be written)

- [x] **Task 1.1**: Define JIRA Go model types and interfaces
  - **ID**: `task-1.1`
  - **BlockedBy**: `none`
  - **File**: `internal/domains/integrations/types.go`
  - **Change**: Added `JiraVersion`, `JiraParent`, `JiraIssue`, `CoverageCount` structs. Added `CoverageJiraClient` interface (`GetVersions`, `SearchIssues`) and `CoverageTagRepository` interface (`GetJiraTagCoverageByProject`) as narrow interfaces for the coverage service to depend on — avoids extending the existing `JiraClient` interface which would break compilation.
  - **Outcome**: All shared types and interfaces exist; downstream TDD cycles can begin.
  - **Context**: Req 1 AC1 (version fields), Req 2 AC1 (`parent` field — not `customfield_epicLink`). Interfaces let task-4.1 start in parallel with tasks 2.x and 3.x.

- [x] **Task 1.2**: Add GraphQL schema types and run codegen
  - **ID**: `task-1.2`
  - **BlockedBy**: `none`
  - **File**: `internal/reporter/graphql/schema.graphql`
  - **Change**: Added types `JiraRelease` (named to avoid confusion with domain `JiraVersion`), `JiraIssueSummary`, `TestRunCoverage`, `StoryCoverageNode`, `EpicCoverageNode`, `RequirementCoverageTree`. Added queries `jiraFixVersions(projectId: ID!): [JiraRelease!]!` and `requirementCoverage(projectId: ID!, fixVersionName: String!): RequirementCoverageTree!`. Ran `go run github.com/99designs/gqlgen generate` — regenerated `model/models_gen.go` with new types and added resolver stubs in `schema.resolvers.go`.
  - **Outcome**: Schema compiles; generated Go types and `panic`-stub resolver methods exist; full `go build ./...` clean.
  - **Context**: Design §3 GraphQL types. Schema-first. Note: GraphQL type is `JiraRelease`; domain Go type is `integrations.JiraVersion` — resolver maps between them.

- [x] **Task 1.3**: Add mock JIRA server endpoints
  - **ID**: `task-1.3`
  - **BlockedBy**: `task-1.1`
  - **File**: `acceptance/helpers/mock_jira_server.go`
  - **Change**: Added `MockVersion`, `MockIssue`, `MockIssueParent` fixture types. Extended `MockJiraServer` with `versions`, `issuesByVersion`, `issuesByKey`, `unavailable` fields. Registered `GET /rest/api/3/project/{key}/versions` (handleProjectV3) and `POST /rest/api/3/issue/search` (handleIssueSearch) on a new `/rest/api/3/` prefix. Issue search paginates via `startAt`/`maxResults`. JQL parser supports `fixVersion = "..."` and `issueKey IN (...)`. Added `SetVersions`, `SetIssuesForVersion`, `AddIssueByKey`, `SimulateUnavailable` helper methods. Acceptance module is separate (`acceptance/go.mod`).
  - **Outcome**: Tests can configure versions and issues per project, drive paginated responses, and simulate JIRA unavailability.
  - **Context**: Design §3 mock endpoints.

---

### Phase 2: Repository TDD (parallel with Phase 3)

- [x] **Task 2.1** 🔴 RED: Write failing tag repository tests
  - **ID**: `task-2.1`
  - **BlockedBy**: `task-1.1`
  - **File**: `internal/repository/tag_repository_test.go`
  - **Change**: Write Ginkgo/Gomega specs using go-sqlmock. Define the method signature `GetJiraTagCoverageByProject(projectID string) (map[string]CoverageCount, error)` in the test file (it won't exist yet on the interface — add it to `TagRepositoryInterface` if not already there). Test cases: (1) returns correct total/passed/failed per issue key; (2) excludes rows with `category != 'jira'`; (3) excludes test runs from other projects. Tests must fail (RED) — the method does not exist yet.
  - **Outcome**: Failing test suite precisely specifies the repository contract. `make test` fails on this file.
  - **Context**: Req 3 AC1. Design §3 SQL and §6 unit tests. go-sqlmock patterns from existing repository tests.

- [x] **Task 2.2** 🟢 GREEN: Implement tag repository method
  - **ID**: `task-2.2`
  - **BlockedBy**: `task-2.1`
  - **File**: `internal/repository/tag_repository.go`
  - **Change**: Add `GetJiraTagCoverageByProject(projectID string) (map[string]CoverageCount, error)`. Join `tags → test_run_tags → test_runs` where `category = 'jira'` and `project_id = $1`, group by `t.value`, aggregate total/passed/failed. Implement until all tests from task-2.1 pass.
  - **Outcome**: `make test` passes for tag_repository_test.go. No behaviour beyond what the tests specify.
  - **Context**: Design §3 SQL. Minimum implementation to go green — no extra error handling for scenarios the tests don't cover.

---

### Phase 3: JIRA Client TDD (parallel with Phase 2)

- [x] **Task 3.1** 🔴 RED: Write failing JIRA client tests
  - **ID**: `task-3.1`
  - **BlockedBy**: `task-1.1, task-1.3`
  - **File**: `internal/jira/client_test.go`
  - **Change**: Write Ginkgo/Gomega specs using mock JIRA server from task-1.3. Define method signatures `GetVersions` and `SearchIssues` on `JiraClientInterface`. Test cases: (1) `SearchIssues` stitches pages when `total > maxResults` (assert all items returned across pages); (2) POST body contains correct `jql` and `fields`; (3) `GetVersions` returns parsed unreleased and released versions. Tests must fail (RED) — methods do not exist yet.
  - **Outcome**: Failing tests specify client contract including pagination behaviour. `make test` fails on this file.
  - **Context**: Req 4 AC4 (up to 500 issues). Design §3 client methods, Decision 3 (paginate), Decision 4 (POST).

- [ ] **Task 3.2** 🟢 GREEN: Implement JIRA client methods
  - **ID**: `task-3.2`
  - **BlockedBy**: `task-3.1`
  - **File**: `internal/jira/client.go`
  - **Change**: Add `GetVersions(projectKey string) ([]JiraVersion, error)` — `GET /rest/api/3/project/{projectKey}/versions`. Add `SearchIssues(jql string, fields []string) ([]JiraIssue, error)` — `POST /rest/api/3/issue/search` with `maxResults=100`, loop until `startAt + len(issues) >= total`. Implement until all tests from task-3.1 pass.
  - **Outcome**: `make test` passes for client_test.go. Pagination works correctly.
  - **Context**: Design §3 client methods. Existing client usage patterns in client.go for auth/HTTP setup.

---

### Phase 4: Coverage Service TDD

- [ ] **Task 4.1** 🔴 RED: Write failing coverage service tests
  - **ID**: `task-4.1`
  - **BlockedBy**: `task-1.1`
  - **File**: `internal/jira/coverage_service_test.go`
  - **Change**: Write Ginkgo/Gomega specs using mocks for `JiraClientInterface` and `TagRepositoryInterface` (defined in task-1.1) — use gomock or testify/mock. Define the `CoverageService` interface and `Build` method signature. Test cases: (1) Phase 2 fires when parent epics are absent from Phase 1; (2) Phase 2 skipped when all parent epics already in Phase 1; (3) stories grouped under correct epic; (4) orphan stories appear in Unassigned; (5) covered/uncovered status correct from tag map (`Total > 0` = covered); (6) empty fix version returns empty tree without error. Tests must fail (RED).
  - **Outcome**: Failing tests specify the two-phase orchestration and tree assembly contract precisely. Can start in parallel with tasks 2.x and 3.x since it mocks the interfaces.
  - **Context**: Req 2 AC1–5, Req 3 AC2–4, Req 4 AC1–2. Design §2 data flow. A story is "covered" when `CoverageCount.Total > 0`.

- [ ] **Task 4.2** 🟢 GREEN: Implement CoverageService
  - **ID**: `task-4.2`
  - **BlockedBy**: `task-4.1, task-2.2, task-3.2`
  - **File**: `internal/jira/coverage_service.go`
  - **Change**: Create `CoverageService` implementing `Build(ctx, projectID, fixVersionName string) (*CoverageTree, error)`. Steps: (1) fetch `JiraConnection`; (2) Phase 1 JQL `fixVersion = "<escaped>" ORDER BY issuetype`; (3) collect missing parent epic keys; (4) Phase 2 batched `issueKey IN (...)` only when needed; (5) `GetJiraTagCoverageByProject`; (6) assemble tree. Escape `fixVersionName` (replace `"` with `\"`). Define internal `CoverageTree`, `EpicNode`, `StoryNode`. Implement until all tests from task-4.1 pass.
  - **Outcome**: `make test` passes for coverage_service_test.go. Real client and repo implementations wired in (not mocks).
  - **Context**: Design §2 data flow, §5 error handling. Req 4 AC2 (batch Phase 2, never one-call-per-epic).

---

### Phase 5: Resolver TDD

- [ ] **Task 5.1** 🔴 RED: Write failing resolver tests
  - **ID**: `task-5.1`
  - **BlockedBy**: `task-1.2, task-4.2`
  - **File**: `internal/graph/resolvers/coverage_resolvers_test.go`
  - **Change**: Write Ginkgo/Gomega specs testing the GraphQL resolver layer. Test cases: (1) `jiraFixVersions` returns correct `[JiraVersion]` for a project with a connection; (2) `requirementCoverage` returns a `RequirementCoverageTree` with correct structure; (3) returns GraphQL error (not panic) when project has no JIRA connection; (4) returns GraphQL error when JIRA API unavailable. Tests must fail (RED) — resolver bodies are empty stubs from codegen.
  - **Outcome**: Failing tests specify resolver contract including all error cases. `make test` fails on this file.
  - **Context**: Design §5 error handling table — test each error scenario. Req 1 AC4 (no connection message).

- [ ] **Task 5.2** 🟢 GREEN: Implement coverage resolvers
  - **ID**: `task-5.2`
  - **BlockedBy**: `task-5.1`
  - **File**: `internal/graph/resolvers/coverage.resolvers.go`
  - **Change**: Fill in resolver stubs. `JiraFixVersions`: fetch `JiraConnection`, call `GetVersions(projectKey)`, map to `[]*model.JiraVersion`. `RequirementCoverage`: call `CoverageService.Build(...)`, map `CoverageTree` → `*model.RequirementCoverageTree`. Return GraphQL errors (not panics) for all error cases. Implement until all tests from task-5.1 pass.
  - **Outcome**: `make test` passes for coverage_resolvers_test.go. All error cases return proper GraphQL errors.
  - **Context**: Design §5 error messages — use the exact strings specified. Never panic; always return `(nil, err)`.

---

### Phase 6: Frontend TDD (acceptance tests first)

- [ ] **Task 6.1** 🔴 RED: Write failing acceptance tests for frontend behaviour
  - **ID**: `task-6.1`
  - **BlockedBy**: `task-1.2, task-1.3`
  - **File**: `acceptance/coverage_test.go`
  - **Change**: Write failing Ginkgo acceptance tests using mock JIRA server from task-1.3. All tests will fail because the frontend doesn't exist yet. Scenarios: (1) Coverage tab visible on project with JIRA connection, absent without one; (2) fix version picker loads, groups unreleased-first then released-newest-first, and filters by typing; (3) selecting a version renders epic/story hierarchy with coverage indicators; (4) "Show uncovered only" toggle hides covered stories; (5) JIRA unavailable — error shown, tab doesn't crash. Do not implement any frontend yet.
  - **Outcome**: Full acceptance test suite is written and failing (RED). Precisely specifies all frontend behaviour. `make test-acceptance` fails.
  - **Context**: Design §6 acceptance tests. Req 1 AC2/AC4, Req 2 AC2–5, Req 3 AC5, Req 4 AC3. Writing tests first forces the UI contract to be explicit before any HTML is written.

- [ ] **Task 6.2** 🟢 GREEN: Frontend Coverage tab, picker, and hierarchy tree
  - **ID**: `task-6.2`
  - **BlockedBy**: `task-6.1, task-5.2`
  - **File**: `web/index.html`
  - **Change**: Add Coverage tab button and panel. Implement fix version picker: call `jiraFixVersions` on tab open, group client-side (unreleased first alphabetical, released newest-first), filter as user types. On version selection call `requirementCoverage` and render two-level tree: collapsible epic rows (key, summary, status, coverage %), story rows (key, summary, status, covered/uncovered badge, test run count + pass/fail if covered), Unassigned section. Show spinner while loading. Implement until acceptance tests (1)–(3) from task-6.1 pass.
  - **Outcome**: Core coverage view works; tasks 6.1 scenarios 1–3 pass. `make test-acceptance` partial green.
  - **Context**: Req 1 AC1–3, Req 2 AC2–5, Req 3 AC2–4. Design §3 frontend. CLAUDE.md: static HTML/JS, no build pipeline. Hide tab when `jiraFixVersions` errors due to no connection.

- [ ] **Task 6.3** 🟢 GREEN: Toggle, no-connection message, and error states
  - **ID**: `task-6.3`
  - **BlockedBy**: `task-6.2`
  - **File**: `web/index.html`
  - **Change**: Add "Show uncovered only" checkbox above tree — client-side DOM filter, no re-query. Show "No JIRA connection configured" message in panel when project has no connection. Display specific GraphQL error messages for JIRA unavailable/auth failure. Implement until all 5 acceptance test scenarios from task-6.1 pass.
  - **Outcome**: `make test-acceptance` fully green. All acceptance scenarios pass.
  - **Context**: Req 1 AC4, Req 3 AC5, Req 4 AC3. Design §5 error handling.

---

## Dependency Diagram

```
task-1.1 (types+ifaces) ──┬──▶ task-2.1 🔴 ──▶ task-2.2 🟢 ──────────────────────┐
                           │                                                          ▼
                           ├──▶ task-3.1 🔴 ──▶ task-3.2 🟢 ──────────────────▶ task-4.2 🟢 ──▶ task-5.1 🔴 ──▶ task-5.2 🟢 ──▶ task-6.2 🟢 ──▶ task-6.3 🟢
                           │                                                          ▲
                           └──▶ task-4.1 🔴 ───────────────────────────────────────┘

task-1.2 (schema+codegen) ──────────────────────────────────────────────────────▶ task-5.1 🔴
                            └──▶ task-6.1 🔴 ──▶ task-6.2 🟢 ──▶ task-6.3 🟢

task-1.3 (mock server) ──────────────────────────────────────────────────────── task-3.1 🔴
                         └──▶ task-6.1 🔴
```

**Parallel opportunities:**
- Tasks 1.1, 1.2: both root tasks — start simultaneously
- Task 1.3 starts as soon as 1.1 is done (types needed to define mock shapes)
- Tasks 2.1 (repo RED) and 4.1 (service RED) both start after 1.1 — run simultaneously
- Tasks 2.1 and 3.1 are independent RED tracks — run simultaneously after their prerequisites
- Task 4.1 (service RED, uses interface mocks) runs in parallel with 2.x and 3.x — no implementation needed to write the service tests
- Frontend track (1.2 → 6.1 → 6.2 → 6.3) runs in parallel with backend service/resolver track

**Critical path:** task-1.1 → task-3.1 → task-3.2 → task-4.2 → task-5.1 → task-5.2 → task-6.2 → task-6.3 (8 tasks)

Note: task-4.2 also waits for task-2.2 and task-4.1, but those can be completed in parallel with the 3.x chain.

---

## Completion Criteria

- [ ] All 13 tasks checked off
- [ ] `make test` fully green (all unit + resolver tests pass)
- [ ] `make test-acceptance` fully green (all 5 acceptance scenarios pass)
- [ ] Every implementation task was preceded by a failing test
- [ ] Coverage tab visible on projects with a JIRA connection; absent without one
- [ ] Fix version picker loads, filters, and groups correctly (unreleased before released)
- [ ] Two-level hierarchy renders with correct epic/story grouping
- [ ] Covered stories show test run count and pass/fail; uncovered clearly marked
- [ ] "Show uncovered only" toggle works client-side
- [ ] JIRA API errors show user-friendly messages; tab does not crash
- [ ] No regressions in existing JIRA integration features
