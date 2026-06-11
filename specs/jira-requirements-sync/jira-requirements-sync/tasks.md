# JIRA-Level Test Coverage — Tasks (MVP)

**Status:** Active — scoped down from the original 30-task / 9-phase plan (parked in `./future/tasks.md`).
**Spec:** `./requirements.md`, `./design.md`.
**Tracking issues:** #28 (foundation/docs), #29 (Per-Epic view), #30 (Per-Release view).

---

## Phase 0 — ADR + wire-contract docs (issue #28)

The "foundation" work that survives the MVP simplification. Mostly documentation; the data layer and ingest path already exist.

### Task 0.1 — Simplify `adr/test-correlation/tag-schema.md`

- **Owner:** docs / architect
- **Change:** remove `scenario:` namespace section (deferred per `./future/`). Add a note describing the existing `domain.NewTag()` auto-split + case-folding behavior. Document the case-fold caveat for downstream consumers.
- **Outcome:** ADR reflects MVP-only contract (`jira:<KEY>` is the sole namespaced tag in scope).

### Task 0.2 — Simplify `adr/test-correlation/mapping-lifecycle.md`

- **Owner:** docs / architect
- **Change:** Path A only. Strip Path B (sync-side backfill — depended on the parked sync subsystem) and any scenario-binding references. Note that ingest writes happen via the existing tag flow, not new code.
- **Outcome:** ADR reflects MVP-only lifecycle (write at ingest, read at dashboard render).

### Task 0.3 — Write developer-facing wire-contract doc

- **Owner:** docs
- **Path:** `docs/coverage/jira-tagging-wire-contract.md`
- **Content:**
  - The contract: `tags: ["jira:<JIRA-KEY>", ...]` in the spec_run JSON payload
  - How Fern stores it (auto-split, lowercased — link to the simplified tag-schema ADR)
  - How dashboards use it (#29, #30 link)
  - Per-language adoption guidance for fern-clients reporter teams (Go-ginkgo, JS-jest, Python-pytest, JUnit, Cucumber) — short table, one row per language, with the right syntax
- **Outcome:** A reporter team in fern-clients can read this doc and ship the tag-emission change for their language without further coordination.

### Task 0.4 — Coordination tracking for fern-clients

- **Owner:** PM / tech lead
- **Format:** GitHub issue checklist on #28 OR separate issues in fern-clients repos
- **Items:** one row per reporter library:
  - [ ] fern-ginkgo-client emits `jira:<KEY>` when test carries `Label("jira:<KEY>")` (or equivalent)
  - [ ] fern-junit-client emits the tag from `@Tag("jira:<KEY>")`
  - [ ] fern-jest-client emits the tag from `test('...', { tags: ['jira:<KEY>'] })`
  - [ ] fern-pytest-client emits the tag from `@pytest.mark.jira("<KEY>")`
  - [ ] fern-cucumber-client emits the tag from `@jira:<KEY>`
- **Outcome:** progress visibility on per-language adoption — gates #29/#30's ability to render real data.

**Acceptance for Phase 0:** ADRs updated, wire-contract doc published, fern-clients coordination tracking in place. Closes #28.

---

## Phase 1 — Per-Epic resolver + view (issue #29 — owner: cwrm56)

**Status:** in flight on `feature/29-jira-coverage-hierarchy`. TDD foundations already committed (`ff2c5dbb`). The tasks below are owned by cwrm56's branch; this spec only mirrors them for visibility and to keep #30's dependency story clear.

The tasks tracked on cwrm56's branch include (paraphrased):

- 1.1 — JIRA client extensions: `SearchIssues(jql, fields)` with pagination + `GetVersions(projectKey)` ✅ scaffolded
- 1.2 — Mock JIRA server for acceptance tests ✅ scaffolded
- 1.3 — `tags` domain: `coverage.go` for the join + `GetJiraTagCoverageByProject` ⏳ in progress
- 1.4 — GraphQL schema: `JiraVersion`, `JiraIssueSummary`, `TestRunCoverage`, `StoryCoverageNode`, `EpicCoverageNode`, `RequirementCoverageTree`, `jiraFixVersions`, `requirementCoverage` ✅ scaffolded
- 1.5 — `requirementCoverage` resolver: per the flow in `design.md`
- 1.6 — Unit + acceptance tests
- 1.7 — Per-Epic UI page at `/projects/:projectId/epics/:epicKey`

### Integration tasks owned by this branch (the user's #30 branch)

These exist to keep #30 unblocked and ensure clean integration with cwrm56's work:

#### Task 1.A — Review cwrm56's `RequirementCoverageTree` shape

- **Owner:** bsekar
- **What:** Once cwrm56 opens a draft PR, confirm the type shapes match what #30 needs to consume. Specifically: `jiraFixVersions` query is callable from #30's picker, and the `EpicCoverageNode` shape is reused inside #30's `RequirementCoverageTree` returns (if we expose drill via GraphQL rather than UI nav).
- **Outcome:** sign-off comment on the PR; no merge conflicts when #30 lands.

#### Task 1.B — Ensure the UNION-of-junctions SQL pattern lands in `GetJiraTagCoverageByProject`

- **Owner:** bsekar (review) / cwrm56 (implement)
- **What:** cwrm56's current SQL (per his branch's `tag_repository.go`) joins only `test_run_tags` + `test_runs`. The MVP needs UNION across `test_run_tags`, `suite_run_tags`, and `spec_run_tags` per the design `Coverage Join` section. Either cwrm56 extends the SQL on his branch, or we follow up with a thin PR that does.
- **Outcome:** repository method returns coverage counts that correctly reflect tags attached at any of the three granularities.

