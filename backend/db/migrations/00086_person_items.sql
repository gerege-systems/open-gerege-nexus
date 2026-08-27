-- Иргэний хүсэлтүүд гэрийн мужид проекц болж суудаг.
--
-- Асуулт нь: нэг суурилуулалт дээрх 100 нийлүүлэгчид хүсэлт гаргасан иргэн
-- «миний хүсэлтүүд»-ээ хаанаас уншдаг вэ.
--
-- Анхны хариулт нь гурав дахь урсгал байсан: `person` schema, дөрөв дэх role
-- `gerege_nexus_person`, `app.current_person` GUC, нийлүүлэгчийн хүснэгт дээр
-- мөрийн түвшний бодлого. Иргэн байгууллага бүрийг дамжин уншина гэсэн үг.
--
-- `00085_personal_workspace` тэр асуултыг өөрчилсөн. Гэр бол **муж мөр**
-- учраас иргэний session-д `app.current_tenant` ҮРГЭЛЖ тавигдана. Тэгвэл
-- «миний хүсэлтүүд» нь гэрийн мужид харьяалагдах ердийн хүснэгт байж болно:
-- байгаа `tenant_isolation`, байгаа `gerege_nexus_tenant`, иргэн зүгээр л
-- өөрийн мужаа уншиж байна. Байгууллага дамнасан уншилт **огт үүсэхгүй** тул
-- түүнээс хамгаалах role ч хэрэггүй.
--
-- Тиймээс энэ файлд шинэ schema, шинэ role, шинэ GUC, шинэ бодлогын хэлбэр
-- байхгүй. Нэг хүснэгт, нэг функц.
--
--
-- ТӨЛӨВ, АГУУЛГА БИШ.
--
-- Мөр бүр нь нийлүүлэгчийн эх бичлэгийн **проекц**: код, төлөв, хугацаа,
-- хариу. Баримт, нотолгоо, хувийн мэдээлэл энд орохгүй — тэдгээр нь
-- нийлүүлэгчийн мужид үлдэнэ, ажил тэнд хийгддэг, audit тэднийх.
--
-- Энэ бол зүгээр нэг зохиомж биш, ухамсартай хязгаар. Проекцууд нийлээд бүх
-- иргэний бүх хүсэлтийн төлөвийг нэг өгөгдлийн санд цуглуулна — Эстони 1996
-- оны алдагдлын дараа X-Road-ыг барихдаа зориудаар татгалзсан төвлөрсөн
-- «superdatabase» рүү нэг алхам. Зөөлрүүлэлт нь гурав: агуулга биш төлөв,
-- мөр бүр гэрийн RLS-ийн ард, ба `answer` дээрх уртын хязгаар. Сүүлийнх нь
-- хамгийн уйтгартай бөгөөд хамгийн үр дүнтэй: 2000 тэмдэгтэд баримт багтахгүй,
-- тиймээс энэ хүснэгт баримтын сан болж «ургах» зам хаалттай.
--
-- Өртөө үүнийг аль хэдийн нэрлэсэн (`00065`): *«толь мөр нь нөгөө талын
-- төлөвийн хуулбар»*. Ялгаа нь ганц — толь нь сүлжээгээр биш, нэг өгөгдлийн
-- сан дотор гүйлгээгээр бичигдэнэ.

-- +goose Up

