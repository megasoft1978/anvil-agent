import assert from "node:assert/strict";
import { test } from "node:test";
import { parseFootprintJSON, parseFootprintText, evaluateFootprintGate } from "./memory-gate.mjs";

test("parseFootprintJSON reads the first process's footprint bytes", () => {
  const text = JSON.stringify({ processes: [{ pid: 1, footprint: 2196416 }] });
  assert.equal(parseFootprintJSON(text), 2196416);
});

test("parseFootprintJSON is null, never zero, for missing or malformed input", () => {
  assert.equal(parseFootprintJSON("not json"), null);
  assert.equal(parseFootprintJSON(JSON.stringify({ processes: [] })), null);
  assert.equal(parseFootprintJSON(JSON.stringify({ processes: [{ pid: 1 }] })), null);
  assert.equal(parseFootprintJSON(JSON.stringify({ processes: [{ footprint: -1 }] })), null);
  assert.equal(parseFootprintJSON(JSON.stringify({ processes: [{ footprint: "2196416" }] })), null);
  assert.equal(parseFootprintJSON(JSON.stringify({})), null);
});

test("parseFootprintText reads phys_footprint at any observed unit", () => {
  assert.equal(parseFootprintText("    phys_footprint: 2177 KB\n    phys_footprint_peak: 3201 KB"), 2177 * 1024);
  assert.equal(parseFootprintText("phys_footprint: 54 MB"), 54 * 1024 ** 2);
  assert.equal(parseFootprintText("phys_footprint: 1.2 GB"), 1.2 * 1024 ** 3);
  assert.equal(parseFootprintText("phys_footprint: 512 B"), 512);
});

test("parseFootprintText is null, never zero, for missing or malformed output", () => {
  assert.equal(parseFootprintText(""), null);
  assert.equal(parseFootprintText("no such process"), null);
  assert.equal(parseFootprintText("phys_footprint_peak: 3201 KB"), null);
  assert.equal(parseFootprintText("phys_footprint: -5 KB"), null);
  assert.equal(parseFootprintText("phys_footprint: NaN KB"), null);
});

test("parseFootprintText requires the report's own pid to match expectedPid", () => {
  const report = "zsh [35425]: 64-bit    Footprint: 2257 KB\nAuxiliary data:\n    phys_footprint: 2273 KB\n";
  assert.equal(parseFootprintText(report, 35425), 2273 * 1024);
  assert.equal(parseFootprintText(report, 99999), null, "footprint's -p fallback matched a different process by name; must not attribute its footprint to the wrong pid");
  assert.equal(parseFootprintText(report), 2273 * 1024, "no expectedPid given -> skip the check");
});

test("evaluateFootprintGate passes with no configured ceiling", () => {
  assert.deepEqual(evaluateFootprintGate(5_000_000_000, null), { pass: true, reason: "no server-footprint ceiling configured" });
  assert.deepEqual(evaluateFootprintGate(null, undefined), { pass: true, reason: "no server-footprint ceiling configured" });
});

test("evaluateFootprintGate fails closed when footprint is unmeasured but a ceiling is set", () => {
  const result = evaluateFootprintGate(null, 2 * 1024 ** 3);
  assert.equal(result.pass, false);
  assert.match(result.reason, /unavailable/);
});

test("evaluateFootprintGate compares against the ceiling, independent of system pressure", () => {
  const ceiling = 2 * 1024 ** 3;
  assert.equal(evaluateFootprintGate(1.5 * 1024 ** 3, ceiling).pass, true);
  assert.equal(evaluateFootprintGate(2 * 1024 ** 3, ceiling).pass, true);
  const over = evaluateFootprintGate(2.1 * 1024 ** 3, ceiling);
  assert.equal(over.pass, false);
  assert.match(over.reason, /exceeds the configured ceiling/);
});
