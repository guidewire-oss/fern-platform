-- Postgres init script for the docker-compose test stack.
-- Postgres entrypoint runs this exactly once on first boot (when the
-- volume is fresh) for any *.sql in /docker-entrypoint-initdb.d.
--
-- Why this exists: a handful of legacy migrations (15, 16, ...) end
-- with `ALTER TABLE ... OWNER TO app; GRANT ALL ... TO app;` because
-- the k3d/CloudNativePG deployment uses an "app" role by convention.
-- Plain Postgres doesn't have it, so the migrations error out unless
-- we pre-create the role.
--
-- We make `app` a NOLOGIN role and explicitly grant `postgres` the
-- ability to set it as an owner. The fern container still authenticates
-- as `postgres`; this role exists only so OWNER TO / GRANT TO clauses
-- don't fail.

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app') THEN
        CREATE ROLE app NOLOGIN;
    END IF;
END
$$;

GRANT app TO postgres;
