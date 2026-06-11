# Parked ADRs (future iteration)

ADRs in this directory were written for the original scenario-level coverage design (see `specs/jira-requirements-sync/future/`). They are not relevant to the JIRA-level coverage MVP and are preserved for reactivation if/when scenario-level coverage returns as a roadmap item.

See `../tag-schema.md` and `../mapping-lifecycle.md` (active in the parent directory) for the simplified ADRs that apply to the MVP.

## Files

| File | What it covers | Why parked |
|---|---|---|
| `gherkin-parsing-tiers.md` | Tier 1 (deterministic `cucumber/gherkin` parser over markdown-AST-extracted code blocks) + Tier 2 (opt-in LLM extraction with provider-agnostic adapter and human-in-the-loop review queue) | The MVP does not extract scenarios at all. Tests are tagged at the JIRA issue level (`jira:<KEY>`); there is no scenario unit to map to. Reactivate when scenario-level coverage returns. |
