-- «Хэн бидэн рүү орж болох вэ» гэдгийг байгууллага өөрөө хэлнэ.
--
-- 00089 хүнд байгууллага руу хүсэх зам нээсэн. Тэр зам нь бүх байгууллагад
-- нэг ижил: хэн ч slug-аар нь хүсэлт илгээж чадна, админ нь нэг бүрчлэн
-- хариулна. Хоёр төрлийн байгууллага үүнд таарахгүй байв:
--
--   * нээлттэй нэгдэлтэй байгууллага — жишээ нь салбар бүрийн ажилтан өөрөө
--     нэгдэх ёстой газар — админыг товч дарах машин болгоно;
--   * хаалттай байгууллага — татгалзахаас өөр аргагүй, гэтэл татгалзал бүр
--     нь хүнд «магадгүй дараа» гэсэн дохио үлдээнэ.
--
-- Тиймээс байгууллагын мөрөнд нэг багана: хүн яаж орох вэ.
--
--   on_request  хүсэлт илгээнэ, админ шийднэ (00089-ийн зан төлөв, анхдагч)
--   open        шууд гишүүн болно
--
-- ЯАГААД АНХДАГЧ НЬ `on_request` ВЭ. Энэ баганыг нэмэх нь одоо байгаа
-- байгууллагуудын бодлогыг өөрчлөх ёсгүй. Нээлттэй болгох нь хүн хийх
-- шийдвэр, миграцын дайвар үр дүн биш.
--
--
-- ШУУД НЭГДЭЛТ Ч ГЭСЭН МӨР ҮЛДЭЭНЭ.
--
-- `open` дээр гишүүнчлэлийг чимээгүй бичээд өнгөрч болох байсан. Тэгсэн бол
-- байгууллага «энэ хүн хэзээ, яаж орсон бэ» гэдгийг хэзээ ч хариулж чадахгүй
-- болно. Оронд нь 00089-ийн хүснэгтэд `ACCEPTED` мөр үүснэ — `decided_at`
-- нь одоо, `decided_by` нь **NULL**: шийдвэрийг хүн биш, бодлого гаргасан.
-- Хүснэгтийн `join_requests_decision_is_whole` шалгуур яг үүнийг зөвшөөрдөг
-- (шийдэгдсэн мөр цагтайгаа байна, шийдэгч нь заавал биш).
--
-- Ингэснээр админы дараалал, хүний өөрийнх нь проекц, audit гурвуулаа
-- байрандаа үлдэнэ — шинэ уншилтын зам нэмэгдэхгүй.
--
--
-- ШИНЭ ГИШҮҮН ЮУ БАРЬЖ ИРЭХ ВЭ.
--
-- Гишүүнчлэл бол хоосон биш. 00008-ийн `membership_default_role` trigger нь
-- `memberships`-д мөр орох болгонд тухайн байгууллагын `user` роль өгдөг, тэр
-- роль нь `%.read` бүх зөвшөөрөл ба `gov.apply`-г агуулна. Энэ нь зөвшөөрөгдсөн
-- хүсэлт дээр ч ижилхэн болдог — шинэ зам биш, ижил зам.
--
-- Тиймээс `open` гэдэг нь «хаалга онгорхой» биш, «хаалга онгорхой бөгөөд
-- орсон хүн уншиж чадна» гэсэн үг. Энэ бол байгууллагын мэдээллийн тухай
-- бодит шийдвэр тул тохиргооны дэлгэц түүнийг ил хэлнэ; энд бичиж байгаа нь
-- дараагийн уншигч миграцаас тэр баримтыг олохын тулд.
--
-- Анхны төсөлд «шинэ гишүүн ямар ч рольгүй ирнэ» гэж бичигдсэн байсныг тест
-- худал болохыг нь баталсан (`TestAnOpenOrganisationGrantsThePlatformsDefaultRole`).
-- Тэр таамаг кодод үлдсэн бол админ нээлттэй болгохдоо юу өгч байгаагаа
-- мэдэхгүй байх байв.
--
--
-- ФУНКЦИЙН БУЦААХ ТӨРӨЛ ӨӨРЧЛӨГДӨЖ БАЙНА.
--
-- Дуудагч «хүсэлт илгээгдэв» үү, «нэгдлээ» юу гэдгийг ялгах ёстой: хоёр өөр
-- өгүүлбэр, хоёр өөр дараагийн алхам. PostgreSQL нь `RETURNS TABLE`-ийн
-- төрөл өөрчлөгдөхөд `CREATE OR REPLACE`-ыг зөвшөөрдөггүй тул функцийг
-- буулгаад дахин үүсгэнэ. `EXECUTE` эрх нь функцтэйгээ хамт алга болдог тул
-- дор нь дахин олгоно — үүнийг мартвал нэгдэх товч бүхэлдээ 500 хариулна.

-- +goose Up

ALTER TABLE registry.tenants
    ADD COLUMN IF NOT EXISTS join_policy VARCHAR(16) NOT NULL DEFAULT 'on_request';

ALTER TABLE registry.tenants
    DROP CONSTRAINT IF EXISTS tenants_join_policy_known;
ALTER TABLE registry.tenants
    ADD CONSTRAINT tenants_join_policy_known
        CHECK (join_policy IN ('on_request', 'open'));

