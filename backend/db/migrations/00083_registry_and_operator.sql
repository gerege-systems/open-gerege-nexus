-- `platform` schema хоёр болж хуваагдана: `registry` ба `operator`.
--
-- 00079 нэг `public`-ийг хоёр болгосон: `tenant` ба `platform`. Тэр хуваалт
-- урсгалын дагуу байсан — мөр тенант бүрд тусад нь оршдог уу, эсвэл
-- суурилуулалтад ганцхан удаа оршдог уу. Энэ хуваалт өөр асуултынх:
-- **суурилуулалтад ганц удаа оршдог мөрүүдээс аль нь операторынх вэ.**
--
-- `platform` гэдэг нэр дөрвөн зүйл заадаг байв — операторын урсгал, энэ
-- schema, host багц, бүтээгдэхүүн өөрөө. Эхний хоёр нь бүр эсрэг дүрэм
-- үүрч байсан: `internal/platform` руу тенантын код импорт хийж болохгүй,
-- `platform` schema руу харин заавал ханддаг. Кодын хоёр нь аль хэдийн
-- засагдсан (`internal/operator`, `pkg/host`). Энэ нь гурав дахь нь.
--
--
-- ХУВААЛТЫГ ЯАЖ ТОГТООВ.
--
-- Таамгаар биш. Хоёр эх сурвалжийг тааруулав:
--
--   1. **Grant.** 27 хүснэгтийн дөрөвт `gerege_nexus_tenant` ямар ч эрхгүй:
--      `operator_accounts`, `operator_audit`, `operator_sessions`,
--      `platform_credentials`. Эдгээрийг тенантын урсгал өнөөдөр ч хүрч
--      чадахгүй — өгөгдлийн сан өөрөө зогсоож байгаа.
--
--   2. **Код.** Үлдсэн гурвыг — `pending_approvals`, `platform_backups`,
--      `platform_settings_history` — `internal/operator`-оос өөр газар нэг ч
--      query нэрлэдэггүй. Тэдний дээрх тенантын grant нь 00079-өөс өмнөх,
--      бүх хүснэгт `public`-д байхад бүгдэд нь нэг мөрөөр өгөгдсөн эрхийн
--      үлдэгдэл. `internal/kernel/settings`-ийн `Set` ба `History` хоёр
--      `platform_settings_history`-г бичиж уншдаг ч тэднийг дуудагч нь зөвхөн
--      операторын консол: тенантын урсгал тохиргоог зөвхөн `Get`-ээр уншина.
--
-- Тэр долоо нь `operator`. Үлдсэн хорь нь `registry` — суурилуулалтын
-- бүртгэл: хэн байна (`users`, `tenants`, identity), юу байна (`apps`,
-- `permissions`), ямар хязгаартай (`tenant_quotas`, `feature_flags`).
-- Хоёулаа уншдаг зүйл энд байна.
--
-- `platform_settings` нь `registry`-д, `platform_settings_history` нь
-- `operator`-т очсон нь санамсаргүй биш: одоогийн тохиргоог хоёр урсгал
-- уншина (хандалтын горим, брэнд, session-ий хугацаа), харин **хэн хэзээ
-- юуг өөрчилснийг** зөвхөн оператор хардаг. Тэр бол аудит.
--
--
-- ЭНЭ ХУВААЛТ ЮУГ НЭМЖ БАРИВ.
--
-- 00079 нэг зүйлийг хийж чадаагүй: `platform` дээрх USAGE-ийг тенантын
-- role-оос авч чадаагүй, учир нь хилийн таван хүснэгт тэр schema дотор
-- байсан. Тиймээс хил нь зөвхөн хүснэгт тус бүрийн grant дээр тогтож,
-- schema-ийн түвшний хамгаалалт байхгүй байв — `schema_split_test.go`
-- өөрөө үүнийг бичсэн: *«platform USAGE cannot be revoked from the tenant
-- role»*.
--
-- Одоо болно. Хилийн таван хүснэгт `registry`-д үлдсэн тул тенантын role
-- `operator` schema дээр USAGE **огт авахгүй**. `operator.operator_audit`-ыг
-- одоо тенантын урсгал нэрлэж ч чадахгүй: нэр нь ч харагдахгүй.
-- Хамгаалалт нэг давхраас хоёр болов.
--
--
-- QUERY БҮР SCHEMA-ГАА НЭРЛЭДЭГ ТУЛ ЭНЭ НЬ МЕХАНИК.
--
-- `qualification_test.go` нь цөмийн Go SQL бүрийг `tenant.<хүснэгт>` эсвэл
-- `platform.<хүснэгт>` гэж бичихийг шаарддаг. Тиймээс 392 ишлэлийг хоёр
-- schema-ийн аль нэг рүү шилжүүлэх нь хайж-солих ажил бөгөөд алдаа гарвал
-- compile-д биш, тестэд харагдана. `search_path` нь өөр репогийн модулийн
-- SQL-д зориулагдсан хэвээр.

-- +goose Up

CREATE SCHEMA IF NOT EXISTS registry;
CREATE SCHEMA IF NOT EXISTS operator;

-- Операторынх — тенантын урсгал хэзээ ч уншихгүй долоо.
-- `SET SCHEMA` нь index, constraint, эзэмшдэг sequence, RLS бодлогуудыг
-- бүгдийг дагуулна: 00079 үүнийг аль хэдийн баталсан.
-- +goose StatementBegin
DO $operator$
DECLARE target TEXT;
BEGIN
    FOREACH target IN ARRAY ARRAY[
        'operator_accounts', 'operator_audit', 'operator_sessions',
        'pending_approvals', 'platform_backups', 'platform_credentials',
        'platform_settings_history'
    ] LOOP
        IF EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = 'platform' AND tablename = target) THEN
            EXECUTE format('ALTER TABLE platform.%I SET SCHEMA operator', target);
        END IF;
    END LOOP;
