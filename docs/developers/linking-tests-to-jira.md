# Linking Tests to JIRA (Release Coverage)

This guide answers the most common question from teams adopting Fern:

> **"How do I start tagging my tests with JIRA so I can see release coverage?"**

The short version: **tag each test with a `jira:<ISSUE-KEY>` label**, report the run to
Fern as usual, connect your project to JIRA, and pick a release. Coverage lights up.

---

## The end‑to‑end flow

```
 1. Tag the test          →  jira:GWCP-1234   (your framework's label/tag idiom)
 2. Report the run        →  the Fern reporter sends spec_run.tags: ["jira:GWCP-1234"]
 3. Connect JIRA          →  Project → Integrations → add JIRA connection
 4. Map the release field →  Field Mapping → "Release Version" → your JIRA field
 5. View coverage         →  Project → Coverage → pick a release
```

Coverage is computed by matching the `jira:` tags on your ingested test runs against the
issue tree (release → epics → stories) that Fern reads live from JIRA. A story shows as
**covered** when at least one ingested test run is tagged with its issue key; **passing**
when those runs pass, **failing** when any fail.

---

## Step 1 — Tag your tests

The unit of correlation is the **JIRA issue key** (Story, Bug, Task…). Add it as a
`jira:<KEY>` label using your framework's native idiom. Each Fern reporter normalizes that
to the wire form `jira:<KEY>` — you don't emit raw JSON yourself.

| Framework | How you tag it |
|---|---|
| **Ginkgo (Go)** | `It("checks out", Label("jira:GWCP-1234"), func() { ... })` |
| **JUnit (Java)** | `@Tag("jira:GWCP-1234")` |
| **pytest (Python)** | `@pytest.mark.jira("GWCP-1234")` |
| **Jest (JS/TS)** | `test("checks out", { tags: ["jira:GWCP-1234"] }, () => { ... })` |
| **Cucumber / Godog** | `@jira:GWCP-1234` above the scenario |

Notes:
- **Multiple keys per test are fine** — add more than one `jira:` label; the test counts
  toward each issue.
- **Case doesn't matter on your side** — Fern lowercases tags at ingest and case‑folds when
  matching JIRA's (uppercase) keys.
- Only the `jira:` prefix is load‑bearing for coverage. Other labels (`type:`, `team:`, …)
  are stored but ignored by the coverage view.

See [`examples/jira-coverage/`](../../examples/jira-coverage/) for a copy‑paste sample,
or [`examples/jira-coverage/ginkgo/`](../../examples/jira-coverage/ginkgo/) for a
copy‑paste Ginkgo example (label + reporter wiring) to drop into your own suite.

## Step 2 — Report the run

Use the **Fern reporter for your framework** (the `fern-clients` libraries) exactly as you
already do to send results to Fern. Once a test carries a `jira:` label, the reporter
includes it in that spec's `tags` array — no extra wiring.

Under the hood, the reporter POSTs to `POST /api/v1/test-runs` with the tag attached at the
spec level:

```jsonc
{
  "test_project_id": "<your-fern-project-id>",
  "git_branch": "main",
  "environment": "ci",
  "suite_runs": [{
    "suite_name": "checkout",
    "start_time": "2026-01-01T00:00:00Z",
    "end_time":   "2026-01-01T00:00:10Z",
    "spec_runs": [{
      "spec_description": "completes a guest checkout",
      "status": "passed",                       // passed | failed | skipped
      "start_time": "2026-01-01T00:00:00Z",
      "end_time":   "2026-01-01T00:00:02Z",
      "tags": [{ "name": "jira:GWCP-1234" }]    // <-- the link to JIRA
    }]
  }]
}
```

Tags also work at the **suite** and **test‑run** level (not just per spec) — Fern's coverage
query unions all three granularities, so tag wherever it fits your suite.

> The ingest endpoint is authenticated in the default deployment; the reporter libraries
> handle credentials for you. For a manual `curl`, supply your session/token — see the
> example's `send.sh`.

## Step 3 — Connect JIRA

In the Fern UI: **Project → Integrations → Add JIRA Connection**. Provide the JIRA URL,
project key, username, and API token, then **Test** it until it shows *Connected*.

## Step 4 — Map the "Release Version" field

This is the step people miss. Fern needs to know **which JIRA field defines a "release"** for
your project. In **Project → Integrations → Configure Field Mapping**, connect the Fern
**Release Version** field to the JIRA field your team uses:

- native **Fix Version**, or
- a **custom field** (e.g. a roadmap "Release" field).

Without this mapping the Coverage tab returns *"release_version field not mapped"*.

## Step 5 — View coverage

**Project → Coverage** → pick a release. You'll get the readiness dashboard (overall
coverage donut, per‑epic bars) and the release → epic → story tree, with each issue linked
back to JIRA.

---

## Troubleshooting

| Symptom | Cause / fix |
|---|---|
| *"release_version field not mapped"* | Do Step 4 — map the Release Version field. |
| Release picker is empty | The mapped field has no values, or it's a free‑text field (not enumerable) — type the release value manually. |
| Everything shows **uncovered** | No ingested runs carry matching `jira:` tags yet, **or** the runs are under a different Fern project than the one you're viewing. Confirm `test_project_id` matches the project that holds the JIRA connection. |
| Epics show but **0 stories** | The JIRA project must be **team‑managed (next‑gen)** so Fern can read `parent` links; classic projects use a separate Epic Link field (not yet supported). |
| A tagged test doesn't show as covered | Check the key exists in that release's tree and the spelling matches (a typo'd key like `GWCP-9999` simply never matches). |

---

## Reference

- Wire contract & tag namespacing: [`adr/test-correlation/tag-schema.md`](../../adr/test-correlation/tag-schema.md)
- Raw-payload example (curl): [`examples/jira-coverage/`](../../examples/jira-coverage/)
- Ginkgo example (copy-paste): [`examples/jira-coverage/ginkgo/`](../../examples/jira-coverage/ginkgo/)
- Building a reporter for a new framework: [Integration Guide](integration-guide.md#building-your-own-client-library)
