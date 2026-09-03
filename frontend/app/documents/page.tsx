"use client";

import React, { useRef, useState } from "react";
import { api } from "@/lib/api";
import { useLoadOnMount } from "@/lib/useResource";
import { useAccess } from "@/lib/access";
import { useI18n } from "@/lib/i18n";
import { Banner, LoadingBlock, Modal, TableCard, fieldClass, selectClass } from "@/components/ui";
import { DocumentRecord, PENDING, RowActions, SignatureCell, SignatureDialog, SignatureHistoryButton, SignatureHistoryDialog, SignatureProgress, StatusBadge, useDocumentActions } from "@/components/documents/shared";
import { FileText, Pencil, Plus } from "lucide-react";

export default function DocumentsPage() {
  const { t } = useI18n();
  const { can } = useAccess();
  const [documents, setDocuments] = useState<DocumentRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [showModal, setShowModal] = useState(false);
  const [form, setForm] = useState({ title: "", doc_type: "CONTRACT" });
  const [signTarget, setSignTarget] = useState<DocumentRecord | null>(null);
  const [historyTarget, setHistoryTarget] = useState<DocumentRecord | null>(null);

  // A failed load must never be reported as an empty list: "No documents yet" is a
  // claim about the tenant, and twelve contracts may be sitting in the queue.
  const [loadFailed, setLoadFailed] = useState(false);

  // How many rows this screen has asked for. The list is answered one page at a time —
  // each row counts its own signatures and outstanding steps — so more rows means
  // walking OFFSET, not asking for a bigger limit: the server clamps a limit at
  // ListLimitMax, so raising it stopped at 500 and left the last documents of a large
  // tenant unreachable from any screen.
  const PAGE = 200;
  const [total, setTotal] = useState(0);
  // Whether a further page may exist. Kept separately from the total, which other
  // people can change between two of this walk's requests.
  const [hasMore, setHasMore] = useState(false);

  // What the operator is looking for. Mirrored in a ref because loadSpan is also called
  // by the actions hook, which captured it at an earlier render: a refresh after signing
  // must not silently drop the filter the rows on screen were fetched under.
  const [search, setSearch] = useState("");
  const [docType, setDocType] = useState("");
  const [status, setStatus] = useState("");
  const filterRef = useRef({ q: "", doc_type: "", status: "" });
  filterRef.current = { q: search, doc_type: docType, status };

  // The filter the rows on screen were actually fetched under, so a blur that changed
  // nothing does not throw away the pages the operator has loaded, and the empty state
  // describes the question that was asked rather than the one being typed.
  const loadedFilter = useRef({ q: "", doc_type: "", status: "" });

  // Only the newest load may write. A load walks several pages, so without this a
  // filter change part-way through assembled a row set from two different filters, with
  // a total belonging to neither — and the screen presented it as the whole answer to
  // the filter shown.
  const loadTicket = useRef(0);

  // A load asks ONE question, given to it explicitly.
  //
  // Refreshes and Load more pass nothing and get the filter the rows on screen were
  // fetched under; only the controls themselves pass the live one. Reading the live
  // filter for every load meant a half-typed search changed what the table showed the
  // next time an action refreshed it — the operator signed a document and the list
  // became the answer to a question they had not finished asking.
  const loadSpan = async (rows: number, asked?: { q: string; doc_type: string; status: string }) => {
    const mine = ++loadTicket.current;
    const filter = { ...(asked ?? loadedFilter.current) };
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
        // Гэрээ энэ бүртгэлд харагдахгүй: түүнд өөрийн дэлгэц, өөрийн
        // төлөв, өөрийн гарын үсгийн зам бий — хоёр дэлгэцэд зэрэг гарахад
        // хүмүүс талуудыг мэдэхгүй энэ дэлгэцийн товчоор зурдаг байв.
        const page = await api.getDocuments({ ...filter, limit: PAGE, contracts: "exclude", ...cursor });
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
      // What the rows on screen are the answer to. Trimmed, because that is what the
      // server compared, and kept so the empty state can speak about the filter the
      // rows were fetched under rather than whatever is in the boxes now.
      loadedFilter.current = { q: filter.q.trim(), doc_type: filter.doc_type, status: filter.status };
      setLoadFailed(false);
    } catch (err: any) {
      // A load that has been superseded says nothing: the newer one speaks for the
      // screen, and its own failure would be news about a question nobody is asking.
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

  // Refreshes exactly what is on screen, so signing a document 350 rows down does not
  // throw the operator back to the first page.
  const loadData = () => loadSpan(documents.length);
  const loadMore = () => loadSpan(documents.length + PAGE);

  const { isBusy, message, setMessage, succeed, fail, route, reject } = useDocumentActions(loadData);

  useLoadOnMount(() => loadSpan(PAGE, filterRef.current));

  // Which row's title is being corrected, and what has been typed into it. A title may
  // be fixed until the first signature: creating a document routes it immediately, which
  // is one step instead of two, and this is what stops that costing a permanent typo.
  const [renaming, setRenaming] = useState<{ id: string; title: string } | null>(null);

  const commitRename = async () => {
    if (!renaming) return;
    const { id, title } = renaming;
    const row = documents.find((d) => d.id === id);
    setRenaming(null);
    if (!row || title.trim() === "" || title.trim() === row.title) return;
    setMessage(null);
    try {
      const saved = (await api.renameDocument(id, title.trim())) as DocumentRecord | undefined;
      // Only this row, with what the server stored.
      if (saved?.id) setDocuments((current) => current.map((d) => (d.id === id ? { ...d, ...saved } : d)));
      setMessage({ type: "success", text: t("documents.message.renamed", { title: saved?.title ?? title.trim() }) });
    } catch (err: any) {
      fail(`${t("documents.message.rename_failed")}: ${err?.message ?? ""}`);
    }
  };

  // Nothing between the click and the POST changed any state, so a second click —
  // or Enter held down — created a second document, each one routed for approval.
  const [creating, setCreating] = useState(false);
  // The modal's own overlay covers the page banner, so a refusal reported only there
  // is a refusal the operator cannot read: the dialog just sat there with the button
  // re-enabled and no reason given. This is shown inside the modal.
  const [createFailure, setCreateFailure] = useState<string | null>(null);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (creating) return;
    setCreating(true);
    // Cleared before the attempt, and replaced after it. A refused create left its
    // red banner up; when the operator corrected the title and succeeded, that
    // banner was still the only thing the screen said about the operation — and a
    // document read as "failed" is submitted again, which lands a duplicate in the
    // approval queue, since a new document is routed for approval on creation.
    setMessage(null);
    setCreateFailure(null);
    const title = form.title;
    try {
      await api.createDocument(form);
      setShowModal(false);
      setForm({ title: "", doc_type: "CONTRACT" });
      await succeed(t("documents.message.create_success", { title }));
      return;
    } catch (err: any) {
      // Both: inside the modal, where it is readable now, and on the page banner for
      // after the operator gives up and closes it.
      const text = `${t("documents.message.create_failed")}: ${err.message}`;
      setCreateFailure(text);
      setMessage({ type: "error", text });
    } finally {
      setCreating(false);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-foreground flex items-center space-x-2">
            <FileText className="w-7 h-7 text-indigo-600" />
            <span>{t("documents.view.title")}</span>
          </h1>
          <p className="text-sm text-muted mt-1">{t("documents.view.subtitle")}</p>
        </div>
        {can("documents.manage") && (
          <button
            onClick={() => setShowModal(true)}
            className="bg-indigo-600 hover:bg-indigo-700 text-white text-xs font-semibold px-4 py-2 rounded-lg flex items-center space-x-2 shadow-sm transition"
          >
            <Plus className="w-4 h-4" />
            <span>{t("documents.action.create")}</span>
          </button>
        )}
      </div>

      {message && <Banner tone={message.type} message={message.text} onDismiss={() => setMessage(null)} />}

      {/* With no rows there is no table footer to carry this, and a refresh that failed
          after an action is exactly when there are none: the news that the list is stale
          — and the way to try again — must not live only inside the table. */}
      {loadFailed && documents.length === 0 && (
        <div className="flex items-center justify-between gap-3 px-4 py-3 border border-amber-200 rounded-xl bg-amber-50">
          <p className="text-[11px] text-amber-800">{t("documents.message.stale_rows")}</p>
          <button
            type="button"
            disabled={loading}
            onClick={() => loadData()}
            className="text-[11px] font-semibold px-3 py-1.5 rounded-lg bg-surface border border-amber-300 text-amber-800 hover:bg-amber-100 disabled:opacity-50"
          >
            {t("documents.action.retry")}
          </button>
        </div>
      )}

      {/* Paging reads the list; this is how a particular document is found. Each change
          starts again from the first page, because the rows below have to be the answer
          to what is in these controls and nothing else. */}
      <section className="flex flex-wrap items-center gap-2">
        <input
          type="text"
          value={search}
          placeholder={t("documents.field.search_placeholder")}
          maxLength={255}
          onChange={(e) => setSearch(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") loadSpan(PAGE, filterRef.current);
          }}
          onBlur={() => {
            // Only when it actually changed. Reloading on every blur threw away the
            // pages the operator had loaded — and, because the table came down while
            // the load ran, the click that caused the blur (Sign, Reject, Load more)
            // was swallowed with it and the action never happened.
            if (search.trim() !== loadedFilter.current.q) loadSpan(PAGE, filterRef.current);
          }}
          className="flex-1 min-w-[12rem] px-3 py-2 text-sm border border-input rounded-lg focus:ring-2 focus:ring-indigo-500"
        />
        <select
          value={docType}
          onChange={(e) => {
            setDocType(e.target.value);
            filterRef.current = { ...filterRef.current, doc_type: e.target.value };
            loadSpan(PAGE, filterRef.current);
          }}
          className={selectClass}
        >
          <option value="">{t("documents.field.any_type")}</option>
          <option value="CONTRACT">{t("documents.category.legal_contract")}</option>
          <option value="REQUEST">{t("documents.category.official_request")}</option>
          <option value="APPROVAL">{t("documents.category.internal_approval")}</option>
        </select>
        <select
          value={status}
          onChange={(e) => {
            setStatus(e.target.value);
            filterRef.current = { ...filterRef.current, status: e.target.value };
            loadSpan(PAGE, filterRef.current);
          }}
          className={selectClass}
        >
          <option value="">{t("documents.field.any_status")}</option>
          <option value="DRAFT">{t("documents.state.draft")}</option>
          <option value="PENDING_APPROVAL">{t("documents.state.pending")}</option>
          <option value="APPROVED">{t("documents.state.approved")}</option>
          <option value="REJECTED">{t("documents.state.rejected")}</option>
        </select>
      </section>

      {loading && documents.length === 0 ? (
        <LoadingBlock label={t("documents.message.loading")} />
      ) : documents.length === 0 ? (
        // Only a load that succeeded may claim the tenant has no documents; a
        // failed one says so in the banner and shows whatever it already had.
        loadFailed ? null : (
          <div className="bg-surface border border-line rounded-xl p-8 text-center text-muted text-sm">
            {/* With a filter on, an empty answer says nothing about the tenant — and the
                filter that matters is the one these rows were fetched under. Reading the
                live boxes instead let clearing the search box turn "nothing matches"
                into "no documents yet", about a tenant holding hundreds. */}
            {loadedFilter.current.q || loadedFilter.current.doc_type || loadedFilter.current.status
              ? t("documents.message.no_matches")
              : t("documents.message.empty")}
          </div>
        )
      ) : (
        <TableCard
          head={
            <tr>
              <th className="px-4 py-3">{t("documents.field.title")}</th>
              <th className="px-4 py-3">{t("base.field.type")}</th>
              <th className="px-4 py-3">{t("base.field.status")}</th>
              <th className="px-4 py-3">{t("documents.field.signature")}</th>
              <th className="px-4 py-3">{t("documents.field.created")}</th>
              <th className="px-4 py-3 text-right">{t("base.field.actions")}</th>
            </tr>
          }
          footer={
            <>
            {/* A stale list says so for as long as it is stale — the banner can be
                dismissed, and a refresh that failed after an action must not be the only
                thing that says the rows are old. */}
            {loadFailed && (
              <div className="flex items-center justify-between gap-3 px-4 py-3 border-t border-amber-200 bg-amber-50">
                <p className="text-[11px] text-amber-800">{t("documents.message.stale_rows")}</p>
                <button
                  type="button"
                  disabled={loading}
                  onClick={() => loadData()}
                  className="text-[11px] font-semibold px-3 py-1.5 rounded-lg bg-surface border border-amber-300 text-amber-800 hover:bg-amber-100 disabled:opacity-50"
                >
                  {t("documents.action.retry")}
                </button>
              </div>
            )}


            {/* A partial list says so. Silence here is what makes an operator search for
                a contract that is simply on the next page. */}
            {hasMore && (
              <div className="flex items-center justify-between gap-3 px-4 py-3 border-t border-line bg-surface-2">
                <p className="text-[11px] text-muted">
                  {t("documents.message.showing_some", { shown: documents.length, total })}
                </p>
                <button
                  type="button"
                  disabled={loading}
                  onClick={loadMore}
                  className="text-[11px] font-semibold px-3 py-1.5 rounded-lg bg-surface border border-input text-foreground hover:bg-surface-hover disabled:opacity-50"
                >
                  {t("documents.action.load_more")}
                </button>
              </div>
            )}
            </>
          }
        >
          {documents.map((doc) => (
            <tr key={doc.id} className="hover:bg-surface-hover">
              <td className="px-4 py-3 font-semibold text-foreground">
                {renaming?.id === doc.id ? (
                  <input
                    autoFocus
                    value={renaming.title}
                    maxLength={255}
                    onChange={(e) => setRenaming({ id: doc.id, title: e.target.value })}
                    onBlur={commitRename}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") commitRename();
                      if (e.key === "Escape") setRenaming(null);
                    }}
                    className="w-full px-2 py-1 text-xs font-semibold border border-indigo-300 rounded focus:ring-2 focus:ring-indigo-500"
                  />
                ) : (
                  <span className="flex items-center gap-1.5">
                    <span>{doc.title}</span>
                    {/* Offered only while it can actually be changed: nobody has signed,
                        the document is still open, and this operator may author. */}
                    {can("documents.manage") &&
                      doc.signature_count === 0 &&
                      (doc.status === PENDING || doc.status === "DRAFT") && (
                        <button
                          type="button"
                          title={t("documents.action.rename")}
                          aria-label={t("documents.action.rename")}
                          onClick={() => setRenaming({ id: doc.id, title: doc.title })}
                          className="shrink-0 text-subtle hover:text-indigo-600"
                        >
                          <Pencil className="w-3.5 h-3.5" />
                        </button>
                      )}
                  </span>
                )}
              </td>
              <td className="px-4 py-3 font-mono text-muted">{doc.doc_type}</td>
              <td className="px-4 py-3">
                <div className="flex flex-wrap items-center gap-1.5">
                  <StatusBadge status={doc.status} />
                  <SignatureProgress doc={doc} />
                </div>
              </td>
              <td className="px-4 py-3">
                <div className="flex items-start gap-2">
                  <SignatureCell doc={doc} />
                  <SignatureHistoryButton doc={doc} onOpen={setHistoryTarget} />
                </div>
              </td>
              <td className="px-4 py-3 text-muted">{new Date(doc.created_at).toLocaleDateString()}</td>
              <td className="px-4 py-3 text-right">
                <RowActions
                  doc={doc}
                  busy={isBusy(doc.id)}
                  canSign={can("documents.sign")}
                  canManage={can("documents.manage")}
                  onSign={setSignTarget}
                  onReject={reject}
                  onRoute={route}
                />
              </td>
            </tr>
          ))}
        </TableCard>
      )}

      {/* Modal */}
      {showModal && (
        <Modal label={t("documents.view.create_title")}>
          <h2 className="text-xl font-semibold text-foreground mb-4">{t("documents.view.create_title")}</h2>

          {createFailure && (
            <p className="mb-4 text-xs text-red-700 bg-red-50 border border-red-200 rounded-lg p-3">
              {createFailure}
            </p>
          )}

          <form onSubmit={handleCreate} className="space-y-4">
            {/* maxLength is what document_records.title holds, in the characters
                Postgres counts. The server refuses more, so there is no reason to
                let it be typed and then refused. */}
            <div>
              <label className="block text-xs font-semibold text-foreground mb-1">
                {t("documents.field.title")} *
              </label>
              <input
                type="text"
                placeholder={t("documents.field.title_placeholder")}
                value={form.title}
                onChange={(e) => setForm({ ...form, title: e.target.value })}
                className={fieldClass}
                maxLength={255}
                required
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-foreground mb-1">{t("documents.field.category")}</label>
              <select
                value={form.doc_type}
                onChange={(e) => setForm({ ...form, doc_type: e.target.value })}
                className={selectClass}
              >
                <option value="CONTRACT">{t("documents.category.legal_contract")}</option>
                <option value="REQUEST">{t("documents.category.official_request")}</option>
                <option value="APPROVAL">{t("documents.category.internal_approval")}</option>
              </select>
            </div>

            <div className="flex items-center space-x-2 pt-2">
              <button
                type="button"
                onClick={() => {
                  setShowModal(false);
                  setCreateFailure(null);
                }}
                className="w-1/2 bg-surface-2 hover:bg-slate-200 text-foreground font-medium py-2 rounded-lg text-xs"
              >
                {t("base.action.cancel")}
              </button>
              <button
                type="submit"
                disabled={creating || !form.title.trim()}
                className="w-1/2 bg-indigo-600 hover:bg-indigo-700 text-white font-medium py-2 rounded-lg text-xs disabled:opacity-50"
              >{t("documents.action.create")}</button>
            </div>
          </form>
        </Modal>
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
