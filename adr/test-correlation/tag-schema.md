# ADR: Test-to-Requirement Correlation Tag Schema

**Status:** Accepted (MVP-scoped)
**Date:** 2026-06-11 (simplified from 2026-05-21 draft)
**Driven by:** Epic #22 — issues #28 (foundation/wire-contract), #29 (Per-Epic view), #30 (Per-Release view)
**Supersedes:** the broader original draft including `scenario:` and `requirement:` namespaces. The `scenario:`-related parts moved to `./future/` along with the spec-level coverage design (see `specs/jira-requirements-sync/future/`).

## Context

Fern correlates `spec_run` rows to the JIRA issues they verify. For the MVP, the unit of correlation is the **JIRA issue itself** (Epic, Story, Task, Bug) — not Gherkin scenarios within issue descriptions. The scenario-level work is parked.

Two facts shape the design:

- **Tests in different languages emit different native annotations** — Ginkgo `Label("...")`, Cucumber `@jira:KEY`, pytest `@pytest.mark.jira(...)`, JUnit XML attributes, Jest `tags: [...]`. Accepting each native form server-side makes ingest N parsers wide.
- **Fern-platform receives one normalized wire shape over REST** — `spec_run.tags: []string` in the JSON payload. The fern-clients reporter libraries (per language) own translating their framework's native form into this wire format.

This ADR defines the **normalized on-the-wire tag contract**. Per-framework extraction is each reporter library's responsibility.

## Decision

A `spec_run.tags` value is a string in one of two forms:

- **Free-form** — no `:` in the first 16 chars. Informational only.
- **Namespaced** — `<prefix>:<value>`. Both prefix and value are **lowercased** during normalization (see "Ingest behavior" below). Multiple tags per prefix allowed.

### Reserved prefixes (MVP scope)

| Prefix | Value | Semantics | Cardinality per spec_run |
|---|---|---|---|
| `jira` | JIRA issue key (e.g., `GWCP-97108`) | Correlation target | Many |
| `release` | Release identifier (e.g., `2026.Bolinas`) | Release context. **Prefer the `test_run.target_release` column over this tag.** | 0..1 |
| `coverage` | Category (`acceptance`, `unit`, `smoke`, …) | Filter / dashboard grouping | Many |

Unknown prefixes are accepted and stored without server-side semantics — adopters can add private namespaces (`team:`, `owner:`) without coordination.

**Deferred (in `./future/gherkin-parsing-tiers.md`):**
- `scenario:<TITLE>` — used by the parked scenario-level coverage design to disambiguate which scenario under a sibling `jira:` tag a test exercises.
- `requirement:<source>:<key>` — forward-compatible namespace for non-JIRA correlation sources.

These are not used by the MVP. Reactivate them when the parked design returns.

### Ingest behavior (existing, no new code needed)

Fern's existing `domain.NewTag(name)` already handles the normalization (see `internal/domains/tags/domain/tag.go`):

```go
normalizedName := strings.TrimSpace(strings.ToLower(name))
if idx := strings.Index(normalizedName, ":"); idx == -1 {
    value = normalizedName
} else {
    category = strings.TrimSpace(normalizedName[:idx])
    value = strings.TrimSpace(normalizedName[idx+1:])
}
```

A tag string `"jira:GWCP-12345"` arriving in a `spec_run.tags` array persists as:

```
tags(name="jira:gwcp-12345", category="jira", value="gwcp-12345")
spec_run_tags(spec_run_id=…, tag_id=<tags.id>)
```

The `spec_run_tags` row is the test-to-issue correlation. `UNIQUE (spec_run_id, tag_id)` makes repeated ingestion idempotent.

### Resolver case-folding caveat

Because `domain.NewTag` lowercases everything, JIRA-returned keys (which are uppercase canonically) must be lowercased before joining:

```sql
WHERE t.category = 'jira'
  AND LOWER(t.value) IN (LOWER('GWCP-12345'), LOWER('GWCP-12346'), ...)
  AND t.deleted_at IS NULL
```

