# 29-jira-coverage-hierarchy - Design

## 1. System Architecture

### Directory Structure

```
fern-platform/
├── internal/
│   ├── domains/integrations/
│   │   ├── jira_client_coverage.go      (modify: GetEpicReleases, SearchIssues)
│   │   ├── coverage_service.go          (modify: Epic-first two-phase cascade)
│   │   └── types.go                     (modify: JiraIssue, JiraVersion, JiraParent)
│   ├── reporter/graphql/
│   │   ├── schema.graphql               (modify: update query signatures)
│   │   └── schema.resolvers.go          (modify: jiraReleases, requirementCoverage)
│   └── domains/tags/
│       └── infrastructure/
│           └── gorm_tag_repository.go   (unchanged: GetJiraTagCoverageByProject)
├── migrations/
│   └── 000024_add_release_field_id_to_jira_connections.up.sql  (new)
└── web/
    └── index.html                       (modify: release picker, tree UI)
```

### Component Diagram

```
Browser                  GraphQL Server             External
  │                           │
  │──jiraReleases(projId)─────▶│
  │                           │──GetEpicReleases(fieldId,key)─▶ JIRA REST API
  │◀──[String]────────────────│   (distinct cf[fieldId] values on Epics)
  │                           │
  │──requirementCoverage()────▶│
  │                           │──SearchIssues(Phase1: cf[]=value, Epic)──▶ JIRA
  │                           │──SearchIssues(Phase2: parent IN epics)───▶ JIRA
  │                           │──SearchIssues(Phase3: parent IN stories)─▶ JIRA
  │                           │──GetJiraTagCoverage()────────────────────▶ Fern DB
  │◀──RequirementCoverageTree──│
```

## 2. Data Flow

### Release Picker Load

1. User opens the Coverage tab for a project.
2. Frontend calls GraphQL `jiraReleases(projectId: "...")`.
3. Resolver fetches `JiraConnection` for the project, reads `releaseFieldId` and `projectKey`.
4. `jira_client.GetEpicReleases(ctx, baseURL, projectKey, releaseFieldId, username, credential, authType)` calls:
   ```
   GET /rest/api/3/search/jql?jql=project={key} AND issuetype=Epic AND cf[{fieldId}] is not EMPTY AND updated >= -52w ORDER BY cf[{fieldId}] ASC&fields=cf[{fieldId}]&maxResults=100
   ```
   Paginates until all matching Epics are fetched. Collects distinct non-null values of the custom field.
   The `updated >= -52w` clause scopes the picker to releases active in the past year (see Decision 2).
5. Distinct release values returned as `[String]` — sorted alphabetically.
6. Frontend renders the searchable picker.

### Coverage Tree Load

1. User selects a release value from the picker.
2. Frontend calls `requirementCoverage(projectId: "...", releaseValue: "OLOS (2025.06M)")`.
3. Resolver calls `CoverageService.Build(ctx, projectId, releaseValue)`.
4. Service reads `JiraConnection` → `releaseFieldId`, `projectKey`, credentials.
5. **Phase 1 — Epics:** JQL `cf[{fieldId}] = "{releaseValue}" AND issuetype = Epic ORDER BY key`,
   fields `key,summary,status,issuetype`, paginated via cursor (`nextPageToken`).
6. **Phase 2 — Stories:** For each chunk of ≤50 epic keys:
   `parent IN (PROJ-1,...,PROJ-50) ORDER BY key`, fields `key,summary,status,issuetype,parent`.
   Results merged across chunks. Any issue with `issuetype.subtask = true` is discarded
   (sub-tasks are excluded — see Decision 3); there is no Phase 3 sub-task fetch.
7. **Fern coverage:** `GetJiraTagCoverageByProject(ctx, projectID)` returns
   `map[issueKey]CoverageCount{Total, Passed, Failed, Skipped, LastRunAt}`.
8. **Assemble tree:** stories attached to their parent epic; stories with unresolvable parents
   go to "Issues without an Epic". Cross-reference all keys against the Fern coverage map.
10. Return `RequirementCoverageTree`.

## 3. Interface Specifications

### GraphQL Types (schema.graphql)

