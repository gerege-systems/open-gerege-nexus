"use client";

/**
 * Choosing a password from an invitation or a reset.
 *
 * The link that leads here was sent by the operator console through the
 * platform's verification service; this page is where it is spent. It is a
 * public route by necessity — somebody who cannot sign in is by definition not
 * signed in — and everything that makes it safe is on the server: the token is
 * single-use, short-lived, and stored only as a digest.
 *
 * The page checks the link before showing the form, because the alternative is
 * letting somebody type a password twice and then telling them the link
 * expired yesterday.
 */

import React, { Suspense, useCallback, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { KeyRound } from "lucide-react";

import { apiBase } from "@/lib/apiBase";
import { useI18n } from "@/lib/i18n";

export default function SetPasswordPage() {
  // useSearchParams needs a Suspense boundary in the app router; without it
  // the build refuses to prerender the route.
  return (
    <Suspense fallback={null}>
      <SetPassword />
    </Suspense>
  );
}

const MIN_LENGTH = 10;

function SetPassword() {
  const { t } = useI18n();
  const router = useRouter();
  const token = useSearchParams().get("token") ?? "";

  const [state, setState] = useState<"checking" | "ready" | "dead" | "done">("checking");
  const [email, setEmail] = useState("");
  const [purpose, setPurpose] = useState("");
  const [password, setPassword] = useState("");
  const [again, setAgain] = useState("");
  const [failure, setFailure] = useState("");
  // Set by Save, not by typing: flagging the repeat field while somebody is
  // still halfway through typing it says "wrong" about a value they have not
  // finished giving.
  const [mismatch, setMismatch] = useState(false);
  const [busy, setBusy] = useState(false);

  const check = useCallback(async () => {
    if (!token) {
      setState("dead");
      return;
    }
    try {
      const response = await fetch(`${apiBase()}/auth/credential?token=${encodeURIComponent(token)}`);
      const body = await response.json();
      if (!body.valid) {
        setState("dead");
        return;
      }
      setEmail(body.email ?? "");
      setPurpose(body.purpose ?? "");
      setState("ready");
    } catch {
      setState("dead");
    }
  }, [token]);

  useEffect(() => {
    void check();
  }, [check]);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    if (password !== again) {
      setFailure(t("auth.message.password_mismatch"));
      setMismatch(true);
      return;
    }
    setBusy(true);
    setFailure("");
    setMismatch(false);
    try {
      const response = await fetch(`${apiBase()}/auth/credential/redeem`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token, password }),
      });
      if (!response.ok) {
        const body = await response.json().catch(() => ({}));
        setFailure(body.error || t("auth.message.password_failed"));
        return;
      }
      setState("done");
      // Straight to the sign-in screen: the password they have just chosen is
      // the one they are about to use, and every session the account had was
      // ended by the server a moment ago.
      setTimeout(() => router.push("/login"), 1500);
    } catch {
      setFailure(t("auth.message.password_failed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="min-h-dvh grid place-items-center bg-surface-2 px-4">
      <div className="w-full max-w-sm bg-surface rounded-xl border border-line p-6 space-y-4">
        <div className="flex items-center gap-2 text-foreground">
          <KeyRound className="w-5 h-5 text-blue-600" />
          <h1 className="text-lg font-semibold">
            {purpose === "invite" ? t("auth.view.set_password_invite") : t("auth.view.set_password_reset")}
          </h1>
        </div>

        {state === "checking" && <p className="text-sm text-muted">…</p>}

        {state === "dead" && (
          <p role="alert" className="text-sm rounded-lg bg-red-50 text-red-700 border border-red-200 px-3 py-2">
            {t("auth.message.link_dead")}
          </p>
        )}

        {state === "done" && (
          <p className="text-sm rounded-lg bg-emerald-50 text-emerald-700 border border-emerald-200 px-3 py-2">
            {t("auth.message.password_saved")}
          </p>
        )}

        {state === "ready" && (
          <form onSubmit={submit} className="space-y-4">
            <p className="text-sm text-muted">{email}</p>

            {/* role="alert" because the message is written after the page has
                loaded, in response to Save. Without it a screen reader user
                presses the button, hears nothing, and has no way to learn the
                two passwords did not match. */}
            {failure && (
              <p
                id="password-failure"
                role="alert"
                className="text-sm rounded-lg bg-red-50 text-red-700 border border-red-200 px-3 py-2"
              >
                {failure}
              </p>
            )}

            <label className="block text-sm">
              <span className="text-muted">{t("auth.field.new_password")}</span>
              <input
                type="password"
                autoComplete="new-password"
                required
                minLength={MIN_LENGTH}
                value={password}
                onChange={(event) => { setPassword(event.target.value); setMismatch(false); }}
                aria-describedby="password-length-hint"
                className="mt-1 w-full rounded-lg border border-input px-3 py-2"
              />
            </label>

            {/* The mismatch is a fact about this field, so it is this field
                that carries the invalid state and points at the message. */}
            <label className="block text-sm">
              <span className="text-muted">{t("auth.field.repeat_password")}</span>
              <input
                type="password"
                autoComplete="new-password"
                required
                minLength={MIN_LENGTH}
                value={again}
                onChange={(event) => { setAgain(event.target.value); setMismatch(false); }}
                aria-invalid={mismatch || undefined}
                aria-describedby={
                  mismatch ? "password-failure password-length-hint" : "password-length-hint"
                }
                className="mt-1 w-full rounded-lg border border-input px-3 py-2"
              />
            </label>

            <p id="password-length-hint" className="text-xs text-muted">
              {t("auth.hint.password_length")}
            </p>

            <button
              type="submit"
              disabled={busy}
              className="w-full rounded-lg bg-blue-600 text-white py-2.5 font-medium hover:bg-blue-700 disabled:opacity-60 transition"
            >
              {t("auth.action.save_password")}
            </button>
          </form>
        )}
      </div>
    </div>
  );
}
