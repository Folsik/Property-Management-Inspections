"use client";
import {FormEvent,useState} from "react";
import Link from "next/link";
import {ArrowRight,Building2,Mail,ShieldCheck} from "lucide-react";
import {Button} from "@/components/ui/button";
import {Card} from "@/components/ui/card";
import {Input} from "@/components/ui/input";

export default function ForgotPassword(){
 const [email,setEmail]=useState("");
 const [pending,setPending]=useState(false);
 const [done,setDone]=useState(false);
 const [error,setError]=useState("");
 const [devToken,setDevToken]=useState("");
 const [devCode,setDevCode]=useState("");
 async function submit(e:FormEvent){
  e.preventDefault();setPending(true);setError("");
  try{
   const response=await fetch("/api/auth/recovery-request",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({email})});
   const data=await response.json();
   if(!response.ok)throw Error(data.error||"Recovery is currently unavailable");
   setDevToken(typeof data.dev_token==="string"?data.dev_token:"");
   setDevCode(typeof data.dev_code==="string"?data.dev_code:"");
   setDone(true);
  }catch(e){setError(e instanceof Error?e.message:"Request failed");}finally{setPending(false);}
 }
 return <main className="flex min-h-screen items-center justify-center bg-slate-50 px-5 py-12">
  <Card className="w-full max-w-md p-8">
   <Link href="/login" className="mb-8 flex items-center gap-2 text-lg font-bold text-slate-900"><Building2 className="text-blue-600"/>Inspectly</Link>
   <div className="mb-4 flex size-12 items-center justify-center rounded-xl bg-blue-50 text-blue-600"><Mail/></div>
   <h1 className="text-2xl font-bold text-slate-900">Reset your password</h1>
   {!done?<><p className="mt-2 text-sm text-slate-500">Enter the verified email linked to your account. We will send a reset link to your email and a separate code by SMS.</p>
    <form className="mt-7 space-y-5" onSubmit={submit}><label className="block space-y-2"><span className="text-sm font-semibold">Verified email</span><Input type="email" autoComplete="email" required value={email} onChange={e=>setEmail(e.target.value)} placeholder="you@example.com"/></label>
     {error&&<p role="alert" className="rounded-lg bg-red-50 p-3 text-sm text-red-700">{error}</p>}
     <Button className="w-full" disabled={pending}>{pending?"Sending…":"Request recovery"}<ArrowRight className="size-4"/></Button>
    </form>
   </>:<><p className="mt-3 text-sm text-slate-600">If your account is eligible, we have sent recovery instructions. Check your email and phone. The link expires in 10 minutes.</p>
    {devToken&&<div className="mt-5 space-y-3 rounded-xl bg-amber-50 p-4 text-xs text-amber-900"><p className="font-bold">Local development recovery</p><Link className="break-words text-blue-700 underline" href={"/reset-password?token="+devToken}>Open the local reset link</Link>{devCode&&<p>Development SMS code: <strong>{devCode}</strong></p>}</div>}
    <p className="mt-5 text-xs text-slate-500">If the verified email or phone is unavailable, contact support for assisted identity review. Phone SMS alone cannot reset a password.</p>
   </>}
   <Link href="/login" className="mt-7 inline-flex text-sm font-semibold text-blue-600">Back to sign in</Link>
   <p className="mt-7 flex items-center gap-2 text-xs text-slate-400"><ShieldCheck className="size-4"/>An email link and phone code are both required.</p>
  </Card>
 </main>;
}
