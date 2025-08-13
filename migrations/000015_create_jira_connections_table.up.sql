-- Create JIRA connections table
CREATE TABLE IF NOT EXISTS jira_connections (
    id BIGSERIAL PRIMARY KEY,
    project_id VARCHAR(36) NOT NULL,
    name VARCHAR(255) NOT NULL,
    jira_url VARCHAR(500) NOT NULL,
    authentication_type VARCHAR(50) NOT NULL CHECK (authentication_type IN ('api_token', 'oauth', 'personal_access_token')),
    project_key VARCHAR(50) NOT NULL,
    username VARCHAR(255) NOT NULL,
    encrypted_credential TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'connected', 'failed')),
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    last_tested_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    
    -- Foreign key constraint
    CONSTRAINT fk_jira_connection_project
        FOREIGN KEY (project_id) 
        REFERENCES project_details(project_id) 
        ON DELETE CASCADE
);

-- Create indexes
CREATE INDEX idx_jira_connection_project_id ON jira_connections (project_id);
CREATE INDEX idx_jira_connection_status ON jira_connections (status);
CREATE INDEX idx_jira_connection_is_active ON jira_connections (is_active);
CREATE INDEX idx_jira_connection_deleted_at ON jira_connections (deleted_at);

-- Add comment on table
COMMENT ON TABLE jira_connections IS 'Stores JIRA integration connections for projects';

-- Add column comments
COMMENT ON COLUMN jira_connections.id IS 'Unique identifier for the connection';
COMMENT ON COLUMN jira_connections.project_id IS 'Reference to the project this connection belongs to';
COMMENT ON COLUMN jira_connections.name IS 'User-friendly name for the connection';
COMMENT ON COLUMN jira_connections.jira_url IS 'Base URL of the JIRA instance';
COMMENT ON COLUMN jira_connections.authentication_type IS 'Type of authentication used (api_token, oauth, personal_access_token)';
COMMENT ON COLUMN jira_connections.project_key IS 'JIRA project key';
COMMENT ON COLUMN jira_connections.username IS 'Username or email for authentication';
COMMENT ON COLUMN jira_connections.encrypted_credential IS 'Encrypted authentication credential (token/password)';
COMMENT ON COLUMN jira_connections.status IS 'Current connection status';
COMMENT ON COLUMN jira_connections.is_active IS 'Whether the connection is active';
COMMENT ON COLUMN jira_connections.last_tested_at IS 'Timestamp of last connection test';
COMMENT ON COLUMN jira_connections.deleted_at IS 'Soft delete timestamp';

-- Grant permissions to app user
ALTER TABLE jira_connections OWNER TO app;
GRANT ALL PRIVILEGES ON TABLE jira_connections TO app;