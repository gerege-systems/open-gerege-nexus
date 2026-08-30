/**
 * core — the organisation and the people in it.
 *
 * Terms here name what an organisation *is* rather than what any one app does
 * with it, so they are deliberately the plainest words available: a
 * registration number is called a registration number, not a "company code".
 */
export const core = {
  // Байгууллага юуг олон нийтэд зарлаж байгаа (00090). Хүсэлт хүлээж авах нь
  // дотоод шийдвэр, лавлахад гарах нь гадаад амлалт — тиймээс тусдаа дэлгэц.
  "core.view.services_title": { mn: "Нийтэлсэн үйлчилгээ", en: "Published services" },
  "core.view.services_hint": {
    mn: "Эдгээр нь суурилуулалт даяарх лавлахад гарч, иргэн танайхыг олж хүсэлт гаргана. Дотоод кодоо (local.) нийтлэх боломжгүй.",
    en: "These appear in the deployment-wide directory, where a citizen can find you and ask. Your own local. codes cannot be published.",
  },
  "core.message.no_services": { mn: "Нийтэлсэн үйлчилгээ алга.", en: "Nothing published yet." },
  "core.field.service_code": { mn: "код", en: "code" },
  "core.field.service_title": { mn: "юу гэж нэрлэх вэ", en: "what you call it" },
  "core.action.publish": { mn: "Нийтлэх", en: "Publish" },
  "core.action.withdraw": { mn: "Хасах", en: "Withdraw" },
  "core.view.organisation_title": { mn: "Байгууллага", en: "Organisation" },
  "core.view.organisation_subtitle": {
    mn: "Байгууллагын албан ёсны мэдээлэл — баримт бичиг, тайланд хэвлэгдэнэ",
    en: "The organisation's official details — printed on documents and reports",
  },
  "core.view.people_title": { mn: "Ажилтнууд", en: "People" },
  "core.view.people_subtitle": {
    mn: "Энэ байгууллагад ажиллаж буй хүмүүс, тэдний албан тушаал, харьяалал",
    en: "Who works in this organisation, their job title and where they belong",
  },
  "core.view.departments_title": { mn: "Хэлтэс, нэгж", en: "Departments" },
  "core.view.departments_subtitle": {
    mn: "Байгууллагын бүтэц — аль нэгж хэнд харьяалагдахыг тодорхойлно",
    en: "How the organisation is arranged — which unit reports to which",
  },
  "core.view.archived": { mn: "Архивласан нэгж ({count})", en: "Archived units ({count})" },

  "core.group.identity": { mn: "Албан ёсны нэр, бүртгэл", en: "Legal identity" },
  "core.group.address": { mn: "Хаяг", en: "Address" },
  "core.group.contact": { mn: "Холбоо барих", en: "Contact" },
  // Not "settings": these are what the organisation uses when nobody has said
  // otherwise, and a person may still choose their own language over them.
  "core.group.defaults": { mn: "Үндсэн тохиргоо", en: "Defaults" },
  "core.group.affiliation": { mn: "Харьяалал", en: "Affiliation" },
  "core.group.joining": { mn: "Хаалга", en: "The door" },

  "core.field.name": { mn: "Дэлгэцэнд харагдах нэр", en: "Display name" },
  "core.field.legal_name": { mn: "Албан ёсны нэр", en: "Legal name" },
  "core.field.registration_number": { mn: "Улсын бүртгэлийн дугаар", en: "Registration number" },
  "core.field.tax_number": { mn: "Татвар төлөгчийн дугаар", en: "Tax number" },
  "core.field.province": { mn: "Аймаг / нийслэл", en: "Province / city" },
  "core.field.district": { mn: "Сум / дүүрэг", en: "District" },
  "core.field.khoroo": { mn: "Баг / хороо", en: "Khoroo" },
  "core.field.address_line": { mn: "Дэлгэрэнгүй хаяг", en: "Address" },
  "core.field.postal_code": { mn: "Шуудангийн код", en: "Postal code" },
  "core.field.website": { mn: "Вэб хуудас", en: "Website" },
  "core.field.logo_url": { mn: "Лого (URL)", en: "Logo URL" },
  "core.field.timezone": { mn: "Цагийн бүс", en: "Time zone" },
  "core.field.currency": { mn: "Валют", en: "Currency" },

  "core.field.code": { mn: "Код", en: "Code" },
  "core.field.parent": { mn: "Харьяалагдах нэгж", en: "Reports to" },
  "core.field.parent_organisation": { mn: "Толгой байгууллага", en: "Parent organisation" },
  "core.state.independent": { mn: "Бие даасан", en: "Independent" },
  "core.state.join_on_request": {
    mn: "Хүсэлтээр — админ шийднэ",
    en: "On request — an administrator decides",
  },
  "core.state.join_open": {
    mn: "Нээлттэй — шууд нэгдэнэ",
    en: "Open — they join at once",
  },
  "core.field.manager": { mn: "Хариуцсан ажилтан", en: "Manager" },
  "core.field.people_count": { mn: "{count} ажилтан", en: "{count} people" },
  "core.field.job_title": { mn: "Албан тушаал", en: "Job title" },
  "core.field.department": { mn: "Хэлтэс, нэгж", en: "Department" },
  "core.field.organisation": { mn: "Байгууллага", en: "Organisation" },
  "core.field.roles": { mn: "Эрх", en: "Roles" },
  "core.label.admin": { mn: "Админ", en: "Admin" },

  "core.action.deactivate": { mn: "Идэвхгүй болгох", en: "Deactivate" },
  "core.action.reactivate": { mn: "Сэргээх", en: "Reactivate" },
  "core.action.archive": { mn: "Архивлах", en: "Archive" },
  // Its own term rather than reusing the people one: in Mongolian both are
  // "Сэргээх", but a person is reactivated and a unit is restored, and the
  // languages that distinguish the two should be allowed to.
  "core.action.restore": { mn: "Сэргээх", en: "Restore" },

  // Named, because "are you sure?" is a question nobody can answer without
  // being told what they are about to lose.
  "core.message.confirm_delete": {
    mn: "{name} нэгжийг устгах уу? Энэ үйлдлийг буцаах боломжгүй — түүхээ хадгалах бол оронд нь архивлана уу.",
    en: "Delete {name}? This cannot be undone — archive it instead to keep its history.",
  },
  "core.action.core_sync": { mn: "Core-оос шинэчлэх", en: "Refresh from the register" },
  "core.hint.core_sync": {
    mn: "Регистрийн дугаараар Gerege Core-оос албан ёсны нэр, хаяг, холбоо барих мэдээллийг татаж шинэчилнэ. Регистрд байхгүй талбарыг хөндөхгүй.",
    en: "Looks the organisation up in Gerege Core by its registration number and refreshes the legal name, address and contact details. A field the register does not hold is left alone.",
  },
  "core.message.core_synced": {
    mn: "Байгууллагын мэдээллийг Gerege Core-оос шинэчиллээ.",
    en: "The organisation was refreshed from the Gerege Core register.",
  },
  "core.message.saved": { mn: "Байгууллагын мэдээлэл хадгалагдлаа", en: "The organisation's details were saved" },
  // Says what this is and, just as importantly, what it is not: recording a
  // parent grants nobody anything.
  "core.field.join_policy": { mn: "Хэн орж болох вэ", en: "Who may come in" },
  "core.message.join_policy_hint": {
    mn: "Гишүүн бус хүн байгууллагын богино нэрээр нэгдэхийг хүсэхэд юу болохыг заана.",
    en: "What happens when somebody who is not a member asks to join by this organisation's short name.",
  },
  "core.message.join_open_note": {
    mn: "Богино нэрийг мэдсэн хэн ч гишүүн болно. Шинэ гишүүн платформын анхдагч «user» ролийг авдаг тул суусан аппуудын уншиж болох мэдээллийг шууд харна — хүсэлтийг зөвшөөрсөнтэй ижил.",
    en: "Anybody who knows the short name becomes a member. A new member receives the platform's default «user» role, so they can read what the installed apps allow — exactly as an approved request would.",
  },

  "core.message.parent_hint": {
    mn: "Салбар, нэгжийг Хэлтэс, нэгж хэсэгт бүртгэнэ. Энд зөвхөн тусдаа хуулийн этгээд болох толгой байгууллагаа заана — өөрийн ажилладаг байгууллагуудаас сонгоно. Энэ нь мэдээллийн хандалтыг өөрчлөхгүй.",
    en: "Branches and offices are departments. This is for a parent that is a separate legal entity, chosen from the organisations you work in. Recording it changes nothing about who can see what.",
  },
  // Said once, at the bottom, rather than as a disabled "Add person" button:
  // the button would suggest the screen could do it and is merely refusing.
  "core.message.people_hint": {
    mn: "Шинэ ажилтныг Тохиргоо → Хандалтын эрх хэсгээс урина. Ажилтныг устгахгүй, идэвхгүй болгодог — түүний хийсэн ажлын түүх хэвээр үлдэнэ.",
    en: "Invite somebody from Settings → Access control. People are deactivated rather than deleted, so what they did stays readable.",
  },
} as const;
