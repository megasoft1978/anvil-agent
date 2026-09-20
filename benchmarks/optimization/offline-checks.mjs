#!/usr/bin/env node

// Subprocess checks for runner behavior that must remain testable without a model server.
// These checks intentionally stop before fixture execution, server startup, or inference.

import assert from "node:assert/strict";
import { copyFileSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { createHash } from "node:crypto";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const runner = path.join(root, "benchmarks/optimization/runner.mjs");
const sessionIndex = process.argv.indexOf("--session");
const session = sessionIndex >= 0 && process.argv[sessionIndex + 1]
  ? process.argv[sessionIndex + 1]
  : path.join(root, ".optimization-results/20260915T092827090Z-b401994db269");
const cli = process.env.OPTIMIZATION_CLI || "/tmp/anvil-agent-opt";
const config = path.join(root, "benchmarks/optimization/experiments-local-model-screen-qwen3-coder.json");

function cleanEnvironment() {
  const env = { ...process.env };
  for (const name of ["ANVIL_LIVE", "ANVIL_REPLAY_TRACE", "ANVIL_PILOT", "ANVIL_REPO_STAGE", "ANVIL_LIVE_ENDPOINT", "ANVIL_LIVE_MODEL"]) delete env[name];
  return env;
}

function invoke(args) {
  const result = spawnSync(process.execPath, [runner, ...args], {
    cwd: root,
    env: cleanEnvironment(),
    encoding: "utf8",
    maxBuffer: 8 * 1024 * 1024,
  });
  return { ...result, output: `${result.stdout || ""}\n${result.stderr || ""}` };
}

function expectFailure(result, text) {
  assert.notEqual(result.status, 0, `expected failure containing ${text}, got ${result.output}`);
  assert.match(result.output, new RegExp(text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
}

const temp = mkdtempSync(path.join(os.tmpdir(), "anvil-optimization-offline-"));
const checks = [];
try {
  let result = invoke(["validate", "--dry-run", "--session", session]);
  assert.equal(result.status, 0, result.output);
  assert.match(result.output, /"inference_requested": false/);
  checks.push("validate dry-run without approval");

  result = invoke(["run", "--dry-run", "--session", session, "--cli", cli, "--experiment", "LLAMA-IQ2-8K-REAL"]);
  assert.equal(result.status, 0, result.output);
  assert.match(result.output, /"inference_requested": false/);
  checks.push("run dry-run without approval");

  result = invoke(["summarize", "--session", session]);
  assert.equal(result.status, 0, result.output);
  assert.ok(existsSync(path.join(session, "summary.json")), "summarize did not write summary.json");
  checks.push("summary path");

  const manifestPath = path.join(session, "manifest.json");
  const manifest = JSON.parse(readFileSync(manifestPath, "utf8"));
  const cliSha256 = createHash("sha256").update(readFileSync(cli)).digest("hex");
  assert.equal(cliSha256, manifest.binary?.sha256, "prepared CLI hash does not match the current binary");
  manifest.binary_preflight = { path: cli, sha256: cliSha256, matches_prepared: true, checked_at: new Date().toISOString() };
  manifest.pending_validations = (manifest.pending_validations ?? []).filter((item) => item !== "Go CLI build hash");
  writeFileSync(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`, { mode: 0o600 });
  checks.push("prepared CLI hash");

  const malformed = JSON.parse(readFileSync(config, "utf8"));
  malformed.experiments[0].tasks = [];
  const malformedConfig = path.join(temp, "malformed-experiments.json");
  writeFileSync(malformedConfig, `${JSON.stringify(malformed, null, 2)}\n`, { mode: 0o600 });
  result = invoke(["prepare", "--cli", cli, "--experiments", malformedConfig, "--results-root", path.join(temp, "results")]);
  expectFailure(result, "must declare tasks");
  checks.push("experiment shape rejection");

  const tamperedSession = path.join(temp, "tampered-session");
  const originalSession = path.join(session, "session.json");
  const originalManifest = path.join(session, "manifest.json");
  mkdirSync(tamperedSession);
  copyFileSync(originalSession, path.join(tamperedSession, "session.json"));
  const tamperedManifest = JSON.parse(readFileSync(originalManifest, "utf8"));
  tamperedManifest.input_lock.files = [];
  writeFileSync(path.join(tamperedSession, "manifest.json"), `${JSON.stringify(tamperedManifest, null, 2)}\n`, { mode: 0o600 });
  result = invoke(["validate", "--session", tamperedSession]);
  expectFailure(result, "prepared inputs changed");
  checks.push("input-lock rejection");

  console.log(JSON.stringify({ model_server_started: false, inference_requested: false, checks }, null, 2));
} finally {
  rmSync(temp, { recursive: true, force: true });
}
