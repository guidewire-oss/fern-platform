# ADR: Test-to-Requirement Mapping Lifecycle (MVP)

**Status:** Accepted (MVP-scoped)
**Date:** 2026-06-11 (simplified from 2026-05-25 draft)
**Driven by:** Epic #22 — issues #28 (foundation/wire-contract), #29 (Per-Epic view), #30 (Per-Release view)
**Related:** [`tag-schema.md`](./tag-schema.md)
**Supersedes:** the broader original draft including Path B (sync-side backfill) and scenario binding. Those depended on the parked sync subsystem and scenario-level coverage; see `specs/jira-requirements-sync/future/` for the parked design.

## Context

Fern correlates test runs to JIRA issues. With the MVP scoped to JIRA issue-level coverage (no scenario extraction, no JIRA metadata mirror), the question of "when is the mapping written?" collapses to a single answer.

The mapping is the existing `spec_run_tags → tags` join (no new table — see `tag-schema.md`). The question this ADR answers: **at what moment, and by what code, does that join row get written?**

## Decision

**Path A only — at test ingest, via the existing tag flow. No new code, no new path.**

When a `spec_run` is ingested with a `tags: ["jira:GWCP-12345", ...]` array in the JSON payload:

1. The **existing** tag ingest code (in `internal/domains/tags/`) processes the tag string.
2. `domain.NewTag("jira:GWCP-12345")` auto-splits and normalizes → `(name="jira:gwcp-12345", category="jira", value="gwcp-12345")`.
3. The tag is upserted into `tags` (UNIQUE on `name` provides natural dedup).
4. A row is inserted into `spec_run_tags(spec_run_id, tag_id)` with `PRIMARY KEY (spec_run_id, tag_id)` enforcing idempotency.
5. **Done.** No further code runs to "correlate" the test to a JIRA issue — the join row IS the correlation.

That's the entire write path. It already exists in production. The MVP adds zero ingest code.

### Idempotency

`spec_run_tags.PRIMARY KEY (spec_run_id, tag_id)` makes repeated ingestion of the same `(spec_run, tag)` pair a no-op. `INSERT ... ON CONFLICT DO NOTHING` in the existing tag ingest code.

### What the resolvers do at read time

Coverage views (`epicCoverage` in #29, `releaseCoverage` in #30) query JIRA live for the issue tree (Epic → children) and JOIN the result against `spec_run_tags ⨝ tags` filtered by `category='jira'`. Case-folding caveat per `tag-schema.md`.

The join is computed at read time but reads only local indexed tables — no N+1 against external systems. Performance is dominated by the JIRA enumeration call, not the local SQL.

## What's NOT in this MVP (deferred)

The following parts of the original design are parked. They reactivate when scenario-level coverage returns:

- **Path B — sync-side backfill.** Required only when Fern maintains a local mirror of JIRA issues. The MVP has no mirror; every dashboard query enumerates JIRA live. So there's nothing to "backfill" against.
- **Scenario binding via `scenario:` tag.** Required only for scenario-level granularity. MVP correlates at issue level.
- **Confidence + source discriminator** (`tag` / `name_match` / `epic_fallback`). With no scenarios to disambiguate, every binding is "tag" with implicit confidence 1.0. The column isn't needed.
- **Name-match fallback** (`spec_name` similarity against parsed scenarios). No parsed scenarios exist.
- **Pending / deferred-correlation handling.** Without a local issue mirror, there is no "tag stored, no join row yet" state. Every tag becomes a `spec_run_tags` row immediately.
- **Orphaned-requirement handling.** Without a local mirror, there are no orphaned-requirement rows. JIRA issue deletion → that key simply stops appearing in `parent = …` JQL queries → naturally drops from dashboards. The `spec_run_tags` history remains (as a historical record).

See `specs/jira-requirements-sync/future/` and `./future/gherkin-parsing-tiers.md` for the parked design including these capabilities.

## Alternatives considered

- **Query-time computation** (no persistent join, recompute test↔JIRA correlation from raw `spec_run.tags` on every dashboard view). Rejected — PR #156's N+1 pain demonstrates the cost on every coverage view load. The persistent `spec_run_tags` junction IS the materialized index that avoids it.
- **New dedicated `spec_run_requirements` table** keyed by `jira_key` string. Rejected — the existing `tags` + `spec_run_tags` schema (with `category` / `value` columns from migration 000017 and the `idx_tags_category_value` index) already provides exactly this functionality. Adding a new table would duplicate infrastructure.
- **Per-source link tables** (`spec_run_jira`, `spec_run_github`, etc.). Rejected — each new correlation source becomes a migration; ugly cross-source coverage queries. The `tags.category` discriminator scales without schema changes.
- **Path B (sync-side backfill).** Removed entirely for MVP — no sync subsystem exists. Would only be needed if Fern maintained a local JIRA mirror, which it doesn't.

## Consequences

- **Zero new code in the data layer.** The existing tag ingest path handles the write.
- **Mapping latency is essentially zero.** The join row is written in-transaction with the spec_run.
- **Tests can be ingested before their JIRA issues are even visited in a coverage dashboard.** The mapping persists; the dashboard just needs the issue to exist in JIRA at render time.
- **Idempotent re-ingestion.** Existing UNIQUE constraint on `spec_run_tags` makes the write safe under retry.
- **One source of truth for the join.** Every coverage query reads from `spec_run_tags ⨝ tags`. No alternate path, no Path B, no batch reconciliation.
- **JIRA issue deletion in JIRA doesn't break Fern.** The local `spec_run_tags` row stays as historical data. The deleted JIRA key naturally drops out of dashboards because it stops appearing in JIRA tree-enumeration queries.

## Open questions

1. **Stale `jira:` tags that no longer match any JIRA issue.** A test tagged with a typo (`jira:GWCO-12345` vs `jira:GWCP-12345`) or referencing a deleted JIRA issue persists in `tags`/`spec_run_tags` but never shows up in any coverage view (no JIRA enumeration query returns it). Acceptable — it's local noise that doesn't surface to users. Future hygiene job could flag "tags with `category='jira'` whose value never matched any JIRA enumeration in the last 90 days" as candidates for cleanup. **Defer.**
2. **Multiple `jira:` tags per spec_run.** Fully supported — each tag becomes its own `spec_run_tags` row. The same spec_run shows up as covering multiple JIRA issues. Expected behavior.
3. **Case-folding inconsistency** between tag value (lowercased) and JIRA key (uppercase). Handled by resolvers via `LOWER(value)` in the IN clause (or app-side normalization). Documented in `tag-schema.md`.

## References

- Companion ADR: [`tag-schema.md`](./tag-schema.md) — the wire format and tag namespacing rules
- Active spec: `specs/jira-requirements-sync/requirements.md` + `design.md`
- Parked Path B + scenario binding: `specs/jira-requirements-sync/future/`
- Parked Gherkin tiers: `./future/gherkin-parsing-tiers.md`
- Decision rationale: `reviews/jira-sync-persistence-analysis/SUMMARY.md` (dev workspace)
