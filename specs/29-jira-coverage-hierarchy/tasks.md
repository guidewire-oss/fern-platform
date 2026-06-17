# 29-jira-coverage-hierarchy - Tasks

## Status: In progress (18/18 complete — acceptance tests still pending)

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
  - **Change**: Added `MockVersion`, `MockIssue`, `MockIssueParent` fixture types. Extended `MockJiraServer` with `versions`, `issuesByVersion`, `issuesByKey`, `unavailable` fields. Registered `GET /rest/api/3/project/{key}/versions` (handleProjectV3) and `GET /rest/api/3/search/jql` (handleIssueSearch) — the Atlassian Cloud cursor-based endpoint. Response format: `{nextPageToken, issues}` (no `startAt`/`total`); mock returns all matching issues in one shot (no server-side pagination). JQL parser supports `fixVersion = "..."` and `issueKey IN (...)`. Added `SetVersions`, `SetIssuesForVersion`, `AddIssueByKey`, `SimulateUnavailable` helper methods. Acceptance module is separate (`acceptance/go.mod`).
  - **Outcome**: Tests can configure versions and issues per project and simulate JIRA unavailability.
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
  - **File**: `internal/domains/tags/infrastructure/gorm_tag_repository.go`
  - **Change**: Implemented `GetJiraTagCoverageByProject` using a raw SQL UNION query that aggregates from both `spec_run_tags` (individual test level, joined via `spec_runs → suite_runs → test_runs`) and `test_run_tags` (whole-run level, joined via `test_runs`) where `category = 'jira'` and `project_id = ?`, grouped by `t.value`. Both tagging granularities contribute to `Total`/`Passed`/`Failed` counts. Uses `db.Raw(...)` rather than the fluent GORM API because GORM cannot express UNION queries.
  - **Outcome**: `make test` passes for coverage repository tests. Tags at either granularity show an issue as covered.
  - **Context**: Design §3 SQL. Supports two tag workflows: whole-run tags (e.g. from CI reporter) and per-spec tags (e.g. from seed script or future reporter field).

---

### Phase 3: JIRA Client TDD (parallel with Phase 2)

- [x] **Task 3.1** 🔴 RED: Write failing JIRA client tests
  - **ID**: `task-3.1`
  - **BlockedBy**: `task-1.1, task-1.3`
  - **File**: `internal/jira/client_test.go`
  - **Change**: Write Ginkgo/Gomega specs using mock JIRA server from task-1.3. Define method signatures `GetVersions` and `SearchIssues` on `JiraClientInterface`. Test cases: (1) `SearchIssues` stitches pages when `total > maxResults` (assert all items returned across pages); (2) POST body contains correct `jql` and `fields`; (3) `GetVersions` returns parsed unreleased and released versions. Tests must fail (RED) — methods do not exist yet.
  - **Outcome**: Failing tests specify client contract including pagination behaviour. `make test` fails on this file.
  - **Context**: Req 4 AC4 (up to 500 issues). Design §3 client methods, Decision 3 (paginate), Decision 4 (POST).

- [x] **Task 3.2** 🟢 GREEN: Implement JIRA client methods
  - **ID**: `task-3.2`
  - **BlockedBy**: `task-3.1`
  - **File**: `internal/domains/integrations/jira_client_coverage.go`
  - **Change**: Implemented `GetVersions` — `GET /rest/api/3/project/{projectKey}/versions`. Implemented `SearchIssues` — `GET /rest/api/3/search/jql` (Atlassian Cloud cursor-based endpoint; `POST /rest/api/3/issue/search` returns 405 on Cloud and `GET /rest/api/3/issue/search` returns 404 by routing to a single-issue handler). Pagination uses `nextPageToken` in the response (not `startAt`/`total` offset). Loop breaks when response `nextPageToken` is empty. Error bodies included in error messages via `io.LimitReader` for diagnosability.
  - **Outcome**: `make test` passes for JIRA client tests. Cursor-based pagination retrieves all pages.
  - **Context**: Design §3 client methods. Atlassian Cloud only supports `GET /rest/api/3/search/jql` with cursor pagination — the v2 search endpoints return 410 Gone on Cloud instances.

---

### Phase 4: Coverage Service TDD

