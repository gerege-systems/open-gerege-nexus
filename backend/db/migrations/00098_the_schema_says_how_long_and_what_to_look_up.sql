-- Схем нь уртаа хэлж, эцгээ хайж олдог болов.
--
-- Хоёр зүйл, аль аль нь хүснэгт нэмэгдэх бүрт нэг мөр мартагдсанаас
-- хуримтлагдсан.
--
--
-- 1. ГАДААД ТҮЛХҮҮР БҮРД ИНДЕКС.
--
-- PostgreSQL нь гадаад түлхүүрийн эх мөрд индекс өөрөө үүсгэдэггүй. Эцэг мөр
-- устах эсвэл шинэчлэгдэх болгонд хүүхэд хүснэгт бүрийг бүтнээр нь уншиж,
-- лавлагааг хайдаг. Энэ платформ дээр тэр нь онолын зардал биш: байгууллага
-- устгах нь 30 хоногийн дараа бодитоор ажилладаг функц (`SweepDeletions`) ба
-- `tenants` мөр устахад дөчөөд хүүхэд хүснэгт cascade-аар дагадаг.
--
-- Хоёр `*_probe` хүснэгт орхигдсон: тэдгээр нь миграц ажилласан эсэхийг
-- шалгах хэдэн мөр бөгөөд индекс нь уншихаас нь илүү зардалтай.
--
--
-- 2. ТЕКСТ БАГАНА БҮР УРТТАЙ ТӨРӨЛТЭЙ.
--
-- Хязгааргүй `text` нь өгөгдлийн сангийн хувьд «хэдэн ч тэмдэгт байж болно»
-- гэсэн үг: нэг л шалгаагүй handler нэг мөрөнд мегабайт бичих боломжтой ба
-- тэр мөрийг уншдаг дэлгэц, экспорт, аудит бүр түүнийг үүрнэ.
--
-- Хязгаарыг `CHECK (length(x) <= n)` биш, **төрлөөр нь** тавьсан:
-- `varchar(n)` нь уртыг схемийн тодорхойлолтод бичдэг тул `\d`, information_schema,
-- ORM-гүй ч гэсэн клиентийн драйвер, баримт үүсгэгч бүр түүнийг хардаг.
-- CHECK нь мөн хамгаална, харин уншиж байж л мэдэгдэнэ.
--
-- Уртууд өгөөмөр бөгөөд агуулгаараа сонгогдсон: hash 128, PEM 8192, URL 2048,
-- и-мэйл 320 (RFC 5321), нэр/гарчиг 200, тайлбар/шалтгаан 4000, төлөв/төрөл
-- 16-32, AI-гийн мэдлэгийн эх 100000. Одоо байгаа мөр бүр хязгаартаа багтаж
-- байгааг миграц бичихийн өмнө өгөгдлийн сангаас асууж баталсан.
--
-- `text` → `varchar(n)` нь хоёртын хувьд нийцтэй хөрвүүлэлт тул хүснэгт дахин
-- бичигдэхгүй; PostgreSQL мөр бүрийн уртыг шалгах уншилт хийж, багана дээрх
-- индексүүдийг сэргээнэ. Тэр хугацаанд хүснэгт ACCESS EXCLUSIVE-ээр
-- түгжигдэнэ — энэ хэмжээний суулгацад миллисекундын асуудал, харин том
-- хүснэгттэй суулгац үүнийг ачаалал багатай цагт ажиллуулах нь зөв.

-- +goose Up

-- Гадаад түлхүүрийн индексүүд

