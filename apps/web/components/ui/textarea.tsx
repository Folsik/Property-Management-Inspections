import * as React from "react";
import {cn} from "@/lib/utils";
export const Textarea=React.forwardRef<HTMLTextAreaElement,React.TextareaHTMLAttributes<HTMLTextAreaElement>>(({className,...props},ref)=>
 <textarea ref={ref} className={cn("flex min-h-24 w-full rounded-xl border border-slate-200 bg-white px-3.5 py-2.5 text-sm outline-none placeholder:text-slate-400 focus:border-blue-400 focus:ring-4 focus:ring-blue-50",className)} {...props}/>
);
Textarea.displayName="Textarea";
