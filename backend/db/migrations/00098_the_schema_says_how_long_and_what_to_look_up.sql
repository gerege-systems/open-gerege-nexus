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
-- 2. ТЕКСТ БАГАНА БҮР УРТТАЙ.
--
-- Хязгааргүй `text` нь өгөгдлийн сангийн хувьд «хэдэн ч байж болно» гэсэн үг:
-- нэг л шалгаагүй handler нэг мөрөнд мегабайт бичих боломжтой, ба тэр мөрийг
-- уншдаг дэлгэц, экспорт, аудит бүр түүнийг үүрнэ. Хязгаар нь баталгаажуулалт
-- биш — Go тал нь хэвээр шалгана — харин хамгийн сүүлийн хана.
--
-- Хязгаарууд нь өгөөмөр бөгөөд агуулгаараа сонгогдсон: hash 128, PEM 8192,
-- URL 2048, и-мэйл 320 (RFC 5321), нэр/гарчиг 200, тайлбар/шалтгаан 4000,
-- төлөв/төрөл 16-32, AI-гийн мэдлэгийн эх 100000. Одоо байгаа мөр бүр
-- хязгаартаа багтаж байгааг миграц бичихийн өмнө өгөгдлийн сангаас асууж
-- баталсан.
--
-- Нэршил нь 00089, 00090-ийн загвараар: `<хүснэгт>_<багана>_is_bounded`.

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

-- Текст баганы уртын хязгаар

ALTER TABLE registry.announcements ADD CONSTRAINT announcements_body_is_bounded CHECK (length(body) <= 20000) NOT VALID;
ALTER TABLE registry.announcements ADD CONSTRAINT announcements_kind_is_bounded CHECK (length(kind) <= 32) NOT VALID;
ALTER TABLE registry.announcements ADD CONSTRAINT announcements_title_is_bounded CHECK (length(title) <= 200) NOT VALID;

ALTER TABLE registry.app_versions ADD CONSTRAINT app_versions_package_sha256_is_bounded CHECK (length(package_sha256) <= 128) NOT VALID;
ALTER TABLE registry.app_versions ADD CONSTRAINT app_versions_package_url_is_bounded CHECK (length(package_url) <= 2048) NOT VALID;

ALTER TABLE registry.apps ADD CONSTRAINT apps_description_is_bounded CHECK (length(description) <= 4000) NOT VALID;
ALTER TABLE registry.apps ADD CONSTRAINT apps_icon_url_is_bounded CHECK (length(icon_url) <= 2048) NOT VALID;

ALTER TABLE registry.credential_grants ADD CONSTRAINT credential_grants_purpose_is_bounded CHECK (length(purpose) <= 200) NOT VALID;

ALTER TABLE registry.eid_sign_state ADD CONSTRAINT eid_sign_state_key_is_bounded CHECK (length(key) <= 200) NOT VALID;
ALTER TABLE registry.eid_sign_state ADD CONSTRAINT eid_sign_state_value_is_bounded CHECK (length(value) <= 8192) NOT VALID;

ALTER TABLE registry.feature_flag_overrides ADD CONSTRAINT feature_flag_overrides_flag_key_is_bounded CHECK (length(flag_key) <= 200) NOT VALID;

ALTER TABLE registry.feature_flags ADD CONSTRAINT feature_flags_description_is_bounded CHECK (length(description) <= 4000) NOT VALID;
ALTER TABLE registry.feature_flags ADD CONSTRAINT feature_flags_key_is_bounded CHECK (length(key) <= 200) NOT VALID;
ALTER TABLE registry.feature_flags ADD CONSTRAINT feature_flags_kind_is_bounded CHECK (length(kind) <= 32) NOT VALID;
ALTER TABLE registry.feature_flags ADD CONSTRAINT feature_flags_owner_is_bounded CHECK (length(owner) <= 200) NOT VALID;

ALTER TABLE registry.identity_binding_sessions ADD CONSTRAINT identity_binding_sessions_email_is_bounded CHECK (length(email) <= 320) NOT VALID;
ALTER TABLE registry.identity_binding_sessions ADD CONSTRAINT identity_binding_sessions_issuer_is_bounded CHECK (length(issuer) <= 255) NOT VALID;
ALTER TABLE registry.identity_binding_sessions ADD CONSTRAINT identity_binding_sessions_name_is_bounded CHECK (length(name) <= 200) NOT VALID;
ALTER TABLE registry.identity_binding_sessions ADD CONSTRAINT identity_binding_sessions_subject_is_bounded CHECK (length(subject) <= 255) NOT VALID;

ALTER TABLE registry.oauth2_signing_keys ADD CONSTRAINT oauth2_signing_keys_private_key_pem_is_bounded CHECK (length(private_key_pem) <= 8192) NOT VALID;
ALTER TABLE registry.oauth2_signing_keys ADD CONSTRAINT oauth2_signing_keys_public_key_pem_is_bounded CHECK (length(public_key_pem) <= 8192) NOT VALID;

