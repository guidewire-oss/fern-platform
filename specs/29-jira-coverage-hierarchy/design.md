# 29-jira-coverage-hierarchy - Design

## Overview

To be completed after requirements are finalised.

## Key Design Decisions

### Live JIRA Queries (No Local Cache)

For the initial implementation, JIRA hierarchy data is fetched live on every page load using two batched JQL calls. Caching is deferred to a future iteration.

### Two-Phase JIRA Fetch

1. **Phase 1:** `fixVersion = "{version}" ORDER BY issuetype` — returns all issues in the release (stories, bugs, tasks, epics).
2. **Phase 2:** `issueKey IN ({parent keys})` — fetches epic details for any parents referenced in Phase 1 results but not already returned.

### Coverage Query

Fern DB query: find all distinct `tag.value` where `tag.category = 'jira'` and the tag is associated with a test run belonging to the given project. Then count test runs per issue key for pass/fail breakdown.

## Components to Build

### Backend

- `jira_client.go` — add `SearchIssues(jql string) ([]JiraIssue, error)` and `GetVersions(projectKey string) ([]JiraVersion, error)`
- New service: `jira_coverage_service.go` — orchestrates the two-phase fetch and coverage cross-reference
- GraphQL schema additions: `JiraVersion`, `JiraIssueNode`, `RequirementCoverageNode`, `RequirementCoverageTree`
- New resolver: `requirementCoverage(projectId, fixVersion)` query

### Frontend

- New "Coverage" tab in project detail page
- Fix version picker component (grouped dropdown)
- Hierarchy tree component: epic rows expand to story rows, each showing coverage indicator
- "Show uncovered only" toggle
