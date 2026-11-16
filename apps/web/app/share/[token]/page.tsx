"use client";
import {useEffect,useState} from "react";
import {useParams} from "next/navigation";
import {Building2,Printer,Shield,Loader2} from "lucide-react";
import {Button} from "@/components/ui/button";
import {Badge} from "@/components/ui/badge";
type PublicItem={title:string;notes:string;severity:string;status:string};
type PublicReport={inspection_type:string;status:string;property_name:string;created_at:string;items:PublicItem[]};
export default function SharedReport(){
 const {token}=useParams<{token:string}>();
 const [data,setData]=useState<PublicReport|null>(null);
 const [error,setError]=useState("");
 const [loading,setLoading]=useState(true);
 useEffect(()=>{
  let mounted=true;
  fetch("/api/public/reports/"+encodeURIComponent(token),{cache:"no-store"})
   .then(async response=>{const json=await response.json();if(!response.ok)throw Error(json.error||"Report unavailable");return json as PublicReport;})
   .then(json=>{if(mounted)setData(json);})
   .catch(e=>{if(mounted)setError(e instanceof Error?e.message:"Report unavailable");})
   .finally(()=>{if(mounted)setLoading(false);});
  return()=>{mounted=false};
 },[token]);
 return <main className="min-h-screen bg-slate-50 px-4 py-10 print:bg-white print:p-0">
  <div className="mx-auto max-w-[850px]">
   {loading?<div className="flex items-center justify-center gap-2 py-20 text-slate-500"><Loader2 className="animate-spin"/>Loading shared report…</div>:
   error||!data?<div className="rounded-2xl bg-white p-12 text-center"><Shield className="mx-auto mb-4 size-9 text-slate-400"/><h1 className="text-2xl font-bold">Report unavailable</h1><p role="alert" className="mt-3 text-slate-500">{error||"This report link has expired or been revoked."}</p></div>:
   <><div className="mb-5 flex items-center justify-between print:hidden"><span className="flex items-center gap-2 text-lg font-bold text-blue-700"><Building2 className="size-5"/>inspectly.</span><Button onClick={()=>window.print()}><Printer className="size-4"/>Print / PDF</Button></div>
   <article className="rounded-2xl border border-slate-200 bg-white p-7 shadow-sm print:border-0 print:p-0 print:shadow-none sm:p-12">
    <div className="mb-8 flex items-center gap-2 font-extrabold text-blue-700"><Building2 className="size-6"/>Inspectly · Shared report</div>
    <header className="border-b border-slate-200 pb-7"><p className="text-xs font-bold uppercase tracking-widest text-blue-600">Property inspection</p><h1 className="mt-3 text-3xl font-bold">{data.property_name}</h1><p className="mt-2 text-sm text-slate-500">{data.inspection_type.replaceAll("_"," ")} · {new Date(data.created_at).toLocaleDateString("en-GB")}</p><div className="mt-4"><Badge tone={data.status==="completed"?"green":"amber"}>{data.status.replaceAll("_"," ")}</Badge></div></header>
    <section className="py-8"><h2 className="mb-5 text-lg font-bold">Inspection findings ({data.items.length})</h2>{data.items.length===0?<p className="text-sm text-slate-500">No findings recorded.</p>:<div className="space-y-3">{data.items.map((item,index)=><div key={index} className="break-inside-avoid rounded-xl border border-slate-200 p-5"><div className="flex justify-between gap-4"><h3 className="font-semibold">{index+1}. {item.title}</h3><span className="text-xs font-semibold capitalize text-slate-500">{item.status.replaceAll("_"," ")} · {item.severity}</span></div>{item.notes&&<p className="mt-2 whitespace-pre-wrap text-sm text-slate-600">{item.notes}</p>}</div>)}</div>}</section>
    <footer className="border-t border-slate-200 pt-6 text-xs leading-relaxed text-slate-400">This limited report was shared by a property organization. The link is time-limited and may be revoked. All values come from saved inspection records.</footer>
   </article></>}
  </div>
  <style jsx global>{`@media print {@page{size:A4;margin:18mm} body{background:white!important}}`}</style>
 </main>;
}
