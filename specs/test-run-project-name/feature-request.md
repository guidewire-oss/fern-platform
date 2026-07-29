# Feature request: Show project name (not project ID) in the Test Runs list and filter

> Filed as [guidewire-oss/fern-platform#216](https://github.com/guidewire-oss/fern-platform/issues/216).
> This file is the source text for that issue.

## Problem

On the Test Runs page (`/test-runs`) the **Project** column renders the raw
`project_id`, an opaque identifier. The **Project** facet in the filter sidebar has the
same problem: it lists `project_id` values, so users must know the ID of the project
they want to filter by.

Project display names already exist in `project_details.name`, but neither
`GET /api/v2/test-runs` nor its `facets.byProject` block returns them.

## Proposal

1. `GET /api/v2/test-runs` returns `project_name` on each test-run node, resolved from
   `project_details` in a single batched lookup keyed by the project IDs in play.
2. `facets.byProject` entries carry an optional `label` holding the project name. Each
   entry's `value` stays the `project_id`, so saved views and existing `?project=`
   links keep working unchanged.
3. The Test Runs table shows the project name, falling back to the ID when a project
   row is missing or unnamed, with the ID available on hover.
4. The Project facet in the filter sidebar lists and sorts by project name, still
   submitting the ID.

## Out of scope

- Changing the wire format of the `project` filter parameter (must stay ID-based).
- v1 API endpoints.

## Acceptance criteria

- A run whose project has a name shows the name in the Project column; hovering shows
  the ID.
- A run whose project has no `project_details` row still shows its ID (no blank cell).
- The Project filter section shows names, sorted naturally by name.
- Selecting a project by name filters by the underlying ID; existing saved views still
  apply.
- Resolving names adds at most one extra query per list request.

## Spec

Filed as [#216](https://github.com/guidewire-oss/fern-platform/issues/216).

Spec: `specs/test-run-project-name/` — `requirements.md`, `design.md`, `tasks.md`.
