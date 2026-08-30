/**
 * setup — The first-run wizard: the screen a deployment is opened from, before
 * there is an organisation or anybody to sign in as.
 */
export const setup = {
  "setup.view.title": { mn: "Анхны тохиргоо", en: "First-run setup" },
  "setup.view.subtitle": {
    mn: "Энэ суулгацад байгууллага хараахан алга. Эхний байгууллага, түүнийг ажиллуулах админыг үүсгэе.",
    en: "This deployment has no organisation yet. Let us open the first one and the administrator who runs it.",
  },
  "setup.view.step_organisation": { mn: "Байгууллага", en: "Organisation" },
  "setup.view.step_admin": { mn: "Админ", en: "Administrator" },
  "setup.view.step_password": { mn: "Нууц үг", en: "Password" },
  "setup.view.step_console": { mn: "Консол", en: "Console" },

  "setup.field.registration_number": { mn: "Регистрийн дугаар", en: "Registration number" },
  "setup.field.organisation_name": { mn: "Байгууллагын нэр", en: "Organisation name" },
  "setup.field.legal_name": { mn: "Албан ёсны нэр", en: "Legal name" },
  "setup.field.slug": { mn: "Богино нэр (URL)", en: "Slug (URL)" },
  "setup.hint.super_admin": {
    mn: "Эхний бүртгэл нь Super Admin нэрээр автоматаар үүснэ. Энэ нь хүн биш — байгууллага үүсгэж, ажилчдаа урих хаалга. И-мэйл нь нууц үг сэргээх мессеж хүлээн авахад хэрэгтэй.",
    en: "The first account is created as Super Admin. It is a door rather than a person: it creates the organisations and invites the people who will work here. The address is where a password reset is sent.",
  },
  "setup.field.password_again": { mn: "Нууц үгээ давтах", en: "Repeat the password" },
  "setup.field.totp_code": { mn: "Authenticator-ийн код", en: "Code from the authenticator" },

  "setup.action.lookup": { mn: "Core-оос хайх", en: "Look up in Gerege Core" },
  "setup.action.finish": { mn: "Тохиргоог дуусгах", en: "Finish setup" },
  "setup.action.skip_console": { mn: "Консолгүйгээр дуусгах", en: "Finish without a console" },
  "setup.action.sign_in": { mn: "Нэвтрэх", en: "Sign in" },

  "setup.message.slug_hint": {
    mn: "Нэрнээс санал болгосон; засаж болно. Хаяг, OAuth audience-д ордог тул жижиг латин үсэг, тоо, зураас — 3-64 тэмдэгт.",
    en: "Suggested from the name; edit it freely. It appears in URLs and in the OAuth audience: lowercase letters, digits and hyphens, 3-64 characters.",
  },
  "setup.message.core_off": {
    mn: "GEREGE_CORE_TOKEN тохируулаагүй тул регистрээр хайх боломжгүй. Талбаруудыг гараар бөглөнө үү.",
    en: "GEREGE_CORE_TOKEN is not set, so the register cannot be searched. Fill the fields in by hand.",
  },
  "setup.message.console_lede": {
    mn: "Операторын консол {host} дээр нээгдэнэ. Түүний эхний бүртгэл нь байгууллагын админаас тусдаа: өөр нэвтрэлт, өөр cookie, өөр audit. Нэвтрэхэд нууц үг ба authenticator-ийн код хоёулаа шаардлагатай.",
    en: "The operator console will answer at {host}. Its first account is separate from the organisation's administrator: its own sign-in, its own cookie, its own audit trail. Signing in needs both a password and a code from an authenticator.",
  },
  "setup.message.operator_password_rule": {
    mn: "Дор хаяж 12 тэмдэгт — консолын бүртгэл платформ өөрөө тул илүү урт.",
    en: "At least 12 characters — a console account is the platform itself, so the rule is longer.",
  },
  "setup.message.enrolment": {
    mn: "Энэ кодыг authenticator-т нэмээд (1Password, Aegis, Google Authenticator) гарсан зургаан оронтой тоог доор бичнэ үү. Энэ түлхүүр дахин харагдахгүй.",
    en: "Add this to an authenticator — 1Password, Aegis, Google Authenticator — and type the six digits it shows. This secret is not shown again.",
  },

  "setup.message.password_rule": {
    mn: "Дор хаяж 10 тэмдэгт.",
    en: "At least 10 characters.",
  },
  "setup.message.password_mismatch": { mn: "Хоёр нууц үг таарахгүй байна.", en: "The two passwords are not the same." },
  "setup.message.not_required": {
    mn: "Энэ суулгац аль хэдийн тохируулагдсан байна.",
    en: "This deployment has already been set up.",
  },
  "setup.field.token": { mn: "Тохиргооны токен", en: "Setup token" },
  "setup.action.use_token": { mn: "Үргэлжлүүлэх", en: "Continue" },
  "setup.message.token_missing": {
    mn: "Энэ суулгацыг нээх токеныг сервер асахдаа лог руугаа нэг удаа бичсэн байгаа. Тэр мөрөөс хуулж энд буулгана уу.",
    en: "The token that opens this deployment was written once to the server's log at boot. Copy it from that line and paste it here.",
  },

  "setup.message.token_stale": {
    mn: "Энэ токен хүчингүй болжээ — сервер дахин асахад шинэ токен үүсдэг. Логийн хамгийн сүүлийн мөрөөс аваарай.",
    en: "That token is no longer valid — a restart mints a new one. Take the most recent line from the log.",
  },

  "setup.message.not_armed": {
    mn: "Тохиргооны токен байхгүй байна. Сервер асахад бичсэн лог мөрөөс хаягийг нь аваарай, эсвэл серверийг дахин асаагаад шинэ токен авна уу.",
    en: "There is no setup token. Take the address from the line the server wrote to its log at boot, or restart it for a new one.",
  },
  "setup.message.done": {
    mn: "Байгууллага нээгдлээ. Одоо шинэ бүртгэлээрээ нэвтэрнэ үү.",
    en: "The organisation is open. Sign in with the account you just made.",
  },
  "setup.message.apps_next": {
    mn: "Аппуудаа нэвтэрсний дараа Апп Дэлгүүрээс суулгана.",
    en: "Apps are installed from the store once you are in.",
  },
} as const;
