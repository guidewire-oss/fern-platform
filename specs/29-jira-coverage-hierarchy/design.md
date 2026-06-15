# 29-jira-coverage-hierarchy - Design

## 1. System Architecture

### Directory Structure

```
fern-platform/
├── internal/
│   ├── jira/
│   │   ├── client.go                    (modify: add SearchIssues, GetVersions, pagination)
│   │   ├── model.go                     (modify: add JiraVersion, JiraIssue, JiraParent)
│   │   └── coverage_service.go          (new: two-phase fetch + Fern coverage cross-ref)
│   ├── graph/
│   │   ├── schema.graphql               (modify: add JiraVersion, coverage types + queries)
│   │   ├── model/
│   │   │   └── models_gen.go            (auto-generated — do not edit)
│   │   └── resolvers/
│   │       └── coverage.resolvers.go    (new: requirementCoverage + jiraFixVersions)
│   └── repository/
│       └── tag_repository.go            (modify: add GetJiraTagCoverageByProject)
└── web/
    └── index.html                       (modify: add Coverage tab, version picker, tree UI)
```

### Component Diagram

```
Browser                  GraphQL Server             External
  │                           │
  │──jiraFixVersions(proj)────▶│
  │                           │──GetVersions(projectKey)──▶ JIRA REST API
  │◀──[JiraVersion]────────────│
  │                           │
  │──requirementCoverage()────▶│
  │                           │──SearchIssues(JQL phase1)─▶ JIRA REST API
  │                           │──SearchIssues(JQL phase2)─▶ JIRA REST API (if needed)
  │                           │──GetJiraTagCoverage()─────▶ Fern DB (PostgreSQL)
  │◀──RequirementCoverageTree──│
```

## 2. Data Flow

### Fix Version Picker Load

1. User opens the Coverage tab for a project.
2. Frontend calls GraphQL `jiraFixVersions(projectId: "...")`.
3. Resolver fetches `JiraConnection` for the project (existing service call).
4. `jira_client.GetVersions(projectKey)` calls `GET /rest/api/3/project/{projectKey}/versions`.
5. Results returned as `[JiraVersion]` — unreleased/released flag + releaseDate included.
6. Frontend groups client-side (unreleased first, alphabetical; released below, newest first) and renders the searchable picker.

### Coverage Tree Load

1. User selects a fix version name from the picker.
2. Frontend calls `requirementCoverage(projectId: "...", fixVersionName: "Atmos vNext")`.
3. Resolver calls `CoverageService.Build(projectId, fixVersionName)`.
4. **Phase 1 — fix version issues:** JQL `fixVersion = "Atmos vNext" ORDER BY issuetype`, fields `key,summary,status,issuetype,parent`, paginated (`maxResults=100`, loop until `startAt + len(results) >= total`).
5. Separate results into epics (issuetype = Epic) and non-epics. Collect parent epic keys from non-epic issues.
6. **Phase 2 — missing epics:** Keys in parent set but absent from Phase 1 → single batched JQL `issueKey IN (PROJ-10,PROJ-11,...)`; skip if set is empty.
7. **Fern coverage:** `tag_repository.GetJiraTagCoverageByProject(ctx, projectID)` — returns `map[issueKey]CoverageCount{Total, Passed, Failed, Skipped, LastRunAt}`, aggregating JIRA tags at both spec-run and test-run granularity.
8. **Assemble tree:** For each epic, collect its stories from Phase 1/2 results. Cross-reference story keys against the Fern coverage map. Group stories with no parent under "Unassigned".
9. Return `RequirementCoverageTree` — epics with story children + unassigned stories.

## 3. Interface Specifications

### New GraphQL Types (graph/schema.graphql)

