"use client";

import React, { useCallback, useEffect, useId, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import {
  Alert,
  Button,
  CommandDialog,
  CommandInput,
  CommandItem,
  CommandList,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuTrigger,
  IconButton,
  Input,
  Sheet,
  SheetContent,
  SheetTitle,
  Sidebar,
  SidebarItem,
  Spinner,
  Tooltip,
  TopNav,
  cn,
} from "@gerege-systems/ui";
import { api, APP_MENU_CHANGED_EVENT } from "@/lib/api";
import { resetAccess } from "@/lib/access";
import { useI18n } from "@/lib/i18n";
import { useBrand } from "@/lib/brandContext";
import UserMenu from "@/components/UserMenu";
import { TenantChoices, forgetTenants, useTenants } from "@/components/TenantChoices";
import AICopilot from "@/components/AICopilot";
import { invokeShell, useShell, SHELL_EVENTS, SHELL_METHODS, type ShellNavigatePayload, type ShellSearchPayload } from "@/lib/shell";
import { currentDeviceLine, type DeviceLine } from "@/lib/deviceLine";
import { MenuIcon } from "@/lib/icons";
import { isPublicPath } from "@/lib/publicRoutes";
import { homeScreensVisible, organisationScreensVisible } from "@/lib/workspaceKind.mjs";
import { LayoutGrid, Settings, Menu as HamburgerIcon, Palette, Building2, Megaphone, Search, Ellipsis, ShieldCheck, RefreshCw, ChevronDown, ChevronsDownUp, ChevronsUpDown, ExternalLink, Inbox} from "lucide-react";

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

/**
 * The workspace shell, in the design system's "rail + panel" arrangement.
 *
 * A column of app tiles (the rail), beside it the panel of the current app's
 * groups (a library `Sidebar`), and over the content the `TopNav` with the
 * search and the account menu. Below `lg` the panel folds into a `Sheet`
 * drawer behind the hamburger, and the apps become a tab bar along the bottom
 * of the screen. The console (components/cp/Console) draws the same
 * arrangement from the same parts.
 */
export default function Layout({children}:{children:React.ReactNode}){
  const [menus,setMenus]=useState<MenuItem[]>([]),[user,setUser]=useState<any>(null),[loading,setLoading]=useState(true);
  const [mobileOpen,setMobileOpen]=useState(false),[mobileMoreOpen,setMobileMoreOpen]=useState(false),[panelOpen,setPanelOpen]=useState(true);
  const [query,setQuery]=useState("");
  const searchListId=useId();
  // The hamburger that opens the drawer — focus returns to it on close.
  const drawerTrigger=useRef<HTMLButtonElement>(null);
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
  const pathname=usePathname(),router=useRouter(),{t,locale}=useI18n(),brand=useBrand();
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
  const go=(path:string)=>{router.push(path);setQuery("");setShellSearchOpen(false)};

  function togglePanel(){if(window.matchMedia("(min-width:1024px)").matches){setPanelOpen(v=>{localStorage.setItem("gerege_sidebar_open",String(!v));return !v})}else setMobileOpen(v=>!v)}
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
  // The rail: the platform first, then every app that has a tile.
  const railTiles:Tile[]=[
    {id:"platform",href:"/apps",external:false,active:platformActive,label:t("web.label.platform"),icon:<LayoutGrid/>},
    ...railApps.map(app=>({id:app.id,href:app.path,external:!!app.externalUrl,active:selected?.id===app.id,label:app.name,icon:<MenuIcon name={app.icon} className="size-5"/>})),
  ];
  const mobileAppTabs:Tile[]=[
    // The platform tab is the way back out of an app on a phone, so it always
    // exists — it is where it goes that changes. The app store is the shelf a
    // company buys from; a home has nothing to buy, and the person's own record
    // is what they came back to the shell for.
    {...railTiles[0],href:company?"/apps":"/profile"},
    ...railTiles.slice(1),
  ];
  const hasMobileMore=mobileAppTabs.length>5;
  const primaryMobileTabs=hasMobileMore?mobileAppTabs.slice(0,4):mobileAppTabs;
  const remainingMobileTabs=hasMobileMore?mobileAppTabs.slice(4):[];

  if(isPublic)return <>{children}</>;
  if(loading)return <div role="status" className="flex min-h-dvh items-center justify-center gap-3 bg-surface-2 text-sm font-medium text-muted"><Spinner decorative/>{t("web.message.loading_platform")}</div>;

  const platformMenus=(onNavigate?:()=>void)=><><MenuGroup id={PLATFORM_GROUPS.modules} title={t("web.group.modules")} closed={closedGroups.includes(PLATFORM_GROUPS.modules)} onToggle={toggleGroup}>
    {/* The mirror of the two lines below: an organisation's screens are hidden
        in a home, and the home's own screen is hidden in an organisation. A
        member of a company asks for things through the company, so this list
        would be permanently empty for them — and an empty entry in a rail is a
        promise the screen behind it cannot keep. */}
    {ownHome&&<NavLink href="/me" active={pathname==="/me"} icon={<Inbox/>} label={t("web.menu.my_requests")} onNavigate={onNavigate}/>}
    {company&&<NavLink href="/apps" active={pathname==="/apps"} icon={<LayoutGrid/>} label={t("web.menu.app_store")} onNavigate={onNavigate}/>}
    {/* The organisation's own legal identity. A platform screen rather than a
        menu entry the organisation app contributes: it is read by the control
        plane, by the state registry rail and by an SSO consent screen, so it
        has to stay reachable on a tenant that has removed every app.
        Under Modules rather than Settings, because it is a thing you look at
        and edit — the organisation itself — not a switch that changes how the
        platform behaves. */}
    {company&&<NavLink href="/organisation" active={pathname==="/organisation"} icon={<Building2/>} label={t("web.menu.organisation")} onNavigate={onNavigate}/>}
    {/* What the organisation offers the public, beside what it is. It was a
        card at the bottom of the organisation's own screen, which is where a
        thing goes when nobody has decided it is a thing: publishing a service
        is an outward promise — a stranger finds it in the directory and asks —
        and it deserves the same standing in the menu as the identity above
        it. */}
    {company&&<NavLink href="/organisation/services" active={pathname==="/organisation/services"} icon={<Megaphone/>} label={t("core.view.services_title")} onNavigate={onNavigate}/>}
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
        icon={<MenuIcon name={item.icon} className="size-4"/>} label={item.label} onNavigate={onNavigate}/>)}
  </MenuGroup><MenuGroup id={PLATFORM_GROUPS.settings} title={t("web.group.settings")} closed={closedGroups.includes(PLATFORM_GROUPS.settings)} onToggle={toggleGroup}>
    {/* Under Settings, where its screen already lives: /settings/apps is what
        the address bar says, and a sidebar that files it under Modules asks
        somebody to hold two answers for where the same page is. */}
    {company&&<NavLink href="/settings/apps" active={pathname==="/settings/apps"} icon={<Settings/>} label={t("web.menu.installed_apps")} onNavigate={onNavigate}/>}
    <NavLink href="/settings/appearance" active={pathname==="/settings/appearance"} icon={<Palette/>} label={t("web.menu.appearance")} onNavigate={onNavigate}/>
    {chromeEntries("settings").map(item=>
      <NavLink key={item.id} href={item.path!} active={item.path===pathname}
        icon={<MenuIcon name={item.icon} className="size-4"/>} label={item.label} onNavigate={onNavigate}/>)}
    {/* Issuing a key that sends mail in the tenant's name is administrative, and
        the API behind this screen is admin-only, so the link follows it. */}
    {user?.is_admin&&<NavLink href="/settings/access" active={pathname==="/settings/access"} icon={<ShieldCheck/>} label={t("access.view.title")} onNavigate={onNavigate}/>}
  </MenuGroup></>;
  const panel=(onNavigate?:()=>void)=>selected
    ?<AppMenuGroups menus={selected.menus} pathname={pathname} closedGroups={closedGroups} onToggle={toggleGroup} onNavigate={onNavigate}/>
    :platformMenus(onNavigate);
  const panelIcon=selected?<MenuIcon name={selected.icon} className="size-5"/>:<LayoutGrid/>;
  const panelHeader=<PanelHeader title={brandTitle} icon={panelIcon} allOpen={allGroupsOpen} onToggleAll={visibleGroups.length>1?toggleAllGroups:undefined}/>;

  // Бүрхүүл дотор ба төхөөрөмжийн domain шугам дээр: толгой хэсэг ба мобайл
  // навигаци зурагдахгүй — хайлт, хэрэглэгч, нэвтрэлт, цонхны үйлдлүүдийг
  // native тал эзэмшинэ. Хажуугийн цэс нь ЭНД үлдэнэ: тэр бол ажлын мужийн
  // доторх навигаци бөгөөд аль апп идэвхтэй, ямар эрхтэй, ямар хэлээр гэдгийг
  // web тал аль хэдийн мэддэг.
  if(workAreaOnly)return <div className="flex h-dvh flex-col overflow-hidden bg-background text-foreground print:h-auto print:overflow-visible">
    <ImpersonationBanner active={!!user?.impersonated}/>
    <RibbonBar selected={selected} brandTitle={brandTitle} user={user} onSearch={()=>setShellSearchOpen(true)} onLogout={logout}/>
    <div className="flex min-h-0 flex-1 overflow-hidden">
      <AppRail tiles={railTiles} label={t("web.label.apps")} always/>
      <Sidebar aria-label={brandTitle} header={panelHeader} className="flex h-auto bg-chrome print:hidden [&>button:last-child]:hidden">{panel()}</Sidebar>
      <main className="relative min-h-0 min-w-0 flex-1 overflow-y-auto px-5 py-4 print:overflow-visible print:p-0">{children}</main>
    </div>
    <WorkareaFooter/>
    {/* Native толгой хэсгээс ирсэн хайлтын query-г харуулах давхарга. */}
    <CommandDialog open={shellSearchOpen} onOpenChange={open=>{setShellSearchOpen(open);if(!open)setQuery("")}} title={t("web.view.search_placeholder")}>
      <CommandInput value={query} onValueChange={setQuery} placeholder={t("web.view.search_placeholder")}/>
      <CommandList>
        {results.map(item=><CommandItem key={item.path} value={`${item.label} ${item.app}`} onSelect={()=>go(item.path)}>
          <span className="text-accent"><MenuIcon name={item.icon} className="size-4"/></span>
          <span className="min-w-0 flex-1 truncate">{item.label}</span>
          <span className="truncate text-xs text-muted">{item.app}</span>
        </CommandItem>)}
      </CommandList>
    </CommandDialog>
    <AICopilot className="top-10"/>
  </div>;

  // The frame is exactly one screen tall and only <main> scrolls, so the rail
  // stays where it is while a long page moves under the bar.
  return <div className="flex h-dvh flex-col overflow-hidden bg-background text-foreground print:h-auto print:overflow-visible">
    <ImpersonationBanner active={!!user?.impersonated}/>
    <PlatformNotices notices={user?.notices}/>
    <div className="flex min-h-0 flex-1">
      {/* The design system's dual shell: rail and panel run the full height on
          the left, the brand mark on the rail opening the organisation
          switcher, the organisation in the panel's header level with the top
          bar — and the top bar covers the content only. */}
      <AppRail tiles={railTiles} label={t("web.label.apps")}
        brand={<TenantSwitcher variant="mark" current={user?.tenant_id} currentName={user?.tenant_name}>
          {/* The mark used to arrive as a static import, which gave it a hashed,
              permanently cacheable URL and made it part of the build. A logo the
              build owns is a logo no deployment can change, so it is an address
              now — see lib/brand.ts. */}
          <img src={brand.logoUrl} width={32} height={32} alt={brand.name} className="size-8 shrink-0 rounded-md"/>
        </TenantSwitcher>}/>
      {/* The library's panel is `hidden md:flex`; this shell promotes the
          breakpoint to lg and serves a drawer below it. Beside a rail the
          panel is fixed-width, so its own collapse control is hidden — the
          rail already is the icon tier. */}
      <Sidebar aria-label={brandTitle}
        header={<TenantSwitcher variant="panel" current={user?.tenant_id} currentName={user?.tenant_name}><TenantMark/></TenantSwitcher>}
        className={cn("h-auto bg-chrome md:hidden print:hidden [&>button:last-child]:hidden",panelOpen&&"lg:flex")}>
        {panel()}
      </Sidebar>
      <div className="flex min-w-0 flex-1 flex-col">
        <TopNav
          className="shrink-0 bg-chrome print:hidden"
          logo={<div className="flex min-w-0 items-center gap-2">
            {/* Two controls for one function: below lg the button opens the
                drawer, from lg it folds the panel, and aria-expanded has to
                report the state of the thing it actually acts on. */}
            <IconButton ref={drawerTrigger} aria-label={t("web.action.toggle_menu")} aria-expanded={mobileOpen} icon={<HamburgerIcon/>} size="sm" className="lg:hidden" onClick={togglePanel}/>
            <IconButton aria-label={t("web.action.toggle_menu")} aria-expanded={panelOpen} icon={<HamburgerIcon/>} size="sm" className="hidden lg:inline-flex" onClick={togglePanel}/>
            {/* Below lg the panel — and the organisation in its header — is
                off screen, so the switcher stands in the bar. */}
            <span className="lg:hidden"><TenantSwitcher current={user?.tenant_id} currentName={user?.tenant_name}><TenantMark/></TenantSwitcher></span>
            {/* Which app the panel shows, named once: the panel lists only the
                screens. */}
            <span className="ms-1 hidden min-w-0 items-center gap-2 sm:flex">
              <span className="shrink-0 text-accent [&_svg]:size-5">{panelIcon}</span>
              <strong className="truncate text-sm text-foreground">{brandTitle}</strong>
            </span>
            {/* Beside the control that opens the panel, because it acts on
                what that panel contains. */}
            {visibleGroups.length>1&&<IconButton size="sm" className="hidden lg:inline-flex" onClick={toggleAllGroups} aria-expanded={allGroupsOpen}
              aria-label={allGroupsOpen?t("web.action.collapse_all"):t("web.action.expand_all")} title={allGroupsOpen?t("web.action.collapse_all"):t("web.action.expand_all")}
              icon={allGroupsOpen?<ChevronsDownUp/>:<ChevronsUpDown/>}/>}
          </div>}
          search={<div className="relative hidden md:block">
            {/* Enter picks the first row, so that row is the selected option and is drawn as such. */}
            <Input type="search" size="sm" label={t("base.action.search")} hideLabel placeholder={t("web.view.search_placeholder")}
              prefix={<Search className="size-4" aria-hidden/>}
              value={query} onChange={e=>setQuery(e.target.value)}
              onKeyDown={e=>{if(e.key==="Enter"&&results[0])go(results[0].path);if(e.key==="Escape")setQuery("")}}
              role="combobox" aria-expanded={results.length>0} aria-controls={searchListId} aria-autocomplete="list"/>
            {results.length>0&&<div id={searchListId} role="listbox" className="absolute inset-x-0 top-full z-dropdown mt-1 rounded-lg border border-line bg-surface p-1 shadow-md">
              {results.map((item,i)=><button key={item.path} type="button" role="option" aria-selected={i===0} onClick={()=>go(item.path)}
                className={cn("flex w-full items-center gap-3 rounded-md px-3 py-2 text-start outline-none hover:bg-surface-2 focus-visible:ring-2 focus-visible:ring-ring",i===0&&"bg-surface-2")}>
                <span className="shrink-0 text-accent"><MenuIcon name={item.icon} className="size-4"/></span>
                <span className="min-w-0"><strong className="block truncate text-sm font-medium text-foreground">{item.label}</strong><small className="block truncate text-xs text-muted">{item.app}</small></span>
              </button>)}
            </div>}
          </div>}
          actions={<>
            {/* Профайл нь аватарын цэсэн дотор. Толгой хэсэгт тусдаа товч байсныг
                авав: нэвтэрсний дараа хүн профайл дээрээ бууж ирдэг болсон тул тэр
                нь өөрөө хаана байгааг заасан хэрэг — хажууд нь бас байнга шахагдаж
                зогсох товч илүү. */}
            <AICopilot/>
            <UserMenu user={user} onLogout={logout}/>
          </>}
        />
        {/* `relative`, so an sr-only label deep in a long page resolves against
            this pane and not the document. Below lg the 4rem tab bar is fixed
            over the page, and the padding keeps the last row above it. */}
        <main className="relative min-h-0 min-w-0 flex-1 overflow-y-auto p-4 sm:p-6 lg:p-8 max-lg:pb-[calc(4rem+env(safe-area-inset-bottom)+1.5rem)] print:overflow-visible print:p-0">{children}</main>
      </div>
    </div>

    {/* Below lg the panel slides in as a drawer. Escape, the backdrop and the
        focus trap are the Sheet's; a destination chosen inside it closes it.
        The tab bar switches apps, so the drawer carries only the panel. */}
    <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
      <SheetContent side="left" showClose={false} aria-describedby={undefined} returnFocusTo={drawerTrigger} className="w-72 p-0">
        <SheetTitle className="sr-only">{brandTitle}</SheetTitle>
        <Sidebar aria-label={brandTitle} header={panelHeader} className="flex h-full w-full border-r-0 [&>button:last-child]:hidden">
          {panel(()=>setMobileOpen(false))}
        </Sidebar>
      </SheetContent>
    </Sheet>

    {/* The apps as a tab bar, at most four and a "more" that opens the rest. */}
    <Sheet open={mobileMoreOpen} onOpenChange={setMobileMoreOpen}>
      <SheetContent side="bottom" aria-describedby={undefined} className="max-h-[70dvh] overflow-y-auto pb-[calc(4rem+env(safe-area-inset-bottom)+1rem)]">
        <SheetTitle className="text-base">{t("web.view.more_apps")}</SheetTitle>
        <div className="grid grid-cols-2 gap-2">{remainingMobileTabs.map(tab=><MobileMoreApp key={tab.id} {...tab}/>)}</div>
      </SheetContent>
    </Sheet>
    <nav aria-label={t("web.label.apps")} className="fixed inset-x-0 bottom-0 z-sticky flex h-[calc(4rem+env(safe-area-inset-bottom))] border-t border-line bg-chrome pb-[env(safe-area-inset-bottom)] lg:hidden print:hidden">
      {primaryMobileTabs.map(tab=><MobileAppTab key={tab.id} {...tab}/>)}
      {hasMobileMore&&<button type="button" onClick={()=>setMobileMoreOpen(v=>!v)} aria-expanded={mobileMoreOpen} className={mobileTabClass(remainingMobileTabs.some(tab=>tab.active)||mobileMoreOpen)}><Ellipsis aria-hidden/><small>{t("web.action.more")}</small></button>}
    </nav>
  </div>;
}

