# v2 JIRA & Coverage Parity — Requirements

## Context

The v2 SPA (RFC-004) reached parity with most of the v1 UI, but three
JIRA/coverage features that shipped in v1 were never ported:

- **#26** JIRA field-mapping configuration (v1 commit `31ec433`)
- **#29** JIRA requirement-coverage hierarchy (v1 commit `f6ef2f3`)
- **#30** Release-dashboard coverage UI (v1 commit `0460fff`)

The backend and GraphQL surface for all three already exist and are
unchanged by this work. This spec covers only the **v2 SPA** additions
that consume that existing surface, plus the navigation to reach them.

Existing GraphQL surface (source of truth, do not modify):

- `jiraFieldMapping(projectId: String!): JiraFieldMapping!`
- `jiraFields(connectionId: ID!): [JiraFieldGQL!]!`
- `saveJiraFieldMapping(input: SaveJiraFieldMappingInput!): JiraFieldMapping!`
- `resetJiraFieldMapping(projectId: String!): JiraFieldMapping!`
- `jiraFixVersions(projectId: ID!): [JiraRelease!]!`
- `requirementCoverage(projectId: ID!, fixVersionName: String!): RequirementCoverageTree!`
- enums `FernField`, `ReductionStrategy`

v1 reference implementation: `web/index.html` — field mapping (search
"field mapping"), coverage components (`~6775` onward).

## Requirements (EARS)

### FR-1 Field-mapping view
- WHEN a user with manage rights opens a project's JIRA settings, the
  system SHALL display the current field mapping (Fern field → JIRA
  field, multi-value flag, reduction strategy) from `jiraFieldMapping`.
- WHERE the project has a connected JIRA connection, the system SHALL
  populate JIRA-field choices from `jiraFields(connectionId)`.
- WHEN the user edits and saves, the system SHALL call
  `saveJiraFieldMapping` and reflect the returned mapping without a
  full reload.
- WHEN the user resets, the system SHALL call `resetJiraFieldMapping`
  and show the default mapping.
- IF a required Fern field is left unmapped, the system SHALL block save
  and show a validation message (mirrors v1 server rule).

### FR-2 Requirement-coverage view
- WHEN a user opens a project's coverage view, the system SHALL list
  available releases from `jiraFixVersions(projectId)` in a selector.
- WHEN a release is selected, the system SHALL render the tree from
  `requirementCoverage(projectId, fixVersionName)`: epics → stories →
  sub-tasks, each showing covered/uncovered state and, when covered,
  test-run coverage (total/passed/failed/skipped, last run).
- The system SHALL render unassigned stories in their own section.
- WHEN a covered story's coverage is clicked, the system SHALL show the
  contributing spec runs (v1 `CoverageSpecRunsModal` behavior).

### FR-3 Access & scoping
- The coverage and field-mapping surfaces SHALL be reachable only for
  projects the user can access (same team scoping as the rest of v2).
- Field-mapping **edit/save/reset** SHALL be gated on `canManage`
  (admin/manager), matching project-settings mutations. Read-only view
  is allowed for any user who can see the project.

### FR-4 Navigation & empty states
- The features SHALL be reachable from the project detail / settings
  area (exact placement in design.md).
- WHERE a project has no JIRA connection, the system SHALL show a clear
  empty state pointing to connection setup rather than erroring.

## Non-goals
- No changes to the GraphQL schema, resolvers, or JIRA backend.
- No new REST endpoints (these features are GraphQL-only in v2).
- The legacy v1 UI is untouched.

## Acceptance
- Field mapping and coverage render against real seeded/JIRA data in the
  running app (end-to-end), not just unit tests.
- Non-managers see read-only field mapping; the save/reset controls are
  hidden. Non-members cannot reach either surface.
