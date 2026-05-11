-- Migration: Add Performance Indexes
-- Description: Add indexes to improve GraphQL query performance by eliminating N+1 queries
-- Related: Performance optimization PR - reduces 1.4s queries to <50ms
-- See: PERFORMANCE_FIX.md for full details

-- High Priority: Fix N+1 queries for nested GraphQL resolvers
-- These queries were taking 1.4-1.7s each without indexes
CREATE INDEX IF NOT EXISTS idx_suite_runs_test_run_id ON suite_runs(test_run_id);
CREATE INDEX IF NOT EXISTS idx_spec_runs_suite_run_id ON spec_runs(suite_run_id);

-- Medium Priority: Speed up tag resolution
CREATE INDEX IF NOT EXISTS idx_spec_run_tags_spec_run_id ON spec_run_tags(spec_run_id);
CREATE INDEX IF NOT EXISTS idx_suite_run_tags_suite_run_id ON suite_run_tags(suite_run_id);

-- Low Priority: Common query patterns
CREATE INDEX IF NOT EXISTS idx_test_runs_project_id ON test_runs(project_id);
CREATE INDEX IF NOT EXISTS idx_test_runs_start_time ON test_runs(start_time);
