# JIRA-Level Test Coverage — Design (MVP)

**Status:** Active — scoped down from the original scenario-level design (parked in `./future/`).
**Companion:** `requirements.md` (this directory), `reviews/jira-sync-persistence-analysis/SUMMARY.md` (decision rationale).

---

## Architecture overview

```
┌────────────────────────────────────────────────────────────────────┐
│                                                                    │
│  fern-clients (per language — outside this spec)                   │
│    Test reporters emit:  tags: ["jira:GWCP-12345", "env:dev", ...]│
│                                                                    │
└──────────────────────────────────┬─────────────────────────────────┘
                                   │ POST /api/v1/test-runs (REST)
                                   ▼
┌────────────────────────────────────────────────────────────────────┐
│  fern-platform: existing ingestion path (no change)                │
│    Tag-bearing JSON → domain.NewTag() auto-splits + lowercases     │
│      (jira:GWCP-12345 → category=jira, value=gwcp-12345)           │
│    Writes:  tags  +  one of {test_run_tags,                        │
│                              suite_run_tags,                        │
│                              spec_run_tags}                         │
│             (whichever level the reporter attached the tag at)     │
└──────────────────────────────────┬─────────────────────────────────┘
                                   │  (no new tables, no new ingest code)
                                   ▼
┌────────────────────────────────────────────────────────────────────┐
│  fern-platform: NEW resolvers for coverage views                   │
│                                                                    │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │  #29 — requirementCoverage(projectId, epicKey)               │  │
│  │  (owner: cwrm56 — feature/29-jira-coverage-hierarchy)        │  │
│  │    1. JIRA JQL: parent = <epicKey>                           │  │
│  │    2. SQL: UNION over the 3 junctions (per Coverage Join     │  │
│  │              section below) WHERE category='jira'            │  │
│  │              AND LOWER(value) IN (LOWER(child_keys))         │  │
│  │    3. Merge in-memory → RequirementCoverageTree              │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                    │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │  #30 — releaseCoverage(projectId, dimensionId, release)      │  │
│  │  (owner: bsekar — this spec)                                 │  │
│  │    0. Resolve release dimension by id (built-in fixVersion   │  │
│  │       or a configured custom field) → selector + enumerator  │  │
│  │    1. projectReleases(dimensionId) for the picker payload    │  │
│  │    2. JIRA JQL: project=X AND issuetype=Epic                 │  │
│  │                AND <dimension.selector(release)>             │  │
│  │       e.g. fixVersion="R" | cf[10042]="R" | labels="R"       │  │
│  │    3. JIRA JQL: parent IN (epicKeys)  (paginated)            │  │
│  │    4. SQL: same UNION as #29 over the union of child keys    │  │
│  │    5. Aggregate per Epic + release-level rollup              │  │
│  │       → ReleaseCoverage with EpicCoverageSummary[]           │  │
│  │    6. Epic cards link to /…/epics/:key (drills into #29)     │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                    │
└──────────────────────────────────┬─────────────────────────────────┘
                                   │ GraphQL
                                   ▼
┌────────────────────────────────────────────────────────────────────┐
│  UI                                                                │
│    /projects/:id/coverage                  → Release picker + #30  │
│    /projects/:id/epics/:epicKey            → #29 drill view        │
└────────────────────────────────────────────────────────────────────┘
```

### Ownership

| Issue | Resolver | UI route | Owner | Branch |
|---|---|---|---|---|
| #29 | `requirementCoverage(projectId, epicKey)` | `/projects/:id/epics/:epicKey` | cwrm56 | `feature/29-jira-coverage-hierarchy` (in flight) |
| #30 | `releaseCoverage(projectId, dimensionId, release)` + `projectReleases(projectId, dimensionId)` + `projectReleaseDimensions(projectId)` | `/projects/:id/coverage` (dimension + release picker) | bsekar | `feature/30-release-coverage-dashboard` |

cwrm56's branch already contains TDD foundations: mock JIRA server, `JiraClient.SearchIssues` / `.GetVersions`, GraphQL schema additions, repository scaffolding. #30 reuses these directly — no parallel implementation.

**Schema generalization (coordinate with cwrm56):** #30 generalizes the release vocabulary from `fixVersion` to a configurable, source-agnostic **release dimension** (Req 8). `jiraFixVersions(projectId)` becomes a special case of `projectReleases(projectId, "fixVersion")`. The built-in `fixVersion` dimension preserves cwrm56's existing behavior; custom dimensions are additive. See the coordination note in `reviews/jira-sync-persistence-analysis/`.

