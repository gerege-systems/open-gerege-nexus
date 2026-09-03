"use client";

/**
 * The reporting screen.
 *
 * It is written against the engine rather than against any report: it asks the
 * API which reports this organisation may run, asks a report to describe itself,
 * renders a form from that description, and draws whatever comes back. Adding a
 * report to a Go module puts it on this screen with no change here — which is
 * the point of a Report being a declaration.
 */

import React, { useCallback, useEffect, useMemo, useState } from "react";
import {
  BarChart3,
  Building2,
  CalendarClock,
  Download,
  FileSpreadsheet,
  Play,
  Plus,
  Trash2,
} from "lucide-react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import {
  api,
  type ReportColumn,
  type ReportGroup,
  type ReportMetadata,
  type ReportResult,
  type ReportSchedule,
  type ReportSummary,
} from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import {
  Banner,
  EmptyState,
  LoadingBlock,
  Modal,
  PageHeader,
  cardClass,
  fieldClass,
  tableHeadClass,
} from "@/components/ui";

// The app ids the platform ships, so a group heading reads as a name rather
// than as a reverse-domain string. An app not in this list falls back to its
// id, which is the right answer for a third party's module.
const APP_NAMES: Record<string, { mn: string; en: string }> = {
  "io.gerege.nexus.organisation": { mn: "Байгууллага", en: "Organisation" },
  "io.gerege.nexus.billing": { mn: "Нэхэмжлэх", en: "Billing" },
  "io.gerege.nexus.inventory": { mn: "Агуулах", en: "Inventory" },
  "io.gerege.nexus.esign": { mn: "Цахим гарын үсэг", en: "E-signature" },
  "io.gerege.nexus.documents": { mn: "Баримт бичиг", en: "Documents" },
  "io.gerege.nexus.contacts": { mn: "Харилцагч", en: "Contacts" },
  "io.gerege.nexus.products": { mn: "Бараа", en: "Products" },
};

/**
 * `appFilter` narrows the screen to one app's reports — the documents app
 * mounts this same screen at /module/documents/reports so its reports live
 * INSIDE the app, beside the contracts they describe. Absent, every installed
 * app's reports are listed, which is what the standalone /reports page wants.
 */
