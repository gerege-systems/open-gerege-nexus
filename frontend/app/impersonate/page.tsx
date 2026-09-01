"use client";

/**
 * Where an operator's handover becomes a session.
 *
 * The console cannot set a cookie on this hostname, so it hands the operator a
 * link here instead. The page does one thing — spend the token — and then
 * sends them to the organisation's own screens, where the amber banner is
 * waiting.
 *
 * It runs immediately rather than asking for a confirmation. The decision was
 * already made, with a reason and a second factor, in the console; a second
 * "are you sure" here would only give the sixty-second handover a chance to
 * expire.
 */

import React, { Suspense, useEffect, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { ShieldCheck } from "lucide-react";

import { apiBase } from "@/lib/apiBase";
import { useI18n } from "@/lib/i18n";

export default function ImpersonatePage() {
  return (
    <Suspense fallback={null}>
      <Impersonate />
    </Suspense>
  );
}

function Impersonate() {
  const { t } = useI18n();
  const router = useRouter();
  const token = useSearchParams().get("token") ?? "";
  const [failure, setFailure] = useState("");
  // React runs effects twice in development, and this token is single-use: the
  // second run would spend a token the first one had already redeemed and show
  // the operator an error for a session that actually started.
  const started = useRef(false);

  useEffect(() => {
    if (started.current) return;
    started.current = true;

    void (async () => {
      if (!token) {
        setFailure(t("auth.message.link_dead"));
        return;
      }
      try {
        const response = await fetch(`${apiBase()}/auth/impersonation/redeem`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          credentials: "include",
          body: JSON.stringify({ token }),
        });
        if (!response.ok) {
          const body = await response.json().catch(() => ({}));
          setFailure(body.error || t("auth.message.link_dead"));
          return;
        }
        // A full navigation rather than a client-side push: the shell reads
        // /me on mount, and it needs to do that with the new cookie.
        window.location.assign("/");
      } catch {
        setFailure(t("auth.message.link_dead"));
      }
    })();
  }, [token, t, router]);

  return (
    <div className="min-h-dvh grid place-items-center bg-surface-2 px-4">
      <div className="w-full max-w-sm bg-surface rounded-xl border border-line p-6 space-y-3 text-center">
        <ShieldCheck className="w-6 h-6 text-amber-500 mx-auto" />
        {failure ? (
          <p role="alert" className="text-sm rounded-lg bg-red-50 text-red-700 border border-red-200 px-3 py-2">
            {failure}
          </p>
        ) : (
          <p className="text-sm text-muted">{t("auth.message.impersonation_starting")}</p>
        )}
      </div>
    </div>
  );
}
