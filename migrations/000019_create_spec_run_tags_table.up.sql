-- Create spec_run_tags junction table
CREATE TABLE IF NOT EXISTS spec_run_tags (
    spec_run_id BIGINT NOT NULL,
    tag_id BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    PRIMARY KEY (spec_run_id, tag_id),
    CONSTRAINT fk_spec_run_tags_spec_run_id
        FOREIGN KEY (spec_run_id)
        REFERENCES spec_runs(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_spec_run_tags_tag_id
        FOREIGN KEY (tag_id)
        REFERENCES tags(id)
        ON DELETE CASCADE
);

-- Create indexes for spec_run_tags
CREATE INDEX IF NOT EXISTS idx_spec_run_tags_spec_run_id ON spec_run_tags(spec_run_id);
CREATE INDEX IF NOT EXISTS idx_spec_run_tags_tag_id ON spec_run_tags(tag_id);
