#!/usr/bin/env node

// Decision-grade local runner for the local model screen.
//
// The runner owns orchestration, fixture materialization, host verification, memory sampling,
// result persistence, and resumption. The Go binary remains the only model-facing agent loop:
// every scored attempt below spawns that binary and lets it make native read/search/edit/write
// calls against a fresh worktree.

import {
  appendFileSync,
  closeSync,
  cpSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  openSync,
  readFileSync,
  readlinkSync,
  readdirSync,
  readSync,
  rmSync,
  statSync,
  symlinkSync,
  writeFileSync,
} from "node:fs";
import { createHash } from "node:crypto";
import { spawn, execFile } from "node:child_process";
import { promisify } from "node:util";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";
import { cachedPromptTokens } from "./cache-metrics.mjs";
import { parseFootprintText, evaluateFootprintGate } from "./memory-gate.mjs";

// Default server-footprint ceiling. The prior gates assumed a rebooted,
// app-free 16 GiB host and aborted on ambient "critical" system pressure --
// but this host's actual normal condition runs close to its swap ceiling,
// and that ambient pressure is not itself a contamination signal. What
// matters is whether the model server's own dirty footprint fits. Live
// vm_stat checks on 2026-09-17 measured reclaimable memory (free +
// speculative + inactive) at 2.9 and 3.67 GiB minutes apart under kernel
// pressure level 2 (warning) both times; 3 GiB is the working default,
// deliberately below the low end of that range. Override per-experiment
// with `policy.max_server_footprint_bytes`.
const DEFAULT_MAX_SERVER_FOOTPRINT_BYTES = 3 * 1024 ** 3;



const execFileAsync = promisify(execFile);
const HERE = path.dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = path.resolve(HERE, "../..");
// New preparation sessions default to the verified current Qwen3-Coder winner.
const DEFAULT_EXPERIMENTS = path.join(HERE, "experiments-local-model-screen-qwen3-coder.json");
const DEFAULT_CONTRACT = path.join(HERE, "fixture-contract.json");
const DEFAULT_PILOT = path.join(REPO_ROOT, "go-agent/pilot");
const DEFAULT_RESULTS_ROOT = path.join(REPO_ROOT, ".optimization-results");
const MAX_CAPTURE_BYTES = 128 * 1024;

let interruption = null;
const ownedChildren = new Set();
function trackChild(child) {
  ownedChildren.add(child);
  child.once("close", () => ownedChildren.delete(child));
  return child;
}
function interrupt(signal) {
  interruption ??= `operator interrupted with ${signal}`;
  for (const child of ownedChildren) {
    killGroup(child.pid, "SIGTERM");
    const timer = setTimeout(() => killGroup(child.pid, "SIGKILL"), 10_000);
    timer.unref();
    child.once("close", () => clearTimeout(timer));
  }
}

function die(message) {
  throw new Error(message);
}

function parseArgs(argv) {
  const out = { _: [] };
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (!arg.startsWith("--")) {
      out._.push(arg);
      continue;
    }
    const eq = arg.indexOf("=");
    if (eq !== -1) {
      out[arg.slice(2, eq)] = arg.slice(eq + 1);
      continue;
    }
    const key = arg.slice(2);
    if (key === "resume" || key === "start-server" || key === "external-server" || key === "fetch-reference" || key === "force" || key === "approved" || key === "evidence-approved" || key === "dry-run") {
      out[key] = true;
    } else {
      if (i + 1 >= argv.length || argv[i + 1].startsWith("--")) die(`missing value for --${key}`);
      out[key] = argv[++i];
    }
  }
  return out;
}

function loadJSON(file) {
  try {
    return JSON.parse(readFileSync(file, "utf8"));
  } catch (error) {
    die(`cannot read JSON ${file}: ${error.message}`);
  }
}

function writeJSON(file, value) {
  mkdirSync(path.dirname(file), { recursive: true, mode: 0o700 });
  writeFileSync(file, `${JSON.stringify(value, null, 2)}\n`, { mode: 0o600 });
}

function appendJSONL(file, value) {
  mkdirSync(path.dirname(file), { recursive: true, mode: 0o700 });
  appendFileSync(file, `${JSON.stringify(value)}\n`, { mode: 0o600 });
}

function readJSONL(file) {
  if (!existsSync(file)) return [];
  return readFileSync(file, "utf8")
    .split("\n")
    .filter(Boolean)
    .map((line, index) => {
      try {
        return JSON.parse(line);
      } catch (error) {
        die(`invalid JSONL ${file}:${index + 1}: ${error.message}`);
      }
    });
}

function sha256(data) {
  return createHash("sha256").update(data).digest("hex");
}

function hashFile(file) {
  const hash = createHash("sha256");
  const bytes = statSync(file).size;
  const fd = openSync(file, "r");
  const buffer = Buffer.allocUnsafe(8 << 20);
  let position = 0;
  try {
    while (position < bytes) {
      const count = readSync(fd, buffer, 0, Math.min(buffer.length, bytes - position), position);
      if (count === 0) break;
      hash.update(buffer.subarray(0, count));
      position += count;
    }
  } finally {
    closeSync(fd);
  }
  return { sha256: hash.digest("hex"), bytes };
}

function isRegularFile(file) {
  try {
    return statSync(file).isFile();
  } catch {
    return false;
  }
}

function hashTree(root, { skip = new Set([".git", "node_modules"]) } = {}) {
  const entries = [];
  function visit(dir, relDir) {
    for (const entry of readdirSync(dir, { withFileTypes: true }).sort((a, b) => a.name.localeCompare(b.name))) {
      if (skip.has(entry.name)) continue;
      const rel = relDir ? `${relDir}/${entry.name}` : entry.name;
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        visit(full, rel);
      } else if (entry.isFile()) {
        const h = hashFile(full);
        entries.push({ path: rel.replaceAll(path.sep, "/"), ...h });
      } else if (entry.isSymbolicLink()) {
        entries.push({ path: rel.replaceAll(path.sep, "/"), symlink: readlinkSafe(full) });
      }
    }
  }
  visit(root, "");
  return { files: entries, tree_sha256: sha256(JSON.stringify(entries)) };
}

function readlinkSafe(file) {
  try {
    return `->${readlinkSync(file)}`;
  } catch {
    return null;
  }
}

async function command(argv, options = {}) {
  const { cwd = REPO_ROOT, env = process.env, timeoutMs = 30_000, maxBytes = MAX_CAPTURE_BYTES } = options;
  try {
    const result = await execFileAsync(argv[0], argv.slice(1), {
      cwd,
      env,
      timeout: timeoutMs,
      maxBuffer: maxBytes,
      windowsHide: true,
    });
    return { code: 0, stdout: result.stdout, stderr: result.stderr };
  } catch (error) {
    return {
      code: typeof error.code === "number" ? error.code : 1,
      signal: error.signal ?? null,
      stdout: error.stdout ?? "",
      stderr: error.stderr ?? error.message,
      timedOut: error.killed === true && error.signal === "SIGTERM",
    };
  }
}

function parseNumber(value) {
  const digits = String(value ?? "").replace(/[^0-9.-]/g, "");
  if (!digits) return null;
  const n = Number(digits);
  return Number.isFinite(n) ? n : null;
}

async function readHardware() {
  const [uname, swVers, mem, cores, vm, pressure, swap] = await Promise.all([
    command(["uname", "-a"]),
    command(["sw_vers"]),
    command(["sysctl", "-n", "hw.memsize"]),
    command(["sysctl", "-n", "hw.ncpu"]),
    command(["vm_stat"]),
    command(["memory_pressure", "-Q"]),
    command(["sysctl", "-n", "vm.swapusage"]),
  ]);
  return {
    uname: uname.stdout.trim() || null,
    sw_vers: swVers.stdout.trim() || null,
    physical_memory_bytes: parseNumber(mem.stdout),
    cpu_cores: parseNumber(cores.stdout),
    vm_stat_baseline: vm.stdout.trim() || null,
    memory_pressure_baseline: pressure.stdout.trim() || null,
    swap_usage_baseline: swap.stdout.trim() || null,
  };
}

async function backgroundBaseline() {
  const ps = await command(["ps", "-axo", "pid=,ppid=,rss=,state=,command="]);
  if (ps.code !== 0) return { status: "unavailable", explanation: ps.stderr.trim() || "ps failed" };
  const lines = ps.stdout.trim().split("\n").filter(Boolean);
  const rows = lines.map((line) => {
    const match = line.trim().match(/^(\d+)\s+(\d+)\s+(\d+)\s+(\S+)\s+(.*)$/);
    return match ? { pid: Number(match[1]), ppid: Number(match[2]), rss_kb: Number(match[3]), state: match[4], command: match[5] } : null;
  }).filter(Boolean);
  rows.sort((a, b) => b.rss_kb - a.rss_kb);
  return { status: "captured", process_count: rows.length, top_processes: rows.slice(0, 20) };
}

async function gitInfo() {
  const [hash, branch, status] = await Promise.all([
    command(["git", "rev-parse", "HEAD"]),
    command(["git", "branch", "--show-current"]),
    command(["git", "status", "--short"]),
  ]);
  return {
    commit: hash.stdout.trim() || null,
    branch: branch.stdout.trim() || null,
    status_short: status.stdout.trim() || null,
  };
}

async function binaryInfo(file) {
  if (!file || !isRegularFile(file)) return { path: file ?? null, sha256: null, bytes: null, explanation: "binary not built yet" };
  const stat = hashFile(file);
  return { path: path.resolve(file), ...stat };
}

async function modelInfo(model) {
  if (!existsSync(model.path)) return { path: model.path, bytes: null, sha256: null, explanation: "model path is not present; no download is attempted" };
  const stat = statSync(model.path);
  if (!stat.isDirectory()) {
    const file = hashFile(model.path);
    return {
      path: model.path,
      ...file,
      expected_bytes: model.bytes ?? null,
      size_matches: model.bytes == null ? null : file.bytes === model.bytes,
      expected_sha256: model.sha256 ?? null,
      hash_matches: model.sha256 == null ? null : file.sha256 === model.sha256,
    };
  }
  const tree = hashTree(model.path, { skip: new Set([".cache", ".git", "node_modules"]) });
  const weightBytes = tree.files.filter((entry) => entry.path.endsWith(".safetensors")).reduce((sum, entry) => sum + entry.bytes, 0);
  const allBytes = tree.files.filter((entry) => Number.isFinite(entry.bytes)).reduce((sum, entry) => sum + entry.bytes, 0);
  return {
    path: model.path,
    directory: true,
    bytes: weightBytes,
    total_bytes: allBytes,
    sha256: tree.tree_sha256,
    expected_bytes: model.bytes ?? null,
    size_matches: model.bytes == null ? null : weightBytes === model.bytes,
    expected_sha256: model.tree_sha256 ?? null,
    hash_matches: model.tree_sha256 == null ? null : tree.tree_sha256 === model.tree_sha256,
    files: tree.files,
  };
}

function modelRequestID(experiments) {
  if (experiments.server?.backend === "mlx-lm" || experiments.server?.backend === "mlx-vlm") {
    // MLX-LM maps the request's model field to the exact --model value supplied at
    // startup. A semantic alias would make it attempt a second Hugging Face load.
    return experiments.server.request_model_id ?? experiments.model.path;
  }
  return experiments.model.request_id ?? experiments.model.cli_id ?? experiments.model.id ?? experiments.model.path;
}

function serverBackend(experiments) {
  return experiments.server.backend ?? "llama";
}

function rootURL(endpoint) {
  return new URL(endpoint).origin;
}

async function httpText(url, { method = "GET", body = null, timeoutMs = 5000 } = {}) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const response = await fetch(url, {
      method,
      signal: controller.signal,
      headers: body ? { "content-type": "application/json" } : undefined,
      body: body ? JSON.stringify(body) : undefined,
    });
    return { status: response.status, text: await response.text() };
  } catch (error) {
    return { status: null, text: "", error: error.message };
  } finally {
    clearTimeout(timer);
  }
}

async function serverSnapshot(experiments) {
  const endpoint = experiments.server.endpoint;
  const root = rootURL(endpoint);
  const health = await httpText(`${root}${experiments.server.health_path}`);
  const props = await httpText(`${root}${experiments.server.props_path ?? "/props"}`);
  let parsedProps = null;
  try { parsedProps = JSON.parse(props.text); } catch { /* optional endpoint */ }
  let template = parsedProps?.chat_template ?? parsedProps?.model?.chat_template ?? null;
  if (typeof template !== "string" && experiments.server.template_path && existsSync(experiments.server.template_path)) {
    template = readFileSync(experiments.server.template_path, "utf8");
  }
  const models = experiments.server.models_path ? await httpText(`${root}${experiments.server.models_path}`) : null;
  let parsedModels = null;
  try { parsedModels = models ? JSON.parse(models.text) : null; } catch { /* optional endpoint */ }
  return {
    endpoint,
    health: { status: health.status, body: health.text.trim() || null, error: health.error ?? null },
    props: props.status === null ? null : parsedProps,
    models: models?.status === null ? null : parsedModels,
    template_sha256: typeof template === "string" ? sha256(template) : null,
    template_explanation: typeof template === "string" ? null : "server properties and configured template path did not expose chat_template",
  };
}

async function serverBinaryInfo(experiments) {
  const probe = experiments.server.version_probe ?? [experiments.server.binary, "--version"];
  const result = await command(probe);
  const text = `${result.stdout}\n${result.stderr}`.trim();
  const match = text.match(/build\s+(\d+),\s+commit\s+([0-9a-f]+)/i)
    ?? text.match(/(?:^|\n)version:\s*(\d+)\s*\(([0-9a-f]+)\)/i);
  const build = match?.[1] ?? null;
  const commit = match?.[2] ?? null;
  const version = text.match(/(?:^|\n)Version:\s*([^\s]+)/i)?.[1]
    ?? text.match(/TurboQuant[^\n]*?([0-9]+\.[0-9]+\.[0-9]+)/i)?.[1]
    ?? text.match(/(?:^|\n)v?([0-9]+\.[0-9]+\.[0-9]+)(?:\s|$)/i)?.[1]
    ?? null;
  const expected = String(experiments.server.version_expected ?? "");
  const backend = serverBackend(experiments);
  const matches = backend === "turboquant"
    ? result.code === 0 && version === expected
    : backend === "mlx-lm" || backend === "mlx-vlm"
      ? result.code === 0 && version === expected
      : backend === "hebrus"
        ? result.code === 0 && (!experiments.server.commit_expected || text.includes(experiments.server.commit_expected))
      : backend === "mference"
        // MferenceServer's binary prints no version/commit string of its own
        // (confirmed: --help exits 0 with usage text only). Provenance is
        // instead the source git commit and applied local patch recorded in
        // this config's server.commit_expected / policy.backend_note.
        ? result.code === 0
      : result.code === 0 && build === expected.replace(/^b/i, "") && commit === experiments.server.commit_expected;
  return {
    binary: experiments.server.binary,
    probe,
    exit_code: result.code,
    version,
    build,
    commit,
    output: text || null,
    matches_expected: matches,
  };
}

