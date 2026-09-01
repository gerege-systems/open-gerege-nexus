"use client";

/**
 * Signing keys — what the JWKS publishes.
 *
 * An integrator verifying an id_token reads the header's kid and looks it up in
 * the JWKS. This screen is the other side of that lookup: which kid is signing
 * right now, which are kept around only so older tokens still verify. Public
 * metadata only — the API that feeds this never selects the private half.
 */

import { useEffect, useState } from "react";
import { KeySquare } from "lucide-react";
import { api, type SigningKey } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { Chip, CopyButton, Empty, ErrorNote, Loading, Panel, Screen, useCopy } from "../shared";
import { formatMoment } from "@/lib/datetime";

export default function SigningKeysPage() {
  const { t } = useI18n();
  const { copied, copy } = useCopy();
  const [keys, setKeys] = useState<SigningKey[]>([]);
  const [jwksURI, setJwksURI] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    void (async () => {
      try {
        const data = await api.getSSOSigningKeys();
        setKeys(data.keys || []);
        setJwksURI(data.jwks_uri || "");
      } catch (err) {
        setError(err instanceof Error ? err.message : t("base.message.error"));
      } finally {
        setLoading(false);
      }
    })();
  }, [t]);

  return (
    <Screen
      icon={<KeySquare className="w-5 h-5" />}
      title={t("sso_clients.signing.title")}
      subtitle={t("sso_clients.signing.subtitle")}
    >
      {error && <ErrorNote>{error}</ErrorNote>}

      <Panel className="p-4 bg-surface-2">
        <p className="text-xs text-muted leading-relaxed">{t("sso_clients.signing.explainer")}</p>
        {jwksURI && (
          <div className="flex items-center gap-2 mt-3 bg-surface border border-line rounded-lg px-3 py-2">
            <span className="text-[11px] font-semibold text-muted shrink-0">jwks_uri</span>
            <code className="text-xs font-mono text-foreground break-all flex-1">{jwksURI}</code>
            <CopyButton value={jwksURI} id="jwks" copied={copied} onCopy={copy} />
          </div>
        )}
      </Panel>

      {loading ? (
        <Loading label={t("sso_clients.message.loading")} />
      ) : keys.length === 0 ? (
        <Empty icon={<KeySquare className="w-9 h-9 mx-auto" />}>{t("sso_clients.signing.none")}</Empty>
      ) : (
        <div className="space-y-3">
          {keys.map((key) => (
            <Panel key={key.kid} className={`p-4 ${key.active ? "border-emerald-200" : ""}`}>
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="flex items-center gap-2 min-w-0">
                  <code className="text-sm font-mono font-semibold text-foreground break-all">{key.kid}</code>
                  <CopyButton value={key.kid} id={key.kid} copied={copied} onCopy={copy} />
                </div>
                <div className="flex items-center gap-1.5">
                  <Chip mono>{key.algorithm}</Chip>
                  {key.active
                    ? <Chip tone="emerald">{t("sso_clients.signing.active")}</Chip>
                    : <Chip tone="slate">{t("sso_clients.signing.retired")}</Chip>}
                </div>
              </div>
              <p className="text-[11px] text-muted mt-2">
                {t("sso_clients.field.created")}: {formatMoment(key.created_at)}
                {key.retired_at && ` · ${formatMoment(key.retired_at)}`}
              </p>
            </Panel>
          ))}
          <p className="text-[11px] text-muted">{t("sso_clients.signing.retired_note")}</p>
        </div>
      )}
    </Screen>
  );
}
