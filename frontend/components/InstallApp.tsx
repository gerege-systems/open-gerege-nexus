"use client";

import React, { useCallback, useEffect, useState } from "react";
import { currentDeviceLine } from "@/lib/deviceLine";
import { Download, X, Share } from "lucide-react";
import { useI18n } from "@/lib/i18n";

/**
 * Installing the platform from the browser, and the worker that makes it
 * possible.
 *
 * Both live here because they are one feature: a browser will not offer to
 * install anything without a registered service worker and a manifest, and a
 * worker registered for a page nobody can install is just a cache.
 *
 * The offer is only made when the browser says it can be made. Chrome and Edge
 * — on desktop and on Android — fire beforeinstallprompt when the criteria are
 * met, and that event is the only reliable signal that installing will work.
 * Safari fires nothing and installs through the Share menu instead, so it gets
 * a sentence telling it where to look rather than a button that would do
 * nothing.
 */

/** The event Chromium fires; not in the DOM lib because it is not standardised. */
interface InstallPromptEvent extends Event {
  prompt: () => Promise<void>;
  userChoice: Promise<{ outcome: "accepted" | "dismissed" }>;
}

const DISMISSED_KEY = "gerege_install_dismissed";

/** Whether this is already running as an installed app rather than in a tab. */
function runningInstalled(): boolean {
  if (typeof window === "undefined") return false;
  return (
    window.matchMedia("(display-mode: standalone)").matches ||
    window.matchMedia("(display-mode: window-controls-overlay)").matches ||
    // Safari's own flag, which predates display-mode and is still what it sets.
    (window.navigator as unknown as { standalone?: boolean }).standalone === true
  );
}

/**
 * Whether this is iOS or iPadOS, where installing goes through the Share menu.
 *
 * No browser check beyond the platform: every browser on iOS is WebKit
 * underneath, so Chrome and Firefox there install the same way Safari does.
 */
function isAppleMobile(): boolean {
  if (typeof window === "undefined") return false;
  const ua = window.navigator.userAgent;
  // iPadOS reports itself as a Mac, so a Mac that reports touch points is one.
  return /iPad|iPhone|iPod/.test(ua) || (/Macintosh/.test(ua) && navigator.maxTouchPoints > 1);
}

export default function InstallApp() {
  const { t } = useI18n();
  const [prompt, setPrompt] = useState<InstallPromptEvent | null>(null);
  const [showAppleHint, setShowAppleHint] = useState(false);
  const [hidden, setHidden] = useState(true);

  // The worker is registered whether or not the app can be installed: it is
  // also what answers a navigation when the network is gone.
  useEffect(() => {
    if (!("serviceWorker" in navigator)) return;
    // After load, so registering never competes with the first render for
    // bandwidth on a slow connection.
    const register = () => {
      navigator.serviceWorker.register("/sw.js", { scope: "/" }).catch(() => {
        // A refused registration costs the offline page and the install offer,
        // and nothing else. There is nothing for a person to do about it.
      });
    };
    if (document.readyState === "complete") register();
    else window.addEventListener("load", register, { once: true });
    return () => window.removeEventListener("load", register);
  }, []);

  useEffect(() => {
    if (runningInstalled()) return;
    // Төхөөрөмжийн domain шугам дээр энэ санал утгагүй: хүн аль хэдийн native
    // апп дотор сууж байгаа бөгөөд түүнийг PWA болгож "суулгах" нь тэр аппыг
    // өөрийг нь хоёр дахин үүсгэнэ гэсэн үг. Хөтчийн шугам дээр урьдын адил.
    if (currentDeviceLine()) return;
    if (window.localStorage.getItem(DISMISSED_KEY) === "1") return;

    const onPrompt = (event: Event) => {
      // Holding the event is what lets the offer be made in our own words, at a
      // moment of our choosing, instead of in whatever bar the browser puts up.
      event.preventDefault();
      setPrompt(event as InstallPromptEvent);
      setHidden(false);
    };
    window.addEventListener("beforeinstallprompt", onPrompt);

    // Safari never fires it, so the hint is offered on its own after a moment —
    // long enough that it does not arrive on top of a page still loading.
    let timer: ReturnType<typeof setTimeout> | undefined;
    if (isAppleMobile()) {
      timer = setTimeout(() => {
        setShowAppleHint(true);
        setHidden(false);
      }, 4000);
    }

    // Chromium fires this when the install completes, by our button or its own.
    const onInstalled = () => setHidden(true);
    window.addEventListener("appinstalled", onInstalled);

    return () => {
      window.removeEventListener("beforeinstallprompt", onPrompt);
      window.removeEventListener("appinstalled", onInstalled);
      if (timer) clearTimeout(timer);
    };
  }, []);

  const install = useCallback(async () => {
    if (!prompt) return;
    await prompt.prompt();
    const { outcome } = await prompt.userChoice;
    // The event is single-use whatever the answer, and Chromium will fire a
    // fresh one if the browser decides to offer again.
    setPrompt(null);
    setHidden(true);
    if (outcome === "dismissed") window.localStorage.setItem(DISMISSED_KEY, "1");
  }, [prompt]);

  const dismiss = useCallback(() => {
    setHidden(true);
    window.localStorage.setItem(DISMISSED_KEY, "1");
  }, []);

  if (hidden || (!prompt && !showAppleHint)) return null;

  return (
    // Above the native switcher rather than beside it: two cards competing for
    // the same corner is how a screen starts to look unmaintained.
    <div className="fixed bottom-24 right-5 z-[9998] w-[min(20rem,calc(100vw-2.5rem))]">
      <div
        className="flex flex-col gap-3 rounded-2xl border p-4 shadow-2xl"
        style={{
          background: "var(--gerege-surface)",
          borderColor: "var(--gerege-border)",
          color: "var(--gerege-fg)",
        }}
      >
        <div className="flex items-start gap-3">
          <img src="/icons/app-192.png" alt="" className="h-10 w-10 rounded-lg" />
          <div className="min-w-0 flex-1">
            <p className="text-sm font-semibold">{t("pwa.install.title")}</p>
            <p className="mt-0.5 text-xs opacity-70">{t("pwa.install.body")}</p>
          </div>
          <button
            type="button"
            onClick={dismiss}
            aria-label={t("base.action.close")}
            className="rounded-md p-1 opacity-60 transition hover:opacity-100"
          >
            <X size={15} />
          </button>
        </div>

        {prompt ? (
          <button
            type="button"
            onClick={install}
            className="inline-flex items-center justify-center gap-2 rounded-lg bg-[var(--gerege-blue)] px-3 py-2 text-sm font-semibold text-[var(--gerege-on-blue)] transition hover:opacity-90"
          >
            <Download size={15} />
            {t("pwa.install.action")}
          </button>
        ) : (
          <p className="flex items-center gap-1.5 text-xs opacity-70">
            <Share size={14} className="shrink-0" />
            {t("pwa.install.ios")}
          </p>
        )}
      </div>
    </div>
  );
}
