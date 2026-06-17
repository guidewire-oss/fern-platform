# 29-jira-coverage-hierarchy - Requirements

## Introduction

Fern already ingests test tags in the form `jira:PROJ-123`, creating an association between tests and JIRA issue keys. This feature adds a Requirements Coverage view that lets managers see, for a selected **release**, which epics and stories have test coverage in Fern and which do not — without syncing JIRA data into Fern. The hierarchy is fetched live from JIRA using the project's stored connection credentials, and rolls up Release → Epic → Story with results-aware status. (Sub-tasks were originally a fourth level but have been removed — see Amendment: sub-tasks excluded.)

Release scope is determined by a **custom JIRA field on Epics** (e.g. "Aha Release (edit only in Aha)"). The field name and JIRA field ID are configurable per JIRA connection. The fetch strategy is **Epic-first and cascades downward**: Epics are fetched by custom field value, then Stories by parent Epic. This approach naturally handles the common case where stories do not carry the release signal themselves — only their parent Epic does. (Sub-tasks are not fetched — see Amendment: sub-tasks excluded.)

> **Archived strategy:** the original implementation used JIRA `fixVersion` as the release-scope signal (`fixVersion = "{version}" ORDER BY issuetype`). This approach is valid for open-source projects and teams that use the standard JIRA Releases/Versions feature. It is preserved as a reference implementation for a future **pluggable release-scope module** — see the *Deferred Strategies* section below.

## Requirements

### Requirement 1: Release Selection

**User Story:** As a manager, I want to select a release so that I can see coverage for a specific release.

#### Acceptance Criteria

1. WHEN a user opens the Requirements Coverage view for a project THEN the system SHALL fetch the list of available releases by querying JIRA for Epics in the project that have a non-empty value in the configured release custom field AND were updated within the past year:
   ```
   project = {projectKey} AND issuetype = Epic AND cf[{fieldId}] is not EMPTY AND updated >= -52w ORDER BY cf[{fieldId}] ASC
   ```
   The distinct non-null values of that field become the release picker options. The `updated >= -52w` window keeps the picker fast and scoped to currently active releases; releases tied only to Epics untouched for over a year are intentionally excluded (see design Decision 2).
2. THE SYSTEM SHALL display releases in a searchable picker (placeholder "Filter releases…"). The user narrows the list by typing to filter by name.
3. WHEN a user selects a release THE SYSTEM SHALL fetch the full hierarchy for that release using the three-phase cascade described in Requirement 2.
4. IF a project has no active JIRA connection THEN THE SYSTEM SHALL display an appropriate message prompting the user to configure one.
5. The configured release custom field ID (`releaseFieldId`) SHALL be stored on the project's JIRA connection record. It is required before the coverage view can be used. The JIRA project key is already a required field on the JIRA connection (`JiraConnection.ProjectKey()`) — no schema change is needed for that.

### Requirement 2: Hierarchy Fetch and Display

**User Story:** As a manager, I want to see test coverage organized by JIRA epic and story so that I can identify gaps. Coverage reports at the main-task (Story) level; sub-tasks are excluded as noise (see Amendment: sub-tasks excluded).

#### Acceptance Criteria

1. WHEN a release is selected THE SYSTEM SHALL fetch the hierarchy using a two-phase cascade:
   - **Phase 1 — Epics:** `cf[{fieldId}] = "{releaseValue}" AND issuetype = Epic ORDER BY key`
   - **Phase 2 — Stories:** `parent IN ({epicKey1},{epicKey2},...)` chunked ≤50 keys per request

   Both phases request fields: `key, summary, status, issuetype, parent`. Any issue with `issuetype.subtask = true` returned by Phase 2 is discarded; no sub-task fetch is performed.

2. THE SYSTEM SHALL display issues in a two-level hierarchy: epics at the top and stories grouped beneath their epic. Sub-tasks are not displayed.
3. WHEN an issue has no parent epic THE SYSTEM SHALL group it under an **"Issues without an Epic"** section. That section SHALL be sortable by coverage percentage (most-covered first or least-covered first).
4. WHEN displaying each story THE SYSTEM SHALL show: issue key, summary, status, and a results-aware coverage indicator — see Requirement 4 for color semantics (grey = uncovered, red = has a failing test, green = covered & passing).
5. WHEN displaying each epic THE SYSTEM SHALL show: coverage percentage (e.g. "60% (3/5 stories)") and a visual coverage bar whose fill corresponds to that percentage, an `✗ N failing` chip when any descendant story has a failing test, and a color conveying aggregate health (see Requirement 4).
6. WHEN displaying a covered story THE SYSTEM SHALL show: total number of tests linked, pass/fail/skipped breakdown, and date of last test execution.
7. Every JIRA issue key displayed in the hierarchy (epic, story) SHALL be rendered as a hyperlink to the corresponding issue in the JIRA instance (`<jira-base-url>/browse/<key>`), opening in a new tab.

