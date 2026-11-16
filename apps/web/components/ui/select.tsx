"use client";
import * as React from "react";
import * as SelectPrimitive from "@radix-ui/react-select";
import {Check,ChevronDown} from "lucide-react";
import {cn} from "@/lib/utils";
export const Select=SelectPrimitive.Root;
export function SelectTrigger({className,children,...props}:React.ComponentPropsWithoutRef<typeof SelectPrimitive.Trigger>){
 return <SelectPrimitive.Trigger className={cn("inline-flex h-11 w-full items-center justify-between gap-2 rounded-xl border border-slate-200 bg-white px-3.5 text-sm text-slate-700 focus:outline-none focus:ring-4 focus:ring-blue-50",className)} {...props}>{children}<SelectPrimitive.Icon><ChevronDown className="size-4 text-slate-400"/></SelectPrimitive.Icon></SelectPrimitive.Trigger>;
}
export const SelectValue=SelectPrimitive.Value;
export function SelectContent({className,children,...props}:React.ComponentPropsWithoutRef<typeof SelectPrimitive.Content>){
 return <SelectPrimitive.Portal><SelectPrimitive.Content position="popper" sideOffset={4} className={cn("z-[60] max-h-64 min-w-[var(--radix-select-trigger-width)] overflow-auto rounded-xl border border-slate-200 bg-white p-1 shadow-xl",className)} {...props}><SelectPrimitive.Viewport>{children}</SelectPrimitive.Viewport></SelectPrimitive.Content></SelectPrimitive.Portal>;
}
export function SelectItem({className,children,...props}:React.ComponentPropsWithoutRef<typeof SelectPrimitive.Item>){
 return <SelectPrimitive.Item className={cn("relative flex cursor-pointer select-none items-center rounded-lg py-2 pl-8 pr-3 text-sm text-slate-700 outline-none focus:bg-blue-50 focus:text-blue-700",className)} {...props}><SelectPrimitive.ItemIndicator className="absolute left-2"><Check className="size-4"/></SelectPrimitive.ItemIndicator><SelectPrimitive.ItemText>{children}</SelectPrimitive.ItemText></SelectPrimitive.Item>;
}
