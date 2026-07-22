#!/usr/bin/env bash
# Send the sample test run (with jira: tags) to Fern's ingest API, so the tagged
# JIRA issues show up as "covered" in the Coverage tab.
#
# This is the reporter-agnostic way to see the wire contract in action — in real
# usage your framework's Fern reporter sends this payload for you (see
# docs/developers/linking-tests-to-jira.md).
#
# Usage:
#   FERN_URL=http://localhost:8080 \
#   FERN_PROJECT_ID=<your-fern-project-id> \
#   FERN_TOKEN=<session-or-bearer-token> \
#   ./send.sh
set -euo pipefail

FERN_URL="${FERN_URL:-http://localhost:8080}"
FERN_PROJECT_ID="${FERN_PROJECT_ID:?set FERN_PROJECT_ID to your Fern project id (SELECT project_id FROM projects)}"
FERN_TOKEN="${FERN_TOKEN:-}"

here="$(cd "$(dirname "$0")" && pwd)"
# Inject the project id into the sample payload.
payload="$(sed "s/REPLACE_WITH_YOUR_FERN_PROJECT_ID/${FERN_PROJECT_ID}/" "${here}/sample-testrun.json")"

# The default deployment authenticates this endpoint. Pass a token if you have one;
# reporters normally supply credentials from their config.
auth=()
if [ -n "${FERN_TOKEN}" ]; then
  auth=(-H "Authorization: Bearer ${FERN_TOKEN}")
fi

curl -sS -X POST "${FERN_URL}/api/v1/test-runs" \
  -H "Content-Type: application/json" \
  "${auth[@]}" \
  --data "${payload}"

echo
echo "Sent. Now: connect JIRA, map the Release Version field, and open Project → Coverage."
echo "GWCP-1234 and GWCP-1236 should show covered/passing; GWCP-1235 covered/failing."
