# Test Runs by Tag — Requirements

## Context

Clients need "the runs in project P that carry tag T", for any tag, not
just JIRA keys (e.g. `jira:GWCP-123`, `smoke`). Ingest stores tags in
`tags` and links them at three levels: `test_run_tags` (the run),
`suite_run_tags` (a suite in the run) and `spec_run_tags` (a spec in the
run; `@jira:<KEY>` labels land here).

`GET /api/v2/test-runs` already takes `project=` and `tag=` with cursor
pagination, facets and project scoping, but its tag filter only looked at
`suite_run_tags`, so run- and spec-level tags (including every JIRA tag)
never matched. This change fixes that filter instead of adding a
JIRA-specific endpoint. It supersedes the earlier
`GET /api/v2/projects/{projectId}/jira/{issueKey}/tests` (boolean
`hasTests`), which is removed.

## Requirements (EARS)

- **R1** When a client sends `GET /api/v2/test-runs` with one or more
  `tag` params, the system shall return the runs carrying a matching tag
  on the run itself, on any of its suites, or on any of its specs.
- **R2** Where `tagMode=AND`, the system shall return only runs that carry
  every requested tag, each at any level; otherwise (default `OR`) runs
  carrying at least one.
- **R3** The system shall ignore links through soft-deleted suite runs or
  spec runs, and shall return each matching run once regardless of how
  many levels or specs carry the tag (`totalCount` counts runs).
- **R4** When a client sends `GET /api/v2/projects/{projectId}/test-runs`,
  the system shall respond exactly as `GET /api/v2/test-runs` with
  `project={projectId}`, ignoring any `project` query params; all other
  params (`tag`, `tagMode`, `status`, `branch`, `from`/`to`/`allTime`,
  `first`/`after`, `facets`) behave identically.
- **R5** The project scope shall apply to both routes: a project the
  caller cannot read yields an empty connection (`200`, no edges), as
  `/test-runs?project=` already does.
- **R6** Where `facets=tag` is requested, the tag facet shall count runs
  per tag for tags attached at run or suite level. Spec-level tags are
  excluded from the facet (cost: `spec_run_tags` is the largest link
  table) but still filter (R1).

Tag names match exactly (`t.name`), as before.

## Design

- `internal/domains/testing/infrastructure/test_run_filter_query.go`:
  `tagExistsClause` is one `EXISTS` over a `UNION ALL` of the three link
  levels, each branch yielding `(run_id, tag_name)` with `tags` joined
  inside the branch. One placeholder per clause (the callers' contract).
  Filtering on `rt.tag_name` is a plain restriction, which Postgres pushes
  into every branch, so each starts from the `tags.name` / `*_tags.tag_id`
  indexes. Joining `tags` once outside the union does not work: the
  suite/spec branches contain joins, stay subqueries, cannot receive the
  join qual, and scan their link tables whole.
- `TestRunQueryRepo.tagFacet` joins `RunAndSuiteTagLinksSQL` (run + suite
  branches only).
- `internal/api/v2/test_run_handler.go`: `listForProject` copies the query,
  pins `project` to the path segment, and shares `serveList` with `list`.

## Verification

- SQLite repo tests (`test_run_tag_filter_test.go`): each level matches,
  project scoping, no duplicates, OR/AND across levels, soft-deleted specs
  ignored, facet counts run + suite but not spec. These fail on the
  previous suite-only filter.
- Handler tests: path pins project over `?project=`, params pass through,
  default 7-day window, scope (readable / unreadable → empty / unrestricted).
- Against Postgres with seeded data: for 300 (tag, project) pairs the
  filter returned exactly the runs found by the JIRA coverage lookup SQL;
  `EXPLAIN ANALYZE` shows index access in all three branches.

## Known issue (pre-existing, not changed here)

The facet cache key (`filterKeyView`) omits `IncludeTagFacet`, so a
request without `facets=tag` can cache an empty tag facet that a later
`facets=tag` request with the same filter receives until the entry
expires.
