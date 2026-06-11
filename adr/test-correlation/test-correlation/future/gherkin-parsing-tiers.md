# ADR: Gherkin Scenario Extraction — Two-Tier Parsing

**Status:** Proposed
**Date:** 2026-05-25
**Driven by:** Epic #22 — issue #27 (sync, where parsing fires)
**Related:** [`mapping-lifecycle.md`](./mapping-lifecycle.md) (consumes scenario rows), [`tag-schema.md`](./tag-schema.md) (`scenario:` tags reference parsed scenarios)

## Context

Fern needs an Epic → Story → Scenario hierarchy to render PM-relevant coverage views (#29, #30). Scenarios live in JIRA Epic/Story description bodies, typically as Gherkin code blocks (verified against DE-5249, GWCP-97108).

Two non-negotiable constraints compete:

- **OSS contributors** running Fern locally must not be required to obtain an LLM API key or set up a third-party service.
- **Internal Guidewire teams** want a fallback for legacy or less-disciplined PM content where descriptions don't follow strict Gherkin.

Going forward, PMs are committing to a fixed Gherkin format that parses deterministically — but legacy epics and external contributors won't all comply on day one.

## Decision

Two-tier extraction pipeline. Runs during JIRA sync when an issue's description has changed since the last sync (content-hash compared).

### Tier 1 — Deterministic Gherkin parser. Always on, OSS-safe.

1. **Markdown pre-processor.** Walk the description's markdown AST (`goldmark`). Extract every fenced code block AND track the heading hierarchy preceding each block. This builds the Slice / Story containment around scenarios.
2. **Gherkin parser.** Feed each code block to `github.com/cucumber/gherkin/v32/go` — the official, maintained Cucumber implementation. Returns a typed AST: `Feature` → `Scenario` / `ScenarioOutline` + `Examples` → `Step` (with optional `DataTable` / `DocString`).
3. **Persist** as `requirement` rows of type `scenario`, with:
   - `source = "parsed"`, `confidence = 1.0`
   - `parent_id` pointing to the row representing the nearest preceding heading
   - `gherkin_body` storing the raw Given/When/Then text
4. **Scenario Outline handling.** Each row of the `Examples` table becomes its own scenario row (per the tag-schema ADR's open-question resolution). Name: `<base-title>/<row-key>`.

### Tier 2 — LLM-assisted extraction. Opt-in fallback, off by default, OSS-safe.

**Activation conditions — all must hold:**
- Tier 1 produced zero scenarios for the issue.
- Issue type implies scenarios should exist (`Epic`, `Story`).
- Feature flag `gherkin_llm_extraction.enabled = true` for the Fern instance.
- An LLM provider is configured (API key + provider selection).

**Implementation:**
- **Provider-agnostic adapter.** Small interface; supports Anthropic API, OpenAI, AWS Bedrock, and a local-model (Llama-class) provider. The operator chooses one in config.
- **Prompt** — return strict JSON: `[{title, given, when, then}]`, no prose. Few-shot with positive examples drawn from known-good Gherkin.
- **Confidence threshold.** Model's stated confidence × heuristic checks (do the scenarios reference entities mentioned in the description? do they sound like product behavior?). Reject low-confidence output silently.
- **Human-in-the-loop confirmation.** Extracted scenarios land in a "review queue" UI before being added to the requirement tree. Manager can accept / reject / edit each one.
- **Persisted** with `source = "llm_extracted"`, `confidence < 1.0` (the model's adjusted confidence). Coverage queries can distinguish parsed vs LLM-extracted rows.

**Default config:** disabled. Explicit operator action (set the flag + configure provider) to turn on.

### Reporter responsibility (unchanged from tag-schema ADR)

fern-clients are unaffected by which tier produced a scenario row. The wire contract (`jira:KEY` + optional `scenario:TITLE` tags) is the same regardless. Tier-of-origin matters only for UI display ("AI-extracted" badge) and coverage-report annotations.

## Alternatives considered

- **LLM-first (skip Tier 1).** Rejected — cost, latency, hallucination risk, and blocks OSS adoption with a hard API-key requirement. Also wastes compute on the disciplined-Gherkin case.
- **Deterministic only, no LLM tier at all.** Rejected — leaves a real gap for legacy / unstructured descriptions. Guidewire has explicit appetite for the optional tier; building it doesn't impose cost on OSS users.
- **Regex-only `Scenario:` extraction.** Rejected — doesn't handle Scenario Outline, Examples tables, DocStrings, multi-line steps, localized keywords (`Scénario:`, `シナリオ:`). The Cucumber library is the right tool; rolling our own loses fidelity for no gain.
- **Pre-sync description normalization** (force PMs to clean up before sync). Rejected — not Fern's place to gatekeep JIRA content. The PM-format commitment makes Tier 1 succeed; Tier 2 handles the long tail.
- **Single-tier with "best-effort" mode-switching.** Rejected — confuses two orthogonal concerns (deterministic vs probabilistic; free vs costed). Keeping them tiered makes the `source` field meaningful for audit.

## Consequences

### Positive

- **OSS-safe by default.** Tier 1 is free, fast, deterministic, no external dependencies. Most teams never touch Tier 2.
- **Internal-Guidewire option available.** Tier 2 turns on with operator action — no code change required.
- **No vendor lock-in.** Provider-agnostic adapter; can swap models without touching call sites.
- **Auditable.** Every scenario row carries `source` and `confidence`. Coverage reports differentiate parsed vs LLM-extracted; audits can trace any number back to its origin.
- **PM format commitment pays off immediately** — once teams adopt the disciplined format, Tier 2 activations decline; the LLM tier becomes a long-tail safety net rather than a primary path.

### Negative

- **Two code paths to maintain.** Tier 2 adds an adapter, prompt engineering, confidence handling, review UI. Not free.
- **Tier 2 quality requires evaluation discipline.** Prompt regressions, model drift, false-positive scenarios. Need an evaluation fixture suite with known-good extractions to detect regressions on model upgrades.
- **Two sources of truth for "scenarios exist."** Mitigated by the `source` field, but adds a small mental tax for downstream consumers.

### Neutral

- **Re-parse cost.** Tier 1 runs on every sync where the description hash changed; Tier 2 runs only on Tier-1-empty cases. Bounded.
- **Tier transitions.** A description that grows new Gherkin between syncs will see Tier-1 rows added; Tier-2 rows for the same content can be deprecated/marked superseded. Tracked via `source` and a future `superseded_by` link (deferred).

## Open questions

1. **Re-parse triggers.** Recommendation: parse on sync iff the description hash changed since the last sync. Avoids unnecessary parser invocations on no-op syncs. **Confirm before implementation.**
2. **Tier 2 review-queue UX.** Where do "AI suggestions to review" surface — per-project inbox? Notification badge on the sync history page? Defer to #29 design or a dedicated extraction-review sub-issue.
3. **Scenario rename detection across syncs.** A PM rename should update the existing scenario row (preserving join rows in `spec_run_requirement`), not create a new one. Title-based fuzzy match within the same parent's child set, similarity > 0.85 → update; else treat as new. Defer detail to design.md.
4. **Tier 2 cost monitoring.** Operators turning on Tier 2 should see a per-project token/call metric. Defer to design.md; out of scope here.