/**
 * Ажлын мужийн толгойн мөр — зөвхөн бүрхүүл ба төхөөрөмжийн шугам дээр.
 *
 * Хөтчийн толгой хэсгийг орлоно: тэнд байсан брэнд, tenant, хайлт, хэрэглэгч
 * дөрвүүлээ энэ 2.5rem мөрөнд багтана. Native тал цонхны үйлдлүүд болон
 * нэвтрэлтийг өөрөө эзэмшдэг тул давхардуулах шаардлагагүй.
 */
function RibbonBar({selected,brandTitle,user,onSearch,onLogout}:{selected:AppNav|null;brandTitle:string;user:any;onSearch:()=>void;onLogout:()=>void}){
  const {t}=useI18n();
  const brand=useBrand();
  return <div className="z-sticky flex h-10 shrink-0 select-none items-center justify-between gap-3 border-b border-line bg-chrome px-4 text-xs print:hidden">
    <div className="flex min-w-0 items-center gap-2.5">
      <span className="shrink-0 text-accent [&_svg]:size-4">{selected?<MenuIcon name={selected.icon} className="size-4"/>:<LayoutGrid/>}</span>
      <div className="flex min-w-0 items-center gap-1.5">
        <span className="font-semibold text-foreground">{brand.name}</span>
        <span className="text-subtle">/</span>
        <span className="truncate font-semibold text-accent">{brandTitle}</span>
      </div>
      {user?.tenant_name&&<span className="hidden items-center gap-1.5 border-s border-line ps-3 md:inline-flex">
        <span aria-hidden className="size-1.5 rounded-full bg-success-solid"/>
        <span className="max-w-36 truncate font-medium text-muted">{user.tenant_name}</span>
      </span>}
    </div>
    <div className="flex shrink-0 items-center gap-1">
      <Button variant="ghost" size="sm" onClick={onSearch} className="text-muted"><Search aria-hidden/>{t("web.view.search_placeholder")}</Button>
      <IconButton aria-label={t("web.action.reload")} title={t("web.action.reload")} icon={<RefreshCw/>} size="sm" onClick={()=>window.location.reload()}/>
      <UserMenu user={user} onLogout={onLogout}/>
    </div>
  </div>;
}

