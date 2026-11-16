export function backendURL(path:string){
 const base=process.env.GO_API_URL||"http://localhost:8080";
 // Fixed, server-controlled origin; user input must never set this value.
 return base.replace(/\/$/,"")+"/"+path.replace(/^\//,"");
}
export function sameOrigin(request:Request):boolean{
 const origin=request.headers.get("origin");
 if(!origin)return false;
 try{return new URL(origin).origin===new URL(request.url).origin;}catch{return false;}
}
export async function fetchBackend(path:string,init:RequestInit={}){
 try{return await fetch(backendURL(path),{...init,cache:"no-store",signal:AbortSignal.timeout(12000)});}
 catch{return new Response(JSON.stringify({error:"Backend unavailable"}),{status:503,headers:{"Content-Type":"application/json"}});}
}