function parseVMStat(text) {
  const pageMatch = text.match(/page size of\s+(\d+)\s+bytes/i);
  const pageSize = pageMatch ? Number(pageMatch[1]) : 4096;
  const pages = {};
  for (const line of text.split("\n")) {
    const match = line.match(/^([^:]+):\s+([0-9]+)\.?/);
    if (match) pages[match[1].trim()] = Number(match[2]);
  }
  const bytes = (label) => pages[label] == null ? null : pages[label] * pageSize;
  return {
    page_size_bytes: pageSize,
    wired_bytes: bytes("Pages wired down"),
    compressed_bytes: bytes("Pages occupied by compressor"),
    free_bytes: bytes("Pages free"),
    cache_bytes: bytes("Pages purgeable"),
    active_bytes: bytes("Pages active"),
    inactive_bytes: bytes("Pages inactive"),
    speculative_bytes: bytes("Pages speculative"),
    pageouts: pages.Pageouts ?? null,
    swapins: pages.Swapins ?? null,
    swapouts: pages.Swapouts ?? null,
  };
}

function parseSwapUsage(text) {
  const used = text.match(/used\s*=\s*([0-9.]+)([MGK])?/i);
  if (!used) return null;
  const unit = { K: 1024, M: 1024 ** 2, G: 1024 ** 3 }[String(used[2] ?? "M").toUpperCase()] ?? 1;
  return Number(used[1]) * unit;
}

function pressureStatus(text) {
  const lower = text.toLowerCase();
  if (lower.includes("critical")) return "critical";
  if (lower.includes("warn") || lower.includes("high")) return "warning";
  const freeMatch = lower.match(/free percentage:\s*([0-9]+(?:\.[0-9]+)?)%/);
  if (freeMatch) {
    const freePercent = Number(freeMatch[1]);
    if (freePercent < 10) return "critical";
    if (freePercent < 25) return "warning";
    if (freePercent >= 50) return "normal";
  }
  if (lower.includes("normal") || lower.includes("ok")) return "normal";
  return null;
}

async function sampleMemory(serverPid, baselineCounters) {
  const [ps, footprint, vm, swap, pressure, pressureLevel] = await Promise.all([
    serverPid ? command(["ps", "-o", "rss=", "-p", String(serverPid)]) : Promise.resolve({ code: 1, stdout: "" }),
    serverPid ? command(["footprint", "-p", String(serverPid)]) : Promise.resolve({ code: 1, stdout: "" }),
    command(["vm_stat"]),
    command(["sysctl", "-n", "vm.swapusage"]),
    command(["memory_pressure", "-Q"]),
    command(["sysctl", "-n", "kern.memorystatus_vm_pressure_level"]),
  ]);
  const parsedVM = parseVMStat(vm.stdout);
  const counters = { pageouts: parsedVM.pageouts, swapins: parsedVM.swapins, swapouts: parsedVM.swapouts };
  const delta = (key) => counters[key] == null || baselineCounters[key] == null ? null : counters[key] - baselineCounters[key];
  const kernelPressure = pressureLevel.code === 0
    ? ({ 1: "normal", 2: "warning", 4: "critical" }[Number(pressureLevel.stdout.trim())] ?? null)
    : null;
  const fallbackPressure = pressureStatus(pressure.stdout);
  const measurements = {
    server_rss_bytes: ps.code === 0 && parseNumber(ps.stdout) != null ? parseNumber(ps.stdout) * 1024 : null,
    server_footprint_bytes: footprint.code === 0 ? parseFootprintText(footprint.stdout, serverPid) : null,
    wired_bytes: parsedVM.wired_bytes,
    compressed_bytes: parsedVM.compressed_bytes,
    free_bytes: parsedVM.free_bytes,
    cache_bytes: parsedVM.cache_bytes,
    pressure: kernelPressure ?? fallbackPressure,
    pressure_source: kernelPressure != null
      ? "kern.memorystatus_vm_pressure_level"
      : "memory_pressure -Q free-percentage fallback; kernel pressure sysctl unavailable",
    pressure_level_raw: pressureLevel.stdout.trim() || null,
    pressure_raw: pressure.stdout.trim() || null,
    swap_used_bytes: parseSwapUsage(swap.stdout),
    pageout_delta: delta("pageouts"),
    swapin_delta: delta("swapins"),
    swapout_delta: delta("swapouts"),
    counter_explanation: "pageout/swapin/swapout are deltas from the session baseline; server RSS overlaps system wired memory and is never added to it",
  };
  const missing = [];
  if (measurements.server_rss_bytes == null) missing.push("server_rss_bytes: ps did not return a live server RSS");
  if (measurements.server_footprint_bytes == null) missing.push("server_footprint_bytes: footprint did not return a parsable phys_footprint line");
  for (const field of ["wired_bytes", "compressed_bytes", "free_bytes", "cache_bytes", "pressure", "swap_used_bytes", "pageout_delta", "swapin_delta", "swapout_delta"]) {
    if (measurements[field] == null) missing.push(`${field}: platform command returned no value`);
  }
  return { ...measurements, missing_measurements: missing };
}

class MemorySampler {
  constructor(file, serverPid = null, maxServerFootprintBytes = DEFAULT_MAX_SERVER_FOOTPRINT_BYTES) {
    this.file = file;
    this.serverPid = serverPid;
    this.maxServerFootprintBytes = maxServerFootprintBytes;
    this.phase = "pre_run";
    this.running = false;
    this.timer = null;
    this.samples = [];
    this.baseline = { pageouts: null, swapins: null, swapouts: null };
    this.unsafe = null;
    this.swapoutGrowth = 0;
    this.previousSwapoutDelta = 0;
    this.sampleQueue = Promise.resolve();
    this.executionId = null;
  }

  async start(attemptId = null) {
    this.attemptId = attemptId;
    this.executionId = null;
    this.baseline = await this.readCounters();
    await this.sample();
    this.running = true;
    this.loop();
  }

  async readCounters() {
    const vm = await command(["vm_stat"]);
    const parsed = parseVMStat(vm.stdout);
    return { pageouts: parsed.pageouts, swapins: parsed.swapins, swapouts: parsed.swapouts };
  }

  setPhase(phase) { this.phase = phase; }

  sample() {
    const next = this.sampleQueue.then(() => this.collectSample());
    this.sampleQueue = next.catch((error) => { this.unsafe ??= `memory sampler failed: ${error.message}`; });
    return next;
  }

  async collectSample() {
    const attemptId = this.attemptId;
    const executionId = this.executionId;
    const phase = this.phase;
    const data = await sampleMemory(this.serverPid, this.baseline);
    const row = { time: new Date().toISOString(), attempt_id: attemptId, execution_id: executionId, phase, ...data };
    this.samples.push(row);
    appendJSONL(this.file, row);
    if (data.swapout_delta != null && this.previousSwapoutDelta != null && data.swapout_delta > this.previousSwapoutDelta) this.swapoutGrowth += 1;
    else this.swapoutGrowth = 0;
    this.previousSwapoutDelta = data.swapout_delta;
    if (data.pressure == null) this.unsafe ??= "memory pressure unavailable";
    if (data.pressure === "critical") this.unsafe ??= "critical memory pressure";
    // Ambient system pressure from the user's own other apps is the expected,
    // permanent condition on this host, not a contamination signal -- so it
    // is only ever a hard stop at "critical". The gate that actually decides
    // fitness is the server's own dirty footprint against the configured
    // ceiling, once the server exists.
    if (this.serverPid != null) {
      const footprintGate = evaluateFootprintGate(data.server_footprint_bytes, this.maxServerFootprintBytes);
      if (!footprintGate.pass) this.unsafe ??= footprintGate.reason;
    }
    if (this.swapoutGrowth >= 3) this.unsafe ??= "swapout increased in three successive samples";
  }

  loop() {
    if (!this.running) return;
    this.timer = setTimeout(async () => {
      try { await this.sample(); } catch (error) { this.unsafe ??= `memory sampler failed: ${error.message}`; } finally { this.loop(); }
    }, 1000);
  }

  async stop() {
    this.running = false;
    if (this.timer) clearTimeout(this.timer);
    await this.sample();
    return this.samples;
  }

  peakSummary(attemptId = null, executionId = null) {
    const samples = this.samples.filter((row) => (attemptId == null || row.attempt_id === attemptId) && (executionId == null || row.execution_id === executionId));
    const min = (field) => {
      const values = samples.map((row) => row[field]).filter((value) => value != null);
      return values.length ? Math.min(...values) : null;
    };
    return {
      sample_count: samples.length,
      peak_server_rss_bytes: Math.max(...samples.map((row) => row.server_rss_bytes ?? 0), 0) || null,
      peak_server_footprint_bytes: Math.max(...samples.map((row) => row.server_footprint_bytes ?? 0), 0) || null,
      peak_wired_bytes: Math.max(...samples.map((row) => row.wired_bytes ?? 0), 0) || null,
      peak_compressed_bytes: Math.max(...samples.map((row) => row.compressed_bytes ?? 0), 0) || null,
      min_free_bytes: min("free_bytes"),
      peak_cache_bytes: Math.max(...samples.map((row) => row.cache_bytes ?? 0), 0) || null,
      max_swap_used_bytes: Math.max(...samples.map((row) => row.swap_used_bytes ?? 0), 0) || null,
      max_pageout_delta: Math.max(...samples.map((row) => row.pageout_delta ?? 0), 0) || null,
      max_swapin_delta: Math.max(...samples.map((row) => row.swapin_delta ?? 0), 0) || null,
      max_swapout_delta: Math.max(...samples.map((row) => row.swapout_delta ?? 0), 0) || null,
      unsafe: this.unsafe,
      phases: [...new Set(samples.map((row) => row.phase))],
    };
  }
}

async function discoverServerPid(experiments) {
  const pattern = experiments.server.process_pattern
    ?? `${path.basename(experiments.server.binary)}.*--port ${experiments.server.port}`;
  const pgrep = await command(["pgrep", "-f", pattern]);
  for (const raw of pgrep.stdout.split("\n").filter(Boolean)) {
    const pid = Number(raw.trim());
    if (!Number.isInteger(pid)) continue;
    const ps = await command(["ps", "-o", "command=", "-p", String(pid)]);
    if (ps.stdout.includes(experiments.model.path)) return pid;
  }
  return null;
}

async function waitForHealth(experiments, timeoutMs = 120_000, abortCheck = () => null) {
  const url = `${rootURL(experiments.server.endpoint)}${experiments.server.health_path}`;
  const expectedStatus = experiments.server.health_accept_status ?? 200;
  const expectedBody = experiments.server.health_body_regex
    ? new RegExp(experiments.server.health_body_regex, "i")
    : /ok|healthy/i;
  const started = Date.now();
  while (Date.now() - started < timeoutMs) {
    const reason = interruption ?? abortCheck();
    if (reason) die(`startup stopped: ${reason}`);
    const result = await httpText(url, { timeoutMs: 2000 });
    if (result.status === expectedStatus && expectedBody.test(result.text)) return { at: new Date().toISOString(), response: result.text.trim() };
    await new Promise((resolve) => setTimeout(resolve, 1000));
  }
  die(`server did not become healthy within ${timeoutMs}ms`);
}

function killGroup(pid, signal) {
  if (!pid) return;
  try { process.kill(-pid, signal); } catch (error) { if (error.code !== "ESRCH") throw error; }
}

function spawnCapture(argv, options = {}) {
  const {
    cwd = REPO_ROOT,
    env = process.env,
    timeoutMs = 0,
    maxBytes = MAX_CAPTURE_BYTES,
    onStart = () => {},
    onTimeout = () => {},
    abortCheck = () => null,
  } = options;
  if (interruption) die(interruption);
  return new Promise((resolve) => {
    const startedAt = Date.now();
    const child = trackChild(spawn(argv[0], argv.slice(1), { cwd, env, detached: true, stdio: ["ignore", "pipe", "pipe"], windowsHide: true }));
    let stdout = "";
    let stderr = "";
    let stdoutTruncated = false;
    let stderrTruncated = false;
    const capture = (chunk, current, truncated) => {
      const next = current + chunk.toString();
      if (Buffer.byteLength(next) <= maxBytes) return [next, truncated];
      return [next.slice(0, maxBytes), true];
    };
    child.stdout.on("data", (chunk) => ([stdout, stdoutTruncated] = capture(chunk, stdout, stdoutTruncated)));
    child.stderr.on("data", (chunk) => ([stderr, stderrTruncated] = capture(chunk, stderr, stderrTruncated)));
    onStart(child.pid);
    let timedOut = false;
    let cancelRequestedAt = null;
    let forceKilledAt = null;
    let timeoutTimer = null;
    let forceTimer = null;
    let safetyTimer = null;
    const cancel = (reason) => {
      if (cancelRequestedAt) return;
      timedOut = true;
      cancelRequestedAt = new Date().toISOString();
      onTimeout(cancelRequestedAt, reason);
      killGroup(child.pid, "SIGTERM");
      forceTimer = setTimeout(() => {
        forceKilledAt = new Date().toISOString();
        killGroup(child.pid, "SIGKILL");
      }, 10_000);
    };
    if (timeoutMs > 0) {
      timeoutTimer = setTimeout(() => {
        cancel("deadline");
      }, timeoutMs);
    }
    safetyTimer = setInterval(() => {
      const reason = interruption ?? abortCheck();
      if (reason) cancel(reason);
    }, 250);
    child.on("error", (error) => {
      if (timeoutTimer) clearTimeout(timeoutTimer);
      if (forceTimer) clearTimeout(forceTimer);
      if (safetyTimer) clearInterval(safetyTimer);
      resolve({ code: null, error: error.message, stdout, stderr, timedOut, cancelRequestedAt, forceKilledAt, wallMs: Date.now() - startedAt });
    });
    child.on("close", (code, signal) => {
      if (timeoutTimer) clearTimeout(timeoutTimer);
      if (forceTimer) clearTimeout(forceTimer);
      if (safetyTimer) clearInterval(safetyTimer);
      resolve({ code, signal, stdout, stderr, stdoutTruncated, stderrTruncated, timedOut, cancelRequestedAt, forceKilledAt, cliExitAt: new Date().toISOString(), wallMs: Date.now() - startedAt, pid: child.pid });
    });
  });
}

async function waitForServerIdle(experiments, timeoutMs = experiments.server.drain.timeout_ms) {
  if (experiments.server.drain?.mode === "not_applicable" || !experiments.server.slots_path) {
    return {
      status: "not_applicable",
      at: new Date().toISOString(),
      explanation: experiments.server.drain?.explanation ?? "backend exposes no slot-idle endpoint",
    };
  }
  const url = `${rootURL(experiments.server.endpoint)}${experiments.server.slots_path}`;
  const started = Date.now();
  while (Date.now() - started < timeoutMs) {
    const response = await httpText(url, { timeoutMs: 2000 });
    if (response.status === 200) {
      let parsed;
      try { parsed = JSON.parse(response.text); } catch { parsed = null; }
      const slots = Array.isArray(parsed) ? parsed : parsed?.slots;
      if (Array.isArray(slots)) {
        const busy = slots.some((slot) => slot.is_processing === true || slot.state === "processing" || slot.state === "generating");
        if (!busy) return { status: "idle", at: new Date().toISOString(), response: parsed };
      }
    }
    await new Promise((resolve) => setTimeout(resolve, experiments.server.drain.poll_ms));
  }
  return { status: "unknown", at: null, explanation: "server /slots did not confirm an idle slot before the drain deadline" };
}