- [x] **Task 4.1** 🔴 RED: Write failing coverage service tests
  - **ID**: `task-4.1`
  - **BlockedBy**: `task-1.1`
  - **File**: `internal/jira/coverage_service_test.go`
  - **Change**: Write Ginkgo/Gomega specs using mocks for `JiraClientInterface` and `TagRepositoryInterface` (defined in task-1.1) — use gomock or testify/mock. Define the `CoverageService` interface and `Build` method signature. Test cases: (1) Phase 2 fires when parent epics are absent from Phase 1; (2) Phase 2 skipped when all parent epics already in Phase 1; (3) stories grouped under correct epic; (4) orphan stories appear in Unassigned; (5) covered/uncovered status correct from tag map (`Total > 0` = covered); (6) empty fix version returns empty tree without error. Tests must fail (RED).
  - **Outcome**: Failing tests specify the two-phase orchestration and tree assembly contract precisely. Can start in parallel with tasks 2.x and 3.x since it mocks the interfaces.
  - **Context**: Req 2 AC1–5, Req 3 AC2–4, Req 4 AC1–2. Design §2 data flow. A story is "covered" when `CoverageCount.Total > 0`.

- [x] **Task 4.2** 🟢 GREEN: Implement CoverageService
  - **ID**: `task-4.2`
  - **BlockedBy**: `task-4.1, task-2.2, task-3.2`
  - **File**: `internal/jira/coverage_service.go`
  - **Change**: Create `CoverageService` implementing `Build(ctx, projectID, fixVersionName string) (*CoverageTree, error)`. Steps: (1) fetch `JiraConnection`; (2) Phase 1 JQL `fixVersion = "<escaped>" ORDER BY issuetype`; (3) collect missing parent epic keys; (4) Phase 2 batched `issueKey IN (...)` only when needed; (5) `GetJiraTagCoverageByProject`; (6) assemble tree. Escape `fixVersionName` (replace `"` with `\"`). Define internal `CoverageTree`, `EpicNode`, `StoryNode`. Implement until all tests from task-4.1 pass.
  - **Outcome**: `make test` passes for coverage_service_test.go. Real client and repo implementations wired in (not mocks).
  - **Context**: Design §2 data flow, §5 error handling. Req 4 AC2 (batch Phase 2, never one-call-per-epic).

---

### Phase 5: Resolver TDD

- [x] **Task 5.1** 🔴 RED: Write failing resolver tests
  - **ID**: `task-5.1`
  - **BlockedBy**: `task-1.2, task-4.2`
  - **File**: `internal/graph/resolvers/coverage_resolvers_test.go`
  - **Change**: Write Ginkgo/Gomega specs testing the GraphQL resolver layer. Test cases: (1) `jiraFixVersions` returns correct `[JiraVersion]` for a project with a connection; (2) `requirementCoverage` returns a `RequirementCoverageTree` with correct structure; (3) returns GraphQL error (not panic) when project has no JIRA connection; (4) returns GraphQL error when JIRA API unavailable. Tests must fail (RED) — resolver bodies are empty stubs from codegen.
  - **Outcome**: Failing tests specify resolver contract including all error cases. `make test` fails on this file.
  - **Context**: Design §5 error handling table — test each error scenario. Req 1 AC4 (no connection message).

- [x] **Task 5.2** 🟢 GREEN: Implement coverage resolvers
  - **ID**: `task-5.2`
  - **BlockedBy**: `task-5.1`
  - **File**: `internal/graph/resolvers/coverage.resolvers.go`
  - **Change**: Fill in resolver stubs. `JiraFixVersions`: fetch `JiraConnection`, call `GetVersions(projectKey)`, map to `[]*model.JiraVersion`. `RequirementCoverage`: call `CoverageService.Build(...)`, map `CoverageTree` → `*model.RequirementCoverageTree`. Return GraphQL errors (not panics) for all error cases. Implement until all tests from task-5.1 pass.
  - **Outcome**: `make test` passes for coverage_resolvers_test.go. All error cases return proper GraphQL errors.
  - **Context**: Design §5 error messages — use the exact strings specified. Never panic; always return `(nil, err)`.

---

### Phase 6: Frontend TDD (acceptance tests first)