---

## Data layer (no new tables)

The MVP uses existing schema in production:

| Table | Source migration | What it stores |
|---|---|---|
| `tags` | `000004_create_tags_table` + `000017_add_category_value_to_tags` | Unique tag names with auto-split `(category, value)` |
| `test_run_tags` | `000004` | Junction: `(test_run_id, tag_id)` — tags on a whole test run |
| `suite_run_tags` | `000018_create_suite_run_tags_table` | Junction: `(suite_run_id, tag_id)` — tags on a suite within a run |
| `spec_run_tags` | `000019_create_spec_run_tags_table` | Junction: `(spec_run_id, tag_id)` — tags on individual specs |
| `test_runs`, `suite_runs`, `spec_runs` | (existing) | Run records at each granularity |

Three junction tables exist because tests can be tagged at three different granularities depending on framework convention:

- **`@Tag` on a JUnit class** → typically lands at test_run level
- **`Label("jira:...")` on a Ginkgo `Describe` block** → typically lands at suite_run level
- **`Label("jira:...")` on a Ginkgo `It` block** → typically lands at spec_run level

A test reporter is free to attach at any of the three levels. For coverage, a JIRA issue is "covered" iff **any** of the three junctions carries a matching tag. **Coverage queries MUST union all three.**

Relevant indexes already in place:

- `idx_tags_name` (UNIQUE) — natural dedup
- `idx_tags_category` — filter by `category='jira'`
- `idx_tags_category_value` — composite for the coverage join
- `idx_test_run_tags_test_run_id`, `idx_test_run_tags_tag_id`
- `idx_suite_run_tags_suite_run_id`, `idx_suite_run_tags_tag_id`
- `idx_spec_run_tags_spec_run_id`, `idx_spec_run_tags_tag_id`

### The Coverage Join (UNION across granularities)

The core query both #29 and #30 use — abstracted as a single repository method:

```go
// GetJiraTagCoverageByProject returns pass/fail/total counts per JIRA issue key
// for all test runs in the given project, joining across all three tag
// granularities. Case-folded on the value column.
GetJiraTagCoverageByProject(
    ctx context.Context,
    projectID string,
    jiraKeys []string,   // optional filter; nil = all jira: tags in project
) (map[string]CoverageCount, error)

type CoverageCount struct {
    JiraKey  string  // lowercased
    Total    int
    Passed   int
    Failed   int
}
```

SQL (PostgreSQL):

```sql
WITH covered_runs AS (
  -- Test-run-level tags
  SELECT t.value AS jira_key, tr.id AS test_run_id, tr.result
  FROM   tags t
  JOIN   test_run_tags trt ON t.id  = trt.tag_id
  JOIN   test_runs     tr  ON tr.id = trt.test_run_id
  WHERE  t.category = 'jira'
    AND  t.deleted_at IS NULL
    AND  tr.project_id = $1
    AND  (cardinality($2::text[]) = 0 OR t.value = ANY($2))

  UNION

  -- Suite-run-level tags (rolled up to parent test_run)
  SELECT t.value AS jira_key, sr.test_run_id, tr.result
  FROM   tags t
  JOIN   suite_run_tags strt ON t.id  = strt.tag_id
  JOIN   suite_runs     sr   ON sr.id = strt.suite_run_id
  JOIN   test_runs      tr   ON tr.id = sr.test_run_id
  WHERE  t.category = 'jira'
    AND  t.deleted_at IS NULL
    AND  tr.project_id = $1
    AND  (cardinality($2::text[]) = 0 OR t.value = ANY($2))

  UNION

  -- Spec-run-level tags (rolled up to parent test_run)
  SELECT t.value AS jira_key, sp.test_run_id, tr.result
  FROM   tags t
  JOIN   spec_run_tags sprt ON t.id  = sprt.tag_id
  JOIN   spec_runs     sp   ON sp.id = sprt.spec_run_id
  JOIN   test_runs     tr   ON tr.id = sp.test_run_id
  WHERE  t.category = 'jira'
    AND  t.deleted_at IS NULL
    AND  tr.project_id = $1
    AND  (cardinality($2::text[]) = 0 OR t.value = ANY($2))
)
SELECT jira_key,
       COUNT(DISTINCT test_run_id)                                  AS total,
       COUNT(DISTINCT test_run_id) FILTER (WHERE result = 'passed') AS passed,
       COUNT(DISTINCT test_run_id) FILTER (WHERE result = 'failed') AS failed
FROM   covered_runs
GROUP BY jira_key;
```

