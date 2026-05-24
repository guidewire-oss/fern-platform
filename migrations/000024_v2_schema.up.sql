-- Migration: v2 schema bundle
-- Description: All schema additions required by the v2 list / filter /
--              treemap / search code paths. Consolidated into a single
--              migration before v2 ships so operators only see one
--              "v2 schema" change in their migration history.
--
-- Lock profile: index builds take ACCESS EXCLUSIVE on the indexed
-- table for the build's duration. On large prod databases run the
-- equivalent CONCURRENTLY commands via scripts/v2-preflight-indexes.sh
-- BEFORE deploying — afterwards the IF NOT EXISTS guards below make
-- this migration a no-op in milliseconds.
--
-- Idempotent: every statement uses IF [NOT] EXISTS.
-- Forward-only: nothing is renamed or dropped from the v1 schema.

-- ─── Indexes for the filtered list path (/api/v2/test-runs) ────────────────

-- Project + descending time supports the per-project recent-runs view
-- and the project-scoped keyset pagination plan.
CREATE INDEX IF NOT EXISTS idx_test_runs_project_started_desc
    ON test_runs (project_id, start_time DESC);

-- Cross-project keyset pagination orders by (start_time, id) DESC.
CREATE INDEX IF NOT EXISTS idx_test_runs_keyset
    ON test_runs (start_time DESC, id DESC);

-- Triage path: "failed or flaky in the last week" is the first thing
-- an SRE filters on, so a partial index keeps that query cheap even
-- as the table grows.
CREATE INDEX IF NOT EXISTS idx_test_runs_failed_started
    ON test_runs (start_time DESC)
    WHERE status IN ('failed', 'flaky');

-- Branch facet lookup is always project-scoped.
CREATE INDEX IF NOT EXISTS idx_test_runs_project_branch
    ON test_runs (project_id, branch);

-- ─── Substring search over spec output ─────────────────────────────────────
--
-- Error messages and spec names are not English prose — they're
-- camelCase identifiers and stack-trace fragments. The English FTS
-- dictionary stems "DataIntegrityViolationException" to a single
-- token, which is the opposite of what users want. Trigrams give
-- us substring matching ('ILIKE %foo%') with index-backed speed.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS idx_spec_runs_search_trgm
    ON spec_runs
    USING GIN (
        (COALESCE(spec_name, '') || ' ' || COALESCE(error_message, ''))
        gin_trgm_ops
    );

-- ─── Saved views: user-scoped filter presets ───────────────────────────────
CREATE TABLE IF NOT EXISTS saved_views (
    id          BIGSERIAL PRIMARY KEY,
    user_id     VARCHAR(255) NOT NULL
                REFERENCES users(user_id) ON DELETE CASCADE,
    page        VARCHAR(64)  NOT NULL,
    name        VARCHAR(255) NOT NULL,
    filter_json JSONB        NOT NULL,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT saved_views_unique_per_user UNIQUE (user_id, page, name)
);

CREATE INDEX IF NOT EXISTS idx_saved_views_user_page
    ON saved_views (user_id, page);