- [x] **Task 6.1** 🔴 RED: Write failing acceptance tests for frontend behaviour
  - **ID**: `task-6.1`
  - **BlockedBy**: `task-1.2, task-1.3`
  - **File**: `acceptance/coverage_test.go`
  - **Change**: Write failing Ginkgo acceptance tests using mock JIRA server from task-1.3. All tests will fail because the frontend doesn't exist yet. Scenarios: (1) Coverage tab visible on project with JIRA connection, absent without one; (2) fix version picker loads, groups unreleased-first then released-newest-first, and filters by typing; (3) selecting a version renders epic/story hierarchy with coverage indicators; (4) "Show uncovered only" toggle hides covered stories; (5) JIRA unavailable — error shown, tab doesn't crash. Do not implement any frontend yet.
  - **Outcome**: Full acceptance test suite is written and failing (RED). Precisely specifies all frontend behaviour. `make test-acceptance` fails.
  - **Context**: Design §6 acceptance tests. Req 1 AC2/AC4, Req 2 AC2–5, Req 3 AC5, Req 4 AC3. Writing tests first forces the UI contract to be explicit before any HTML is written.

- [x] **Task 6.2** 🟢 GREEN: Frontend Coverage tab, picker, and hierarchy tree
  - **ID**: `task-6.2`
  - **BlockedBy**: `task-6.1, task-5.2`
  - **File**: `web/index.html`
  - **Change**: Add Coverage tab button and panel. Implement fix version picker: call `jiraFixVersions` on tab open, group client-side (unreleased first alphabetical, released newest-first), filter as user types. On version selection call `requirementCoverage` and render two-level tree: collapsible epic rows (key, summary, status, coverage %), story rows (key, summary, status, covered/uncovered badge, test run count + pass/fail if covered), Unassigned section. Show spinner while loading. Implement until acceptance tests (1)–(3) from task-6.1 pass.
  - **Outcome**: Core coverage view works; tasks 6.1 scenarios 1–3 pass. `make test-acceptance` partial green.
  - **Context**: Req 1 AC1–3, Req 2 AC2–5, Req 3 AC2–4. Design §3 frontend. CLAUDE.md: static HTML/JS, no build pipeline. Hide tab when `jiraFixVersions` errors due to no connection.

- [x] **Task 6.3** 🟢 GREEN: Toggle, no-connection message, and error states
  - **ID**: `task-6.3`
  - **BlockedBy**: `task-6.2`
  - **File**: `web/index.html`
  - **Change**: Add "Show uncovered only" checkbox above tree — client-side DOM filter, no re-query. Show "No JIRA connection configured" message in panel when project has no connection. Display specific GraphQL error messages for JIRA unavailable/auth failure. Implement until all 5 acceptance test scenarios from task-6.1 pass.
  - **Outcome**: `make test-acceptance` fully green. All acceptance scenarios pass.
  - **Context**: Req 1 AC4, Req 3 AC5, Req 4 AC3. Design §5 error handling.

- [x] **Task 6.4** 🟢 GREEN: JIRA issue keys as external links
  - **ID**: `task-6.4`
  - **BlockedBy**: `task-6.2`
  - **File**: `web/index.html`
  - **Change**: Render every issue key (epic and story) in the hierarchy tree as an `<a>` tag linking to `<jira-base-url>/browse/<issue-key>` in a new tab. The base URL is already available from the `JiraConnection` record — pass it through the `requirementCoverage` GraphQL response or read it from the connection state already held client-side when the coverage tab is opened.
  - **Outcome**: Clicking any issue key in the coverage tree opens the JIRA issue directly. No new GraphQL field required if the base URL is already in client state.
  - **Context**: UX improvement — closes the loop between coverage indicators and the source JIRA issues.

- [x] **Task 6.5** 🟢 GREEN: Drill-down from covered story to its tagged spec runs
  - **ID**: `task-6.5`
  - **BlockedBy**: `task-6.2`
  - **File**: `web/index.html`
  - **Change**: On covered story rows, make the test run count a clickable link or button. On click, open a panel or modal showing the spec runs tagged with that issue key for this project — spec name, status, suite, and a link to the full test run. Query the existing spec run / test run data filtered by the JIRA tag value. No new backend endpoint needed if the tag value can be passed as a filter to an existing query; add a lightweight GraphQL query (`specRunsByJiraTag(projectId, issueKey)`) if not.
  - **Outcome**: A user can navigate from "3 tests cover PROJ-123" directly to those 3 tests. Closes the loop between coverage summary and the actual test evidence.
  - **Context**: Complements task-6.4 — together they make the coverage view a navigation hub, not just a status display.

