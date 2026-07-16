# Roles and Permissions

This document describes the authorization model in Fern Platform — who
can do what, how roles are assigned, and how fine-grained scopes work
alongside roles.

If you are looking for **how to enable authentication** (OAuth/OIDC
setup, env vars, Okta/Keycloak configuration), see
`docs/configuration/` and `docs/deployment/`. This doc covers the
*authorization* model that takes over once a user is signed in.

## The three roles

Fern has three roles, defined in
`internal/domains/auth/domain/user.go`:

| Role      | Intent                                                                 |
| --------- | ---------------------------------------------------------------------- |
| `admin`   | Platform operator. Full access to user administration and all data.    |
| `manager` | Owns one or more teams' projects. Can manage projects, not platform.   |
| `user`    | Read-only by default; can be granted write access per-project via scopes. |

Every authenticated principal has exactly one role. There is no "guest"
or "anonymous" role — unauthenticated requests are rejected by the
auth middleware (unless `AUTH_ENABLED=false`, see the dev-admin section
below).

## How roles are assigned

Roles are derived **at login time** from the IdP's `groups` claim.
`AuthenticationService.determineUserRole`
(`internal/domains/auth/application/authenticate.go:210`) walks the
user's groups against the configured allowlists and picks the first
match:

| User's IdP groups include …                                                                          | Resulting role |
| ---------------------------------------------------------------------------------------------------- | -------------- |
| Literal `admin` or `/admin`, **or** any group listed under `auth.oauth.adminGroups` (config / env)   | `admin`        |
| Literal `manager` or `/manager`, **or** any group under `auth.oauth.managerGroups`                   | `manager`      |
| (no match)                                                                                           | `user`         |

`config.yaml` configures the allowlists; environment variables
`OAUTH_ADMIN_GROUPS` / `OAUTH_MANAGER_GROUPS` and
`OAUTH_ADMIN_USERS` override them at deploy time. Group names are
matched case-sensitively as plain strings.

The role is **re-derived on every login**. This has two consequences:

1. Removing a user from an admin group at the IdP demotes them on
   their next login. No app-side action required.
2. Calling `PUT /api/v1/admin/users/:userId/role` (described below)
   changes the stored role on the user row, **but the next login
   recomputes from the IdP groups and overwrites the change**. The
   durable way to grant a role is via IdP group membership.

## What each role can do

The columns are: admin / manager / user (default) / user + scope (a
plain user with a specific permission scope granted by an admin —
see the scopes section below).

### Authentication and profile

| Action                                              | admin | manager | user | user + scope |
| --------------------------------------------------- | :---: | :-----: | :--: | :----------: |
| Sign in via SSO                                     | ✅    | ✅      | ✅   | ✅           |
| Sign out                                            | ✅    | ✅      | ✅   | ✅           |
| Edit own profile (theme, timezone, favorites, etc.) | ✅    | ✅      | ✅   | ✅           |
| Change own role                                     | ❌    | ❌      | ❌   | ❌           |

### Projects — read

| Action                                                  | admin | manager | user | user + scope |
| ------------------------------------------------------- | :---: | :-----: | :--: | :----------: |
| List projects                                           | ✅    | ✅      | ✅   | ✅           |
| View project detail                                     | ✅    | ✅      | ✅   | ✅           |
| View test runs / suites / specs                         | ✅    | ✅      | ✅   | ✅           |
| View treemap / summaries / dashboards                   | ✅    | ✅      | ✅   | ✅           |
| View saved views                                        | ✅    | ✅      | ✅   | ✅           |
| View tags                                               | ✅    | ✅      | ✅   | ✅           |

### Projects — manage

The `Project.canManage` GraphQL field
(`internal/reporter/graphql/schema.resolvers.go:486`) governs whether
the Settings UI unlocks edit controls for a given project.

| Action                                              | admin | manager | user | user + scope |
| --------------------------------------------------- | :---: | :-----: | :--: | :----------: |
| `Project.canManage = true`                          | ✅    | ✅      | ❌   | ✅ (with `project:write:<id>` / `project:*:<id>` / team-scoped variant) |
| Edit project settings (general, integrations, etc.) | ✅    | ✅      | ❌   | ✅ (same scopes) |
| Create new project                                  | ✅    | ✅      | ❌   | ❌           |
| Delete project                                      | ✅    | ✅      | ❌   | ✅ (with `project:delete:<id>` or `project:*:<id>`) |
| Configure Jira integration                          | ✅    | ✅      | ❌   | ✅ (project-write scope) |

### Tags

| Action                       | admin | manager | user | user + scope |
| ---------------------------- | :---: | :-----: | :--: | :----------: |
| View tags                    | ✅    | ✅      | ✅   | ✅           |
| Create / edit / delete tags  | ✅    | ✅      | ❌   | ❌           |

### Saved views

Saved views are scoped to the user who created them.

| Action                                  | admin | manager | user | user + scope |
| --------------------------------------- | :---: | :-----: | :--: | :----------: |
| Use own saved views                     | ✅    | ✅      | ✅   | ✅           |
| Create / edit / delete own saved views  | ✅    | ✅      | ✅   | ✅           |

### Team membership

Today the Team tab on Project Settings is read-only — the backend
work is parked. See `docs/specs/frontend-modernization/PHASES.md`
parked items 13-14.

| Action                                              | admin | manager | user | user + scope |
| --------------------------------------------------- | :---: | :-----: | :--: | :----------: |
| View team membership of a project                   | ✅    | ✅      | ✅   | ✅           |
| Edit team membership                                | (parked) | (parked, for own team) | ❌   | ❌           |

### User administration

