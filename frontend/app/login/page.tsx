"use client";
import {useEffect,useState} from "react";
import {useRouter} from "next/navigation";
import Link from "next/link";
import EIDLogin from "@/components/EIDLogin";
import LanguageSwitcher from "@/components/LanguageSwitcher";
import {api} from "@/lib/api";
import {resetAccess} from "@/lib/access";
import {useI18n} from "@/lib/i18n";
import {useBrand} from "@/lib/brandContext";
import type {TranslationKey} from "@/lib/i18n";
import {ChevronDown,HelpCircle,Lock,Mail,ShieldCheck} from "lucide-react";
import {Alert,Button,Input,Separator,Spinner} from "@gerege-systems/ui";
import { GoogleMark } from "@/components/ProviderMark";
import {safeReturnPath} from "@/lib/safeReturnPath.mjs";

/** Серверийн богино кодыг хүн уншихаар мессеж болгоно. Танихгүй кодыг — жишээ
    нь провайдерийн өөрийнх нь илгээсэн ер бусын алдааг — ерөнхий мэдэгдэл
    авна: код нь бүртгэлд үлддэг, дэлгэц дээр гарах ёсгүй. */
const SSO_ERRORS:Record<string,TranslationKey>={no_account:"auth.sso.error_no_account",binding_failed:"auth.sso.error_binding",provider_unreachable:"auth.sso.error_unreachable",stale_request:"auth.sso.error_stale",access_denied:"auth.sso.error_denied",email_unverified:"auth.sso.error_email_unverified",domain_not_allowed:"auth.sso.error_domain_not_allowed"};

/** Google-ийн албан ёсны дөрвөн өнгийн "G". */

/**
 * Талбарууд ХООСОН эхэлнэ.
 *
 * Энэ хоёр нь `useState("admin@example.com")` ба `useState("Password123!")`
 * гэж бөглөгдсөн байсан бөгөөд хоёр зүйл буруу байв.
 *
 * Нэг нь харагдах байдал: тэдгээр нь placeholder мэт харагддаг ч жинхэнэ утга
 * тул хүн бичиж эхлэхэд арилдаггүй, шинэ текст нь ард нь наалдаж хоёр дахин
 * бичигдсэн мэт болно. Placeholder бол саарал бичээс биш, **утгагүй** талбарын
 * шинж — тиймээс жинхэнэ placeholder болов.
 *
 * Хоёр дахь нь илүү чухал: нийтэд нээлттэй нэвтрэх дэлгэц бүр зочин болгонд
 * ажиллаж магадгүй админы нэр, нууц үгийн таамгийг бэлэн бичээд өгч байсан.
 * Демо суулгацад тохирох байсан ч бүх суулгацад тохирохгүй, мөн демо гэдгийг
 * энэ дэлгэц мэдэх ч аргагүй.
 */
