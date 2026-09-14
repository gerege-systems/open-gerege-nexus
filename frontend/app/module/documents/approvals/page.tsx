"use client";

import React, { useMemo, useRef, useState } from "react";
import { api } from "@/lib/api";
import { useLoadOnMount } from "@/lib/useResource";
import { useAccess } from "@/lib/access";
import { useI18n } from "@/lib/i18n";
import { PageHeader } from "@/components/ui";
import {
  Alert,
  Card,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@gerege-systems/ui";
import { DocumentRecord, ListEmpty, ListSkeleton, LoadMoreFooter, PENDING, RowActions, StaleNotice, StatTile, SignatureDialog, SignatureHistoryButton, SignatureHistoryDialog, SignatureProgress, useDocumentActions } from "@/components/documents/shared";
import { ListChecks } from "lucide-react";

/**
 * Approval queue: the documents list narrowed to what is actually waiting for a
 * decision. It reads the same /documents collection and drives the same sign
 * and reject endpoints — the queue is a lens on that data, not a second store.
 */
export default function DocumentApprovalsPage() {
  const { t } = useI18n();
  const { can, ready } = useAccess();
  const [documents, setDocuments] = useState<DocumentRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [signTarget, setSignTarget] = useState<DocumentRecord | null>(null);
  const [historyTarget, setHistoryTarget] = useState<DocumentRecord | null>(null);

  // "Nothing is waiting for approval" is a claim, and an approver who reads it
  // closes the tab. A load that failed says so instead.
  const [loadFailed, setLoadFailed] = useState(false);

  const [total, setTotal] = useState(0);
  // Whether a further page may exist. Kept separately from the total, which other
  // people can change between two of this walk's requests.
  const [hasMore, setHasMore] = useState(false);
  // Whether the queue has been read at all yet. Until it has, the tiles have nothing to
  // state and say so with a dash instead of standing at zero.
  const [answered, setAnswered] = useState(false);

  // The queue asks the SERVER for what is waiting, oldest first. Filtering a capped
  // page in the browser would let a document waiting for a signature fall off the end
  // of a page full of approved ones — the one document this screen exists to show.
  //
  // More rows means walking OFFSET: the server clamps a limit at ListLimitMax, so
  // asking for a bigger one stopped at 500 and left the rest of a long queue with no
  // way to be signed or rejected from any screen.
  const PAGE = 200;

  // Only the newest load may write. A load walks several pages, so two of them
  // overlapping — Load more while an action's refresh is still running — would
  // otherwise assemble a queue out of both and report a total belonging to neither.
  const loadTicket = useRef(0);

  const loadSpan = async (rows: number) => {
    const mine = ++loadTicket.current;
    setLoading(true);
    try {
      const wanted = Math.max(PAGE, rows);
      const collected: DocumentRecord[] = [];
      let counted = 0;
      // Walked by CURSOR, not by offset: offset counts from the start of a set other
      // people are changing, so a document approved between two of these requests
      // shifts the rest up and the next request skips one — and a skipped document is
      // on no screen at all. The cursor names the last row actually seen.
      //
      // The end is a page that comes back short, which is a fact about the data rather
      // than a comparison against a total that may have moved in the meantime.
      let cursor: { after_at: string; after_id: string } | undefined;
      let ranOut = false;
      while (collected.length < wanted) {
        const page = await api.getDocuments({ status: PENDING, order: "oldest", limit: PAGE, ...cursor });
        if (loadTicket.current !== mine) return;
        counted = page?.total ?? 0;
        const rowsBack = page?.documents || [];
        collected.push(...rowsBack);
        if (rowsBack.length < PAGE) {
          ranOut = true;
          break;
        }
        const last = rowsBack[rowsBack.length - 1];
        cursor = { after_at: last.created_at, after_id: last.id };
      }
      // There is more to read when the walk stopped because it had enough, not because
      // the data ran out. That is a fact about what came back, so it holds even when the
      // total has moved; the total only says roughly how much more.
      setHasMore(!ranOut);
      setDocuments(collected);
      setTotal(counted);
      setAnswered(true);
      setLoadFailed(false);
    } catch (err: any) {
      // A superseded load says nothing: the newer one speaks for the screen.
      if (loadTicket.current !== mine) return;
      setLoadFailed(true);
      // Only when there is nothing on screen to carry the news. With rows showing, the
      // footer says the list is stale and stays saying it — and a banner here would
      // overwrite what the action that triggered this refresh had just reported, so a
      // completed signature read as a failure.
      if (documents.length === 0) {
        setMessage({ type: "error", text: err?.message || t("documents.message.load_failed") });
      }
    } finally {
      // The spinner belongs to the load that is still running. A superseded one clearing
      // it let the screen fall through to "no documents yet" while the newest load was
      // still in flight — a claim about the tenant, made in the gap. The newest load
      // always clears it in its own finally, so nothing can be left spinning.
      if (loadTicket.current === mine) setLoading(false);
    }
  };

  // Refreshes exactly what is on screen, so approving the document at row 350 does not
  // throw the approver back to the head of the queue.
  const loadData = () => loadSpan(documents.length);
  const loadMore = () => loadSpan(documents.length + PAGE);

  const { isBusy, message, setMessage, succeed, fail, reject } = useDocumentActions(loadData);

  useLoadOnMount(() => loadSpan(PAGE));

  const pending = useMemo(() => documents.filter((doc) => doc.status === PENDING), [documents]);

  const byType = useMemo(() => {
    const counts = new Map<string, number>();
    for (const doc of pending) {
      counts.set(doc.doc_type, (counts.get(doc.doc_type) || 0) + 1);
    }
    return [...counts.entries()].sort((a, b) => b[1] - a[1]);
  }, [pending]);

  // The oldest document in the queue is the one an approver should worry about — and
  // the server sends this page oldest first, so it is the first row rather than the
  // oldest of whatever this page happens to hold.
  const waitingSince = useMemo(() => pending[0] ?? null, [pending]);

  const days = (iso: string) => Math.floor((Date.now() - new Date(iso).getTime()) / 86_400_000);

  return (
    <div className="space-y-6">
      <PageHeader
        icon={<ListChecks className="w-7 h-7 text-accent" />}
        title={t("documents.menu.approvals")}
        subtitle={t("documents.view.approvals_hint")}
      />

      {message && (
        <Alert variant={message.type === "error" ? "danger" : message.type} live dismissible onDismiss={() => setMessage(null)}>
          {message.text}
        </Alert>
      )}

      {/* With no rows there is no table footer to carry this, and a refresh that failed
          after an action is exactly when there are none: the news that the list is stale
          — and the way to try again — must not live only inside the table. */}
      {loadFailed && pending.length === 0 && <StaleNotice busy={loading} onRetry={() => loadData()} />}

      {/* A tile is as much a claim as a sentence. "Awaiting signature: 0" over a
          queue the page could not read is the same falsehood the prose below is
          careful not to tell, so a failed load shows a dash in both places. */}
      <section className="grid grid-cols-2 md:grid-cols-4 gap-3">
        {/* The queue's real size, not this page's — the server counted it. A dash
            until there is something to state: during the first load these read 0,
            which is the same falsehood the dash exists to prevent. */}
        <StatTile tone="warning" value={loadFailed || !answered ? "—" : total} label={t("documents.stat.awaiting")} />
        <StatTile
          value={loadFailed || !answered ? "—" : waitingSince ? days(waitingSince.created_at) : 0}
          label={t("documents.stat.oldest_days")}
        />
        {/* Counted over the rows this page holds, so a partial queue is marked: the
            server counts the queue as a whole but not its breakdown by type. */}
        {byType.slice(0, 2).map(([docType, count]) => (
          <StatTile key={docType} tone="accent" mono value={total > pending.length ? `≥${count}` : count} label={docType} />
        ))}
      </section>

      {/* The table stays up while a refresh runs, so the click that started it is not
          swallowed by the table coming down under the pointer. */}
      {loading && pending.length === 0 ? (
        <ListSkeleton label={t("documents.message.loading")} />
      ) : pending.length === 0 ? (
        // "Nothing is waiting" is a claim about the queue, so only a load that
        // succeeded may make it.
        loadFailed ? null : <ListEmpty icon={<ListChecks />} title={t("documents.message.no_pending")} />
      ) : (
        <Card padding="none" className="overflow-hidden">
          <Table containerClassName="rounded-none border-0" className="text-xs" scrollLabel={t("documents.menu.approvals")}>
            <TableHeader>
              <TableRow>
                <TableHead>{t("documents.field.title")}</TableHead>
                <TableHead>{t("base.field.type")}</TableHead>
                <TableHead>{t("documents.field.created")}</TableHead>
                <TableHead>{t("documents.field.waiting_days")}</TableHead>
                <TableHead align="right">{t("base.field.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {pending.map((doc) => (
                <TableRow key={doc.id}>
                  <TableCell className="font-semibold">{doc.title}</TableCell>
                  <TableCell className="font-mono text-muted">
                    <div className="flex flex-wrap items-center gap-1.5">
                      <span>{doc.doc_type}</span>
                      <SignatureProgress doc={doc} />
                      <SignatureHistoryButton doc={doc} onOpen={setHistoryTarget} />
                    </div>
                  </TableCell>
                  <TableCell className="text-muted">{new Date(doc.created_at).toLocaleDateString()}</TableCell>
                  <TableCell className="text-muted">{days(doc.created_at)}</TableCell>
                  <TableCell align="right">
                    <RowActions
                      doc={doc}
                      busy={isBusy(doc.id)}
                      canSign={can("documents.sign")}
                      onSign={setSignTarget}
                      onReject={reject}
                    />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>

          {/* A stale list says so for as long as it is stale — the banner can be
              dismissed, and a refresh that failed after an action must not be the only
              thing that says the rows are old. */}
          {loadFailed && <StaleNotice inset busy={loading} onRetry={() => loadData()} />}

          {/* A queue shown in part says so, and can be read to the end. */}
          {hasMore && (
            <LoadMoreFooter
              text={t("documents.message.showing_some_oldest", { shown: pending.length, total })}
              busy={loading}
              onMore={loadMore}
            />
          )}
        </Card>
      )}

      {ready && !can("documents.sign") && pending.length > 0 && (
        <p className="text-xs text-muted">{t("documents.message.sign_not_granted")}</p>
      )}

      {signTarget && (
        <SignatureDialog
          key={signTarget.id}
          doc={signTarget}
          onClose={() => setSignTarget(null)}
          onDone={async (text) => {
            setSignTarget(null);
            await succeed(text);
          }}
          onError={fail}
        />
      )}

      {historyTarget && (
        <SignatureHistoryDialog doc={historyTarget} onClose={() => setHistoryTarget(null)} />
      )}
    </div>
  );
}
