-- «Намайг байгууллагадаа оруулна уу.»
--
-- Одоо хүртэл байгууллагад орох ганц зам нь **урилга** байсан: оператор
-- эсвэл админ хүнийг сонгож, `credential_grants`-аар холбоос илгээнэ. Хүн
-- өөрөө хүсэх зам байхгүй.
--
-- Энэ нь 00085-аас хойш ил тод дутуу болов. Гишүүнчлэлгүй хүн одоо нэвтэрч,
-- өөрийн гэртээ буудаг — гэтэл тэндээс хаашаа ч хөдлөх аргагүй. Хүлээх л
-- үлддэг, хэн нэгэн түүнийг санаж урих хүртэл.
--
--
-- ЯАГААД ЭНЭ НЬ ЦӨМИЙНХ ВЭ.
--
-- Гишүүнчлэл бол цөмийн ойлголт: `workspace.memberships`, `membership_roles`,
-- ба тэдгээрийг үүсгэдэг trigger бүгд энд байна. Хүсэлт нь гишүүнчлэл рүү
-- хүргэдэг зам тул мөн энд байх ёстой — апп биш.
--
-- Бас 00086-ийн `person_items` рельсийг **цөм өөрөө** ашигладаг болгож байна.
-- `pkg/nexus/capability.go`-ийн MeetingBooker-ийн түүх үүнийг аль хэдийн
-- нэрлэсэн: *«Half a capability is none.»* Гэрээ зарлаад, хүснэгт барьж,
-- функц бичээд, дэлгэц гаргачихаад бичигчийг нь өөр репод үлдээх нь яг тэр
-- алдааны хэлбэр байлаа.
--
--
-- МӨР НЬ БАЙГУУЛЛАГЫН МУЖИД СУУНА.
--
-- Ажил тэнд хийгдэнэ: админ хардаг, админ шийднэ, audit тэднийх. Тиймээс
-- `tenant_id` нь **хүсэгчийн гэр биш, хүсэж буй байгууллага**.
--
-- Хүсэгч түүнийг тэндээс уншиж чадахгүй — гишүүн биш учраас `tenant_isolation`
-- зогсооно, зөв зогсооно. Тэр яг л 00086-гийн проекц шийддэг асуудал:
-- төлөв өөрчлөгдөх бүрд `registry.publish_person_item` хүсэгчийн гэрт хуулбар
-- бичнэ. Хоёр дахь уншилтын зам нэмэгдэхгүй.
--
--
-- ҮҮСГЭЛТ НЬ ЯАГААД ФУНКЦ ВЭ.
--
-- Хүсэгч өөрийн гэрт уягдсан session-тэй байхад **өөр мужид** мөр үүсгэнэ.
-- `tenant_isolation`-ий `WITH CHECK` үүнийг зогсооно. 00034 ба 00086-тай
-- ижил тохиолдол, ижил хариулт: нарийн `SECURITY DEFINER` функц.
--
-- Энэ бол репо дэх тав дахь ийм функц бөгөөд тоо нь өөрөө анхааруулга.
-- Хамгаалалт нь дахин функцийн нарийн байдалд:
--
--   1. Зөвхөн `kind='organisation'`, түдгэлзээгүй, устгах товлоогүй муж руу.
--      Гэр рүү хүсэлт явуулах боломжгүй — гэр гишүүн авдаггүй.
--   2. Аль хэдийн гишүүн бол алдаа. Хүсэлт нь гишүүнчлэлийг орлохгүй.
--   3. Нэг хосд нэг л нээлттэй хүсэлт — хэсэгчилсэн unique index барина.
--      Дахин дуудвал байгаа мөрөө буцаана, шинийг үүсгэхгүй.
--   4. `EXECUTE` зөвхөн `gerege_nexus_tenant`-д.
--
-- Шийдэх тал нь функц **биш**: админ өөрийн мужид, ердийн RLS-ийн дор,
-- ердийн эрхээрээ ажиллана. Тэнд тойрох юм байхгүй.

-- +goose Up

-- Эхлээд 00086-ийн функцийн түлхүүрийг засна.
--
-- Тэр нь `p_ge_id BIGINT` авдаг байсан: нийлүүлэгчийн модуль хүсэгчийг
-- Гэрэгэ дугаараар нь мэддэг гэсэн үндэслэлээр. Цөм өөрөө анхны бичигч
-- болохоор тэр үндэслэл нурав — `registry.users.ge_id` нь **NULL байж
-- болно**, зөвхөн eID-ээр нэвтэрсэн хүнд утга агуулна. Нууц үгээр
-- бүртгүүлсэн хүн байгууллагад хүсэлт гаргахад функц түүний гэрийг олохгүй.
--
-- Гэрийг олох жинхэнэ түлхүүр нь `owner_user_id` бөгөөд `ge_id` нь түүн рүү
-- хүрэх нэг алхам байсан. Тиймээс функц одоо хэрэглэгчийн id авна, харин
-- `pkg/nexus.PersonFeed` нь модулийн хувьд байгалийн үг болох Гэрэгэ дугаарыг
-- хэвээр авч, Go талдаа нэг SELECT-ээр хөрвүүлнэ. Хоёр аудитор, хоёр үг.
--
-- Аюулгүй байдал сулраагүй: аль ч хэлбэрт функц дуудагчийн нэрлэсэн хүнд
-- итгэдэг. Ялгаа нь ганц — шинэ түлхүүр бүх дансанд хүрнэ.
DROP FUNCTION IF EXISTS registry.publish_person_item(BIGINT, UUID, TEXT, TEXT, TEXT, TEXT, TEXT);