ALTER TABLE registry.operator_impersonations ADD CONSTRAINT operator_impersonations_operator_email_is_bounded CHECK (length(operator_email) <= 320) NOT VALID;
ALTER TABLE registry.operator_impersonations ADD CONSTRAINT operator_impersonations_reason_is_bounded CHECK (length(reason) <= 4000) NOT VALID;

ALTER TABLE registry.permissions ADD CONSTRAINT permissions_description_is_bounded CHECK (length(description) <= 4000) NOT VALID;

ALTER TABLE registry.person_items ADD CONSTRAINT person_items_code_is_bounded CHECK (length(code) <= 200) NOT VALID;
ALTER TABLE registry.person_items ADD CONSTRAINT person_items_source_app_is_bounded CHECK (length(source_app) <= 200) NOT VALID;
ALTER TABLE registry.person_items ADD CONSTRAINT person_items_source_ref_is_bounded CHECK (length(source_ref) <= 200) NOT VALID;
ALTER TABLE registry.person_items ADD CONSTRAINT person_items_status_is_bounded CHECK (length(status) <= 32) NOT VALID;

ALTER TABLE registry.platform_settings ADD CONSTRAINT platform_settings_key_is_bounded CHECK (length(key) <= 200) NOT VALID;
ALTER TABLE registry.platform_settings ADD CONSTRAINT platform_settings_value_is_bounded CHECK (length(value) <= 8192) NOT VALID;

ALTER TABLE registry.service_directory ADD CONSTRAINT service_directory_title_is_bounded CHECK (length(title) <= 200) NOT VALID;

ALTER TABLE registry.store_app_versions ADD CONSTRAINT store_app_versions_package_sha256_is_bounded CHECK (length(package_sha256) <= 128) NOT VALID;
ALTER TABLE registry.store_app_versions ADD CONSTRAINT store_app_versions_package_url_is_bounded CHECK (length(package_url) <= 2048) NOT VALID;
ALTER TABLE registry.store_app_versions ADD CONSTRAINT store_app_versions_review_note_is_bounded CHECK (length(review_note) <= 4000) NOT VALID;

ALTER TABLE registry.tenant_quotas ADD CONSTRAINT tenant_quotas_enforcement_is_bounded CHECK (length(enforcement) <= 32) NOT VALID;

ALTER TABLE registry.tenants ADD CONSTRAINT tenants_kind_is_bounded CHECK (length(kind) <= 32) NOT VALID;
ALTER TABLE registry.tenants ADD CONSTRAINT tenants_maintenance_message_is_bounded CHECK (length(maintenance_message) <= 200) NOT VALID;
ALTER TABLE registry.tenants ADD CONSTRAINT tenants_suspension_reason_is_bounded CHECK (length(suspension_reason) <= 200) NOT VALID;

ALTER TABLE registry.usage_events ADD CONSTRAINT usage_events_metric_is_bounded CHECK (length(metric) <= 100) NOT VALID;

ALTER TABLE registry.user_sso_identities ADD CONSTRAINT user_sso_identities_email_is_bounded CHECK (length(email) <= 320) NOT VALID;
ALTER TABLE registry.user_sso_identities ADD CONSTRAINT user_sso_identities_issuer_is_bounded CHECK (length(issuer) <= 255) NOT VALID;
ALTER TABLE registry.user_sso_identities ADD CONSTRAINT user_sso_identities_name_is_bounded CHECK (length(name) <= 200) NOT VALID;
ALTER TABLE registry.user_sso_identities ADD CONSTRAINT user_sso_identities_subject_is_bounded CHECK (length(subject) <= 255) NOT VALID;

ALTER TABLE workspace.ai_knowledge ADD CONSTRAINT ai_knowledge_content_is_bounded CHECK (length(content) <= 100000) NOT VALID;
ALTER TABLE workspace.ai_knowledge ADD CONSTRAINT ai_knowledge_source_url_is_bounded CHECK (length(source_url) <= 2048) NOT VALID;

ALTER TABLE workspace.ai_prompts ADD CONSTRAINT ai_prompts_content_is_bounded CHECK (length(content) <= 20000) NOT VALID;

ALTER TABLE workspace.audit_events ADD CONSTRAINT audit_events_action_is_bounded CHECK (length(action) <= 100) NOT VALID;
ALTER TABLE workspace.audit_events ADD CONSTRAINT audit_events_resource_is_bounded CHECK (length(resource) <= 200) NOT VALID;
ALTER TABLE workspace.audit_events ADD CONSTRAINT audit_events_user_id_is_bounded CHECK (length(user_id) <= 64) NOT VALID;

ALTER TABLE workspace.dbguard_probe ADD CONSTRAINT dbguard_probe_name_is_bounded CHECK (length(name) <= 200) NOT VALID;

ALTER TABLE workspace.default_app_probe ADD CONSTRAINT default_app_probe_code_is_bounded CHECK (length(code) <= 200) NOT VALID;
ALTER TABLE workspace.default_app_probe ADD CONSTRAINT default_app_probe_name_is_bounded CHECK (length(name) <= 200) NOT VALID;

