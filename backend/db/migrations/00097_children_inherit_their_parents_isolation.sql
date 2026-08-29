-- Эцгийнхээ тусгаарлалтыг өвлөх хүүхэд хүснэгтүүд, ба өөрийгөө засаж
-- чадахгүй аудит.
--
--
-- ЮУ ХӨНДӨГДӨӨГҮЙ БАЙВ.
--
-- 00029 нь `tenant_id` баганатай хүснэгт бүрийг олж бодлого тавьсан. Тэр нь
-- зөв дүрэм ч бүрэн жагсаалт биш: зургаан хүснэгт тенантын мөр агуулдаг
-- атлаа `tenant_id` баганагүй, учир нь эзэн нь эцэг мөрөндөө байдаг
-- (db/migrations/ownership_test.go энэ зургааг нэрлэсэн). Тэдгээрийн тав нь
-- өнөөдрийг хүртэл ямар ч бодлогогүй байсан:
--
--     esign_batch_items      → esign_batches(id)
--     installation_events    → app_installations(id)
--     membership_roles       → memberships(id)
--     role_permissions       → roles(id)
--     oauth2_access_tokens   → oauth2_clients(client_id)
--
-- (Зургаа дахь нь report_grants — өөрийн хоёр tenant баганатай тул 00071-д
-- аль хэдийн бодлоготой.)
--
-- Эдгээр нь апп талын `WHERE tenant_id` шүүлтээр хамгаалагдсан хэвээр —
-- dbguard-ийн хэлдгээр тэр нь үндсэн хамгаалалт. Дутуу байсан нь доод давхарга:
-- заалтаа мартсан handler эдгээр таван хүснэгтээс өөр байгууллагын мөр буцааж
-- чадна.
--
--
-- ЯАГААД ДҮРМИЙГ ДАХИН БИЧИХГҮЙ ВЭ.
--
-- Бодлого бүр эцэг мөр нь харагдаж байна уу гэдгийг л асууна:
--
--     EXISTS (SELECT 1 FROM эцэг WHERE эцэг.түлхүүр = хүүхэд.түлхүүр)
--
-- Эцэг нь өөрөө RLS-ийн дор тул энэ EXISTS нь дуудагчид харагдах мөрийг л
-- олно. Ингэснээр хүүхэд нь эцгийнхээ дүрмийг — өргөн ч бай, нарийн ч бай —
-- яг тэр чигээр нь өвлөнө. `current_setting('app.current_tenant')`-ыг энд
-- хуулбарлавал хоёр газарт бичигдсэн нэг дүрэм болох бөгөөд 00037 өргөн
-- хэлбэр рүү шилжихэд яг ийм хуулбарууд хоцорч үлдсэн (policy_shape_test.go).
--
-- Бодлогын нэр нь `tenant_isolation` биш `parent_isolation`: policy_shape_test
-- нь `tenant_isolation` бүрээс өргөн/нарийн хэлбэрийн аль нэгийг шаарддаг ба
-- өвлөсөн бодлого хоёрын аль нь ч биш. Өөр нэр нь өөр дүрэм гэдгийг хэлж
-- байгаа юм.
--
--
-- АУДИТ ӨӨРИЙГӨӨ ЗАСАЖ ЧАДАХГҮЙ.
--
-- `operator_audit` нь 00049-өөс хойш trigger-ээр append-only. Тенантын
-- `audit_events` тийм биш байсан бөгөөд `gerege_nexus_tenant` роль түүн дээр
-- UPDATE, DELETE эрхтэй байв: аудитын мөрийг тэр мөрийг бичүүлсэн үйлдлийг
-- хийж чадах роль өөрөө устгаж чадна гэсэн үг.
--
-- UPDATE-ыг trigger хаана. DELETE-ыг хаахгүй бөгөөд энэ нь санаатай:
-- `audit_events.tenant_id` нь `tenants(id)`-д ON DELETE CASCADE-ээр холбоотой
-- тул устгалын хугацаа дуусахад мөрүүд нь эцэг мөртэйгээ хамт явах ёстой.
-- Оронд нь тенантын ролиос UPDATE, DELETE эрхийг нь авна — эрхгүй роль
-- trigger-гүйгээр ч засаж чадахгүй, харин RI-ийн cascade нь эрхийн шалгалтаас
-- гадуур ажилладаг тул устгал хэвээр ажиллана.

-- +goose Up

-- +goose StatementBegin
DO $children$
DECLARE
    child RECORD;
