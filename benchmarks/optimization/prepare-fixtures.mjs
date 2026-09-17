#!/usr/bin/env node

// Prepare dependency trees for the real-repository fixtures without running their tests.
// Dependencies stay outside model-visible worktrees and are linked by the benchmark runner.

import { execFileSync } from "node:child_process";
import { existsSync, lstatSync, readFileSync, rmSync, symlinkSync } from "node:fs";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(HERE, "../..");
const CONTRACT = path.join(HERE, "fixture-contract.json");

function loadJSON(file) {
  return JSON.parse(readFileSync(file, "utf8"));
}

function hasDependencyTree(directory) {
  try {
    return lstatSync(path.join(directory, "node_modules")).isDirectory()
      || lstatSync(path.join(directory, "node_modules")).isSymbolicLink();
  } catch {
    return false;
  }
}

function needsDependencyTree(task) {
  const command = (task.test_command ?? []).map(String);
  return task.dependencies_required === true
    || command.some((arg) => arg.includes("node_modules/"))
    || path.basename(task.source_root ?? "") === "qs-stringify-date-filter";
}

function manifestTask(contractTask) {
  const manifestPath = contractTask.pilot_manifest;
  if (!manifestPath) return {};
  const manifest = loadJSON(path.resolve(ROOT, manifestPath));
  return manifest.tasks?.[path.basename(contractTask.source_root)] ?? {};
}

function install(directory) {
  const common = ["--ignore-scripts", "--no-audit", "--no-fund", "--prefer-offline"];
  if (existsSync(path.join(directory, "pnpm-lock.yaml"))) {
    execFileSync("pnpm", ["install", "--frozen-lockfile", "--ignore-scripts", "--prefer-offline", "--reporter=append-only"], {
      cwd: directory,
      stdio: "inherit",
    });
    return "pnpm install";
  }
  if (existsSync(path.join(directory, "yarn.lock"))) {
    execFileSync("yarn", ["install", "--frozen-lockfile", "--ignore-scripts", "--non-interactive"], {
      cwd: directory,
      stdio: "inherit",
    });
    return "yarn install";
  }
  execFileSync("npm", ["install", ...common], { cwd: directory, stdio: "inherit" });
  return "npm install";
}

function share(directory, source) {
  const target = path.join(directory, "node_modules");
  if (existsSync(target)) return false;
  symlinkSync(path.join(source, "node_modules"), target, "junction");
  return true;
}

const checkOnly = process.argv.includes("--check");
const contract = loadJSON(CONTRACT);
const sharedSources = {
  "immer-array-methods-noop-sharing": "go-agent/pilot/worktrees/immer-array-push-fix",
};
const results = [];

for (const [id, task] of Object.entries(contract.tasks)) {
  const preparedTask = { ...task, ...manifestTask(task) };
  if (task.kind !== "pilot" || !needsDependencyTree(preparedTask)) {
    results.push({ task: id, status: "not_required" });
    continue;
  }
  const directory = path.resolve(ROOT, task.source_root);
  if (hasDependencyTree(directory)) {
    results.push({ task: id, status: "ready", path: directory });
    continue;
  }
  const shared = sharedSources[id] ? path.resolve(ROOT, sharedSources[id]) : null;
  if (shared && hasDependencyTree(shared)) {
    if (checkOnly) {
      results.push({ task: id, status: "pending_shared_link", path: directory, source: shared });
    } else {
      share(directory, shared);
      results.push({ task: id, status: "shared", path: directory, source: shared });
    }
    continue;
  }
  if (checkOnly) {
    results.push({ task: id, status: "pending_install", path: directory });
  } else {
    const command = install(directory);
    results.push({ task: id, status: "installed", path: directory, command });
  }
}

console.log(JSON.stringify({ check_only: checkOnly, results }, null, 2));