CREATE TABLE IF NOT EXISTS workspace.person_items (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Гэрийн муж. Энэ баганаас болж мөр нь `tenant_isolation`-ий дор унана —
    -- өөрөөр хэлбэл иргэн өөрийн мөрийг хардаг гэдэг нь энэ файлын шинэ код
    -- биш, 00029-ийн бодлого.
    tenant_id UUID NOT NULL REFERENCES registry.tenants(id) ON DELETE CASCADE,
    -- Хэн хийж байгаа. Иргэн «хаана явна» гэдгээ мэдэх ёстой бөгөөд
    -- байгууллагын нэр нь нийтийн баримт — хувийн мэдээлэл биш.
    provider_tenant_id UUID REFERENCES registry.tenants(id) ON DELETE SET NULL,
    -- Аль модуль нийтэлсэн, ба түүний дотоод id. Хосоороо идемпотент байдлын
    -- түлхүүр: нийлүүлэгч төлөв өөрчлөгдөх бүрд дуудна, мөр нэг л удаа үүснэ.
    source_app TEXT NOT NULL,
    source_ref TEXT NOT NULL,
    code       TEXT NOT NULL,
    status     TEXT NOT NULL,
    answer     TEXT NOT NULL DEFAULT '',
    opened_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT person_items_source_uniq UNIQUE (tenant_id, source_app, source_ref),
    -- Дээрх «агуулга биш» гэсэн шийдвэрийг схем өөрөө барина. Хүн уншихад
    -- зориулсан хариулт 2000 тэмдэгтэд багтана; баримт багтахгүй.
    CONSTRAINT person_items_answer_is_not_a_document CHECK (length(answer) <= 2000)
);

CREATE INDEX IF NOT EXISTS idx_person_items_home ON workspace.person_items (tenant_id, updated_at DESC);

-- Бодлого нь 00029-ийн нарийн хэлбэр, зохиомол биш. `db/migrations/policy_shape_test.go`-д
-- шалтгаантайгаа бүртгэгдэнэ: гэр бол нэг хүний орон зай тул «хамт харах»
-- (`app.allowed_tenants`) энд утгагүй — өөр муж эдгээр мөрийг уншихгүй.
ALTER TABLE workspace.person_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE workspace.person_items FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON workspace.person_items;
CREATE POLICY tenant_isolation ON workspace.person_items TO gerege_nexus_tenant
    USING (tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid);

-- Уншина, бичихгүй.
--
-- `GRANT SELECT` дангаараа хангалтгүй. 00079 нь `ALTER DEFAULT PRIVILEGES IN
-- SCHEMA tenant GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO
-- gerege_nexus_tenant` гэж тавьсан бөгөөд тэр бичлэг 00084-ийн schema
-- нэрлэлтийг даган ирсэн. Тиймээс `workspace`-д шинээр үүсэх хүснэгт бүр
-- **автоматаар** дөрвөн эрхийг авдаг: доорх `GRANT` нь юу ч нэмээгүй, харин
-- `REVOKE` нь жинхэнэ хязгаарлалт.
--
-- Үүнгүйгээр иргэн өөрийн гэрт мөр **зохиож** чадна — RLS нь зөвхөн өөр мужид
-- бичихийг зогсоодог, өөрийн мужид биш. «Яам зөвшөөрлөө» гэсэн мөрийг өөрөө
-- бичих боломж үлдэнэ. Энэ мөрийг `home_db_test.go` барина, тэр тест үүнийг
-- бичиж байх үед нь олсон.
GRANT SELECT ON workspace.person_items TO gerege_nexus_tenant;
REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON workspace.person_items FROM gerege_nexus_tenant;

