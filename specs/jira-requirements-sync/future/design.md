# Design: JIRA Requirements Sync

## UI Reference

A UI mockup is at `specs/jira-requirements-sync/jira-requirements-sync-mock.png`. It shows the Configure → Syncing → Complete wizard plus the inventory list. The implementation lives in the **Integrations** tab of the Project detail page (not a top-level nav tab — the mockup is illustrative).

## Overview

This feature introduces a local cache of JIRA issues — the **requirements store** — populated from the JIRA REST API and used by downstream coverage views (#29, #30) to render without depending on live JIRA availability. It also extracts Gherkin scenarios from issue descriptions so coverage can be tracked at scenario granularity, and runs a **sync-side backfill pass (Path B)** that materializes `spec_run ↔ requirement` mappings for tests whose JIRA references became known only after sync.

The design extends `internal/domains/integrations` (already owns JIRA connection + field mapping) and introduces a new `internal/domains/requirements` bounded context for **requirements** and **sync runs**. Three sync triggers (initial bulk, on-demand incremental, staleness refresh) drive a single underlying sync pipeline. A two-tier Gherkin parser (deterministic Tier 1 always on; LLM Tier 2 opt-in) runs as a post-fetch step per issue whose description hash changed. The Path B backfill writes mapping rows inline within the sync job before the summary is emitted.

Architectural decisions cross-referenced from this design:

- [`tag-schema.md`](../../adr/test-correlation/tag-schema.md) — wire contract for `jira:KEY` / `scenario:TITLE` tags
- [`mapping-lifecycle.md`](../../adr/test-correlation/mapping-lifecycle.md) — Path A (test ingest, #28) + Path B (sync backfill, this spec); idempotency
- [`gherkin-parsing-tiers.md`](../../adr/test-correlation/gherkin-parsing-tiers.md) — Tier 1 deterministic / Tier 2 LLM extraction

## System Architecture

### Directory layout

```
internal/
├── domains/integrations/
│   ├── jira_client.go                          # EXTEND — add SearchIssues, GetIssuesByKeys
│   ├── jira_connection.go                      # UNCHANGED
│   ├── jira_field_mapping.go                   # UNCHANGED (from #26)
│   ├── repository.go                           # UNCHANGED
│   └── types.go                                # EXTEND — JiraIssue DTO
│
├── domains/requirements/                       # NEW — bounded context
│   ├── domain/
│   │   ├── requirement.go                      # NEW — Requirement aggregate
│   │   ├── requirement_test.go                 # NEW
│   │   ├── sync_run.go                         # NEW — SyncRun aggregate
│   │   ├── sync_run_test.go                    # NEW
│   │   ├── spec_run_requirement.go             # NEW — join value type
│   │   ├── project_sync_settings.go            # NEW — settings aggregate
│   │   └── repository.go                       # NEW — interfaces
│   │
│   └── application/
│       ├── sync_service.go                     # NEW — orchestrates sync triggers
│       ├── sync_service_test.go                # NEW
│       ├── gherkin_parser.go                   # NEW — Tier 1 pipeline
│       ├── gherkin_parser_test.go              # NEW
│       ├── llm_extractor.go                    # NEW — Tier 2 interface + stub
│       ├── backfill_service.go                 # NEW — Path B mapping writer
│       └── backfill_service_test.go            # NEW
│
├── infrastructure/repositories/
│   ├── gorm_requirement_repository.go          # NEW
│   ├── gorm_requirement_repository_test.go     # NEW
│   ├── gorm_sync_run_repository.go             # NEW
│   ├── gorm_sync_run_repository_test.go        # NEW
│   ├── gorm_spec_run_requirement_repository.go # NEW
│   ├── gorm_spec_run_requirement_repository_test.go # NEW
│   └── gorm_project_sync_settings_repository.go # NEW
│
└── reporter/graphql/
    ├── schema.graphql                          # EXTEND — new types + queries + mutations
    ├── domain_resolvers_jira_sync.go           # NEW — sync resolvers
    ├── domain_resolvers_jira_sync_test.go      # NEW
    ├── domain_resolvers_requirements.go        # NEW — requirements query resolvers
    └── domain_resolvers_requirements_test.go   # NEW

migrations/
├── 000023_create_requirements_sync_tables.up.sql   # NEW — all four tables in one
└── 000023_create_requirements_sync_tables.down.sql

web/
└── index.html                                  # EXTEND — inventory page in Integrations tab

acceptance/
├── helpers/
│   └── mock_jira_server.go                     # EXTEND — /search, /issue endpoints
├── jira_sync_test.go                           # NEW — end-to-end sync acceptance tests
└── jira_sync_backfill_test.go                  # NEW — Path B mapping acceptance test
```

### Component diagram

```
              JIRA Cloud
                 │
                 │ REST API
                 ▼
   ┌────────────────────────────────┐
   │      JiraClient (extended)     │
   │  SearchIssues, GetIssuesByKeys │
   └─────────────┬──────────────────┘
                 │
                 ▼
   ┌────────────────────────────────────────────────┐
   │       SyncService  (background job)            │
   │                                                │
   │  initial / incremental / staleness triggers    │
   │  ↓                                             │
   │  1. fetch  (paginate, backoff, audit)          │
   │  2. parse  (Tier 1 Gherkin; Tier 2 if enabled) │
   │  3. upsert (requirements; description-hash diff)│
   │  4. backfill (Path B; spec_run_requirements)   │
   │  5. summary (sync_runs row, status=succeeded)  │
   └─────────────┬──────────────────────────────────┘
                 │
                 ▼
   ┌────────────────────────────────────────────────┐
   │  requirements    spec_run_requirements         │
   │  sync_runs       project_jira_sync_settings    │
   │      (Postgres)                                │
   └─────────────┬──────────────────────────────────┘
                 │
                 ▼
   ┌────────────────────────────────────────────────┐
   │   GraphQL — queries (requirements, sync runs)  │
   │           mutations (startJiraSync, ...)       │
   └─────────────┬──────────────────────────────────┘
                 │
                 ▼
       Inventory page (web/index.html, Integrations tab)
       #29 coverage view, #30 release dashboard (out of scope)
```

## Data Flow

### Initial sync (Req 1)

1. Manager opens **Configure Sync** dialog (sync defaults pre-populated from `project_jira_sync_settings`).
2. Manager confirms → GraphQL `startJiraSync(input)` mutation → returns `sync_run_id` immediately (202 Accepted equivalent).
3. Background worker:
   a. `INSERT sync_runs (status='running', trigger_source='initial', config_snapshot=...)`.
   b. Build JQL: `(issuetype in (selected)) AND updated >= -<release_window_days>d AND (<user JQL>)`.
   c. Call `JiraClient.SearchIssues(jql, page, 100)` repeatedly; backoff on 429/5xx.
   d. For each issue: apply field mapping (from #26) → upsert `requirements` row → compute `description_hash`; if changed, run Tier 1 Gherkin parser → upsert scenario rows.
   e. Run backfill pass (see below).
   f. `UPDATE sync_runs SET status='succeeded', completed_at=NOW, processed_count, ..., mappings_created=X`.
4. UI polls `jiraSyncRun(id)` every 2–5s; renders progress, then switches to summary on terminal state.

### Incremental sync (Req 2)

Same pipeline as initial, with JQL clause `updated > <last_succeeded_sync_started_at>`. Orphan handling: any `requirements` row whose `jira_key` was in the project's scope at last sync but is absent from this scope's JQL result is marked `orphaned=TRUE` (not deleted; preserves test associations).

### Staleness refresh (Req 3)

Triggered by GraphQL coverage query (consumed by #29) when `MAX(requirements.last_synced_at) < NOW - staleness_threshold_hours`. Renders from current local data; kicks off `startJiraSync(trigger='staleness', scope=epic_id)` asynchronously. UI surfaces "refresh in progress" badge; subsequent polls reflect the updated rows once the sync finishes.

### Path B backfill (Req 10; ADR [`mapping-lifecycle.md`](../../adr/test-correlation/mapping-lifecycle.md))

Step 4 of the sync pipeline. After requirements are written/updated:

1. Collect the set `K` of `jira_key`s touched by this sync run.
2. Single SQL: `SELECT sr.id, t.name FROM spec_runs sr JOIN spec_run_tags srt ON srt.spec_run_id=sr.id JOIN tags t ON t.id=srt.tag_id JOIN test_runs tr ON tr.id=sr.test_run_id WHERE tr.project_id=? AND tr.created_at >= NOW() - INTERVAL '<release_window_days>' AND t.name LIKE 'jira:%' AND substring(t.name, 6) = ANY(?)`.
3. For each `(spec_run_id, jira_key)` pair, resolve to `requirement_id` (scenario row if a sibling `scenario:` tag matches; else issue row).
4. `INSERT INTO spec_run_requirements (...) ON CONFLICT (spec_run_id, requirement_id) DO NOTHING` — idempotent per the ADR.
5. Count rows inserted; record in `sync_runs.mappings_created`.

### Inventory page render (Req 11)

GraphQL `requirements(projectId, filters)` returns paginated rows for the inventory list. Each row is `type ∈ {epic, story, task, bug, subtask}`; clicking expands to load scenario children via the same query with `parentId=<row.id>`. No JIRA call — pure read from local store.

## Interface Specifications

### Domain types — `internal/domains/requirements/domain/`

**`Requirement` aggregate** (one type, polymorphic via `Type` field):

```go
type Requirement struct {
    ID              RequirementID
    ProjectID       string
    JiraKey         *string                    // nil for parsed/llm_extracted
    Source          RequirementSource          // "jira_sync" | "parsed" | "llm_extracted"
    Type            RequirementType            // "epic" | "story" | "task" | "bug" | "subtask" | "scenario"
    ParentID        *RequirementID             // nil for top-level epics
    Title           string
    Status          string
    FixVersion      string
    Description     string                     // markdown, jira_sync only
    GherkinBody     string                     // scenarios only
    Confidence      float32
    Orphaned        bool
    DescriptionHash string                     // sha256 of Description for re-parse trigger
    ReviewStatus    ReviewStatus               // "accepted" | "pending" | "rejected" (default accepted)
    LastSyncedAt    time.Time
}

func NewRequirement(...) (*Requirement, error)         // validating constructor
func ReconstructRequirement(...) *Requirement          // repo hydration
func (r *Requirement) MarkOrphaned()
func (r *Requirement) UpdateFromJira(issue JiraIssue)  // returns true if changed
func (r *Requirement) Snapshot() RequirementSnapshot   // read-only view
```

**`SyncRun` aggregate**:

```go
type SyncRun struct {
    ID              SyncRunID
    ProjectID       string
    TriggerSource   TriggerSource    // "initial" | "incremental" | "staleness"
    InitiatedBy     string           // user_id or "system"
    StartedAt       time.Time
    CompletedAt     *time.Time
    Status          SyncRunStatus    // "running" | "succeeded" | "failed" | "cancelled"
    TotalCount      int
    ProcessedCount  int
    SucceededCount  int
    FailedCount     int
    MappingsCreated int
    ErrorSummary    string
    ConfigSnapshot  json.RawMessage  // JQL + types + window at run time
}
```

**`ProjectJiraSyncSettings` aggregate** (one row per project):

```go
type ProjectJiraSyncSettings struct {
    ProjectID                   string
    ReleaseWindowDays           int                  // 120 default
    DefaultJQL                  string
    DefaultIssueTypes           []string             // {epic,story,task,bug,subtask}
    StalenessThresholdHours     int                  // 24 default
    GherkinLLMExtractionEnabled bool                 // false default
    LLMProvider                 string               // empty | "anthropic" | "openai" | "bedrock" | "local"
}
```

**`SpecRunRequirement` value type** (no behavior, just data):

```go
type SpecRunRequirement struct {
    SpecRunID     uint
    RequirementID RequirementID
    Source        MappingSource    // "tag" | "name_match" | "epic_fallback"
    Confidence    float32
}
```

### JiraClient extensions — `internal/domains/integrations/jira_client.go`

```go
// SearchIssues paginates over JIRA REST /search.
// Returns the issue page + total count + nextStartAt for follow-up calls.
SearchIssues(ctx, jql string, startAt, maxResults int) (IssuePage, error)

// GetIssuesByKeys fetches specific keys in batches (max 50/batch).
// Reserved for follow-on use; not called from initial #27 sync paths.
GetIssuesByKeys(ctx, keys []string) ([]JiraIssue, error)
```

Both honor rate-limit headers (`Retry-After`) and apply exponential backoff on 429/5xx (max 5 retries, capped at 30s).

### Repository interfaces — `internal/domains/requirements/domain/repository.go`

```go
type RequirementRepository interface {
    Upsert(ctx, req *Requirement) error                     // ON CONFLICT updates
    GetByJiraKey(ctx, projectID, jiraKey string) (*Requirement, error)
    GetByID(ctx, id RequirementID) (*Requirement, error)
    ListByProject(ctx, projectID string, filter ListFilter) ([]*Requirement, error)
    ListChildren(ctx, parentID RequirementID) ([]*Requirement, error)
    MarkOrphaned(ctx, projectID string, missingKeys []string) error
    GetKeysWithStaleDescription(ctx, projectID string, freshHashes map[string]string) ([]RequirementID, error)
}

type SyncRunRepository interface {
    Create(ctx, run *SyncRun) error
    UpdateStatus(ctx, id SyncRunID, fields SyncRunUpdate) error
    GetByID(ctx, id SyncRunID) (*SyncRun, error)
    ListByProject(ctx, projectID string, limit, offset int) ([]*SyncRun, error)
    HasRunning(ctx, projectID string, kind TriggerSource) (bool, error)  // for concurrency guard
}

type SpecRunRequirementRepository interface {
    BackfillForKeys(ctx, projectID string, keys []string, windowDays int) (int, error)
    // Returns count of rows newly inserted. Idempotent via ON CONFLICT DO NOTHING.
}

type ProjectJiraSyncSettingsRepository interface {
    Get(ctx, projectID string) (*ProjectJiraSyncSettings, error)        // returns defaults if no row
    Save(ctx, settings *ProjectJiraSyncSettings) error
}
```

### GraphQL surface — `internal/reporter/graphql/schema.graphql`

```graphql
# Types
enum SyncRunStatus { RUNNING SUCCEEDED FAILED CANCELLED }
enum SyncTriggerSource { INITIAL INCREMENTAL STALENESS }
enum RequirementType { EPIC STORY TASK BUG SUBTASK SCENARIO }
enum RequirementSource { JIRA_SYNC PARSED LLM_EXTRACTED }
enum MappingSource { TAG NAME_MATCH EPIC_FALLBACK }

type Requirement {
  id: ID!
  projectId: String!
  jiraKey: String
  source: RequirementSource!
  type: RequirementType!
  parentId: ID
  title: String!
  status: String
  fixVersion: String
  confidence: Float!
  orphaned: Boolean!
  lastSyncedAt: Time!
  scenarioCount: Int!    # only meaningful when type != SCENARIO
}

type JiraSyncRun {
  id: ID!
  projectId: String!
  triggerSource: SyncTriggerSource!
  initiatedBy: String!
  startedAt: Time!
  completedAt: Time
  status: SyncRunStatus!
  totalCount: Int!
  processedCount: Int!
  succeededCount: Int!
  failedCount: Int!
  mappingsCreated: Int!
  errorSummary: String
}

input StartJiraSyncInput {
  projectId: String!
  triggerSource: SyncTriggerSource!     # INITIAL or INCREMENTAL (STALENESS is system-only)
  issueTypes: [String!]                 # overrides project default
  jql: String                           # overrides project default
  releaseWindowDays: Int                # overrides project default
}

type ProjectJiraSyncSettings {
  projectId: String!
  releaseWindowDays: Int!
  defaultJql: String
  defaultIssueTypes: [String!]!
  stalenessThresholdHours: Int!
  gherkinLlmExtractionEnabled: Boolean!
  llmProvider: String
}

input ProjectJiraSyncSettingsInput {
  projectId: String!
  releaseWindowDays: Int
  defaultJql: String
  defaultIssueTypes: [String!]
  stalenessThresholdHours: Int
  gherkinLlmExtractionEnabled: Boolean
  llmProvider: String
}

# Queries
type Query {
  # ... existing ...
  requirements(projectId: String!, type: RequirementType, parentId: ID, after: String, first: Int): RequirementConnection!
  requirement(id: ID!): Requirement
  jiraSyncRun(id: ID!): JiraSyncRun
  jiraSyncRuns(projectId: String!, first: Int = 20): [JiraSyncRun!]!
  projectJiraSyncSettings(projectId: String!): ProjectJiraSyncSettings!
}

# Mutations
type Mutation {
  # ... existing ...
  startJiraSync(input: StartJiraSyncInput!): JiraSyncRun!
  cancelJiraSync(syncRunId: ID!): JiraSyncRun!
  saveProjectJiraSyncSettings(input: ProjectJiraSyncSettingsInput!): ProjectJiraSyncSettings!
}
```

Auth: all mutations and project-scoped queries follow the project-scoped manager check (the pattern from `domain_resolvers.go:620-644` — load project, check team membership + role group). Apply via a shared `authorizeProjectManagement(ctx, projectID)` helper (introduce it in this PR if PR #181 didn't already; otherwise reuse).

### Configuration

New env vars (read at startup, propagated into `SyncService`):

| Env var | Default | Purpose |
|---|---|---|
| `JIRA_SYNC_MAX_PAGE` | 100 | Page size for JIRA search |
| `JIRA_SYNC_MAX_KEYS_PER_BATCH` | 50 | Batch size for `GetIssuesByKeys` |
| `JIRA_SYNC_MAX_RETRIES` | 5 | Backoff retry cap on 429/5xx |
| `JIRA_SYNC_BACKOFF_MAX_SECONDS` | 30 | Backoff cap |
| `JIRA_SYNC_WORKER_CONCURRENCY` | 4 | Max concurrent sync workers (per node) |
| `GHERKIN_LLM_EXTRACTION_ENABLED` | false | Global override; per-project setting still gates use |
| `GHERKIN_LLM_PROVIDER` | (empty) | `anthropic` / `openai` / `bedrock` / `local` |
| `GHERKIN_LLM_API_KEY` | (empty) | provider-specific credential |

Per-project knobs live in `project_jira_sync_settings`; env vars are platform-wide defaults / kill switches.

## Architectural Decisions (References)

The cross-cutting decisions live as ADRs under `adr/test-correlation/`. They are referenced inline above and not duplicated here:

- [`tag-schema.md`](../../adr/test-correlation/tag-schema.md) — namespaced tags (`jira:`, `scenario:`, `release:`, `coverage:`); reporter normalization contract.
- [`mapping-lifecycle.md`](../../adr/test-correlation/mapping-lifecycle.md) — Path A (test ingest, #28) + Path B (sync backfill, this spec); idempotency via `UNIQUE (spec_run_id, requirement_id)`.
- [`gherkin-parsing-tiers.md`](../../adr/test-correlation/gherkin-parsing-tiers.md) — Tier 1 deterministic (`cucumber/gherkin`) always-on; Tier 2 LLM opt-in with human-in-the-loop confirmation, OSS-safe default.

Sync-specific decisions captured below (not ADR-grade, but worth flagging):

### Decision 1: One sync pipeline, three triggers (initial / incremental / staleness)

All three share the fetch → parse → upsert → backfill → summary flow. The triggers differ only in their JQL clause and (for `staleness`) their scoping to a specific Epic + descendants. Single pipeline = single code path to test, single audit trail, single concurrency lock.

### Decision 2: One migration for all four tables (000023)

Tables are introduced together as a coherent unit. Splitting per table buys nothing — they're all needed for any of the new code paths to work. Down migration reverses all four in FK-safe order.

### Decision 3: Description-hash-based re-parse, not blind reparse

Re-running the Gherkin parser on every sync is wasted work when most descriptions don't change. Storing `description_hash` (SHA-256 of the raw description) lets the parser skip unchanged descriptions cheaply. Re-parse only when hash differs from the stored value.

### Decision 4: Tier 2 (LLM) extraction lives behind two gates

Both `gherkin_llm_extraction_enabled` (per-project flag) AND `GHERKIN_LLM_EXTRACTION_ENABLED` (platform env) must be true. Platform operator can disable globally; project manager opts in per-project. OSS-safe default is off everywhere. Tier 2 extracted scenarios land with `review_status='pending'` and require human confirmation before they appear in the live tree.

### Decision 5: Polling, not subscriptions, for sync progress

The existing GraphQL setup doesn't have subscriptions configured. Adding them for this one use case is infrastructure overhead disproportionate to value (sync completion at minute-scale is fine to poll every few seconds). Revisit if Fern adds subscriptions for other features.

### Decision 6: Backfill inline within the sync job, not a separate background task

Atomic UX (one progress indicator), bounded work (release-window-clamped), no new queue infra. If backfill ever scales to minutes, split.

### Decision 7: New `requirements` bounded context, not extending `integrations`

`integrations` already owns JIRA connection + field mapping (from #26). The new aggregates — `Requirement`, `SyncRun`, `ProjectJiraSyncSettings` — have a distinct lifecycle: they outlive any single JIRA connection (a project could re-point to a different JIRA instance and requirements persist with `orphaned=true` flags). Promoting them into their own `requirements` bounded context keeps `integrations` focused on connection concerns and gives downstream consumers (#29, #30) a clear domain to depend on.

## Data Model

### `requirements`

| Column | Type | Constraints / Notes |
|---|---|---|
| `id` | `BIGSERIAL` | PK |
| `project_id` | `VARCHAR(36)` | NOT NULL · FK → `project_details(project_id)` ON DELETE CASCADE |
| `jira_key` | `VARCHAR(255)` | NULL for `source ∈ {parsed, llm_extracted}` |
| `source` | `VARCHAR(32)` | NOT NULL · CHECK IN (`'jira_sync'`, `'parsed'`, `'llm_extracted'`) |
| `type` | `VARCHAR(32)` | NOT NULL · CHECK IN (`'epic'`, `'story'`, `'task'`, `'bug'`, `'subtask'`, `'scenario'`) |
| `parent_id` | `BIGINT` | FK → `requirements(id)` ON DELETE CASCADE; nullable |
| `title` | `VARCHAR(1024)` | NOT NULL |
| `status` | `VARCHAR(64)` | nullable |
| `fix_version` | `VARCHAR(255)` | nullable |
| `description` | `TEXT` | raw markdown — `jira_sync` rows only |
| `gherkin_body` | `TEXT` | scenarios only |
| `confidence` | `REAL` | NOT NULL DEFAULT `1.0` |
| `orphaned` | `BOOLEAN` | NOT NULL DEFAULT `FALSE` |
| `description_hash` | `VARCHAR(64)` | nullable; sha256 of description for re-parse detection |
| `review_status` | `VARCHAR(16)` | NOT NULL DEFAULT `'accepted'` · CHECK IN (`'accepted'`, `'pending'`, `'rejected'`) |
| `last_synced_at` | `TIMESTAMP` | NOT NULL DEFAULT `NOW()` |
| `created_at`, `updated_at`, `deleted_at` | `TIMESTAMP` | standard audit + soft-delete |

Indexes: `UNIQUE (project_id, jira_key) WHERE source='jira_sync' AND deleted_at IS NULL`; `UNIQUE (project_id, parent_id, title) WHERE source IN ('parsed','llm_extracted') AND deleted_at IS NULL`; `(project_id, type)`; `(parent_id)`; `(jira_key)` for backfill lookups.

### `spec_run_requirements`

| Column | Type | Notes |
|---|---|---|
| `id` | `BIGSERIAL` | PK |
| `spec_run_id` | `BIGINT` | NOT NULL · FK → `spec_runs(id)` ON DELETE CASCADE |
| `requirement_id` | `BIGINT` | NOT NULL · FK → `requirements(id)` ON DELETE CASCADE |
| `source` | `VARCHAR(32)` | NOT NULL · CHECK IN (`'tag'`, `'name_match'`, `'epic_fallback'`) |
| `confidence` | `REAL` | NOT NULL |
| `created_at` | `TIMESTAMP` | NOT NULL DEFAULT `NOW()` |

Indexes: `UNIQUE (spec_run_id, requirement_id)` — enforces idempotency per [`mapping-lifecycle.md`](../../adr/test-correlation/mapping-lifecycle.md); `(requirement_id)`; `(spec_run_id)`.

### `sync_runs`

| Column | Type | Notes |
|---|---|---|
| `id` | `BIGSERIAL` | PK |
| `project_id` | `VARCHAR(36)` | NOT NULL · FK → `project_details(project_id)` ON DELETE CASCADE |
| `trigger_source` | `VARCHAR(32)` | NOT NULL · CHECK IN (`'initial'`, `'incremental'`, `'staleness'`) |
| `initiated_by` | `VARCHAR(255)` | NOT NULL |
| `started_at` | `TIMESTAMP` | NOT NULL DEFAULT `NOW()` |
| `completed_at` | `TIMESTAMP` | nullable |
| `status` | `VARCHAR(32)` | NOT NULL DEFAULT `'running'` · CHECK IN (`'running'`, `'succeeded'`, `'failed'`, `'cancelled'`) |
| `total_count`, `processed_count`, `succeeded_count`, `failed_count`, `mappings_created` | `INTEGER` | NOT NULL DEFAULT `0` |
| `error_summary` | `TEXT` | nullable |
| `config_snapshot` | `JSONB` | NOT NULL — JQL + issue types + window at run time |

Indexes: `(project_id, started_at DESC)`; partial `UNIQUE (project_id, trigger_source) WHERE status='running'` — enforces Req 6.2 concurrency guard at the DB level.

### `project_jira_sync_settings`

| Column | Type | Notes |
|---|---|---|
| `project_id` | `VARCHAR(36)` | PK · FK → `project_details(project_id)` ON DELETE CASCADE |
| `release_window_days` | `INTEGER` | NOT NULL DEFAULT `120` · CHECK BETWEEN 7 AND 1825 |
| `default_jql` | `TEXT` | nullable |
| `default_issue_types` | `TEXT[]` | NOT NULL DEFAULT `'{}'` |
| `staleness_threshold_hours` | `INTEGER` | NOT NULL DEFAULT `24` |
| `gherkin_llm_extraction_enabled` | `BOOLEAN` | NOT NULL DEFAULT `FALSE` |
| `llm_provider` | `VARCHAR(64)` | nullable |
| `created_at`, `updated_at` | `TIMESTAMP` | standard audit |

Single row per project; auto-created on first save with defaults if missing.

## Error Handling

- **Per-issue failure** (one JIRA issue can't be persisted): log, increment `failed_count`, continue with remaining issues. Never abort the whole sync.
- **JIRA unreachable mid-sync**: retry with backoff up to `JIRA_SYNC_MAX_RETRIES`; if still failing, mark sync `status='failed'`, write `error_summary`, preserve any successfully-synced rows (no rollback).
- **Concurrency violation** (Req 6.2): partial unique index on `sync_runs (project_id, trigger_source) WHERE status='running'` rejects the second concurrent same-kind start; GraphQL mutation returns a clear "sync of this kind already in progress" error.
- **LLM extraction failure** (Tier 2): silently fall back to "no scenarios extracted for this issue this sync"; log a warn-level event; don't fail the sync.
- **Auth/credential failure on JIRA**: fail the sync run with `error_summary='JIRA authentication failed; check connection credentials'`. Decryption is handled by the existing `jira_connections` flow.
- **Validation error on issue type or JQL**: surface JIRA's own error response in `error_summary`; fail fast at sync start, not partway through.

All errors recorded in `sync_runs.error_summary`. UI shows the summary on the completion panel.

## Testing Approach

### Unit tests (Ginkgo v2 + Gomega)

- **Domain aggregates**: invariants — empty title rejected, scenario without parent rejected, status transitions validated.
- **Gherkin parser**: golden-file tests using DE-5249 and GWCP-97108 descriptions as fixtures (committed under `internal/domains/requirements/application/testdata/`).
- **Backfill service**: in-memory repo stubs; verify idempotency on re-run, verify confidence-source selection.
- **Sync service**: mock JIRA client + mock repo; verify the five-step pipeline including the description-hash skip-reparse path.

### Integration tests

- **Repositories**: GORM against an in-memory SQLite or testcontainer Postgres; verify SQL generation matches expectations (especially the partial unique indexes and ON CONFLICT clauses).
- **Concurrency guard**: spawn two goroutines calling `startJiraSync` for the same project + kind; verify only one succeeds, the other gets the rejection.

### Acceptance tests (`acceptance/jira_sync_test.go`)

- **Full sync flow**: extend `mock_jira_server.go` to serve a fixture project of ~250 issues across pages. Trigger initial sync, poll status, verify final counts match fixture.
- **Incremental sync preserves associations**: seed a `spec_run_requirement` row; run incremental sync; verify the row is untouched.
- **Path B backfill**: pre-seed `spec_run` rows with `jira:KEY` tags for an unknown key; run sync that includes that key; verify `spec_run_requirement` rows are created with `source='tag'`, `confidence=1.0`.
- **Gherkin parsing**: sync an issue whose description contains 3 Gherkin scenarios; verify 3 `requirement` rows of type `scenario` exist with correct `parent_id`.
- **Orphan handling**: sync, delete an issue from mock JIRA, sync again; verify the row exists with `orphaned=true` and any `spec_run_requirement` rows remain.

### Smoke tests

Manual checklist in PR template:
- [ ] Start initial sync from the UI, watch progress bar to completion
- [ ] Open inventory page, verify list renders + expansion shows scenarios
- [ ] Re-run as incremental sync; verify per-row "synced N minutes ago" updates
- [ ] Manually edit a project's `release_window_days` setting; confirm the next sync respects it
