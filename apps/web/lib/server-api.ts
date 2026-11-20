export function backendURL(path:string){
 const base=process.env.GO_API_URL||"http://localhost:8080";
 // Fixed, server-controlled origin; user input must never set this value.
 return base.replace(/\/$/,"")+"/"+path.replace(/^\//,"");
}
export function sameOrigin(request:Request):boolean{
 const origin=request.headers.get("origin");
 const host=request.headers.get("host");
 // Next.js may see an internal URL behind Docker / a reverse proxy.
 // Compare the browser's Origin with the public Host on the same request.
 // Do not trust x-forwarded-host as a substitute for Host.
 if(!origin||!host)return false;
 try{
  const parsed=new URL(origin);
  if(!["http:","https:"].includes(parsed.protocol)||parsed.origin!==origin)return false;
  if(parsed.username||parsed.password||parsed.pathname!=="/"||parsed.search||parsed.hash)return false;
  return parsed.host===new URL("http://"+host).host;
 }catch{return false;}
}
export async function fetchBackend(path:string,init:RequestInit={}){
 try{return await fetch(backendURL(path),{...init,cache:"no-store",signal:AbortSignal.timeout(/\/media\/[0-9a-f-]{36}\/confirm$/.test(path)?55000:12000)});}
 catch{return new Response(JSON.stringify({error:"Backend unavailable"}),{status:503,headers:{"Content-Type":"application/json"}});}
}
