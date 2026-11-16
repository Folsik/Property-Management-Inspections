"use client";
import * as React from "react";
import * as DialogPrimitive from "@radix-ui/react-dialog";
import {X} from "lucide-react";
import {cn} from "@/lib/utils";
export const Dialog=DialogPrimitive.Root;
export const DialogTrigger=DialogPrimitive.Trigger;
export const DialogClose=DialogPrimitive.Close;
export function DialogContent({className,children,...props}:React.ComponentPropsWithoutRef<typeof DialogPrimitive.Content>){
 return <DialogPrimitive.Portal><DialogPrimitive.Overlay className="fixed inset-0 z-50 bg-slate-950/45 backdrop-blur-[2px] data-[state=open]:animate-in" />
 <DialogPrimitive.Content className={cn("fixed left-1/2 top-1/2 z-50 w-[min(94vw,490px)] -translate-x-1/2 -translate-y-1/2 rounded-2xl bg-white p-6 shadow-2xl outline-none",className)} {...props}>
 {children}<DialogPrimitive.Close aria-label="Close" className="absolute right-4 top-4 rounded-lg p-1.5 text-slate-500 hover:bg-slate-100"><X className="size-4"/></DialogPrimitive.Close></DialogPrimitive.Content></DialogPrimitive.Portal>;
}
export const DialogTitle=DialogPrimitive.Title;
export const DialogDescription=DialogPrimitive.Description;
