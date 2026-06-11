# Requirements: JIRA Requirements Sync

## Introduction

Fern projects connected to JIRA need a local copy of JIRA issue data so coverage views render without hitting JIRA on every page load, and so test runs carrying JIRA tags can be correlated to requirements. This spec covers what is synced, when it is synced, how it is kept current, how Gherkin scenarios are extracted from issue descriptions, and how test-to-requirement mappings are written. The downstream coverage UI (#29, #30) is out of scope; test-side JIRA tagging (#28) is owned separately. Architectural decisions called out below are recorded in three ADRs under `adr/test-correlation/`:

- [`tag-schema.md`](../../adr/test-correlation/tag-schema.md) — wire contract for test-to-requirement tags
- [`mapping-lifecycle.md`](../../adr/test-correlation/mapping-lifecycle.md) — when `spec_run_requirement` rows are written
- [`gherkin-parsing-tiers.md`](../../adr/test-correlation/gherkin-parsing-tiers.md) — Tier 1 deterministic / Tier 2 LLM extraction

## Requirements

### Requirement 1: Initial Bulk Sync

**User Story:** As a manager, I want to populate Fern with the JIRA requirements relevant to my project, so that coverage views work from a complete picture without waiting for individual lookups.

#### Acceptance Criteria

1. WHEN a manager triggers an initial sync from the project's JIRA settings THE SYSTEM SHALL fetch JIRA issues matching the configured sync filters (issue types, JQL, release window — see Requirement 8) using the configured field mapping.
2. WHEN issues are returned from JIRA THE SYSTEM SHALL persist for each issue: JIRA key, summary, issue type, parent JIRA key, status, fix version, description (raw markdown body), and last synced timestamp.
3. WHEN the JIRA result set exceeds one API page THE SYSTEM SHALL paginate through the full result using JIRA REST API pagination, requesting no more than 100 issues per call.
4. WHILE a sync is in progress THE SYSTEM SHALL display progress including total issues, processed count, succeeded count, and failed count.
5. WHEN a sync is started THE SYSTEM SHALL return the sync run identifier asynchronously and continue execution as a background job that survives the user navigating away from the trigger page.
6. WHEN a client polls the sync run status THE SYSTEM SHALL return current progress and a terminal state (`succeeded`, `failed`, `cancelled`) when the run is complete.
7. IF the sync is interrupted (network failure, JIRA unreachable) THEN THE SYSTEM SHALL preserve successfully synced issues and allow the operation to be retried.
8. WHEN the sync completes THE SYSTEM SHALL show a summary listing imported, updated, skipped (unchanged) counts, and the number of test runs newly associated by the mapping backfill pass (see Requirement 10).

### Requirement 2: On-Demand Incremental Sync

**User Story:** As a manager, I want to refresh JIRA data manually before reviewing coverage, so that I see the current state of requirements without waiting for a scheduled job.

#### Acceptance Criteria

1. WHEN a manager triggers an incremental sync THE SYSTEM SHALL fetch only JIRA issues whose `updated` timestamp is newer than the most recent successful sync for that project, scoped further by the project's sync filters.
2. WHEN incremental sync identifies a changed issue THE SYSTEM SHALL update the local record without removing any test-to-requirement associations.
3. IF an issue is no longer returned by JIRA (deleted or moved out of scope) THEN THE SYSTEM SHALL mark the local record as orphaned rather than deleting it, preserving test history.
4. WHEN an incremental sync fails for individual issues THE SYSTEM SHALL continue processing remaining issues and report per-issue failures at the end.

### Requirement 3: Staleness Refresh on Coverage View

**User Story:** As a manager, I want coverage views to reflect reasonably current JIRA data without me clicking sync first, so that I see accurate status when I open a report.

#### Acceptance Criteria

1. WHEN a coverage view is opened for an Epic and the local requirements data for that Epic is older than the configured staleness threshold THE SYSTEM SHALL trigger an incremental refresh scoped to that Epic and its descendants before rendering.
2. WHILE a staleness refresh is in flight THE SYSTEM SHALL render the view from existing local data and indicate that a refresh is in progress.
3. IF the staleness refresh fails THEN THE SYSTEM SHALL render the view from existing local data and surface a non-blocking warning that the data may be out of date.
4. THE SYSTEM SHALL NOT trigger any automatic refresh when test results arrive for issues already present in the local store.

### Requirement 4: Local Requirements Data Model

**User Story:** As a developer of the coverage features, I want a stable local schema for JIRA requirements that supports the Epic → Story → Scenario hierarchy, so that downstream views can query without depending on JIRA availability.

#### Acceptance Criteria

1. THE SYSTEM SHALL store each requirement scoped to a Fern project, with fields: id (Fern-side), JIRA key (nullable for non-JIRA sources), source (`jira_sync` | `parsed` | `llm_extracted`), type (`epic` | `story` | `task` | `bug` | `subtask` | `scenario`), parent id (nullable; references another requirement), title (the issue summary, or the scenario title), status, fix version, description (raw markdown, JIRA-sourced types only), gherkin body (scenarios only), confidence (1.0 for sync/parsed, <1.0 for LLM/name-match), last synced timestamp.
2. THE SYSTEM SHALL enforce uniqueness on (project, JIRA key) for `source = jira_sync` rows.
3. THE SYSTEM SHALL enforce uniqueness on (project, parent id, title) for `source ∈ {parsed, llm_extracted}` rows (scenarios within a parent).
4. WHEN a requirement record is created or updated THE SYSTEM SHALL set the last synced timestamp to the time of the sync operation.
5. WHEN a parent issue's description changes between syncs (content hash comparison) THE SYSTEM SHALL re-run scenario extraction (Requirement 9) and reconcile scenario rows, preserving join rows in `spec_run_requirement` for unchanged scenarios.

### Requirement 5: Sync History and Audit

**User Story:** As a manager, I want a record of past sync operations, so that I can diagnose issues and understand when data was last refreshed.

#### Acceptance Criteria

1. WHEN a sync operation starts THE SYSTEM SHALL record its trigger source (initial, incremental, staleness, lazy-backfill), start time, and the user or system actor that initiated it.
2. WHEN a sync operation completes (success or failure) THE SYSTEM SHALL record completion time, counts (processed, succeeded, failed, mappings created), and any error summary.
3. THE SYSTEM SHALL expose the most recent sync record per project to the manager via the project's JIRA settings view.
4. THE SYSTEM SHALL expose the full sync history for a project, ordered by most recent first, so that managers and operators can diagnose past failures.

### Requirement 6: Non-Functional Requirements

**User Story:** As an operator, I want JIRA sync to be resilient and respectful of JIRA's rate limits, so that one project's sync activity does not destabilise Fern or get the connection throttled.

#### Acceptance Criteria

1. THE SYSTEM SHALL respect JIRA API rate limits, backing off and retrying on `429` and `5xx` responses with exponential delay.
2. WHILE a sync is in progress for a project THE SYSTEM SHALL prevent a second concurrent sync of the same kind for the same project.
3. THE SYSTEM SHALL run long-running syncs as background jobs that survive a user navigating away from the trigger page.
4. THE SYSTEM SHALL encrypt or otherwise protect any JIRA credentials used during sync at rest and in transit.
5. THE SYSTEM SHALL make sync status pollable via a stable GraphQL query so that the UI can render progress without long-polling or WebSocket dependencies.

### Requirement 7: Sync Configuration Filters

**User Story:** As a manager, I want to filter which JIRA issues are synced by issue type and an optional JQL expression, so that Fern only pulls in the issues relevant to my project.

#### Acceptance Criteria

1. WHEN a manager opens the sync configuration dialog THE SYSTEM SHALL present checkboxes for issue types to include (Epic, Story, Task, Bug, Sub-task) with the project's saved default pre-selected.
2. WHEN a manager opens the sync configuration dialog THE SYSTEM SHALL pre-populate the JQL filter field with the project's saved default JQL, if one exists.
3. WHEN a sync is triggered THE SYSTEM SHALL apply the selected issue types as an `issuetype in (...)` clause, the project's release-window constraint (Requirement 8), and the JQL filter as additional AND clauses to all JIRA search queries.
4. WHEN a manager runs a sync with a JQL filter and issue type selection THE SYSTEM SHALL save those values as the new default for the project so subsequent syncs are pre-populated.
5. IF the JQL filter field is left blank THE SYSTEM SHALL sync all issues matching the selected issue types and the release-window constraint with no additional JQL.
6. IF a manager provides an invalid JQL expression THEN THE SYSTEM SHALL surface the JIRA error message before starting the sync.

### Requirement 8: Release Window Configuration

**User Story:** As a manager, I want to bound which JIRA issues and which historical test runs are considered for my project's coverage, so that data volumes stay manageable and only release-relevant work appears.

#### Acceptance Criteria

1. THE SYSTEM SHALL expose a per-project setting `release_window_days` (integer; default 120; min 7; max 1825).
2. WHEN a sync is triggered THE SYSTEM SHALL constrain the JIRA query to issues whose `updated` timestamp is within `release_window_days` of the sync start time, in addition to the JQL filter and issue type filter.
3. WHEN the sync backfill pass runs (Requirement 10, Path B) THE SYSTEM SHALL scan only `test_run` rows whose `created_at` is within `release_window_days` of now.
4. THE SYSTEM SHALL surface the current value of `release_window_days` in the sync configuration dialog with a tooltip explaining both effects (sync scope and backfill scope).

### Requirement 9: Gherkin Scenario Extraction

**User Story:** As a manager, I want each Gherkin scenario in my JIRA Epics and Stories to become a first-class requirement row in Fern, so that test coverage can be tracked at the scenario level, not just the issue level.

Architectural detail in [`gherkin-parsing-tiers.md`](../../adr/test-correlation/gherkin-parsing-tiers.md).

#### Acceptance Criteria

1. WHEN a JIRA issue of type `epic` or `story` is synced AND its description has changed since the last sync (content-hash comparison) THE SYSTEM SHALL run Tier 1 Gherkin extraction on the description.
2. WHEN Tier 1 extraction runs THE SYSTEM SHALL walk the markdown AST, identify fenced code blocks, parse each via the official Cucumber Gherkin library, and persist each resulting scenario as a `requirement` row with `type=scenario`, `source=parsed`, `confidence=1.0`, and `parent_id` set to the nearest preceding heading or to the issue itself if no heading precedes.
3. WHEN Tier 1 extraction encounters a `Scenario Outline` with an `Examples` table THE SYSTEM SHALL persist one scenario row per `Examples` row, named `<base-title>/<row-key>`.
4. WHEN Tier 1 extraction produces zero scenarios for an issue of type `epic` or `story` AND the feature flag `gherkin_llm_extraction.enabled` is true AND an LLM provider is configured THE SYSTEM SHALL invoke Tier 2 LLM extraction.
5. WHEN Tier 2 extraction runs THE SYSTEM SHALL return strict-JSON-shaped scenarios, apply confidence thresholding, and persist accepted candidates with `source=llm_extracted` and `confidence<1.0` into a review queue rather than directly into the live requirement tree.
6. THE SYSTEM SHALL default `gherkin_llm_extraction.enabled` to `false`; activation requires explicit operator configuration.
7. WHEN scenarios extracted from a previously-synced description are no longer present in the latest description THE SYSTEM SHALL mark the corresponding scenario rows as orphaned rather than deleting them, preserving any existing `spec_run_requirement` associations.

### Requirement 10: Test-to-Requirement Mapping (Sync-Side Path B)

**User Story:** As a developer, I want tests that referenced a JIRA key before it was synced to become correlated automatically the moment Fern learns about that key, so that my coverage data is complete without manual reconciliation.

Architectural detail (including Path A, owned by #28's test ingest) in [`mapping-lifecycle.md`](../../adr/test-correlation/mapping-lifecycle.md).

#### Acceptance Criteria

1. WHEN a sync run writes or updates one or more `requirement` rows THE SYSTEM SHALL execute a backfill pass before emitting the final sync summary.
2. WHEN the backfill pass runs THE SYSTEM SHALL scan `spec_run` rows within the project's `release_window_days` whose `tags` contain any `jira:` reference to the just-synced keys, and SHALL create `spec_run_requirement` join rows for each match.
3. THE SYSTEM SHALL apply the same binding logic as Path A (test-ingest): scenario-level when `scenario:` tag matches a parsed scenario; otherwise issue-level via name-match; otherwise epic-fallback. Source and confidence values follow [`mapping-lifecycle.md`](../../adr/test-correlation/mapping-lifecycle.md).
4. THE SYSTEM SHALL enforce a uniqueness constraint on (`spec_run_id`, `requirement_id`) and use `ON CONFLICT DO NOTHING` so that backfill runs are idempotent.
5. THE SYSTEM SHALL include backfill counts (mappings created, mappings skipped due to existing rows) in the sync summary returned to the user.
6. THE SYSTEM SHALL NOT call JIRA from the backfill pass; backfill operates only on data already persisted.

### Requirement 11: Imported Requirements Inventory Page

**User Story:** As a manager, I want a page in Fern that lists the JIRA requirements imported into my project, so that I can verify the sync result and navigate the requirements without leaving Fern. (Rich coverage rollup with passed/failed counts is owned by #29; this page is inventory and sync management only.)

#### Acceptance Criteria

1. WHEN a manager navigates to the project's JIRA Integration tab THE SYSTEM SHALL display a list of `requirement` rows for that project where `type ∈ {epic, story, task, bug, subtask}` (scenarios are not first-class in this page; they are visible on each issue's detail row).
2. THE SYSTEM SHALL allow filtering the list by `type`, `status`, and `fix_version`.
3. WHEN a manager clicks a row THE SYSTEM SHALL expand to show the issue's parsed scenarios (rows with `type=scenario` and `parent_id` matching the row) with their `source` and `confidence` displayed.
4. WHEN a row's source row was last synced THE SYSTEM SHALL display the relative timestamp (e.g., "synced 2 hours ago").
5. THE SYSTEM SHALL expose the JIRA link on each row so the manager can open the source issue in JIRA in one click.
6. THE SYSTEM SHALL display the most recent sync's summary panel and a button to start a new sync, both accessible from this page.

## Constraints

- JIRA REST API pagination limits sync to 100 issues per request; full syncs of large projects must page.
- JIRA credentials and connection details are managed by the existing JIRA connection feature (#23, #24) and are not redefined here.
- Field mapping between JIRA fields and Fern requirement fields is owned by #26 (see `specs/jira-field-mapping/`); this spec consumes the saved mapping.
- Test-side `jira:KEY` tagging on `spec_run.tags` is owned by #28; this spec only consumes the resulting tag on a test run.
- Test ingest mapping (Path A) is owned by #28; this spec implements the sync-side backfill (Path B) of the same mapping lifecycle described in [`mapping-lifecycle.md`](../../adr/test-correlation/mapping-lifecycle.md).
- Coverage UI (#29) and release readiness dashboard (#30) are downstream consumers of the local requirements store and `spec_run_requirement` table; their rendering is not in scope here.
- Webhook updates from Fern back to JIRA (#31) are out of scope.
- The staleness threshold default for Requirement 3 is an open design question.
- The Tier 2 LLM review-queue UX (Requirement 9.5) is an open design question; the requirement guarantees only that LLM-extracted scenarios are quarantined from the live tree until a human accepts them.