- [x] **Task 6.6** 🟢 GREEN: Epic coverage % format, visual bar, and story pass/fail + last-run date
  - **ID**: `task-6.6`
  - **BlockedBy**: `task-6.2`
  - **File**: `web/index.html`, `internal/reporter/graphql/schema.graphql`, `internal/domains/tags/infrastructure/gorm_tag_repository.go`
  - **Change**:
    1. **Backend** — add `lastRunAt: Time` field to `TestRunCoverage` GraphQL type. Add `LastRunAt *time.Time` to `domain.CoverageCount` and `coverageRow`. Update `GetJiraTagCoverageByProject` SQL to include `MAX(tagged.run_at)` in both UNION branches (use `sr.start_time` for spec-run branch, `tr.start_time` for test-run branch). Run `go generate`. Update resolver to populate the new field.
    2. **Frontend epic row** — change `{coveredCount}/{totalCount} covered` to `{pct}% ({coveredCount}/{totalCount})` where `pct = Math.round(coveredCount/totalCount*100)`. Add a thin visual bar (e.g. `<div style={{width: pct+'%', height:'4px', background:'var(--primary)', ...}}>`) under the text.
    3. **Frontend story row** — change badge from `✓ N` to `✓ N (Xp Yf)` using `testRunCoverage.passed` and `testRunCoverage.failed`. Show `lastRunAt` date beside the badge (e.g. `toLocaleDateString()`).
  - **Outcome**: Epic rows show "60% (3/5)" with a partial fill bar. Covered story rows show pass/fail split and last execution date.
  - **Context**: Req 2 AC5, Req 2 AC6, Req 3 AC3. `testRunCoverage` already carries `total/passed/failed`; only `lastRunAt` is new backend work.

- [x] **Task 6.7** 🟢 GREEN: Sort unassigned section by coverage percentage
  - **ID**: `task-6.7`
  - **BlockedBy**: `task-6.2`
  - **File**: `web/index.html`
  - **Change**: Add a sort control (e.g. a small `<select>` or toggle button) in the Unassigned section header with options "All (default)", "Covered first", "Uncovered first". Apply client-side sort to the `stories` array before rendering — covered stories sorted by `testRunCoverage.total` desc, uncovered after. No re-query required.
  - **Outcome**: User can sort the Unassigned section to surface the most or least covered stories.
  - **Context**: Req 2 AC3.

- [x] **Task 6.8** 🟢 GREEN: Sub-tasks as a third hierarchy level
  - **ID**: `task-6.8`
  - **BlockedBy**: `task-6.2`
  - **Agent**: `chief-programmer`
  - **File**: `internal/reporter/graphql/schema.graphql`, `internal/domains/integrations/coverage_service.go`, `web/index.html`
  - **Change**:
    1. **GraphQL** — add `SubTaskCoverageNode` type (same fields as `StoryCoverageNode`). Add `subTasks: [SubTaskCoverageNode!]!` to `StoryCoverageNode`. Run `go generate`.
    2. **Service** — in `CoverageService.Build`, identify sub-tasks by `issue.IssueType == "Sub-task"` (or having a story-type parent). Group them under their parent story node. Orphan sub-tasks (parent not in result set) go to Unassigned alongside stories.
    3. **Frontend** — render sub-task rows indented beneath each story (3rd level). A story with sub-tasks shows a collapse toggle. Sub-task rows follow the same covered/uncovered badge pattern as stories.
  - **Outcome**: Three-level hierarchy renders when the fix version contains sub-tasks. Projects with no sub-tasks are unaffected.
  - **Context**: Req 2 AC2. Note: this is the most complex new task — it touches service logic, schema, codegen, and frontend. Implement after 6.6 and 6.7.

---

## Dependency Diagram

