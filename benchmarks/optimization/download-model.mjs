#!/usr/bin/env node

// Download exactly one pinned candidate into an isolated local directory.
// This helper performs a disk preflight and never starts an inference server.

import {
  existsSync,
  mkdirSync,
  readdirSync,
  statSync,
  statfsSync,
  writeFileSync,
} from "node:fs";
import { spawn } from "node:child_process";
import path from "node:path";
import process from "node:process";

const GIB = 1024 ** 3;

function die(message) {
  console.error(`model download: ${message}`);
  process.exitCode = 2;
}

function parseArgs(argv) {
  const out = {};
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    if (!arg.startsWith("--")) throw new Error(`unexpected argument ${arg}`);
    const equal = arg.indexOf("=");
    if (equal >= 0) {
      out[arg.slice(2, equal)] = arg.slice(equal + 1);
      continue;
    }
    const key = arg.slice(2);
    if (key === "help") {
      out[key] = true;
      continue;
    }
    if (index + 1 >= argv.length || argv[index + 1].startsWith("--")) throw new Error(`missing value for --${key}`);
    out[key] = argv[++index];
  }
  return out;
}

function existingParent(target) {
  let directory = path.resolve(target);
  while (!existsSync(directory)) {
    const parent = path.dirname(directory);
    if (parent === directory) throw new Error(`cannot find a filesystem path for ${target}`);
    directory = parent;
  }
  if (!statSync(directory).isDirectory()) throw new Error(`filesystem path is not a directory: ${directory}`);
  return directory;
}

function bytes(value, label) {
  const parsed = Number(value);
  if (!Number.isSafeInteger(parsed) || parsed < 0) throw new Error(`${label} must be a non-negative integer byte count`);
  return parsed;
}

function help() {
  console.log(`Usage:
  node benchmarks/optimization/download-model.mjs \
    --repo OWNER/REPO --revision REVISION --local-dir ABSOLUTE_DIR \
    --expected-bytes BYTES [--reserve-bytes BYTES] [--max-workers N]

The command uses the installed hf CLI, resumes a partial local directory, and writes
model-download.json only after hf exits successfully. It does not start a model server.`);
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  if (args.help) {
    help();
    return;
  }
  for (const required of ["repo", "revision", "local-dir", "expected-bytes"]) {
    if (!args[required]) throw new Error(`--${required} is required`);
  }
  const localDir = path.resolve(args["local-dir"]);
  if (!path.isAbsolute(localDir)) throw new Error("--local-dir must resolve to an absolute path");
  const expectedBytes = bytes(args["expected-bytes"], "--expected-bytes");
  const reserveBytes = bytes(args["reserve-bytes"] ?? 4 * GIB, "--reserve-bytes");
  const maxWorkers = String(args["max-workers"] ?? "4");
  if (!/^\d+$/.test(maxWorkers) || Number(maxWorkers) < 1) throw new Error("--max-workers must be a positive integer");
  if (existsSync(localDir) && !statSync(localDir).isDirectory()) throw new Error(`--local-dir is not a directory: ${localDir}`);
  const parent = existingParent(localDir);
  const filesystem = statfsSync(parent);
  const freeBytes = Number(filesystem.bavail) * Number(filesystem.bsize);
  const requiredBytes = expectedBytes + reserveBytes;
  if (freeBytes < requiredBytes) {
    throw new Error(`insufficient free space on ${parent}: ${freeBytes} bytes available, ${requiredBytes} required (${expectedBytes} model + ${reserveBytes} reserve)`);
  }

  mkdirSync(localDir, { recursive: true, mode: 0o700 });
  const command = args["hf-bin"] ?? "hf";
  const commandArgs = ["download", args.repo, "--revision", args.revision, "--local-dir", localDir, "--max-workers", maxWorkers];
  if (args.include) {
    for (const pattern of String(args.include).split(",").map((value) => value.trim()).filter(Boolean)) commandArgs.push("--include", pattern);
  }
  console.error(`disk preflight: ${freeBytes} bytes free; ${requiredBytes} required`);
  console.error([command, ...commandArgs].map((value) => JSON.stringify(value)).join(" "));
  const exitCode = await new Promise((resolve, reject) => {
    const child = spawn(command, commandArgs, { cwd: process.cwd(), stdio: "inherit", windowsHide: true });
    child.once("error", reject);
    child.once("close", (code, signal) => resolve(signal ? 128 : code ?? 1));
  });
  if (exitCode !== 0) throw new Error(`hf download exited with ${exitCode}; no inference was attempted`);
  const entries = readdirSync(localDir, { withFileTypes: true }).map((entry) => entry.name).sort();
  writeFileSync(path.join(localDir, "model-download.json"), `${JSON.stringify({
    schema_version: 1,
    repo: args.repo,
    revision: args.revision,
    local_dir: localDir,
    expected_bytes: expectedBytes,
    reserve_bytes: reserveBytes,
    preflight_free_bytes: freeBytes,
    command: [command, ...commandArgs],
    top_level_entries: entries,
    completed_at: new Date().toISOString(),
  }, null, 2)}\n`, { mode: 0o600 });
  console.log(`downloaded ${args.repo}@${args.revision} to ${localDir}`);
}

main().catch((error) => die(error.message));