async function clearSlotCache(experiments) {
  if (experiments.server.slot_cache_clear?.mode === "not_applicable" || !experiments.server.slot_cache_clear?.path) {
    return {
      status: "not_applicable",
      at: new Date().toISOString(),
      explanation: experiments.server.slot_cache_clear?.explanation ?? "backend prompt cache is disabled",
    };
  }
  const spec = experiments.server.slot_cache_clear;
  const response = await httpText(`${rootURL(experiments.server.endpoint)}${spec.path}`, { method: spec.method, timeoutMs: 5000 });
  if (response.status === spec.success_status) return { status: "cleared", at: new Date().toISOString(), response: response.text.trim() || null };
  return { status: "unsupported", http_status: response.status, error: response.error ?? (response.text.trim() || null) };
}

function serverArgs(experiments, profileName) {
  const profile = experiments.server.profiles[profileName];
  if (!profile) die(`unknown server profile ${profileName}`);
  const backend = serverBackend(experiments);
  if (backend === "hebrus") {
    return [
      "-m", experiments.model.path,
      ...(experiments.server.common_flags ?? []),
      ...(profile.flags ?? []),
      "--host", experiments.server.host,
      "--port", String(experiments.server.port),
    ];
  }
  if (backend === "turboquant") {
    return [
      experiments.server.model_flag ?? "--model",
      experiments.model.path,
      ...(experiments.server.common_flags ?? []),
      ...(profile.flags ?? []),
      "--host", experiments.server.host,
      "--port", String(experiments.server.port),
    ];
  }
  if (backend === "mlx-lm" || backend === "mlx-vlm") {
    return [
      experiments.server.model_flag ?? "--model",
      experiments.model.path,
      ...(experiments.server.common_flags ?? []),
      ...(profile.flags ?? []),
      "--host", experiments.server.host,
      "--port", String(experiments.server.port),
    ];
  }
  const flags = [...experiments.server.common_flags];
  const replace = (flag, value) => {
    const index = flags.indexOf(flag);
    if (index >= 0 && index + 1 < flags.length) flags[index + 1] = String(value);
    else flags.push(flag, String(value));
  };
  replace("-c", profile.context);
  replace("--spec-type", profile.spec_type);
  replace("-ub", profile.ubatch ?? 256);
  replace("-b", profile.batch ?? 256);
  if (profile.reasoning === "on") {
    replace("--reasoning", "on");
  }
  if (profile.kv) {
    replace("--cache-type-k", profile.kv);
    replace("--cache-type-v", profile.kv);
  }
  const args = ["-m", experiments.model.path, ...flags, "--host", experiments.server.host, "--port", String(experiments.server.port)];
  if (profile.spec_draft != null) args.push("--draft-max", String(profile.spec_draft));
  return args;
}

const MODEL_SERVER_NAMES = ["llama-server", "turboquant-serve", "mlx_lm.server", "mlx_vlm.server", "hebrus-server"];

async function existingModelServer() {
  const processes = await command(["ps", "-axo", "pid=,command="]);
  if (processes.code === 0) {
    return { status: processes.stdout.split("\n").some((line) => MODEL_SERVER_NAMES.some((name) => line.includes(name))) ? "found" : "clear", source: "ps" };
  }
  // Some managed shells deny both ps and pgrep. The configured listener check in
  // startManagedServer remains authoritative for collision prevention; preserve
  // this visibility gap in the runtime manifest instead of treating it as a
  // confirmed existing server.
  for (const name of MODEL_SERVER_NAMES) {
    const pgrep = await command(["pgrep", "-f", name]);
    if (pgrep.code === 0 && pgrep.stdout.trim()) return { status: "found", source: "pgrep", name };
    if (pgrep.code !== 1) {
      return {
        status: "unknown",
        source: "ps+pgrep",
        explanation: `process inspection unavailable (${processes.stderr.trim() || "ps unavailable"}; ${pgrep.stderr.trim() || name})`,
      };
    }
  }
  return { status: "clear", source: "pgrep" };
}

async function startManagedServer(experiments, profileName, resultDir, memorySampler = null) {
  if (!existsSync(experiments.model.path)) die(`model path is missing at ${experiments.model.path}; no download is attempted`);
  const processCheck = await existingModelServer();
  if (processCheck.status === "found") die("an existing model server must be stopped before managed startup");
  const listener = await command(["lsof", "-nP", `-iTCP:${experiments.server.port}`, "-sTCP:LISTEN", "-t"]);
  if (listener.code === 0 || listener.stdout.trim()) die("configured server port is already occupied");
  if (listener.code !== 1 || listener.stderr.trim()) die("could not establish that the server port is free");
  if (interruption) die(interruption);
  const log = path.join(resultDir, "server.log");
  const args = serverArgs(experiments, profileName);
  const stream = trackChild(spawn(experiments.server.binary, args, {
    cwd: REPO_ROOT,
    env: { ...process.env, ...(experiments.server.env ?? {}) },
    detached: true,
    stdio: ["ignore", "ignore", "pipe"],
    windowsHide: true,
  }));
  let closed = false;
  let spawnError = null;
  const closedPromise = new Promise((resolve) => {
    stream.once("close", () => { closed = true; resolve(); });
    stream.once("error", (error) => { spawnError = error; });
  });
  const logFile = await import("node:fs").then(({ createWriteStream }) => createWriteStream(log, { mode: 0o600, flags: "a" }));
  stream.stderr.pipe(logFile, { end: false });
  if (memorySampler) {
    memorySampler.serverPid = stream.pid;
    memorySampler.setPhase("startup");
  }
  const stop = async () => {
    if (!closed) {
      killGroup(stream.pid, "SIGTERM");
      const timer = setTimeout(() => killGroup(stream.pid, "SIGKILL"), 10_000);
      try { await closedPromise; } finally { clearTimeout(timer); }
    }
    logFile.end();
  };
  const startedAt = new Date().toISOString();
  try {
    await waitForHealth(experiments, 120_000, () => memorySampler?.unsafe ?? spawnError?.message ?? (closed ? "server exited" : null));
  } catch (error) {
    await stop();
    throw error;
  }
  return { pid: stream.pid, args, log, started_at: startedAt, process_check: processCheck, stop };
}

function copyTree(source, destination) {
  rmSync(destination, { recursive: true, force: true });
  cpSync(source, destination, {
    recursive: true,
    dereference: false,
    force: true,
    errorOnExist: false,
    // Keep dependency trees outside each temporary worktree. The benchmark must
    // expose the same installed dependencies to the model and to the host grader,
    // while avoiding a fresh multi-gigabyte copy for every attempt.
    filter: (sourcePath) => path.basename(sourcePath) !== "node_modules",
  });
  const dependencySource = path.join(source, "node_modules");
  const dependencyDestination = path.join(destination, "node_modules");
  if (existsSync(dependencySource)) symlinkSync(dependencySource, dependencyDestination, "junction");
}

function writeRelative(root, rel, content) {
  const full = path.resolve(root, rel);
  if (!full.startsWith(`${path.resolve(root)}${path.sep}`)) die(`fixture path escapes root: ${rel}`);
  mkdirSync(path.dirname(full), { recursive: true, mode: 0o700 });
  writeFileSync(full, content, { mode: 0o600 });
}

function applyPatches(root, patches) {
  for (const patch of patches ?? []) {
    const file = path.join(root, patch.path);
    const source = readFileSync(file, "utf8");
    const count = source.split(patch.oldText).length - 1;
    if (count !== 1) die(`reference patch for ${patch.path} matched ${count} times`);
    writeFileSync(file, source.replace(patch.oldText, patch.newText), { mode: 0o600 });
  }
}

function injectSyntaxError(root, invalid) {
  const file = path.join(root, invalid.path);
  const source = readFileSync(file, "utf8");
  writeFileSync(file, `${source}\nthis is an intentionally invalid fixture probe\n`, { mode: 0o600 });
}

function loadPilotTask(contractTask, pilotRoot) {
  const manifest = loadJSON(path.resolve(REPO_ROOT, contractTask.pilot_manifest));
  const task = manifest.tasks?.[path.basename(contractTask.source_root)];
  if (!task) die(`pilot task ${contractTask.source_root} is absent from ${contractTask.pilot_manifest}`);
  return { ...task, id: path.basename(contractTask.source_root), source_root: path.resolve(REPO_ROOT, contractTask.source_root), pilot_root: pilotRoot };
}

function scenarioData(contractTask) {
  const scenario = loadJSON(path.resolve(REPO_ROOT, contractTask.scenario));
  if (scenario.id !== path.basename(contractTask.scenario, ".json")) die(`scenario id/path mismatch for ${contractTask.scenario}`);
  return scenario;
}

function validateContractShape(contract) {
  for (const [id, task] of Object.entries(contract.tasks ?? {})) {
    if (!Array.isArray(task.source_files) || task.source_files.length === 0) {
      die(`fixture ${id} must declare a non-empty source_files allowlist`);
    }
    for (const sourceFile of task.source_files) {
      const relative = String(sourceFile);
      const normalized = path.posix.normalize(relative.replaceAll(path.sep, "/"));
      if (path.posix.isAbsolute(relative) || normalized === ".." || normalized.startsWith("../") || normalized !== relative) {
        die(`fixture ${id} has an unsafe source_files entry: ${relative}`);
      }
    }
    if (task.invalid?.path && !task.source_files.includes(task.invalid.path)) {
      die(`fixture ${id} invalid probe must target an allowlisted source file`);
    }
    for (const absentFile of task.base_absent_files ?? []) {
      const relative = String(absentFile);
      const normalized = path.posix.normalize(relative.replaceAll(path.sep, "/"));
      if (path.posix.isAbsolute(relative) || normalized === ".." || normalized.startsWith("../") || normalized !== relative) {
        die(`fixture ${id} has an unsafe base_absent_files entry: ${relative}`);
      }
      if (!task.source_files.includes(relative)) die(`fixture ${id} base_absent_files entry is not allowlisted: ${relative}`);
    }
    for (const patch of task.reference_local_patches ?? []) {
      const relative = String(patch.path ?? "");
      const normalized = path.posix.normalize(relative.replaceAll(path.sep, "/"));
      if (!relative || path.posix.isAbsolute(relative) || normalized === ".." || normalized.startsWith("../") || normalized !== relative) {
        die(`fixture ${id} has an unsafe reference_local_patches entry: ${relative}`);
      }
      if (!task.source_files.includes(relative)) die(`fixture ${id} local reference patch is not allowlisted: ${relative}`);
      if (typeof patch.oldText !== "string" || typeof patch.newText !== "string") die(`fixture ${id} local reference patch for ${relative} must contain string oldText/newText`);
    }
    if (task.reference_local_root != null) {
      const relative = String(task.reference_local_root);
      const normalized = path.posix.normalize(relative.replaceAll(path.sep, "/"));
      if (!relative || path.posix.isAbsolute(relative) || normalized === ".." || normalized.startsWith("../") || normalized !== relative) {
        die(`fixture ${id} has an unsafe reference_local_root entry: ${relative}`);
      }
    }
  }
}

function taskDescription(contractTask) {
  if (contractTask.kind === "scenario") return scenarioData(contractTask).report;
  const task = loadPilotTask(contractTask, DEFAULT_PILOT);
  return task.report.replace(/\n?Test command:[\s\S]*$/i, "").trim();
}