CREATE INDEX IF NOT EXISTS idx_announcements_tenant ON registry.announcements (tenant_id);
CREATE INDEX IF NOT EXISTS idx_app_dependencies_app_version ON registry.app_dependencies (app_version_id);
CREATE INDEX IF NOT EXISTS idx_feature_flag_overrides_tenant ON registry.feature_flag_overrides (tenant_id);
CREATE INDEX IF NOT EXISTS idx_operator_impersonations_user ON registry.operator_impersonations (user_id);
CREATE INDEX IF NOT EXISTS idx_person_items_provider_tenant ON registry.person_items (provider_tenant_id);
CREATE INDEX IF NOT EXISTS idx_service_directory_published_by ON registry.service_directory (published_by);
CREATE INDEX IF NOT EXISTS idx_access_change_events_actor_user ON workspace.access_change_events (actor_user_id);
CREATE INDEX IF NOT EXISTS idx_app_installations_app ON workspace.app_installations (app_id);
CREATE INDEX IF NOT EXISTS idx_device_enrollment_codes_created_by ON workspace.device_enrollment_codes (created_by);
CREATE INDEX IF NOT EXISTS idx_device_enrollment_codes_tenant ON workspace.device_enrollment_codes (tenant_id);
CREATE INDEX IF NOT EXISTS idx_device_telemetry_device ON workspace.device_telemetry (device_id);
CREATE INDEX IF NOT EXISTS idx_esign_batch_items_session ON workspace.esign_batch_items (session_id);
CREATE INDEX IF NOT EXISTS idx_esign_batch_items_document ON workspace.esign_batch_items (document_id);
CREATE INDEX IF NOT EXISTS idx_esign_batches_created_by ON workspace.esign_batches (created_by);
CREATE INDEX IF NOT EXISTS idx_esign_documents_uploaded_by ON workspace.esign_documents (uploaded_by);
CREATE INDEX IF NOT EXISTS idx_esign_settings_updated_by ON workspace.esign_settings (updated_by);
CREATE INDEX IF NOT EXISTS idx_esign_sign_sessions_document ON workspace.esign_sign_sessions (document_id);
CREATE INDEX IF NOT EXISTS idx_esign_sign_sessions_signer_user ON workspace.esign_sign_sessions (signer_user_id);
CREATE INDEX IF NOT EXISTS idx_esign_signature_logs_actor_user ON workspace.esign_signature_logs (actor_user_id);
CREATE INDEX IF NOT EXISTS idx_join_requests_decided_by ON workspace.join_requests (decided_by);
CREATE INDEX IF NOT EXISTS idx_join_requests_user ON workspace.join_requests (user_id);
CREATE INDEX IF NOT EXISTS idx_membership_roles_role ON workspace.membership_roles (role_id);
CREATE INDEX IF NOT EXISTS idx_oauth2_access_tokens_client ON workspace.oauth2_access_tokens (client_id);
CREATE INDEX IF NOT EXISTS idx_oauth2_authorization_codes_user ON workspace.oauth2_authorization_codes (user_id);
CREATE INDEX IF NOT EXISTS idx_oauth2_authorization_codes_client ON workspace.oauth2_authorization_codes (client_id);
CREATE INDEX IF NOT EXISTS idx_oauth2_authorization_codes_tenant ON workspace.oauth2_authorization_codes (tenant_id);
CREATE INDEX IF NOT EXISTS idx_oauth2_clients_created_by ON workspace.oauth2_clients (created_by);
CREATE INDEX IF NOT EXISTS idx_oauth2_consents_tenant ON workspace.oauth2_consents (tenant_id);
CREATE INDEX IF NOT EXISTS idx_oauth2_consents_client ON workspace.oauth2_consents (client_id);
CREATE INDEX IF NOT EXISTS idx_oauth2_tokens_parent ON workspace.oauth2_tokens (parent_id);
CREATE INDEX IF NOT EXISTS idx_oauth2_tokens_tenant ON workspace.oauth2_tokens (tenant_id);
CREATE INDEX IF NOT EXISTS idx_oauth2_tokens_user ON workspace.oauth2_tokens (user_id);
CREATE INDEX IF NOT EXISTS idx_push_tokens_device ON workspace.push_tokens (device_id);
CREATE INDEX IF NOT EXISTS idx_push_tokens_tenant ON workspace.push_tokens (tenant_id);
CREATE INDEX IF NOT EXISTS idx_push_tokens_user ON workspace.push_tokens (user_id);
CREATE INDEX IF NOT EXISTS idx_report_grants_created_by ON workspace.report_grants (created_by);
CREATE INDEX IF NOT EXISTS idx_report_grants_accepted_by ON workspace.report_grants (accepted_by);
CREATE INDEX IF NOT EXISTS idx_report_schedules_created_by ON workspace.report_schedules (created_by);
CREATE INDEX IF NOT EXISTS idx_role_permissions_permission ON workspace.role_permissions (permission_id);
CREATE INDEX IF NOT EXISTS idx_sessions_tenant ON workspace.sessions (tenant_id);
CREATE INDEX IF NOT EXISTS idx_staff_pin_credentials_tenant ON workspace.staff_pin_credentials (tenant_id);

-- Текст баганы урт нь төрөлдөө