ALTER TABLE workspace.device_enrollment_codes ADD CONSTRAINT device_enrollment_codes_code_hash_is_bounded CHECK (length(code_hash) <= 128) NOT VALID;

ALTER TABLE workspace.device_telemetry ADD CONSTRAINT device_telemetry_event_is_bounded CHECK (length(event) <= 100) NOT VALID;
ALTER TABLE workspace.device_telemetry ADD CONSTRAINT device_telemetry_level_is_bounded CHECK (length(level) <= 16) NOT VALID;

ALTER TABLE workspace.devices ADD CONSTRAINT devices_app_version_is_bounded CHECK (length(app_version) <= 64) NOT VALID;
ALTER TABLE workspace.devices ADD CONSTRAINT devices_form_factor_is_bounded CHECK (length(form_factor) <= 32) NOT VALID;
ALTER TABLE workspace.devices ADD CONSTRAINT devices_name_is_bounded CHECK (length(name) <= 200) NOT VALID;
ALTER TABLE workspace.devices ADD CONSTRAINT devices_os_version_is_bounded CHECK (length(os_version) <= 64) NOT VALID;
ALTER TABLE workspace.devices ADD CONSTRAINT devices_platform_is_bounded CHECK (length(platform) <= 32) NOT VALID;
ALTER TABLE workspace.devices ADD CONSTRAINT devices_site_is_bounded CHECK (length(site) <= 200) NOT VALID;
ALTER TABLE workspace.devices ADD CONSTRAINT devices_status_is_bounded CHECK (length(status) <= 32) NOT VALID;
ALTER TABLE workspace.devices ADD CONSTRAINT devices_token_hash_is_bounded CHECK (length(token_hash) <= 128) NOT VALID;

ALTER TABLE workspace.email_verifications ADD CONSTRAINT email_verifications_redirect_url_is_bounded CHECK (length(redirect_url) <= 2048) NOT VALID;

ALTER TABLE workspace.esign_batch_items ADD CONSTRAINT esign_batch_items_error_is_bounded CHECK (length(error) <= 4000) NOT VALID;

ALTER TABLE workspace.esign_sign_sessions ADD CONSTRAINT esign_sign_sessions_eid_session_id_is_bounded CHECK (length(eid_session_id) <= 128) NOT VALID;

ALTER TABLE workspace.esign_signature_logs ADD CONSTRAINT esign_signature_logs_detail_is_bounded CHECK (length(detail) <= 4000) NOT VALID;

ALTER TABLE workspace.join_requests ADD CONSTRAINT join_requests_status_is_bounded CHECK (length(status) <= 32) NOT VALID;

ALTER TABLE workspace.oauth2_access_tokens ADD CONSTRAINT oauth2_access_tokens_scope_is_bounded CHECK (length(scope) <= 1000) NOT VALID;

ALTER TABLE workspace.oauth2_authorization_codes ADD CONSTRAINT oauth2_authorization_codes_nonce_is_bounded CHECK (length(nonce) <= 512) NOT VALID;
ALTER TABLE workspace.oauth2_authorization_codes ADD CONSTRAINT oauth2_authorization_codes_redirect_uri_is_bounded CHECK (length(redirect_uri) <= 2048) NOT VALID;

ALTER TABLE workspace.oauth2_clients ADD CONSTRAINT oauth2_clients_client_uri_is_bounded CHECK (length(client_uri) <= 2048) NOT VALID;
ALTER TABLE workspace.oauth2_clients ADD CONSTRAINT oauth2_clients_logo_uri_is_bounded CHECK (length(logo_uri) <= 2048) NOT VALID;

ALTER TABLE workspace.push_tokens ADD CONSTRAINT push_tokens_app_id_is_bounded CHECK (length(app_id) <= 128) NOT VALID;
ALTER TABLE workspace.push_tokens ADD CONSTRAINT push_tokens_provider_is_bounded CHECK (length(provider) <= 32) NOT VALID;
ALTER TABLE workspace.push_tokens ADD CONSTRAINT push_tokens_token_ciphertext_is_bounded CHECK (length(token_ciphertext) <= 4096) NOT VALID;
ALTER TABLE workspace.push_tokens ADD CONSTRAINT push_tokens_token_hash_is_bounded CHECK (length(token_hash) <= 128) NOT VALID;

ALTER TABLE workspace.report_grants ADD CONSTRAINT report_grants_counterparty_ref_is_bounded CHECK (length(counterparty_ref) <= 200) NOT VALID;
ALTER TABLE workspace.report_grants ADD CONSTRAINT report_grants_note_is_bounded CHECK (length(note) <= 4000) NOT VALID;
ALTER TABLE workspace.report_grants ADD CONSTRAINT report_grants_report_key_is_bounded CHECK (length(report_key) <= 200) NOT VALID;
ALTER TABLE workspace.report_grants ADD CONSTRAINT report_grants_scope_is_bounded CHECK (length(scope) <= 1000) NOT VALID;