```graphql
# A release value returned by the release picker.
# Simple string — the distinct value of the configured custom field on Epics.
# (Named JiraRelease to reserve the type name; may gain metadata in future.)
type JiraRelease {
  name: String!
}

type JiraIssueSummary {
  key:        String!
  summary:    String!
  statusName: String!
  issueType:  String!
}

type TestRunCoverage {
  total:     Int!
  passed:    Int!
  failed:    Int!
  skipped:   Int!
  lastRunAt: String   # ISO datetime, null if not available
}

type StoryCoverageNode {
  issue:           JiraIssueSummary!
  covered:         Boolean!
  testRunCoverage: TestRunCoverage   # null if not covered
  subTasks:        [StoryCoverageNode!]!
}

type EpicCoverageNode {
  issue:        JiraIssueSummary!
  stories:      [StoryCoverageNode!]!
  coveredCount: Int!
  totalCount:   Int!
}

type RequirementCoverageTree {
  release:    JiraRelease!
  epics:      [EpicCoverageNode!]!
  unassigned: [StoryCoverageNode!]!
}

extend type Query {
  # Returns distinct release values from the configured custom field on Epics.
  jiraReleases(projectId: ID!): [JiraRelease!]!

  # Builds the full coverage tree for the selected release value.
  requirementCoverage(projectId: ID!, releaseValue: String!): RequirementCoverageTree!

  # Returns spec runs tagged with a specific JIRA issue key (drill-down).
  specRunsByJiraTag(projectId: ID!, issueKey: String!): [SpecRunSummary!]!
}
```

### JIRA Client Methods

| Method | JIRA Endpoint | Notes |
|--------|--------------|-------|
| `GetEpicReleases(ctx, baseURL, projectKey, releaseFieldId, username, credential, authType)` | `GET /rest/api/3/search/jql` | JQL: `project={key} AND issuetype=Epic AND cf[{fieldId}] is not EMPTY AND updated >= -52w`; collect distinct non-null field values; paginated. The `updated >= -52w` window bounds the picker to the past year (Decision 2). |
| `SearchIssues(ctx, baseURL, username, credential, authType, jql, fields)` | `GET /rest/api/3/search/jql` | Cursor-based pagination via `nextPageToken`; `maxResults=100` per page |

### New/Modified Service Methods

```go
// GetReleasesForProject returns distinct release values from the custom field
// on Epics for the project's JIRA connection.
func (s *CoverageService) GetReleasesForProject(ctx context.Context, projectID string) ([]string, error)

// Build fetches the Epic→Story hierarchy for the release value
// and cross-references with Fern tag coverage. (Sub-tasks are excluded — see Decision 3.)
func (s *CoverageService) Build(ctx context.Context, projectID, releaseValue string) (*CoverageTree, error)
```

### New Migration

```sql
-- 000024_add_release_field_id_to_jira_connections.up.sql
ALTER TABLE jira_connections ADD COLUMN release_field_id TEXT NOT NULL DEFAULT '';
```

`release_field_id` stores the JIRA custom field ID for release-scope determination
(e.g. `"12345"` for `cf[12345]`). Empty string means not yet configured; the coverage
resolver returns an appropriate error when unset.

### Repository Method (unchanged)

```go
// GetJiraTagCoverageByProject returns per-JIRA-issue-key coverage counts
// for all test runs in the given project, honoring JIRA tags applied at
// BOTH granularities: spec-run level (individual tests) and test-run level
// (whole run).
GetJiraTagCoverageByProject(ctx context.Context, projectID string) (map[string]CoverageCount, error)
```

**Both tagging granularities count.** A `category='jira'` tag may be applied to a
`spec_run` (an individual test) or to a whole `test_run`. Coverage aggregates over both.
Expressed as a `UNION ALL` of the two tag tables — `spec_run_tags` and `test_run_tags`.
The query uses `db.Raw(...)` because GORM's fluent API cannot express UNION.
Soft-deleted rows are excluded on both legs.

SQL:
```sql
SELECT t.value AS issue_key,
       COUNT(*)                                                    AS total,
       SUM(CASE WHEN tagged.status = 'passed'  THEN 1 ELSE 0 END) AS passed,
       SUM(CASE WHEN tagged.status = 'failed'  THEN 1 ELSE 0 END) AS failed,
       SUM(CASE WHEN tagged.status = 'skipped' THEN 1 ELSE 0 END) AS skipped,
       MAX(tagged.run_at)                                          AS last_run_at
FROM   tags t
JOIN (
    SELECT srt.tag_id, sr.status, sr.start_time AS run_at
    FROM   spec_run_tags srt
    JOIN   spec_runs  sr ON sr.id = srt.spec_run_id
    JOIN   suite_runs su ON su.id = sr.suite_run_id
    JOIN   test_runs  tr ON tr.id = su.test_run_id
    WHERE  tr.project_id = $1 AND sr.deleted_at IS NULL

    UNION ALL

    SELECT trt.tag_id, tr.status, tr.start_time AS run_at
    FROM   test_run_tags trt
    JOIN   test_runs tr ON tr.id = trt.test_run_id
    WHERE  tr.project_id = $1 AND tr.deleted_at IS NULL
) tagged ON tagged.tag_id = t.id
WHERE  t.category = 'jira'
GROUP  BY t.value
LIMIT  1000
```

### Mock JIRA Endpoints (acceptance/helpers/mock_jira_server.go)

| Endpoint | Purpose |
|----------|---------|
| `GET /rest/api/3/search/jql` | Returns paginated issues filtered by JQL (used for all three phases + release picker) |

### Frontend (web/index.html)

| Element | Location | Notes |
|---------|----------|-------|
| "Coverage" tab | Project **Settings** page left-nav | Visible only when project has active JIRA connection with `releaseFieldId` set |
| Release picker | Top of Coverage panel | Text `<input>` filter + dropdown; options from `jiraReleases` query |
| Coverage tree | Below picker | Epic rows expand to story rows |
| "Show uncovered only" toggle | Above tree | Checkbox; client-side filter |
| No-connection / not-configured message | Coverage panel | Shown when project has no JIRA connection or `releaseFieldId` not set |