export default function ReportsPage({ appFilter }: { appFilter?: string } = {}) {
  const { t, locale } = useI18n();

  const [groups, setGroups] = useState<ReportGroup[]>([]);
  const [selected, setSelected] = useState<ReportSummary | null>(null);
  const [metadata, setMetadata] = useState<ReportMetadata | null>(null);
  const [values, setValues] = useState<Record<string, string>>({});
  const [result, setResult] = useState<ReportResult | null>(null);
  const [title, setTitle] = useState("");
  // Whether the last run crossed organisations. Kept beside the result rather
  // than derived from it: the two runs answer different questions and the
  // table must never be labelled as one while showing the other.
  const [consolidated, setConsolidated] = useState(false);
  const [byCompany, setByCompany] = useState(true);

  const [loading, setLoading] = useState(true);
  const [running, setRunning] = useState(false);
  const [failure, setFailure] = useState("");
  const [notice, setNotice] = useState("");

  const [schedules, setSchedules] = useState<ReportSchedule[]>([]);
  const [deliveryConfigured, setDeliveryConfigured] = useState(true);
  const [scheduleOpen, setScheduleOpen] = useState(false);

  const label = useCallback(
    (titles: Record<string, string> | undefined, fallback: string) =>
      titles?.[locale] || titles?.mn || titles?.en || fallback,
    [locale],
  );

  useEffect(() => {
    (async () => {
      try {
        const [list, scheduleList] = await Promise.all([
          api.getReports(),
          api.getReportSchedules(),
        ]);
        setGroups((list.groups || []).filter((group) => !appFilter || group.app === appFilter));
        setSchedules(scheduleList.schedules || []);
        setDeliveryConfigured(scheduleList.delivery_configured);
      } catch (err) {
        setFailure(err instanceof Error ? err.message : String(err));
      } finally {
        setLoading(false);
      }
    })();
  }, [appFilter]);

  // Choosing a report clears the previous one's rows before the new
  // declaration arrives: leaving them up under a new heading is how somebody
  // reads one report's numbers as another's.
  const choose = async (report: ReportSummary) => {
    setSelected(report);
    setResult(null);
    setFailure("");
    setMetadata(null);
    try {
      const meta = await api.getReport(report.key);
      setMetadata(meta);
      setValues(defaultsFor(meta));
    } catch (err) {
      setFailure(err instanceof Error ? err.message : String(err));
    }
  };

  const run = async (across = false) => {
    if (!selected) return;
    setRunning(true);
    setFailure("");
    try {
      const answer = across
        ? await api.runConsolidatedReport(selected.key, values)
        : await api.runReport(selected.key, values);
      setResult(answer.result);
      setTitle(answer.title);
      setConsolidated(across);
    } catch (err) {
      setFailure(`${t("reports.message.run_failed")}: ${err instanceof Error ? err.message : err}`);
    } finally {
      setRunning(false);
    }
  };

  const download = async (format: "xlsx" | "csv") => {
    if (!selected) return;
    try {
      const { blob, filename } = await api.exportReport(selected.key, values, format);
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = filename;
      anchor.click();
      URL.revokeObjectURL(url);
    } catch {
      setFailure(t("reports.message.export_failed"));
    }
  };

  const reloadSchedules = async () => {
    const list = await api.getReportSchedules();
    setSchedules(list.schedules || []);
    setDeliveryConfigured(list.delivery_configured);
  };

  const removeSchedule = async (id: string) => {
    await api.deleteReportSchedule(id);
    setNotice(t("reports.message.schedule_removed"));
    reloadSchedules();
  };

  // The organisation breakdown, folded away when the reader wants one set of
  // figures. Merging is done here rather than asked of the server: the rows are
  // already in hand, and a second round trip to see the same numbers grouped
  // differently is a round trip for nothing.
  const displayed = useMemo(
    () => (consolidated && !byCompany && result ? mergeOrganisations(result) : result),
    [consolidated, byCompany, result],
  );
  const chart = useMemo(() => chartShape(displayed), [displayed]);

  if (loading) return <LoadingBlock label={t("base.message.loading")} />;

  const hasReports = groups.some((group) => group.reports.length > 0);

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<BarChart3 className="w-7 h-7 text-indigo-600" />}
        title={t("reports.view.title")}
        subtitle={t("reports.view.subtitle")}
        actions={
          selected ? (
            <div className="flex flex-wrap gap-2">
              <button
                onClick={() => run(false)}
                disabled={running}
                className="bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 text-white text-xs font-semibold px-4 py-2 rounded-lg flex items-center gap-2 shadow-sm transition"
              >
                <Play className="w-4 h-4" />
                {t("reports.action.run")}
              </button>
              {/* The consolidated run is a separate button rather than a
                  mode, because it answers a different question and a toggle
                  somebody left on would silently change what the numbers mean. */}
              <button
                onClick={() => run(true)}
                disabled={running}
                className="bg-white border border-indigo-200 hover:bg-indigo-50 disabled:opacity-50 text-indigo-700 text-xs font-semibold px-4 py-2 rounded-lg flex items-center gap-2 transition"
                title={t("reports.hint.consolidated")}
              >
                <Building2 className="w-4 h-4" />
                {t("reports.action.run_consolidated")}
              </button>
              <button
                onClick={() => download("xlsx")}
                disabled={!result}
                className="bg-white border border-slate-200 hover:bg-slate-50 disabled:opacity-50 text-slate-700 text-xs font-semibold px-4 py-2 rounded-lg flex items-center gap-2 transition"
              >
                <FileSpreadsheet className="w-4 h-4" />
                {t("reports.action.export_xlsx")}
              </button>
              <button
                onClick={() => download("csv")}
                disabled={!result}
                className="bg-white border border-slate-200 hover:bg-slate-50 disabled:opacity-50 text-slate-700 text-xs font-semibold px-4 py-2 rounded-lg flex items-center gap-2 transition"
              >
                <Download className="w-4 h-4" />
                {t("reports.action.export_csv")}
              </button>
              <button
                onClick={() => setScheduleOpen(true)}
                className="bg-white border border-slate-200 hover:bg-slate-50 text-slate-700 text-xs font-semibold px-4 py-2 rounded-lg flex items-center gap-2 transition"
              >
                <CalendarClock className="w-4 h-4" />
                {t("reports.action.schedule")}
              </button>
            </div>
          ) : undefined
        }
      />

      {failure && <Banner tone="error" message={failure} onDismiss={() => setFailure("")} />}
      {notice && <Banner tone="success" message={notice} onDismiss={() => setNotice("")} />}

      <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
        {/* The list, grouped by app. The grouping is the app gate made
            visible: a section for an app this organisation does not have
            simply is not here. */}
        <aside className={`${cardClass} p-4 lg:col-span-1 h-max`}>
          <h2 className="text-xs font-semibold uppercase tracking-wide text-slate-400 mb-3">
            {t("reports.section.reports")}
          </h2>
          {!hasReports ? (
            <p className="text-sm text-slate-500">{t("reports.message.no_reports")}</p>
          ) : (
            <div className="space-y-4">
              {groups.map((group) => (
                <div key={group.app}>
                  <p className="text-[11px] font-bold uppercase tracking-wide text-slate-500 mb-1">
                    {APP_NAMES[group.app]?.[locale === "en" ? "en" : "mn"] || group.app}
                  </p>
                  <ul className="space-y-1">
                    {group.reports.map((report) => (
                      <li key={report.key}>
                        <button
                          onClick={() => choose(report)}
                          className={`w-full text-left text-sm px-3 py-2 rounded-lg transition ${
                            selected?.key === report.key
                              ? "bg-indigo-50 text-indigo-700 font-semibold"
                              : "text-slate-700 hover:bg-slate-50"
                          }`}
                        >
                          {label(report.titles, report.key)}
                        </button>
                      </li>
                    ))}
                  </ul>
                </div>
              ))}
            </div>
          )}
        </aside>

        <section className="lg:col-span-3 space-y-6">
          {!selected && <EmptyState message={t("reports.message.select")} />}

          {metadata && metadata.params.length > 0 && (
            <div className={`${cardClass} p-4`}>
              <h2 className="text-xs font-semibold uppercase tracking-wide text-slate-400 mb-3">
                {t("reports.section.parameters")}
              </h2>
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                {metadata.params.map((param) => (
                  <ParamField
                    key={param.key}
                    param={param}
                    values={values}
                    onChange={(key, value) => setValues((prev) => ({ ...prev, [key]: value }))}
                    label={label}
                  />
                ))}
              </div>
            </div>
          )}

          {running && <LoadingBlock label={t("reports.message.running")} />}

          {result && displayed && !running && (
            <>
              {result.notes?.map((note, index) => (
                <Banner
                  key={index}
                  tone={note.level === "warning" ? "warning" : "info"}
                  message={note.message}
                />
              ))}

              {chart && (
                <div className={`${cardClass} p-4`}>
                  <h2 className="text-xs font-semibold uppercase tracking-wide text-slate-400 mb-3">
                    {t("reports.section.chart")}
                  </h2>
                  <div className="h-72">
                    <ResponsiveContainer width="100%" height="100%">
                      <BarChart data={chart.data}>
                        <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
                        <XAxis dataKey="__category" tick={{ fontSize: 11 }} />
                        <YAxis tick={{ fontSize: 11 }} />
                        <Tooltip />
                        <Legend wrapperStyle={{ fontSize: 12 }} />
                        {chart.series.map((series, index) => (
                          <Bar
                            key={series.key}
                            dataKey={series.key}
                            name={label(series.titles, series.key)}
                            fill={SERIES_COLOURS[index % SERIES_COLOURS.length]}
                          />
                        ))}
                      </BarChart>
                    </ResponsiveContainer>
                  </div>
                </div>
              )}

              {consolidated && (
                <div className="flex flex-wrap items-center gap-3">
                  <span className="bg-indigo-50 text-indigo-700 text-[11px] font-semibold px-2.5 py-1 rounded-full border border-indigo-200 flex items-center gap-1">
                    <Building2 className="w-3 h-3" />
                    {t("reports.badge.consolidated")}
                  </span>
                  <label className="flex items-center gap-2 text-xs text-slate-600">
                    <input
                      type="checkbox"
                      checked={byCompany}
                      onChange={(e) => setByCompany(e.target.checked)}
                      className="rounded border-slate-300"
                    />
                    {t("reports.toggle.by_company")}
                  </label>
                </div>
              )}

              <ResultTable result={displayed} title={title} label={label} totalLabel={t("reports.field.total")} rowsLabel={t("reports.field.rows")} emptyLabel={t("reports.message.empty")} locale={locale} />
            </>
          )}

          <SchedulesCard
            schedules={schedules}
            deliveryConfigured={deliveryConfigured}
            onRemove={removeSchedule}
            onAdd={() => setScheduleOpen(true)}
            canAdd={Boolean(selected)}
            label={label}
          />
        </section>
      </div>

      {scheduleOpen && selected && (
        <ScheduleModal
          reportKey={selected.key}
          reportTitle={label(selected.titles, selected.key)}
          values={values}
          onClose={() => setScheduleOpen(false)}
          onSaved={async () => {
            setScheduleOpen(false);
            setNotice(t("reports.message.schedule_saved"));
            await reloadSchedules();
          }}
        />
      )}
    </div>
  );
}