/** Ажлын мужийн хөл. Native footer нь цонхны мөр — энэ нь ажлын мужийнх. */
function WorkareaFooter(){
  const brand=useBrand();
  return <footer className="z-sticky flex h-7 shrink-0 select-none items-center justify-between border-t border-line bg-chrome px-4 text-xs text-muted print:hidden">
    <span className="flex items-center gap-1.5 font-medium text-foreground">
      <span aria-hidden className="size-2 rounded-full bg-success-solid"/>
      <span>{brand.name}</span>
    </span>
    <span className="hidden items-center gap-1 sm:inline-flex">
      <ShieldCheck className="size-3.5 text-success" aria-hidden/>
      <span>TLS</span>
    </span>
  </footer>;
}

function TenantMark(){
  return <span className="grid size-8 shrink-0 place-items-center rounded-md bg-accent-soft text-accent"><Building2 className="size-4" aria-hidden/></span>;
}

/**
 * Which organisation this is, and the menu that changes it.
 *
 * Which tenant a session belonged to used to be decided once, by whichever
 * membership was oldest, and somebody who works for two organisations could
 * reach only the first — signing out and back in landed them in the same one
 * again. `mark` is the brand mark on the rail; `panel` fills the sidebar's
 * header, level with the top bar; `bar` is the compact trigger the top bar
 * shows below lg, where the panel is off screen. The account menu offers the
 * same list.
 */