DROP FUNCTION IF EXISTS registry.request_to_join(UUID, TEXT, TEXT);

-- +goose StatementBegin
CREATE FUNCTION registry.request_to_join(
    p_user_id UUID,
    p_slug    TEXT,
    p_message TEXT
) RETURNS TABLE (request_id UUID, workspace_id UUID, workspace_name TEXT, joined BOOLEAN)
LANGUAGE plpgsql SECURITY DEFINER SET search_path = workspace, registry AS $fn$
DECLARE
    target registry.tenants%ROWTYPE;
    found  UUID;
    admitted BOOLEAN := FALSE;
BEGIN
    SELECT * INTO target FROM registry.tenants
     WHERE slug = p_slug AND kind = 'organisation';
    IF target.id IS NULL THEN
        RAISE EXCEPTION 'request_to_join: no organisation with slug %', p_slug
            USING ERRCODE = 'no_data_found';
    END IF;
    IF target.suspended_at IS NOT NULL OR target.deletion_scheduled_at IS NOT NULL THEN
        RAISE EXCEPTION 'request_to_join: % is not accepting members', p_slug
            USING ERRCODE = 'object_not_in_prerequisite_state';
    END IF;
    IF EXISTS (SELECT 1 FROM workspace.memberships
                WHERE tenant_id = target.id AND user_id = p_user_id) THEN
        RAISE EXCEPTION 'request_to_join: already a member of %', p_slug
            USING ERRCODE = 'unique_violation';
    END IF;

    IF target.join_policy = 'open' THEN
        -- Гишүүнчлэл эхэлж: мөр нь түүнийг тайлбарлаж байгаа болохоос
        -- эсрэгээрээ биш. Хоёулаа нэг гүйлгээнд тул нэг нь бүтэхгүй бол
        -- нөгөө нь ч үлдэхгүй.
        INSERT INTO workspace.memberships (tenant_id, user_id)
        VALUES (target.id, p_user_id);

        -- Хүлээгдэж байсан хүсэлт байвал түүнийг л шийднэ: хүн хүсэлт
        -- илгээчихээд, дараа нь байгууллага нээлттэй болсон тохиолдол.
        SELECT j.id INTO found FROM workspace.join_requests j
         WHERE j.tenant_id = target.id AND j.user_id = p_user_id AND j.status = 'PENDING';
        IF found IS NULL THEN
            INSERT INTO workspace.join_requests
                   (tenant_id, user_id, message, status, decided_at)
            VALUES (target.id, p_user_id, coalesce(left(p_message, 500), ''),
                    'ACCEPTED', NOW())
            RETURNING id INTO found;
        ELSE
            UPDATE workspace.join_requests
               SET status = 'ACCEPTED', decided_at = NOW()
             WHERE id = found;
        END IF;

        admitted := TRUE;
    ELSE
        SELECT j.id INTO found FROM workspace.join_requests j
         WHERE j.tenant_id = target.id AND j.user_id = p_user_id AND j.status = 'PENDING';
        IF found IS NULL THEN
            INSERT INTO workspace.join_requests (tenant_id, user_id, message)
            VALUES (target.id, p_user_id, coalesce(left(p_message, 500), ''))
            RETURNING id INTO found;
        END IF;
    END IF;

    RETURN QUERY SELECT found, target.id, target.name::text, admitted;
END
$fn$;
-- +goose StatementEnd

GRANT EXECUTE ON FUNCTION registry.request_to_join(UUID, TEXT, TEXT) TO gerege_nexus_tenant;

-- +goose Down

DROP FUNCTION IF EXISTS registry.request_to_join(UUID, TEXT, TEXT);

-- +goose StatementBegin
CREATE FUNCTION registry.request_to_join(
    p_user_id UUID,
    p_slug    TEXT,
    p_message TEXT
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
    IF target.suspended_at IS NOT NULL OR target.deletion_scheduled_at IS NOT NULL THEN
        RAISE EXCEPTION 'request_to_join: % is not accepting members', p_slug
            USING ERRCODE = 'object_not_in_prerequisite_state';
    END IF;
    IF EXISTS (SELECT 1 FROM workspace.memberships
                WHERE tenant_id = target.id AND user_id = p_user_id) THEN
        RAISE EXCEPTION 'request_to_join: already a member of %', p_slug
            USING ERRCODE = 'unique_violation';
    END IF;

    SELECT j.id INTO found FROM workspace.join_requests j
     WHERE j.tenant_id = target.id AND j.user_id = p_user_id AND j.status = 'PENDING';
    IF found IS NULL THEN
        INSERT INTO workspace.join_requests (tenant_id, user_id, message)
        VALUES (target.id, p_user_id, coalesce(left(p_message, 500), ''))
        RETURNING id INTO found;
    END IF;

    RETURN QUERY SELECT found, target.id, target.name::text;
END
$fn$;
-- +goose StatementEnd

GRANT EXECUTE ON FUNCTION registry.request_to_join(UUID, TEXT, TEXT) TO gerege_nexus_tenant;

ALTER TABLE registry.tenants DROP CONSTRAINT IF EXISTS tenants_join_policy_known;
ALTER TABLE registry.tenants DROP COLUMN IF EXISTS join_policy;