### Requirement 3: Coverage Cross-Reference and Drill-Down

**User Story:** As a manager, I want to see which stories have been exercised by tests and navigate to the test evidence so that I can assess release readiness.

#### Acceptance Criteria

1. WHEN building the coverage view THE SYSTEM SHALL query Fern's tag data to find all `category="jira"` tags on test runs belonging to the project, at both test-run and spec-run granularity.
2. A story (or sub-task) SHALL be considered "covered" if at least one tagged test run or spec run in the project carries a tag matching `jira:{issueKey}`.
3. WHEN a story is covered THE SYSTEM SHALL show the count of associated runs plus a pass/fail breakdown (total, passed, failed) and the date of the most recent tagged execution.
4. WHEN a story is not covered THE SYSTEM SHALL show it clearly as uncovered (no test association found).
5. THE SYSTEM SHALL support a toggle to show only uncovered stories, hiding fully covered ones while maintaining the hierarchical structure and the path from epic to uncovered story.
6. WHEN a user clicks the test count badge on a covered story THE SYSTEM SHALL open a detail view listing the individual spec runs tagged with that issue key, including spec name, status, suite name, branch, and execution date.

### Requirement 4: Release Roll-Up and Status / Color Semantics

**User Story:** As a manager, I want one top-level release status and consistent colors so that I can read release readiness at a glance without confusing coverage with pass/fail.

#### Acceptance Criteria

1. THE SYSTEM SHALL display a release-level roll-up row at the top of the tree showing the selected release and aggregate coverage across all epics and the Issues-without-an-Epic bucket. Hierarchy is **Release → Epic → Story**.
2. Coverage breadth and test health SHALL be presented as **separate** elements so neither is misread as the other: a labelled coverage figure (`<covered>/<total> covered · <pct>%`) and a distinct health pill.
3. The release health pill SHALL be one of: **Release ready** (fully covered, no failures), **✗ N failing** (N = issues with a failing test), **In progress** (partially covered, no failures), **Not started** (0% covered).
4. Color SHALL encode health, not coverage: **grey** = uncovered / not started, **red** = has ≥1 failing test, **green** = covered with no failures, neutral = partially covered with no failures. Failures block "ready"; **skips do not** (skip counts are shown as `↺N` in text but never change color).
5. A story/epic/release that is fully covered but has a failing test SHALL NOT appear green; red SHALL NOT be used to mean "uncovered" at any level.

### Requirement 5: Non-Functional Requirements

**User Story:** As a user, I want the coverage view to load in a reasonable time so that it is usable in practice.

#### Acceptance Criteria

1. THE SYSTEM SHALL fetch all issues using batched JQL queries (one per phase) rather than one call per issue.
2. Phase 2 (Stories) SHALL chunk its `parent IN (...)` key list into batches of ≤50 keys per JIRA API request to stay within URL length limits. Results across chunks are merged before assembly.
3. IF the JIRA API is unreachable THEN THE SYSTEM SHALL display an error and not crash the page.
4. THE SYSTEM SHALL handle releases with up to 500 total issues (epics + stories) without timeout. A 30-second context timeout is applied per `Build` call.

## Constraints

- Fern does not store a local copy of JIRA issues — hierarchy is always fetched live from JIRA.
- Only projects with an active JIRA connection AND a configured `releaseFieldId` can use this feature.
- Historical coverage data is limited to test runs submitted to Fern after the JIRA integration was configured; pre-integration runs will show 0% coverage.
- Initial implementation does not support multi-select of releases (single selection only).

## Out of Scope (deferred)

- **Export to PDF/Excel** — not implemented; raise a follow-on issue if confirmed needed.
- **Lazy loading / virtual scrolling for 1000+ nodes** — current implementation loads the full tree upfront; acceptable for typical release sizes; revisit if performance becomes a problem in practice.
- **Release picker owned by #30** — the release selection dropdown and the overall release coverage % display are surfaced by the readiness dashboard (issue #30), which is built on top of this feature's service and GraphQL types.

## Amendments

### Sub-tasks excluded (2026-06-17)