function TenantSwitcher({current,currentName,variant="bar",children}:{current?:string;currentName?:string;variant?:"panel"|"bar"|"mark";children:React.ReactNode}){
  const {t}=useI18n();
  const [open,setOpen]=useState(false);
  const {tenants,activeIDs,switching,failed,switchTo,toggleActive}=useTenants(open);
  const label=currentName?`${currentName} — ${t("web.action.switch_tenant")}`:t("web.action.switch_tenant");
  const panel=variant==="panel",mark=variant==="mark";
  return <DropdownMenu open={open} onOpenChange={setOpen}>
    <DropdownMenuTrigger asChild>
      <button type="button" aria-label={label} title={label}
        className={cn("flex min-w-0 items-center gap-2 rounded-md text-start outline-none",
          panel?"h-10 w-full px-1.5":mark?"size-10 justify-center":"h-9 px-1.5",
          "transition-colors hover:bg-surface-2 data-[state=open]:bg-surface-2",
          "focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background")}>
        {children}
        {!mark&&<span className={cn("truncate text-sm font-semibold text-foreground",panel?"min-w-0 flex-1":"hidden max-w-56 md:block")}>{currentName||"Demo Tenant"}</span>}
        {!mark&&<ChevronsUpDown className={cn("size-4 shrink-0 text-subtle",!panel&&"hidden md:block")} aria-hidden/>}
      </button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="start" className="w-72">
      <DropdownMenuLabel>{t("web.view.tenants")}</DropdownMenuLabel>
      <TenantChoices current={current} tenants={tenants} activeIDs={activeIDs} switching={switching} failed={failed} onChoose={id=>void switchTo(id)} onStay={()=>setOpen(false)} onToggleActive={id=>void toggleActive(id,current||"")}/>
    </DropdownMenuContent>
  </DropdownMenu>;
}
// Leaving this product is stated, not implied: an external destination gets a
// new tab, an icon that says so, and rel="noopener" — the page it opens is
// somebody else's and must not be handed a reference back to this window.
function ExternalAnchor({href,className,children,...rest}:{href:string;className?:string;children:React.ReactNode}&React.AnchorHTMLAttributes<HTMLAnchorElement>){
  return <a href={href} target="_blank" rel="noopener noreferrer" className={className} {...rest}>{children}</a>;
}

