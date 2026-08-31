"use client";

import {useCallback,useEffect,useRef,useState} from "react";
import {QRCodeSVG} from "qrcode.react";
import {api} from "@/lib/api";
import {useI18n} from "@/lib/i18n";
import {Fingerprint,RefreshCw,ShieldCheck,Smartphone,X} from "lucide-react";
import {safeReturnPath} from "@/lib/safeReturnPath.mjs";

type Method="id"|"qr";
type Phase="idle"|"starting"|"waiting"|"expired"|"refused"|"error"|"success";
// expires_at is absent for push sessions: eID reports no deadline for them, and
// the API no longer invents one.
type Start={session_id:string;device_link_url?:string;verification_code:string;expires_at?:string};

// The API holds every /auth/eid/poll open for up to 25s and answers the moment
// the citizen approves, so this gap is the only stretch where an approval is
// not being watched. Keep it short: at 25s per check it costs almost nothing in
// requests, and it is pure delay between approving and being signed in.
const GAP=400;
// A dropped long-poll is ordinary on a mobile network, so a session survives
// a few in a row before the citizen is told the sign-in failed.
const TOLERATED_FAILURES=3;
// Backstop, not a deadline. eID decides when a session dies and says so with
// EXPIRED; this only stops the card polling forever if that answer never comes,
// so it sits well beyond any real session — one was still RUNNING after nine
// minutes. A tighter figure here abandons approvals that were still in progress.
const BACKSTOP=15*60_000;

