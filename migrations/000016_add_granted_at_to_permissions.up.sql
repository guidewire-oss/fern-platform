-- Add granted_at column to project_permissions table
ALTER TABLE project_permissions ADD COLUMN IF NOT EXISTS granted_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP;