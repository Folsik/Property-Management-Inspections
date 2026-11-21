import {NextRequest,NextResponse} from "next/server";
import {clearAuthCookies,renewSession,setAuthCookies} from "@/lib/session";
import {fetchBackend,sameOrigin} from "@/lib/server-api";

// A closed API surface: no arbitrary paths, URLs, or unscoped proxy access.
const allowed=[
 /^v1\/me$/,
 /^v1\/auth\/sessions$/,
 /^v1\/auth\/sessions\/[0-9a-f-]{36}$/,
 /^v1\/onboarding$/,
 /^v1\/onboarding\/complete$/,
 /^v1\/email\/(request|verify)$/,
 /^v1\/auth\/password$/,
 /^v1\/auth\/password\/change$/,
 /^v1\/invitations\/[0-9a-f]{64}\/accept$/,
 /^v1\/organizations\/[0-9a-f-]{36}\/invitations$/,
 /^v1\/organizations\/[0-9a-f-]{36}\/invitations\/[0-9a-f-]{36}$/,
 /^v1\/organizations$/,
 /^v1\/organizations\/[0-9a-f-]{36}$/,
 /^v1\/organizations\/[0-9a-f-]{36}\/(properties|inspections|members|jobs|contacts)$/,
 /^v1\/organizations\/[0-9a-f-]{36}\/inspections\/[0-9a-f-]{36}$/,
 /^v1\/organizations\/[0-9a-f-]{36}\/inspections\/[0-9a-f-]{36}\/(share|shares)$/,
 /^v1\/organizations\/[0-9a-f-]{36}\/shares\/[0-9a-f-]{36}$/,
 /^v1\/organizations\/[0-9a-f-]{36}\/inspections\/[0-9a-f-]{36}\/(items|media|media\/presign)$/,
 /^v1\/organizations\/[0-9a-f-]{36}\/inspections\/[0-9a-f-]{36}\/items\/[0-9a-f-]{36}$/,
 /^v1\/organizations\/[0-9a-f-]{36}\/sync\/(pull|push)$/,
 /^v1\/organizations\/[0-9a-f-]{36}\/jobs\/metrics$/,
 /^v1\/organizations\/[0-9a-f-]{36}\/jobs\/[0-9a-f-]{36}\/retry$/,
 /^v1\/organizations\/[0-9a-f-]{36}\/inspections\/[0-9a-f-]{36}\/transcriptions$/,
 /^v1\/organizations\/[0-9a-f-]{36}\/inspections\/[0-9a-f-]{36}\/reports\/[0-9a-f-]{36}\/pdf$,
 /^v1\/organizations\/[0-9a-f-]{36}\/inspections\/[0-9a-f-]{36}\/media\/[0-9a-f-]{36}(?:\/confirm|\/download)?$/,
];
async function handler(req:NextRequest,{params}:{params:Promise<{path:string[]}>}){
 const {path}=await params;
 const target=path.join("/");
 if(!allowed.some(pattern=>pattern.test(target)))return NextResponse.json({error:"Unknown endpoint"},{status:404});
 if(!["GET","POST","PATCH","DELETE"].includes(req.method))return NextResponse.json({error:"Method not allowed"},{status:405});
 if(req.method!=="GET"&&!sameOrigin(req))return NextResponse.json({error:"Invalid origin"},{status:403});
 const current=req.cookies.get("inspectly_session")?.value||"";
 const refresh=req.cookies.get("inspectly_refresh")?.value||"";
 let body:string|undefined;
 if(req.method!=="GET"){
  try{body=await req.text();if(body.length>1_048_576)throw Error("too large");}
  catch{return NextResponse.json({error:"Invalid request"},{status:400});}
 }
 const queryParams=new URLSearchParams();
 for(const [key,value] of req.nextUrl.searchParams){
  if(key!=="cursor"&&key!=="limit")return NextResponse.json({error:"Unknown query parameter"},{status:400});
  if(value.length>512)return NextResponse.json({error:"Query parameter too large"},{status:400});
  queryParams.append(key,value);
 }
 const apiPath=target+(queryParams.size?"?"+queryParams.toString():"");
 const invoke=(token:string)=>fetchBackend(apiPath,{method:req.method,headers:{
  "Authorization":"Bearer "+token,
  ...(body!==undefined?{"Content-Type":"application/json"}:{})
 },body});
 let response=current?await invoke(current):null;
 let pair=null;
 // Retry once after a 401 (expired JWT or rotated session generation).
 // Each response carries both replacement HttpOnly cookies back to the browser.
 if((!response||response.status===401)&&refresh){
  const renewal=await renewSession(refresh);
  if(renewal.status==="unavailable")return NextResponse.json({error:"Authentication temporarily unavailable"},{status:503});
  if(renewal.status==="expired")return clearAuthCookies(NextResponse.json({error:"Session expired"},{status:401}));
  pair=renewal.pair;
  response=await invoke(pair.access_token);
 }
 if(!response)return NextResponse.json({error:"Unauthorized"},{status:401});
 // PDF reports are binary; response.text() would corrupt bytes above ASCII.
 const isPDF=target.endsWith("/pdf")&&response.ok&&response.headers.get("content-type")?.startsWith("application/pdf");
 if(isPDF){
  const bytes=await response.arrayBuffer();
  if(bytes.byteLength>1_048_576)return NextResponse.json({error:"Report too large"},{status:502});
  const pdf=new NextResponse(bytes,{status:response.status,headers:{
   "Content-Type":"application/pdf",
   "Content-Disposition":"attachment; filename=inspection-report.pdf",
   "X-Content-Type-Options":"nosniff",
   "Cache-Control":"private, no-store"
  }});
  return pair?setAuthCookies(pdf,pair):pdf;
 }
 let text=await response.text();
 if(target==="v1/email/request"){
  const host=req.headers.get("host")||"";
  const local=process.env.WEB_LOCAL_OTP==="true"&&["localhost","127.0.0.1"].includes(host.split(":")[0]);
  if(!local){try{const result=JSON.parse(text);delete result.dev_code;text=JSON.stringify(result);}catch{}}
 }
 const result=new NextResponse(text,{status:response.status,headers:{
  "Content-Type":response.headers.get("content-type")||"application/json",
  "Cache-Control":"no-store",
 }});
 return pair?setAuthCookies(result,pair):result;
}
export {handler as GET,handler as POST,handler as PATCH,handler as DELETE};
