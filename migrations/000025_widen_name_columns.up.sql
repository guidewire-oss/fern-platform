-- Widen name columns that were capped at VARCHAR(255) with no
-- truncation/validation before insert. BDD frameworks (Ginkgo, Jasmine,
-- RSpec, ...) build spec/suite names from nested Describe/Context/It
-- text and routinely exceed 255 characters, which made Postgres reject
-- the whole batch insert with "value too long for type character
-- varying(255)" (SQLSTATE 22001) -- silently discarding every spec in
-- the submission, including ones that ran and passed.
-- See https://github.com/guidewire-oss/fern-platform/issues/230.
ALTER TABLE spec_runs ALTER COLUMN spec_name TYPE TEXT;
ALTER TABLE suite_runs ALTER COLUMN suite_name TYPE TEXT;
ALTER TABLE tags ALTER COLUMN name TYPE TEXT;
