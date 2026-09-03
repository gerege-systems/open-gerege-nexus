/**
 * esign — PDF digital signing.
 *
 * Two rails share these screens: eID Mongolia qualified remote signing (the
 * citizen approves with PIN2 on their own phone) and the Gerege eSign HSM
 * (certificate proof plus a drawn signature, stamped server-side).
 */
export const esign = {
  "esign.view.title": { mn: "PDF тоон гарын үсэг", en: "PDF e-signature" },
  "esign.view.subtitle": {
    mn: "PDF баримт бичгийг eID Mongolia (PIN2) эсвэл Gerege eSign HSM-ээр баталгаажуулах",
    en: "Sign PDF documents with eID Mongolia (PIN2) or the Gerege eSign HSM",
  },
  "esign.view.tab_sign": { mn: "Гарын үсэг зурах", en: "Sign" },
  "esign.view.tab_documents": { mn: "Баримт бичиг", en: "Documents" },
  "esign.view.upload_title": { mn: "PDF баримт бичиг оруулах", en: "Upload a PDF document" },
  "esign.view.sign_title": { mn: "Тоон гарын үсгээр баталгаажуулах", en: "Sign with a digital signature" },
  "esign.view.sign_placement": {
    mn: "{title} — гарын үсэг сүүлийн хуудсанд ({page}-р хуудас) байрлана",
    en: "{title} — the signature is placed on the last page (page {page})",
  },
  "esign.view.step_certificate": { mn: "1. Сертификат шалгах", en: "1. Check the certificate" },
  "esign.view.step_signature": { mn: "2. Гарын үсгээ зурна уу", en: "2. Draw your signature" },
  "esign.view.pick_document": { mn: "Баримт сонгох", en: "Choose a document" },
  "esign.view.confirm_on_phone": { mn: "Утсаараа баталгаажуулна уу", en: "Confirm on your phone" },

  // ─── Signature log ─────────────────────────────────────────────────────────
  "esign.view.logs_title": { mn: "Гарын үсгийн лог", en: "Signature log" },
  "esign.view.logs_subtitle": {
    mn: "Сертификат шалгалт, гарын үсэг, таталт бүрийн бүртгэл — амжилтгүй оролдлого мөн адил",
    en: "Every certificate check, signature and download — failures included",
  },

  // ─── Batch signing ─────────────────────────────────────────────────────────
  "esign.view.batch_title": { mn: "Багц баталгаажуулалт", en: "Batch signing" },
  "esign.view.batch_subtitle": {
    mn: "Олон баримтыг нэг дараалалд зурах. Баримт бүрийг утсан дээрээ PIN2-оор тус тусад нь баталгаажуулна.",
    en: "Sign many documents in one run. Each document is still confirmed with PIN2 on your phone.",
  },
  "esign.view.batch_detail_subtitle": {
    mn: "{total}-аас {signed} баримт баталгаажсан",
    en: "{signed} of {total} documents signed",
  },
  "esign.view.batch_documents": { mn: "Багцын баримтууд", en: "Documents in this batch" },
  "esign.view.new_batch_title": { mn: "Шинэ багц үүсгэх", en: "New batch" },

  // ─── Settings ──────────────────────────────────────────────────────────────
  "esign.view.placement_title": { mn: "Тамганы байрлал", en: "Stamp placement" },
  "esign.view.placement_subtitle": {
    mn: "Гарын үсгийн тамга хуудсан дээр хаана буухыг тохируулна",
    en: "Where the signature stamp lands on the page",
  },
  "esign.view.placement_form": { mn: "Байрлалын тохиргоо", en: "Placement" },
  "esign.view.placement_preview": { mn: "Урьдчилан харах", en: "Preview" },

  "esign.view.hsm_title": { mn: "HSM холболт", en: "HSM connection" },
  "esign.view.hsm_subtitle": {
    mn: "Gerege eSign HSM үйлчилгээний холболтын төлөв",
    en: "The state of the Gerege eSign HSM connection",
  },
  "esign.view.hsm_connection": { mn: "Холболтын мэдээлэл", en: "Connection" },
  "esign.view.hsm_last_probe": { mn: "Сүүлийн шалгалт", en: "Last connection test" },

  "esign.view.policies_title": { mn: "Гарын үсгийн бодлого", en: "Signing policy" },
  "esign.view.policies_subtitle": {
    mn: "Энэ байгууллагад ямар гарын үсэг хүчинтэй болохыг тодорхойлно",
    en: "What counts as a valid signature in this organisation",
  },
  "esign.view.policy_rails": { mn: "Гарын үсгийн суваг", en: "Signing rails" },
  "esign.view.policy_rules": { mn: "Дүрэм", en: "Rules" },

  // ─── Fields ────────────────────────────────────────────────────────────────
  "esign.field.document": { mn: "Баримт бичиг", en: "Document" },
  "esign.field.title": { mn: "Баримт бичгийн нэр", en: "Document title" },
  "esign.field.title_placeholder": { mn: "ж: Хамтран ажиллах гэрээ 2026", en: "e.g. Partnership agreement 2026" },
  "esign.field.file": { mn: "PDF файл (макс {max}MB)", en: "PDF file ({max}MB max)" },
  "esign.field.pages": { mn: "Хуудас", en: "Pages" },
  "esign.field.signer": { mn: "Гарын үсэг зурсан", en: "Signed by" },
  "esign.field.phone": { mn: "Утасны дугаар", en: "Phone number" },
  "esign.field.civil_id": { mn: "Регистр / Civil ID", en: "Registration / Civil ID" },
  "esign.field.sign_as": { mn: "Хэний нэрийн өмнөөс", en: "Sign as" },
  "esign.field.sign_as_self": { mn: "Өөрийн нэрийн өмнөөс", en: "Myself" },
  "esign.field.verification_code": { mn: "Баталгаажуулах код", en: "Verification code" },
  "esign.field.provider": { mn: "Суваг", en: "Rail" },
  "esign.field.action": { mn: "Үйлдэл", en: "Action" },
  "esign.field.outcome": { mn: "Үр дүн", en: "Outcome" },
  "esign.field.detail": { mn: "Тайлбар", en: "Detail" },
  "esign.field.from": { mn: "Эхлэх огноо", en: "From" },
  "esign.field.to": { mn: "Дуусах огноо", en: "To" },
  "esign.field.search_placeholder": { mn: "Нэр, регистр, баримтаар хайх", en: "Search by name, ID or document" },

  "esign.field.batch_name": { mn: "Багцын нэр", en: "Batch name" },
  "esign.field.batch_name_placeholder": { mn: "ж: 2026 оны 8-р сарын гэрээнүүд", en: "e.g. August 2026 contracts" },
  "esign.field.batch_documents": { mn: "Баримт сонгох ({count} сонгосон)", en: "Documents ({count} selected)" },
  "esign.field.progress": { mn: "Явц", en: "Progress" },

  "esign.field.x": { mn: "X (зүүнээс)", en: "X (from the left)" },
  "esign.field.x_hint": { mn: "Цэгээр, А4 өргөн 595", en: "In points; A4 is 595 wide" },
  "esign.field.y": { mn: "Y (дээрээс)", en: "Y (from the top)" },
  "esign.field.y_hint": { mn: "eSign дээд-зүүн буланг тэгээс тоолно", en: "eSign measures from the top-left corner" },
  "esign.field.width": { mn: "Өргөн", en: "Width" },
  "esign.field.height": { mn: "Өндөр", en: "Height" },
  "esign.field.page_number": { mn: "Хуудасны дугаар", en: "Page number" },
  "esign.field.page_number_hint": { mn: "0 бол сүүлийн хуудсанд байрлана", en: "0 places it on the last page" },
  "esign.field.caption": { mn: "Тамганы бичвэр", en: "Stamp caption" },

  "esign.field.login_url": { mn: "Нэвтрэх URL", en: "Login URL" },
  "esign.field.sign_url": { mn: "Гарын үсгийн URL", en: "Signing URL" },
  "esign.field.mode": { mn: "Горим", en: "Mode" },
  "esign.field.token": { mn: "Токен", en: "Token" },
  "esign.field.latency": { mn: "Хариу өгөх хугацаа", en: "Latency" },
  "esign.field.checked_at": { mn: "Шалгасан огноо", en: "Checked at" },
  "esign.field.checked_by": { mn: "Шалгасан хэрэглэгч", en: "Checked by" },

  "esign.field.default_provider": { mn: "Үндсэн суваг", en: "Default rail" },
  "esign.field.default_provider_hint": {
    mn: "Шинэ гарын үсэг өөрөөр заагаагүй бол энэ сувгаар зурагдана",
    en: "A new signature uses this rail unless it says otherwise",
  },
  "esign.field.require_eid": { mn: "Зөвхөн eID Mongolia шаардах", en: "Require eID Mongolia" },
  "esign.field.require_eid_hint": {
    mn: "Хуулийн хүчин төгөлдөр (qualified) гарын үсгийг зөвхөн eID гаргадаг. Асаавал HSM суваг бүрэн хаагдана.",
    en: "Only eID produces a qualified signature. Turning this on disables the HSM rail everywhere.",
  },
  "esign.field.min_certificate_level": { mn: "Гэрчилгээний доод түвшин", en: "Minimum certificate level" },
  "esign.field.min_certificate_level_hint": {
    mn: "QUALIFIED нь «тоон гарын үсэг»-ийн хуулийн шаардлага",
    en: "QUALIFIED is what the law means by a digital signature",
  },
  "esign.field.allow_on_behalf_of": { mn: "Байгууллагын нэрийн өмнөөс зурахыг зөвшөөрөх", en: "Allow signing for an organisation" },
  "esign.field.allow_on_behalf_of_hint": {
    mn: "Гарын үсэг нь иргэний PIN2 гэрчилгээгээр зурагдана; eID төлөөллийн эрхийг бүртгэлээс шалгана.",
    en: "The signature still uses the citizen's PIN2 certificate; eID checks the representation right against the registry.",
  },
  "esign.field.allow_self_sign": { mn: "Өөрийн оруулсан баримтад зурахыг зөвшөөрөх", en: "Allow signing your own upload" },
  "esign.field.allow_self_sign_hint": {
    mn: "Үүргийн зааглалт шаардлагатай бол унтраана",
    en: "Turn off where separation of duties is required",
  },
  "esign.field.retention_days": { mn: "Хадгалах хугацаа (хоног)", en: "Retention (days)" },
  "esign.field.retention_days_hint": { mn: "0 бол хугацаагүй хадгална", en: "0 keeps documents forever" },
  "esign.field.max_upload_mb": { mn: "Файлын дээд хэмжээ (MB)", en: "Maximum upload (MB)" },
  "esign.field.max_upload_mb_hint": { mn: "Платформын дээд хэмжээ 25MB", en: "The platform ceiling is 25MB" },

  // ─── States ────────────────────────────────────────────────────────────────
  "esign.state.signed": { mn: "БАТАЛГААЖСАН", en: "SIGNED" },
  "esign.state.pending": { mn: "ХҮЛЭЭГДЭЖ БУЙ", en: "PENDING" },
  "esign.state.mock": { mn: "Туршилтын (mock)", en: "Mock" },
  "esign.state.live": { mn: "Бодит", en: "Live" },
  "esign.state.token_present": { mn: "Тохируулсан", en: "Configured" },
  "esign.state.token_missing": { mn: "Тохируулаагүй", en: "Not configured" },

  "esign.batch.draft": { mn: "Ноорог", en: "Draft" },
  "esign.batch.running": { mn: "Явагдаж буй", en: "Running" },
  "esign.batch.completed": { mn: "Дууссан", en: "Completed" },
  "esign.batch.failed": { mn: "Амжилтгүй", en: "Failed" },
  "esign.batch.cancelled": { mn: "Цуцалсан", en: "Cancelled" },

  "esign.item.pending": { mn: "Хүлээгдэж буй", en: "Pending" },
  "esign.item.running": { mn: "Явагдаж буй", en: "Running" },
  "esign.item.signed": { mn: "Баталгаажсан", en: "Signed" },
  "esign.item.failed": { mn: "Амжилтгүй", en: "Failed" },
  "esign.item.skipped": { mn: "Алгассан", en: "Skipped" },

  "esign.outcome.ok": { mn: "Амжилттай", en: "OK" },
  "esign.outcome.failed": { mn: "Амжилтгүй", en: "Failed" },
  "esign.outcome.rejected": { mn: "Татгалзсан", en: "Rejected" },
  "esign.outcome.expired": { mn: "Хугацаа дууссан", en: "Expired" },
  "esign.outcome.cancelled": { mn: "Цуцалсан", en: "Cancelled" },
  "esign.outcome.unverified": { mn: "Баталгаажаагүй", en: "Unverified" },

  "esign.action_type.sign": { mn: "Гарын үсэг", en: "Signature" },
  "esign.action_type.sign_start": { mn: "Гарын үсэг эхлүүлсэн", en: "Signature started" },
  "esign.action_type.batch_sign": { mn: "Багц гарын үсэг", en: "Batch signature" },
  "esign.action_type.cert_check": { mn: "Сертификат шалгалт", en: "Certificate check" },
  "esign.action_type.download": { mn: "Татсан", en: "Download" },

  // ─── Actions ───────────────────────────────────────────────────────────────
  "esign.action.upload": { mn: "PDF оруулах", en: "Upload a PDF" },
  "esign.action.submit_upload": { mn: "Оруулах", en: "Upload" },
  "esign.action.pick_pdf": { mn: "PDF файл сонгох", en: "Choose a PDF" },
  "esign.action.sign_eid": { mn: "eID", en: "eID" },
  "esign.action.sign_hsm": { mn: "HSM", en: "HSM" },
  "esign.action.sign_another": { mn: "Шинээр зурах", en: "Sign another" },
  "esign.action.check_certificate": { mn: "Сертификат шалгах", en: "Check the certificate" },
  "esign.action.clear": { mn: "Арилгах", en: "Clear" },
  "esign.action.archive": { mn: "Архивлах", en: "Archive" },
  "esign.action.download_original": { mn: "Эх хувь татах", en: "Download the original" },
  "esign.action.download_signed": { mn: "Баталгаажсан хувь татах", en: "Download the signed copy" },
  "esign.action.original_short": { mn: "Эх", en: "Original" },
  "esign.action.signed_short": { mn: "Баталгаажсан", en: "Signed" },
  "esign.action.export_csv": { mn: "CSV татах", en: "Export CSV" },
  "esign.action.new_batch": { mn: "Шинэ багц", en: "New batch" },
  "esign.action.create_batch": { mn: "Багц үүсгэх", en: "Create batch" },
  "esign.action.run_batch": { mn: "Багцыг эхлүүлэх", en: "Run the batch" },
  "esign.action.back_to_batches": { mn: "Багцууд руу буцах", en: "Back to batches" },
  "esign.action.test_connection": { mn: "Холболт шалгах", en: "Test the connection" },
  "esign.action.reset_default": { mn: "Анхны утга", en: "Reset to default" },

  // ─── Messages ──────────────────────────────────────────────────────────────
  "esign.message.loading": { mn: "Баримт бичгүүдийг ачаалж байна...", en: "Loading documents..." },
  "esign.message.empty": {
    mn: "Баримт бичиг алга. “PDF оруулах” товчоор эхлүүлнэ үү.",
    en: "No documents yet. Start with “Upload a PDF”.",
  },
  "esign.message.uploading": { mn: "Илгээж байна...", en: "Uploading..." },
  "esign.message.uploaded": { mn: "Баримт бичиг орууллаа.", en: "The document was uploaded." },
  "esign.message.checking": { mn: "Шалгаж байна...", en: "Checking..." },
  "esign.message.testing": { mn: "Шалгаж байна...", en: "Testing..." },
  "esign.message.signing": { mn: "Баталгаажуулж байна...", en: "Signing..." },
  "esign.message.signed": { mn: "Гарын үсэг амжилттай зурлаа.", en: "The document was signed." },
  "esign.message.archived": { mn: "Баримт бичгийг архивлалаа.", en: "The document was archived." },
  "esign.message.certificate_valid": { mn: "Сертификат хүчинтэй: {name}", en: "Certificate valid: {name}" },
  "esign.message.title_optional": {
    mn: "Хоосон орхивол файлын нэрийг ашиглана",
    en: "Left empty, the filename is used",
  },
  "esign.message.confirm_archive": {
    mn: "«{title}»-г архивлах уу? Баталгаажсан хувь устахгүй, зөвхөн жагсаалтаас нуугдана.",
    en: "Archive “{title}”? The signed copy is kept — it is only hidden from the list.",
  },

  "esign.message.pdf_only": { mn: "Зөвхөн PDF, дээд тал нь 25 MB.", en: "PDF only, up to 25 MB." },
  "esign.message.not_a_pdf": { mn: "PDF файл оруулна уу.", en: "Please choose a PDF file." },
  "esign.message.file_too_large": {
    mn: "Файл хэт том ({size} MB) — дээд тал нь 25 MB.",
    en: "That file is too large ({size} MB) — the limit is 25 MB.",
  },
  "esign.message.sign_as_hint": {
    mn: "Өөрийн eID гэрчилгээгээр зурна",
    en: "Signed with your own eID certificate",
  },
  "esign.message.on_behalf_hint": {
    mn: "Гарын үсэг таны PIN2 гэрчилгээгээр зурагдаж, байгууллагын төлөөллийг тэмдэглэнэ",
    en: "The signature uses your PIN2 certificate and records the organisation you represent",
  },
  "esign.message.on_behalf_of": { mn: "{org}-ийн нэрийн өмнөөс", en: "on behalf of {org}" },
  "esign.message.no_organizations": {
    mn: "Та ямар нэг байгууллага төлөөлдөггүй байна",
    en: "You do not represent any organisation",
  },
  "esign.message.pin2_instruction": {
    mn: "eID Mongolia аппдаа энэ кодыг шалгаад PIN2-оороо гарын үсэг зурна уу.",
    en: "Check this code in your eID Mongolia app, then sign with PIN2.",
  },
  // ─── Backend error codes ───────────────────────────────────────────────────
  // The API answers with a machine code and an English message. The code is
  // what the screens branch on, so it is also what gets translated — otherwise
  // a Mongolian interface shows an English sentence at exactly the moment
  // something has gone wrong.
  "esign.error.NO_SIGNER_IDENTITY": {
    mn: "Энэ бүртгэл eID Mongolia-тай холбогдоогүй байна. Регистрийн дугаараа оруулна уу.",
    en: "This account is not linked to eID Mongolia. Enter your registration number.",
  },
  "esign.error.SIGNER_NOT_ENROLLED": {
    mn: "Энэ иргэн eID Mongolia-д гарын үсгийн гэрчилгээ бүртгүүлээгүй байна.",
    en: "This citizen has no signing certificate registered with eID Mongolia.",
  },
  "esign.error.EID_RP_REJECTED": {
    mn: "Энэ систем eID Mongolia-д гарын үсэг зурах эрхгүй байна. Админтай холбогдоно уу.",
    en: "This deployment is not authorised to sign with eID Mongolia. Contact your administrator.",
  },
  "esign.error.EID_NOT_CONFIGURED": {
    mn: "eID Mongolia гарын үсэг энэ системд тохируулагдаагүй байна.",
    en: "eID Mongolia signing is not configured on this deployment.",
  },
  "esign.error.EID_UNAVAILABLE": {
    mn: "eID Mongolia түр хариу өгөхгүй байна. Дахин оролдоно уу.",
    en: "eID Mongolia is not responding. Please try again.",
  },
  "esign.error.FORBIDDEN": {
    mn: "Танд энэ үйлдлийг хийх эрх алга.",
    en: "You do not have permission to do this.",
  },
  "esign.error.PAYLOAD_TOO_LARGE": { mn: "Файл хэт том байна.", en: "That file is too large." },
  "esign.error.NOT_A_PDF": { mn: "Энэ файл PDF биш байна.", en: "That file is not a PDF." },
  "esign.error.TRUNCATED_PDF": {
    mn: "PDF файл гэмтсэн эсвэл дутуу байна.",
    en: "The PDF appears to be truncated or corrupt.",
  },
  "esign.error.INVALID_SIGNER": {
    mn: "Регистрийн дугаар буруу байна.",
    en: "That registration number is not valid.",
  },
  "esign.error.ALREADY_SIGNED": {
    mn: "Энэ баримт аль хэдийн баталгаажсан байна.",
    en: "This document is already signed.",
  },

  // ─── Signer identity ───────────────────────────────────────────────────────
  "esign.view.signer_identity": { mn: "Гарын үсэг зурах хүн", en: "Who is signing" },
  "esign.field.signer_id": {
    mn: "Регистрийн дугаар эсвэл иргэний дугаар",
    en: "Registration number or civil ID",
  },
  "esign.field.signer_id_placeholder": { mn: "ж: УА00112233", en: "e.g. УА00112233" },
  "esign.message.signer_id_hint": {
    mn: "eID Mongolia-д бүртгэлтэй регистрийн дугаар (УА00112233) эсвэл иргэний дугаараа (111949212017) оруулна уу — баталгаажуулах хүсэлт тухайн хүний утас руу очно.",
    en: "Enter the registration number (УА00112233) or civil ID (111949212017) registered with eID Mongolia — the approval request goes to that person's phone.",
  },
  "esign.message.signer_id_link_hint": {
    mn: "eID-ээр нэвтэрвэл дараагийн удаа автоматаар танигдана.",
    en: "Sign in with eID and it will be recognised automatically next time.",
  },
  "esign.action.continue_signing": { mn: "Үргэлжлүүлэх", en: "Continue" },

  "esign.message.sign_success": { mn: "Гарын үсэг амжилттай зурлаа", en: "The document is signed" },
  "esign.message.sign_expired": {
    mn: "Хугацаа дууслаа. Дахин эхлүүлнэ үү.",
    en: "The request timed out. Please start again.",
  },
  "esign.message.sign_rejected": {
    mn: "Гарын үсэг зурахаас татгалзлаа.",
    en: "The signature was declined.",
  },
  "esign.message.sign_failed": {
    mn: "Гарын үсэг зурж чадсангүй.",
    en: "The document could not be signed.",
  },
  "esign.message.error_download_short": { mn: "Татаж чадсангүй.", en: "The download failed." },
  "esign.message.hsm_disabled": {
    mn: "Энэ байгууллага зөвхөн eID Mongolia гарын үсэг хүлээн авдаг",
    en: "This organisation accepts only eID Mongolia signatures",
  },

  "esign.message.logs_empty": { mn: "Тохирох бүртгэл олдсонгүй.", en: "No matching log entries." },

  "esign.message.batch_empty": {
    mn: "Багц алга. “Шинэ багц” товчоор эхлүүлнэ үү.",
    en: "No batches yet. Start with “New batch”.",
  },
  "esign.message.batch_running": { mn: "Явагдаж байна...", en: "Running..." },
  "esign.message.no_pending_documents": {
    mn: "Баталгаажуулаагүй баримт алга.",
    en: "There are no unsigned documents.",
  },

  "esign.message.placement_saved": { mn: "Байрлалыг хадгаллаа.", en: "The placement was saved." },
  "esign.message.placement_off_page": {
    mn: "Тамга А4 хуудаснаас гарч байна. Хэмжээ эсвэл байрлалаа тохируулна уу.",
    en: "The stamp falls outside an A4 page. Adjust its size or position.",
  },
  "esign.message.placement_preview_hint": {
    mn: "А4 хуудас ({width}×{height} цэг), дээд-зүүн буланг тэгээс тоолов",
    en: "An A4 page ({width}×{height} points), measured from the top-left corner",
  },

  "esign.message.hsm_mock_mode": {
    mn: "HSM туршилтын горимд байна — гарын үсэг бодит HSM-ээр зурагдахгүй.",
    en: "The HSM is in mock mode — no live hardware module is contacted.",
  },
  "esign.message.hsm_no_token": {
    mn: "ESIGN_TOKEN тохируулаагүй тул бодит HSM холбогдохгүй.",
    en: "ESIGN_TOKEN is unset, so the live HSM cannot be reached.",
  },
  "esign.message.hsm_env_managed": {
    mn: "Эдгээр утгыг серверийн орчны хувьсагчаар (ESIGN_*) удирдана. Токеныг браузерт хэзээ ч буцаахгүй.",
    en: "These values come from the server's environment (ESIGN_*). The token is never returned to a browser.",
  },
  "esign.message.no_probe_yet": {
    mn: "Холболтыг хараахан шалгаагүй байна.",
    en: "The connection has not been tested yet.",
  },

  "esign.message.policy_saved": { mn: "Бодлогыг хадгаллаа.", en: "The policy was saved." },

} as const;
