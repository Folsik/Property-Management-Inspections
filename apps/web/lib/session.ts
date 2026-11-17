import {NextResponse} from "next/server";
import {fetchBackend} from "@/lib/server-api";

// These credentials are always HttpOnly cookies, never localStorage or a JS response body.
export type AuthPair={
 access_token:string;
 refresh_token:string;
 expires_at:string;
 refresh_expires_at:string;
 session_id:string;
};
export type RenewalResult={status:"ok";pair:AuthPair}|{status:"expired"|"unavailable"};
const flights=new Map<string,{promise:Promise<RenewalResult>;until:number}>();
function validPair(value:unknown):value is AuthPair{
 if(!value||typeof value!=="object")return false;
 const v=value as Partial<AuthPair>;
 return typeof v.access_token==="string"&&v.access_token.length>30&&
  typeof v.refresh_token==="string"&&v.refresh_token.length===43&&
  typeof v.session_id==="string"&&v.session_id.length===36&&
  typeof v.expires_at==="string"&&!Number.isNaN(Date.parse(v.expires_at))&&
  typeof v.refresh_expires_at==="string"&&!Number.isNaN(Date.parse(v.refresh_expires_at));
}
export function sessionPair(value:unknown):AuthPair|null{return validPair(value)?value:null;}
function secureCookies(){return process.env.NODE_ENV==="production"&&process.env.SESSION_COOKIE_SECURE!=="false";}
export function setAuthCookies(res:NextResponse,pair:AuthPair){
 const settings={httpOnly:true as const,secure:secureCookies(),sameSite:"lax" as const,path:"/"};
 res.cookies.set("inspectly_session",pair.access_token,{...settings,expires:new Date(pair.expires_at)});
 res.cookies.set("inspectly_refresh",pair.refresh_token,{...settings,expires:new Date(pair.refresh_expires_at)});
 res.headers.set("Cache-Control","no-store");
 return res;
}
export function clearAuthCookies(res:NextResponse){
 const settings={path:"/",httpOnly:true as const,secure:secureCookies(),sameSite:"lax" as const,maxAge:0};
 res.cookies.set("inspectly_session","",settings);
 res.cookies.set("inspectly_refresh","",settings);
 res.headers.set("Cache-Control","no-store");
 return res;
}
export async function renewSession(refreshToken:string):Promise<RenewalResult>{
 if(!/^[a-zA-Z0-9_-]{43}$/.test(refreshToken))return {status:"expired"};
 const existing=flights.get(refreshToken);
 if(existing&&existing.until>Date.now())return existing.promise;
 const promise=(async()=>{
  const res=await fetchBackend("v1/auth/refresh",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({refresh_token:refreshToken})});
  if(res.status===401||res.status===403)return {status:"expired" as const};
  if(!res.ok)return {status:"unavailable" as const};
  const data=await res.json().catch(()=>null);
  const pair=sessionPair(data);
  return pair?{status:"ok" as const,pair}:{status:"unavailable" as const};
 })().catch(()=>({status:"unavailable" as const}));
 flights.set(refreshToken,{promise,until:Date.now()+2500});
 // Retain the small result for concurrent requests using the same cookie,
 // so the BFF does not mistake its own overlapping requests for token theft.
 void promise.finally(()=>setTimeout(()=>{
  if(flights.get(refreshToken)?.promise===promise)flights.delete(refreshToken);
 },2500));
 return promise;
}
export function accessNearlyExpired(access:string,skewSeconds=45):boolean{
 try{
  const payload=access.split(".")[1];
  if(!payload)return true;
  const claim=JSON.parse(atob(payload.replace(/-/g,"+").replace(/_/g,"/"))) as {exp?:number};
  return typeof claim.exp!=="number"||claim.exp<=Math.floor(Date.now()/1000)+skewSeconds;
 }catch{return true;}
}
// Response header propagation for refreshed SSR requests. Cookie values here
// are validated opaque credentials produced by the Go auth service.
export function refreshedRequestCookies(original:string,pair:AuthPair){
 const rest=original.split(";").map(c=>c.trim()).filter(v=>v&&!v.startsWith("inspectly_session=")&&!v.startsWith("inspectly_refresh="));
 return [...rest,"inspectly_session="+pair.access_token,"inspectly_refresh="+pair.refresh_token].join("; ");
}
