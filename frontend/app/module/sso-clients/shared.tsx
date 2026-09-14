"use client";

/**
 * What only the SSO-client screens share: the copy button and the one-time
 * secret dialog.
 *
 * Not a route: only page.tsx and route.ts are routable in the app router.
 */

import { useState } from "react";
import { Check, Copy, KeyRound } from "lucide-react";
import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  IconButton,
} from "@gerege-systems/ui";
import { useI18n } from "@/lib/i18n";

/** useCopy tracks which value was last copied, so one button can show a tick. */
export function useCopy() {
  const [copied, setCopied] = useState("");
  return {
    copied,
    copy(value: string, id: string) {
      void navigator.clipboard.writeText(value);
      setCopied(id);
      setTimeout(() => setCopied(""), 2000);
    },
  };
}

export function CopyButton({ value, id, copied, onCopy }: {
  value: string; id: string; copied: string; onCopy: (value: string, id: string) => void;
}) {
  return (
    <IconButton
      size="sm"
      variant="ghost"
      className="shrink-0 -m-1"
      onClick={() => onCopy(value, id)}
      aria-label="copy"
      icon={copied === id ? <Check className="text-success" /> : <Copy />}
    />
  );
}

/**
 * SecretDialog is the only place a client secret is ever readable. The server
 * stores a digest, so closing this is the last chance to copy it.
 */
export function SecretDialog({ clientID, secret, onClose }: {
  clientID: string; secret: string; onClose: () => void;
}) {
  const { t } = useI18n();
  const { copied, copy } = useCopy();
  return (
    <Dialog open onOpenChange={(open) => { if (!open) onClose(); }}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto">
        <div className="space-y-4">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <KeyRound className="size-5 text-warning" aria-hidden />
              {t("sso_clients.message.secret_once_title")}
            </DialogTitle>
            <DialogDescription>{t("sso_clients.message.secret_once_body")}</DialogDescription>
          </DialogHeader>
          {[["client_id", clientID], ["client_secret", secret]].map(([label, value], index) => (
            <div
              key={label}
              className={`flex items-center gap-2 p-3 rounded-lg border ${index === 1 ? "bg-warning-soft border-warning-border" : "bg-surface-2 border-line"}`}
            >
              <span className="text-xs font-semibold text-muted w-24 shrink-0">{label}</span>
              <code className="text-xs font-mono text-foreground break-all flex-1">{value}</code>
              <CopyButton value={value} id={label} copied={copied} onCopy={copy} />
            </div>
          ))}
          <div className="flex justify-end">
            <Button onClick={onClose}>{t("sso_clients.action.done")}</Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
