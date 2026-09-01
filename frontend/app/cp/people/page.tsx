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

import { Badge, Card, formatMoment, Table } from "@/components/cp/ui";
import { cp, type Roster } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";
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
        <p role="alert" className="text-sm rounded-lg bg-red-50 text-red-700 border border-red-200 px-3 py-2">{failure}</p>
      )}

      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Stat label={t("cp.metric.people")} value={roster?.total} />
        <Stat label={t("cp.metric.verified")} value={roster?.counts.verified} />
        <Stat label={t("cp.metric.signed_in")} value={roster?.counts.signed_in} />
        <Stat label={t("cp.metric.homeless")} value={roster?.counts.homeless} hint={t("cp.hint.homeless")} />
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <input
          value={search}
          onChange={(event) => {
            setOffset(0);
            setSearch(event.target.value);
          }}
          placeholder={t("cp.field.search_people")}
          className="flex-1 min-w-56 rounded-lg border border-input px-3 py-2 text-sm"
        />
        {FILTERS.map((option) => (
          <button
            key={option || "all"}
            type="button"
            onClick={() => {
              setOffset(0);
              setFilter(option);
            }}
            className={`rounded-lg border px-3 py-2 text-sm ${
              filter === option
                ? "border-accent bg-accent-soft text-accent"
                : "border-input text-muted hover:bg-surface-hover"
            }`}
          >
            {t(`cp.filter.${option || "everybody"}` as "cp.filter.everybody")}
          </button>
        ))}
      </div>

      <Card title={t("cp.section.people")}>
        <Table
          head={[
            t("cp.field.person"),
            t("cp.field.identities"),
            t("cp.field.organisations"),
            t("cp.field.sessions"),
            t("cp.field.last_seen"),
          ]}
          rows={people.map((person) => [
            <span key="n" className="min-w-0">
              <Link href={`/cp/people/${person.id}`} className="font-medium text-accent hover:underline">
                {person.name || person.email}
              </Link>
              <span className="block text-xs text-muted font-mono truncate">{person.email}</span>
              {!person.active && <Badge tone="slate">{t("cp.state.disabled")}</Badge>}
              {person.locked_until && <Badge tone="red">{t("cp.state.locked")}</Badge>}
            </span>,
            <span key="i" className="flex flex-wrap gap-1">
              {person.verified && <Badge tone="emerald">eID</Badge>}
              {person.providers > 0 && <Badge tone="slate">SSO × {person.providers}</Badge>}
              {!person.verified && person.providers === 0 && (
                <span className="text-xs text-muted">{t("cp.state.password_only")}</span>
              )}
            </span>,
            <span key="o" className="tabular-nums">
              {person.organisations || <span className="text-amber-700">0</span>}
            </span>,
            <span key="s" className="tabular-nums">{person.sessions}</span>,
            formatMoment(person.last_seen_at) || <span key="l" className="text-xs text-muted">{t("cp.state.never")}</span>,
          ])}
          empty={t("cp.message.no_people")}
        />
      </Card>

      {roster && roster.total > people.length && (
        <div className="flex items-center justify-between text-sm text-muted">
          <span>{t("cp.message.showing", { shown: String(people.length), total: String(roster.total) })}</span>
          <span className="flex gap-2">
            <button
              type="button"
              disabled={offset === 0}
              onClick={() => setOffset(Math.max(0, offset - 100))}
              className="rounded-lg border border-input px-3 py-1.5 disabled:opacity-40"
            >
              {t("cp.action.previous")}
            </button>
            <button
              type="button"
              disabled={people.length < 100}
              onClick={() => setOffset(offset + 100)}
              className="rounded-lg border border-input px-3 py-1.5 disabled:opacity-40"
            >
              {t("cp.action.next")}
            </button>
          </span>
        </div>
      )}
    </div>
  );
}

function Stat({ label, value, hint }: { label: string; value?: number; hint?: string }) {
  return (
    <div className="rounded-xl border border-line bg-surface p-4">
      <p className="text-xs font-semibold uppercase tracking-wider text-muted">{label}</p>
      <p className="mt-1 text-2xl font-semibold text-foreground">{value ?? "—"}</p>
      {hint && <p className="text-xs text-muted">{hint}</p>}
    </div>
  );
}
