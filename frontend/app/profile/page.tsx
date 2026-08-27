"use client";
import {useEffect,useState} from "react";
import {api,apiBase} from "@/lib/api";
import {useI18n} from "@/lib/i18n";
import {Building2,House,KeyRound,MonitorSmartphone,ShieldCheck,Unlink} from "lucide-react";
import {ProviderMark,GoogleMark} from "@/components/ProviderMark";
import EIDLogin from "@/components/EIDLogin";

/**
 * Хүний өөрийнх нь тухай бичлэг.
 *
 * Платформын дэлгэц, суулгадаг апп биш. Апп нь байгууллага тус бүрд суудаг
 * бөгөөд админ нь устгаж чадна — хүн ямар таних тэмдгээр нэвтэрдгээ харах
 * эрхийг ажил олгогч нь авч болдог байх нь буруу. Мөн олон байгууллагад
 * харьяалагдах хүнд нэг л профайл байна, гишүүнчлэл тутамд нэг биш.
 */

type Identity = {
  kind: string; provider: string; subject: string;
  email?: string; name?: string; surname?: string;
  linked_at: string; last_seen_at: string;
  claims?: Record<string, unknown>;
  issuer?: string;
  removable?: boolean;
};
type Profile = {
  id: string; name: string; email: string; created_at: string; is_admin: boolean;
  organisations: Array<{id:string;name:string;slug:string}>;
  home?: {id:string;name:string;slug:string}|null;
  identities: Identity[]; active_sessions: number;
};

function when(iso: string) {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "—" : d.toLocaleDateString();
}

/**
 * Провайдер энэ хүнийг баталгаажуулсан эсэх.
 *
 * eID нь тодорхойлолтоороо баталгаажсан — иргэний цахим үнэмлэхээр нэвтэрсэн
 * хүн бол тэр хүн. Google-ийн хувьд `email_verified` л утга агуулна: тэр
 * талбаргүй Google хаяг нь зөвхөн хэн нэгэн тэр хаягийг бичсэн гэсэн үг.
 */
function verified(id: Identity) {
  if (id.kind === "eid") return true;
  return id.claims?.email_verified === true;
}

/**
 * Провайдерын өгсөн зураг. Байхгүй бол нэрний эхний үсэг.
 *
 * Зургийг provider-ийн CDN-ээс шууд ачаална — өөр дээрээ хуулбарлавал хүний
 * царайг тэдний устгасны дараа ч хадгалсан хэвээр үлдэнэ.
 */
function Avatar({ identity, person }: { identity: Identity; person: string }) {
  const src = typeof identity.claims?.picture === "string" ? identity.claims.picture : "";
  if (src) return <img className="profile__photo" src={src} alt="" referrerPolicy="no-referrer"/>;
  return <span className="profile__photo profile__photo--letter">{person.trim().charAt(0).toUpperCase()}</span>;
}

/**
 * Холболтын алдааны кодыг хүний хэл рүү. Танихгүй кодыг өөрийг нь үзүүлнэ —
 * ойлгомжгүй ч гэсэн үнэн, "ямар нэг зүйл буруу боллоо" гэхээс тусалдаг.
 */
function linkErrorText(t:(k:any,v?:any)=>string,code:string){
  const known=["session_expired","already_linked_elsewhere","google_not_configured","sso_required","provider_unreachable","email_unverified","domain_not_allowed"];
  return known.includes(code)?t(("profile.link_error."+code) as any):t("profile.link_error.unknown",{code});
}

