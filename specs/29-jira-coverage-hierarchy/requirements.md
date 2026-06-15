# 29-jira-coverage-hierarchy - Requirements

## Introduction

Fern already ingests test tags in the form `jira:PROJ-123`, creating an association between tests and JIRA issue keys. This feature adds a Requirements Coverage view that lets managers see, for a selected JIRA **release version** (a JIRA fix version, e.g. "Atmos vNext"), which epics and stories have test coverage in Fern and which do not — without syncing JIRA data into Fern. The hierarchy is fetched live from JIRA using the project's stored connection credentials, and rolls up Release → Epic → Story → Sub-task with results-aware status.

> Terminology: the UI labels these **"release versions"** (matching JIRA's Releases page); the JIRA API/JQL field is `fixVersion`, which is retained verbatim in queries.

## Requirements

### Requirement 1: Release Version Selection

**User Story:** As a manager, I want to select a JIRA release version so that I can see coverage for a specific release.

#### Acceptance Criteria

1. WHEN a user opens the Requirements Coverage view for a project THEN the system SHALL fetch the full list of release versions (JIRA fix versions / Releases) from the JIRA project configured in the project's connection, using `GET /rest/api/3/project/{projectKey}/versions`.
2. THE SYSTEM SHALL display release versions in a searchable picker (placeholder "Filter releases…"): unreleased versions first (sorted alphabetically), released versions below (sorted newest first by release date). The user narrows the list by typing (e.g. "Atmos") to filter by name.
3. WHEN a user selects a release version THE SYSTEM SHALL fetch all JIRA issues assigned to that version via JQL: `fixVersion = "{version}" ORDER BY issuetype`.
4. IF a project has no active JIRA connection THEN THE SYSTEM SHALL display an appropriate message prompting the user to configure one.
5. The JIRA project key is already a required field on the JIRA connection (`JiraConnection.ProjectKey()`) — no schema change is needed.

### Requirement 2: Hierarchy Fetch and Display

**User Story:** As a manager, I want to see test coverage organized by JIRA epic, story, and sub-task so that I can identify gaps.

#### Acceptance Criteria

1. WHEN issues are fetched for a fix version THE SYSTEM SHALL request the `parent` field in the JQL search response (JIRA Cloud supports `parent` uniformly for both classic and next-gen projects). Issues whose `parent` is an Epic are linked directly; no custom epic-link field lookup is required.
2. THE SYSTEM SHALL display issues in a three-level hierarchy: epics at the top, stories grouped beneath their epic, and sub-tasks grouped beneath their parent story (where sub-tasks exist in the fix version result set).
3. WHEN an issue has no parent epic THE SYSTEM SHALL group it under an **"Issues without an Epic"** section. That section SHALL be sortable by coverage percentage (most-covered first or least-covered first).
4. WHEN displaying each story THE SYSTEM SHALL show: issue key, summary, status, and a results-aware coverage indicator — see Requirement 4 for color semantics (grey = uncovered, red = has a failing test, green = covered & passing).
5. WHEN displaying each epic THE SYSTEM SHALL show: coverage percentage (e.g. "60% (3/5 stories)") and a visual coverage bar whose fill corresponds to that percentage, an `✗ N failing` chip when any descendant story has a failing test, and a color conveying aggregate health (see Requirement 4).
6. WHEN displaying a covered story or sub-task THE SYSTEM SHALL show: total number of tests linked, pass/fail/skipped breakdown, and date of last test execution.
7. Every JIRA issue key displayed in the hierarchy (epic, story, sub-task) SHALL be rendered as a hyperlink to the corresponding issue in the JIRA instance (`<jira-base-url>/browse/<key>`), opening in a new tab.

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

1. THE SYSTEM SHALL display a release-level roll-up row at the top of the tree showing the selected release version and aggregate coverage across all epics and the Issues-without-an-Epic bucket (walking sub-tasks). Hierarchy is **Release → Epic → Story → Sub-task**.
2. Coverage breadth and test health SHALL be presented as **separate** elements so neither is misread as the other: a labelled coverage figure (`<covered>/<total> covered · <pct>%`) and a distinct health pill.
3. The release health pill SHALL be one of: **Release ready** (fully covered, no failures), **✗ N failing** (N = issues with a failing test), **In progress** (partially covered, no failures), **Not started** (0% covered).
4. Color SHALL encode health, not coverage: **grey** = uncovered / not started, **red** = has ≥1 failing test, **green** = covered with no failures, neutral = partially covered with no failures. Failures block "ready"; **skips do not** (skip counts are shown as `↺N` in text but never change color).
5. A story/epic/release that is fully covered but has a failing test SHALL NOT appear green; red SHALL NOT be used to mean "uncovered" at any level.

### Requirement 5: Non-Functional Requirements

**User Story:** As a user, I want the coverage view to load in a reasonable time so that it is usable in practice.

#### Acceptance Criteria

1. THE SYSTEM SHALL fetch JIRA issue details using a single batched JQL query rather than one call per issue, to minimise JIRA API round-trips.
2. WHEN parent epics are not already present in the fix version result set THE SYSTEM SHALL fetch them using a single batched JQL query (`issueKey IN (...)`) rather than one call per epic. Epics already returned in the fix version results require no second fetch.
3. IF the JIRA API is unreachable THEN THE SYSTEM SHALL display an error and not crash the page.
4. THE SYSTEM SHALL handle fix versions with up to 500 issues without timeout.

## Constraints

- Fern does not store a local copy of JIRA issues — hierarchy is always fetched live from JIRA.
- Only projects with an active JIRA connection can use this feature.
- Historical coverage data is limited to test runs that were submitted to Fern after the JIRA integration was configured; pre-integration runs will show 0% coverage.
- Initial implementation does not support multi-select of fix versions (single selection only).
- Sub-task support is limited to sub-tasks that appear in the fix version result set; sub-tasks not assigned to the fix version are not fetched separately.

## Out of Scope (deferred)

The following items appear in GitHub issue #29 but are deferred pending confirmation that they are still needed:

- **Export to PDF/Excel** — not implemented; raise a follow-on issue if confirmed needed
- **Lazy loading / virtual scrolling for 1000+ nodes** — current implementation loads the full tree upfront; acceptable for typical fix-version sizes; revisit if performance becomes a problem in practice
