"use client";
import {useEffect,useState} from "react";
import {useRouter} from "next/navigation";
import Link from "next/link";
import EIDLogin from "@/components/EIDLogin";
import LanguageSwitcher from "@/components/LanguageSwitcher";
import {api} from "@/lib/api";
import {resetAccess} from "@/lib/access";
import {useI18n} from "@/lib/i18n";
import {ChevronDown,Lock,Mail} from "lucide-react";

export default function LoginPage(){const router=useRouter();const {t}=useI18n();const [next,setNext]=useState("/apps"),[admin,setAdmin]=useState(false),[email,setEmail]=useState("admin@example.com"),[password,setPassword]=useState("Password123!"),[error,setError]=useState("");
  useEffect(()=>{const requested=new URLSearchParams(location.search).get("next");if(requested?.startsWith("/")&&!requested.startsWith("//"))setNext(requested)},[]);
  // resetAccess before the push: router.push is a client-side navigation, so
  // whatever the previous session left cached would answer for this one.
  async function passwordLogin(e:React.FormEvent){e.preventDefault();setError("");try{await api.login(email,password);resetAccess();
    // /oauth2/* belongs to the API, not to this Next app, so a client-side
    // push would 404 instead of resuming the authorization request. eID sign-in
    // already navigates for real; this path has to as well.
    if(next.startsWith("/oauth2/"))window.location.assign(next);else router.push(next)}catch(err:any){setError(err.message||t("auth.message.error_password"))}}
  return <main className="eid-page"><div className="eid-page__pattern"/><header><Link href="/" className="gp-brand"><img src="/brand.webp" alt=""/><span>Gerege Nexus</span></Link><LanguageSwitcher variant="dark"/></header><section className="eid-page__content"><div className="eid-page__intro"><span className="gp-eyebrow"><i/> {t("auth.view.eyebrow")}</span><h1>{t("auth.view.title_lead")}<br/><em>{t("auth.view.title_highlight")}</em> {t("auth.view.title_tail")}</h1><p>{t("auth.view.lede")}</p><ul><li>{t("auth.view.point_push")}</li><li>{t("auth.view.point_qr")}</li><li>{t("auth.view.point_rbac")}</li></ul></div><div><EIDLogin next={next}/><button className="admin-disclosure" onClick={()=>setAdmin(v=>!v)}><Lock/> {t("auth.action.admin_disclosure")} <ChevronDown className={admin?"rotate-180":""}/></button>{admin&&<form className="admin-login" onSubmit={passwordLogin}>{error&&<p>{error}</p>}<label><Mail/> <input type="email" value={email} onChange={e=>setEmail(e.target.value)} required/></label><label><Lock/> <input type="password" value={password} onChange={e=>setPassword(e.target.value)} required/></label><button>{t("auth.action.admin_sign_in")}</button></form>}</div></section></main>}