#### Task 1.C — Verify case-fold works end-to-end

- **Owner:** shared
- **What:** mock test in cwrm56's acceptance suite that ingests a spec_run with `jira:GWCP-1` (uppercase), then queries `requirementCoverage` with the same Epic key (uppercase). Asserts the count = 1.
- **Outcome:** confirmation that the UI works regardless of which case JIRA returns or which case the tag arrives in.

**Acceptance for Phase 1:** cwrm56's PR merges with `requirementCoverage` + `jiraFixVersions` resolvers green, UNION-of-junctions SQL in place, and the case-fold acceptance test passing. Closes #29.

---

## Phase 2 — Per-Release resolver + dashboard (issue #30 — owner: bsekar)

### Task 2.1 — Add `releaseCoverage` GraphQL schema + types

- **File:** `internal/reporter/graphql/schema.graphql` (or the equivalent in cwrm56's chosen location after his PR lands)
- **Change:** add `EpicCoverageSummary`, `ReleaseCoverage`, and the `releaseCoverage(projectId, fixVersionName)` query field. The `JiraVersion`, `JiraIssueSummary` types already exist (from #29). Regenerate via the project's gqlgen flow.
- **Outcome:** server compiles; new types resolvable as scaffolds.

### Task 2.2 — Implement `releaseCoverage` resolver

- **File:** alongside or in the same package as #29's resolver (e.g., `internal/reporter/graphql/coverage.resolvers.go` per cwrm56's layout)
- **Change:** Per design.md flow — see "Resolver flow detail → `releaseCoverage`":
  1. Authorize
  2. JIRA JQL #1: `project = <projectKey> AND issuetype = Epic AND fixVersion = "<fixVersionName>"` → epic list (paginated)
  3. JIRA JQL #2: `parent IN (<epicKey1>, ..., <epicKeyN>)` → all children (paginated)
  4. ONE call to `GetJiraTagCoverageByProject(projectID, allChildKeys)` — covers the whole release in one SQL
  5. In-memory aggregation: per-Epic counts + release-level rollup
  6. Return `ReleaseCoverage`
- **Outcome:** unit tests with fixture pass.

### Task 2.3 — Unit + acceptance tests for `releaseCoverage`

- **Files:** new test alongside #29's tests; reuses cwrm56's mock JIRA server
- **Cases:**
  - Release with 0 epics → empty, no error
  - Release with epics that have 0 children → epics included with 0/0 counts
  - Release with 200 children across the epic set → pagination tested
  - Mixed coverage at the release level (some epics fully covered, some partially, some uncovered)
  - Case-fold consistency
  - JIRA unavailable → `JIRA_UNAVAILABLE` error
  - Authorization
- **Outcome:** all green; CI passes.

### Task 2.4 — Per-Release UI page

- **Files:** `web/index.html` + relevant components
- **Change:** Page route `/projects/:projectId/coverage`. Renders:
  - Fix-version picker (calls `jiraFixVersions`, reuses cwrm56's data) — groups unreleased first / released below, searchable
  - Once a version is selected: calls `releaseCoverage(projectId, fixVersionName)`
  - Header: release name, total counts, coverage % progress bar
  - Epic cards in a responsive grid: Epic key + title + mini rollup
  - Each Epic card is a link to `/projects/:projectId/epics/:epicKey` (cwrm56's #29 view)
- **Outcome:** dashboard renders for PALISADES-scale releases within latency budget (P95 < 5s).

### Task 2.5 — Smoke test for #30 against a real release

- **Mechanism:** Ginkgo test gated by `RUN_LIVE_JIRA_SMOKE=1`
- **Targets:** a known release in GW JIRA at PALISADES scale (≈40 epics, ≈800 children)
- **Outcome:** confirmed latency within budget; documented in #30 comment.

**Acceptance for Phase 2:** `releaseCoverage` resolver shipped, dashboard renders for a real release, drill-into-Epic via cwrm56's view works. Closes #30.

---

## Phase 3 — Cross-cut validation

### Task 3.1 — Smoke test against real GW JIRA

- **Mechanism:** Ginkgo test gated by `RUN_LIVE_JIRA_SMOKE=1` env var so it stays out of normal CI
- **Cases:**
  - `epicCoverage` against a known Epic (e.g., GWCP-86523 with ~178 children)
  - `releaseCoverage` against a known release (e.g., the current MVP-scope release at the time of execution)
- **Outcome:** confirms live JQL pagination, real auth, real performance within budget.

### Task 3.2 — Performance verification

- **Mechanism:** measure dashboard render time using browser dev tools + server logs against real-scale data
- **Target:** P50 < 3 s for a release with ~40 epics and ~800 children
- **Outcome:** numbers documented in a comment on #30 for future reference.

### Task 3.3 — Pre-PR review pass

- **Cleanup pass:** any duplicated logic across `epicCoverage` and `releaseCoverage` extracted to a helper (e.g., shared "build coverage map from child keys" function)
- **Test pass:** all unit + acceptance green; smoke test passes
- **Doc pass:** ADRs updated, wire-contract doc in place, README links work

---

## Out of scope (deferred / for future iterations)

- Scenario-level (sub-issue) coverage — `./future/` carries the full design
- Background sync of JIRA metadata into Fern — not needed (live JIRA)
- Gherkin parsing / LLM extraction — `./future/`
- Trend charts / projections — requires historical snapshots (post-MVP)
- Risk heat maps — post-MVP
- Custom-field-based rollups (pod, component) using #26 mapping — post-MVP enhancement
- Webhook back to JIRA on test failures — tracked separately in #31

---

## Dependency graph

```
Task 0.1 (tag-schema ADR)   ───┐
Task 0.2 (mapping ADR)      ───┼──→ Task 0.3 (wire-contract doc) ──→ Task 0.4 (coordination)
                               │
                               ▼
                          (issue #28 done)
                               │
                               ▼
       ┌───────────────────────┴───────────────────────┐
       ▼ owner: cwrm56                                 ▼ owner: bsekar
  feature/29-jira-coverage-hierarchy             waits for #29 PR
       ▼
  Phase 1 tasks (cwrm56's branch — mirrored above)
       ▼
  Task 1.A/B/C (integration review by bsekar)
       ▼
  (issue #29 done)                                    │
                                                      ▼
                                              Task 2.1 (schema additions for release types)
                                                      ▼
                                              Task 2.2 (releaseCoverage resolver)
                                                      ▼
                                              Task 2.3 (tests)
                                                      ▼
                                              Task 2.4 (release dashboard UI)
                                                      ▼
                                              Task 2.5 (smoke test)
                                                      ▼
                                              (issue #30 done)
                                                      │
       ┌──────────────────────────────────────────────┘
       ▼
Task 3.1 (cross-cut smoke), 3.2 (perf), 3.3 (pre-PR)
       │
       ▼
   MVP shipped
```

#30 has a soft dependency on #29: the picker + types are defined on cwrm56's branch. #30 can scaffold its resolver in parallel using stub types, but final integration needs #29's GraphQL surface merged or rebased onto the #30 branch.

If cwrm56's #29 PR is in review when #30 starts, the cleanest approach is to branch #30 off the #29 branch (not main) so the shared types are available without rebasing.

---

## Completion criteria

- [ ] ADRs simplified and published
- [ ] Wire-contract doc published; fern-clients coordination in place
- [ ] `epicCoverage` resolver + UI shipped; unit + acceptance tests green
- [ ] `releaseCoverage` resolver + UI shipped; unit + acceptance tests green
- [ ] Smoke test passes against real GW JIRA at PALISADES-scale
- [ ] Performance within budget (documented in #30)
- [ ] Pre-PR review pass complete; ready to merge

Issues #28, #29, #30 all closed.