Routes under `/api/v1/admin/users` are wrapped with `RequireAdmin()`
(`pkg/middleware/oauth.go:133`). Any non-admin who calls them gets
HTTP 403.

| Action                                                | admin | manager | user | user + scope |
| ----------------------------------------------------- | :---: | :-----: | :--: | :----------: |
| Open the `/admin` panel                               | ✅    | ❌      | ❌   | ❌           |
| List all users                                        | ✅    | ❌      | ❌   | ❌           |
| View any user's detail                                | ✅    | ❌      | ❌   | ❌           |
| Change another user's role                            | ✅    | ❌      | ❌   | ❌           |
| Suspend / activate a user                             | ✅    | ❌      | ❌   | ❌           |
| Delete a user                                         | ✅    | ❌      | ❌   | ❌           |
| Grant or revoke a user's scopes                       | ✅    | ❌      | ❌   | ❌           |

### System

| Action                                              | admin | manager | user | user + scope |
| --------------------------------------------------- | :---: | :-----: | :--: | :----------: |
| View `/v2/settings` (read-only system view)         | ✅    | ✅      | ✅   | ✅           |
| Modify system config                                | operator (env / config.yaml + restart) | ❌ | ❌ | ❌ |
| View dev / telemetry endpoints                      | ✅    | ✅      | ✅   | ✅           |

### Ingestion (client libraries)

Test result ingestion (`fern-ginkgo-client`, `fern-junit-client`,
`fern-jest-client`) uses service tokens, not user roles. Any caller
that holds a valid token can post to `/api/v1/reports/*`. There is no
per-user gating on ingestion.

## Scopes — fine-grained per-project access

A `user`-role principal is read-only by default. Admins can grant
specific write capabilities without promoting the user to manager by
attaching **scopes** to their account. Scopes are stored in
`user_scopes` and consulted by the `Project.canManage` resolver
(`schema.resolvers.go:503-525`) when role-based access doesn't already
grant it.

| Scope                              | Grants                                          |
| ---------------------------------- | ----------------------------------------------- |
| `project:write:<projectId>`        | Edit the named project                          |
| `project:delete:<projectId>`       | Delete the named project                        |
| `project:*:<projectId>`            | Any action on the named project                 |
| `project:write:<team>:*`           | Edit any project owned by the named team        |
| `project:*:<team>:*`               | Any action on any project of the named team     |

Scopes layer on top of roles — they grant additional permissions, they
do not revoke role-given ones. An admin remains an admin even if they
also have specific scopes.

Use scopes for federated ownership: e.g. grant a feature team's lead
`project:write:checkout-tests` without making them platform manager.

## Special case — dev-admin

When the binary is started with `AUTH_ENABLED=false` (typical for the
docker-compose smoke), `DevAuth` middleware
(`cmd/fern-platform/main.go:101-111`) injects a synthetic principal:

- `userId = dev-admin`
- `role = admin`

This unblocks single-user local dev (no OAuth flow required) and lets
GraphQL resolvers that require `currentUser` serve seeded data. The
v2 UI surfaces an amber "Local dev — admin user injected" banner in
the TopBar when this principal is in effect.

The dev-admin row is upserted into the real `users` table at startup
so foreign keys (`saved_views.user_id`, etc.) resolve, but no
provider session exists; for that reason the v2 Sign-out button is
hidden for this principal.

This bypass is **not** safe for any environment that isn't a
single-user developer machine.

## How the frontend uses this

The v2 chrome (`web-v2/src/components/layout/AppShell.tsx`) reads the
current user via the `useCurrentUser` hook
(`features/auth/useCurrentUser.ts`) and exposes a TopBar dropdown
menu with:

- The user's name, email, and role badge
- A link to **View profile and preferences**
- A link to the **Admin panel** — shown **only when role is admin**
- A **Sign out** action — hidden for the dev-admin principal

Page-level controls (project Settings edit affordances, the `/admin`
nav item, etc.) react to the `Project.canManage` boolean and the
user's role rather than re-checking scopes on the client. The server
is the source of truth; the UI just renders what the server's resolved
permissions say.

## Practical scenarios

**"How do I make someone an admin?"**
Add them to one of the groups configured in `auth.oauth.adminGroups`
(or `OAUTH_ADMIN_USERS` for individual emails). Their next login
picks up the new role. Calling `PUT /admin/users/:id/role` is also
possible but the change is overwritten on the next login.

**"How do I let a tester run only this one project's CI gates?"**
Keep them at `user` role. Grant `project:write:<projectId>` (or
`project:*:<projectId>` if they also need to delete). The Settings UI
unlocks for that project only; everything else stays read-only.

**"How do I let a feature team lead manage everything their team
owns, but nothing else?"**
Keep them at `user` role. Grant `project:*:<team>:*`. They get full
management on any project owned by `<team>` without seeing the
platform-wide admin panel.

**"How do I let an admin test what a regular user sees?"**
Not supported in-product. Sign in as a different identity, or
temporarily remove yourself from admin groups at the IdP and re-login.

## Source of truth

The authoritative references in the code are:

- Role constants — `internal/domains/auth/domain/user.go:30-33`
- Role assignment — `internal/domains/auth/application/authenticate.go:210`
- Admin route gating — `internal/api/auth_handler.go:677-683`
- `Project.canManage` logic — `internal/reporter/graphql/schema.resolvers.go:486-525`
- Dev-admin injection — `cmd/fern-platform/main.go:101-111`
- v2 chrome role rendering — `web-v2/src/components/layout/AppShell.tsx`

If anything in this document disagrees with those files, the code wins
— please update this doc to match and open a PR.
