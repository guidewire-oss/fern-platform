# 29-jira-coverage-hierarchy - Requirements

## Introduction

Fern already ingests test tags in the form `jira:PROJ-123`, creating an association between tests and JIRA issue keys. This feature adds a Requirements Coverage view that lets managers see, for a selected JIRA fix version (e.g. "Atmos vNext"), which epics and stories have test coverage in Fern and which do not — without syncing JIRA data into Fern. The hierarchy is fetched live from JIRA using the project's stored connection credentials.

## Requirements

### Requirement 1: Fix Version Selection

**User Story:** As a manager, I want to select a JIRA fix version so that I can see coverage for a specific release.

#### Acceptance Criteria

1. WHEN a user opens the Requirements Coverage view for a project THEN the system SHALL fetch the full list of fix versions (JIRA Releases) from the JIRA project configured in the project's connection, using `GET /rest/api/3/project/{projectKey}/versions`.
2. THE SYSTEM SHALL display fix versions in a searchable picker: unreleased versions first (sorted alphabetically), released versions below (sorted newest first by release date). The user narrows the list by typing (e.g. "Atmos") to filter by name.
3. WHEN a user selects a fix version THE SYSTEM SHALL fetch all JIRA issues assigned to that version via JQL: `fixVersion = "{version}" ORDER BY issuetype`.
4. IF a project has no active JIRA connection THEN THE SYSTEM SHALL display an appropriate message prompting the user to configure one.
5. The JIRA project key is already a required field on the JIRA connection (`JiraConnection.ProjectKey()`) — no schema change is needed.

### Requirement 2: Hierarchy Fetch and Display

**User Story:** As a manager, I want to see test coverage organized by JIRA epic, story, and sub-task so that I can identify gaps.

#### Acceptance Criteria

1. WHEN issues are fetched for a fix version THE SYSTEM SHALL request the `parent` field in the JQL search response (JIRA Cloud supports `parent` uniformly for both classic and next-gen projects). Issues whose `parent` is an Epic are linked directly; no custom epic-link field lookup is required.
2. THE SYSTEM SHALL display issues in a three-level hierarchy: epics at the top, stories grouped beneath their epic, and sub-tasks grouped beneath their parent story (where sub-tasks exist in the fix version result set).
3. WHEN an issue has no parent epic THE SYSTEM SHALL group it under an "Unassigned" section. The Unassigned section SHALL be sortable by coverage percentage (most-covered first or least-covered first).
4. WHEN displaying each story THE SYSTEM SHALL show: issue key, summary, status, and coverage indicator (covered / not covered).
5. WHEN displaying each epic THE SYSTEM SHALL show: coverage percentage (e.g. "60% (3/5 stories)"), total story count, and a visual coverage bar whose fill corresponds to the coverage percentage.
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

### Requirement 4: Non-Functional Requirements

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
