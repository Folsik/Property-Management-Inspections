import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import ts from "typescript";
import { runInNewContext } from "node:vm";

const source = readFileSync(new URL("../lib/phone.ts", import.meta.url), "utf8");
const transpiled = ts.transpileModule(source,{
  compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}
}).outputText;
const exports = {};
runInNewContext(transpiled,{exports});
const {formatE164}=exports;

test("US +1 number from selected country and national digits",()=>{
 assert.equal(formatE164("+1","(415) 555-2671"),"+14155552671");
});
test("UK trunk 0 removed for country code +44",()=>{
 assert.equal(formatE164("+44","07700 900123"),"+447700900123");
});
test("Italian leading zero is retained",()=>{
 assert.equal(formatE164("+39","06 6982 1"),"+390669821");
});
test("rejects only a dial code, too short, too long or nonnumeric user input",()=>{
 assert.equal(formatE164("+1",""),null);
 assert.equal(formatE164("+1","123"),null);
 assert.equal(formatE164("+1","12345678901234567890"),null);
 assert.equal(formatE164("+1","415 ABC 1234"),null);
});
test("rejects phone prefix injection",()=>{
 assert.equal(formatE164("+1","+44 7700900123"),null);
 assert.equal(formatE164("+abc","4155552671"),null);
});
