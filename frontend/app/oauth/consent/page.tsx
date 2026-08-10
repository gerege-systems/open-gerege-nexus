"use client";

/**
 * OAuth2 consent screen.
 *
 * /oauth2/auth on the API bounces the browser here when a client asks for
 * scopes the user has not granted it yet. This page only renders and collects
 * an answer: the query string is handed straight back to the API, which
 * re-validates every parameter against the client registration. Nothing the
 * user could edit in the address bar can widen what is being granted.
 */

import { useEffect, useState } from "react";
import Link from "next/link";
import { AlertTriangle, Check, Loader2, ShieldCheck } from "lucide-react";
import LanguageSwitcher from "@/components/LanguageSwitcher";
import { api, type ConsentPrompt } from "@/lib/api";
import { useI18n } from "@/lib/i18n";

export default function ConsentPage() {
  const { t, locale } = useI18n();
  const [query, setQuery] = useState("");
  const [prompt, setPrompt] = useState<ConsentPrompt | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState<"allow" | "deny" | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const search = window.location.search.replace(/^\?/, "");
    setQuery(search);
    void (async () => {
      try {
        setPrompt(await api.getConsentPrompt(search));
      } catch (err) {
        const message = err instanceof Error ? err.message : t("oauth.consent.invalid");
        // A rejected session means sign in first, then come back to this exact
        // request — the authorization parameters have to survive the detour.
        if (/sign in|unauthor|login/i.test(message)) {
          window.location.assign(`/login?next=${encodeURIComponent(`/oauth/consent?${search}`)}`);
          return;
        }
        setError(message);
      } finally {
        setLoading(false);
      }
    })();
  }, [t]);

  async function decide(approved: boolean) {
    setBusy(approved ? "allow" : "deny");
    setError("");
    try {
      const { redirect_to } = await api.decideConsent(query, approved);
      // A full navigation, not a router push: the destination belongs to the
      // relying party, not to this app.
      window.location.assign(redirect_to);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("oauth.consent.invalid"));
      setBusy(null);
    }
  }

  const describe = (scope: { description: string; description_mn: string }) =>
    locale === "mn" ? scope.description_mn : scope.description;

  return (
    <main className="min-h-screen bg-slate-50 flex flex-col">
      <header className="h-[72px] px-6 max-w-3xl w-full mx-auto flex items-center justify-between">
        <Link href="/" className="flex items-center gap-2.5 font-extrabold tracking-tight text-slate-900">
          <img src="/brand.webp" alt="" className="w-9 h-9 rounded-lg" />
          <span>Gerege Nexus</span>
        </Link>
        <LanguageSwitcher />
      </header>

      <div className="flex-1 flex items-start justify-center px-4 pb-16">
        <div className="w-full max-w-lg bg-white border border-slate-200 rounded-2xl shadow-sm overflow-hidden">
          {loading ? (
            <p className="p-12 text-center text-sm text-slate-500 flex items-center justify-center gap-2">
              <Loader2 className="w-4 h-4 animate-spin" /> {t("oauth.consent.loading")}
            </p>
          ) : !prompt ? (
            <div className="p-10 text-center space-y-3">
              <AlertTriangle className="w-8 h-8 text-amber-500 mx-auto" />
              <h1 className="font-bold text-slate-900">{t("oauth.consent.invalid")}</h1>
              {error && <p className="text-sm text-slate-500">{error}</p>}
            </div>
          ) : (
            <>
              <div className="p-6 border-b border-slate-100 flex items-start gap-4">
                {prompt.logo_uri ? (
                  <img src={prompt.logo_uri} alt="" className="w-12 h-12 rounded-xl object-cover border border-slate-200" />
                ) : (
                  <div className="w-12 h-12 rounded-xl bg-indigo-50 text-indigo-600 grid place-content-center">
                    <ShieldCheck className="w-6 h-6" />
                  </div>
                )}
                <div className="min-w-0">
                  <h1 className="text-lg font-bold text-slate-900">{t("oauth.consent.title")}</h1>
                  <p className="text-sm text-slate-600 mt-0.5">
                    {t("oauth.consent.lede", { app: prompt.client_name })}
                  </p>
                  {prompt.client_uri && (
                    <a
                      href={prompt.client_uri}
                      className="text-xs text-indigo-600 hover:underline font-mono break-all"
                      rel="noreferrer noopener"
                      target="_blank"
                    >
                      {prompt.client_uri}
                    </a>
                  )}
                </div>
              </div>

              <div className="p-6 space-y-3">
                <p className="text-xs font-semibold text-slate-700 uppercase tracking-wide">
                  {t("oauth.consent.will_be_able")}
                </p>
                <ul className="space-y-2">
                  {prompt.scopes.map((scope) => {
                    const known = prompt.already_granted.includes(scope.name);
                    return (
                      <li key={scope.name} className="flex items-start gap-3 text-sm">
                        <Check
                          className={`w-4 h-4 mt-0.5 shrink-0 ${scope.sensitive ? "text-amber-600" : "text-emerald-600"}`}
                        />
                        <span className="flex-1 text-slate-700">
                          {describe(scope)}
                          <span className="ml-2 text-[11px] font-mono text-slate-400">{scope.name}</span>
                          {scope.sensitive && (
                            <span className="ml-2 text-[11px] bg-amber-50 text-amber-700 border border-amber-200 px-1.5 py-0.5 rounded">
                              {t("oauth.consent.sensitive")}
                            </span>
                          )}
                          {known && (
                            <span className="ml-2 text-[11px] text-slate-400">
                              · {t("oauth.consent.already_granted")}
                            </span>
                          )}
                        </span>
                      </li>
                    );
                  })}
                </ul>
              </div>

              <div className="px-6 pb-4">
                <p className="text-[11px] text-slate-400">
                  {t("oauth.consent.redirect_note")}{" "}
                  <span className="font-mono break-all text-slate-500">{prompt.redirect_uri}</span>
                </p>
              </div>

              {error && (
                <p className="mx-6 mb-4 text-sm text-rose-700 bg-rose-50 border border-rose-200 rounded-lg px-3 py-2">
                  {error}
                </p>
              )}

              <div className="p-4 bg-slate-50 border-t border-slate-100 flex gap-2 justify-end">
                <button
                  onClick={() => decide(false)}
                  disabled={busy !== null}
                  className="px-4 py-2 text-sm font-semibold text-slate-600 hover:bg-slate-200 rounded-lg disabled:opacity-50"
                >
                  {t("oauth.consent.deny")}
                </button>
                <button
                  onClick={() => decide(true)}
                  disabled={busy !== null}
                  className="px-5 py-2 text-sm font-semibold bg-indigo-600 hover:bg-indigo-700 text-white rounded-lg flex items-center gap-2 disabled:opacity-50"
                >
                  {busy === "allow" && <Loader2 className="w-4 h-4 animate-spin" />}
                  {t("oauth.consent.allow")}
                </button>
              </div>
            </>
          )}
        </div>
      </div>
    </main>
  );
}
