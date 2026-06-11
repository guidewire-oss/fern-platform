# Parked: Spec-level coverage design (future iteration)

These files capture the **original**, fuller-scope design for JIRA Requirements Sync — including Gherkin scenario extraction from JIRA descriptions, two-tier parsing (deterministic + LLM), the four-table data model, background sync orchestration, staleness management, and Path B mapping backfill.

## Why parked

After engineering leadership review, the MVP was scoped down to **JIRA issue-level coverage** (issue as unit, not scenario). The full design here was not wrong — it was sized for a richer feature than the MVP needs. Preserving it intact so future-us doesn't have to re-derive it.

See `../requirements.md` / `../design.md` / `../tasks.md` (the active files in the parent directory) for the current MVP scope.

The detailed rationale for the simplification — Option A vs Option B trade-off, OSS scaling, the "we already have a tags table" pruning — lives in `reviews/jira-sync-persistence-analysis/SUMMARY.md` in the dev workspace.

## When to reactivate this

Move these files back up to replace the slim MVP versions when at least one of these triggers fires:

- PMs / VPs ask for **per-scenario drill-down** in coverage views (not just per-issue)
- Test coverage views need to surface specific Gherkin steps tested vs untested
- Customers ask for stability-bounded reads ("show me coverage as of yesterday")
- The MVP's live-JIRA approach starts failing under load (then revisit Option C — sync-light — from the SUMMARY)

## Related parked ADR

`adr/test-correlation/future/gherkin-parsing-tiers.md` — the Tier 1 (deterministic) + Tier 2 (LLM) Gherkin extraction ADR — is parked at the same time and reactivates alongside this spec.

## Files

| File | What it covers |
|---|---|
| `requirements.md` | 11 EARS-style requirements covering initial sync, on-demand sync, staleness, audit, NFRs, sync filters, release window, Gherkin extraction, Path B backfill, inventory page |
| `design.md` | Full architecture: 4-table data model, sync orchestration with goroutine workers, two-tier Gherkin parser, GraphQL surface with polling, error handling, testing pyramid |
| `tasks.md` | 30 tasks across 9 phases — foundation, domain, repos, parser, sync orchestration, GraphQL, UI, acceptance, cleanup |
