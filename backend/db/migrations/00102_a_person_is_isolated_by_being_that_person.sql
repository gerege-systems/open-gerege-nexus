-- Хүнээр тусгаарлагдах гурван хүснэгт, ба платформын гарын үсгийн түлхүүр.
--
--
-- ЮУ ХӨНДӨГДӨӨГҮЙ БАЙВ.
--
-- 00029 нь `tenant_id` баганатай бүхнийг, 00093 нь `person_items`-ыг хүнээр
-- хамгаалсан. Гэвч `registry.users` болон түүний хоёр танилтын хүснэгт
-- (`user_sso_identities`, `user_eid_identities`) хоёулаа `tenant_id`-гүй,
-- хүнээр ч түгжигдээгүй үлдсэн бөгөөд тенантын роль тэдгээр дээр
-- SELECT/INSERT/UPDATE/DELETE-тэй байв.
--
-- Өөрөөр хэлбэл заалтаа мартсан нэг handler бүх суулгацын хэрэглэгчийн
-- жагсаалт, тэдний и-мэйл, тэдний eID/SSO холбоосыг буцааж чадна. Апп талын
-- `WHERE` хэвээр хамгаалсаар — dbguard-ийн хэлдгээр тэр нь үндсэн хамгаалалт —
-- харин доод давхарга энд байгаагүй.
--
--
-- ДҮРЭМ: ӨӨРӨӨ, ЭСВЭЛ ХАМТ АЖИЛЛАДАГ ХҮН.
--
--     USING (id = app.current_user  OR  EXISTS (гишүүнчлэл))
--
-- `workspace.memberships` нь өөрөө RLS-ийн дор (00037-ийн өргөн хэлбэр) тул
-- EXISTS нь дуудагчийн байгууллагуудын гишүүнчлэлийг л олно. Ингэснээр:
--
--   * хүн өөрийгөө үргэлж хардаг (профайл, төхөөрөмж, тохиргоо);
--   * байгууллагын админ гишүүдээ хардаг (эрхийн дэлгэц, аудитын нэр);
--   * хөрш байгууллагын хэн нэгэн харагдахгүй.
--
-- Тенантгүй боловч хүнтэй холболт (dbguard-ийн «person path») энд яг зөв
-- ажиллана: `app.current_user` тавигдсан, `allowed_tenants` хоосон тул EXISTS
-- юу ч олохгүй бөгөөд хүн зөвхөн өөрийгөө хардаг.
--
-- Нэвтрэлт, бүртгэл, session сэргээх, урилга зэрэг нь платформын зам дээр
-- (login role) ажилладаг тул эдгээр бодлогод огт хамаарахгүй — 00029-ийн
-- тайлбарт бичсэн шалтгаан.
--
--
-- ГАРЫН ҮСГИЙН ТҮЛХҮҮР.
--
-- `registry.oauth2_signing_keys` бол суулгацын хувийн түлхүүр: түүгээр
-- гарын үсэг зурсан id_token бүрийг найдвартай гэж үздэг. Тенантын роль
-- түүнийг унших ч, солих ч эрхтэй байсан.
--
-- Түлхүүрийг уншдаг хоёр л газар бий — `HandleJWKS` ба `mintIDToken` — хоёулаа
-- session-гүй, өөрөөр хэлбэл платформын зам дээр ажилладаг. Тиймээс тенантын
-- ролиос эрхийг нь бүхэлд нь авна: бодлого биш, эрх — хамгийн энгийн хэлбэр.

-- +goose Up

-- +goose StatementBegin
DO $person$
DECLARE
    target RECORD;
BEGIN
    FOR target IN
        SELECT * FROM (VALUES
            ('users',               'id'),
            ('user_sso_identities', 'user_id'),
            ('user_eid_identities', 'user_id')
        ) AS t(table_name, person_column)
    LOOP
        EXECUTE format('ALTER TABLE registry.%I ENABLE ROW LEVEL SECURITY', target.table_name);
        EXECUTE format('ALTER TABLE registry.%I FORCE ROW LEVEL SECURITY', target.table_name);
        EXECUTE format('DROP POLICY IF EXISTS person_isolation ON registry.%I', target.table_name);
        EXECUTE format(
            'CREATE POLICY person_isolation ON registry.%I TO gerege_nexus_tenant '
            'USING (%I = NULLIF(current_setting(''app.current_user'', true), '''')::uuid '
            '       OR EXISTS (SELECT 1 FROM workspace.memberships m WHERE m.user_id = registry.%I.%I)) '
            'WITH CHECK (%I = NULLIF(current_setting(''app.current_user'', true), '''')::uuid '
            '       OR EXISTS (SELECT 1 FROM workspace.memberships m WHERE m.user_id = registry.%I.%I))',
            target.table_name,
            target.person_column, target.table_name, target.person_column,
            target.person_column, target.table_name, target.person_column);
    END LOOP;