function repairPrompt(report, sourceHints = [], agent = {}) {
  const bashMode = agent.bash_mode ?? "";
  const prompt = [
    "Repair the reported bug or bugs in the provided repository.",
    bashMode === "only"
      ? "Use the Bash tool only to inspect and modify the source, then run the relevant existing checks. Apply the complete repair; do not only explain the diagnosis."
      : bashMode === "guarded"
        ? "Use the native read, search, edit, and write tools or the Bash tool to inspect and modify the source, then run the relevant existing checks. Apply the complete repair; do not only explain the diagnosis."
        : "Use the native read, search, edit, and write tools to inspect and modify the source. Apply the complete repair; do not only explain the diagnosis.",
  ];
  if (sourceHints.length) prompt.push(`Start by reading the relevant source paths directly: ${sourceHints.map((file) => `\`${file}\``).join(", ")}.`);
  prompt.push("", report);
  return prompt.join("\n");
}

function createBaseWorktree(contractTask, destination) {
  if (contractTask.kind === "scenario") {
    const scenario = scenarioData(contractTask);
    for (const [rel, content] of Object.entries(scenario.files ?? {})) writeRelative(destination, rel, content);
    return { scenario, task: null };
  }
  const task = loadPilotTask(contractTask, DEFAULT_PILOT);
  copyTree(task.source_root, destination);
  return { scenario: null, task };
}

function installHiddenTests(root, task, report) {
  for (const [rel, content] of Object.entries(task.test_files ?? {})) writeRelative(root, rel, content);
  return (task.test_command ?? []).map((arg) => String(arg).replaceAll("{{REPORT}}", report));
}

function taskManifest(contractTask) {
  if (contractTask.kind === "scenario") return null;
  return loadPilotTask(contractTask, DEFAULT_PILOT);
}

function snapshot(root) {
  return hashTree(root).files.filter((entry) => !entry.symlink);
}

function changedFiles(before, after) {
  const a = new Map(before.map((entry) => [entry.path, entry.sha256]));
  const b = new Map(after.map((entry) => [entry.path, entry.sha256]));
  const changed = [];
  for (const [file, hash] of a) if (b.get(file) !== hash) changed.push(file);
  for (const file of b.keys()) if (!a.has(file)) changed.push(file);
  return [...new Set(changed)].sort();
}

function readTextIfPresent(root, relative) {
  const file = path.join(root, relative);
  try {
    return existsSync(file) ? readFileSync(file, "utf8") : null;
  } catch {
    return null;
  }
}

function sourceOnlyChange(changed, sourceFiles) {
  if (!changed.length) return { ok: false, reason: "no source edit" };
  const disallowed = changed.filter((file) => !sourceFiles.includes(file));
  return disallowed.length ? { ok: false, reason: `disallowed files changed: ${disallowed.join(", ")}` } : { ok: true, reason: null };
}

function parseReport(reportPath) {
  try { return JSON.parse(readFileSync(reportPath, "utf8")); } catch (error) { return { parse_error: error.message }; }
}

function gradeRepoReport(reportPath, task, fixed) {
  const report = parseReport(reportPath);
  if (report.parse_error) return { pass: false, reason: report.parse_error, expected: [], actual: [] };
  const testFilePrefixes = (task.test_command ?? [])
    .filter((arg) => /\.(?:test|spec)\.[cm]?[jt]sx?$/.test(String(arg)))
    .map((arg) => String(arg).trim());
  const nameVariants = (name) => {
    const trimmed = String(name).trim();
    const variants = new Set([trimmed]);
    for (const prefix of testFilePrefixes) {
      if (trimmed.startsWith(`${prefix} `)) variants.add(trimmed.slice(prefix.length + 1));
      else variants.add(`${prefix} ${trimmed}`);
    }
    return [...variants];
  };
  const expectedNames = [...(task.fail_to_pass ?? []), ...(task.pass_to_pass ?? [])].map((name) => String(name).trim());
  const tracked = new Set(expectedNames.flatMap(nameVariants));
  const actual = new Map();
  for (const suite of report.testResults ?? []) {
    for (const assertion of suite.assertionResults ?? []) {
      const name = String(assertion.fullName ?? assertion.FullName ?? "").trim();
      const status = String(assertion.status ?? assertion.Status ?? "");
      if (tracked.has(name)) (actual.get(name) ?? actual.set(name, []).get(name)).push(status);
    }
  }
  const missing = [];
  const wrong = [];
  const check = (name, expected) => {
    const statuses = nameVariants(name).flatMap((variant) => actual.get(variant) ?? []);
    if (!statuses.length) missing.push(name);
    else if (statuses.some((status) => status !== expected)) wrong.push({ name, statuses, expected });
  };
  for (const name of task.pass_to_pass ?? []) check(name, "passed");
  for (const name of task.fail_to_pass ?? []) check(name, fixed ? "passed" : "failed");
  return {
    pass: missing.length === 0 && wrong.length === 0,
    reason: missing.length ? `missing expected tests: ${missing.join(" | ")}` : wrong.length ? `unexpected test statuses: ${JSON.stringify(wrong)}` : null,
    missing,
    wrong,
    actual: [...actual.entries()],
  };
}

async function runHostCommand(argv, cwd, timeoutMs, reportPath, abortCheck = () => null) {
  const result = await spawnCapture(argv, { cwd, timeoutMs, env: { ...process.env, CI: "1" }, abortCheck });
  if (reportPath && !existsSync(reportPath)) {
    // Report absence is a verification failure, never an implicit pass.
    result.report_missing = true;
  }
  return result;
}

async function verifyDirectoryScenario(contractTask, root, scratchDir, abortCheck = () => null) {
  const scenario = scenarioData(contractTask);
  const configuredOracle = path.resolve(REPO_ROOT, contractTask.oracle);
  const oracleRoot = existsSync(path.join(configuredOracle, scenario.id, "test", "run.mjs"))
    ? configuredOracle
    : path.dirname(configuredOracle);
  const input = path.join(scratchDir, "grade-input.json");
  const output = path.join(scratchDir, "grade-output.json");
  rmSync(output, { force: true });
  writeJSON(input, { scenario, root, oracleRoot, scratchDir, options: {
    successMarker: contractTask.success_marker, expectedOracleKeys: contractTask.oracle_keys,
  } });
  const result = await spawnCapture([process.execPath, path.join(HERE, "grade-worker.mjs"), input, output], { timeoutMs: 90_000, abortCheck });
  if (result.code !== 0 || !existsSync(output)) return { pass: false, infrastructure_error: true, command: result };
  return loadJSON(output);
}

async function verifyDirectoryPilot(contractTask, root, verifyDir, scratchDir, fixed, timeoutMs, abortCheck = () => null) {
  const task = taskManifest(contractTask);
  const reportPath = path.join(scratchDir, "oracle.json");
  copyTree(root, verifyDir);
  const commandArgs = installHiddenTests(verifyDir, task, reportPath);
  const before = snapshot(verifyDir);
  const commandResult = await runHostCommand(commandArgs, verifyDir, timeoutMs, reportPath, abortCheck);
  const after = snapshot(verifyDir);
  const testChanges = changedFiles(before, after);
  const report = gradeRepoReport(reportPath, task, fixed);
  return {
    pass: commandResult.code === 0 && !commandResult.timedOut && !commandResult.report_missing && report.pass && testChanges.length === 0,
    command: commandResult,
    report,
    test_changes: testChanges,
    report_path: reportPath,
  };
}

async function fetchReferenceRoot(contractTask, target, allowFetch) {
  const localPatches = contractTask.reference_local_patches ?? [];
  if (localPatches.length > 0) {
    // The pinned upstream checkout is authoritative when it is available. This
    // deterministic local reconstruction keeps fixture validation usable in an
    // offline shell while retaining the upstream commit and the exact derived
    // patch set in the validation artifact.
    const sourceRoot = path.resolve(REPO_ROOT, contractTask.source_root);
    if (!existsSync(sourceRoot)) die(`local reference source root is absent: ${sourceRoot}`);
    copyTree(sourceRoot, target);
    applyPatches(target, localPatches);
    return {
      root: target,
      source: "local-derived-patch",
      commit: contractTask.reference?.commit ?? null,
      patch_sha256: sha256(JSON.stringify(localPatches)),
      explanation: "offline fallback derived from the pinned base fixture plus the recorded reference patch; replace with the upstream checkout when network access is available",
    };
  }
  if (contractTask.reference_local_root) {
    const localRepo = path.resolve(REPO_ROOT, contractTask.reference_local_root);
    const reference = contractTask.reference;
    if (!reference?.commit) die(`local reference repository is configured without a pinned commit for ${contractTask.source_root}`);
    if (!existsSync(path.join(localRepo, ".git")) && !existsSync(path.join(localRepo, "HEAD"))) {
      die(`local reference repository is absent: ${localRepo}`);
    }
    if (existsSync(target)) rmSync(target, { recursive: true, force: true });
    mkdirSync(path.dirname(target), { recursive: true, mode: 0o700 });
    const archive = path.join(path.dirname(target), `${path.basename(target)}.tar`);
    rmSync(archive, { force: true });
    const materialize = await spawnCapture(["git", "-C", localRepo, "archive", "--format=tar", "-o", archive, reference.commit], { cwd: REPO_ROOT, timeoutMs: 120_000 });
    if (materialize.code !== 0) die(`could not materialize local reference ${localRepo}@${reference.commit}: ${materialize.stderr}`);
    mkdirSync(target, { recursive: true, mode: 0o700 });
    const extract = await spawnCapture(["tar", "-xf", archive, "-C", target], { cwd: REPO_ROOT, timeoutMs: 120_000 });
    rmSync(archive, { force: true });
    if (extract.code !== 0) die(`could not extract local reference ${localRepo}@${reference.commit}: ${extract.stderr}`);
    return {
      root: target,
      source: "local-reference-repository",
      repository: localRepo,
      commit: reference.commit,
      explanation: "offline reference materialized from the pinned commit in the local holdout repository; no network fetch was used",
    };
  }
  const referenceReady = existsSync(target) && (contractTask.source_files ?? []).every((sourceFile) => existsSync(path.join(target, sourceFile)));
  if (referenceReady) return { root: target, source: "local" };
  if (existsSync(target)) rmSync(target, { recursive: true, force: true });
  const reference = contractTask.reference;
  if (!reference) return { root: null, status: "pending", explanation: "no reference source is configured" };
  if (!allowFetch) return { root: null, status: "pending", explanation: `reference checkout absent; rerun validation with --fetch-reference for ${reference.repo}@${reference.commit}` };
  mkdirSync(path.dirname(target), { recursive: true, mode: 0o700 });
  const clone = await spawnCapture(["git", "clone", "--filter=blob:none", "--no-checkout", reference.repo, target], { cwd: REPO_ROOT, timeoutMs: 120_000 });
  if (clone.code !== 0) die(`could not clone reference ${reference.repo}: ${clone.stderr}`);
  const checkout = await spawnCapture(["git", "-C", target, "fetch", "--depth=1", "origin", reference.commit], { cwd: REPO_ROOT, timeoutMs: 120_000 });
  if (checkout.code !== 0) die(`could not fetch reference commit ${reference.commit}: ${checkout.stderr}`);
  const materialize = await spawnCapture(["git", "-C", target, "archive", "--format=tar", "-o", path.join(target, "../reference.tar"), reference.commit], { cwd: REPO_ROOT, timeoutMs: 120_000 });
  if (materialize.code !== 0) die(`could not materialize reference commit ${reference.commit}: ${materialize.stderr}`);
  const extract = await spawnCapture(["tar", "-xf", path.join(target, "../reference.tar"), "-C", target], { cwd: REPO_ROOT, timeoutMs: 120_000 });
  if (extract.code !== 0) die(`could not extract reference commit ${reference.commit}: ${extract.stderr}`);
  rmSync(path.join(target, "../reference.tar"), { force: true });
  return { root: target, source: "fetched", commit: reference.commit };
}

function scenarioBaseImportFailuresAreExpected(contractTask, output) {
  if (output.includes("SyntaxError")) return false;
  const expected = contractTask.base_absent_files ?? [];
  const importFailureLines = output.split("\n").filter((line) => /could not (?:import|be imported)|ERR_MODULE_NOT_FOUND|Cannot find module/.test(line));
  return importFailureLines.every((line) => expected.some((file) => line.includes(file)));
}

async function validateTask(contractTask, sessionDir, { fetchReference = false, timeoutMs = 180_000 } = {}) {
  const baseDir = mkdtempSync(path.join(sessionDir, `fixture-${contractTask.kind}-`));
  const base = createBaseWorktree(contractTask, baseDir);
  const invalidDir = mkdtempSync(path.join(sessionDir, `invalid-${contractTask.kind}-`));
  copyTree(baseDir, invalidDir);
  if (contractTask.invalid) injectSyntaxError(invalidDir, contractTask.invalid);
  const scratch = mkdtempSync(path.join(sessionDir, "fixture-grade-"));
  let baseResult;
  let invalidResult;
  if (contractTask.kind === "scenario") {
    baseResult = await verifyDirectoryScenario(contractTask, baseDir, scratch);
    invalidResult = await verifyDirectoryScenario(contractTask, invalidDir, scratch);
  } else {
    const baseVerify = mkdtempSync(path.join(sessionDir, "fixture-base-verify-"));
    const invalidVerify = mkdtempSync(path.join(sessionDir, "fixture-invalid-verify-"));
    baseResult = await verifyDirectoryPilot(contractTask, baseDir, baseVerify, scratch, false, timeoutMs);
    invalidResult = await verifyDirectoryPilot(contractTask, invalidDir, invalidVerify, scratch, false, timeoutMs);
  }
  let referenceResult = { pass: false, status: "pending", explanation: "reference validation not run" };
  if (contractTask.kind === "scenario") {
    const referenceDir = mkdtempSync(path.join(sessionDir, `reference-${contractTask.kind}-`));
    copyTree(baseDir, referenceDir);
    for (const [rel, content] of Object.entries(contractTask.reference_files ?? {})) writeRelative(referenceDir, rel, content);
    applyPatches(referenceDir, contractTask.reference_patches);
    referenceResult = await verifyDirectoryScenario(contractTask, referenceDir, scratch);
  } else {
    const referenceTarget = path.join(sessionDir, "references", path.basename(contractTask.source_root));
    const fetched = await fetchReferenceRoot(contractTask, referenceTarget, fetchReference);
    if (fetched.root) {
      const referenceDir = mkdtempSync(path.join(sessionDir, "reference-pilot-"));
      copyTree(baseDir, referenceDir);
      for (const sourceFile of contractTask.source_files ?? []) {
        const source = path.join(fetched.root, sourceFile);
        if (!existsSync(source)) die(`reference source file missing: ${source}`);
        writeRelative(referenceDir, sourceFile, readFileSync(source));
      }
      const referenceVerify = mkdtempSync(path.join(sessionDir, "reference-verify-"));
      referenceResult = await verifyDirectoryPilot(contractTask, referenceDir, referenceVerify, scratch, true, timeoutMs);
      referenceResult.reference_source = fetched;
    }
  }
  const result = {
    task: path.basename(contractTask.scenario ?? contractTask.source_root),
    base: { pass: contractTask.kind === "pilot"
      ? baseResult.report?.pass === true && baseResult.command?.code === 1 && !baseResult.command?.timedOut && !baseResult.command?.report_missing && baseResult.test_changes?.length === 0
      : baseResult.pass === false && !baseResult.infrastructure_error && baseResult.oracle?.exit_code === 1 && /^FAIL [A-Z]:/m.test(baseResult.oracle?.output ?? "") && scenarioBaseImportFailuresAreExpected(contractTask, baseResult.oracle?.output ?? ""), details: baseResult },
    reference: referenceResult,
    invalid: { pass: invalidResult.pass === false, details: invalidResult },
    completed_at: new Date().toISOString(),
  };
  // A base must reproduce a failure, the reference must pass every expected check, and the invalid
  // state must fail closed. No missing/pending result is promoted to pass.
  result.pass = result.base.pass === true && referenceResult.pass === true && result.invalid.pass === true;
  return result;
}

function parseTrace(file) {
  if (!existsSync(file)) return [];
  return readJSONL(file);
}

function traceHasTool(trace, name) {
  return trace.some((event) => {
    if (event.type !== "tool_start") return false;
    const data = event.data ?? {};
    return data.function?.name === name || data.Function?.Name === name || data.name === name;
  });
}

function traceRequestData(data) {
  if (data?.request) return { request: data.request, meta: data.meta ?? {} };
  return { request: data, meta: {} };
}

function firstNumber(object, keys) {
  for (const key of keys) if (object && object[key] != null && Number.isFinite(Number(object[key]))) return Number(object[key]);
  return null;
}

function extractTurnRows(trace, attemptId) {
  const requests = trace.filter((event) => event.type === "request");
  const rows = [];
  for (let i = 0; i < requests.length; i += 1) {
    const requestEvent = requests[i];
    const parsedRequest = traceRequestData(requestEvent.data);
    const turn = parsedRequest.meta.turn ?? i + 1;
    const responseEvent = trace.slice(trace.indexOf(requestEvent) + 1).find((event) => event.type === "response");
    const responseIndex = responseEvent ? trace.indexOf(responseEvent) : trace.length;
    const nextRequestIndex = i + 1 < requests.length ? trace.indexOf(requests[i + 1]) : trace.length;
    const relevant = trace.slice(responseIndex, nextRequestIndex);
    const response = responseEvent?.data ?? {};
    const choice = response.choices?.[0] ?? response.Choices?.[0] ?? {};
    const message = choice.message ?? choice.Message ?? {};
    const usage = response.usage ?? response.Usage ?? {};
    const timings = response.timings ?? response.Timings ?? {};
    const responseMeta = relevant.find((event) => event.type === "response_meta")?.data ?? {};
    const toolStart = relevant.find((event) => event.type === "tool_start");
    const toolResult = relevant.find((event) => event.type === "tool_result");
    const output = toolResult?.data?.output ?? {};
    const toolValue = output.result ?? {};
    const reasoningDetails = usage.completion_tokens_details ?? usage.CompletionTokensDetails ?? {};
    const cacheTokens = cachedPromptTokens(response);
    const speculation = {
      accepted: firstNumber(timings, ["accepted_n", "accepted_tokens", "draft_accepted"]),
      drafted: firstNumber(timings, ["draft_n", "drafted_n", "draft_tokens"]),
      rejected: firstNumber(timings, ["rejected_n", "rejected_tokens"]),
      explanation: "null means the server response did not expose a speculation counter",
    };
    const row = {
      attempt_id: attemptId,
      request_index: turn,
      tool_schema_hash: parsedRequest.meta.tool_schema_sha256 ?? null,
      request_bytes: parsedRequest.meta.request_bytes ?? null,
      request_tokens: firstNumber(usage, ["prompt_tokens", "PromptTokens"]),
      cache_tokens: cacheTokens,
      newly_evaluated_tokens: firstNumber(timings, ["prompt_n", "prompt_tokens", "newly_evaluated_tokens"]),
      prompt_ms: firstNumber(timings, ["prompt_ms", "PromptMS"]),
      generation_ms: firstNumber(timings, ["predicted_ms", "PredictedMS", "generation_ms"]),
      response_ms: firstNumber(responseMeta, ["elapsed_ms"]),
      reasoning_tokens: firstNumber(reasoningDetails, ["reasoning_tokens", "reasoningTokens"]) ?? (message.reasoning_content || message.reasoning ? null : 0),
      output_tokens: firstNumber(usage, ["completion_tokens", "CompletionTokens"]) ?? firstNumber(timings, ["predicted_n", "PredictedN"]),
      tool_type: toolStart?.data?.function?.name ?? toolStart?.data?.Function?.Name ?? null,
      tool_time_ms: toolResult?.data?.duration_ms ?? null,
      edit_success: toolValue.applied === true ? true : toolValue.applied === false ? false : toolValue.created === true ? true : null,
      finish_reason: choice.finish_reason ?? choice.FinishReason ?? null,
      status: null,
      speculation,
      missing_measurements: [
        ...(parsedRequest.meta.request_bytes == null ? ["request_bytes"] : []),
        ...(cacheTokens == null ? ["cache_tokens"] : []),
        ...(speculation.accepted == null && speculation.drafted == null ? ["speculative_counters"] : []),
        ...(firstNumber(usage, ["prompt_tokens", "PromptTokens"]) == null ? ["request_tokens"] : []),
        ...(firstNumber(timings, ["prompt_n", "prompt_tokens", "newly_evaluated_tokens"]) == null ? ["newly_evaluated_tokens"] : []),
        ...(firstNumber(timings, ["prompt_ms", "PromptMS"]) == null ? ["prompt_ms"] : []),
        ...(firstNumber(timings, ["predicted_ms", "PredictedMS", "generation_ms"]) == null ? ["generation_ms"] : []),
        ...(responseMeta.elapsed_ms == null ? ["response_ms"] : []),
        ...(firstNumber(usage, ["completion_tokens", "CompletionTokens"]) == null && firstNumber(timings, ["predicted_n", "PredictedN"]) == null ? ["output_tokens"] : []),
      ],
    };
    rows.push(row);
  }
  const summary = trace.findLast?.((event) => event.type === "summary") ?? [...trace].reverse().find((event) => event.type === "summary");
  const summaryStatus = summary?.data?.status ?? null;
  for (const row of rows) row.status = summaryStatus;
  return rows;
}

function parseHebrusServerMetrics(file, offset = 0) {
  if (!existsSync(file)) return { status: "missing", metrics: [], missing_measurements: ["server_log"] };
  const bytes = readFileSync(file);
  const fragment = bytes.subarray(Math.min(offset, bytes.length)).toString("utf8");
  const metrics = [];
  let current = null;
  for (const line of fragment.split("\n")) {
    const prompt = line.match(/(?:chat|completion) ctx=(\d+)\.\.(\d+):(\d+).*prompt done ([0-9]+(?:\.[0-9]+)?)s/);
    if (prompt) {
      const promptTokens = Number(prompt[2]);
      const newlyEvaluated = Number(prompt[3]);
      const promptSeconds = Number(prompt[4]);
      current = {
        prompt_tokens: promptTokens,
        cached_tokens: Number(prompt[1]),
        newly_evaluated_tokens: newlyEvaluated,
        prefill_seconds: promptSeconds,
        prefill_tokens_per_sec: promptSeconds > 0 ? newlyEvaluated / promptSeconds : null,
        completion_tokens: null,
        generation_seconds: null,
        generation_tokens_per_sec: null,
        finish_reason: null,
      };
      metrics.push(current);
      continue;
    }
    const generation = line.match(/(?:chat|completion) ctx=\d+\.\.\d+:\d+ gen=(\d+).*finish=([^\s]+).* ([0-9]+(?:\.[0-9]+)?)s(?:\r)?$/);
    if (generation && current) {
      const completionTokens = Number(generation[1]);
      const generationSeconds = Number(generation[3]);
      current.completion_tokens = completionTokens;
      current.generation_seconds = generationSeconds;
      current.generation_tokens_per_sec = generationSeconds > 0 ? completionTokens / generationSeconds : null;
      current.finish_reason = generation[2];
    }
  }
  return {
    status: metrics.length > 0 ? "parsed" : "no_request_metrics",
    metrics,
    missing_measurements: metrics.length > 0 ? [] : ["prefill/generation server log metrics"],
    raw_fragment_sha256: sha256(Buffer.from(fragment)),
  };
}

function sourcePatchFromTrace(trace) {
  const patches = [];
  let pending = null;
  for (const event of trace) {
    if (event.type === "edit_backup") pending = { kind: "edit", ...event.data };
    if (event.type === "write") patches.push({ kind: "write", ...event.data });
    if (event.type === "tool_result" && event.data?.name === "edit") {
      const output = event.data.output ?? {};
      const result = output.result ?? {};
      if (pending) patches.push({ ...pending, applied: output.error == null && result.applied === true, result, error: output.error ?? null });
      pending = null;
    }
  }
  return patches;
}

async function findTrace(cliArtifactParent, stderr) {
  const match = stderr.match(/artifacts:\s*(\S+)/);
  if (match && existsSync(path.join(match[1], "trace.jsonl"))) return path.join(match[1], "trace.jsonl");
  if (!existsSync(cliArtifactParent)) return null;
  const dirs = readdirSync(cliArtifactParent, { withFileTypes: true }).filter((entry) => entry.isDirectory()).map((entry) => path.join(cliArtifactParent, entry.name));
  dirs.sort().reverse();
  return dirs.map((dir) => path.join(dir, "trace.jsonl")).find(existsSync) ?? null;
}

async function clearCacheOrStop(experiments) {
  const idle = await waitForServerIdle(experiments);
  if (idle.status !== "idle" && idle.status !== "not_applicable") {
    die(`cannot establish a clean prompt-cache state because the server did not become idle: ${idle.explanation ?? idle.status}; stop and resume with a fresh managed server`);
  }
  const cleared = await clearSlotCache(experiments);
  if (cleared.status === "cleared" || cleared.status === "not_applicable") return { idle, ...cleared };
  die(`cannot establish a clean prompt-cache state: ${cleared.error ?? cleared.status}; stop and resume with a fresh managed server`);
}

function validateExperimentShape(experiments, contract) {
  const definitions = experiments.experiments;
  if (!Array.isArray(definitions) || definitions.length === 0) die("experiment configuration must contain experiments");
  const ids = new Set();
  for (let index = 0; index < definitions.length; index += 1) {
    const experiment = definitions[index];
    if (!experiment.id || ids.has(experiment.id)) die(`experiment id is missing or duplicated: ${experiment.id ?? "<empty>"}`);
    ids.add(experiment.id);
    if (!Array.isArray(experiment.tasks) || experiment.tasks.length === 0) die(`experiment ${experiment.id} must declare tasks`);
    if (!Array.isArray(experiment.seeds) || experiment.seeds.length === 0) die(`experiment ${experiment.id} must declare seeds`);
    if (!experiments.budgets?.[experiment.budget]) die(`experiment ${experiment.id} names unknown budget ${experiment.budget}`);
    if (!experiments.server.profiles?.[experiment.server_profile]) die(`experiment ${experiment.id} names unknown server profile ${experiment.server_profile}`);
    if (new Set(experiment.tasks).size !== experiment.tasks.length) die(`experiment ${experiment.id} contains duplicate tasks`);
    if (new Set(experiment.seeds.map((seed) => String(seed))).size !== experiment.seeds.length) die(`experiment ${experiment.id} contains duplicate seeds`);
    if (experiment.requires != null && experiment.requires !== "evidence-selected") die(`experiment ${experiment.id} has unsupported requires value ${experiment.requires}`);
    for (const task of experiment.tasks) if (!contract.tasks[task]) die(`experiment ${experiment.id} names unknown task ${task}`);
    for (const dependency of experiment.depends_on ?? []) {
      const dependencyIndex = definitions.findIndex((candidate) => candidate.id === dependency.experiment_id);
      if (dependencyIndex < 0) die(`experiment ${experiment.id} depends on unknown experiment ${dependency.experiment_id}`);
      if (dependencyIndex >= index) die(`experiment ${experiment.id} must depend on an earlier experiment: ${dependency.experiment_id}`);
      if (dependency.condition !== "verified_repair_no_pressure") die(`unsupported dependency condition for ${experiment.id}`);
    }
  }
}

async function buildAttemptPlan(experiments, contract) {
  const plan = [];
  for (let experimentIndex = 0; experimentIndex < experiments.experiments.length; experimentIndex += 1) {
    const experiment = experiments.experiments[experimentIndex];
    for (let seedIndex = 0; seedIndex < experiment.seeds.length; seedIndex += 1) {
      const orderedTasks = seedIndex % 2 === 0 ? experiment.tasks : [...experiment.tasks].reverse();
      for (const task of orderedTasks) {
        if (!contract.tasks[task]) die(`experiment ${experiment.id} names unknown task ${task}`);
        plan.push({
          attempt_id: `${experiment.id}__seed-${experiment.seeds[seedIndex]}__${task}`,
          experiment_id: experiment.id,
          task,
          seed: experiment.seeds[seedIndex],
          task_order: orderedTasks.indexOf(task),
          counterbalance_block: seedIndex % 2 === 0 ? "forward" : "reverse",
          experiment_index: experimentIndex,
          scored: experiment.scored !== false,
        });
      }
    }
  }
  return plan;
}

function writeDecisions(file, experiments, experimentsFile = DEFAULT_EXPERIMENTS) {
  const body = [
    "# Optimization decisions",
    "",
    "Preparation completed; live measurements and fixture verification remain pending.",
    "",
    `Harness configuration: ${path.relative(REPO_ROOT, experimentsFile)}`,
    "",
    ...experiments.experiments.map((experiment) => [
      `## ${experiment.id}`,
      "",
      `Hypothesis: ${experiment.hypothesis}`,
      "",
      "- Baseline: pending",
      "- Evidence: pending",
      "- Decision: pending",
      "- Next command: pending fixture validation and the ordered gate",
      "",
    ].join("\n")),
  ].join("\n");
  writeFileSync(file, body, { mode: 0o600 });
}

