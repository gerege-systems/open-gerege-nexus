"use client";
import {useEffect,useState} from "react";
import {api,apiBase} from "@/lib/api";
import {useI18n} from "@/lib/i18n";
import {Building2,House,KeyRound,MonitorSmartphone,ShieldCheck,Unlink} from "lucide-react";
import {Alert,Badge,Button,Card,Spinner} from "@gerege-systems/ui";
import {ProviderMark,GoogleMark} from "@/components/ProviderMark";
import EIDLogin from "@/components/EIDLogin";
import { formatDay } from "@/lib/datetime";

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
  return Number.isNaN(d.getTime()) ? "—" : formatDay(d);
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
  if (src) return <img className="size-10 shrink-0 rounded-full object-cover" src={src} alt="" referrerPolicy="no-referrer"/>;
  return <span className="grid size-10 shrink-0 place-items-center rounded-full bg-surface-2 text-sm font-semibold text-muted">{person.trim().charAt(0).toUpperCase()}</span>;
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

  const page="mx-auto w-full max-w-3xl space-y-6";
  if(error)return <main className={page}><Alert variant="danger" live>{error}</Alert></main>;
  if(!profile)return <main className={page}><p className="flex items-center gap-2 text-sm text-muted" role="status"><Spinner size="sm" decorative/>{t("profile.loading")}</p></main>;

  return <main className={page}>
    <header className="flex items-center gap-4">
      <div className="grid size-14 shrink-0 place-items-center rounded-full bg-accent-soft text-xl font-semibold text-accent" aria-hidden="true">{(profile.name||profile.email||"?").trim().charAt(0).toUpperCase()}</div>
      <div className="min-w-0">
        <h1 className="truncate text-2xl font-semibold text-foreground">{profile.name||profile.email}</h1>
        <p className="truncate text-sm text-muted">{profile.email}</p>
      </div>
    </header>

    {/* Тойм: тоо биш, хариулт. Хэдэн байгууллагад, хэдэн аргаар нэвтэрдэг,
        хаана нээлттэй байна. */}
    <section className="grid grid-cols-2 gap-3 sm:grid-cols-4">
      <Stat icon={<Building2/>} value={profile.organisations.length} label={t("profile.stat.organisations")}/>
      <Stat icon={<KeyRound/>} value={profile.identities.length} label={t("profile.stat.identities")}/>
      <Stat icon={<MonitorSmartphone/>} value={profile.active_sessions} label={t("profile.stat.sessions")}/>
      <Stat icon={<ShieldCheck/>} value={when(profile.created_at)} label={t("profile.stat.since")}/>
    </section>

    <Card asChild padding="lg" className="space-y-4">
      <section>
      <div>
        <h2 className="text-lg font-semibold text-foreground">{t("profile.identities")}</h2>
        <p className="text-sm text-muted">{t("profile.identities_lede")}</p>
      </div>
      <ul className="m-0 list-none space-y-3 p-0">
        {profile.identities.map(id=>{
          const key=id.kind+id.subject;
          const claims=Object.entries(id.claims||{});
          const person=[id.surname,id.name].filter(Boolean).join(" ")||id.name||id.email||id.subject;
          return <li key={key} className="space-y-3 rounded-md border border-line p-4">
            {/* Толгой мөр нь провайдерыг нэрлэнэ, доод мөр нь тэнд байгаа
                хүнийг. Хоёр өөр зүйл — аль Google гэдэг нь нэг асуулт,
                тэр Google дотор хэн байгаа нь өөр асуулт. */}
            <div className="flex items-center gap-3">
              <span className="grid size-9 shrink-0 place-items-center rounded-md border border-line bg-surface-2"><ProviderMark kind={id.kind} issuer={id.issuer}/></span>
              <div className="min-w-0 flex-1">
                <b className="block text-sm font-semibold text-foreground">{t("profile.linked_provider",{provider:id.provider})}</b>
                <span className="block truncate text-xs text-muted">{id.issuer||id.provider}</span>
              </div>
              {id.removable&&<Button type="button" size="sm" variant="outline"
                className="border-danger-border text-danger hover:bg-danger-soft"
                loading={busy===key}
                leadingIcon={<Unlink/>}
                onClick={()=>void unlink(id,key)}>
                {busy===key?t("profile.unlinking"):t("profile.unlink")}
              </Button>}
            </div>

            <div className="flex flex-wrap items-center gap-3">
              <Avatar identity={id} person={person}/>
              <div className="min-w-0 flex-1">
                <b className="flex flex-wrap items-center gap-2 text-sm font-semibold text-foreground">{person}{verified(id)&&<Badge variant="subtle" tone="success">{t("profile.verified")}</Badge>}</b>
                <span className="block text-xs text-muted">
                  {id.email&&<code>{id.email}</code>}
                  {id.email&&" · "}
                  {t("profile.linked_at")} {when(id.linked_at)}
                </span>
              </div>
              <span className="text-xs text-muted">{t("profile.last_seen")} {when(id.last_seen_at)}</span>
            </div>

            {claims.length>0&&<>
              <Button type="button" variant="link" size="sm" className="px-0" aria-expanded={open===key} onClick={()=>setOpen(open===key?"":key)}>
                {open===key?t("profile.hide_claims"):t("profile.show_claims",{count:String(claims.length)})}
              </Button>
              {open===key&&<dl className="m-0 grid gap-1 rounded-md bg-surface-2 p-3 text-xs">
                {claims.map(([k,v])=><div key={k} className="grid grid-cols-[minmax(0,10rem)_minmax(0,1fr)] gap-3"><dt className="truncate font-mono text-muted">{k}</dt><dd className="m-0 break-all text-foreground">{typeof v==="object"?JSON.stringify(v):String(v)}</dd></div>)}
              </dl>}
            </>}
          </li>;
        })}
        {profile.identities.length===0&&<li className="text-sm text-muted">{t("profile.no_identities")}</li>}
      </ul>

      {/* Холбох нь навигаци, fetch биш — Google дээр очиж, зөвшөөрөл асууж,
          буцаж ирдэг. Аль хэдийн холбогдсон бол харагдахгүй: энэ товч нэг л
          зүйл хийдэг бөгөөд түүнийг хийчихсэн байна. */}
      {linkError&&<Alert variant="danger">{linkErrorText(t,linkError)}</Alert>}
      {canLinkGoogle&&!hasGoogle&&<Button asChild variant="outline">
        <a href={`${apiBase()}/auth/google/link`}><GoogleMark/> {t("profile.link_google")}</a>
      </Button>}
      {canLinkGoogle&&!hasGoogle&&<p className="text-xs text-muted">{t("profile.link_google_note")}</p>}

      {/* eID нь Google-ээс өөр байрлалтай.
          Google бол нэвтрэх нэмэлт зам; eID бол хүн хэн болохын **нотолгоо**,
          дагалдан Гэрэгэ дугаар авчирдаг. Тэр дугаар нь нийлүүлэгчийн модуль
          хүнийг нэрлэх цорын ганц үг (pkg/nexus.PersonFeed, 00086) — нууц
          үгээр нээсэн дансанд огт байхгүй. Тиймээс энэ товч нь чимэглэл биш:
          үүнгүйгээр иргэн хүсэлт гаргаад хариуг нь хүлээж авах аргагүй. */}
      {!hasEID&&<div className="space-y-2">
        {!linking
          ? <Button type="button" variant="outline" leadingIcon={<ShieldCheck/>} onClick={()=>setLinking(true)}>
              {t("profile.link_eid")}
            </Button>
          : <EIDLogin link variant="signin" onLinked={()=>{setLinking(false);setRound(n=>n+1)}}/>}
        <p className="text-xs text-muted">{t("profile.link_eid_note")}</p>
      </div>}
      </section>
    </Card>

    <Card asChild padding="lg" className="space-y-4">
      <section>
      <h2 className="text-lg font-semibold text-foreground">{t("profile.organisations")}</h2>
      <ul className="m-0 list-none divide-y divide-line p-0">
        {/* Гэр эхэнд, тусдаа тэмдэгтэйгээ. Slug нь хэрэглэгчийн id-аас гардаг
            тул уншигчид юу ч хэлэхгүй — доод мөрөнд юу болохыг нь бичнэ. */}
        {profile.home&&<li key={profile.home.id}><OrgRow icon={<House/>} name={profile.home.name} note={t("web.label.my_home")}/></li>}
        {profile.organisations.map(o=><li key={o.id}><OrgRow icon={<Building2/>} name={o.name} note={o.slug}/></li>)}
        {profile.organisations.length===0&&<li className="py-3 text-sm text-muted">{t("profile.message.no_organisations")}</li>}
      </ul>
      </section>
    </Card>
  </main>
}

function Stat({icon,value,label}:{icon:React.ReactNode;value:React.ReactNode;label:string}){
  return <Card padding="none" className="flex flex-col gap-1 px-4 py-3">
    <span className="text-muted [&>svg]:size-4" aria-hidden="true">{icon}</span>
    <b className="text-lg font-semibold tabular-nums text-foreground">{value}</b>
    <span className="text-xs text-muted">{label}</span>
  </Card>
}

function OrgRow({icon,name,note}:{icon:React.ReactNode;name:string;note:string}){
  return <div className="flex items-center gap-3 py-3">
    <span className="grid size-9 shrink-0 place-items-center rounded-md bg-surface-2 text-muted [&>svg]:size-4" aria-hidden="true">{icon}</span>
    <div className="min-w-0"><b className="block truncate text-sm font-semibold text-foreground">{name}</b><span className="block truncate text-xs text-muted">{note}</span></div>
  </div>
}
