-- «Хэн Д-101 нээсэн бэ» — суурилуулалт даяарх лавлах.
--
-- 00089-өөр иргэн байгууллагад өөрөө хүсэлт гаргадаг болсон ч байгууллагаа
-- **slug-аар** нэрлэх ёстой: хаяган дахь богино нэрийг хэн нэгэн түүнд
-- урьдчилж хэлсэн байх ёстой. Тэр нь танил байгууллагад ажилладаг, олон
-- нийтийн үйлчилгээнд ажиллахгүй. Иргэн «жолооны үнэмлэх сунгуулах» гэдгээ
-- мэднэ, тэрийг хэн хийдгийг мэдэхгүй.
--
-- Тиймээс код → түүнийг нээсэн байгууллагууд гэсэн хайлт хэрэгтэй.
--
--
-- ЯАГААД `registry`-Д ВЭ.
--
-- Энэ бол нэг гэрийн ч, нэг байгууллагын ч биш, **суурилуулалтын** баримт.
-- `registry` schema-гийн зарлагдсан зорилго үүнийг яг багтаана: «суурилуулалтын
-- бүртгэл: хэн байна, **юу байна**». Гэр бүрд хуулбарлах нь утгагүй бөгөөд
-- 00086-гийн проекцоос ялгаатай нь энд хувийн зүйл байхгүй — нийтэд зарлагдсан
-- үйлчилгээ.
--
-- Хоёр role аль аль нь `registry` дээр USAGE-тай тул уншилт нэмэлт эрх
-- шаардахгүй.
--
--
-- НИЙТЛЭХ НЬ OPT-IN.
--
-- Байгууллага код нээсэн болгон энд гарахгүй. Мөр энд байгаа нь өөрөө
-- шийдвэр — хүснэгт нь **нийтлэл мөн**, тусгай `published` багана хэрэггүй.
--
-- Яагаад гэдэг нь чухал: код нээх нь **дотоод зохион байгуулалт** («бид энэ
-- төрлийн хүсэлтийг хүлээж авдаг»), лавлахад гарах нь **олон нийтэд өгсөн
-- амлалт** («бидэн рүү хандаж болно»). Хоёрыг нэг үйлдэл болговол
-- байгууллага дотоод урсгалаа тохируулаад олон нийтийн үйлчилгээ санамсаргүй
-- зарласан байна.
--
--
-- `local.` УГТВАРТАЙ КОД ХЭЗЭЭ Ч ОРОХГҮЙ.
--
-- 00062 үгсийн сангийн нэрийн орон зайг тогтоосон: ring-д байхгүй, байгууллага
-- өөрөө зохиосон код заавал `local.` угтвартай. Тэдгээр нь тухайн байгууллагын
-- дотоод орон зай — суурилуулалт даяарх лавлахад гарвал нэрийн орон зайн
-- дүрэм задарна, мөн хоёр өөр байгууллагын `local.x` нэг мөр мэт харагдана.
--
-- Дүрэм схемд суусан, Go-д биш. 00062-ын өөрийнх нь өгүүлбэр:
-- *«Код бүрийг шалгадаг Go функц нэг өдөр мартагдана; CHECK мартагдахгүй.»*
--
--
-- ЭРЭМБЭ: ЦАГААН ТОЛГОЙ.
--
-- Зориудаар сонгосон бөгөөд уйтгартай нь давуу тал. Хувилбарууд ба тэднийг
-- сонгоогүй шалтгаан:
--
--   * **Санамсаргүй** — «шударга» мэт сонсогдох ч хэн ч давтаж шалгаж
--     чадахгүй, гомдол ирэхэд хариулах зүйлгүй.
--   * **Ачаалал, хурдаар** — нийлүүлэгчийн гүйцэтгэлийг тэдний зөвшөөрөлгүй
--     нийтэлнэ, мөн жижиг байгууллагыг үүрд доор байлгана.
--   * **Нийтэлсэн огноогоор** — эрт нийтэлсэн нь дээр гарна, тэр нь зөвхөн
--     хэн эхэлж мэдсэнийг шагнана.
--
-- Цагаан толгойн эрэмбэ нь **давтагдана**, тайлбарлагдана, хэн ч давуу эрх
-- худалдаж авахгүй. Дүрэмгүй үлдээх нь хамгийн муу нь: тэр үед эрэмбэ нь
-- хэн нэгний бичсэн `ORDER BY`-ийн санамсаргүй үр дүн болж, санаагүйгээр
-- зар сурталчилгааны талбар болно.

-- +goose Up

CREATE TABLE IF NOT EXISTS registry.service_directory (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL REFERENCES registry.tenants(id) ON DELETE CASCADE,
    code         TEXT NOT NULL,
    -- Байгууллага энэ кодыг юу гэж нэрлэдэг. Үгсийн сангийн албан ёсны нэр нь
    -- ring дээр байдаг; энэ нь тухайн байгууллагын хэлсэн үг, хоосон байж
    -- болно.
    title        TEXT NOT NULL DEFAULT '',
    published_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_by UUID REFERENCES registry.users(id) ON DELETE SET NULL,
    CONSTRAINT service_directory_unique UNIQUE (tenant_id, code),
    CONSTRAINT service_directory_code_is_a_code
        CHECK (code <> '' AND length(code) <= 128 AND title = btrim(title)),
    -- Дээрх «`local.` хэзээ ч орохгүй». Go-гийн шалгалт биш.
    CONSTRAINT service_directory_no_local_namespace
        CHECK (code NOT LIKE 'local.%')
);

CREATE INDEX IF NOT EXISTS idx_service_directory_code ON registry.service_directory (code);

-- Нийтэд уншигдана, эзэндээ бичигдэнэ.
--
-- Уншилт нь хилгүй байх нь энэ хүснэгтийн бүх учир — иргэн хайхын тулд бүх
-- байгууллагын мөрийг харах ёстой. Бичилт нь харин `WITH CHECK`-ээр өөрийн
-- мужид хязгаарлагдана: `registry`-гийн бусад хүснэгтийн адил RLS-гүй
-- үлдээвэл нэг байгууллагын session өөр байгууллагын нэрээр үйлчилгээ
-- зарлаж чадах байлаа.
ALTER TABLE registry.service_directory ENABLE ROW LEVEL SECURITY;
ALTER TABLE registry.service_directory FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS directory_is_public_but_owned ON registry.service_directory;
CREATE POLICY directory_is_public_but_owned ON registry.service_directory TO gerege_nexus_tenant
    USING (true)
    WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid);
DROP POLICY IF EXISTS directory_is_public_to_the_console ON registry.service_directory;
CREATE POLICY directory_is_public_to_the_console ON registry.service_directory
    FOR SELECT TO gerege_nexus_operator USING (true);

GRANT SELECT, INSERT, DELETE ON registry.service_directory TO gerege_nexus_tenant;
GRANT SELECT ON registry.service_directory TO gerege_nexus_operator;

-- +goose Down

DROP TABLE IF EXISTS registry.service_directory;