```
task-1.1 (types+ifaces) ──┬──▶ task-2.1 🔴 ──▶ task-2.2 🟢 ──────────────────────┐
                           │                                                          ▼
                           ├──▶ task-3.1 🔴 ──▶ task-3.2 🟢 ──────────────────▶ task-4.2 🟢 ──▶ task-5.1 🔴 ──▶ task-5.2 🟢 ──▶ task-6.2 🟢 ──▶ task-6.3 🟢
                           │                                                          ▲
                           └──▶ task-4.1 🔴 ───────────────────────────────────────┘

task-1.2 (schema+codegen) ──────────────────────────────────────────────────────▶ task-5.1 🔴
                            └──▶ task-6.1 🔴 ──▶ task-6.2 🟢 ──▶ task-6.3 🟢 ──┬──▶ task-6.4 🟢
                                                                                   └──▶ task-6.5 🟢

task-1.3 (mock server) ──────────────────────────────────────────────────────── task-3.1 🔴
                         └──▶ task-6.1 🔴
```

New tasks (all unblock after 6.2):
```
task-6.2 ──┬──▶ task-6.3 🟢
            ├──▶ task-6.4 🟢 (done)
            ├──▶ task-6.5 🟢 (done)
            ├──▶ task-6.6 🟢 (% bar + pass/fail + last-run date)
            ├──▶ task-6.7 🟢 (sort unassigned)
            └──▶ task-6.8 🟢 (sub-tasks — most complex, do last)
```

**Parallel opportunities:**
- Tasks 1.1, 1.2: both root tasks — start simultaneously
- Task 1.3 starts as soon as 1.1 is done (types needed to define mock shapes)
- Tasks 2.1 (repo RED) and 4.1 (service RED) both start after 1.1 — run simultaneously
- Tasks 2.1 and 3.1 are independent RED tracks — run simultaneously after their prerequisites
- Task 4.1 (service RED, uses interface mocks) runs in parallel with 2.x and 3.x — no implementation needed to write the service tests
- Frontend track (1.2 → 6.1 → 6.2 → 6.3) runs in parallel with backend service/resolver track
- Tasks 6.4–6.8 all unblock after 6.2 and can be developed simultaneously (6.6 and 6.7 are frontend-only; 6.8 touches service layer)

**Critical path:** task-1.1 → task-3.1 → task-3.2 → task-4.2 → task-5.1 → task-5.2 → task-6.2 → task-6.8 (8 tasks, including new sub-task work)

Note: task-4.2 also waits for task-2.2 and task-4.1, but those can be completed in parallel with the 3.x chain.

---

## Completion Criteria

- [ ] All 18 tasks checked off
- [ ] `make test` fully green (all unit + resolver tests pass)
- [ ] `make test-acceptance` fully green (all 5 acceptance scenarios pass)
- [ ] Every implementation task was preceded by a failing test
- [ ] Coverage tab visible on projects with a JIRA connection; absent without one
- [ ] Fix version picker loads, filters, and groups correctly (unreleased before released)
- [ ] Three-level hierarchy renders (epics → stories → sub-tasks where present)
- [ ] Epic rows show `N% (X/Y)` with a visual fill bar
- [ ] Covered stories show test run count, pass/fail split, and last execution date
- [ ] Uncovered stories clearly marked
- [ ] "Show uncovered only" toggle works client-side, maintains hierarchy structure
- [ ] Unassigned section sortable by coverage
- [ ] JIRA API errors show user-friendly messages; tab does not crash
- [ ] No regressions in existing JIRA integration features
- [ ] Every JIRA issue key in the coverage tree links to the issue in JIRA (new tab)
- [ ] Clicking the test count on a covered story shows the tagged spec runs

---

## Post-original-scope enhancements (2026-06-15, demo prep)

Frontend-only changes to `web/index.html` (no build step; served from disk).
Satisfy Requirement 4 and the terminology updates in requirements.md. All done;
**uncommitted** in the working tree pending review.

- [x] **Results-aware color semantics** — badge/epic/release color encodes health,
  not just covered/uncovered: grey = uncovered, red = has a failing test, green =
  covered & passing; **skips no longer change color** (shown as `↺N` in text).
  Fixes "red = uncovered" colliding with "red = failing", and covered-but-failing
  stories no longer show green. (`StoryRow`, `EpicRow`.)
