# Tasks: Project Name in Test Runs List and Filter

Each task is red → green → refactor. Run the relevant suite after every step.

- [x] **1. Domain fields**
  - Add `ProjectName string \`json:"project_name"\`` to `domain.TestRun`.
  - Add `Label string` to `domain.FacetCount`.
  - _Requirements: 1.1, 2.1_

- [x] **2. Project name resolver (infrastructure)**
  - Test first: `infrastructure/project_name_repo_test.go` — batched lookup returns a
    map, de-duplicates repeated IDs, returns an empty map (and issues no query) for an
    empty input, and omits IDs with no `project_details` row.
  - Implement `ProjectNameRepo.NamesByProjectID` as one `SELECT project_id, name FROM
    project_details WHERE project_id IN (?)`.
  - _Requirements: 1.2, 1.3, 1.4_

- [x] **3. Enrichment in the application service**
  - Test first: `application/test_run_query_service_test.go` — edges get
    `ProjectName`; `Facets.ByProject` entries get `Label`; IDs that appear only in the
    facet (not on the page) are still resolved; a resolver error leaves the page intact
    with empty names; a service with no resolver behaves as before.
  - Add the `ProjectNameResolver` port and `WithProjectNames` builder; collect the
    distinct IDs from edges + `ByProject`, resolve once, apply after facets attach.
  - _Requirements: 1.1–1.5, 2.1–2.3_

- [x] **4. Handler DTO**
  - Test first: `api/v2/test_run_handler_test.go` — `project_name` appears on a node;
    `label` appears on a project facet entry and is absent from status/branch/tag
    entries.
  - Add `Label string \`json:"label,omitempty"\`` to `facetCountDTO` and pass it through
    `toFacetDTOs`.
  - _Requirements: 1.1, 2.1, 2.3, 2.4_

- [x] **5. Production wiring**
  - Chain `.WithProjectNames(infrastructure.NewProjectNameRepo(db))` where the v2
    query service is constructed.
  - _Requirements: 1.1_

- [x] **6. Frontend types**
  - `TestRunNode.project_name?: string`; `FacetCount.label?: string`.
  - _Requirements: 1.1, 2.1_

- [x] **7. Test Runs table column**
  - Test first: `TestRunsList.test.tsx` — renders the name; falls back to the ID when
    `project_name` is absent; puts the ID in the cell's `title`; keeps the
    `/projects/$projectId` link target.
  - _Requirements: 1.6, 1.7, 1.8_

- [x] **8. Project filter facet**
  - Test first: `FilterSidebar.test.tsx` — the Project section shows labels, sorts by
    displayed text, falls back to the ID when a label is missing, and toggling emits
    the `value` (ID).
  - Change `FacetGroup` to take `FacetCount[]` and render `label ?? value`; add
    `sortFacetsByLabel`; keep the branch facet's priority ordering.
  - _Requirements: 2.5, 2.6, 2.7, 2.8_

- [x] **9. Refactor and dedupe pass** — extracted `LabeledValue` + `facetSort`
  helpers shared by the table, the facet, and the detail header.

- [x] **10. End-to-end verification** — verified against the seeded local stack with a
  real browser: list shows name over id, detail header matches, Duration populated,
  clicking a named facet issues `?project=flux-1-001`, and one ~1ms `project_details`
  query per list request.

- [x] **11. Duration passthrough (Requirement 3, found during implementation)**
  - Test first: repo carries `duration_ms` (including for a run with no `end_time`);
    handler emits `duration_ms`; the table prefers it and falls back to the timestamps.
  - Set `Duration` in `toDomain`; wrap the node in `nodeDTO` to emit milliseconds;
    add `runDuration()` in `TestRunsList`.
  - _Requirements: 3.1–3.5_

- [x] **12. Cache-safety fix (review finding)**
  - Test first: the cached facet set is not mutated; a cache hit with a failing
    resolver serves no stale labels.
  - Copy `ByProject` before writing labels.
  - _Requirements: 2.1–2.3_

- [x] **13. Zero-duration fix (review finding)**
  - `duration_ms` has no `omitempty`, so an unrecorded duration arrives as `0`.
    Treat zero as "not recorded" and fall back rather than rendering `0ms`.
  - _Requirements: 3.3–3.5_

- [x] **14. Project name on the detail page**
  - Add nullable `projectName` to the GraphQL `TestRun`, regenerate, populate via
    `attachProjectName` in both run query paths, render with `LabeledValue`.
  - _Requirements: 4.1–4.5_

- [x] **15. Project name on the project detail page and dashboard**
  - Test first: header leads with the name and keeps the id; dashboard rows lead with
    the name and keep the id on the detail line; both fall back to the id alone.
  - New `useProject` hook reads the existing `projectByProjectId` GraphQL query; the
    dashboard reads `project_name` already present in the v2 payload.
  - _Requirements: 5.1, 5.2_

- [x] **16. Shared date-range control and project time filter**
  - Extract the sidebar's date section into `DateRangeFilter`; move `DATE_PRESETS`,
    `presetRange`, and `matchPresetDays` into `dateRange.ts`; use the component on both
    the sidebar and the project page.
  - Test first: default 7-day window, preset switching, `allTime=1` on clear, project
    filter preserved, and clearing a single custom bound.
  - _Requirements: 6.1–6.5_
