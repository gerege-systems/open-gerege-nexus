/**
 * website — The public landing page: what the platform is, before anyone
 * signs in.
 */
export const website = {
  // The menu follows the order of the page, so a reader who picks the first
  // item lands near the top rather than at the bottom. Which of these appear
  // depends on which sections the deployment renders — see lib/landing.ts.
  "website.menu.architecture": { mn: "Архитектур", en: "Architecture" },
  "website.menu.applications": { mn: "Аппууд", en: "Applications" },
  "website.menu.platform": { mn: "Платформын суурь", en: "Platform" },
  "website.menu.trust": { mn: "Аюулгүй байдал", en: "Security" },
  "website.menu.technology": { mn: "Технологи", en: "Technology" },
  // The section is `#features`; what it argues is that identity is the floor.
  "website.menu.identity": { mn: "Нэвтрэлт", en: "Identity" },
  // Leaves the product for the published documentation. The other menu items
  // scroll within this page, so this one is marked as leaving.
  "website.menu.docs": { mn: "Баримт бичиг", en: "Documentation" },
  "website.menu.toggle": { mn: "Цэс", en: "Menu" },

  "website.action.sign_in": { mn: "Нэвтрэх", en: "Sign in" },
  "website.action.eid_sign_in": { mn: "eID-ээр нэвтрэх", en: "Sign in with eID" },
  "website.action.see_features": { mn: "Боломжийг үзэх", en: "See what it does" },

  // The hero headline is one sentence with a highlighted middle, so it is
  // stored in three parts rather than as markup inside a translation. The split
  // falls mid-phrase on purpose: what is highlighted is the claim, not a whole
  // clause, and each language chooses its own break.
  "website.view.hero_title_lead": { mn: "Модульт дижитал үйлчилгээний", en: "An open core for" },
  "website.view.hero_title_highlight": { mn: "нээлттэй цөм", en: "modular digital services" },
  "website.view.hero_title_tail": { mn: "юм", en: "on your infrastructure" },
  "website.view.hero_lede": {
    mn: "{brand}-ийн base repository нь identity, tenant, RBAC, SSO, апп суулгах runtime болон нэг built-in SSO Clients апп агуулна. Бизнес аппууд тусдаа product distribution-аас нэмэгдэнэ.",
    en: "The {brand} base repository provides identity, tenancy, RBAC, SSO, the app runtime and one built-in SSO Clients app. Product distributions add business applications separately.",
  },

  // Three numbers that hold still. Keep the app count in step with the base
  // catalogue; distributions can override it with BRAND_COPY.
  //
  // The figures are keys rather than literals so that a deployment counting
  // something else can say so: an identity provider shipping four modules does
  // not have nine business applications, and a number the deployment cannot
  // correct is a number its landing page states wrongly. Same value in every
  // language — they are digits — but they go through `t()` so BRAND_COPY can
  // reach them like any other line.
  "website.stat.apps_count": { mn: "1", en: "1" },
  "website.stat.apps": { mn: "үндсэн репод багтсан апп", en: "app included in the base repository" },
  "website.stat.languages_count": { mn: "7", en: "7" },
  "website.stat.languages": { mn: "хэл — монгол + НҮБ-ын 6", en: "languages: Mongolian plus the UN six" },
  "website.stat.binary_count": { mn: "1", en: "1" },
  "website.stat.binary": { mn: "Go API бинари", en: "Go API binary" },

  "website.view.features_eyebrow": { mn: "IDENTITY БА ACCESS", en: "IDENTITY AND ACCESS" },
  "website.view.features_title": {
    mn: "Нэг серверийн identity ба access урсгал",
    en: "One server-side identity and access flow",
  },
  "website.view.features_lede": {
    mn: "{brand} нь local login, eID, federated SSO-г tenant membership, role, audit болон OIDC provider-тэй нэг server-side урсгалд холбодог.",
    en: "{brand} connects local login, eID and federated SSO to tenant membership, roles, audit and its OIDC provider in one server-side flow.",
  },

  "website.feature.instant_title": { mn: "eID нэвтрэх сувгууд", en: "eID sign-in channels" },
  "website.feature.instant_body": {
    mn: "Тохируулсан deployment дээр регистрийн дугаарын push, desktop QR болон mobile App2App урсгалыг backend эхлүүлж, төлвийг шалгана.",
    en: "When a deployment is configured for eID, the backend starts and polls registration-number push, desktop QR and mobile App2App flows.",
  },
  "website.feature.sso_title": { mn: "Нэг нэвтрэлт — олон систем", en: "One sign-in, many systems" },
  "website.feature.sso_body": {
    mn: "Built-in OAuth2/OIDC provider нь идэвхтэй platform session-ийг бүртгэлтэй redirect URI бүхий клиентүүдэд дахин ашиглах боломж олгоно.",
    en: "The built-in OAuth2/OIDC provider can reuse an active platform session for clients with registered redirect URIs.",
  },
  "website.feature.passwordless_title": { mn: "Credential сервер талд үлдэнэ", en: "Credentials stay server-side" },
  "website.feature.passwordless_body": {
    mn: "eID RP secret болон бусад provider credential browser-д очихгүй. Session token database-д зөвхөн hash хэлбэрээр хадгалагдана.",
    en: "The eID RP secret and other provider credentials do not reach the browser. Session tokens are stored in the database only as hashes.",
  },
  "website.feature.channels_title": { mn: "Апп ба вэбийн нэг урсгал", en: "One flow across app and web" },
  "website.feature.channels_body": {
    mn: "Web болон native shell нь backend-ийн эхлүүлэх, төлөв шалгах endpoint-уудыг ашиглана; provider credential клиентэд хадгалагдахгүй.",
    en: "Web and native shells use backend start and status endpoints; provider credentials are not stored in either client.",
  },

  "website.view.trust_eyebrow": { mn: "ИДЭВХТЭЙ ХАМГААЛАЛТ", en: "ACTIVE PROTECTION" },
  "website.view.trust_title": {
    mn: "Танилтаас эрх хүртэл нэг баталгааны гинж",
    en: "One chain of proof, from identity to permission",
  },
  "website.view.trust_lede": {
    mn: "eID identity → серверийн session → tenant membership → RBAC → OIDC client. Алхам бүр сервер талд шалгагдана.",
    en: "eID identity → server session → tenant membership → RBAC → OIDC client. Every link is checked on the server.",
  },
  "website.trust.cookie": { mn: "httpOnly, SameSite session cookie", en: "httpOnly, SameSite session cookie" },
  "website.trust.rbac": { mn: "Tenant-аар тусгаарласан role ба permission", en: "Roles and permissions isolated per tenant" },
  "website.trust.allowlist": { mn: "Бүртгэлтэй OAuth2 redirect URI шалгалт", en: "Registered OAuth2 redirect URI validation" },
  "website.trust.audit": { mn: "Login ба access audit event", en: "Login and access audit events" },

  // Key kept as-is: it is internal, and renaming it would touch every caller
  // for no user-visible gain. The value is what reaches the screen.
  "website.tech.erp_body": { mn: "Модульт аппууд, tenant тусгаарлалт, RBAC", en: "Modular apps, tenant isolation, RBAC" },
  "website.tech.eid_body": { mn: "Push, QR, App2App, баталгаажсан identity", en: "Push, QR, App2App, verified identity" },
  "website.tech.sso_body": { mn: "Холбогдсон аппууд, нэг session", en: "Connected applications, one session" },

  // ─── The platform itself ───────────────────────────────────────────────────
  // Everything above this line argues for the sign-in. Everything below argues
  // for the platform behind it, and was moved here from the documentation site,
  // which used to make the same case in a second place that could drift.

  "website.arch.eyebrow": { mn: "ЯАГААД ЭНЭ АРХИТЕКТУР", en: "WHY THIS ARCHITECTURE" },
  "website.arch.title": {
    mn: "Нэг API процесс, тенант бүрт тусдаа суулгалт",
    en: "One API process, separate installation state per tenant",
  },
  "website.arch.lede": {
    mn: "Compile хийсэн module-ууд Go process дотор route, menu, permission, migration-аа бүртгэнэ. Аль модуль идэвхтэйг tenant бүрийн installation state шийднэ.",
    en: "Compiled modules register routes, menus, permissions and migrations in the Go process. Each tenant's installation state decides which modules are enabled.",
  },
  "website.arch.modules_title": { mn: "Компиллогдсон Go модулиуд", en: "Compiled-in Go modules" },
  "website.arch.modules_body": {
    mn: "Модуль бүр `pkg/nexus` Go гэрээг хэрэгжүүлж нэг API бинарид компиллогдоно. Маршрут, цэс, эрх, миграцаа module өөрөө бүртгэнэ.",
    en: "Each module implements the `pkg/nexus` Go contract and compiles into one API binary. The module registers its routes, menus, permissions and migrations.",
  },
  "website.arch.store_title": { mn: "Тенант бүрийн апп стор", en: "An app store for each tenant" },
  "website.arch.store_body": {
    mn: "Аль байгууллагад аль апп идэвхтэйг өгөгдлийн сан шийднэ. Суулгаагүй апп руу хандвал хориглоно — код нь байгаа ч хаалга нь хаалттай.",
    en: "The database decides which apps an organisation runs. An app that is not installed refuses the request: the code is there, the door is not open.",
  },
  "website.arch.dag_title": { mn: "Хамаарал шийдвэрлэгч", en: "Dependency resolution" },
  "website.arch.dag_body": {
    mn: "Рекурсив шийдвэрлэлт, мөчлөг илрүүлэлт, хувилбарын шалгалт. Апп суулгахад түүний хамаарал бүр тохирох хувилбартайгаа хамт орно.",
    en: "Recursive resolution with cycle detection and version checks, so installing an app brings every dependency it needs at a version that fits.",
  },
  "website.arch.catalog_title": { mn: "Bundled эсвэл signed remote каталог", en: "Bundled or signed remote catalogue" },
  "website.arch.catalog_body": {
    mn: "Default горимд release-тэй ирсэн `catalog/apps.json`-ийг уншина. `APP_CATALOG_URL` тохируулбал Ed25519 гарын үсэгтэй remote catalog-ийг шалгаж, cache болон bundled файл руу аюулгүй fallback хийнэ.",
    en: "By default the release reads its bundled `catalog/apps.json`. With `APP_CATALOG_URL`, it verifies an Ed25519-signed remote catalogue and safely falls back to its cache or bundled file.",
  },

  "website.apps.eyebrow": { mn: "ҮНДСЭН DISTRIBUTION", en: "BASE DISTRIBUTION" },
  "website.apps.title": { mn: "Нэг built-in апп, нэмэгдэх боломжтой платформ", en: "One built-in app, an extensible platform" },
  "website.apps.lede": {
    mn: "Энэ репогийн каталогт SSO клиент удирдах апп л байна. Бизнес аппуудыг тусдаа distribution репо компиллож, өөрийн каталогоор нэмдэг.",
    en: "This repository's catalogue contains only SSO client management. Product distributions compile in business apps and publish their own catalogue.",
  },
  "website.apps.sso_clients": { mn: "SSO клиентүүд — OAuth2/OIDC клиент бүртгэл", en: "SSO Clients — OAuth2/OIDC client registration" },

  "website.depth.eyebrow": { mn: "ПЛАТФОРМЫН СУУРЬ", en: "UNDER THE PLATFORM" },
  "website.depth.title": {
    mn: "Бүтээгдэхүүн болгонд дахин бичих шаардлагагүй зүйлс",
    en: "The parts you would otherwise rewrite for every product",
  },
  "website.depth.lede": {
    mn: "Эдгээр нь base runtime-д бодитоор байгаа shared capability-ууд. Гадаад provider ашигладаг хэсэг нь production credential болон тохиргоо шаарддаг.",
    en: "These shared capabilities exist in the base runtime. Features that call external providers still require production credentials and configuration.",
  },
  "website.depth.resilience_title": { mn: "Хүсэлтийн хамгаалалт", en: "Request protection" },
  "website.depth.resilience_body": {
    mn: "Хэт олон зэрэг хүсэлтийг 503-аар хязгаарлах load shedder, гадаад дуудлагын timeout, зориулалтын retry бодлого платформд хэрэгжсэн.",
    en: "The platform implements concurrency load shedding, outbound timeouts and operation-specific retry policies.",
  },
  "website.depth.gov_title": { mn: "Төрийн системийн connector", en: "State-system connectors" },
  "website.depth.gov_body": {
    mn: "XYP-ийн иргэн, хуулийн этгээдийн лавлагаа болон eID/ДАН identity connector код багтсан. Live үйлчилгээ нь тус бүрийн endpoint, client credential шаардана.",
    en: "The codebase includes XYP citizen/legal-entity lookups and eID/DAN identity connectors. Live use requires each provider's endpoint and client credentials.",
  },
  "website.depth.security_title": { mn: "Кодонд хэрэгжсэн хамгаалалт", en: "Controls implemented in code" },
  "website.depth.security_body": {
    mn: "Session token hash-аар, нууц үг bcrypt-ээр хадгалагдана. Tenant хүсэлт database role, tenant context болон RLS хамгаалалттай хүснэгтүүдээр тусгаарлагдана.",
    en: "Session tokens are hashed and passwords use bcrypt. Tenant requests are isolated with a database role, tenant context and RLS on declared tenant tables.",
  },
  "website.depth.ai_title": { mn: "Өөрийн өгөгдөлд холбогдсон AI", en: "AI wired to your own data" },
  "website.depth.ai_body": {
    mn: "Gemini түлхүүр өгвөл чат, яриа таних, унших, орчуулга ажиллана. Бизнес өгөгдөлд хандах хэрэгслийг тухайн distribution-ийн апп өөрөө бүртгэнэ.",
    en: "With a Gemini key, chat, speech, text-to-speech and translation are available. Product apps register the tools that expose their own business data.",
  },
  "website.depth.i18n_title": { mn: "Долоон locale, англи fallback", en: "Seven locales with English fallback" },
  "website.depth.i18n_body": {
    mn: "Монгол, англи эх мөрүүд дээр НҮБ-ын бусад таван хэлний overlay нэмэгдэнэ. Орчуулга дутвал англи руу fallback хийж, CI үлдсэн цоорхойг тайлагнана.",
    en: "Mongolian and English source strings are joined by five UN-language overlays. Missing translations fall back to English and CI reports the remaining gaps.",
  },
  "website.depth.observability_title": { mn: "Ажиглалт ба аудит", en: "Observability and audit" },
  "website.depth.observability_body": {
    mn: "Хэмжүүр, амьд ба бэлэн байдлын шалгалт, хэн юуг хэзээ өөрчилснийг бүртгэсэн ул мөр — эхний өдрөөс.",
    en: "Metrics, liveness and readiness probes, and a trail of who changed what and when — from the first day.",
  },

  "website.message.footer_note": {
    mn: "Apache 2.0 · Go · Next.js · PostgreSQL",
    en: "Apache 2.0 · Go · Next.js · PostgreSQL",
  },

  // Shown only by a deployment running under its own name — see SiteFooter.
  //
  // Deliberately not `{brand}`: this sentence names the platform underneath,
  // and interpolating the deployment's own name would have Gerege Salus
  // announcing that it is powered by Gerege Salus.
  "website.message.powered_by": {
    mn: "Gerege Nexus дээр суурилсан",
    en: "Powered by Gerege Nexus",
  },

  // Нэвтэрсэн хүний hero: eID картын оронд ирсэн гэрээ.
  "website.action.my_contracts": { mn: "Надад ирсэн гэрээ", en: "My incoming contracts" },
  "website.action.open_platform": { mn: "Платформ руу", en: "Open the platform" },
  "website.view.hero_inbox_title": { mn: "Танд ирсэн гэрээ", en: "Contracts sent to you" },
  "website.service.eyebrow": { mn: "ЭНЭ СУУЛГАЦ", en: "THIS DEPLOYMENT" },
  "website.service.title": { mn: "Платформын хажууд юу ажиллаж байна вэ", en: "What runs beside the platform" },
  "website.service.lede": {
    mn: "Nexus бол нэг хаяг биш. Консол, дата агуулах, нөөцлөлт, хяналт, баримт бичиг тус бүр өөрийн хаягтай. Эхнийх нь бидний биш — иргэнийг таних үндэсний дэд бүтэц.",
    en: "Nexus is not one hostname. The console, the warehouse, the backup store, the monitoring stack and the manual each answer on their own address. The first is not ours — it is the national identity infrastructure.",
  },

  "website.service.eid_title": { mn: "eID Mongolia", en: "eID Mongolia" },
  "website.service.eid_body": {
    mn: "Иргэнийг таних үндэсний дэд бүтэц. Энэ платформ нууц үг хадгалахын оронд түүн рүү асууна — QR, регистр, эсвэл App2App.",
    en: "The national identity infrastructure. Rather than keeping a password, this platform asks it — by QR, registration number or App2App.",
  },
  "website.service.admin_title": { mn: "Операторын консол", en: "Operator console" },
  "website.service.admin_body": {
    mn: "Байгууллага үүсгэх, түдгэлзүүлэх, эрх олгох, audit унших. Хэрэглэгчийн бүртгэлээс тусдаа identity, тусдаа cookie.",
    en: "Create and suspend organisations, grant capabilities, read the audit trail. A separate identity and cookie from a user account.",
  },
  "website.service.dwh_title": { mn: "Дата агуулах", en: "Data warehouse" },
  "website.service.dwh_body": {
    mn: "Үйл ажиллагааны системүүдээс өдөр бүр татаж, цэвэрлэж, бизнесийн хэлээр загварчилсан нэг агуулах. Дээр нь BI ба AI давхарга.",
    en: "One warehouse fed daily from the operational systems, cleaned and modelled in business language. BI and an AI layer on top.",
  },
  "website.service.backups_title": { mn: "Нөөцлөлт", en: "Backups" },
  "website.service.backups_body": {
    mn: "Өдөр бүр 03:15-д. Хостыг орхихоосоо өмнө шифрлэгдэж, эх сервер нь түүхээ устгаж чадахгүй сан руу очно.",
    en: "Daily at 03:15. Encrypted before it leaves the host, into a store whose history the source server cannot delete.",
  },
  "website.service.monitor_title": { mn: "Хяналт", en: "Monitoring" },
  "website.service.monitor_body": {
    mn: "Хэмжүүр, лог, trace, сэрэмжлүүлэг. Хэмжигдээгүй зүйл эвдэрсэн ч мэдэгддэггүй тул энэ нь нэмэлт биш, суурь.",
    en: "Metrics, logs, traces and alerts. What is not measured fails silently, which is why this is a foundation rather than an extra.",
  },
  "website.service.docs_title": { mn: "Баримт бичиг", en: "Documentation" },
  "website.service.docs_body": {
    mn: "Кодоос уншиж бичсэн архитектур, модуль, танилт, ажиллагаа. Долоон хэлээр.",
    en: "Architecture, modules, identity and operations — written from the code. In seven languages.",
  },

  "website.view.hero_inbox_empty": {
    mn: "Одоогоор хариу хүлээж буй гэрээ алга.",
    en: "Nothing is waiting for your signature right now.",
  },
  "website.view.hero_inbox_new": { mn: "Шинэ", en: "New" },
  "website.view.hero_inbox_opened": { mn: "Уншсан", en: "Opened" },
  "website.view.hero_inbox_more": { mn: "өөр гэрээ", en: "more" },
} as const;
