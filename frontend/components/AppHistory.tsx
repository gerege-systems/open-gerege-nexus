"use client";

/**
 * An app's history, as one timeline.
 *
 * Two records that have never been read together: what the publisher shipped,
 * and what this organisation did about it. Separately each answers half a
 * question — an administrator looking at an app wants the other half. So the
 * lines are interleaved by time and told apart by their marker rather than by
 * being in two lists.
 *
 * The server has already reduced every release note to one language and merged
 * the two sources, so this component sorts nothing and chooses nothing: it
 * renders what it is handed, in order.
 */

import { useCallback, useEffect, useState } from "react";
import { Bot, CheckCircle2, Clock, Hand, Sparkles, User, X } from "lucide-react";
import { api, type AppHistoryEntry } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { formatDay } from "@/lib/datetime";

/** Which marker a line gets. Releases are the publisher's; the rest are ours. */
function marker(entry: AppHistoryEntry) {
  if (entry.type === "release") return <Sparkles className="w-4 h-4 text-indigo-500" />;
  if (entry.type === "held") return <Hand className="w-4 h-4 text-amber-500" />;
  // A version that moved on its own says so with a different mark, because
  // "who did this" is the first thing anybody asks of a line like it.
  if (entry.system) return <Bot className="w-4 h-4 text-muted" />;
  if (entry.type === "upgraded" || entry.type === "installed") {
    return <CheckCircle2 className="w-4 h-4 text-emerald-500" />;
  }
  return <Clock className="w-4 h-4 text-muted" />;
}

export default function AppHistory({ slug, onClose }: { slug: string; onClose: () => void }) {
  const { t } = useI18n();
  const [entries, setEntries] = useState<AppHistoryEntry[]>([]);
  const [title, setTitle] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const data = await api.getAppHistory(slug);
      setEntries(data.timeline || []);
      setTitle(data.name);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("base.message.error"));
    } finally {
      setLoading(false);
    }
  }, [slug, t]);

  useEffect(() => {
    void load();
  }, [load]);

  // Escape closes it. A drawer that can only be dismissed by finding a small
  // button is a drawer people leave open.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  /**
   * The name of an event kind.
   *
   * The set is closed here rather than interpolated into a translation key,
   * because the keys are typed and a server that grows a sixth event type
   * should render that type's raw name rather than fail to compile — or worse,
   * render a missing-key placeholder to a user.
   */
  const eventLabel = (type: string) => {
    switch (type) {
      case "release":
        return t("app_history.event.release");
      case "installed":
        return t("app_history.event.installed");
      case "upgraded":
        return t("app_history.event.upgraded");
      case "held":
        return t("app_history.event.held");
      case "disabled":
        return t("app_history.event.disabled");
      default:
        return type;
    }
  };

  // Нэг л формат, хэлнээс үл хамааран: 09-localization-mn.md-г үз.
  const day = (iso: string) => formatDay(iso);

  /** Who a line is attributable to, in words rather than an id. */
  const actor = (entry: AppHistoryEntry) => {
    if (entry.type === "release") return null;
    if (entry.system) return t("app_history.actor.system");
    return entry.actor_name || t("app_history.actor.unknown");
  };

  return (
    <div className="fixed inset-0 z-modal flex justify-end" role="dialog" aria-modal="true">
      <button
        aria-label={t("base.action.close")}
        onClick={onClose}
        className="absolute inset-0 bg-slate-900/20"
      />
      <aside className="relative w-full max-w-md bg-surface h-full shadow-lg border-l border-line flex flex-col">
        <header className="px-5 py-4 border-b border-line flex items-start justify-between gap-3">
          <div className="min-w-0">
            <h2 className="font-semibold text-foreground truncate">{title || slug}</h2>
            <p className="text-xs text-muted">{t("app_history.view.subtitle")}</p>
          </div>
          <button
            onClick={onClose}
            aria-label={t("base.action.close")}
            className="p-1.5 rounded-lg text-muted hover:bg-surface-hover shrink-0"
          >
            <X className="w-4 h-4" />
          </button>
        </header>

        <div className="flex-1 overflow-y-auto px-5 py-4">
          {loading ? (
            <p className="text-sm text-muted">{t("base.message.loading")}</p>
          ) : error ? (
            <p className="text-sm text-rose-700 bg-rose-50 border border-rose-200 rounded-lg px-3 py-2">
              {error}
            </p>
          ) : entries.length === 0 ? (
            <p className="text-sm text-muted">{t("app_history.message.empty")}</p>
          ) : (
            <ol className="space-y-4">
              {entries.map((entry, i) => (
                <li key={`${entry.at}-${entry.type}-${i}`} className="flex gap-3">
                  <div className="shrink-0 mt-0.5">{marker(entry)}</div>
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
                      <span className="text-sm font-semibold text-foreground">
                        {eventLabel(entry.type)}
                      </span>
                      {entry.from && entry.version ? (
                        <span className="text-xs font-mono text-muted">
                          v{entry.from} → v{entry.version}
                        </span>
                      ) : entry.version ? (
                        <span className="text-xs font-mono text-muted">v{entry.version}</span>
                      ) : null}
                      <span className="text-xs text-muted">{day(entry.at)}</span>
                    </div>

                    {entry.summary && <p className="text-sm text-foreground mt-0.5">{entry.summary}</p>}
                    {entry.details && <p className="text-xs text-muted mt-0.5">{entry.details}</p>}
                    {/* Why an update is waiting, and what it asked for. */}
                    {entry.reason && (
                      <p className="text-xs text-amber-700 mt-0.5">
                        {entry.reason}
                        {entry.added ? ` · ${entry.added}` : ""}
                      </p>
                    )}

                    {actor(entry) && (
                      <p className="text-xs text-muted mt-0.5 flex items-center gap-1">
                        <User className="w-3 h-3" />
                        {actor(entry)}
                      </p>
                    )}
                    {entry.refs && entry.refs.length > 0 && (
                      <p className="text-xs text-muted mt-0.5 font-mono">{entry.refs.join(" · ")}</p>
                    )}
                  </div>
                </li>
              ))}
            </ol>
          )}
        </div>
      </aside>
    </div>
  );
}