-- Бичилт: өөр мужид мөр үүсгэх ганц зам.
--
-- Модуль нийлүүлэгчийн мужид ажиллаж байхдаа иргэний гэрт бичих ёстой.
-- `tenant_isolation`-ий `WITH CHECK` үүнийг зогсооно, зөв зогсооно. Репо энэ
-- асуудлыг аль хэдийн нэг удаа шийдсэн — `00034_core_organisation`-ий
-- `create_tenant_profile`:
--
--     SECURITY DEFINER because the insert would otherwise be judged by the RLS
--     policy below, which refuses a row for any tenant other than the bound
--     one — and creating a tenant is precisely the moment when none is bound.
--
-- Үг үсгээрээ ижил тохиолдол, тиймээс ижил хэлбэр.
--
-- ЭНЭ ФУНКЦ RLS-ИЙГ БҮРЭН ТОЙРНО. Тэр нь migration-ыг ажиллуулсан superuser-ийн
-- эрхээр гүйцэтгэгддэг бөгөөд superuser-т `FORCE ROW LEVEL SECURITY` ч
-- үйлчлэхгүй. Тиймээс **функцийн нарийн байдал нь бүх хамгаалалт** — дөрвөн
-- дүрэм, тус бүр нь тестээр баригдана:
--
--   1. Зөвхөн `kind='personal'` муж руу, зөвхөн эзний `ge_id` таарвал. Өөр
--      ямар ч муж руу бичихгүй; таарахгүй бол алдаа, чимээгүй өнгөрөхгүй.
--   2. Зөвхөн `workspace.person_items`, зөвхөн нэрлэсэн багана.
--   3. `UPDATE`/`DELETE` мэдэгдэл байхгүй — зөвхөн `ON CONFLICT … DO UPDATE`.
--      Нийлүүлэгч өөрийн нийтэлсэн мөрөө шинэчилнэ, өөр хэний ч мөрийг биш:
--      мөрийг олох түлхүүр нь `(гэр, source_app, source_ref)` бөгөөд эхний
--      элементийг дуудагч биш функц тогтооно.
--   4. `PUBLIC`-аас эрх хасагдаж, зөвхөн `gerege_nexus_tenant`-д өгөгдөнө.
--
-- Гэр байхгүй бол **алдаа**. Гэр 00085-аар залхуугаар үүсдэг — хүн нэвтрэх
-- агшинд. Нийтлэх агшин нь тэр биш: хэн нэгний өмнөөс гэр үүсгэх нь тэр хүн
-- энэ суурилуулалтад хэзээ ч нэвтрээгүй байхад түүнд орон зай нээж байгаа
-- хэрэг, бөгөөд алдаатай `ge_id` нь чимээгүй шинэ гэр болно.
--
-- `search_path` пиннэсэн: 00079, 00080, 00083 гурав нь функцүүдийн замыг
-- дахин пиннэдэг журамтай бөгөөд шинэ функц тэр жагсаалтад орно. Дотор нь
-- бүх нэр бүтнээр бичигдсэн ч байгаа — зам нь хоёр дахь давхарга.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION registry.publish_person_item(
    p_ge_id              BIGINT,
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
    IF p_ge_id IS NULL OR p_ge_id = 0 THEN
        RAISE EXCEPTION 'publish_person_item: a Gerege number is required'
            USING ERRCODE = 'null_value_not_allowed';
    END IF;
    IF coalesce(p_source_app, '') = '' OR coalesce(p_source_ref, '') = '' THEN
        RAISE EXCEPTION 'publish_person_item: source_app and source_ref are required'
            USING ERRCODE = 'null_value_not_allowed';
    END IF;

    -- Дүрэм 1. Гэрийг эзнээр нь олно, эзнийг `ge_id`-ээр. Хоёр нөхцөл нэг
    -- query-д: муж нь гэр байх ба эзэн нь дамжуулсан хүн байх.
    SELECT t.id INTO home
      FROM registry.tenants t
      JOIN registry.users u ON u.id = t.owner_user_id
     WHERE t.kind = 'personal' AND u.ge_id = p_ge_id;

    IF home IS NULL THEN
        RAISE EXCEPTION 'publish_person_item: no personal workspace for Gerege number %', p_ge_id
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

-- Дүрэм 4. `PUBLIC` нь анхдагчаар `EXECUTE` авдаг тул хасах нь заавал.
REVOKE ALL ON FUNCTION registry.publish_person_item(BIGINT, UUID, TEXT, TEXT, TEXT, TEXT, TEXT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION registry.publish_person_item(BIGINT, UUID, TEXT, TEXT, TEXT, TEXT, TEXT) TO gerege_nexus_tenant;

-- +goose Down

DROP FUNCTION IF EXISTS registry.publish_person_item(BIGINT, UUID, TEXT, TEXT, TEXT, TEXT, TEXT);
DROP TABLE IF EXISTS workspace.person_items;
