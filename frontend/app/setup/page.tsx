"use client";

/**
 * The first screen a deployment ever shows.
 *
 * Three steps, in the order the answers are actually available: which
 * organisation this is, who runs it, and the password they will sign in with.
 * The first two are filled from the Gerege Core register rather than typed —
 * an organisation's name retyped by whoever installed the software is how a
 * deployment ends up calling itself something the invoices do not.
 *
 * The token in the address bar is the whole of the authority here. It is held
 * in React state and never stored: a token in localStorage would outlive the
 * one act it authorises, on a shared machine, in a browser nobody clears.
 */

import { useEffect, useState } from "react";
import Link from "next/link";
import { QRCodeSVG } from "qrcode.react";
import { Building2, Loader2, Lock, Search, ShieldCheck, UserRound } from "lucide-react";

import LanguageSwitcher from "@/components/LanguageSwitcher";
import { api, type SetupEnrolment, type SetupStatus } from "@/lib/api";
import { useBrand } from "@/lib/brandContext";
import { useI18n } from "@/lib/i18n";
import { MIN_OPERATOR_PASSWORD, MIN_SETUP_PASSWORD } from "@/lib/setup";

export default function SetupPage() {
  const { t } = useI18n();
  const brand = useBrand();

  const [token, setToken] = useState("");
  // What somebody types when the address bar carries no token. Kept apart from
  // `token` so that a half-typed value is never sent as one.
  const [typedToken, setTypedToken] = useState("");
  // Whether the address bar has been read yet. Without it the screen below
  // renders for one frame before the effect runs, so somebody who arrived on a
  // perfectly good link is asked to paste the token they are already holding.
  const [addressRead, setAddressRead] = useState(false);
  const [status, setStatus] = useState<SetupStatus | undefined>();
  const [step, setStep] = useState<1 | 2 | 3 | 4 | 5>(1);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const [regNo, setRegNo] = useState("");
  const [name, setName] = useState("");
  const [legalName, setLegalName] = useState("");
  const [slug, setSlug] = useState("");

  const [adminRegNo, setAdminRegNo] = useState("");
  const [adminName, setAdminName] = useState("");
  const [adminEmail, setAdminEmail] = useState("");

  const [password, setPassword] = useState("");
  const [again, setAgain] = useState("");

  // The console's first operator. A separate account from the organisation's
  // administrator by design — different identity, different cookie, different
  // audit — even when it is the same person, which on a first deployment it
  // usually is. The fields are seeded from the administrator's for that reason
  // and can be typed over.
  const [operatorName, setOperatorName] = useState("");
  const [operatorEmail, setOperatorEmail] = useState("");
  const [operatorPassword, setOperatorPassword] = useState("");
  const [enrolment, setEnrolment] = useState<SetupEnrolment | undefined>();
  const [code, setCode] = useState("");

  // Whether to offer the console step at all: this deployment must have an
  // address to serve one on, and must not have an operator already.
  const consoleOffered = Boolean(status?.console?.host && status.console.empty);

  useEffect(() => {
    setToken(new URLSearchParams(location.search).get("token") || "");
    setAddressRead(true);
    // A failure here is not fatal: the wizard refuses on the server anyway, and
    // a screen that renders nothing because one GET was slow is worse than one
    // that shows the form and is told no.
    void api.setupStatus().then(setStatus).catch(() => setStatus({ required: true, armed: true, core: false }));
  }, []);

  async function lookupOrganisation() {
    setError("");
    setBusy(true);
    try {
      const found = await api.setupFindOrganisation(token, regNo);
      setName(found.name);
      setLegalName(found.legal_name);
      setSlug(found.suggested_slug);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  }

  async function lookupPerson() {
    setError("");
    setBusy(true);
    try {
      const found = await api.setupFindPerson(token, adminRegNo);
      setAdminName(found.name);
      setAdminEmail(found.email);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  }

  async function finish() {
    setError("");
    if (password !== again) {
      setError(t("setup.message.password_mismatch"));
      return;
    }
    setBusy(true);
    try {
      await api.setupComplete(
        token,
        { name, slug, legal_name: legalName, registration_number: regNo },
        { email: adminEmail, name: adminName },
        password,
      );
      setStep(5);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  }

  // The console's first account, and the code that proves its authenticator.
  //
  // Both run before the organisation is created, and they have to: completing
  // the wizard drops the token these two calls carry. An enrolment that is
  // started and not confirmed leaves an account that cannot sign in — the
  // platform knows that state and the bootstrap command's -confirm finishes it,
  // which is what the message on a failure here points at.
  async function createOperator(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      setEnrolment(await api.setupCreateOperator(token, {
        email: operatorEmail,
        name: operatorName,
        password: operatorPassword,
      }));
    } catch (err: any) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  }

  async function confirmOperator(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      await api.setupConfirmOperator(token, operatorEmail, code);
      await finish();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  }

  if (status && !status.required && step !== 5) {
    return (
      <Shell brand={brand}>
        <h1 className="signin-card__title">{t("setup.view.title")}</h1>
        <p className="signin-card__lede">{t("setup.message.not_required")}</p>
        <Link className="signin-btn signin-btn--primary" href="/login">{t("setup.action.sign_in")}</Link>
      </Shell>
    );
  }

  if (step === 5) {
    return (
      <Shell brand={brand}>
        <h1 className="signin-card__title">{name}</h1>
        <p className="signin-card__lede">{t("setup.message.done")}</p>
        <p className="signin-note">{t("setup.message.apps_next")}</p>
        <Link className="signin-btn signin-btn--primary" href="/login">{t("setup.action.sign_in")}</Link>
      </Shell>
    );
  }

  // Armed is checked after required: a deployment that is already set up should
  // read "already set up", not "the token is missing".
  if (status && !status.armed) {
    return (
      <Shell brand={brand}>
        <h1 className="signin-card__title">{t("setup.view.title")}</h1>
        <p className="signin-alert">{t("setup.message.not_armed")}</p>
      </Shell>
    );
  }

  // No token in the address bar, so ask for it.
  //
  // The wizard used to render its form regardless and let every lookup answer
  // 404 — the gate refuses without the token and says nothing about why, which
  // is right for a stranger and useless for the operator. It became the
  // ordinary way in the moment the landing page started sending people here:
  // a redirect cannot carry the token (that would publish it to every visitor),
  // so the person arrives holding nothing and has to be asked.
  //
  // Typed rather than pasted into the URL because the address bar is a place
  // things are remembered — history, sync, a screen share — and this is the
  // one act the token authorises.
  if (addressRead && !token) {
    return (
      <Shell brand={brand}>
        <h1 className="signin-card__title">{t("setup.view.title")}</h1>
        <p className="signin-card__lede">{t("setup.message.token_missing")}</p>
        <form className="setup-form" onSubmit={(e) => { e.preventDefault(); setToken(typedToken.trim()); }}>
          <label>
            <span>{t("setup.field.token")}</span>
            <input value={typedToken} onChange={(e) => setTypedToken(e.target.value)} autoFocus required />
          </label>
          <button className="signin-btn signin-btn--primary" type="submit" disabled={!typedToken.trim()}>
            {t("setup.action.use_token")}
          </button>
        </form>
      </Shell>
    );
  }

  return (
    <Shell brand={brand}>
      <div>
        <h1 className="signin-card__title">{t("setup.view.title")}</h1>
        <p className="signin-card__lede">{t("setup.view.subtitle")}</p>
      </div>

      <ol className="setup-steps">
        <li className={step === 1 ? "is-current" : step > 1 ? "is-done" : ""}>
          <Building2 size={16} /> {t("setup.view.step_organisation")}
        </li>
        <li className={step === 2 ? "is-current" : step > 2 ? "is-done" : ""}>
          <UserRound size={16} /> {t("setup.view.step_admin")}
        </li>
        <li className={step === 3 ? "is-current" : step > 3 ? "is-done" : ""}>
          <Lock size={16} /> {t("setup.view.step_password")}
        </li>
        {consoleOffered && (
          <li className={step === 4 ? "is-current" : ""}>
            <ShieldCheck size={16} /> {t("setup.view.step_console")}
          </li>
        )}
      </ol>

      {status && !status.core && <p className="signin-note">{t("setup.message.core_off")}</p>}
      {error && <p className="signin-alert">{error}</p>}

      {step === 1 && (
        <form
          className="setup-form"
          onSubmit={(e) => {
            e.preventDefault();
            setStep(2);
          }}
        >
          <label>
            <span>{t("setup.field.registration_number")}</span>
            <div className="setup-lookup">
              <input value={regNo} onChange={(e) => setRegNo(e.target.value)} required />
              <button type="button" onClick={lookupOrganisation} disabled={busy || !status?.core || !regNo}>
                {busy ? <Loader2 size={16} className="animate-spin" /> : <Search size={16} />}
                {t("setup.action.lookup")}
              </button>
            </div>
          </label>
          <label>
            <span>{t("setup.field.organisation_name")}</span>
            <input value={name} onChange={(e) => setName(e.target.value)} required />
          </label>
          <label>
            <span>{t("setup.field.legal_name")}</span>
            <input value={legalName} onChange={(e) => setLegalName(e.target.value)} />
          </label>
          <label>
            <span>{t("setup.field.slug")}</span>
            <input
              value={slug}
              onChange={(e) => setSlug(e.target.value)}
              pattern="[a-z0-9][a-z0-9-]{1,62}[a-z0-9]"
              required
            />
            <small>{t("setup.message.slug_hint")}</small>
          </label>
          <button className="signin-btn signin-btn--primary" type="submit">{t("base.action.next")}</button>
        </form>
      )}

      {step === 2 && (
        <form
          className="setup-form"
          onSubmit={(e) => {
            e.preventDefault();
            setStep(3);
          }}
        >
          <label>
            <span>{t("setup.field.person_registration_number")}</span>
            <div className="setup-lookup">
              <input value={adminRegNo} onChange={(e) => setAdminRegNo(e.target.value)} />
              <button type="button" onClick={lookupPerson} disabled={busy || !status?.core || !adminRegNo}>
                {busy ? <Loader2 size={16} className="animate-spin" /> : <Search size={16} />}
                {t("setup.action.lookup")}
              </button>
            </div>
          </label>
          <label>
            <span>{t("setup.field.admin_name")}</span>
            <input value={adminName} onChange={(e) => setAdminName(e.target.value)} required />
          </label>
          <label>
            <span>{t("base.field.email")}</span>
            <input type="email" value={adminEmail} onChange={(e) => setAdminEmail(e.target.value)} required />
          </label>
          <div className="setup-actions">
            <button type="button" className="signin-btn signin-btn--quiet" onClick={() => setStep(1)}>
              {t("base.action.previous")}
            </button>
            <button className="signin-btn signin-btn--primary" type="submit">{t("base.action.next")}</button>
          </div>
        </form>
      )}

      {step === 3 && (
        <form
          className="setup-form"
          onSubmit={(e) => {
            e.preventDefault();
            if (!consoleOffered) {
              void finish();
              return;
            }
            // Seeded from the administrator, not shared with them: on a first
            // deployment it is the same person, and making them type the same
            // address twice is how the two accounts end up one letter apart.
            setOperatorName(operatorName || adminName);
            setOperatorEmail(operatorEmail || adminEmail);
            setStep(4);
          }}
        >
          <label>
            <span>{t("auth.field.password")}</span>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              minLength={MIN_SETUP_PASSWORD}
              required
            />
            <small>{t("setup.message.password_rule")}</small>
          </label>
          <label>
            <span>{t("setup.field.password_again")}</span>
            <input
              type="password"
              value={again}
              onChange={(e) => setAgain(e.target.value)}
              minLength={MIN_SETUP_PASSWORD}
              required
            />
          </label>
          <div className="setup-actions">
            <button type="button" className="signin-btn signin-btn--quiet" onClick={() => setStep(2)}>
              {t("base.action.previous")}
            </button>
            <button className="signin-btn signin-btn--primary" type="submit" disabled={busy}>
              {busy ? <Loader2 size={16} className="animate-spin" /> : null}
              {consoleOffered ? t("base.action.next") : t("setup.action.finish")}
            </button>
          </div>
        </form>
      )}

      {step === 4 && !enrolment && (
        <form className="setup-form" onSubmit={createOperator}>
          <p className="signin-card__lede">
            {t("setup.message.console_lede", { host: status?.console?.host ?? "" })}
          </p>
          <label>
            <span>{t("setup.field.admin_name")}</span>
            <input value={operatorName} onChange={(e) => setOperatorName(e.target.value)} required />
          </label>
          <label>
            <span>{t("auth.field.email")}</span>
            <input
              type="email"
              value={operatorEmail}
              onChange={(e) => setOperatorEmail(e.target.value)}
              required
            />
          </label>
          <label>
            <span>{t("auth.field.password")}</span>
            <input
              type="password"
              value={operatorPassword}
              onChange={(e) => setOperatorPassword(e.target.value)}
              minLength={MIN_OPERATOR_PASSWORD}
              required
            />
            <small>{t("setup.message.operator_password_rule")}</small>
          </label>
          <div className="setup-actions">
            {/* Skipping is a first-class answer, not a way out of a form that
                went wrong: a deployment can open its console later with
                operator-bootstrap, and one that never opens a console is an
                ordinary deployment rather than an unfinished one. */}
            <button
              type="button"
              className="signin-btn signin-btn--quiet"
              onClick={() => void finish()}
              disabled={busy}
            >
              {t("setup.action.skip_console")}
            </button>
            <button className="signin-btn signin-btn--primary" type="submit" disabled={busy}>
              {busy ? <Loader2 size={16} className="animate-spin" /> : null}
              {t("base.action.next")}
            </button>
          </div>
        </form>
      )}

      {step === 4 && enrolment && (
        <form className="setup-form" onSubmit={confirmOperator}>
          <p className="signin-card__lede">{t("setup.message.enrolment")}</p>
          {/* Shown once and never again: the secret is in the account's row and
              nothing here stores it. A deployment that closes this screen
              without confirming finishes with operator-bootstrap -confirm. */}
          <div className="setup-enrolment">
            <QRCodeSVG value={enrolment.uri} size={168} marginSize={2} />
            <code>{enrolment.secret}</code>
          </div>
          <label>
            <span>{t("setup.field.totp_code")}</span>
            <input
              value={code}
              onChange={(e) => setCode(e.target.value)}
              inputMode="numeric"
              autoComplete="one-time-code"
              required
            />
          </label>
          <div className="setup-actions">
            <button className="signin-btn signin-btn--primary" type="submit" disabled={busy}>
              {busy ? <Loader2 size={16} className="animate-spin" /> : null}
              {t("setup.action.finish")}
            </button>
          </div>
        </form>
      )}
    </Shell>
  );
}

/** The sign-in screen's frame, because this is the same moment in the same
    journey: somebody standing in front of a deployment they cannot get into. */
function Shell({ brand, children }: { brand: { name: string; logoUrl: string }; children: React.ReactNode }) {
  return (
    <main className="signin-shell">
      <header className="signin-shell__nav">
        <span className="gp-brand">
          <img src={brand.logoUrl} alt="" />
          <span>{brand.name}</span>
        </span>
        <LanguageSwitcher />
      </header>
      <section className="signin-shell__body">
        <div className="signin-card">{children}</div>
      </section>
    </main>
  );
}
