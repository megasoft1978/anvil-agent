#!/usr/bin/env node

// Materialize the two frozen post-boundary holdout records without running their tests.
// The validation command remains the only place that can promote fixture status to complete.

import { execFileSync } from "node:child_process";
import { existsSync, lstatSync, mkdirSync, symlinkSync, unlinkSync, writeFileSync } from "node:fs";
import path from "node:path";
import process from "node:process";

const root = path.resolve(path.dirname(new URL(import.meta.url).pathname), "../..");
const holdoutRoot = path.join(root, "go-agent/holdouts");

function gitShow(repo, revision, file) {
  return execFileSync("git", ["-C", repo, "show", `${revision}:${file}`], { encoding: "utf8", maxBuffer: 8 * 1024 * 1024 });
}

function ensureZodDependencies() {
  const source = path.join(root, "go-agent/pilot/worktrees/zod-int-json-schema/node_modules");
  const target = path.join(holdoutRoot, "worktrees/zod-prefault-undefined/node_modules");
  if (!existsSync(source) || existsSync(target)) return;
  mkdirSync(path.dirname(target), { recursive: true });
  symlinkSync(source, target, "junction");
}

const zodRepo = path.join(holdoutRoot, "repos/zod");
const undiciRepo = path.join(holdoutRoot, "repos/undici");
const zodBase = "0c483c58849fdb6445aea4f54b1bac6b57ab3d22";
const zodReference = "9446b5cc14c5bf137790f1f66abf602044871223";
const undiciBase = "7f50f74d02c275cb94e0d04c74c37f3d61b70cef";
const undiciReference = "4ae29b24d6c14c5ee7e3644b94c964a602c786e1";

const manifest = {
  schema_version: 1,
  tasks: {
    "zod-prefault-undefined": {
      report: "Zod's prefault can lose an object property when the prefault resolves to undefined. With zod 4.6.1, a schema containing a required field whose prefault accepts undefined returns {} when parsing an object that omits that field, even though the expected result contains the field with value undefined. The inferred output type also omits undefined. The same behavior must remain correct for synchronous and asynchronous parsing, transforms, wrappers, and compiled parsing.",
      test_files: {
        "packages/zod/src/v4/classic/tests/prefault.test.ts": gitShow(zodRepo, zodReference, "packages/zod/src/v4/classic/tests/prefault.test.ts"),
        "packages/zod/src/v4/mini/tests/index.test.ts": gitShow(zodRepo, zodReference, "packages/zod/src/v4/mini/tests/index.test.ts"),
      },
      fail_to_pass: [
        "undefined prefault preserves object keys (jitless: false)",
        "undefined prefault preserves object keys (jitless: true)",
        "prefault preserves transformed undefined output",
        "z.prefault preserves undefined output",
      ],
      pass_to_pass: [
        "basic prefault",
        "direction-aware prefault",
        "z.boolean",
        "z.date",
      ],
      test_command: [
        "node",
        "node_modules/vitest/vitest.mjs",
        "run",
        "--config",
        "vitest.root.mjs",
        "--typecheck.enabled=false",
        "packages/zod/src/v4/classic/tests/prefault.test.ts",
        "packages/zod/src/v4/mini/tests/index.test.ts",
        "--reporter=json",
        "--outputFile={{REPORT}}",
      ],
      source_files: [
        "packages/zod/src/v4/core/compile.ts",
        "packages/zod/src/v4/core/schemas.ts",
        "packages/zod/src/v4/mini/schemas.ts",
      ],
    },
    "undici-mockagent-global-fetch": {
      report: "In undici 8.0.3 and 8.1.0, setting a MockAgent as the global dispatcher no longer intercepts native Node.js fetch requests. A configured mock for https://example.com/v1/test should return the mocked 200 JSON response, but the request reaches the real Example Domain and returns a 404 HTML page instead. Restore the expected MockAgent interception behavior while preserving the other global-dispatcher cases.",
      test_files: {
        "test/node-test/global-dispatcher-version.js": gitShow(undiciRepo, undiciReference, "test/node-test/global-dispatcher-version.js"),
        "scripts/tap-json.mjs": `import { spawnSync } from "node:child_process";
import { writeFileSync } from "node:fs";

const [, , testFile, reportPath] = process.argv;
const result = spawnSync(process.execPath, ["--test", "--test-reporter=tap", testFile], { encoding: "utf8" });
const output = String(result.stdout || "") + "\\n" + String(result.stderr || "");
const lines = output.split("\\n");
const assertionResults = [];
for (const line of lines) {
  const match = line.match(/^(not )?ok\\s+\\d+\\s+(.*)$/);
  if (match) assertionResults.push({ FullName: match[2].trim().replace(/^-\\s+/, ""), Status: match[1] ? "failed" : "passed" });
}
writeFileSync(reportPath, JSON.stringify({ testResults: [{ assertionResults }] }));
process.exit(result.error || result.status !== 0 || assertionResults.length === 0 || assertionResults.some((assertion) => assertion.Status === "failed") ? 1 : 0);
`,
      },
      fail_to_pass: ["setGlobalDispatcher lets Node.js global fetch use a MockAgent interceptor"],
      pass_to_pass: [
        "setGlobalDispatcher does not break Node.js global fetch",
        "setGlobalDispatcher mirrors a v1-compatible dispatcher that Node.js global fetch uses",
        "Dispatcher1Wrapper bridges legacy handlers to a new Agent",
      ],
      test_command: ["node", "scripts/tap-json.mjs", "test/node-test/global-dispatcher-version.js", "{{REPORT}}"],
      source_files: ["lib/mock/mock-agent.js"],
    },
  },
};

ensureZodDependencies();
mkdirSync(path.dirname(path.join(root, "go-agent/holdouts/manifest.json")), { recursive: true });
writeFileSync(path.join(root, "go-agent/holdouts/manifest.json"), `${JSON.stringify(manifest, null, 2)}\n`, { mode: 0o600 });
console.log(JSON.stringify({ manifest: path.join(root, "go-agent/holdouts/manifest.json"), tasks: Object.keys(manifest.tasks), validation: "pending" }, null, 2));
