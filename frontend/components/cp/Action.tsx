"use client";

/**
 * How the console asks for the two things every dangerous action needs: a
 * reason, and — when the step-up window has closed — the authenticator code
 * again.
 *
 * One hook rather than a dialog per screen. Every write in this console has the
 * same shape (say why, do it, maybe prove yourself again), and a screen that
 * implemented it for itself would be the screen that forgot the retry, or
 * asked for a reason it then did not send.
 *
 *     const action = useAction();
 *     action.run({
 *       title: t("cp.action.suspend"),
 *       perform: (reason) => cp.suspend(tenant.id, reason),
 *       onDone: reload,
 *     });
 */

import React, { useCallback, useState } from "react";

import { cp, StepUpRequired } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";
import { Modal } from "@/components/ui";

interface Request {
  title: string;
  /** What the operator is about to do to what, in one line. */
  detail?: string;
  perform: (reason: string) => Promise<unknown>;
  onDone?: () => void;
  /** Destructive actions get the red button. */
  danger?: boolean;
}

export function useAction() {
  const [request, setRequest] = useState<Request | null>(null);
  const run = useCallback((next: Request) => setRequest(next), []);
  const dialog = request ? (
    <ActionDialog request={request} onClose={() => setRequest(null)} />
  ) : null;
  return { run, dialog };
}

function ActionDialog({ request, onClose }: { request: Request; onClose: () => void }) {
  const { t } = useI18n();
  const [reason, setReason] = useState("");
  const [code, setCode] = useState("");
  // The code field appears only once the server has asked for it, so an
  // operator inside the five-minute window is never shown a box they do not
  // need to fill in.
  const [needsCode, setNeedsCode] = useState(false);
  const [failure, setFailure] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    setFailure("");
    try {
      if (needsCode) {
        // Prove ourselves first, then do what was asked. The other order —
        // trying the action and stepping up on the refusal — would perform the
        // step-up as a side effect of a failure, and a failed action would
        // leave the window open behind it.
        await cp.stepUp(code);
      }
      await request.perform(reason);
      request.onDone?.();
      onClose();
    } catch (error) {
      if (error instanceof StepUpRequired) {
        setNeedsCode(true);
        setFailure(t("cp.message.step_up"));
        return;
      }
      setFailure(error instanceof Error ? error.message : String(error));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal label={request.title}>
      <form onSubmit={submit} className="p-5 space-y-4">
        <div>
          <h2 className="text-lg font-semibold text-slate-900">{request.title}</h2>
          {request.detail && <p className="mt-1 text-sm text-slate-500">{request.detail}</p>}
        </div>

        {failure && (
          <p className="text-sm rounded-lg bg-amber-50 text-amber-900 border border-amber-200 px-3 py-2">
            {failure}
          </p>
        )}

        <label className="block text-sm">
          <span className="text-slate-600">{t("cp.field.reason")}</span>
          <textarea
            required
            rows={2}
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 focus:outline-none focus:ring-2 focus:ring-slate-900/10"
          />
          <span className="mt-1 block text-xs text-slate-400">{t("cp.hint.reason")}</span>
        </label>

        {needsCode && (
          <label className="block text-sm">
            <span className="text-slate-600">{t("cp.field.code")}</span>
            <input
              inputMode="numeric"
              autoComplete="one-time-code"
              maxLength={6}
              required
              value={code}
              onChange={(event) => setCode(event.target.value.replace(/\D/g, ""))}
              className="mt-1 w-full rounded-lg border border-slate-300 px-3 py-2 font-mono tracking-[0.4em] focus:outline-none focus:ring-2 focus:ring-slate-900/10"
            />
          </label>
        )}

        <div className="flex justify-end gap-2 pt-1">
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg px-4 py-2 text-sm text-slate-600 hover:bg-slate-100"
          >
            {t("cp.action.cancel")}
          </button>
          <button
            type="submit"
            disabled={busy}
            className={`rounded-lg px-4 py-2 text-sm font-medium text-[var(--gerege-on-blue)] disabled:opacity-60 ${
              request.danger ? "bg-red-600 hover:bg-red-700" : "bg-[var(--gerege-blue)] hover:brightness-105"
            }`}
          >
            {t("cp.action.confirm")}
          </button>
        </div>
      </form>
    </Modal>
  );
}
