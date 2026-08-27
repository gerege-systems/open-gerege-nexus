/**
 * web — The client shell — sidebar, header, user menu and the placeholder a
 * menu falls back to before its screen exists.
 */
export const web = {
  "web.message.impersonated": {
    mn: "Энэ бол платформын операторын түр хандалт. Таны бүртгэлээр хийгдэж буй үйлдлүүд байгууллагын бүртгэлд «операторын» тэмдэгтэйгээр үлдэнэ.",
    en: "A platform operator is working in this account. Everything done here is marked as theirs in your organisation's audit trail.",
  },
  "web.menu.app_store": { mn: "Апп Дэлгүүр", en: "App Store" },
  "web.menu.installed_apps": { mn: "Суулгасан аппууд", en: "Installed Apps" },
  "web.menu.settings": { mn: "Тохиргоо", en: "Settings" },
  "web.menu.organisation": { mn: "Байгууллага", en: "Organisation" },
  "web.menu.appearance": { mn: "Харагдац", en: "Appearance" },
  "web.menu.preferences": { mn: "Тохиргоо", en: "Preferences" },
  "web.menu.ai_settings": { mn: "AI тохиргоо", en: "AI settings" },
  "web.menu.email_verification": { mn: "И-мэйл баталгаажуулалт", en: "Email verification" },

  "web.group.modules": { mn: "Модулиуд", en: "Modules" },
  "web.group.settings": { mn: "Тохиргоо", en: "Settings" },

  "web.field.theme": { mn: "Загвар", en: "Theme" },

  "web.label.platform": { mn: "Платформ", en: "Platform" },
  "web.label.apps": { mn: "Аппууд", en: "Apps" },

  "web.action.logout": { mn: "Гарах", en: "Sign out" },
  "web.action.close_menu": { mn: "Цэс хаах", en: "Close menu" },
  "web.action.toggle_menu": { mn: "Цэс нээх, хаах", en: "Toggle menu" },
  "web.action.more": { mn: "Бусад", en: "More" },
  "web.action.close_more": { mn: "Бусад аппыг хаах", en: "Close more apps" },
  "web.action.expand_all": { mn: "Бүгдийг нээх", en: "Expand all" },
  "web.action.collapse_all": { mn: "Бүгдийг хаах", en: "Collapse all" },

  "web.action.switch_tenant": { mn: "Байгууллага солих", en: "Switch organisation" },

  "web.view.tenants": { mn: "Байгууллагууд", en: "Organisations" },
  "web.view.more_apps": { mn: "Бусад апп", en: "More apps" },
  "web.view.search_placeholder": { mn: "Апп, цэс хайх...", en: "Search apps and menus..." },
  "web.view.coming_soon": { mn: "Удахгүй", en: "Coming soon" },
  "web.view.coming_soon_body": { mn: "Энэ хэсэг хөгжүүлэлтийн шатанд байна. Бэлэн болмогц энд харагдана.", en: "This screen is still being built. It will appear here once it ships." },

  "web.message.loading_platform": { mn: "Платформыг ачаалж байна...", en: "Loading {brand}..." },
  "web.message.only_tenant": { mn: "Та зөвхөн энэ байгууллагад харьяалагдаж байна.", en: "You belong to this organisation only." },
  // Reading alongside is not switching, and the wording has to carry that or
  // somebody will tick a box expecting new records to land somewhere else.
  "web.label.read_alongside": { mn: "Хамт харах", en: "Read alongside" },
  // The switcher's second line for a personal workspace. Its slug is derived
  // from a user id and says nothing, so the row says what it is instead.
  "web.label.my_home": { mn: "Миний гэр", en: "My home" },
  // The rail entry for /me. Only drawn in a personal workspace: an
  // organisation's members ask through their organisation, so the screen would
  // be permanently empty for them — see lib/workspaceKind.mjs.
  "web.menu.my_requests": { mn: "Миний хүсэлтүүд", en: "My requests" },
  "web.message.read_alongside_hint": {
    mn: "Сонгосон байгууллагуудын жагсаалтыг хамт харна. Шинэ бичлэг одоогийн байгууллагад л үүснэ.",
    en: "Lists span the organisations you tick. New records are still created in the one you are working in.",
  },
  "web.message.tenant_switch_failed": { mn: "Байгууллага солиж чадсангүй. Дахин оролдоно уу.", en: "Could not switch organisation. Please try again." },
} as const;
