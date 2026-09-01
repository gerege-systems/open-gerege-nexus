/**
 * base — Terms every screen shares: field labels, generic actions and
 * system messages. Nothing here belongs to one application.
 */
export const base = {
  "base.field.name": { mn: "Нэр", en: "Name" },
  "base.field.email": { mn: "И-мэйл", en: "Email" },
  "base.field.phone": { mn: "Утас", en: "Phone" },
  "base.field.company": { mn: "Байгууллага", en: "Company" },
  "base.field.status": { mn: "Төлөв", en: "Status" },
  "base.field.type": { mn: "Төрөл", en: "Type" },
  "base.field.date": { mn: "Огноо", en: "Date" },
  "base.field.language": { mn: "Хэл", en: "Language" },
  "base.field.actions": { mn: "Үйлдэл", en: "Actions" },

  "base.action.save": { mn: "Хадгалах", en: "Save" },
  "base.action.create": { mn: "Үүсгэх", en: "Create" },
  "base.action.edit": { mn: "Засах", en: "Edit" },
  "base.action.delete": { mn: "Устгах", en: "Delete" },
  "base.action.close": { mn: "Хаах", en: "Close" },
  "base.action.cancel": { mn: "Цуцлах", en: "Cancel" },
  "base.action.previous": { mn: "Өмнөх", en: "Previous" },
  "base.action.next": { mn: "Дараах", en: "Next" },
  "base.action.retry": { mn: "Дахин оролдох", en: "Try again" },
  "base.action.download": { mn: "Татаж авах", en: "Download" },
  "base.action.search": { mn: "Хайх", en: "Search" },
  "base.action.open": { mn: "Нээх", en: "Open" },

  "base.label.all": { mn: "Бүгд", en: "All" },

  "base.state.active": { mn: "Идэвхтэй", en: "Active" },
  "base.state.inactive": { mn: "Идэвхгүй", en: "Inactive" },

  "base.value.yes": { mn: "Тийм", en: "Yes" },
  "base.value.no": { mn: "Үгүй", en: "No" },

  "base.message.loading": { mn: "Ачаалж байна...", en: "Loading..." },
  "base.message.saving": { mn: "Хадгалж байна...", en: "Saving..." },
  "base.message.read_only": { mn: "Танд зөвхөн харах эрх бий. Өөрчлөлт хийхэд дараах эрх шаардана:", en: "You have read access only. Changing anything needs this permission:" },
  "base.message.admin_only_edit": { mn: "Танд зөвхөн харах эрх бий. Байгууллагын хуулийн мэдээллийг зөвхөн тенантын администратор өөрчилнө.", en: "You have read access only. The organisation's legal details are changed by a tenant administrator." },
  "base.message.admin_only_title": { mn: "Зөвхөн админд", en: "Administrators only" },
  "base.message.admin_only_body": { mn: "Энэ хэсгийг үзэхэд танай байгууллагын админ эрх шаардана. Тохиргоо → Хандалтын удирдлага хэсгээс админ эрх олгоно.", en: "This section needs tenant administrator rights. An administrator can grant them under Settings → Access control." },
  "base.message.no_data": { mn: "Одоогоор өгөгдөл алга.", en: "No data yet." },
  "base.message.error": { mn: "Алдаа гарлаа", en: "Something went wrong" },
  "base.message.page_of": { mn: "{page} / {total} хуудас", en: "Page {page} of {total}" },
  // The paged variant also names how many records the filter matched, which
  // "page 2 of 7" alone does not tell an auditor scanning a log.
  "base.message.page_summary": {
    mn: "{page} / {pages} хуудас · нийт {total}",
    en: "Page {page} of {pages} · {total} records",
  },

  // The pages a person lands on when the address is wrong or the screen threw.
  // Until these existed both were Next's own English default — a stack-trace
  // shell on a Mongolian platform, with no way back other than the browser's
  // Back button. They belong to the shell, so they sit in base.
  "base.error.not_found_title": {
    mn: "Хуудас олдсонгүй",
    en: "Page not found",
  },
  "base.error.not_found_body": {
    mn: "Хаяг буруу эсвэл энэ хуудсыг устгасан байна. Хаягаа шалгаад дахин оролдоно уу.",
    en: "The address is wrong, or this page has been removed. Check the address and try again.",
  },
  "base.error.crash_title": {
    mn: "Энэ хуудсыг нээж чадсангүй",
    en: "This page could not be opened",
  },
  "base.error.crash_body": {
    mn: "Алдааг бүртгэлээ. Дахин оролдоод үзнэ үү; давтагдвал системийн админд хандана уу.",
    en: "The error has been logged. Try again; if it keeps happening, contact your administrator.",
  },
  "base.error.home": {
    mn: "Нүүр хуудас руу",
    en: "Go to the home page",
  },

  // Installing the platform from the browser. It sits in base rather than in an
  // addon because it belongs to the shell, not to any one application.
  "pwa.install.title": {
    mn: "Апп болгон суулгах",
    en: "Install as an app",
  },
  "pwa.install.body": {
    mn: "Хөтчийн таб биш, өөрийн цонхтойгоор ажиллуулна.",
    en: "Runs in its own window instead of a browser tab.",
  },
  "pwa.install.action": {
    mn: "Суулгах",
    en: "Install",
  },
  "pwa.install.ios": {
    mn: "Хуваалцах → Нүүр дэлгэцэд нэмэх",
    en: "Share → Add to Home Screen",
  },
} as const;
