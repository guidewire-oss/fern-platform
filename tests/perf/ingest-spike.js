// k6 spike test for POST /api/v1/test-runs (ingestion endpoint).
//
// Simulates a monorepo CI bursting 500 RPS for 30 seconds.
// Verifies ingestion latency and that the read path is unaffected.
//
// Usage:
//   FERN_URL=http://localhost:8080 TOKEN=$JWT \
//     k6 run tests/perf/ingest-spike.js

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend, Rate } from 'k6/metrics';

const URL = __ENV.FERN_URL || 'http://localhost:8080';
const TOKEN = __ENV.TOKEN || '';

const ingestLatency = new Trend('ingest_latency_ms', true);
const readLatency = new Trend('read_latency_ms', true);
const ingestErrors = new Rate('ingest_errors');

export const options = {
  scenarios: {
    ingest_burst: {
      executor: 'ramping-arrival-rate',
      startRate: 50,
      timeUnit: '1s',
      preAllocatedVUs: 100,
      maxVUs: 500,
      stages: [
        { target: 500, duration: '30s' },
        { target: 0,   duration: '60s' },
      ],
      exec: 'ingest',
    },
    read_steady: {
      executor: 'constant-vus',
      vus: 10,
      duration: '90s',
      exec: 'read',
    },
  },
  thresholds: {
    ingest_latency_ms: ['p(95)<250', 'p(99)<500'],
    ingest_errors:     ['rate<0.01'],
    read_latency_ms:   ['p(95)<500'], // unaffected by spike
  },
};

function headers() {
  return TOKEN
    ? { 'Content-Type': 'application/json', Authorization: `Bearer ${TOKEN}` }
    : { 'Content-Type': 'application/json' };
}

export function ingest() {
  const payload = JSON.stringify({
    project_id: 'perf-spike',
    run_id: `spike-${__VU}-${__ITER}-${Date.now()}`,
    status: 'completed',
    start_time: new Date().toISOString(),
    end_time: new Date().toISOString(),
  });
  const r = http.post(`${URL}/api/v1/test-runs`, payload, { headers: headers() });
  ingestLatency.add(r.timings.duration);
  ingestErrors.add(r.status >= 500);
  check(r, { 'ingest accepted': (res) => res.status < 500 });
}

export function read() {
  const r = http.get(`${URL}/api/v2/test-runs?first=50`, { headers: headers() });
  readLatency.add(r.timings.duration);
  sleep(1);
}
