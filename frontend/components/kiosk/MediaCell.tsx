"use client";

import React, { useState } from "react";
import { useI18n } from "@/lib/i18n";
import { Play, ImageOff, FileQuestion, ExternalLink } from "lucide-react";
import { Button, Dialog, DialogContent, DialogHeader, DialogTitle, EmptyState } from "@gerege-systems/ui";

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

  if (!url) return <span className="text-muted">—</span>;

  // A thumbnail, not a button-shaped button: the frame is the size of the
  // preview it holds, which is why the library's IconButton is not used here.
  const frame =
    "w-16 h-10 rounded-md border border-line grid place-items-center overflow-hidden shrink-0 " +
    "outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2";

  if (kind === "image" && !broken) {
    return (
      <button type="button" onClick={onOpen} className={`${frame} bg-surface-2 hover:border-accent`} title={name || url}>
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
        type="button"
        onClick={onOpen}
        className={`${frame} bg-foreground text-surface hover:border-accent`}
        title={name || url}
        aria-label={t("kiosk.action.play")}
      >
        <Play className="w-4 h-4" aria-hidden />
      </button>
    );
  }

  return (
    <button type="button" onClick={onOpen} className={`${frame} bg-surface-2 text-muted hover:border-accent`} title={name || url}>
      {broken ? <ImageOff className="w-4 h-4" aria-hidden /> : <FileQuestion className="w-4 h-4" aria-hidden />}
    </button>
  );
}

/** Full-size viewer: images are shown, videos play. */
export function MediaViewer({ url, name, onClose }: { url: string; name?: string; onClose: () => void }) {
  const { t } = useI18n();
  const [failed, setFailed] = useState(false);
  const kind = mediaKind(url);

  return (
    <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent size="lg" className="max-h-[90dvh] overflow-y-auto" aria-describedby={undefined}>
        <DialogHeader className="flex-row items-center justify-between gap-4 pe-10">
          <DialogTitle className="truncate" title={name || url}>{name || url}</DialogTitle>
          <Button variant="ghost" size="sm" className="shrink-0 px-2" asChild>
            <a href={url} target="_blank" rel="noreferrer" aria-label={t("kiosk.action.open_original")}>
              <ExternalLink aria-hidden />
            </a>
          </Button>
        </DialogHeader>

        <div className="overflow-auto grid place-items-center bg-surface-2 rounded-md min-h-[240px] p-4">
          {failed || kind === "unknown" ? (
            // Roughly one row in ten carries a malformed URL — a doubled CDN
            // prefix, or a name with no file behind it. Saying so beats a
            // silently empty box.
            <EmptyState
              icon={<ImageOff />}
              title={t("kiosk.message.media_unavailable")}
              className="border-0 bg-transparent"
              action={
                <Button variant="link" size="sm" asChild>
                  <a href={url} target="_blank" rel="noreferrer" className="break-all">{url}</a>
                </Button>
              }
            />
          ) : kind === "image" ? (
            <img src={url} alt={name || ""} className="max-h-[70dvh] max-w-full object-contain" onError={() => setFailed(true)} />
          ) : (
            <video
              src={url}
              controls
              autoPlay
              className="max-h-[70dvh] max-w-full"
              onError={() => setFailed(true)}
            />
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
