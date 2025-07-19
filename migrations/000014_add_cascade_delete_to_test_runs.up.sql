-- Add foreign key constraint to cascade delete test runs when project is deleted

-- First, let's clean up any orphaned test runs (where project doesn't exist)
DELETE FROM test_runs 
WHERE project_id NOT IN (
    SELECT project_id FROM project_details WHERE deleted_at IS NULL
);

-- Now add the foreign key constraint with CASCADE DELETE
ALTER TABLE test_runs 
ADD CONSTRAINT fk_test_runs_project_id 
FOREIGN KEY (project_id) 
REFERENCES project_details(project_id) 
ON DELETE CASCADE;

-- Add a comment explaining the cascade behavior
COMMENT ON CONSTRAINT fk_test_runs_project_id ON test_runs IS 
'Ensures test runs are automatically deleted when the associated project is deleted';