BEGIN
    FOR child IN
        SELECT * FROM (VALUES
            ('esign_batch_items',    'esign_batches',   'id',        'batch_id'),
            ('installation_events',  'app_installations','id',       'installation_id'),
            ('membership_roles',     'memberships',     'id',        'membership_id'),
            ('role_permissions',     'roles',           'id',        'role_id'),
            ('oauth2_access_tokens', 'oauth2_clients',  'client_id', 'client_id')
        ) AS t(child_table, parent_table, parent_key, child_key)
    LOOP
        EXECUTE format('ALTER TABLE workspace.%I ENABLE ROW LEVEL SECURITY', child.child_table);
        EXECUTE format('ALTER TABLE workspace.%I FORCE ROW LEVEL SECURITY', child.child_table);
        EXECUTE format('DROP POLICY IF EXISTS parent_isolation ON workspace.%I', child.child_table);
        -- Хоёр роль нэг илэрхийлэл дээр. Роль бүр эцгийг өөрийн нүдээр
        -- хардаг тул хүүхэд нь тухайн ролийн хувьд эцэгтэйгээ яг ижил
        -- хэмжээнд нээгдэнэ: тенант нь өөрийн байгууллагынхыг, консол нь
        -- эцэг дээрээ бодлоготой байвал түүнийг, байхгүй бол юуг ч үгүй.
        -- Консол `role_permissions`, `membership_roles` хоёрыг байгууллага
        -- үүсгэхдээ бичдэг (00049-ийн нэрлэсэн эрхүүд) тул энэ нь онолын
        -- сайжруулалт биш: түүнийг орхивол байгууллага үүсэхээ болино.
        EXECUTE format(
            'CREATE POLICY parent_isolation ON workspace.%I TO gerege_nexus_tenant, gerege_nexus_operator '
            'USING (EXISTS (SELECT 1 FROM workspace.%I p WHERE p.%I = workspace.%I.%I)) '
            'WITH CHECK (EXISTS (SELECT 1 FROM workspace.%I p WHERE p.%I = workspace.%I.%I))',
            child.child_table,
            child.parent_table, child.parent_key, child.child_table, child.child_key,
            child.parent_table, child.parent_key, child.child_table, child.child_key);
    END LOOP;
END
$children$;
-- +goose StatementEnd

-- Бодлого бүрийн EXISTS нь хүүхдийн түлхүүрээр эцэг рүү очно. Дөрөв нь аль
-- хэдийн индекстэй (PK эсвэл unique constraint-ийн эхний багана); энэ нэг нь
-- байгаагүй, одоо бодлогын улмаас мөр бүр дээр уншигдана.
CREATE INDEX IF NOT EXISTS idx_installation_events_installation
    ON workspace.installation_events (installation_id);

REVOKE UPDATE, DELETE ON workspace.audit_events FROM gerege_nexus_tenant;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION workspace.audit_events_is_append_only() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_events is append-only: % is not allowed', TG_OP
        USING ERRCODE = 'insufficient_privilege';
END;
$$ LANGUAGE plpgsql SET search_path = pg_catalog, public;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS audit_events_append_only ON workspace.audit_events;
CREATE TRIGGER audit_events_append_only
    BEFORE UPDATE ON workspace.audit_events
    FOR EACH ROW EXECUTE FUNCTION workspace.audit_events_is_append_only();

-- +goose Down

DROP TRIGGER IF EXISTS audit_events_append_only ON workspace.audit_events;
DROP FUNCTION IF EXISTS workspace.audit_events_is_append_only();
GRANT UPDATE, DELETE ON workspace.audit_events TO gerege_nexus_tenant;

DROP INDEX IF EXISTS workspace.idx_installation_events_installation;

-- +goose StatementBegin
DO $children$
DECLARE
    child TEXT;
BEGIN
    FOREACH child IN ARRAY ARRAY['esign_batch_items', 'installation_events', 'membership_roles',
                                 'role_permissions', 'oauth2_access_tokens']
    LOOP
        EXECUTE format('DROP POLICY IF EXISTS parent_isolation ON workspace.%I', child);
        EXECUTE format('ALTER TABLE workspace.%I NO FORCE ROW LEVEL SECURITY', child);
        EXECUTE format('ALTER TABLE workspace.%I DISABLE ROW LEVEL SECURITY', child);
    END LOOP;
END
$children$;
-- +goose StatementEnd
