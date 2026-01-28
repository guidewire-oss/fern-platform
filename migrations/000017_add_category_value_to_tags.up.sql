-- Add category and value columns to tags table
ALTER TABLE tags
ADD COLUMN IF NOT EXISTS category VARCHAR(255),
ADD COLUMN IF NOT EXISTS value VARCHAR(255);

-- Create indexes for category and value
CREATE INDEX IF NOT EXISTS idx_tags_category ON tags(category);
CREATE INDEX IF NOT EXISTS idx_tags_category_value ON tags(category, value);
