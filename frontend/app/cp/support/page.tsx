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
import {
  Badge,
  Button,
  Card,
  CardHeader,
  EmptyState,
  Input,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@gerege-systems/ui";

import { useAction } from "@/components/cp/Action";
import { cp, type Person } from "@/lib/cp";
import { formatMoment } from "@/lib/datetime";
import { useI18n } from "@/lib/i18n";
import { useUrlState } from "@/lib/urlState";

export default function Support() {
  const { t } = useI18n();
  const action = useAction();
  // Хайлт хаягт үлдэнэ — дэмжлэгийн ажилтан олсон хүнийхээ линкийг
  // хамтрагчдаа явуулж чадна.
  const [urlState, setUrlState] = useUrlState({ q: "" });
  const [query, setQuery] = useState(urlState.q);
  const [people, setPeople] = useState<Person[]>([]);
  const [failure, setFailure] = useState("");
  // Only the first answer is waited for with a placeholder; later searches
  // keep the previous list on screen until the new one arrives.
  const [loading, setLoading] = useState(true);

  const load = useCallback(async (search: string) => {
    try {
      const result = await cp.people(search);
      setPeople(result.people);
      setFailure("");
    } catch (error) {
      setFailure(error instanceof Error ? error.message : String(error));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    const timer = setTimeout(() => {
      setUrlState({ q: query });
      void load(query);
    }, 250);
    return () => clearTimeout(timer);
  }, [query, load, setUrlState]);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold text-foreground">{t("cp.section.support")}</h1>
        <p className="mt-1 max-w-2xl text-sm text-muted">{t("cp.hint.search_people")}</p>
      </div>

      <Input
        label={t("cp.field.email")}
        hideLabel
        prefix={<Search className="size-4" />}
        value={query}
        onChange={(event) => setQuery(event.target.value)}
        placeholder={t("cp.field.email")}
      />

      {loading && !failure && (
        <div role="status" aria-busy="true" className="space-y-3">
          <span className="sr-only">{t("base.message.loading")}</span>
          <Skeleton className="h-24" />
          <Skeleton className="h-24" />
        </div>
      )}

      {failure && (
        <p role="alert" className="text-sm rounded-lg bg-danger-soft text-danger border border-danger-border px-3 py-2">{failure}</p>
      )}

      <div className="space-y-4">
        {people.map((person) => (
          <Card key={person.id} padding="none" className="overflow-hidden">
            <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
              <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{person.email}</h2>
            </CardHeader>
            <div className="p-4 space-y-4">
              <div className="flex flex-wrap items-center gap-2 text-sm text-muted">
                <span>{person.name}</span>
                {person.locked_until && (
                  <Badge tone="warning">
                    {t("cp.state.locked")} · {formatMoment(person.locked_until)}
                  </Badge>
                )}
                <Badge tone="neutral">
                  {person.sessions} · {t("cp.field.state")}
                </Badge>
              </div>

              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("cp.field.organisation")}</TableHead>
                    <TableHead>{t("cp.field.roles")}</TableHead>
                    <TableHead>{t("cp.field.state")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {person.memberships.map((membership) => (
                    <TableRow key={membership.tenant_id}>
                      <TableCell>
                        <Link href={`/cp/tenants/${membership.tenant_id}`} className="hover:underline">
                          {membership.tenant_name}
                        </Link>
                      </TableCell>
                      <TableCell>{membership.roles.length ? membership.roles.join(", ") : "—"}</TableCell>
                      <TableCell>
                        {membership.suspended ? (
                          <Badge tone="warning">{t("cp.state.suspended")}</Badge>
                        ) : (
                          <Badge tone="success">{t("cp.state.active")}</Badge>
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
                  {person.memberships.length === 0 && (
                    <TableRow>
                      <TableCell colSpan={3} className="py-8 text-center text-muted">
                        {t("cp.message.no_activity")}
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>

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
          <EmptyState icon={<Search className="size-6" />} title={t("cp.message.no_people")} />
        )}
      </div>

      {action.dialog}
    </div>
  );
}

function SupportButton({ icon, label, onClick }: { icon: React.ReactNode; label: string; onClick: () => void }) {
  return (
    <Button type="button" variant="outline" leadingIcon={icon} onClick={onClick}>
      {label}
    </Button>
  );
}
