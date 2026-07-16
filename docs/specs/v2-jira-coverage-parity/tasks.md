# v2 JIRA & Coverage Parity — Tasks

TDD: write the failing test first for each unit of logic, then the
minimum code to pass, then refactor. Run `vitest` + `tsc` per step.
End-to-end verify each feature in the running app before marking done.

## Phase 1 — JIRA field mapping (#26)
- [x] T1.1 Types + `fieldMappingHooks.ts` (`useJiraFieldMapping`,
      `useJiraFields`, `useSaveJiraFieldMapping`, `useResetJiraFieldMapping`)
      via `graphqlFetch`. (tsc + eslint clean)
- [x] T1.2 (test-first) `validateMapping()` + `FERN_FIELDS` in
      `fieldMapping.ts` — required-unmapped, duplicate JIRA field, and
      multi-value-needs-strategy rules. `fieldMapping.test.ts` 7/7 green.
- [x] T1.3 `FieldMappingEditor.tsx` table UI (Fern field → JIRA field
      select, multi-value, reduction strategy). Read-only when
      `!canManage`. (renamed from FieldMapping.tsx — casing clash with
      fieldMapping.ts)
- [x] T1.4 Save/Reset wired to mutations; empty state (FR-4) when no
      connection; per-field validation errors block Save.
- [x] T1.5 Mounted in `ProjectSettings.tsx` Integrations tab; render
      test (`FieldMappingEditor.test.tsx`, 3 cases) with mocked hooks.
- [ ] T1.6 End-to-end verify (view, edit, save, reset, validation) —
      pending redeploy of the built image.

## Phase 2 — Requirement coverage (#29 + #30)
- [x] T2.1 `coverageHooks.ts` (`useJiraFixVersions`,
      `useRequirementCoverage`).
- [x] T2.2 (test-first) `coverage.ts` helpers: `coveragePercent`,
      `treeSummary`, `defaultRelease`. `coverage.test.ts` 7/7.
- [x] T2.3 `CoverageTree.tsx` — epic/story/subtask tree + unassigned
      section + per-epic percent bar + coverage pills.
- [x] T2.4 `CoverageView.tsx` (release selector + summary) + route
      `projects/$projectId/coverage` (lazy) + Coverage link on ProjectDetail.
- [x] T2.5 Empty states: no releases, load error, no requirements.
- [x] T2.6 `CoverageTree.test.tsx` render test (3 cases).
- [ ] T2.7 End-to-end verify against a project with a JIRA connection —
      pending redeploy.

## Phase 1b — Visual (drag+SVG) field-mapping editor (v1 parity)
- [x] `mappingLines.ts` pure geometry (`portCoords`, `bezierPath`) + tests.
- [x] `FieldMappingVisual.tsx` — two columns (Fern | JIRA), SVG connectors,
      drag a JIRA field onto a Fern field to map, ✕ to unmap, searchable
      JIRA column, reduction-strategy select for multi-value→single-value.
      Read-only when `!canManage`; validates via `validateMapping`.
- [x] Replaced the dropdown `FieldMappingEditor` in ProjectSettings with
      the visual editor (old editor + test removed).
- [x] `FieldMappingVisual.test.tsx` render tests (empty/columns/existing
      mapping+validation/read-only). Full web-v2 suite green (109 tests).
- [ ] a11y: drag is pointer-only (matches v1); keyboard mapping path is a
      future enhancement.
- [ ] End-to-end verify against a connected JIRA (needs valid creds).

## Phase 3 — Follow-ups (optional / confirm first)
- [ ] T3.1 Coverage spec-runs drill-in modal (v1 `CoverageSpecRunsModal`)
      — confirm the spec-runs query in schema first.
- [ ] T3.2 Docs: note the new v2 surfaces in the migration guide.

## Cross-cutting
- [ ] C.1 Access: read for project-visible users; write gated on
      `canManage`; non-members blocked (rely on server scoping).
- [ ] C.2 `tsc --noEmit`, `eslint --max-warnings 0`, `vitest` green.
- [ ] C.3 Update `claude-progress.md` and this file as tasks complete.
