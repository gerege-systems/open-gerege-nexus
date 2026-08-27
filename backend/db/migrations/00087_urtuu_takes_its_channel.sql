-- Өртөө сувгаа авч явлаа. Цөмд Өртөөгийн юу ч үлдэхгүй.
--
-- 00078 нь даалгаврын самбарын гурван хүснэгтийг аппд өгөөд сувгийн зургааг
-- нь үлдээж, яагаад үлдээж байгаагаа бичсэн: «холбоос нь администраторын
-- тогтоосон зүйл, дээр нь ямар апп ирж очих нь тэр холбоосын асуудал биш».
-- Тэр маргаан нэг нөхцөлтэй байсан — сувгийг **хоёр ба түүнээс дээш** апп
-- ашиглана гэдэг. Гурван сарын дараа ч ашиглагч нь ганц хэвээр: Өртөөгийн
-- самбар, client-gerege-nexus дээр. Ганц хэрэглэгчтэй рельс бол рельс биш,
-- тэр аппын дотор эрхтэн.
--
-- Тиймээс тээвэр нь ч аппынхаа хамт явлаа: modules/urtuu/channel. Түүнтэй
-- хамт `pkg/urtuu` (дугтуйн гэрээ) ба `pkg/nexus`-ийн Link, PeerDirectory
-- гэрээнүүд мөн явав — цөмд хэрэгжүүлэгчгүй үлдэх гэрээ нь SDK-д аль хэдийн
-- нэг удаа гарсан алдаа (MeetingBooker, pkg/nexus/capability.go-гийн
-- тайлбарыг үз): interface бий, адаптер бий, авах арга нь байхгүй.
--
-- Зургаан хүснэгт:
--
--   urtuu_peers          холбоос, нөгөө талын нийтийн түлхүүр, token-ы hash
--   urtuu_outbox         гарах дугтуй
--   urtuu_deliveries     хүргэлт бүрийн оролдлого, баталгаа
--   urtuu_inbox          ирсэн дугтуй, идемпотент хүлээн авалт
--   urtuu_request_codes  хүсэлтийн кодын толь
--   urtuu_peer_codes     тухайн холбоос дээр нээгдсэн кодууд
--
-- Аппын өөрийн миграц (client-gerege-nexus, modules/urtuu/migrations/
-- 00002_the_channel_is_ours.sql) зургуулангийнх нь эцсийн хэлбэрийг
-- `IF NOT EXISTS`-ээр дахин зарлана — 00001-ийн яг тэр арга.
--
-- ӨГӨГДӨЛ УСТАНА. 00078-ийн адил ил тод шийдвэр, мөн адил тоолсны эцэст:
-- 2026-08-27-нд nexus.gerege.mn, client.gerege.mn, sso.gerege.mn гурван
-- суулгац дээр зургаан хүснэгтийн мөрийн тоо **бүгд 0** байв. Аль нэг хост
-- дээр холбоос байгуулагдсан бол дараалал нь энгийн: миграцаас өмнө
-- `CREATE TABLE urtuu_peers_keep AS SELECT * FROM workspace.urtuu_peers`
-- (зургуулангаар), rollout, дараа нь гадаад түлхүүрийн дарааллаар буцааж
-- INSERT — appstore-ийн store_* хүснэгтүүдэд ажилласан яг тэр алхам.
--
-- CASCADE: зургаа нь бие бие рүүгээ заадаг (outbox → peers, deliveries →
-- outbox, peer_codes → peers). Дарааллыг гараар бичихийн оронд CASCADE —
-- 00075-ийн шийдэл, ижил шалтгаанаар.

-- +goose Up

DROP TABLE IF EXISTS workspace.urtuu_peer_codes CASCADE;
DROP TABLE IF EXISTS workspace.urtuu_request_codes CASCADE;
DROP TABLE IF EXISTS workspace.urtuu_inbox CASCADE;
DROP TABLE IF EXISTS workspace.urtuu_deliveries CASCADE;
DROP TABLE IF EXISTS workspace.urtuu_outbox CASCADE;
DROP TABLE IF EXISTS workspace.urtuu_peers CASCADE;

-- +goose Down

-- Буцаах зам байхгүй, 00075 / 00077 / 00078-ийн адил. Хүснэгтүүд нь одоо
-- аппынх; энд дахин үүсгэх нь эзэмшлийг эргүүлэн авах гэсэн үг бөгөөд
-- аппын миграц дараа нь өөрийнхөө гэж зарлахад хоёр түүх нэг хүснэгтийг
-- нэхэмжилнэ.
SELECT 1;
