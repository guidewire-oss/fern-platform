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

**User Story:** As a manager, I want to see test coverage organized by JIRA epic and story so that I can identify gaps.

#### Acceptance Criteria

1. WHEN issues are fetched for a fix version THE SYSTEM SHALL request the `parent` field in the JQL search response (JIRA Cloud supports `parent` uniformly for both classic and next-gen projects). Issues whose `parent` is an Epic are linked directly; no custom epic-link field lookup is required.
2. THE SYSTEM SHALL display issues in a two-level hierarchy: epics at the top, stories grouped beneath their epic.
3. WHEN an issue has no parent epic THE SYSTEM SHALL group it under an "Unassigned" section.
4. WHEN displaying each story THE SYSTEM SHALL show: issue key, summary, status, and coverage indicator (covered / not covered).
5. WHEN displaying each epic THE SYSTEM SHALL show: coverage percentage (stories with at least one test / total stories) and total story count.

### Requirement 3: Coverage Cross-Reference

**User Story:** As a manager, I want to see which stories have been exercised by tests so that I can assess release readiness.

#### Acceptance Criteria

1. WHEN building the coverage view THE SYSTEM SHALL query Fern's tag data to find all `category="jira"` tags on test runs belonging to the project.
2. A story SHALL be considered "covered" if at least one test run in the project has a tag matching `jira:{issueKey}`.
3. WHEN a story is covered THE SYSTEM SHALL show the count of associated test runs and a pass/fail breakdown (latest run result per test).
4. WHEN a story is not covered THE SYSTEM SHALL show it clearly as uncovered (no test association found).
5. THE SYSTEM SHALL support a toggle to show only uncovered stories, hiding fully covered ones.

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
