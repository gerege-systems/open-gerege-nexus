"use client";

/**
 * The help desk.
 *
 * Find somebody by their address, see which organisations they belong to and
 * whether they are locked out, and do the three things that get them back to
 * work: unlock, end every session, send them a link to choose a new password.
 *
 * Everything on this screen is about access. Nothing here shows what anybody
 * keeps on the platform — that is impersonation, and it is a button on the
 * organisation's own page with a reason and a banner attached to it.
 */

import React, { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { KeyRound, LockOpen, LogOut, Search } from "lucide-react";

import { useAction } from "@/components/cp/Action";
import { Badge, Card, formatMoment, Table } from "@/components/cp/ui";
import { cp, type Person } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";

export default function Support() {
  const { t, locale } = useI18n();
  const action = useAction();
  const [query, setQuery] = useState("");
  const [people, setPeople] = useState<Person[]>([]);
  const [failure, setFailure] = useState("");

  const load = useCallback(async (search: string) => {
    try {
      const result = await cp.people(search);
      setPeople(result.people);
      setFailure("");
    } catch (error) {
      setFailure(error instanceof Error ? error.message : String(error));
    }
  }, []);

  useEffect(() => {
    const timer = setTimeout(() => void load(query), 250);
    return () => clearTimeout(timer);
  }, [query, load]);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold text-slate-900">{t("cp.section.support")}</h1>
        <p className="mt-1 text-sm text-slate-500">{t("cp.hint.search_people")}</p>
      </div>

      <div className="relative">
        <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
        <input
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder={t("cp.field.email")}
          className="w-full rounded-xl border border-slate-300 bg-white pl-9 pr-3 py-2.5 focus:outline-none focus:ring-2 focus:ring-slate-900/10"
        />
      </div>

      {failure && (
        <p className="text-sm rounded-lg bg-red-50 text-red-700 border border-red-200 px-3 py-2">{failure}</p>
      )}

      <div className="space-y-4">
        {people.map((person) => (
          <Card key={person.id} title={person.email}>
            <div className="p-4 space-y-4">
              <div className="flex flex-wrap items-center gap-2 text-sm text-slate-600">
                <span>{person.name}</span>
                {person.locked_until && (
                  <Badge tone="amber">
                    {t("cp.state.locked")} · {formatMoment(person.locked_until, locale)}
                  </Badge>
                )}
                <Badge tone="slate">
                  {person.sessions} · {t("cp.field.state")}
                </Badge>
              </div>

              <Table
                head={[t("cp.field.organisation"), t("cp.field.roles"), t("cp.field.state")]}
                rows={person.memberships.map((membership) => [
                  <Link key={membership.tenant_id} href={`/cp/tenants/${membership.tenant_id}`} className="hover:underline">
                    {membership.tenant_name}
                  </Link>,
                  membership.roles.length ? membership.roles.join(", ") : "—",
                  membership.suspended ? (
                    <Badge tone="amber">{t("cp.state.suspended")}</Badge>
                  ) : (
                    <Badge tone="emerald">{t("cp.state.active")}</Badge>
                  ),
                ])}
                empty={t("cp.message.no_activity")}
              />

              <div className="flex flex-wrap gap-2">
                <SupportButton
                  icon={<LockOpen className="w-4 h-4" />}
                  label={t("cp.action.unlock")}
                  onClick={() =>
                    action.run({
                      title: t("cp.action.unlock"),
                      detail: person.email,
                      perform: (reason) => cp.unlock(person.id, reason),
                      onDone: () => void load(query),
                    })
                  }
                />
                <SupportButton
                  icon={<LogOut className="w-4 h-4" />}
                  label={t("cp.action.revoke_sessions")}
                  onClick={() =>
                    action.run({
                      title: t("cp.action.revoke_sessions"),
                      detail: person.email,
                      danger: true,
                      perform: (reason) => cp.revokeSessions(person.id, reason),
                      onDone: () => void load(query),
                    })
                  }
                />
                {person.memberships.length > 0 && (
                  <SupportButton
                    icon={<KeyRound className="w-4 h-4" />}
                    label={t("cp.action.send_reset")}
                    onClick={() =>
                      action.run({
                        title: t("cp.action.send_reset"),
                        detail: person.email,
                        perform: (reason) =>
                          cp.credentialLink(person.id, {
                            // The organisation the mail is sent on behalf of —
                            // the verification service counts its quota per
                            // organisation, so it cannot be "none".
                            tenant_id: person.memberships[0].tenant_id,
                            purpose: "reset",
                            reason,
                          }),
                      })
                    }
                  />
                )}
              </div>
            </div>
          </Card>
        ))}

        {people.length === 0 && query.length >= 3 && (
          <p className="text-center text-slate-500 py-8">{t("cp.message.no_people")}</p>
        )}
      </div>

      {action.dialog}
    </div>
  );
}

function SupportButton({ icon, label, onClick }: { icon: React.ReactNode; label: string; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="inline-flex items-center gap-2 rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-700 hover:bg-slate-50"
    >
      {icon}
      {label}
    </button>
  );
}
