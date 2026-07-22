# Migration Guide — Fern Platform v1 → v2

**Audience:** existing Fern Platform users (operators, integrators, end users)
**Related:** [requirements.md](./requirements.md), [design.md](./design.md), [tasks.md](./tasks.md)

> **TL;DR**
> - You do not have to migrate today.
> - All v1 APIs keep working for **at least 12 months** after v2 GA.
> - Existing client libraries (`fern-ginkgo-client`, `fern-junit-client`,
>   `fern-jest-client`) keep working **without code changes**.
> - The UI you bookmarked keeps working until the cutover; then it
>   redirects to the equivalent v2 page automatically.
> - **Operators with > 1M test runs:** run `scripts/v2-preflight-indexes.sh`
>   before the rollout (details in §4) — it builds the v2 indexes
>   concurrently against your live DB so the deploy itself doesn't
>   stall on minutes-long index builds.
> - New users should follow [`docs/quick-start.md`](../../quick-start.md)
>   — it points at v2 directly. There is no v1 path for new users.

---

## 1. Who needs to do what

| Audience | Action required? | When | Effort |
|---|---|---|---|
| End users of the UI | No | Automatic redirect at cutover | None |
| Operators self-hosting Fern | Bump container tag; no config change | Anytime after release N | Minutes |
| CI integrators using official client libs | No | At v1 sunset (12 mo after GA) | None |
| CI integrators with hand-rolled v1 calls | Migrate to v2 endpoints | Before v1 sunset | Hours per integration |
| GraphQL API consumers | Optional — adopt new `filter` input for better perf | Anytime | Hours |
| Forked / modified deployments | Review breaking-change notes per release | Each release | Varies |

If you are in row 1, 2, or 3 you can stop reading now and come back at the
sunset announcement.

---

## 2. Compatibility promise

For the duration of this migration we hold ourselves to these rules:

1. **No v1 endpoint changes shape.** Request schemas, response schemas,
   status codes, and error formats are frozen. CI golden-file tests prove
   it on every PR.
2. **No GraphQL field is removed or retyped** without a one-release
   `@deprecated` window. A schema-diff bot blocks PRs that violate this.
3. **No env-var or config-file key changes meaning.** New keys are
   introduced with safe defaults. The single switch `FERN_V2_UI_ENABLED`
   is the *only* new required-ish setting, and even it defaults sensibly.
4. **No database column is renamed or dropped** during the migration
   window. Schema changes are forward-only; index builds that would
   take more than a few seconds on large databases ship with a
   concurrent pre-flight script (see §4 "DB schema migrations").
5. **No client-library version bump is required.** If we change a client
   library in this period, the change is purely additive.

If any of these promises is broken in a release, that release is yanked
and a fixed release follows within 24 hours.

---

## 3. Release timeline

```
              today                           v2 GA                        sunset
  ───────────────┬────────────────────────────┬────────────────────────────┬──────────►
                 │                            │                            │
  Phases 0–3      Phases 4 (cutover)            12-month sunset window        v1 removed
  Both UIs run    v2 default, legacy reachable   v1 still served + warned      from server
  on every URL    at /legacy/* for one release  Deprecation/Sunset headers
```

Concrete dates ship with each release announcement.

---

## 4. UI migration

### What changes for end users

- Same login (Keycloak SSO, same realm, same groups).
- Same URLs continue to work via redirect (`/projects/abc` →
  `/v2/projects/abc` for one release after cutover, then the `/v2` prefix
  is dropped).
- Same data — no re-ingestion required.
- Faster pages, filterable lists, shareable URLs that encode your filters,
  and saved views that follow you across machines.

### What you should know

- During the dual-run window, you can opt **into** v2 early via the banner
  "Try the new UI", or opt **out** of v2 at cutover via "Revert to legacy
  UI" for one release. After that, only v2 is served.
- Browser bookmarks keep working. There is no need to update them.
- If you have custom CSS injected through a browser extension or proxy,
  it will not apply to v2 — class names changed (Tailwind). Either remove
  the customization or rewrite it; both UIs target the same semantics.

### What operators do

```bash
# 1. (Recommended for large databases) Pre-build the v2 indexes
#    out-of-band so the rollout itself doesn't carry the index-build
#    cost. See "DB schema migrations" below.
FERN_DB_URL='postgres://...' ./scripts/v2-preflight-indexes.sh

# 2. Bump the image. The new image ships with FERN_V2_UI_ENABLED=false
#    by default in the bundled KubeVela manifest, so the upgrade is a
#    no-op for the running UI — only the DB migration runs.
helm upgrade fern fern/fern-platform --version <next>

# 3. After validation, flip v2 on
kubectl set env deployment/fern-platform FERN_V2_UI_ENABLED=true
```