interface Tile { id:string; href:string; external:boolean; active:boolean; label:string; icon:React.ReactNode }

/** One app's tile: an icon, named by its tooltip and for assistive tech. */
function RailTile({href,external,active,label,icon}:Tile){
  const classes=cn(
    "relative inline-flex size-10 shrink-0 items-center justify-center rounded-md outline-none [&_svg]:size-5",
    "transition-colors",
    "focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background",
    // Current = accent bar on the rail's edge + background + accent icon,
    // never colour alone.
    "before:absolute before:top-2 before:bottom-2 before:-left-2 before:w-0.5 before:rounded-r-full before:bg-accent before:opacity-0",
    active?"bg-accent-soft text-accent before:opacity-100":"text-muted hover:bg-surface-2 hover:text-foreground",
  );
  return <Tooltip label={label} side="right">
    {external
      ?<ExternalAnchor href={href} aria-label={label} className={classes}>{icon}</ExternalAnchor>
      :<Link href={href} aria-label={label} aria-current={active?"page":undefined} className={classes}>{icon}</Link>}
  </Tooltip>;
}

/**
 * Division one of the shell: the rail, a tile per app. The tiles scroll on
 * their own when there are more than fit a short laptop. `always` is the
 * workarea, which has no drawer to fold the rail into.
 */
