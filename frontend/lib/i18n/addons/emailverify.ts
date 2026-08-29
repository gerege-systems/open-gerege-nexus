/**
 * emailverify — Proving an address. The mail is sent by the hosted
 * verification service; this screen shows what this platform asked for.
 */
export const emailverify = {
  "emailverify.view.title": { mn: "И-мэйл баталгаажуулалт", en: "Email verification" },
  "emailverify.view.subtitle": {
    mn: "Хаяг эзэмшлийг батлах нэгдсэн урсгал — платформын бүх апп үүнийг ашиглана",
    en: "One address-proving flow, shared by every app on the platform",
  },
  "emailverify.view.service_title": { mn: "Илгээх үйлчилгээ", en: "Sending service" },
  "emailverify.view.recent_title": { mn: "Сүүлийн баталгаажуулалтууд", en: "Recent verifications" },
  "emailverify.view.usage_title": { mn: "Хэрхэн ашиглах вэ", en: "How to use it" },
  "emailverify.view.test_title": { mn: "Туршилтын илгээлт", en: "Test send" },

  "emailverify.stat.total": { mn: "Нийт", en: "Total" },
  "emailverify.stat.verified": { mn: "Баталгаажсан", en: "Verified" },
  "emailverify.stat.pending": { mn: "Хүлээгдэж буй", en: "Pending" },
  "emailverify.stat.expired": { mn: "Хугацаа дууссан", en: "Expired" },
  "emailverify.stat.last_24h": { mn: "Сүүлийн 24 цаг", en: "Last 24 hours" },
  "emailverify.field.email": { mn: "Хаяг", en: "Address" },
  "emailverify.stat.verified_pct": { mn: "Баталгаажсан хувь", en: "Verified rate" },

  "emailverify.field.source": { mn: "Хүссэн", en: "Requested by" },
  "emailverify.field.purpose": { mn: "Зорилго", en: "Purpose" },
  "emailverify.field.provider": { mn: "Үйлчилгээ", en: "Service" },
  "emailverify.field.return_url": { mn: "Хариу хүлээн авах хаяг", en: "Return address" },

  "emailverify.state.pending": { mn: "Хүлээгдэж буй", en: "Pending" },
  "emailverify.state.verified": { mn: "Баталгаажсан", en: "Verified" },
  "emailverify.state.expired": { mn: "Хугацаа дууссан", en: "Expired" },

  "emailverify.action.send_test": { mn: "Туршилт илгээх", en: "Send test" },
  "emailverify.action.open_admin": { mn: "Түлхүүр удирдах", en: "Manage keys" },

  "emailverify.message.loading": { mn: "Ачаалж байна...", en: "Loading…" },
  "emailverify.message.no_verifications": {
    mn: "Одоогоор баталгаажуулалт байхгүй байна.",
    en: "Nothing has been requested yet.",
  },
  "emailverify.message.reachable": { mn: "Үйлчилгээ хэвийн ажиллаж байна", en: "The service is responding" },
  "emailverify.message.unreachable": {
    mn: "Үйлчилгээтэй холбогдож чадсангүй: {reason}",
    en: "The service could not be reached: {reason}",
  },
  // The key is a server-side secret and never reaches this screen — so the
  // screen can only say whether one is present, which is what an administrator
  // seeing nothing arrive needs to know first.
  "emailverify.message.not_configured": {
    mn: "EMAIL_VERIFY_API_KEY тохируулаагүй тул баталгаажуулах захидал илгээгдэхгүй. Түлхүүрийг серверийн орчны хувьсагчид хадгална — хөтөч рүү хэзээ ч өгөхгүй.",
    en: "EMAIL_VERIFY_API_KEY is not set, so no verification mail can be requested. The key belongs in a server-side environment variable and never reaches a browser.",
  },
  "emailverify.message.usage": {
    mn: "Захидлыг тус үйлчилгээ илгээнэ. Хэрэглэгч холбоос дээр дарахад энэ платформ руу буцаж ирж, баталгаажуулалт бүртгэгдээд, дараа нь таны заасан хаяг руу шилжинэ. Буцах утга нэг л удаа ажиллана.",
    en: "The service sends the mail. When the recipient follows the link they come back to this platform, the verification is recorded, and they are sent on to the destination you named. The return works exactly once.",
  },
  "emailverify.message.in_app_usage": {
    mn: "Апп модулиуд үүнийг Go дуудлагаар шууд ашиглана: emailverify.Service.Send.",
    en: "App modules call the same service in process, through emailverify.Service.Send.",
  },
  // Worth saying on the screen rather than only in the code: an administrator
  // reading "pending" should know what it does and does not mean.
  "emailverify.message.no_webhook_note": {
    mn: "Үйлчилгээнээс серверийн шууд мэдэгдэл (webhook) одоогоор алга. Тиймээс хэрэглэгч буцаж ирсэн тохиолдолд л баталгаажсан гэж тэмдэглэнэ — захидлыг өөр төхөөрөмж дээр нээгээд буцаж ирээгүй бол энд «хүлээгдэж буй» хэвээр харагдана.",
    en: "The service has no webhook yet, so a verification is recorded only when the person comes back here. Someone who confirms on another device and never returns stays “pending” on this screen.",
  },
  "emailverify.message.test_sent": {
    mn: "{email} рүү баталгаажуулах захидал илгээх хүсэлт өглөө.",
    en: "A verification mail was requested for {email}.",
  },
  "emailverify.message.load_failed": { mn: "Ачаалж чадсангүй", en: "Could not load this screen" },
  "emailverify.message.send_failed": { mn: "Захидал илгээж чадсангүй", en: "Could not request the verification" },
} as const;
