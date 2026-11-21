import {NextRequest,NextResponse} from "next/server";
import {fetchBackend,sameOrigin} from "@/lib/server-api";
import {clearAuthCookies,renewSession,sessionPair,setAuthCookies} from "@/lib/session";

const routes:Record<string,string>={
 "request":"otp/request","verify":"otp/verify","password-login":"password/login",
 "recovery-request":"recovery/request","recovery-complete":"recovery/complete"
};
export async function POST(req:NextRequest,{params}:{params:Promise<{action:string}>}){
 const {action}=await params;
 if(!sameOrigin(req))return NextResponse.json({error:"Invalid origin"},{status:403});
 if(action==="logout"){
  const refresh=req.cookies.get("inspectly_refresh")?.value;
  const access=req.cookies.get("inspectly_session")?.value;
  // Resolve even a just-rotated refresh cookie via the BFF single-flight result.
  // Failure to reach Go must not falsely claim that the device was signed out.
  let fresh=null;
  if(refresh){
   const renewal=await renewSession(refresh,req.headers);
   if(renewal.status==="unavailable")return NextResponse.json({error:"Unable to revoke session right now"},{status:503});
   if(renewal.status==="ok")fresh=renewal.pair;
  }
  if(fresh){
   const revoked=await fetchBackend("v1/auth/refresh/logout",{
    method:"POST",headers:{"Content-Type":"application/json"},
    body:JSON.stringify({refresh_token:fresh.refresh_token})
   },req.headers);
   if(!revoked.ok && revoked.status>=500){
    return setAuthCookies(NextResponse.json({error:"Unable to revoke session right now"},{status:503}),fresh);
   }
  }else if(access){
   const revoked=await fetchBackend("v1/auth/logout",{method:"POST",headers:{"Authorization":"Bearer "+access}},req.headers);
   if(!revoked.ok && revoked.status>=500){
    return NextResponse.json({error:"Unable to revoke session right now"},{status:503});
   }
  }
  return clearAuthCookies(NextResponse.json({ok:true}));
 }
 if(action==="refresh"){
  const refresh=req.cookies.get("inspectly_refresh")?.value;
  const renewal=refresh?await renewSession(refresh,req.headers):{status:"expired" as const};
  if(renewal.status==="unavailable")return NextResponse.json({error:"Authentication temporarily unavailable"},{status:503});
  if(renewal.status!=="ok")return clearAuthCookies(NextResponse.json({error:"Session expired"},{status:401}));
  return setAuthCookies(NextResponse.json({ok:true}),renewal.pair);
 }
 const backendRoute=routes[action];
 if(!backendRoute)return NextResponse.json({error:"Not found"},{status:404});
 let body:string;
 try{body=await req.text();if(body.length>4096)throw Error("too large");}
 catch{return NextResponse.json({error:"Invalid payload"},{status:400});}
 const result=await fetchBackend("v1/auth/"+backendRoute,{method:"POST",body,headers:{
  "Content-Type":"application/json","User-Agent":req.headers.get("user-agent")||"Inspectly Web"
 }},req.headers);
 const data=await result.json().catch(()=>({error:"Unexpected backend response"}));
 if(!result.ok)return NextResponse.json(data,{status:result.status});
 if(action==="request"){
  const host=req.headers.get("host")||"";
  const local=process.env.WEB_LOCAL_OTP==="true"&&["localhost","127.0.0.1"].includes(new URL("http://"+host).hostname);
  return NextResponse.json({status:"accepted",...(local&&typeof data.dev_code==="string"?{dev_code:data.dev_code}:{})},{status:202});
 }
 if(action==="recovery-request"){
  const host=req.headers.get("host")||"";
  const local=process.env.WEB_LOCAL_OTP==="true"&&["localhost","127.0.0.1"].includes(new URL("http://"+host).hostname);
  if(!local){delete data.dev_token;delete data.dev_code;}
  return NextResponse.json(data,{status:202,headers:{"Cache-Control":"no-store"}});
 }
 if(action==="recovery-complete"){
  return clearAuthCookies(NextResponse.json(data,{status:200,headers:{"Cache-Control":"no-store"}}));
 }
 const pair=sessionPair(data);
 if(!pair)return NextResponse.json({error:"Invalid authentication response"},{status:502});
 return setAuthCookies(NextResponse.json({ok:true}),pair);
}
