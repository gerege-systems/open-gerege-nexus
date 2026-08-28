-- Гэрт байгууллагын эрхийн бүтэц хэрэггүй.
--
-- `seed_tenant_access_roles()` нь tenant үүсэх бүрд гурван role (`admin`,
-- `manager`, `user`) болон тэдгээрийн зөвшөөрлийг бичдэг. Энэ нь байгууллагын
-- хувьд яг зөв: админ нь ажилтан бүрдээ өөр түвшин олгохын тулд сонголттой
-- байх ёстой.
--
-- Гэрт ажилтан байхгүй. 00085-аас хойш хүн бүр өөрийн workspace-тай болох
-- боломжтой бөгөөд тэр workspace-д хэзээ ч эзнээсээ өөр гишүүн орохгүй. Гурван
-- role-ийн хоёр нь хэнд ч олгогдохгүйгээр л сууна.
--
-- Хэмжсэн: 1,000,000 гэртэй өгөгдлийн санд `role_permissions` нь **17 сая
-- мөр, 2 GB** болж, сангийн хамгийн том хүснэгт нь нэг гишүүнтэй, тэр нь
-- өөрөө эзэн нь байгаа workspace-үүдийн зөвшөөрлийн жагсаалт болов. Бүхэлдээ
-- 3.9 GB-ийн 70 орчим хувь нь энэ хоёр хүснэгт.
--
-- **Аль role үлдэх вэ гэдэг нь сонголт биш.** `assign_default_membership_role`
-- нь гишүүнчлэл бүрд `user`-ийг олгодог, өөр аль ч role-ийг олгодоггүй.
-- Тиймээс гэрийн эзэн өнөөдөр яг `user`-ийн зөвшөөрлийг л эдэлж байгаа бөгөөд
-- үлдэхээр байгаа нь тэр. `admin`-ийг үлдээвэл гишүүнчлэлийн trigger юу ч
-- олохгүй, эзэн нь өөрийнхөө гэрт ямар ч зөвшөөрөлгүй үлдэнэ.
--
-- Өөрөөр хэлбэл энэ файл юуг ч зөвшөөрөхгүй, юуг ч хориглохгүй: хэн ч аваагүй
-- байсан хоёр role бичихээ болино.

-- +goose Up

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION public.seed_tenant_access_roles()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
BEGIN
    IF NEW.kind = 'personal' THEN
        -- Гишүүнчлэлийн trigger нь яг үүнийг хайдаг. Энэ мөрийг нэрлэхдээ
        -- өөрчилвөл гэрийн эзэн эрхгүй үлдэнэ — хоёр trigger нэг мөрөөр
        -- холбогдож байгааг санах хэрэгтэй.
        INSERT INTO roles (tenant_id, code, name, description, is_system)
        VALUES (NEW.id, 'user', 'User', 'Standard read access and self-service actions', TRUE)
        ON CONFLICT (tenant_id, code) DO NOTHING;

        INSERT INTO role_permissions (role_id, permission_id)
        SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
        WHERE r.tenant_id = NEW.id AND r.code = 'user' AND p.code LIKE '%.read'
        ON CONFLICT DO NOTHING;

        RETURN NEW;
    END IF;

    INSERT INTO roles (tenant_id, code, name, description, is_system)
    VALUES
        (NEW.id, 'admin', 'Administrator', 'Full access to this organisation', TRUE),
        (NEW.id, 'manager', 'Manager', 'Operational access to installed apps', TRUE),
        (NEW.id, 'user', 'User', 'Standard read access and self-service actions', TRUE)
    ON CONFLICT (tenant_id, code) DO NOTHING;

    INSERT INTO role_permissions (role_id, permission_id)
    SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
    WHERE r.tenant_id = NEW.id AND (
        r.code = 'admin'
        OR (r.code = 'manager' AND (p.code LIKE '%.read' OR p.code LIKE '%.manage'))
        OR (r.code = 'user' AND p.code LIKE '%.read')
    ) ON CONFLICT DO NOTHING;
    RETURN NEW;
END;
$function$;
-- +goose StatementEnd

-- Аль хэдийн бичигдсэн гэрүүдээс хэнд ч олгогдоогүй role-уудыг авна.
--
-- `NOT EXISTS (membership_roles)` нь болгоомжлол: гэрт зөвхөн `user` олгогддог
-- тул `admin`, `manager` хоосон байх ёстой. Хэрэв аль нэг нь ямар нэг замаар
-- хэн нэгэнд олгогдсон бол энэ нь тэр эрхийг чимээгүй тасалж болохгүй —
-- тэгвэл үлдээе. `role_permissions` нь role-оо дагаж cascade-аар арилна.
DELETE FROM workspace.roles r
 USING registry.tenants t
 WHERE t.id = r.tenant_id
   AND t.kind = 'personal'
   AND r.code IN ('admin', 'manager')
   AND NOT EXISTS (SELECT 1 FROM workspace.membership_roles mr WHERE mr.role_id = r.id);

-- +goose Down

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION public.seed_tenant_access_roles()
RETURNS trigger
LANGUAGE plpgsql
AS $function$
BEGIN
    INSERT INTO roles (tenant_id, code, name, description, is_system)
    VALUES
        (NEW.id, 'admin', 'Administrator', 'Full access to this organisation', TRUE),
        (NEW.id, 'manager', 'Manager', 'Operational access to installed apps', TRUE),
        (NEW.id, 'user', 'User', 'Standard read access and self-service actions', TRUE)
    ON CONFLICT (tenant_id, code) DO NOTHING;

    INSERT INTO role_permissions (role_id, permission_id)
    SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
    WHERE r.tenant_id = NEW.id AND (
        r.code = 'admin'
        OR (r.code = 'manager' AND (p.code LIKE '%.read' OR p.code LIKE '%.manage'))
        OR (r.code = 'user' AND p.code LIKE '%.read')
    ) ON CONFLICT DO NOTHING;
    RETURN NEW;
END;
$function$;
-- +goose StatementEnd

-- Хасагдсан role-уудыг эргүүлж бичнэ, ингэснээр буцаалт нь бүрэн болно.
INSERT INTO workspace.roles (tenant_id, code, name, description, is_system)
SELECT t.id, v.code, v.name, v.description, TRUE
  FROM registry.tenants t
 CROSS JOIN (VALUES
        ('admin', 'Administrator', 'Full access to this organisation'),
        ('manager', 'Manager', 'Operational access to installed apps')
     ) AS v(code, name, description)
 WHERE t.kind = 'personal'
    ON CONFLICT (tenant_id, code) DO NOTHING;

INSERT INTO workspace.role_permissions (role_id, permission_id)
SELECT r.id, p.id
  FROM workspace.roles r
  JOIN registry.tenants t ON t.id = r.tenant_id AND t.kind = 'personal'
 CROSS JOIN registry.permissions p
 WHERE (r.code = 'admin'
     OR (r.code = 'manager' AND (p.code LIKE '%.read' OR p.code LIKE '%.manage')))
    ON CONFLICT DO NOTHING;