ALTER TABLE workspace.report_schedules ADD CONSTRAINT report_schedules_cron_is_bounded CHECK (length(cron) <= 100) NOT VALID;
ALTER TABLE workspace.report_schedules ADD CONSTRAINT report_schedules_format_is_bounded CHECK (length(format) <= 16) NOT VALID;
ALTER TABLE workspace.report_schedules ADD CONSTRAINT report_schedules_last_error_is_bounded CHECK (length(last_error) <= 4000) NOT VALID;
ALTER TABLE workspace.report_schedules ADD CONSTRAINT report_schedules_last_status_is_bounded CHECK (length(last_status) <= 32) NOT VALID;
ALTER TABLE workspace.report_schedules ADD CONSTRAINT report_schedules_name_is_bounded CHECK (length(name) <= 200) NOT VALID;
ALTER TABLE workspace.report_schedules ADD CONSTRAINT report_schedules_report_key_is_bounded CHECK (length(report_key) <= 200) NOT VALID;

ALTER TABLE workspace.reporting_probe ADD CONSTRAINT reporting_probe_contact_name_is_bounded CHECK (length(contact_name) <= 200) NOT VALID;

ALTER TABLE workspace.roles ADD CONSTRAINT roles_description_is_bounded CHECK (length(description) <= 4000) NOT VALID;

ALTER TABLE workspace.sessions ADD CONSTRAINT sessions_user_agent_is_bounded CHECK (length(user_agent) <= 512) NOT VALID;

ALTER TABLE workspace.staff_pin_credentials ADD CONSTRAINT staff_pin_credentials_pin_hash_is_bounded CHECK (length(pin_hash) <= 128) NOT VALID;

ALTER TABLE workspace.tenant_profiles ADD CONSTRAINT tenant_profiles_logo_url_is_bounded CHECK (length(logo_url) <= 2048) NOT VALID;

