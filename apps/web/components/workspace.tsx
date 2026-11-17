"use client";

import {FormEvent,useCallback,useEffect,useMemo,useState} from "react";
import {
 ArrowDownRight,ArrowRight,ArrowUpRight,Building2,
 Check,CheckCircle2,ClipboardCheck,ClipboardList,FileText,
 Image as ImageIcon,LayoutDashboard,LogOut,Menu,
 Plus,Search,Settings,Shield,ShieldCheck,Upload,Users,X,AlertCircle,Loader2,
 MapPin,RefreshCw,House,Camera,ExternalLink,FolderOpen,
} from "lucide-react";
import {Button} from "@/components/ui/button";
import {Badge} from "@/components/ui/badge";
import {Input} from "@/components/ui/input";
import {Textarea} from "@/components/ui/textarea";
import {Card} from "@/components/ui/card";
import {Dialog,DialogContent,DialogDescription,DialogTitle} from "@/components/ui/dialog";
import {Select,SelectTrigger,SelectValue,SelectContent,SelectItem} from "@/components/ui/select";
import {api,endpoints,get,patch,post} from "@/lib/api";
import type {Contact,Inspection,InspectionItem,InspectionStatus,InspectionType,Job,Media,Member,Organization,Profile,Property,ItemStatus,Invitation} from "@/lib/types";
import {statusNames,typeNames} from "@/lib/types";
import {cn} from "@/lib/utils";

type Section="overview"|"properties"|"inspections"|"team"|"contacts"|"reports"|"settings";
type DialogKind="organization"|"property"|"inspection"|"item"|"member"|"contact"|null;
type LoadingMap={orgs:boolean;workspace:boolean;detail:boolean};
const sections:{id:Section;label:string;icon:typeof LayoutDashboard}[]=[
 {id:"overview",label:"Overview",icon:LayoutDashboard},
 {id:"properties",label:"Properties",icon:Building2},
 {id:"inspections",label:"Inspections",icon:ClipboardCheck},
 {id:"team",label:"Team members",icon:Users},
 {id:"contacts",label:"Contacts",icon:FolderOpen},
 {id:"reports",label:"Reports & AI",icon:FileText},
 {id:"settings",label:"Settings",icon:Settings},
];
function humanDate(value:string){const date=new Date(value);return Number.isFinite(date.getTime())?date.toLocaleDateString("en-GB",{day:"numeric",month:"short",year:"numeric"}):"—";}
function statusTone(status:string):"green"|"amber"|"red"|"slate"|"blue"{
 if(["completed","resolved","succeeded","uploaded"].includes(status))return "green";
 if(["pending","draft","open"].includes(status))return "amber";
 if(["failed","cancelled","critical","rejected"].includes(status))return "red";
 if(["running","in_progress"].includes(status))return "blue";
 return "slate";
}
function prettyStatus(value:string){return value.replaceAll("_"," ");}
function SectionHeading({eyebrow,title,description,action}:{eyebrow:string;title:string;description:string;action?:React.ReactNode}){
 return <div className="mb-7 flex flex-wrap items-end justify-between gap-4"><div><p className="mb-2 text-xs font-bold uppercase tracking-[.17em] text-blue-600">{eyebrow}</p><h1 className="text-2xl font-bold tracking-tight text-[#12223d] sm:text-[30px]">{title}</h1><p className="mt-2 text-sm text-slate-500">{description}</p></div>{action}</div>;
}
function Empty({icon:Icon,title,description,action}:{icon:typeof Building2;title:string;description:string;action?:React.ReactNode}){
 return <div className="flex min-h-64 flex-col items-center justify-center px-6 py-12 text-center"><div className="mb-4 flex size-14 items-center justify-center rounded-2xl bg-blue-50 text-blue-600"><Icon className="size-6"/></div><h3 className="text-base font-semibold text-slate-900">{title}</h3><p className="mt-2 max-w-xs text-sm leading-relaxed text-slate-500">{description}</p>{action&&<div className="mt-5">{action}</div>}</div>;
}
function Loader(){return <div className="flex min-h-56 items-center justify-center gap-3 text-sm text-slate-500"><Loader2 className="size-5 animate-spin text-blue-600"/>Loading live data…</div>;}
function Field({name,children}:{name:string;children:React.ReactNode}){return <label className="block space-y-2"><span className="text-sm font-semibold text-slate-700">{name}</span>{children}</label>;}
function Stat({icon:Icon,label,value,caption}:{icon:typeof Building2;label:string;value:number;caption:string}){
 return <Card className="p-5 sm:p-6"><div className="flex items-center justify-between"><p className="text-sm font-semibold text-slate-500">{label}</p><div className="flex size-10 items-center justify-center rounded-xl bg-blue-50 text-blue-600"><Icon className="size-5"/></div></div><p className="mt-4 text-3xl font-bold tracking-tight text-slate-900">{value.toLocaleString()}</p><p className="mt-2 text-xs text-slate-400">{caption}</p></Card>;
}
function InspectionRows({items,onSelect}:{items:Inspection[];onSelect:(id:string)=>void}){
 return <div className="divide-y divide-slate-100">{items.map(item=><button type="button" key={item.id} onClick={()=>onSelect(item.id)} className="grid w-full grid-cols-[1fr_auto] items-center gap-4 px-5 py-4 text-left transition-colors hover:bg-slate-50 sm:grid-cols-[minmax(0,1.1fr)_minmax(0,.8fr)_minmax(0,.6fr)_auto]">
  <div className="flex min-w-0 items-center gap-3"><span className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-blue-50 text-blue-600"><ClipboardList className="size-5"/></span><div className="min-w-0"><p className="truncate text-sm font-semibold text-slate-800">{item.property_name}</p><p className="mt-1 text-xs text-slate-400">{typeNames[item.type]}</p></div></div>
  <span className="hidden text-sm text-slate-500 sm:block">{humanDate(item.created_at)}</span>
  <span className="hidden text-sm text-slate-500 sm:block">{item.open_items} open / {item.total_items} total</span>
  <div className="flex items-center gap-2"><Badge tone={statusTone(item.status)}>{statusNames[item.status]}</Badge><ArrowRight className="size-4 text-slate-400"/></div>
 </button>)}</div>;
}

