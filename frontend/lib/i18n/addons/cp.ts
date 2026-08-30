/**
 * cp — the operator console.
 *
 * Its own file rather than lines in `web`, because none of these terms belong
 * to the product tenants use: nobody outside the operating team ever sees a
 * string from here, and mixing them in would put the console's vocabulary into
 * every translator's queue for the tenant application.
 */
export const cp = {
  "cp.view.title": { mn: "Удирдлагын самбар", en: "Control Plane" },
  "cp.view.subtitle": {
    mn: "Платформын операторын консол. Байгууллагууд, тэдгээрийн төлөв, операторын үйлдлийн бүртгэл.",
    en: "The platform operator's console: organisations, their state, and what operators have done.",
  },

  "cp.login.title": { mn: "Операторын нэвтрэлт", en: "Operator sign-in" },
  "cp.login.hint": {
    mn: "И-мэйл, нууц үг, баталгаажуулагчийн код гурвуулаа шаардлагатай.",
    en: "Your e-mail, password and authenticator code are all required.",
  },
  "cp.login.failed": {
    mn: "И-мэйл, нууц үг эсвэл код таарсангүй.",
    en: "The e-mail address, password or code was not right.",
  },

  "cp.field.email": { mn: "И-мэйл", en: "E-mail" },
  "cp.field.password": { mn: "Нууц үг", en: "Password" },
  "cp.field.code": { mn: "Баталгаажуулагчийн код", en: "Authenticator code" },
  "cp.field.search": { mn: "Нэр, slug, регистрээр хайх", en: "Search by name, slug or registration number" },
  "cp.field.organisation": { mn: "Байгууллага", en: "Organisation" },
  "cp.field.slug": { mn: "Slug", en: "Slug" },
  "cp.field.registration": { mn: "Регистр", en: "Registration" },
  "cp.field.users": { mn: "Хэрэглэгч", en: "People" },
  "cp.field.apps": { mn: "Апп", en: "Apps" },
  "cp.field.created": { mn: "Үүссэн", en: "Created" },
  "cp.field.last_activity": { mn: "Сүүлийн идэвх", en: "Last activity" },
  "cp.field.legal_name": { mn: "Албан ёсны нэр", en: "Legal name" },
  "cp.field.tax_number": { mn: "Татварын дугаар", en: "Tax number" },
  "cp.field.version": { mn: "Хувилбар", en: "Version" },
  "cp.field.status": { mn: "Төлөв", en: "Status" },
  "cp.field.installed": { mn: "Суусан", en: "Installed" },
  "cp.field.roles": { mn: "Үүрэг", en: "Roles" },
  "cp.field.action": { mn: "Үйлдэл", en: "Action" },
  "cp.field.role": { mn: "Эрх", en: "Role" },
  "cp.field.last_login": { mn: "Сүүлд нэвтэрсэн", en: "Last sign-in" },
  "cp.field.operator": { mn: "Оператор", en: "Operator" },
  "cp.field.reason": { mn: "Шалтгаан", en: "Reason" },
  "cp.field.when": { mn: "Хэзээ", en: "When" },
  "cp.field.resource": { mn: "Обьект", en: "Resource" },
  "cp.field.target_type": { mn: "Объектын төрөл", en: "Target type" },
  "cp.field.target_id": { mn: "Объектын ID", en: "Target ID" },

  "cp.action.sign_in": { mn: "Нэвтрэх", en: "Sign in" },
  "cp.action.sign_out": { mn: "Гарах", en: "Sign out" },
  "cp.action.back": { mn: "Жагсаалт руу", en: "Back to the list" },
  "cp.action.search": { mn: "Хайх", en: "Search" },
  "cp.action.clear": { mn: "Цэвэрлэх", en: "Clear" },
  "cp.action.refresh": { mn: "Шинэчлэх", en: "Refresh" },

  "cp.section.tenants": { mn: "Байгууллагууд", en: "Organisations" },
  "cp.section.apps": { mn: "Суусан аппууд", en: "Installed apps" },
  "cp.section.members": { mn: "Хэрэглэгчид", en: "People" },
  "cp.section.activity": { mn: "Сүүлийн идэвх", en: "Recent activity" },
  "cp.section.operator_actions": { mn: "Операторын үйлдэл", en: "Operator actions" },
  "cp.section.audit": { mn: "Операторын бүртгэл", en: "Operator audit" },
  "cp.audit.append_only": { mn: "Өөрчлөгдөшгүй мөр", en: "Append-only ledger" },
  "cp.audit.hint": {
    mn: "Операторын бичих үйлдэл бүр хэн, юунд, ямар шалтгаанаар хүрснийг шинэ мөр болгон үлдээнэ.",
    en: "Every operator write leaves a new row saying who acted, what it touched, and why.",
  },
  "cp.audit.filter": { mn: "Бүртгэлийг нарийсгах", en: "Narrow the ledger" },
  "cp.audit.empty": { mn: "Энэ нөхцөлд таарах бүртгэл алга.", en: "Nothing matches these filters." },
  "cp.audit.change": { mn: "Өмнөх ба дараах утга", en: "Before and after" },
  "cp.audit.before": { mn: "Өмнө", en: "Before" },
  "cp.audit.after": { mn: "Дараа", en: "After" },

  "cp.role.superadmin": { mn: "Ерөнхий админ", en: "Superadmin" },
  "cp.role.operator": { mn: "Оператор", en: "Operator" },
  "cp.role.support": { mn: "Дэмжлэг", en: "Support" },
  "cp.role.auditor": { mn: "Аудитор", en: "Auditor" },

  "cp.message.read_only": {
    mn: "Үйлдэл бүр шалтгаан шаардана, audit-д бичигдэнэ. Устгал нь хоёр дахь superadmin-ий зөвшөөрөл ба 30 хоногийн хугацаатай.",
    en: "Every action needs a reason and is recorded. Deletion needs a second superadmin and takes thirty days.",
  },
  "cp.message.no_tenants": { mn: "Байгууллага олдсонгүй.", en: "No organisations found." },
  "cp.message.no_activity": { mn: "Бүртгэл алга.", en: "Nothing recorded." },
  "cp.message.load_failed": {
    mn: "Мэдээллийг уншиж чадсангүй.",
    en: "That could not be loaded.",
  },
  "cp.message.never": { mn: "Хэзээ ч", en: "Never" },

  "cp.action.cancel": { mn: "Болих", en: "Cancel" },
  "cp.action.confirm": { mn: "Гүйцэтгэх", en: "Confirm" },
  "cp.action.suspend": { mn: "Түдгэлзүүлэх", en: "Suspend" },
  "cp.action.resume": { mn: "Сэргээх", en: "Resume" },
  "cp.action.delete": { mn: "Устгах хүсэлт", en: "Ask to delete" },
  "cp.action.cancel_deletion": { mn: "Устгалыг цуцлах", en: "Cancel the deletion" },
  "cp.action.export": { mn: "Өгөгдлийг татах", en: "Export the data" },
  "cp.action.quota": { mn: "Хязгаар", en: "Limits" },
  "cp.action.impersonate": { mn: "Дотор нь орж харах", en: "Look inside" },
  "cp.action.new_tenant": { mn: "Шинэ байгууллага", en: "New organisation" },
  "cp.action.create": { mn: "Үүсгэх", en: "Create" },
  "cp.action.approve": { mn: "Зөвшөөрөх", en: "Approve" },
  "cp.action.reject": { mn: "Татгалзах", en: "Reject" },
  "cp.action.unlock": { mn: "Түгжээ тайлах", en: "Unlock" },
  "cp.action.revoke_sessions": { mn: "Бүх session хаах", en: "End every session" },
  "cp.action.send_reset": { mn: "Нууц үг сэргээх холбоос", en: "Send a reset link" },

  "cp.field.max_users": { mn: "Хэрэглэгчийн дээд тоо", en: "Maximum people" },
  "cp.field.max_storage": { mn: "Хадгалалт (MB)", en: "Storage (MB)" },
  "cp.field.max_ai": { mn: "AI дуудлага / сар", en: "AI calls / month" },
  "cp.field.enforcement": { mn: "Горим", en: "Mode" },
  "cp.field.name": { mn: "Нэр", en: "Name" },
  "cp.field.admin_email": { mn: "Эхний админы и-мэйл", en: "First administrator's e-mail" },
  "cp.field.install_apps": { mn: "Суулгах аппууд", en: "Apps to install" },
  "cp.field.person": { mn: "Хүн", en: "Person" },
  "cp.field.state": { mn: "Төлөв", en: "State" },
  "cp.field.requested_by": { mn: "Хүссэн", en: "Asked by" },

  "cp.state.active": { mn: "Идэвхтэй", en: "Active" },
  "cp.state.suspended": { mn: "Түдгэлзсэн", en: "Suspended" },
  "cp.state.deleting": { mn: "Устгал хүлээж буй", en: "Awaiting deletion" },
  "cp.state.soft": { mn: "Зөөлөн (анхааруулна)", en: "Soft (warns)" },
  "cp.state.hard": { mn: "Хатуу (хориглоно)", en: "Hard (refuses)" },
  "cp.state.locked": { mn: "Түгжигдсэн", en: "Locked" },

  "cp.section.support": { mn: "Дэмжлэг", en: "Support" },
  "cp.section.approvals": { mn: "Хүлээгдэж буй зөвшөөрөл", en: "Waiting for approval" },
  "cp.section.impersonations": { mn: "Дотор нь орсон түүх", en: "Who has looked inside" },
  "cp.section.limits": { mn: "Хязгаарууд", en: "Limits" },
  "cp.section.actions": { mn: "Үйлдэл", en: "Actions" },

  "cp.hint.reason": {
    mn: "Энэ бичиг audit-д үлдэж, зарим тохиолдолд тухайн байгууллагад ч харагдана.",
    en: "This is recorded in the audit trail, and for some actions the organisation sees it too.",
  },
  "cp.hint.search_people": { mn: "И-мэйл эсвэл нэрээр (3-аас дээш тэмдэгт)", en: "By e-mail or name (three characters or more)" },
  "cp.hint.not_enforced": {
    mn: "Бүртгэгдэнэ, гэхдээ хараахан хэрэгжихгүй — CP-5-ын хэрэглээний хэмжилт ирэхэд ажиллана.",
    en: "Recorded but not yet enforced — it starts working when CP-5's usage metering lands.",
  },
  "cp.message.step_up": {
    mn: "Баталгаажуулагчийн кодоо дахин оруулна уу.",
    en: "Enter your authenticator code again.",
  },
  "cp.message.deletion_requested": {
    mn: "Хоёр дахь superadmin зөвшөөрөх хүртэл юу ч болохгүй. Зөвшөөрсний дараа 30 хоногийн дотор буцаах боломжтой.",
    en: "Nothing happens until a second superadmin agrees. After that, thirty days in which it can be undone.",
  },
  "cp.message.no_people": { mn: "Хэн ч олдсонгүй.", en: "Nobody found." },
  "cp.message.no_approvals": { mn: "Хүлээгдэж буй зүйл алга.", en: "Nothing is waiting." },

  "cp.section.config": { mn: "Тохиргоо", en: "Configuration" },
  "cp.section.settings": { mn: "Платформын тохиргоо", en: "Platform settings" },
  "cp.section.flags": { mn: "Feature flag", en: "Feature flags" },
  "cp.section.announcements": { mn: "Зарлал", en: "Announcements" },

  "cp.field.setting": { mn: "Түлхүүр", en: "Setting" },
  "cp.field.value": { mn: "Утга", en: "Value" },
  "cp.field.source": { mn: "Хаанаас", en: "Source" },
  "cp.field.default": { mn: "Анхдагч", en: "Default" },
  "cp.field.flag": { mn: "Flag", en: "Flag" },
  "cp.field.kind": { mn: "Төрөл", en: "Kind" },
  "cp.field.rollout": { mn: "Хувь", en: "Rollout" },
  "cp.field.expires": { mn: "Хугацаа", en: "Expires" },
  "cp.field.description": { mn: "Тайлбар", en: "Description" },
  "cp.field.owner": { mn: "Эзэмшигч", en: "Owner" },
  "cp.field.title": { mn: "Гарчиг", en: "Title" },
  "cp.field.body": { mn: "Текст", en: "Body" },
  "cp.field.until": { mn: "Хүртэл", en: "Until" },

  "cp.source.database": { mn: "Консол", en: "Console" },
  "cp.source.environment": { mn: "Env", en: "Environment" },
  "cp.source.default": { mn: "Анхдагч", en: "Default" },
  "cp.source.unset": { mn: "Тохируулаагүй", en: "Not set" },

  // Түлхүүрүүд. Утгыг нь буцаадаг API байхгүй тул энд ч гэсэн "харах" гэсэн
  // үйлдэл байхгүй — зөвхөн тавих ба цэвэрлэх.
  "cp.section.credentials": { mn: "Түлхүүрүүд", en: "Credentials" },
  "cp.hint.credentials": {
    mn: "Гадаад системд хандах түлхүүрүүд. Утга нь шифрлэгдэж хадгалагдана, буцааж харагдахгүй — зөвхөн сүүлийн дөрвөн тэмдэгт.",
    en: "The keys this deployment reaches other systems with. Values are stored sealed and never read back — only the last four characters.",
  },
  "cp.field.credential": { mn: "Түлхүүр", en: "Credential" },
  "cp.field.updated": { mn: "Шинэчилсэн", en: "Updated" },
  "cp.action.set_credential": { mn: "Тавих", en: "Set" },
  "cp.action.clear_credential": { mn: "Цэвэрлэх", en: "Clear" },
  "cp.message.no_credentials": { mn: "Түлхүүр бүртгэгдээгүй байна.", en: "No credentials are registered." },
  "cp.message.sealing_off": {
    mn: "INTEGRATION_ENCRYPTION_KEY тохируулаагүй тул түлхүүр хадгалах боломжгүй. Цэвэр текстээр хадгалахын оронд бичилт татгалзана.",
    en: "INTEGRATION_ENCRYPTION_KEY is not set, so no credential can be stored. The write is refused rather than storing it in the clear.",
  },
  "cp.message.credential_write_only": {
    mn: "Одоогийн утгыг харуулах боломжгүй. Шинэ утга оруулбал хуучныг орлоно.",
    en: "The current value cannot be shown. A new value replaces the old one.",
  },

  "cp.kind.release": { mn: "Гаргалт", en: "Release" },
  "cp.kind.kill_switch": { mn: "Унтраалга", en: "Kill switch" },
  "cp.kind.experiment": { mn: "Туршилт", en: "Experiment" },
  "cp.kind.info": { mn: "Мэдээлэл", en: "Information" },
  "cp.kind.warning": { mn: "Анхааруулга", en: "Warning" },
  "cp.kind.maintenance": { mn: "Засвар", en: "Maintenance" },

  "cp.state.off": { mn: "Унтраалттай", en: "Off" },
  "cp.state.everyone": { mn: "Бүх байгууллага", en: "Everybody" },

  "cp.action.change": { mn: "Өөрчлөх", en: "Change" },
  "cp.action.rollback": { mn: "Буцаах", en: "Roll back" },
  "cp.action.new_flag": { mn: "Шинэ flag", en: "New flag" },
  "cp.action.delete_flag": { mn: "Flag устгах", en: "Delete the flag" },
  "cp.action.turn_on": { mn: "Асаах", en: "Turn on" },
  "cp.action.turn_off": { mn: "Унтраах", en: "Turn off" },
  "cp.action.announce": { mn: "Зарлал нийтлэх", en: "Publish an announcement" },
  "cp.action.withdraw": { mn: "Буцаан авах", en: "Withdraw" },
  "cp.action.maintenance_on": { mn: "Засварын горим", en: "Maintenance mode" },
  "cp.action.maintenance_off": { mn: "Засвар дуусгах", en: "End the maintenance" },

  "cp.hint.config": {
    mn: "Утга бүр хаанаас ирснийг харуулна. Өөрчлөлт бүр шалтгаантай, түүхтэй, нэг товчоор буцаана — платформыг дахин ачаалахгүйгээр.",
    en: "Every value says where it came from. Every change has a reason, a history, and one button to undo it — with no restart.",
  },
  "cp.message.no_flags": { mn: "Flag алга.", en: "No flags." },
  "cp.message.no_announcements": { mn: "Зарлал алга.", en: "No announcements." },

  "cp.section.health": { mn: "Платформын эрүүл мэнд", en: "Platform health" },
  "cp.section.alerts": { mn: "Идэвхтэй дохио", en: "Firing alerts" },
  "cp.section.external": { mn: "Гадаад системүүд", en: "External systems" },
  "cp.section.infra": { mn: "Дэд бүтэц", en: "Infrastructure" },
  "cp.section.background": { mn: "Арын ажлууд", en: "Background jobs" },
  "cp.section.tenant_trouble": { mn: "Асуудалтай байгууллагууд", en: "Organisations in trouble" },
  "cp.section.backups": { mn: "Нөөцлөлт", en: "Backups" },
  "cp.section.catalog": { mn: "Каталог", en: "Catalogue" },

  "cp.stat.rps": { mn: "Хүсэлт/сек", en: "Requests/s" },
  "cp.stat.errors": { mn: "Алдааны хувь", en: "Error rate" },
  "cp.stat.p95": { mn: "p95 хугацаа", en: "p95 latency" },

  "cp.field.alert": { mn: "Дохио", en: "Alert" },
  "cp.field.severity": { mn: "Түвшин", en: "Severity" },
  "cp.field.job": { mn: "Ажил", en: "Job" },
  "cp.field.last_run": { mn: "Сүүлд ажилласан", en: "Last run" },
  "cp.field.failures": { mn: "Алдаа", en: "Failures" },
  "cp.field.last_backup": { mn: "Сүүлийн нөөцлөлт", en: "Last backup" },
  "cp.field.last_restore_test": { mn: "Сүүлийн сэргээлтийн туршилт", en: "Last restore test" },
  "cp.field.last_sync": { mn: "Сүүлийн синк", en: "Last sync" },

  "cp.job.scheduled_reports": { mn: "Товлосон тайлан", en: "Scheduled reports" },
  "cp.job.catalog_sync": { mn: "Каталогийн синк", en: "Catalogue sync" },
  "cp.job.deletion_sweep": { mn: "Устгалын цэвэрлэгээ", en: "Deletion sweep" },
  // Өртөө: the console sees two numbers about the channel — how much is queued
  // for another installation and how many links have gone quiet — and never
  // what was actually said. See migration 00064.

  "cp.state.ok": { mn: "Хэвийн", en: "Healthy" },
  "cp.state.failing": { mn: "Алдаатай", en: "Failing" },
  "cp.state.silenced": { mn: "Чимээгүй болгосон", en: "Silenced" },

  "cp.action.deploy": { mn: "Deploy эхлүүлэх", en: "Start a deployment" },
  "cp.action.open_grafana": { mn: "Grafana", en: "Grafana" },
  "cp.action.record_restore_test": { mn: "Сэргээлт туршсанаа бүртгэх", en: "Record a restore test" },
  "cp.action.sync_catalog": { mn: "Каталог синк хийх", en: "Sync catalog" },

  "cp.hint.deploy": {
    mn: "GitHub Actions-ийн deploy workflow-г main дээр өдөөнө. Серверт юу ч гүйцэтгэхгүй, env хөндөхгүй — явцыг GitHub дээр хараарай.",
    en: "Triggers the GitHub Actions deploy workflow on main. Nothing is executed on the server and no environment is touched — watch it on GitHub.",
  },
  "cp.hint.restore_test": {
    mn: "Туршаагүй нөөцлөлт бол нөөцлөлт биш. Сэргээлтийг бодитоор туршиж үзсэн бол огноог нь эндээс бүртгэнэ.",
    en: "An untested backup is not a backup. Record the date when a restore has actually been tried.",
  },
  "cp.hint.sync_catalog": {
    mn: "Регистрээс шинэ аппын каталогийг татаж платформыг шинэчилнэ.",
    en: "Fetches the latest app catalogue from the registry and updates the deployment.",
  },
  "cp.message.no_monitoring": {
    mn: "Энэ суулгацад PROMETHEUS_URL тохируулаагүй тул хэмжүүрийн хэсэг хоосон байна. docs/OPERATIONS.md-г үз.",
    en: "PROMETHEUS_URL is not set on this deployment, so the metric panels are empty. See docs/OPERATIONS.md.",
  },
  "cp.message.no_alerts": { mn: "Идэвхтэй дохио алга.", en: "Nothing is alerting." },
  "cp.message.no_backups": {
    mn: "Нөөцлөлт хэзээ ч бүртгэгдээгүй байна. deploy/scripts/backup.sh-г cron-д тавина уу.",
    en: "No backup has ever reported. Install deploy/scripts/backup.sh in cron.",
  },
  "cp.message.never_tested": { mn: "Хэзээ ч туршаагүй", en: "Never tested" },

  "cp.section.usage": { mn: "Хэрэглээ", en: "Usage" },
  "cp.action.usage": { mn: "Хэрэглээ", en: "Usage" },
  "cp.field.counted": { mn: "Сүүлд тоолсон", en: "Last counted" },

  "cp.metric.active_users": { mn: "Идэвхтэй хэрэглэгч (өдрийн дээд)", en: "Active people (daily peak)" },
  "cp.metric.actions": { mn: "Бүртгэгдсэн үйлдэл", en: "Recorded actions" },
  "cp.metric.ai_calls": { mn: "AI дуудлага", en: "AI calls" },
  "cp.metric.reports_sent": { mn: "Илгээсэн тайлан", en: "Reports sent" },
  "cp.metric.storage_mb": { mn: "Хадгалалт (MB)", en: "Storage (MB)" },

  "cp.state.not_enforced": { mn: "хэрэгжихгүй", en: "not enforced" },
  "cp.message.no_usage": { mn: "Энэ хугацаанд тоолол алга.", en: "Nothing counted in this window." },
  "cp.message.never_counted": {
    mn: "Хэрэглээ хараахан тоологдоогүй байна — тоолол шөнө бүр ажиллана.",
    en: "Nothing has been counted yet — the collection runs nightly.",
  },

  // The two screens the console took over from the workspace: the assistant
  // every organisation meets, and the ledger of addresses the platform was
  // asked to write to.
  "cp.section.assistant": { mn: "AI туслах", en: "Assistant" },
  "cp.hint.assistant": {
    mn: "Бүх байгууллагад нийтлэг үйлчлэх заавар ба мэдлэгийн сан. Байгууллага өөрийн зааврыг бичээгүй үед туслах эдгээрийг хэрэглэнэ.",
    en: "The instructions and the corpus every organisation shares. An organisation that has written none of its own is answered with these.",
  },
  "cp.section.verifications": { mn: "И-мэйл баталгаажуулалт", en: "Email verification" },
  "cp.hint.verifications": {
    mn: "Платформ хэнд бичсэн, үйлчилгээ нь ажиллаж байна уу — бүх байгууллагыг нэг дор.",
    en: "Who the platform has written to, and whether the service is answering — every organisation at once.",
  },
  "cp.hint.tenants_touched": { mn: "{count} байгууллага", en: "{count} organisations" },
  "cp.field.active": { mn: "Идэвхтэй", en: "Active" },
  "cp.state.never": { mn: "хэзээ ч", en: "never" },
  "cp.state.deleted": { mn: "устсан байгууллага", en: "deleted organisation" },

  // System Operations — консолын хоёр дахь апп: байрлуулалтаа ажиллуулах
  // (хяналт), юу үйлдвэрлэгдэж байгаа (тайлан), юу хадгалагдаж байгаа (нөөц).
  "cp.app.ops": { mn: "Системийн үйл ажиллагаа", en: "System Operations" },
  "cp.group.monitor": { mn: "Хяналт", en: "Monitor" },
  "cp.group.report": { mn: "Тайлан", en: "Report" },
  "cp.group.backup": { mn: "Нөөцлөлт", en: "Backup" },

  "cp.section.metrics": { mn: "Үзүүлэлт", en: "Metrics" },
  "cp.section.jobs": { mn: "Арын ажлууд", en: "Background jobs" },
  "cp.section.schedules": { mn: "Товлосон тайлан", en: "Scheduled reports" },
  "cp.section.infrastructure": { mn: "Дэд бүтэц", en: "Infrastructure" },
  "cp.section.firing": { mn: "Идэвхтэй сэрэмжлүүлэг", en: "Firing" },
  "cp.section.warnings": { mn: "Тохиргооны анхааруулга", en: "Configuration warnings" },
  "cp.section.by_organisation": { mn: "Байгууллагаар", en: "By organisation" },
  "cp.section.history": { mn: "Түүх", en: "History" },

  "cp.hint.metrics": {
    mn: "API-ийн гурван тоо, хамаарах гадаад системүүд, чимээгүй дүүрдэг дөрвөн хэмжүүр.",
    en: "The API's own three numbers, the systems it depends on, and the four gauges that fill up quietly.",
  },
  "cp.hint.alerts": {
    mn: "Одоо асаж буй сэрэмжлүүлэг ба платформын өөрийн тохиргооны гомдол — хоёр өөр зүйл, нэг мөчид уншигддаг.",
    en: "What is firing now, and what the platform says about its own configuration — two different things, read at the same moment.",
  },
  "cp.hint.jobs": {
    mn: "Өөрөө ажиллах ёстой бүхэн. Эдгээр нь чимээгүй уналттай: ажиллахаа больсныг долоо хоногийн дараа хүн анзаарна.",
    en: "Everything that should run on its own. These fail silently: somebody notices a week later.",
  },
  "cp.hint.usage": {
    mn: "Бүх байгууллагын энэ сарын хэрэглээ. Тоолуур бүр өөрийн утгаараа нэгтгэгдэнэ:",
    en: "Every organisation this month. Each metric is rolled up the way it means:",
  },
  "cp.hint.schedules": {
    mn: "Нүүр хуудсан дээрх тоо аль хуваарийнх болохыг энд харна. Асуудалтай нь эхэндээ.",
    en: "Which schedule the front page is counting. The ones in trouble come first.",
  },
  "cp.hint.backups": {
    mn: "Юу хадгалагдсан, хэн нь сэргээж үзсэн. Туршиж үзээгүй нөөц бол нөөц биш.",
    en: "What has been kept, and whether anybody has checked that it restores. An untested backup is not a backup.",
  },

  "cp.metric.rps": { mn: "Хүсэлт/сек", en: "Requests/sec" },
  "cp.metric.error_rate": { mn: "Алдааны хувь", en: "Error rate" },
  "cp.metric.p95": { mn: "P95 хугацаа", en: "P95 latency" },

  "cp.field.gauge": { mn: "Хэмжүүр", en: "Gauge" },
  "cp.field.warning_at": { mn: "Анхааруулах босго", en: "Warns at" },
  "cp.field.system": { mn: "Систем", en: "System" },
  "cp.field.since": { mn: "Хэзээнээс", en: "Since" },
  "cp.field.pending": { mn: "Хүлээгдэж буй", en: "Pending" },
  "cp.field.sample": { mn: "Жишээ", en: "Sample" },
  "cp.field.collected": { mn: "Тоологдсон", en: "Counted" },
  "cp.field.report": { mn: "Тайлан", en: "Report" },
  "cp.field.cron": { mn: "Хуваарь", en: "Schedule" },
  "cp.field.recipients": { mn: "Хүлээн авагч", en: "Recipients" },
  "cp.field.size": { mn: "Хэмжээ", en: "Size" },
  "cp.field.detail": { mn: "Тайлбар", en: "Detail" },
  "cp.kind.backup": { mn: "Нөөц", en: "Backup" },
  "cp.kind.restore_test": { mn: "Сэргээлтийн турших", en: "Restore test" },
  "cp.action.runbook": { mn: "Заавар", en: "Runbook" },

  // The four states the monitoring panels speak in. "unknown" is a system
  // Prometheus holds no sample for, and it is not a colour of health.
  "cp.state.green": { mn: "Хэвийн", en: "Green" },
  "cp.state.amber": { mn: "Анхаар", en: "Amber" },
  "cp.state.red": { mn: "Ноцтой", en: "Red" },
  "cp.state.unknown": { mn: "Хэмжигдээгүй", en: "Not measured" },
  "cp.state.unmeasured": { mn: "хэмжигдээгүй", en: "not measured" },
  "cp.state.normal": { mn: "Хэвийн", en: "Normal" },
  "cp.state.never_counted": { mn: "тоологдоогүй", en: "never counted" },

  "cp.message.nothing_firing": { mn: "Одоогоор асаж буй сэрэмжлүүлэг алга.", en: "Nothing is firing." },
  "cp.message.no_warnings": { mn: "Тохиргооны анхааруулга алга.", en: "No configuration warnings." },
  "cp.message.no_trouble": { mn: "Давтагдсан алдаатай байгууллага алга.", en: "No organisation is failing repeatedly." },
  "cp.message.no_schedules": { mn: "Товлосон тайлан алга.", en: "Nothing is scheduled." },
  "cp.message.schedules_trouble": {
    mn: "{failing} хуваарь алдаатай, {never} нь хэзээ ч ажиллаж үзээгүй.",
    en: "{failing} schedules are failing and {never} have never run.",
  },
  "cp.message.no_backups_configured": {
    mn: "Нөөцлөлт хэзээ ч бүртгэгдээгүй байна. deploy/scripts/backup.sh-г cron-д тавина уу.",
    en: "No backup has ever been recorded. Install deploy/scripts/backup.sh in cron.",
  },

  // Who may reach this console — the screen that replaced a shell on the
  // production host for everybody after the first operator.
  "cp.section.operators": { mn: "Операторууд", en: "Operators" },
  "cp.hint.operators": {
    mn: "Энэ консолд хэн хүрч болох вэ. Оператор нэмэх нь бүх бусад үйлдлийг хийж чадах хүмүүсийг өргөжүүлдэг тул зөвхөн ерөнхий админ, хоёр дахь хүчин зүйлтэй, бүртгэл үлдээж хийнэ.",
    en: "Who can reach this console. Adding one widens the set of people who can do everything else, so it is superadmin only, with a second factor, and it leaves a record.",
  },
  "cp.action.add_operator": { mn: "Оператор нэмэх", en: "Add operator" },
  "cp.action.change_password": { mn: "Нууц үг солих", en: "Change password" },
  "cp.action.change_role": { mn: "Эрх солих", en: "Change role" },
  "cp.action.disable": { mn: "Идэвхгүй болгох", en: "Disable" },
  "cp.action.enable": { mn: "Идэвхжүүлэх", en: "Enable" },
  "cp.field.secret": { mn: "Нууц түлхүүр", en: "Secret" },
  "cp.field.current_password": { mn: "Одоогийн нууц үг", en: "Current password" },
  "cp.field.new_password": { mn: "Шинэ нууц үг", en: "New password" },
  "cp.field.repeat_password": { mn: "Дахин", en: "Repeat" },
  "cp.state.disabled": { mn: "Идэвхгүй", en: "Disabled" },
  "cp.state.enrolment_pending": { mn: "Баталгаажаагүй", en: "Not enrolled" },
  "cp.state.you": { mn: "та", en: "you" },
  "cp.view.handover": { mn: "Шинэ операторт дамжуулах", en: "Hand this over" },
  "cp.message.handover_once": {
    mn: "Эдгээр утга зөвхөн ОДОО харагдана. Нууц үг ба түлхүүрийг сервер дахин харуулж чадахгүй — хаавал дахин үүсгэхээс өөр арга байхгүй.",
    en: "These values exist only now. Nothing on the server can show the password or the secret again — close this and the account has to be created afresh.",
  },
  "cp.hint.confirm_enrolment": {
    mn: "Шинэ оператор QR-аа уншуулаад аппаасаа код оруулна. Үүнийг хийх хүртэл тэр хүн нэвтэрч чадахгүй.",
    en: "The new operator scans the QR and types the code their app shows. Until that is done they cannot sign in.",
  },
  "cp.hint.step_up": {
    mn: "Оператор нэмэхийн өмнө өөрийн баталгаажуулагчийн кодыг оруулна.",
    en: "Confirm your own authenticator before adding an operator.",
  },
  "cp.message.enrolled": { mn: "Баталгаажлаа. Тэр хүн одоо нэвтэрч чадна.", en: "Enrolled. They can sign in now." },
  "cp.message.passwords_differ": { mn: "Хоёр нууц үг таарахгүй байна.", en: "The two passwords are not the same." },

  // Tenant удирдлага — консолын гурав дахь апп: энэ суулгац дээрх
  // байгууллагууд, тэдний эрх, тэдэнд суусан аппууд.
  "cp.app.tenants": { mn: "Tenant удирдлага", en: "Tenant management" },
  "cp.group.entitlements": { mn: "Эрх ба суулгац", en: "Entitlements" },
  "cp.section.quotas": { mn: "Квот", en: "Limits" },
  "cp.section.installations": { mn: "Аппын суулгац", en: "Installations" },
  "cp.hint.quotas": {
    mn: "Аль байгууллагад ямар хязгаар тавигдсаныг нэг дор. Хязгаарыг байгууллагынх нь хуудаснаас, шалтгаантайгаар л тавина.",
    en: "Which limits are set where, in one place. Setting one stays on the organisation's own page, with a reason.",
  },
  "cp.hint.installations": {
    mn: "Аль апп аль байгууллагад суусан, ямар хувилбартай. Каталог нь зөвхөн тоог хэлдэг — энэ нь хэн болохыг хэлнэ.",
    en: "Which app is installed where, and on which version. The catalogue gives the count; this says who.",
  },
  "cp.field.app": { mn: "Апп", en: "App" },
  "cp.state.no_limit": { mn: "хязгааргүй", en: "no limit" },
  "cp.state.every_app": { mn: "Бүх апп", en: "Every app" },
  "cp.message.unlimited_count": {
    mn: "{count} байгууллагад хэрэглэгчийн хязгаар тавиагүй байна.",
    en: "{count} organisations have no limit on people.",
  },
  "cp.message.versions_in_the_field": {
    mn: "Талбар дээр хэд хэдэн хувилбар байна: {versions}",
    en: "More than one version is in the field: {versions}",
  },
  "cp.message.no_installations": { mn: "Суусан апп алга.", en: "Nothing is installed." },

  // Байгууллага нээх: дэлгэрэнгүйг нь бүртгэлээс, эхний админыг нь eID-ээр
  // баталгаажсан хүмүүсээс.
  "cp.action.add_person": { mn: "Хүн нэмэх", en: "Add person" },
  "cp.hint.member_is_chosen": {
    mn: "eID-ээр баталгаажсан хүмүүсээс сонгоно. Тэр хүн платформын хамгийн бага эрхтэйгээр («Хэрэглэгч») орно — түүнээс дээшхийг байгууллагын өөрийнх нь админ өгнө.",
    en: "Chosen from the people verified with eID. They arrive with the smallest role the platform has — anything above it is granted by the organisation's own administrator.",
  },
  "cp.action.look_up": { mn: "Бүртгэлээс хайх", en: "Look up" },
  "cp.field.admin": { mn: "Эхний админ", en: "First administrator" },
  "cp.field.search_people": { mn: "Нэр, и-мэйл, эсвэл регистрээр хайх", en: "Search by name, address or register number" },
  "cp.hint.admin_is_chosen": {
    mn: "eID-ээр нэвтэрч баталгаажсан хүмүүсээс сонгоно. Бичсэн хаяг биш, платформ өөрөө хараад баталсан хүн.",
    en: "Chosen from the people who have signed in with eID — somebody this platform watched prove who they are, rather than an address typed into a dialog.",
  },
  "cp.message.from_the_register": { mn: "Бүртгэлээс: {name}", en: "From the register: {name}" },
  "cp.message.already_in": { mn: "{count} байгууллагад", en: "in {count} organisations" },
  "cp.message.no_verified_people": {
    mn: "eID-ээр баталгаажсан хэрэглэгч алга. Тэр хүн эхлээд энэ платформ дээр eID-ээр нэг удаа нэвтрэх хэрэгтэй.",
    en: "Nobody has signed in with eID yet. The person has to sign in here once before they can be chosen.",
  },

  // Хэрэглэгч — платформ дээрх бүх бүртгэл, ба нэгийнх нь бүх зүйл.
  "cp.group.people": { mn: "Хэрэглэгч", en: "People" },
  "cp.section.people": { mn: "Бүх хэрэглэгч", en: "Everybody" },
  "cp.hint.people": {
    mn: "Энэ суулгац дээр бүртгэлтэй бүх хүн. Дэмжлэгийн дэлгэц нэг хүнийг хайдаг; энэ нь хүн амын тухай асуултад хариулна.",
    en: "Everybody with an account here. The help desk finds one person; this answers the questions about the population.",
  },
  "cp.metric.people": { mn: "Нийт бүртгэл", en: "Accounts" },
  "cp.metric.verified": { mn: "eID-ээр баталгаажсан", en: "Verified with eID" },
  "cp.metric.signed_in": { mn: "Нээлттэй session-той", en: "With an open session" },
  "cp.metric.homeless": { mn: "Байгууллагагүй", en: "In no organisation" },
  "cp.hint.homeless": {
    mn: "Бүртгэлтэй ч хэрэглэх газаргүй",
    en: "An account with nowhere to use it",
  },
  "cp.filter.everybody": { mn: "Бүгд", en: "Everybody" },
  "cp.filter.verified": { mn: "eID-тэй", en: "Verified" },
  "cp.filter.locked": { mn: "Түгжигдсэн", en: "Locked" },
  "cp.filter.homeless": { mn: "Байгууллагагүй", en: "No organisation" },
  "cp.field.identities": { mn: "Нэвтрэх аргууд", en: "Ways in" },
  "cp.field.organisations": { mn: "Байгууллагууд", en: "Organisations" },
  "cp.field.sessions": { mn: "Нээлттэй session", en: "Open sessions" },
  "cp.field.subject": { mn: "Танигч", en: "Subject" },
  "cp.field.linked": { mn: "Холбогдсон", en: "Linked" },
  "cp.field.joined": { mn: "Элссэн", en: "Joined" },
  "cp.field.last_seen": { mn: "Сүүлд харагдсан", en: "Last seen" },
  "cp.state.password_only": { mn: "зөвхөн нууц үг", en: "password only" },
  "cp.action.previous": { mn: "Өмнөх", en: "Previous" },
  "cp.action.next": { mn: "Дараах", en: "Next" },
  "cp.message.showing": { mn: "{total}-аас {shown} харуулж байна", en: "Showing {shown} of {total}" },
  "cp.message.password_only": {
    mn: "Гадаад таних тэмдэг холбогдоогүй — зөвхөн нууц үгээр нэвтэрнэ.",
    en: "No external identity is linked — this account signs in with a password only.",
  },
  "cp.message.no_organisations": {
    mn: "Ямар ч байгууллагад харьяалагдахгүй байна.",
    en: "This person is in no organisation.",
  },
  "cp.message.no_sessions": { mn: "Нээлттэй session алга.", en: "No session is open." },
  "cp.message.never_impersonated": {
    mn: "Энэ хүний нэрээр хэн ч ороогүй байна.",
    en: "Nobody has looked at the platform as this person.",
  },

  "cp.group.watch": { mn: "Ажиглалт", en: "Watch" },
  "cp.group.organisations": { mn: "Байгууллага", en: "Organisations" },
  "cp.group.platform": { mn: "Платформ", en: "Platform" },
  "cp.group.investigation": { mn: "Мөрдлөг", en: "Investigation" },
};
