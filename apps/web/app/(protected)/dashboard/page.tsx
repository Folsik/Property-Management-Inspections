import {cookies} from "next/headers";
import {redirect} from "next/navigation";
import {fetchBackend} from "@/lib/server-api";
import {Workspace} from "@/components/workspace";
export default async function Dashboard(){
 const token=(await cookies()).get("inspectly_session")?.value;
 if(token){
  const res=await fetchBackend("v1/onboarding",{headers:{Authorization:"Bearer "+token}});
  if(res.ok){
   const state=await res.json() as {completed?:boolean;password_set?:boolean};
   if(!state.completed||!state.password_set)redirect("/onboarding");
  }
 }
 return <Workspace/>;
}
