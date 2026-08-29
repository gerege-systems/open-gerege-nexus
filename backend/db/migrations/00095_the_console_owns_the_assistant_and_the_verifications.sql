-- +goose Up

-- The operator console takes over two screens the workspace used to carry: the
-- assistant's shared prompts and knowledge, and the email-verification ledger.
--
-- Both live in the workspace schema and both are behind FORCE row level
-- security, so a GRANT alone shows the console nothing: the tenant policy names
-- gerege_nexus_app, and a role with no policy sees no rows. Each table needs a
-- policy for the console's own role, and each policy says exactly how much of
-- the table the console owns.

GRANT SELECT, INSERT, UPDATE, DELETE ON workspace.ai_prompts   TO gerege_nexus_operator;
GRANT SELECT, INSERT, UPDATE, DELETE ON workspace.ai_knowledge TO gerege_nexus_operator;
GRANT SELECT                          ON workspace.email_verifications TO gerege_nexus_operator;

-- The shared rows, and only those. `tenant_id IS NULL` in USING and again in
-- WITH CHECK: an operator can neither read an organisation's own prompt nor
-- write one, and the console cannot turn a platform row into a tenant's by
-- setting the column.
DROP POLICY IF EXISTS operator_shared ON workspace.ai_prompts;
CREATE POLICY operator_shared ON workspace.ai_prompts TO gerege_nexus_operator
    USING (tenant_id IS NULL) WITH CHECK (tenant_id IS NULL);

DROP POLICY IF EXISTS operator_shared ON workspace.ai_knowledge;
CREATE POLICY operator_shared ON workspace.ai_knowledge TO gerege_nexus_operator
    USING (tenant_id IS NULL) WITH CHECK (tenant_id IS NULL);

-- Who the platform has written to is a platform-wide question, so this one is
-- every row — and personal data, so it is SELECT and nothing else. Nobody
-- deletes a verification from the console; housekeeping ages them out.
DROP POLICY IF EXISTS operator_read ON workspace.email_verifications;
CREATE POLICY operator_read ON workspace.email_verifications FOR SELECT TO gerege_nexus_operator
    USING (true);

-- A shared prompt is one row per key, and UNIQUE (tenant_id, prompt_key) does
-- not enforce that: two NULLs are distinct in Postgres, so the constraint
-- allows a second global 'scope' — and the copilot, which takes the NULL row
-- first, would then take whichever of them the plan happened to reach. The
-- console's save is an UPDATE that falls back to an INSERT, which is only safe
-- if there cannot be two.
-- Any deployment that already has two is deduplicated first, newest kept, id
-- as the tiebreaker so two rows written in the same transaction still resolve.
DELETE FROM workspace.ai_prompts a
      USING workspace.ai_prompts b
      WHERE a.tenant_id IS NULL AND b.tenant_id IS NULL
        AND a.prompt_key = b.prompt_key
        AND (a.updated_at, a.id) < (b.updated_at, b.id);

CREATE UNIQUE INDEX IF NOT EXISTS ai_prompts_one_shared_per_key
    ON workspace.ai_prompts (prompt_key) WHERE tenant_id IS NULL;

-- +goose Down

DROP INDEX IF EXISTS workspace.ai_prompts_one_shared_per_key;
DROP POLICY IF EXISTS operator_read ON workspace.email_verifications;
DROP POLICY IF EXISTS operator_shared ON workspace.ai_knowledge;
DROP POLICY IF EXISTS operator_shared ON workspace.ai_prompts;
REVOKE ALL ON workspace.email_verifications FROM gerege_nexus_operator;
REVOKE ALL ON workspace.ai_knowledge FROM gerege_nexus_operator;
REVOKE ALL ON workspace.ai_prompts FROM gerege_nexus_operator;
