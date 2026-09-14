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

import React, { useCallback, useEffect, useId, useMemo, useState } from "react";
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
  Alert,
  Badge,
  Button,
  Card,
  Checkbox,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  EmptyState,
  IconButton,
  Input,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableFooter,
  TableHead,
  TableHeader,
  TableRow,
} from "@gerege-systems/ui";

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
import { PageHeader } from "@/components/ui";

// Radix Select refuses an empty-string item value, so "no choice" is carried
// under a sentinel and written back to the form as "".
const NO_CHOICE = "__none__";

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

  if (loading) return <RowsPlaceholder label={t("base.message.loading")} />;

  const hasReports = groups.some((group) => group.reports.length > 0);

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<BarChart3 className="w-7 h-7 text-accent" />}
        title={t("reports.view.title")}
        subtitle={t("reports.view.subtitle")}
        actions={
          selected ? (
            <div className="flex flex-wrap gap-2">
              <Button size="sm" disabled={running} leadingIcon={<Play />} onClick={() => run(false)}>
                {t("reports.action.run")}
              </Button>
              {/* The consolidated run is a separate button rather than a
                  mode, because it answers a different question and a toggle
                  somebody left on would silently change what the numbers mean. */}
              <Button
                size="sm"
                variant="outline"
                className="border-accent-border text-accent hover:bg-accent-soft"
                disabled={running}
                leadingIcon={<Building2 />}
                title={t("reports.hint.consolidated")}
                onClick={() => run(true)}
              >
                {t("reports.action.run_consolidated")}
              </Button>
              <Button size="sm" variant="outline" disabled={!result} leadingIcon={<FileSpreadsheet />} onClick={() => download("xlsx")}>
                {t("reports.action.export_xlsx")}
              </Button>
              <Button size="sm" variant="outline" disabled={!result} leadingIcon={<Download />} onClick={() => download("csv")}>
                {t("reports.action.export_csv")}
              </Button>
              <Button size="sm" variant="outline" leadingIcon={<CalendarClock />} onClick={() => setScheduleOpen(true)}>
                {t("reports.action.schedule")}
              </Button>
            </div>
          ) : undefined
        }
      />

      {failure && <Alert variant="danger" live dismissible onDismiss={() => setFailure("")}>{failure}</Alert>}
      {notice && <Alert variant="success" live dismissible onDismiss={() => setNotice("")}>{notice}</Alert>}

      <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
        {/* The list, grouped by app. The grouping is the app gate made
            visible: a section for an app this organisation does not have
            simply is not here. */}
        <Card asChild padding="sm" className="lg:col-span-1 h-max">
          <aside>
            <h2 className="text-xs font-semibold uppercase tracking-wide text-subtle mb-3">
              {t("reports.section.reports")}
            </h2>
            {!hasReports ? (
              <EmptyState icon={<BarChart3 className="size-6" />} title={t("reports.message.no_reports")} className="py-6" />
            ) : (
              <div className="space-y-4">
                {groups.map((group) => (
                  <div key={group.app}>
                    <p className="text-xs font-bold uppercase tracking-wide text-muted mb-1">
                      {APP_NAMES[group.app]?.[locale === "en" ? "en" : "mn"] || group.app}
                    </p>
                    <ul className="space-y-1">
                      {group.reports.map((report) => (
                        <li key={report.key}>
                          <Button
                            variant="ghost"
                            size="sm"
                            aria-pressed={selected?.key === report.key}
                            onClick={() => choose(report)}
                            className={`w-full justify-start text-start font-normal ${
                              selected?.key === report.key ? "bg-accent-soft text-accent font-semibold" : ""
                            }`}
                          >
                            {label(report.titles, report.key)}
                          </Button>
                        </li>
                      ))}
                    </ul>
                  </div>
                ))}
              </div>
            )}
          </aside>
        </Card>

        <section className="lg:col-span-3 min-w-0 space-y-6">
          {!selected && <EmptyState title={t("reports.message.select")} className="py-10" />}

          {metadata && metadata.params.length > 0 && (
            <Card padding="sm">
              <h2 className="text-xs font-semibold uppercase tracking-wide text-subtle mb-3">
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
            </Card>
          )}

          {running && <RowsPlaceholder label={t("reports.message.running")} />}

          {result && displayed && !running && (
            <>
              {result.notes?.map((note, index) => (
                <Alert key={index} variant={note.level === "warning" ? "warning" : "info"}>
                  {note.message}
                </Alert>
              ))}

              {chart && (
                <Card padding="sm">
                  <h2 className="text-xs font-semibold uppercase tracking-wide text-subtle mb-3">
                    {t("reports.section.chart")}
                  </h2>
                  <div className="h-72">
                    <ResponsiveContainer width="100%" height="100%">
                      <BarChart data={chart.data}>
                        <CartesianGrid strokeDasharray="3 3" stroke="var(--color-line)" />
                        <XAxis dataKey="__category" tick={{ fontSize: 12 }} />
                        <YAxis tick={{ fontSize: 12 }} />
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
                </Card>
              )}

              {consolidated && (
                <div className="flex flex-wrap items-center gap-3">
                  <Badge variant="outline" tone="accent" icon={<Building2 />}>
                    {t("reports.badge.consolidated")}
                  </Badge>
                  <Checkbox
                    checked={byCompany}
                    onCheckedChange={(checked) => setByCompany(checked === true)}
                    label={<span className="text-xs text-muted">{t("reports.toggle.by_company")}</span>}
                  />
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

// The theme's chart hues, handed out in order. Recharts takes a CSS variable as
// a fill, so the chart follows the colour mode the way the table does.
const SERIES_COLOURS = [
  "var(--color-chart-1)",
  "var(--color-chart-2)",
  "var(--color-chart-3)",
  "var(--color-chart-4)",
  "var(--color-chart-5)",
];

/** Rows of the right shape while something is outstanding, with the label
 *  announced — a skeleton says nothing to a screen reader. */
function RowsPlaceholder({ label }: { label: string }) {
  return (
    <div className="space-y-3 py-4" role="status" aria-live="polite" aria-busy="true">
      <span className="sr-only">{label}</span>
      {Array.from({ length: 4 }, (_, row) => (
        <div key={row} className="flex items-center gap-3">
          <Skeleton variant="circle" className="size-4 shrink-0" />
          <Skeleton className="h-4 flex-1" />
          <Skeleton className="h-4 w-24 shrink-0" />
        </div>
      ))}
    </div>
  );
}

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
  const fieldId = useId();
  const name = label(param.titles, param.key);

  if (param.kind === "date_range") {
    return (
      <fieldset className="sm:col-span-2">
        <legend className="text-sm font-medium text-foreground mb-1.5">{name}</legend>
        <div className="flex items-center gap-2">
          <Input
            type="date"
            label={name}
            hideLabel
            className="flex-1"
            value={values[`${param.key}_from`] || ""}
            onChange={(e) => onChange(`${param.key}_from`, e.target.value)}
          />
          <span className="text-subtle" aria-hidden="true">—</span>
          <Input
            type="date"
            label={name}
            hideLabel
            className="flex-1"
            value={values[`${param.key}_to`] || ""}
            onChange={(e) => onChange(`${param.key}_to`, e.target.value)}
          />
        </div>
      </fieldset>
    );
  }

  if (param.kind === "select" || param.kind === "uuid") {
    return (
      <div className="flex flex-col gap-1.5">
        <label htmlFor={fieldId} className="text-sm font-medium text-foreground">{name}</label>
        <Select
          value={values[param.key] || NO_CHOICE}
          onValueChange={(next) => onChange(param.key, next === NO_CHOICE ? "" : next)}
        >
          <SelectTrigger id={fieldId} />
          <SelectContent>
            <SelectItem value={NO_CHOICE}>—</SelectItem>
            {(param.options || []).filter((option) => option.value !== "").map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {label(option.titles, option.value)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    );
  }

  if (param.kind === "bool") {
    return (
      <Checkbox
        className="mt-6"
        checked={values[param.key] === "true"}
        onCheckedChange={(checked) => onChange(param.key, String(checked === true))}
        label={name}
      />
    );
  }

  return (
    <Input
      label={name}
      value={values[param.key] || ""}
      onChange={(e) => onChange(param.key, e.target.value)}
    />
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
    return <EmptyState title={emptyLabel} className="py-10" />;
  }

  return (
    <Card padding="none" className="overflow-hidden">
      <div className="px-4 py-3 border-b border-line flex items-center justify-between">
        <h2 className="text-sm font-semibold text-foreground">{title}</h2>
        <span className="text-xs text-subtle">
          {result.rows.length} {rowsLabel}
        </span>
      </div>
      <Table containerClassName="rounded-none border-0">
        <TableHeader>
          <TableRow>
            {result.columns.map((column) => (
              <TableHead key={column.key} align={isNumeric(column) ? "right" : "left"}>
                {label(column.titles, column.key)}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {result.rows.map((row, index) => (
            <TableRow key={index}>
              {result.columns.map((column) => (
                <TableCell key={column.key} align={isNumeric(column) ? "right" : "left"}>
                  {formatCell(row[column.key], column, locale)}
                </TableCell>
              ))}
            </TableRow>
          ))}
        </TableBody>
        {result.totals && Object.keys(result.totals).length > 0 && (
          <TableFooter>
            <TableRow>
              {result.columns.map((column, index) => (
                <TableCell key={column.key} align={isNumeric(column) ? "right" : "left"} className="font-semibold">
                  {index === 0
                    ? totalLabel
                    : column.total
                      ? formatCell(result.totals?.[column.key], column, locale)
                      : ""}
                </TableCell>
              ))}
            </TableRow>
          </TableFooter>
        )}
      </Table>
    </Card>
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
    <Card padding="sm">
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-xs font-semibold uppercase tracking-wide text-subtle">
          {t("reports.section.schedules")}
        </h2>
        {canAdd && (
          <Button variant="link" size="sm" leadingIcon={<Plus />} onClick={onAdd}>
            {t("reports.action.new_schedule")}
          </Button>
        )}
      </div>

      {/* Said once, where it matters: a schedule created on a deployment with
          no mail transport looks active and delivers nothing. */}
      {!deliveryConfigured && schedules.length > 0 && (
        <Alert variant="warning" className="mb-3">{t("reports.message.delivery_off")}</Alert>
      )}

      {schedules.length === 0 ? (
        <p className="text-sm text-muted">{t("reports.message.no_schedules")}</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("reports.field.report")}</TableHead>
              <TableHead>{t("reports.field.cron")}</TableHead>
              <TableHead>{t("reports.field.recipients")}</TableHead>
              <TableHead>{t("reports.field.last_run")}</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {schedules.map((schedule) => (
              <TableRow key={schedule.id}>
                <TableCell>
                  {schedule.name || label(schedule.titles, schedule.report_key)}
                </TableCell>
                <TableCell className="font-mono text-xs text-muted">{schedule.cron}</TableCell>
                <TableCell className="text-muted">{schedule.recipients.join(", ")}</TableCell>
                <TableCell className="text-xs">
                  {schedule.last_run_at ? (
                    <span
                      className={
                        schedule.last_status === "FAILED" ? "text-danger" : "text-muted"
                      }
                      title={schedule.last_error || undefined}
                    >
                      {new Date(schedule.last_run_at).toLocaleString()}
                      {schedule.last_status === "FAILED" ? " ⚠" : ""}
                    </span>
                  ) : (
                    <span className="text-subtle">—</span>
                  )}
                </TableCell>
                <TableCell align="right">
                  <IconButton
                    size="sm"
                    className="text-subtle hover:text-danger"
                    aria-label={t("base.action.delete")}
                    icon={<Trash2 />}
                    onClick={() => onRemove(schedule.id)}
                  />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </Card>
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

  // Closed only by its own Cancel, as before: Escape and a click outside are
  // held back so a half-typed schedule is not lost to a stray gesture.
  return (
    <Dialog open>
      <DialogContent
        showClose={false}
        aria-describedby={undefined}
        onEscapeKeyDown={(event) => event.preventDefault()}
        onPointerDownOutside={(event) => event.preventDefault()}
        onInteractOutside={(event) => event.preventDefault()}
      >
        <DialogHeader>
          <DialogTitle>{t("reports.action.schedule")}</DialogTitle>
        </DialogHeader>
        <form onSubmit={save} className="space-y-4">
          {failure && <Alert variant="danger" live>{failure}</Alert>}

          <Input label={t("reports.field.schedule_name")} value={name} onChange={(e) => setName(e.target.value)} />

          <Input
            label={t("reports.field.cron")}
            className="[&_input]:font-mono"
            value={cron}
            onChange={(e) => setCron(e.target.value)}
            required
            helperText={t("reports.hint.cron")}
          />

          <div className="flex flex-col gap-1.5">
            <label htmlFor="report-schedule-format" className="text-sm font-medium text-foreground">
              {t("reports.field.format")}
            </label>
            <Select value={format} onValueChange={(next) => setFormat(next as "xlsx" | "csv")}>
              <SelectTrigger id="report-schedule-format" />
              <SelectContent>
                <SelectItem value="xlsx">Excel (.xlsx)</SelectItem>
                <SelectItem value="csv">CSV</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <Input
            label={t("reports.field.recipients")}
            value={recipients}
            onChange={(e) => setRecipients(e.target.value)}
            placeholder="a@example.mn, b@example.mn"
            required
            helperText={t("reports.hint.recipients")}
          />

          <div className="flex justify-end gap-2 pt-2">
            <Button type="button" variant="ghost" onClick={onClose}>
              {t("base.action.cancel")}
            </Button>
            <Button type="submit" loading={saving}>
              {t("base.action.save")}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
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