export function Workspace(){
 const [section,setSection]=useState<Section>("overview");
 const [orgs,setOrgs]=useState<Organization[]>([]);
 const [orgID,setOrgID]=useState("");
 const [profile,setProfile]=useState<Profile|null>(null);
 const [properties,setProperties]=useState<Property[]>([]);
 const [inspections,setInspections]=useState<Inspection[]>([]);
 const [members,setMembers]=useState<Member[]>([]);
 const [invitations,setInvitations]=useState<Invitation[]>([]);
 const [inviteEmail,setInviteEmail]=useState("");
 const [invitationLink,setInvitationLink]=useState("");
 const [contacts,setContacts]=useState<Contact[]>([]);
 const [contactCategory,setContactCategory]=useState<"owner"|"tenant"|"vendor"|"other">("tenant");
 const [contactEmail,setContactEmail]=useState("");
 const [jobs,setJobs]=useState<Job[]>([]);
 const [selectedID,setSelectedID]=useState("");
 const [selected,setSelected]=useState<Inspection|null>(null);
 const [items,setItems]=useState<InspectionItem[]>([]);
 const [media,setMedia]=useState<Media[]>([]);
 const [dialog,setDialog]=useState<DialogKind>(null);
 const [loading,setLoading]=useState<LoadingMap>({orgs:true,workspace:false,detail:false});
 const [saving,setSaving]=useState(false);
 const [uploading,setUploading]=useState(false);
 const [error,setError]=useState("");
 const [notice,setNotice]=useState("");
 const [search,setSearch]=useState("");
 const [mobileMenu,setMobileMenu]=useState(false);
 const [name,setName]=useState("");
 const [city,setCity]=useState("");
 const [street,setStreet]=useState("");
 const [propertyID,setPropertyID]=useState("");
 const [inspectionType,setInspectionType]=useState<InspectionType>("turnover");
 const [severity,setSeverity]=useState<"low"|"normal"|"high"|"critical">("normal");
 const [notes,setNotes]=useState("");
 const [phone,setPhone]=useState("");
 const [role,setRole]=useState<"manager"|"inspector">("inspector");
 const [currentPassword,setCurrentPassword]=useState("");
 const [newPassword,setNewPassword]=useState("");
 const [confirmPassword,setConfirmPassword]=useState("");
 const [devices,setDevices]=useState<{id:string;device_name:string;created_at:string;last_seen_at:string;expires_at:string;current:boolean}[]>([]);
 const [devicesLoading,setDevicesLoading]=useState(false);
 const [managedReport,setManagedReport]=useState("");
 const [links,setLinks]=useState<{id:string;expires_at:string;revoked_at:string|null;created_at:string}[]>([]);
 const [shareURL,setShareURL]=useState("");
 const [linksLoading,setLinksLoading]=useState(false);
 const [profileName,setProfileName]=useState("");
 const activeOrg=orgs.find(o=>o.id===orgID);
 const canManage=activeOrg?.role==="owner"||activeOrg?.role==="manager";
 const showError=(e:unknown)=>{setError(e instanceof Error?e.message:"Unable to complete this request");};

 const loadOrgs=useCallback(async()=>{
  setLoading(old=>({...old,orgs:true}));setError("");
  try{
   const [orgData,user]=await Promise.all([endpoints.orgs(),endpoints.me()]);
   setOrgs(orgData.items);setProfile(user);setProfileName(user.full_name);
   const previous=typeof window!=="undefined"?window.localStorage.getItem("inspectly_org"):"";
   setOrgID(current=>orgData.items.some(o=>o.id===current)?current:orgData.items.some(o=>o.id===previous)?previous||"":orgData.items[0]?.id||"");
  }catch(e){showError(e);}finally{setLoading(old=>({...old,orgs:false}));}
 },[]);
 useEffect(()=>{void loadOrgs();},[loadOrgs]);
 const reloadWorkspace=useCallback(async(id:string)=>{
  if(!id)return;
  setLoading(old=>({...old,workspace:true}));setError("");
  try{
   const [props,insp,team,queue]=await Promise.all([
    endpoints.properties(id),endpoints.inspections(id),endpoints.members(id),endpoints.jobs(id),
   ]);
   setProperties(props.items);setInspections(insp.items);setMembers(team.items);setJobs(queue.items);
  }catch(e){showError(e);}finally{setLoading(old=>({...old,workspace:false}));}
 },[]);
 useEffect(()=>{
  if(!orgID){setProperties([]);setInspections([]);setMembers([]);setJobs([]);return;}
  window.localStorage.setItem("inspectly_org",orgID);
  setSelectedID("");setSelected(null);setItems([]);setMedia([]);
  void reloadWorkspace(orgID);
 },[orgID,reloadWorkspace]);
 const loadContacts=useCallback(async(id:string)=>{
  try{const result=await endpoints.contacts(id);setContacts(result.items);}catch(e){showError(e);}
 },[]);
 useEffect(()=>{if(section==="contacts"&&orgID&&canManage)void loadContacts(orgID);},[section,orgID,canManage,loadContacts]);
 const loadDevices=useCallback(async()=>{
  setDevicesLoading(true);
  try{const list=await get<{items:typeof devices}>("v1/auth/sessions");setDevices(list.items);}
  catch(e){showError(e);}
  finally{setDevicesLoading(false);}
 },[]);
 useEffect(()=>{if(section==="settings")void loadDevices();},[section,loadDevices]);
 async function revokeDevice(id:string,current:boolean){
  if(!window.confirm(current?"Sign out this device?":"Revoke this device session?"))return;
  setSaving(true);setError("");
  try{
   await api(`v1/auth/sessions/${id}`,{method:"DELETE"});
   if(current){await signOut();return;}
   await loadDevices();setNotice("Device session revoked.");
  }catch(e){showError(e);}finally{setSaving(false);}
 }
 const reloadInvitations=useCallback(async(id:string)=>{try{const data=await get<{items:Invitation[]}>(`v1/organizations/${id}/invitations`);setInvitations(data.items);}catch(e){showError(e);}},[]);
 useEffect(()=>{if(section==="team"&&orgID&&canManage)void reloadInvitations(orgID);},[section,orgID,canManage,reloadInvitations]);
 const reloadDetail=useCallback(async(id:string,inspection:string)=>{
  setLoading(old=>({...old,detail:true}));setError("");
  try{
   const [detail,itemData,mediaData]=await Promise.all([endpoints.inspection(id,inspection),endpoints.items(id,inspection),endpoints.media(id,inspection)]);
   setSelected(detail);setItems(itemData.items);setMedia(mediaData.items);
  }catch(e){showError(e);}finally{setLoading(old=>({...old,detail:false}));}
 },[]);
 useEffect(()=>{if(orgID&&selectedID)void reloadDetail(orgID,selectedID);},[orgID,selectedID,reloadDetail]);
 const chooseSection=(value:Section)=>{setSection(value);setSelectedID("");setMobileMenu(false);setSearch("");setNotice("");setError("");};
 const openDialog=(value:DialogKind)=>{setDialog(value);setName("");setCity("");setStreet("");setNotes("");setPhone("");setSeverity("normal");setRole("inspector");setInviteEmail("");setInvitationLink("");setContactCategory("tenant");setContactEmail("");setPropertyID(properties[0]?.id||"");setInspectionType("turnover");setError("");};
 const refresh=async()=>{await reloadWorkspace(orgID);if(selectedID)await reloadDetail(orgID,selectedID);setNotice("Information refreshed.");};
 const completed=inspections.filter(v=>v.status==="completed").length;
 const outstanding=inspections.reduce((sum,v)=>sum+v.open_items,0);
 const recent=inspections.slice(0,5);
 const filteredInspections=useMemo(()=>inspections.filter(v=>(v.property_name+" "+v.type+" "+v.status).toLowerCase().includes(search.toLowerCase())),[inspections,search]);
 const filteredProperties=useMemo(()=>properties.filter(v=>(v.name+" "+Object.values(v.address||{}).join(" ")).toLowerCase().includes(search.toLowerCase())),[properties,search]);
 const formatAddress=(p:Property)=>Object.values(p.address||{}).filter(Boolean).join(", ")||"No address provided";
 async function submit(event:FormEvent<HTMLFormElement>){
  event.preventDefault();if(!dialog)return;setSaving(true);setError("");setNotice("");
  try{
   if(dialog==="organization"){
    const created=await post<Organization>("v1/organizations",{name:name.trim()});
    setDialog(null);await loadOrgs();setOrgID(created.id);setNotice("Organization created.");return;
   }
   if(dialog==="property")await post(`v1/organizations/${orgID}/properties`,{name:name.trim(),address:{city:city.trim(),street:street.trim()}});
   if(dialog==="inspection")await post(`v1/organizations/${orgID}/inspections`,{property_id:propertyID,type:inspectionType});
   if(dialog==="item")await post(`v1/organizations/${orgID}/inspections/${selectedID}/items`,{client_id:crypto.randomUUID(),title:name.trim(),notes:notes.trim(),severity});
   if(dialog==="contact"){await post(`v1/organizations/${orgID}/contacts`,{name:name.trim(),category:contactCategory,email:contactEmail.trim(),phone:phone.trim(),notes:notes.trim(),property_id:propertyID||null});await loadContacts(orgID);}
   if(dialog==="member"){
    const invitation=await post<{token:string}>(`v1/organizations/${orgID}/invitations`,{email:inviteEmail.trim(),role});
    setInvitationLink(window.location.origin+"/invite/"+invitation.token);
    await reloadInvitations(orgID);
    setNotice("Invitation created. Send the link to your teammate or use the configured email provider.");
    return;
   }
   setDialog(null);setNotice("Saved successfully.");
   await reloadWorkspace(orgID);if(selectedID)await reloadDetail(orgID,selectedID);
  }catch(e){showError(e);}finally{setSaving(false);}
 }
 async function resendInvitation(inv:Invitation){
  setSaving(true);setError("");
  try{
   const created=await post<{token:string}>(`v1/organizations/${orgID}/invitations`,{email:inv.email,role:inv.role});
   setInviteEmail(inv.email);setRole(inv.role);
   setInvitationLink(window.location.origin+"/invite/"+created.token);
   setDialog("member");
   await reloadInvitations(orgID);
   setNotice("New invitation issued. The previous link has been revoked.");
  }catch(e){showError(e);}finally{setSaving(false);}
 }
 async function revokeInvitation(invitationID:string){
  setSaving(true);setError("");
  try{await api(`v1/organizations/${orgID}/invitations/${invitationID}`,{method:"DELETE"});await reloadInvitations(orgID);setNotice("Invitation revoked.");}
  catch(e){showError(e);}finally{setSaving(false);}
 }
 async function loadLinks(inspectionID:string){
  setLinksLoading(true);setError("");
  try{
   const rows=await get<{items:{id:string;expires_at:string;revoked_at:string|null;created_at:string}[]}>(`v1/organizations/${orgID}/inspections/${inspectionID}/shares`);
   setLinks(rows.items);
  }catch(e){showError(e);}finally{setLinksLoading(false);}
 }
 function openLinks(inspectionID:string){
  setManagedReport(inspectionID);setShareURL("");void loadLinks(inspectionID);
 }
 async function shareReport(inspectionID:string){
  setSaving(true);setError("");
  try{
   const link=await post<{token:string;expires_at:string}>(`v1/organizations/${orgID}/inspections/${inspectionID}/share`,{});
   const url=window.location.origin+"/share/"+link.token;
   setManagedReport(inspectionID);setShareURL(url);await loadLinks(inspectionID);
   setNotice("Share link created. Copy it now; the original token cannot be retrieved later.");
  }catch(e){showError(e);}finally{setSaving(false);}
 }
 async function revokeLink(linkID:string){
  setSaving(true);setError("");
  try{await api(`v1/organizations/${orgID}/shares/${linkID}`,{method:"DELETE"});await loadLinks(managedReport);setNotice("Guest link revoked.");}
  catch(e){showError(e);}finally{setSaving(false);}
 }
 async function setInspectionStatus(status:InspectionStatus){
  if(!selected)return;setSaving(true);setError("");
  try{await patch(`v1/organizations/${orgID}/inspections/${selectedID}`,{status});await Promise.all([reloadDetail(orgID,selectedID),reloadWorkspace(orgID)]);setNotice("Inspection updated.");}
  catch(e){showError(e);}finally{setSaving(false);}
 }
 async function setItemStatus(itemID:string,status:ItemStatus){
  setSaving(true);setError("");
  try{await patch(`v1/organizations/${orgID}/inspections/${selectedID}/items/${itemID}`,{status});await Promise.all([reloadDetail(orgID,selectedID),reloadWorkspace(orgID)]);}
  catch(e){showError(e);}finally{setSaving(false);}
 }
 async function uploadFile(event:React.ChangeEvent<HTMLInputElement>){
  const file=event.target.files?.[0];event.target.value="";
  if(!file||!selectedID)return;
  if(file.size>50*1024*1024){setError("The maximum file size is 50 MB.");return;}
  const mime=file.type==="image/heif"?"image/heic":file.type;
  if(!["image/jpeg","image/png","image/heic","audio/mp4","audio/mpeg"].includes(mime)){setError("Supported formats: JPEG, PNG, HEIC, MP4 audio or MPEG audio.");return;}
  setUploading(true);setError("");
  try{
   const ticket=await post<{id:string;upload_url:string;headers:Record<string,string>}>(`v1/organizations/${orgID}/inspections/${selectedID}/media/presign`,{mime_type:mime,size_bytes:file.size});
   const uploaded=await fetch(ticket.upload_url,{method:"PUT",headers:ticket.headers,body:file});
   if(!uploaded.ok)throw Error("Storage rejected the upload ("+uploaded.status+"). Verify R2/S3 bucket CORS permissions.");
   const confirmed=await post<{state:string}>(`v1/organizations/${orgID}/inspections/${selectedID}/media/${ticket.id}/confirm`,{});
   if(confirmed.state!=="uploaded")throw Error("Media verification failed");
   setNotice("Media verified and ready to download.");
   await reloadDetail(orgID,selectedID);
  }catch(e){showError(e);}finally{setUploading(false);}
 }
 async function downloadMediaObject(id:string){
  setError("");
  try{
   const link=await get<{download_url:string}>(`v1/organizations/${orgID}/inspections/${selectedID}/media/${id}/download`);
   const url=new URL(link.download_url);
   if(url.protocol!=="https:")throw Error("Insecure download URL");
   window.location.assign(url.toString());
  }catch(e){showError(e);}
 }
 async function deleteMediaObject(id:string){
  if(!window.confirm("Permanently delete this file?"))return;
  setSaving(true);setError("");
  try{await api(`v1/organizations/${orgID}/inspections/${selectedID}/media/${id}`,{method:"DELETE"});
   await reloadDetail(orgID,selectedID);setNotice("Media deleted.");
  }catch(e){showError(e);}finally{setSaving(false);}
 }
 async function retryMediaConfirm(id:string){
  setSaving(true);setError("");
  try{await post(`v1/organizations/${orgID}/inspections/${selectedID}/media/${id}/confirm`,{});
   await reloadDetail(orgID,selectedID);setNotice("Media verified.");
  }catch(e){showError(e);}finally{setSaving(false);}
 }
 async function saveProfile(event:FormEvent){
  event.preventDefault();setSaving(true);setError("");
  try{await patch("v1/me",{full_name:profileName.trim()});setProfile(current=>current?{...current,full_name:profileName.trim()}:current);setNotice("Profile saved.");}
  catch(e){showError(e);}finally{setSaving(false);}
 }
 async function savePasswordChange(event:FormEvent){
  event.preventDefault();setSaving(true);setError("");setNotice("");
  try{
   if(newPassword!==confirmPassword)throw Error("The new passwords do not match.");
   await post("v1/auth/password/change",{current_password:currentPassword,new_password:newPassword});
   setCurrentPassword("");setNewPassword("");setConfirmPassword("");
   // Password changes invalidate all sessions, including this one.
   await fetch("/api/auth/logout",{method:"POST"}).catch(()=>{});
   window.location.assign("/login?password=changed");
   return;
  }catch(e){showError(e);}finally{setSaving(false);}
 }
 async function signOut(){
  try{
   const response=await fetch("/api/auth/logout",{method:"POST"});
   if(!response.ok)throw Error("Could not revoke your session. Try again.");
   window.location.assign("/login");
  }catch(e){showError(e);}
 }
 return <div className="flex min-h-screen bg-[#f6f8fc]">
  {mobileMenu&&<button className="fixed inset-0 z-30 bg-slate-900/40 lg:hidden" aria-label="Close menu" onClick={()=>setMobileMenu(false)}/>}
  <aside className={cn("fixed inset-y-0 left-0 z-40 flex w-[252px] flex-col border-r border-slate-200/80 bg-white transition-transform lg:sticky lg:top-0 lg:h-screen lg:translate-x-0",mobileMenu?"translate-x-0":"-translate-x-full")}>
   <div className="flex h-[86px] items-center justify-between px-6"><div className="flex items-center gap-2.5"><span className="flex size-10 items-center justify-center rounded-xl bg-blue-600 text-white"><Building2 className="size-5"/></span><span className="text-xl font-extrabold tracking-[-.035em] text-[#102446]">inspectly<span className="text-blue-600">.</span></span></div><button className="lg:hidden" onClick={()=>setMobileMenu(false)} aria-label="Close"><X className="size-5"/></button></div>
   <div className="mx-4 mt-3 rounded-xl border border-slate-200 bg-slate-50/70 p-3"><p className="mb-1 text-[10px] font-bold uppercase tracking-[.16em] text-slate-400">Workspace</p>
    {orgs.length>0?<Select value={orgID} onValueChange={setOrgID}><SelectTrigger aria-label="Active organization" className="h-10 w-full min-w-0 rounded-lg border-slate-200 bg-white px-3 text-sm font-semibold text-slate-800 shadow-sm hover:border-blue-300 focus-visible:ring-2 focus-visible:ring-blue-100"><SelectValue placeholder="Choose workspace"/></SelectTrigger><SelectContent>{orgs.map(o=><SelectItem key={o.id} value={o.id}>{o.name}</SelectItem>)}</SelectContent></Select>:<p className="text-sm text-slate-500">No organization</p>}
    <button onClick={()=>openDialog("organization")} className="mt-2 flex items-center gap-1.5 text-xs font-semibold text-blue-600 hover:text-blue-800"><Plus className="size-3.5"/> New organization</button>
   </div>
   <nav className="mt-9 flex-1 space-y-1 px-3" aria-label="Main navigation">
    <p className="mb-3 px-3 text-[10px] font-bold uppercase tracking-[.17em] text-slate-400">Management</p>
    {sections.slice(0,6).filter(entry=>entry.id!=="contacts"||canManage).map(({id,label,icon:Icon})=><button key={id} onClick={()=>chooseSection(id)} className={cn("flex w-full items-center gap-3 rounded-xl px-3.5 py-3 text-left text-sm font-semibold transition-colors",section===id?"bg-blue-50 text-blue-700":"text-slate-500 hover:bg-slate-50 hover:text-slate-800")}><Icon className="size-[19px]"/>{label}{id==="inspections"&&inspections.length>0&&<span className="ml-auto rounded-md bg-slate-100 px-2 py-0.5 text-[11px] font-bold text-slate-500">{inspections.length}</span>}</button>)}
    <p className="mb-3 mt-9 px-3 text-[10px] font-bold uppercase tracking-[.17em] text-slate-400">Account</p>
    <button onClick={()=>chooseSection("settings")} className={cn("flex w-full items-center gap-3 rounded-xl px-3.5 py-3 text-left text-sm font-semibold",section==="settings"?"bg-blue-50 text-blue-700":"text-slate-500 hover:bg-slate-50")}><Settings className="size-[19px]"/>Settings</button>
   </nav>
   <div className="border-t border-slate-100 p-4"><div className="flex items-center gap-3"><span className="flex size-10 shrink-0 items-center justify-center rounded-full bg-blue-100 text-sm font-bold text-blue-700">{(profile?.full_name||profile?.phone||"U").slice(0,1).toUpperCase()}</span><div className="min-w-0 flex-1"><p className="truncate text-sm font-semibold text-slate-800">{profile?.full_name||profile?.phone||"Account"}</p><p className="text-xs capitalize text-slate-400">{activeOrg?.role||"Member"}</p></div><button aria-label="Sign out" title="Sign out" onClick={signOut} className="rounded-lg p-2 text-slate-400 hover:bg-slate-100 hover:text-red-600"><LogOut className="size-4"/></button></div></div>
  </aside>
  <div className="min-w-0 flex-1">
   <header className="sticky top-0 z-20 flex h-[74px] items-center justify-between border-b border-slate-200/70 bg-white/90 px-5 backdrop-blur sm:px-9">
    <div className="flex items-center gap-3"><button aria-label="Open navigation" className="rounded-lg p-2 text-slate-500 lg:hidden" onClick={()=>setMobileMenu(true)}><Menu/></button><span className="hidden text-xs font-semibold text-slate-400 sm:inline">Workspace</span><span className="hidden text-slate-300 sm:inline">/</span><span className="text-sm font-semibold text-slate-800">{sections.find(v=>v.id===section)?.label}</span></div>
    <div className="flex items-center gap-3"><span className="hidden items-center gap-1.5 rounded-full bg-emerald-50 px-3 py-1.5 text-xs font-semibold text-emerald-700 sm:inline-flex"><span className="size-1.5 rounded-full bg-emerald-500"/> Live workspace</span><Button variant="ghost" size="icon" aria-label="Refresh data" onClick={()=>void refresh()} disabled={!orgID||loading.workspace}><RefreshCw className={cn("size-4",loading.workspace&&"animate-spin")}/></Button><span className="flex size-9 items-center justify-center rounded-full bg-blue-600 text-sm font-bold text-white">{(profile?.full_name||profile?.phone||"U").slice(0,1).toUpperCase()}</span></div>
   </header>
   <main className="mx-auto max-w-[1400px] px-5 py-8 sm:px-9 sm:py-10">
    {notice&&<div role="status" className="mb-5 flex items-center gap-2 rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm font-medium text-emerald-800"><CheckCircle2 className="size-4"/>{notice}<button className="ml-auto" aria-label="Dismiss" onClick={()=>setNotice("")}><X className="size-4"/></button></div>}
    {error&&<div role="alert" className="mb-5 flex items-center gap-2 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700"><AlertCircle className="size-4 shrink-0"/>{error}<button className="ml-auto" aria-label="Dismiss" onClick={()=>setError("")}><X className="size-4"/></button></div>}
    {loading.orgs?<Loader/>:orgs.length===0?<Card><Empty icon={Building2} title="Create your first organization" description="Your properties, inspections and team members will be kept within their own secure workspace." action={<Button onClick={()=>openDialog("organization")}><Plus className="size-4"/>Create organization</Button>}/></Card>:<>
     {section==="overview"&&<>
      <SectionHeading eyebrow="Your workspace" title={"Good to see you"+(profile?.full_name?", "+profile.full_name.split(" ")[0]:"")+". 👋"} description="A real-time view of your properties and active inspections." action={<Button onClick={()=>openDialog("inspection")} disabled={!properties.length}><Plus className="size-4"/>New inspection</Button>}/>
      {loading.workspace?<Loader/>:<>
       <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Stat icon={Building2} label="Total properties" value={properties.length} caption="Registered locations"/>
        <Stat icon={ClipboardList} label="Inspections" value={inspections.length} caption="Across all properties"/>
        <Stat icon={CheckCircle2} label="Completed" value={completed} caption="Finished inspections"/>
        <Stat icon={AlertCircle} label="Open issues" value={outstanding} caption="Unresolved punch-list items"/>
       </div>
       <div className="mt-7 grid gap-6 xl:grid-cols-[1.7fr_1fr]">
        <Card className="overflow-hidden"><div className="flex items-center justify-between border-b border-slate-100 px-6 py-5"><div><h2 className="text-base font-bold">Recent inspections</h2><p className="mt-1 text-xs text-slate-400">Latest activity across your workspace</p></div><button onClick={()=>chooseSection("inspections")} className="flex items-center gap-1 text-xs font-bold text-blue-600">View all <ArrowRight className="size-4"/></button></div>
         {recent.length?<InspectionRows items={recent} onSelect={id=>{setSection("inspections");setSelectedID(id);}}/>:<Empty icon={ClipboardCheck} title="No inspections yet" description="Create an inspection to start documenting your property's condition." action={<Button onClick={()=>openDialog("inspection")} disabled={!properties.length}><Plus className="size-4"/>New inspection</Button>}/>}
        </Card>
        <Card className="p-6"><h2 className="text-base font-bold">Quick actions</h2><p className="mt-1 text-xs text-slate-400">Get work moving</p><div className="mt-6 space-y-3">
         {[{label:"Add property",desc:"Register another location",icon:Building2,click:()=>openDialog("property"),disabled:!canManage},{label:"Start inspection",desc:"Document a property condition",icon:ClipboardCheck,click:()=>openDialog("inspection"),disabled:!properties.length},{label:"Invite team member",desc:"Assign an inspector or manager",icon:Users,click:()=>openDialog("member"),disabled:!canManage}].map(x=><button key={x.label} disabled={x.disabled} onClick={x.click} className="flex w-full items-center gap-3 rounded-xl border border-slate-100 p-3 text-left hover:border-blue-200 hover:bg-blue-50/40 disabled:cursor-not-allowed disabled:opacity-50"><span className="flex size-11 items-center justify-center rounded-lg bg-slate-100 text-slate-700"><x.icon className="size-5"/></span><div className="flex-1"><p className="text-sm font-bold">{x.label}</p><p className="mt-0.5 text-xs text-slate-400">{x.desc}</p></div><ArrowUpRight className="size-4 text-slate-400"/></button>)}
        </div></Card>
       </div>
      </>}
     </>}
     {section==="properties"&&<>
      <SectionHeading eyebrow="Portfolio" title="Your properties" description="All the locations managed by your organization." action={canManage?<Button onClick={()=>openDialog("property")}><Plus className="size-4"/>Add property</Button>:undefined}/>
      <div className="mb-6 max-w-sm"><div className="relative"><Search className="absolute left-3 top-3.5 size-4 text-slate-400"/><Input className="pl-10" placeholder="Search properties..." value={search} onChange={e=>setSearch(e.target.value)}/></div></div>
      {loading.workspace?<Loader/>:filteredProperties.length?<div className="grid gap-5 md:grid-cols-2 xl:grid-cols-3">{filteredProperties.map(p=><Card key={p.id} className="group overflow-hidden"><div className="flex h-36 items-center justify-center bg-gradient-to-br from-blue-50 via-slate-50 to-indigo-100"><House className="size-12 text-blue-200"/></div><div className="p-5"><div className="mb-3 flex items-start justify-between"><h3 className="text-base font-bold">{p.name}</h3><Badge tone="blue">Property</Badge></div><p className="flex items-center gap-2 text-sm text-slate-500"><MapPin className="size-4 shrink-0"/><span className="truncate">{formatAddress(p)}</span></p><div className="mt-5 flex items-center justify-between border-t border-slate-100 pt-4"><span className="text-xs text-slate-500">{inspections.filter(i=>i.property_id===p.id).length} inspections</span><button onClick={()=>{setSection("inspections");setSearch(p.name);}} className="text-xs font-bold text-blue-600">View inspections →</button></div></div></Card>)}</div>:<Card><Empty icon={Building2} title={search?"No matching properties":"No properties yet"} description={search?"Try a different search.":"Add the first property to begin scheduling inspections."} action={!search&&canManage?<Button onClick={()=>openDialog("property")}><Plus className="size-4"/>Add property</Button>:undefined}/></Card>}
     </>}
     {section==="inspections"&&<>
      {selectedID?<><button onClick={()=>{setSelectedID("");setError("");}} className="mb-4 flex items-center gap-1 text-sm font-bold text-blue-600"><ArrowDownRight className="size-4 rotate-45"/> All inspections</button>
       {loading.detail?<Loader/>:selected&&<><SectionHeading eyebrow="Inspection details" title={selected.property_name} description={typeNames[selected.type]+" • Created "+humanDate(selected.created_at)} action={<div className="flex flex-wrap gap-2"><Badge tone={statusTone(selected.status)} className="self-center">{statusNames[selected.status]}</Badge>{selected.status==="draft"&&<Button disabled={saving} onClick={()=>void setInspectionStatus("in_progress")}>Start inspection</Button>}{selected.status==="in_progress"&&<Button disabled={saving} onClick={()=>void setInspectionStatus("completed")}><Check className="size-4"/>Complete</Button>}{["draft","in_progress"].includes(selected.status)&&<Button variant="outline" disabled={saving} onClick={()=>void setInspectionStatus("cancelled")}>Cancel</Button>}</div>}/>
        <div className="mb-5 grid gap-4 sm:grid-cols-3"><Stat icon={ClipboardCheck} label="Items" value={items.length} caption="Documented punch-list items"/><Stat icon={AlertCircle} label="Open" value={items.filter(x=>x.status!=="resolved").length} caption="Need attention"/><Stat icon={Camera} label="Attachments" value={media.length} caption="Recorded media objects"/></div>
        <Card className="mb-6 overflow-hidden"><div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-100 px-6 py-5"><div><h2 className="text-base font-bold">Punch list</h2><p className="mt-1 text-xs text-slate-400">Issues and work to complete</p></div>{["draft","in_progress"].includes(selected.status)&&<Button onClick={()=>openDialog("item")} size="sm"><Plus className="size-4"/>Add item</Button>}</div>
         {items.length?<div className="divide-y divide-slate-100">{items.map(item=><div key={item.id} className="flex flex-wrap items-center justify-between gap-4 px-6 py-4"><div className="flex min-w-0 items-start gap-3"><span className={cn("mt-1 flex size-9 shrink-0 items-center justify-center rounded-lg",item.status==="resolved"?"bg-emerald-50 text-emerald-600":"bg-amber-50 text-amber-700")}>{item.status==="resolved"?<Check className="size-4"/>:<AlertCircle className="size-4"/>}</span><div><p className="text-sm font-bold text-slate-800">{item.title}</p>{item.notes&&<p className="mt-1 max-w-2xl whitespace-pre-wrap text-xs text-slate-500">{item.notes}</p>}<div className="mt-2 flex gap-2"><Badge tone={statusTone(item.severity)}>{item.severity}</Badge><Badge tone={statusTone(item.status)}>{prettyStatus(item.status)}</Badge></div></div></div>
          {["draft","in_progress"].includes(selected.status)&&<Select value={item.status} disabled={saving} onValueChange={value=>void setItemStatus(item.id,value as ItemStatus)}><SelectTrigger aria-label={"Status for "+item.title} className="h-9 w-[148px] shrink-0 rounded-lg border-slate-200 bg-white px-3 text-xs font-semibold text-slate-700 shadow-sm hover:border-blue-300"><SelectValue/></SelectTrigger><SelectContent><SelectItem value="open">Open</SelectItem><SelectItem value="in_progress">In progress</SelectItem><SelectItem value="resolved">Resolved</SelectItem></SelectContent></Select>}</div>)}</div>:<Empty icon={ClipboardList} title="No punch-list items" description="Create the first item to record a condition, issue, or outstanding task." action={["draft","in_progress"].includes(selected.status)?<Button onClick={()=>openDialog("item")}><Plus className="size-4"/>Add item</Button>:undefined}/>}
        </Card>
        <Card className="overflow-hidden"><div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-100 px-6 py-5"><div><h2 className="text-base font-bold">Photos & audio</h2><p className="mt-1 text-xs text-slate-400">Direct encrypted transfer to configured S3/R2 storage</p></div><label className={cn("inline-flex h-9 cursor-pointer items-center justify-center gap-2 rounded-xl bg-blue-600 px-3 text-xs font-bold text-white",uploading&&"pointer-events-none opacity-50")}><Upload className="size-4"/>{uploading?"Uploading…":"Upload media"}<input type="file" className="sr-only" accept="image/jpeg,image/png,image/heic,audio/mp4,audio/mpeg" onChange={e=>void uploadFile(e)} disabled={uploading}/></label></div>
         {media.length?<div className="divide-y divide-slate-100">{media.map(m=><div key={m.id} className="flex items-center justify-between gap-3 px-6 py-4"><div className="flex items-center gap-3"><span className="flex size-10 items-center justify-center rounded-xl bg-slate-50 text-slate-500"><ImageIcon className="size-5"/></span><div><p className="text-sm font-semibold text-slate-700">{m.mime_type}</p><p className="text-xs text-slate-400">{(m.size_bytes/1024/1024).toFixed(2)} MB · {humanDate(m.created_at)}</p></div></div><div className="flex flex-wrap items-center gap-2"><Badge tone={statusTone(m.state)}>{m.state}</Badge>{m.state==="uploaded"&&<Button size="sm" variant="outline" onClick={()=>void downloadMediaObject(m.id)}>Download</Button>}{m.state==="pending"&&<Button size="sm" variant="outline" disabled={saving} onClick={()=>void retryMediaConfirm(m.id)}>Verify</Button>}{m.state!=="rejected"&&<Button size="sm" variant="outline" disabled={saving} onClick={()=>void deleteMediaObject(m.id)}>Delete</Button>}</div></div>)}</div>:<Empty icon={ImageIcon} title="No media attached" description="Upload photos or audio recorded during the inspection."/>}
        </Card>
       </>}
      </>:<>
       <SectionHeading eyebrow="Field operations" title="Inspections" description="Create, track, and complete inspections across your portfolio." action={<Button onClick={()=>openDialog("inspection")} disabled={!properties.length}><Plus className="size-4"/>New inspection</Button>}/>
       <div className="mb-6 max-w-sm"><div className="relative"><Search className="absolute left-3 top-3.5 size-4 text-slate-400"/><Input className="pl-10" placeholder="Search inspections..." value={search} onChange={e=>setSearch(e.target.value)}/></div></div>
       <Card className="overflow-hidden">{loading.workspace?<Loader/>:filteredInspections.length?<InspectionRows items={filteredInspections} onSelect={setSelectedID}/>:<Empty icon={ClipboardCheck} title={search?"No matching inspections":"No inspections yet"} description={search?"Change your search to see more inspections.":"Create an inspection for any registered property."} action={!search?<Button onClick={()=>openDialog("inspection")} disabled={!properties.length}><Plus className="size-4"/>New inspection</Button>:undefined}/>}</Card>
      </>}
     </>}
     {section==="team"&&<>
      <SectionHeading eyebrow="Collaboration" title="Team members" description="Invite teammates by email and control access to your organization's workspace." action={canManage?<Button onClick={()=>openDialog("member")}><Plus className="size-4"/>Invite member</Button>:undefined}/>
      <Card className="overflow-hidden">{loading.workspace?<Loader/>:members.length?<div className="divide-y divide-slate-100">{members.map(member=><div key={member.id} className="flex flex-wrap items-center justify-between gap-3 px-6 py-4"><div className="flex items-center gap-3"><span className="flex size-10 items-center justify-center rounded-full bg-blue-50 font-bold text-blue-600">{(member.full_name||member.phone).slice(0,1).toUpperCase()}</span><div><p className="text-sm font-bold">{member.full_name||member.phone}</p><p className="mt-1 text-xs text-slate-500">{member.phone}</p></div></div><div className="flex items-center gap-4"><span className="hidden text-xs text-slate-400 sm:inline">Joined {humanDate(member.joined_at)}</span><Badge tone={member.role==="owner"?"blue":"slate"}>{member.role}</Badge></div></div>)}</div>:<Empty icon={Users} title="No team members yet" description="Invite a teammate to help with inspections."/>}</Card>
      {canManage&&<><div className="mt-7 mb-3 flex items-center justify-between"><h3 className="text-lg font-bold text-slate-900">Team invitations</h3><span className="text-xs text-slate-500">Links expire after seven days</span></div>
       <Card className="overflow-hidden">{invitations.length?<div className="divide-y divide-slate-100">{invitations.map(inv=><div key={inv.id} className="flex flex-wrap items-center justify-between gap-3 px-5 py-4"><div className="min-w-0"><p className="truncate text-sm font-semibold">{inv.email}</p><p className="mt-1 text-xs text-slate-500">Role: {inv.role} · Expires {humanDate(inv.expires_at)}</p></div><div className="flex items-center gap-3">{inv.accepted_at?<Badge tone="green">Accepted</Badge>:inv.revoked_at?<Badge tone="slate">Revoked</Badge>:new Date(inv.expires_at)<=new Date()?<Badge tone="red">Expired</Badge>:<><Badge tone="amber">Pending</Badge>{(activeOrg?.role==="owner"||inv.role!=="manager")&&<Button variant="outline" size="sm" disabled={saving} onClick={()=>void revokeInvitation(inv.id)}>Revoke</Button>}</>}{!inv.accepted_at&&(activeOrg?.role==="owner"||inv.role!=="manager")&&<Button variant="outline" size="sm" disabled={saving} onClick={()=>void resendInvitation(inv)}>Resend</Button>}</div></div>)}</div>:<Empty icon={Users} title="No invitations yet" description="Create an invitation to add your first teammate." action={<Button onClick={()=>openDialog("member")}>Invite member</Button>}/>}</Card>
       <p className="mt-4 text-xs text-slate-500">Only verified recipients can accept an invitation. Invite links are shown once after creation; revoked or expired links cannot be reused.</p>
      </>}
     </>}
     {section==="contacts"&&<>
      <SectionHeading eyebrow="CRM-lite" title="Contacts" description="Real owners, tenants and vendor records associated with your organization." action={canManage?<Button onClick={()=>openDialog("contact")}><Plus className="size-4"/>New contact</Button>:undefined}/>
      <div className="mb-6 max-w-sm"><div className="relative"><Search className="absolute left-3 top-3.5 size-4 text-slate-400"/><Input className="pl-10" placeholder="Search contacts..." value={search} onChange={e=>setSearch(e.target.value)}/></div></div>
      <Card className="overflow-hidden">{contacts.filter(c=>(c.name+" "+c.phone+" "+c.email+" "+c.category).toLowerCase().includes(search.toLowerCase())).length?<div className="divide-y divide-slate-100">{contacts.filter(c=>(c.name+" "+c.phone+" "+c.email+" "+c.category).toLowerCase().includes(search.toLowerCase())).map(contact=><div key={contact.id} className="flex flex-wrap items-center justify-between gap-3 px-6 py-4"><div className="flex items-center gap-3"><span className="flex size-10 items-center justify-center rounded-full bg-blue-50 font-bold text-blue-600">{contact.name.slice(0,1).toUpperCase()}</span><div><p className="text-sm font-bold">{contact.name}</p><p className="mt-1 text-xs text-slate-500">{[contact.email,contact.phone].filter(Boolean).join(" · ")||"No contact details"}</p>{contact.notes&&<p className="mt-1 text-xs text-slate-500">{contact.notes}</p>}</div></div><Badge tone="blue">{contact.category}</Badge></div>)}</div>:<Empty icon={Users} title="No contacts found" description="Add a real contact to associate owners, tenants or vendors with your properties." action={canManage?<Button onClick={()=>openDialog("contact")}><Plus className="size-4"/>New contact</Button>:undefined}/>}</Card>
     </>}
     {section==="reports"&&<>
      <SectionHeading eyebrow="Operations" title="Reports & AI jobs" description="Monitor background processing. Results appear only when a configured worker completes a job."/>
      <div className="mb-6 rounded-xl border border-blue-100 bg-blue-50 p-5"><div className="flex items-start gap-3"><ShieldCheck className="mt-0.5 size-5 text-blue-600"/><div><h3 className="text-sm font-bold text-blue-900">Report generation is not yet enabled</h3><p className="mt-1 text-sm leading-relaxed text-blue-700">The database job queue is ready, but the AI/transcription and PDF workers are not deployed. No reports are generated or shown as complete until processing is implemented.</p></div></div></div>
      <Card className="mb-6 overflow-hidden"><div className="border-b border-slate-100 px-6 py-5"><h2 className="text-base font-bold">Print-ready inspection reports</h2><p className="mt-1 text-xs text-slate-400">Export actual inspection records through your browser’s Print / Save as PDF feature</p></div>{inspections.length?<div className="divide-y divide-slate-100">{inspections.map(report=><div key={report.id} className="flex flex-wrap items-center justify-between gap-3 px-6 py-4"><div><p className="text-sm font-bold">{report.property_name}</p><p className="mt-1 text-xs text-slate-400">{typeNames[report.type]} · {humanDate(report.created_at)} · {report.total_items} items</p></div><div className="flex flex-wrap gap-2"><a target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-2 rounded-xl border border-slate-200 px-3 py-2 text-xs font-bold text-blue-600 hover:bg-blue-50" href={`/report/${orgID}/${report.id}`}><FileText className="size-4"/>Open report</a>{canManage&&<><Button variant="outline" size="sm" disabled={saving} onClick={()=>void shareReport(report.id)}><ExternalLink className="size-4"/>Share link</Button><Button variant="ghost" size="sm" onClick={()=>openLinks(report.id)}>Manage links</Button></>}</div></div>)}</div>:<Empty icon={FileText} title="No inspections to export" description="Create an inspection to generate a print-ready record from real data."/>}</Card>
      <Card className="overflow-hidden"><div className="border-b border-slate-100 px-6 py-5"><h2 className="text-base font-bold">Background jobs</h2><p className="mt-1 text-xs text-slate-400">Live records from the organization&apos;s job queue</p></div>{loading.workspace?<Loader/>:jobs.length?<div className="divide-y divide-slate-100">{jobs.map(job=><div key={job.id} className="flex items-center justify-between gap-3 px-6 py-4"><div><p className="text-sm font-bold capitalize">{prettyStatus(job.kind)}</p><p className="mt-1 text-xs text-slate-400">{humanDate(job.created_at)} · {job.attempts} attempts</p></div><Badge tone={statusTone(job.state)}>{prettyStatus(job.state)}</Badge></div>)}</div>:<Empty icon={FileText} title="No background jobs" description="Job activity will appear here when processing pipelines are enabled."/>}</Card>
     </>}
     {section==="settings"&&<>
      <SectionHeading eyebrow="Preferences" title="Account settings" description="Your profile and account security."/>
      <div className="grid max-w-4xl gap-6 lg:grid-cols-2"><Card className="p-6"><div className="mb-5 flex items-center gap-3"><div className="flex size-10 items-center justify-center rounded-xl bg-blue-50 text-blue-600"><Users className="size-5"/></div><div><h2 className="font-bold">Personal information</h2><p className="text-xs text-slate-400">Saved to your user profile</p></div></div>
       <form onSubmit={saveProfile} className="space-y-4"><Field name="Phone number"><Input disabled value={profile?.phone||""} readOnly/></Field><Field name="Display name"><Input maxLength={120} placeholder="Your full name" value={profileName} onChange={e=>setProfileName(e.target.value)}/></Field><Button type="submit" disabled={saving}>Save profile</Button></form>
      </Card><Card className="p-6"><div className="mb-5 flex items-center gap-3"><div className="flex size-10 items-center justify-center rounded-xl bg-blue-50 text-blue-600"><Shield className="size-5"/></div><div><h2 className="font-bold">Account password</h2><p className="text-xs text-slate-400">Change your sign-in password securely</p></div></div>
       <form className="space-y-4" onSubmit={savePasswordChange}>
        <Field name="Current password"><Input type="password" autoComplete="current-password" required value={currentPassword} onChange={e=>setCurrentPassword(e.target.value)} placeholder="Current password"/></Field>
        <Field name="New password"><Input type="password" autoComplete="new-password" minLength={15} required value={newPassword} onChange={e=>setNewPassword(e.target.value)} placeholder="At least 15 characters"/></Field>
        <Field name="Confirm new password"><Input type="password" autoComplete="new-password" minLength={15} required value={confirmPassword} onChange={e=>setConfirmPassword(e.target.value)} placeholder="Repeat new password"/></Field>
        <p className="text-xs text-slate-500">Use at least 15 characters. Spaces and Unicode are allowed; common passwords are blocked. No symbol or uppercase rule.</p>
        <Button type="submit" disabled={saving||newPassword!==confirmPassword}>Update password</Button></form></Card>
      </div>
      <Card className="mt-6 max-w-4xl p-6">
       <div className="flex flex-wrap items-center justify-between gap-3">
        <div><h2 className="font-bold">Signed-in devices</h2><p className="mt-1 text-xs text-slate-500">Review active sessions. Revoking a device immediately invalidates its access and refresh tokens.</p></div>
        <Button variant="outline" size="sm" onClick={()=>void loadDevices()} disabled={devicesLoading}>Refresh</Button>
       </div>
       {devicesLoading?<div className="mt-5 text-sm text-slate-500">Loading devices…</div>:devices.length?
        <div className="mt-5 divide-y divide-slate-100">{devices.map(device=><div key={device.id} className="flex flex-wrap items-center justify-between gap-4 py-4">
         <div className="min-w-0"><div className="flex items-center gap-2"><p className="max-w-sm truncate text-sm font-semibold text-slate-800" title={device.device_name}>{device.device_name}</p>{device.current&&<Badge tone="blue">This device</Badge>}</div>
         <p className="mt-1 text-xs text-slate-500">Last active {humanDate(device.last_seen_at)} · Expires {humanDate(device.expires_at)}</p></div>
         <Button variant="outline" size="sm" disabled={saving} onClick={()=>void revokeDevice(device.id,device.current)}>{device.current?"Sign out":"Revoke session"}</Button>
        </div>)}</div>:
        <p className="mt-5 text-sm text-slate-500">No active device sessions found.</p>}
      </Card>
      <Card className="mt-6 max-w-4xl p-6"><h2 className="font-bold">Current organization</h2><p className="mt-1 text-sm text-slate-500">{activeOrg?.name} · Your role: <span className="capitalize">{activeOrg?.role}</span></p><p className="mt-3 text-xs text-slate-400">Organization access controls are enforced by the API for every request.</p></Card>
     </>}
    </>}
   </main>
  </div>
  <Dialog open={managedReport!==""} onOpenChange={open=>{if(!open){setManagedReport("");setShareURL("");setError("");}}}>
   <DialogContent>
    <DialogTitle className="text-lg font-bold">Guest report links</DialogTitle>
    <DialogDescription className="mt-2 text-sm text-slate-500">Guests can view the shared inspection without an account. Links expire after 7 days and can be revoked.</DialogDescription>
    {shareURL&&<div className="mt-5 space-y-2"><label htmlFor="guest-link" className="text-sm font-bold">New share link — copy now</label><Input id="guest-link" readOnly value={shareURL} onFocus={e=>e.currentTarget.select()}/><Button size="sm" onClick={()=>{void navigator.clipboard.writeText(shareURL).then(()=>setNotice("Link copied.")).catch(()=>setError("Clipboard unavailable. Select and copy the link manually."));}}>Copy link</Button></div>}
    <div className="mt-6"><h3 className="mb-3 text-sm font-bold">Existing links</h3>{linksLoading?<Loader/>:links.length===0?<p className="text-sm text-slate-500">No links have been created.</p>:<div className="max-h-64 divide-y divide-slate-100 overflow-auto">{links.map(link=><div key={link.id} className="flex items-center justify-between gap-3 py-3"><div><p className="text-xs font-semibold text-slate-700">Created {humanDate(link.created_at)}</p><p className="mt-1 text-xs text-slate-500">Expires {humanDate(link.expires_at)}</p></div>{link.revoked_at?<Badge tone="red">Revoked</Badge>:new Date(link.expires_at)<new Date()?<Badge>Expired</Badge>:<Button disabled={saving} size="sm" variant="outline" onClick={()=>void revokeLink(link.id)}>Revoke</Button>}</div>)}</div>}</div>
    {error&&<p role="alert" className="mt-3 text-sm text-red-600">{error}</p>}
   </DialogContent>
  </Dialog>
  <Dialog open={dialog!==null} onOpenChange={open=>{if(!open){setDialog(null);setError("");}}}>
   <DialogContent>
    <DialogTitle className="text-xl font-bold">{dialog==="organization"?"Create organization":dialog==="property"?"Add property":dialog==="inspection"?"New inspection":dialog==="member"?"Add team member":dialog==="contact"?"Create contact":"Add punch-list item"}</DialogTitle>
    <DialogDescription className="mt-2 text-sm text-slate-500">{dialog==="organization"?"Set up a private workspace for your team.":dialog==="property"?"Register a real property in your portfolio.":dialog==="inspection"?"Select a property and inspection type.":dialog==="member"?"Send an invite link to a teammate. They can register after receiving it.":dialog==="contact"?"Save an actual contact for this organization.":"Document a condition or task for this inspection."}</DialogDescription>
    <form className="mt-6 space-y-4" onSubmit={submit}>
     {["organization","property","item","contact"].includes(dialog||"")&&<Field name={dialog==="organization"?"Organization name":dialog==="property"?"Property name":dialog==="contact"?"Contact name":"Item title"}><Input autoFocus required maxLength={dialog==="item"?500:dialog==="property"||dialog==="contact"?160:120} value={name} onChange={e=>setName(e.target.value)} placeholder={dialog==="organization"?"Your company name":dialog==="property"?"Property or unit name":dialog==="contact"?"Full name":"Describe the issue"}/></Field>}
     {dialog==="property"&&<><Field name="Street / address"><Input value={street} onChange={e=>setStreet(e.target.value)} placeholder="Street and building"/></Field><Field name="City"><Input value={city} onChange={e=>setCity(e.target.value)} placeholder="City"/></Field></>}
     {dialog==="inspection"&&<><Field name="Property"><Select value={propertyID} onValueChange={setPropertyID}><SelectTrigger><SelectValue placeholder="Select a property"/></SelectTrigger><SelectContent>{properties.map(p=><SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>)}</SelectContent></Select></Field><Field name="Inspection type"><Select value={inspectionType} onValueChange={v=>setInspectionType(v as InspectionType)}><SelectTrigger><SelectValue/></SelectTrigger><SelectContent>{(Object.keys(typeNames) as InspectionType[]).map(t=><SelectItem key={t} value={t}>{typeNames[t]}</SelectItem>)}</SelectContent></Select></Field></>}
     {dialog==="item"&&<><Field name="Notes"><Textarea value={notes} maxLength={10000} onChange={e=>setNotes(e.target.value)} placeholder="Additional details (optional)"/></Field><Field name="Severity"><Select value={severity} onValueChange={v=>setSeverity(v as typeof severity)}><SelectTrigger><SelectValue/></SelectTrigger><SelectContent>{["low","normal","high","critical"].map(t=><SelectItem key={t} value={t}>{t[0].toUpperCase()+t.slice(1)}</SelectItem>)}</SelectContent></Select></Field></>}
     {dialog==="contact"&&<><Field name="Category"><Select value={contactCategory} onValueChange={v=>setContactCategory(v as typeof contactCategory)}><SelectTrigger><SelectValue/></SelectTrigger><SelectContent>{["owner","tenant","vendor","other"].map(v=><SelectItem key={v} value={v}>{v[0].toUpperCase()+v.slice(1)}</SelectItem>)}</SelectContent></Select></Field><Field name="Email"><Input type="email" value={contactEmail} onChange={e=>setContactEmail(e.target.value)} placeholder="email@example.com"/></Field><Field name="Phone (optional)"><Input type="tel" value={phone} onChange={e=>setPhone(e.target.value)} placeholder="+44 7700 900123"/></Field><Field name="Property (optional)"><Select value={propertyID||"none"} onValueChange={v=>setPropertyID(v==="none"?"":v)}><SelectTrigger><SelectValue/></SelectTrigger><SelectContent><SelectItem value="none">No property</SelectItem>{properties.map(p=><SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>)}</SelectContent></Select></Field><Field name="Notes"><Textarea maxLength={2000} value={notes} onChange={e=>setNotes(e.target.value)}/></Field></>}
     {dialog==="member"&&<><Field name="Teammate email"><Input type="email" autoComplete="email" required maxLength={254} value={inviteEmail} onChange={e=>setInviteEmail(e.target.value)} placeholder="colleague@example.com" disabled={!!invitationLink}/></Field><Field name="Role"><Select value={role} onValueChange={v=>setRole(v as typeof role)} disabled={!!invitationLink}><SelectTrigger><SelectValue/></SelectTrigger><SelectContent><SelectItem value="inspector">Inspector</SelectItem>{activeOrg?.role==="owner"&&<SelectItem value="manager">Manager</SelectItem>}</SelectContent></Select></Field>{invitationLink&&<div className="rounded-xl border border-blue-100 bg-blue-50 p-4"><p className="mb-2 text-sm font-semibold text-blue-800">Invite created</p><p className="break-all text-xs text-blue-700">{invitationLink}</p><Button type="button" variant="outline" className="mt-3 w-full" onClick={()=>{void navigator.clipboard.writeText(invitationLink);setNotice("Invitation link copied.");}}>Copy invitation link</Button><p className="mt-2 text-xs text-blue-600">Share this link securely; it is shown only once. The invited teammate must verify their email.</p></div>}</>}
     {error&&<p role="alert" className="text-sm text-red-600">{error}</p>}
     <div className="flex justify-end gap-2 pt-2"><Button type="button" variant="outline" onClick={()=>{setDialog(null);setError("");}}>Cancel</Button><Button type="submit" disabled={saving||!!invitationLink||(dialog==="inspection"&&!propertyID)}>{saving?"Saving…":dialog==="member"?"Send invitation":"Create"}<ArrowRight className="size-4"/></Button></div>
    </form>
   </DialogContent>
  </Dialog>
 </div>;
}
