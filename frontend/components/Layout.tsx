"use client";

import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { api, APP_MENU_CHANGED_EVENT } from "@/lib/api";
import { resetAccess } from "@/lib/access";
import { useI18n } from "@/lib/i18n";
import { useTheme } from "@/lib/theme";
import { useBrand } from "@/lib/brandContext";
import UserMenu from "@/components/UserMenu";
import { TenantChoices, forgetTenants, useTenants } from "@/components/TenantChoices";
import AICopilot from "@/components/AICopilot";
import { invokeShell, useShell, SHELL_EVENTS, SHELL_METHODS, type ShellNavigatePayload, type ShellSearchPayload } from "@/lib/shell";
import { currentDeviceLine, type DeviceLine } from "@/lib/deviceLine";
import { MenuIcon } from "@/lib/icons";
import { isPublicPath } from "@/lib/publicRoutes";
import { homeScreensVisible, organisationScreensVisible } from "@/lib/workspaceKind.mjs";
import { LayoutGrid, Settings, Menu as HamburgerIcon, Palette, Building2, Megaphone, Search, Ellipsis, ShieldCheck, RefreshCw, MailCheck, ChevronDown, ChevronsDownUp, ChevronsUpDown, ExternalLink, Sparkles, Inbox} from "lucide-react";

// app_order and app_chrome describe the app rather than the entry: where its
// tile sits in the rail, and whether it has a tile at all. Both come from the
// app's manifest — see backend/pkg/catalog.Manifest.
interface MenuItem { id:string; app_id?:string; app_name?:string; parent_id?:string; label:string; path?:string; external_url?:string; icon:string; order:number; app_order?:number; app_chrome?:boolean }
// path is a route in this application; external_url is somewhere else. An app
// installed from the store may be either, so an AppNav carries whichever its
// first menu entry has and the rail renders a Link or an anchor accordingly.
interface AppNav { id:string; name:string; icon:string; path:string; externalUrl?:string; order:number; chrome:boolean; menus:MenuItem[] }

// The platform groups are the only ones not backed by a server menu row, so
// they need ids of their own. Not the translated title: the collapsed set is
// remembered across sessions and a Mongolian operator who switches to English
// would otherwise find every group open again.
const PLATFORM_GROUPS={modules:"platform.modules",settings:"platform.settings"};
const GROUPS_KEY="gerege_sidebar_groups";
// Whether a route lives under a menu path. Compared segment by segment, because
// a raw prefix test also matches a sibling whose path merely begins with the
// same characters: "/products-catalog".startsWith("/products") is true, so the
// Products app would claim the other app's routes, highlight its own tile in
// the rail and render its own menu — leaving the sibling unreachable whenever
// both are installed.
function isUnder(pathname:string,path:string){return pathname===path||pathname.startsWith(path.endsWith("/")?path:path+"/")}
// Which entry is *the* current one, when several of them match.
//
// isUnder is the right test for "does this app own this route" — an app claims
// its whole subtree — but the wrong one for highlighting a single link, because
// a menu that nests answers yes twice: /organisation/people is under
// /organisation, so Organisation and People both lit up and the sidebar showed
// two current pages. The longest match is the specific one, and the specific
// one is where you are.
function currentPath(pathname:string,paths:string[]){
  let best="";
  for(const path of paths) if(path&&isUnder(pathname,path)&&path.length>best.length) best=path;
  return best;
}

// Where an unordered app sits: after every app that asked for a place, and then
// in id order among its equals. Kept from the list this replaced, which used
// the same 999 for anything it did not name — most apps have no opinion about
// their position and should not have to invent one.
const UNORDERED=999;

