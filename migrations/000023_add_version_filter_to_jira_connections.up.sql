ALTER TABLE jira_connections ADD COLUMN version_filter VARCHAR(500) NOT NULL DEFAULT '';
COMMENT ON COLUMN jira_connections.version_filter IS 'Comma-separated version name prefixes to show in the Coverage picker (empty = show all)';
