// k6 load test for GET /api/v2/test-runs.
//
// Mirrors the user mix from RFC-004 Performance § Load model.
// Thresholds match perf-budgets.json (backend.load_test_default_filter).
//
// Usage:
//   FERN_URL=http://localhost:8080 TOKEN=$JWT \
//     k6 run tests/perf/test-runs-list.js
//
// CI:
//   - 50 VUs for 10 min against the seeded 1M-row dataset.
//   - Build fails on threshold breach.

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend, Rate } from 'k6/metrics';

const URL = __ENV.FERN_URL || 'http://localhost:8080';
const TOKEN = __ENV.TOKEN || '';

const listLatency = new Trend('list_latency_ms', true);
const errors = new Rate('errors');

export const options = {
  scenarios: {
    steady: {
      executor: 'constant-vus',
      vus: parseInt(__ENV.VUS || '50', 10),
      duration: __ENV.DURATION || '10m',
    },
  },
  thresholds: {
    'list_latency_ms{kind:default}':  ['p(95)<500',  'p(99)<900'],
    'list_latency_ms{kind:filtered}': ['p(95)<500',  'p(99)<900'],
    'list_latency_ms{kind:search}':   ['p(95)<700',  'p(99)<1200'],
    errors:                            ['rate<0.005'],
    http_req_failed:                   ['rate<0.01'],
  },
};

const PROJECTS = ['p1', 'p2', 'p3', 'p4', 'p5'];
const BRANCHES = ['main', 'release', 'develop'];

function request(path, kind) {
  const headers = TOKEN ? { Authorization: `Bearer ${TOKEN}` } : {};
  const r = http.get(`${URL}${path}`, { headers, tags: { kind } });
  listLatency.add(r.timings.duration, { kind });
  errors.add(r.status !== 200, { kind });
  check(r, {
    'is 200': (res) => res.status === 200,
    'has edges': (res) => res.json('edges') !== undefined,
  }, { kind });
}

export default function () {
  const project = PROJECTS[Math.floor(Math.random() * PROJECTS.length)];
  const branch = BRANCHES[Math.floor(Math.random() * BRANCHES.length)];

  // 70%: filtered list (the most common SRE path)
  // 20%: default list
  // 10%: full-text search
  const dice = Math.random();
  if (dice < 0.7) {
    request(`/api/v2/test-runs?project=${project}&branch=${branch}&first=50`, 'filtered');
  } else if (dice < 0.9) {
    request(`/api/v2/test-runs?first=50`, 'default');
  } else {
    request(`/api/v2/test-runs?q=timeout&first=50`, 'search');
  }
  sleep(1);
}
