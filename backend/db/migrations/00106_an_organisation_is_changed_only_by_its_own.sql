-- Байгууллагын мөр, платформын тохиргоо, ба нэвтрэлтийн хоёр түр хүснэгт.
--
--
-- ЮУ ХӨНДӨГДӨӨГҮЙ БАЙВ.
--
-- 00029 нь `tenant_id` баганатай бүхнийг, 00102 нь хүнээр тусгаарлагдах
-- гурвыг хамгаалсан. `registry` дотор аль алинд нь багтахгүй хүснэгтүүд
-- үлдсэн — тэдгээр дээр тенантын роль өнөөг хүртэл RLS-гүй, дөрвөн эрхтэй
-- (SELECT/INSERT/UPDATE/DELETE) байв. Хамгийн хүндрэлтэй нь `tenants` өөрөө:
-- заалтаа мартсан, эсвэл id-гаа буруу хувьсагчаас авсан нэг UPDATE хөрш
-- байгууллагын нэрийг сольж, нэг DELETE түүнийг бүхэлд нь устгана
-- (00001-ээс хойш бүх хүснэгт cascade-аар холбоотой).
--
--
-- УНШИЛТ ХААГДАХГҮЙ, ЗАСВАР ХААГДАНА.
--
-- `tenants` дээр `tenant_isolation`-ы ердийн хэлбэр буруу байх нэг шалтгаан
-- бий: энэ хүснэгтийг байгууллага хоорондоо уншдаг бөгөөд тэр нь алдаа биш,
-- зориулалт нь юм —
--
--   * `internal/person` — тенантгүй хүн үйлчилгээ үзүүлэгчдийн лавлахыг үздэг;
--   * `auth/tenants.go` — «би хаана ажилладаг вэ» жагсаалт (сэлгэгч);
--   * `reporting/grants.go` — тайлангийн хуваалцлага нь ХОЁР байгууллагын
--     нэрийг зэрэг харуулдаг;
--   * `profile/organisation.go` — хүүхэд байгууллага эцгийнхээ нэрийг уншдаг.
--
-- Эдгээрийн аль нэг нь бодлогод тусаагүй бол хариу нь алдаа биш, ХООСОН
-- нэр байх байсан — 00102-ын тайлбарт бичсэн «эрх нь хэвээр атлаа хариу нь
-- хоосон» гэдэг яг тэр. Тиймээс уншилтыг зориудаар нээлттэй үлдээв
-- (USING (true)) ба энэ нь тайлбартай шийдвэр болохоос хуулбар биш.
--
-- Засварыг харин идэвхтэй байгууллагаараа хаана. Ажлын талын БҮХ бичилт
-- өөрийнх нь мөр рүү явдаг (`profile/organisation.go`: нэр, элсэлтийн журам).
-- Шинэ байгууллага үүсгэх, түдгэлзүүлэх, устгах нь консол (operator role)
-- эсвэл платформын зам (нэвтрэх үеийн хувийн орон зай, wizard) — тенантын
-- ролид INSERT/DELETE хэрэггүй тул эрхийг нь авна.
--
--
-- ПЛАТФОРМЫН ТОХИРГОО.
--
-- `platform_settings`-ыг бичдэг цорын ганц зам бол консолын гүйлгээ
-- (`settings.Store.Set`, түүхийн мөртэйгээ хамт). Уншилт нь ачаалах үед ба
-- кэш дахин уншихад — хоёулаа `context.Background()`, өөрөөр хэлбэл платформын
-- зам. Тенантын холболтоос энэ хүснэгт рүү бичих ямар ч шалтгаан байхгүй тул
-- гурван бичих эрхийг авна; SELECT үлдэнэ (нууц утга энд байдаггүй — түлхүүр,
-- нууцууд `internal/kernel/credentials`-д).
--
--
-- НЭВТРЭЛТИЙН ХОЁР ТҮР ХҮСНЭГТ.
--
-- `credential_grants` (урилга, нууц үг сэргээх нэг удаагийн токен) ба
-- `identity_binding_sessions` (Google-ийн танилтыг eID-тэй холбох хүртэл
-- түр хадгалагдах баталгаажсан claim-ууд) хоёуланг нь зөвхөн session-гүй
-- маршрутууд хөнддөг: /auth/credential*, /auth/bind/* нь authn middleware-ийн
-- ГАДНА, цэвэрлэгээ нь фон дээр. Тэгэхээр эдгээр нь платформын зам дээр л
-- ажиллана — тенантын ролиос эрхийг бүхэлд нь авна. Бодлого биш, эрх.
--
-- `eid_sign_state` энд ОРООГҮЙ. Түүнийг байгууллагын дотроос гарын үсэг
-- зурах урсгал бичдэг (`signing/handlers_batch.go` → `eid.SignPDF`, session-тэй,
-- тенантаар холбогдсон), мөн нэвтрэх урсгал бичдэг. Хүснэгт нь `tenant_id`-гүй,
-- түлхүүр нь таамаглашгүй, мөр нь хорин минутын настай: RLS-ийн бодлого нь
-- `true` л байх байсан бөгөөд `true` бол хамгаалалт биш. Багана нэмэх нь энэ
-- миграцын ажил биш тул хэвээр үлдээж, шалтгааныг нь энд бичив.

-- +goose Up

ALTER TABLE registry.tenants ENABLE ROW LEVEL SECURITY;
ALTER TABLE registry.tenants FORCE ROW LEVEL SECURITY;

-- Уншилт: зориудаар бүгд. Дээрх дөрвөн хэрэглэгч байгууллага хооронд уншдаг.
CREATE POLICY organisations_are_read_across ON registry.tenants
    FOR SELECT TO gerege_nexus_tenant USING (true);

-- Засвар: зөвхөн идэвхтэй байгууллага. Өргөн хэлбэр (`allowed_tenants`) энд
-- буруу байх байсан — хүн хоёр байгууллагад харьяалагддаг гэдэг нь тэр хоёрыг
-- нэгээс нь зэрэг ЗАСАХ эрх биш; 00037 өөрөө уншилтыг өргөтгөж, бичилтийг
-- идэвхтэй байгууллагад нь үлдээсэн.
CREATE POLICY an_organisation_is_changed_by_its_own ON registry.tenants
    FOR UPDATE TO gerege_nexus_tenant
    USING (id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)
    WITH CHECK (id = NULLIF(current_setting('app.current_tenant', true), '')::uuid);

-- Консол нь 00049-ийн эрхээрээ: RLS асаагдсан тул бодлогогүй роль юу ч
-- хийхгүй болно. Түдгэлзүүлэх, сэргээх, устгалыг товлох, засвар үйлчилгээний
-- горим — бүгд энэ хүснэгт дээрх UPDATE.
CREATE POLICY the_console_sees_every_organisation ON registry.tenants
    FOR ALL TO gerege_nexus_operator USING (true) WITH CHECK (true);

-- Шинэ байгууллага нь консол эсвэл платформын зам. Ажлын тал нэгийг ч
-- үүсгэдэггүй, устгадаггүй.
REVOKE INSERT, DELETE ON registry.tenants FROM gerege_nexus_tenant;

-- Платформын тохиргоог консол бичнэ.
REVOKE INSERT, UPDATE, DELETE ON registry.platform_settings FROM gerege_nexus_tenant;

-- Session-гүй маршрутуудын хоёр хүснэгт.
REVOKE ALL ON registry.credential_grants FROM gerege_nexus_tenant;
REVOKE ALL ON registry.identity_binding_sessions FROM gerege_nexus_tenant;

-- +goose Down

GRANT SELECT, INSERT, UPDATE, DELETE ON registry.identity_binding_sessions TO gerege_nexus_tenant;
GRANT SELECT, INSERT, UPDATE, DELETE ON registry.credential_grants TO gerege_nexus_tenant;
GRANT INSERT, UPDATE, DELETE ON registry.platform_settings TO gerege_nexus_tenant;
GRANT INSERT, DELETE ON registry.tenants TO gerege_nexus_tenant;

DROP POLICY IF EXISTS the_console_sees_every_organisation ON registry.tenants;
DROP POLICY IF EXISTS an_organisation_is_changed_by_its_own ON registry.tenants;
DROP POLICY IF EXISTS organisations_are_read_across ON registry.tenants;
ALTER TABLE registry.tenants NO FORCE ROW LEVEL SECURITY;
ALTER TABLE registry.tenants DISABLE ROW LEVEL SECURITY;