function mobile(){return typeof navigator!=="undefined"&&/Android|iPhone|iPad|iPod/i.test(navigator.userAgent)}
// Аль browser-ээс эхэлснийг eID апп руу дамжуулна — approve хийсний дараа ЯГ ТЭР browser руу
// буцна. Үгүй бол апп системийн default browser-ыг нээж (iOS дээр ихэвчлэн Safari), иргэн
// нэвтэрч эхэлсэн цонхоо алддаг. iOS: browser-ийн scheme/token; Android: package. Танихгүй
// browser (ж Brave — UA нь ялгарахгүй) → hint байхгүй → default. Зөвшөөрөх жагсаалт апп талд.
const IOS_BROWSERS:Array<[RegExp,string]>=[[/CriOS/i,"googlechromes"],[/EdgiOS/i,"microsoft-edge-https"],[/FxiOS/i,"firefox"],[/Focus\//i,"firefox-focus"],[/OPT\//i,"touch-https"],[/OPiOS|OPR\//i,"opera-https"],[/DuckDuckGo/i,"ddgQuickLink"],[/YaBrowser|YaSearchBrowser/i,"yandexbrowser-open-url"]]
// "Chrome/" хамгийн сүүлд: Chromium суурьтай бүх browser (Edge, Opera, Samsung, Yandex…) UA-даа
// түүнийг агуулдаг тул эхэнд шалгавал бүгд Chrome болж таарна.
const ANDROID_BROWSERS:Array<[RegExp,string]>=[[/EdgA\//i,"com.microsoft.emmx"],[/OPR\//i,"com.opera.browser"],[/Opera Mini/i,"com.opera.mini.native"],[/SamsungBrowser\//i,"com.sec.android.app.sbrowser"],[/YaBrowser\//i,"com.yandex.browser"],[/DuckDuckGo\//i,"com.duckduckgo.mobile.android"],[/Vivaldi\//i,"com.vivaldi.browser"],[/Whale\//i,"com.naver.whale"],[/UCBrowser\//i,"com.UCMobile.intl"],[/Focus\//i,"org.mozilla.focus"],[/Firefox\//i,"org.mozilla.firefox"],[/Chrome\//i,"com.android.chrome"]]
export function browserHint(ua:string){const table=/iPhone|iPad|iPod/i.test(ua)?IOS_BROWSERS:/Android/i.test(ua)?ANDROID_BROWSERS:[];const hit=table.find(([re])=>re.test(ua));return hit?`?${table===IOS_BROWSERS?"retScheme":"retPkg"}=${encodeURIComponent(hit[1])}`:""}
function callbackURL(){return typeof window==="undefined"?"":`${window.location.origin}/auth/eid/callback${browserHint(navigator.userAgent)}`}
function deadlineOf(start:Start){const at=Date.parse(start.expires_at??"");return Number.isNaN(at)?Date.now()+BACKSTOP:at}
function hasDeadline(start:Start){return !Number.isNaN(Date.parse(start.expires_at??""))}
function sleep(ms:number){return new Promise(resolve=>setTimeout(resolve,ms))}
function clock(seconds:number){return `${Math.floor(seconds/60)}:${String(seconds%60).padStart(2,"0")}`}

/**
 * `variant="signin"` нь картын чимэглэлээ орхино — толгой ба хөл нь эргэн тойрны
 * signin-card дээр аль хэдийн байгаа тул давхарлах нь утгагүй. Логик нь адилхан:
 * зөвхөн юу зурагдах нь өөр.
 */
/**
 * `link` нь гурав дахь горим.
 *
 * Нэвтрэх (`binding` хоосон), парклагдсан таних тэмдгийг батлах (`binding`),
 * ба **аль хэдийн нэвтэрсэн данс дээрээ eID нэмэх** (`link`). Гуравт нь
 * эхлүүлэх алхам нь ижил — eID-ийн session эхлүүлэх нь юу ч нотлохгүй —
 * зөвхөн дуусгах нь өөр: хариу нь session-ий эзэнд бичигдэнэ, шинэ session
 * нээхгүй. Тиймээс энд ялгаа нь ганц мөр, нэг бүрэн component биш.
 */
export default function EIDLogin({next="/profile",compact=false,variant="card",binding,link=false,onLinked}:{next?:string;compact?:boolean;variant?:"card"|"signin";binding?:string;link?:boolean;onLinked?:()=>void}){
  const {t}=useI18n();
  const returnTo=safeReturnPath(next);
  const [method,setMethod]=useState<Method>("id"),[phase,setPhase]=useState<Phase>("idle"),[nationalId,setNationalId]=useState(""),[start,setStart]=useState<Start|null>(null),[error,setError]=useState(""),[left,setLeft]=useState(0);
  // Each attempt takes a ticket. Anything asynchronous compares its ticket
  // before touching state, so a cancelled or superseded attempt cannot revive
  // the card once the citizen has moved on.
  const ticket=useRef(0),inflight=useRef<AbortController|null>(null);
  const stop=useCallback(()=>{ticket.current++;inflight.current?.abort();inflight.current=null},[]);

  /* One check at a time. Polling on an interval instead stacked long-polls —
     ten of them within the first 25s — until the browser ran out of
     connections to the host and the page stopped responding altogether. */
  const watch=useCallback(async(data:Start)=>{
    stop();
    const mine=ticket.current,deadline=deadlineOf(data);
    let failures=0;
    setStart(data);setPhase("waiting");
    while(ticket.current===mine){
      if(Date.now()>=deadline){setPhase("expired");return}
      const control=new AbortController();
      inflight.current=control;
      try{
        const res=link?await api.linkEID(data.session_id,control.signal):binding?await api.bindingEIDPoll(binding,data.session_id,control.signal):await api.pollEID(data.session_id,control.signal);
        if(ticket.current!==mine)return;
        failures=0;
        // Холбох горимд хуудас солигдохгүй: хүн профайл дээрээ байсаар,
        // жагсаалт нь шинэчлэгдэнэ. Нэвтрэх горимд эсрэгээрээ — session
        // үүссэн тул тэр хүн хаа нэгтээ очих ёстой.
        if(res.state==="COMPLETE"){ticket.current++;setPhase("success");if(link)onLinked?.();else window.location.assign(returnTo);return}
        if(res.state==="EXPIRED"){setPhase("expired");return}
        if(res.state==="REFUSED"){setPhase("refused");return}
      }catch(e:any){
        if(ticket.current!==mine)return;
        if(++failures>TOLERATED_FAILURES){setError(e?.message||t("auth.message.error_link"));setPhase("error");return}
      }
      await sleep(GAP);
    }
  },[binding,link,onLinked,returnTo,stop,t]);

  const begin=useCallback(async(selected:Method)=>{
    stop();
    const mine=ticket.current;
    setError("");setStart(null);setPhase("starting");
    try{
      const cb=mobile()?callbackURL():"";
      const data=binding
        ?await api.bindingEIDStart(binding,selected==="qr"?undefined:nationalId.trim().toUpperCase())
        :(selected==="qr"?await api.startEID(cb):await api.startEIDByNationalID(nationalId.trim().toUpperCase(),cb));
      if(ticket.current!==mine)return;
      if(selected==="qr"&&mobile())window.location.href=`geregesmartid://approve?sessionId=${encodeURIComponent(data.session_id)}`;
      void watch(data);
    }catch(e:any){
      if(ticket.current!==mine)return;
      setError(e?.message||t("auth.message.error_service"));setPhase("error");
    }
  },[binding,nationalId,stop,t,watch]);

  /* Runs down the backstop, and shows it only when eID actually gave a deadline
     to run down. A countdown the API made up is worse than none: it hurried the
     citizen and then cut the session off while eID was still waiting on them. */
  useEffect(()=>{
    if(phase!=="waiting"||!start)return;
    const deadline=deadlineOf(start);
    const tick=()=>{const secs=Math.max(0,Math.ceil((deadline-Date.now())/1000));setLeft(secs);if(secs===0){stop();setPhase("expired")}};
    tick();
    const id=setInterval(tick,1000);
    return ()=>clearInterval(id);
  },[phase,start,stop]);

  useEffect(()=>()=>{stop()},[stop]);
  const cancel=()=>{stop();setPhase("idle");setStart(null);setError("")};
  const switchMethod=(value:Method)=>{stop();setMethod(value);setPhase("idle");setStart(null);setError("");if(value==="qr")void begin("qr")};
  const pending=phase==="starting"||phase==="waiting";
  const terminal=phase==="error"||phase==="expired"||phase==="refused";
  const bare=variant==="signin";
  return <div className={`eid-login ${compact?"eid-login--compact":""}`} aria-live="polite">
    {!bare&&<header className="eid-login__head"><span><Fingerprint/></span><div><h2>{t("auth.eid.title")}</h2><p>{t("auth.eid.subtitle")}</p></div></header>}
    <div className="eid-tabs" role="tablist"><button className={method==="id"?"is-active":""} onClick={()=>switchMethod("id")}>{t("auth.eid.tab_id")}</button><button className={method==="qr"?"is-active":""} onClick={()=>switchMethod("qr")}>{t("auth.eid.tab_qr")}</button></div>
    {bare&&method==="id"&&!pending&&<p className="signin-instruction">{t("auth.eid.instruction")}</p>}
    {error&&<p className="eid-alert eid-alert--error">{error}</p>}{phase==="expired"&&<p className="eid-alert">{t("auth.message.expired")}</p>}{phase==="refused"&&<p className="eid-alert eid-alert--error">{t("auth.message.refused")}</p>}
    {method==="id"&&!pending&&<form onSubmit={e=>{e.preventDefault();void begin("id")}}><label htmlFor="eid-rd">{t("auth.eid.reg_number")}</label><input id="eid-rd" value={nationalId} onChange={e=>setNationalId(e.target.value.toUpperCase())} placeholder={t("auth.eid.reg_number_placeholder")} autoComplete="off" required minLength={8}/><button className="eid-primary"><Smartphone/> {t("auth.eid.send_request")}</button></form>}
    {phase==="starting"&&<div className="eid-status"><RefreshCw className="animate-spin"/> {t("auth.message.starting")}</div>}
    {phase==="waiting"&&start&&<div className="eid-wait">
      {method==="qr"&&start.device_link_url&&<div className="eid-qr"><QRCodeSVG value={start.device_link_url} size={compact?154:190} level="M"/></div>}
      <p>{t(method==="qr"?"auth.message.scan_qr":"auth.message.sent_push")}</p><small>{t("auth.eid.verification_code")}</small><strong>{start.verification_code}</strong><span><ShieldCheck/> {t("auth.eid.confirm_hint")}</span>
      {hasDeadline(start)&&<span className="eid-countdown">{t("auth.message.expires_in",{time:clock(left)})}</span>}
    </div>}
    {pending&&<button className="eid-cancel" onClick={cancel}><X/> {t("auth.action.cancel")}</button>}
    {phase==="success"&&<p className="eid-alert eid-alert--success">{t("auth.message.success")}</p>}
    {terminal&&<button className="eid-retry" onClick={()=>void begin(method)}><RefreshCw/> {t("auth.action.retry")}</button>}
    {!bare&&<footer><ShieldCheck/> {t("auth.eid.footer")}</footer>}
  </div>
}
