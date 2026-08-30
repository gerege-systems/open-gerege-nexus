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

  {src: "docs/README.md", slug: "documents", title: "Баримтын индекс", group: "Платформ"},
  {src: "docs/ARCHITECTURE.md", slug: "architecture", title: "Архитектур", group: "Платформ"},
  {src: "docs/IDENTITY.md", slug: "identity", title: "Танилт ба эрх", group: "Платформ"},

  {src: "docs/MODULES.md", slug: "modules", title: "Модуль бичих", group: "Модуль"},
  {src: "docs/REPORTS.md", slug: "reports", title: "Тайлан", group: "Модуль"},
  {src: "docs/SIGNING.md", slug: "signing", title: "Баримт ба гарын үсэг", group: "Модуль"},

  {src: "docs/OPERATIONS.md", slug: "operations", title: "Ажиллагаа", group: "Ажиллагаа"},
  {src: "docs/RUNBOOKS.md", slug: "runbooks", title: "Гарын авлага", group: "Ажиллагаа"},

  {src: "docs/SHELL_CONTRACT.md", slug: "shell-contract", title: "Bridge гэрээ", group: "Клиент"},
  {src: "docs/TRANSLATION.md", slug: "translation", title: "Орчуулга", group: "Клиент"},

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
