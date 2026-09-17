// Node port of llm-memory-wall-research's scripts/grade_lib.py, so this kit can grade its own benchmark suite
// with zero dependencies beyond the Node that `pi` already requires -- no Python, no separate install.
//
// Faithfulness note: every one of the 173 regex patterns across all 7 scenarios was checked to compile as a
// JS RegExp before this port was written (`new RegExp(p)` on each, zero failures) -- Python's `re` and JS's
// RegExp differ in a few corners (named groups, some lookbehind edge cases), and this suite happens to avoid
// all of them, so a straight regex-semantics port is safe here. If a future scenario adds a pattern that
// doesn't compile in JS, that will surface immediately as a thrown SyntaxError, not a silent wrong grade.
//
// Two grading paths, matching the original:
//   1. Pattern rules (`all`/`any`/`forbid`, or legacy single `pass`) scoped to a named file or file list.
//   2. Executable-oracle bugs (`"oracle": true`) -- graded by actually running the code, see runOracle().

import { readFileSync, writeFileSync, mkdirSync, rmSync, existsSync, cpSync, readdirSync, lstatSync } from "node:fs";
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import path from "node:path";

// Python's original used `\Z` (absolute end-of-string) for the "unterminated fence" fallback. JS has no
// direct equivalent under the `m` flag -- a bare `$` there matches at the end of EVERY line, not just the
// true end of input, which made the lazy body capture stop after just the first line (confirmed by testing:
// it silently truncated every multi-line file to one line). `(?![\s\S])` is the correct "true end of input"
// assertion: a negative lookahead for "any character at all", which only succeeds when there is none left.
const FENCE_RE = /^\s*```[a-zA-Z0-9_+-]*\s*\n(?<body>[\s\S]*?)(?:^\s*```\s*$|(?![\s\S]))/m;

