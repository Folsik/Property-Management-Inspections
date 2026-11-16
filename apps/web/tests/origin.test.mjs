import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import ts from "typescript";
import { runInNewContext } from "node:vm";

// Executes the real Next.js origin helper, transpiled without changing its behavior.
const source = readFileSync(new URL("../lib/server-api.ts", import.meta.url), "utf8");
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 }
}).outputText;
const exports = {};
runInNewContext(compiled, { exports, process, URL, AbortSignal, fetch, Response });
const { sameOrigin } = exports;

const request = (origin, host, url="http://api:3000/api/auth/request") => ({
  url,
  headers: new Headers({
    ...(origin?{Origin:origin}:{}),
    Host:host
  })
});

test("accepts browser origin matching public Host behind Docker reverse proxy", () => {
  assert.equal(sameOrigin(request("http://localhost:3000","localhost:3000")), true);
  assert.equal(sameOrigin(request("http://127.0.0.1:3000","127.0.0.1:3000")), true);
});

test("rejects different origin even when internal URL matches it", () => {
  assert.equal(sameOrigin(request("https://evil.example","localhost:3000")), false);
});
test("rejects missing Origin and port mismatch", () => {
  assert.equal(sameOrigin(request(null,"localhost:3000")), false);
  assert.equal(sameOrigin(request("http://localhost:5000","localhost:3000")), false);
});
test("rejects malformed Origin, forged hostname lookalikes and multiple origins", () => {
  assert.equal(sameOrigin(request("null","localhost:3000")), false);
  assert.equal(sameOrigin(request("http://localhost:3000.evil.example","localhost:3000")), false);
  assert.equal(sameOrigin(request("http://localhost:3000 http://other","localhost:3000")), false);
});
test("works with HTTPS public origin and internal HTTP backend URL", () => {
  assert.equal(sameOrigin(request("https://app.example.com","app.example.com")), true);
});
