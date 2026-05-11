-- Migration: Rollback Performance Indexes
-- Drop indexes added for performance optimization

DROP INDEX IF EXISTS idx_test_runs_start_time;
DROP INDEX IF EXISTS idx_test_runs_project_id;
DROP INDEX IF EXISTS idx_suite_run_tags_suite_run_id;
DROP INDEX IF EXISTS idx_spec_run_tags_spec_run_id;
DROP INDEX IF EXISTS idx_spec_runs_suite_run_id;
DROP INDEX IF EXISTS idx_suite_runs_test_run_id;
