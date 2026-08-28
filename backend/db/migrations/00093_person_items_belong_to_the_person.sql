-- Хүний өөрийнх нь мөр хүнийх болов.
--
-- 00086 нь `person_items`-ийг `workspace`-д, гэрийн мужаар түлхүүрлэн үүсгэсэн.
-- Тухайн үед хамгийн хямд зөв хариулт байсан: гэр бол нэг хүний орон зай тул
-- мужаар тусгаарлах нь хүнээр тусгаарлахтай ижил утгатай байв, бөгөөд 00029-ийн
-- бодлого аль хэдийн байсан.
--
-- Гурван зүйл үүнийг өөрчилж байна.
--
-- **Нэг.** Мөр нь хүнийг дагах ёстой. Иргэн маргааш компанид орж, нөгөөдөр
-- өөрөө компани нээж болно. Түүний «яамнаас ирсэн хариу» нь тэр бүх шилжилтийн
-- туршид түүнийх хэвээр байх ёстой — харин муж дотор сууж байвал тэр нь гэр
-- гэдэг нэг workspace-ийнх бөгөөд workspace уствал хамт устана.
--
-- **Хоёр.** Тусгаарлах түлхүүр нь тусгаарлаж буй зүйл өөрөө байх ёстой.
-- Тусгаарлаж буй зүйл нь хүн бол түлхүүр нь хүн. Муж бол хүнийг илэрхийлэх
-- төлөөлөгч байсан бөгөөд төлөөлөгч хэзээ нэгэн цагт саладаг.
--
-- **Гурав.** Энэ нь гэрийг **шаардлагагүй** болгодог. Нэвтэрсэн хүн бүрд
-- workspace үүсгэх нь `registry.tenants`-ийг хүн амаар өсгөж байсан бөгөөд
-- өнөөдөр гэр нь голчлон энэ хүснэгтийн төлөө оршиж байна. Дараагийн алхам
-- түүнийг авна; энэ файл түүнийг боломжтой болгоно.
--
-- ХҮСНЭГТ БАЙРАНДАА ХУВИРНА, ШИНЭЭР ҮҮСЭХГҮЙ.
--
-- `SET SCHEMA` + баганын засвар нь `CREATE` + хуулах + `DROP`-оос гурван
-- шалтгаанаар дээр: өгөгдөл хөдлөхгүй тул хуулах явцад мөр алдагдах зам байхгүй,
-- эрх ба бодлого хүснэгтээ дагаж явна (00084 үүнийг schema нэр солихдоо аль
-- хэдийн баталсан), мөн хүснэгтийн ижид хэвээр үлдэнэ — `ownership_test.go`
-- миграцуудыг уншиж «энэ нэр эцэст нь оршиж байна уу» гэдгийг тоолдог бөгөөд
-- нэг файл дотор ижил нэрийг үүсгээд устгах нь тэр тооллыг тэглэдэг.
--
-- ШИНЭ БАЙГАА ЗҮЙЛ: `app.current_user`.
--
-- Хүртэл dbguard холболт бүрд `role`, `app.current_tenant`,
-- `app.allowed_tenants` гурвыг холбодог байв. Одоо дөрөв дэх нь нэмэгдэнэ.
-- Хэлбэр нь `app.current_tenant`-тай яг ижил бөгөөд бүтэлгүйтэх нь ч ижил:
-- тавигдаагүй бол `NULLIF` нь NULL өгнө, `user_id = NULL` нь NULL, мөр
-- харагдахгүй. **Хаалттай тал руугаа унана** — RLS-д энэ хэлбэр л зөв.

-- +goose Up

-- Схемээ дагаж эрх, бодлого, индекс бүгд хамт нүүнэ.
ALTER TABLE workspace.person_items SET SCHEMA registry;

ALTER TABLE registry.person_items
    ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES registry.users(id) ON DELETE CASCADE;

-- Гэр бүрийн `owner_user_id` нь яг хайж буй хүн: 00085-ийн
-- `tenants_home_has_an_owner` шалгуур нь `kind='personal'` мөр бүрд эзэн
-- байхыг баталгаажуулна.
UPDATE registry.person_items i
   SET user_id = t.owner_user_id
  FROM registry.tenants t
 WHERE t.id = i.tenant_id;

-- Эзэнгүй муж руу нийтлэгдсэн мөр байвал — байх ёсгүй, учир нь функц
-- `kind='personal'`-аас өөр муж руу бичдэггүй байсан — түүнийг авч явна.
-- Хэнийх нь мэдэгдэхгүй мөрийг «хэн нэгнийх» гэж таамаглах нь буруу хүнд
-- харуулах зам.
DELETE FROM registry.person_items WHERE user_id IS NULL;

ALTER TABLE registry.person_items ALTER COLUMN user_id SET NOT NULL;

-- Хуучин бодлого эхлээд, учир нь тэр `tenant_id`-г уншдаг: багана дээр
-- түшиглэсэн объект байхад Postgres түүнийг хаяхыг зөвшөөрөхгүй. Дараалал нь
-- шаардлага бөгөөд алдаа нь чимээгүй биш — миграц зогсоно.
DROP POLICY IF EXISTS tenant_isolation ON registry.person_items;

-- `tenant_id`-г хаях нь түүн дээрх `person_items_source_uniq` хязгаарлалт ба
-- `idx_person_items_home` индексийг хамт авна, тиймээс шинээр тавина.
ALTER TABLE registry.person_items DROP COLUMN tenant_id;

ALTER TABLE registry.person_items
    ADD CONSTRAINT person_items_source_uniq UNIQUE (user_id, source_app, source_ref);
CREATE INDEX IF NOT EXISTS idx_person_items_person
    ON registry.person_items (user_id, updated_at DESC);

