-- Drop indexes
DROP INDEX IF EXISTS idx_tags_category_value;
DROP INDEX IF EXISTS idx_tags_category;

-- Remove category and value columns from tags table
ALTER TABLE tags
DROP COLUMN IF EXISTS value,
DROP COLUMN IF EXISTS category;
