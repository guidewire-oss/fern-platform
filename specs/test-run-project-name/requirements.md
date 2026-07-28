# Requirements: Project Name in Test Runs List and Filter

## Introduction

The Test Runs page (`/test-runs`, `web-v2/src/features/test-runs/`) renders the
**Project** column and the **Project** facet in the filter sidebar using the raw
`project_id` — an opaque identifier. Users who think in project names have to know
the ID of the project they want to read or filter by.

Project display names already exist in `project_details.name`, keyed by the same
`project_id` carried on `test_runs`. Neither the `GET /api/v2/test-runs` node payload
nor its `facets.byProject` block surfaces that name today.

This feature resolves and returns the project name from the v2 test-runs endpoint and
displays it in both the table column and the filter facet, while keeping the *filter
wire format* keyed by `project_id` so existing saved views and `?project=` links keep
working.

## Requirements

### Requirement 1: Project name on test-run nodes

**User Story:** As an engineer triaging test runs, I want the Project column to show
the project's display name, so that I can identify a project without memorising IDs.

#### Acceptance Criteria

1. WHEN a client calls `GET /api/v2/test-runs` THE SYSTEM SHALL include a
   `project_name` field on every node in `edges[].node`.
2. WHERE a `project_details` row exists for a node's `project_id` THE SYSTEM SHALL set
   `project_name` to that row's `name`.
3. WHERE no `project_details` row exists for a node's `project_id`, OR that row's
   `name` is empty, THE SYSTEM SHALL leave `project_name` empty rather than failing
   the request.
4. THE SYSTEM SHALL resolve names for a page of runs using at most one additional
   database query per list request, regardless of how many runs the page contains.
5. WHEN the name lookup fails THE SYSTEM SHALL still return the page of runs with
   empty `project_name` values (names are advisory, like facets).
6. WHEN the Test Runs table renders a run whose `project_name` is non-empty THE SYSTEM
   SHALL display both the name (as the primary text) and the `project_id` (as secondary
   text) in the Project column.
7. WHERE `project_name` is empty THE SYSTEM SHALL display the `project_id` alone,
   without an empty primary line or a duplicated ID.
8. THE SYSTEM SHALL keep the Project cell's link target unchanged
   (`/projects/$projectId` with the `project_id`).

### Requirement 2: Project name in the filter facet

**User Story:** As an engineer filtering test runs, I want the Project filter to list
project names, so that I can pick a project without looking up its ID.

#### Acceptance Criteria

1. WHEN the v2 test-runs endpoint returns `facets.byProject` THE SYSTEM SHALL include
   an optional `label` on each entry holding the project's display name.
2. THE SYSTEM SHALL keep each `byProject` entry's `value` set to the `project_id`.
3. WHERE a faceted `project_id` has no name THE SYSTEM SHALL omit `label` for that
   entry.
4. THE SYSTEM SHALL NOT add a `label` to the status, branch, or tag facets.
5. WHEN the filter sidebar renders a Project entry that has a `label` THE SYSTEM SHALL
   display both the label and the `value` (the ID); WHERE the entry has no `label` THE
   SYSTEM SHALL display the `value` alone.
6. THE SYSTEM SHALL sort the Project facet naturally by its primary text (name where
   present, ID otherwise).
7. WHEN a user toggles a Project facet entry THE SYSTEM SHALL send the entry's `value`
   (the `project_id`) in the `project` query parameter, unchanged from today.
8. THE SYSTEM SHALL continue to apply saved views and `?project=<id>` links that were
   created before this change, without migration.

### Requirement 3: Duration populates in the Test Runs list

**User Story:** As an engineer scanning test runs, I want the Duration column to show a
run's wall-clock time, so that I can spot slow runs without opening each one.

Found while implementing Requirements 1 and 2: the v2 read path dropped the stored
`duration_ms`, leaving the SPA to derive duration from `end_time - start_time`. Runs
with no `end_time` (still running, or ended abnormally) rendered an em dash.

#### Acceptance Criteria

1. THE SYSTEM SHALL carry the stored `duration_ms` from `test_runs` through the v2 read
   model into `domain.TestRun.Duration`.
2. WHEN a client calls `GET /api/v2/test-runs` THE SYSTEM SHALL include `duration_ms`
   (milliseconds, not nanoseconds) on every node.
3. WHEN the Test Runs table renders a run THE SYSTEM SHALL display the server-reported
   `duration_ms`.
4. WHERE `duration_ms` is absent THE SYSTEM SHALL fall back to `end_time - start_time`.
5. WHERE neither is available THE SYSTEM SHALL render an em dash.

### Requirement 4: Project name on the test-run detail page

**User Story:** As an engineer who drilled into a run from the list, I want the run
header to identify the project the same way the list did, so that the name does not
disappear when I click through.

This page reads through GraphQL, not the v2 REST endpoint, so it needs its own field.

#### Acceptance Criteria

1. THE SYSTEM SHALL expose a nullable `projectName` field on the GraphQL `TestRun` type.
2. WHEN a client queries `testRun` or `testRunByRunId` THE SYSTEM SHALL populate
   `projectName` from `project_details`.
3. WHERE the project has no record or no name, OR the lookup fails, THE SYSTEM SHALL
   return `projectName` as null and SHALL NOT fail the query.
4. WHEN the detail page renders the run header THE SYSTEM SHALL display the project name
   and the `projectId` together, using the same treatment as the list; WHERE there is no
   name it SHALL display the `projectId` alone.
5. THE SYSTEM SHALL keep the header's project link pointing at `/projects/$projectId`.

### Requirement 5: Project name everywhere a run is listed

**User Story:** As a user, I want a project identified the same way on every screen, so
that the name I recognise does not disappear when I navigate.

#### Acceptance Criteria

1. WHEN the project detail page renders its header THE SYSTEM SHALL show the project
   name as the heading with the `projectId` beneath it; WHERE no name resolves it SHALL
   show the `projectId` as the heading alone.
2. WHEN the dashboard renders a recent-run row THE SYSTEM SHALL lead with the project
   name and keep the `projectId` on the row's detail line; WHERE no name resolves it
   SHALL show the `projectId` alone.

### Requirement 6: Time filter on the project detail page

**User Story:** As an engineer reviewing a project, I want to choose the time window, so
that I can widen past the default week without editing a URL.

The v2 endpoint already clamps to the last 7 days when a client sends no bounds, so this
page had an invisible window. These criteria make it explicit and adjustable.

#### Acceptance Criteria

1. THE SYSTEM SHALL offer the same date-range control the test-runs filter sidebar uses:
   the same rolling presets and the same custom from/to inputs.
2. THE SYSTEM SHALL default the project page to the last 7 days, matching the window the
   server previously applied silently.
3. WHEN the user picks a preset or a custom range THE SYSTEM SHALL scope both the run
   list and the test-history chart to it.
4. WHEN the user clears the range THE SYSTEM SHALL request all time by sending
   `allTime=1`, opting out of the server's 7-day default.
5. WHEN the user clears one bound of a custom range THE SYSTEM SHALL drop that bound
   rather than retaining its previous value.

## Out of Scope

- Changing the `project` filter parameter to accept names.
- Any v1 (`internal/api/`) endpoint.
