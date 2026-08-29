"use client";

import Link from "next/link";
import {useEffect, useState} from "react";
import {ArrowRight, FileSignature} from "lucide-react";

import EIDLogin from "@/components/EIDLogin";
import {APPLICATIONS} from "@/components/landing/content";
import {useAccess} from "@/lib/access";
import {contracts, InboxItem} from "@/lib/contracts";
import {LOCALES, useI18n} from "@/lib/i18n";

/**
 * The first screen: what the platform is, and the eID panel to act on it.
 *
 * The headline is about the platform rather than the sign-in, because that is
 * the question a visitor arrives with. The sign-in panel still sits beside it
 * rather than behind the header button: the shortest path from landing to
 * signed-in is worth keeping even when it is no longer the argument.
 *
 * `seeMoreAnchor` is where the second button goes — the first section the menu
 * links, which is a page of its own (`app/[section]`) named by that same word.
 * It is decided by the page rather than written in here, because a deployment
 * may not render the section this used to point at. Absent, the menu links
 * nothing and the button is not drawn: a page whose only call to action leads
 * nowhere is worse than one with a single button.
 *
 * `localSignIn` is whether this deployment signs people in itself. False means
 * it is a client of somebody else's provider, and then the eID card here is a
 * form that cannot submit: the endpoints behind it sit past `requireLocalLogin`
 * and answer 403 — a visitor types a registration number, presses the button
 * and nothing happens. So the card goes, and the first button becomes what the
 * header's already is: a link to /login, which knows to hand the visitor on.
 */
export default function Hero({
  seeMoreAnchor,
  localSignIn = true,
}: {seeMoreAnchor?: string; localSignIn?: boolean}) {
  const {t} = useI18n();
  // Нэвтэрсэн хүнд энэ хуудас ӨӨР асуултад хариулна: «надад юу ирсэн бэ».
  // Нөхцөл нь зөвхөн `me` — эхний client render дээр null тул сервертэй яг
  // ижил markup гарна (hydration зөрөхгүй), нэвтрээгүй зочинд юу ч
  // өөрчлөгдөхгүй.
  const {me} = useAccess();
  const [contractInbox, setContractInbox] = useState<InboxItem[] | null>(null);

  // A signed-in account, or even an installed Documents module, is not evidence
  // that this distribution carries the newer contracts API. The screen is a
  // capability client, so only a successful endpoint response may advertise
  // it. Current base and client distributions do not provide that endpoint.
  useEffect(() => {
    let alive = true;
    if (!me) {
      setContractInbox(null);
      return () => {
        alive = false;
      };
    }
    contracts.inbox(false)
      .then((response) => {
        if (alive) setContractInbox(response.items);
      })
      .catch(() => {
        if (alive) setContractInbox(null);
      });
    return () => {
      alive = false;
    };
  }, [me]);

  const showContracts = Boolean(me && contractInbox !== null);

  return (
    <section className="gp-hero">
      <div className="gp-pattern" />
      <div className={`gp-hero__inner${localSignIn ? "" : " gp-hero__inner--solo"}`}>
        <div className="gp-copy">
          <span className="gp-eyebrow">
            <i /> OPEN SOURCE · APACHE 2.0 · GO
          </span>
          <h1>
            {t("website.view.hero_title_lead")} <em>{t("website.view.hero_title_highlight")}</em>{" "}
            {t("website.view.hero_title_tail")}
          </h1>
          <p>{t("website.view.hero_lede")}</p>
          <div className="gp-cta">
            {showContracts ? (
              <Link href="/module/documents/inbox" className="gp-gold gp-gold--large">
                {t("website.action.my_contracts")} <ArrowRight />
              </Link>
            ) : me ? (
              <Link href="/apps" className="gp-gold gp-gold--large">
                {t("website.action.open_platform")} <ArrowRight />
              </Link>
            ) : localSignIn ? (
              <a href="#eid-login" className="gp-gold gp-gold--large">
                {t("website.action.eid_sign_in")} <ArrowRight />
              </a>
            ) : (
              <Link href="/login" className="gp-gold gp-gold--large">
                {t("website.action.sign_in")} <ArrowRight />
              </Link>
            )}
            {showContracts ? (
              <Link href="/apps" className="gp-outline">
                {t("website.action.open_platform")}
              </Link>
            ) : seeMoreAnchor ? (
              <Link href={`/${seeMoreAnchor}`} className="gp-outline">
                {t("website.action.see_features")}
              </Link>
            ) : null}
          </div>
          <div className="gp-stats">
            <span>
              <b>{APPLICATIONS.length}</b>
              {t("website.stat.apps")}
            </span>
            <span>
              <b>{LOCALES.length}</b>
              {t("website.stat.languages")}
            </span>
            <span>
              <b>{t("website.stat.binary_count")}</b>
              {t("website.stat.binary")}
            </span>
          </div>
        </div>
        {showContracts ? (
          <div className="gp-login-slot">
            <HeroInbox items={contractInbox || []} />
          </div>
        ) : localSignIn ? (
          <div id="eid-login" className="gp-login-slot">
            <EIDLogin compact />
          </div>
        ) : null}
      </div>
    </section>
  );
}

/**
 * Нэвтэрсэн хүний ИРСЭН ГЭРЭЭ, eID картын суусан яг тэр нүдэнд.
 *
 * Захирал eID-ээрээ орж ирээд өөр юу ч хайхгүй: гэрээ нь нүүрэн дээр нь
 * байна. Жагсаалт нь хариу хүлээж буй гэрээ л — түүх биш: нүүр хуудас бол
 * ажлын ширээ, архив нь Ирсэн гэрээ дэлгэцэд.
 */
function HeroInbox({items}: {items: InboxItem[]}) {
  const {t} = useI18n();

  return (
    <div className="rounded-2xl bg-white/95 shadow-xl border border-slate-200 p-6 w-full max-w-md">
      <div className="flex items-center gap-2 mb-4">
        <FileSignature className="w-5 h-5 text-indigo-600" />
        <h3 className="font-bold text-slate-900">{t("website.view.hero_inbox_title")}</h3>
      </div>
      {items.length === 0 ? (
        <p className="text-sm text-slate-500">{t("website.view.hero_inbox_empty")}</p>
      ) : (
        <ul className="space-y-2">
          {items.slice(0, 4).map((item) => (
            <li key={item.party_id}>
              <Link
                href={`/module/documents/inbox/${item.party_id}`}
                className="block rounded-xl border border-slate-200 hover:border-indigo-300 hover:bg-indigo-50/40 px-4 py-3"
              >
                <div className="text-sm font-semibold text-slate-800">{item.title}</div>
                <div className="text-xs text-slate-500">
                  {item.issuer_name}
                  {" · "}
                  {item.state === "invited"
                    ? t("website.view.hero_inbox_new")
                    : t("website.view.hero_inbox_opened")}
                </div>
              </Link>
            </li>
          ))}
          {items.length > 4 ? (
            <li className="text-xs text-slate-400 pt-1">
              <Link href="/module/documents/inbox" className="hover:underline">
                +{items.length - 4} {t("website.view.hero_inbox_more")}
              </Link>
            </li>
          ) : null}
        </ul>
      )}
    </div>
  );
}
