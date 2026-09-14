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
import { Bot, CheckCircle2, Clock, Hand, Sparkles, User } from "lucide-react";
import { Alert, Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle, Skeleton } from "@gerege-systems/ui";
import { api, type AppHistoryEntry } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { formatDay } from "@/lib/datetime";

/** Which marker a line gets. Releases are the publisher's; the rest are ours. */
function marker(entry: AppHistoryEntry) {
  if (entry.type === "release") return <Sparkles className="w-4 h-4 text-accent" />;
  if (entry.type === "held") return <Hand className="w-4 h-4 text-warning" />;
  // A version that moved on its own says so with a different mark, because
  // "who did this" is the first thing anybody asks of a line like it.
  if (entry.system) return <Bot className="w-4 h-4 text-muted" />;
  if (entry.type === "upgraded" || entry.type === "installed") {
    return <CheckCircle2 className="w-4 h-4 text-success" />;
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
    // The design system's sheet owns Escape, the backdrop, the focus trap and
    // the close button; the title names the dialog for a screen reader.
    <Sheet open onOpenChange={(open) => { if (!open) onClose(); }}>
      <SheetContent side="right" className="flex w-full max-w-md flex-col gap-0 p-0">
        <SheetHeader className="border-b border-line px-5 py-4 pe-12">
          <SheetTitle className="truncate">{title || slug}</SheetTitle>
          <SheetDescription>{t("app_history.view.subtitle")}</SheetDescription>
        </SheetHeader>

        <div className="flex-1 overflow-y-auto px-5 py-4">
          {loading ? (
            <div role="status" aria-busy="true" className="space-y-3">
              <span className="sr-only">{t("base.message.loading")}</span>
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-4 w-1/2" />
              <Skeleton className="h-4 w-2/3" />
            </div>
          ) : error ? (
            <Alert variant="danger" live>{error}</Alert>
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
                      <p className="text-xs text-warning mt-0.5">
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
      </SheetContent>
    </Sheet>
  );
}
