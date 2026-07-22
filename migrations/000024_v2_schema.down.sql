-- Reverse of 000024_v2_schema.up.sql. The pg_trgm extension is left
-- in place since other code may rely on it being installed.
DROP TABLE IF EXISTS saved_views;
DROP INDEX IF EXISTS idx_spec_runs_search_trgm;
DROP INDEX IF EXISTS idx_test_runs_project_branch;
DROP INDEX IF EXISTS idx_test_runs_failed_started;
DROP INDEX IF EXISTS idx_test_runs_keyset;
DROP INDEX IF EXISTS idx_test_runs_project_started_desc;
