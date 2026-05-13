# Claude Progress

## Current State

- **Branch:** feature/26-jira-field-mapping
- **Last updated:** 2026-05-18

## What to do next

### Immediate (Task 7.4 — refactor/dedup pass)
Scan all changed files for duplication before opening the PR:
- `domain_resolvers_jira_mapping.go` — check model conversion helpers vs other JIRA resolvers
- `jira_field_mapping_service.go` + `jira_fields_service.go` — confirm no logic duplication
- `web/index.html` — `applyMappingEntries` is used in two places; confirm no further duplication in modal state logic
- Run full test suite after any change

### Then open the PR
- Run `/review-pr` before creating the PR
- Reference issue #26 and link the spec directory in the PR description

### After #26 merges — next issues (in order)
- **#27** — Synchronize JIRA requirements with Fern (depends on #26)
  - Most complex: fetch JIRA issues via JQL, paginate, apply field mapping, create/update/delete Fern requirements, sync trigger UI, error recovery
  - ~2-3x the scope of #26
- **#28** — Link tests to JIRA requirements using tags (depends on #27)
  - Parse JIRA tags from test results (e.g. `@JIRA:PROJ-123`), look up requirement by key, render pills in spec detail view (~line 5347 in web/index.html)
  - Closer in scope to #26

## Key decisions / bugs fixed this session

- **`TestConnection` activation bug**: service never called `conn.Activate()` after a successful test — `is_active` stayed false. Fixed in `service.go`; 3 new unit tests in `service_test.go`.
- **`updatedAt: Time!` null violation**: gqlgen's `MarshalTime` returns `graphql.Null` for zero times; `defaultSnapshot()` had zero `UpdatedAt`. Fixed by making `updatedAt`/`updatedBy` nullable in schema and regenerating.
- **Save required active connection**: `saveJiraFieldMapping` used `FindActiveByProjectID` — blocked saves until connection was tested. Changed to `FindByProjectID`; liveness check belongs at sync time (#27), not config-save time.
- **Upsert `ON CONFLICT` predicate mismatch**: migration created partial unique index (`WHERE deleted_at IS NULL`) but upsert used `ON CONFLICT ("project_id")` without predicate → `SQLSTATE 42P10`. Fixed by adding `WHERE "deleted_at" IS NULL` to conflict target.
- **`config/config.yaml`**: NOT committed — contains local Okta client ID and guidewire-specific URLs. Keep excluded.

## Task 7.3 manual verification — all passed
- ✅ Mapping loads real JIRA fields from live connection
- ✅ Save persists and survives page reload
- ✅ Reset to Defaults restores defaults
- ✅ Non-manager user gets access denied
- ✅ Configure Mapping button hidden when no connection configured

## Session Log

### 2026-05-15 (session 3)
Bug fixes and manual verification: service_test.go (new), service.go (activate fix), jira_field_mapping_service.go (save precondition), schema.graphql + models_gen.go + generated.go (nullable updatedAt), domain_resolvers_jira_mapping.go (pointer fields), gorm_jira_field_mapping_repository.go (ON CONFLICT fix), specs/jira-field-mapping/tasks.md + design.md (updated)

### 2026-05-15 (session 2)
Tasks completed: 6.1 (frontend wiring), 6.2 (no-connection prompt), 7.2 (acceptance tests).
Files: web/index.html, web/js/graphql-client.js, internal/reporter/graphql/schema.resolvers.go, acceptance/jira_field_mapping_test.go, acceptance/jira_suite_test.go, specs/jira-field-mapping/tasks.md
## Past Sessions