The single env var `FERN_V2_UI_ENABLED` controls both the new SPA
mount at `/v2/` and the `/api/v2/*` REST surface; they flip together
to avoid the half-loaded "SPA up, API 404s" state. Until the flag is
flipped, the v2 code is shipped but inert.

### DB schema migrations

v2 ships a single schema migration. It is forward-only and idempotent
(`IF NOT EXISTS` everywhere); no column is renamed or dropped.

| Migration             | Adds                                                                                                                                                                                                                                                                  | Cost on a large DB                                                                                                                                              |
| --------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `000022_v2_schema`    | indexes on `test_runs (project_id, start_time DESC)`, `(start_time DESC, id DESC)`, partial failed/flaky, `(project_id, branch)`; `pg_trgm` extension + GIN trigram index over `spec_runs(spec_name + error_message)`; `saved_views` table with composite unique key  | seconds on `test_runs`; **minutes to tens of minutes** on the trigram index over `spec_runs` (24M rows ≈ 12 min). Run the pre-flight script to avoid the wait.  |

The application runs migrations at container startup via golang-migrate.
On small databases (CI, dev, single-team installs under 100k test runs)
this is invisible — the startup migrations complete in under a minute
and you can simply `helm upgrade` and let it run.

On larger production databases the startup migrations will:

- Block the pod's `/readyz` for the duration of the index builds.
- Hold `ACCESS EXCLUSIVE` on `test_runs` and `spec_runs` while each
  index builds, briefly blocking concurrent writes.

For those installs, use the pre-flight script:

```bash
export FERN_DB_URL='postgres://USER:PASS@HOST:PORT/fern_platform?sslmode=require'
./scripts/v2-preflight-indexes.sh
```

The script builds the same indexes with `CREATE INDEX CONCURRENTLY`,
which does not block writes and is safe to interrupt and retry. After
it succeeds, the startup migrations detect the indexes via
`IF NOT EXISTS` and complete in milliseconds.

The script is idempotent — running it before every release is harmless.

#### Recovering from a dirty migration

If a pod is killed (SIGKILL, OOM, eviction) while a migration is mid-flight,
`schema_migrations.dirty` will be `true` and subsequent startups will
fail with:

```
fatal: failed to run migrations: Dirty database version N. Fix and force version.
```

This does not mean data was corrupted — `CREATE INDEX` either fully
builds or rolls back. Check whether the index from the failed
migration exists, then clear the dirty flag:

```sql
-- Inspect what migration N attempted, find the index name
SELECT indexname FROM pg_indexes WHERE indexname = '<expected_index>';

-- If the index is present, the migration completed before the kill.
-- If absent, run the .up.sql manually (or the pre-flight script).
-- Either way, clear the dirty flag so startup can proceed:
UPDATE schema_migrations SET dirty = false WHERE version = N;
```

Then restart the pod. The next startup will skip the (now-clean) migration.

---

## 5. REST API migration (`/api/v1` → `/api/v2`)

### v1 stays available

Every existing v1 endpoint continues to serve the same response. You do
**not** need to migrate to ship a working integration.

### What v2 adds

- Server-side filtering with rich filter inputs (status, branch, tag,
  date range, search).
- Cursor-based pagination with stable ordering under inserts.
- Faceted counts in the list response (no extra requests for badges).
- An estimated `totalCount` for very large filters (avoids `COUNT(*)`).
- Saved views API (`/api/v2/me/saved-views`).
- Trends aggregate API (`/api/v2/test-runs/trends`) for sparkline /
  dashboard surfaces — one request returns daily sums for many
  projects, replacing the N-request fan-out pattern.
- Default 7-day window on unscoped `/api/v2/test-runs` queries
  (matches typical triage scope; bypass with `?allTime=1`).

### When to migrate

You should migrate to v2 if any of the following are true:

- Your integration paginates large result sets and currently does offset
  pagination (slow and unstable on writes).
- Your integration filters in the client (you fetch everything and grep).
- You hit timeouts on broad queries.
- Your integration is hand-rolled and not maintained by Guidewire.

Otherwise, stay on v1 until sunset.

### Side-by-side examples

**List failed test runs for project `abc` in the last 24h, page 2 of 50.**

v1 (still works, fetches everything; client filters and slices):

```bash
curl -H 'Authorization: Bearer $TOKEN' \
  'https://fern.example.com/api/v1/test-runs?project_id=abc' \
  | jq '.[] | select(.status=="failed") | select(.started_at > "...")' \
  | head -100 | tail -50
```

v2 (server-side, single response, paginated):

```bash
curl -H 'Authorization: Bearer $TOKEN' \
  'https://fern.example.com/api/v2/test-runs?\
project=abc&status=failed&from=2026-05-13T00:00:00Z&first=50&after=eyJpZCI6...'
```

Response wraps results in a connection with `edges`, `pageInfo`,
`totalCount`, and `facets`. See
[design.md §4.2](./design.md#42-rest-v2) for the full schema.

### Endpoint mapping

| v1 endpoint | v2 equivalent | Notes |
|---|---|---|
| `GET /api/v1/projects` | `GET /api/v2/projects` | Paginated and filterable |
| `GET /api/v1/projects/:id` | `GET /api/v2/projects/:id` | Same fields + `+links` |
| `POST /api/v1/projects` | `POST /api/v2/projects` | Same body, stricter validation |
| `GET /api/v1/test-runs` | `GET /api/v2/test-runs` | Returns connection, not array |
| `GET /api/v1/test-runs/:id` | `GET /api/v2/test-runs/:id` | Same |
| `GET /api/v1/test-runs/:id/specs` | `GET /api/v2/test-runs/:id/specs` | Paginated |
| `POST /api/v1/test-runs` *(ingestion)* | **stays v1** | Used by client libs; do not move |
| `GET /api/v1/flaky` | `GET /api/v2/flaky-tests` | Paginated, filterable |
| `GET /api/v1/tags` | `GET /api/v2/tags` | Paginated |
| (none) | `GET/POST/DELETE /api/v2/me/saved-views` | New |
| (none) | `GET /api/v2/test-runs/trends` | New — per-(project, day) sums for sparkline dashboards. Replaces N parallel `/api/v2/test-runs` calls with one SQL aggregate. 60s server cache. |

> **Note on ingestion.** `POST /api/v1/test-runs` is the endpoint client
> libraries use to push test results. It is deliberately **not** moved to
> v2 because it does not benefit from the new contract and is shipped in
> binaries we cannot recall. It is exempt from the v1 sunset.

### Response-shape changes

v1 list responses are arrays; v2 wraps them:

```json
// v1
[
  { "id": "...", "status": "passed", ... },
  ...
]

// v2
{
  "edges":      [{ "cursor": "...", "node": { "id": "...", ... } }, ...],
  "pageInfo":   { "hasNextPage": true, "endCursor": "..." },
  "totalCount": 12448,
  "totalCountIsEstimate": true,
  "facets":     { "byStatus": [...], "byBranch": [...], ... }
}
```

If you migrate a script, the rewrite is mechanical:

```bash
# Replace:
jq '.[]'
# With:
jq '.edges[].node'
```

### Deprecation headers

Every v1 response carries these headers from v2 GA forward:

```
Deprecation: true
Sunset: Fri, 14 May 2027 00:00:00 GMT
Link: <https://docs.fern/migrate-v2>; rel="deprecation"
```

Standard tooling (e.g., `curl -i`, Postman, your HTTP library's middleware)
will surface them. Wire them into your alerting so you know if a job in
your fleet still calls v1 after sunset.

### Migration-readiness CLI

```bash
fern migrate check --since 7d
```

Reads your server's access log (or a connected log source) and reports:

- which v1 endpoints are still in use,
- by which clients (User-Agent),
- with what frequency,
- and which v2 endpoints they map to.

Output is consumed by humans and CI. Exit code is non-zero if any v1
traffic is observed in the lookback window — useful as a pre-sunset gate.

---

## 6. GraphQL migration

There is no v2 GraphQL endpoint. The schema evolves additively at
`/graphql`.

### What is new

- Optional `filter: TestRunFilter` and `page: ConnectionArgs` inputs on
  list queries.
- New `TestRunConnection`-style return types (additive — existing
  resolvers keep their old return shape until clients opt in via fragment
  selection).
- New `facets` field returning counts.

### What is deprecated

Fields and arguments superseded by the new inputs are annotated:

```graphql
extend type Query {
  testRuns(
    "Deprecated: use filter.projectIds"
    projectId: ID @deprecated(reason: "use filter")
    "Deprecated: use page.first"
    first:     Int @deprecated(reason: "use page")
    filter:    TestRunFilter
    page:      ConnectionArgs
  ): TestRunConnection!
}
```

Deprecated fields keep working through the next major release.

### Tools

- `graphql-inspector` runs in our CI and blocks any non-additive change.
- `apollo-client`, `urql`, and `graphql-codegen` users all see deprecation
  warnings in their generated types as `@deprecated` JSDoc.

---

## 7. Client-library migration

| Client library | Action | When |
|---|---|---|
| `fern-ginkgo-client` | None | At v1 sunset, bump to the version that targets v1 ingestion endpoint (unchanged) |
| `fern-junit-client` | None | Same |
| `fern-jest-client` | None | Same |
| `fern-junit-gradle-plugin` | None | Same |

The client libraries call ingestion endpoints that are **not** being
versioned. There is no client-library migration step for users of the
official libraries.

For hand-rolled clients, follow the REST migration in §5.

---

## 8. Database and operations

### Schema changes introduced by this spec

All schema additions ship in a single forward-only migration,
`000022_v2_schema`. The migration is additive — no column is
renamed or dropped, no existing index is removed. For the full
change list, lock profile, and the pre-flight script that builds
the indexes out-of-band without blocking writes, see
[§4 → DB schema migrations](#db-schema-migrations).

### Rollback procedure

If a release misbehaves:

```bash
# 1. Roll back the image to the previous revision.
helm rollback fern <previous-revision>
```

The added indexes and `saved_views` table are safe to leave in
place even after rollback — v1 code paths never touch them.

If you need to drop them as well (rare — only if a column or index
is causing real harm), the binary does not currently expose a
`migrate down` subcommand. Use one of:

```bash
# Option A — apply the .down.sql directly with psql:
psql "$FERN_DB_URL" -f migrations/000022_v2_schema.down.sql

# Option B — use the golang-migrate CLI:
migrate -path migrations -database "$FERN_DB_URL" down 1
```

Either path is idempotent thanks to the `IF EXISTS` guards in
`000022_v2_schema.down.sql`. After running it, clear the
`schema_migrations.dirty` flag if needed — see §4's "Recovering
from a dirty migration" for the SQL.

### Configuration

No config changes are required to upgrade. The relevant
environment variables are:

| Key | Default | Effect |
|---|---|---|
| `FERN_V2_UI_ENABLED` | unset (off) | Mounts the v2 SPA at `/v2/*` and the deprecation handlers on `/api/v1/*`. Set to `true` after smoke-validating v2 against prod. |
| `FERN_V1_DEPRECATED` | unset (off) | Attaches `Deprecation`, `Sunset`, and `Link` headers to `/api/v1/*` responses. Flip on once v2 has GA'd. |
| `FERN_V1_SUNSET_DATE` | 12 months from process start | RFC 3339 timestamp emitted in the `Sunset` header (only when `FERN_V1_DEPRECATED=true`). |
| `FERN_V1_MIGRATION_GUIDE_URL` | unset | URL emitted in the `Link` header so v1 clients can discover this guide. |
| `JIRA_ENCRYPTION_KEY` | **required at startup** | 32-byte base64 key used to encrypt stored Jira credentials. Generate with `make jira-key` (per-developer) or `openssl rand -base64 32` (per-environment). The process fails fast if unset or malformed. |

Facet cache TTL and Web Vitals sampling rate are currently
hardcoded (5 minutes and client-side respectively); they are not
exposed as environment variables.

---

## 9. FAQ

**Q. We have a Grafana panel hitting `/api/v1/test-runs`. Will it break?**
A. No. Keep it on v1 until sunset, then migrate per §5. The mechanical
change is `jq '.[]'` → `jq '.edges[].node'`.

**Q. Our SRE team has Slack bot that posts failing runs. Same answer?**
A. Yes. Same migration when you are ready.

**Q. Can we run v2 in production today and v1 alongside?**
A. Yes — that is the model during the dual-run window. Set
`FERN_V2_UI_ENABLED=true` and use whichever endpoints suit you.

**Q. Our self-hosted instance has a CDN in front. Will the CSP header
break it?**
A. The CSP is `default-src 'self'`. If your CDN is on the same origin as
the app, no change. If it is a separate origin, add it to `connect-src`
via `FERN_CSP_EXTRA_CONNECT_SRC`.

**Q. We forked the UI to add a custom panel. What now?**
A. You will want to port your panel to a React component in `web/src/`.
The shadcn primitives and `<FilterBar>` reduce the code you carry; see
the contributor guide. Reach out and we will help.

**Q. Will the API URL change at sunset?**
A. No. `/api/v1` returns `410 Gone` with a `Link` to v2; `/api/v2` stays
where it is.

**Q. Do we have a v3?**
A. No, and not planned. The point of the GraphQL evolution model is to
never need a v3. REST v2 is shaped to support filter additions
indefinitely.

---

## 10. Where to get help

- Migration questions: open an issue with the `migration` label on
  `github.com/guidewire-oss/fern-platform`.
- Security disclosures: see [SECURITY.md](../../../SECURITY.md).
- Operational issues: `docs/troubleshooting/`.

---

## 11. Checklist for spec authors

This checklist is reviewed at each phase exit. The spec is not "done"
until every box is checked **on each release**.

- [ ] v1 golden-file contract tests pass byte-for-byte.
- [ ] No GraphQL field removed without one release of `@deprecated`.
- [ ] No config key changed meaning; new keys have safe defaults.
- [ ] No DB column renamed or dropped; migrations forward-only.
- [ ] This document updated to reflect any new contracts or deprecations
      introduced by the release.
- [ ] Release notes link this document.
- [ ] `fern migrate check` exits 0 on a representative customer log
      sample (or, before the CLI ships, manual log review).
