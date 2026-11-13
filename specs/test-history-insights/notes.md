# Test history insights (notes, not started)

Ideas for making the project Test history chart useful on real production data. Not yet a
spec: no requirements or tasks agreed.

## What production data looks like

From a production project, last 20 runs, 2026-10-24 to 2026-10-31:

- Every run reports about 500 specs, but passed ranges from about 47 to about 250. Most of
  each run is skipped.
- Runs of different kinds share one chart: development branches running a few Ginkgo specs,
  full-suite runs, and runs against dev and scratch environments. Their counts aren't
  comparable, so the trend line mixes them.
- A couple of runs report close to zero specs (likely aborted or failed during setup) and
  pull the chart to the floor.
- The latest run's branch shows `NA`, so branch and environment metadata isn't always
  reported.

## Chart problems

- The area is smoothed and placed on a time axis. Runs are discrete events, so the slopes
  between points are invented, and a gap of several hours looks like a long decline.
- Bug: the red "Failed" markers sit on top of the passed band. The failed layer has zero
  height, so its markers land on the passed boundary.

## Ideas, highest value first

1. **Separate "skipped" from "not run" and "out of scope".** Ginkgo reports specs excluded
   by `--label-filter` or `--focus` as skipped, so most of the gray is specs that were never
   meant to run, not specs that called `Skip()`. The two can be told apart (see Findings
   below). See "Proposed design" for how this works without letting a partial run count as
   a full pass.
2. **Group runs into series.** Series = environment (dev, scratch) x scope (full suite or
   label filter) x branch type (main or development branch). Show one series at a time,
   defaulting to main-branch full runs on dev, or small multiples. `environment` already
   exists on a run; it needs to be filled in reliably, and branch shouldn't be `NA`.
3. **Change the chart type.** One evenly spaced bar or dot per run (time in the tooltip),
   plotting pass rate of executed specs or failed count. Show aborted runs as a marker
   instead of a data point.
4. **Later: spec-level history.** Which specs flip between passing and failing, and which
   have failed several runs in a row. More useful for triage than per-run counts.

## Findings: Ginkgo and fern-ginkgo-client (checked 2026-10-31, Ginkgo v2.19.0)

- **Filtered-out specs are reported as `skipped` with no failure message.** `--label-filter`,
  `--focus` and `--skip` set `spec.Skip` (`internal/focus.go`), and `group.evaluateSkipStatus`
  turns that into `SpecStateSkipped` with an empty `Failure`.
- **`Skip()` carries a message.** `Failer.Skip` stores the message passed to `Skip()`, and a
  `Skip()` in a `BeforeAll`/`BeforeEach` reports "Spec skipped because Skip() was called
  in ...". Ordered-container skips also carry a message.
- **Ambiguity.** An empty-message skip can also come from an interrupt, a suite timeout,
  or `Skip()` in `BeforeSuite`, not only from filtering. To tell those apart exactly, the
  client can parse `report.SuiteConfig.LabelFilter` (`types.ParseLabelFilter`) and check it,
  plus the focus/skip regexes, against each spec's `Labels()` and text.
- **`Pending` is its own state** and already arrives as `pending`, not `skipped`.
- **The run's scope is in the report.** `report.SuiteConfig` holds `LabelFilter`,
  `FocusStrings`, `SkipStrings` and `FocusFiles`.
- **What the client sends today** (`pkg/client/ginkgo_fern_reporter.go`): each spec's
  `State.String()` and `Failure.Message`, so the server already gets "skipped, empty
  message" vs "skipped, with message". It doesn't send the label filter or focus. The run's
  environment comes from the `TEST_ENVIRONMENT` env var, which explains runs with no
  environment when the pipeline doesn't set it.

So idea 1 is feasible. A rough split is possible from data already stored (skipped with no
message), and an exact one needs the client to send the scope and mark filtered specs.

## Proposed design: skipped, not run, out of scope

Two goals pull in different directions:

- **Integrity.** A user who runs a few specs and leaves out the rest must not get a run that
  says everything passed. So the client must not just drop unselected specs from the report.
