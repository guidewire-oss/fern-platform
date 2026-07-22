#!/usr/bin/env bash
# v2 pre-flight index build.
#
# Run this BEFORE rolling out the v2 release on a database large
# enough that index builds inside startup migrations would be
# disruptive (anything over ~1M test_runs / ~10M spec_runs).
#
# What it does:
#   Creates the five indexes introduced by migration 000022_v2_schema
#   using CREATE INDEX CONCURRENTLY IF NOT EXISTS. Concurrent index
#   builds do not block writes and can be safely interrupted and
#   re-run. After this script completes successfully, the startup
#   migration becomes a no-op (it detects the existing indexes via
#   IF NOT EXISTS) and the rollout proceeds in seconds rather than
#   minutes.
#
# Skipping this script is safe — the rollout will still succeed,
# but startup will block for the duration of the index builds and
# briefly hold an ACCESS EXCLUSIVE lock on test_runs / spec_runs.
#
# Idempotent: safe to re-run.

set -euo pipefail

if ! command -v psql >/dev/null 2>&1; then
  echo "psql not found on PATH. Install postgresql-client and retry." >&2
  exit 2
fi

PSQL_URL="${FERN_DB_URL:-}"
if [[ -z "$PSQL_URL" ]]; then
  cat >&2 <<'EOF'
FERN_DB_URL is not set. Provide a libpq URL with write access:

  export FERN_DB_URL='postgres://USER:PASS@HOST:PORT/DBNAME?sslmode=require'
  ./scripts/v2-preflight-indexes.sh

For a Docker-Compose dev DB:

  export FERN_DB_URL='postgres://postgres:postgres@localhost:55432/fern_platform?sslmode=disable'
EOF
  exit 2
fi

run() {
  local label="$1"; shift
  local sql="$1"; shift
  echo "→ $label"
  # CONCURRENTLY indexes must run outside a transaction. psql defaults
  # to autocommit for non-explicit blocks, which is what we need.
  psql "$PSQL_URL" -v ON_ERROR_STOP=1 -c "$sql"
}

echo "Pre-flight: building v2 indexes concurrently against $(psql "$PSQL_URL" -tAc 'SELECT current_database()')"
echo

# Filtered list path
run "idx_test_runs_project_started_desc" \
    "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_test_runs_project_started_desc
       ON test_runs (project_id, start_time DESC);"

run "idx_test_runs_keyset" \
    "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_test_runs_keyset
       ON test_runs (start_time DESC, id DESC);"

run "idx_test_runs_failed_started" \
    "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_test_runs_failed_started
       ON test_runs (start_time DESC)
       WHERE status IN ('failed', 'flaky');"

run "idx_test_runs_project_branch" \
    "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_test_runs_project_branch
       ON test_runs (project_id, branch);"

# Substring search (pg_trgm extension is a one-off)
echo "→ pg_trgm extension"
psql "$PSQL_URL" -v ON_ERROR_STOP=1 -c "CREATE EXTENSION IF NOT EXISTS pg_trgm;"

run "idx_spec_runs_search_trgm" \
    "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_spec_runs_search_trgm
       ON spec_runs
       USING GIN ((COALESCE(spec_name, '') || ' ' || COALESCE(error_message, ''))
                  gin_trgm_ops);"

echo
echo "All v2 indexes present. Safe to roll out the v2 release."
