"use client";

import React, { useState } from "react";
import { useI18n } from "@/lib/i18n";
import { Play, ImageOff, FileQuestion, X, ExternalLink } from "lucide-react";

const IMAGE_EXT = ["jpg", "jpeg", "png", "gif", "webp", "avif", "bmp", "svg"];
const VIDEO_EXT = ["mp4", "webm", "mov", "m4v", "ogv", "ogg"];

export type MediaKind = "image" | "video" | "unknown";

/**
 * What a URL points at, judged by extension.
 *
 * The CDN serves the real content type but a HEAD per row would be fifty
 * requests to render one page, and the rows that carry no extension are the
 * ones that 404 anyway — bad data rather than an unlabelled file.
 */
export function mediaKind(url: string): MediaKind {
  const m = /\.([a-z0-9]+)(?:[?#].*)?$/i.exec(url || "");
  if (!m) return "unknown";
  const ext = m[1].toLowerCase();
  if (IMAGE_EXT.includes(ext)) return "image";
  if (VIDEO_EXT.includes(ext)) return "video";
  return "unknown";
}

/** The small preview in a table cell. Opens the viewer when clicked. */
export function MediaCell({ url, name, onOpen }: { url: string; name?: string; onOpen: () => void }) {
  const { t } = useI18n();
  const [broken, setBroken] = useState(false);
  const kind = mediaKind(url);

  if (!url) return <span className="text-slate-400">—</span>;

  const frame = "w-16 h-10 rounded border border-slate-200 grid place-items-center overflow-hidden shrink-0";

  if (kind === "image" && !broken) {
    return (
      <button onClick={onOpen} className={`${frame} bg-slate-50 hover:border-indigo-400`} title={name || url}>
        {/* Plain img, not next/image: these are arbitrary CDN paths and some of
            them 404, which the optimizer turns into a server-side error. */}
        <img
          src={url}
          alt={name || ""}
          loading="lazy"
          className="w-full h-full object-cover"
          onError={() => setBroken(true)}
        />
      </button>
    );
  }

  if (kind === "video") {
    // No <video> here on purpose: a metadata preload per row is a range
    // request per row. The file is only fetched once the viewer opens.
    return (
      <button
        onClick={onOpen}
        className={`${frame} bg-slate-900 text-white hover:ring-2 hover:ring-indigo-400`}
        title={name || url}
        aria-label={t("kiosk.action.play")}
      >
        <Play className="w-4 h-4" />
      </button>
    );
  }

  return (
    <button onClick={onOpen} className={`${frame} bg-slate-50 text-slate-400 hover:border-indigo-400`} title={name || url}>
      {broken ? <ImageOff className="w-4 h-4" /> : <FileQuestion className="w-4 h-4" />}
    </button>
  );
}

/** Full-size viewer: images are shown, videos play. */
export function MediaViewer({ url, name, onClose }: { url: string; name?: string; onClose: () => void }) {
  const { t } = useI18n();
  const [failed, setFailed] = useState(false);
  const kind = mediaKind(url);

  return (
    <div
      className="fixed inset-0 bg-slate-900/70 flex items-center justify-center z-[60] p-4"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
    >
      <div
        className="bg-white rounded-xl max-w-4xl w-full shadow-xl border border-slate-200 max-h-[90vh] flex flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between gap-4 p-4 border-b border-slate-200">
          <h2 className="font-semibold text-slate-900 truncate" title={name || url}>{name || url}</h2>
          <div className="flex items-center gap-2 shrink-0">
            <a
              href={url}
              target="_blank"
              rel="noreferrer"
              className="p-1.5 rounded-lg text-slate-500 hover:bg-slate-100"
              aria-label={t("kiosk.action.open_original")}
            >
              <ExternalLink className="w-4 h-4" />
            </a>
            <button onClick={onClose} aria-label={t("kiosk.action.cancel")}>
              <X className="w-5 h-5 text-slate-400 hover:text-slate-700" />
            </button>
          </div>
        </div>

        <div className="p-4 overflow-auto grid place-items-center bg-slate-50 min-h-[240px]">
          {failed || kind === "unknown" ? (
            // Roughly one row in ten carries a malformed URL — a doubled CDN
            // prefix, or a name with no file behind it. Saying so beats a
            // silently empty box.
            <div className="text-center py-10">
              <ImageOff className="w-10 h-10 text-slate-300 mx-auto mb-3" />
              <p className="text-slate-600 text-sm">{t("kiosk.message.media_unavailable")}</p>
              <a href={url} target="_blank" rel="noreferrer" className="text-xs text-indigo-600 hover:underline break-all mt-2 inline-block">
                {url}
              </a>
            </div>
          ) : kind === "image" ? (
            <img src={url} alt={name || ""} className="max-h-[70vh] max-w-full object-contain" onError={() => setFailed(true)} />
          ) : (
            <video
              src={url}
              controls
              autoPlay
              className="max-h-[70vh] max-w-full"
              onError={() => setFailed(true)}
            />
          )}
        </div>
      </div>
    </div>
  );
}