END
$person$;
-- +goose StatementEnd

-- Консол нь өөрийн эрхээрээ: 00049-ийн нэрлэсэн жагсаалтад `users` дээр
-- SELECT, INSERT бий (урилга). RLS асаагдсан тул бодлогогүй роль юу ч
-- хийхгүй болно — түүний эрхийг бодлого болгон давтана.
-- Консол нь бүх хүнийг хардаг (дэмжлэгийн дэлгэц), урина, ба түгжээг нь
-- тайлдаг. Юуг бичиж болохыг 00049-ийн БАГАНЫН эрх шийднэ: `locked_until`,
-- `failed_login_attempts` хоёроос өөр багана дээр UPDATE эрхгүй тул нууц
-- үгэнд хүрэх аргагүй. Бодлого нь тэр шийдвэрийг давтахгүй — зөвхөн RLS
-- асаалттай болсны улмаас хаагдсан үүдийг эрхийнх нь хэрээр нээнэ.
CREATE POLICY console_reads_people ON registry.users FOR SELECT TO gerege_nexus_operator USING (true);
CREATE POLICY console_invites_people ON registry.users FOR INSERT TO gerege_nexus_operator WITH CHECK (true);
CREATE POLICY console_unlocks_people ON registry.users FOR UPDATE TO gerege_nexus_operator USING (true) WITH CHECK (true);

-- Танилтын хоёр хүснэгт дээрх консолын уншилт.
--
-- 00099, 00100 нь консолд `user_eid_identities`, `user_sso_identities` дээр
-- SELECT эрх өгсөн: «баталгаажсан хүмүүсээс байгууллагын эхний админыг сонгох»
-- ба «энэ бүртгэл ямар аргаар нэвтэрдэг вэ» гэсэн хоёр дэлгэц түүгээр
-- ажилладаг (internal/operator/tenants/directory.go, internal/operator/people).
-- Дээр RLS-ийг FORCE болгосон тул бодлогогүй роль юу ч харахгүй болно —
-- эрх нь хэвээр атлаа хариу нь хоосон, ямар ч алдаагүй. Тиймээс тэр эрхийг
-- бодлого болгон давтана: SELECT, өөр юу ч биш.
CREATE POLICY console_reads_eid_identities ON registry.user_eid_identities
    FOR SELECT TO gerege_nexus_operator USING (true);
CREATE POLICY console_reads_sso_identities ON registry.user_sso_identities
    FOR SELECT TO gerege_nexus_operator USING (true);

-- Суулгацын хувийн түлхүүр. Уншдаг хоёр газар (JWKS, id_token) session-гүй
-- ажилладаг тул тенантын ролид энэ хүснэгт хэрэггүй.
REVOKE ALL ON registry.oauth2_signing_keys FROM gerege_nexus_tenant;

-- +goose Down

GRANT SELECT, INSERT, UPDATE, DELETE ON registry.oauth2_signing_keys TO gerege_nexus_tenant;

DROP POLICY IF EXISTS console_reads_sso_identities ON registry.user_sso_identities;
DROP POLICY IF EXISTS console_reads_eid_identities ON registry.user_eid_identities;
DROP POLICY IF EXISTS console_unlocks_people ON registry.users;
DROP POLICY IF EXISTS console_invites_people ON registry.users;
DROP POLICY IF EXISTS console_reads_people ON registry.users;

-- +goose StatementBegin
DO $person$
DECLARE
    target TEXT;
BEGIN
    FOREACH target IN ARRAY ARRAY['users', 'user_sso_identities', 'user_eid_identities']
    LOOP
        EXECUTE format('DROP POLICY IF EXISTS person_isolation ON registry.%I', target);
        EXECUTE format('ALTER TABLE registry.%I NO FORCE ROW LEVEL SECURITY', target);
        EXECUTE format('ALTER TABLE registry.%I DISABLE ROW LEVEL SECURITY', target);
    END LOOP;
END
$person$;
-- +goose StatementEnd