function preparationInputs(experimentsFile, contractFile, contract, experiments) {
  const files = new Set([experimentsFile, contractFile, path.join(HERE, "runner.mjs"), path.join(HERE, "grade-worker.mjs"), path.join(REPO_ROOT, "benchmarks/grade.mjs"), experiments.server.binary]);
  if (Array.isArray(experiments.server.version_probe) && isRegularFile(experiments.server.version_probe[0])) files.add(experiments.server.version_probe[0]);
  files.add(path.join(HERE, "cache-metrics.mjs"));
  const trees = new Set();
  if (existsSync(experiments.model.path)) {
    if (statSync(experiments.model.path).isDirectory()) trees.add(experiments.model.path);
    else files.add(experiments.model.path);
  }
  if (experiments.server.template_path && existsSync(experiments.server.template_path)) {
    if (statSync(experiments.server.template_path).isDirectory()) trees.add(experiments.server.template_path);
    else files.add(experiments.server.template_path);
  }
  const runtimePaths = experiments.server.runtime_package_paths
    ?? (experiments.server.runtime_package_path ? [experiments.server.runtime_package_path] : []);
  for (const runtimePath of runtimePaths) {
    if (!existsSync(runtimePath)) continue;
    if (statSync(runtimePath).isDirectory()) trees.add(runtimePath);
    else if (isRegularFile(runtimePath)) files.add(runtimePath);
  }
  for (const task of Object.values(contract.tasks)) {
    if (task.scenario) files.add(path.resolve(REPO_ROOT, task.scenario));
    if (task.pilot_manifest) files.add(path.resolve(REPO_ROOT, task.pilot_manifest));
    if (task.oracle) trees.add(path.resolve(REPO_ROOT, task.oracle));
    if (task.source_root) trees.add(path.resolve(REPO_ROOT, task.source_root));
    if (task.reference_local_root && existsSync(path.resolve(REPO_ROOT, task.reference_local_root))) {
      trees.add(path.resolve(REPO_ROOT, task.reference_local_root));
    }
  }
  return {
    files: [...files].sort().map((file) => ({ path: file, ...hashFile(file) })),
    trees: [...trees].sort().map((root) => ({ path: root, sha256: hashTree(root).tree_sha256 })),
  };
}

function assertPreparedInputs(sessionDir, session) {
  const manifest = loadJSON(path.join(sessionDir, "manifest.json"));
  if (!manifest.input_lock) die("session predates input locking; prepare a new session");
  const current = preparationInputs(session.experiments_file, session.contract_file,
    loadJSON(session.contract_file), loadJSON(session.experiments_file));
  if (JSON.stringify(current) !== JSON.stringify(manifest.input_lock)) die("prepared inputs changed; prepare a new session before validation or inference");
  return sha256(JSON.stringify(current));
}

async function prepare(args) {
  const experimentsFile = path.resolve(args.experiments ?? DEFAULT_EXPERIMENTS);
  const contractFile = path.resolve(args.contract ?? DEFAULT_CONTRACT);
  const experiments = loadJSON(experimentsFile);
  const contract = loadJSON(contractFile);
  if (experiments.schema_version !== 1 || contract.schema_version !== 1) die("unsupported experiment or fixture schema");
  if (!experiments.model?.id || !experiments.model?.path || !experiments.server?.backend) die("configuration must specify a model and server backend");
  validateContractShape(contract);
  validateExperimentShape(experiments, contract);
  const git = await gitInfo();
  const timestamp = new Date().toISOString().replace(/[-:.]/g, "").replace(/Z$/, "Z");
  const resultsRoot = path.resolve(args["results-root"] ?? DEFAULT_RESULTS_ROOT);
  const sessionDir = path.join(resultsRoot, `${timestamp}-${(git.commit ?? "unknown").slice(0, 12)}`);
  mkdirSync(sessionDir, { recursive: true, mode: 0o700 });
  const cli = await binaryInfo(args.cli ?? null);
  const model = await modelInfo(experiments.model);
  const hardware = await readHardware();
  const background = await backgroundBaseline();
  const serverBinary = await serverBinaryInfo(experiments);
  const server = await serverSnapshot(experiments);
  const fixtureHashes = {};
  for (const [id, task] of Object.entries(contract.tasks)) {
    if (task.kind === "scenario") {
      const scenario = scenarioData(task);
      fixtureHashes[id] = { base: { files: Object.entries(scenario.files ?? {}).map(([file, content]) => ({ path: file, ...hashFileFromString(content) })), tree_sha256: sha256(JSON.stringify(scenario.files ?? {})) }, oracle: hashTree(path.resolve(REPO_ROOT, task.oracle)) };
    } else if (existsSync(path.resolve(REPO_ROOT, task.source_root))) {
      fixtureHashes[id] = { base: hashTree(path.resolve(REPO_ROOT, task.source_root)), source_files: task.source_files };
    } else {
      fixtureHashes[id] = { base: null, explanation: "prepared pilot worktree is missing" };
    }
  }
  const manifest = {
    schema_version: 1,
    created_at: new Date().toISOString(),
    preparation_only: true,
    input_lock: preparationInputs(experimentsFile, contractFile, contract, experiments),
    candidate: experiments.candidate ?? { id: experiments.model.id, engine: serverBackend(experiments) },
    experiment_config: {
      path: path.relative(REPO_ROOT, experimentsFile),
      sha256: sha256(readFileSync(experimentsFile)),
    },
    git,
    hardware,
    background_process_baseline: background,
    binary: cli,
    model,
    server: { configured: experiments.server, binary_runtime: serverBinary, observed: server },
    chat_template: { sha256: server.template_sha256, explanation: server.template_explanation },
    sampling: {
      baseline: { model: modelRequestID(experiments), temperature: 0.7, top_p: 0.8, top_k: 20, min_p: null, presence_penalty: 1.5, repeat_penalty: 1, seed: 42 },
      explicit_min_p_candidate: { model: modelRequestID(experiments), temperature: 0.7, top_p: 0.8, top_k: 20, min_p: 0, presence_penalty: 1.5, repeat_penalty: 1, seed: null },
      explanation: "Per-attempt effective values are recorded in the Go CLI run_start trace; null min_p means the field is omitted from the request."
    },
    budgets: experiments.budgets,
    policy: experiments.policy,
    fixture_hashes: fixtureHashes,
    holdouts: contract.holdouts,
    pending_validations: [
      "Go CLI build hash",
      "fixture base/reference/invalid oracle checks",
      "capability gates",
      "effective server settings and slot-cache erase",
      "all benchmark measurements",
    ],
  };
  writeJSON(path.join(sessionDir, "manifest.json"), manifest);
  writeJSON(path.join(sessionDir, "session.json"), { schema_version: 1, session_dir: sessionDir, experiments_file: experimentsFile, contract_file: contractFile, plan_status: "prepared", created_at: new Date().toISOString() });
  writeJSON(path.join(sessionDir, "plan.json"), { schema_version: 1, attempts: await buildAttemptPlan(experiments, contract) });
  for (const name of ["runs.jsonl", "turns.jsonl", "memory.jsonl", "cache-events.jsonl"]) writeFileSync(path.join(sessionDir, name), "", { mode: 0o600 });
  mkdirSync(path.join(sessionDir, "active"), { recursive: true, mode: 0o700 });
  writeDecisions(path.join(sessionDir, "DECISIONS.md"), experiments, experimentsFile);
  console.log(JSON.stringify({ session_dir: sessionDir, git: git.commit, cli: cli.path, model: model.path, pending: manifest.pending_validations }, null, 2));
}