export default function ProfilePage(){const {t}=useI18n();
  const [profile,setProfile]=useState<Profile|null>(null);
  const [error,setError]=useState("");
  const [open,setOpen]=useState<string>("");
  const [busy,setBusy]=useState<string>("");

  const [canLinkGoogle,setCanLinkGoogle]=useState(false);
  // Google-ээс буцаж ирэхэд асуудал гарсан бол шалтгаан нь URL-д ирнэ. Хүн
  // товч дараад юу ч болоогүй мэт байхаас, юу болсныг хэлэх нь дээр.
  const [linkError,setLinkError]=useState("");
  useEffect(()=>{
    const code=new URLSearchParams(location.search).get("link_error");
    if(!code)return;
    setLinkError(code);
    // Хаягаас нь арчина: сэргээхэд дахин гарч ирэх ёсгүй, аль хэдийн уншсан.
    history.replaceState(null,"",location.pathname);
  },[]);

  // round-ыг нэмэхэд дахин уншина: eID холбогдмогц жагсаалт, Гэрэгэ дугаар,
  // «холбогдсон таних тэмдэг алга» гэсэн мөр гурвуулаа хуучирдаг.
  const [round,setRound]=useState(0);
  useEffect(()=>{void api.profile().then(setProfile).catch((e:any)=>setError(e?.message||"—"))},[round]);
  // Серверээс асууна, таамаглахгүй: Google-ээр нэвтрэх тохируулаагүй
  // deployment дээр холбох товч гарч ирээд дарахад л бүтэлгүйтэх нь дор.
  useEffect(()=>{void api.ssoConfig().then(c=>setCanLinkGoogle(!!c.google?.enabled)).catch(()=>{})},[]);

  /**
   * Салгах. Асууж байж — буцаах товч байхгүй үйлдэл тул нэг товшилтоор
   * болохгүй. Сервер шинэ жагсаалтыг буцаадаг учир юу үлдсэнийг таамаглахгүй,
   * зүгээр л түүнийг хэрэглэнэ: сүүлчийнх нь болсон таних тэмдгийн салгах
   * товч ингэснээр өөрөө алга болно.
   */
  async function unlink(id:Identity,key:string){
    if(!window.confirm(t("profile.unlink_confirm",{provider:id.provider})))return;
    setBusy(key);
    try{
      const res=await api.unlinkIdentity({kind:id.kind,issuer:id.issuer,subject:id.subject});
      setProfile(p=>p?{...p,identities:res.identities}:p);
    }catch(e:any){setError(e?.message||"—")}
    finally{setBusy("")}
  }

  // Google аль хэдийн холбогдсон эсэх — issuer-ээр, провайдерын нэрээр биш:
  // нэр нь дэлгэцийн хэл, issuer нь баримт.
  const hasGoogle=profile?.identities.some(i=>i.issuer?.includes("accounts.google.com"))??false;
  // eID-ийг kind-ээр нь шалгана, issuer-ээр биш: тэр нь провайдерын данс биш,
  // үндэсний таних тэмдэг бөгөөд өөрийн хүснэгттэй (registry.user_eid_identities).
  const hasEID=profile?.identities.some(i=>i.kind==="eid")??false;
  const [linking,setLinking]=useState(false);

  if(error)return <main className="profile"><p className="profile__error">{error}</p></main>;
  if(!profile)return <main className="profile"><p className="profile__muted">{t("profile.loading")}</p></main>;

  return <main className="profile">
    <header className="profile__head">
      <div className="profile__avatar">{(profile.name||profile.email||"?").trim().charAt(0).toUpperCase()}</div>
      <div>
        <h1>{profile.name||profile.email}</h1>
        <p>{profile.email}</p>
      </div>
    </header>

    {/* Тойм: тоо биш, хариулт. Хэдэн байгууллагад, хэдэн аргаар нэвтэрдэг,
        хаана нээлттэй байна. */}
    <section className="profile__stats">
      <div><Building2/><b>{profile.organisations.length}</b><span>{t("profile.stat.organisations")}</span></div>
      <div><KeyRound/><b>{profile.identities.length}</b><span>{t("profile.stat.identities")}</span></div>
      <div><MonitorSmartphone/><b>{profile.active_sessions}</b><span>{t("profile.stat.sessions")}</span></div>
      <div><ShieldCheck/><b>{when(profile.created_at)}</b><span>{t("profile.stat.since")}</span></div>
    </section>

    <section className="profile__section">
      <h2>{t("profile.identities")}</h2>
      <p className="profile__muted">{t("profile.identities_lede")}</p>
      <ul className="profile__list">
        {profile.identities.map(id=>{
          const key=id.kind+id.subject;
          const claims=Object.entries(id.claims||{});
          const person=[id.surname,id.name].filter(Boolean).join(" ")||id.name||id.email||id.subject;
          return <li key={key} className="profile__id">
            {/* Толгой мөр нь провайдерыг нэрлэнэ, доод мөр нь тэнд байгаа
                хүнийг. Хоёр өөр зүйл — аль Google гэдэг нь нэг асуулт,
                тэр Google дотор хэн байгаа нь өөр асуулт. */}
            <div className="profile__id-head">
              <span className="profile__mark"><ProviderMark kind={id.kind} issuer={id.issuer}/></span>
              <div className="profile__grow">
                <b>{t("profile.linked_provider",{provider:id.provider})}</b>
                <span>{id.issuer||id.provider}</span>
              </div>
              {id.removable&&<button type="button" className="profile__unlink"
                disabled={busy===key}
                onClick={()=>void unlink(id,key)}>
                <Unlink/> {busy===key?t("profile.unlinking"):t("profile.unlink")}
              </button>}
            </div>

            <div className="profile__id-person">
              <Avatar identity={id} person={person}/>
              <div className="profile__grow">
                <b>{person}{verified(id)&&<em className="profile__badge">{t("profile.verified")}</em>}</b>
                <span>
                  {id.email&&<code>{id.email}</code>}
                  {id.email&&" · "}
                  {t("profile.linked_at")} {when(id.linked_at)}
                </span>
              </div>
              <span className="profile__meta">{t("profile.last_seen")} {when(id.last_seen_at)}</span>
            </div>

            {claims.length>0&&<>
              <button className="profile__toggle" onClick={()=>setOpen(open===key?"":key)}>
                {open===key?t("profile.hide_claims"):t("profile.show_claims",{count:String(claims.length)})}
              </button>
              {open===key&&<dl className="profile__claims">
                {claims.map(([k,v])=><div key={k}><dt>{k}</dt><dd>{typeof v==="object"?JSON.stringify(v):String(v)}</dd></div>)}
              </dl>}
            </>}
          </li>;
        })}
        {profile.identities.length===0&&<li className="profile__muted">{t("profile.no_identities")}</li>}
      </ul>

      {/* Холбох нь навигаци, fetch биш — Google дээр очиж, зөвшөөрөл асууж,
          буцаж ирдэг. Аль хэдийн холбогдсон бол харагдахгүй: энэ товч нэг л
          зүйл хийдэг бөгөөд түүнийг хийчихсэн байна. */}
      {linkError&&<p className="profile__error profile__link-error">{linkErrorText(t,linkError)}</p>}
      {canLinkGoogle&&!hasGoogle&&<a className="profile__link-provider" href={`${apiBase()}/auth/google/link`}>
        <GoogleMark/> {t("profile.link_google")}
      </a>}
      {canLinkGoogle&&!hasGoogle&&<p className="profile__muted profile__link-note">{t("profile.link_google_note")}</p>}

      {/* eID нь Google-ээс өөр байрлалтай.
          Google бол нэвтрэх нэмэлт зам; eID бол хүн хэн болохын **нотолгоо**,
          дагалдан Гэрэгэ дугаар авчирдаг. Тэр дугаар нь нийлүүлэгчийн модуль
          хүнийг нэрлэх цорын ганц үг (pkg/nexus.PersonFeed, 00086) — нууц
          үгээр нээсэн дансанд огт байхгүй. Тиймээс энэ товч нь чимэглэл биш:
          үүнгүйгээр иргэн хүсэлт гаргаад хариуг нь хүлээж авах аргагүй. */}
      {!hasEID&&<div className="profile__eid-link">
        {!linking
          ? <button className="profile__link-provider" onClick={()=>setLinking(true)}>
              <ShieldCheck/> {t("profile.link_eid")}
            </button>
          : <EIDLogin link variant="signin" onLinked={()=>{setLinking(false);setRound(n=>n+1)}}/>}
        <p className="profile__muted profile__link-note">{t("profile.link_eid_note")}</p>
      </div>}
    </section>

    <section className="profile__section">
      <h2>{t("profile.organisations")}</h2>
      <ul className="profile__list">
        {/* Гэр эхэнд, тусдаа тэмдэгтэйгээ. Slug нь хэрэглэгчийн id-аас гардаг
            тул уншигчид юу ч хэлэхгүй — доод мөрөнд юу болохыг нь бичнэ. */}
        {profile.home&&<li key={profile.home.id}><div className="profile__row">
          <span className="profile__icon"><House/></span>
          <div className="profile__grow"><b>{profile.home.name}</b><span>{t("web.label.my_home")}</span></div>
        </div></li>}
        {profile.organisations.map(o=><li key={o.id}><div className="profile__row">
          <span className="profile__icon"><Building2/></span>
          <div className="profile__grow"><b>{o.name}</b><span>{o.slug}</span></div>
        </div></li>)}
        {profile.organisations.length===0&&<li className="profile__muted">{t("profile.message.no_organisations")}</li>}
      </ul>
    </section>
  </main>
}