// A relative import specifier with no recognised extension, e.g. `from "./channels/email"` -- normal for
// bundler-style TypeScript, but Node's native ESM loader needs an explicit extension. Applied only at
// grading/oracle-run time, never to what the model is shown.
const BARE_RELATIVE_IMPORT = /(from\s+|import\s*\(\s*)(['"])(\.\.?\/[^'"]+?)(?<!\.ts)(?<!\.tsx)(?<!\.js)(?<!\.mjs)(?<!\.json)\2/g;

export function fixTsImports(source) {
  return source.replace(BARE_RELATIVE_IMPORT, (_m, kw, q, spec) => `${kw}${q}${spec}.ts${q}`);
}

/** Split harness output into {path: body}. Later emissions of a path win (a follow-up turn supersedes). */
export function splitFiles(out) {
  const files = {};
  const headerRe = /^\s*={2,}\s*([^=\n]+?)\s*={2,}\s*$/gm;
  const marks = [...out.matchAll(headerRe)];
  for (let i = 0; i < marks.length; i++) {
    const m = marks[i];
    const end = i + 1 < marks.length ? marks[i + 1].index : out.length;
    const chunk = out.slice(m.index + m[0].length, end);
    const fence = chunk.match(FENCE_RE);
    const body = fence ? fence.groups.body : chunk;
    let p = m[1].trim().replace(/^`+|`+$/g, "").replace(/^\.?\//, "");
    if (p) files[p] = (files[p] || "") + "\n" + body;
  }
  return files;
}

/** Remove // and /* *\/ comments (and JSX {/* *\/} via the same block-comment path), keeping string literals
 * intact -- comments are not code and must never satisfy a rule. */
export function stripComments(code) {
  const out = [];
  let i = 0;
  const n = code.length;
  while (i < n) {
    const c = code[i];
    if (c === '"' || c === "'" || c === "`") {
      const quote = c;
      out.push(c);
      i++;
      while (i < n) {
        if (code[i] === "\\") { out.push(code.slice(i, i + 2)); i += 2; continue; }
        out.push(code[i]);
        if (code[i] === quote) { i++; break; }
        i++;
      }
      continue;
    }
    if (code.startsWith("//", i)) {
      const j = code.indexOf("\n", i);
      i = j === -1 ? n : j;
      continue;
    }
    if (code.startsWith("/*", i)) {
      const j = code.indexOf("*/", i + 2);
      i = j === -1 ? n : j + 2;
      out.push(" ");
      continue;
    }
    out.push(c);
    i++;
  }
  return out.join("");
}

export function norm(text) {
  return text.replace(/\s+/g, " ");
}

/** Text a rule is evaluated against: the named file(s) when `want` is given, else the whole output. */
function scopeText(out, files, want) {
  let body;
  if (want == null) {
    body = out;
  } else {
    const wants = Array.isArray(want) ? want : [want];
    body = Object.entries(files)
      .filter(([k]) => wants.some((w) => k.includes(w)))
      .map(([, v]) => v)
      .join("\n");
  }
  return norm(stripComments(body));
}

// All 173 patterns in this suite were authored for Python's `re`, then checked to compile as a JS RegExp --
// but "compiles" isn't "means the same thing". `\A` (Python's absolute start-of-string anchor) is not a
// recognised JS escape, and outside Unicode mode JS silently treats an unrecognised letter escape as that
// literal character instead of raising an error -- so `/\A/` in JS matches a literal "A", not "start of
// string". Confirmed directly: `/\A/.test("xAy")` is true, `/\A/.test("yx")` is false. This is exactly the
// kind of thing that only shows up as a wrong grade, never a crash -- caught here by running the SAME
// both-directions validation this repo already uses (unchanged input must reject, a reference fix must
// accept), which is why that check belongs in this port too, not just in the original Python tooling.
// Patterns here are always matched against `norm()`-processed text, which has no embedded newlines (all
// whitespace, including \n, is collapsed to single spaces) -- so `^`/`$` without the `m` flag correctly mean
// "start/end of the whole string", exactly Python's `\A`/`\Z` semantics, with no behavior change needed
// beyond the substitution itself.
function pyPattern(p) {
  return p.replace(/\\A/g, "^").replace(/\\[Zz]/g, "$");
}

function scopeLabel(file) {
  return Array.isArray(file) ? file.join(", ") : file;
}

/** Same pass/fail short-circuit order and conditions as before -- this must never change what passes or
 * fails, only add a human-readable `reason` alongside it. `fileMissing` is folded into whichever branch
 * already decided false, since an empty scope is usually the actual cause (the model never emitted that
 * file, or emitted it under a differently-shaped path) rather than the literal rule that fires against "". */
export function evalBug(bug, out, files) {
  const text = scopeText(out, files, bug.file ?? null);
  const fileMissing = bug.file != null && text.trim() === "";
  const forbidHit = (bug.forbid || []).find((p) => new RegExp(pyPattern(p)).test(text));
  if (forbidHit) {
    return {
      pass: false,
      reason: fileMissing
        ? `file not found in output: ${scopeLabel(bug.file)}`
        : `matched forbidden pattern: ${forbidHit}`,
    };
  }
  if (bug.all) {
    const missing = bug.all.find((p) => !new RegExp(pyPattern(p)).test(text));
    if (missing) {
      return {
        pass: false,
        reason: fileMissing ? `file not found in output: ${scopeLabel(bug.file)}` : `missing required pattern: ${missing}`,
      };
    }
  }
  if (bug.any && !bug.any.some((p) => new RegExp(pyPattern(p)).test(text))) {
    return {
      pass: false,
      reason: fileMissing
        ? `file not found in output: ${scopeLabel(bug.file)}`
        : `none of ${bug.any.length} expected patterns found`,
    };
  }
  if (bug.pass && !new RegExp(pyPattern(bug.pass)).test(text)) {
    return {
      pass: false,
      reason: fileMissing
        ? `file not found in output: ${scopeLabel(bug.file)}`
        : `expected pattern not found: ${bug.pass}`,
    };
  }
  if (!(bug.all || bug.any || bug.pass)) {
    return { pass: false, reason: "bug has no all/any/pass rule defined" };
  }
  return { pass: true, reason: null };
}

/** Overlay a model's emitted files onto the scenario's base files, applying the import fixup to every .ts/.tsx
 * file (base or emitted) so the merged tree is actually runnable under Node's native ESM loader. */
export function materialize(scenario, emitted, workDir) {
  const merged = { ...scenario.files, ...emitted };
  for (const [rel, rawBody] of Object.entries(merged)) {
    const body = /\.tsx?$/.test(rel) ? fixTsImports(rawBody) : rawBody;
    const full = path.join(workDir, rel);
    mkdirSync(path.dirname(full), { recursive: true });
    writeFileSync(full, body);
  }
}

/** Run the scenario's oracle (benchmarks/oracle/<id>/test/) against a materialized directory. Returns
 * {ok, output}. A non-zero exit with no "FAIL <key>:" line at all is the crash case -- callers must treat that
 * as every oracle-covered bug failing, never as a clean pass (a real bug this exact port fixed once already:
 * a crash with no structured verdict must never look identical to zero failures). */
function runOracle(scenarioId, oracleRoot, workDir) {
  const testSrc = path.join(oracleRoot, scenarioId, "test", "run.mjs");
  if (!existsSync(testSrc)) throw new Error(`no oracle defined for ${scenarioId} (expected ${testSrc})`);
  const testDst = path.join(workDir, "test", "run.mjs");
  mkdirSync(path.dirname(testDst), { recursive: true });
  writeFileSync(testDst, readFileSync(testSrc));
  try {
    const out = execFileSync("node", ["--experimental-strip-types", "test/run.mjs"], {
      cwd: workDir, encoding: "utf8", timeout: 60_000, stdio: ["ignore", "pipe", "pipe"],
    });
    return { ok: true, output: out, exitCode: 0 };
  } catch (e) {
    const output = `${e.stdout || ""}${e.stderr || ""}`;
    return { ok: false, output, exitCode: typeof e.status === "number" ? e.status : null };
  }
}

function oracleVerdict(scenario, oracleRoot, workDir, scratchDir, successMarker = null) {
  const oracleKeys = new Set(scenario.bugs.filter((b) => b.oracle).map((b) => b.key));
  const { ok, output, exitCode } = runOracle(scenario.id, oracleRoot, workDir);
  const failedKeys = new Set();
  const failReasons = new Map();
  for (const line of output.split("\n")) {
    if (!line.startsWith("FAIL ")) continue;
    const rest = line.slice(5);
    const key = rest.split(":")[0].split(" ")[0].trim();
    const colon = rest.indexOf(":");
    if (oracleKeys.has(key)) {
      failedKeys.add(key);
      if (colon !== -1) failReasons.set(key, rest.slice(colon + 1).trim());
    }
  }
  if (!ok) {
    // A non-zero exit after one reported failure is still an incomplete oracle run. Mark every
    // expected key failed so an early crash cannot silently pass the checks it never reached.
    for (const key of oracleKeys) {
      failedKeys.add(key);
      if (!failReasons.has(key)) failReasons.set(key, "oracle exited before a complete result set");
    }
  } else if (successMarker && !output.includes(successMarker)) {
    // An empty or unstructured zero exit is not evidence that the expected checks ran.
    for (const key of oracleKeys) {
      failedKeys.add(key);
      failReasons.set(key, `oracle success marker missing: ${successMarker}`);
    }
  }
  return {
    ok: ok && failedKeys.size === 0,
    exitCode,
    output,
    allExpectedChecksObserved: ok ? Boolean(!successMarker || output.includes(successMarker)) : false,
    results: [...oracleKeys].map((key) => ({ key, pass: !failedKeys.has(key), reason: failedKeys.has(key) ? failReasons.get(key) ?? null : null })),
  };
}

function collectDirectoryFiles(root, relative = "", files = {}) {
  for (const entry of readdirSync(root, { withFileTypes: true })) {
    if (entry.name === ".git" || entry.name === "node_modules") continue;
    const rel = relative ? `${relative}/${entry.name}` : entry.name;
    const full = path.join(root, entry.name);
    if (entry.isDirectory()) collectDirectoryFiles(full, rel, files);
    else if (entry.isFile()) files[rel.replaceAll(path.sep, "/")] = readFileSync(full, "utf8");
  }
  return files;
}

function adaptDirectoryImports(root) {
  const before = collectDirectoryFiles(root);
  const adaptations = [];
  for (const [rel, source] of Object.entries(before)) {
    if (!/\.tsx?$/.test(rel)) continue;
    const adapted = fixTsImports(source);
    if (adapted === source) continue;
    writeFileSync(path.join(root, rel), adapted);
    adaptations.push({ path: rel, before_sha256: createHash("sha256").update(source).digest("hex"), after_sha256: createHash("sha256").update(adapted).digest("hex") });
  }
  return adaptations;
}

/** Grade the actual files left by the Go CLI. The model's final answer is not an input. */
export function gradeDirectory(scenario, workDir, oracleRoot, scratchDir, options = {}) {
  const oracleKeys = new Set(scenario.bugs.filter((b) => b.oracle).map((b) => b.key));
  if (options.expectedOracleKeys && (oracleKeys.size !== options.expectedOracleKeys.length || options.expectedOracleKeys.some((key) => !oracleKeys.has(key)))) {
    throw new Error("scenario oracle keys differ from the frozen contract");
  }
  const work = path.join(scratchDir, `directory_${scenario.id}_${process.pid}_${Date.now()}`);
  let oracle = null;
  let adaptations = [];
  try {
    rmSync(work, { recursive: true, force: true });
    cpSync(workDir, work, { recursive: true, dereference: false, force: true });
    adaptations = adaptDirectoryImports(work);
    if (oracleKeys.size > 0) {
      oracle = oracleVerdict(scenario, oracleRoot, work, scratchDir, options.successMarker ?? null);
    }
    const files = collectDirectoryFiles(work);
    const raw = Object.entries(files).map(([file, body]) => `== ${file} ==\n${body}`).join("\n");
    const patternResults = scenario.bugs.filter((bug) => !bug.oracle).map((bug) => {
      const { pass, reason } = evalBug(bug, raw, files);
      return { key: bug.key, label: bug.label, pass, reason: pass ? null : reason };
    });
    const oracleResults = oracle?.results ?? [];
    const results = [...oracleResults, ...patternResults];
    return {
      scenario: scenario.id,
      pass: results.every((result) => result.pass),
      allPassed: results.every((result) => result.pass),
      results,
      oracle: oracle ? { ok: oracle.ok, exit_code: oracle.exitCode, all_expected_checks_observed: oracle.allExpectedChecksObserved, output: oracle.output } : null,
      import_adaptations: adaptations,
    };
  } finally {
    rmSync(work, { recursive: true, force: true });
  }
}

/** Grade every bug in a scenario, in a fresh temp directory that is always cleaned up. A bug with
 * `"oracle": true` is decided by actually running the code; every other bug uses the file-scoped pattern
 * rules. Mirrors grade_lib.py's merge exactly. */
export function grade(scenario, rawOutput, oracleRoot, scratchDir) {
  const oracleKeys = new Set(scenario.bugs.filter((b) => b.oracle).map((b) => b.key));
  let failedKeys = new Set();
  const failReasons = new Map();

  if (oracleKeys.size > 0) {
    const emitted = splitFiles(rawOutput);
    const work = path.join(scratchDir, `grade_${scenario.id}_${process.pid}_${Date.now()}`);
    mkdirSync(work, { recursive: true });
    try {
      materialize(scenario, emitted, work);
      const { ok, output } = runOracle(scenario.id, oracleRoot, work);
      for (const line of output.split("\n")) {
        if (line.startsWith("FAIL ")) {
          const rest = line.slice(5); // e.g. "A: a token issued moments ago ... " or "A some message"
          const key = rest.split(":")[0].split(" ")[0].trim();
          const colon = rest.indexOf(":");
          failedKeys.add(key);
          if (colon !== -1) failReasons.set(key, rest.slice(colon + 1).trim());
        }
      }
      if (!ok) {
        // Any non-zero exit is incomplete, even when one or more FAIL lines were printed. A
        // crash after the first assertion must not silently pass the assertions it never reached.
        const firstLine = output.split("\n").find((l) => l.trim()) || "no output";
        for (const k of oracleKeys) {
          failedKeys.add(k);
          if (!failReasons.has(k)) failReasons.set(k, `oracle exited before a complete result set: ${firstLine.trim()}`);
        }
      }
    } finally {
      rmSync(work, { recursive: true, force: true });
    }
  }

  const files = splitFiles(rawOutput);
  return scenario.bugs.map((b) => {
    if (b.oracle) {
      const pass = !failedKeys.has(b.key);
      return { key: b.key, label: b.label, pass, reason: pass ? null : failReasons.get(b.key) ?? null };
    }
    const { pass, reason } = evalBug(b, rawOutput, files);
    return { key: b.key, label: b.label, pass, reason: pass ? null : reason };
  });
}