Or normalize JIRA's keys to lowercase on the application side. Either approach is acceptable; both must match case.

### Binding rule (simplified for MVP)

For each `jira:KEY` tag on a spec_run:

1. The existing tag ingest creates / finds the `tags` row with `category="jira"`, `value="<key-lowercased>"`.
2. The existing tag ingest writes the `spec_run_tags` junction row.
3. **That's it.** Coverage queries at dashboard render time discover the binding by JOIN.

No `confidence` field, no `source` discriminator, no `name_match` heuristic, no `epic_fallback`. These belonged to the scenario-level binding logic, which is deferred.

### Reporter responsibility

Each fern-clients reporter library normalizes its framework's native annotation to the wire format. Per-language adoption is tracked in fern-clients repos (not Fern-platform):

| Framework | Native | Normalized → wire form |
|---|---|---|
| Ginkgo | `Label("jira:GWCP-97108")` | `"jira:GWCP-97108"` |
| Cucumber / Godog | `@jira:GWCP-97108` | `"jira:GWCP-97108"` |
| pytest | `@pytest.mark.jira("GWCP-97108")` | `"jira:GWCP-97108"` |
| JUnit XML | `<property name="jira" value="GWCP-97108"/>` | `"jira:GWCP-97108"` |
| Jest | `test('...', { tags: ['jira:GWCP-97108'] })` | `"jira:GWCP-97108"` |

Fern-platform never sees the left column — only the right.

## Alternatives considered

- **Typed columns per source on `spec_run`** (`jira_keys`, `github_issue_keys`) — rejected: each new source is a migration; doesn't compose with cross-source coverage queries.
- **Per-source link tables** — rejected: write-amplification at ingest, ugly cross-source coverage queries.
- **Server-side parsing of `spec_run.spec_name`** for tags (no explicit tag) — rejected: brittle; can't disambiguate multiple issues.
- **JSON-shaped tags** (`{"prefix":"jira","value":"KEY"}`) — rejected: requires migrating `spec_run.tags` from `[]string`, breaks every existing reporter, no grep-ability.
- **Server-side multi-format parser** (Ginkgo/Cucumber/JUnit native syntax decoded by Fern-platform) — rejected: layer violation. Fern-platform receives normalized JSON over REST; framework decoding is the reporter library's job.

## Consequences

- **Tool-agnostic ingest** — one shape across frameworks; one normalization rule on the server.
- **Zero new infrastructure** — the existing `tags` + `spec_run_tags` schema (migrations 000004, 000017, 000019) already stores this contract.
- **Extensible** — adding a new reserved prefix later requires only this ADR to be amended; no schema migration.
- **Two semantic layers in one field** — `spec_run.tags` now mixes informational + load-bearing; the prefix convention makes the split mechanical.
- **The `Tags/Labels` mapping in PR #188** (JIRA field-mapping config) maps JIRA `labels` → `Requirement.Tags`, not to `spec_run.tags`. Two different `tags` concepts; documented here so contributors don't conflate them.

## Open questions

1. **`release:` tag vs `test_run.target_release` column** — column preferred. Tag form retained for reporters that can't set the column directly.
2. **Mixed-case JIRA keys from reporters** — fully fine; ingest lowercases. Resolvers case-fold during the join.
3. **Other reserved prefixes worth defining now** (`owner:`, `team:`, `slo:`, `priority:`) — defer until they have load-bearing Fern semantics. Current three (`jira`, `release`, `coverage`) are enough for MVP.

## References

- Active spec: `specs/jira-requirements-sync/requirements.md` + `design.md`
- Companion ADR: `mapping-lifecycle.md` (when the binding write happens)
- Parked design: `specs/jira-requirements-sync/future/`
- Parked Gherkin-tier ADR: `./future/gherkin-parsing-tiers.md`
- Decision rationale: `reviews/jira-sync-persistence-analysis/SUMMARY.md` (dev workspace)