-- Бодлого нь хүнээр, мужаар биш. Нэр нь `tenant_isolation` БИШ бөгөөд энэ нь
-- зориуд: `policy_shape_test.go` тэр нэрийг мужаар тусгаарладаг хүснэгтүүдийн
-- бүртгэл болгон уншдаг, энэ нь тэдний нэг биш.
DROP POLICY IF EXISTS person_isolation ON registry.person_items;
CREATE POLICY person_isolation ON registry.person_items TO gerege_nexus_tenant
    USING (user_id = NULLIF(current_setting('app.current_user', true), '')::uuid)
    WITH CHECK (user_id = NULLIF(current_setting('app.current_user', true), '')::uuid);

-- Уншина, бичихгүй — 00086-гийн шийдвэр хэвээр.
--
-- `registry` дээр `ALTER DEFAULT PRIVILEGES` байхгүй тул `workspace`-ийн шиг
-- чимээгүй өгөгдөх дөрвөн эрх энд байхгүй. Доорх `REVOKE` тиймээс өнөөдөр юу ч
-- хийхгүй бөгөөд зориуд байна: `registry`-д хэзээ нэгэн цагт default privilege
-- нэмэгдвэл иргэн өөртөө «яам зөвшөөрлөө» гэсэн мөр **зохиож** чадах болно.
-- RLS үүнийг зогсоохгүй — тэр зөвхөн өөр хүний мөрийг зогсооно, өөрийнхийг
-- биш. Хямд даатгал, өмнө нь нэг удаа бодитоор хэрэгтэй болсон (00086).
GRANT SELECT ON registry.person_items TO gerege_nexus_tenant;
REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON registry.person_items FROM gerege_nexus_tenant;

-- Бичих ганц зам, одоо гэр хайхгүй.
--
-- ЭНЭ ФУНКЦ RLS-ИЙГ БҮРЭН ТОЙРНО — SECURITY DEFINER нь migration-ыг
-- ажиллуулсан superuser-ийн эрхээр гүйцэтгэгдэх бөгөөд superuser-т
-- `FORCE ROW LEVEL SECURITY` ч үйлчлэхгүй. Тиймээс функцийн нарийн байдал нь
-- бүх хамгаалалт хэвээр: зөвхөн энэ хүснэгт, зөвхөн нэрлэсэн багана,
-- `UPDATE`/`DELETE` мэдэгдэлгүй, `PUBLIC`-аас эрх хасагдсан.
--
-- 00086-гийн «зөвхөн kind='personal' муж руу» гэсэн дүрэм алга болов, учир нь
-- бичих газар нь муж байхаа больж `users`-ийн гадаад түлхүүр болсон. Байхгүй
-- хүн рүү бичих оролдлого одоо тэр түлхүүрээр таслагдана: хайлт амжилтгүй
-- болоод чимээгүй өнгөрөх байсан газарт өгөгдлийн сан татгалзана.
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
LANGUAGE plpgsql SECURITY DEFINER SET search_path = registry, workspace AS $fn$
BEGIN
    IF p_user_id IS NULL THEN
        RAISE EXCEPTION 'publish_person_item: a person is required'
            USING ERRCODE = 'null_value_not_allowed';
    END IF;
    IF coalesce(p_source_app, '') = '' OR coalesce(p_source_ref, '') = '' THEN
        RAISE EXCEPTION 'publish_person_item: source_app and source_ref are required'
            USING ERRCODE = 'null_value_not_allowed';
    END IF;

    INSERT INTO registry.person_items
        (user_id, provider_tenant_id, source_app, source_ref, code, status, answer)
    VALUES
        (p_user_id, p_provider_tenant_id, p_source_app, p_source_ref, p_code, p_status, coalesce(p_answer, ''))
    ON CONFLICT (user_id, source_app, source_ref) DO UPDATE
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

ALTER TABLE registry.person_items
    ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES registry.tenants(id) ON DELETE CASCADE;

UPDATE registry.person_items i
   SET tenant_id = t.id
  FROM registry.tenants t
 WHERE t.owner_user_id = i.user_id AND t.kind = 'personal';

-- Гэргүй хүний мөр буцах газаргүй. Энэ бол `Up`-ийн шууд үр дагавар: хүнээр
-- түлхүүрлэсэн хүснэгт нь мужаар түлхүүрлэсэн хүснэгтээс илүү зүйл илэрхийлж
-- чадна, тиймээс буцаалт нь алдагдалтай — бүрэн бус биш, харин илэрхийлэх
-- боломжгүй мөрийг хаяж байгаа.
DELETE FROM registry.person_items WHERE tenant_id IS NULL;

ALTER TABLE registry.person_items ALTER COLUMN tenant_id SET NOT NULL;

-- Ижил дараалал, ижил шалтгаанаар: бодлого нь баганаа түшинэ.
DROP POLICY IF EXISTS person_isolation ON registry.person_items;
ALTER TABLE registry.person_items DROP COLUMN user_id;

ALTER TABLE registry.person_items
    ADD CONSTRAINT person_items_source_uniq UNIQUE (tenant_id, source_app, source_ref);
CREATE INDEX IF NOT EXISTS idx_person_items_home
    ON registry.person_items (tenant_id, updated_at DESC);

DROP POLICY IF EXISTS tenant_isolation ON registry.person_items;
CREATE POLICY tenant_isolation ON registry.person_items TO gerege_nexus_tenant
    USING (tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid);

ALTER TABLE registry.person_items SET SCHEMA workspace;

GRANT SELECT ON workspace.person_items TO gerege_nexus_tenant;
REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON workspace.person_items FROM gerege_nexus_tenant;

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