export default function Layout({children}:{children:React.ReactNode}){
  const [menus,setMenus]=useState<MenuItem[]>([]),[user,setUser]=useState<any>(null),[loading,setLoading]=useState(true);
  const [mobileOpen,setMobileOpen]=useState(false),[mobileMoreOpen,setMobileMoreOpen]=useState(false),[panelOpen,setPanelOpen]=useState(true);
  const [query,setQuery]=useState("");
  // Бүрхүүлийн доторх хайлт: толгой хэсэг зурагдахгүй тул хайлтын талбар нь
  // ажлын мужид түр нээгддэг давхарга болно.
  const [shellSearchOpen,setShellSearchOpen]=useState(false);
  // Бүрхүүл нэвтрэлтийг барьж авсны дараа өгөгдлөө нэг удаа дахин татахад.
  const [authNonce,setAuthNonce]=useState(0);
  const reLoginTried=useRef(false);
  const {shell,inShell}=useShell();
  // Төхөөрөмжийн domain шугам. Гүүр байгаа эсэхээс үл хамааран эдгээр host нь
  // зөвхөн native хүрээнд үйлчилдэг тул тэнд web өөрийн chrome-оо зурахгүй.
  // SSR-д `window` байхгүй тул mount-ын дараа уншина.
  const [deviceLine,setDeviceLine]=useState<DeviceLine|null>(null);
  // Which groups are shut, not which are open. A newly installed app arrives
  // with ids nobody has an opinion about yet, and the useful default for those
  // is the behaviour before this existed: open.
  const [closedGroups,setClosedGroups]=useState<string[]>([]);
  const pathname=usePathname(),router=useRouter(),{t,locale}=useI18n(),theme=useTheme(),brand=useBrand();
  const isPublic=isPublicPath(pathname);

  useEffect(()=>setPanelOpen(localStorage.getItem("gerege_sidebar_open")!=="false"),[]);
  useEffect(()=>{try{const saved=JSON.parse(localStorage.getItem(GROUPS_KEY)||"[]");if(Array.isArray(saved))setClosedGroups(saved.filter(id=>typeof id==="string"))}catch{/* hand-edited or half-written storage is not worth a crashed shell */}},[]);
  useEffect(()=>setDeviceLine(currentDeviceLine()),[]);
  const workAreaOnly=inShell||deviceLine!==null;
  useEffect(()=>{
    if(isPublic){setLoading(false);return}
    let cancelled=false;
    void(async()=>{
      try{
        const [u,m]=await Promise.all([api.getMe(),api.getMenus()]);
        if(cancelled)return;
        reLoginTried.current=false;
        setUser(u);setMenus(m||[]);
      }catch{
        if(cancelled)return;
        // Бүрхүүл дотор /login гэдэг web хуудас байхгүй — нэвтрэлтийг native тал
        // эзэмшдэг. Тэр барьж авч чадвал өгөгдлөө дахин татна. reLoginTried нь
        // дахин нэвтэрсэн ч session хүчингүй хэвээр байх үед мөчлөг үүсгэхээс
        // сэргийлнэ.
        if(!reLoginTried.current){
          reLoginTried.current=true;
          const result=await invokeShell(SHELL_METHODS.AUTH_RE_LOGIN);
          if(cancelled)return;
          if(result.ok){setAuthNonce(n=>n+1);return}
        }
        // Төхөөрөмжийн шугам дээр `/login` нь шугамын нүүр рүү эргэж
        // шилжүүлэгддэг тул энд түлхвэл мөчлөг үүснэ.
        if(currentDeviceLine())return;
        router.push("/login");
      }finally{
        if(!cancelled)setLoading(false);
      }
    })();
    return()=>{cancelled=true};
  },[pathname,router,isPublic,locale,authNonce]);
  // Бүрхүүлийн цэс, toolbar, deep link нь ижил гэрээгээр ажлын мужтай ярина.
  useEffect(()=>{
    if(!shell)return;
    const offNavigate=shell.on(SHELL_EVENTS.NAVIGATE,payload=>{
      const path=(payload as ShellNavigatePayload|null)?.path;
      // Зөвхөн апп доторх зам — "//host" нь протокол-харьцангуй гадаад хаяг.
      if(typeof path==="string"&&path.startsWith("/")&&!path.startsWith("//"))router.push(path);
    });
    const offSearch=shell.on(SHELL_EVENTS.SEARCH,payload=>{
      const incoming=(payload as ShellSearchPayload|null)?.query;
      if(typeof incoming!=="string")return;
      setQuery(incoming);setShellSearchOpen(true);
    });
    return()=>{offNavigate();offSearch()};
  },[shell,router]);
  useEffect(()=>{
    if(isPublic)return;
    const refreshMenus=()=>{
      void api.getMenus().then(m=>setMenus(m||[])).catch(()=>{});
      // Native цэс нь яг энэ жагсаалтаас баригддаг тул бүрхүүлд ч дуулгана.
      if(shell)void shell.invoke(SHELL_METHODS.MENU_CHANGED,{}).catch(()=>{});
    };
    window.addEventListener(APP_MENU_CHANGED_EVENT,refreshMenus);
    return()=>window.removeEventListener(APP_MENU_CHANGED_EVENT,refreshMenus);
  },[isPublic,locale,shell]);
  useEffect(()=>{setMobileOpen(false);setMobileMoreOpen(false);setShellSearchOpen(false)},[pathname]);

  const apps=useMemo<AppNav[]>(()=>{
    const groups=new Map<string,MenuItem[]>();
    menus.filter(m=>m.app_id).forEach(m=>groups.set(m.app_id!,[...(groups.get(m.app_id!)||[]),m]));
    // An app with nothing to link to is dropped rather than rendered: the tile
    // is built from its first linkable entry, and a group heading alone would
    // have made that undefined.
    return [...groups.entries()].flatMap(([id,items])=>{
      const sorted=items.sort((a,b)=>a.order-b.order),first=sorted.find(item=>item.path||item.external_url);
      if(!first)return[];
      return[{id,name:first.label||first.app_name||id,icon:first.icon,path:first.path||first.external_url!,externalUrl:first.path?undefined:first.external_url,order:first.app_order||UNORDERED,chrome:!!first.app_chrome,menus:sorted}];
    }).sort((a,b)=>a.order-b.order||a.id.localeCompare(b.id));
  },[menus]);
  // An app the shell presents as part of itself rather than as a tile in the
  // rail, and this is the line that makes that true rather than merely drawn.
  // One app claims it today — the organisation — and it claims it in its own
  // manifest rather than being named here.
  //
  // Its screens are still the module's — the server sends them only when the
  // app is installed and enabled, which is what keeps the links honest on a
  // tenant that has removed it — but they are rendered inside the platform's
  // own group. Left in the rail as well, clicking one of them selected the app,
  // and selecting an app replaces the whole sidebar: the menu you clicked in
  // disappeared and you were somewhere else. The screens are the same; where
  // you are should not change under you for opening one.
  // Every app the shell presents as part of itself, not one. Three claim it
  // today — the organisation, the assistant and the connectors — and the shell
  // used to take the first, which meant the other two put nothing anywhere: on
  // a phone, where the tab bar lists only rail apps, their screens had no route
  // at all.
  //
  // Each entry lands in the group its module asked for. The platform decides
  // the parent — an app cannot file a screen under another app's heading — and
  // the id it stamps is what says which of the two this is.
  const chromeApps=useMemo(()=>apps.filter(app=>app.chrome),[apps]);
  const chromeEntries=useCallback((group:"modules"|"settings")=>chromeApps.flatMap(app=>
    app.menus.filter(item=>item.path&&item.parent_id?.endsWith(`_${group}`))),[chromeApps]);
  const railApps=useMemo(()=>apps.filter(app=>!app.chrome),[apps]);
  const selected=railApps.find(app=>app.menus.some(m=>m.path&&isUnder(pathname,m.path)))||null;
  const platformActive=!selected;
  const searchIndex=useMemo(()=>[
    {label:t("web.menu.app_store"),app:t("web.label.platform"),path:"/apps",icon:"grid"},
    {label:t("web.menu.appearance"),app:t("web.label.platform"),path:"/settings/appearance",icon:"palette"},
    {label:t("web.menu.installed_apps"),app:t("web.label.platform"),path:"/settings/apps",icon:"settings"},
    ...apps.flatMap(app=>app.menus.filter(m=>m.path).map(m=>({label:m.label,app:app.name,path:m.path!,icon:m.icon})))
  ],[apps,t]);
  const results=query.trim()?searchIndex.filter(x=>(x.label+" "+x.app).toLocaleLowerCase().includes(query.trim().toLocaleLowerCase())).slice(0,8):[];

  function togglePanel(){if(window.matchMedia("(min-width:901px)").matches){setPanelOpen(v=>{localStorage.setItem("gerege_sidebar_open",String(!v));return !v})}else setMobileOpen(v=>!v)}
  function persistGroups(next:string[]){localStorage.setItem(GROUPS_KEY,JSON.stringify(next));setClosedGroups(next)}
  function toggleGroup(id:string){persistGroups(closedGroups.includes(id)?closedGroups.filter(x=>x!==id):[...closedGroups,id])}
  // Only the groups on screen. Expand-all on the Documents menu should not
  // silently reopen everything the operator shut on Billing — the button says
  // what it does to the panel in front of them, and nothing else.
  const visibleGroups=selected?selected.menus.filter(m=>!m.parent_id).map(m=>m.id):Object.values(PLATFORM_GROUPS);
  const allGroupsOpen=visibleGroups.every(id=>!closedGroups.includes(id));
  function toggleAllGroups(){persistGroups(allGroupsOpen?[...new Set([...closedGroups,...visibleGroups])]:closedGroups.filter(id=>!visibleGroups.includes(id)))}
  // resetAccess before navigating: /login is a client-side route, so the cached
  // identity would otherwise still be the signed-out user's when the next
  // person signs in at this tab.
  // Гарсан хүнийг нүүр хуудас угтана, нэвтрэх дэлгэц биш. Нэвтрэх дэлгэц бол
  // тэр хүний дөнгөж сая орхисон зүйл рүү буцах хаалга — гарлаа гэж хэлсэн
  // хүнд түүнийг шууд харуулах нь "үнэхээр гарах уу?" гэж дахин асуусантай
  // адил. Нүүр хуудас өөрөө eID нэвтрэлт болон толгойн "Нэвтрэх" холбоос
  // хоёуланг агуулдаг тул буцаж орох зам хаагдахгүй.
  //
  // Төхөөрөмжийн domain шугам дээр `/` нь тухайн шугамын нүүр рүү шилждэг
  // (proxy.ts), тиймээс энэ нэг зам хоёр орчинд зөв утгатай.
  //
  // push биш replace: push үлдээвэл Back дарахад дөнгөж сая гарсан хамгаалалттай
  // хуудас руу буцаж, тэндээс 401 аваад яг тэр нэвтрэх дэлгэц рүү шидэгдэнэ.
  //
  // Холбоосон суулгац дээр гарах нь энд дуусдаггүй: провайдер өөрийн session-ээ
  // хэвээр барьж байгаа тул "гарлаа" гээд "нэвтрэх" дарахад шууд буцаж ороход
  // хүн гарсан гэж үзэхгүй. Тиймээс сервер end_session_url буцаавал хөтчийг
  // тийш нь илгээнэ — провайдер өөрийнхөө session-ийг хааж, бүртгэлтэй
  // post-logout хаягаар нь энэ суулгац руу буцаана.
  async function logout(){let endSession="";try{const res=await api.logout();endSession=res.end_session_url||""}catch{}resetAccess();forgetTenants();if(endSession)window.location.assign(endSession);else router.replace("/")}
  const brandTitle=selected?.name||(t("web.label.platform"));
  // A home is a workspace and gets this shell, minus the screens that are about
  // being a company. See lib/workspaceKind.mjs for why the rule lives there
  // rather than as the same condition written out four times here.
  const company=organisationScreensVisible(user?.workspace_kind);
  const ownHome=homeScreensVisible(user?.workspace_kind);
  const mobileAppTabs=[
    // The platform tab is the way back out of an app on a phone, so it always
    // exists — it is where it goes that changes. The app store is the shelf a
    // company buys from; a home has nothing to buy, and the person's own record
    // is what they came back to the shell for.
    {id:"platform",href:company?"/apps":"/profile",external:false,active:platformActive,label:t("web.label.platform"),icon:<LayoutGrid className="w-5 h-5"/>},
    ...railApps.map(app=>({id:app.id,href:app.path,external:!!app.externalUrl,active:selected?.id===app.id,label:app.name,icon:<MenuIcon name={app.icon} className="w-5 h-5"/>})),
  ];
  const hasMobileMore=mobileAppTabs.length>5;
  const primaryMobileTabs=hasMobileMore?mobileAppTabs.slice(0,4):mobileAppTabs;
  const remainingMobileTabs=hasMobileMore?mobileAppTabs.slice(4):[];

  if(isPublic)return <>{children}</>;
  if(loading)return <div className="min-h-screen flex items-center justify-center bg-slate-50 text-slate-500 font-medium">{t("web.message.loading_platform")}</div>;

  const platformMenus=<><MenuGroup id={PLATFORM_GROUPS.modules} title={t("web.group.modules")} closed={closedGroups.includes(PLATFORM_GROUPS.modules)} onToggle={toggleGroup}>
    {/* The mirror of the two lines below: an organisation's screens are hidden
        in a home, and the home's own screen is hidden in an organisation. A
        member of a company asks for things through the company, so this list
        would be permanently empty for them — and an empty entry in a rail is a
        promise the screen behind it cannot keep. */}
    {ownHome&&<NavLink href="/me" active={pathname==="/me"} icon={<Inbox className="w-5 h-5"/>} label={t("web.menu.my_requests")}/>}
    {company&&<NavLink href="/apps" active={pathname==="/apps"} icon={<LayoutGrid className="w-5 h-5"/>} label={t("web.menu.app_store")}/>}
    {/* The organisation's own legal identity. A platform screen rather than a
        menu entry the organisation app contributes: it is read by the control
        plane, by the state registry rail and by an SSO consent screen, so it
        has to stay reachable on a tenant that has removed every app.
        Under Modules rather than Settings, because it is a thing you look at
        and edit — the organisation itself — not a switch that changes how the
        platform behaves. */}
    {company&&<NavLink href="/organisation" active={pathname==="/organisation"} icon={<Building2 className="w-5 h-5"/>} label={t("web.menu.organisation")}/>}
    {/* What the organisation offers the public, beside what it is. It was a
        card at the bottom of the organisation's own screen, which is where a
        thing goes when nobody has decided it is a thing: publishing a service
        is an outward promise — a stranger finds it in the directory and asks —
        and it deserves the same standing in the menu as the identity above
        it. */}
    {company&&<NavLink href="/organisation/services" active={pathname==="/organisation/services"} icon={<Megaphone className="w-5 h-5"/>} label={t("core.view.services_title")}/>}
    {/* Its screens, next in the list rather than nested under it. They were
        indented for a while, which made them look like a second level this
        sidebar does not otherwise have — one entry with children, in a menu
        where nothing else has any. Ordinary rows in the order you would open
        them: the organisation, how it is arranged, who is in it.
        Taken from the server rather than written out here, so a tenant that
        has removed the app sees the profile screen and not two links to 403s —
        and so the labels stay in the seven languages the module declares. */}
    {chromeEntries("modules").map(item=>
      <NavLink key={item.id} href={item.path!} active={item.path===pathname}
        icon={<MenuIcon name={item.icon} className="w-5 h-5"/>} label={item.label}/>)}
  </MenuGroup><MenuGroup id={PLATFORM_GROUPS.settings} title={t("web.group.settings")} closed={closedGroups.includes(PLATFORM_GROUPS.settings)} onToggle={toggleGroup}>
    {/* Under Settings, where its screen already lives: /settings/apps is what
        the address bar says, and a sidebar that files it under Modules asks
        somebody to hold two answers for where the same page is. */}
    {company&&<NavLink href="/settings/apps" active={pathname==="/settings/apps"} icon={<Settings className="w-5 h-5"/>} label={t("web.menu.installed_apps")}/>}
    <NavLink href="/settings/appearance" active={pathname==="/settings/appearance"} icon={<Palette className="w-5 h-5"/>} label={t("web.menu.appearance")}/>
    {chromeEntries("settings").map(item=>
      <NavLink key={item.id} href={item.path!} active={item.path===pathname}
        icon={<MenuIcon name={item.icon} className="w-5 h-5"/>} label={item.label}/>)}
    {/* Issuing a key that sends mail in the tenant's name is administrative, and
        the API behind this screen is admin-only, so the link follows it. */}
    {user?.is_admin&&<NavLink href="/settings/access" active={pathname==="/settings/access"} icon={<ShieldCheck className="w-5 h-5"/>} label={t("access.view.title")}/>}
  </MenuGroup></>;

  // Бүрхүүл дотор ба төхөөрөмжийн domain шугам дээр: толгой хэсэг ба мобайл
  // навигаци зурагдахгүй — хайлт, хэрэглэгч, нэвтрэлт, цонхны үйлдлүүдийг
  // native тал эзэмшинэ. Хажуугийн цэс нь ЭНД үлдэнэ: тэр бол ажлын мужийн
  // доторх навигаци бөгөөд аль апп идэвхтэй, ямар эрхтэй, ямар хэлээр гэдгийг
  // web тал аль хэдийн мэддэг.
  if(workAreaOnly)return <div className="gerege-shell gerege-workarea h-screen flex flex-col overflow-hidden">
    <ImpersonationBanner active={!!user?.impersonated}/>
    <RibbonBar selected={selected} brandTitle={brandTitle} user={user} setShellSearchOpen={setShellSearchOpen} t={t} onLogout={logout} />
    <div className="flex flex-1 min-h-0 overflow-hidden">
      <div className="gerege-sidebar bottom-0 left-0 z-40 flex overflow-hidden is-desktop-open">
        <nav className="w-16 min-w-16 shrink-0 py-3 flex flex-col items-center gap-2 border-r border-[var(--gerege-border)]">
          <AppRailLink href="/apps" active={platformActive} title={t("web.label.platform")} icon={<LayoutGrid className="w-5 h-5"/>}/>
          {railApps.map(app=><AppRailLink key={app.id} href={app.path} external={!!app.externalUrl} active={selected?.id===app.id} title={app.name} icon={<MenuIcon name={app.icon} className="w-5 h-5"/>}/>) }
        </nav>
        <aside className="gerege-menu-panel overflow-hidden">
          <div className="w-56 py-4"><nav className="space-y-1 px-2">
            {selected?<AppMenuGroups menus={selected.menus} pathname={pathname} closedGroups={closedGroups} onToggle={toggleGroup}/>:platformMenus}
          </nav></div>
        </aside>
      </div>
      <main className="gerege-main flex-1 p-4 sm:p-6 lg:p-8 overflow-y-auto min-w-0">{children}</main>
    </div>
    <WorkareaFooter />
    {shellSearchOpen&&<div className="gerege-shell-search" role="dialog" aria-modal="true" aria-label={t("web.view.search_placeholder")}>
      <button type="button" className="gerege-shell-search-backdrop" aria-label={t("base.action.close")} onClick={()=>{setShellSearchOpen(false);setQuery("")}}/>
      <div className="gerege-shell-search-panel">
        <div className="relative"><Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400"/>
          <input autoFocus value={query} onChange={e=>setQuery(e.target.value)} onKeyDown={e=>{
            if(e.key==="Escape"){setShellSearchOpen(false);setQuery("")}
            if(e.key==="Enter"&&results[0]){router.push(results[0].path);setShellSearchOpen(false);setQuery("")}
          }} placeholder={t("web.view.search_placeholder")} className="w-full h-11 rounded-xl border border-slate-200 bg-white pl-10 pr-4 text-sm outline-none focus:border-[var(--gerege-blue)]"/>
        </div>
        {results.length>0&&<div className="mt-2 space-y-0.5">{results.map(item=><button key={item.path} onClick={()=>{router.push(item.path);setShellSearchOpen(false);setQuery("")}} className="w-full flex items-center gap-3 rounded-lg px-3 py-2.5 text-left hover:bg-[var(--gerege-surface-2)]"><span className="text-[var(--gerege-blue)]">{<MenuIcon name={item.icon} className="w-4 h-4"/>}</span><span className="min-w-0"><strong className="block text-sm truncate">{item.label}</strong><small className="text-slate-500 truncate">{item.app}</small></span></button>)}</div>}
      </div>
    </div>}
    <AICopilot/>
  </div>;

  return <div className="gerege-shell min-h-screen flex flex-col">
    <ImpersonationBanner active={!!user?.impersonated}/>
    <PlatformNotices notices={user?.notices}/>
    <header className="gerege-topbar h-16 flex items-center border-b sticky top-0 z-50">
      <TenantSwitcher current={user?.tenant_id} currentName={user?.tenant_name}>
        {/* The mark used to arrive as a static import, which gave it a hashed,
            permanently cacheable URL and made it part of the build. A logo the
            build owns is a logo no deployment can change, so it is an address
            now — see lib/brand.ts. */}
        {theme.design==="gerege"?<img src={brand.logoUrl} width={36} height={36} alt={brand.name} className="w-9 h-9 rounded-lg shadow-sm"/>:<span className="original-brand-mark w-9 h-9 rounded-lg grid place-items-center"><Building2 className="w-6 h-6"/></span>}
      </TenantSwitcher>
      <div className={`gerege-header-context h-full flex items-center gap-3 overflow-hidden transition-all duration-200 ${panelOpen?"is-open":""}`}>
        <span className="shrink-0 text-[var(--gerege-blue)]">{selected?(<MenuIcon name={selected.icon} className="w-5 h-5"/>):<LayoutGrid className="w-5 h-5"/>}</span>
        <span className="min-w-0"><small className="block text-[11px] leading-4 text-slate-500 truncate">{brand.name}</small><strong className="block text-[15px] leading-5 text-slate-900 truncate">{brandTitle}</strong></span>
      </div>
      <div className="gerege-menu-toggle h-full shrink-0 flex items-center justify-center gap-1">
        <button onClick={togglePanel} className="grid place-items-center w-10 h-10 rounded-lg text-slate-600 hover:bg-slate-50" aria-label={t("web.action.toggle_menu")} aria-expanded={mobileOpen}><HamburgerIcon className="w-5 h-5"/></button>
        {/* Beside the control that opens the panel, because it acts on what
            that panel contains. Icon-only: the words fit in a 14rem column,
            not in a header cell next to the menu button. */}
        {visibleGroups.length>1&&<button type="button" onClick={toggleAllGroups} aria-expanded={allGroupsOpen}
          aria-label={allGroupsOpen?t("web.action.collapse_all"):t("web.action.expand_all")}
          title={allGroupsOpen?t("web.action.collapse_all"):t("web.action.expand_all")}
          className="grid place-items-center w-10 h-10 rounded-lg text-slate-600 hover:bg-slate-50">
          {allGroupsOpen?<ChevronsDownUp className="w-5 h-5"/>:<ChevronsUpDown className="w-5 h-5"/>}
        </button>}
      </div>
      <div className="hidden lg:flex items-center gap-2 px-4 min-w-0"><span className="gerege-session-dot w-2 h-2 rounded-full shrink-0"/><strong className="text-base text-slate-800 font-semibold truncate max-w-56">{user?.tenant_name||"Demo Tenant"}</strong></div>
      <div className="gerege-header-search hidden md:flex flex-1 items-center justify-center min-w-0 px-5 relative">
        <div className="relative w-full max-w-md"><Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400"/><input value={query} onChange={e=>setQuery(e.target.value)} onKeyDown={e=>{if(e.key==="Enter"&&results[0]){router.push(results[0].path);setQuery("")}}} placeholder={t("web.view.search_placeholder")} className="w-full h-10 rounded-full border border-slate-200 bg-slate-100/80 pl-10 pr-4 text-sm outline-none focus:border-[var(--gerege-blue)] focus:ring-2 focus:ring-[color-mix(in_srgb,var(--gerege-blue)_15%,transparent)]"/>
          {results.length>0&&<div className="gerege-topbar-onlight absolute top-12 inset-x-0 bg-white border border-slate-200 rounded-xl shadow-xl p-1.5 z-[70]">{results.map(item=><button key={item.path} onClick={()=>{router.push(item.path);setQuery("")}} className="w-full flex items-center gap-3 rounded-lg px-3 py-2.5 text-left hover:bg-[var(--gerege-surface-2)]"><span className="text-[var(--gerege-blue)]">{<MenuIcon name={item.icon} className="w-4 h-4"/>}</span><span className="min-w-0"><strong className="block text-sm truncate">{item.label}</strong><small className="text-slate-500 truncate">{item.app}</small></span></button>)}</div>}
        </div>
      </div>
      {/* Профайл нь аватарын цэсэн дотор. Толгой хэсэгт тусдаа товч байсныг
          авав: нэвтэрсний дараа хүн профайл дээрээ бууж ирдэг болсон тул тэр
          нь өөрөө хаана байгааг заасан хэрэг — хажууд нь бас байнга шахагдаж
          зогсох товч илүү. */}
      <div className="gerege-header-user flex items-center gap-2 pr-2 sm:pr-4 lg:pr-6"><AICopilot/><UserMenu user={user} onLogout={logout}/></div>
    </header>

    <div className="flex flex-1 min-h-0">
      {mobileOpen&&<button className="gerege-mobile-backdrop fixed inset-0 top-16 bg-slate-950/40 z-30" aria-label={t("web.action.close_menu")} onClick={()=>setMobileOpen(false)}/>}
      <div className={`gerege-sidebar top-16 bottom-0 left-0 z-40 flex overflow-hidden ${mobileOpen?"is-mobile-open":""} ${panelOpen?"is-desktop-open":""}`}>
        <nav className="w-16 min-w-16 shrink-0 py-3 flex flex-col items-center gap-2 border-r border-[var(--gerege-border)]">
          <AppRailLink href="/apps" active={platformActive} title={t("web.label.platform")} icon={<LayoutGrid className="w-5 h-5"/>}/>
          {railApps.map(app=><AppRailLink key={app.id} href={app.path} external={!!app.externalUrl} active={selected?.id===app.id} title={app.name} icon={<MenuIcon name={app.icon} className="w-5 h-5"/>}/>) }
        </nav>
        <aside className="gerege-menu-panel overflow-hidden">
          <div className="w-56 py-4">
            <nav className="space-y-1 px-2">
              {selected?<AppMenuGroups menus={selected.menus} pathname={pathname} closedGroups={closedGroups} onToggle={toggleGroup}/>:platformMenus}
            </nav>
          </div>
        </aside>
      </div>
      <main className="gerege-main flex-1 p-4 sm:p-6 lg:p-8 overflow-y-auto min-w-0">{children}</main>
    </div>
    {mobileMoreOpen&&<><button className="gerege-mobile-more-backdrop" aria-label={t("web.action.close_more")} onClick={()=>setMobileMoreOpen(false)}/><section className="gerege-mobile-more-sheet" role="dialog" aria-modal="true" aria-label={t("web.view.more_apps")}><div className="gerege-mobile-more-handle"/><h2>{t("web.view.more_apps")}</h2><div className="gerege-mobile-more-grid">{remainingMobileTabs.map(tab=><MobileMoreApp key={tab.id} {...tab}/>)}</div></section></>}
    <nav className="gerege-mobile-tabs" aria-label={t("web.label.apps")}>
      {primaryMobileTabs.map(tab=><MobileAppTab key={tab.id} {...tab}/>)}
      {hasMobileMore&&<button type="button" onClick={()=>setMobileMoreOpen(v=>!v)} aria-expanded={mobileMoreOpen} className={`gerege-mobile-tab ${remainingMobileTabs.some(tab=>tab.active)||mobileMoreOpen?"is-active":""}`}><span><Ellipsis className="w-5 h-5"/></span><small>{t("web.action.more")}</small></button>}
    </nav>
  </div>;
}

