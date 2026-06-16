# JIRA-Level Test Coverage — Requirements (MVP)

**Status:** Active — scoped down from the original scenario-level design (parked in `./future/`).
**Decision rationale:** `reviews/jira-sync-persistence-analysis/SUMMARY.md` (dev workspace, gitignored).
**Tracking:** GitHub issue #22 (epic) → #28 (foundation) → #29 (Per-Epic view) → #30 (Per-Release view).

---

## Goal

Render test coverage for JIRA-tracked work at the **issue level**: a JIRA issue (Epic, Story, Task, Bug) is "covered" iff at least one ingested test run (at any granularity — test_run, suite_run, or spec_run) carries a tag of the form `jira:<KEY>` matching that issue.

Two user-facing views, split by scope and owner:

| View | Owner | Resolver | Scope |
|---|---|---|---|
| **#29 — Per-Epic** | cwrm56 (in flight on `feature/29-jira-coverage-hierarchy`) | `requirementCoverage(projectId, epicKey)` | Given an Epic key, render its direct children (stories/tasks/bugs) with per-child coverage |
| **#30 — Per-Release** | bsekar (this spec author) | `releaseCoverage(projectId, release)` | Given a **release** (see "Release dimension" below), render all Epics in the release with per-Epic rollups; click on an Epic drills into #29 |

Each view is independently shippable. #30 depends on #29 only at the drill-down hop — the release view itself renders without needing #29 to be live (Epic cards show counts, link to a placeholder if #29 isn't ready).

### The fixed 3-layer hierarchy

Coverage rolls up through exactly three layers. Depth is fixed; only the **top** layer's source is configurable.

| Layer | JIRA object | How tests touch it |
|---|---|---|
| **L1 — leaf** | Story / Bug / Task | tagged `jira:<KEY>` by tests |
| **L2 — mid** | Epic (the "master feature" / initiative) | `parent` of L1 |
| **L3 — top** | **Release** | a configurable grouping of L2 (see below) |

### The "release dimension" is configurable, and there can be more than one

There is no single JIRA field that all teams use for "release." Some use the native `fixVersion`; others use a custom field (e.g. a select-list synced from an external roadmap tool), a label, etc. Fern-platform is OSS, so the top layer MUST NOT hardcode `fixVersion` — but it also should not *force a single choice*: the same PM may want to view a release by `fixVersion` one moment and by a custom "Release" field the next.

So the model is **multiple release dimensions per project, chosen at view time**:

