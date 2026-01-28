-- Create suite_run_tags junction table
CREATE TABLE IF NOT EXISTS suite_run_tags (
    suite_run_id BIGINT NOT NULL,
    tag_id BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    PRIMARY KEY (suite_run_id, tag_id),
    CONSTRAINT fk_suite_run_tags_suite_run_id
        FOREIGN KEY (suite_run_id)
        REFERENCES suite_runs(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_suite_run_tags_tag_id
        FOREIGN KEY (tag_id)
        REFERENCES tags(id)
        ON DELETE CASCADE
);

-- Create indexes for suite_run_tags
CREATE INDEX IF NOT EXISTS idx_suite_run_tags_suite_run_id ON suite_run_tags(suite_run_id);
CREATE INDEX IF NOT EXISTS idx_suite_run_tags_tag_id ON suite_run_tags(tag_id);
