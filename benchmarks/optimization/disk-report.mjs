#!/usr/bin/env node

// Read-only disk report for the local optimization workflow. It deliberately has no delete,
// move, or Trash operation: cleanup decisions remain explicit and reversible.
import { execFileSync } from "node:child_process";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = path.resolve(HERE, "../..");
const USER_ROOT = process.env.USERPROFILE ?? process.env.HOME ?? "/Users/megasoft78";
const RESEARCH_ROOT = path.join(USER_ROOT, "Desktop/Freelance/llm-memory-wall-research");

function size(pathname) {
  if (!existsSync(pathname)) return "absent";
  try {
    return execFileSync("du", ["-sh", pathname], { encoding: "utf8" }).trim().split(/\s+/)[0] ?? "unknown";
  } catch {
    return "unavailable";
  }
}

function configuredModels() {
  const models = new Map();
  for (const name of ["experiments-local-model-screen-qwen3-coder.json", "experiments-turboquant.json", "experiments-llama-qwen36.json", "experiments.json"]) {
    const file = path.join(HERE, name);
    if (!existsSync(file)) continue;
    try {
      const config = JSON.parse(readFileSync(file, "utf8"));
      if (config.model?.path) {
        const pathname = path.resolve(config.model.path);
        const prior = models.get(pathname);
        models.set(pathname, prior ? `${prior}, ${name}` : `${name} model`);
      }
    } catch {
      // The JSON validator reports malformed configs separately; this report stays read-only.
    }
  }
  return [...models.entries()].map(([pathname, label]) => ({ label, pathname, class: "required" }));
}

function resultRows() {
  const root = path.join(REPO_ROOT, ".optimization-results");
  if (!existsSync(root)) return [];
  return readdirSync(root, { withFileTypes: true })
    .filter((entry) => entry.isDirectory())
    .map((entry) => ({ label: `result session ${entry.name}`, pathname: path.join(root, entry.name), class: "evidence" }));
}

const rows = [
  ...configuredModels(),
  { label: "Hugging Face cache", pathname: path.join(USER_ROOT, ".cache/huggingface"), class: "optional" },
  { label: "FLUX cache (known disposable candidate)", pathname: path.join(USER_ROOT, ".cache/huggingface/hub/models--black-forest-labs--FLUX.2-klein-4B"), class: "optional" },
  { label: "Go build cache (regenerable)", pathname: path.join(USER_ROOT, "Library/Caches/go-build"), class: "optional" },
  { label: "Homebrew cache (regenerable)", pathname: path.join(USER_ROOT, "Library/Caches/Homebrew"), class: "optional" },
  { label: "pip cache (regenerable)", pathname: path.join(USER_ROOT, "Library/Caches/pip"), class: "optional" },
  { label: "optimization results", pathname: path.join(REPO_ROOT, ".optimization-results"), class: "evidence" },
  { label: "research project (inspect before cleanup)", pathname: RESEARCH_ROOT, class: "inspect" },
  { label: "research candidate models (untracked; inspect before cleanup)", pathname: path.join(RESEARCH_ROOT, "models"), class: "inspect" },
  { label: "research result artifacts (untracked; inspect before cleanup)", pathname: path.join(RESEARCH_ROOT, "results"), class: "inspect" },
  ...resultRows(),
];

const unique = new Map(rows.map((row) => [row.pathname, row]));
console.log("Read-only optimization disk report");
console.log(`repository: ${REPO_ROOT}`);
try {
  console.log(execFileSync("df", ["-h", "/System/Volumes/Data"], { encoding: "utf8" }).trim());
} catch {
  console.log("disk volume: unavailable");
}
console.log("\nclassification\tsize\tpath");
for (const row of unique.values()) {
  const label = `${row.class}: ${row.label}`;
  console.log(`${label}\t${size(row.pathname)}\t${row.pathname}`);
}
console.log("\nNo files were deleted, moved, or emptied by this command.");
