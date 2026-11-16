"use client";
import {useEffect,useState} from "react";
import {useParams,useRouter} from "next/navigation";
import {ArrowRight,ShieldCheck,Building2,Loader2} from "lucide-react";
import {Card} from "@/components/ui/card";
import {Button} from "@/components/ui/button";
export default function InvitationPage(){
 const {token}=useParams<{token:string}>();
 const router=useRouter();
 const [invite,setInvite]=useState<{organization_name:string;role:string}|null>(null);
 const [error,setError]=useState("");
 useEffect(()=>{
  let active=true;
  fetch("/api/public/invitations/"+token,{cache:"no-store"}).then(async r=>{
   const data=await r.json();
   if(!r.ok)throw new Error(data.error||"Invitation expired or unavailable");
   if(active)setInvite(data);
  }).catch(e=>{if(active)setError(e.message);});
  return()=>{active=false;};
 },[token]);
 const next="/onboarding?invite="+token;
 return <main className="flex min-h-screen items-center justify-center bg-slate-50 px-5">
  <Card className="w-full max-w-md p-8 text-center">
   <div className="mx-auto mb-5 flex size-14 items-center justify-center rounded-2xl bg-blue-50 text-blue-600"><Building2/></div>
   <p className="mb-2 text-xs font-bold uppercase tracking-widest text-blue-600">Team invitation</p>
   {error?<><h1 className="text-xl font-bold">Invitation unavailable</h1><p className="mt-3 text-sm text-red-600">{error}</p></>:
    invite?<><h1 className="text-2xl font-bold">Join {invite.organization_name}</h1><p className="mt-3 text-sm text-slate-500">You were invited as <strong>{invite.role}</strong>. Verify the email that received this invitation before joining.</p><Button className="mt-6 w-full" onClick={()=>router.push("/login?next="+encodeURIComponent(next))}>Continue securely <ArrowRight className="size-4"/></Button></>:
    <p className="flex justify-center gap-2 text-sm text-slate-500"><Loader2 className="size-4 animate-spin"/>Checking invitation…</p>}
   <p className="mt-5 flex items-center justify-center gap-2 text-xs text-slate-400"><ShieldCheck className="size-4"/>Single-use invitation. Expires after 7 days.</p>
  </Card>
 </main>;
}