The hierarchy originally had a fourth level — sub-tasks (Release → Epic → Story → Sub-task),
fetched by a Phase 3 `parent IN (story keys)` cascade and displayed as indented rows under each
story. This has been **removed**. Coverage now reports at the **main-task (Story) level only**;
the cascade is two-phase (Epics → Stories) and sub-tasks are neither fetched nor displayed.

**Why:** teams report on the main task, so sub-tasks were visual noise. Dropping the Phase 3
fetch also removes an entire pagination pass, so coverage builds faster.

**No coverage-number impact:** sub-tasks never rolled up into epic or story counts (an epic's
total counted only its stories; a story's covered state came from its own tag). Removal therefore
changes only what is fetched and displayed, not any percentage.

**Sub-task tags drop from view:** a test tagged directly on a sub-task key (`jira:PROJ-123` where
123 is a sub-task) used to appear as a sub-task node; it now simply does not appear anywhere. This
is intended (consistent with "report on the main task") and matches prior counting, which never
attributed sub-task tags to the parent story.

**Scope of the change (deliberately minimal):** the GraphQL `subTasks` field on
`StoryCoverageNode` is **retained** and now always returns an empty array — no schema change, so
#30 (rebasing onto #29) is unaffected. The `assembleTree` helper keeps its latent sub-task
attachment logic (still unit-tested) but production never feeds it sub-tasks.

## Deferred Strategies

### fixVersion-based release scope (archived reference implementation)

The original implementation determined release scope using the standard JIRA `fixVersion` field.
This approach is valid for open-source projects and teams that use the standard JIRA
Releases/Versions feature, and is preserved here as the reference design for a future
**pluggable release-scope module**.

**Fetch topology (fixVersion):**
1. Release picker: `GET /rest/api/3/project/{projectKey}/versions` — returns all fix versions;
   display unreleased first (alphabetical), released below (newest first by release date).
2. Phase 1: `fixVersion = "{versionName}" ORDER BY issuetype` — returns epics, stories, and any
   sub-tasks that happen to carry the fix version.
3. Phase 2 (upward): collect parent epic keys from non-epic issues that are absent from Phase 1
   results; fetch via `issueKey IN ({key1},{key2},...)`.
4. No Phase 3 — sub-tasks only appear if they individually carry the `fixVersion`, which is
   uncommon because the `fixVersion` field is often excluded from the sub-task issue type screen.

**Limitations that motivated the pivot to custom-field-on-Epics:**
- Sub-tasks almost never carry `fixVersion` → sub-task level is effectively invisible.
- Stories may not carry `fixVersion` even when their epic is in scope → coverage gaps.
- The `fixVersion` field cannot be set on sub-tasks in many JIRA project configurations.

**Future pluggable module:** a `ReleaseDimension` interface would allow each deployment to
configure its own release-scope strategy without forking Fern:
```go
type ReleaseDimension interface {
    // GetReleases returns the picker options for the project.
    GetReleases(ctx context.Context, ...) ([]Release, error)
    // Phase1JQL returns the JQL to fetch the top-level issues for a release value.
    Phase1JQL(value string) string
}
```
`FixVersionDimension` and `CustomFieldEpicDimension` would be the two reference implementations.
See GitHub issue [#197](https://github.com/guidewire-oss/fern-platform/issues/197) —
*Customizable release-scope mapping modules*.

## Known Limitations & Follow-ups

### Discovered 2026-06-16 — stale coverage after credential rotation

1. **Coverage data may be stale if JIRA connection credentials are changed.**
   The coverage service decrypts credentials from the active JIRA connection on every request.
   If an API token is rotated and the old token is invalidated before the new token is saved in
   Fern, all coverage tree requests will fail with an authentication error until the new
   credentials are stored.

   There is no background cache to flush — each request goes live to JIRA. If a token rotation
   happens silently, users will see an error in the coverage UI until an administrator re-enters
   the credentials on the Integrations settings page.

   **Recommendation:** Update JIRA credentials in Fern *before* revoking the old token to avoid
   a coverage outage during rotation.

### Discovered 2026-06-16 — releaseFieldId must be configured per connection

2. **The `releaseFieldId` is not yet stored on the JIRA connection record.**
   The custom field ID (e.g. `cf[12345]`) is organization-specific and must be configurable.
   Until it is stored and exposed via the JIRA connection settings UI, the coverage view cannot
   be used against real project data. A new `release_field_id` column on `jira_connections`
   (with a corresponding migration and UI field) is required before the Epic-first strategy
   can be exercised end-to-end.
