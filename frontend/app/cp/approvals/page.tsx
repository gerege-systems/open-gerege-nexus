"use client";

/**
 * What is waiting for a second person.
 *
 * One screen, deliberately plain, showing every open request with who asked
 * and why. The two buttons are the whole of it — and the one that agrees is
 * refused by the server if the operator looking at the screen is the one who
 * made the request, whatever this page renders.
 */

import React, { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { Check, X } from "lucide-react";

import { useAction } from "@/components/cp/Action";
import { Card, formatMoment } from "@/components/cp/ui";
import { cp, type Approval } from "@/lib/cp";
import { useI18n } from "@/lib/i18n";

export default function Approvals() {
  const { t, locale } = useI18n();
  const action = useAction();
  const [approvals, setApprovals] = useState<Approval[]>([]);
  const [failure, setFailure] = useState("");

  const load = useCallback(async () => {
    try {
      setApprovals((await cp.approvals()).approvals);
      setFailure("");
    } catch (error) {
      setFailure(error instanceof Error ? error.message : String(error));
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-semibold text-slate-900">{t("cp.section.approvals")}</h1>
        <p className="mt-1 text-sm text-slate-500">{t("cp.message.deletion_requested")}</p>
      </div>

      {failure && (
        <p className="text-sm rounded-lg bg-red-50 text-red-700 border border-red-200 px-3 py-2">{failure}</p>
      )}

      {approvals.length === 0 && (
        <p className="text-center text-slate-500 py-10">{t("cp.message.no_approvals")}</p>
      )}

      {approvals.map((approval) => (
        <Card key={approval.id} title={approval.action}>
          <div className="p-4 space-y-3">
            <dl className="grid gap-3 sm:grid-cols-3 text-sm">
              <div>
                <dt className="text-xs uppercase tracking-wide text-slate-400">
                  {t("cp.field.organisation")}
                </dt>
                <dd className="mt-0.5">
                  <Link href={`/cp/tenants/${approval.target_id}`} className="text-slate-900 hover:underline">
                    {approval.target_name || approval.target_id}
                  </Link>
                </dd>
              </div>
              <div>
                <dt className="text-xs uppercase tracking-wide text-slate-400">
                  {t("cp.field.requested_by")}
                </dt>
                <dd className="mt-0.5 text-slate-700">{approval.requested_by_name}</dd>
              </div>
              <div>
                <dt className="text-xs uppercase tracking-wide text-slate-400">{t("cp.field.when")}</dt>
                <dd className="mt-0.5 text-slate-700">{formatMoment(approval.requested_at, locale)}</dd>
              </div>
            </dl>

            <p className="text-sm rounded-lg bg-slate-50 border border-slate-200 px-3 py-2 text-slate-700">
              {approval.requested_reason}
            </p>

            <div className="flex gap-2">
              <button
                type="button"
                onClick={() =>
                  action.run({
                    title: t("cp.action.approve"),
                    detail: approval.target_name,
                    danger: true,
                    perform: (reason) => cp.approve(approval.id, reason),
                    onDone: load,
                  })
                }
                className="inline-flex items-center gap-2 rounded-lg bg-red-600 px-3 py-2 text-sm font-medium text-white hover:bg-red-700"
              >
                <Check className="w-4 h-4" />
                {t("cp.action.approve")}
              </button>
              <button
                type="button"
                onClick={() =>
                  action.run({
                    title: t("cp.action.reject"),
                    detail: approval.target_name,
                    perform: (reason) => cp.reject(approval.id, reason),
                    onDone: load,
                  })
                }
                className="inline-flex items-center gap-2 rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-700 hover:bg-slate-50"
              >
                <X className="w-4 h-4" />
                {t("cp.action.reject")}
              </button>
            </div>
          </div>
        </Card>
      ))}

      {action.dialog}
    </div>
  );
}