ALTER TABLE registry.announcements ALTER COLUMN body TYPE varchar(20000);
ALTER TABLE registry.announcements ALTER COLUMN kind TYPE varchar(32);
ALTER TABLE registry.announcements ALTER COLUMN title TYPE varchar(200);

ALTER TABLE registry.app_versions ALTER COLUMN package_sha256 TYPE varchar(128);
ALTER TABLE registry.app_versions ALTER COLUMN package_url TYPE varchar(2048);

ALTER TABLE registry.apps ALTER COLUMN description TYPE varchar(4000);
ALTER TABLE registry.apps ALTER COLUMN icon_url TYPE varchar(2048);

ALTER TABLE registry.credential_grants ALTER COLUMN purpose TYPE varchar(200);

ALTER TABLE registry.eid_sign_state ALTER COLUMN key TYPE varchar(200);
ALTER TABLE registry.eid_sign_state ALTER COLUMN value TYPE varchar(8192);

ALTER TABLE registry.feature_flag_overrides ALTER COLUMN flag_key TYPE varchar(200);

ALTER TABLE registry.feature_flags ALTER COLUMN description TYPE varchar(4000);
ALTER TABLE registry.feature_flags ALTER COLUMN key TYPE varchar(200);
ALTER TABLE registry.feature_flags ALTER COLUMN kind TYPE varchar(32);
ALTER TABLE registry.feature_flags ALTER COLUMN owner TYPE varchar(200);

ALTER TABLE registry.identity_binding_sessions ALTER COLUMN email TYPE varchar(320);
ALTER TABLE registry.identity_binding_sessions ALTER COLUMN issuer TYPE varchar(255);
ALTER TABLE registry.identity_binding_sessions ALTER COLUMN name TYPE varchar(200);
ALTER TABLE registry.identity_binding_sessions ALTER COLUMN subject TYPE varchar(255);

ALTER TABLE registry.oauth2_signing_keys ALTER COLUMN private_key_pem TYPE varchar(8192);
ALTER TABLE registry.oauth2_signing_keys ALTER COLUMN public_key_pem TYPE varchar(8192);

ALTER TABLE registry.operator_impersonations ALTER COLUMN operator_email TYPE varchar(320);
ALTER TABLE registry.operator_impersonations ALTER COLUMN reason TYPE varchar(4000);

ALTER TABLE registry.permissions ALTER COLUMN description TYPE varchar(4000);

ALTER TABLE registry.person_items ALTER COLUMN code TYPE varchar(200);
ALTER TABLE registry.person_items ALTER COLUMN source_app TYPE varchar(200);
ALTER TABLE registry.person_items ALTER COLUMN source_ref TYPE varchar(200);
ALTER TABLE registry.person_items ALTER COLUMN status TYPE varchar(32);

ALTER TABLE registry.platform_settings ALTER COLUMN key TYPE varchar(200);
ALTER TABLE registry.platform_settings ALTER COLUMN value TYPE varchar(8192);

ALTER TABLE registry.service_directory ALTER COLUMN title TYPE varchar(200);

ALTER TABLE registry.store_app_versions ALTER COLUMN package_sha256 TYPE varchar(128);
ALTER TABLE registry.store_app_versions ALTER COLUMN package_url TYPE varchar(2048);
ALTER TABLE registry.store_app_versions ALTER COLUMN review_note TYPE varchar(4000);

ALTER TABLE registry.tenant_quotas ALTER COLUMN enforcement TYPE varchar(32);

ALTER TABLE registry.tenants ALTER COLUMN kind TYPE varchar(32);
ALTER TABLE registry.tenants ALTER COLUMN maintenance_message TYPE varchar(200);
ALTER TABLE registry.tenants ALTER COLUMN suspension_reason TYPE varchar(200);

ALTER TABLE registry.usage_events ALTER COLUMN metric TYPE varchar(100);

ALTER TABLE registry.user_sso_identities ALTER COLUMN email TYPE varchar(320);
ALTER TABLE registry.user_sso_identities ALTER COLUMN issuer TYPE varchar(255);
ALTER TABLE registry.user_sso_identities ALTER COLUMN name TYPE varchar(200);
ALTER TABLE registry.user_sso_identities ALTER COLUMN subject TYPE varchar(255);

ALTER TABLE workspace.ai_knowledge ALTER COLUMN content TYPE varchar(100000);
ALTER TABLE workspace.ai_knowledge ALTER COLUMN source_url TYPE varchar(2048);