// Indigo through to amber: five hues that stay distinguishable in the order
// they are handed out, which matters because a report decides its own series.
const SERIES_COLOURS = ["#4f46e5", "#0ea5e9", "#10b981", "#f59e0b", "#ef4444"];

/** defaultsFor seeds the form. Dates are left blank so the server's own
 *  default window applies — every report declares its own, and guessing one
 *  here would override it. */
function defaultsFor(meta: ReportMetadata): Record<string, string> {
  const values: Record<string, string> = {};
  for (const param of meta.params) {
    if (param.kind === "select" && typeof param.default === "string") {
      values[param.key] = param.default;
    }
    if (param.kind === "bool" && typeof param.default === "boolean") {
      values[param.key] = String(param.default);
    }
  }
  return values;
}

function ParamField({
  param,
  values,
  onChange,
  label,
}: {
  param: ReportMetadata["params"][number];
  values: Record<string, string>;
  onChange: (key: string, value: string) => void;
  label: (titles: Record<string, string> | undefined, fallback: string) => string;
}) {
  const name = label(param.titles, param.key);

  if (param.kind === "date_range") {
    return (
      <div className="sm:col-span-2">
        <label className="block text-xs font-semibold text-slate-600 mb-1">{name}</label>
        <div className="flex items-center gap-2">
          <input
            type="date"
            className={fieldClass}
            value={values[`${param.key}_from`] || ""}
            onChange={(e) => onChange(`${param.key}_from`, e.target.value)}
          />
          <span className="text-slate-400">—</span>
          <input
            type="date"
            className={fieldClass}
            value={values[`${param.key}_to`] || ""}
            onChange={(e) => onChange(`${param.key}_to`, e.target.value)}
          />
        </div>
      </div>
    );
  }

  if (param.kind === "select" || param.kind === "uuid") {
    return (
      <div>
        <label className="block text-xs font-semibold text-slate-600 mb-1">{name}</label>
        <select
          className={fieldClass}
          value={values[param.key] || ""}
          onChange={(e) => onChange(param.key, e.target.value)}
        >
          <option value="">—</option>
          {(param.options || []).map((option) => (
            <option key={option.value} value={option.value}>
              {label(option.titles, option.value)}
            </option>
          ))}
        </select>
      </div>
    );
  }

  if (param.kind === "bool") {
    return (
      <label className="flex items-center gap-2 mt-6 text-sm text-slate-700">
        <input
          type="checkbox"
          checked={values[param.key] === "true"}
          onChange={(e) => onChange(param.key, String(e.target.checked))}
          className="rounded border-slate-300"
        />
        {name}
      </label>
    );
  }

  return (
    <div>
      <label className="block text-xs font-semibold text-slate-600 mb-1">{name}</label>
      <input
        type="text"
        className={fieldClass}
        value={values[param.key] || ""}
        onChange={(e) => onChange(param.key, e.target.value)}
      />
    </div>
  );
}

