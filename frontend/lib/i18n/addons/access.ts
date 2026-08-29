/**
 * access — Tenant access control — roles, rights and who holds them.
 */
export const access = {
  "access.view.eyebrow": { mn: "RBAC · Байгууллагын аюулгүй байдал", en: "RBAC · Tenant security" },
  "access.view.title": { mn: "Эрхийн тохиргоо", en: "Access rights" },
  "access.view.subtitle": { mn: "Role бүрийн нэмэгдэх эрх болон хэрэглэгчийн role оноолтыг удирдана.", en: "Manage the additive rights of each role and who holds them." },
  "access.view.tab_roles": { mn: "Role ба эрх", en: "Roles and rights" },
  "access.view.tab_members": { mn: "Хэрэглэгчид", en: "Members" },
  // Хаалган дээр хүлээж байгаа хүмүүс. «Гишүүд» табтай зэрэгцээ: нэг нь
  // дотор байгаа хүмүүс, нөгөө нь орохыг хүсэж байгаа хүмүүс.
  "access.view.tab_requests": { mn: "Хүсэлт", en: "Requests" },
  "access.view.requests_title": { mn: "Нэгдэх хүсэлтүүд", en: "Requests to join" },
  "access.view.requests_hint": {
    mn: "Эдгээр хүн танай байгууллагад нэгдэхийг хүссэн. Зөвшөөрвөл гишүүн болж, анхдагч эрхээ авна.",
    en: "These people asked to join your organisation. Accepting makes them a member with the default role.",
  },
  "access.message.no_requests": { mn: "Хүлээгдэж буй хүсэлт алга.", en: "Nobody is waiting." },
  "access.action.accept": { mn: "Зөвшөөрөх", en: "Accept" },
  "access.action.decline": { mn: "Татгалзах", en: "Decline" },
  "access.message.request_accepted": { mn: "{name} гишүүн боллоо.", en: "{name} is now a member." },
  "access.message.request_declined": { mn: "{name}-ийн хүсэлтээс татгалзлаа.", en: "{name}'s request was declined." },
  "access.message.error_decide": { mn: "Хүсэлтэд хариулж чадсангүй.", en: "Could not answer the request." },
  "access.view.create_role": { mn: "Шинэ role", en: "New role" },
  "access.view.members_title": { mn: "Хэрэглэгчийн role оноолт", en: "Role assignment" },
  "access.view.members_hint": { mn: "Олон role-ийн эрх нийлж үйлчилнэ. Өөрчлөлт шууд хадгалагдана.", en: "Rights from several roles add up. Changes save immediately." },

  "access.field.code_placeholder": { mn: "sales_manager", en: "sales_manager" },
  "access.field.name_placeholder": { mn: "Борлуулалтын менежер", en: "Sales manager" },
  "access.field.description_placeholder": { mn: "Тайлбар", en: "Description" },

  "access.action.create_role": { mn: "Role нэмэх", en: "Add role" },

  "access.message.loading": { mn: "Эрхийн тохиргоо ачаалж байна…", en: "Loading access settings…" },
  "access.message.role_summary": { mn: "{count} эрх", en: "{count} rights" },
  "access.message.no_description": { mn: "Тайлбаргүй", en: "No description" },
  "access.message.admin_note": { mn: "Administrator role бүх эрхийг автоматаар эзэмшинэ.", en: "The administrator role always holds every right." },
  "access.message.confirm_delete": { mn: "{name} role-ийг устгах уу?", en: "Delete the {name} role?" },
  "access.message.saved": { mn: "Эрхүүд хадгалагдлаа", en: "Rights saved" },
  "access.message.role_created": { mn: "Role үүслээ", en: "Role created" },
  "access.message.role_deleted": { mn: "Role устлаа", en: "Role deleted" },
  "access.message.confirm_admin": {
    mn: "{name}-г админ болгох уу? Админ нь гишүүд, эрх, тохиргоог удирдаж, байгууллагын бүх өгөгдөлд хүрнэ.",
    en: "Make {name} an administrator? An administrator manages members, roles and settings, and reaches everything the organisation keeps.",
  },
  "access.message.member_updated": { mn: "Хэрэглэгчийн role шинэчлэгдлээ", en: "Member roles updated" },
  "access.message.error_load": { mn: "Эрхийн мэдээлэл ачаалж чадсангүй", en: "Could not load access settings" },
  "access.message.error_save": { mn: "Хадгалж чадсангүй", en: "Could not save" },
  "access.message.error_create": { mn: "Role үүсгэж чадсангүй", en: "Could not create the role" },
  "access.message.error_delete": { mn: "Устгаж чадсангүй", en: "Could not delete" },
  "access.message.error_assign": { mn: "Role оноож чадсангүй", en: "Could not assign the role" },
} as const;
