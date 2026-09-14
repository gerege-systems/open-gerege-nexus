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

import { cp, type PersonDetail } from "@/lib/cp";
import { formatMoment } from "@/lib/datetime";
import { useI18n } from "@/lib/i18n";
import {
  Badge,
  Card,
  CardHeader,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@gerege-systems/ui";

export default function Person() {
  const { t } = useI18n();
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
        <Link href="/cp/people" className="inline-flex items-center gap-2 text-sm text-accent">
          <ArrowLeft className="w-4 h-4" />
          {t("cp.section.people")}
        </Link>
        <p role="alert" className="text-sm rounded-lg bg-danger-soft text-danger border border-danger-border px-3 py-2">{failure}</p>
      </div>
    );
  }
  if (!person) {
    return (
      <div role="status" aria-busy="true" className="space-y-3 py-4">
        <span className="sr-only">{t("base.message.loading")}</span>
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-4 w-48" />
        <Skeleton className="h-40 w-full" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <Link href="/cp/people" className="inline-flex items-center gap-2 text-sm text-accent">
          <ArrowLeft className="w-4 h-4" />
          {t("cp.section.people")}
        </Link>
        <h1 className="mt-2 text-2xl font-semibold text-foreground flex items-center gap-2">
          <Users className="w-6 h-6 text-accent" />
          {person.name || person.email}
        </h1>
        <p className="mt-1 text-sm text-muted font-mono">{person.email}</p>
        <div className="mt-2 flex flex-wrap gap-2">
          {person.verified && <Badge tone="success">eID</Badge>}
          {!person.active && <Badge tone="neutral">{t("cp.state.disabled")}</Badge>}
          {person.locked_until && <Badge tone="danger">{t("cp.state.locked")}</Badge>}
          <Badge tone="neutral">{t("cp.field.created")}: {formatMoment(person.created_at)}</Badge>
        </div>
      </div>

      <Card padding="none" className="overflow-hidden">
        <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.field.identities")}</h2>
        </CardHeader>
        <Table containerClassName="rounded-none border-0">
          <TableHeader>
            <TableRow>
              <TableHead>{t("cp.field.kind")}</TableHead>
              <TableHead>{t("cp.field.subject")}</TableHead>
              <TableHead>{t("cp.field.linked")}</TableHead>
              <TableHead>{t("cp.field.last_seen")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {person.identities.map((identity) => (
              <TableRow key={`${identity.kind}:${identity.subject}`}>
                <TableCell>
                  <span className="inline-flex items-center gap-1.5">
                    <KeyRound className="w-3.5 h-3.5 text-muted" />
                    <strong className="text-foreground">{identity.kind === "eid" ? "eID Mongolia" : identity.kind}</strong>
                  </span>
                </TableCell>
                <TableCell className="min-w-0">
                  <span className="block font-mono text-xs truncate">{identity.subject}</span>
                  {identity.detail && <span className="block text-xs text-muted truncate">{identity.detail}</span>}
                </TableCell>
                <TableCell>{formatMoment(identity.linked_at)}</TableCell>
                <TableCell>{formatMoment(identity.last_seen_at) || "—"}</TableCell>
              </TableRow>
            ))}
            {person.identities.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} className="py-8 text-center text-muted">
                  {t("cp.message.password_only")}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Card>

      <Card padding="none" className="overflow-hidden">
        <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.field.organisations")}</h2>
        </CardHeader>
        <Table containerClassName="rounded-none border-0">
          <TableHeader>
            <TableRow>
              <TableHead>{t("cp.field.organisation")}</TableHead>
              <TableHead>{t("cp.field.roles")}</TableHead>
              <TableHead>{t("cp.field.joined")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {person.memberships.map((membership) => (
              <TableRow key={membership.tenant_id}>
                <TableCell className="min-w-0">
                  <Link href={`/cp/tenants/${membership.tenant_id}`} className="font-medium text-accent hover:underline">
                    {membership.tenant_name}
                  </Link>
                  <span className="block text-xs text-muted font-mono">{membership.slug}</span>
                </TableCell>
                <TableCell>{membership.roles.length ? membership.roles.join(", ") : <span className="text-muted">—</span>}</TableCell>
                <TableCell>{formatMoment(membership.joined_at)}</TableCell>
              </TableRow>
            ))}
            {person.memberships.length === 0 && (
              <TableRow>
                <TableCell colSpan={3} className="py-8 text-center text-muted">
                  {t("cp.message.no_organisations")}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Card>

      <Card padding="none" className="overflow-hidden">
        <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.field.sessions")}</h2>
        </CardHeader>
        <Table containerClassName="rounded-none border-0">
          <TableHeader>
            <TableRow>
              <TableHead>{t("cp.field.organisation")}</TableHead>
              <TableHead>{t("cp.field.when")}</TableHead>
              <TableHead>{t("cp.field.last_seen")}</TableHead>
              <TableHead>{t("cp.field.until")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {person.open_sessions.map((session, index) => (
              <TableRow key={index}>
                <TableCell>
                  {session.tenant_id
                    ? person.memberships.find((m) => m.tenant_id === session.tenant_id)?.tenant_name || session.tenant_id
                    : <span className="text-muted">—</span>}
                </TableCell>
                <TableCell>{formatMoment(session.created_at)}</TableCell>
                <TableCell>{formatMoment(session.last_seen_at) || "—"}</TableCell>
                <TableCell>{formatMoment(session.expires_at)}</TableCell>
              </TableRow>
            ))}
            {person.open_sessions.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} className="py-8 text-center text-muted">
                  {t("cp.message.no_sessions")}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Card>

      <Card padding="none" className="overflow-hidden">
        <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.impersonations")}</h2>
        </CardHeader>
        <Table containerClassName="rounded-none border-0">
          <TableHeader>
            <TableRow>
              <TableHead>{t("cp.field.when")}</TableHead>
              <TableHead>{t("cp.field.operator")}</TableHead>
              <TableHead>{t("cp.field.reason")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {person.impersonations.map((visit, index) => (
              <TableRow key={index}>
                <TableCell>{formatMoment(visit.created_at)}</TableCell>
                <TableCell>{visit.operator_email}</TableCell>
                <TableCell>{visit.reason}</TableCell>
              </TableRow>
            ))}
            {person.impersonations.length === 0 && (
              <TableRow>
                <TableCell colSpan={3} className="py-8 text-center text-muted">
                  {t("cp.message.never_impersonated")}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Card>
    </div>
  );
}
