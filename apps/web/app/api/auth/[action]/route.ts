import {NextRequest,NextResponse} from "next/server";
import {fetchBackend,sameOrigin} from "@/lib/server-api";
import {cookies} from "next/headers";

const routes:Record<string,string>={"request":"otp/request","verify":"otp/verify","password-login":"password/login","recovery-request":"recovery/request","recovery-complete":"recovery/complete"};
export async function POST(req:NextRequest,{params}:{params:Promise<{action:string}>}){
 const {action}=await params;
 if(!sameOrigin(req))return NextResponse.json({error:"Invalid origin"},{status:403});
 if(action==="logout"){
  const jar=await cookies();const token=jar.get("inspectly_session")?.value;
  if(token){await fetchBackend("v1/auth/logout",{method:"POST",headers:{"Authorization":"Bearer "+token}});}
  const response=NextResponse.json({ok:true});response.cookies.delete("inspectly_session");return response;
 }
 const backendRoute=routes[action];
 if(!backendRoute)return NextResponse.json({error:"Not found"},{status:404});
 let body:string;
 try{body=await req.text();if(body.length>4096)throw Error("too large");}
 catch{return NextResponse.json({error:"Invalid payload"},{status:400});}
 const result=await fetchBackend("v1/auth/"+backendRoute,{method:"POST",body,headers:{"Content-Type":"application/json"}});
 const data=await result.json().catch(()=>({error:"Unexpected backend response"}));
 if(!result.ok)return NextResponse.json(data,{status:result.status});
 if(action==="request"){
  // Never expose dev-only OTP secrets to the browser; production provider required.
  return NextResponse.json({status:"accepted",...((process.env.NODE_ENV==="development"||(process.env.WEB_LOCAL_OTP==="true"&&["localhost","127.0.0.1"].includes(new URL("http://"+(req.headers.get("host")||"invalid")).hostname)))&&typeof data.dev_code==="string"?{dev_code:data.dev_code}:{})},{status:202});
 }
 if(action==="recovery-request"){
  const host=req.headers.get("host")||"";
  const local=process.env.WEB_LOCAL_OTP==="true"&&["localhost","127.0.0.1"].includes(new URL("http://"+host).hostname);
  if(!local){delete data.dev_token;delete data.dev_code;}
  return NextResponse.json(data,{status:202,headers:{"Cache-Control":"no-store"}});
 }
 if(action==="recovery-complete"){
  const response=NextResponse.json(data,{status:200,headers:{"Cache-Control":"no-store"}});
  response.cookies.delete("inspectly_session");return response;
 }
 const token=typeof data.access_token==="string"?data.access_token:"";
 const expiresAt=new Date(data.expires_at);
 if(!token||!Number.isFinite(expiresAt.getTime()))return NextResponse.json({error:"Invalid auth response"},{status:502});
 const response=NextResponse.json({ok:true});
 response.cookies.set("inspectly_session",token,{
  httpOnly:true,secure:process.env.NODE_ENV==="production"&&process.env.SESSION_COOKIE_SECURE!=="false",sameSite:"lax",path:"/",expires:expiresAt,
 });
 return response;
}