CREATE TABLE IF NOT EXISTS workspace.join_requests (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Хүсэж буй байгууллага. Мөрийг эзэмшигч нь энэ.
    tenant_id  UUID NOT NULL REFERENCES registry.tenants(id) ON DELETE CASCADE,
    -- Хүсэгч. `registry.users` руу заана: хүн бол суурилуулалтын түвшний
    -- баримт, тэр нь энэ хүсэлтийн бүх учир.
    user_id    UUID NOT NULL REFERENCES registry.users(id) ON DELETE CASCADE,
    -- Хүн юу гэж бичсэн. Хоосон байж болно; 500 тэмдэгт нь 00086-гийн
    -- `answer`-тэй ижил шалтгаантай хязгаар — энэ бол танилцуулга,
    -- захидал биш.
    message    TEXT NOT NULL DEFAULT '',
    status     TEXT NOT NULL DEFAULT 'PENDING',
    -- Хэн шийдсэн. Гишүүнчлэл биш хэрэглэгч: шийдсэн хүн байгууллагаас
    -- гарсан ч шийдвэр нь тэднийх хэвээр.
    decided_by UUID REFERENCES registry.users(id) ON DELETE SET NULL,
    decided_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT join_requests_status_known CHECK (status IN ('PENDING', 'ACCEPTED', 'DECLINED')),
    CONSTRAINT join_requests_message_is_short CHECK (length(message) <= 500),
    -- Шийдэгдсэн мөр шийдэгчтэйгээ, шийдэгдээгүй нь шийдэгчгүйгээ байна.
    CONSTRAINT join_requests_decision_is_whole
        CHECK ((status = 'PENDING') = (decided_at IS NULL))
);

-- Нэг хүн нэг байгууллагад нэг л удаа дараалалд зогсоно. Шийдэгдсэн мөрүүд
-- нь түүхэн бичлэг тул хязгаарт орохгүй — хүн татгалзсаны дараа дахин хүсэж
-- болно.
CREATE UNIQUE INDEX IF NOT EXISTS join_requests_one_open_per_pair
    ON workspace.join_requests (tenant_id, user_id) WHERE status = 'PENDING';
CREATE INDEX IF NOT EXISTS idx_join_requests_queue
    ON workspace.join_requests (tenant_id, created_at DESC) WHERE status = 'PENDING';

ALTER TABLE workspace.join_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE workspace.join_requests FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON workspace.join_requests;
-- 00037-ийн өргөн хэлбэр: олон байгууллагад ажилладаг админ хамт харах
-- горимдоо байхад хоёуланг нь хардаг байх нь зөв — эдгээр нь тэдний
-- байгууллагуудын өөрсдийнх нь дараалал.
CREATE POLICY tenant_isolation ON workspace.join_requests TO gerege_nexus_tenant
    USING (tenant_id = ANY (COALESCE(
        NULLIF(current_setting('app.allowed_tenants', true), '')::uuid[],
        ARRAY[NULLIF(current_setting('app.current_tenant', true), '')::uuid])))
    WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid);

-- Админ шийднэ — тиймээс `UPDATE` нээлттэй. `INSERT` харин доорх функцийнх:
-- хүсэгч өөр мужид бичих ёстой бөгөөд тэр нь RLS-ийн ард гарах ганц зам.
-- 00079-ийн `ALTER DEFAULT PRIVILEGES` энэ хүснэгтэд дөрвүүлээ өгсөн байхыг
-- 00086-ийн адил хасна.
GRANT SELECT, UPDATE ON workspace.join_requests TO gerege_nexus_tenant;
REVOKE INSERT, DELETE, TRUNCATE ON workspace.join_requests FROM gerege_nexus_tenant;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION registry.request_to_join(
    p_user_id UUID,
    p_slug    TEXT,
    p_message TEXT
-- Гаралтын баганын нэрс `workspace_`-ээр эхэлж байгаа нь санамсаргүй биш:
-- `RETURNS TABLE`-ийн нэр функцийн дотор хувьсагч болж ордог тул `tenant_id`
-- гэж нэрлэвэл `join_requests.tenant_id`-тай мөргөлдөж, PostgreSQL «column
-- reference is ambiguous» гэж татгалзана.
) RETURNS TABLE (request_id UUID, workspace_id UUID, workspace_name TEXT)
LANGUAGE plpgsql SECURITY DEFINER SET search_path = workspace, registry AS $fn$
DECLARE
    target registry.tenants%ROWTYPE;
    found  UUID;
