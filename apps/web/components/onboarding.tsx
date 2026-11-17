"use client";

import {FormEvent,useCallback,useEffect,useMemo,useState} from "react";
import {useRouter} from "next/navigation";
import {ArrowRight,Building2,Check,CheckCircle2,ChevronLeft,Home,KeyRound,Loader2,Mail,ShieldCheck,UserRound,Users} from "lucide-react";
import {Button} from "@/components/ui/button";
import {Card} from "@/components/ui/card";
import {Input} from "@/components/ui/input";
import {Select,SelectContent,SelectItem,SelectTrigger,SelectValue} from "@/components/ui/select";
import {COUNTRY_DIAL_CODES,countryFlag} from "@/lib/country-codes";
import {get,post,patch} from "@/lib/api";
import type {OnboardingStatus,Organization} from "@/lib/types";

type Step="password"|"email"|"organization"|"property"|"profile"|"invitation"|"finish";
type InvitePreview={organization_name:string;role:string;expires_at:string};
const passwordRules=[
 {label:"At least 12 characters",test:(value:string)=>value.length>=12&&new TextEncoder().encode(value).length<=72},
 {label:"An uppercase letter",test:(value:string)=>/[A-Z]/.test(value)},
 {label:"A number",test:(value:string)=>/[0-9]/.test(value)},
 {label:"A special character",test:(value:string)=>/[^\\w\\s]/.test(value)},
];
const milestones=[{label:"Secure your account",icon:KeyRound},{label:"Verify email",icon:Mail},{label:"Your workspace",icon:Building2},{label:"First property",icon:Home}];
function pickStep(state:OnboardingStatus,invitation:boolean):Step{
 if(!state.password_set)return "password";
 if(!state.email_verified)return "email";
 if(invitation)return state.full_name.trim()?"invitation":"profile";
 if(!state.has_organization)return "organization";
 if(state.owns_organization&&!state.has_property)return "property";
 return "finish";
}
const titles:Record<Step,[string,string]>={
 pin:["Create a secure PIN","Choose six digits you'll use to sign in next time."],
 email:["Verify your email","We'll send a six-digit code to your inbox."],
 organization:["Tell us about your workspace","Create a home for your properties, inspections and people."],
 property:["Add your first property","Start with the basics. You can add photos and details later."],
 profile:["How should we call you?","Complete your personal profile to accept the invitation."],
 invitation:["Join your team","Accept the invitation to start working with your colleagues."],
 finish:["Your workspace is ready","We'll take you straight to your dashboard."],
};
function Label({title,children}:{title:string;children:React.ReactNode}){
 return <label className="block space-y-2"><span className="text-sm font-semibold text-slate-700">{title}</span>{children}</label>;
}
export function Onboarding({token}:{token:string}){
 const router=useRouter();
 const [status,setStatus]=useState<OnboardingStatus|null>(null);
 const [organizations,setOrganizations]=useState<Organization[]>([]);
 const [invite,setInvite]=useState<InvitePreview|null>(null);
 const [loading,setLoading]=useState(true);
 const [busy,setBusy]=useState(false);
 const [error,setError]=useState("");
 const [password,setPassword]=useState("");const [confirmPassword,setConfirmPassword]=useState("");
 const [showPassword,setShowPassword]=useState(false);
 const [email,setEmail]=useState("");const [emailCode,setEmailCode]=useState("");
 const [emailSent,setEmailSent]=useState(false);
 const [changeEmail,setChangeEmail]=useState(false);const [localCode,setLocalCode]=useState("");
 const [fullName,setFullName]=useState("");
 const [kind,setKind]=useState<"individual"|"company">("individual");
 const [orgName,setOrgName]=useState("");
 const [country,setCountry]=useState("GB");
 const [propertyName,setPropertyName]=useState("");
 const [street,setStreet]=useState("");
 const [city,setCity]=useState("");
 const [unit,setUnit]=useState("");
 const [propertyCountry,setPropertyCountry]=useState("GB");
 const [newOrganization,setNewOrganization]=useState("");

 const refresh=useCallback(async()=>{
  const [state,orgs]=await Promise.all([get<OnboardingStatus>("v1/onboarding"),get<{items:Organization[]}>("v1/organizations")]);
  setStatus(state);setOrganizations(orgs.items);
  if(state.email)setEmail(old=>old||state.email||"");
  if(state.full_name)setFullName(old=>old||state.full_name);
  if(!newOrganization){const owned=orgs.items.find(o=>o.role==="owner");if(owned){setNewOrganization(owned.id);if(owned.country)setPropertyCountry(owned.country);}}
  return state;
 },[newOrganization]);

 useEffect(()=>{
  let active=true;
  async function initial(){
   try{
    const state=await refresh();
    if(!active)return;
    if(token){
     const response=await fetch("/api/public/invitations/"+token,{cache:"no-store"});
     const result=await response.json();
     if(!response.ok)throw Error(result.error||"This invitation is no longer available");
     if(active)setInvite(result);
    }else if(state.completed&&state.password_set)router.replace("/dashboard");
   }catch(e){if(active)setError(e instanceof Error?e.message:"Unable to load your account");}
   finally{if(active)setLoading(false);}
  }
  void initial();return()=>{active=false;};
 },[refresh,router,token]);

 const step=useMemo<Step>(()=>status?(changeEmail&&status.password_set?"email":pickStep(status,!!token)):"pin",[status,token,changeEmail]);
 const progress=step==="password"?0:step==="email"?1:step==="organization"||step==="profile"||step==="invitation"?2:3;
 const execute=async(work:()=>Promise<void>)=>{
  setBusy(true);setError("");
  try{await work();}
  catch(e){setError(e instanceof Error?e.message:"Request failed. Try again.");}
  finally{setBusy(false);}
 };
 const complete=async()=>{
  await post("v1/onboarding/complete",{});
  router.replace("/dashboard");router.refresh();
 };
 const savePIN=(event:FormEvent)=>{event.preventDefault();void execute(async()=>{
  if(!/^\d{6}$/.test(pin)||pin!==confirmPIN)throw Error("PINs must match and contain six digits.");
  await post("v1/auth/pin",{pin});
  setPin("");setConfirmPIN("");await refresh();
 });};
 const requestEmail=(event:FormEvent)=>{event.preventDefault();void execute(async()=>{
  const result=await post<{dev_code?:string}>("v1/email/request",{email:email.trim()});
  setEmailSent(true);setEmailCode("");setLocalCode(result.dev_code||"");
 });};
 const verifyEmail=(event:FormEvent)=>{event.preventDefault();void execute(async()=>{
  await post("v1/email/verify",{email:email.trim(),code:emailCode.trim()});
  setEmailSent(false);setLocalCode("");setChangeEmail(false);await refresh();
 });};
 const saveProfile=(event:FormEvent)=>{event.preventDefault();void execute(async()=>{
  if(!fullName.trim())throw Error("Please enter your name.");
  await patch("v1/me",{full_name:fullName.trim()});await refresh();
 });};
 const createOrg=(event:FormEvent)=>{event.preventDefault();void execute(async()=>{
  if(!fullName.trim()||!orgName.trim())throw Error("Your name and workspace name are required.");
  await patch("v1/me",{full_name:fullName.trim()});
  const org=await post<Organization>("v1/organizations",{name:orgName.trim(),kind,country});
  setNewOrganization(org.id);setPropertyCountry(country);await refresh();
 });};
 const createProperty=(event:FormEvent)=>{event.preventDefault();void execute(async()=>{
  const org=newOrganization||organizations.find(o=>o.role==="owner")?.id;
  if(!org)throw Error("Create your workspace before adding a property.");
  if(!street.trim()||!city.trim()||!propertyName.trim())throw Error("Enter property name, street and city.");
  await post(`v1/organizations/${org}/properties`,{name:propertyName.trim(),address:{street:street.trim(),city:city.trim(),country:propertyCountry,unit:unit.trim()}});
  await refresh();await complete();
 });};
 const joinTeam=()=>{void execute(async()=>{
  if(!token||!invite)throw Error("Invitation unavailable");
  await post(`v1/invitations/${token}/accept`,{});
  await refresh();await complete();
 });};

 if(loading)return <main className="flex min-h-screen items-center justify-center gap-3 text-sm text-slate-500"><Loader2 className="size-5 animate-spin"/>Preparing your workspace…</main>;
 return <main className="min-h-screen bg-gradient-to-br from-[#f4f8ff] via-white to-[#f9fbff] px-4 py-10 sm:py-16">
  <div className="mx-auto max-w-2xl">
   <div className="mb-10 flex items-center justify-between">
    <div className="flex items-center gap-3 text-lg font-bold text-[#0c2040]"><span className="flex size-10 items-center justify-center rounded-xl bg-blue-600 text-white"><Building2 className="size-5"/></span>Inspectly</div>
    <span className="rounded-full bg-blue-50 px-3 py-1 text-xs font-semibold text-blue-700">{token?"Team setup":"Account setup"}</span>
   </div>
   <div className="mb-8 grid grid-cols-4 gap-2" aria-label="Onboarding progress">
    {milestones.map((item,index)=>{const Icon=item.icon;return <div key={item.label} className="space-y-2">
      <div className={`flex h-1.5 rounded-full ${index<=progress?"bg-blue-600":"bg-slate-200"}`}/>
      <div className={`flex items-center gap-1.5 text-[10px] font-medium sm:text-xs ${index<=progress?"text-blue-700":"text-slate-400"}`}><Icon className="hidden size-3.5 sm:block"/>{item.label}</div>
    </div>;})}
   </div>
   <Card className="overflow-hidden p-6 shadow-xl shadow-blue-950/5 sm:p-10">
    <div className="mb-6 flex size-14 items-center justify-center rounded-2xl bg-blue-50 text-blue-600">
     {step==="password"?<KeyRound/>:step==="email"?<Mail/>:step==="organization"?<Building2/>:step==="property"?<Home/>:step==="profile"?<UserRound/>:<Users/>}
    </div>
    <h1 className="text-2xl font-bold tracking-tight text-[#0c2040] sm:text-3xl">{titles[step][0]}</h1>
    <p className="mb-8 mt-2 text-sm leading-relaxed text-slate-500">{titles[step][1]}</p>
    {error&&<div role="alert" className="mb-5 rounded-xl border border-red-100 bg-red-50 p-3 text-sm text-red-700">{error}</div>}
    {step==="password"&&<form className="space-y-5" onSubmit={savePassword}>
      <Label title="Create password"><Input type={showPassword?"text":"password"} autoComplete="new-password" minLength={12} maxLength={72} required placeholder="Create a unique password" value={password} onChange={e=>setPassword(e.target.value)}/></Label>
      <div className="grid gap-1.5">{passwordRules.map(rule=><p key={rule.label} className={`flex items-center gap-2 text-xs ${rule.test(password)?"text-emerald-700":"text-slate-500"}`}><Check className="size-3.5"/>{rule.label}</p>)}</div>
      <Label title="Confirm password"><Input type={showPassword?"text":"password"} autoComplete="new-password" minLength={12} maxLength={72} required placeholder="Repeat password" value={confirmPassword} onChange={e=>setConfirmPassword(e.target.value)}/></Label>
      <label className="flex items-center gap-2 text-xs text-slate-600"><input type="checkbox" checked={showPassword} onChange={e=>setShowPassword(e.target.checked)}/>Show passwords</label>
      <p className="text-xs text-slate-500">Avoid common phrases, personal information and repeated characters. A password manager is recommended.</p>
      <Button className="w-full" disabled={busy||!passwordRules.every(rule=>rule.test(password))||password!==confirmPassword}>{busy?"Saving…":"Save password & continue"}<ArrowRight className="size-4"/></Button>
    </form>}
    {step==="email"&&(!emailSent?<form className="space-y-5" onSubmit={requestEmail}>
      <Label title="Email address"><Input type="email" autoComplete="email" required maxLength={254} placeholder="you@example.com" value={email} onChange={e=>setEmail(e.target.value)}/></Label>
      <Button className="w-full" disabled={busy}>{busy?"Sending…":"Send verification code"}<ArrowRight className="size-4"/></Button>
    </form>:<form className="space-y-5" onSubmit={verifyEmail}>
      <p className="text-sm text-slate-600">Enter the code sent to <strong>{email}</strong>.</p>
      <Label title="Six-digit email code"><Input inputMode="numeric" autoComplete="one-time-code" pattern="[0-9]{6}" required maxLength={6} placeholder="000000" value={emailCode} onChange={e=>setEmailCode(e.target.value)}/></Label>
      {localCode&&<p className="rounded-lg bg-amber-50 p-3 text-xs text-amber-800">Local development email code: <strong>{localCode}</strong>. Production requires email provider credentials.</p>}
      <Button className="w-full" disabled={busy}>{busy?"Verifying…":"Verify email"}<Check className="size-4"/></Button>
      <button type="button" className="flex items-center gap-1 text-sm font-semibold text-blue-600" onClick={()=>{setEmailSent(false);setEmailCode("");setLocalCode("");}}><ChevronLeft className="size-4"/>Change email or resend code</button>
    </form>)}
    {step==="organization"&&<form className="space-y-5" onSubmit={createOrg}>
      <Label title="Your full name"><Input autoComplete="name" required maxLength={120} placeholder="Jane Smith" value={fullName} onChange={e=>setFullName(e.target.value)}/></Label>
      <Label title="I manage properties as"><Select value={kind} onValueChange={v=>setKind(v as "individual"|"company")}><SelectTrigger><SelectValue/></SelectTrigger><SelectContent><SelectItem value="individual">Private property owner</SelectItem><SelectItem value="company">Company / agency / management firm</SelectItem></SelectContent></Select></Label>
      <Label title={kind==="individual"?"Workspace name":"Company or organization name"}><Input required maxLength={120} placeholder={kind==="individual"?"My properties":"Acme Property Management"} value={orgName} onChange={e=>setOrgName(e.target.value)}/></Label>
      <Label title="Country of operation"><Select value={country} onValueChange={setCountry}><SelectTrigger><SelectValue/></SelectTrigger><SelectContent>{COUNTRY_DIAL_CODES.map(c=><SelectItem key={c.iso} value={c.iso}>{countryFlag(c.iso)} {c.name}</SelectItem>)}</SelectContent></Select></Label>
      <p className="text-xs text-slate-500">Company registration and tax details can be provided later if needed.</p>
      <Button className="w-full" disabled={busy}>{busy?"Creating…":"Create workspace"}<ArrowRight className="size-4"/></Button>
    </form>}
    {step==="property"&&<form className="space-y-5" onSubmit={createProperty}>
      <Label title="Property name"><Input required maxLength={160} placeholder="Flat 4B, Riverside House" value={propertyName} onChange={e=>setPropertyName(e.target.value)}/></Label>
      <Label title="Street and building"><Input required placeholder="12 River Street" value={street} onChange={e=>setStreet(e.target.value)}/></Label>
      <div className="grid gap-4 sm:grid-cols-2"><Label title="City"><Input required placeholder="London" value={city} onChange={e=>setCity(e.target.value)}/></Label><Label title="Unit / apartment (optional)"><Input placeholder="4B" value={unit} onChange={e=>setUnit(e.target.value)}/></Label></div>
      <Label title="Country"><Select value={propertyCountry} onValueChange={setPropertyCountry}><SelectTrigger><SelectValue/></SelectTrigger><SelectContent>{COUNTRY_DIAL_CODES.map(c=><SelectItem key={c.iso} value={c.iso}>{countryFlag(c.iso)} {c.name}</SelectItem>)}</SelectContent></Select></Label>
      <Button className="w-full" disabled={busy}>{busy?"Saving…":"Add property & open dashboard"}<ArrowRight className="size-4"/></Button>
    </form>}
    {step==="profile"&&<form className="space-y-5" onSubmit={saveProfile}>
      <Label title="Your full name"><Input required autoComplete="name" maxLength={120} placeholder="Jane Smith" value={fullName} onChange={e=>setFullName(e.target.value)}/></Label>
      <Button className="w-full" disabled={busy}>{busy?"Saving…":"Continue to invitation"}<ArrowRight className="size-4"/></Button>
    </form>}
    {step==="invitation"&&<div className="space-y-5">
      {invite?<div className="rounded-2xl border border-blue-100 bg-blue-50 p-5"><p className="font-bold text-blue-950">{invite.organization_name}</p><p className="mt-2 text-sm text-blue-700">Invited as {invite.role}. Only the verified email address that received this invitation can accept.</p></div>:<p className="text-sm text-red-600">Invitation is invalid, expired or has already been accepted.</p>}
      <p className="text-xs text-slate-500">Signed in with {status?.email}. If this does not match the invited address, verify the correct email first.</p>
      <button type="button" className="text-sm font-semibold text-blue-600 hover:underline" onClick={()=>{setChangeEmail(true);setEmailSent(false);setEmailCode("");setEmail("");setError("");}}>Verify a different email address</button>
      <Button className="w-full" disabled={busy||!invite} onClick={joinTeam}>{busy?"Joining…":"Accept invitation & continue"}<ArrowRight className="size-4"/></Button>
    </div>}
    {step==="finish"&&<div className="space-y-5"><div className="flex items-start gap-3 rounded-xl bg-green-50 p-4 text-sm text-green-800"><CheckCircle2 className="size-5 shrink-0"/>Your email, profile and workspace are configured.</div><Button className="w-full" disabled={busy} onClick={()=>void execute(complete)}>{busy?"Opening…":"Go to dashboard"}<ArrowRight className="size-4"/></Button></div>}
   </Card>
   <p className="mt-7 flex items-center justify-center gap-2 text-xs text-slate-400"><ShieldCheck className="size-4"/>Your progress is saved securely. You can return at any time.</p>
  </div>
 </main>;
}