- [x] **Epic health roll-up** — epic color reflects descendant failures (red even at
  100% coverage); adds an `✗ N failing` chip. (`EpicRow`.)
- [x] **Release roll-up row** — new top-level `ReleaseSummary`: release version +
  labelled coverage (`<covered>/<total> covered · <pct>%`) + quantified health pill
  (Release ready / ✗ N failing / In progress / Not started). Hierarchy is now
  Release → Epic → Story → Sub-task.
- [x] **Terminology** — UI "fix version" → "release version"; picker placeholder
  "Filter versions…" → "Filter releases…"; "Unassigned Stories" → "Issues without
  an Epic". `fixVersion` JQL/API names unchanged.
- [x] **Placement note** — Coverage tab lives in Project Settings as interim home;
  permanent surface is the readiness dashboard (#30). (design.md.)

> Not yet covered by automated tests — the acceptance suite asserts the
> `.coverage-story-row`/`.covered` classes (unchanged), not the new colors/labels.
> Add assertions for Requirement 4 (release pill text, health colors) when the
> verification pass runs.

## Release picker: past-year window (2026-06-17)

Backend change to `internal/domains/integrations/jira_client_coverage.go`. **Uncommitted**
in the working tree pending review. Updates Requirement 1 and design Decision 2.

- [x] 🔴 RED: `TestDefaultJiraClient_GetEpicReleases` asserts the outgoing `jql` query param
  still restricts to `issuetype = Epic` and now contains `updated >= -52w`.
  (`jira_client_coverage_test.go`.)
- [x] 🟢 GREEN: Add `AND updated >= -52w` to the `GetEpicReleases` JQL. Package tests pass.
- **Why:** the picker paginated every release-bearing Epic in the project — slow to build and
  an unsearchably long dropdown. The clause is pushed into JQL so JIRA returns fewer Epics
  (fewer pages → faster) and only currently active releases appear.
- **Subtlety (see design Decision 2):** the release field is a free-text string with no date
  semantics, so the filter is on the **Epic's `updated`** timestamp, not the release value.
  `updated` (not `created`) keeps long-lived Epics still being worked visible. Trade-off: a
  release tied only to Epics untouched for >1 year drops off the picker — intended, but a
  behaviour change, not pure perf. Window hardcoded at `-52w` (JQL relative date, evaluated
  server-side by JIRA — no date math or timezone handling in Fern).

## Sub-tasks excluded (2026-06-17)

Backend + frontend change. **Uncommitted** in the working tree pending review. Updates
Requirement 2 (+ Amendment) and design Decision 3. The hierarchy is now Epic → Story only.

- [x] 🔴 RED: rewrote `coverage_service_test.go` "Phase 3 fetches Sub-tasks…" → "sub-tasks are
  not fetched and never attached to stories": asserts exactly 2 SearchIssues calls (no Phase 3)
  and `Stories[0].SubTasks` empty.
- [x] 🟢 GREEN: `coverage_service.go` — removed the Phase 3 sub-task fetch; Phase 2 now discards
  `issuetype.subtask` issues instead of supplementing them; `assembleTree` called with nil
  sub-tasks. Full integrations package + graphql package tests pass; `go build ./...` clean.
- [x] Frontend (`web/index.html`): removed the sub-task render block under `StoryRow`, dropped the
  now-unused `indent` param, simplified the `hidden` calc, and removed `subTasks` recursion from
  the ReleaseSummary and EpicRow health walks.
- **Why:** teams report on the main task; sub-tasks were noise. Also drops a whole pagination pass.
- **Decisions taken (minimal scope):**
  - GraphQL `subTasks` field on `StoryCoverageNode` **retained**, always returns `[]` — no schema
    change, no codegen, #30 (rebases onto #29) unaffected.
  - Sub-task-tagged tests **drop from view** (no roll-up to parent story) — matches prior counting.
  - `assembleTree` keeps its latent sub-task attachment logic + the assemble unit tests for it;
    production simply never passes sub-tasks. Documented so it isn't mistaken for live behaviour.
- **No coverage-number impact:** sub-tasks never rolled into epic or story counts.
- Not exercised by automated frontend tests (acceptance suite needs a deployed stack); verify the
  tree renders with no sub-task rows during the next e2e pass.
