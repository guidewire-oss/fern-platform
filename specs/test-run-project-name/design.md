# Design: Project Name in Test Runs List and Filter

## Build reconnaissance

- No code generation touches this path. `gqlgen.yml` generates the GraphQL layer, but
  `/api/v2/test-runs` is a hand-written Gin handler (`internal/api/v2/test_run_handler.go`)
  serialising `domain.TestRun` directly via its `json` tags.
- The v2 read path is layered: handler → `application.TestRunQueryService` →
  `infrastructure.TestRunQueryRepo`. The service already owns cross-cutting enrichment
  (it attaches cached facets after the pager returns), which is the natural seam for
  name resolution too.
- Frontend types are hand-maintained in `web-v2/src/lib/types.ts` (the codegen pipeline
  is not wired yet), so the node and facet shapes are edited by hand.
- `project_details.project_id` has a unique index, so a batched `IN (...)` lookup is
  index-served.

## Data model

No migration. `project_details` already holds `project_id` → `name`.

## Backend changes

### 1. Domain (`internal/domains/testing/domain/`)

`test_run.go` — add to `TestRun`:

```go
ProjectName string `json:"project_name"`
```

`filter.go` — add to `FacetCount`:

```go
Label string
```

`Label` is the human-readable name for `Value`. Only the project facet populates it.

### 2. Name resolver (`infrastructure`)

A new narrow port, consumer-owned by the application layer:

```go
// ProjectNameResolver maps project IDs to display names.
type ProjectNameResolver interface {
    NamesByProjectID(ctx context.Context, ids []string) (map[string]string, error)
}
```

Implemented in `internal/domains/testing/infrastructure/project_name_repo.go` as a
single GORM query:

```sql
SELECT project_id, name FROM project_details WHERE project_id IN (?)
```

De-duplicating the ID list before the query keeps it one round trip regardless of page
size (Requirement 1.4). An empty ID list short-circuits to an empty map with no query.

### 3. Enrichment in `TestRunQueryService.Query`

After the pager returns and before/alongside facet attachment:

1. Collect the distinct `ProjectID`s from `res.Edges` **and** from
   `res.Facets.ByProject` (the facet may reference projects absent from the current
   page) into one ID set.
2. One `NamesByProjectID` call for that set.
3. Set `node.ProjectName` for each edge, and `Label` for each `ByProject` entry, when
   the map has a non-empty name.

Ordering matters: facets are attached first (from cache or `ComputeFacets`), then names
resolve for edges *and* facet values in a single lookup. A resolver error is swallowed
the same way a facet error is: the page still returns (Requirement 1.5).

The resolver is optional on the service: `NewTestRunQueryService` keeps its current
signature and a `WithProjectNames(r)` builder attaches the resolver, so existing test
wiring and the no-resolver path stay valid.

Labels are applied *after* the cache read, so a cached facet set still gets fresh
labels — but onto a **copy**, never the cached slice itself. See the cache-safety note
at the end of this document.

### 3b. Duration passthrough (Requirement 3)

`toDomain` in `test_run_query_repo.go` never copied `duration_ms`, so every v2 node
reported a zero duration and the SPA fell back to `end_time - start_time`. It now sets
`Duration: time.Duration(r.Duration) * time.Millisecond`.

`domain.TestRun.Duration` is a `time.Duration`, which marshals as **nanoseconds** — not
what the client wants. Rather than change that shared field's JSON shape (the GraphQL
path also reads it), the v2 handler wraps the node:

```go
type nodeDTO struct {
    *domain.TestRun
    DurationMs int64 `json:"duration_ms"`
}
```

Embedding keeps every existing field's JSON name intact while adding the millisecond
form the SPA reads.

### 4. Handler DTO (`internal/api/v2/test_run_handler.go`)

`facetCountDTO` gains:

```go
Label string `json:"label,omitempty"`
```

`omitempty` satisfies Requirements 2.3 and 2.4 without special-casing per facet: the
status/branch/tag facets never set `Label`, so the key simply does not appear.

`edgeDTO.Node` is already `*domain.TestRun`, so `project_name` ships automatically once
the domain field exists.

### 5. Wiring

Wherever `NewTestRunQueryService` is constructed for production, chain
`.WithProjectNames(infrastructure.NewProjectNameRepo(db))`.

## Frontend changes (`web-v2/src/`)

### `lib/types.ts`

```ts
export interface TestRunNode {
  // …
  project_name?: string;
}

export interface FacetCount {
  value: string;
  count: number;
  label?: string;
}
```

`project_name` is optional so the SPA keeps type-checking against an older backend.

### `features/test-runs/TestRunsList.tsx`

The Project cell renders the name as the primary line with the `project_id` beneath it
in muted, smaller text. When no name resolved, only the ID renders, with no empty primary
line and no duplicate. The `to`/`params` stay as they are.

