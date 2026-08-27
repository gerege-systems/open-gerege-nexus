-- Хүн бүрд өөрийн муж — «гэр».
--
-- Өнөөдөр гишүүнчлэлгүй хүн нэвтэрч чаддаггүй. `FirstTenantFor` нь
-- `workspace.memberships`-аас мөр олохгүй бол `ErrNoOrganisation` буцааж
-- session нээгддэггүй. Схем нь хүнийг байгууллагаас хамааралгүй гэж үздэг
-- (`registry.users` глобал, `ge_id`-д unique index) атал нэвтрэлтийн код
-- түүнийг хориглодог — схемийн зөвшөөрснийг код хориглож буй цорын ганц цэг.
--
-- Одоогийн тойрч гарах зам нь `EID_JIT_TENANT_SLUG` env хувьсагч: eID-ээр
-- анх нэвтэрсэн иргэнийг нэрлэсэн байгууллагад **гишүүн** болгож оруулна.
-- Кодын өөрийнх нь тайлбар үүнийг буруушаасан:
--
--   «provisioning somebody on their first eID sign-in is exactly the path by
--    which an organisation grows without anybody choosing to add a person.»
--
-- Тэр иргэн тухайн байгууллагын квотад тоологдож, лавлахад нь харагдаж,
-- анхдагч эрхийг нь авдаг. Хэн ч түүнийг нэмэхээр шийдээгүй.
--
--
-- ЯАГААД ШИНЭ PLANE БИШ, ШИНЭ ROLE БИШ, ЗҮГЭЭР Л МУЖ ВЭ.
--
-- ADR 0006 гурван хувилбар жагссан: гурав дахь plane, хувийн муж нь муж мөр,
-- эсвэл tenant-гүй session. Механизмаар нь **хоёр дахийг** сонгосон бөгөөд
-- энэ миграц түүний бүх агуулга — хоёр багана.
--
-- Гурав дахь нь (tenant-гүй session) хамгийн үнэн загвар боловч хамгийн
-- үнэтэй: `workspace.sessions.tenant_id` нь NOT NULL, `tenant_isolation`
-- бодлого бүр `app.current_tenant`-аас уншдаг, `dbguard`-ийн switch нь
-- мужгүй context-ыг **login role** руу унагадаг — өөрөөр хэлбэл бүх
-- бодлогын гадна. Нэвтэрсэн иргэний хүсэлт тэр салаанд унах нь эрх
-- нэмэгдүүлэлт болно. Гэр бол муж мөр учир эдгээрийн нэг нь ч хөндөгдөхгүй:
-- session-д муж байна, `current_tenant` тавигдана, RLS хэвээр ажиллана.
--
-- Тиймээс энэ файлд шинэ хүснэгт, шинэ role, шинэ бодлого **байхгүй**.
--
--
-- ХОЁР БАГАНА.
--
-- `kind` — муж хэн болох. Байгаа мөр бүр `organisation`, тэр нь зөв: өнөөдөр
-- бүгд байгууллага. Консол нь `organisation`-ыг л жагсаана (гэр нь операторын
-- жагсаалтад гарах ёсгүй — тэр бол нэг хүний орон зай, удирдах зүйл биш).
--
-- `owner_user_id` — гэр хэнийх. Хоёр зорилготой: гэрийг нь **олох** (нэг
-- SELECT, гишүүнчлэлээр эргэлдэхгүй) ба хоёр дахийг нь **үүсгүүлэхгүй**
-- байх. Хоёр дахийг нь хэсэгчилсэн unique index барина, кодын түгжээ биш:
-- нэг хүн хоёр таб дээр зэрэг нэвтэрч болно.
--
-- CHECK нь хоёр талдаа: гэр эзэнтэй байх ёстой, байгууллага эзэнгүй байх
-- ёстой. Нэг талыг нь бичих нь `kind='organisation'` мөрд эзэн бичих замыг
-- нээлттэй үлдээх байсан бөгөөд тэр мөр аль ч жагсаалтад буруу гарна.

-- +goose Up

ALTER TABLE registry.tenants
    ADD COLUMN IF NOT EXISTS kind          TEXT NOT NULL DEFAULT 'organisation',
    ADD COLUMN IF NOT EXISTS owner_user_id UUID REFERENCES registry.users(id) ON DELETE CASCADE;

ALTER TABLE registry.tenants DROP CONSTRAINT IF EXISTS tenants_kind_known;
ALTER TABLE registry.tenants
    ADD CONSTRAINT tenants_kind_known CHECK (kind IN ('organisation', 'personal'));

ALTER TABLE registry.tenants DROP CONSTRAINT IF EXISTS tenants_home_has_an_owner;
ALTER TABLE registry.tenants
    ADD CONSTRAINT tenants_home_has_an_owner
        CHECK ((kind = 'personal') = (owner_user_id IS NOT NULL));

CREATE UNIQUE INDEX IF NOT EXISTS tenants_one_home_per_person
    ON registry.tenants (owner_user_id) WHERE kind = 'personal';

-- +goose Down

DROP INDEX IF EXISTS registry.tenants_one_home_per_person;
ALTER TABLE registry.tenants DROP CONSTRAINT IF EXISTS tenants_home_has_an_owner;
ALTER TABLE registry.tenants DROP CONSTRAINT IF EXISTS tenants_kind_known;
ALTER TABLE registry.tenants
    DROP COLUMN IF EXISTS owner_user_id,
    DROP COLUMN IF EXISTS kind;