function hashFileFromString(content) {
  return { sha256: sha256(content), bytes: Buffer.byteLength(content) };
}

async function validate(args) {
  const sessionDir = path.resolve(args.session ?? "");
  if (!sessionDir || !existsSync(path.join(sessionDir, "session.json"))) die("--session must point to a prepared optimization session");
  const session = loadJSON(path.join(sessionDir, "session.json"));
  const inputLockHash = assertPreparedInputs(sessionDir, session);
  if (args["dry-run"]) {
    console.log(JSON.stringify({ command: "validate", dry_run: true, session_dir: sessionDir, input_lock_sha256: inputLockHash, inference_requested: false }, null, 2));
    return;
  }
  const contract = loadJSON(session.contract_file);
  const experiments = loadJSON(session.experiments_file);
  const conditionalConfiguration = experiments.experiments?.some((experiment) => experiment.requires === "evidence-selected");
  const defaultTaskIDs = ["permissions-cache", "job-queue", "immer-array-push-fix", "zod-int-json-schema", "notify-channel"];
  if (conditionalConfiguration) defaultTaskIDs.push(...(contract.holdouts?.tasks ?? []));
  const taskIDs = args.task ? String(args.task).split(",") : defaultTaskIDs;
  const results = {};
  try {
    for (const id of taskIDs) {
      if (interruption) die(interruption);
      if (!contract.tasks[id]) die(`unknown fixture ${id}`);
      console.error(`validating fixture ${id}`);
      results[id] = await validateTask(contract.tasks[id], sessionDir, { fetchReference: Boolean(args["fetch-reference"]) });
    }
  } catch (error) {
    // Persist partial validation progress and infrastructure failures so a
    // resumable run never loses the reason it stopped before inference.
    writeJSON(path.join(sessionDir, "fixture-validation.json"), {
      input_lock_sha256: inputLockHash,
      schema_version: 1,
      completed_at: new Date().toISOString(),
      results,
      all_pass: false,
      status: "error",
      error: error instanceof Error ? error.message : String(error),
    });
    const manifestPath = path.join(sessionDir, "manifest.json");
    const manifest = loadJSON(manifestPath);
    manifest.fixture_validation = { path: "fixture-validation.json", all_pass: false, completed_at: new Date().toISOString(), error: error instanceof Error ? error.message : String(error) };
    if (!manifest.pending_validations.includes("fixture base/reference/invalid oracle checks")) manifest.pending_validations.push("fixture base/reference/invalid oracle checks");
    writeJSON(manifestPath, manifest);
    throw error;
  }
  writeJSON(path.join(sessionDir, "fixture-validation.json"), { input_lock_sha256: inputLockHash, schema_version: 1, completed_at: new Date().toISOString(), results, all_pass: Object.values(results).every((result) => result.pass === true) });
  const manifestPath = path.join(sessionDir, "manifest.json");
  const manifest = loadJSON(manifestPath);
  manifest.fixture_validation = { path: "fixture-validation.json", all_pass: Object.values(results).every((result) => result.pass === true), completed_at: new Date().toISOString() };
  manifest.pending_validations = manifest.pending_validations.filter((item) => item !== "fixture base/reference/invalid oracle checks");
  if (!manifest.fixture_validation.all_pass) manifest.pending_validations.push("fixture base/reference/invalid oracle checks");
  writeJSON(manifestPath, manifest);
  console.log(JSON.stringify({ session_dir: sessionDir, all_pass: manifest.fixture_validation.all_pass, results }, null, 2));
  if (!manifest.fixture_validation.all_pass) process.exitCode = 1;
}

function successfulRuns(sessionDir) {
  return new Set(readJSONL(path.join(sessionDir, "runs.jsonl"))
    // An interrupted checkpoint is history, not a completed logical attempt. Keep
    // its artifacts so the next invocation can retry from a fresh worktree.
    .filter((row) => row.state === "finished" && row.interrupted !== true)
    .map((row) => row.attempt_id));
}

async function runWarmSmoke(args, sessionDir, experiments, cliPath, serverPid, memorySampler) {
  memorySampler.attemptId = "warm-smoke";
  memorySampler.executionId = "warm-smoke";
  const smokeDir = mkdtempSync(path.join(sessionDir, "warm-smoke-") );
  writeFileSync(path.join(smokeDir, "README.md"), "warm smoke fixture\n", { mode: 0o600 });
  const configPath = path.join(smokeDir, "config.json");
  writeJSON(configPath, {
    schema_version: 1,
    experiment_id: "warm-smoke",
    seed: 42,
    ...(experiments.cli.warmup_agent ?? { tool_schema_policy: "stable", close_out_reserve: false }),
  });
  const output = path.join(sessionDir, "warm-smoke-cli");
  const result = await runCLI({
    cliPath,
    root: smokeDir,
    prompt: "Read README.md and reply with the exact phrase warm smoke complete. Do not edit any file.",
    output,
    configPath,
    timeoutSeconds: experiments.cli.warmup_timeout_seconds ?? 120,
    maxTokens: experiments.cli.warmup_max_tokens ?? 3072,
    model: modelRequestID(experiments),
    endpoint: experiments.server.endpoint,
    serverPid,
    memorySampler,
  });
  const warmupRow = {
    state: "finished",
    attempt_id: "warm-smoke",
    scored: false,
    status: result.summary?.status ?? "unknown",
    cli_exit_code: result.process.code,
    signal: result.process.signal ?? null,
    timed_out: result.process.timedOut ?? false,
    cancel_requested_at: result.process.cancelRequestedAt ?? null,
    cancel_acknowledged_at: result.process.cliExitAt ?? null,
    pressure_abort: memorySampler.unsafe,
    error: result.process.error ?? result.summary?.error ?? null,
    tool_calls: result.summary?.tool_calls ?? null,
    wall_ms: result.process.wallMs,
    memory_peak: memorySampler.peakSummary("warm-smoke"),
    warmup: true,
    completed_at: new Date().toISOString(),
  };
  appendJSONL(path.join(sessionDir, "runs.jsonl"), warmupRow);
  if (memorySampler.unsafe) die(`warm smoke stopped by memory guard: ${memorySampler.unsafe}`);
  if (result.process.code !== 0 || result.summary?.status !== "completed" || (result.summary.tool_calls ?? 0) < 1) die("warm smoke did not produce a completed real tool call");
  return result;
}

async function runCapabilityGates(sessionDir, experiments, contract, cliPath, serverPid, memorySampler, serverProfile) {
  const gateIDs = ["gate-read-only", "gate-edit", "gate-notify-write"];
  // Capability gates must exercise the same model profile as scored attempts. The old runner
  // forced the focused prompt for two gates, which made a gate failure ambiguous when the actual
  // experiment used the baseline prompt. A config may still opt into focused explicitly.
  const gateAgent = experiments.cli.gate_agent ?? { tool_schema_policy: "stable", close_out_reserve: false };
  const bashOnly = gateAgent.bash_mode === "only";
  const gateAction = bashOnly ? "Bash" : "native tools";
  const gateFingerprint = sha256(JSON.stringify({
    model: modelRequestID(experiments),
    backend: serverBackend(experiments),
    profile: serverProfile,
    args: serverArgs(experiments, serverProfile),
    gate_agent: gateAgent,
  }));
  const priorGates = readJSONL(path.join(sessionDir, "runs.jsonl"))
    .filter((row) => row.gate === true && row.gate_pass === true && row.memory_valid === true && row.server_fingerprint === gateFingerprint)
    .reduce((latest, row) => latest.set(row.attempt_id, row), new Map());
  if (gateIDs.every((id) => priorGates.has(id))) {
    const gates = gateIDs.map((id) => priorGates.get(id));
    writeJSON(path.join(sessionDir, "capability-gates.json"), {
      completed_at: new Date().toISOString(),
      reused: true,
      server_profile: serverProfile,
      server_fingerprint: gateFingerprint,
      all_pass: true,
      gates,
    });
    return gates;
  }
  const gates = [];
  const gate = async (id, root, prompt, write, check, agentOverrides = {}) => {
    if (priorGates.has(id)) {
      gates.push({ ...priorGates.get(id), reused: true });
      return;
    }
    const gateDir = path.join(sessionDir, "gates", id);
    const output = path.join(sessionDir, "gate-cli", id);
    const configPath = path.join(gateDir, "experiment-config.json");
    mkdirSync(gateDir, { recursive: true, mode: 0o700 });
    writeJSON(configPath, {
      schema_version: 1,
      experiment_id: id,
      seed: 42,
      ...gateAgent,
      ...agentOverrides,
    });
    memorySampler.attemptId = id;
    memorySampler.executionId = id;
    const result = await runCLI({ cliPath, root, prompt, output, configPath, timeoutSeconds: experiments.cli.gate_timeout_seconds ?? 120, maxTokens: experiments.cli.gate_max_tokens ?? 3072, model: modelRequestID(experiments), endpoint: experiments.server.endpoint, serverPid, memorySampler, write });
    const checkResult = await check(result);
    const trace = result.trace ? parseTrace(result.trace) : [];
    const recoveries = trace.filter((event) => event.type === "recovery").length;
    if (recoveries > 0) { checkResult.pass = false; checkResult.reason = "native gate used recovered text calls"; }
    await memorySampler.sample();
    const samples = memorySampler.samples.filter((sample) => sample.attempt_id === id && sample.execution_id === id);
    // Ambient system pressure from the user's other apps is expected here, not
    // a contamination signal -- so "warning" pressure throughout does not
    // invalidate a gate by itself. `unsafe` already covers critical pressure,
    // unavailable pressure, runaway swapout, and (once serverPid is set) the
    // server-footprint ceiling.
    const memoryValid = !memorySampler.unsafe && samples.length > 0;
    if (!memoryValid) { checkResult.pass = false; checkResult.reason = `gate memory unsafe: ${memorySampler.unsafe ?? "no samples recorded"}`; }
    const row = { state: "finished", attempt_id: id, scored: false, gate: true, server_profile: serverProfile, server_fingerprint: gateFingerprint, status: result.summary?.status ?? "no_summary", cli_exit_code: result.process.code, cli_wall_ms: result.process.wallMs, tool_calls: result.summary?.tool_calls ?? null, recoveries, memory_valid: memoryValid, gate_pass: checkResult.pass, gate_check: checkResult, completed_at: new Date().toISOString() };
    appendJSONL(path.join(sessionDir, "runs.jsonl"), row);
    gates.push(row);
    if (!checkResult.pass) die(`capability gate ${id} failed: ${checkResult.reason}`);
    const cacheReset = await clearCacheOrStop(experiments);
    appendJSONL(path.join(sessionDir, "cache-events.jsonl"), { at: new Date().toISOString(), attempt_id: id, context: "capability_gate", result: cacheReset });
  };

  const readRoot = mkdtempSync(path.join(sessionDir, "gate-read-"));
  writeFileSync(path.join(readRoot, "README.md"), "capability-read-marker\n", { mode: 0o600 });
  await gate("gate-read-only", readRoot, `${bashOnly ? "Use Bash to read" : "Read"} README.md and reply with the exact marker capability-read-marker. Do not edit any file.`, false, async (result) => ({
    pass: result.process.code === 0 && result.summary?.status === "completed" && result.summary?.tool_calls >= 1 && result.summary?.edited_files?.length === 0 && result.summary.answer?.includes("capability-read-marker"),
    reason: "read-only gate requires one real tool call, the marker, and zero edits",
  }));

  const editRoot = mkdtempSync(path.join(sessionDir, "gate-edit-"));
  writeFileSync(path.join(editRoot, "sum.mjs"), "export function sum(a, b) { return a - b; }\n", { mode: 0o600 });
  await gate("gate-edit", editRoot, `Fix sum.mjs so sum(a, b) returns a + b. Use the ${gateAction} to apply the edit, then finish.`, true, async (result) => ({
    pass: result.summary?.tool_calls >= 1 && traceHasTool(result.trace ? parseTrace(result.trace) : [], bashOnly ? "bash" : "edit") && (bashOnly || result.summary?.edited_files?.includes("sum.mjs")) && readFileSync(path.join(editRoot, "sum.mjs"), "utf8").includes("a + b"),
    reason: "edit gate requires an applied native source edit; a missing final close-out response is recorded separately",
  }));

  const notifyTask = contract.tasks["notify-channel"];
  if (!notifyTask) die("notify-channel capability fixture is missing");
  const notifyRoot = mkdtempSync(path.join(sessionDir, "gate-notify-"));
  const notifyScenario = scenarioData(notifyTask);
  for (const [rel, content] of Object.entries(notifyScenario.files ?? {})) writeRelative(notifyRoot, rel, content);
  const writeGatePath = "write-capability.txt";
  const writeGateContent = "native write capability marker";
  const writeGatePrompt = [
    `Use ${bashOnly ? "Bash" : "the native write tool"} exactly once to create the new file ${writeGatePath}.`,
    `Its complete content must be exactly: ${JSON.stringify(writeGateContent)}`,
    "Do not add quotes, a backslash, a literal n character, or any extra text.",
    "Do not read, search, edit, or explain. After the write, finish.",
  ].join("\n");
  await gate("gate-notify-write", notifyRoot, writeGatePrompt, true, async (result) => ({
    pass: result.summary?.tool_calls >= 1 && traceHasTool(result.trace ? parseTrace(result.trace) : [], bashOnly ? "bash" : "write") && existsSync(path.join(notifyRoot, writeGatePath)) && readFileSync(path.join(notifyRoot, writeGatePath), "utf8") === writeGateContent,
    reason: "notify-channel gate requires one native write call that creates the exact marker file; a missing final close-out response is recorded separately",
  }));
  writeJSON(path.join(sessionDir, "capability-gates.json"), { completed_at: new Date().toISOString(), reused: false, server_profile: serverProfile, server_fingerprint: gateFingerprint, all_pass: gates.every((row) => row.gate_pass), gates });
  return gates;
}

