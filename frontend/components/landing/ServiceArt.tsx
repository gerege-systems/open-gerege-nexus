import type {ServiceId} from "@/lib/services";

/**
 * One drawing per service.
 *
 * Inline SVG rather than image files, for three reasons that all point the
 * same way: they are part of the page's markup so they cost no second request
 * and cannot 404, they take their colours from the same palette the rest of
 * the page uses, and they stay sharp on a phone and on a 5K display without
 * anybody maintaining three sizes of each.
 *
 * The style is deliberately plain-spoken — thick rounded strokes, flat fills,
 * no gradients or shadows. These sit next to a paragraph each, and a drawing
 * that competes with its own caption is decoration. Each one shows the thing
 * the service actually is: an ID card, a desk of controls, a warehouse, a
 * vault, a pulse, a book.
 *
 * `aria-hidden` on every one of them: the card's heading and body already say
 * what the drawing shows, and a screen reader announcing "image" a second time
 * adds nothing but length.
 */

const NAVY = "#0050b0";
const BLUE = "#0064e1";
const GOLD = "#f2bd42";
const PAPER = "#ffffff";
const TINT = "#dbe9fb";
const INK = "#15233b";

type Props = {className?: string};

/** eID Mongolia — an ID card, and the thumb that unlocks it. */
function EidArt({className}: Props) {
  return (
    <svg className={className} viewBox="0 0 160 108" fill="none" aria-hidden="true">
      <rect x="10" y="16" width="140" height="76" rx="12" fill={TINT} />
      {/* the card */}
      <rect x="26" y="30" width="80" height="52" rx="8" fill={PAPER} stroke={NAVY} strokeWidth="3" />
      {/* the citizen */}
      <circle cx="48" cy="49" r="8" fill={NAVY} />
      <path d="M36 68c0-7 5-11 12-11s12 4 12 11" fill={NAVY} />
      {/* the fields */}
      <g stroke={TINT} strokeWidth="4" strokeLinecap="round">
        <path d="M70 46h24" />
        <path d="M70 56h24" />
        <path d="M70 66h16" />
      </g>
      {/* the thumb, reading */}
      <circle cx="120" cy="56" r="20" fill={PAPER} stroke={NAVY} strokeWidth="3" />
      <g stroke={GOLD} strokeWidth="2.6" strokeLinecap="round" fill="none">
        <path d="M120 46a10 10 0 00-10 10v6" />
        <path d="M120 46a10 10 0 0110 10v6" />
        <path d="M120 52a4 4 0 00-4 4v8" />
        <path d="M120 52a4 4 0 014 4v8" />
      </g>
    </svg>
  );
}

/** Операторын консол — a desk of controls, with one switch thrown. */
function AdminArt({className}: Props) {
  return (
    <svg className={className} viewBox="0 0 160 108" fill="none" aria-hidden="true">
      <rect x="10" y="16" width="140" height="76" rx="12" fill={TINT} />
      <rect x="22" y="28" width="116" height="46" rx="8" fill={PAPER} stroke={NAVY} strokeWidth="3" />
      {/* three sliders, the last one pushed over */}
      <g stroke={NAVY} strokeWidth="3" strokeLinecap="round">
        <path d="M34 42h40" />
        <path d="M34 54h40" />
        <path d="M34 66h40" />
      </g>
      <circle cx="46" cy="42" r="5" fill={PAPER} stroke={NAVY} strokeWidth="3" />
      <circle cx="62" cy="54" r="5" fill={PAPER} stroke={NAVY} strokeWidth="3" />
      <circle cx="70" cy="66" r="5" fill={GOLD} stroke={INK} strokeWidth="3" />
      {/* the status panel */}
      <rect x="90" y="38" width="34" height="26" rx="5" fill={NAVY} />
      <path d="M97 51l5 5 10-11" stroke={GOLD} strokeWidth="3.5" strokeLinecap="round" strokeLinejoin="round" />
      {/* the desk edge */}
      <path d="M18 82h124" stroke={NAVY} strokeWidth="3" strokeLinecap="round" />
    </svg>
  );
}

