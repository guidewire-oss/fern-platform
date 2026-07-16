// k6 soak test — 8 hours at 30 VUs.
// Watches RSS, goroutine count, and DB pool drift. Run weekly in
// staging; do NOT run in CI on PR merges.
//
// Usage:
//   FERN_URL=http://staging-fern.example.com TOKEN=$JWT \
//     k6 run tests/perf/soak.js

import http from 'k6/http';
import { sleep } from 'k6';
import { Trend, Rate } from 'k6/metrics';

const URL = __ENV.FERN_URL || 'http://localhost:8080';
const TOKEN = __ENV.TOKEN || '';

const latency = new Trend('latency_ms', true);
const errors = new Rate('errors');

export const options = {
  scenarios: {
    soak: {
      executor: 'constant-vus',
      vus: parseInt(__ENV.VUS || '30', 10),
      duration: __ENV.DURATION || '8h',
    },
  },
  thresholds: {
    // No-drift acceptance: P95 must remain stable through the run.
    // Spot-check via Grafana during the run; threshold here only
    // catches catastrophic regressions.
    latency_ms: ['p(95)<700'],
    errors:     ['rate<0.005'],
  },
};

export default function () {
  const headers = TOKEN ? { Authorization: `Bearer ${TOKEN}` } : {};
  const r = http.get(`${URL}/api/v2/test-runs?first=50`, { headers });
  latency.add(r.timings.duration);
  errors.add(r.status !== 200);
  sleep(2);
}
