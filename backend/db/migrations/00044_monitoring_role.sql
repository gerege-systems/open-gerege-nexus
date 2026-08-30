-- postgres_exporter-т зориулсан унших эрхтэй нэвтрэх role.
--
-- Exporter нь pg_stat_* харагдацуудыг уншина: холболтын тоо, удаан query,
-- түгжээ, replication lag. Эдгээрийг уншихад PostgreSQL-ийн бэлэн `pg_monitor`
-- гэсэн эрх хангалттай бөгөөд энэ нь superuser биш, ямар ч хэрэглэгчийн
-- хүснэгтийн өгөгдөл уншихгүй. Exporter-ыг postgres superuser-ээр ажиллуулах
-- нь хамгийн түгээмэл алдаа: тэр үед мониторингийн контейнер эвдрэлд орох нь
-- өгөгдлийн сангийн бүрэн хандалт алдагдсантай тэнцэнэ.
--
-- Нууц үг энд байхгүй бөгөөд байх ч ёсгүй — миграц бол репод хадгалагддаг
-- файл. Role нь нууц үггүй үүсэх тул одоохондоо нэвтэрч чадахгүй; оператор
-- дараах тушаалаар нэг удаа өгнө (docs/OPERATIONS.md):
--
--   docker exec -i gerege_nexus_postgres psql -U postgres -d platform_db \
--     -c "ALTER ROLE monitoring WITH PASSWORD '<generated>'"
--
-- Тэр утгыг deploy/.env.monitoring дотор MONITORING_DB_PASSWORD болгон
-- бичнэ. Хоёр талдаа тохироогүй бол postgres_exporter нь "password
-- authentication failed" гэж лог бичиж, өөрөө дахин оролдоно — платформын
-- ажиллагаанд нөлөөгүй.
--
-- Энэ хүснэгт биш, role тул RLS бодлого хамаарахгүй. Тенант дамнасан эрсдэл
-- нь өөр: `monitoring` нь pg_monitor-оос өөр юу ч авахгүй тул мөр уншиж
-- чадахгүй, ялангуяа тенантын мөрийг.

-- +goose Up

-- +goose StatementBegin
DO $monitoring$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'monitoring') THEN
        CREATE ROLE monitoring LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT;
        RAISE NOTICE 'created role "monitoring" with no password; set one with ALTER ROLE before starting postgres_exporter';
    END IF;
END
$monitoring$;
-- +goose StatementEnd

GRANT pg_monitor TO monitoring;

-- Exporter нь холбогдохын тулд өгөгдлийн санд нэвтрэх эрхтэй байх ёстой.
-- Схемийн USAGE нь pg_stat_statements зэрэг extension байвал түүнийг уншихад
-- хэрэгтэй; хүснэгтийн SELECT эрх ЗОРИУДААР олгогдоогүй.
-- Өгөгдлийн сангийн нэрийг current_database()-ээс авна: production дээр
-- platform_db, тестийн орчинд өөр нэртэй байдаг ба хатуу бичсэн нэр нь
-- миграцыг тэнд унагаана.
-- +goose StatementBegin
DO $connect$
BEGIN
    EXECUTE format('GRANT CONNECT ON DATABASE %I TO monitoring', current_database());
END
$connect$;
-- +goose StatementEnd

GRANT USAGE ON SCHEMA public TO monitoring;

-- +goose Down

REVOKE USAGE ON SCHEMA public FROM monitoring;

-- +goose StatementBegin
DO $connect$
BEGIN
    EXECUTE format('REVOKE CONNECT ON DATABASE %I FROM monitoring', current_database());
END
$connect$;
-- +goose StatementEnd

REVOKE pg_monitor FROM monitoring;

-- Role нь кластерын өмч бөгөөд энэ хост дээр хөрш стекүүд ажилладаг.
-- Өөр өгөгдлийн сан түүнд ямар нэг эрх олгосон хэвээр бол DROP нь унаж,
-- буцаалтыг бүхэлд нь дагуулан унагана — 00029-ийн gerege_nexus_app-тай ижил
-- шалтгаанаар үлдээгээд, мэдэгдэл бичнэ.
-- +goose StatementBegin
DO $drop$
BEGIN
    DROP ROLE IF EXISTS monitoring;
EXCEPTION WHEN dependent_objects_still_exist OR insufficient_privilege THEN
    RAISE NOTICE 'left role monitoring in place: %', SQLERRM;
END
$drop$;
-- +goose StatementEnd
