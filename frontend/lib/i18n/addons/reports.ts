/**
 * reports — The reporting screen.
 *
 * Deliberately short. Almost every word on that screen — the report's name, its
 * parameters, its column headings — comes from the server, because the report
 * that owns them lives in a Go module and this file cannot know what reports a
 * deployment has. What is here is the chrome around them.
 */
export const reports = {
  "reports.view.title": { mn: "Тайлан", en: "Reports" },
  "reports.view.subtitle": {
    mn: "Байгууллагын суулгасан аппуудын тайлан. Ажиллуулах, график харах, Excel/CSV болгон гаргах.",
    en: "The reports of the apps this organisation has installed. Run them, chart them, export them.",
  },

  "reports.section.reports": { mn: "Тайлангууд", en: "Reports" },
  "reports.section.schedules": { mn: "Товлосон тайлан", en: "Scheduled reports" },
  "reports.section.parameters": { mn: "Үзүүлэлт", en: "Parameters" },
  "reports.section.chart": { mn: "График", en: "Chart" },

  "reports.action.run": { mn: "Ажиллуулах", en: "Run" },
  "reports.action.export_xlsx": { mn: "Excel", en: "Excel" },
  "reports.action.export_csv": { mn: "CSV", en: "CSV" },
  "reports.action.schedule": { mn: "Товлох", en: "Schedule" },
  "reports.action.new_schedule": { mn: "Шинэ хуваарь", en: "New schedule" },

  "reports.field.total": { mn: "Нийт", en: "Total" },
  "reports.field.rows": { mn: "мөр", en: "rows" },
  "reports.field.schedule_name": { mn: "Хуваарийн нэр", en: "Schedule name" },
  "reports.field.cron": { mn: "Хуваарь (cron)", en: "Schedule (cron)" },
  "reports.field.format": { mn: "Хэлбэр", en: "Format" },
  "reports.field.recipients": { mn: "Хүлээн авагчид", en: "Recipients" },
  "reports.field.last_run": { mn: "Сүүлд ажилласан", en: "Last run" },
  "reports.field.report": { mn: "Тайлан", en: "Report" },

  "reports.hint.cron": {
    mn: "Минут цаг өдөр сар гараг. Жишээ: сарын 1-нд 09:00 цагт бол «0 9 1 * *»",
    en: "Minute hour day month weekday. For the 1st of the month at 09:00: “0 9 1 * *”",
  },
  "reports.hint.recipients": {
    mn: "И-мэйл хаягуудыг таслалаар тусгаарлана.",
    en: "E-mail addresses, separated by commas.",
  },

  "reports.message.select": { mn: "Зүүн талаас тайлан сонгоно уу.", en: "Choose a report on the left." },
  "reports.message.empty": { mn: "Энэ хугацаанд өгөгдөл алга.", en: "No data for this period." },
  "reports.message.no_reports": {
    mn: "Тайлантай апп суулгаагүй байна. Апп сторооос апп суулгасны дараа түүний тайлангууд энд гарч ирнэ.",
    en: "No app with reports is installed. Install one from the store and its reports appear here.",
  },
  "reports.message.no_schedules": { mn: "Товлосон тайлан алга.", en: "No scheduled reports." },
  "reports.message.running": { mn: "Тайлан бэлтгэж байна...", en: "Producing the report…" },
  "reports.message.run_failed": { mn: "Тайлан ажиллуулж чадсангүй", en: "The report could not be produced" },
  "reports.message.export_failed": { mn: "Гаргаж чадсангүй", en: "The export failed" },
  "reports.message.schedule_saved": { mn: "Хуваарь хадгалагдлаа.", en: "The schedule was saved." },
  "reports.message.schedule_removed": { mn: "Хуваарь устгагдлаа.", en: "The schedule was removed." },
  "reports.action.run_consolidated": { mn: "Нэгдсэн", en: "Consolidated" },
  "reports.badge.consolidated": { mn: "Нэгдсэн", en: "Consolidated" },
  "reports.toggle.by_company": { mn: "Компаниар задлах", en: "Break down by organisation" },
  "reports.hint.consolidated": {
    mn: "Танд энэ тайланг хуваалцсан байгууллага бүрийн дүн. Хуваалцаагүй бол юу ч харагдахгүй — Тохиргоо → Тайлан хуваалцах.",
    en: "The figures of every organisation that shares this report with you. Nothing is shown without an agreement — Settings → Report sharing.",
  },
  "reports.message.delivery_off": {
    mn: "Энэ суулгацад и-мэйл илгээх тохиргоо байхгүй тул товлосон тайлан бэлтгэгдэх боловч илгээгдэхгүй. REPORT_SMTP_URL-ийг тохируулна уу.",
    en: "This deployment has no mail transport, so a scheduled report is produced but not delivered. Set REPORT_SMTP_URL.",
  },
} as const;