ALTER TABLE workspace.ai_prompts ALTER COLUMN content TYPE varchar(20000);

ALTER TABLE workspace.audit_events ALTER COLUMN action TYPE varchar(100);
ALTER TABLE workspace.audit_events ALTER COLUMN resource TYPE varchar(200);
ALTER TABLE workspace.audit_events ALTER COLUMN user_id TYPE varchar(64);

ALTER TABLE workspace.dbguard_probe ALTER COLUMN name TYPE varchar(200);

ALTER TABLE workspace.default_app_probe ALTER COLUMN code TYPE varchar(200);
ALTER TABLE workspace.default_app_probe ALTER COLUMN name TYPE varchar(200);

ALTER TABLE workspace.device_enrollment_codes ALTER COLUMN code_hash TYPE varchar(128);

ALTER TABLE workspace.device_telemetry ALTER COLUMN event TYPE varchar(100);
ALTER TABLE workspace.device_telemetry ALTER COLUMN level TYPE varchar(16);

ALTER TABLE workspace.devices ALTER COLUMN app_version TYPE varchar(64);
ALTER TABLE workspace.devices ALTER COLUMN form_factor TYPE varchar(32);
ALTER TABLE workspace.devices ALTER COLUMN name TYPE varchar(200);
ALTER TABLE workspace.devices ALTER COLUMN os_version TYPE varchar(64);
ALTER TABLE workspace.devices ALTER COLUMN platform TYPE varchar(32);
ALTER TABLE workspace.devices ALTER COLUMN site TYPE varchar(200);
ALTER TABLE workspace.devices ALTER COLUMN status TYPE varchar(32);
ALTER TABLE workspace.devices ALTER COLUMN token_hash TYPE varchar(128);

ALTER TABLE workspace.email_verifications ALTER COLUMN redirect_url TYPE varchar(2048);

ALTER TABLE workspace.esign_batch_items ALTER COLUMN error TYPE varchar(4000);

ALTER TABLE workspace.esign_sign_sessions ALTER COLUMN eid_session_id TYPE varchar(128);

ALTER TABLE workspace.esign_signature_logs ALTER COLUMN detail TYPE varchar(4000);

ALTER TABLE workspace.join_requests ALTER COLUMN status TYPE varchar(32);

ALTER TABLE workspace.oauth2_access_tokens ALTER COLUMN scope TYPE varchar(1000);

ALTER TABLE workspace.oauth2_authorization_codes ALTER COLUMN nonce TYPE varchar(512);
ALTER TABLE workspace.oauth2_authorization_codes ALTER COLUMN redirect_uri TYPE varchar(2048);

ALTER TABLE workspace.oauth2_clients ALTER COLUMN client_uri TYPE varchar(2048);
ALTER TABLE workspace.oauth2_clients ALTER COLUMN logo_uri TYPE varchar(2048);

ALTER TABLE workspace.push_tokens ALTER COLUMN app_id TYPE varchar(128);
ALTER TABLE workspace.push_tokens ALTER COLUMN provider TYPE varchar(32);
ALTER TABLE workspace.push_tokens ALTER COLUMN token_ciphertext TYPE varchar(4096);
ALTER TABLE workspace.push_tokens ALTER COLUMN token_hash TYPE varchar(128);

ALTER TABLE workspace.report_grants ALTER COLUMN counterparty_ref TYPE varchar(200);
ALTER TABLE workspace.report_grants ALTER COLUMN note TYPE varchar(4000);
ALTER TABLE workspace.report_grants ALTER COLUMN report_key TYPE varchar(200);
ALTER TABLE workspace.report_grants ALTER COLUMN scope TYPE varchar(1000);

ALTER TABLE workspace.report_schedules ALTER COLUMN cron TYPE varchar(100);
ALTER TABLE workspace.report_schedules ALTER COLUMN format TYPE varchar(16);
ALTER TABLE workspace.report_schedules ALTER COLUMN last_error TYPE varchar(4000);
ALTER TABLE workspace.report_schedules ALTER COLUMN last_status TYPE varchar(32);
ALTER TABLE workspace.report_schedules ALTER COLUMN name TYPE varchar(200);
ALTER TABLE workspace.report_schedules ALTER COLUMN report_key TYPE varchar(200);

ALTER TABLE workspace.reporting_probe ALTER COLUMN contact_name TYPE varchar(200);