function AppRail({tiles,label,always,brand}:{tiles:Tile[];label:string;always?:boolean;brand?:React.ReactNode}){
  return <nav aria-label={label} className={cn("w-14 shrink-0 flex-col items-center gap-1 border-r border-line bg-chrome py-2 print:hidden",always?"flex":"hidden lg:flex")}>
    {brand&&<div className="mb-1 flex h-10 shrink-0 items-center">{brand}</div>}
    <div className="flex min-h-0 w-full flex-1 flex-col items-center gap-1 overflow-y-auto py-0.5 [scrollbar-width:none]">
      {tiles.map(tile=><RailTile key={tile.id} {...tile}/>)}
    </div>
  </nav>;
}

/**
 * The drawer's heading: which app this is, and the control that folds or
 * opens every group in it.
 */
function PanelHeader({title,icon,allOpen,onToggleAll}:{title:string;icon:React.ReactNode;allOpen:boolean;onToggleAll?:()=>void}){
  const {t}=useI18n();
  return <div className="flex w-full min-w-0 items-center gap-2">
    <span className="shrink-0 text-accent [&_svg]:size-5">{icon}</span>
    <h2 className="min-w-0 flex-1 truncate text-sm font-semibold text-foreground">{title}</h2>
    {onToggleAll&&<IconButton size="sm" onClick={onToggleAll} aria-expanded={allOpen}
      aria-label={allOpen?t("web.action.collapse_all"):t("web.action.expand_all")} title={allOpen?t("web.action.collapse_all"):t("web.action.expand_all")}
      icon={allOpen?<ChevronsDownUp/>:<ChevronsUpDown/>}/>}
  </div>;
}

