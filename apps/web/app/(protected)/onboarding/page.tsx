import {Onboarding} from "@/components/onboarding";
export default async function OnboardingPage({searchParams}:{searchParams:Promise<{invite?:string}>}){
 const {invite}=await searchParams;
 return <Onboarding token={typeof invite==="string"&&/^[0-9a-f]{64}$/.test(invite)?invite:""}/>;
}
