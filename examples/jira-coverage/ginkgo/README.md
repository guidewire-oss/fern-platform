# Live example: a Ginkgo suite tagged for JIRA coverage

A **runnable** version of "tag a test with `jira:<KEY>` and watch coverage light
up." Unlike the sibling `../` example (which POSTs a raw JSON payload with curl),
this is a real Ginkgo suite that a developer copies into their own project.

Full walkthrough: [../../../docs/developers/linking-tests-to-jira.md](../../../docs/developers/linking-tests-to-jira.md).

## What it shows

- `checkout_test.go` — three specs tagged with `Label("jira:GWCP-1234")` etc.
  (one covers two issues). The label is the only thing you add to a test.
- The `ReportAfterSuite` block — the standard `fern-ginkgo-client` wiring that
  uploads the run. It forwards each spec's labels, and Fern turns the `jira:`
  ones into coverage.

## Run it

```bash
cd examples/jira-coverage/ginkgo
go mod tidy                     # resolves fern-ginkgo-client/v2, ginkgo, gomega

# Without these, the specs still run and pass — they just don't upload.
export FERN_PROJECT_ID=<your-fern-project-id>   # SELECT project_id FROM projects;
export FERN_BASE_URL=http://fern-platform.local:8080

# If your deployment authenticates ingest (the default does), also set the
# client-credentials the reporter reads:
#   export FERN_AUTH_CLIENT_ID=...
#   export FERN_AUTH_CLIENT_SECRET=...
#   export AUTH_URL=http://keycloak:8080/realms/fern/protocol/openid-connect/token

go test ./...
```

Then connect JIRA, map the **Release Version** field, and open **Project →
Coverage**: `GWCP-1234` and `GWCP-1236` show covered & passing, `GWCP-1235`
covered & failing.

> Replace the `GWCP-####` keys in `checkout_test.go` with real issue keys from
> one of your releases, or nothing will match the release tree.

## Notes

- Standalone Go module on purpose — its `fern-ginkgo-client` dependency never
  touches the main `fern-platform` module.
- The exact `client.Report(...)` signature tracks `fern-ginkgo-client/v2`; if
  `go build` complains after `go mod tidy`, check the client's current README
  and adjust the call.