export default function LoginPage(){const router=useRouter();const {t}=useI18n();const brand=useBrand();const [next,setNext]=useState("/profile"),[admin,setAdmin]=useState(false),[email,setEmail]=useState(""),[password,setPassword]=useState(""),[error,setError]=useState("");
  // undefined = хараахан асуугаагүй. Энэ ялгаа чухал: асуухаас өмнө eID
  // хэлбэрийг зурчихвал холбоосон суулгац дээр хүн энд нэвтэрч болно гэж
  // хэсэг хугацаанд итгэж, дараа нь өөр рүү шилжсэн нь будлиантай.
  const [sso,setSSO]=useState<{enabled:boolean;provider_name?:string;start_url?:string;local_login:boolean;google?:{enabled:boolean;start_url?:string};access_mode?:"public"|"private"}|undefined>();
  // Хэн асууж байна. Зөвхөн authorization хүсэлтээс ирсэн үед л утгатай, ба
  // нэрийг нь серверээс асууна — `next` дотор ирсэн client_id-г л ашиглаж,
  // дэлгэц дээр гарах нэрийг хаяг тодорхойлохыг зөвшөөрөхгүй.
  const [asker,setAsker]=useState<{client_name:string}|null>(null);

  useEffect(()=>{const requested=new URLSearchParams(location.search).get("next");setNext(safeReturnPath(requested));
    const failed=new URLSearchParams(location.search).get("sso_error");if(failed)setError(t(SSO_ERRORS[failed]||"auth.sso.error_generic"));
    // Алдаагаа өөрөө барина: тохиргоо ирэхгүй бол энэ суулгац өөрөө нэвтрүүлдэг
    // гэж үзнэ — эс бөгөөс API-гийн түр саатал нэвтрэх дэлгэцийг хоослоно.
    void api.ssoConfig().then(setSSO).catch(()=>setSSO({enabled:false,local_login:true,google:{enabled:false}}))},[t]);

  useEffect(()=>{if(!next.startsWith("/oauth2/auth"))return;
    const clientID=new URLSearchParams(next.slice(next.indexOf("?")+1)).get("client_id");
    if(!clientID)return;
    // Танихгүй client бол чимээгүй өнгөрнө: нэвтрэх дэлгэц ажилласаар байх ёстой
    // ба буруу client_id-г /oauth2/auth өөрөө татгалзана.
    void api.oauthClientInfo(clientID).then(setAsker).catch(()=>{})},[next]);

  // Провайдер руу шилжих нь энэ дэлгэцийн ажил дуусгах цэг: цаашид энд юу ч
  // асуухгүй тул нэмэлт товч харуулахгүйгээр шууд илгээнэ.
  useEffect(()=>{if(sso?.enabled&&!sso.local_login&&sso.start_url&&!error)window.location.assign(`${sso.start_url}?next=${encodeURIComponent(next)}`)},[sso,next,error]);
  function startSSO(){if(sso?.start_url)window.location.assign(`${sso.start_url}?next=${encodeURIComponent(next)}`)}
  function startGoogle(){if(sso?.google?.start_url)window.location.assign(`${sso.google.start_url}?next=${encodeURIComponent(next)}`)}
  // resetAccess before the push: router.push is a client-side navigation, so
  // whatever the previous session left cached would answer for this one.
  async function passwordLogin(e:React.FormEvent){e.preventDefault();setError("");try{await api.login(email,password);resetAccess();
    // /oauth2/* belongs to the API, not to this Next app, so a client-side
    // push would 404 instead of resuming the authorization request. eID sign-in
    // already navigates for real; this path has to as well.
    if(next.startsWith("/oauth2/"))window.location.assign(next);else router.push(next)}catch(err:any){setError(err.message||t("auth.message.error_password"))}}

  const federated=sso?.enabled===true;
  const provider=sso?.provider_name||"";
  // Холбоосон, орон нутгийн нэвтрэлтгүй суулгац дээр энэ дэлгэц бол зөвхөн
  // дамжуулах цэг. Алдаа гарсан үед л энд үлдэж, юу болсныг хэлнэ.
  const redirecting=federated&&!sso?.local_login&&!error;
  const showLocal=!!sso&&(!federated||sso.local_login);

  return <main className="signin-shell">
    <header className="signin-shell__nav">
      <Link href="/" className="gp-brand"><img src={brand.logoUrl} alt=""/><span>{brand.name}</span></Link>
      <LanguageSwitcher/>
    </header>
    <section className="signin-shell__body">
      <div className="signin-card">
        {/* Хэн асууж байна. Authorization хүсэлтээс ирээгүй бол платформ өөрөө
            асууж байна гэсэн үг — тэр үед ч гэсэн карт нэгэн ижил харагдана. */}
        <div className="signin-card__asker">
          <strong>{asker?.client_name||t("auth.view.platform_name")}</strong>
          <span>{t("auth.signin.asker_note")}</span>
        </div>
        <Separator/>

        {sso===undefined&&<p className="flex items-center justify-center gap-2 py-6 text-sm text-muted" role="status"><Spinner size="sm" decorative/>{t("auth.sso.checking")}</p>}

        {redirecting&&<>
          <div><h1 className="signin-card__title">{t("auth.signin.title")}</h1><p className="signin-card__lede">{t("auth.sso.redirecting",{provider})}</p></div>
        </>}

        {federated&&!redirecting&&<>
          <div><h1 className="signin-card__title">{t("auth.signin.title")}</h1><p className="signin-card__lede">{t("auth.sso.card_body",{provider})}</p></div>
          {error&&<Alert variant="danger" live>{error}</Alert>}
          <Button size="xl" className="w-full" leadingIcon={<ShieldCheck/>} onClick={startSSO}>{t("auth.sso.sign_in",{provider})}</Button>
        </>}

        {showLocal&&<>
          {!federated&&<div><h1 className="signin-card__title">{t("auth.signin.title")}</h1><p className="signin-card__lede">{t("auth.signin.lede")}</p></div>}
          {/* A private deployment provisions nobody. Saying so here is the
              difference between somebody understanding why eID signed them in
              nowhere and somebody trying it four more times: the server refuses
              the same way whatever this screen shows, and this is the sentence
              that makes the refusal make sense. */}
          {sso?.access_mode==="private"&&<p className="m-0 text-center text-sm text-muted">{t("auth.message.platform_private")}</p>}
          {error&&!federated&&<Alert variant="danger" live>{error}</Alert>}
          <EIDLogin next={next} variant="signin"/>

          {/* Google. Сервер тохируулсан үед л гарна: тохируулаагүй байхад
              дарж болох мөртлөө юу ч болдоггүй товч харуулах нь амлалт биш,
              эвдрэл. */}
          {sso?.google?.enabled&&<>
            <div className="flex items-center gap-2 text-xs text-muted before:flex-1 before:border-t before:border-line after:flex-1 after:border-t after:border-line">{t("auth.signin.or")}</div>
            <Button variant="outline" size="xl" className="w-full" leadingIcon={<GoogleMark/>} onClick={startGoogle}>{t("auth.signin.google")}</Button>
          </>}

          <div className="flex flex-col gap-3">
            <Separator/>
            <Button variant="ghost" className="w-full text-muted" aria-expanded={admin} leadingIcon={<Lock/>} trailingIcon={<ChevronDown className={admin?"rotate-180":""}/>} onClick={()=>setAdmin(v=>!v)}>{t("auth.action.admin_disclosure")}</Button>
            {admin&&<form className="grid gap-3" onSubmit={passwordLogin}>{error&&<Alert variant="danger" live>{error}</Alert>}<Input type="email" label={t("auth.field.email")} hideLabel prefix={<Mail aria-hidden="true"/>} autoComplete="username" placeholder={t("auth.field.email")} value={email} onChange={e=>setEmail(e.target.value)} required/><Input type="password" label={t("auth.field.password")} hideLabel prefix={<Lock aria-hidden="true"/>} autoComplete="current-password" placeholder={t("auth.field.password")} value={password} onChange={e=>setPassword(e.target.value)} required/><Button type="submit" className="w-full">{t("auth.action.admin_sign_in")}</Button></form>}
            {/* Link rather than an anchor: this points at a page of this
                application, and a full page load here throws away the sign-in
                state the screen is holding. */}
            <Link className="inline-flex items-center justify-center gap-2 text-sm text-muted no-underline hover:text-foreground" href="/"><HelpCircle className="size-4" aria-hidden="true"/> {t("auth.signin.help")}</Link>
          </div>
        </>}
      </div>
    </section>
  </main>
}
