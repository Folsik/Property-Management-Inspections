import type {Collection,Inspection,InspectionItem,Job,Media,Member,Organization,Profile,Property} from "@/lib/types";
export class ApiError extends Error{constructor(public status:number,message:string){super(message);this.name="ApiError";}}
export async function api<T>(path:string,init:RequestInit={}):Promise<T>{
 const response=await fetch("/api/backend/"+path.replace(/^\/+/, ""),{
  ...init,cache:"no-store",headers:{"Content-Type":"application/json",...init.headers},
 });
 if(response.status===401){window.location.assign("/login?session=expired");throw new ApiError(401,"Session expired");}
 let data:unknown=null;
 try{data=await response.json();}catch{}
 if(!response.ok){const value=data as {error?:string}|null;throw new ApiError(response.status,value?.error||"Request failed");}
 return data as T;
}
export const get=<T>(path:string)=>api<T>(path);
export const post=<T>(path:string,body:unknown)=>api<T>(path,{method:"POST",body:JSON.stringify(body)});
export const patch=<T>(path:string,body:unknown)=>api<T>(path,{method:"PATCH",body:JSON.stringify(body)});
export const endpoints={
 me:()=>get<Profile>("v1/me"),
 orgs:()=>get<Collection<Organization>>("v1/organizations"),
 properties:(org:string)=>get<Collection<Property>>(`v1/organizations/${org}/properties`),
 inspections:(org:string)=>get<Collection<Inspection>>(`v1/organizations/${org}/inspections`),
 inspection:(org:string,id:string)=>get<Inspection>(`v1/organizations/${org}/inspections/${id}`),
 items:(org:string,id:string)=>get<Collection<InspectionItem>>(`v1/organizations/${org}/inspections/${id}/items`),
 members:(org:string)=>get<Collection<Member>>(`v1/organizations/${org}/members`),
 media:(org:string,id:string)=>get<Collection<Media>>(`v1/organizations/${org}/inspections/${id}/media`),
 jobs:(org:string)=>get<Collection<Job>>(`v1/organizations/${org}/jobs`),
};
