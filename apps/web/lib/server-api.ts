// Only an ingress that authenticates itself and overwrites client metadata
// may supply the client IP. Never forward browser-provided X-Forwarded-For.
export function trustedClientHeaders(source?:Headers):Record<string,string>{
 const ingress=process.env.TRUSTED_INGRESS_SECRET||"";
 const proxy=process.env.TRUSTED_BFF_SECRET||"";
 const valid=(value:string)=>/^[a-fA-F0-9]{64}$/.test(value);
 if(!source||!valid(ingress)||!valid(proxy)||ingress===proxy)return {};
 const proof=source.get("x-inspectly-ingress-secret")||"";
 if(proof.length!==ingress.length)return {};
 let different=0;
 for(let i=0;i<ingress.length;i++)different|=proof.charCodeAt(i)^ingress.charCodeAt(i);
 if(different!==0)return {};
 const ip=source.get("x-inspectly-ingress-ip")||"";
 // Reject chains, ports with brackets, whitespace and duplicates. Go performs
 // the final IP parse and canonicalizes IPv6 addresses to a /64 scope.
 if(!/^[0-9a-fA-F:.]{2,45}$/.test(ip))return {};
 return {"X-Inspectly-Client-IP":ip,"X-Inspectly-Proxy-Secret":proxy};
}
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
export async function fetchBackend(path:string,init:RequestInit={},clientHeaders?:Headers){
 const headers=new Headers(init.headers);
 for(const name of ["X-Inspectly-Client-IP","X-Inspectly-Proxy-Secret","X-Inspectly-Ingress-IP","X-Inspectly-Ingress-Secret","X-Forwarded-For","X-Real-IP"])headers.delete(name);
 for(const [name,value] of Object.entries(trustedClientHeaders(clientHeaders)))headers.set(name,value);
 try{return await fetch(backendURL(path),{...init,headers,redirect:"error",cache:"no-store",signal:AbortSignal.timeout(/\/media\/[0-9a-f-]{36}\/confirm$/.test(path)?55000:12000)});}
 catch{return new Response(JSON.stringify({error:"Backend unavailable"}),{status:503,headers:{"Content-Type":"application/json"}});}
}
