-- Нэвтрэхэд workspace шаардлагагүй болов.
--
-- 00085 нь нэвтэрсэн хүн бүрд workspace үүсгэдэг болгосон, учир нь сесс нь
-- нэгийг шаарддаг байсан: `sessions.tenant_id` нь NOT NULL. Тэр шаардлага нь
-- `registry.tenants`-ийг **хүн амаар** өсгөж байсан шалтгаан бөгөөд энэ файл
-- түүнийг авна.
--
-- Хэмжсэн зүйл: 1,000,000 гэр нь 3.9 GB, үүний 2 GB нь нэг гишүүнтэй, тэр нь
-- өөрөө эзэн нь байгаа workspace-үүдийн зөвшөөрлийн жагсаалт. 00092 түүний
-- талыг авсан; энэ нь үлдсэнийг нь авна, учир нь тоолох мөр байхгүй болно.
--
-- ХЭРВЭЭ WORKSPACE БАЙХГҮЙ БОЛ ХҮН ХААНА ЗОГСОХ ВЭ.
--
-- Хаана ч биш, бөгөөд энэ нь дутагдал биш. 00093-аас хойш хүний өөрийнх нь
-- мөрүүд `app.current_user`-аар тусгаарлагдана; тэдгээрийг унших нь ямар ч
-- мужид байхыг шаардахгүй. Байгууллагад хамаарах бүх зүйл `app.current_tenant`
-- хоосон үед **хаагдана** — RLS хаалттай тал руугаа унадаг. Өөрөөр хэлбэл
-- workspace-гүй сесс нь илүү бага эрхтэй, илүү их биш.
--
-- НЭГ НҮХИЙГ ЗАСАВ.
--
-- `sessions`-ийн `tenant_isolation` бодлогод `(tenant_id IS NULL) OR …` гэсэн
-- заалт аль хэдийн байсан. Багана нь NOT NULL байсан тул тэр нь өнөөдрийг
-- хүртэл **үхмэл код** байв. NULL-ыг зөвшөөрсөн агшинд тэр амилж, эзэнгүй сесс
-- бүрийг **бүх мужид** харуулах байлаа — token_hash нь hash хэлбэртэй ч,
-- user_id, IP, user agent нь тийм биш.
--
-- Тиймээс NULL-ын салаа нь эзнээрээ хаагдана. Хоёр салаа, хоёр түлхүүр:
-- мужтай мөрийг муж шийднэ, мужгүй мөрийг хүн шийднэ.

-- +goose Up

ALTER TABLE workspace.sessions ALTER COLUMN tenant_id DROP NOT NULL;

-- `sessions_membership_fk` нь (tenant_id, user_id) хосыг `memberships` руу
-- заадаг. MATCH SIMPLE нь анхдагч бөгөөд түүний утга нь: хосын аль нэг нь NULL
-- бол шалгалт огт хийгдэхгүй. Тиймээс энэ гадаад түлхүүр нь мужтай сессүүдэд
-- хэвээрээ хүчинтэй, мужгүйд нь юу ч шаардахгүй — яг хүсэж буй зан төлөв,
-- өөрчлөх шаардлагагүй.

DROP POLICY IF EXISTS tenant_isolation ON workspace.sessions;
CREATE POLICY tenant_isolation ON workspace.sessions TO gerege_nexus_tenant
    USING (
        CASE WHEN tenant_id IS NULL
             THEN user_id = NULLIF(current_setting('app.current_user', true), '')::uuid
             ELSE tenant_id = ANY (COALESCE(
                    NULLIF(current_setting('app.allowed_tenants', true), '')::uuid[],
                    ARRAY[NULLIF(current_setting('app.current_tenant', true), '')::uuid]))
        END
    );

-- Байгаа гэрүүдийг авна.
--
-- Хоёр ангиллын хэрэглэгч үлдээх нь хамгийн муу үр дүн: нэг хэсэг нь гэртээ
-- буудаг, нөгөө хэсэг нь хаана ч буудаггүй, ялгаа нь тэд хэзээ анх нэвтэрсэн
-- байх. Гэр дотор одоо үнэ цэнэтэй юу ч байхгүй — 00093 хүний мөрүүдийг
-- гаргасан — тул үлдсэн нь гишүүнчлэл, сесс, нэг role, нэг профайл.
--
-- Сессүүд нь эхлээд мужаасаа сална, ингэснээр гэр устахдаа хүнийг гаргачихгүй:
-- `sessions_tenant_id_fkey` нь ON DELETE CASCADE тул үүнгүйгээр нэвтэрсэн хүн
-- бүр яг энэ миграцын үед гарах байлаа.
UPDATE workspace.sessions s
   SET tenant_id = NULL
  FROM registry.tenants t
 WHERE t.id = s.tenant_id AND t.kind = 'personal';

DELETE FROM registry.tenants WHERE kind = 'personal';

-- `kind` ба `owner_user_id` нь ҮЛДЭНЭ.
--
-- Тэдгээр нь хоосон боловч утгагүй биш: SaaS-ийн жишиг бол хувь хүний
-- workspace-ийг **байлгах** (Vercel-ийн hobby team, GitHub-ийн хувийн данс),
-- зөвхөн бүтээгдэхүүнийг ашиглаж эхэлсэн хүнд нь үүсгэх явдал. Өнөөдөр тийм
-- өдөөгч энэ кодод байхгүй тул нэгийг зохиох нь таамаг байх байсан — харин
-- схемийг нь буулгах нь ирээдүйд буцааж бичих ажил.
--
-- Тэднийг уншдаг хоёр шүүлтүүр (консолын жагсаалт, хэмжилтийн collector) мөн
-- үлдэнэ: хоёулаа тесттэй, `kind = 'organisation'` гэдэг нь бүх мөр
-- байгууллага байлаа ч зөв хэвээр.

-- +goose Down

-- Устгасан гэрүүдийг сэргээхгүй. Тэдгээрийн агуулга нь эзний гишүүнчлэл
-- байсан бөгөөд гишүүнчлэл нь тэдэнтэй хамт cascade-аар явсан; хуурамч гэр
-- буцааж зохиох нь буруу огноотой, буруу slug-тай мөр үүсгэнэ.
--
-- Буцаах нь хийж чадах зүйл нь хязгаарлалтыг сэргээх: мужгүй сесс байхаа
-- болино. Тэдгээрийг устгах ёстой, өөрчлөх ямар ч зөв утга байхгүй — аль
-- мужид хамаарахыг нь таамаглах нь хүнийг буруу газар оруулна.
DELETE FROM workspace.sessions WHERE tenant_id IS NULL;

ALTER TABLE workspace.sessions ALTER COLUMN tenant_id SET NOT NULL;

DROP POLICY IF EXISTS tenant_isolation ON workspace.sessions;
CREATE POLICY tenant_isolation ON workspace.sessions TO gerege_nexus_tenant
    USING ((tenant_id IS NULL) OR tenant_id = ANY (COALESCE(
        NULLIF(current_setting('app.allowed_tenants', true), '')::uuid[],
        ARRAY[NULLIF(current_setting('app.current_tenant', true), '')::uuid])));
