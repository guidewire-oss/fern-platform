# web-v2

New Fern Platform SPA per **RFC-004** (Frontend Modernization). Lives
alongside the legacy `web/` directory during the strangler migration
described in [docs/specs/frontend-modernization/](../docs/specs/frontend-modernization/).

At Phase 5 cutover, the legacy `web/` is deleted and this directory is
renamed to `web/`.

## Stack

React 18 · TypeScript (strict) · Vite 5 · TanStack Query/Router ·
GraphQL Codegen · Tailwind + shadcn/ui · Vitest · Playwright · size-limit.

## Development

```bash
pnpm install              # first time only
make web-dev              # or: pnpm dev
```

Vite runs on `:5173` and proxies `/api`, `/graphql`, `/query`, `/auth` to
the Go server on `:8080`. GraphQL requests go to `/query`; `/graphql`
serves the playground page.

## Build

```bash
make web-build            # typecheck, lint, test, vite build → dist/
```

`dist/` is copied into `internal/web/dist/` so the Go binary embeds it
via `//go:embed`.

## Deployment

You don't deploy this directory directly — it's built as the first stage
of the repo-root `Dockerfile` (web-v2 → Go → runtime) and served by the
Go binary. From the repo root:

```bash
make deploy-all-v2        # full k3d deploy (cluster + prereqs + build + deploy)
make deploy-quick-v2      # rebuild + redeploy (assumes cluster already exists)
```

The server mounts this SPA at `/v2/`, gated behind `FERN_V2_UI_ENABLED=true`
(RFC-004 FR-25 — the legacy UI stays at `/` until parity is verified). The
Vite `base` is set to `/v2/` in `vite.config.ts` so the embedded
`index.html` requests assets from `/v2/assets/*`.

## Tests

```bash
pnpm test                 # vitest watch
pnpm test:run             # vitest single run
pnpm test:coverage        # with coverage
pnpm playwright           # E2E (needs deployed stack)
```

## Bundle budgets

Enforced by `size-limit`:

- Initial bundle ≤ 150 KB gzipped
- Total app ≤ 600 KB gzipped

## Making v2 the default

Today v2 lives at `/v2/` beside the legacy UI at `/`. When parity is
verified and you want v2 to own the root endpoint, do the following —
each step is small and the code already supports it:

1. **Vite base** — in `vite.config.ts`, change `base: '/v2/'` to
   `base: '/'` so assets resolve from `/assets/*`.
2. **Root mount** — in `cmd/fern-platform/main.go`, replace
   `web.RegisterAtPrefix(router, "/v2")` with `web.Register(router)`.
   `Register` mounts at `/` and installs the SPA `NoRoute` fallback
   (it already exists in `internal/web/embed.go`). Drop the
   `FERN_V2_UI_ENABLED` gate here if v2 is now unconditional.
3. **Retire legacy `web/`** — remove the v1 static serving from the API
   handler and delete the legacy `web/` directory.
4. **Rename** — move `web-v2/` → `web/` and update the paths in
   `Makefile.web`, `Makefile.docker`, and the repo-root `Dockerfile`.
5. **Collapse the Makefile targets** — fold `deploy-all-v2` /
   `deploy-quick-v2` back into `deploy-all` / `deploy-quick` and remove
   the now-dead v1 `build-and-load-image` path.

See [docs/specs/frontend-modernization/design.md](../docs/specs/frontend-modernization/design.md)
for the full design contract.