Notes:

- The `UNION` (not `UNION ALL`) deduplicates rows where the same `(jira_key, test_run_id)` is produced by multiple granularities — e.g., if a test_run has both a test_run-level `jira:X` tag AND a child spec_run with the same tag, count the test_run once.
- The `cardinality($2::text[]) = 0 OR t.value = ANY($2)` predicate allows passing `nil` for "all keys in project" (used by #30's release-wide coverage map) OR a filtered set (used by #29's per-Epic lookup).
- Repository pre-lowercases the `jiraKeys` slice before binding so the equality match works against `t.value` (which is already lowercased).
- The exact SQL above assumes `spec_runs.test_run_id` and `suite_runs.test_run_id` FK paths exist — implementation should verify against the live schema and adjust join paths if intermediate tables exist (e.g., `spec_runs → suite_runs → test_runs`).

### Ingest-time behavior (already correct in production)

### Ingest-time behavior (already correct in production)

`internal/domains/tags/domain/tag.go::NewTag(name)`:

```go
normalizedName := strings.TrimSpace(strings.ToLower(name))
if idx := strings.Index(normalizedName, ":"); idx == -1 {
    value = normalizedName
} else {
    category = strings.TrimSpace(normalizedName[:idx])
    value = strings.TrimSpace(normalizedName[idx+1:])
}
```

A test reporter sending `"jira:GWCP-12345"` results in a row:

```
tags(name="jira:gwcp-12345", category="jira", value="gwcp-12345")
```

This applies to every test that reports tags through the normal `POST /api/v1/test-runs` flow. **No ingest code changes are required for this MVP.**

### Case-folding caveat

`value` is stored lowercased. JIRA returns canonical uppercase keys. Resolvers must case-fold during the join:

```sql
WHERE t.category = 'jira'
  AND LOWER(t.value) IN (LOWER('GWCP-12345'), LOWER('GWCP-12346'), ...)
  AND t.deleted_at IS NULL
```

Or normalize JIRA's keys to lowercase on the application side before binding the `IN` clause. Either approach is acceptable.

---

## GraphQL surface

**Naming convention:** adopts the types defined on `feature/29-jira-coverage-hierarchy` (cwrm56) for consistency. #30 adds release-level wrappers that compose with #29's types.

### Types

Shared (defined by #29 — cwrm56's branch):

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
  testRunCoverage: TestRunCoverage      # null if not covered
}

type EpicCoverageNode {
  issue:        JiraIssueSummary!
  stories:      [StoryCoverageNode!]!
  coveredCount: Int!
  totalCount:   Int!
}

type RequirementCoverageTree {
  fixVersion: JiraVersion        # null when queried per-Epic (no release context)
  epics:      [EpicCoverageNode!]!
  unassigned: [StoryCoverageNode!]!
}
```

Added by #30 (this spec). Vocabulary is generalized from `fixVersion` to a
source-agnostic **release dimension** (Req 8):

```graphql
# A release value, source-agnostic. FIX_VERSION carries id/released/date;
# CUSTOM_FIELD and LABEL leave those empty (name is the JQL selector value).
type ReleaseRef {
  id:          String!
  name:        String!
  released:    Boolean!
  releaseDate: String
}

# An available grouping dimension for a project: built-in fixVersion, or a
# custom field configured during JIRA field-mapping (Req 8).
type ReleaseDimension {
  id:         String!    # "fixVersion" | JIRA field id e.g. "customfield_10042"
  label:      String!    # "Fix Version" | admin label e.g. "Release"
  kind:       String!    # FIX_VERSION | CUSTOM_FIELD | LABEL
  enumerable: Boolean!   # projectReleases can list values
  isDefault:  Boolean!
}

type EpicCoverageSummary {
  issue:        JiraIssueSummary!   # the Epic itself
  coveredCount: Int!                 # work items (epic + children) with ≥1 matching test run
  totalCount:   Int!                 # 1 (the epic) + its direct children
  passingCount: Int!                 # work items where every matching run passed
}

type ReleaseCoverage {
  dimension:         ReleaseDimension!   # which dimension this rollup used
  release:           ReleaseRef!         # was fixVersion: JiraVersion!
  totalEpics:        Int!
  coveredEpics:      Int!            # epic groups with ≥1 covered work item
  fullyCoveredEpics: Int!            # epic groups where every work item is covered
  totalChildren:     Int!
  coveredChildren:   Int!
  epics:             [EpicCoverageSummary!]!   # lightweight; UI drills via #29
}
```

### Queries

```graphql
extend type Query {
  # #29 — cwrm56's branch
  jiraFixVersions(projectId: ID!): [JiraVersion!]!   # == projectReleases(id, "fixVersion"); retained for back-compat
  requirementCoverage(projectId: ID!, epicKey: String!): RequirementCoverageTree!

  # #30 — this spec
  projectReleaseDimensions(projectId: ID!): [ReleaseDimension!]!
  projectReleases(projectId: ID!, dimensionId: String!): [ReleaseRef!]!
  releaseCoverage(projectId: ID!, dimensionId: String!, release: String!): ReleaseCoverage!
}
```

`projectReleases(projectId, "fixVersion")` subsumes cwrm56's `jiraFixVersions`; the latter stays as a thin alias for back-compat until #29 adopts the dimension-aware query.

### Errors

| Code | Condition |
|---|---|
| `JIRA_UNAVAILABLE` | JIRA HTTP timeout (>5s) or 5xx response |
| `JIRA_AUTH_FAILED` | 401/403 from JIRA — credentials in `jira_connections` need refresh |
| `JIRA_NOT_FOUND` | Epic key or release name doesn't exist in JIRA |
| `FORBIDDEN` | Caller lacks project-scoped permissions |
| `JIRA_RATE_LIMITED` | 429 from JIRA — surface retry-after hint to UI |

---

## Resolver flow detail

### `requirementCoverage(projectId, epicKey)` (#29, owner: cwrm56)

```
1. Authorize: authorizeProjectManagement(ctx, projectId)
2. Load jiraConnection for projectId (existing #25 path)
3. Build JQL: `project = <projectKey> AND parent = <epicKey>`
4. Execute via JiraClient.SearchIssues (paginated if >100 children),
   fields: key, summary, status, issuetype
5. Collect child keys (lowercased)
6. Call repo.GetJiraTagCoverageByProject(projectID, lowercasedChildKeys)
   → map[lowercased-jira-key]CoverageCount
7. Merge per-child JIRA metadata with the coverage map
8. Compute Epic-level rollups (coveredCount, totalCount)
9. Return RequirementCoverageTree {
     fixVersion: nil,
     epics: [the single EpicCoverageNode],
     unassigned: [] (per-Epic query has no unassigned bucket)
   }
```

### `releaseCoverage(projectId, dimensionId, release)` (#30, owner: bsekar)

```
 0. Resolve the release dimension by id:
      - "fixVersion" → built-in FIX_VERSION dimension
      - else → look up the configured custom dimension (Req 8); 404 if unknown
    The dimension yields a selector(value) → JQL predicate.
 1. Authorize: authorizeProjectManagement(ctx, projectId)
 2. Load jiraConnection
 3. Build JQL #1: `project = <projectKey> AND issuetype = Epic
                  AND <dimension.selector(release)>`
      FIX_VERSION → fixVersion = "<release>"
      CUSTOM_FIELD → cf[<id>] = "<release>"
      LABEL → labels = "<release>"
    fields: key, summary, status, issuetype
 4. Execute (paginated) → list of Epics in the release
    If empty → return ReleaseCoverage with totalEpics=0 and empty epics[]
 5. Build JQL #2: `parent IN (<epicKey1>, ..., <epicKeyN>)`
    Paginated (100 per page). Fields: key, summary, status, issuetype, parent
 6. Execute → child issues mapped to their parent Epic
 7. Build the full child-key set (lowercased, deduped)
 8. Call repo.GetJiraTagCoverageByProject(projectID)
    → ONE map covering every jira:<KEY> in the project. This is critical:
      do NOT call the repo per-Epic — one query per release.
 9. Aggregate per Epic (work item = epic + its children):
      - count covered / total / passing across the epic and its children
      - Build EpicCoverageSummary
10. Aggregate at release level:
      - totalEpics = len(epics)
      - coveredEpics = count(epic group with any covered work item)
      - fullyCoveredEpics = count(epic group where every work item covered)
      - totalChildren = sum of children across epics
      - coveredChildren = covered children across epics
11. Return ReleaseCoverage { dimension, release, ... }
```

The single SQL call in step 8 — covering the whole project — keeps #30 within budget even at PALISADES scale (827 issues). Per-Epic SQL queries would be O(n) round-trips and break the latency budget.

### `projectReleaseDimensions(projectId)` and `projectReleases(projectId, dimensionId)` (#30 picker)

`projectReleaseDimensions` returns the built-in `fixVersion` dimension plus any
custom dimensions configured for the project (Req 8). `projectReleases` enumerates
values for a chosen dimension:

- `fixVersion` → `JiraClient.GetVersions` (the path cwrm56's `jiraFixVersions` already uses; that resolver becomes an alias).
- `CUSTOM_FIELD` of select-list type → the field's allowed values.
- non-enumerable → empty list; the UI accepts a manually-typed release value.

### Drill-down

The Release dashboard's Epic cards link to `/projects/:id/epics/:epicKey` — the same UI route #29 owns. No cross-resolver call; the link is a UI navigation.

---

## UI sketch

### Per-Epic dashboard (`/projects/:projectId/epics/:epicKey`)

```
┌─────────────────────────────────────────────────────────────┐
│ GWCP-86523  •  Atmos Networking refresh                      │
│ Status: In Progress  •  178 children  •  124 covered (70%)   │
│ 96 passing  •  28 failing  •  54 uncovered                   │
│                                                              │
│ [ filter: ◯ uncovered  ◯ failing  ● all  ]                  │
├─────────────────────────────────────────────────────────────┤
│ Key          Title                          Status   Cov     │
│ GWCP-86524   …                              Done     🟢      │
│ GWCP-86525   …                              In Pr.   🟡      │
│ GWCP-86526   …                              Open     ⚪       │
│ …                                                           │
└─────────────────────────────────────────────────────────────┘
```

### Release dashboard (`/projects/:projectId/releases/:releaseName`)

```
┌─────────────────────────────────────────────────────────────┐
│ PALISADES (2025.10M)                                         │
│ 38 epics  •  789 children  •  31 covered epics (82%)        │
│ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓░░░░░  70% children covered              │
├─────────────────────────────────────────────────────────────┤
│ GWCP-86523  Atmos Networking…    178 children  ▓▓▓▓░  70%  │
│ GWCP-85325  Atmos Databases…      92 children  ▓▓▓▓▓  95%  │
│ GWCP-93474  Atmos Databases…      86 children  ▓▓░░░  40%  │
│ … (rest of 38 epics)                                        │
└─────────────────────────────────────────────────────────────┘
```

Each Epic row links to its per-Epic dashboard.

---

## Design decisions

### Decision 1: No new tables

The proposed `spec_run_requirements` table is redundant. The existing `tags` + `spec_run_tags` schema, plus `domain.NewTag()`'s auto-split logic, already store exactly what we need. The performance characteristics (composite index, idempotent join, cascade-delete) are equivalent.

**Trade-off accepted:** one extra join in coverage queries (`spec_run_tags → tags`). Indexes make this irrelevant at the scales measured (PALISADES = 827 issues = <100 ms).

### Decision 2: Live JIRA queries instead of background sync

Documented in `reviews/jira-sync-persistence-analysis/SUMMARY.md` (Option A). Reasons (summarized):

- No meaningful storage savings from caching JIRA metadata (~1 GB across a 100-project fleet)
- 2-3 second dashboard render acceptable for sprint-review / release-readiness use cases
- Cache → eventually-becomes-Option-A; better to choose explicitly
- No staleness machinery, no sync orchestration, no scheduler — collapsed implementation surface

### Decision 3: Case-fold in the SQL `IN` clause, not via tag re-normalization

Changing `domain.NewTag()` to preserve case for `jira:` tags would touch shared infrastructure used by other tag categories. Cheaper and safer to case-fold at query time, where only the coverage resolvers care.

### Decision 4: Configurable release dimensions — `fixVersion` built-in, custom fields opt-in (supersedes the earlier "fixVersion only" decision)

**Superseded:** an earlier draft fixed the release grouping to the standard `fixVersion` field. That doesn't fit OSS reality — many teams group releases by a custom field (a roadmap "Release" field), and a PM may want to switch between `fixVersion` and that field at view time.

**Now:** the top hierarchy layer is a **release dimension** (Req 8). `fixVersion` is a built-in, zero-config dimension; additional dimensions (custom field / label) are configured per project during JIRA field-mapping configuration (#26 infrastructure — now load-bearing again, not parked). The dimension only changes the Epic-selector JQL predicate and how values enumerate; the leaf→epic→release hierarchy and the coverage join are unchanged. This keeps OSS portability (works with stock JIRA out of the box) while serving teams whose release concept lives in a custom field.

The `#26` field-mapping infrastructure is reused for storing the custom-dimension config; it is no longer "parked" for this purpose. Proprietary external-tool field *names* are never embedded — config references JIRA field ids only.

### Decision 5: Errors over partial render under JIRA outage

If JIRA is unreachable, returning a clear `JIRA_UNAVAILABLE` error is better than rendering stale or partial data. Future iteration (Design C — sync-light) could change this, but the MVP keeps the truth boundary clean.

### Decision 6: Authorization mirrors `CreateJiraConnection` pattern

The same `authorizeProjectManagement(ctx, projectID)` helper that `CreateJiraConnection` uses (from PR #188 / merged code path) — load project, call `GetUserPermissions`, forbid if empty. This matches the existing security posture.

---

## Performance budget

| Operation | P50 | P95 |
|---|---|---|
| `epicCoverage` for typical Epic (20-50 children) | <500 ms | <1.5 s |
| `epicCoverage` for outlier Epic (~200 children) | <1 s | <3 s |
| `releaseCoverage` for a typical release (10-20 epics, ~200 children total) | <2 s | <5 s |
| `releaseCoverage` for PALISADES-scale (38 epics, 789 children) | <3 s | <8 s |

Hot paths:

- JIRA pagination dominates wall-time
- SQL join completes in <50 ms for any realistic key set
- Network latency to JIRA: ~200-500 ms per call

---

## Failure modes

| Failure | Behavior |
|---|---|
| JIRA HTTP 5xx | Retry up to N times via existing JIRA client; surface `JIRA_UNAVAILABLE` if persistent |
| JIRA HTTP 429 (rate-limited) | Existing JIRA client handles backoff; surface retry-after to UI if necessary |
| JIRA HTTP 401/403 | `JIRA_AUTH_FAILED` — UI prompts user to refresh connection credentials |
| JQL syntax error | Should not occur in normal flow (queries are server-controlled); 500 on resolver if it does |
| Empty release / Epic | Return empty `children: []` with `totalChildren: 0` — UI shows "no issues" state |
| Project not authorized | `FORBIDDEN` GraphQL error |

---

## Testing approach

### Unit tests

- Resolver logic with mocked JIRA client + in-memory DB
- Case-folding behavior (mixed-case JIRA keys correctly match lowercased stored values)
- Empty / null Epic and release responses
- Rollup math correctness (any_passed, total counts, etc.)

### Acceptance tests (Ginkgo)

- End-to-end: post a `spec_run` with `jira:GWCP-X` tags via the test ingestion API, query `epicCoverage` for an Epic that contains `GWCP-X`, expect the test_run to appear as covering that child
- Idempotency: re-posting the same spec_run does not change coverage counts
- Authorization: a user without project permissions gets `FORBIDDEN`

### Smoke test

One end-to-end test against a real GW JIRA project (gated by env var so it doesn't run in CI). Verifies pagination, real auth, real JQL execution.

---

## Dependencies

- ✅ #25 — JIRA connection (per-project URL + credentials)
- ✅ #26 — JIRA field mapping (parked infrastructure for MVP; may be reactivated for pod rollups)
- 🔁 fern-clients teams — emit `jira:<KEY>` tags from per-language reporters (outside this epic)

---

## Migration

**None.** All required schema is already in place (migrations 000004, 000017, 000019). The MVP introduces no new tables, no new columns, no new indexes.

If we later add the optional "release timestamp snapshot" feature (post-MVP), that's its own migration.

---

## References

- Active requirements: `./requirements.md`
- Decision rationale: `reviews/jira-sync-persistence-analysis/SUMMARY.md`
- ADRs: `adr/test-correlation/tag-schema.md`, `adr/test-correlation/mapping-lifecycle.md`
- Parked design: `./future/`
- Tracking issues: #22 (epic), #28 (foundation/docs), #29 (Per-Epic), #30 (Per-Release)
