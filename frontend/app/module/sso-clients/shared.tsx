"use client";

/**
 * The SSO-client screens re-export the shared module kit, and add the one
 * piece only they need.
 *
 * Not a route: only page.tsx and route.ts are routable in the app router.
 */

import { KeyRound } from "lucide-react";
import { CopyButton, Modal, useCopy } from "@/components/module/kit";
import { useI18n } from "@/lib/i18n";

export * from "@/components/module/kit";

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
    <Modal onClose={onClose}>
      <div className="space-y-4">
        <h2 className="text-lg font-semibold text-foreground flex items-center gap-2">
          <KeyRound className="w-5 h-5 text-amber-600" />
          {t("sso_clients.message.secret_once_title")}
        </h2>
        <p className="text-sm text-muted">{t("sso_clients.message.secret_once_body")}</p>
        {[["client_id", clientID], ["client_secret", secret]].map(([label, value], index) => (
          <div
            key={label}
            className={`flex items-center gap-2 p-3 rounded-lg border ${index === 1 ? "bg-amber-50 border-amber-200" : "bg-surface-2 border-line"}`}
          >
            <span className="text-[11px] font-semibold text-muted w-24 shrink-0">{label}</span>
            <code className="text-xs font-mono text-foreground break-all flex-1">{value}</code>
            <CopyButton value={value} id={label} copied={copied} onCopy={copy} />
          </div>
        ))}
        <div className="flex justify-end">
          <button onClick={onClose} className="px-4 py-2 text-sm bg-slate-900 hover:bg-slate-800 text-white rounded-lg font-semibold">
            {t("sso_clients.action.done")}
          </button>
        </div>
      </div>
    </Modal>
  );
}