-- NOT VALID-ээр нэмээд тусад нь батална: ADD CONSTRAINT нь хүснэгтийг
-- бичихээс хаадаг ч VALIDATE нь зөвхөн уншина, тиймээс том хүснэгтэй
-- суулгац дээр ч энэ миграц түгжээ барихгүй.
ALTER TABLE registry.announcements VALIDATE CONSTRAINT announcements_body_is_bounded;
ALTER TABLE registry.announcements VALIDATE CONSTRAINT announcements_kind_is_bounded;
ALTER TABLE registry.announcements VALIDATE CONSTRAINT announcements_title_is_bounded;
ALTER TABLE registry.app_versions VALIDATE CONSTRAINT app_versions_package_sha256_is_bounded;
ALTER TABLE registry.app_versions VALIDATE CONSTRAINT app_versions_package_url_is_bounded;
ALTER TABLE registry.apps VALIDATE CONSTRAINT apps_description_is_bounded;
ALTER TABLE registry.apps VALIDATE CONSTRAINT apps_icon_url_is_bounded;
ALTER TABLE registry.credential_grants VALIDATE CONSTRAINT credential_grants_purpose_is_bounded;
ALTER TABLE registry.eid_sign_state VALIDATE CONSTRAINT eid_sign_state_key_is_bounded;
ALTER TABLE registry.eid_sign_state VALIDATE CONSTRAINT eid_sign_state_value_is_bounded;
ALTER TABLE registry.feature_flag_overrides VALIDATE CONSTRAINT feature_flag_overrides_flag_key_is_bounded;
ALTER TABLE registry.feature_flags VALIDATE CONSTRAINT feature_flags_description_is_bounded;
ALTER TABLE registry.feature_flags VALIDATE CONSTRAINT feature_flags_key_is_bounded;
ALTER TABLE registry.feature_flags VALIDATE CONSTRAINT feature_flags_kind_is_bounded;
ALTER TABLE registry.feature_flags VALIDATE CONSTRAINT feature_flags_owner_is_bounded;
ALTER TABLE registry.identity_binding_sessions VALIDATE CONSTRAINT identity_binding_sessions_email_is_bounded;
ALTER TABLE registry.identity_binding_sessions VALIDATE CONSTRAINT identity_binding_sessions_issuer_is_bounded;
ALTER TABLE registry.identity_binding_sessions VALIDATE CONSTRAINT identity_binding_sessions_name_is_bounded;
ALTER TABLE registry.identity_binding_sessions VALIDATE CONSTRAINT identity_binding_sessions_subject_is_bounded;
ALTER TABLE registry.oauth2_signing_keys VALIDATE CONSTRAINT oauth2_signing_keys_private_key_pem_is_bounded;
ALTER TABLE registry.oauth2_signing_keys VALIDATE CONSTRAINT oauth2_signing_keys_public_key_pem_is_bounded;
ALTER TABLE registry.operator_impersonations VALIDATE CONSTRAINT operator_impersonations_operator_email_is_bounded;
ALTER TABLE registry.operator_impersonations VALIDATE CONSTRAINT operator_impersonations_reason_is_bounded;
ALTER TABLE registry.permissions VALIDATE CONSTRAINT permissions_description_is_bounded;
ALTER TABLE registry.person_items VALIDATE CONSTRAINT person_items_code_is_bounded;
ALTER TABLE registry.person_items VALIDATE CONSTRAINT person_items_source_app_is_bounded;
ALTER TABLE registry.person_items VALIDATE CONSTRAINT person_items_source_ref_is_bounded;
ALTER TABLE registry.person_items VALIDATE CONSTRAINT person_items_status_is_bounded;
ALTER TABLE registry.platform_settings VALIDATE CONSTRAINT platform_settings_key_is_bounded;
ALTER TABLE registry.platform_settings VALIDATE CONSTRAINT platform_settings_value_is_bounded;
ALTER TABLE registry.service_directory VALIDATE CONSTRAINT service_directory_title_is_bounded;
ALTER TABLE registry.store_app_versions VALIDATE CONSTRAINT store_app_versions_package_sha256_is_bounded;
ALTER TABLE registry.store_app_versions VALIDATE CONSTRAINT store_app_versions_package_url_is_bounded;
ALTER TABLE registry.store_app_versions VALIDATE CONSTRAINT store_app_versions_review_note_is_bounded;
ALTER TABLE registry.tenant_quotas VALIDATE CONSTRAINT tenant_quotas_enforcement_is_bounded;
ALTER TABLE registry.tenants VALIDATE CONSTRAINT tenants_kind_is_bounded;
ALTER TABLE registry.tenants VALIDATE CONSTRAINT tenants_maintenance_message_is_bounded;
ALTER TABLE registry.tenants VALIDATE CONSTRAINT tenants_suspension_reason_is_bounded;
ALTER TABLE registry.usage_events VALIDATE CONSTRAINT usage_events_metric_is_bounded;
ALTER TABLE registry.user_sso_identities VALIDATE CONSTRAINT user_sso_identities_email_is_bounded;
ALTER TABLE registry.user_sso_identities VALIDATE CONSTRAINT user_sso_identities_issuer_is_bounded;
ALTER TABLE registry.user_sso_identities VALIDATE CONSTRAINT user_sso_identities_name_is_bounded;
ALTER TABLE registry.user_sso_identities VALIDATE CONSTRAINT user_sso_identities_subject_is_bounded;
ALTER TABLE workspace.ai_knowledge VALIDATE CONSTRAINT ai_knowledge_content_is_bounded;
ALTER TABLE workspace.ai_knowledge VALIDATE CONSTRAINT ai_knowledge_source_url_is_bounded;
ALTER TABLE workspace.ai_prompts VALIDATE CONSTRAINT ai_prompts_content_is_bounded;
ALTER TABLE workspace.audit_events VALIDATE CONSTRAINT audit_events_action_is_bounded;
ALTER TABLE workspace.audit_events VALIDATE CONSTRAINT audit_events_resource_is_bounded;
ALTER TABLE workspace.audit_events VALIDATE CONSTRAINT audit_events_user_id_is_bounded;
ALTER TABLE workspace.dbguard_probe VALIDATE CONSTRAINT dbguard_probe_name_is_bounded;
ALTER TABLE workspace.default_app_probe VALIDATE CONSTRAINT default_app_probe_code_is_bounded;
ALTER TABLE workspace.default_app_probe VALIDATE CONSTRAINT default_app_probe_name_is_bounded;
ALTER TABLE workspace.device_enrollment_codes VALIDATE CONSTRAINT device_enrollment_codes_code_hash_is_bounded;
ALTER TABLE workspace.device_telemetry VALIDATE CONSTRAINT device_telemetry_event_is_bounded;
ALTER TABLE workspace.device_telemetry VALIDATE CONSTRAINT device_telemetry_level_is_bounded;
ALTER TABLE workspace.devices VALIDATE CONSTRAINT devices_app_version_is_bounded;
ALTER TABLE workspace.devices VALIDATE CONSTRAINT devices_form_factor_is_bounded;
ALTER TABLE workspace.devices VALIDATE CONSTRAINT devices_name_is_bounded;
ALTER TABLE workspace.devices VALIDATE CONSTRAINT devices_os_version_is_bounded;
ALTER TABLE workspace.devices VALIDATE CONSTRAINT devices_platform_is_bounded;
ALTER TABLE workspace.devices VALIDATE CONSTRAINT devices_site_is_bounded;
ALTER TABLE workspace.devices VALIDATE CONSTRAINT devices_status_is_bounded;
ALTER TABLE workspace.devices VALIDATE CONSTRAINT devices_token_hash_is_bounded;
ALTER TABLE workspace.email_verifications VALIDATE CONSTRAINT email_verifications_redirect_url_is_bounded;
ALTER TABLE workspace.esign_batch_items VALIDATE CONSTRAINT esign_batch_items_error_is_bounded;
ALTER TABLE workspace.esign_sign_sessions VALIDATE CONSTRAINT esign_sign_sessions_eid_session_id_is_bounded;
ALTER TABLE workspace.esign_signature_logs VALIDATE CONSTRAINT esign_signature_logs_detail_is_bounded;
ALTER TABLE workspace.join_requests VALIDATE CONSTRAINT join_requests_status_is_bounded;
ALTER TABLE workspace.oauth2_access_tokens VALIDATE CONSTRAINT oauth2_access_tokens_scope_is_bounded;
ALTER TABLE workspace.oauth2_authorization_codes VALIDATE CONSTRAINT oauth2_authorization_codes_nonce_is_bounded;
ALTER TABLE workspace.oauth2_authorization_codes VALIDATE CONSTRAINT oauth2_authorization_codes_redirect_uri_is_bounded;
ALTER TABLE workspace.oauth2_clients VALIDATE CONSTRAINT oauth2_clients_client_uri_is_bounded;
ALTER TABLE workspace.oauth2_clients VALIDATE CONSTRAINT oauth2_clients_logo_uri_is_bounded;
ALTER TABLE workspace.push_tokens VALIDATE CONSTRAINT push_tokens_app_id_is_bounded;
ALTER TABLE workspace.push_tokens VALIDATE CONSTRAINT push_tokens_provider_is_bounded;
ALTER TABLE workspace.push_tokens VALIDATE CONSTRAINT push_tokens_token_ciphertext_is_bounded;
ALTER TABLE workspace.push_tokens VALIDATE CONSTRAINT push_tokens_token_hash_is_bounded;
ALTER TABLE workspace.report_grants VALIDATE CONSTRAINT report_grants_counterparty_ref_is_bounded;
ALTER TABLE workspace.report_grants VALIDATE CONSTRAINT report_grants_note_is_bounded;
ALTER TABLE workspace.report_grants VALIDATE CONSTRAINT report_grants_report_key_is_bounded;
ALTER TABLE workspace.report_grants VALIDATE CONSTRAINT report_grants_scope_is_bounded;
ALTER TABLE workspace.report_schedules VALIDATE CONSTRAINT report_schedules_cron_is_bounded;
ALTER TABLE workspace.report_schedules VALIDATE CONSTRAINT report_schedules_format_is_bounded;
ALTER TABLE workspace.report_schedules VALIDATE CONSTRAINT report_schedules_last_error_is_bounded;
ALTER TABLE workspace.report_schedules VALIDATE CONSTRAINT report_schedules_last_status_is_bounded;
ALTER TABLE workspace.report_schedules VALIDATE CONSTRAINT report_schedules_name_is_bounded;
ALTER TABLE workspace.report_schedules VALIDATE CONSTRAINT report_schedules_report_key_is_bounded;
ALTER TABLE workspace.reporting_probe VALIDATE CONSTRAINT reporting_probe_contact_name_is_bounded;
ALTER TABLE workspace.roles VALIDATE CONSTRAINT roles_description_is_bounded;
ALTER TABLE workspace.sessions VALIDATE CONSTRAINT sessions_user_agent_is_bounded;
ALTER TABLE workspace.staff_pin_credentials VALIDATE CONSTRAINT staff_pin_credentials_pin_hash_is_bounded;
ALTER TABLE workspace.tenant_profiles VALIDATE CONSTRAINT tenant_profiles_logo_url_is_bounded;

