"use client";
import {FormEvent,useState} from "react";
import {useRouter} from "next/navigation";
import {ArrowRight,Building2,CheckCircle2,ChevronsUpDown,Eye,EyeOff,Search,ShieldCheck} from "lucide-react";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {Card} from "@/components/ui/card";
import {Dialog,DialogContent,DialogDescription,DialogTitle} from "@/components/ui/dialog";
import {COUNTRY_DIAL_CODES,countryFlag} from "@/lib/country-codes";
import {formatE164} from "@/lib/phone";
import Link from "next/link";

export default function Login(){
 const router=useRouter();
 const [mode,setMode]=useState<"signin"|"signup">("signin");
 const [step,setStep]=useState<"phone"|"verify">("phone");
 const [countryISO,setCountryISO]=useState("US");
 const [nationalPhone,setNationalPhone]=useState("");
 const [identifier,setIdentifier]=useState("");
 const [password,setPassword]=useState("");
 const [visible,setVisible]=useState(false);
 const [countryDialog,setCountryDialog]=useState(false);
 const [countrySearch,setCountrySearch]=useState("");
 const selected=COUNTRY_DIAL_CODES.find(c=>c.iso===countryISO)||COUNTRY_DIAL_CODES[0];
 const phone=formatE164(selected.dial,nationalPhone);
 const matches=COUNTRY_DIAL_CODES.filter(c=>(c.name+" "+c.iso+" "+c.dial).toLowerCase().includes(countrySearch.trim().toLowerCase()));
 const [code,setCode]=useState("");
 const [devCode,setDevCode]=useState("");
 const [pending,setPending]=useState(false);
 const [error,setError]=useState("");
 function switchMode(next:"signin"|"signup"){
  setMode(next);setStep("phone");setError("");setDevCode("");setCode("");setPassword("");
 }
 async function submit(event:FormEvent){
  event.preventDefault();setError("");
  if(mode==="signup"&&!phone){setError("Choose the country and enter a complete phone number.");return;}
  setPending(true);
  const action=mode==="signin"?"password-login":step==="phone"?"request":"verify";
  const payload=mode==="signin"?{identifier:identifier.trim(),password}:step==="phone"?{phone}:{phone,code};
  try{
   const result=await fetch("/api/auth/"+action,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(payload)});
   const data=await result.json();
   if(!result.ok)throw Error(data.error||"Authentication failed");
   if(mode==="signup"&&step==="phone"){setDevCode(typeof data.dev_code==="string"?data.dev_code:"");setStep("verify");return;}
   const next=new URLSearchParams(window.location.search).get("next");
   router.replace(next&&/^\/onboarding\?invite=[0-9a-f]{64}$/.test(next)?next:"/onboarding");router.refresh();
  }catch(e){setError(e instanceof Error?e.message:"Unable to authenticate");}
  finally{setPending(false);}
 }
 return <div className="min-h-screen lg:grid lg:grid-cols-2">
  <div className="relative hidden min-h-screen flex-col justify-between overflow-hidden bg-[#0c2040] p-14 text-white lg:flex">
   <div className="absolute -right-40 top-1/4 size-[600px] rounded-full border border-blue-400/20 bg-blue-500/10 blur-3xl"/>
   <div className="relative flex items-center gap-3 text-xl font-bold"><span className="flex size-10 items-center justify-center rounded-xl bg-blue-500"><Building2 className="size-5"/></span>Inspectly</div>
   <div className="relative max-w-lg space-y-7">
    <p className="inline-flex items-center gap-2 rounded-full border border-blue-300/20 bg-white/10 px-4 py-2 text-sm text-blue-100">Property management, simplified</p>
    <h1 className="text-5xl font-semibold leading-tight tracking-tight">Every property.<br/><span className="text-blue-300">Every detail.</span><br/>Under control.</h1>
    <p className="max-w-md text-lg text-slate-300">One workspace for inspections, property operations and your whole team.</p>
    {["Structured property inspections","Secure team collaboration","One place for every property"].map(value=><p key={value} className="flex items-center gap-3 text-sm text-slate-200"><CheckCircle2 className="size-5 text-blue-300"/>{value}</p>)}
   </div>
   <p className="relative text-sm text-slate-400">Property Management Inspections</p>
  </div>
  <div className="flex min-h-screen items-center justify-center px-5 py-12">
   <div className="w-full max-w-[430px]">
    <div className="mb-10 flex items-center gap-3 text-xl font-bold lg:hidden"><span className="flex size-10 items-center justify-center rounded-xl bg-blue-600 text-white"><Building2 className="size-5"/></span>Inspectly</div>
    <h1 className="text-3xl font-bold tracking-tight text-[#0c2040]">{mode==="signin"?"Welcome back":"Create your account"}</h1>
    <p className="mb-7 mt-2 text-sm text-slate-500">{mode==="signin"?"Sign in with your email or phone number.":step==="verify"?"Enter the six-digit SMS code to verify your phone.":"Register securely to start managing properties."}</p>
    <Card className="p-6">
     <div className="mb-6 grid grid-cols-2 rounded-xl bg-slate-100 p-1" role="tablist" aria-label="Authentication">
      <button type="button" role="tab" aria-selected={mode==="signin"} className={"rounded-lg px-4 py-2.5 text-sm font-semibold "+(mode==="signin"?"bg-white text-blue-700 shadow-sm":"text-slate-500")} onClick={()=>switchMode("signin")}>Sign in</button>
      <button type="button" role="tab" aria-selected={mode==="signup"} className={"rounded-lg px-4 py-2.5 text-sm font-semibold "+(mode==="signup"?"bg-white text-blue-700 shadow-sm":"text-slate-500")} onClick={()=>switchMode("signup")}>Sign up</button>
     </div>
     <form onSubmit={submit} className="space-y-5">
      {mode==="signin"?<>
       <label className="block space-y-2"><span className="text-sm font-semibold">Email or phone number</span><Input autoComplete="username" required value={identifier} onChange={e=>setIdentifier(e.target.value)} placeholder="you@example.com or +447700900123"/></label>
       <label className="block space-y-2"><span className="text-sm font-semibold">Password</span><div className="relative"><Input type={visible?"text":"password"} autoComplete="current-password" required value={password} onChange={e=>setPassword(e.target.value)} placeholder="Enter your password" className="pr-12"/><button type="button" aria-label={visible?"Hide password":"Show password"} className="absolute right-3 top-3 text-slate-500" onClick={()=>setVisible(v=>!v)}>{visible?<EyeOff className="size-5"/>:<Eye className="size-5"/>}</button></div></label>
       <div className="text-right"><Link href="/forgot-password" className="text-sm font-semibold text-blue-600 hover:underline">Forgot password?</Link></div>
      </>:<>
       <label className="block space-y-2"><span className="text-sm font-semibold">Phone number</span><div className="flex gap-2"><button type="button" disabled={step==="verify"} aria-label="Choose country calling code" onClick={()=>{setCountryDialog(true);setCountrySearch("");}} className="flex h-11 min-w-[126px] items-center justify-between rounded-xl border border-slate-200 bg-white px-3 text-sm font-semibold disabled:opacity-50">{countryFlag(selected.iso)} {selected.dial}<ChevronsUpDown className="size-4 text-slate-400"/></button><Input type="tel" autoComplete="tel-national" inputMode="tel" required placeholder="National phone number" value={nationalPhone} disabled={step==="verify"} onChange={e=>setNationalPhone(e.target.value)}/></div></label>
       {phone&&<p className="text-xs text-blue-600">International number: {phone}</p>}
       {step==="verify"&&<><label className="block space-y-2"><span className="text-sm font-semibold">SMS verification code</span><Input autoComplete="one-time-code" inputMode="numeric" required maxLength={6} pattern="[0-9]{6}" value={code} onChange={e=>setCode(e.target.value)} placeholder="000000"/></label>{devCode&&<p className="rounded-xl bg-amber-50 p-3 text-xs text-amber-800"><strong>Local development code:</strong> {devCode}</p>}</>}
      </>}
      {error&&<p role="alert" className="rounded-lg bg-red-50 p-3 text-sm text-red-700">{error}</p>}
      <Button type="submit" className="w-full" disabled={pending}>{pending?"Please wait…":mode==="signin"?"Sign in":step==="phone"?"Send SMS code":"Verify phone & continue"}<ArrowRight className="size-4"/></Button>
      {mode==="signup"&&step==="verify"&&<button type="button" className="text-sm font-medium text-blue-600" onClick={()=>{setStep("phone");setCode("");setDevCode("");}}>Change phone number</button>}
     </form>
    </Card>
    <p className="mt-6 flex items-center justify-center gap-2 text-xs text-slate-400"><ShieldCheck className="size-4"/>Credentials are sent over a protected session.</p>
    <p className="mt-3 text-center text-xs text-slate-500">Existing PIN account? Use Sign up to verify your phone once and create a password.</p>
   </div>
  </div>
  <Dialog open={countryDialog} onOpenChange={setCountryDialog}><DialogContent className="max-h-[85vh] overflow-hidden">
   <DialogTitle className="text-lg font-bold">Select your country</DialogTitle><DialogDescription>Choose your telephone calling code.</DialogDescription>
   <div className="relative mt-4"><Search className="absolute left-3 top-3.5 size-4 text-slate-400"/><Input autoFocus className="pl-10" placeholder="Search country or calling code" value={countrySearch} onChange={e=>setCountrySearch(e.target.value)}/></div>
   <div className="mt-3 max-h-[50vh] overflow-y-auto" role="listbox" aria-label="Countries">{matches.map(c=><button key={c.iso} type="button" role="option" aria-selected={c.iso===countryISO} onClick={()=>{setCountryISO(c.iso);setCountryDialog(false);setError("");}} className="flex w-full items-center gap-3 rounded-lg px-3 py-3 text-left text-sm hover:bg-blue-50"><span className="text-xl">{countryFlag(c.iso)}</span><span className="flex-1 truncate">{c.name}</span><span className="font-semibold text-slate-500">{c.dial}</span></button>)}</div>
  </DialogContent></Dialog>
 </div>;
}