```graphql
type JiraVersion {
  id:          String!
  name:        String!
  released:    Boolean!
  releaseDate: String      # ISO date string, null if not released
}

type JiraIssueSummary {
  key:        String!
  summary:    String!
  statusName: String!
  issueType:  String!
}

type TestRunCoverage {
  total:  Int!
  passed: Int!
  failed: Int!
}

type StoryCoverageNode {
  issue:           JiraIssueSummary!
  covered:         Boolean!
  testRunCoverage: TestRunCoverage  # null if not covered
}

type EpicCoverageNode {
  issue:        JiraIssueSummary!
  stories:      [StoryCoverageNode!]!
  coveredCount: Int!
  totalCount:   Int!
}

type RequirementCoverageTree {
  fixVersion: JiraVersion!
  epics:      [EpicCoverageNode!]!
  unassigned: [StoryCoverageNode!]!
}

extend type Query {
  jiraFixVersions(projectId: ID!): [JiraVersion!]!
  requirementCoverage(projectId: ID!, fixVersionName: String!): RequirementCoverageTree!
}
```

### New JIRA Client Methods (internal/jira/client.go)

| Method | JIRA Endpoint | Notes |
|--------|--------------|-------|
| `GetVersions(projectKey string) ([]JiraVersion, error)` | `GET /rest/api/3/project/{projectKey}/versions` | Full list; no pagination |
| `SearchIssues(jql string, fields []string) ([]JiraIssue, error)` | `POST /rest/api/3/issue/search` | Paginates via `startAt`; `maxResults=100` per page |

Issue search uses POST to avoid URL length limits when the `issueKey IN (...)` batch is large.

### New Repository Method (internal/repository/tag_repository.go)

```go
// GetJiraTagCoverageByProject returns per-JIRA-issue-key coverage counts
// for all test runs in the given project, honoring JIRA tags applied at
// BOTH granularities: spec-run level (individual tests) and test-run level
// (whole run).
GetJiraTagCoverageByProject(ctx context.Context, projectID string) (map[string]CoverageCount, error)
```

**Both tagging granularities count.** A `category='jira'` tag may be applied to a
`spec_run` (an individual test) or to a whole `test_run`. Per requirements §1–2, a
story is covered if *either* carries a matching `jira:{issueKey}` tag, so coverage
aggregates over both. This is expressed as a `UNION ALL` of the two tag tables —
`spec_run_tags` (joined `spec_runs → suite_runs → test_runs`) and `test_run_tags`
(joined `test_runs`) — each contributing its `status` and `start_time`. The query
must use `db.Raw(...)` rather than GORM's fluent API, which cannot express UNION.
Soft-deleted rows are excluded on both legs.

SQL:
```sql
SELECT t.value AS issue_key,
       COUNT(*)                                                   AS total,
       SUM(CASE WHEN tagged.status = 'passed'  THEN 1 ELSE 0 END) AS passed,
       SUM(CASE WHEN tagged.status = 'failed'  THEN 1 ELSE 0 END) AS failed,
       SUM(CASE WHEN tagged.status = 'skipped' THEN 1 ELSE 0 END) AS skipped,
       MAX(tagged.run_at)                                         AS last_run_at
FROM   tags t
JOIN (
    -- spec-run-level tags (individual test granularity)
    SELECT srt.tag_id, sr.status, sr.start_time AS run_at
    FROM   spec_run_tags srt
    JOIN   spec_runs  sr ON sr.id = srt.spec_run_id
    JOIN   suite_runs su ON su.id = sr.suite_run_id
    JOIN   test_runs  tr ON tr.id = su.test_run_id
    WHERE  tr.project_id = $1 AND sr.deleted_at IS NULL

    UNION ALL

    -- test-run-level tags (whole-run granularity)
    SELECT trt.tag_id, tr.status, tr.start_time AS run_at
    FROM   test_run_tags trt
    JOIN   test_runs tr ON tr.id = trt.test_run_id
    WHERE  tr.project_id = $1 AND tr.deleted_at IS NULL
) tagged ON tagged.tag_id = t.id
WHERE  t.category = 'jira'
GROUP  BY t.value
```

