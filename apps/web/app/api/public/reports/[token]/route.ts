import {NextRequest,NextResponse} from "next/server";
import {fetchBackend} from "@/lib/server-api";

export async function GET(_req:NextRequest,{params}:{params:Promise<{token:string}>}){
 const {token}=await params;
 if(!/^[A-Za-z0-9_-]{43}$/.test(token))return NextResponse.json({error:"Invalid report link"},{status:404});
 const response=await fetchBackend("v1/public/reports/"+token);
 return new NextResponse(await response.text(),{status:response.status,headers:{
  "Content-Type":"application/json",
  "Cache-Control":"no-store",
  "Referrer-Policy":"no-referrer",
 }});
}
