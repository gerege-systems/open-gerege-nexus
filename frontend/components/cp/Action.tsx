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
import {
  Alert,
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  Textarea,
} from "@gerege-systems/ui";

import { cp, StepUpRequired } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";

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
  // The server asking for the code again is a condition, not a failure, and
  // is drawn as one; anything else the request answered with is an error.
  const [failure, setFailure] = useState<{ tone: "warning" | "danger"; text: string } | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    setFailure(null);
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
        setFailure({ tone: "warning", text: t("cp.message.step_up") });
        return;
      }
      setFailure({ tone: "danger", text: error instanceof Error ? error.message : String(error) });
    } finally {
      setBusy(false);
    }
  }

  // The backdrop dismisses only while nothing has been typed: a stray click
  // must not throw away a half-written reason. Escape always closes.
  const typed = reason.trim() !== "" || code !== "";

  return (
    <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent
        showClose={false}
        onInteractOutside={(event) => { if (typed) event.preventDefault(); }}
        {...(request.detail ? {} : { "aria-describedby": undefined })}
      >
        <form onSubmit={submit} className="space-y-4">
          <DialogHeader className="pb-0">
            <DialogTitle>{request.title}</DialogTitle>
            {request.detail && <DialogDescription>{request.detail}</DialogDescription>}
          </DialogHeader>

          {failure && (
            <Alert variant={failure.tone} live>
              {failure.text}
            </Alert>
          )}

          <Textarea
            label={t("cp.field.reason")}
            helperText={t("cp.hint.reason")}
            required
            rows={2}
            value={reason}
            onChange={(event) => setReason(event.target.value)}
          />

          {needsCode && (
            <Input
              label={t("cp.field.code")}
              inputMode="numeric"
              autoComplete="one-time-code"
              maxLength={6}
              required
              value={code}
              onChange={(event) => setCode(event.target.value.replace(/\D/g, ""))}
              className="[&_input]:font-mono [&_input]:tracking-[0.4em]"
            />
          )}

          <DialogFooter className="pt-2">
            <Button variant="outline" onClick={onClose}>
              {t("cp.action.cancel")}
            </Button>
            <Button type="submit" variant={request.danger ? "destructive" : "primary"} loading={busy}>
              {t("cp.action.confirm")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
