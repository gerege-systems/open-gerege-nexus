-- `tenant` schema нь `workspace` болно.
--
-- `tenant` бол байршлын үг: олон эзэмшигчтэй нэг суурилуулалт дээрх
-- түрээслэгчийг заана. Байгууллага харин хаана ажиллаж байгаагаас үл
-- хамааран байгууллага хэвээр. ADR 0001-ийн дүрмээр энэ нь адаптерын үг
-- домэйний байрыг эзэлсэн тохиолдол.
--
-- Код өөрөө үүнийг аль хэдийн мэдэж байсан: `ErrNoOrganisation`,
-- `urtuu_reads_across_organisations`, `the_org_chart_leaves_the_membership`,
-- `frontend/app/organisation`. Тайлбар, миграцын нэр, ADR бүгд «organisation»
-- руу нүүсэн байхад зөвхөн identifier нь `tenant` хэвээр үлдсэн байв.
--
-- `organisation` биш `workspace` сонгосон шалтгаан нь ADR 0006-д бий: муж
-- хоёр төрөлтэй болно — байгууллага ба нэг хүний гэр. «Personal org» бол
-- өөрийгөө няцаасан нэр. `workspace` хоёуланг зөв багтаана.
--
--
-- НЭГ МӨР ХАНГАЛТТАЙ.
--
-- `ALTER SCHEMA … RENAME` нь namespace объектыг өөрийг нь дахин нэрлэдэг тул
-- дотор нь байгаа бүхэн — хүснэгт, index, sequence, RLS бодлого, хүснэгтийн
-- grant, schema дээрх USAGE, бүр `ALTER DEFAULT PRIVILEGES`-ийн бичлэг хүртэл
-- — OID-ээрээ хамт явна. Тусад нь шинэчлэх шаардлагагүй. Үүнийг хоосон
-- өгөгдлийн санд туршиж баталсан: нэрлэсний дараа үүсгэсэн хүснэгт ч мөн
-- анхдагч эрхээ авсаар байв.
--
-- Хөндөгдөөгүй зүйлс, санаатайгаар:
--
--   * **Хүснэгт ба баганын нэр.** `tenant_profiles`, `tenant_quotas`,
--     `tenant_id` хэвээр. `tenant_id` нь `pkg/nexus`-ийн `tenantID`-тай
--     хосолсон бөгөөд тэр нь semver-т хөлдсөн гадаргуу — дараагийн major.
--   * **`gerege_nexus_tenant` role ба `tenant_isolation` бодлогын нэр.**
--     Эдгээр нь schema биш; тэдгээрийг сольсноор энэ миграц эрх, бодлогын
--     ажил болж хувирна. Тусад нь.
--   * **`app.current_tenant` GUC.** RLS бодлого бүрийн доторх нэр.
--
-- Тиймээс энэ миграцын дараа `workspace.tenant_profiles` гэсэн нэр гарна.
-- Тэр нь дундын байдал бөгөөд ADR 0006-ийн үе шатууд үүнийг хүлээн зөвшөөрсөн:
-- бүгдийг нэг өдөр солих нь нэг ч зүйлийг найдвартай солихгүй байх зам.

-- +goose Up

ALTER SCHEMA tenant RENAME TO workspace;

-- 00079 ба 00080-ийн шалтгаан хэвээр: `SET ROLE` нь зорилтот role-ын
-- тохиргоог хэрэглэхгүй тул database дээрх мөр нь жинхэнэ ажилладаг нь.
-- Модулийн хүснэгт эхний элементэд үүсдэг — тэр нь `workspace` байх ёстой.
-- +goose StatementBegin
DO $search_path$
BEGIN
    EXECUTE format('ALTER DATABASE %I SET search_path = workspace, registry, operator',
                   current_database());
END
$search_path$;
-- +goose StatementEnd

ALTER ROLE gerege_nexus_tenant SET search_path = workspace, registry;
ALTER ROLE gerege_nexus_operator SET search_path = operator, registry, workspace;

ALTER FUNCTION public.create_tenant_profile() SET search_path = workspace, registry;
ALTER FUNCTION public.resolve_device_enrollment(TEXT) SET search_path = workspace, registry;
ALTER FUNCTION public.authenticate_device(TEXT) SET search_path = workspace, registry;

-- +goose Down

ALTER FUNCTION public.create_tenant_profile() SET search_path = tenant, registry;
ALTER FUNCTION public.resolve_device_enrollment(TEXT) SET search_path = tenant, registry;
ALTER FUNCTION public.authenticate_device(TEXT) SET search_path = tenant, registry;

ALTER ROLE gerege_nexus_tenant SET search_path = tenant, registry;
ALTER ROLE gerege_nexus_operator SET search_path = operator, registry, tenant;

-- +goose StatementBegin
DO $search_path$
BEGIN
    EXECUTE format('ALTER DATABASE %I SET search_path = tenant, registry, operator',
                   current_database());
END
$search_path$;
-- +goose StatementEnd

ALTER SCHEMA workspace RENAME TO tenant;
