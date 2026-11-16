import {NextRequest,NextResponse} from "next/server";
import {cookies} from "next/headers";
import {fetchBackend,sameOrigin} from "@/lib/server-api";

// A closed API surface: no arbitrary paths, URLs, or unscoped proxy access.
const allowed=[
 /^v1\/me$/,
 /^v1\/organizations$/,
 /^v1\/organizations\/[0-9a-f-]{36}$/,
 /^v1\/organizations\/[0-9a-f-]{36}\/(properties|inspections|members|jobs)$/,
 /^v1\/organizations\/[0-9a-f-]{36}\/inspections\/[0-9a-f-]{36}$/,
 /^v1\/organizations\/[0-9a-f-]{36}\/inspections\/[0-9a-f-]{36}\/(share|shares)$/,
 /^v1\/organizations\/[0-9a-f-]{36}\/shares\/[0-9a-f-]{36}$/,
 /^v1\/organizations\/[0-9a-f-]{36}\/inspections\/[0-9a-f-]{36}\/(items|media|media\/presign)$/,
 /^v1\/organizations\/[0-9a-f-]{36}\/inspections\/[0-9a-f-]{36}\/items\/[0-9a-f-]{36}$/,
];
async function handler(req:NextRequest,{params}:{params:Promise<{path:string[]}>}){
 const {path}=await params;
 const target=path.join("/");
 if(!allowed.some(pattern=>pattern.test(target)))return NextResponse.json({error:"Unknown endpoint"},{status:404});
 if(!["GET","POST","PATCH","DELETE"].includes(req.method))return NextResponse.json({error:"Method not allowed"},{status:405});
 if(req.method!=="GET"&&!sameOrigin(req))return NextResponse.json({error:"Invalid origin"},{status:403});
 const cookieJar=await cookies();
 const token=cookieJar.get("inspectly_session")?.value;
 if(!token)return NextResponse.json({error:"Unauthorized"},{status:401});
 let body: string|undefined;
 if(req.method!=="GET"){
  try{body=await req.text();if(body.length>1_048_576)throw Error("too large");}
  catch{return NextResponse.json({error:"Invalid request"},{status:400});}
 }
 const response=await fetchBackend(target,{method:req.method,headers:{
  "Authorization":"Bearer "+token,
  ...(body!==undefined?{"Content-Type":"application/json"}:{})
 },body});
 return new NextResponse(await response.text(),{status:response.status,headers:{
  "Content-Type":response.headers.get("content-type")||"application/json",
  "Cache-Control":"no-store",
 }});
}
export {handler as GET,handler as POST,handler as PATCH,handler as DELETE};
