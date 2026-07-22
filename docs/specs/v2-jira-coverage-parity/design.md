# v2 JIRA & Coverage Parity — Design

## Build reconnaissance (findings)

- v2 SPA: React + TanStack Query + TanStack Router. Routes in
  `web-v2/src/router.tsx` via `createRoute` + `lazyRouteComponent`.
  A project-settings route already exists (`projects/$projectId/settings`,
  `ProjectSettings.tsx`) and hosts `JiraConnections`.
- Data access helpers in `web-v2/src/lib/api.ts`:
  `restFetch<T>(path, init)` and `graphqlFetch<T>(query, variables)`.
  Field mapping & coverage are **GraphQL-only** → use `graphqlFetch`.
- Existing hook conventions: `web-v2/src/features/jira/hooks.ts`
  (React Query, `useQuery`/`useMutation`, query-key helpers).
- Access: `useCurrentUser()` for role; per-project `canManage` comes from
  the project GraphQL node. Team scoping is enforced server-side.
- v1 reference UX: `web/index.html` field-mapping section and coverage
  components (`CoverageSpecRunsModal`, story/epic rows) ~line 6775+.

## Component & file plan

### Feature: JIRA field mapping (FR-1)
- `web-v2/src/features/jira/fieldMappingHooks.ts`
  - `useJiraFieldMapping(projectId)` → `jiraFieldMapping`
  - `useJiraFields(connectionId)` → `jiraFields` (enabled when a
    connection exists)
  - `useSaveJiraFieldMapping(projectId)` / `useResetJiraFieldMapping(projectId)`
- `web-v2/src/features/jira/FieldMapping.tsx` — table of Fern fields
  (from `FernField` enum) each with: JIRA-field select (from
  `jiraFields`), multi-value flag, reduction-strategy select
  (`ReductionStrategy`). Save/Reset buttons gated on `canManage`.
  Read-only rendering otherwise.
- Surface it inside `ProjectSettings.tsx` (a "JIRA field mapping"
  section beneath the connection panel) so it shares the settings route
  and the project's `canManage`.

### Feature: Requirement coverage (FR-2, covers #29 + #30)
- `web-v2/src/features/jira/coverageHooks.ts`
  - `useJiraFixVersions(projectId)` → `jiraFixVersions`
  - `useRequirementCoverage(projectId, fixVersionName)` →
    `requirementCoverage` (enabled when a version is selected)
- `web-v2/src/features/jira/CoverageTree.tsx` — release selector +
  epic/story/subtask tree. Pure render helpers (percent, covered flag)
  extracted for unit testing.
- `web-v2/src/features/jira/CoverageView.tsx` — page wrapper.
- Route `projects/$projectId/coverage` (lazy). Linked from project
  detail/settings.
- Optional spec-runs drill-in mirrors v1 `CoverageSpecRunsModal`
  (uses the `CoveredSpecRun` type / its query) — deferred to a follow
  task if the query needs confirmation.

## GraphQL types (already defined — client mirrors)
- `FernField`, `ReductionStrategy` enums.
- `JiraFieldMapping { projectId, entries[], updatedBy, updatedAt }`,
  `FieldMappingEntry { fernField, jiraFieldId, jiraFieldIsMultiValue, reductionStrategy }`.
- `RequirementCoverageTree { fixVersion, epics[], unassigned[] }`,
  `EpicCoverageNode`, `StoryCoverageNode`, `TestRunCoverage`,
  `JiraRelease`, `JiraIssueSummary`.

## Testing strategy
- Pure logic (coverage percent, covered/uncovered derivation, mapping
  validation "required field unmapped") → Vitest unit tests, TDD-first.
- Hooks/components: light render tests with mocked `graphqlFetch`
  (mirror `ProjectFormModal.test.tsx` mocking style).
- End-to-end: exercise both surfaces in the running app against a
  project with a JIRA connection before marking tasks done.

## Access control
- Read: any user who can see the project (server scopes the project
  query already).
- Write (save/reset mapping): gate UI on the project's `canManage`;
  server still enforces.

## Risks / open items
- Release selector default: pick the most recent unreleased version, or
  first in list — confirm against v1.
- `jiraFields` requires a `connectionId`; when no connection exists,
  show the FR-4 empty state and skip the query.
- Spec-runs drill-in query name to confirm in schema before building
  that sub-task.