/**
 * Ажлын мужийн толгойн мөр — зөвхөн бүрхүүл ба төхөөрөмжийн шугам дээр.
 *
 * Хөтчийн 4rem өндөртэй толгой хэсгийг орлоно: тэнд байсан брэнд, tenant,
 * хайлт, хэрэглэгч дөрвүүлээ энэ 2.5rem мөрөнд багтана. Native тал цонхны
 * үйлдлүүд болон нэвтрэлтийг өөрөө эзэмшдэг тул давхардуулах шаардлагагүй.
 */
function RibbonBar({
  selected,
  brandTitle,
  user,
  setShellSearchOpen,
  t,
  onLogout,
}: {
  selected: AppNav | null;
  brandTitle: string;
  user: any;
  setShellSearchOpen: (open: boolean) => void;
  t: (key: any) => string;
  onLogout: () => void;
}) {
  const brand = useBrand();
  return (
    <div className="gerege-ribbon h-10 shrink-0 border-b border-[var(--gerege-border)] bg-[var(--gerege-chrome)] px-4 flex items-center justify-between text-xs z-30 select-none">
      <div className="flex items-center gap-2.5 min-w-0">
        <span className="text-[var(--gerege-blue)] shrink-0">
          {selected ? (<MenuIcon name={selected.icon} className="w-4 h-4"/>) : <LayoutGrid className="w-4 h-4" />}
        </span>
        <div className="flex items-center gap-1.5 text-xs min-w-0">
          <span className="font-bold text-slate-800 dark:text-slate-100">{brand.name}</span>
          <span className="text-slate-300 dark:text-slate-600">/</span>
          <span className="font-semibold text-[var(--gerege-blue)] truncate">{brandTitle}</span>
        </div>
        {user?.tenant_name && (
          <div className="hidden md:flex items-center pl-3 border-l border-slate-200 dark:border-slate-800">
            <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[11px] font-medium bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-500" />
              <span className="truncate max-w-36">{user.tenant_name}</span>
            </span>
          </div>
        )}
      </div>
      <div className="flex items-center gap-2.5 shrink-0 text-xs">
        <button
          onClick={() => setShellSearchOpen(true)}
          className="flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[11px] font-medium text-slate-600 dark:text-slate-400 bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 dark:hover:bg-slate-700 transition"
        >
          <Search className="w-3.5 h-3.5 text-slate-400" />
          <span>{t("web.view.search_placeholder")}</span>
        </button>
        <button
          onClick={() => window.location.reload()}
          title={t("web.action.reload")}
          className="p-1.5 text-slate-500 hover:text-slate-800 dark:hover:text-slate-200 rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 transition"
        >
          <RefreshCw className="w-3.5 h-3.5" />
        </button>
        <UserMenu user={user} onLogout={onLogout} />
      </div>
    </div>
  );
}

