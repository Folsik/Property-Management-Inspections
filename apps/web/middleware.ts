import {NextRequest,NextResponse} from "next/server";
import {accessNearlyExpired,clearAuthCookies,refreshedRequestCookies,renewSession,setAuthCookies} from "@/lib/session";

export async function middleware(req:NextRequest){
 const access=req.cookies.get("inspectly_session")?.value||"";
 const refresh=req.cookies.get("inspectly_refresh")?.value||"";
 if(access&&!accessNearlyExpired(access))return NextResponse.next();
 if(!refresh){
  const response=NextResponse.redirect(new URL("/login?session=expired",req.url));
  return clearAuthCookies(response);
 }
 const pair=await renewSession(refresh);
 if(!pair){
  const response=NextResponse.redirect(new URL("/login?session=expired",req.url));
  return clearAuthCookies(response);
 }
 const headers=new Headers(req.headers);
 headers.set("cookie",refreshedRequestCookies(req.headers.get("cookie")||"",pair));
 const response=NextResponse.next({request:{headers}});
 return setAuthCookies(response,pair);
}
export const config={matcher:["/dashboard/:path*","/onboarding/:path*","/report/:path*"]};
