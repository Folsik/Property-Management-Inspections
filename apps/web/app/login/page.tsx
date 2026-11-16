"use client";
import {FormEvent,useState} from "react";
import {useRouter} from "next/navigation";
import {ArrowRight,Building2,CheckCircle2,ClipboardCheck,LockKeyhole,ShieldCheck} from "lucide-react";
import {Button} from "@/components/ui/button";
import {Input} from "@/components/ui/input";
import {Card} from "@/components/ui/card";

export default function Login(){
 const router=useRouter();
 const [method,setMethod]=useState<"otp"|"pin">("otp");
 const [step,setStep]=useState<"phone"|"verify">("phone");
 const [phone,setPhone]=useState("");
 const [code,setCode]=useState("");
 const [pin,setPin]=useState("");
 const [error,setError]=useState("");
 const [developmentCode,setDevelopmentCode]=useState("");
 const [pending,setPending]=useState(false);
 async function submit(event:FormEvent){
  event.preventDefault();setPending(true);setError("");
  const path=method==="pin"?"pin-login":step==="phone"?"request":"verify";
  const body=method==="pin"?{phone,pin}:step==="phone"?{phone}:{phone,code};
  try{
   const res=await fetch("/api/auth/"+path,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(body)});
   const data=await res.json();
   if(!res.ok)throw Error(data.error||"Authentication failed");
   if(method==="otp"&&step==="phone"){setDevelopmentCode(typeof data.dev_code==="string"?data.dev_code:"");setStep("verify");return;}
   router.replace("/dashboard");router.refresh();
  }catch(err){setError(err instanceof Error?err.message:"Could not sign in");}
  finally{setPending(false);}
 }
 return <div className="min-h-screen lg:grid lg:grid-cols-[1fr_1fr]">
  <div className="relative hidden flex-col justify-between overflow-hidden bg-[#0c2040] p-14 text-white lg:flex">
   <div className="absolute -right-40 top-1/4 size-[600px] rounded-full border border-blue-400/20 bg-blue-500/10 blur-3xl"/>
   <div className="relative flex items-center gap-3 text-xl font-bold tracking-tight"><div className="flex size-10 items-center justify-center rounded-xl bg-blue-500"><Building2 className="size-5"/></div>Inspectly</div>
   <div className="relative max-w-lg space-y-8">
    <div className="inline-flex items-center gap-2 rounded-full border border-blue-300/20 bg-white/10 px-4 py-2 text-sm text-blue-100"><span className="size-2 rounded-full bg-emerald-400"/>Built for property teams</div>
    <h1 className="text-5xl font-semibold leading-[1.12] tracking-tight">Every property.<br/><span className="text-blue-300">Every detail.</span><br/>Under control.</h1>
    <p className="max-w-md text-lg leading-relaxed text-slate-300">One workspace for inspections, maintenance punch lists, people, and property operations.</p>
    <div className="grid max-w-md gap-4">{[
     ["Structured inspections","Document conditions and issues consistently."],
     ["Team collaboration","Keep owners, managers and inspectors aligned."],
     ["Secure by design","Organization-scoped access and encrypted sessions."]
    ].map(([title,desc])=><div key={title} className="flex items-start gap-3"><CheckCircle2 className="mt-1 size-5 shrink-0 text-blue-300"/><div><p className="font-semibold">{title}</p><p className="mt-1 text-sm text-slate-400">{desc}</p></div></div>)}</div>
   </div>
   <p className="relative text-sm text-slate-400">Property Management Inspections</p>
  </div>
  <div className="flex min-h-screen items-center justify-center px-5 py-12">
   <div className="w-full max-w-[420px]">
    <div className="mb-10 flex items-center gap-3 text-xl font-bold lg:hidden"><div className="flex size-10 items-center justify-center rounded-xl bg-blue-600 text-white"><Building2 className="size-5"/></div>Inspectly</div>
    <div className="mb-8"><div className="mb-5 flex size-12 items-center justify-center rounded-2xl bg-blue-50 text-blue-600">{method==="pin"?<LockKeyhole/>:<ClipboardCheck/>}</div>
     <h2 className="text-3xl font-bold tracking-tight">Welcome back</h2><p className="mt-2 text-slate-500">{step==="verify"?"Enter the 6-digit code sent to your phone.":"Sign in to manage your inspections and properties."}</p>
    </div>
    <Card className="p-6">
     <div className="mb-6 grid grid-cols-2 gap-1 rounded-xl bg-slate-100 p-1">
      <button className={"rounded-lg p-2 text-sm font-semibold "+(method==="otp"?"bg-white text-blue-700 shadow-sm":"text-slate-500")} onClick={()=>{setMethod("otp");setStep("phone");setError("");}}>SMS code</button>
      <button className={"rounded-lg p-2 text-sm font-semibold "+(method==="pin"?"bg-white text-blue-700 shadow-sm":"text-slate-500")} onClick={()=>{setMethod("pin");setStep("phone");setError("");}}>PIN</button>
     </div>
     <form className="space-y-5" onSubmit={submit}>
      <div><label htmlFor="phone" className="mb-2 block text-sm font-semibold">Phone number</label><Input id="phone" type="tel" autoComplete="tel" placeholder="+44 7700 900123" value={phone} onChange={e=>setPhone(e.target.value)} required pattern="\+[1-9][0-9]{7,14}" disabled={step==="verify"}/><p className="mt-2 text-xs text-slate-400">Use international E.164 format.</p></div>
      {developmentCode&&method==="otp"&&step==="verify"&&<p className="rounded-xl bg-amber-50 p-3 text-xs text-amber-800"><strong>Local development OTP:</strong> {developmentCode}. Configure Twilio for real SMS delivery.</p>}
      {method==="otp"&&step==="verify"&&<div><label htmlFor="code" className="mb-2 block text-sm font-semibold">Verification code</label><Input id="code" inputMode="numeric" autoComplete="one-time-code" maxLength={6} pattern="[0-9]{6}" placeholder="000000" value={code} onChange={e=>setCode(e.target.value)} required/></div>}
      {method==="pin"&&<div><label htmlFor="pin" className="mb-2 block text-sm font-semibold">6-digit PIN</label><Input id="pin" type="password" inputMode="numeric" autoComplete="current-password" maxLength={6} pattern="[0-9]{6}" placeholder="••••••" value={pin} onChange={e=>setPin(e.target.value)} required/></div>}
      {error&&<p role="alert" className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">{error}</p>}
      <Button type="submit" className="w-full" disabled={pending}>{pending?"Please wait…":method==="otp"&&step==="phone"?"Send verification code":"Sign in"}<ArrowRight className="size-4"/></Button>
      {step==="verify"&&<button type="button" onClick={()=>{setStep("phone");setCode("");}} className="text-sm font-medium text-blue-600">Change phone number</button>}
     </form>
    </Card>
    <div className="mt-6 flex items-center justify-center gap-2 text-xs text-slate-400"><ShieldCheck className="size-4"/> Your session is stored in a secure, HTTP-only cookie.</div>
   </div>
  </div>
 </div>;
}