/** mergeOrganisations folds a consolidated result back into one set of figures.
 *
 *  Every row keyed by the same category is summed across organisations, and the
 *  organisation column is dropped. Text columns other than the category are
 *  dropped too rather than being picked arbitrarily from one company's row —
 *  showing one organisation's label above another's numbers is worse than
 *  showing none. */
function mergeOrganisations(result: ReportResult): ReportResult {
  const columns = result.columns.filter((column) => column.key !== "__organisation");
  const category = columns.find((column) => column.chart === "category");
  if (!category) {
    return { ...result, columns };
  }

  const grouped = new Map<string, Record<string, unknown>>();
  for (const row of result.rows) {
    const key = String(row[category.key] ?? "");
    const existing = grouped.get(key);
    if (!existing) {
      const copy: Record<string, unknown> = { [category.key]: row[category.key] };
      for (const column of columns) {
        if (isNumeric(column)) copy[column.key] = Number(row[column.key] ?? 0);
      }
      grouped.set(key, copy);
      continue;
    }
    for (const column of columns) {
      if (isNumeric(column)) {
        existing[column.key] = Number(existing[column.key] ?? 0) + Number(row[column.key] ?? 0);
      }
    }
  }

  return { ...result, columns, rows: Array.from(grouped.values()) };
}