/** Ажлын мужийн хөл. Native footer нь цонхны мөр — энэ нь ажлын мужийнх. */
function WorkareaFooter() {
  const brand = useBrand();
  return (
    <footer className="gerege-footer h-7 shrink-0 border-t border-[var(--gerege-border)] bg-[var(--gerege-chrome)] px-4 flex items-center justify-between text-[11px] text-slate-500 dark:text-slate-400 select-none z-30">
      <span className="flex items-center gap-1.5 font-medium text-slate-700 dark:text-slate-300">
        <span className="w-2 h-2 rounded-full bg-emerald-500"></span>
        <span>{brand.name}</span>
      </span>
      <span className="hidden sm:inline-flex items-center gap-1 text-slate-400">
        <ShieldCheck className="w-3.5 h-3.5 text-emerald-500" />
        <span>TLS</span>
      </span>
    </footer>
  );
}

/**
 * The brand mark, and now the way to change which organisation you are working
 * in.
 *
 * The mark used to link to /apps; the Platform tile directly beneath it in the
 * rail still does, so nothing is lost. What was missing had no home at all:
 * which tenant a session belonged to was decided once, by whichever membership
 * was oldest, and somebody who works for two organisations could reach only the
 * first — signing out and back in landed them in the same one again.
 *
 * Below 900px the header brand is hidden by the mobile shell, so this control
 * is not reachable there yet.
 */
