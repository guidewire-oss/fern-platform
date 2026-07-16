#!/usr/bin/env bash
#
# Smoke probe for the local docker-compose stack started by
# `make docker-test-up`. Exits non-zero on the first failed check.
#
# Checks:
#   - /healthz returns 200
#   - /readyz returns 200 (DB reachable)
#   - /metrics returns Prometheus text exposition
#   - /api/v1/health returns 200 (legacy probe still works)
#   - /api/v2/test-runs returns a connection-shaped JSON
#   - /api/v2/me/saved-views returns 200 with empty list under the
#     dev-auth bypass that compose enables (AUTH_ENABLED=false)
#   - /api/v2/telemetry/vitals 202s on a valid payload
#   - CSP header appears on an HTML response

set -euo pipefail

BASE="${FERN_URL:-http://localhost:8080}"
PASS=0
FAIL=0

# ANSI colors (no-op when stdout isn't a TTY)
if [ -t 1 ]; then
  GREEN=$'\033[32m'; RED=$'\033[31m'; DIM=$'\033[2m'; RESET=$'\033[0m'
else
  GREEN=""; RED=""; DIM=""; RESET=""
fi

probe() {
  local name="$1" expected_status="$2" url="$3" expect_substr="${4:-}"
  local extra=("${@:5}")
  local body status
  body=$(curl -sS -o /tmp/.fern-smoke-body -w "%{http_code}" "${extra[@]}" "$url" 2>/dev/null) || status="ERR"
  status="$body"
  if [ "$status" != "$expected_status" ]; then
    printf "%s✗%s %-50s  status=%s want=%s\n" "$RED" "$RESET" "$name" "$status" "$expected_status"
    sed -n '1,6p' /tmp/.fern-smoke-body | sed "s/^/   ${DIM}/; s/$/${RESET}/"
    FAIL=$((FAIL+1))
    return
  fi
  if [ -n "$expect_substr" ] && ! grep -q -F "$expect_substr" /tmp/.fern-smoke-body; then
    printf "%s✗%s %-50s  body missing %q\n" "$RED" "$RESET" "$name" "$expect_substr"
    FAIL=$((FAIL+1))
    return
  fi
  printf "%s✓%s %-50s  status=%s\n" "$GREEN" "$RESET" "$name" "$status"
  PASS=$((PASS+1))
}

probe_header() {
  local name="$1" header="$2" url="$3" expect_substr="$4"
  local got
  got=$(curl -sSI "$url" 2>/dev/null | tr -d '\r' | awk -F': ' -v h="$header" 'tolower($1) == tolower(h) { for (i=2; i<=NF; i++) printf "%s%s", $i, (i<NF?": ":""); print "" }')
  if [ -z "$got" ]; then
    printf "%s✗%s %-50s  header %q missing\n" "$RED" "$RESET" "$name" "$header"
    FAIL=$((FAIL+1))
    return
  fi
  if ! printf "%s" "$got" | grep -q -F "$expect_substr"; then
    printf "%s✗%s %-50s  header=%q (no %q)\n" "$RED" "$RESET" "$name" "$got" "$expect_substr"
    FAIL=$((FAIL+1))
    return
  fi
  printf "%s✓%s %-50s  %s: %s\n" "$GREEN" "$RESET" "$name" "$header" "$got"
  PASS=$((PASS+1))
}

echo ""
echo "Smoke probe → $BASE"
echo ""

# Health
probe "/healthz (liveness)"        200 "$BASE/healthz"               '"status":"ok"'
probe "/readyz  (readiness)"       200 "$BASE/readyz"                '"status":"ok"'
probe "/api/v1/health (legacy)"    200 "$BASE/api/v1/health"

# Metrics
probe "/metrics (Prom text)"       200 "$BASE/metrics"               "fern_http_requests_total"

# v2 surface
probe "/api/v2/test-runs (empty filter)"  200 "$BASE/api/v2/test-runs?first=5"  '"edges"'
probe "/api/v2/test-runs (status=failed)" 200 "$BASE/api/v2/test-runs?status=failed&first=5" '"facets"'
probe "/api/v2/me/saved-views (dev admin)" 200 "$BASE/api/v2/me/saved-views"   '"views"'

# Telemetry
probe "/api/v2/telemetry/vitals POST" 202 "$BASE/api/v2/telemetry/vitals" "" \
  -X POST -H "Content-Type: application/json" \
  -d '{"name":"LCP","value":1850,"rating":"good","route":"/test-runs"}'

probe "/api/v2/telemetry/vitals (bad metric)" 400 "$BASE/api/v2/telemetry/vitals" "" \
  -X POST -H "Content-Type: application/json" \
  -d '{"name":"BANANA","value":1,"route":"/x"}'

# CSP on the SPA shell (legacy / route serves HTML)
probe_header "/ CSP header"  Content-Security-Policy "$BASE/"  "default-src 'self'"

echo ""
if [ "$FAIL" -eq 0 ]; then
  printf "%s%d passed%s, %d failed.\n" "$GREEN" "$PASS" "$RESET" "$FAIL"
  exit 0
else
  printf "%s%d failed%s, %d passed.\n" "$RED" "$FAIL" "$RESET" "$PASS"
  exit 1
fi