> **Placement is interim.** The Coverage view lives in Project Settings for now. The
> intended permanent surface for release-readiness display is the **readiness dashboard**
> ([issue #30](https://github.com/guidewire-oss/fern-platform/issues/30)), owned
> separately. When #30 ships, this view should move or be superseded there.

**Color semantics (all levels).** Coverage breadth and test health are SEPARATE axes.
Color encodes *health*; failures (red) block "ready", **skips do not**:
- **grey** — uncovered / not started (no tests)
- **red** — has ≥1 failing test
- **green** — covered, no failures (skips allowed)
- **neutral** — (rollups only) partially covered with no failures = in progress

Story badge: grey `uncovered` / red `✗ total (✓p ✗f ↺s)` / green `✓ total`.

Epic row: the **% and bar width** are coverage breadth (covered/total stories); the
**color** is health. A red `✗ N failing` chip shows the count of failing issues.

**Release roll-up (top line).** The tree is headed by a release summary (`ReleaseSummary`):
the selected release value, a health pill (**Release ready** / **✗ N failing** /
**In progress** / **Not started**), and a labelled coverage figure
`<covered>/<total> covered · <pct>%` aggregated across all epics + Issues-without-an-Epic.
Hierarchy is **Release → Epic → Story**.

## 4. Technical Decisions

### Decision 1: Release value passed as plain string

**Choice:** `requirementCoverage(releaseValue: String!)` — pass the release value as a string.

**Rationale:** The release value is whatever the custom field contains (e.g. `"OLOS (2025.06M)"`).
It is used verbatim in JQL: `cf[{fieldId}] = "{releaseValue}"`. No ID lookup is needed.

**Alternatives Considered:** Pass a structured `JiraRelease` object — unnecessary; the name is
the only field used in queries.

### Decision 2: Release picker populated from Epic custom field values

**Choice:** `jiraReleases` queries Epics for distinct non-null values of `cf[releaseFieldId]`.

**Rationale:** There is no standard JIRA API for listing custom field values in use. Querying
Epics that have the field set gives the actual in-use release values for this project, which
is exactly what the picker needs.

**Alternatives Considered:** JIRA's field context options API (`/rest/api/3/field/{id}/context`)
— returns all *possible* values, not just those in use; produces a noisy picker for large field
option lists.

**Refinement — past-year window (`updated >= -52w`):** The picker's cost is dominated by
paginating every release-bearing Epic in the project, and for long-lived projects the resulting
dropdown was both slow to build and hard to search. We bound the query with `AND updated >= -52w`,
pushing the filter into the JQL so JIRA returns fewer Epics (fewer pages → faster) and the
dropdown only shows currently active releases.

The subtlety is *what* we filter on. The release custom field is a **free-text string** (e.g.
`v9.3`) with no date semantics — the client unmarshals it as a `string` — so we cannot filter on
the release value itself. Instead we filter on the Epic's **`updated`** timestamp as a proxy for
"recent release":

- `updated` (not `created`) is deliberate: an Epic created years ago but still being worked toward
  an upcoming release has a recent `updated` time and stays visible; using `created` would wrongly
  drop it.
- Trade-off: a release attached *only* to Epics untouched in the past year disappears from the
  picker. For a release picker this is the desired behaviour (you want current releases), but it
  is a behaviour change, not a pure performance optimization — old releases are no longer listed.
- The window is **hardcoded** at 52 weeks. If a tunable window is needed later, surface it via
  `JiraFieldMapping` (feature #26) rather than hardcoding a different constant.
- `-52w` is JQL relative-date syntax (52 weeks before now), evaluated by JIRA server-side, so no
  date math happens in Fern and there is no timezone ambiguity on our side.

### Decision 3: Two-phase cascade with chunked batches (sub-tasks excluded)

**Choice:** Phase 1 fetches Epics; Phase 2 fetches Stories by `parent IN (epics)`, chunking the
key list into ≤50 keys per request. **Sub-tasks are not fetched or displayed** — coverage reports
at the main-task (Story) level.

**Rationale:** The cascade naturally handles the case where stories do not carry the release
signal themselves (only their parent Epic does). Chunking at 50 keeps JQL URL lengths well within
JIRA's limits even for large releases.

**Sub-tasks dropped (2026-06-17):** originally a Phase 3 fetched sub-tasks by
`parent IN (stories)` and they rendered as indented rows under each story. Teams report on the
main task, so sub-tasks were noise; removing the phase also drops a whole pagination pass.
Implementation notes:
- Phase 2 already separates out `issuetype.subtask = true` issues; those are now simply discarded
  instead of kept and supplemented by Phase 3.
- No coverage-number impact: sub-tasks never contributed to epic or story counts.
- A test tagged directly on a sub-task key no longer appears anywhere (intended).
- Minimal scope: the GraphQL `subTasks` field on `StoryCoverageNode` is retained and always
  returns `[]` (no schema change → #30 unaffected); `assembleTree` keeps its latent sub-task
  attachment logic (still unit-tested) but the service passes it no sub-tasks.

**Alternatives Considered:** Single JQL with `cf[id] in subtaskOf(...)` — not supported by
JIRA Cloud. Fetching all children recursively per-epic — one request per epic; too many calls.

### Decision 4: GET for issue search

**Choice:** Use `GET /rest/api/3/search/jql` with query parameters.

**Rationale:** The JIRA Cloud cursor-based search endpoint (`/rest/api/3/search/jql`) is
GET-only. Cursor-based pagination (`nextPageToken`) is simpler than `startAt`/`total`
arithmetic. URL length is manageable with ≤50-key chunks.

**Alternatives Considered:** `POST /rest/api/3/issue/search` — not supported on the cursor
endpoint; requires the older offset-based pagination API.

### Decision 5: No caching in v1

**Choice:** Every coverage tree request fetches live from JIRA.

**Rationale:** Simplest correct implementation. Cache invalidation (status changes, new issues
added) adds complexity. Coverage view is a management dashboard used infrequently.

**Alternatives Considered:** Server-side TTL cache per `(projectId, releaseValue)` — deferred
to v2 if latency proves problematic.

### Decision 6: `parent` field for story/sub-task linking

**Choice:** Use the JIRA `parent` field for all parent-child relationships.

**Rationale:** `parent` works uniformly across JIRA Cloud classic and next-gen projects.
`customfield_epicLink` is legacy and absent from next-gen projects.

**Alternatives Considered:** `customfield_epicLink` — only classic projects; breaks next-gen.

### Decision 7: `releaseFieldId` stored on JiraConnection

**Choice:** Add `release_field_id TEXT` column to `jira_connections` table.

**Rationale:** The custom field ID is organization-specific and must be configurable per JIRA
connection. The JIRA connection record is already the canonical store for per-project JIRA
configuration, so this is the natural home.

**Alternatives Considered:** Store in `JiraFieldMapping` (feature #26) — that table maps Fern
result fields to JIRA issue fields; `releaseFieldId` is a structural configuration, not a
field mapping. Different concern, different table.

### Decision 8: fixVersion approach archived as pluggable strategy reference

**Choice:** The original `fixVersion`-based implementation is removed from the live code path
and preserved as a documented reference design.

**Rationale:** `fixVersion` does not work for teams where the release signal is on Epics via a
custom field. Sub-tasks in those projects can't carry `fixVersion`, making the sub-task level
invisible. The Epic-first cascade is strictly more powerful.

The `fixVersion` approach remains valid for open-source adopters and standard JIRA users. It is
the reference implementation for a future `ReleaseDimension` pluggable module interface
(see requirements *Deferred Strategies* section and
[issue #197](https://github.com/guidewire-oss/fern-platform/issues/197)).

## 5. Error Handling

| Scenario | Detection | Response |
|----------|-----------|----------|
| No JIRA connection for project | `FindActiveByProjectID` returns empty | GraphQL error: "No JIRA connection configured for this project" |
| `releaseFieldId` not configured | `conn.ReleaseFieldID()` is empty string | GraphQL error: "Release field ID not configured — set it in Integration settings" |
| JIRA API unreachable / timeout | HTTP error or 30s context timeout | GraphQL error: "JIRA API unavailable — please try again" |
| JIRA authentication failure (401/403) | HTTP 401/403 from JIRA | GraphQL error: "JIRA credentials are invalid or expired" |
| Release value has no Epics | Phase 1 returns 0 results | Return tree with empty `epics` and `unassigned`; not an error |
| Phase 2 or 3 chunk fails | JIRA error on `parent IN (...)` call | Propagate error; do not return partial tree silently |
| Fern DB query fails | DB error in `GetJiraTagCoverageByProject` | GraphQL error: "Failed to load coverage data" |
| Release value contains special chars | Raw value embedded in JQL | Validate: reject values containing `;`; quote value with `%q` in JQL string |

## 6. Testing Approach

### Unit Tests (Go, standard testing)

**coverage_service_test.go:**
- Phase 1 returns Epics; Phase 2 fetches Stories
- Sub-tasks are not fetched (no Phase 3) and never attached to stories
- Phase 2 is skipped when the parent list is empty
- Key lists are correctly chunked into ≤50 per request
- Stories correctly grouped under their parent epics
- Issues with `issuetype.subtask = true` returned by Phase 2 are discarded
- Orphaned stories go to Unassigned
- Coverage cross-reference: covered/uncovered correctly computed from tag map
- Empty release: empty tree returned without error
- `releaseFieldId` empty: error returned before any JIRA calls

**coverage_assemble_test.go (package integrations):**
- `assembleTree`: empty version, orphan story, epic coverage counts, `buildStoryNode`
  covered/uncovered flag. (Retains orphan/attached sub-task cases — the helper keeps its
  latent sub-task support even though the service no longer feeds it sub-tasks.)

**jira_client_coverage_test.go:**
- `GetEpicReleases` returns distinct non-null field values; handles pagination
- `SearchIssues` paginates correctly via `nextPageToken`
- `issuetype.subtask` boolean correctly parsed

**tag_repository_test.go (go-sqlmock):**
- `GetJiraTagCoverageByProject` returns correct counts per issue key
- Rows with `category != 'jira'` excluded

### Acceptance Tests (Ginkgo, mock JIRA server)

- **Happy path:** mock returns Phase 1 Epics, Phase 2 Stories; verify the two-level
  Epic → Story hierarchy renders correctly (no sub-task rows)
- **No-connection error:** project without JIRA connection shows appropriate message
- **releaseFieldId not set:** appropriate error shown; no JIRA calls made
- **JIRA unavailable:** mock returns 503; frontend shows error, no crash
- **"Show uncovered only" toggle:** covered stories hidden client-side
- **Release picker:** options populated from distinct Epic custom field values

### Smoke Tests

1. Open Coverage tab on a project with an active JIRA connection and `releaseFieldId` set
2. Select a known release value; verify hierarchy renders with at least one epic
3. Verify at least one covered story appears if jira-tagged test runs exist
4. Toggle "Show uncovered only" and confirm covered stories disappear