function TenantSwitcher({current,currentName,children}:{current?:string;currentName?:string;children:React.ReactNode}){
  const {t}=useI18n();
  const [open,setOpen]=useState(false);
  const {tenants,activeIDs,switching,failed,switchTo,toggleActive}=useTenants(open);
  const box=useRef<HTMLDivElement>(null);
  const label=currentName?`${currentName} — ${t("web.action.switch_tenant")}`:t("web.action.switch_tenant");

  useEffect(()=>{
    if(!open)return;
    const onPointerDown=(event:MouseEvent)=>{if(!box.current?.contains(event.target as Node))setOpen(false)};
    const onKeyDown=(event:KeyboardEvent)=>{if(event.key==="Escape")setOpen(false)};
    document.addEventListener("mousedown",onPointerDown);
    document.addEventListener("keydown",onKeyDown);
    return()=>{document.removeEventListener("mousedown",onPointerDown);document.removeEventListener("keydown",onKeyDown)};
  },[open]);

  return <div ref={box} className="gerege-header-brand relative w-16 h-full shrink-0 grid place-items-center border-r border-[var(--gerege-chrome-border)]">
    <button type="button" onClick={()=>setOpen(v=>!v)} aria-haspopup="menu" aria-expanded={open} aria-label={label} title={label}
      className="grid place-items-center rounded-lg focus:outline-none focus-visible:ring-2 focus-visible:ring-[var(--gerege-blue)]">
      {children}
    </button>
    {open&&<div role="menu" aria-label={t("web.view.tenants")} className="gerege-topbar-onlight absolute left-2 top-14 w-64 bg-white border border-slate-200 rounded-xl shadow-xl p-1.5 z-[70]">
      <p className="px-4 py-1.5 text-[11px] font-bold uppercase tracking-wider text-slate-400">{t("web.view.tenants")}</p>
      <TenantChoices current={current} tenants={tenants} activeIDs={activeIDs} switching={switching} failed={failed} onChoose={id=>void switchTo(id)} onStay={()=>setOpen(false)} onToggleActive={id=>void toggleActive(id,current||"")}/>
    </div>}
  </div>;
}
// Leaving this product is stated, not implied: an external destination gets a
// new tab, an icon that says so, and rel="noopener" — the page it opens is
// somebody else's and must not be handed a reference back to this window.
function ExternalAnchor({href,className,children,...rest}:{href:string;className?:string;children:React.ReactNode}&React.AnchorHTMLAttributes<HTMLAnchorElement>){
  return <a href={href} target="_blank" rel="noopener noreferrer" className={className} {...rest}>{children}</a>;
}
function AppRailLink({href,external,active,title,icon}:{href:string;external?:boolean;active:boolean;title:string;icon:React.ReactNode}){
  const className=`w-11 h-11 rounded-xl grid place-items-center transition ${active?"bg-[var(--gerege-blue-soft)] text-[var(--gerege-blue)] shadow-sm":"text-slate-500 hover:bg-[var(--gerege-surface-2)] hover:text-slate-800"}`;
  if(external)return <ExternalAnchor href={href} title={title} aria-label={title} className={className}>{icon}</ExternalAnchor>;
  return <Link href={href} title={title} aria-label={title} className={className}>{icon}</Link>;
}
function MobileAppTab({href,external,active,label,icon}:{href:string;external?:boolean;active:boolean;label:string;icon:React.ReactNode}){
  const className=`gerege-mobile-tab ${active?"is-active":""}`;
  if(external)return <ExternalAnchor href={href} aria-label={label} className={className}><span>{icon}</span><small>{label}</small></ExternalAnchor>;
  return <Link href={href} aria-label={label} aria-current={active?"page":undefined} className={className}><span>{icon}</span><small>{label}</small></Link>;
}
function MobileMoreApp({href,external,active,label,icon}:{href:string;external?:boolean;active:boolean;label:string;icon:React.ReactNode}){
  const className=`gerege-mobile-more-app ${active?"is-active":""}`;
  if(external)return <ExternalAnchor href={href} className={className}><span>{icon}</span><strong>{label}</strong></ExternalAnchor>;
  return <Link href={href} aria-current={active?"page":undefined} className={className}><span>{icon}</span><strong>{label}</strong></Link>;
}
function NavLink({href,active,icon,label}:{href:string;active:boolean;icon:React.ReactNode;label:string}){return <Link href={href} className={`gerege-nav-link flex items-center gap-3 px-3 py-2.5 text-sm font-medium transition ${active?"gerege-nav-link-active font-semibold":""}`}><span className="gerege-nav-icon">{icon}</span><span>{label}</span></Link>}
// The same row as a NavLink, minus the highlight: an external destination has
// no path under this application, so nothing it opens can ever be "the page
// you are on".
function ExternalNavLink({href,icon,label}:{href:string;icon:React.ReactNode;label:string}){return <ExternalAnchor href={href} className="gerege-nav-link flex items-center gap-3 px-3 py-2.5 text-sm font-medium transition"><span className="gerege-nav-icon">{icon}</span><span className="flex-1">{label}</span><ExternalLink className="w-3.5 h-3.5 shrink-0 opacity-60"/></ExternalAnchor>}
function MenuGroup({id,title,closed,onToggle,children}:{id:string;title:string;closed:boolean;onToggle:(id:string)=>void;children:React.ReactNode}){
  const bodyId=`menu-group-${id}`;
  return <section className="gerege-menu-group mb-6">
    {/* Still a heading, so the panel keeps its outline for a screen reader;
        the button inside is what the heading names, which is the pairing
        aria-expanded/aria-controls expects. */}
    <h3 className="mb-2">
      <button type="button" onClick={()=>onToggle(id)} aria-expanded={!closed} aria-controls={bodyId} className="w-full flex items-center gap-1.5 px-3 py-1 rounded-md text-[11px] font-bold uppercase tracking-wider text-slate-400 hover:text-slate-600 hover:bg-[var(--gerege-surface-2)] transition">
        <span className="min-w-0 truncate text-left">{title}</span>
        <ChevronDown className={`w-3.5 h-3.5 ml-auto shrink-0 transition-transform duration-200 ${closed?"":"rotate-180"}`}/>
      </button>
    </h3>
    {/* inert and not just hidden by overflow: a link folded away is still a
        link, and without this Tab would walk into a group the operator can
        see is shut and land focus somewhere off-screen. */}
    <div id={bodyId} data-collapsed={closed} inert={closed} className="gerege-menu-group-body">
      <div className="space-y-1">{children}</div>
    </div>
  </section>;
}
function AppMenuGroups({menus,pathname,closedGroups,onToggle}:{menus:MenuItem[];pathname:string;closedGroups:string[];onToggle:(id:string)=>void}){const roots=menus.filter(item=>!item.parent_id).sort((a,b)=>a.order-b.order);
  // Decided once across the whole menu, not per link: the answer depends on
  // what the other entries claim.
  const here=currentPath(pathname,menus.map(item=>item.path||""));
  // Which group an entry belongs to. The server parents every entry a module
  // declares under the app's "Modules" group — the "Settings" group used to be
  // filled from the platform's own blueprint, and an app that has left the
  // platform has none. The module still knows which of its screens are
  // configuration, and says so in the entry's id: ".settings." re-parents it
  // under the sibling Settings group, when that group exists.
  const parentOf=(item:MenuItem)=>{
    if(item.parent_id?.endsWith("_modules")&&item.id.includes(".settings.")){
      const settings=item.parent_id.replace(/_modules$/,"_settings");
      if(menus.some(m=>m.id===settings))return settings;
    }
    return item.parent_id;
  };
  const childrenOf=(rootId:string)=>menus.filter(item=>parentOf(item)===rootId&&(item.path||item.external_url)).sort((a,b)=>a.order-b.order);
  // A group with nothing in it is a heading over silence — the Settings group
  // of every extracted app, until that app declares configuration screens.
  return <>{roots.filter(root=>childrenOf(root.id).length>0).map(root=><MenuGroup key={root.id} id={root.id} title={root.label} closed={closedGroups.includes(root.id)} onToggle={onToggle}>{childrenOf(root.id).map(item=>item.path
  ?<NavLink key={item.id} href={item.path} active={item.path===here} icon={<MenuIcon name={item.icon} className="w-5 h-5"/>} label={item.label}/>
  :<ExternalNavLink key={item.id} href={item.external_url!} icon={<MenuIcon name={item.icon} className="w-5 h-5"/>} label={item.label}/>)}</MenuGroup>)}</>}

