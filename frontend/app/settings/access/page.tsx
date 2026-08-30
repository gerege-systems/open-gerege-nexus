"use client";

import {useEffect,useMemo,useState} from "react";
import { useLoadOnMount } from "@/lib/useResource";
import {api} from "@/lib/api";
import {useI18n} from "@/lib/i18n";
import {Check,DoorOpen,Plus,Save,ShieldCheck,Trash2,Users,X} from "lucide-react";

type Role={id:string;code:string;name:string;description:string;active:boolean;system:boolean;permissions:string[]};
type Permission={code:string;name:string;description:string;app:string};
type Member={membership_id:string;user_id:string;name:string;email:string;is_admin:boolean;roles:string[]};
type JoinRequest={id:string;user_id:string;name:string;email:string;message:string;status:string;created_at:string};

export default function AccessSettingsPage(){
  const {t}=useI18n();
  const [roles,setRoles]=useState<Role[]>([]),[permissions,setPermissions]=useState<Permission[]>([]),[members,setMembers]=useState<Member[]>([]);
  const [queue,setQueue]=useState<JoinRequest[]>([]);
  const [selected,setSelected]=useState(""),[tab,setTab]=useState<"roles"|"members"|"requests">("roles"),[loading,setLoading]=useState(true),[saving,setSaving]=useState(false),[error,setError]=useState(""),[notice,setNotice]=useState("");
  const [draft,setDraft]=useState<string[]>([]),[newRole,setNewRole]=useState({code:"",name:"",description:""});
  const current=roles.find(r=>r.id===selected);
  const grouped=useMemo(()=>Object.entries(permissions.reduce<Record<string,Permission[]>>((all,p)=>{(all[p.app]??=[]).push(p);return all},{})),[permissions]);

  async function load(){setLoading(true);setError("");try{const data=await api.getAccessOverview();setRoles(data.roles);setPermissions(data.permissions);setMembers(data.members);
    // Дараалал нь тусдаа хүсэлт: хоосон байх нь ердийн байдал тул түүний
    // алдаа энэ дэлгэцийг бүхэлд нь унагаах ёсгүй.
    void api.getJoinRequests().then(answer=>setQueue(answer.requests||[])).catch(()=>setQueue([]));setSelected(cur=>cur&&data.roles.some(r=>r.id===cur)?cur:(data.roles.find(r=>r.code==="manager")||data.roles[0])?.id||"")}catch(e){setError(e instanceof Error?e.message:t("access.message.error_load"))}finally{setLoading(false)}}
  useLoadOnMount(load);
  // Joined into one string so the dependency is a value the rule can check, and so
  // the draft is rebuilt when the role's permissions actually change rather than
  // whenever the array is rebuilt with the same contents.
  const currentPermissions=current?.permissions.join("|");
  useEffect(()=>{setDraft(currentPermissions?currentPermissions.split("|"):[])},[selected,currentPermissions]);

  function flash(message:string){setNotice(message);setTimeout(()=>setNotice(""),2500)}
  // Хариулсны дараа бүхэлд нь дахин ачаална: зөвшөөрөх нь гишүүн нэмдэг тул
  // «Гишүүд» таб мөн хуучирдаг — нэг мөрийг жагсаалтаас хасаад орхивол
  // дэлгэцийн хоёр хагас зөрнө.
  async function decide(request:JoinRequest,accept:boolean){setSaving(true);setError("");try{await api.decideJoinRequest(request.id,accept);await load();flash(t(accept?"access.message.request_accepted":"access.message.request_declined",{name:request.name}))}catch(e){setError(e instanceof Error?e.message:t("access.message.error_decide"))}finally{setSaving(false)}}
  function togglePermission(code:string){if(current?.code==="admin")return;setDraft(v=>v.includes(code)?v.filter(x=>x!==code):[...v,code])}
  async function savePermissions(){if(!current)return;setSaving(true);setError("");try{await api.setRolePermissions(current.id,draft);await load();flash(t("access.message.saved"))}catch(e){setError(e instanceof Error?e.message:t("access.message.error_save"))}finally{setSaving(false)}}
  async function createRole(){setSaving(true);setError("");try{const created=await api.createRole(newRole);setNewRole({code:"",name:"",description:""});await load();setSelected(created.id);flash(t("access.message.role_created"))}catch(e){setError(e instanceof Error?e.message:t("access.message.error_create"))}finally{setSaving(false)}}
  async function removeRole(role:Role){if(role.system||!confirm(t("access.message.confirm_delete",{name:role.name})))return;setSaving(true);try{await api.deleteRole(role.id);await load();flash(t("access.message.role_deleted"))}catch(e){setError(e instanceof Error?e.message:t("access.message.error_delete"))}finally{setSaving(false)}}
  // Granting `admin` is asked about; every other role is one click.
  //
  // These chips sit side by side and the strongest of them used to be as easy
  // to press by accident as the weakest. Two people on nexus.gerege.mn became
  // administrators of organisations they had just been let into, twenty
  // seconds after their requests were approved, and nothing on the screen had
  // asked whether that was meant.
  async function toggleMemberRole(member:Member,roleID:string){const role=roles.find(r=>r.id===roleID);const adding=!member.roles.includes(roleID);
    if(adding&&role?.code==="admin"&&!confirm(t("access.message.confirm_admin",{name:member.name||member.email})))return;
    const next=member.roles.includes(roleID)?member.roles.filter(id=>id!==roleID):[...member.roles,roleID];setMembers(all=>all.map(m=>m.membership_id===member.membership_id?{...m,roles:next}:m));try{await api.setMembershipRoles(member.membership_id,next);flash(t("access.message.member_updated"))}catch(e){setError(e instanceof Error?e.message:t("access.message.error_assign"));await load()}}

  if(loading)return <div className="p-8 text-slate-500">{t("access.message.loading")}</div>;
  return <div className="w-full space-y-6">
    <header className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between"><div><p className="text-xs font-bold uppercase tracking-[.18em] text-[var(--gerege-blue)]">{t("access.view.eyebrow")}</p><h1 className="mt-1 flex items-center gap-2 text-2xl font-bold text-slate-900"><ShieldCheck className="w-6 h-6"/>{t("access.view.title")}</h1><p className="mt-1 text-sm text-slate-500">{t("access.view.subtitle")}</p></div><div className="inline-flex rounded-xl border border-slate-200 bg-white p-1"><button onClick={()=>setTab("roles")} className={`px-4 py-2 rounded-lg text-sm font-semibold ${tab==="roles"?"bg-[var(--gerege-blue-soft)] text-[var(--gerege-blue)]":"text-slate-500"}`}><ShieldCheck className="inline w-4 h-4 mr-2"/>{t("access.view.tab_roles")}</button><button onClick={()=>setTab("members")} className={`px-4 py-2 rounded-lg text-sm font-semibold ${tab==="members"?"bg-[var(--gerege-blue-soft)] text-[var(--gerege-blue)]":"text-slate-500"}`}><Users className="inline w-4 h-4 mr-2"/>{t("access.view.tab_members")}</button><button onClick={()=>setTab("requests")} className={`px-4 py-2 rounded-lg text-sm font-semibold ${tab==="requests"?"bg-[var(--gerege-blue-soft)] text-[var(--gerege-blue)]":"text-slate-500"}`}><DoorOpen className="inline w-4 h-4 mr-2"/>{t("access.view.tab_requests")}{queue.length>0&&<span className="ml-2 rounded-full bg-[var(--gerege-blue)] px-2 py-0.5 text-[10px] font-bold text-[var(--gerege-on-blue)]">{queue.length}</span>}</button></div></header>
    {error&&<div className="rounded-xl border border-red-200 bg-red-50 p-3 text-sm text-red-700">{error}</div>}{notice&&<div className="rounded-xl border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700"><Check className="inline w-4 h-4 mr-2"/>{notice}</div>}
    {tab==="requests"?<section className="rounded-2xl border border-slate-200 bg-white overflow-hidden">
      <div className="border-b p-5"><h2 className="font-bold">{t("access.view.requests_title")}</h2><p className="text-sm text-slate-500">{t("access.view.requests_hint")}</p></div>
      {queue.length===0
        ? <p className="p-6 text-sm text-slate-500 text-center italic">{t("access.message.no_requests")}</p>
        : <div className="divide-y">{queue.map(request=><div key={request.id} className="flex flex-wrap items-start justify-between gap-3 p-4">
            <div className="min-w-0"><strong className="block text-sm">{request.name}</strong><span className="text-xs text-slate-500">{request.email}</span>
              {/* Хүн юу гэж бичсэн нь шийдвэрийн гол мэдээлэл тул тайрахгүй. */}
              {request.message&&<p className="mt-1 text-sm text-slate-700">{request.message}</p>}</div>
            <div className="flex gap-2">
              <button disabled={saving} onClick={()=>void decide(request,true)} className="rounded-lg bg-[var(--gerege-blue)] px-3 py-2 text-xs font-semibold text-[var(--gerege-on-blue)] disabled:opacity-50"><Check className="inline w-4 h-4 mr-1"/>{t("access.action.accept")}</button>
              <button disabled={saving} onClick={()=>void decide(request,false)} className="rounded-lg border border-slate-200 px-3 py-2 text-xs font-semibold text-slate-600 disabled:opacity-50"><X className="inline w-4 h-4 mr-1"/>{t("access.action.decline")}</button>
            </div></div>)}</div>}
    </section>:tab==="roles"?<div className="grid gap-5 xl:grid-cols-[280px_minmax(0,1fr)]">
      <aside className="space-y-3"><div className="rounded-2xl border border-slate-200 bg-white p-3 space-y-2">{roles.map(role=><button key={role.id} onClick={()=>setSelected(role.id)} className={`w-full rounded-xl border p-3 text-left ${selected===role.id?"border-[var(--gerege-blue)] bg-[var(--gerege-blue-soft)]":"border-transparent hover:bg-slate-50"}`}><span className="flex justify-between gap-2"><strong className="text-sm">{role.name}</strong><code className="text-[10px] text-slate-500">{role.code}</code></span><small className="mt-1 block text-slate-500">{t("access.message.role_summary",{count:role.permissions.length})} · {role.active?t("base.state.active"):t("base.state.inactive")}</small></button>)}</div>
        <form onSubmit={e=>{e.preventDefault();void createRole()}} className="rounded-2xl border border-slate-200 bg-white p-4 space-y-3"><h2 className="font-bold text-sm">{t("access.view.create_role")}</h2><input required value={newRole.code} onChange={e=>setNewRole({...newRole,code:e.target.value})} placeholder={t("access.field.code_placeholder")} className="w-full rounded-lg border px-3 py-2 text-sm"/><input required value={newRole.name} onChange={e=>setNewRole({...newRole,name:e.target.value})} placeholder={t("access.field.name_placeholder")} className="w-full rounded-lg border px-3 py-2 text-sm"/><textarea value={newRole.description} onChange={e=>setNewRole({...newRole,description:e.target.value})} placeholder={t("access.field.description_placeholder")} className="w-full rounded-lg border px-3 py-2 text-sm"/><button disabled={saving} className="w-full rounded-lg bg-[var(--gerege-blue)] px-3 py-2 text-sm font-semibold text-[var(--gerege-on-blue)]"><Plus className="inline w-4 h-4 mr-1"/>{t("access.action.create_role")}</button></form>
      </aside>
      <section className="rounded-2xl border border-slate-200 bg-white overflow-hidden">{current&&<><div className="flex items-start justify-between gap-4 border-b p-5"><div><h2 className="font-bold text-lg">{current.name}</h2><p className="text-sm text-slate-500">{current.description||t("access.message.no_description")}</p></div><div className="flex gap-2">{!current.system&&<button onClick={()=>void removeRole(current)} aria-label={t("base.action.delete")} className="rounded-lg border border-red-200 p-2 text-red-600"><Trash2 className="w-4 h-4"/></button>}<button disabled={saving||current.code==="admin"} onClick={()=>void savePermissions()} className="rounded-lg bg-[var(--gerege-blue)] px-4 py-2 text-sm font-semibold text-[var(--gerege-on-blue)] disabled:opacity-50"><Save className="inline w-4 h-4 mr-1"/>{t("base.action.save")}</button></div></div><div className="p-5 space-y-5">{current.code==="admin"&&<p className="rounded-xl bg-amber-50 border border-amber-200 p-3 text-sm text-amber-800">{t("access.message.admin_note")}</p>}{grouped.map(([app,items])=><fieldset key={app}><legend className="mb-2 text-xs font-bold uppercase tracking-wider text-slate-500">{app}</legend><div className="grid gap-2 md:grid-cols-2">{items.map(p=><label key={p.code} className={`flex gap-3 rounded-xl border p-3 ${draft.includes(p.code)?"border-[var(--gerege-blue)] bg-[var(--gerege-blue-soft)]":"border-slate-200"}`}><input type="checkbox" checked={current.code==="admin"||draft.includes(p.code)} disabled={current.code==="admin"} onChange={()=>togglePermission(p.code)} className="mt-1 h-4 w-4 accent-blue-600"/><span><strong className="block text-sm">{p.name}</strong><code className="text-[10px] text-slate-500">{p.code}</code><small className="block mt-1 text-slate-500">{p.description}</small></span></label>)}</div></fieldset>)}</div></>}</section>
    </div>:<section className="rounded-2xl border border-slate-200 bg-white overflow-hidden"><div className="border-b p-5"><h2 className="font-bold">{t("access.view.members_title")}</h2><p className="text-sm text-slate-500">{t("access.view.members_hint")}</p></div><div className="divide-y">{members.map(member=><div key={member.membership_id} className="grid gap-3 p-4 lg:grid-cols-[minmax(180px,1fr)_2fr]"><div><strong className="block text-sm">{member.name}</strong><span className="text-xs text-slate-500">{member.email}</span>{member.is_admin&&<span className="ml-2 rounded bg-amber-100 px-2 py-0.5 text-[10px] font-bold text-amber-800">ADMIN</span>}</div><div className="flex flex-wrap gap-2">{roles.filter(r=>r.active).map(role=><label key={role.id} className={`rounded-lg border px-3 py-2 text-xs font-semibold cursor-pointer ${member.roles.includes(role.id)?"border-[var(--gerege-blue)] bg-[var(--gerege-blue-soft)] text-[var(--gerege-blue)]":"border-slate-200"}`}><input className="sr-only" type="checkbox" checked={member.roles.includes(role.id)} onChange={()=>void toggleMemberRole(member,role.id)}/>{role.name}</label>)}</div></div>)}</div></section>}
  </div>;
}