- **`fixVersion` is always available** — a built-in dimension, zero config, matching native JIRA.
- **Zero or more custom dimensions** are added during **JIRA field-mapping configuration** (#26 infrastructure) — each a named field (display label + JIRA field id + kind).
- **At the dashboard, the PM picks which dimension to group by**, then picks a release value within it.

The unit of correlation stays JIRA-issue-level; only **how an Epic is bound to a release** varies per dimension. Two operations depend on the chosen dimension:

- **Select** ("which Epics are in release R, by this dimension?") — a JQL predicate; **field-agnostic** (`fixVersion = R`, `cf[10042] = R`, `labels = R` are all equally simple).
- **Enumerate** ("what release values exist for this dimension?") — fills the picker; **field-type-specific** (native `fixVersion` and custom select-lists are enumerable; free-text fields are not, and degrade to manual entry).

---

## Stakeholders

- **PMs / Release Managers:** consume the dashboards to assess release readiness
- **VP-level reviewers:** consume the same dashboards for cross-team coverage rollups
- **Engineers / QA:** tag tests with `jira:<KEY>` from the test code (mechanics live in fern-clients repos — outside this spec)

---

## Scope

**In scope (this spec):**

- The GraphQL resolvers, UI pages, and documentation for the two views
- Documentation of the `jira:<KEY>` wire contract
- Coordination checklist for fern-clients reporter teams

**Out of scope:**

- Scenario-level coverage (`scenario:<TITLE>` tagging, Gherkin parsing) — see `./future/`
- Background sync of JIRA metadata into Fern — not needed (no requirements mirror)
- Per-language reporter implementation (lives in respective fern-clients repos)
- Webhook back to JIRA — tracked separately in #31

---

## Requirements (EARS notation)

### Req 1 — JIRA-level coverage join (existing infrastructure)

**Background:** Fern already persists test tags via the `tags` master table plus three junction tables of increasing granularity: `test_run_tags` (000004), `suite_run_tags` (000018), `spec_run_tags` (000019). The `tags.category` / `tags.value` columns were added in 000017. The existing `domain.NewTag()` auto-splits colon-namespaced tags into `(category, value)` and lowercases both.

1. WHEN a test artifact (test_run / suite_run / spec_run) is ingested with a tag of the form `jira:<JIRA-KEY>` THE SYSTEM SHALL persist it via the existing tag flow such that the row in `tags` has `category = "jira"` and `value = "<jira-key-lowercased>"`.
2. THE SYSTEM SHALL NOT introduce a new persistence table for this contract. The existing `tags` + junctions schema is sufficient.
3. THE SYSTEM SHALL consider a JIRA issue "covered" if any of the three junctions (`test_run_tags`, `suite_run_tags`, `spec_run_tags`) carries a tag with `category='jira'` and matching `value`. Coverage queries SHALL UNION across all three junctions to compute this.
4. THE SYSTEM SHALL treat repeated ingestion of the same `(run_id, tag_id)` pair as idempotent (already enforced by each junction's composite PRIMARY KEY).

### Req 2 — Per-Epic test coverage view (#29 — owner: cwrm56)

**User story:** As a manager, I want to see test coverage for every direct child of a JIRA Epic so I can identify uncovered work within the Epic.

1. WHEN a user requests `requirementCoverage(projectId, epicKey)` THE SYSTEM SHALL:
   1. Resolve the project's JIRA connection via the existing service
   2. Issue ONE JIRA JQL query: `project = <projectKey> AND parent = <epicKey>` (paginated if children > 100), fields `key,summary,status,issuetype`
   3. Query local coverage via the UNION across junctions (per Req 1.3) joined to `tags` where `category='jira' AND LOWER(value) IN (<lowercased-child-keys>)`
   4. Return per-child coverage status (covered + run_count + pass/fail) and Epic-level rollup
2. THE SYSTEM SHALL render the result via the GraphQL `RequirementCoverageTree` shape (per `feature/29-jira-coverage-hierarchy`):
   - `EpicCoverageNode { issue, stories, coveredCount, totalCount }`
   - `StoryCoverageNode { issue, covered, testRunCoverage }`
   - `JiraIssueSummary { key, summary, statusName, issueType }`
   - `TestRunCoverage { total, passed, failed }`
3. WHEN JIRA is unreachable (timeout > 5 s) THE SYSTEM SHALL return a `JIRA_UNAVAILABLE` GraphQL error rather than blocking on retry.

**Note:** This view is being implemented on branch `feature/29-jira-coverage-hierarchy` by cwrm56. This spec captures the contract; per-task ownership lives in that branch's spec dir and migrates here on merge.

### Req 3 — Per-Release test coverage dashboard (#30 — owner: this spec author)

**User story:** As a manager, I want to see test coverage for all Epics in a release — grouped by whichever release dimension I choose (Req 8) — so I can assess release readiness, and drill into any Epic for detail.

1. WHEN a user opens the Release Coverage view THE SYSTEM SHALL list the project's available release dimensions via `projectReleaseDimensions(projectId)` (always includes built-in `fixVersion`; plus any configured per Req 8) and let the user pick one. The default selection is `fixVersion` unless the project sets a different default.
2. WHEN a user picks a dimension THE SYSTEM SHALL enumerate its values via `projectReleases(projectId, dimensionId)` and render a searchable picker. WHEN the dimension is not enumerable (free-text custom field) THE SYSTEM SHALL accept a manually-typed release value instead.
3. WHEN a user selects (or types) a release value THE SYSTEM SHALL request `releaseCoverage(projectId, dimensionId, release)` which SHALL:
   1. Issue ONE JIRA JQL query selecting Epics in the release using the chosen dimension's **selector predicate** (Req 8.3): `project = <projectKey> AND issuetype = Epic AND <release-selector>` (paginated if epics > 100)
   2. Issue ONE JIRA JQL query for those Epics' direct children: `parent in (<epic-keys>)` (paginated / chunked)
   3. Compute coverage by joining the JIRA issue tree against the local UNION-of-junctions coverage (Req 1.3, Req 4)
   4. Return release-level totals (epic count, covered-epic count, covered-children count) + per-Epic rollup cards
4. THE SYSTEM SHALL render Epic cards as drill links to the Per-Epic view (#29) for the same Epic.
5. WHEN a project has no active JIRA connection THE SYSTEM SHALL surface a clear message prompting the user to configure one.
6. WHEN JIRA is unreachable THE SYSTEM SHALL return a `JIRA_UNAVAILABLE` error and the UI SHALL surface it; no partial / cached render.

**Implementation note:** The release-level shape is a thin wrapper aggregating per-Epic counts. Vocabulary is generalized from `fixVersion` to `release`, and both `releaseCoverage` and `projectReleases` take a `dimensionId`:

```graphql
type ReleaseCoverage {
  dimension: ReleaseDimension!   # which dimension this rollup was grouped by
  release: ReleaseRef!           # was fixVersion: JiraVersion!
  totalEpics: Int!
  coveredEpics: Int!
  fullyCoveredEpics: Int!
  totalChildren: Int!
  coveredChildren: Int!
  epics: [EpicCoverageSummary!]!  # lightweight — drill calls requirementCoverage
}

# A release value, source-agnostic. For FIX_VERSION it carries the version id +
# released flag; for CUSTOM_FIELD/LABEL those may be empty.
type ReleaseRef {
  id: String!          # version id for FIX_VERSION; the value itself otherwise
  name: String!        # display + JQL selector value
  released: Boolean!   # FIX_VERSION only; false otherwise
  releaseDate: String  # FIX_VERSION only; null otherwise
}

type EpicCoverageSummary {
  issue: JiraIssueSummary!
  coveredCount: Int!   # work items (epic + children) with >=1 matching test run
  totalCount: Int!     # 1 (the epic) + its direct children
  passingCount: Int!   # work items where every matching run passed
}
```

### Req 8 — Configurable release dimensions (#30 — owner: this spec author)

**User story:** As a project admin, I want `fixVersion` available out of the box AND the ability to add a custom field (e.g. a roadmap "Release" field) as an alternative grouping, so PMs can switch between them at view time.

1. THE SYSTEM SHALL always expose a built-in `fixVersion` dimension (id `"fixVersion"`, kind `FIX_VERSION`), with no configuration required.
2. THE SYSTEM SHALL let a project admin configure **zero or more additional** release dimensions during JIRA field-mapping configuration (#26 infrastructure). Each carries: a display `label`, `kind ∈ {CUSTOM_FIELD, LABEL}`, and the JIRA field id (e.g. `customfield_10042`) for `CUSTOM_FIELD`. A project MAY mark one dimension as its default. Stored alongside the existing per-project JIRA config; no new subsystem.
3. THE SYSTEM SHALL derive each dimension's Epic **selector predicate** from its kind:
   - `FIX_VERSION` → `fixVersion = "<value>"`
   - `CUSTOM_FIELD` → `cf[<id>] = "<value>"`
   - `LABEL` → `labels = "<value>"`
4. THE SYSTEM SHALL enumerate a dimension's values per `projectReleases(projectId, dimensionId)`:
   - `FIX_VERSION` → `GET /rest/api/3/project/{projectKey}/versions` (unreleased first alphabetical; released below newest-first)
   - `CUSTOM_FIELD` of select-list type → the field's allowed values
   - otherwise (free-text custom field, label) → return empty; the UI accepts a manually-typed value
5. THE SYSTEM SHALL expose available dimensions via `projectReleaseDimensions(projectId): [ReleaseDimension!]!` where:
   ```graphql
   type ReleaseDimension {
     id: String!          # "fixVersion" for built-in; JIRA field id for custom
     label: String!       # "Fix Version" | admin-chosen label e.g. "Release"
     kind: String!        # FIX_VERSION | CUSTOM_FIELD | LABEL
     enumerable: Boolean! # true if projectReleases can list values
     isDefault: Boolean!
   }
   ```
6. THE SYSTEM SHALL NOT leak any proprietary external-tool field names in OSS artifacts; config references JIRA field ids/keys only (admin-chosen labels are the project's own responsibility).

**Priority:** the built-in `FIX_VERSION` is the zero-config default; the configurable `CUSTOM_FIELD` path is the primary verification target for this MVP (the common case for the maintaining teams).

### Req 4 — Resolver case-folding

`domain.NewTag()` lowercases names during normalization. JIRA returns canonical uppercase keys.

1. THE SYSTEM SHALL case-fold during the SQL join, e.g., `WHERE category = 'jira' AND LOWER(value) IN (LOWER('GWCP-12345'), ...)`, OR normalize JIRA-returned keys to lowercase on the application side before binding the IN clause.
2. THE SYSTEM SHALL apply this rule consistently across the UNION over all three junction tables (test_run / suite_run / spec_run).

### Req 5 — Live JIRA dependency (no Fern-side mirror)

1. THE SYSTEM SHALL fetch issue metadata (title, status, type, parent linkage) from JIRA at dashboard render time. No local mirror of JIRA issue metadata is maintained.
2. THE SYSTEM SHALL surface JIRA unavailability as a clear error in the dashboards; partial/cached renders are out of scope.
3. THE SYSTEM SHALL respect JIRA API rate limits via the existing JIRA client retry/backoff (no new rate-limit machinery).

### Req 6 — Authorization

1. THE SYSTEM SHALL require project-scoped permissions to view coverage dashboards (mirrors the pattern in `CreateJiraConnection`: load project → `GetUserPermissions` → forbid if empty).
2. THE SYSTEM SHALL NOT expose JIRA issue metadata for projects the caller is not authorized to view.

### Req 7 — Wire contract documentation

1. THE SYSTEM SHALL publish a developer-facing wire contract describing:
   - The tag format: `jira:<JIRA-KEY>` as a string element of the `tags` array in the spec_run JSON payload
   - Case-folding behavior: keys are lowercased at ingest
   - Per-language adoption guidance for fern-clients reporter teams (Go-ginkgo, JS-jest, Python-pytest, JUnit, Cucumber)
2. THE SYSTEM SHALL update `adr/test-correlation/tag-schema.md` to reflect this contract (drop the `scenario:` namespace; this is deferred per `./future/`).

---

## Accepted trade-offs

| Trade-off | Why accepted for MVP |
|---|---|
| Tests can be empty or meaningless and still register as "covered" by tagging | Per engineering leadership; gameable but visible. Will revisit if it becomes a real problem. |
| Dashboard latency ~2-3 s P50 driven by live JIRA queries | Dashboards are sprint-review / release-readiness use; not realtime. |
| JIRA outage → dashboards return error | JIRA is the source of truth; if JIRA is down, the data is unknowable. |
| No staleness window | Live JIRA = always fresh; no need for "synced N hours ago" UI. |
| No persistent issue metadata | If/when offline-tolerance or sub-second dashboards are needed, upgrade path is documented in SUMMARY ("Design C — sync-light"). |

---

## References

- Epic: #22 (JIRA Integration)
- Foundation: #28 (wire contract documentation; near-zero code work)
- Dashboards: #29 (Per-Epic), #30 (Per-Release)
- Connection: #25 (merged) — JIRA auth, URL, project key
- Field mapping: #26 (merged) — parked infrastructure; not load-bearing for MVP
- Decision rationale: `reviews/jira-sync-persistence-analysis/SUMMARY.md` (dev-workspace local working doc)
- Parked design: `./future/` (full scenario-level scope for the future iteration)
- ADRs: `adr/test-correlation/tag-schema.md`, `adr/test-correlation/mapping-lifecycle.md`
