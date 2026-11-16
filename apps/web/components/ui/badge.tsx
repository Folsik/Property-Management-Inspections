import * as React from "react";
import {cn} from "@/lib/utils";
type Tone="slate"|"blue"|"green"|"amber"|"red"|"violet";
const tones:Record<Tone,string>={slate:"bg-slate-100 text-slate-600",blue:"bg-blue-50 text-blue-700",green:"bg-emerald-50 text-emerald-700",amber:"bg-amber-50 text-amber-700",red:"bg-red-50 text-red-700",violet:"bg-violet-50 text-violet-700"};
export function Badge({tone="slate",className,...props}:React.HTMLAttributes<HTMLSpanElement>&{tone?:Tone}){return <span className={cn("inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-[11px] font-semibold capitalize",tones[tone],className)} {...props}/>;}
