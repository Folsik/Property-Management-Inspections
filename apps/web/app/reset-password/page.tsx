"use client";
import {FormEvent,useEffect,useState} from "react";
import Link from "next/link";
import {ArrowRight,Building2,KeyRound,ShieldCheck} from "lucide-react";
import {Button} from "@/components/ui/button";
import {Card} from "@/components/ui/card";
import {Input} from "@/components/ui/input";

export default function ResetPassword(){
 const [token,setToken]=useState("");
 const [code,setCode]=useState("");
 const [password,setPassword]=useState("");
 const [confirm,setConfirm]=useState("");
 const [busy,setBusy]=useState(false);
 const [success,setSuccess]=useState(false);
 const [error,setError]=useState("");
 useEffect(()=>{setToken(new URLSearchParams(window.location.search).get("token")||"");},[]);
 async function submit(e:FormEvent){
  e.preventDefault();setError("");
  if(password!==confirm){setError("Passwords do not match.");return;}
  if(Array.from(password.normalize("NFC")).length<15){setError("Use at least 15 characters.");return;}
  setBusy(true);
  try{
   const res=await fetch("/api/auth/recovery-complete",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({token,code,password})});
   const data=await res.json();
   if(!res.ok)throw Error(data.error||"Invalid or expired recovery details");
   setSuccess(true);setPassword("");setConfirm("");setCode("");window.history.replaceState(null,"","/reset-password");
  }catch(e){setError(e instanceof Error?e.message:"Recovery failed");}finally{setBusy(false);}
 }
 return <main className="flex min-h-screen items-center justify-center bg-slate-50 px-5 py-12"><Card className="w-full max-w-md p-8">
  <Link href="/login" className="mb-8 flex items-center gap-2 text-lg font-bold text-slate-900"><Building2 className="text-blue-600"/>Inspectly</Link>
  <div className="mb-4 flex size-12 items-center justify-center rounded-xl bg-blue-50 text-blue-600"><KeyRound/></div>
  <h1 className="text-2xl font-bold">Choose a new password</h1>
  {success?<><p className="mt-3 text-sm text-emerald-700">Your password has been changed. Previous sessions were invalidated. Sign in again with your new password.</p><Link href="/login" className="mt-6 inline-flex font-semibold text-blue-600">Sign in <ArrowRight className="ml-2 size-4"/></Link></>:
  <><p className="mt-2 text-sm text-slate-500">Enter the six-digit code sent to your phone and choose a new passphrase. The email link and SMS code must both be valid.</p>
  <form className="mt-7 space-y-5" onSubmit={submit}>
   <label className="block space-y-2"><span className="text-sm font-semibold">SMS code</span><Input required autoComplete="one-time-code" inputMode="numeric" pattern="[0-9]{6}" maxLength={6} value={code} onChange={e=>setCode(e.target.value)} placeholder="000000"/></label>
   <label className="block space-y-2"><span className="text-sm font-semibold">New password</span><Input type="password" required autoComplete="new-password" minLength={15} value={password} onChange={e=>setPassword(e.target.value)} placeholder="At least 15 characters"/></label>
   <label className="block space-y-2"><span className="text-sm font-semibold">Confirm password</span><Input type="password" required autoComplete="new-password" minLength={15} value={confirm} onChange={e=>setConfirm(e.target.value)} placeholder="Repeat new password"/></label>
   <p className="text-xs text-slate-500">Spaces, Unicode and passphrases are allowed. Common passwords will be rejected.</p>
   {error&&<p role="alert" className="rounded-lg bg-red-50 p-3 text-sm text-red-700">{error}</p>}
   <Button className="w-full" disabled={busy||!token}>{busy?"Updating…":"Reset password"}<ArrowRight className="size-4"/></Button>
  </form></>}
  <Link href="/forgot-password" className="mt-7 inline-flex text-sm font-semibold text-blue-600">Start over</Link>
  <p className="mt-7 flex items-center gap-2 text-xs text-slate-400"><ShieldCheck className="size-4"/>Recovery links are single-use and expire after 10 minutes.</p>
 </Card></main>;
}
