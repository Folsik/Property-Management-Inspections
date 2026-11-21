import test from "node:test";
import assert from "node:assert/strict";
import {readFileSync} from "node:fs";
import {runInNewContext} from "node:vm";
import ts from "typescript";

const source=readFileSync(new URL("../lib/server-api.ts",import.meta.url),"utf8");
const compiled=ts.transpileModule(source,{
 compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}
}).outputText;
const ingress="ab".repeat(32),proxy="cd".repeat(32);
function load(env={TRUSTED_INGRESS_SECRET:ingress,TRUSTED_BFF_SECRET:proxy},mockFetch=fetch){
 const exports={};
 runInNewContext(compiled,{exports,process:{env},URL,AbortSignal,fetch:mockFetch,Response,Headers});
 return exports;
}
function metadata(ip="192.0.2.10",secret=ingress){
 return new Headers({"X-Inspectly-Ingress-IP":ip,"X-Inspectly-Ingress-Secret":secret,
  "X-Forwarded-For":"198.51.100.1","X-Inspectly-Client-IP":"198.51.100.2",
  "X-Inspectly-Proxy-Secret":"spoof"});
}
test("BFF forwards only authenticated ingress metadata with its own proof",()=>{
 const actual=load().trustedClientHeaders(metadata());
 assert.equal(actual["X-Inspectly-Client-IP"],"192.0.2.10");
 assert.equal(actual["X-Inspectly-Proxy-Secret"],proxy);
 assert.equal(Object.keys(actual).length,2);
});
test("invalid, missing, duplicate or disabled ingress proof fails closed",()=>{
 const helper=load().trustedClientHeaders;
 for(const source of [undefined,new Headers({"X-Forwarded-For":"192.0.2.5"}),metadata("192.0.2.5","wrong"),metadata("192.0.2.5,192.0.2.6"),metadata("192.0.2.5",ingress+","+ingress)]){
  assert.equal(Object.keys(helper(source)).length,0);
 }
 const duplicate=metadata();duplicate.append("X-Inspectly-Ingress-IP","192.0.2.11");
 assert.equal(Object.keys(helper(duplicate)).length,0);
 assert.equal(Object.keys(load({}).trustedClientHeaders(metadata())).length,0);
 assert.equal(Object.keys(load({TRUSTED_INGRESS_SECRET:ingress,TRUSTED_BFF_SECRET:ingress}).trustedClientHeaders(metadata())).length,0);
});
test("backend fetch strips spoofed headers and refuses redirects",async()=>{
 const calls=[];
 const api=load({TRUSTED_INGRESS_SECRET:ingress,TRUSTED_BFF_SECRET:proxy},async(url,options)=>{
  calls.push({url,...options});return new Response("{}");
 });
 const headers=new Headers({"X-Inspectly-Client-IP":"198.51.100.9","X-Inspectly-Proxy-Secret":"wrong","X-Forwarded-For":"198.51.100.8","Authorization":"Bearer test"});
 await api.fetchBackend("v1/auth/password/login",{method:"POST",headers},metadata("192.0.2.5","wrong"));
 assert.equal(calls[0].headers.get("X-Inspectly-Client-IP"),null);
 assert.equal(calls[0].headers.get("X-Inspectly-Proxy-Secret"),null);
 assert.equal(calls[0].headers.get("X-Forwarded-For"),null);
 assert.equal(calls[0].headers.get("Authorization"),"Bearer test");
 await api.fetchBackend("v1/auth/password/login",{method:"POST",headers},metadata());
 assert.equal(calls[1].headers.get("X-Inspectly-Client-IP"),"192.0.2.10");
 assert.equal(calls[1].headers.get("X-Inspectly-Proxy-Secret"),proxy);
 assert.equal(calls[1].redirect,"error");
});
