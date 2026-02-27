-- Migration: Add composite index on test_runs(project_id, start_time)
-- Description: FindByDateRangeForProjects filters on both project_id and start_time;
-- a composite index eliminates the full-scan on every treemap/dashboard request.
CREATE INDEX IF NOT EXISTS idx_test_runs_project_id_start_time ON test_runs(project_id, start_time);
