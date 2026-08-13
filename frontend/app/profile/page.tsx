"use client";
import {useEffect,useState} from "react";
import {api} from "@/lib/api";
import {useI18n} from "@/lib/i18n";
import {Building2,KeyRound,MonitorSmartphone,ShieldCheck,Unlink} from "lucide-react";
import {ProviderMark} from "@/components/ProviderMark";

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

export default function ProfilePage(){const {t}=useI18n();
  const [profile,setProfile]=useState<Profile|null>(null);
  const [error,setError]=useState("");
  const [open,setOpen]=useState<string>("");
  const [busy,setBusy]=useState<string>("");

  useEffect(()=>{void api.profile().then(setProfile).catch((e:any)=>setError(e?.message||"—"))},[]);

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
    </section>

    <section className="profile__section">
      <h2>{t("profile.organisations")}</h2>
      <ul className="profile__list">
        {profile.organisations.map(o=><li key={o.id}><div className="profile__row">
          <span className="profile__icon"><Building2/></span>
          <div className="profile__grow"><b>{o.name}</b><span>{o.slug}</span></div>
        </div></li>)}
      </ul>
    </section>
  </main>
}