### New Mock JIRA Endpoints (acceptance/helpers/mock_jira_server.go)

| Endpoint | Purpose |
|----------|---------|
| `GET /rest/api/3/project/{key}/versions` | Returns test version list |
| `POST /rest/api/3/issue/search` | Returns paginated issue results filtered by JQL |

### Frontend (web/index.html)

| Element | Location | Notes |
|---------|----------|-------|
| "Coverage" tab | Project **Settings** page left-nav (alongside General / Integrations / Team / Notifications) | Visible only when project has active JIRA connection |
| Fix version picker | Top of Coverage panel | Text `<input>` filter + dropdown; grouped unreleased/released |
| Coverage tree | Below picker | Epic rows expand to story rows; key, summary, status, coverage badge |
| "Show uncovered only" toggle | Above tree | Checkbox; client-side filter |
| No-connection message | Coverage panel | Shown when project has no JIRA connection |

> **Placement is interim.** The Coverage view lives in the Project Settings page
> for now as the pragmatic home while the feature lands. The intended permanent
> surface for JIRA completion / release-readiness display is the **readiness
> dashboard** ([issue #30](https://github.com/guidewire-oss/fern-platform/issues/30)),
> owned separately. When #30 ships, this Coverage view should move (or be
> superseded) there; nothing here should be read as committing to Project
> Settings as the final location.

**Color semantics (all levels).** Coverage breadth and test health are SEPARATE
axes. Color encodes *health*; failures (red) block "ready", **skips do not**
(skip counts still appear as `↺N` in text):
- **grey** — uncovered / not started (no tests)
- **red** — has ≥1 failing test
- **green** — covered, no failures (skips allowed)
- **neutral** — (rollups only) partially covered with no failures = in progress

Story badge: grey `uncovered` / red `✗ total (✓p ✗f ↺s)` / green `✓ total`.

Epic row: the **% and bar width** are coverage breadth (covered/total stories);
the **color** is health (red if any story incl. sub-tasks has a failing test,
even at 100%; green at 100% no fails; grey at 0%; neutral when partial). A red
`✗ N failing` chip shows the count of failing issues.

**Release roll-up (top line).** The tree is headed by a release summary
(`ReleaseSummary`): the selected release version, a quantified health pill
(**Release ready** / **✗ N failing** / **In progress** / **Not started**), and a
**labelled** coverage figure `<covered>/<total> covered · <pct>%` aggregated
across all epics + Issues-without-an-Epic (walking sub-tasks). Coverage and health
are shown as distinct elements so neither is misread as the other. Hierarchy is
**Release → Epic → Story → Sub-task**.

## 4. Technical Decisions

### Decision 1: Fix version passed as name, not ID

**Choice:** `requirementCoverage(fixVersionName: String!)` — pass the version name.

**Rationale:** JIRA JQL uses the version name (`fixVersion = "Atmos vNext"`), not the internal numeric ID. Passing the name avoids a name→ID lookup step on the backend.

**Alternatives Considered:** Pass ID and look up name server-side — extra round-trip with no benefit.

### Decision 2: Client-side fix version filtering

**Choice:** Fetch all versions once (`jiraFixVersions`) and filter client-side as the user types.

**Rationale:** The JIRA versions endpoint returns the full list in a single call (measured: 3,437 GWCP versions in ~0.67s). There is no server-side search API. Client-side substring match on the pre-loaded list is instant.

**Alternatives Considered:** Re-query on every keystroke — unnecessary latency, no API to support it.

### Decision 3: Paginate Phase 1 JQL (maxResults=100)

**Choice:** Loop Phase 1 calls using `startAt`/`total` until all results are fetched.

**Rationale:** JIRA Cloud caps results at 100 per call. Requirement supports up to 500 issues per fix version. Pagination is required.

**Alternatives Considered:** Request `maxResults=500` — JIRA API silently clamps to 100; would truncate silently.

### Decision 4: POST for issue search

**Choice:** Use `POST /rest/api/3/issue/search` with a JSON body.

**Rationale:** JQL strings and `issueKey IN (...)` batches for large epics sets can exceed URL length limits with GET.

**Alternatives Considered:** GET with URL-encoded JQL — unreliable for batches of 50+ keys.

### Decision 5: No caching in v1

**Choice:** Every coverage tree request fetches live from JIRA.

**Rationale:** Simplest correct implementation. Cache invalidation (issue status changes, new issues added to a version) adds complexity. Coverage view is a management dashboard used infrequently.

**Alternatives Considered:** Server-side TTL cache per (projectId, fixVersionName) — deferred to v2 if latency proves problematic.

### Decision 6: `parent` field for epic linking

**Choice:** Use the JIRA `parent` field, not `customfield_epicLink`.

**Rationale:** `parent` works uniformly across JIRA Cloud classic and next-gen projects. `customfield_epicLink` is legacy and absent from next-gen projects. Aligns with `FernFieldParentRequirement: "parent"` already in the field mapping service.

**Alternatives Considered:** `customfield_epicLink` — only classic projects; breaks next-gen.

## 5. Error Handling

| Scenario | Detection | Response |
|----------|-----------|----------|
| No JIRA connection for project | `JiraConnectionService.GetByProject` returns not-found | GraphQL error: "No JIRA connection configured for this project" |
| JIRA API unreachable / timeout | HTTP error or network timeout from `jira_client` | GraphQL error: "JIRA API unavailable — please try again" |
| JIRA authentication failure (401/403) | HTTP 401/403 from JIRA | GraphQL error: "JIRA credentials are invalid or expired" |
| Fix version has no issues | Phase 1 returns 0 results | Return tree with empty `epics` and `unassigned`; not an error |
| Phase 2 parent batch fails | JIRA error on `issueKey IN (...)` call | Return tree with Phase 1 epics only; log warning; partial result acceptable |
| Fern DB query fails | DB error in `GetJiraTagCoverageByProject` | GraphQL error: "Failed to load coverage data" |
| Fix version name contains special chars | Unescaped name in JQL string | Escape `fixVersionName` before embedding (replace `"` with `\"`); reject names containing `;` |

## 6. Testing Approach

### Unit Tests (Go, Ginkgo/Gomega)

**coverage_service_test.go:**
- Phase 2 triggered only when parent epic keys are absent from Phase 1
- Phase 2 skipped when all parent epics are already in Phase 1 results
- Stories correctly grouped under their parent epics
- Stories with no parent grouped under unassigned
- Coverage cross-reference: covered/uncovered correctly computed from tag map
- Empty fix version: empty tree returned without error

**tag_repository_test.go (go-sqlmock):**
- `GetJiraTagCoverageByProject` returns correct counts per issue key
- Rows with `category != 'jira'` excluded

**jira_client_test.go:**
- `SearchIssues` paginates correctly when `total > maxResults`
- POST body includes correct `fields` and `jql` params
- `GetVersions` parses unreleased and released versions correctly

### Acceptance Tests (Ginkgo, mock JIRA server)

- **Happy path:** mock returns Phase 1 issues (mix of epics and stories, some parents missing from Phase 1); verify Phase 2 fetches only the missing parent epics; verify tree structure
- **No-connection error:** project without JIRA connection returns appropriate GraphQL error; frontend shows message
- **JIRA unavailable:** mock returns 503; frontend shows error, no crash
- **"Show uncovered only" toggle:** covered stories hidden client-side, uncovered remain visible
- **Fix version picker grouping:** unreleased versions appear before released; released sorted newest-first

### Smoke Tests

1. Open Coverage tab on a project with an active JIRA connection
2. Select a known fix version; verify hierarchy renders with at least one epic
3. Verify at least one covered story appears if jira-tagged test runs exist for the project
4. Toggle "Show uncovered only" and confirm covered stories disappear
