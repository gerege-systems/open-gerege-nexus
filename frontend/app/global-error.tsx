"use client";

/**
 * The root layout itself threw.
 *
 * This boundary replaces the whole document, so nothing above it survives: no
 * providers, no i18n, and not even `globals.css` — that import belongs to the
 * layout this file is standing in for. Everything it needs is therefore inline
 * and dependency-free, which is the point of a last-resort screen: it must
 * render when the thing that renders everything else did not.
 *
 * The language is read straight from the cookie the switcher writes, because
 * the provider that would normally answer that question is one of the things
 * that is gone. Two strings is the whole vocabulary; a dictionary here would be
 * one more thing to fail.
 */

import React from "react";

import { DEFAULT_LOCALE, LOCALE_KEY } from "@/lib/locale";

const COPY = {
  mn: {
    lang: "mn",
    title: "Энэ хуудсыг нээж чадсангүй",
    body: "Алдааг бүртгэлээ. Дахин оролдоод үзнэ үү; давтагдвал системийн админд хандана уу.",
    retry: "Дахин оролдох",
  },
  en: {
    lang: "en",
    title: "This page could not be opened",
    body: "The error has been logged. Try again; if it keeps happening, contact your administrator.",
    retry: "Try again",
  },
} as const;

function chooseCopy() {
  if (typeof document === "undefined") return COPY[DEFAULT_LOCALE === "mn" ? "mn" : "en"];
  const match = document.cookie.match(new RegExp(`(?:^|; )${LOCALE_KEY}=([^;]*)`));
  // Only the two languages this file speaks; every other locale falls back to
  // Mongolian, which is the deployment's default.
  return match?.[1] === "en" ? COPY.en : COPY.mn;
}

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  const copy = chooseCopy();
  return (
    <html lang={copy.lang}>
      <body
        style={{
          margin: 0,
          minHeight: "100dvh",
          display: "grid",
          placeItems: "center",
          padding: "24px",
          background: "#f9fafc",
          color: "#11161e",
          font: "400 14px/1.5 system-ui, -apple-system, 'Segoe UI', sans-serif",
        }}
      >
        <main role="alert" style={{ maxWidth: "34rem" }}>
          <h1 style={{ margin: "0 0 8px", fontSize: "22px", fontWeight: 600 }}>{copy.title}</h1>
          <p style={{ margin: "0 0 20px", color: "#5e646c" }}>{copy.body}</p>
          <button
            type="button"
            onClick={reset}
            style={{
              border: 0,
              borderRadius: "8px",
              background: "#0064e1",
              color: "#fff",
              font: "500 14px/1 system-ui, sans-serif",
              padding: "12px 18px",
              cursor: "pointer",
            }}
          >
            {copy.retry}
          </button>
          {error.digest && (
            <p style={{ marginTop: "20px", font: "400 12px/1.5 ui-monospace, monospace", color: "#5e646c" }}>
              {error.digest}
            </p>
          )}
        </main>
      </body>
    </html>
  );
}
