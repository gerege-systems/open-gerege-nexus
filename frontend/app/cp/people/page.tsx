"use client";

/**
 * Everybody with an account on this deployment.
 *
 * The help desk searches: type three characters, act on one person. That is
 * the right shape for "somebody rang up" and the wrong one for every question
 * about the population — how many accounts there are, how many can actually
 * sign in, who is in no organisation at all. Those are asked of the whole list
 * or not at all, so this screen counts first and lists second.
 */

import React, { useCallback, useEffect, useState } from "react";
import { Users } from "lucide-react";
import Link from "next/link";

import { cp, type Roster } from "@/lib/cp";
import { formatMoment } from "@/lib/datetime";
import { useI18n } from "@/lib/i18n";
import {
  Badge,
  Button,
  Card,
  CardHeader,
  Input,
  Spinner,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@gerege-systems/ui";
import { useUrlState } from "@/lib/urlState";

const FILTERS = ["", "verified", "locked", "homeless"] as const;

export default function People() {
  const { t } = useI18n();
  const [roster, setRoster] = useState<Roster | null>(null);
  // Хайлт, шүүлтүүр, байрлал гурвуулаа хаягт: refresh хийхэд алдагдахгүй,
  // back нь шүүлтүүрийг буцаана, линк нь ижил жагсаалт нээнэ.
  const [urlState, setUrlState] = useUrlState({ q: "", filter: "", offset: "0" });
  const [search, setSearch] = useState(urlState.q);
  const filter = urlState.filter;
  const setFilter = (which: string) => setUrlState({ filter: which, offset: "0" });
  const offset = Number(urlState.offset) || 0;
  const setOffset = (from: number) => setUrlState({ offset: String(from) });
  const [failure, setFailure] = useState("");

  const load = useCallback(async (query: string, which: string, from: number) => {
    try {
      setRoster(await cp.roster(query, which, from));
      setFailure("");
    } catch (error) {
      setFailure(error instanceof Error ? error.message : String(error));
    }
  }, []);

  useEffect(() => {
    // Бичихэд хайлт нь 250ms хүлээнэ — үсэг бүрт нэг хүсэлт биш.
    const timer = setTimeout(() => {
      setUrlState({ q: search });
      void load(search, filter, offset);
    }, 250);
    return () => clearTimeout(timer);
  }, [load, search, filter, offset, setUrlState]);

  const people = roster?.people ?? [];

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold text-foreground flex items-center gap-2">
          <Users className="w-6 h-6 text-accent" />
          {t("cp.section.people")}
        </h1>
        <p className="mt-1 text-sm text-muted">{t("cp.hint.people")}</p>
      </div>

      {failure && (
        <p role="alert" className="text-sm rounded-lg bg-danger-soft text-danger border border-danger-border px-3 py-2">{failure}</p>
      )}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Stat label={t("cp.metric.people")} value={roster?.total} />
        <Stat label={t("cp.metric.verified")} value={roster?.counts.verified} />
        <Stat label={t("cp.metric.signed_in")} value={roster?.counts.signed_in} />
        <Stat label={t("cp.metric.homeless")} value={roster?.counts.homeless} hint={t("cp.hint.homeless")} />
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <div className="flex-1 min-w-56">
          <Input
            type="search"
            label={t("cp.field.search_people")}
            hideLabel
            value={search}
            onChange={(event) => {
              setOffset(0);
              setSearch(event.target.value);
            }}
            placeholder={t("cp.field.search_people")}
          />
        </div>
        {FILTERS.map((option) => (
          <Button
            key={option || "all"}
            variant={filter === option ? "secondary" : "outline"}
            aria-pressed={filter === option}
            onClick={() => {
              setOffset(0);
              setFilter(option);
            }}
          >
            {t(`cp.filter.${option || "everybody"}` as "cp.filter.everybody")}
          </Button>
        ))}
      </div>

      <Card padding="none" className="overflow-hidden">
        <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
          <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{t("cp.section.people")}</h2>
        </CardHeader>
        <Table containerClassName="rounded-none border-0">
          <TableHeader>
            <TableRow>
              <TableHead>{t("cp.field.person")}</TableHead>
              <TableHead>{t("cp.field.identities")}</TableHead>
              <TableHead>{t("cp.field.organisations")}</TableHead>
              <TableHead>{t("cp.field.sessions")}</TableHead>
              <TableHead>{t("cp.field.last_seen")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {people.map((person) => (
              <TableRow key={person.id}>
                <TableCell className="min-w-0">
                  <Link href={`/cp/people/${person.id}`} className="font-medium text-accent hover:underline">
                    {person.name || person.email}
                  </Link>
                  <span className="block text-xs text-muted font-mono truncate">{person.email}</span>
                  {!person.active && <Badge tone="neutral">{t("cp.state.disabled")}</Badge>}
                  {person.locked_until && <Badge tone="danger">{t("cp.state.locked")}</Badge>}
                </TableCell>
                <TableCell>
                  <span className="flex flex-wrap gap-1">
                    {person.verified && <Badge tone="success">eID</Badge>}
                    {person.providers > 0 && <Badge tone="neutral">SSO × {person.providers}</Badge>}
                    {!person.verified && person.providers === 0 && (
                      <span className="text-xs text-muted">{t("cp.state.password_only")}</span>
                    )}
                  </span>
                </TableCell>
                <TableCell className="tabular-nums">
                  {person.organisations || <span className="text-warning">0</span>}
                </TableCell>
                <TableCell className="tabular-nums">{person.sessions}</TableCell>
                <TableCell>
                  {formatMoment(person.last_seen_at) || <span className="text-xs text-muted">{t("cp.state.never")}</span>}
                </TableCell>
              </TableRow>
            ))}
            {people.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="py-8 text-center text-muted">
                  {roster || failure ? (
                    t("cp.message.no_people")
                  ) : (
                    <span role="status" className="inline-flex items-center gap-2">
                      <Spinner size="sm" decorative />
                      {t("base.message.loading")}
                    </span>
                  )}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </Card>

      {roster && roster.total > people.length && (
        <div className="flex items-center justify-between text-sm text-muted">
          <span>{t("cp.message.showing", { shown: String(people.length), total: String(roster.total) })}</span>
          <span className="flex gap-2">
            <Button variant="outline" size="sm" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - 100))}>
              {t("cp.action.previous")}
            </Button>
            <Button variant="outline" size="sm" disabled={people.length < 100} onClick={() => setOffset(offset + 100)}>
              {t("cp.action.next")}
            </Button>
          </span>
        </div>
      )}
    </div>
  );
}

function Stat({ label, value, hint }: { label: string; value?: number; hint?: string }) {
  return (
    <div className="rounded-lg border border-line bg-surface p-4">
      <p className="text-xs font-semibold uppercase tracking-wider text-muted">{label}</p>
      <p className="mt-1 text-2xl font-semibold text-foreground">{value ?? "—"}</p>
      {hint && <p className="text-xs text-muted">{hint}</p>}
    </div>
  );
}