/**
 * The banner an operator's borrowed session wears.
 *
 * Rendered above everything, in a colour nothing else on this platform uses,
 * with no way to dismiss it. That is the whole specification: the person whose
 * account this is — and anybody standing behind them — must be able to see at
 * a glance that what is on the screen is not an ordinary session. It is drawn
 * from `impersonated` on /me, which comes from the session row itself, so no
 * client-side state can turn it off.
 */
function ImpersonationBanner({active}:{active:boolean}){
  // Its own useI18n rather than the translate function passed down: the type of
  // `t` is the dictionary's key union, and threading it through a prop turns
  // every call site into a cast.
  const {t}=useI18n();
  if(!active) return null;
  return <div role="status" className="w-full bg-amber-500 text-amber-950 text-sm font-medium px-4 py-2 flex items-center gap-2 shadow-inner">
    <ShieldCheck className="w-4 h-4 shrink-0"/>
    <span>{t("web.message.impersonated")}</span>
  </div>;
}

/**
 * What the platform is telling everybody: a maintenance window, or an
 * announcement an operator broadcast from the console.
 *
 * Above the chrome and not dismissible, like the impersonation banner beside
 * it, because both answer a question somebody is about to ask support: "why
 * can I not save this" and "what is happening tonight". A notice that can be
 * closed is a notice the next person does not see.
 */
function PlatformNotices({notices}:{notices?:Array<{kind:string;title:string;body:string}>}){
  if(!notices?.length) return null;
  const tone=(kind:string)=>kind==="maintenance"?"bg-slate-800 text-slate-100"
    :kind==="warning"?"bg-amber-100 text-amber-900 border-b border-amber-200"
    :"bg-blue-50 text-blue-900 border-b border-blue-100";
  return <>{notices.map((notice,index)=>
    <div key={index} role="status" className={`w-full text-sm px-4 py-2 ${tone(notice.kind)}`}>
      <strong className="font-medium">{notice.title}</strong>
      {notice.body&&<span className="ml-2 opacity-90">{notice.body}</span>}
    </div>)}</>;
}