-- +goose Down

ALTER TABLE registry.announcements DROP CONSTRAINT IF EXISTS announcements_body_is_bounded;
ALTER TABLE registry.announcements DROP CONSTRAINT IF EXISTS announcements_kind_is_bounded;
ALTER TABLE registry.announcements DROP CONSTRAINT IF EXISTS announcements_title_is_bounded;
ALTER TABLE registry.app_versions DROP CONSTRAINT IF EXISTS app_versions_package_sha256_is_bounded;
ALTER TABLE registry.app_versions DROP CONSTRAINT IF EXISTS app_versions_package_url_is_bounded;
ALTER TABLE registry.apps DROP CONSTRAINT IF EXISTS apps_description_is_bounded;
ALTER TABLE registry.apps DROP CONSTRAINT IF EXISTS apps_icon_url_is_bounded;
ALTER TABLE registry.credential_grants DROP CONSTRAINT IF EXISTS credential_grants_purpose_is_bounded;
ALTER TABLE registry.eid_sign_state DROP CONSTRAINT IF EXISTS eid_sign_state_key_is_bounded;
ALTER TABLE registry.eid_sign_state DROP CONSTRAINT IF EXISTS eid_sign_state_value_is_bounded;
ALTER TABLE registry.feature_flag_overrides DROP CONSTRAINT IF EXISTS feature_flag_overrides_flag_key_is_bounded;
ALTER TABLE registry.feature_flags DROP CONSTRAINT IF EXISTS feature_flags_description_is_bounded;
ALTER TABLE registry.feature_flags DROP CONSTRAINT IF EXISTS feature_flags_key_is_bounded;
ALTER TABLE registry.feature_flags DROP CONSTRAINT IF EXISTS feature_flags_kind_is_bounded;
ALTER TABLE registry.feature_flags DROP CONSTRAINT IF EXISTS feature_flags_owner_is_bounded;
ALTER TABLE registry.identity_binding_sessions DROP CONSTRAINT IF EXISTS identity_binding_sessions_email_is_bounded;
ALTER TABLE registry.identity_binding_sessions DROP CONSTRAINT IF EXISTS identity_binding_sessions_issuer_is_bounded;
ALTER TABLE registry.identity_binding_sessions DROP CONSTRAINT IF EXISTS identity_binding_sessions_name_is_bounded;
ALTER TABLE registry.identity_binding_sessions DROP CONSTRAINT IF EXISTS identity_binding_sessions_subject_is_bounded;
ALTER TABLE registry.oauth2_signing_keys DROP CONSTRAINT IF EXISTS oauth2_signing_keys_private_key_pem_is_bounded;
ALTER TABLE registry.oauth2_signing_keys DROP CONSTRAINT IF EXISTS oauth2_signing_keys_public_key_pem_is_bounded;
ALTER TABLE registry.operator_impersonations DROP CONSTRAINT IF EXISTS operator_impersonations_operator_email_is_bounded;
ALTER TABLE registry.operator_impersonations DROP CONSTRAINT IF EXISTS operator_impersonations_reason_is_bounded;
ALTER TABLE registry.permissions DROP CONSTRAINT IF EXISTS permissions_description_is_bounded;
ALTER TABLE registry.person_items DROP CONSTRAINT IF EXISTS person_items_code_is_bounded;
ALTER TABLE registry.person_items DROP CONSTRAINT IF EXISTS person_items_source_app_is_bounded;
ALTER TABLE registry.person_items DROP CONSTRAINT IF EXISTS person_items_source_ref_is_bounded;
ALTER TABLE registry.person_items DROP CONSTRAINT IF EXISTS person_items_status_is_bounded;
ALTER TABLE registry.platform_settings DROP CONSTRAINT IF EXISTS platform_settings_key_is_bounded;
ALTER TABLE registry.platform_settings DROP CONSTRAINT IF EXISTS platform_settings_value_is_bounded;
ALTER TABLE registry.service_directory DROP CONSTRAINT IF EXISTS service_directory_title_is_bounded;
ALTER TABLE registry.store_app_versions DROP CONSTRAINT IF EXISTS store_app_versions_package_sha256_is_bounded;
ALTER TABLE registry.store_app_versions DROP CONSTRAINT IF EXISTS store_app_versions_package_url_is_bounded;
ALTER TABLE registry.store_app_versions DROP CONSTRAINT IF EXISTS store_app_versions_review_note_is_bounded;
ALTER TABLE registry.tenant_quotas DROP CONSTRAINT IF EXISTS tenant_quotas_enforcement_is_bounded;
ALTER TABLE registry.tenants DROP CONSTRAINT IF EXISTS tenants_kind_is_bounded;
ALTER TABLE registry.tenants DROP CONSTRAINT IF EXISTS tenants_maintenance_message_is_bounded;
ALTER TABLE registry.tenants DROP CONSTRAINT IF EXISTS tenants_suspension_reason_is_bounded;
ALTER TABLE registry.usage_events DROP CONSTRAINT IF EXISTS usage_events_metric_is_bounded;
ALTER TABLE registry.user_sso_identities DROP CONSTRAINT IF EXISTS user_sso_identities_email_is_bounded;
ALTER TABLE registry.user_sso_identities DROP CONSTRAINT IF EXISTS user_sso_identities_issuer_is_bounded;
ALTER TABLE registry.user_sso_identities DROP CONSTRAINT IF EXISTS user_sso_identities_name_is_bounded;
ALTER TABLE registry.user_sso_identities DROP CONSTRAINT IF EXISTS user_sso_identities_subject_is_bounded;
ALTER TABLE workspace.ai_knowledge DROP CONSTRAINT IF EXISTS ai_knowledge_content_is_bounded;
ALTER TABLE workspace.ai_knowledge DROP CONSTRAINT IF EXISTS ai_knowledge_source_url_is_bounded;
ALTER TABLE workspace.ai_prompts DROP CONSTRAINT IF EXISTS ai_prompts_content_is_bounded;
ALTER TABLE workspace.audit_events DROP CONSTRAINT IF EXISTS audit_events_action_is_bounded;
ALTER TABLE workspace.audit_events DROP CONSTRAINT IF EXISTS audit_events_resource_is_bounded;
ALTER TABLE workspace.audit_events DROP CONSTRAINT IF EXISTS audit_events_user_id_is_bounded;
ALTER TABLE workspace.dbguard_probe DROP CONSTRAINT IF EXISTS dbguard_probe_name_is_bounded;
ALTER TABLE workspace.default_app_probe DROP CONSTRAINT IF EXISTS default_app_probe_code_is_bounded;
ALTER TABLE workspace.default_app_probe DROP CONSTRAINT IF EXISTS default_app_probe_name_is_bounded;
ALTER TABLE workspace.device_enrollment_codes DROP CONSTRAINT IF EXISTS device_enrollment_codes_code_hash_is_bounded;
ALTER TABLE workspace.device_telemetry DROP CONSTRAINT IF EXISTS device_telemetry_event_is_bounded;
ALTER TABLE workspace.device_telemetry DROP CONSTRAINT IF EXISTS device_telemetry_level_is_bounded;
ALTER TABLE workspace.devices DROP CONSTRAINT IF EXISTS devices_app_version_is_bounded;
ALTER TABLE workspace.devices DROP CONSTRAINT IF EXISTS devices_form_factor_is_bounded;
ALTER TABLE workspace.devices DROP CONSTRAINT IF EXISTS devices_name_is_bounded;
ALTER TABLE workspace.devices DROP CONSTRAINT IF EXISTS devices_os_version_is_bounded;
ALTER TABLE workspace.devices DROP CONSTRAINT IF EXISTS devices_platform_is_bounded;
ALTER TABLE workspace.devices DROP CONSTRAINT IF EXISTS devices_site_is_bounded;
ALTER TABLE workspace.devices DROP CONSTRAINT IF EXISTS devices_status_is_bounded;
ALTER TABLE workspace.devices DROP CONSTRAINT IF EXISTS devices_token_hash_is_bounded;
ALTER TABLE workspace.email_verifications DROP CONSTRAINT IF EXISTS email_verifications_redirect_url_is_bounded;
ALTER TABLE workspace.esign_batch_items DROP CONSTRAINT IF EXISTS esign_batch_items_error_is_bounded;
ALTER TABLE workspace.esign_sign_sessions DROP CONSTRAINT IF EXISTS esign_sign_sessions_eid_session_id_is_bounded;
ALTER TABLE workspace.esign_signature_logs DROP CONSTRAINT IF EXISTS esign_signature_logs_detail_is_bounded;
ALTER TABLE workspace.join_requests DROP CONSTRAINT IF EXISTS join_requests_status_is_bounded;
ALTER TABLE workspace.oauth2_access_tokens DROP CONSTRAINT IF EXISTS oauth2_access_tokens_scope_is_bounded;
ALTER TABLE workspace.oauth2_authorization_codes DROP CONSTRAINT IF EXISTS oauth2_authorization_codes_nonce_is_bounded;
ALTER TABLE workspace.oauth2_authorization_codes DROP CONSTRAINT IF EXISTS oauth2_authorization_codes_redirect_uri_is_bounded;
ALTER TABLE workspace.oauth2_clients DROP CONSTRAINT IF EXISTS oauth2_clients_client_uri_is_bounded;
ALTER TABLE workspace.oauth2_clients DROP CONSTRAINT IF EXISTS oauth2_clients_logo_uri_is_bounded;
ALTER TABLE workspace.push_tokens DROP CONSTRAINT IF EXISTS push_tokens_app_id_is_bounded;
ALTER TABLE workspace.push_tokens DROP CONSTRAINT IF EXISTS push_tokens_provider_is_bounded;
ALTER TABLE workspace.push_tokens DROP CONSTRAINT IF EXISTS push_tokens_token_ciphertext_is_bounded;
ALTER TABLE workspace.push_tokens DROP CONSTRAINT IF EXISTS push_tokens_token_hash_is_bounded;
ALTER TABLE workspace.report_grants DROP CONSTRAINT IF EXISTS report_grants_counterparty_ref_is_bounded;
ALTER TABLE workspace.report_grants DROP CONSTRAINT IF EXISTS report_grants_note_is_bounded;
ALTER TABLE workspace.report_grants DROP CONSTRAINT IF EXISTS report_grants_report_key_is_bounded;
ALTER TABLE workspace.report_grants DROP CONSTRAINT IF EXISTS report_grants_scope_is_bounded;
ALTER TABLE workspace.report_schedules DROP CONSTRAINT IF EXISTS report_schedules_cron_is_bounded;
ALTER TABLE workspace.report_schedules DROP CONSTRAINT IF EXISTS report_schedules_format_is_bounded;
ALTER TABLE workspace.report_schedules DROP CONSTRAINT IF EXISTS report_schedules_last_error_is_bounded;
ALTER TABLE workspace.report_schedules DROP CONSTRAINT IF EXISTS report_schedules_last_status_is_bounded;
ALTER TABLE workspace.report_schedules DROP CONSTRAINT IF EXISTS report_schedules_name_is_bounded;
ALTER TABLE workspace.report_schedules DROP CONSTRAINT IF EXISTS report_schedules_report_key_is_bounded;
ALTER TABLE workspace.reporting_probe DROP CONSTRAINT IF EXISTS reporting_probe_contact_name_is_bounded;
ALTER TABLE workspace.roles DROP CONSTRAINT IF EXISTS roles_description_is_bounded;
ALTER TABLE workspace.sessions DROP CONSTRAINT IF EXISTS sessions_user_agent_is_bounded;
ALTER TABLE workspace.staff_pin_credentials DROP CONSTRAINT IF EXISTS staff_pin_credentials_pin_hash_is_bounded;
ALTER TABLE workspace.tenant_profiles DROP CONSTRAINT IF EXISTS tenant_profiles_logo_url_is_bounded;

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
