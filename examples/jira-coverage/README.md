# Example: linking tests to JIRA for release coverage

A minimal, copy‑pasteable example of the one thing that makes release coverage work:
**tagging a test with `jira:<ISSUE-KEY>`.**

Full walkthrough: [docs/developers/linking-tests-to-jira.md](../../docs/developers/linking-tests-to-jira.md).

## 1. Tag a test (your framework's idiom)

The tag is the only thing you add to a test. A Ginkgo example:

```go
var _ = Describe("Checkout", func() {
    It("completes a guest checkout", Label("jira:GWCP-1234"), func() {
        // ...assertions...
    })

    // A test can cover multiple issues:
    It("shows tax for a multi-item cart", Label("jira:GWCP-1234", "jira:GWCP-1236"), func() {
        // ...
    })
})
```

Equivalents: JUnit `@Tag("jira:GWCP-1234")`, pytest `@pytest.mark.jira("GWCP-1234")`,
Jest `{ tags: ["jira:GWCP-1234"] }`, Cucumber `@jira:GWCP-1234`.

Your framework's Fern reporter turns that label into the wire tag automatically — no manual
JSON. `sample-testrun.json` here shows exactly what the reporter ends up sending.

## 2. See it without a reporter (raw ingest)

To prove the end‑to‑end without wiring a client, POST the sample payload directly:

```bash
FERN_URL=http://localhost:8080 \
FERN_PROJECT_ID=<your-fern-project-id> \
FERN_TOKEN=<session-or-bearer-token> \
./send.sh
```

Find your project id with `SELECT project_id, name FROM projects;`. Then connect JIRA, map
the **Release Version** field, and open **Project → Coverage** — `GWCP-1234`/`GWCP-1236` show
covered & passing, `GWCP-1235` covered & failing.

> Replace the `GWCP-####` keys with real issue keys from one of your releases, or nothing
> will match the release tree.

## A runnable version

Prefer a real test over a raw payload? [`ginkgo/`](ginkgo/) is a standalone,
runnable Ginkgo suite with `Label("jira:...")` and the `fern-ginkgo-client`
reporter wired in — `go test` it against a live Fern and watch coverage light up.

## Files
- `sample-testrun.json` — the ingest payload (jira: tags at the spec level)
- `send.sh` — POSTs it to `POST /api/v1/test-runs`
- `ginkgo/` — a runnable Ginkgo suite demonstrating idiomatic `jira:` tagging
