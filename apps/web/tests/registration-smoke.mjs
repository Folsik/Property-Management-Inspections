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
const org=await request("/api/backend/v1/organizations","POST",{name:"CI registration smoke"});
assert.equal(org.status,201,JSON.stringify(org.data));
assert.ok(org.data.id);
const properties=await request("/api/backend/v1/organizations/"+org.data.id+"/properties","POST",{name:"CI test apartment",address:{city:"Local integration test"}});
assert.equal(properties.status,201,JSON.stringify(properties.data));
const inspection=await request("/api/backend/v1/organizations/"+org.data.id+"/inspections","POST",{property_id:properties.data.id,type:"turnover"});
assert.equal(inspection.status,201,JSON.stringify(inspection.data));
const crossOrigin=await fetch(base+"/api/backend/v1/organizations",{method:"POST",headers:{Origin:"https://attacker.example","Content-Type":"application/json",Cookie:session},body:JSON.stringify({name:"Must not create"})});
assert.equal(crossOrigin.status,403,"Cross-site writes must be rejected");
const logout=await request("/api/auth/logout","POST",{});
assert.equal(logout.status,200);
console.log("PASS: OTP registration → authenticated profile → organization → property → inspection → CSRF rejection → logout");
