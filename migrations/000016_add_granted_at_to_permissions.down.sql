-- Remove granted_at column from project_permissions table
ALTER TABLE project_permissions DROP COLUMN IF EXISTS granted_at;