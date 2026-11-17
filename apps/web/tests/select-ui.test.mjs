import {test} from "node:test";
import assert from "node:assert/strict";
import {readFileSync} from "node:fs";
import {resolve} from "node:path";

const workspace=readFileSync(resolve("components/workspace.tsx"),"utf8");
const select=readFileSync(resolve("components/ui/select.tsx"),"utf8");
const styles=readFileSync(resolve("app/globals.css"),"utf8");

test("workspace never renders browser-native HTML select or option menus",()=>{
 assert.doesNotMatch(workspace,/<\s*select\b/,"Use shared Radix Select instead of native select in workspace");
 assert.doesNotMatch(workspace,/<\s*option\b/,"Use shared Radix SelectItem instead of native option in workspace");
});

test("workspace organization switcher keeps controlled organization selection and accessible name",()=>{
 assert.match(workspace,/<Select\s+value=\{orgID\}\s+onValueChange=\{setOrgID\}/);
 assert.match(workspace,/SelectTrigger[^>]*aria-label="Active organization"/);
 assert.match(workspace,/orgs\.map\(o=><SelectItem\s+key=\{o.id\}\s+value=\{o.id\}/);
});

test("inspection item status uses controlled Radix select with disabled saving state",()=>{
 assert.match(workspace,/<Select\s+value=\{item.status\}\s+disabled=\{saving\}/);
 assert.match(workspace,/onValueChange=\{value=>void setItemStatus\(item.id,value as ItemStatus\)\}/);
 assert.match(workspace,/aria-label=\{"Status for "\+item.title\}/);
});

test("shared select is Radix-based and Tailwind is loaded globally",()=>{
 assert.match(select,/@radix-ui\/react-select/);
 assert.match(styles,/@import "tailwindcss"/);
});
