-- Migration: Rollback composite index on test_runs(project_id, start_time)
DROP INDEX IF EXISTS idx_test_runs_project_id_start_time;
