/**
 * Public documentation manifest.
 *
 * This is intentionally data-only. The builder and verifier both consume the
 * same manifest, so adding a page cannot silently update navigation without
 * also updating the checks that protect the published site.
 */
export const GITHUB = "https://github.com/gerege-systems/open-gerege-nexus";
export const BLOB = `${GITHUB}/blob/main`;
export const TREE = `${GITHUB}/tree/main`;

export const PAGES = [
  {src: "README.md", slug: "index", title: "Тойм", group: "Танилцуулга", lang: "mn"},
  {src: "docs/README_EN.md", slug: "overview-en", title: "Overview", group: "Танилцуулга", lang: "en"},
  {src: "docs/README_AR.md", slug: "overview-ar", title: "نظرة عامة", group: "Танилцуулга", lang: "ar", rtl: true},
  {src: "docs/README_ZH.md", slug: "overview-zh", title: "概览", group: "Танилцуулга", lang: "zh"},
  {src: "docs/README_FR.md", slug: "overview-fr", title: "Aperçu", group: "Танилцуулга", lang: "fr"},
  {src: "docs/README_RU.md", slug: "overview-ru", title: "Обзор", group: "Танилцуулга", lang: "ru"},
  {src: "docs/README_ES.md", slug: "overview-es", title: "Resumen", group: "Танилцуулга", lang: "es"},

  {src: "docs/README.md", slug: "documents", title: "Баримтын индекс", group: "Архитектур"},
  {src: "docs/ARCHITECTURE_SPECIFICATION.md", slug: "architecture", title: "Архитектурын тодорхойлолт", group: "Архитектур"},
  {src: "docs/ARCHITECTURE_SPECIFICATION_EN.md", slug: "architecture-en", title: "Architecture specification (EN)", group: "Архитектур"},
  {src: "docs/SSO_FEDERATION.md", slug: "sso-federation", title: "SSO холбоос", group: "Архитектур"},
  {src: "docs/ECOSYSTEM_GIT_STRATEGY.md", slug: "ecosystem-git-strategy", title: "Экосистемийн git стратеги", group: "Архитектур"},
  {src: "docs/adr/0001-domain-first.md", slug: "adr-0001-domain-first", title: "ADR 0001 — Домэйн эхэнд", group: "Архитектур"},
  {src: "docs/adr/0002-one-signing-rail.md", slug: "adr-0002-one-signing-rail", title: "ADR 0002 — Гарын үсгийн зам нэг", group: "Архитектур"},
  {src: "docs/adr/0003-a-document-carries-what-is-signed.md", slug: "adr-0003-document-carries", title: "ADR 0003 — Баримт файлаа авч явна", group: "Архитектур"},
  {src: "docs/adr/0004-a-pilot-that-did-not-ship.md", slug: "adr-0004-pilot", title: "ADR 0004 — Гараагүй pilot", group: "Архитектур"},
  {src: "docs/adr/0005-two-planes-one-origin-each.md", slug: "adr-0005-two-origins", title: "ADR 0005 — Нэг бинарь, хоёр origin", group: "Архитектур"},
  {src: "docs/adr/0007-a-rail-needs-a-second-caller.md", slug: "adr-0007-a-rail-needs-a-second-caller", title: "ADR 0007 — Рельс болохын тулд хоёр дахь дуудагч", group: "Архитектур"},
  {src: "docs/TWO_PLANES_REVIEW.md", slug: "two-planes-review", title: "Хоёр урсгалын хэрэгжилтийн шалгалт", group: "Архитектур"},

  {src: "docs/MODULE_AUTHORING_GUIDE.md", slug: "module-authoring", title: "Модуль хөгжүүлэх заавар", group: "Хөгжүүлэлт"},
  {src: "docs/APPSTORE_OPERATIONS.md", slug: "appstore-operations", title: "Апп сторын ажиллагаа", group: "Хөгжүүлэлт"},
  {src: "docs/RELEASING.md", slug: "releasing", title: "Хувилбар гаргах", group: "Хөгжүүлэлт"},
  {src: "docs/TRANSLATION_GUIDE.md", slug: "translation", title: "Орчуулгын гарын авлага", group: "Хөгжүүлэлт"},

  {src: "docs/GOV_SERVICES_WORKFLOW.md", slug: "gov-services", title: "Төрийн үйлчилгээний урсгал", group: "Модулиуд"},
  {src: "docs/DOCUMENTS_SIGNING.md", slug: "documents-signing", title: "Цахим гарын үсэг", group: "Модулиуд"},
  {src: "docs/REPORTS.md", slug: "reports", title: "Тайлангийн хөдөлгүүр", group: "Модулиуд"},
  {src: "docs/REPORT_SHARING.md", slug: "report-sharing", title: "Тенант дамнасан тайлан", group: "Модулиуд"},

  {src: "docs/MONITORING.md", slug: "monitoring", title: "Мониторинг", group: "Ажиллагаа"},
  {src: "docs/RUNBOOKS.md", slug: "runbooks", title: "Runbook-ууд", group: "Ажиллагаа"},
  {src: "docs/CONTROL_PLANE.md", slug: "control-plane", title: "Control plane", group: "Ажиллагаа"},
  {src: "docs/MONITORING_AND_REPORTING_PROPOSAL.md", slug: "monitoring-proposal", title: "Ажиглалт ба тайлангийн санал", group: "Ажиллагаа"},

  {src: "docs/SHELL_CONTRACT.md", slug: "shell-contract", title: "Bridge гэрээ", group: "Native клиентүүд"},
  {src: "docs/NATIVE_LOGIN_SPEC.md", slug: "native-login", title: "Native нэвтрэлт", group: "Native клиентүүд"},
  {src: "docs/NATIVE_SETTINGS_SPEC.md", slug: "native-settings", title: "Native тохиргоо", group: "Native клиентүүд"},

  {src: "CONTRIBUTING.md", slug: "contributing", title: "Хувь нэмэр оруулах", group: "Төслийн журам"},
  {src: "docs/CONTRIBUTING_EN.md", slug: "contributing-en", title: "Contributing (EN)", group: "Төслийн журам"},
  {src: "SECURITY.md", slug: "security", title: "Аюулгүй байдал", group: "Төслийн журам"},
  {src: "docs/SECURITY_EN.md", slug: "security-en", title: "Security policy (EN)", group: "Төслийн журам"},
  {src: "CODE_OF_CONDUCT.md", slug: "code-of-conduct", title: "Ёс зүйн дүрэм", group: "Төслийн журам"},
  {src: "docs/CODE_OF_CONDUCT_EN.md", slug: "code-of-conduct-en", title: "Code of conduct (EN)", group: "Төслийн журам"},
  {src: "CHANGELOG.md", slug: "changelog", title: "Өөрчлөлтийн түүх", group: "Төслийн журам"},
];

export const LANGUAGES = [
  {lang: "mn", label: "Монгол", flag: "flag-mn.png"},
  {lang: "ar", label: "العربية", flag: "flag-ar.png"},
  {lang: "zh", label: "中文", flag: "flag-zh.png"},
  {lang: "en", label: "English", flag: "flag-en.png"},
  {lang: "fr", label: "Français", flag: "flag-fr.png"},
  {lang: "ru", label: "Русский", flag: "flag-ru.png"},
  {lang: "es", label: "Español", flag: "flag-es.png"},
];
