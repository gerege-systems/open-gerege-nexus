"use client";
import {useEffect,useState} from "react";
import Link from "next/link";
import EIDLogin from "@/components/EIDLogin";
import LanguageSwitcher from "@/components/LanguageSwitcher";
import {api} from "@/lib/api";
import {useI18n} from "@/lib/i18n";
import {useBrand} from "@/lib/brandContext";
import {ShieldCheck} from "lucide-react";
import {Alert,Button,Separator} from "@gerege-systems/ui";
import {safeReturnPath} from "@/lib/safeReturnPath.mjs";

/**
 * Гадны провайдераар анх удаа ирсэн хүнийг eID-ээр баталгаажуулах дэлгэц.
 *
 * Хоёр алхам, энэ дараалалтай нь санаатай. Эхлээд юу хаанаас хаашаа дамжихыг
 * үзүүлж зөвшөөрөл авна — утсанд push мэдэгдэл ирсэн хойно тайлбарлах нь
 * оройтсон байна. Дараа нь eID. Бүртгэл нь хоёулаа биелсэн хойно үүснэ.
 */
export default function BindPage(){const {t}=useI18n();const brand=useBrand();
  const [binding,setBinding]=useState("");
  const [info,setInfo]=useState<{provider:string;email:string;name:string;consented:boolean;claims:Record<string,unknown>;eid_claims:string[]}|null>(null);
  const [error,setError]=useState("");
  const [consented,setConsented]=useState(false);
  const [next,setNext]=useState("/profile");

  useEffect(()=>{const params=new URLSearchParams(location.search);const b=params.get("b")||"";setBinding(b);setNext(safeReturnPath(params.get("next")));
    if(!b){setError(t("auth.bind.expired"));return}
    void api.bindingSession(b).then(s=>{setInfo(s);setConsented(s.consented)}).catch(()=>setError(t("auth.bind.expired")))},[t]);

  async function agree(){try{await api.bindingConsent(binding);setConsented(true)}catch(e:any){setError(e?.message||t("auth.bind.expired"))}}

  // Провайдерийн хэлснийг тайлбарлахын оронд шууд үзүүлнэ: хуваалцахыг
  // зөвшөөрч буй хүн тэр зүйлээ харах эрхтэй.
  const shared=info?[["Нэр",info.name],["И-мэйл",info.email]].filter(([,v])=>v):[];

  return <main className="signin-shell">
    <header className="signin-shell__nav">
      <Link href="/" className="gp-brand"><img src={brand.logoUrl} alt=""/><span>{brand.name}</span></Link>
      <LanguageSwitcher/>
    </header>
    <section className="signin-shell__body">
      <div className="signin-card">
        <div className="signin-card__asker">
          <strong>{t("auth.bind.title")}</strong>
          <span>{info?t("auth.bind.subtitle",{provider:info.provider}):t("auth.sso.checking")}</span>
        </div>
        <Separator/>

        {error&&<Alert variant="danger" live>{error}</Alert>}

        {info&&!consented&&<>
          <section className="rounded-md border border-line bg-surface-2 p-4">
            <h3 className="m-0 mb-2 text-xs font-semibold uppercase tracking-wide text-muted">{t("auth.bind.from_provider",{provider:info.provider})}</h3>
            <ul className="m-0 grid list-none gap-2 p-0">{shared.map(([k,v])=><li key={k as string} className="flex justify-between gap-3 text-sm"><span className="text-muted">{k}</span><b className="font-semibold">{v as string}</b></li>)}</ul>
          </section>
          <section className="rounded-md border border-line bg-surface-2 p-4">
            <h3 className="m-0 mb-2 text-xs font-semibold uppercase tracking-wide text-muted">{t("auth.bind.from_eid")}</h3>
            <ul className="m-0 grid list-none gap-2 p-0">{info.eid_claims.map(c=><li key={c} className="text-sm text-muted">{c}</li>)}</ul>
          </section>
          <p className="m-0 text-center text-sm text-muted">{t("auth.bind.consent_body")}</p>
          <Button size="xl" className="w-full" leadingIcon={<ShieldCheck/>} onClick={agree}>
            {t("auth.bind.agree")}
          </Button>
        </>}

        {info&&consented&&<>
          <div><h1 className="signin-card__title">{t("auth.bind.verify_title")}</h1>
            <p className="signin-card__lede">{t("auth.bind.verify_lede")}</p></div>
          <EIDLogin next={next} variant="signin" binding={binding}/>
        </>}
      </div>
    </section>
  </main>
}
