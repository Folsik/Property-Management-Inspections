// Runs against the live Docker Compose web+API+PostgreSQL stack, not mocks.
import assert from "node:assert/strict";
import {randomInt} from "node:crypto";

const base=process.env.SMOKE_ORIGIN||"http://127.0.0.1:3000";
const phone="+1555"+String(randomInt(10000000,99999999));
let session="";
async function request(path,method="GET",body=null,authenticated=true){
 const response=await fetch(base+path,{
  method,
  headers:{
   ...(method!=="GET"?{"Origin":base,"Content-Type":"application/json"}:{}),
   ...(authenticated&&session?{"Cookie":session}:{})
  },
  ...(body===null?{}:{body:JSON.stringify(body)}),
  redirect:"manual",
 });
 const text=await response.text();
 let data;
 try{data=JSON.parse(text);}catch{data={raw:text.slice(0,200)};}
 return {status:response.status,data,cookie:response.headers.get("set-cookie")};
}
const denied=await request("/api/auth/request","POST",{phone},false);
assert.notEqual(denied.data.error,"Invalid origin","Valid loopback Origin must never be rejected");

assert.equal(denied.status,202,JSON.stringify(denied.data));
assert.match(denied.data.dev_code,/^\d{6}$/,"Local development should expose code only on loopback and with explicit configuration");
const verify=await request("/api/auth/verify","POST",{phone,code:denied.data.dev_code},false);
assert.equal(verify.status,200,JSON.stringify(verify.data));
assert.equal(verify.data.ok,true);
session=verify.cookie?.split(";")[0]||"";
assert.ok(session.startsWith("inspectly_session="),"Expected HttpOnly session cookie");
const profile=await request("/api/backend/v1/me");
assert.equal(profile.status,200,JSON.stringify(profile.data));
assert.equal(profile.data.phone,phone);
const statusBefore=await request("/api/backend/v1/onboarding");
assert.equal(statusBefore.status,200,JSON.stringify(statusBefore.data));
assert.equal(statusBefore.data.completed,false);
assert.equal(statusBefore.data.pin_set,false);
const pin=await request("/api/backend/v1/auth/pin","POST",{pin:"918245"});
assert.equal(pin.status,200,JSON.stringify(pin.data));
const email="owner-"+phone.slice(1)+"@example.test";
const emailChallenge=await request("/api/backend/v1/email/request","POST",{email});
assert.equal(emailChallenge.status,202,JSON.stringify(emailChallenge.data));
assert.match(emailChallenge.data.dev_code,/^\\d{6}$/);
const verifyEmail=await request("/api/backend/v1/email/verify","POST",{email,code:emailChallenge.data.dev_code});
assert.equal(verifyEmail.status,200,JSON.stringify(verifyEmail.data));
const profileName=await request("/api/backend/v1/me","PATCH",{full_name:"CI Real Owner"});
assert.equal(profileName.status,200,JSON.stringify(profileName.data));
const notYetComplete=await request("/api/backend/v1/onboarding/complete","POST",{});
assert.equal(notYetComplete.status,409,"Cannot finish without organization and property");
const org=await request("/api/backend/v1/organizations","POST",{name:"CI registration smoke",kind:"individual",country:"GB"});
assert.equal(org.status,201,JSON.stringify(org.data));
assert.ok(org.data.id);
const properties=await request("/api/backend/v1/organizations/"+org.data.id+"/properties","POST",{name:"CI test apartment",address:{city:"Local integration test"}});
assert.equal(properties.status,201,JSON.stringify(properties.data));
const finish=await request("/api/backend/v1/onboarding/complete","POST",{});
assert.equal(finish.status,200,JSON.stringify(finish.data));
const finalStatus=await request("/api/backend/v1/onboarding");
assert.equal(finalStatus.data.completed,true);
const inspection=await request("/api/backend/v1/organizations/"+org.data.id+"/inspections","POST",{property_id:properties.data.id,type:"turnover"});
assert.equal(inspection.status,201,JSON.stringify(inspection.data));
const workerEmail="worker-"+phone.slice(1)+"@example.test";
const invitation=await request("/api/backend/v1/organizations/"+org.data.id+"/invitations","POST",{email:workerEmail,role:"inspector"});
assert.equal(invitation.status,201,JSON.stringify(invitation.data));
assert.match(invitation.data.token,/^[0-9a-f]{64}$/);
const publicPreview=await request("/api/public/invitations/"+invitation.data.token,"GET",null,false);
assert.equal(publicPreview.status,200,JSON.stringify(publicPreview.data));
assert.equal(publicPreview.data.organization_name,"CI registration smoke");
const originalSession=session;
const workerPhone="+1666"+phone.slice(5);
const workerOTP=await request("/api/auth/request","POST",{phone:workerPhone},false);
assert.equal(workerOTP.status,202,JSON.stringify(workerOTP.data));
const workerVerify=await request("/api/auth/verify","POST",{phone:workerPhone,code:workerOTP.data.dev_code},false);
assert.equal(workerVerify.status,200,JSON.stringify(workerVerify.data));
session=workerVerify.cookie?.split(";")[0]||"";
assert.ok(session.startsWith("inspectly_session="));
const beforeVerification=await request("/api/backend/v1/invitations/"+invitation.data.token+"/accept","POST",{});
assert.equal(beforeVerification.status,403);
const workerPin=await request("/api/backend/v1/auth/pin","POST",{pin:"918245"});
assert.equal(workerPin.status,200);
const workerName=await request("/api/backend/v1/me","PATCH",{full_name:"CI Inspector"});
assert.equal(workerName.status,200);
const workerEmailChallenge=await request("/api/backend/v1/email/request","POST",{email:workerEmail});
assert.equal(workerEmailChallenge.status,202,JSON.stringify(workerEmailChallenge.data));
const workerEmailVerify=await request("/api/backend/v1/email/verify","POST",{email:workerEmail,code:workerEmailChallenge.data.dev_code});
assert.equal(workerEmailVerify.status,200,JSON.stringify(workerEmailVerify.data));
const acceptInvite=await request("/api/backend/v1/invitations/"+invitation.data.token+"/accept","POST",{});
assert.equal(acceptInvite.status,200,JSON.stringify(acceptInvite.data));
const workerComplete=await request("/api/backend/v1/onboarding/complete","POST",{});
assert.equal(workerComplete.status,200,JSON.stringify(workerComplete.data));
const memberOrgs=await request("/api/backend/v1/organizations");
assert.equal(memberOrgs.status,200,JSON.stringify(memberOrgs.data));
assert.ok(memberOrgs.data.items.some(item=>item.id===org.data.id&&item.role==="inspector"));
session=originalSession;
const crossOrigin=await fetch(base+"/api/backend/v1/organizations",{method:"POST",headers:{Origin:"https://attacker.example","Content-Type":"application/json",Cookie:session},body:JSON.stringify({name:"Must not create"})});
assert.equal(crossOrigin.status,403,"Cross-site writes must be rejected");
const logout=await request("/api/auth/logout","POST",{});
assert.equal(logout.status,200);
console.log("PASS: OTP → PIN → email verified → personal/company workspace → property → inspection → secure invitation/acceptance → CSRF rejection → logout");
