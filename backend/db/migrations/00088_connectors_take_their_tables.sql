-- Гадаад системийн холбогч ба webhook аппаа дагав.
--
-- Гурван хүснэгт: `integrations` (холбогч бүрийн тохиргоо, шифрлэгдсэн
-- итгэмжлэл), `integration_oauth_states` (нэг удаагийн OAuth state),
-- `integration_deliveries` (юу гадагш явсны бүртгэл).
--
-- Каталогийн бичлэг нь 2026-08-23-нд аль хэдийн апп болсон
-- (`io.gerege.nexus.integrations`) — тэр өдөр дэлгэц нь хаалганы ард орсон
-- бөгөөд рельс нь цөмд үлдэх шалтгаан бичигдсэн байв: «PDF гарын үсгийн рельс
-- баримтаа түүгээр илгээдэг, nexus.MeetingBooker нь түүний адаптер».
--
-- Хоёулаа шалгалтыг давсангүй. `MeetingBooker`-ыг ганц ч модуль хэзээ ч
-- дуудсангүй — гэрээ бий, адаптер бий, дуудагч алга — тул тэр нь рельс биш
-- байв. Экспорт нь ганц дуудагчтай: esign рельс өөрөө. Түүнийг үлдээхийн тулд
-- цөм «холбогч суулгагдсан уу» гэж бүртгэлээс асуух хэрэгтэй болох байсан тул
-- 2026-08-27-нд экспортыг цөмөөс бүрмөсөн хасав (`internal/workspace/signing/
-- export.go`, `POST /esign/documents/{id}/export`). Гарын үсэг зурагдаж,
-- хадгалагдаж, татагдсаар байна; хаана хуулахыг нь татсан тал шийднэ.
--
-- Цөмд үлдсэн нь **шифр**: нэг суулгацад нэг л түлхүүр байх ёстой тул
-- `INTEGRATION_ENCRYPTION_KEY` ба AES-GCM нь `internal/kernel/security`-д
-- үлдэж, гадагш `nexus.SecretSealer` гэрээгээр гарна. Хувьсагчийн нэр
-- өөрчлөгдөөгүй: тэр нь ижил түлхүүр бөгөөд нэр солих нь суулгац бүрийн
-- хийх ажил, харин ашиг нь зөвхөн үг.
--
-- ӨГӨГДӨЛ УСТАНА. 2026-08-27-нд гурван суулгац дээр тоолсон: nexus.gerege.mn
-- дээр 2 мөр (Dropbox, Google Drive — хоёулаа INACTIVE, хэзээ ч холбогдож
-- байгаагүй, connected_at NULL), client.gerege.mn ба sso.gerege.mn дээр 0.
-- Холбогдсон грант байгаа хост дээр зөөх зам нээлттэй: миграцаас өмнө
-- `CREATE TABLE integrations_keep AS SELECT * FROM workspace.integrations`,
-- rollout, дараа нь буцааж INSERT — appstore-ийн store_* хүснэгтүүдэд
-- ажилласан яг тэр алхам. Шифрлэгдсэн баганууд нь ижил түлхүүрээр
-- уншигдана: аппын код ижил cipher-ийг `nexus.SecretSealer`-ээр дуудна.
--
-- CASCADE: oauth_states ба deliveries хоёул integrations руу заана.

-- +goose Up

DROP TABLE IF EXISTS workspace.integration_deliveries CASCADE;
DROP TABLE IF EXISTS workspace.integration_oauth_states CASCADE;
DROP TABLE IF EXISTS workspace.integrations CASCADE;

-- +goose Down

-- Буцаах зам байхгүй, 00075 / 00077 / 00078 / 00087-ийн адил: хүснэгтүүд нь
-- одоо аппынх бөгөөд энд дахин үүсгэх нь эзэмшлийг эргүүлэн авах гэсэн үг.
SELECT 1;
