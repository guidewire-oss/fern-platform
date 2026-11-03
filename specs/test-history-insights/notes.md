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

1. **Separate "skipped" from "not selected".** Ginkgo reports specs excluded by
   `--label-filter` or `--focus` as skipped, so most of the gray is specs that were never
   meant to run, not specs that called `Skip()`. The two can be told apart (see Findings
   below), so fern-ginkgo-client can send the run's scope and mark filtered-out specs. The
   chart would then show only executed specs. Needs a client change plus a small schema
   addition.
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

## Open questions

- Do we use the empty-message split on existing data as a stopgap, accepting that
  interrupted or timed-out runs count as "not selected"?
- What makes up a series key, and where does scope come from if the client can't send it
  (for example, older clients)?

## Related

Ideas 1 and 2 add environment and scope filters to the project run table. That is the
trigger in `specs/table-sort-filter/design.md` for moving that table to `FilterSidebar`.
