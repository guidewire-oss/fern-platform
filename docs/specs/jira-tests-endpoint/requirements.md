# JIRA Tests Lookup Endpoint — Requirements

## Context

Test runs report `@jira:<KEY>` labels, which ingest stores in `tags`
(`category='jira'`) and links through `spec_run_tags` / `test_run_tags`.
Runs belong to a project via `test_runs.project_id`. Clients need a REST
call that answers "does project P have any tests tagged with KEY?".

Existing query reused unchanged:
`CoverageService.GetSpecRunsByJiraTag(ctx, projectID, issueKey)` (the same
path the GraphQL `specRunsByJiraTag` resolver uses).

## Requirements (EARS)

- **R1** When a client sends `GET /api/v2/projects/{projectId}/jira/{issueKey}/tests`,
  the system shall return `200` with `{projectId, issueKey, hasTests}`,
  where `hasTests` is true if any non-deleted spec run or test run in that
  project carries the JIRA tag.
- **R2** The system shall match the issue key case-insensitively and
  return it upper-cased.
- **R3** If the upper-cased `issueKey` does not match
  `integrations.JiraKeyPattern` (`^[A-Z][A-Z0-9]+-\d+$`, the same rule the
  coverage tree uses), the system shall return `400`.
- **R4** If the caller has no read access to `projectId` (and is not
  unrestricted), the system shall return `403` without querying.
- **R5** If the authorization lookup or the query fails, the system shall
  return `500`.
- **R6** The endpoint shall sit behind the same auth middleware as the
  other `/api/v2` routes.

## Design

- `internal/api/v2/jira_tests_handler.go`: `JiraTestsHandler` with a
  one-method `JiraTestLookup` interface (satisfied by
  `integrations.CoverageService`) and the shared `ProjectScoper`.
- `MountOptions.JiraTests` wires it in `MountV2`; `cmd/fern-platform/main.go`
  passes `domainFactory.GetCoverageService()`, which is always constructed
  and does not need `JIRA_ENCRYPTION_KEY` for this lookup.
- Access is project read (same as other v2 reads), not manager-level like
  the GraphQL `specRunsByJiraTag`.
- Known ceiling: the reused query fetches up to 200 rows to answer a
  boolean. Swap for an `EXISTS` query if it shows up in latency.

## Tasks

- [x] T1 Handler + interface (R1–R5)
- [x] T2 Mount in `MountV2` and `main.go` (R6)
- [x] T3 Handler tests covering R1–R5
- [x] T4 End-to-end check against a running server
