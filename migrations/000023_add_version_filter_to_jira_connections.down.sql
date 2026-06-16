-- WARNING: This rollback is destructive. Any version_filter values configured
-- by users will be permanently lost. Take a backup before running this migration
-- if the data needs to be preserved.
ALTER TABLE jira_connections DROP COLUMN version_filter;
