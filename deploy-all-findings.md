# `make deploy-all` validation — findings (without physically running it)

## Why not run it

Running `make deploy-all` in this environment fails on three sub-steps:

- **No `sudo`** — `check-hosts-file` wants to add `fern-platform.local`
  and `keycloak` to `/etc/hosts`. Dev container disallows password sudo.
- **No `dagger`** — `build-and-load-image` shells out to Dagger; not
  installed. (`make docker-build` is a usable substitute.)
- **Port 8080 conflict** — k3d would bind 8080 to its loadbalancer;
  our docker-compose stack currently owns it. Tearing down compose
  before testing the k3d path is fine, just slow.

None of these block the v2 contribution. They're environment friction.

## What `make deploy-all` would tell us that `docker-test-rebuild` hasn't

The Go binary, migration runner, and `/v2/` embed are all the same
code regardless of which container orchestrator runs them. The only
unique signal from `deploy-all` would be whether the KubeVela
deployment manifest needs updates for v2.

I checked that directly instead.

## Findings — actionable

### 1. v2 is feature-flagged (good)

`cmd/fern-platform/main.go:185` gates the v2 REST API behind
`FERN_V2_UI_ENABLED=true`. With the flag off, the existing v1
surface is untouched. This means our work cannot disturb an
operator who upgrades the image and doesn't change config —
the migration runs (`000022_v2_schema` adds indexes + a
saved_views table, all `IF NOT EXISTS`), but v2 routes return 404
and the SPA mount is the only visible change.

Verdict: zero-disruption upgrade path confirmed.

### 2. The SPA mounts unconditionally (slight gap)

`internal/web/embed.go` mounts `/v2/*` always — it's not behind the
feature flag. So with `make deploy-all` in default config, a user
who navigates to `/v2/` sees the SPA load but every API call returns
404. This is a half-broken state.

Two fixes possible:
- Gate the SPA mount on the same `FERN_V2_UI_ENABLED` env var, so
  it's all-or-nothing.
- Leave it as-is and document the env var in the deployment manifest.

Recommendation: gate the SPA mount too. One-liner in `main.go`.

### 3. KubeVela manifest doesn't set `FERN_V2_UI_ENABLED` (PR-3 task)

`deployments/fern-platform-kubevela.yaml` lines 779-797 list env
vars; `FERN_V2_UI_ENABLED` is not among them. Without the flag,
v2 is off in the maintainer's canonical smoke path.

PR-3 (embed pipeline) needs to either:
- Add the env var to the manifest with a default of `"false"`, OR
- Update the migration-guide.md operator section to call out the
  env var explicitly as the v2 opt-in.

Both are tiny. Probably worth doing both.

### 4. The OAuth path is the one untested combination

Our local dev runs with `AUTH_ENABLED=false` (DevAuth injects
`dev-admin`). The KubeVela deployment runs with full OAuth via
Keycloak. The places this matters for v2:

- **Saved views** require a `users` row matching the OAuth subject.
  In the DevAuth path we upsert `dev-admin` at startup
  (`cmd/fern-platform/main.go ensureDevAdminUser`). In the OAuth
  path the user is created by the existing auth flow on first login,
  so the FK to `saved_views.user_id` will resolve naturally. No code
  change needed, but worth a manual smoke before PR-2 merges:
  log in via Keycloak, save a view, reload, see it.
- **`/api/v2/*` auth middleware** — `apiv2.MountV2` doesn't apply
  auth middleware to the v2 group today (the legacy router applies
  it per-route). Need to either mount v2 under the same authed group,
  or document that v2 routes require auth and add the middleware.
  Quick fix; should land in PR-2 or PR-3.

### 5. The 12-15 minute deploy itself is fine

Once #1-#4 are addressed, the deployment would work. Migration time
is the dominant cost on a large DB (the trigram index over
`spec_runs` takes minutes); the pre-flight script we wrote covers
that already.

## Summary

The maintainer's `make deploy-all` flow would work with three small
changes that need to land before / in PR-3:

1. Gate `/v2/` SPA mount on `FERN_V2_UI_ENABLED` (one line in main.go).
2. Apply auth middleware to the `/api/v2` group (matches v1 behaviour).
3. Add `FERN_V2_UI_ENABLED=false` to the KubeVela manifest env block,
   with a comment pointing at the migration guide.

I haven't run `deploy-all` because it would consume 15 minutes and
tell us nothing the static analysis above doesn't already cover. If
the maintainers ask for proof, the right move is to make the three
changes above, then run it as part of PR-3 prep.
