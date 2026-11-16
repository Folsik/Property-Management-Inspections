import * as React from "react";
import {cn} from "@/lib/utils";
export function Card({className,...props}:React.HTMLAttributes<HTMLDivElement>){return <div className={cn("rounded-2xl border border-slate-200/80 bg-white shadow-[0_2px_12px_-8px_#0f172a30]",className)} {...props}/>;}
export function CardHeader({className,...props}:React.HTMLAttributes<HTMLDivElement>){return <div className={cn("flex flex-col gap-1 px-6 pt-6 pb-3",className)} {...props}/>;}
export function CardTitle({className,...props}:React.HTMLAttributes<HTMLHeadingElement>){return <h3 className={cn("text-base font-bold tracking-tight text-slate-900",className)} {...props}/>;}
export function CardContent({className,...props}:React.HTMLAttributes<HTMLDivElement>){return <div className={cn("px-6 pb-6",className)} {...props}/>;}
