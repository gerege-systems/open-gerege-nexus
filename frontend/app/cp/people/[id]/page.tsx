"use client";

/**
 * One person, and everything this console holds about them.
 *
 * The question behind the screen is "why can this account get in, and where
 * does it get to" — answered by the ways in and the memberships rather than by
 * the fact that the account exists. Below them are the sessions open right
 * now, and every time an operator looked at the platform as this person: they
 * are entitled to that answer, and the operator reading this is the one who
 * has to give it.
 *
 * Read-only. Unlocking somebody, ending their sessions and sending them a way
 * back in stay on the help desk, where each is one action with a reason.
 */

import React, { useCallback, useEffect, useState } from "react";
import { ArrowLeft, KeyRound, Users } from "lucide-react";
import Link from "next/link";
import { useParams } from "next/navigation";

import { Badge, Card, formatMoment, Table } from "@/components/cp/ui";
import { cp, type PersonDetail } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";

export default function Person() {
  const { t, locale } = useI18n();
  const params = useParams<{ id: string }>();
  const id = params?.id ?? "";
  const [person, setPerson] = useState<PersonDetail | null>(null);
  const [failure, setFailure] = useState("");

  const load = useCallback(async () => {
    try {
      setPerson(await cp.person(id));
      setFailure("");
    } catch (error) {
      setFailure(error instanceof Error ? error.message : String(error));
    }
  }, [id]);

  useEffect(() => {
    void load();
  }, [load]);

  if (failure) {
    return (
      <div className="space-y-4">
        <Link href="/cp/people" className="inline-flex items-center gap-2 text-sm text-[var(--gerege-blue)]">
          <ArrowLeft className="w-4 h-4" />
          {t("cp.section.people")}
        </Link>
        <p className="text-sm rounded-lg bg-red-50 text-red-700 border border-red-200 px-3 py-2">{failure}</p>
      </div>
    );
  }
  if (!person) return <p className="text-sm text-slate-500">…</p>;

  return (
    <div className="space-y-6">
      <div>
        <Link href="/cp/people" className="inline-flex items-center gap-2 text-sm text-[var(--gerege-blue)]">
          <ArrowLeft className="w-4 h-4" />
          {t("cp.section.people")}
        </Link>
        <h1 className="mt-2 text-2xl font-semibold text-slate-900 flex items-center gap-2">
          <Users className="w-6 h-6 text-[var(--gerege-blue)]" />
          {person.name || person.email}
        </h1>
        <p className="mt-1 text-sm text-slate-500 font-mono">{person.email}</p>
        <div className="mt-2 flex flex-wrap gap-2">
          {person.verified && <Badge tone="emerald">eID</Badge>}
          {!person.active && <Badge tone="slate">{t("cp.state.disabled")}</Badge>}
          {person.locked_until && <Badge tone="red">{t("cp.state.locked")}</Badge>}
          <Badge tone="slate">{t("cp.field.created")}: {formatMoment(person.created_at, locale)}</Badge>
        </div>
      </div>

      <Card title={t("cp.field.identities")}>
        <Table
          head={[t("cp.field.kind"), t("cp.field.subject"), t("cp.field.linked"), t("cp.field.last_seen")]}
          rows={person.identities.map((identity) => [
            <span key="k" className="inline-flex items-center gap-1.5">
              <KeyRound className="w-3.5 h-3.5 text-slate-400" />
              <strong className="text-slate-900">{identity.kind === "eid" ? "eID Mongolia" : identity.kind}</strong>
            </span>,
            <span key="s" className="min-w-0">
              <span className="block font-mono text-xs truncate">{identity.subject}</span>
              {identity.detail && <span className="block text-xs text-slate-500 truncate">{identity.detail}</span>}
            </span>,
            formatMoment(identity.linked_at, locale),
            formatMoment(identity.last_seen_at, locale) || "—",
          ])}
          empty={t("cp.message.password_only")}
        />
      </Card>

      <Card title={t("cp.field.organisations")}>
        <Table
          head={[t("cp.field.organisation"), t("cp.field.roles"), t("cp.field.joined")]}
          rows={person.memberships.map((membership) => [
            <span key="t" className="min-w-0">
              <Link href={`/cp/tenants/${membership.tenant_id}`} className="font-medium text-[var(--gerege-blue)] hover:underline">
                {membership.tenant_name}
              </Link>
              <span className="block text-xs text-slate-500 font-mono">{membership.slug}</span>
            </span>,
            membership.roles.length ? membership.roles.join(", ") : <span key="r" className="text-slate-400">—</span>,
            formatMoment(membership.joined_at, locale),
          ])}
          empty={t("cp.message.no_organisations")}
        />
      </Card>

      <Card title={t("cp.field.sessions")}>
        <Table
          head={[t("cp.field.organisation"), t("cp.field.when"), t("cp.field.last_seen"), t("cp.field.until")]}
          rows={person.open_sessions.map((session) => [
            session.tenant_id
              ? person.memberships.find((m) => m.tenant_id === session.tenant_id)?.tenant_name || session.tenant_id
              : <span key="n" className="text-slate-400">—</span>,
            formatMoment(session.created_at, locale),
            formatMoment(session.last_seen_at, locale) || "—",
            formatMoment(session.expires_at, locale),
          ])}
          empty={t("cp.message.no_sessions")}
        />
      </Card>

      <Card title={t("cp.section.impersonations")}>
        <Table
          head={[t("cp.field.when"), t("cp.field.operator"), t("cp.field.reason")]}
          rows={person.impersonations.map((visit) => [
            formatMoment(visit.created_at, locale),
            visit.operator_email,
            visit.reason,
          ])}
          empty={t("cp.message.never_impersonated")}
        />
      </Card>
    </div>
  );
}
