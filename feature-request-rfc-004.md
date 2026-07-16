# Feature: Frontend modernization (v2 SPA + filtered REST + perf rewrite)

## Background

I'd like to contribute a substantial body of work to fern-platform that
spans frontend, REST surface, and database/perf. It builds on, and
overlaps with, two existing issues:

- **#183 — Vite build pipeline.** My contribution goes further than
  that issue's scope: it ships a full TypeScript + Vite + Tailwind +
  TanStack SPA at `/v2/`, alongside the legacy UI (12-month dual-run
  window per the RFC), rather than just moving the existing JSX into
  Vite. Happy to scope this to match #183 first if that's preferred,
  but I think the broader rewrite is what most reviewers of #183 were
  actually asking for.
- **#178 — Slow DB queries causing UI hangs.** My perf work directly
  fixes the treemap N+1 (replaces the hydration path with SQL
  aggregates + 60s in-memory cache), adds a parallel facet cache, and
  fixes a separate cursor-pagination bug that made "Next" return the
  same page repeatedly.

The design is captured in [RFC-004 Frontend Modernization](https://github.com/<my-fork>/fern-platform/blob/<branch>/docs/rfc/rfc-004-frontend-modernization.md)
in my fork, with the spec triplet under
[`docs/specs/frontend-modernization/`](https://github.com/<my-fork>/fern-platform/tree/<branch>/docs/specs/frontend-modernization).
A migration guide for operators is included.

## What's delivered (in the local branch)

Backend:
- `/api/v2/test-runs` — cursor-paginated, filtered, faceted list endpoint with `saved_views` user-scoped presets.
- Trigram substring search over `spec_name + error_message` (English FTS couldn't match camelCase identifiers; see RFC Appendix B.1).
- Default 30-day window on unscoped queries with `?allTime=1` escape (RFC Appendix B.2).
- Opt-in tag facet via `?facets=tag` (RFC Appendix B.3).
- Treemap resolver rewritten to SQL aggregates with a 60s per-user cache (fixes #178).
- Single consolidated schema migration `000022_v2_schema` (+ pre-flight script for `CREATE INDEX CONCURRENTLY` on large DBs).

Frontend (`web-v2/`):
- React 18 + TS strict + Vite 5 + TanStack Router/Query + Tailwind tokens for light/dark.
- Pages: Dashboard, Projects (+ Settings 4-tab), Project Detail, Test Runs (filter sidebar, saved views, cursor pagination with Previous), Test Run Detail, Test Summaries, Treemap (drill-down, tile-anchored tooltip), Profile, Admin, Users, Jira Connections.
- Shared filter bar reused across Projects + Summaries; URL state for filters is parked.

Operator surface:
- `scripts/v2-preflight-indexes.sh`, updated `docs/specs/frontend-modernization/migration-guide.md` with cost table + dirty-migration recovery procedure.

Stats: 37 commits, ~29k LOC across 230 files, ~7 months of seeded data (1000 projects × 6 months × 100 runs/day = ~18M `test_runs`) used to validate perf claims.

## How I'd like to proceed

This is too large for one PR. I'd propose splitting roughly like this:

1. **PR-1: RFC-004 + spec triplet** — docs only. Gets design alignment with no behaviour change.
2. **PR-2: Backend foundation** — REST v2 surface, filter infra, cursor codec, saved views, migration. No UI.
3. **PR-3: Embed pipeline** — docker-compose for local smoke, SPA mount at `/v2/`, dev-auth bypass.
4. **PR-4: The v2 SPA itself** — real pages.
5. **PR-5: Perf hardening** — cursor fix, errgroup facets, trigram search, treemap aggregates (closes #178).
6. **PR-6: Bulk perf seeder + pre-flight script**.

Each PR rebases on `main`, runs `make test` + `cd acceptance-go && make test-existing`, and uses Conventional Commits per CONTRIBUTING.md.

## Questions for maintainers

1. Is this scope welcome? If not, what's the right cut to bring forward?
2. Do you want PR-1 (RFC) merged first to align before code lands?
3. Should I close #183 in favour of this, or scope PR-3/4 against it explicitly?
4. Anything in the proposed slicing you'd reshape?

Happy to adjust to whatever review cadence works for the team. Thank
you for considering it.
