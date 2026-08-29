-- +goose Up

-- A database created a second time in a cluster that already ran these
-- migrations comes up unusable, and says so in a way that reads like anything
-- but its cause: `permission denied for table users`.
--
-- Roles are cluster-wide; databases are not. 00079 renames gerege_nexus_app to
-- gerege_nexus_tenant only `IF NOT EXISTS (tenant)`, which is right the first
-- time and skipped every time after, because the role from the first database
-- is still there. So in the second database every grant written before 00079 —
-- and every row-level policy naming that role — lands on gerege_nexus_app,
-- while the platform connects and SET ROLEs to gerege_nexus_tenant. The two
-- halves never meet.
--
-- It is not hypothetical: nexus.gerege.mn was rebuilt from empty on
-- 2026-08-29 to test the first-run wizard, and came up with 220 privileges on
-- the old role and 11 on the new one. Signing in worked; every screen after it
-- answered 500.
--
-- This migration makes the second database come up like the first: whatever
-- the old role still holds is moved to the new one, and the old role is taken
-- out of the cluster so the next database cannot inherit the same confusion.

-- +goose StatementBegin
DO $move$
DECLARE
    grant_row  RECORD;
    policy_row RECORD;
    acl_row    RECORD;
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'gerege_nexus_app') THEN
        -- The ordinary case: one database in the cluster, 00079 renamed the
        -- role, there is nothing here to move.
        RETURN;
    END IF;

    -- Table privileges, one GRANT per privilege actually held. Read from the
    -- catalogue rather than repeated from the earlier migrations: what the old
    -- role holds is the whole truth about what the new one is missing, and a
    -- list written here would drift from the twenty migrations that wrote it.
    FOR grant_row IN
        SELECT DISTINCT table_schema, table_name, privilege_type
          FROM information_schema.role_table_grants
         WHERE grantee = 'gerege_nexus_app'
    LOOP
        EXECUTE format('GRANT %s ON %I.%I TO gerege_nexus_tenant',
                       grant_row.privilege_type, grant_row.table_schema, grant_row.table_name);
    END LOOP;

    -- Schemas. USAGE is what makes a name inside them resolvable at all, so a
    -- table grant without it is a grant on something that cannot be reached.
    FOR grant_row IN
        SELECT nspname FROM pg_namespace
         WHERE nspname NOT LIKE 'pg\_%' AND nspname <> 'information_schema'
           AND has_schema_privilege('gerege_nexus_app', oid, 'USAGE')
    LOOP
        EXECUTE format('GRANT USAGE ON SCHEMA %I TO gerege_nexus_tenant', grant_row.nspname);
    END LOOP;

    -- Sequences, which the inserts behind those tables need.
    --
    -- The privilege is asked for inside the loop rather than in the WHERE
    -- clause: a planner is free to evaluate the two conditions in either
    -- order, and has_sequence_privilege() on an index — which is what
    -- `relkind = 'S'` was there to exclude — raises rather than answering
    -- false. It did, on the first run of this migration.
    FOR grant_row IN
        SELECT schemaname AS nspname, sequencename AS relname FROM pg_sequences
    LOOP
        IF has_sequence_privilege('gerege_nexus_app',
                                  format('%I.%I', grant_row.nspname, grant_row.relname)::regclass, 'USAGE') THEN
            EXECUTE format('GRANT USAGE, SELECT ON SEQUENCE %I.%I TO gerege_nexus_tenant',
                           grant_row.nspname, grant_row.relname);
        END IF;
    END LOOP;

    -- Row-level policies. A policy stores its roles by OID, which is why
    -- 00079's rename carries them and this case does not: here the OID is the
    -- old role's, so the new one is simply not in the list and FORCE row level
    -- security answers with no rows — data that looks deleted rather than
    -- privileges that look missing.
    FOR policy_row IN
        SELECT schemaname, tablename, policyname
          FROM pg_policies
         WHERE 'gerege_nexus_app' = ANY(roles)
    LOOP
        EXECUTE format('ALTER POLICY %I ON %I.%I TO gerege_nexus_tenant',
                       policy_row.policyname, policy_row.schemaname, policy_row.tablename);
    END LOOP;

    -- Default privileges: what a table created later is granted automatically.
    -- Without these a module that brings its own schema installs fine and is
    -- unreadable, one release after anybody would connect the two.
    FOR acl_row IN
        SELECT DISTINCT n.nspname
          FROM pg_default_acl d
          JOIN pg_namespace n ON n.oid = d.defaclnamespace
         WHERE array_to_string(d.defaclacl, ',') LIKE '%gerege_nexus_app=%'
    LOOP
        EXECUTE format('ALTER DEFAULT PRIVILEGES IN SCHEMA %I '
                       'GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO gerege_nexus_tenant', acl_row.nspname);
        EXECUTE format('ALTER DEFAULT PRIVILEGES IN SCHEMA %I '
                       'GRANT USAGE, SELECT ON SEQUENCES TO gerege_nexus_tenant', acl_row.nspname);
    END LOOP;

    -- And then the old role goes, so the next database in this cluster is the
    -- first one again. DROP OWNED clears what it still holds here; the role
    -- itself may survive if another database in the cluster is using it, which
    -- is a deployment nobody has and a failure nobody should stop a migration
    -- for.
    EXECUTE 'DROP OWNED BY gerege_nexus_app';
    BEGIN
        EXECUTE 'DROP ROLE gerege_nexus_app';
    EXCEPTION WHEN dependent_objects_still_exist OR insufficient_privilege THEN
        RAISE WARNING 'gerege_nexus_app still holds objects elsewhere in this cluster; left in place';
    END;
END
$move$;
-- +goose StatementEnd

-- The database's own search_path, re-asserted.
--
-- 00084 sets it, and `pg_dump` of a single database does not carry it: a
-- deployment restored from a dump has every table and no search_path, and the
-- queries that name no schema — `roles`, in the path that gives a person their
-- own home — fail with `relation "roles" does not exist`. Setting it again
-- costs nothing where it is already right.
-- +goose StatementBegin
DO $search_path$
BEGIN
    EXECUTE format('ALTER DATABASE %I SET search_path = workspace, registry, operator',
                   current_database());
END
$search_path$;
-- +goose StatementEnd

-- The console's front page reads which migration this database has actually
-- seen. It never had the grant for it, so that panel has been quietly warning
-- `permission denied for table goose_db_version` on every deployment and
-- showing a blank where the schema version belongs.
GRANT SELECT ON public.goose_db_version TO gerege_nexus_operator;

-- +goose Down

REVOKE SELECT ON public.goose_db_version FROM gerege_nexus_operator;
-- The moved privileges and the dropped role are not restored: putting them
-- back would recreate a database that cannot serve a request.
