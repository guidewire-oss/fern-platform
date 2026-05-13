-- Create JIRA field mappings table
CREATE TABLE IF NOT EXISTS jira_field_mappings (
    id         BIGSERIAL    PRIMARY KEY,
    project_id VARCHAR(36)  NOT NULL,
    entries    JSONB        NOT NULL DEFAULT '[]',
    updated_by VARCHAR(255) NOT NULL DEFAULT '',
    created_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,

    -- Foreign key constraint
    CONSTRAINT fk_jira_field_mappings_project
        FOREIGN KEY (project_id)
        REFERENCES project_details(project_id)
        ON DELETE CASCADE
);

-- Unique partial index: one active mapping per project
CREATE UNIQUE INDEX idx_jira_field_mappings_project_id
    ON jira_field_mappings (project_id)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_jira_field_mappings_deleted_at ON jira_field_mappings (deleted_at);

-- Grant permissions to app user
ALTER TABLE jira_field_mappings OWNER TO app;
GRANT ALL PRIVILEGES ON TABLE jira_field_mappings TO app;