async function runCLI({ cliPath, root, prompt, output, configPath, timeoutSeconds, maxTokens, model, endpoint, serverPid, memorySampler, write = true }) {
  const promptFile = path.join(output, "task.json");
  mkdirSync(output, { recursive: true, mode: 0o700 });
  writeJSON(promptFile, { id: "benchmark-task", report: prompt });
  const args = [cliPath, "--root", root, "--endpoint", endpoint, "--task-file", promptFile, "--output", output, "--timeout", `${timeoutSeconds}s`, "--max-tokens", String(maxTokens), "--model", model, "--experiment-config", configPath, "--server-pid", String(serverPid)];
  if (!write) args.push("--write=false");
  memorySampler?.setPhase("inference");
  const processResult = await spawnCapture(args, {
    cwd: REPO_ROOT,
    timeoutMs: timeoutSeconds * 1000 + 15_000,
    env: process.env,
    onTimeout: () => memorySampler && (memorySampler.phase = "cancellation"),
    abortCheck: () => memorySampler?.unsafe,
  });
  memorySampler?.setPhase("verification");
  let summary = null;
  try { summary = JSON.parse(processResult.stdout.trim().split("\n").at(-1)); } catch { /* CLI may have been interrupted before stdout */ }
  const trace = await findTrace(output, processResult.stderr);
  return { process: processResult, summary, trace };
}

async function runAttempt(attempt, sessionDir, experiments, contract, cliPath, serverPid, memorySampler, args, executionId = attempt.attempt_id) {
  const attemptStartedAt = Date.now();
  if (memorySampler) {
    memorySampler.attemptId = attempt.attempt_id;
    memorySampler.executionId = executionId;
  }
  const experiment = experiments.experiments.find((item) => item.id === attempt.experiment_id);
  const contractTask = contract.tasks[attempt.task];
  const budget = experiments.budgets[experiment.budget];
  // attempt_id is the logical plan row. executionId identifies one concrete try so
  // an interrupted try is never overwritten by a resumed retry.
  const runDir = path.join(sessionDir, "runs", executionId);
  mkdirSync(runDir, { recursive: true, mode: 0o700 });
  const worktree = path.join(runDir, "worktree");
  rmSync(worktree, { recursive: true, force: true });
  const setup = createBaseWorktree(contractTask, worktree);
  const sourceFiles = contractTask.source_files ?? (contractTask.kind === "scenario" ? Object.keys(setup.scenario.files ?? {}) : []);
  const sourceHints = contractTask.kind === "scenario"
    ? [...new Set((contractTask.reference_patches ?? []).map((patch) => patch.path))]
    : [...new Set(contractTask.source_files ?? setup.task.source_files ?? [])];
  const before = snapshot(worktree);
  const beforeHashes = new Map(before.map((entry) => [entry.path, entry.sha256]));
  const beforeText = new Map(sourceFiles.map((relative) => [relative, readTextIfPresent(worktree, relative)]));
  const config = {
    schema_version: 1,
    experiment_id: attempt.experiment_id,
    seed: attempt.seed,
    ...(experiment.agent ?? {}),
    max_turns: budget.max_turns,
  };
  const configPath = path.join(runDir, "experiment-config.json");
  writeJSON(configPath, config);
  const report = contractTask.kind === "scenario" ? setup.scenario.report : setup.task.report.replace(/\n?[Tt]est command:[\s\S]*$/i, "").trim();
  const prompt = repairPrompt(report, sourceHints, experiment.agent ?? {});
  writeJSON(path.join(runDir, "attempt.json"), { attempt, execution_id: executionId, experiment, budget, task: attempt.task, prompt, cli: cliPath, server_pid: serverPid, server_profile: experiment.server_profile, server_args: serverArgs(experiments, experiment.server_profile), worktree, model: experiments.model, started_at: new Date().toISOString() });
  const activePath = path.join(sessionDir, "active", `${attempt.attempt_id}.json`);
  writeJSON(activePath, { attempt, execution_id: executionId, started_at: new Date().toISOString(), run_dir: runDir });
  const serverLogPath = path.join(sessionDir, "server.log");
  const serverLogOffset = existsSync(serverLogPath) ? statSync(serverLogPath).size : 0;
  const cliResult = await runCLI({ cliPath, root: worktree, prompt, output: path.join(runDir, "cli-artifacts"), configPath, timeoutSeconds: budget.timeout_seconds, maxTokens: budget.max_tokens, model: modelRequestID(experiments), endpoint: experiments.server.endpoint, serverPid, memorySampler });
  writeFileSync(path.join(runDir, "stdout.json"), cliResult.process.stdout || "", { mode: 0o600 });
  writeFileSync(path.join(runDir, "stderr.log"), cliResult.process.stderr || "", { mode: 0o600 });
  const after = snapshot(worktree);
  const changed = changedFiles(before, after);
  const afterHashes = new Map(after.map((entry) => [entry.path, entry.sha256]));
  const changeCheck = sourceOnlyChange(changed, sourceFiles);
  const traceEvents = cliResult.trace ? parseTrace(cliResult.trace) : [];
  const turnRows = extractTurnRows(traceEvents, attempt.attempt_id).map((row) => ({ ...row, execution_id: executionId }));
  const serverMetrics = experiments.server.backend === "hebrus"
    ? parseHebrusServerMetrics(serverLogPath, serverLogOffset)
    : null;
  if (serverMetrics && existsSync(serverLogPath)) {
    const logBytes = readFileSync(serverLogPath);
    writeFileSync(path.join(runDir, "server-log-fragment.log"), logBytes.subarray(Math.min(serverLogOffset, logBytes.length)), { mode: 0o600 });
  }
  writeJSON(path.join(runDir, "source-patch.json"), {
    source_files: sourceFiles,
    changed_files: changed,
    change_check: changeCheck,
    files: changed.map((relative) => {
      const beforeValue = beforeText.get(relative) ?? null;
      const afterValue = readTextIfPresent(worktree, relative);
      return {
        path: relative,
        before: beforeValue,
        after: afterValue,
        before_sha256: beforeHashes.get(relative) ?? (beforeValue == null ? null : sha256(beforeValue)),
        after_sha256: afterHashes.get(relative) ?? (afterValue == null ? null : sha256(afterValue)),
      };
    }),
    trace_patches: sourcePatchFromTrace(traceEvents),
  });
  if (cliResult.trace) {
    cpSync(cliResult.trace, path.join(runDir, "trace.jsonl"));
    for (const row of turnRows) appendJSONL(path.join(sessionDir, "turns.jsonl"), row);
  }
  let grade = { pass: false, status: "pending", explanation: "verification not run" };
  const verificationStartedAt = Date.now();
  memorySampler?.setPhase("verification");
  if (memorySampler?.unsafe) {
    grade = { pass: false, status: "pressure_abort", explanation: memorySampler.unsafe };
  } else if (changed.length === 0) {
    // A no-edit run is a real model/protocol outcome, but there is no candidate
    // patch for the pristine oracle to grade. Keep it distinct from an oracle
    // failure and avoid spending verification time on an unchanged worktree.
    grade = { pass: false, status: "no_edit", explanation: "model produced no source edit; pristine oracle skipped" };
  } else if (contractTask.kind === "scenario") {
    const scratch = mkdtempSync(path.join(runDir, "grade-"));
    grade = await verifyDirectoryScenario(contractTask, worktree, scratch, () => memorySampler?.unsafe ?? null);
  } else {
    const verifyDir = path.join(runDir, "verification-worktree");
    const scratch = mkdtempSync(path.join(runDir, "verification-"));
    grade = await verifyDirectoryPilot(contractTask, worktree, verifyDir, scratch, true, budget.timeout_seconds, () => memorySampler?.unsafe ?? null);
    writeJSON(path.join(runDir, "oracle-report.json"), grade);
  }
  if (contractTask.kind === "scenario") writeJSON(path.join(runDir, "oracle-report.json"), grade);
  await memorySampler?.sample();
  const memory = memorySampler?.peakSummary(attempt.attempt_id, executionId) ?? null;
  const attemptSamples = memorySampler?.samples.filter((sample) => sample.attempt_id === attempt.attempt_id && sample.execution_id === executionId) ?? [];
  // See the gate check above: ambient pressure need not be "normal" throughout,
  // only not-unsafe (which already covers critical pressure and the
  // server-footprint ceiling).
  const memoryValid = attemptSamples.length > 0 && !memorySampler?.unsafe;
  const verificationWallMs = Date.now() - verificationStartedAt;
  const summary = cliResult.summary;
  const recoveries = Number.isFinite(summary?.recoveries)
    ? summary.recoveries
    : traceEvents.filter((event) => event.type === "recovery").length;
  const toolCalls = Number.isFinite(summary?.tool_calls) ? summary.tool_calls : null;
  const row = {
    state: "finished",
    attempt_id: attempt.attempt_id,
    execution_id: executionId,
    experiment_id: attempt.experiment_id,
    task: attempt.task,
    seed: attempt.seed,
    scored: attempt.scored,
    status: summary?.status ?? (cliResult.process.timedOut ? "timeout" : "no_summary"),
    cli_exit_code: cliResult.process.code,
    cli_signal: cliResult.process.signal ?? null,
    cli_wall_ms: cliResult.process.wallMs,
    verification_wall_ms: verificationWallMs,
    total_wall_ms: Date.now() - attemptStartedAt,
    timed_out: cliResult.process.timedOut,
    cancel_requested_at: cliResult.process.cancelRequestedAt ?? null,
    cancel_acknowledged_at: cliResult.process.cliExitAt ?? null,
    source_changed: changed.length > 0,
    zero_edit: changed.length === 0,
    change_check: changeCheck,
    memory_valid: memoryValid,
    grade_pass: grade.pass === true && changeCheck.ok === true && memoryValid,
    grade,
    tool_calls: toolCalls,
    native_tool_calls: toolCalls == null ? null : Math.max(0, toolCalls - recoveries),
    recoveries,
    turns: summary?.turns ?? null,
    first_edit_request_index: turnRows.find((turn) => turn.edit_success === true)?.request_index ?? null,
    edited_files: summary?.edited_files ?? [],
    context_limit: summary?.status === "context_limit",
    turn_limit: summary?.status === "turn_limit",
    pressure_abort: memorySampler?.unsafe ?? null,
    memory,
    server_metrics: serverMetrics,
    server_log: existsSync(path.join(sessionDir, "server.log")) ? "server.log" : null,
    completed_at: new Date().toISOString(),
  };
  writeJSON(path.join(runDir, "run-result.json"), row);
  // Keep the checkpoint until the result has been durably appended to the session log.
  return row;
}