END
$operator$;
-- +goose StatementEnd

-- Үлдсэн бүхэн `registry`. Нэрсийг нь энд бичихгүй байгаа нь санаатай:
-- 00079-ийн дараа `platform`-д хүснэгт нэмэгдсэн (00081), цаашид ч
-- нэмэгдэж болно. Жагсаалт бичвэл дараагийн ийм хүснэгт хоцорч, доорх
-- `DROP SCHEMA` дээр migration унана. Нээлттэй тал нь `registry` байх нь
-- зөв: шинэ хүснэгт анхдагчаар оператор биш, бүртгэл байх ёстой.
-- +goose StatementBegin
DO $registry$
DECLARE target RECORD;
BEGIN
    FOR target IN SELECT tablename AS table_name FROM pg_tables WHERE schemaname = 'platform' LOOP
        EXECUTE format('ALTER TABLE platform.%I SET SCHEMA registry', target.table_name);
    END LOOP;
END
$registry$;
-- +goose StatementEnd

-- RESTRICT нь анхдагч. Хоосон биш бол энд унах нь зөв: юу нь үлдсэнийг
-- чимээгүй авч үлдэхээс тэр дээр зогсох нь дээр.
DROP SCHEMA IF EXISTS platform RESTRICT;

-- Хилийн мөрүүд. Тенантын role `operator`-ыг нэрлэхгүй — энэ хуваалтын
-- бүх утга энэ мөр байхгүйд оршино.
GRANT USAGE ON SCHEMA registry TO gerege_nexus_tenant;
GRANT USAGE ON SCHEMA registry TO gerege_nexus_operator;
GRANT USAGE ON SCHEMA operator TO gerege_nexus_operator;

-- 00079-өөс өмнөх эрхийн үлдэгдлийг цэвэрлэв. USAGE байхгүй тул эдгээр
-- grant аль хэдийн хүрэхгүй болсон; устгаж байгаа нь санааг ил болгохын
-- тулд. Grant нь хүснэгтээ дагаж нүүсэн бөгөөд тэднийг ашигладаг query
-- байхгүй — байсан бол `operator` руу нүүлгэхийн өмнө олдох байв.
REVOKE ALL ON operator.pending_approvals, operator.platform_backups,
              operator.platform_settings_history
    FROM gerege_nexus_tenant;

-- `search_path`. 00079 ба 00080-ийн шалтгаан хэвээр: ALTER ROLE-ийн
-- тохиргоо `SET ROLE`-оор идэвхжихгүй тул database дээрх нь жинхэнэ
-- ажилладаг мөр. Дараалал нь чухал биш — 67 нэрийн хооронд давхардал
-- байхгүй — гэхдээ role бүрийнх нь өөрийн урсгалаас эхлэх нь уншихад
-- зөв. Тенантын role `operator`-ыг замдаа огт агуулахгүй: USAGE-гүй
-- schema-г замд бичих нь худал амлалт.
-- +goose StatementBegin
DO $search_path$
BEGIN
    EXECUTE format('ALTER DATABASE %I SET search_path = tenant, registry, operator',
                   current_database());
END
$search_path$;
-- +goose StatementEnd

ALTER ROLE gerege_nexus_tenant SET search_path = tenant, registry;
ALTER ROLE gerege_nexus_operator SET search_path = operator, registry, tenant;

-- SECURITY DEFINER функц өөрийн замтай бол database-ийн шинэ анхдагчийг
-- авахгүй. Гурвуулаа `tenant`-ийн хүснэгтүүд дээр ажилладаг ч замдаа
-- `platform` нэрлэсэн байсан тул хамт шинэчилнэ.
ALTER FUNCTION public.create_tenant_profile() SET search_path = tenant, registry;
ALTER FUNCTION public.resolve_device_enrollment(TEXT) SET search_path = tenant, registry;
ALTER FUNCTION public.authenticate_device(TEXT) SET search_path = tenant, registry;

-- +goose Down

ALTER FUNCTION public.create_tenant_profile() SET search_path = tenant, platform;
ALTER FUNCTION public.resolve_device_enrollment(TEXT) SET search_path = tenant, platform;
ALTER FUNCTION public.authenticate_device(TEXT) SET search_path = tenant, platform;

ALTER ROLE gerege_nexus_tenant SET search_path = tenant, platform;
ALTER ROLE gerege_nexus_operator SET search_path = platform, tenant;

-- +goose StatementBegin
DO $search_path$
BEGIN
    EXECUTE format('ALTER DATABASE %I SET search_path = tenant, platform',
                   current_database());
END
$search_path$;
-- +goose StatementEnd

CREATE SCHEMA IF NOT EXISTS platform;

-- +goose StatementBegin
DO $back$
DECLARE target RECORD;
BEGIN
    FOR target IN
        SELECT schemaname, tablename AS table_name FROM pg_tables WHERE schemaname IN ('registry', 'operator')
    LOOP
        EXECUTE format('ALTER TABLE %I.%I SET SCHEMA platform', target.schemaname, target.table_name);
    END LOOP;
END
$back$;
-- +goose StatementEnd

DROP SCHEMA IF EXISTS registry RESTRICT;
DROP SCHEMA IF EXISTS operator RESTRICT;

GRANT USAGE ON SCHEMA platform TO gerege_nexus_operator;
GRANT USAGE ON SCHEMA platform TO gerege_nexus_tenant;
GRANT SELECT, INSERT, UPDATE, DELETE ON platform.pending_approvals,
                                        platform.platform_backups,
                                        platform.platform_settings_history
    TO gerege_nexus_tenant;