ALTER TABLE workspace.roles ALTER COLUMN description TYPE varchar(4000);

ALTER TABLE workspace.sessions ALTER COLUMN user_agent TYPE varchar(512);

ALTER TABLE workspace.staff_pin_credentials ALTER COLUMN pin_hash TYPE varchar(128);

ALTER TABLE workspace.tenant_profiles ALTER COLUMN logo_url TYPE varchar(2048);

-- Гурван багана аль хэдийн `CHECK (length(...) <= n)`-тэй байсан (00086, 00089,
-- 00090). Тэдгээрийн хязгаарыг өөрчилсөнгүй, зөвхөн тэр тоог төрөлд нь давхар
-- бичив: CHECK нь хэвээр — `service_directory.code`-ынх урт төдийгүй хоосон
-- биш байхыг шаарддаг — харин «энэ багана хэр урт вэ» гэсэн асуултын хариу
-- одоо схемийн тодорхойлолт дээр өөр дээр нь байна.
ALTER TABLE registry.person_items ALTER COLUMN answer TYPE varchar(2000);
ALTER TABLE registry.service_directory ALTER COLUMN code TYPE varchar(128);
ALTER TABLE workspace.join_requests ALTER COLUMN message TYPE varchar(500);

-- +goose Down

ALTER TABLE registry.person_items ALTER COLUMN answer TYPE text;
ALTER TABLE registry.service_directory ALTER COLUMN code TYPE text;
ALTER TABLE workspace.join_requests ALTER COLUMN message TYPE text;


ALTER TABLE registry.announcements ALTER COLUMN body TYPE text;
ALTER TABLE registry.announcements ALTER COLUMN kind TYPE text;
ALTER TABLE registry.announcements ALTER COLUMN title TYPE text;

ALTER TABLE registry.app_versions ALTER COLUMN package_sha256 TYPE text;
ALTER TABLE registry.app_versions ALTER COLUMN package_url TYPE text;

ALTER TABLE registry.apps ALTER COLUMN description TYPE text;
ALTER TABLE registry.apps ALTER COLUMN icon_url TYPE text;

ALTER TABLE registry.credential_grants ALTER COLUMN purpose TYPE text;

ALTER TABLE registry.eid_sign_state ALTER COLUMN key TYPE text;
ALTER TABLE registry.eid_sign_state ALTER COLUMN value TYPE text;

ALTER TABLE registry.feature_flag_overrides ALTER COLUMN flag_key TYPE text;

ALTER TABLE registry.feature_flags ALTER COLUMN description TYPE text;
ALTER TABLE registry.feature_flags ALTER COLUMN key TYPE text;
ALTER TABLE registry.feature_flags ALTER COLUMN kind TYPE text;
ALTER TABLE registry.feature_flags ALTER COLUMN owner TYPE text;

ALTER TABLE registry.identity_binding_sessions ALTER COLUMN email TYPE text;
ALTER TABLE registry.identity_binding_sessions ALTER COLUMN issuer TYPE text;
ALTER TABLE registry.identity_binding_sessions ALTER COLUMN name TYPE text;
ALTER TABLE registry.identity_binding_sessions ALTER COLUMN subject TYPE text;

ALTER TABLE registry.oauth2_signing_keys ALTER COLUMN private_key_pem TYPE text;
ALTER TABLE registry.oauth2_signing_keys ALTER COLUMN public_key_pem TYPE text;

ALTER TABLE registry.operator_impersonations ALTER COLUMN operator_email TYPE text;
ALTER TABLE registry.operator_impersonations ALTER COLUMN reason TYPE text;

ALTER TABLE registry.permissions ALTER COLUMN description TYPE text;

ALTER TABLE registry.person_items ALTER COLUMN code TYPE text;
ALTER TABLE registry.person_items ALTER COLUMN source_app TYPE text;
ALTER TABLE registry.person_items ALTER COLUMN source_ref TYPE text;
ALTER TABLE registry.person_items ALTER COLUMN status TYPE text;

ALTER TABLE registry.platform_settings ALTER COLUMN key TYPE text;
ALTER TABLE registry.platform_settings ALTER COLUMN value TYPE text;

ALTER TABLE registry.service_directory ALTER COLUMN title TYPE text;

ALTER TABLE registry.store_app_versions ALTER COLUMN package_sha256 TYPE text;
ALTER TABLE registry.store_app_versions ALTER COLUMN package_url TYPE text;
ALTER TABLE registry.store_app_versions ALTER COLUMN review_note TYPE text;

