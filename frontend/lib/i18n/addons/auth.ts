/**
 * auth — Signing in through eID Mongolia, plus the administrator's password
 * fallback.
 */
export const auth = {
  "auth.message.platform_private": {
    mn: "Энэ платформ хаалттай горимд байна: зөвхөн урьдчилан бүртгэгдсэн хүн нэвтэрнэ. eID, ДАН, Google-ээр баталгаажсан ч бүртгэлгүй бол данс үүсэхгүй — байгууллагынхаа админаас урилга хүсэн үү.",
    en: "This platform is private: only people who have already been registered can sign in. Proving who you are with eID, ДАН or Google will not create an account — ask your organisation's administrator for an invitation.",
  },
  "auth.view.set_password_invite": { mn: "Нууц үгээ тохируулах", en: "Choose your password" },
  "auth.view.set_password_reset": { mn: "Нууц үгээ шинэчлэх", en: "Set a new password" },
  "auth.field.new_password": { mn: "Шинэ нууц үг", en: "New password" },
  "auth.field.repeat_password": { mn: "Дахин бичих", en: "Repeat it" },
  "auth.action.save_password": { mn: "Хадгалах", en: "Save" },
  "auth.hint.password_length": {
    mn: "Дор хаяж 10 тэмдэгт. Урт нь нарийн төвөгтэйгээс илүү чухал.",
    en: "At least 10 characters. Length matters more than punctuation.",
  },
  "auth.message.password_mismatch": { mn: "Хоёр нууц үг тохирохгүй байна.", en: "The two passwords are not the same." },
  "auth.message.password_failed": { mn: "Хадгалж чадсангүй.", en: "That could not be saved." },
  "auth.message.password_saved": {
    mn: "Хадгаллаа. Нэвтрэх дэлгэц рүү шилжиж байна…",
    en: "Saved. Taking you to the sign-in screen…",
  },
  "auth.message.link_dead": {
    mn: "Энэ холбоос хэрэглэгдсэн эсвэл хугацаа нь дууссан байна. Шинийг хүсэлт гаргана уу.",
    en: "That link has already been used or has expired. Ask for a new one.",
  },
  "auth.message.impersonation_starting": {
    mn: "Байгууллага руу орж байна…",
    en: "Stepping into the organisation…",
  },
  "auth.view.eyebrow": { mn: "ҮНДЭСНИЙ ЦАХИМ ТАНИЛТ", en: "NATIONAL DIGITAL IDENTITY" },
  // The headline highlights its middle phrase, so it is stored in parts
  // rather than as markup inside a translation.
  "auth.view.title_lead": { mn: "Баталгаатай identity.", en: "Verified identity." },
  "auth.view.title_highlight": { mn: "Нэг удаагийн", en: "Sign in" },
  "auth.view.title_tail": { mn: "нэвтрэлт.", en: "once." },
  "auth.view.lede": {
    mn: "eID Mongolia апп дээр хүсэлтийг зөвшөөрөхөд {brand} болон холбогдсон SSO аппууд таны баталгаажсан session-ийг ашиглана.",
    en: "Approve the request in the eID Mongolia app and {brand} — along with every connected SSO app — uses that verified session.",
  },
  "auth.view.point_push": { mn: "Регистрийн дугаараар push хүсэлт", en: "Push request by registration number" },
  "auth.view.point_qr": { mn: "QR болон mobile App2App", en: "QR and mobile App2App" },
  "auth.view.point_rbac": { mn: "Tenant RBAC ба audit хамгаалалт", en: "Tenant RBAC and audit protection" },

  // Нэгдсэн нэвтрэлтийн карт. Толгой мөр нь "хэн асууж байна"-г хэлнэ:
  // authorization хүсэлтээс ирсэн бол тухайн аппын нэр, эс бөгөөс платформ өөрөө.
  "auth.view.platform_name": { mn: "{brand}", en: "{brand}" },
  "auth.signin.asker_note": {
    mn: "{brand} — нэгдсэн нэвтрэлтээр нэвтрэх гэж байна",
    en: "{brand} — signing you in through the unified login",
  },
  "auth.signin.title": { mn: "Нэвтрэх", en: "Sign in" },
  "auth.signin.lede": { mn: "eID Mongolia App-аар нэвтрэх", en: "Sign in with the eID Mongolia app" },
  "auth.signin.or": { mn: "эсвэл", en: "or" },
  "auth.signin.google": { mn: "Google-ээр нэвтрэх", en: "Sign in with Google" },
  "auth.signin.google_soon": { mn: "Google нэвтрэлт удахгүй", en: "Google sign-in is coming soon" },
  "auth.signin.help": { mn: "Тусламж хэрэгтэй юу?", en: "Need help?" },

  "auth.eid.title": { mn: "eID Mongolia", en: "eID Mongolia" },
  "auth.eid.instruction": {
    mn: "РД эсвэл иргэний бүртгэлийн дугаараа оруулна уу. Утсан дээрх eID Mongolia App-д мэдэгдэл ирнэ.",
    en: "Enter your registration or civil ID number. A notification arrives in the eID Mongolia app on your phone.",
  },
  "auth.eid.subtitle": { mn: "Үндэсний цахим үнэмлэхээр баталгаажна", en: "Verified by the national digital ID" },
  "auth.eid.tab_id": { mn: "Регистрийн дугаар", en: "Registration number" },
  "auth.eid.tab_qr": { mn: "QR код", en: "QR code" },
  "auth.eid.reg_number": { mn: "Регистрийн дугаар", en: "Registration number" },
  "auth.eid.reg_number_placeholder": { mn: "АА00112233", en: "AA00112233" },
  "auth.eid.send_request": { mn: "eID апп руу хүсэлт илгээх", en: "Send a request to the eID app" },
  "auth.eid.verification_code": { mn: "Баталгаажуулах код", en: "Verification code" },
  "auth.eid.confirm_hint": { mn: "Апп дээрх кодтой тулгаад зөвшөөрнө үү", en: "Match the code shown in the app, then approve" },
  "auth.eid.footer": {
    mn: "Нууц үг дамжуулахгүй · Баталгаажуулалт eID апп дотор хийгдэнэ",
    en: "No password is sent · Approval happens inside the eID app",
  },

  "auth.action.retry": { mn: "Дахин оролдох", en: "Try again" },
  "auth.action.cancel": { mn: "Цуцлах", en: "Cancel" },
  "auth.action.admin_disclosure": { mn: "Системийн админ нэвтрэлт", en: "System administrator sign-in" },
  "auth.action.admin_sign_in": { mn: "Админаар нэвтрэх", en: "Sign in as administrator" },
  "auth.field.email": { mn: "И-мэйл", en: "Email" },
  "auth.field.password": { mn: "Нууц үг", en: "Password" },
  "auth.field.staff_pin": { mn: "Ажилтны PIN", en: "Staff PIN" },
  "auth.action.app_to_app": { mn: "eID апп-аар шууд нэвтрэх", en: "Open the eID app" },
  "auth.action.staff_sign_in": { mn: "Ээлжийн ажилтнаар нэвтрэх", en: "Sign in as shift staff" },

  "auth.message.starting": { mn: "Хүсэлт үүсгэж байна…", en: "Creating the request…" },
  "auth.message.scan_qr": { mn: "eID Mongolia апп-аар QR кодыг уншуулна уу.", en: "Scan the QR code with the eID Mongolia app." },
  "auth.message.sent_push": { mn: "Таны eID Mongolia апп руу хүсэлт илгээлээ.", en: "A request has been sent to your eID Mongolia app." },
  "auth.message.expires_in": { mn: "Хүсэлтийн хугацаа: {time}", en: "Request expires in {time}" },
  "auth.message.expired": { mn: "Хүсэлтийн хугацаа дууслаа.", en: "The request has expired." },
  "auth.message.refused": { mn: "Та нэвтрэх хүсэлтийг татгалзлаа.", en: "You declined the sign-in request." },
  "auth.message.success": { mn: "Амжилттай. Систем рүү шилжиж байна…", en: "Signed in. Taking you to the platform…" },
  "auth.message.error_link": {
    mn: "eID баталгаажуулалтыг {brand} хэрэглэгчтэй холбож чадсангүй",
    en: "The eID verification could not be linked to a {brand} user",
  },
  "auth.message.error_service": {
    mn: "eID Mongolia үйлчилгээтэй холбогдож чадсангүй",
    en: "Could not reach the eID Mongolia service",
  },
  "auth.message.error_password": { mn: "Нэвтрэх боломжгүй байна", en: "Could not sign in" },

  // Энэ суулгац өөрөө биш, өөр SSO провайдер таньдаг үе. {provider} нь тухайн
  // провайдерийн нэр — орчуулгын мөрөнд үлдээж, дэлгэц дээр орлуулна.
  "auth.sso.checking": { mn: "Нэвтрэх аргыг тодруулж байна…", en: "Working out how to sign you in…" },
  "auth.sso.redirecting": { mn: "{provider} рүү шилжиж байна…", en: "Taking you to {provider}…" },
  "auth.sso.lede": {
    mn: "Энэ суулгац хэн болохыг тань {provider} дээр баталгаажуулна. Тэнд нэвтэрсэн бол дахин асуухгүй.",
    en: "This deployment confirms who you are at {provider}. If you are already signed in there, you will not be asked again.",
  },
  "auth.sso.card_title": { mn: "{provider}-ээр нэвтэрнэ", en: "Sign in through {provider}" },
  "auth.sso.card_body": {
    mn: "Нууц үг энд оруулахгүй. {provider} таныг баталгаажуулаад буцаана.",
    en: "No password is entered here. {provider} confirms you and sends you back.",
  },
  "auth.sso.sign_in": { mn: "{provider}-ээр үргэлжлүүлэх", en: "Continue with {provider}" },
  "auth.sso.error_generic": { mn: "Нэвтрэлт дуусгаж чадсангүй. Дахин оролдоно уу.", en: "The sign-in could not be completed. Please try again." },
  "auth.sso.error_stale": {
    mn: "Нэвтрэх хүсэлтийн хугацаа дууссан байна. Дахин эхлүүлнэ үү.",
    en: "That sign-in attempt has expired. Please start again.",
  },
  "auth.sso.error_unreachable": {
    mn: "Нэвтрэлтийн провайдертай холбогдож чадсангүй.",
    en: "The sign-in provider could not be reached.",
  },
  "auth.sso.error_denied": { mn: "Нэвтрэх хүсэлт зөвшөөрөгдсөнгүй.", en: "The sign-in request was not approved." },
  "auth.sso.error_email_unverified": {
    mn: "Google таны и-мэйл хаягийг баталгаажуулаагүй байна.",
    en: "Google has not verified that email address.",
  },
  "auth.sso.error_domain_not_allowed": {
    mn: "Энэ и-мэйлийн домэйнд нэвтрэх эрх нээгээгүй байна.",
    en: "Sign-in is not open to that email domain.",
  },
  // Анх удаа нэвтэрч буй хүн: бүртгэл нь байхгүй нь **зөв**, тэр яг одоо
  // үүсэх гэж байсан. Тиймээс "админдаа хандана уу" гэж хэлэх нь худал —
  // баталгаажуулалтын дэлгэц рүү хүрч чадаагүй нь л асуудал.
  "auth.sso.error_binding": {
    mn: "Баталгаажуулалтыг эхлүүлж чадсангүй. Түр хүлээгээд дахин оролдоно уу.",
    en: "The verification step could not be started. Please try again in a moment.",
  },
  // Хувийн профайл.
  "profile.loading": { mn: "Ачаалж байна…", en: "Loading…" },
  "profile.link_eid": { mn: "eID-ээ холбох", en: "Link your eID" },
  "profile.link_eid_note": {
    mn: "eID холбосноор Гэрэгэ дугаар тань энэ дансанд бүртгэгдэнэ. Байгууллагууд таны хүсэлтийн хариуг тэр дугаараар тань олж мэдэгддэг тул үүнгүйгээр хариу ирэхгүй.",
    en: "Linking your eID records your Gerege number on this account. Organisations answer a request by that number, so without it their answer has nowhere to arrive.",
  },
  // Гишүүнчлэлгүй хүнд энэ жагсаалт хоосон байх нь ердийн байдал — 00085-аас
  // хойш тэр хүн гэртээ ажилладаг.
  "profile.message.no_organisations": {
    mn: "Та ямар нэг байгууллагад харьяалагдаагүй байна.",
    en: "You do not belong to any organisation.",
  },

  // /me — юу гуйсан, хаана явна. Хүний өөрийнх нь бичлэг тул profile-ийн
  // хажууд: аль аль нь тэр хүнийхэн, байгууллагынх нь биш.
  "me.view.requests_title": { mn: "Миний хүсэлтүүд", en: "My requests" },
  "me.view.requests_subtitle": {
    mn: "Та байгууллагуудад гаргасан хүсэлтүүдийнхээ төлөвийг эндээс харна. Баримт, нотолгоо нь тухайн байгууллагад үлдэнэ.",
    en: "Where the requests you have made to organisations have got to. The documents and evidence stay with the organisation doing the work.",
  },
  "me.message.no_requests": {
    mn: "Одоогоор хүсэлт алга. Байгууллагад хүсэлт гаргахад тэр нь энд гарч ирнэ.",
    en: "No requests yet. Anything you ask an organisation for will appear here.",
  },
  "me.field.code": { mn: "Үйлчилгээ", en: "Service" },
  "me.field.provider": { mn: "Хаана", en: "With" },
  "me.field.status": { mn: "Төлөв", en: "Status" },
  "me.field.answer": { mn: "Хариу", en: "Answer" },
  "me.field.updated": { mn: "Шинэчлэгдсэн", en: "Updated" },

  // Байгууллагад орох хүсэлт. Иргэн өөрөө эхлүүлнэ — 00089 хүртэл орох ганц
  // зам нь урилга байсан.
  "me.view.ask_title": { mn: "Байгууллагад нэгдэх", en: "Join an organisation" },
  "me.message.joined": {
    mn: "{name}-д нэгдлээ.",
    en: "You are now a member of {name}.",
  },
  "me.action.open_workspace": { mn: "Тийш нь орох", en: "Go there" },
  "me.view.ask_subtitle": {
    mn: "Байгууллагын нэрийг (хаяган дахь богино нэр) бичээд хүсэлт илгээнэ. Тэдний админ хариулна.",
    en: "Give the organisation's short name — the one in its address — and ask. Their administrator answers.",
  },
  "me.field.slug_placeholder": { mn: "байгууллагын богино нэр", en: "organisation short name" },
  "me.field.message_placeholder": { mn: "Хэн болохоо танилцуулна уу (заавал биш)", en: "Say who you are (optional)" },
  "me.action.ask": { mn: "Хүсэлт илгээх", en: "Ask" },
  // Лавлах: хэнд хандахаа мэдэхгүй хүнд зориулав. Богино нэрийг орлуулахгүй —
  // олсон нэрийг дээрх талбарт хийж өгнө.
  "me.view.lookup_hint": {
    mn: "Хэнд хандахаа мэдэхгүй бол үйлчилгээгээрээ хайна уу.",
    en: "If you do not know who to ask, look the service up.",
  },
  "me.field.lookup_placeholder": { mn: "үйлчилгээний нэр эсвэл код", en: "service name or code" },
  "me.action.lookup": { mn: "Хайх", en: "Look up" },
  "me.action.choose": { mn: "Сонгох", en: "Choose" },
  "me.message.no_providers": {
    mn: "Ийм үйлчилгээг нийтэлсэн байгууллага алга.",
    en: "No organisation has published that service.",
  },
  "profile.stat.organisations": { mn: "Байгууллага", en: "Organisations" },
  "profile.stat.identities": { mn: "Таних тэмдэг", en: "Identities" },
  "profile.stat.sessions": { mn: "Нээлттэй session", en: "Open sessions" },
  "profile.stat.since": { mn: "Бүртгүүлсэн", en: "Member since" },
  // Хуудсыг нэрлэх нэр — цэс болон толгой хэсгийн товч хоёулаа үүнийг хэлнэ.
  // "Таны таних тэмдгүүд" нь доторх нэг л хэсгийн гарчиг: тэрийг цэсэнд тавихад
  // хүн статистик, байгууллага, session-оо тэндээс олохоо мэдэхгүй өнгөрдөг.
  "profile.title": { mn: "Миний профайл", en: "My profile" },
  "profile.identities": { mn: "Таны таних тэмдгүүд", en: "Your identities" },
  "profile.identities_lede": {
    mn: "Эдгээрээр та энэ платформд нэвтэрдэг. Провайдер бүрийн юу хэлснийг доор нь харж болно.",
    en: "These are how you sign in here. What each provider said is below it.",
  },
  "profile.linked_provider": { mn: "Холбогдсон {provider}", en: "{provider}, connected" },
  "profile.verified": { mn: "Баталгаажсан", en: "Verified" },
  "profile.link_error.session_expired": {
    mn: "Нэвтрэлт дуусжээ. Дахин нэвтэрч байж холбоно уу.",
    en: "Your session ended. Sign in again, then connect.",
  },
  "profile.link_error.already_linked_elsewhere": {
    mn: "Энэ Google хаяг өөр бүртгэлд холбогдсон байна. Тэндээс нь салгаж байж энд холбоно.",
    en: "That Google account belongs to another record here. Unlink it there first.",
  },
  "profile.link_error.google_not_configured": {
    mn: "Энэ платформ дээр Google-ээр нэвтрэх тохируулаагүй байна.",
    en: "Google sign-in is not configured on this deployment.",
  },
  "profile.link_error.sso_required": {
    mn: "Энэ платформ нь өөрийн SSO провайдераар нэвтэрдэг тул Google холбох боломжгүй.",
    en: "This deployment signs in through its SSO provider, so Google cannot be connected.",
  },
  "profile.link_error.provider_unreachable": {
    mn: "Google-тэй холбогдож чадсангүй. Дараа дахин оролдоно уу.",
    en: "Could not reach Google. Try again shortly.",
  },
  "profile.link_error.email_unverified": {
    mn: "Google тэр хаягийг баталгаажуулаагүй байна.",
    en: "Google has not verified that address.",
  },
  "profile.link_error.domain_not_allowed": {
    mn: "Тэр хаягийн домэйныг энэ платформ зөвшөөрдөггүй.",
    en: "That address's domain is not allowed here.",
  },
  "profile.link_error.unknown": {
    mn: "Холбож чадсангүй ({code}).",
    en: "Could not connect ({code}).",
  },
  "profile.link_google": { mn: "Google хаягаа холбох", en: "Connect your Google account" },
  "profile.link_google_note": {
    mn: "Та хэн болохоо аль хэдийн баталсан тул зөвхөн Google дээрээ зөвшөөрөл өгнө. Дараа нь Google-ээр шууд нэвтэрч болно.",
    en: "You have already proved who you are, so this only asks Google. Afterwards you can sign in with Google directly.",
  },
  "profile.unlink": { mn: "Салгах", en: "Unlink" },
  "profile.unlinking": { mn: "Салгаж байна…", en: "Unlinking…" },
  "profile.unlink_confirm": {
    mn: "{provider}-ийг салгах уу? Дараа нь түүгээр нэвтрэхэд эхнийх шигээ eID-ээр баталгаажуулна.",
    en: "Unlink {provider}? Signing in with it again will ask for eID, as the first time did.",
  },
  "profile.linked_at": { mn: "Холбогдсон", en: "Linked" },
  "profile.last_seen": { mn: "Сүүлд", en: "Last used" },
  "profile.show_claims": { mn: "{count} мэдээллийг харах", en: "Show {count} fields" },
  "profile.hide_claims": { mn: "Нуух", en: "Hide" },
  "profile.no_identities": { mn: "Холбогдсон таних тэмдэг алга.", en: "No identities are linked yet." },
  "profile.organisations": { mn: "Харьяалагдах байгууллага", en: "Your organisations" },

  // Гадны провайдераар анх ирсэн хүнийг eID-ээр баталгаажуулах урсгал.
  "auth.bind.title": { mn: "Бүртгэл үүсгэх", en: "Set up your account" },
  "auth.bind.subtitle": {
    mn: "{provider}-ээр таныг баталгаажууллаа. Үлдсэн нэг алхам.",
    en: "{provider} has confirmed you. One step remains.",
  },
  "auth.bind.from_provider": { mn: "{provider}-ээс ирсэн", en: "From {provider}" },
  "auth.bind.from_eid": { mn: "eID Mongolia-гаас авах", en: "From eID Mongolia" },
  "auth.bind.consent_body": {
    mn: "Эдгээр мэдээллийг {brand}-д хадгалж, таны профайл дээр харуулна. Хэн болохыг тань eID баталгаажуулна — энэ нь зөвхөн нэг удаа.",
    en: "This is stored in {brand} and shown on your profile. eID confirms who you are — once only.",
  },
  "auth.bind.agree": { mn: "Зөвшөөрч, үргэлжлүүлэх", en: "Agree and continue" },
  "auth.bind.verify_title": { mn: "eID-ээр баталгаажуулах", en: "Confirm with eID" },
  "auth.bind.verify_lede": {
    mn: "Регистрийн дугаараа оруулах эсвэл QR уншуулна уу. Дараагийн удаад шууд нэвтэрнэ.",
    en: "Enter your registration number or scan the QR. Next time you will go straight in.",
  },
  "auth.bind.expired": {
    mn: "Энэ хүсэлтийн хугацаа дууссан байна. Дахин нэвтэрнэ үү.",
    en: "That attempt has expired. Please sign in again.",
  },

  "auth.sso.error_no_account": {
    mn: "Таныг баталгаажууллаа, гэхдээ энэ систем дээр танд бүртгэл алга. Байгууллагынхаа админд хандана уу.",
    en: "You are verified, but this deployment has no account for you. Ask your administrator.",
  },
} as const;
