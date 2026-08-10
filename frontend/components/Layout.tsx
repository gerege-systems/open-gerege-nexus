"use client";

import React, { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import brandLogo from "@/public/brand.webp";
import { usePathname, useRouter } from "next/navigation";
import { api, APP_MENU_CHANGED_EVENT } from "@/lib/api";
import { resetAccess } from "@/lib/access";
import { useI18n } from "@/lib/i18n";
import { useTheme } from "@/lib/theme";
import UserMenu from "@/components/UserMenu";
import AICopilot from "@/components/AICopilot";
import { Landmark, LayoutGrid, Settings, Users, Package, Boxes, Share2, CreditCard, FileText, Code2, Menu as MenuIcon, Palette, Building2, BrainCircuit, Search, Ellipsis, ShieldCheck, PenTool, ScrollText, Layers, Move, ServerCog, Activity, Copy, Upload, Tags, BadgeDollarSign, Ruler, Sliders, Percent, ArrowRightLeft, RefreshCw, Warehouse, Route, Calculator, Wallet, ChartColumn, ListOrdered, Receipt, ListChecks, Files, Workflow, Archive, KeyRound, Webhook, Inbox, CalendarClock, Timer, MailCheck, ChevronDown, ChevronsDownUp, ChevronsUpDown } from "lucide-react";

interface MenuItem { id:string; app_id?:string; app_name?:string; parent_id?:string; label:string; path?:string; icon:string; order:number }
interface AppNav { id:string; name:string; icon:string; path:string; menus:MenuItem[] }

// Every icon the server can name in a menu definition. A name missing here
// falls back to a generic box, which is why the sub-menus under an app used to
// render as a column of identical squares — the blueprint icons in
// platform/menu were never mapped.
const iconMap: Record<string, React.ReactNode> = {
  users:<Users className="w-5 h-5"/>, package:<Package className="w-5 h-5"/>, boxes:<Boxes className="w-5 h-5"/>,
  "credit-card":<CreditCard className="w-5 h-5"/>, "file-text":<FileText className="w-5 h-5"/>, code:<Code2 className="w-5 h-5"/>, landmark:<Landmark className="w-5 h-5"/>,
  "pen-tool":<PenTool className="w-5 h-5"/>, settings:<Settings className="w-5 h-5"/>,
  "mail-check":<MailCheck className="w-5 h-5"/>,
  // esign
  "scroll-text":<ScrollText className="w-5 h-5"/>, layers:<Layers className="w-5 h-5"/>,
  move:<Move className="w-5 h-5"/>, "server-cog":<ServerCog className="w-5 h-5"/>,
  "shield-check":<ShieldCheck className="w-5 h-5"/>,
  // contacts
  activity:<Activity className="w-5 h-5"/>, copy:<Copy className="w-5 h-5"/>, upload:<Upload className="w-5 h-5"/>,
  // products
  tags:<Tags className="w-5 h-5"/>, "badge-dollar-sign":<BadgeDollarSign className="w-5 h-5"/>,
  ruler:<Ruler className="w-5 h-5"/>, sliders:<Sliders className="w-5 h-5"/>, percent:<Percent className="w-5 h-5"/>,
  // inventory
  "arrow-right-left":<ArrowRightLeft className="w-5 h-5"/>, "refresh-cw":<RefreshCw className="w-5 h-5"/>,
  warehouse:<Warehouse className="w-5 h-5"/>, route:<Route className="w-5 h-5"/>, calculator:<Calculator className="w-5 h-5"/>,
  // billing
  wallet:<Wallet className="w-5 h-5"/>, "chart-column":<ChartColumn className="w-5 h-5"/>,
  "list-ordered":<ListOrdered className="w-5 h-5"/>, receipt:<Receipt className="w-5 h-5"/>,
  // documents
  "list-checks":<ListChecks className="w-5 h-5"/>, files:<Files className="w-5 h-5"/>,
  workflow:<Workflow className="w-5 h-5"/>, archive:<Archive className="w-5 h-5"/>,
  // developer portal
  "key-round":<KeyRound className="w-5 h-5"/>, webhook:<Webhook className="w-5 h-5"/>,
  // gov services
  inbox:<Inbox className="w-5 h-5"/>, "calendar-clock":<CalendarClock className="w-5 h-5"/>, timer:<Timer className="w-5 h-5"/>,
};
const PUBLIC_ROUTES=["/","/login","/auth/eid/callback"];
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

const APP_ORDER=["io.example.contacts","io.example.products","io.example.inventory","io.example.billing","io.example.documents","io.example.esign","io.example.developer_portal","io.example.gov_services"];

export default function Layout({children}:{children:React.ReactNode}){
  const [menus,setMenus]=useState<MenuItem[]>([]),[user,setUser]=useState<any>(null),[loading,setLoading]=useState(true);
  const [mobileOpen,setMobileOpen]=useState(false),[mobileMoreOpen,setMobileMoreOpen]=useState(false),[panelOpen,setPanelOpen]=useState(true);
  const [query,setQuery]=useState("");
  // Which groups are shut, not which are open. A newly installed app arrives
  // with ids nobody has an opinion about yet, and the useful default for those
  // is the behaviour before this existed: open.
  const [closedGroups,setClosedGroups]=useState<string[]>([]);
  const pathname=usePathname(),router=useRouter(),{t,locale}=useI18n(),theme=useTheme();
  const isPublic=PUBLIC_ROUTES.includes(pathname);

  useEffect(()=>setPanelOpen(localStorage.getItem("gerege_sidebar_open")!=="false"),[]);
  useEffect(()=>{try{const saved=JSON.parse(localStorage.getItem(GROUPS_KEY)||"[]");if(Array.isArray(saved))setClosedGroups(saved.filter(id=>typeof id==="string"))}catch{/* hand-edited or half-written storage is not worth a crashed shell */}},[]);
  useEffect(()=>{if(isPublic){setLoading(false);return}void(async()=>{try{const [u,m]=await Promise.all([api.getMe(),api.getMenus()]);setUser(u);setMenus(m||[])}catch{router.push("/login")}finally{setLoading(false)}})()},[pathname,router,isPublic,locale]);
  useEffect(()=>{
    if(isPublic)return;
    const refreshMenus=()=>{void api.getMenus().then(m=>setMenus(m||[])).catch(()=>{})};
    window.addEventListener(APP_MENU_CHANGED_EVENT,refreshMenus);
    return()=>window.removeEventListener(APP_MENU_CHANGED_EVENT,refreshMenus);
  },[isPublic,locale]);
  useEffect(()=>{setMobileOpen(false);setMobileMoreOpen(false)},[pathname]);

  const apps=useMemo<AppNav[]>(()=>{
    const groups=new Map<string,MenuItem[]>();
    menus.filter(m=>m.app_id).forEach(m=>groups.set(m.app_id!,[...(groups.get(m.app_id!)||[]),m]));
    return [...groups.entries()].map(([id,items])=>{const sorted=items.sort((a,b)=>a.order-b.order),first=sorted.find(item=>item.path)!;return{id,name:first.label||first.app_name||id,icon:first.icon,path:first.path!,menus:sorted}}).sort((a,b)=>{const ai=APP_ORDER.indexOf(a.id),bi=APP_ORDER.indexOf(b.id);return (ai<0?999:ai)-(bi<0?999:bi)||a.id.localeCompare(b.id)});
  },[menus]);
  const selected=apps.find(app=>app.menus.some(m=>m.path&&isUnder(pathname,m.path)))||null;
  const platformActive=!selected;
  const searchIndex=useMemo(()=>[
    {label:t("web.menu.app_store"),app:t("web.label.platform"),path:"/apps",icon:"grid"},
    {label:t("web.menu.appearance"),app:t("web.label.platform"),path:"/settings/appearance",icon:"palette"},
    {label:t("web.menu.installed_apps"),app:t("web.label.platform"),path:"/settings/apps",icon:"settings"},
    {label:t("web.menu.email_verification"),app:t("web.label.platform"),path:"/settings/email-verification",icon:"mail-check"},
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
  async function logout(){try{await api.logout()}catch{}resetAccess();router.push("/login")}
  const brandTitle=selected?.name||(t("web.label.platform"));
  const mobileAppTabs=[
    {id:"platform",href:"/apps",active:platformActive,label:t("web.label.platform"),icon:<LayoutGrid className="w-5 h-5"/>},
    ...apps.map(app=>({id:app.id,href:app.path,active:selected?.id===app.id,label:app.name,icon:iconMap[app.icon]||<Package className="w-5 h-5"/>})),
  ];
  const hasMobileMore=mobileAppTabs.length>5;
  const primaryMobileTabs=hasMobileMore?mobileAppTabs.slice(0,4):mobileAppTabs;
  const remainingMobileTabs=hasMobileMore?mobileAppTabs.slice(4):[];

  if(isPublic)return <>{children}</>;
  if(loading)return <div className="min-h-screen flex items-center justify-center bg-slate-50 text-slate-500 font-medium">{t("web.message.loading_platform")}</div>;

  const platformMenus=<><MenuGroup id={PLATFORM_GROUPS.modules} title={t("web.group.modules")} closed={closedGroups.includes(PLATFORM_GROUPS.modules)} onToggle={toggleGroup}>
    <NavLink href="/apps" active={pathname==="/apps"} icon={<LayoutGrid className="w-5 h-5"/>} label={t("web.menu.app_store")}/><NavLink href="/settings/apps" active={pathname==="/settings/apps"} icon={<Settings className="w-5 h-5"/>} label={t("web.menu.installed_apps")}/><NavLink href="/settings/ai" active={pathname==="/settings/ai"} icon={<BrainCircuit className="w-5 h-5"/>} label={t("web.menu.ai_settings")}/>
  </MenuGroup><MenuGroup id={PLATFORM_GROUPS.settings} title={t("web.group.settings")} closed={closedGroups.includes(PLATFORM_GROUPS.settings)} onToggle={toggleGroup}>
    <NavLink href="/settings/appearance" active={pathname==="/settings/appearance"} icon={<Palette className="w-5 h-5"/>} label={t("web.menu.appearance")}/>
    <NavLink href="/settings/integrations" active={pathname==="/settings/integrations"} icon={<Share2 className="w-5 h-5"/>} label={t("web.menu.integrations")}/>
    {/* Issuing a key that sends mail in the tenant's name is administrative, and
        the API behind this screen is admin-only, so the link follows it. */}
    {user?.is_admin&&<NavLink href="/settings/email-verification" active={pathname==="/settings/email-verification"} icon={<MailCheck className="w-5 h-5"/>} label={t("web.menu.email_verification")}/>}
    {user?.is_admin&&<NavLink href="/settings/access" active={pathname==="/settings/access"} icon={<ShieldCheck className="w-5 h-5"/>} label={t("access.view.title")}/>}
  </MenuGroup></>;

  return <div className="gerege-shell min-h-screen flex flex-col">
    <header className="gerege-topbar h-16 flex items-center border-b sticky top-0 z-50">
      <Link href="/apps" className="gerege-header-brand w-16 h-full shrink-0 grid place-items-center border-r border-[var(--gerege-chrome-border)]">
        {theme.design==="gerege"?<img src={brandLogo.src} width={36} height={36} alt="Gerege Nexus" className="w-9 h-9 rounded-lg shadow-sm"/>:<span className="original-brand-mark w-9 h-9 rounded-lg grid place-items-center"><Building2 className="w-6 h-6"/></span>}
      </Link>
      <div className={`gerege-header-context h-full flex items-center gap-3 overflow-hidden transition-all duration-200 ${panelOpen?"is-open":""}`}>
        <span className="shrink-0 text-[var(--gerege-blue)]">{selected?(iconMap[selected.icon]||<Package className="w-5 h-5"/>):<LayoutGrid className="w-5 h-5"/>}</span>
        <span className="min-w-0"><small className="block text-[11px] leading-4 text-slate-500 truncate">Gerege Nexus</small><strong className="block text-[15px] leading-5 text-slate-900 truncate">{brandTitle}</strong></span>
      </div>
      <div className="gerege-menu-toggle w-16 h-full shrink-0 grid place-items-center"><button onClick={togglePanel} className="grid place-items-center w-10 h-10 rounded-lg text-slate-600 hover:bg-slate-50" aria-label={t("web.action.toggle_menu")} aria-expanded={mobileOpen}><MenuIcon className="w-5 h-5"/></button></div>
      <div className="hidden lg:flex items-center gap-2 px-4 min-w-0"><span className="gerege-session-dot w-2 h-2 rounded-full shrink-0"/><strong className="text-base text-slate-800 font-semibold truncate max-w-56">{user?.tenant_name||"Demo Tenant"}</strong></div>
      <div className="gerege-header-search hidden md:flex flex-1 items-center justify-center min-w-0 px-5 relative">
        <div className="relative w-full max-w-md"><Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400"/><input value={query} onChange={e=>setQuery(e.target.value)} onKeyDown={e=>{if(e.key==="Enter"&&results[0]){router.push(results[0].path);setQuery("")}}} placeholder={t("web.view.search_placeholder")} className="w-full h-10 rounded-full border border-slate-200 bg-slate-100/80 pl-10 pr-4 text-sm outline-none focus:border-[var(--gerege-blue)] focus:ring-2 focus:ring-[color-mix(in_srgb,var(--gerege-blue)_15%,transparent)]"/>
          {results.length>0&&<div className="gerege-topbar-onlight absolute top-12 inset-x-0 bg-white border border-slate-200 rounded-xl shadow-xl p-1.5 z-[70]">{results.map(item=><button key={item.path} onClick={()=>{router.push(item.path);setQuery("")}} className="w-full flex items-center gap-3 rounded-lg px-3 py-2.5 text-left hover:bg-[var(--gerege-surface-2)]"><span className="text-[var(--gerege-blue)]">{iconMap[item.icon]||<Search className="w-4 h-4"/>}</span><span className="min-w-0"><strong className="block text-sm truncate">{item.label}</strong><small className="text-slate-500 truncate">{item.app}</small></span></button>)}</div>}
        </div>
      </div>
      <div className="gerege-header-user flex pr-2 sm:pr-4 lg:pr-6"><UserMenu user={user} onLogout={logout}/></div>
    </header>

    <div className="flex flex-1 min-h-0">
      {mobileOpen&&<button className="gerege-mobile-backdrop fixed inset-0 top-16 bg-slate-950/40 z-30" aria-label={t("web.action.close_menu")} onClick={()=>setMobileOpen(false)}/>}
      <div className={`gerege-sidebar top-16 bottom-0 left-0 z-40 flex overflow-hidden ${mobileOpen?"is-mobile-open":""} ${panelOpen?"is-desktop-open":""}`}>
        <nav className="w-16 min-w-16 shrink-0 py-3 flex flex-col items-center gap-2 border-r border-[var(--gerege-border)]">
          <AppRailLink href="/apps" active={platformActive} title={t("web.label.platform")} icon={<LayoutGrid className="w-5 h-5"/>}/>
          {apps.map(app=><AppRailLink key={app.id} href={app.path} active={selected?.id===app.id} title={app.name} icon={iconMap[app.icon]||<Package className="w-5 h-5"/>}/>) }
        </nav>
        <aside className="gerege-menu-panel overflow-hidden">
          <div className="w-56 py-4">
            {visibleGroups.length>1&&<div className="px-2 pb-2 flex">
              <button type="button" onClick={toggleAllGroups} aria-expanded={allGroupsOpen} className="ml-auto flex items-center gap-1 px-2 py-1 rounded-md text-[11px] font-bold uppercase tracking-wider text-slate-400 hover:text-slate-600 hover:bg-[var(--gerege-surface-2)] transition">
                {allGroupsOpen?<ChevronsDownUp className="w-3.5 h-3.5"/>:<ChevronsUpDown className="w-3.5 h-3.5"/>}
                {allGroupsOpen?t("web.action.collapse_all"):t("web.action.expand_all")}
              </button>
            </div>}
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
    <AICopilot/>
  </div>;
}

function AppRailLink({href,active,title,icon}:{href:string;active:boolean;title:string;icon:React.ReactNode}){return <Link href={href} title={title} aria-label={title} className={`w-11 h-11 rounded-xl grid place-items-center transition ${active?"bg-[var(--gerege-blue-soft)] text-[var(--gerege-blue)] shadow-sm":"text-slate-500 hover:bg-[var(--gerege-surface-2)] hover:text-slate-800"}`}>{icon}</Link>}
function MobileAppTab({href,active,label,icon}:{href:string;active:boolean;label:string;icon:React.ReactNode}){return <Link href={href} aria-label={label} aria-current={active?"page":undefined} className={`gerege-mobile-tab ${active?"is-active":""}`}><span>{icon}</span><small>{label}</small></Link>}
function MobileMoreApp({href,active,label,icon}:{href:string;active:boolean;label:string;icon:React.ReactNode}){return <Link href={href} aria-current={active?"page":undefined} className={`gerege-mobile-more-app ${active?"is-active":""}`}><span>{icon}</span><strong>{label}</strong></Link>}
function NavLink({href,active,icon,label}:{href:string;active:boolean;icon:React.ReactNode;label:string}){return <Link href={href} className={`gerege-nav-link flex items-center gap-3 px-3 py-2.5 text-sm font-medium transition ${active?"gerege-nav-link-active font-semibold":""}`}><span className="gerege-nav-icon">{icon}</span><span>{label}</span></Link>}
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
function AppMenuGroups({menus,pathname,closedGroups,onToggle}:{menus:MenuItem[];pathname:string;closedGroups:string[];onToggle:(id:string)=>void}){const roots=menus.filter(item=>!item.parent_id).sort((a,b)=>a.order-b.order);return <>{roots.map(root=><MenuGroup key={root.id} id={root.id} title={root.label} closed={closedGroups.includes(root.id)} onToggle={onToggle}>{menus.filter(item=>item.parent_id===root.id&&item.path).sort((a,b)=>a.order-b.order).map(item=><NavLink key={item.id} href={item.path!} active={isUnder(pathname,item.path!)} icon={iconMap[item.icon]||<Package className="w-5 h-5"/>} label={item.label}/>)}</MenuGroup>)}</>}