function mobileTabClass(active:boolean){
  return cn("flex min-w-0 flex-1 flex-col items-center justify-center gap-1 border-t-2 px-1 text-xs outline-none [&_svg]:size-5 [&_small]:max-w-full [&_small]:truncate [&_small]:text-xs",
    "transition-colors focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring",
    active?"border-accent font-medium text-accent":"border-transparent text-muted hover:text-foreground");
}
function MobileAppTab({href,external,active,label,icon}:Tile){
  const className=mobileTabClass(active);
  if(external)return <ExternalAnchor href={href} aria-label={label} className={className}>{icon}<small>{label}</small></ExternalAnchor>;
  return <Link href={href} aria-label={label} aria-current={active?"page":undefined} className={className}>{icon}<small>{label}</small></Link>;
}
function MobileMoreApp({href,external,active,label,icon}:Tile){
  const className=cn("flex min-w-0 items-center gap-2 rounded-md border px-3 py-2.5 text-sm outline-none [&_svg]:size-5",
    "focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background",
    active?"border-accent bg-accent-soft text-accent":"border-line text-foreground hover:bg-surface-2");
  const body=<><span className={cn("shrink-0",!active&&"text-muted")}>{icon}</span><strong className="truncate font-medium">{label}</strong></>;
  if(external)return <ExternalAnchor href={href} className={className}>{body}</ExternalAnchor>;
  return <Link href={href} aria-current={active?"page":undefined} className={className}>{body}</Link>;
}
/**
 * One destination. The library item wraps the router's Link so a click stays
 * a client-side navigation — the frame is a layout and must survive it.
 * Active = accent bar + background, never colour alone.
 */
