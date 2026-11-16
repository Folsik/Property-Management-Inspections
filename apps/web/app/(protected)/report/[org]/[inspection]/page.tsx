"use client";
import {useEffect,useState} from "react";
import {useParams} from "next/navigation";
import Link from "next/link";
import {Building2,Printer,ArrowLeft,ClipboardList,AlertCircle,Loader2} from "lucide-react";
import {Button} from "@/components/ui/button";
import {Badge} from "@/components/ui/badge";
import {Card} from "@/components/ui/card";
import {endpoints} from "@/lib/api";
import {typeNames,statusNames} from "@/lib/types";
import type {Inspection,InspectionItem,Media} from "@/lib/types";

export default function PrintableReport(){
 const params=useParams<{org:string;inspection:string}>();
 const [inspection,setInspection]=useState<Inspection|null>(null);
 const [items,setItems]=useState<InspectionItem[]>([]);
 const [media,setMedia]=useState<Media[]>([]);
 const [error,setError]=useState("");
 const [loading,setLoading]=useState(true);
 useEffect(()=>{
  let active=true;
  Promise.all([endpoints.inspection(params.org,params.inspection),endpoints.items(params.org,params.inspection),endpoints.media(params.org,params.inspection)])
   .then(([detail,punch,attachments])=>{if(active){setInspection(detail);setItems(punch.items);setMedia(attachments.items);}})
   .catch(e=>{if(active)setError(e instanceof Error?e.message:"Failed to load inspection");})
   .finally(()=>{if(active)setLoading(false);});
  return()=>{active=false;};
 },[params.org,params.inspection]);
 if(loading)return <div className="flex min-h-screen items-center justify-center gap-2"><Loader2 className="animate-spin text-blue-600"/>Loading inspection data…</div>;
 if(error||!inspection)return <main className="mx-auto max-w-2xl p-10"><p role="alert" className="text-red-700">{error||"Inspection not found"}</p><Link href="/dashboard" className="mt-4 inline-block text-blue-600">Back to dashboard</Link></main>;
 const formattedDate=new Date(inspection.created_at).toLocaleDateString("en-GB",{day:"numeric",month:"long",year:"numeric"});
 return <main className="mx-auto max-w-[940px] px-5 py-10 print:max-w-none print:p-0">
  <div className="mb-8 flex flex-wrap items-center justify-between gap-3 print:hidden"><Link href="/dashboard" className="inline-flex items-center gap-2 text-sm font-bold text-blue-600"><ArrowLeft className="size-4"/>Back to dashboard</Link><Button onClick={()=>window.print()}><Printer className="size-4"/>Print / Save as PDF</Button></div>
  <Card className="overflow-hidden p-8 shadow-sm print:border-none print:p-0 print:shadow-none sm:p-12">
   <header className="flex flex-wrap items-start justify-between gap-5 border-b border-slate-200 pb-9"><div><div className="mb-7 flex items-center gap-2 text-xl font-extrabold text-blue-700"><Building2 className="size-7"/>inspectly.</div><p className="text-xs font-bold uppercase tracking-widest text-blue-600">Property condition report</p><h1 className="mt-3 text-3xl font-bold tracking-tight">{inspection.property_name}</h1><p className="mt-2 text-slate-500">{typeNames[inspection.type]} inspection · {formattedDate}</p></div><Badge tone={inspection.status==="completed"?"green":"amber"}>{statusNames[inspection.status]}</Badge></header>
   <section className="grid grid-cols-2 gap-4 border-b border-slate-200 py-8 sm:grid-cols-3"><div><p className="text-xs font-semibold uppercase text-slate-500">Inspection type</p><p className="mt-2 font-bold">{typeNames[inspection.type]}</p></div><div><p className="text-xs font-semibold uppercase text-slate-500">Punch-list items</p><p className="mt-2 text-xl font-bold">{inspection.total_items}</p></div><div><p className="text-xs font-semibold uppercase text-slate-500">Outstanding</p><p className="mt-2 text-xl font-bold">{inspection.open_items}</p></div></section>
   <section className="py-9"><h2 className="mb-6 flex items-center gap-2 text-lg font-bold"><ClipboardList className="size-5 text-blue-600"/>Inspection findings</h2>{items.length?<div className="space-y-3">{items.map((item,index)=><div key={item.id} className="break-inside-avoid rounded-xl border border-slate-200 p-5"><div className="flex flex-wrap items-start justify-between gap-2"><h3 className="font-bold">{index+1}. {item.title}</h3><div className="flex gap-2"><Badge tone={item.severity==="critical"?"red":"slate"}>{item.severity}</Badge><Badge tone={item.status==="resolved"?"green":"amber"}>{item.status.replaceAll("_"," ")}</Badge></div></div>{item.notes&&<p className="mt-3 whitespace-pre-wrap text-sm leading-relaxed text-slate-600">{item.notes}</p>}</div>)}</div>:<p className="text-sm text-slate-500">No inspection findings have been recorded.</p>}</section>
   <section className="border-t border-slate-200 py-8"><h2 className="mb-3 text-lg font-bold">Attachments</h2><p className="text-sm text-slate-500">{media.length} media record{media.length!==1?"s":""} registered. File previews are not included in this print-ready report until secure media retrieval is implemented.</p></section>
   <footer className="border-t border-slate-200 pt-6 text-xs leading-relaxed text-slate-500">Generated directly from live inspection records. This is a client-side print-ready report, not an AI-generated or certified PDF. No data was invented for this document.</footer>
  </Card>
  <style jsx global>{`@media print { @page {size:A4; margin:18mm} body {background:white!important} }`}</style>
 </main>;
}
