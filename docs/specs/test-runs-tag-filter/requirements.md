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
- **R6** Where `facets=tag` is requested, the tag facet shall count live
  (not soft-deleted) runs per tag for tags attached at run or suite
  level. Spec-level tags are excluded from the facet (cost:
  `spec_run_tags` is the largest link table) but still filter (R1).
- **R7** Facet cache entries shall be keyed on whether the tag facet was
  requested, so a list request without `facets=tag` never serves its
  (empty) tag facet to a later `facets=tag` request.

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
  branches only). It and the status/branch/project facets add
  `test_runs.deleted_at IS NULL` explicitly: they use `Table` + `Scan`,
  which skips GORM's soft-delete scope that the list and count get.
- `application.filterKeyView` includes `IncludeTagFacet` in the facet
  cache key.
- `internal/api/v2/test_run_handler.go`: `listForProject` copies the query,
  pins `project` to the path segment, and shares `serveList` with `list`.
  Documented in `docs/api/v2-openapi.yaml` (shared list parameters live
  under `components/parameters`). Like every v1 `/projects/:projectId`
  route, it cannot address a project ID containing `/`.

## Verification

- SQLite repo tests (`test_run_tag_filter_test.go`): each level matches,
  project scoping, no duplicates, OR/AND across levels, soft-deleted specs
  ignored, facet counts run + suite but not spec, facets skip
  soft-deleted runs. These fail on the previous code.
- Cache key test: with and without `IncludeTagFacet` differ.
- Handler tests: path pins project over `?project=`, params pass through,
  default window is now − 7 days, scope (readable / unreadable → empty / unrestricted).
- Against Postgres with seeded data: for 300 (tag, project) pairs the
  filter returned exactly the runs found by the JIRA coverage lookup SQL;
  `EXPLAIN ANALYZE` shows index access in all three branches.