A shared `LabeledValue` presentational piece (label + value stacked, value-only
fallback) backs this cell, the filter facet entry, and the detail-page header, so all
three stay consistent. Its sorting helpers live in `facetSort.ts` rather than the
component file so fast refresh keeps working.

### `features/test-runs/FilterSidebar.tsx`

`FacetGroup` currently takes `values: string[]` and looks counts up by value. It changes
to take the `FacetCount[]` directly so each entry can render the same stacked
name-over-ID treatment as the table cell (ID alone when unlabelled) while still toggling
on `value`.

Sorting moves from "sort the raw values" to "sort by display text": a small
`sortFacetsNatural(facets)` helper sorts `displayText(entry)` with the existing
`localeCompare(..., {numeric: true, sensitivity: 'base'})` collation. The branch facet
keeps its priority-branch ordering; only the display-text extraction is shared.

`toggleArr('project', v)` is unchanged; it already receives the facet `value`.

## Testing strategy

| Layer | Test | Covers |
| --- | --- | --- |
| `infrastructure/project_name_repo_test.go` | batched lookup, dedup, empty input, missing rows | 1.2, 1.3, 1.4 |
| `application/test_run_query_service_test.go` | edges enriched, facet labels set, facet-only IDs resolved, resolver error tolerated, nil resolver | 1.1–1.5, 2.1–2.3 |
| `api/v2/test_run_handler_test.go` | `project_name` in node JSON; `label` present on project facet and absent elsewhere | 1.1, 2.1, 2.3, 2.4 |
| `TestRunsList.test.tsx` | name + ID shown, ID-only fallback, link unchanged, duration incl. zero/absent cases | 1.6–1.8, 3.3–3.5 |
| `TestRunDetail.test.tsx` | header shows name + ID, ID-only fallback, link unchanged | 4.4, 4.5 |
| `FilterSidebar.test.tsx` | labels rendered, sorted by label, toggle emits ID, fallback to ID | 2.5–2.7 |

## Risks

- **Cached facets + labels.** Applying labels after the cache read (onto a copy) avoids
  both staleness when a project is renamed and mutation of shared cached state.
- **N+1 regression.** The single batched query is the whole point; the repo test asserts
  one call for a multi-project page.

## Detail page (Requirement 4)

The run detail page reads through GraphQL, so the v2 REST work above does not reach it.

- `schema.graphql` gains a nullable `projectName` on `TestRun`; gqlgen regenerates
  `generated.go` and `model/models_gen.go`.
- Because the generated model carries `projectName` as a plain field (not a resolver
  stub), it is populated by a shared `attachProjectName` helper in the two query
  paths that build a run: `GetTestRun_domain` and `TestRunByRunID`. The helper sits on
  the resolver. The helper reads through the existing `projectService`, tolerates a nil
  service (some test wirings leave it unset), and swallows lookup errors so a missing
  name never fails an otherwise-good query.
- The SPA adds `projectName` to the detail query and renders it through the same
  `LabeledValue` used by the list, so both pages read identically.

Not using a dataloader here is deliberate: the detail page fetches exactly one run, so
the lookup is a single extra indexed query. The existing `GetProjectLoader` also keys on
`project_details.id`, not `project_id`, so it would not fit without change.

## Cache-safety note (found in review)

`MemoryFacetCache.Get` returns the stored `TestRunFacets` by value, but its slices share
one backing array with the cached entry. Writing labels in place would therefore race
with a concurrent request marshalling the same slice, and would persist labels into the
cache, where a later request whose lookup failed would serve them as if fresh.
`applyProjectNames` copies `ByProject` before labelling.

## Shared date-range control (Requirement 6)

The test-runs sidebar had its date section written inline. Rather than build a
lookalike for the project page, that section moved into
`features/test-runs/DateRangeFilter.tsx` and both pages render it, so the two windows
are picked identically by construction. `DATE_PRESETS`, `presetRange`, and
`matchPresetDays` moved alongside the existing helpers in `dateRange.ts`.

The component is stateless — each page owns its bounds, because they feed different
queries. Two props cover the presentational difference: `showHeading` (off on the
project page, which already has a header) and `clearLabel` (the sidebar says "clear",
the project page says "All time", where clearing genuinely means all time).

`setBound` always emits both `from` and `to`, even when one is undefined.
`customRangeToQuery` omits a key whose input is empty, and the sidebar spreads the
result over its existing filter — so emitting a partial object let a cleared bound keep
its old value. That was a regression caught in review and is covered by a test.

### The invisible window

`ProjectDetail` sent no bounds, so the handler's 7-day default applied (see
`test_run_handler.go`). The page now defaults to an explicit 7 days, which preserves
what users saw, and clearing the range sends `allTime=1` to opt out of that default
rather than silently landing back on a week.