function NavLink({href,active,icon,label,onNavigate}:{href:string;active:boolean;icon:React.ReactNode;label:string;onNavigate?:()=>void}){
  // The icon goes through the item's prop: with asChild the Link's own
  // children become the label, wrapped in an inline truncating span, so an
  // icon passed as a child breaks onto its own line.
  return <SidebarItem asChild active={active} tooltip={label} icon={icon}>
    <Link href={href} onClick={onNavigate} className={cn("relative before:absolute before:top-1.5 before:bottom-1.5 before:left-0 before:w-0.5 before:rounded-full before:bg-accent before:opacity-0",active&&"bg-accent-soft text-accent before:opacity-100")}>
      {label}
    </Link>
  </SidebarItem>;
}
// The same row as a NavLink, minus the highlight: an external destination has
// no path under this application, so nothing it opens can ever be "the page
// you are on".
function ExternalNavLink({href,icon,label}:{href:string;icon:React.ReactNode;label:string}){
  return <SidebarItem asChild tooltip={label} icon={icon} trailing={<ExternalLink className="size-3.5 shrink-0 opacity-60" aria-hidden/>}>
    <ExternalAnchor href={href}>{label}</ExternalAnchor>
  </SidebarItem>;
}
function MenuGroup({id,title,closed,onToggle,children}:{id:string;title:string;closed:boolean;onToggle:(id:string)=>void;children:React.ReactNode}){
  const bodyId=`menu-group-${id}`;
  return <section className="mb-3">
    {/* Still a heading, so the panel keeps its outline for a screen reader;
        the button inside is what the heading names, which is the pairing
        aria-expanded/aria-controls expects. */}
    <h3 className="mb-0.5 px-2">
      <button type="button" onClick={()=>onToggle(id)} aria-expanded={!closed} aria-controls={bodyId}
        className={cn("flex h-8 w-full items-center gap-1.5 rounded-md px-2 text-xs font-medium text-subtle outline-none",
          "transition-colors hover:bg-surface-2 hover:text-foreground",
          "focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background")}>
        <span className="min-w-0 flex-1 truncate text-start">{title}</span>
        <ChevronDown className={cn("size-3.5 shrink-0 transition-transform duration-200",!closed&&"rotate-180")} aria-hidden/>
      </button>
    </h3>
    {/* inert and not just hidden by overflow: a link folded away is still a
        link, and without this Tab would walk into a group the operator can
        see is shut and land focus somewhere off-screen. A 1fr → 0fr grid row
        rather than an animated height: the list is of unknown length and a
        height transition needs a number to travel to. */}
    <div id={bodyId} data-collapsed={closed} inert={closed} className="grid [grid-template-rows:1fr] transition-[grid-template-rows] duration-200 ease-out data-[collapsed=true]:[grid-template-rows:0fr] motion-reduce:transition-none">
      <ul className="flex min-h-0 flex-col gap-px overflow-hidden">{children}</ul>
    </div>
  </section>;
}
function AppMenuGroups({menus,pathname,closedGroups,onToggle,onNavigate}:{menus:MenuItem[];pathname:string;closedGroups:string[];onToggle:(id:string)=>void;onNavigate?:()=>void}){const roots=menus.filter(item=>!item.parent_id).sort((a,b)=>a.order-b.order);
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
  ?<NavLink key={item.id} href={item.path} active={item.path===here} icon={<MenuIcon name={item.icon} className="size-4"/>} label={item.label} onNavigate={onNavigate}/>
  :<ExternalNavLink key={item.id} href={item.external_url!} icon={<MenuIcon name={item.icon} className="size-4"/>} label={item.label}/>)}</MenuGroup>)}</>}

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
  return <Alert variant="warning" icon={<ShieldCheck className="mt-0.5 size-4 shrink-0" aria-hidden/>} role="status" className="shrink-0 rounded-none border-x-0 border-t-0 px-4 py-2 font-medium">{t("web.message.impersonated")}</Alert>;
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
  return <>{notices.map((notice,index)=>
    <Alert key={index} role="status" icon={false} variant={notice.kind==="warning"?"warning":notice.kind==="maintenance"?"default":"info"}
      className={cn("shrink-0 rounded-none border-x-0 border-t-0 px-4 py-2",notice.kind==="maintenance"&&"border-transparent bg-foreground text-background")}>
      <strong className="font-medium">{notice.title}</strong>
      {notice.body&&<span className="ms-2 opacity-90">{notice.body}</span>}
    </Alert>)}</>;
}
