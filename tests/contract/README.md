# Contract tests (Pact provider verification)

The Go server is the **provider**. The web-v2 SPA records **consumer
contracts** (one per GraphQL/REST query it issues) and publishes them
to a shared location:

- **Preferred:** a Pact broker. Set `PACT_BROKER_URL` (and optionally
  `PACT_BROKER_TOKEN`). This is what CI uses.
- **Fallback:** a filesystem path. Set `PACT_DIR` to a directory
  containing one JSON file per consumer interaction. Useful in
  air-gapped environments.

When neither is set, `go test ./tests/contract/...` skips with a
clear message. That is the current default — the scaffold is in
place but the broker / consumer pacts have not landed yet.

## Local run

```bash
# Against a Pact broker
PACT_BROKER_URL=https://pact.fern.internal \
PACT_BROKER_TOKEN=$(cat .pact-token) \
  go test ./tests/contract/...

# Against a directory of pact JSON files
PACT_DIR=$(pwd)/tests/contract/pacts \
  go test ./tests/contract/...
```

## CI integration

The `go-v2.yml` workflow gains a `contract` job once a broker URL is
provisioned. Until then the harness is gated on env, so the rest of
CI runs unaffected.

## What a contract looks like

Consumer side (web-v2, illustrative — actual implementation lands
with frontend Phase 1):

```ts
pact.addInteraction({
  uponReceiving: 'a list of failed test runs',
  withRequest: {
    method: 'GET',
    path: '/api/v2/test-runs',
    query: 'status=failed&first=50',
  },
  willRespondWith: {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
    body: like({
      edges: eachLike({ cursor: like('...'), node: like({ id: 'r1' }) }),
      pageInfo: { hasNextPage: like(false), endCursor: like('') },
      totalCount: like(0),
      totalCountIsEstimate: like(true),
      facets: like({ byStatus: [], byBranch: [], byTag: [], byProject: [] }),
    }),
  },
});
```

Provider side here verifies that the same request against the actual
v2 handler produces a response matching the contract's matchers.

## What it catches

- Frontend asks for a field that disappeared from the response.
- Frontend sends a filter param the server stopped honoring.
- Server changes a field's type (string → number).
- Server's error shape drifts away from the contract.

What it does **not** catch: behavioral correctness, data semantics,
or performance. Those need their own test layers.
