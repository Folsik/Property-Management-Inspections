import {NextRequest,NextResponse} from "next/server";
import {fetchBackend} from "@/lib/server-api";
export async function GET(_request:NextRequest,{params}:{params:Promise<{token:string}>}){
 const {token}=await params;
 if(!/^[0-9a-f]{64}$/.test(token))return NextResponse.json({error:"Invitation not found"},{status:404});
 const res=await fetchBackend("v1/invitations/"+token);
 return new NextResponse(await res.text(),{status:res.status,headers:{"Content-Type":"application/json","Cache-Control":"no-store","Referrer-Policy":"no-referrer"}});
}
