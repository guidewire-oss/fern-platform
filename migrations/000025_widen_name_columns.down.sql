-- Lossy, best-effort rollback: any spec_name/suite_name/tag name longer
-- than 255 characters is truncated to fit rather than aborting the
-- migration and forcing an operator to find and delete/edit the
-- offending rows by hand first.
--
-- tags.name additionally carries a UNIQUE constraint. Two distinct long
-- tag names that happen to share the same first-255-character prefix
-- would collide once both are truncated to that prefix, which would
-- otherwise abort the ALTER with a duplicate-key error. Disambiguate
-- every tag row that needs truncation by appending "~<id>" -- id is the
-- primary key, so it's guaranteed unique per row regardless of what any
-- other row's name is, which means this can never itself produce a
-- collision. Only rows that actually need truncation are touched.
UPDATE tags
SET name = left(name, 255 - (length(id::text) + 1)) || '~' || id::text
WHERE length(name) > 255;

ALTER TABLE tags ALTER COLUMN name TYPE VARCHAR(255) USING left(name, 255);
ALTER TABLE suite_runs ALTER COLUMN suite_name TYPE VARCHAR(255) USING left(suite_name, 255);
ALTER TABLE spec_runs ALTER COLUMN spec_name TYPE VARCHAR(255) USING left(spec_name, 255);
