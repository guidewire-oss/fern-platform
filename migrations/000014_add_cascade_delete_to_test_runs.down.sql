-- Remove the foreign key constraint
ALTER TABLE test_runs 
DROP CONSTRAINT IF EXISTS fk_test_runs_project_id;