ALTER TABLE registry.tenant_quotas ALTER COLUMN enforcement TYPE text;

ALTER TABLE registry.tenants ALTER COLUMN kind TYPE text;
ALTER TABLE registry.tenants ALTER COLUMN maintenance_message TYPE text;
ALTER TABLE registry.tenants ALTER COLUMN suspension_reason TYPE text;

ALTER TABLE registry.usage_events ALTER COLUMN metric TYPE text;

ALTER TABLE registry.user_sso_identities ALTER COLUMN email TYPE text;
ALTER TABLE registry.user_sso_identities ALTER COLUMN issuer TYPE text;
ALTER TABLE registry.user_sso_identities ALTER COLUMN name TYPE text;
ALTER TABLE registry.user_sso_identities ALTER COLUMN subject TYPE text;

ALTER TABLE workspace.ai_knowledge ALTER COLUMN content TYPE text;
ALTER TABLE workspace.ai_knowledge ALTER COLUMN source_url TYPE text;

ALTER TABLE workspace.ai_prompts ALTER COLUMN content TYPE text;

ALTER TABLE workspace.audit_events ALTER COLUMN action TYPE text;
ALTER TABLE workspace.audit_events ALTER COLUMN resource TYPE text;
ALTER TABLE workspace.audit_events ALTER COLUMN user_id TYPE text;

ALTER TABLE workspace.dbguard_probe ALTER COLUMN name TYPE text;

ALTER TABLE workspace.default_app_probe ALTER COLUMN code TYPE text;
ALTER TABLE workspace.default_app_probe ALTER COLUMN name TYPE text;

ALTER TABLE workspace.device_enrollment_codes ALTER COLUMN code_hash TYPE text;

ALTER TABLE workspace.device_telemetry ALTER COLUMN event TYPE text;
ALTER TABLE workspace.device_telemetry ALTER COLUMN level TYPE text;

ALTER TABLE workspace.devices ALTER COLUMN app_version TYPE text;
ALTER TABLE workspace.devices ALTER COLUMN form_factor TYPE text;
ALTER TABLE workspace.devices ALTER COLUMN name TYPE text;
ALTER TABLE workspace.devices ALTER COLUMN os_version TYPE text;
ALTER TABLE workspace.devices ALTER COLUMN platform TYPE text;
ALTER TABLE workspace.devices ALTER COLUMN site TYPE text;
ALTER TABLE workspace.devices ALTER COLUMN status TYPE text;
ALTER TABLE workspace.devices ALTER COLUMN token_hash TYPE text;

ALTER TABLE workspace.email_verifications ALTER COLUMN redirect_url TYPE text;

ALTER TABLE workspace.esign_batch_items ALTER COLUMN error TYPE text;

ALTER TABLE workspace.esign_sign_sessions ALTER COLUMN eid_session_id TYPE text;

ALTER TABLE workspace.esign_signature_logs ALTER COLUMN detail TYPE text;

ALTER TABLE workspace.join_requests ALTER COLUMN status TYPE text;

ALTER TABLE workspace.oauth2_access_tokens ALTER COLUMN scope TYPE text;

ALTER TABLE workspace.oauth2_authorization_codes ALTER COLUMN nonce TYPE text;
ALTER TABLE workspace.oauth2_authorization_codes ALTER COLUMN redirect_uri TYPE text;

ALTER TABLE workspace.oauth2_clients ALTER COLUMN client_uri TYPE text;
ALTER TABLE workspace.oauth2_clients ALTER COLUMN logo_uri TYPE text;

ALTER TABLE workspace.push_tokens ALTER COLUMN app_id TYPE text;
ALTER TABLE workspace.push_tokens ALTER COLUMN provider TYPE text;
ALTER TABLE workspace.push_tokens ALTER COLUMN token_ciphertext TYPE text;
ALTER TABLE workspace.push_tokens ALTER COLUMN token_hash TYPE text;

ALTER TABLE workspace.report_grants ALTER COLUMN counterparty_ref TYPE text;
ALTER TABLE workspace.report_grants ALTER COLUMN note TYPE text;
ALTER TABLE workspace.report_grants ALTER COLUMN report_key TYPE text;
ALTER TABLE workspace.report_grants ALTER COLUMN scope TYPE text;

