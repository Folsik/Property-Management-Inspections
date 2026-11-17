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
  if(refresh)await fetchBackend("v1/auth/refresh/logout",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({refresh_token:refresh})});
  if(access)await fetchBackend("v1/auth/logout",{method:"POST",headers:{"Authorization":"Bearer "+access}});
  return clearAuthCookies(NextResponse.json({ok:true}));
 }
 if(action==="refresh"){
  const refresh=req.cookies.get("inspectly_refresh")?.value;
  const pair=refresh?await renewSession(refresh):null;
  if(!pair)return clearAuthCookies(NextResponse.json({error:"Session expired"},{status:401}));
  return setAuthCookies(NextResponse.json({ok:true}),pair);
 }
 const backendRoute=routes[action];
 if(!backendRoute)return NextResponse.json({error:"Not found"},{status:404});
 let body:string;
 try{body=await req.text();if(body.length>4096)throw Error("too large");}
 catch{return NextResponse.json({error:"Invalid payload"},{status:400});}
 const result=await fetchBackend("v1/auth/"+backendRoute,{method:"POST",body,headers:{
  "Content-Type":"application/json","User-Agent":req.headers.get("user-agent")||"Inspectly Web"
 }});
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