- **Noise.** Some projects always leave out a fixed, irrelevant part of the suite, and those
  specs shouldn't fill the UI. Example: ccs-atmos-tests reports to two Fern projects,
  Observability and Atmos NG acceptance. Each runs only its own part of the suite, so each
  shows the other's specs as a big gray block.

Three categories instead of one "skipped":

| Category | Meaning | Counted | Shown |
|---|---|---|---|
| Skipped | Spec called `Skip()` | Yes, as today | Yes |
| Not run | In the project's scope, but excluded by this run's label filter or focus | Yes | Yes, clearly |
| Out of scope | Outside the project's scope | No | Hidden; at most a collapsed count on run detail |

A run that passes everything it executed but has any "not run" specs gets the new run status
**partial**, never `passed`.

**Scope is a project setting in Fern: the project's standard label filter.** It's the
Ginkgo label filter the project's standard CI run uses, set by the project owner or an admin
and evaluated against each spec's labels. A run using it is a full run; a run with a narrower
filter leaves in-scope specs unrun and is partial. It must not come from the reporting
client: otherwise a user running a subset could declare the rest out of scope and get a clean
pass again.

The ccs-atmos-tests filters as of 2026-11-13:

- Observability: `lhs_tenant_log_metrics || apm_tenant_with_tag_based_routing ||
  apm_ops_with_tag_based_routing || llm-traces-lhs || rds || lhs_otel || lhs_audit_logs_s3 ||
  lhs_audit_logs_s3_readback` (the last two added 2026-11-13).
- Atmos NG acceptance: `testtype:acceptance && !component:oort && !component:argo-workflows &&
  !component:argoworkflows`. Observability specs carry no `testtype` label, so this filter
  already excludes them; no separate exclusion list is needed.

Rules that keep this safe as the filter changes:

- **Scope only classifies specs that didn't run.** A spec that executed (passed, failed) always
  counts, whatever its labels, so a stale setting can never hide a failure. At worst an
  unselected spec with a new label is hidden instead of shown as "not run".
- **Classify at ingest, against the scope in force then, and store the result.** Applying
  today's scope to old runs would mark runs from before a label was added as partial. Backfill
  old runs only on request.
- **Evaluate with Ginkgo's own parser** (`types.ParseLabelFilter`) so `key:value` labels and
  `!` exclusions behave exactly as in CI.
- **Flag drift.** The client sends each run's label filter; when it differs from the project's
  standard filter, say so on the run, and offer to update the standard when the run's filter
  is broader. The filter lives in both TeamCity and Fern, so this keeps them from silently
  diverging.

Why this works with data Fern already has:

- fern-ginkgo-client sends spec labels as spec tags, so the server can decide in or out of
  scope without the client knowing about scope.
- Spec tags and skip messages are stored, so existing runs could be reclassified, not just
  new ones (except runs reported before the client sent labels).

Work split:

- **fern-ginkgo-client:** mark unselected specs (empty-message skip, no suite-level
  interrupt, timeout or `BeforeSuite` skip in `SpecialSuiteFailureReasons`), and send the run's
  `LabelFilter` and focus strings as run metadata.
- **fern-platform:** project scope setting; "not run" and "out of scope" counts; `partial` run
  status; out-of-scope specs hidden from the history chart and the run tables.

### Decisions

- **Scope is per project** (user, 2026-11-13). One scope per Fern project is enough. Separate
  Fern projects already cover the case of one repo with several kinds of run, as with
  ccs-atmos-tests (Observability, Atmos NG acceptance).
- **`partial` is a new run status** (user, 2026-11-13), not `passed` with a badge. Anything that
  filters or counts on `status=passed` must handle it: the Test runs status filter, the project
  history, saved views, and the Jira release-readiness checks.

## Open questions

- Do ccs-atmos-tests specs carry consistent labels for each project's part? The scope
  expression only works if they do.
- Backfill: which existing runs, if any, to classify, and against which scope.
- Do we use the empty-message split on existing data as a stopgap, accepting that
  interrupted or timed-out runs count as "not run"?
- What makes up a series key, and where does scope come from if the client can't send it
  (for example, older clients)?

## Related

Ideas 1 and 2 add environment and scope filters to the project run table. That is the
trigger in `specs/table-sort-filter/design.md` for moving that table to `FilterSidebar`.
