# Performance tests (k6)

Scenarios derived from RFC-004 § Performance Testing. Thresholds match
[`perf-budgets.json`](../../perf-budgets.json) at the repo root; keep
them in sync.

## Scenarios

| File | Purpose | Cadence | Duration |
|---|---|---|---|
| `test-runs-list.js` | Load — verify P95 SLOs at 50 VUs across the realistic user mix | every release candidate | 10 min |
| `ingest-spike.js` | Spike — 500 RPS burst on POST `/api/v1/test-runs`, read path must stay green | every release candidate | 90 s |
| `soak.js` | Soak — 30 VUs for 8 hours, detect leaks / drift | weekly in staging | 8 h |

## Prerequisites

- k6 ≥ 0.50
- A reachable Fern instance (`FERN_URL`)
- A valid auth token (`TOKEN`) if the instance enforces auth
- A seeded dataset at ~1 M test_runs for realistic load (`Makefile`
  target `seed-perf` will land in a future PR; until then, point at
  staging)

## Running

```bash
# Load test against local stack
FERN_URL=http://localhost:8080 TOKEN=$(cat .token) \
  k6 run tests/perf/test-runs-list.js

# Smaller smoke run (5 VUs, 1 min)
FERN_URL=http://localhost:8080 VUS=5 DURATION=1m \
  k6 run tests/perf/test-runs-list.js

# Spike
FERN_URL=http://staging.fern.example.com TOKEN=$TOKEN \
  k6 run tests/perf/ingest-spike.js

# Soak (run on a tmux session; report finishes after 8h)
FERN_URL=http://staging.fern.example.com TOKEN=$TOKEN \
  k6 run --out json=soak-$(date +%F).json tests/perf/soak.js
```

## CI integration

The release-candidate pipeline runs `test-runs-list.js` and
`ingest-spike.js` against an ephemeral staging deploy and gates the
release on threshold violations. See
[`.github/workflows/perf.yml`](../../.github/workflows/perf.yml).

## Adding a scenario

1. Mirror an existing file's structure.
2. Read thresholds from `perf-budgets.json` (paste, do not invent).
3. Tag requests with `kind:<x>` so thresholds can be split by mix.
4. Document the new file in this README's scenarios table.
