import {cookies} from "next/headers";
import {redirect} from "next/navigation";
export default async function ProtectedLayout({children}:{children:React.ReactNode}){
 const jar=await cookies();
 if(!jar.has("inspectly_session"))redirect("/login");
 return children;
}