async function run(args) {
  const sessionDir = path.resolve(args.session ?? "");
  if (!sessionDir || !existsSync(path.join(sessionDir, "session.json"))) die("--session must point to a prepared optimization session");
  const session = loadJSON(path.join(sessionDir, "session.json"));
  const inputLockHash = assertPreparedInputs(sessionDir, session);
  if (args["dry-run"]) {
    console.log(JSON.stringify({ command: "run", dry_run: true, session_dir: sessionDir, input_lock_sha256: inputLockHash, external_server: Boolean(args["external-server"]), inference_requested: false }, null, 2));
    return;
  }
  const experiments = loadJSON(session.experiments_file);
  const contract = loadJSON(session.contract_file);
  const plan = loadJSON(path.join(sessionDir, "plan.json")).attempts;
  const requested = args.experiment ? new Set(String(args.experiment).split(",")) : null;
  const planRows = plan.filter((attempt) => !requested || requested.has(attempt.experiment_id));
  const fixtureValidation = existsSync(path.join(sessionDir, "fixture-validation.json")) ? loadJSON(path.join(sessionDir, "fixture-validation.json")) : null;
  if (fixtureValidation?.input_lock_sha256 !== inputLockHash) die("fixture validation does not match prepared inputs");
  const selectedDefinitions = new Map(experiments.experiments.map((experiment) => [experiment.id, experiment]));
  const evidenceSelected = planRows.some((attempt) => selectedDefinitions.get(attempt.experiment_id)?.requires === "evidence-selected");
  const requiredFixtureIDs = new Set([...planRows.map((row) => row.task), "notify-channel"]);
  if (evidenceSelected) {
    const holdouts = contract.holdouts;
    const requiredHoldouts = Number(holdouts?.required_before_tuning ?? 0);
    if (!Array.isArray(holdouts?.tasks) || holdouts.tasks.length < requiredHoldouts || requiredHoldouts < 1) {
      die("conditional experiments require a fixture contract with the configured frozen holdouts");
    }
    for (const id of holdouts.tasks) requiredFixtureIDs.add(id);
  }
  for (const id of requiredFixtureIDs) {
    if (fixtureValidation?.results?.[id]?.pass !== true) die(`fixture validation pending for ${id}`);
  }
  const cliPath = path.resolve(args.cli ?? "");
  if (!cliPath || !isRegularFile(cliPath)) die("--cli must point to the one built go-agent binary");
  const preparedManifest = loadJSON(path.join(sessionDir, "manifest.json"));
  const currentCLI = await binaryInfo(cliPath);
  if (!preparedManifest.binary?.sha256 || preparedManifest.binary.sha256 !== currentCLI.sha256) {
    die("the CLI path does not match the binary hash captured by prepare; build once and prepare a new session");
  }
  const currentModel = await modelInfo(experiments.model);
  const modelMissing = currentModel.sha256 == null;
  const modelSizeMismatch = currentModel.size_matches === false;
  const modelExpectedHashMismatch = currentModel.hash_matches === false;
  const modelHashMismatch = preparedManifest.model?.sha256 != null && preparedManifest.model.sha256 !== currentModel.sha256;
  if (modelMissing || modelSizeMismatch || modelExpectedHashMismatch || modelHashMismatch) {
    die(`the current ${experiments.model.format ?? "model"} does not match the prepared manifest; no model download is attempted`);
  }
  preparedManifest.pending_validations = preparedManifest.pending_validations.filter((item) => item !== "Go CLI build hash");
  writeJSON(path.join(sessionDir, "manifest.json"), preparedManifest);
  if (experiments.policy.one_server_one_attempt !== true) die("one-server/one-attempt policy is required");
  const existing = successfulRuns(sessionDir);
  for (const file of readdirSync(path.join(sessionDir, "active"), { withFileTypes: true }).filter((entry) => entry.isFile())) {
    const active = loadJSON(path.join(sessionDir, "active", file.name));
    if (!existing.has(active.attempt.attempt_id)) {
      const executionId = active.execution_id ?? active.attempt.execution_id ?? active.attempt.attempt_id;
      const resultDir = active.run_dir ?? path.join(sessionDir, "runs", executionId);
      const resultPath = path.join(resultDir, "run-result.json");
      const saved = existsSync(resultPath) ? loadJSON(resultPath) : null;
      if (saved?.state === "finished" && saved.attempt_id === active.attempt.attempt_id) {
        appendJSONL(path.join(sessionDir, "runs.jsonl"), { ...saved, execution_id: saved.execution_id ?? executionId, recovered_checkpoint: true });
        existing.add(active.attempt.attempt_id);
      } else {
        // Preserve the interrupted row for auditability, but leave the logical
        // attempt pending so the normal loop retries it below from a new worktree.
        appendJSONL(path.join(sessionDir, "runs.jsonl"), { state: "finished", attempt_id: active.attempt.attempt_id, execution_id: executionId, experiment_id: active.attempt.experiment_id, task: active.attempt.task, seed: active.attempt.seed, status: "interrupted", scored: active.attempt.scored, interrupted: true, completed_at: new Date().toISOString() });
      }
    }
    rmSync(path.join(sessionDir, "active", file.name), { force: true });
  }
  const selected = planRows.filter((attempt) => !existing.has(attempt.attempt_id));
  if (!selected.length) {
    console.log("No pending attempts in the selected experiment set.");
    return;
  }
  const conditionalSelected = selected.some((attempt) => selectedDefinitions.get(attempt.experiment_id)?.requires === "evidence-selected");
  if (conditionalSelected && !evidenceSelected) die("conditional experiment selection changed after fixture validation; prepare a new session");
  const finishedRows = readJSONL(path.join(sessionDir, "runs.jsonl")).filter((row) => row.interrupted !== true);
  for (const id of new Set(selected.map((attempt) => attempt.experiment_id))) {
    const arm = experiments.experiments.find((item) => item.id === id);
    if (!arm) die(`experiment ${id} is absent from the experiment configuration`);
    if (arm.requires === "evidence-selected" && !args["evidence-approved"]) die(`${id} requires explicit evidence selection before startup`);
    for (const dependency of arm.depends_on ?? []) {
      if (dependency.condition !== "verified_repair_no_pressure") die(`unsupported dependency condition for ${id}`);
      const prior = finishedRows.filter((row) => row.state === "finished" && row.experiment_id === dependency.experiment_id);
      if (!prior.some((row) => row.grade_pass === true && row.memory_valid === true && !row.pressure_abort)) die(`${id} requires a verified normal-pressure repair from ${dependency.experiment_id}`);
      if (dependency.complete === true) {
        const required = plan.filter((row) => row.experiment_id === dependency.experiment_id);
        if (!required.length || !required.every((attempt) => prior.some((row) => row.attempt_id === attempt.attempt_id && row.memory_valid === true))) die(`${id} requires a complete memory-valid screen`);
      }
    }
  }
  const firstExperiment = experiments.experiments.find((item) => item.id === selected[0].experiment_id);
  const serverBinary = await serverBinaryInfo(experiments);
  if (!serverBinary.matches_expected) die(`${serverBackend(experiments)} server runtime does not match the configured version: ${JSON.stringify(serverBinary)}`);
  if (args["start-server"] && args["external-server"]) die("choose either --start-server or --external-server, not both");
  let externalPID = null;
  if (args["external-server"]) {
    const rawPID = String(args["server-pid"] ?? "");
    externalPID = Number(rawPID);
    if (!/^\d+$/.test(rawPID) || !Number.isSafeInteger(externalPID) || externalPID <= 0) {
      die("--external-server requires a positive integer --server-pid owned by the documented external server");
    }
  }
  const pid = args["start-server"] || args["external-server"] ? externalPID : await discoverServerPid(experiments);
  let managedServer = null;
  const maxServerFootprintBytes = experiments.policy?.max_server_footprint_bytes ?? DEFAULT_MAX_SERVER_FOOTPRINT_BYTES;
  const memory = new MemorySampler(path.join(sessionDir, "memory.jsonl"), pid, maxServerFootprintBytes);
  await memory.start("pre_server");
  let runCount = 0;
  try {
    // Ambient pressure from the user's other running apps is this host's
    // normal condition, not a contamination signal, so a pre-server baseline
    // of "warning" is expected and does not block startup by itself. Only a
    // hard-unsafe reading (critical pressure, unavailable pressure, or --
    // once the server exists -- a footprint over the configured ceiling)
    // stops the run before it starts. The pressure label is still recorded
    // on every sample for later inspection.
    if (memory.unsafe) die(`idle memory baseline is unsafe: ${memory.unsafe}`);
    if (args["start-server"]) {
      // Managed startup is explicit and never downloads a model. It is intentionally absent from
      // preparation commands so the operator can free memory before the batch begins.
      managedServer = await startManagedServer(experiments, firstExperiment.server_profile, sessionDir, memory);
    }
    const serverPid = managedServer?.pid ?? pid;
    if (!serverPid) die(`no matching ${serverBackend(experiments)} server PID found; pass --start-server after freeing memory or start the documented server externally`);
    memory.serverPid = serverPid;
    memory.setPhase("startup");
    await waitForHealth(experiments);
    const runtimeServer = await serverSnapshot(experiments);
    await memory.sample();
    const runtimeManifestPath = path.join(sessionDir, "manifest.json");
    const runtimeManifest = loadJSON(runtimeManifestPath);
    runtimeManifest.server.runtime = {
      profile: firstExperiment.server_profile,
      binary_runtime: serverBinary,
      args: serverArgs(experiments, firstExperiment.server_profile),
      process_check: managedServer?.process_check ?? (args["external-server"]
        ? { status: "operator-provided", source: "external-server", pid: serverPid }
        : { status: "not_checked", source: "external-server" }),
      observed: runtimeServer,
      recorded_at: new Date().toISOString(),
    };
    writeJSON(runtimeManifestPath, runtimeManifest);
    if (memory.unsafe) die(`stopping batch after startup memory guard: ${memory.unsafe}`);
    await runWarmSmoke(args, sessionDir, experiments, cliPath, serverPid, memory);
    await runCapabilityGates(sessionDir, experiments, contract, cliPath, serverPid, memory, firstExperiment.server_profile);
    if (memory.unsafe) die(`stopping batch after capability-gate memory guard: ${memory.unsafe}`);
    const gateManifest = loadJSON(runtimeManifestPath);
    gateManifest.pending_validations = gateManifest.pending_validations.filter((item) => item !== "capability gates");
    writeJSON(runtimeManifestPath, gateManifest);
    const cacheReset = await clearCacheOrStop(experiments);
    appendJSONL(path.join(sessionDir, "cache-events.jsonl"), { at: new Date().toISOString(), attempt_id: "before_first_attempt", context: "batch_start", result: cacheReset });
    const cacheManifest = loadJSON(runtimeManifestPath);
    cacheManifest.server.slot_cache_runtime = cacheReset;
    if (cacheReset.status === "cleared" || cacheReset.status === "not_applicable") {
      cacheManifest.pending_validations = cacheManifest.pending_validations.filter((item) => item !== "effective server settings and slot-cache erase");
    }
    writeJSON(runtimeManifestPath, cacheManifest);
    if (managedServer && managedServer.pid !== serverPid) die("managed server changed during cache reset; stop and resume explicitly");
    for (const attempt of selected) {
      if (interruption) die(interruption);
      if (args["max-runs"] && runCount >= Number(args["max-runs"])) break;
      const experiment = experiments.experiments.find((item) => item.id === attempt.experiment_id);
      if (experiment.requires === "evidence-selected" && !args["evidence-approved"]) die(`${experiment.id} is conditional; pass --evidence-approved only after DECISIONS.md records the selected arm`);
      const currentProfile = experiment.server_profile;
      if (currentProfile !== firstExperiment.server_profile) die(`server profile changes at ${attempt.attempt_id}; stop this batch and restart the server with profile ${currentProfile}`);
      memory.attemptId = attempt.attempt_id;
      memory.setPhase("pre_run");
      const cacheReset = await clearCacheOrStop(experiments);
      appendJSONL(path.join(sessionDir, "cache-events.jsonl"), { at: new Date().toISOString(), attempt_id: attempt.attempt_id, context: "before_attempt", result: cacheReset });
      const priorInterrupted = readJSONL(path.join(sessionDir, "runs.jsonl"))
        .filter((row) => row.attempt_id === attempt.attempt_id && row.interrupted === true).length;
      const executionId = priorInterrupted > 0
        ? `${attempt.attempt_id}__retry-${priorInterrupted + 1}`
        : attempt.attempt_id;
      const row = await runAttempt(attempt, sessionDir, experiments, contract, cliPath, serverPid, memory, args, executionId);
      runCount += 1;
      const idle = await waitForServerIdle(experiments);
      row.server_idle = idle;
      writeJSON(path.join(sessionDir, "runs", row.execution_id ?? attempt.attempt_id, "run-result.json"), row);
      appendJSONL(path.join(sessionDir, "runs.jsonl"), row);
      rmSync(path.join(sessionDir, "active", `${attempt.attempt_id}.json`), { force: true });
      if (idle.status !== "idle" && idle.status !== "not_applicable") die("server did not acknowledge idle before the next attempt; resume only after a clean server state");
      if (memory.unsafe) die(`stopping batch after memory guard: ${memory.unsafe}`);
    }
    console.log(JSON.stringify({ session_dir: sessionDir, completed_attempts: runCount, remaining: selected.length - runCount, pending: true }, null, 2));
  } catch (error) {
    appendJSONL(path.join(sessionDir, "runs.jsonl"), {
      state: "finished",
      attempt_id: "batch-abort",
      scored: false,
      status: "aborted",
      phase: memory.phase,
      error: error instanceof Error ? error.message : String(error),
      pressure_abort: memory.unsafe,
      memory: memory.peakSummary(),
      completed_at: new Date().toISOString(),
    });
    throw error;
  } finally {
    try { await memory.stop(); } finally { if (managedServer) await managedServer.stop(); }
  }
}

async function summarize(args) {
  const sessionDir = path.resolve(args.session ?? "");
  const rows = readJSONL(path.join(sessionDir, "runs.jsonl"));
  const byExperiment = {};
  // Keep interrupted attempts in the reported denominator. They are excluded only
  // from the completed-attempt set so a resume can retry them.
  for (const row of rows.filter((item) => item.state === "finished" && item.scored !== false)) {
    const bucket = byExperiment[row.experiment_id] ??= { attempts: 0, completed: 0, verified: 0, first_edit: 0, timeouts: 0, zero_edit: 0, total_wall_ms: [], cli_wall_ms: [] };
    bucket.attempts += 1;
    if (row.status === "completed") bucket.completed += 1;
    if (row.grade_pass) bucket.verified += 1;
    if (row.first_edit_request_index != null) bucket.first_edit += 1;
    if (row.timed_out) bucket.timeouts += 1;
    if (row.zero_edit) bucket.zero_edit += 1;
    if (Number.isFinite(row.total_wall_ms)) bucket.total_wall_ms.push(row.total_wall_ms);
    if (Number.isFinite(row.cli_wall_ms)) bucket.cli_wall_ms.push(row.cli_wall_ms);
  }
  for (const bucket of Object.values(byExperiment)) {
    bucket.median_total_wall_ms = bucket.total_wall_ms.sort((a, b) => a - b)[Math.floor(bucket.total_wall_ms.length / 2)] ?? null;
    bucket.median_cli_wall_ms = bucket.cli_wall_ms.sort((a, b) => a - b)[Math.floor(bucket.cli_wall_ms.length / 2)] ?? null;
  }
  writeJSON(path.join(sessionDir, "summary.json"), { generated_at: new Date().toISOString(), by_experiment: byExperiment, note: "This is a compact decision aid; inspect at most two representative failed traces per hypothesis. Unrun validations remain pending." });
  console.log(JSON.stringify({ session_dir: sessionDir, by_experiment: byExperiment }, null, 2));
}

function compactRunRow(row) {
  if (!row) return null;
  return {
    state: row.state ?? null,
    attempt_id: row.attempt_id ?? null,
    experiment_id: row.experiment_id ?? null,
    task: row.task ?? null,
    status: row.status ?? null,
    grade_pass: row.grade_pass ?? null,
    memory_valid: row.memory_valid ?? null,
    pressure_abort: row.pressure_abort ?? null,
    cli_wall_ms: row.cli_wall_ms ?? row.wall_ms ?? null,
    total_wall_ms: row.total_wall_ms ?? null,
  };
}

async function status(args) {
  const sessionDir = path.resolve(args.session ?? "");
  if (!sessionDir || !existsSync(path.join(sessionDir, "session.json"))) die("--session must point to a prepared optimization session");
  const session = loadJSON(path.join(sessionDir, "session.json"));
  const manifest = existsSync(path.join(sessionDir, "manifest.json")) ? loadJSON(path.join(sessionDir, "manifest.json")) : {};
  const runs = readJSONL(path.join(sessionDir, "runs.jsonl"));
  const memory = readJSONL(path.join(sessionDir, "memory.jsonl"));
  const active = existsSync(path.join(sessionDir, "active"))
    ? readdirSync(path.join(sessionDir, "active"), { withFileTypes: true }).filter((entry) => entry.isFile()).map((entry) => entry.name).sort()
    : [];
  const counts = {};
  for (const row of runs) {
    const key = row.status ?? row.state ?? "unknown";
    counts[key] = (counts[key] ?? 0) + 1;
  }
  const lastMemory = memory.at(-1) ?? null;
  console.log(JSON.stringify({
    session_dir: sessionDir,
    plan_status: session.plan_status ?? null,
    created_at: session.created_at ?? null,
    active,
    run_rows: runs.length,
    run_status_counts: counts,
    last_run: compactRunRow(runs.at(-1)),
    memory_samples: memory.length,
    last_memory: lastMemory ? {
      time: lastMemory.time ?? null,
      phase: lastMemory.phase ?? null,
      pressure: lastMemory.pressure ?? null,
      free_bytes: lastMemory.free_bytes ?? null,
      server_rss_bytes: lastMemory.server_rss_bytes ?? null,
      swapout_delta: lastMemory.swapout_delta ?? null,
      missing_measurements: lastMemory.missing_measurements ?? [],
    } : null,
    pending_validations: manifest.pending_validations ?? [],
    runtime_server: manifest.server?.runtime ? {
      profile: manifest.server.runtime.profile ?? null,
      endpoint: manifest.server.runtime.observed?.endpoint ?? manifest.server.configured?.endpoint ?? null,
      health_status: manifest.server.runtime.observed?.health?.status ?? null,
      process_check: manifest.server.runtime.process_check ?? null,
    } : null,
  }, null, 2));
}

async function exclusiveExecution(action) {
  mkdirSync(DEFAULT_RESULTS_ROOT, { recursive: true, mode: 0o700 });
  const lockPath = path.join(DEFAULT_RESULTS_ROOT, "execution.lock");
  let fd;
  try { fd = openSync(lockPath, "wx", 0o600); }
  catch { die(`execution lock exists at ${lockPath}; inspect its owner before removing a stale lock`); }
  try {
    writeFileSync(fd, JSON.stringify({ pid: process.pid, started_at: new Date().toISOString() }));
    await action();
  } finally { closeSync(fd); rmSync(lockPath, { force: true }); }
}

async function main() {
  process.on("SIGINT", () => interrupt("SIGINT"));
  process.on("SIGTERM", () => interrupt("SIGTERM"));
  const args = parseArgs(process.argv.slice(2));
  const subcommand = args._[0] ?? "help";
  if (subcommand === "prepare") await prepare(args);
  else if (subcommand === "validate") await exclusiveExecution(() => validate(args));
  else if (subcommand === "run") await exclusiveExecution(() => run(args));
  else if (subcommand === "summarize") await summarize(args);
  else if (subcommand === "status") await status(args);
  else {
    console.log(`Usage:\n  node benchmarks/optimization/runner.mjs prepare [--cli /tmp/anvil-agent]\n  node benchmarks/optimization/runner.mjs validate --session DIR [--fetch-reference] [--dry-run]\n  node benchmarks/optimization/runner.mjs run --session DIR --cli /tmp/anvil-agent --experiment QWEN3-CODER-30B-A3B-LOCAL-QUICK-REAL [--start-server | --external-server --server-pid PID] [--dry-run]\n  node benchmarks/optimization/runner.mjs summarize --session DIR\n  node benchmarks/optimization/runner.mjs status --session DIR`);
  }
}

main().catch((error) => {
  console.error(`optimization runner: ${error.stack ?? error.message}`);
  process.exitCode = 2;
});
