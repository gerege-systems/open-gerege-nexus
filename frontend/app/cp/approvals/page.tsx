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
import { Button, Card, CardHeader, EmptyState } from "@gerege-systems/ui";

import { useAction } from "@/components/cp/Action";
import { cp, type Approval } from "@/lib/cp";
import { formatMoment } from "@/lib/datetime";
import { useI18n } from "@/lib/i18n";

export default function Approvals() {
  const { t } = useI18n();
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
        <h1 className="text-2xl font-semibold text-foreground">{t("cp.section.approvals")}</h1>
        <p className="mt-1 text-sm text-muted">{t("cp.message.deletion_requested")}</p>
      </div>

      {failure && (
        <p role="alert" className="text-sm rounded-lg bg-danger-soft text-danger border border-danger-border px-3 py-2">{failure}</p>
      )}

      {approvals.length === 0 && (
        <EmptyState icon={<Check className="size-6" />} title={t("cp.message.no_approvals")} />
      )}

      {approvals.map((approval) => (
        <Card key={approval.id} padding="none" className="overflow-hidden">
          <CardHeader className="flex-row items-center gap-3 border-b border-line px-4 py-3">
            <h2 className="flex-1 text-base leading-tight font-semibold text-foreground">{approval.action}</h2>
          </CardHeader>
          <div className="p-4 space-y-3">
            <dl className="grid gap-3 sm:grid-cols-3 text-sm">
              <div>
                <dt className="text-xs uppercase tracking-wide text-muted">
                  {t("cp.field.organisation")}
                </dt>
                <dd className="mt-0.5">
                  <Link href={`/cp/tenants/${approval.target_id}`} className="text-foreground hover:underline">
                    {approval.target_name || approval.target_id}
                  </Link>
                </dd>
              </div>
              <div>
                <dt className="text-xs uppercase tracking-wide text-muted">
                  {t("cp.field.requested_by")}
                </dt>
                <dd className="mt-0.5 text-foreground">{approval.requested_by_name}</dd>
              </div>
              <div>
                <dt className="text-xs uppercase tracking-wide text-muted">{t("cp.field.when")}</dt>
                <dd className="mt-0.5 text-foreground">{formatMoment(approval.requested_at)}</dd>
              </div>
            </dl>

            <p className="text-sm rounded-lg bg-surface-2 border border-line px-3 py-2 text-foreground">
              {approval.requested_reason}
            </p>

            <div className="flex gap-2">
              <Button
                type="button"
                variant="destructive"
                leadingIcon={<Check className="w-4 h-4" />}
                onClick={() =>
                  action.run({
                    title: t("cp.action.approve"),
                    detail: approval.target_name,
                    danger: true,
                    perform: (reason) => cp.approve(approval.id, reason),
                    onDone: load,
                  })
                }
              >
                {t("cp.action.approve")}
              </Button>
              <Button
                type="button"
                variant="outline"
                leadingIcon={<X className="w-4 h-4" />}
                onClick={() =>
                  action.run({
                    title: t("cp.action.reject"),
                    detail: approval.target_name,
                    perform: (reason) => cp.reject(approval.id, reason),
                    onDone: load,
                  })
                }
              >
                {t("cp.action.reject")}
              </Button>
            </div>
          </div>
        </Card>
      ))}

      {action.dialog}
    </div>
  );
}