BEGIN
    SELECT * INTO target FROM registry.tenants
     WHERE slug = p_slug AND kind = 'organisation';
    IF target.id IS NULL THEN
        RAISE EXCEPTION 'request_to_join: no organisation with slug %', p_slug
            USING ERRCODE = 'no_data_found';
    END IF;
    -- Түдгэлзсэн эсвэл устгах товлосон байгууллага гишүүн авахгүй. Хүнд
    -- «хүлээгдэж байна» гэж хэлээд хэзээ ч хариулагдахгүй мөр үлдээх нь
    -- татгалзахаас дор.
    IF target.suspended_at IS NOT NULL OR target.deletion_scheduled_at IS NOT NULL THEN
        RAISE EXCEPTION 'request_to_join: % is not accepting members', p_slug
            USING ERRCODE = 'object_not_in_prerequisite_state';
    END IF;
    IF EXISTS (SELECT 1 FROM workspace.memberships
                WHERE tenant_id = target.id AND user_id = p_user_id) THEN
        RAISE EXCEPTION 'request_to_join: already a member of %', p_slug
            USING ERRCODE = 'unique_violation';
    END IF;

    -- Дүрэм 3. Байгаа нээлттэй хүсэлтээ буцаана — давхар дарсан товч хоёр
    -- дахь мөр үүсгэх ёсгүй.
    SELECT j.id INTO found FROM workspace.join_requests j
     WHERE j.tenant_id = target.id AND j.user_id = p_user_id AND j.status = 'PENDING';
    IF found IS NULL THEN
        INSERT INTO workspace.join_requests (tenant_id, user_id, message)
        VALUES (target.id, p_user_id, coalesce(left(p_message, 500), ''))
        RETURNING id INTO found;
    END IF;

    -- `name` нь `VARCHAR` тул `TEXT` рүү шууд хөрвүүлнэ: `RETURNS TABLE`-ийн
    -- төрөл яг таарахгүй бол PostgreSQL «structure of query does not match»
    -- гэж дуудлагын үед татгалзана, тодорхойлолтын үед биш.
    RETURN QUERY SELECT found, target.id, target.name::text;
END
$fn$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION registry.request_to_join(UUID, TEXT, TEXT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION registry.request_to_join(UUID, TEXT, TEXT) TO gerege_nexus_tenant;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION registry.publish_person_item(
    p_user_id            UUID,
    p_provider_tenant_id UUID,
    p_source_app         TEXT,
    p_source_ref         TEXT,
    p_code               TEXT,
    p_status             TEXT,
    p_answer             TEXT
) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path = workspace, registry AS $fn$
DECLARE
    home UUID;
BEGIN
    IF p_user_id IS NULL THEN
        RAISE EXCEPTION 'publish_person_item: a person is required'
            USING ERRCODE = 'null_value_not_allowed';
    END IF;
    IF coalesce(p_source_app, '') = '' OR coalesce(p_source_ref, '') = '' THEN
        RAISE EXCEPTION 'publish_person_item: source_app and source_ref are required'
            USING ERRCODE = 'null_value_not_allowed';
    END IF;

    -- Дүрэм 1, өмнөхтэй ижил: зөвхөн гэр, зөвхөн энэ хүний. Одоо нэг холбоос
    -- богино — эзнийг шууд нэрлэж байна.
    SELECT t.id INTO home FROM registry.tenants t
     WHERE t.kind = 'personal' AND t.owner_user_id = p_user_id;

    IF home IS NULL THEN
        RAISE EXCEPTION 'publish_person_item: no personal workspace for %', p_user_id
            USING ERRCODE = 'no_data_found';
    END IF;

    INSERT INTO workspace.person_items
        (tenant_id, provider_tenant_id, source_app, source_ref, code, status, answer)
    VALUES
        (home, p_provider_tenant_id, p_source_app, p_source_ref, p_code, p_status, coalesce(p_answer, ''))
    ON CONFLICT (tenant_id, source_app, source_ref) DO UPDATE
       SET code       = EXCLUDED.code,
           status     = EXCLUDED.status,
           answer     = EXCLUDED.answer,
           provider_tenant_id = EXCLUDED.provider_tenant_id,
           updated_at = NOW();
END
$fn$;
-- +goose StatementEnd

REVOKE ALL ON FUNCTION registry.publish_person_item(UUID, UUID, TEXT, TEXT, TEXT, TEXT, TEXT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION registry.publish_person_item(UUID, UUID, TEXT, TEXT, TEXT, TEXT, TEXT) TO gerege_nexus_tenant;

-- +goose Down

DROP FUNCTION IF EXISTS registry.publish_person_item(UUID, UUID, TEXT, TEXT, TEXT, TEXT, TEXT);
DROP FUNCTION IF EXISTS registry.request_to_join(UUID, TEXT, TEXT);
DROP TABLE IF EXISTS workspace.join_requests;