/** chartShape decides whether a result can be drawn, using the hints the report
 *  declared: exactly one category column and at least one value column. Nothing
 *  else is guessed — a report that does not say how it should be charted is
 *  shown as a table, which is the honest outcome. */
function chartShape(result: ReportResult | null | undefined) {
  if (!result || result.rows.length === 0) return null;

  const category = result.columns.find((column) => column.chart === "category");
  const series = result.columns.filter((column) => column.chart === "value");
  if (!category || series.length === 0) return null;

  const data = result.rows.slice(0, 60).map((row) => {
    const point: Record<string, unknown> = { __category: formatCell(row[category.key], category, "mn") };
    for (const column of series) {
      point[column.key] = Number(row[column.key] ?? 0);
    }
    return point;
  });
  return { data, series };
}

function ResultTable({
  result,
  title,
  label,
  totalLabel,
  rowsLabel,
  emptyLabel,
  locale,
}: {
  result: ReportResult;
  title: string;
  label: (titles: Record<string, string> | undefined, fallback: string) => string;
  totalLabel: string;
  rowsLabel: string;
  emptyLabel: string;
  locale: string;
}) {
  if (result.rows.length === 0) {
    return <EmptyState message={emptyLabel} />;
  }

  return (
    <div className={`${cardClass} overflow-hidden`}>
      <div className="px-4 py-3 border-b border-slate-100 flex items-center justify-between">
        <h2 className="text-sm font-semibold text-slate-800">{title}</h2>
        <span className="text-xs text-slate-400">
          {result.rows.length} {rowsLabel}
        </span>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead className={tableHeadClass}>
            <tr>
              {result.columns.map((column) => (
                <th
                  key={column.key}
                  className={`px-4 py-3 ${isNumeric(column) ? "text-right" : "text-left"}`}
                >
                  {label(column.titles, column.key)}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {result.rows.map((row, index) => (
              <tr key={index} className="hover:bg-slate-50">
                {result.columns.map((column) => (
                  <td
                    key={column.key}
                    className={`px-4 py-2.5 ${
                      isNumeric(column) ? "text-right tabular-nums text-slate-800" : "text-slate-700"
                    }`}
                  >
                    {formatCell(row[column.key], column, locale)}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
          {result.totals && Object.keys(result.totals).length > 0 && (
            <tfoot className="bg-slate-50 border-t-2 border-slate-200">
              <tr>
                {result.columns.map((column, index) => (
                  <td
                    key={column.key}
                    className={`px-4 py-2.5 font-semibold ${
                      isNumeric(column) ? "text-right tabular-nums" : "text-left"
                    }`}
                  >
                    {index === 0
                      ? totalLabel
                      : column.total
                        ? formatCell(result.totals?.[column.key], column, locale)
                        : ""}
                  </td>
                ))}
              </tr>
            </tfoot>
          )}
        </table>
      </div>
    </div>
  );
}

function SchedulesCard({
  schedules,
  deliveryConfigured,
  onRemove,
  onAdd,
  canAdd,
  label,
}: {
  schedules: ReportSchedule[];
  deliveryConfigured: boolean;
  onRemove: (id: string) => void;
  onAdd: () => void;
  canAdd: boolean;
  label: (titles: Record<string, string> | undefined, fallback: string) => string;
}) {
  const { t } = useI18n();

  return (
    <div className={`${cardClass} p-4`}>
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-xs font-semibold uppercase tracking-wide text-slate-400">
          {t("reports.section.schedules")}
        </h2>
        {canAdd && (
          <button
            onClick={onAdd}
            className="text-xs font-semibold text-indigo-600 hover:text-indigo-700 flex items-center gap-1"
          >
            <Plus className="w-3.5 h-3.5" />
            {t("reports.action.new_schedule")}
          </button>
        )}
      </div>

      {/* Said once, where it matters: a schedule created on a deployment with
          no mail transport looks active and delivers nothing. */}
      {!deliveryConfigured && schedules.length > 0 && (
        <div className="mb-3">
          <Banner tone="warning" message={t("reports.message.delivery_off")} />
        </div>
      )}

      {schedules.length === 0 ? (
        <p className="text-sm text-slate-500">{t("reports.message.no_schedules")}</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead className={tableHeadClass}>
              <tr>
                <th className="px-3 py-2 text-left">{t("reports.field.report")}</th>
                <th className="px-3 py-2 text-left">{t("reports.field.cron")}</th>
                <th className="px-3 py-2 text-left">{t("reports.field.recipients")}</th>
                <th className="px-3 py-2 text-left">{t("reports.field.last_run")}</th>
                <th className="px-3 py-2" />
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {schedules.map((schedule) => (
                <tr key={schedule.id} className="hover:bg-slate-50">
                  <td className="px-3 py-2 text-slate-800">
                    {schedule.name || label(schedule.titles, schedule.report_key)}
                  </td>
                  <td className="px-3 py-2 font-mono text-xs text-slate-600">{schedule.cron}</td>
                  <td className="px-3 py-2 text-slate-600">{schedule.recipients.join(", ")}</td>
                  <td className="px-3 py-2 text-xs">
                    {schedule.last_run_at ? (
                      <span
                        className={
                          schedule.last_status === "FAILED" ? "text-red-600" : "text-slate-500"
                        }
                        title={schedule.last_error || undefined}
                      >
                        {new Date(schedule.last_run_at).toLocaleString()}
                        {schedule.last_status === "FAILED" ? " ⚠" : ""}
                      </span>
                    ) : (
                      <span className="text-slate-400">—</span>
                    )}
                  </td>
                  <td className="px-3 py-2 text-right">
                    <button
                      onClick={() => onRemove(schedule.id)}
                      className="text-slate-400 hover:text-red-600 transition"
                      aria-label={t("base.action.delete")}
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function ScheduleModal({
  reportKey,
  reportTitle,
  values,
  onClose,
  onSaved,
}: {
  reportKey: string;
  reportTitle: string;
  values: Record<string, string>;
  onClose: () => void;
  onSaved: () => void;
}) {
  const { t } = useI18n();
  const [name, setName] = useState(reportTitle);
  const [cron, setCron] = useState("0 9 1 * *");
  const [format, setFormat] = useState<"xlsx" | "csv">("xlsx");
  const [recipients, setRecipients] = useState("");
  const [failure, setFailure] = useState("");
  const [saving, setSaving] = useState(false);

  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    setSaving(true);
    setFailure("");
    try {
      await api.createReportSchedule({
        report_key: reportKey,
        name,
        params: values,
        cron,
        format,
        recipients: recipients
          .split(",")
          .map((address) => address.trim())
          .filter(Boolean),
      });
      onSaved();
    } catch (err) {
      setFailure(err instanceof Error ? err.message : String(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal label={t("reports.action.schedule")}>
      <h2 className="text-xl font-bold text-slate-900 mb-4">{t("reports.action.schedule")}</h2>
      <form onSubmit={save} className="space-y-4">
        {failure && <Banner tone="error" message={failure} />}

        <div>
          <label className="block text-xs font-semibold text-slate-600 mb-1">
            {t("reports.field.schedule_name")}
          </label>
          <input className={fieldClass} value={name} onChange={(e) => setName(e.target.value)} />
        </div>

        <div>
          <label className="block text-xs font-semibold text-slate-600 mb-1">
            {t("reports.field.cron")}
          </label>
          <input
            className={`${fieldClass} font-mono`}
            value={cron}
            onChange={(e) => setCron(e.target.value)}
            required
          />
          <p className="text-[11px] text-slate-400 mt-1">{t("reports.hint.cron")}</p>
        </div>

        <div>
          <label className="block text-xs font-semibold text-slate-600 mb-1">
            {t("reports.field.format")}
          </label>
          <select
            className={fieldClass}
            value={format}
            onChange={(e) => setFormat(e.target.value as "xlsx" | "csv")}
          >
            <option value="xlsx">Excel (.xlsx)</option>
            <option value="csv">CSV</option>
          </select>
        </div>

        <div>
          <label className="block text-xs font-semibold text-slate-600 mb-1">
            {t("reports.field.recipients")}
          </label>
          <input
            className={fieldClass}
            value={recipients}
            onChange={(e) => setRecipients(e.target.value)}
            placeholder="a@example.mn, b@example.mn"
            required
          />
          <p className="text-[11px] text-slate-400 mt-1">{t("reports.hint.recipients")}</p>
        </div>

        <div className="flex justify-end gap-2 pt-2">
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 text-xs font-semibold text-slate-600 hover:text-slate-800"
          >
            {t("base.action.cancel")}
          </button>
          <button
            type="submit"
            disabled={saving}
            className="bg-indigo-600 hover:bg-indigo-700 disabled:opacity-50 text-white text-xs font-semibold px-4 py-2 rounded-lg"
          >
            {t("base.action.save")}
          </button>
        </div>
      </form>
    </Modal>
  );
}

function isNumeric(column: ReportColumn): boolean {
  return column.kind === "number" || column.kind === "money" || column.kind === "percent";
}

/** formatCell renders one value the way its column declared it. The server
 *  sends numbers as numbers and dates as ISO strings; the shaping is done here
 *  so the same Result can be a table, a chart and an export without three
 *  disagreeing ideas of what a money column looks like. */
function formatCell(value: unknown, column: ReportColumn, locale: string): string {
  if (value === null || value === undefined || value === "") return "—";

  switch (column.kind) {
    case "money":
      return `₮${Number(value).toLocaleString(locale === "en" ? "en-US" : "mn-MN", {
        minimumFractionDigits: 2,
        maximumFractionDigits: 2,
      })}`;
    case "number":
      return Number(value).toLocaleString(locale === "en" ? "en-US" : "mn-MN");
    case "percent":
      return `${(Number(value) * 100).toFixed(1)}%`;
    case "month":
      return String(value).slice(0, 7);
    case "date":
      return String(value).slice(0, 10);
    default:
      return String(value);
  }
}
