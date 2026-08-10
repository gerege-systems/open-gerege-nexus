/**
 * developer — OAuth2 / OIDC client registration for third parties, and the
 * consent screen a user sees when one of those clients asks to sign them in.
 */
export const developer = {
  "developer.view.title": { mn: "Хөгжүүлэгчийн аппууд", en: "Developer apps" },
  "developer.view.subtitle": { mn: "Гуравдагч системд зориулсан OAuth2 / OIDC client бүртгэл", en: "OAuth2 / OIDC client registration for third-party systems" },
  "developer.view.create_title": { mn: "Шинэ OAuth2 client апп бүртгэх", en: "Register New OAuth2 Client App" },
  "developer.view.edit_title": { mn: "Апп тохируулах", en: "Configure application" },
  "developer.view.endpoints_title": { mn: "Холболтын хаягууд", en: "Connection endpoints" },
  "developer.view.empty_title": { mn: "Бүртгэлтэй апп алга", en: "No applications yet" },
  "developer.view.empty_body": { mn: "Гуравдагч систем Gerege-ээр дамжуулан нэвтрэхийн тулд эхлээд OAuth2 client бүртгэнэ.", en: "Register an OAuth2 client so a third-party system can sign users in through Gerege." },

  "developer.field.redirect_uris": { mn: "Redirect URI-ууд", en: "Redirect URIs" },
  "developer.field.scopes": { mn: "Scope-ууд", en: "Scopes" },
  "developer.field.name": { mn: "Аппын нэр", en: "Application name" },
  "developer.field.client_type": { mn: "Клиентийн төрөл", en: "Client type" },
  "developer.field.grant_types": { mn: "Grant төрлүүд", en: "Grant types" },
  "developer.field.homepage": { mn: "Вэб хаяг", en: "Homepage URL" },
  "developer.field.last_used": { mn: "Сүүлд ашигласан", en: "Last used" },

  "developer.type.confidential": { mn: "Нууцлагдсан (сервер талын)", en: "Confidential (server-side)" },
  "developer.type.confidential_hint": { mn: "Secret хадгалж чадах backend. Secret-ийг сервер дээрээ л хадгална.", en: "A backend that can keep a secret. Store it server-side only." },
  "developer.type.public": { mn: "Нээлттэй (SPA / мобайл)", en: "Public (SPA / mobile)" },
  "developer.type.public_hint": { mn: "Secret өгөхгүй — хэрэглэгчийн төхөөрөмж дэх secret нь нууц биш. Оронд нь PKCE ашиглана.", en: "No secret is issued: one shipped to a user's device is not a secret. PKCE stands in for it." },

  "developer.action.create": { mn: "OAuth2 client бүртгэх", en: "Register OAuth2 Client" },
  "developer.action.rotate": { mn: "Secret солих", en: "Rotate secret" },
  "developer.action.disable": { mn: "Идэвхгүй болгох", en: "Disable" },
  "developer.action.done": { mn: "Ойлголоо", en: "Got it" },

  "developer.message.loading": { mn: "OAuth2 client аппуудыг ачаалж байна...", en: "Loading OAuth2 client apps..." },
  "developer.message.secret_hidden": { mn: "үүсгэх үед нэг удаа харагдана", en: "shown once, at creation" },
  "developer.message.secret_once_title": { mn: "Secret-ийг одоо хуулж аваарай", en: "Copy this secret now" },
  "developer.message.secret_once_body": { mn: "Бид зөвхөн хэшийг нь хадгалдаг тул энэ цонхыг хаасны дараа secret дахин харагдахгүй. Алдвал шинээр солино.", en: "Only a digest is stored, so this cannot be shown again once you close this. If you lose it, rotate for a new one." },
  "developer.message.rotate_warning": { mn: "Хуучин secret тэр дороо ажиллахаа болино. Интеграцаа шинэчлэхэд бэлэн үү?", en: "The old secret stops working immediately. Ready to update your integration?" },
  "developer.message.delete_warning": { mn: "Энэ аппыг устгавал түүний олгосон бүх токен, зөвшөөрөл хамт устана. Буцаах боломжгүй.", en: "Deleting this application revokes every token and consent it ever issued. This cannot be undone." },
  "developer.message.disabled": { mn: "Идэвхгүй", en: "Disabled" },
  "developer.message.never_used": { mn: "хараахан ашиглаагүй", en: "never used" },
  "developer.message.pkce_note": { mn: "Бүх урсгалд PKCE (S256) заавал шаардана.", en: "PKCE (S256) is required on every flow." },

  // API keys — machine credentials

  // Access audit

  // OAuth scopes

  // Redirect policy

  // Signing keys

  // Consent screen
  "oauth.consent.title": { mn: "Нэвтрэх зөвшөөрөл", en: "Authorize access" },
  "oauth.consent.lede": { mn: "{app} таны Gerege бүртгэлээр нэвтрэхийг хүсэж байна.", en: "{app} wants to sign you in with your Gerege account." },
  "oauth.consent.will_be_able": { mn: "Энэ апп дараахыг хийх боломжтой болно:", en: "This application will be able to:" },
  "oauth.consent.already_granted": { mn: "Өмнө нь зөвшөөрсөн", en: "Already granted" },
  "oauth.consent.sensitive": { mn: "Эмзэг", en: "Sensitive" },
  "oauth.consent.redirect_note": { mn: "Зөвшөөрвөл таныг энэ хаяг руу буцаана:", en: "If you allow this, you will be returned to:" },
  "oauth.consent.allow": { mn: "Зөвшөөрөх", en: "Allow" },
  "oauth.consent.deny": { mn: "Татгалзах", en: "Deny" },
  "oauth.consent.loading": { mn: "Хүсэлтийг шалгаж байна...", en: "Checking the request..." },
  "oauth.consent.invalid": { mn: "Энэ зөвшөөрлийн хүсэлт хүчингүй байна.", en: "This authorization request is not valid." },
} as const;
