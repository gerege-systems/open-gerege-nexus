"use client";

import {useEffect,useMemo,useState} from "react";
import { useLoadOnMount } from "@/lib/useResource";
import {api} from "@/lib/api";
import {useI18n} from "@/lib/i18n";
import {Alert,Badge,Button,Card,Checkbox,EmptyState,IconButton,Input,Spinner,Tabs,TabsContent,TabsList,TabsTrigger,Textarea} from "@gerege-systems/ui";
import {Check,DoorOpen,Plus,Save,ShieldCheck,Trash2,Users,X} from "lucide-react";

type Role={id:string;code:string;name:string;description:string;active:boolean;system:boolean;permissions:string[]};
type Permission={code:string;name:string;description:string;app:string};
type Member={membership_id:string;user_id:string;name:string;email:string;is_admin:boolean;roles:string[]};
type JoinRequest={id:string;user_id:string;name:string;email:string;message:string;status:string;created_at:string};
type Tab="roles"|"members"|"requests";

export default function AccessSettingsPage(){
  const {t}=useI18n();
  const [roles,setRoles]=useState<Role[]>([]),[permissions,setPermissions]=useState<Permission[]>([]),[members,setMembers]=useState<Member[]>([]);
  const [queue,setQueue]=useState<JoinRequest[]>([]);
  const [selected,setSelected]=useState(""),[tab,setTab]=useState<Tab>("roles"),[loading,setLoading]=useState(true),[saving,setSaving]=useState(false),[error,setError]=useState(""),[notice,setNotice]=useState("");
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
  // to press by accident as the weakest. Two people on open.gerege.mn became
  // administrators of organisations they had just been let into, twenty
  // seconds after their requests were approved, and nothing on the screen had
  // asked whether that was meant.
  async function toggleMemberRole(member:Member,roleID:string){const role=roles.find(r=>r.id===roleID);const adding=!member.roles.includes(roleID);
    if(adding&&role?.code==="admin"&&!confirm(t("access.message.confirm_admin",{name:member.name||member.email})))return;
    const next=member.roles.includes(roleID)?member.roles.filter(id=>id!==roleID):[...member.roles,roleID];setMembers(all=>all.map(m=>m.membership_id===member.membership_id?{...m,roles:next}:m));try{await api.setMembershipRoles(member.membership_id,next);flash(t("access.message.member_updated"))}catch(e){setError(e instanceof Error?e.message:t("access.message.error_assign"));await load()}}

  if(loading)return <div className="p-8 flex items-center gap-2 text-sm text-muted" role="status"><Spinner size="md" decorative/>{t("access.message.loading")}</div>;
  return <Tabs value={tab} onValueChange={value=>setTab(value as Tab)} className="w-full space-y-6">
    <header className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between"><div><p className="text-xs font-semibold uppercase tracking-[.18em] text-accent">{t("access.view.eyebrow")}</p><h1 className="mt-1 flex items-center gap-2 text-2xl font-semibold text-foreground"><ShieldCheck className="w-6 h-6" aria-hidden="true"/>{t("access.view.title")}</h1><p className="mt-1 text-sm text-muted">{t("access.view.subtitle")}</p></div>
      <TabsList variant="pills"><TabsTrigger value="roles"><ShieldCheck aria-hidden="true"/>{t("access.view.tab_roles")}</TabsTrigger><TabsTrigger value="members"><Users aria-hidden="true"/>{t("access.view.tab_members")}</TabsTrigger><TabsTrigger value="requests"><DoorOpen aria-hidden="true"/>{t("access.view.tab_requests")}{queue.length>0&&<Badge variant="subtle" tone="accent" className="ms-1">{queue.length}</Badge>}</TabsTrigger></TabsList></header>
    {error&&<Alert variant="danger" live>{error}</Alert>}{notice&&<Alert variant="success" live>{notice}</Alert>}
    <TabsContent value="requests" className="mt-0"><Card asChild padding="none" className="overflow-hidden"><section>
      <div className="border-b border-line p-5"><h2 className="font-semibold">{t("access.view.requests_title")}</h2><p className="text-sm text-muted">{t("access.view.requests_hint")}</p></div>
      {queue.length===0
        ? <EmptyState icon={<DoorOpen className="size-6"/>} title={t("access.message.no_requests")} className="py-8"/>
        : <div className="divide-y divide-line">{queue.map(request=><div key={request.id} className="flex flex-wrap items-start justify-between gap-3 p-4">
            <div className="min-w-0"><strong className="block text-sm">{request.name}</strong><span className="text-xs text-muted">{request.email}</span>
              {/* Хүн юу гэж бичсэн нь шийдвэрийн гол мэдээлэл тул тайрахгүй. */}
              {request.message&&<p className="mt-1 text-sm text-foreground">{request.message}</p>}</div>
            <div className="flex gap-2">
              <Button size="sm" disabled={saving} leadingIcon={<Check/>} onClick={()=>void decide(request,true)}>{t("access.action.accept")}</Button>
              <Button size="sm" variant="outline" disabled={saving} leadingIcon={<X/>} onClick={()=>void decide(request,false)}>{t("access.action.decline")}</Button>
            </div></div>)}</div>}
    </section></Card></TabsContent>
    <TabsContent value="roles" className="mt-0"><div className="grid gap-5 xl:grid-cols-[280px_minmax(0,1fr)]">
      <aside className="space-y-3"><Card padding="none" className="p-3 space-y-2">{roles.map(role=><button type="button" key={role.id} aria-pressed={selected===role.id} onClick={()=>setSelected(role.id)} className={`w-full rounded-md border p-3 text-start ${selected===role.id?"border-accent bg-accent-soft":"border-transparent hover:bg-surface-2"}`}><span className="flex justify-between gap-2"><strong className="text-sm">{role.name}</strong><code className="text-xs text-muted">{role.code}</code></span><small className="mt-1 block text-xs text-muted">{t("access.message.role_summary",{count:role.permissions.length})} · {role.active?t("base.state.active"):t("base.state.inactive")}</small></button>)}</Card>
        <Card asChild padding="sm"><form onSubmit={e=>{e.preventDefault();void createRole()}} className="space-y-3"><h2 className="font-semibold text-sm">{t("access.view.create_role")}</h2><Input required label={t("access.field.code_placeholder")} hideLabel value={newRole.code} onChange={e=>setNewRole({...newRole,code:e.target.value})} placeholder={t("access.field.code_placeholder")}/><Input required label={t("access.field.name_placeholder")} hideLabel value={newRole.name} onChange={e=>setNewRole({...newRole,name:e.target.value})} placeholder={t("access.field.name_placeholder")}/><Textarea label={t("access.field.description_placeholder")} hideLabel value={newRole.description} onChange={e=>setNewRole({...newRole,description:e.target.value})} placeholder={t("access.field.description_placeholder")}/><Button type="submit" className="w-full" disabled={saving} leadingIcon={<Plus/>}>{t("access.action.create_role")}</Button></form></Card>
      </aside>
      <Card asChild padding="none" className="overflow-hidden"><section>{current&&<><div className="flex items-start justify-between gap-4 border-b border-line p-5"><div><h2 className="font-semibold text-lg">{current.name}</h2><p className="text-sm text-muted">{current.description||t("access.message.no_description")}</p></div><div className="flex gap-2">{!current.system&&<IconButton variant="outline" className="border-danger-border text-danger hover:bg-danger-soft" onClick={()=>void removeRole(current)} aria-label={t("base.action.delete")} icon={<Trash2/>}/>}<Button disabled={saving||current.code==="admin"} leadingIcon={<Save/>} onClick={()=>void savePermissions()}>{t("base.action.save")}</Button></div></div><div className="p-5 space-y-5">{current.code==="admin"&&<Alert variant="warning">{t("access.message.admin_note")}</Alert>}{grouped.map(([app,items])=><fieldset key={app}><legend className="mb-2 text-xs font-semibold uppercase tracking-wider text-muted">{app}</legend><div className="grid gap-2 md:grid-cols-2">{items.map(p=><Checkbox key={p.code} className={`rounded-md border p-3 ${draft.includes(p.code)?"border-accent bg-accent-soft":"border-line"}`} checked={current.code==="admin"||draft.includes(p.code)} disabled={current.code==="admin"} onCheckedChange={()=>togglePermission(p.code)} label={<span><strong className="block text-sm">{p.name}</strong><code className="text-xs text-muted">{p.code}</code></span>} description={p.description}/>)}</div></fieldset>)}</div></>}</section></Card>
    </div></TabsContent>
    <TabsContent value="members" className="mt-0"><Card asChild padding="none" className="overflow-hidden"><section><div className="border-b border-line p-5"><h2 className="font-semibold">{t("access.view.members_title")}</h2><p className="text-sm text-muted">{t("access.view.members_hint")}</p></div><div className="divide-y divide-line">{members.map(member=><div key={member.membership_id} className="grid gap-3 p-4 lg:grid-cols-[minmax(180px,1fr)_2fr]"><div><strong className="block text-sm">{member.name}</strong><span className="text-xs text-muted">{member.email}</span>{member.is_admin&&<Badge variant="subtle" tone="warning" className="ms-2">ADMIN</Badge>}</div><fieldset className="flex flex-wrap gap-2"><legend className="sr-only">{t("access.view.tab_roles")}</legend>{roles.filter(r=>r.active).map(role=><Checkbox key={role.id} className={`rounded-md border px-3 py-2 text-xs font-semibold ${member.roles.includes(role.id)?"border-accent bg-accent-soft text-accent":"border-line"}`} checked={member.roles.includes(role.id)} onCheckedChange={()=>void toggleMemberRole(member,role.id)} label={role.name}/>)}</fieldset></div>)}</div></section></Card></TabsContent>
  </Tabs>;
}