ALTER TABLE workspace.report_schedules ALTER COLUMN cron TYPE text;
ALTER TABLE workspace.report_schedules ALTER COLUMN format TYPE text;
ALTER TABLE workspace.report_schedules ALTER COLUMN last_error TYPE text;
ALTER TABLE workspace.report_schedules ALTER COLUMN last_status TYPE text;
ALTER TABLE workspace.report_schedules ALTER COLUMN name TYPE text;
ALTER TABLE workspace.report_schedules ALTER COLUMN report_key TYPE text;

ALTER TABLE workspace.reporting_probe ALTER COLUMN contact_name TYPE text;

ALTER TABLE workspace.roles ALTER COLUMN description TYPE text;

ALTER TABLE workspace.sessions ALTER COLUMN user_agent TYPE text;

ALTER TABLE workspace.staff_pin_credentials ALTER COLUMN pin_hash TYPE text;

ALTER TABLE workspace.tenant_profiles ALTER COLUMN logo_url TYPE text;

DROP INDEX IF EXISTS registry.idx_announcements_tenant;
DROP INDEX IF EXISTS registry.idx_app_dependencies_app_version;
DROP INDEX IF EXISTS registry.idx_feature_flag_overrides_tenant;
DROP INDEX IF EXISTS registry.idx_operator_impersonations_user;
DROP INDEX IF EXISTS registry.idx_person_items_provider_tenant;
DROP INDEX IF EXISTS registry.idx_service_directory_published_by;
DROP INDEX IF EXISTS workspace.idx_access_change_events_actor_user;
DROP INDEX IF EXISTS workspace.idx_app_installations_app;
DROP INDEX IF EXISTS workspace.idx_device_enrollment_codes_created_by;
DROP INDEX IF EXISTS workspace.idx_device_enrollment_codes_tenant;
DROP INDEX IF EXISTS workspace.idx_device_telemetry_device;
DROP INDEX IF EXISTS workspace.idx_esign_batch_items_session;
DROP INDEX IF EXISTS workspace.idx_esign_batch_items_document;
DROP INDEX IF EXISTS workspace.idx_esign_batches_created_by;
DROP INDEX IF EXISTS workspace.idx_esign_documents_uploaded_by;
DROP INDEX IF EXISTS workspace.idx_esign_settings_updated_by;
DROP INDEX IF EXISTS workspace.idx_esign_sign_sessions_document;
DROP INDEX IF EXISTS workspace.idx_esign_sign_sessions_signer_user;
DROP INDEX IF EXISTS workspace.idx_esign_signature_logs_actor_user;
DROP INDEX IF EXISTS workspace.idx_join_requests_decided_by;
DROP INDEX IF EXISTS workspace.idx_join_requests_user;
DROP INDEX IF EXISTS workspace.idx_membership_roles_role;
DROP INDEX IF EXISTS workspace.idx_oauth2_access_tokens_client;
DROP INDEX IF EXISTS workspace.idx_oauth2_authorization_codes_user;
DROP INDEX IF EXISTS workspace.idx_oauth2_authorization_codes_client;
DROP INDEX IF EXISTS workspace.idx_oauth2_authorization_codes_tenant;
DROP INDEX IF EXISTS workspace.idx_oauth2_clients_created_by;
DROP INDEX IF EXISTS workspace.idx_oauth2_consents_tenant;
DROP INDEX IF EXISTS workspace.idx_oauth2_consents_client;
DROP INDEX IF EXISTS workspace.idx_oauth2_tokens_parent;
DROP INDEX IF EXISTS workspace.idx_oauth2_tokens_tenant;
DROP INDEX IF EXISTS workspace.idx_oauth2_tokens_user;
DROP INDEX IF EXISTS workspace.idx_push_tokens_device;
DROP INDEX IF EXISTS workspace.idx_push_tokens_tenant;
DROP INDEX IF EXISTS workspace.idx_push_tokens_user;
DROP INDEX IF EXISTS workspace.idx_report_grants_created_by;
DROP INDEX IF EXISTS workspace.idx_report_grants_accepted_by;
DROP INDEX IF EXISTS workspace.idx_report_schedules_created_by;
DROP INDEX IF EXISTS workspace.idx_role_permissions_permission;
DROP INDEX IF EXISTS workspace.idx_sessions_tenant;
DROP INDEX IF EXISTS workspace.idx_staff_pin_credentials_tenant;