/** Дата агуулах — a warehouse, with rows going in and one shelf filled. */
function DwhArt({className}: Props) {
  return (
    <svg className={className} viewBox="0 0 160 108" fill="none" aria-hidden="true">
      <rect x="10" y="16" width="140" height="76" rx="12" fill={TINT} />
      {/* roof */}
      <path d="M34 48l46-24 46 24" stroke={NAVY} strokeWidth="3.5" strokeLinecap="round" strokeLinejoin="round" fill={PAPER} />
      {/* body */}
      <rect x="40" y="48" width="80" height="34" rx="5" fill={PAPER} stroke={NAVY} strokeWidth="3" />
      {/* three shelves; the bottom one full */}
      <g stroke={NAVY} strokeWidth="2.5">
        <rect x="49" y="55" width="18" height="9" rx="2" fill={PAPER} />
        <rect x="71" y="55" width="18" height="9" rx="2" fill={PAPER} />
        <rect x="93" y="55" width="18" height="9" rx="2" fill={PAPER} />
        <rect x="49" y="68" width="18" height="9" rx="2" fill={BLUE} />
        <rect x="71" y="68" width="18" height="9" rx="2" fill={BLUE} />
        <rect x="93" y="68" width="18" height="9" rx="2" fill={GOLD} />
      </g>
      {/* the arriving load */}
      <path d="M18 40h14" stroke={NAVY} strokeWidth="3" strokeLinecap="round" />
      <path d="M27 35l6 5-6 5" stroke={NAVY} strokeWidth="3" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

/** Нөөцлөлт — a vault, shut, with the day's copy already inside. */
function BackupsArt({className}: Props) {
  return (
    <svg className={className} viewBox="0 0 160 108" fill="none" aria-hidden="true">
      <rect x="10" y="16" width="140" height="76" rx="12" fill={TINT} />
      <rect x="40" y="28" width="80" height="60" rx="10" fill={PAPER} stroke={NAVY} strokeWidth="3.5" />
      {/* the dial */}
      <circle cx="80" cy="58" r="17" fill={NAVY} />
      <circle cx="80" cy="58" r="7" fill={GOLD} />
      <g stroke={PAPER} strokeWidth="3" strokeLinecap="round">
        <path d="M80 41v5" />
        <path d="M80 70v5" />
        <path d="M63 58h5" />
        <path d="M92 58h5" />
      </g>
      {/* hinges */}
      <g stroke={NAVY} strokeWidth="3" strokeLinecap="round">
        <path d="M40 42h-8" />
        <path d="M40 74h-8" />
      </g>
      {/* the arriving copy, sealed */}
      <rect x="118" y="44" width="26" height="20" rx="4" fill={GOLD} stroke={INK} strokeWidth="2.5" />
      <path d="M124 54h14" stroke={INK} strokeWidth="2.5" strokeLinecap="round" />
    </svg>
  );
}

/** Хяналт — a screen with a pulse, and the bell that rings when it stops. */
function MonitorArt({className}: Props) {
  return (
    <svg className={className} viewBox="0 0 160 108" fill="none" aria-hidden="true">
      <rect x="10" y="16" width="140" height="76" rx="12" fill={TINT} />
      <rect x="26" y="26" width="108" height="52" rx="8" fill={PAPER} stroke={NAVY} strokeWidth="3" />
      {/* the trace */}
      <path
        d="M36 60l12 0 6-14 8 26 7-18 6 8 5-4h30"
        stroke={BLUE}
        strokeWidth="3.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      {/* the stand */}
      <path d="M80 78v8" stroke={NAVY} strokeWidth="3" strokeLinecap="round" />
      <path d="M64 86h32" stroke={NAVY} strokeWidth="3" strokeLinecap="round" />
      {/* the bell */}
      <path
        d="M116 40c0-5 4-9 9-9s9 4 9 9v7l3 4h-24l3-4z"
        fill={GOLD}
        stroke={INK}
        strokeWidth="2.5"
        strokeLinejoin="round"
      />
      <path d="M122 55a3 3 0 006 0" stroke={INK} strokeWidth="2.5" strokeLinecap="round" />
    </svg>
  );
}

/** Баримт бичиг — an open book, with the page somebody left marked. */
function DocsArt({className}: Props) {
  return (
    <svg className={className} viewBox="0 0 160 108" fill="none" aria-hidden="true">
      <rect x="10" y="16" width="140" height="76" rx="12" fill={TINT} />
      {/* two leaves meeting at the spine */}
      <path
        d="M80 36c-9-6-20-8-32-7v42c12-1 23 1 32 7 9-6 20-8 32-7V29c-12-1-23 1-32 7z"
        fill={PAPER}
        stroke={NAVY}
        strokeWidth="3.5"
        strokeLinejoin="round"
      />
      <path d="M80 36v42" stroke={NAVY} strokeWidth="3" strokeLinecap="round" />
      {/* lines of text */}
      <g stroke={TINT} strokeWidth="3" strokeLinecap="round">
        <path d="M58 46h14" />
        <path d="M58 55h14" />
        <path d="M58 64h10" />
        <path d="M88 46h14" />
        <path d="M88 55h14" />
        <path d="M88 64h10" />
      </g>
      {/* the bookmark */}
      <path d="M100 29v22l7-6 7 6V30" fill={GOLD} stroke={INK} strokeWidth="2.5" strokeLinejoin="round" />
    </svg>
  );
}

const ART: Record<ServiceId, (props: Props) => React.JSX.Element> = {
  eid: EidArt,
  admin: AdminArt,
  dwh: DwhArt,
  backups: BackupsArt,
  monitor: MonitorArt,
  docs: DocsArt,
};

export default function ServiceArt({id, className}: {id: ServiceId; className?: string}) {
  const Art = ART[id];
  return <Art className={className} />;
}
