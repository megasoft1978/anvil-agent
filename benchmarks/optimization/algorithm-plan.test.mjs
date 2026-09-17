import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

const read = (name) => JSON.parse(readFileSync(new URL(name, import.meta.url), "utf8"));
const config = read("./experiments-gemma4-algorithms.json");
const original = read("./experiments-gemma4-local.json");
const baseline = config.experiments.find((e) => e.id === "GEMMA4-LOCAL-SCREEN");
const differences = (a, b) => Object.keys({ ...a, ...b }).filter((k) => JSON.stringify(a[k]) !== JSON.stringify(b[k])).sort();

test("algorithm matrix preserves the existing fit, screen, model and server flags", () => {
  assert.deepEqual(config.model, original.model);
  assert.deepEqual(config.server.common_flags, original.server.common_flags);
  assert.deepEqual(config.cli, original.cli);
  assert.deepEqual(config.experiments.slice(0, 2), original.experiments.slice(0, 2));
});

test("every new arm is paired and requires a complete successful baseline screen", () => {
  const expected = {
    "NGRAM-SIMPLE": [[], ["spec_type"]],
    "KV-Q8": [[], ["kv"]],
    "READ-8K": [["read_output_limit"], []],
    "SEARCH-8K": [["search_output_limit"], []],
    "FOCUSED": [["prompt_profile"], []],
    "DYNAMIC": [["tool_schema_policy"], []],
    "NO-EDIT-RETRY": [["retry_without_edit"], []],
  };
  assert.equal(config.experiments.length, 2 + Object.keys(expected).length);
  for (const [name, [agentDiff, serverDiff]] of Object.entries(expected)) {
    const arm = config.experiments.find((e) => e.id === `GEMMA4-ALGO-${name}-SCREEN`);
    assert.ok(arm, name);
    for (const key of ["tasks", "seeds", "budget", "scored"]) assert.deepEqual(arm[key], baseline[key], `${name}: ${key}`);
    assert.equal(arm.requires, "evidence-selected");
    assert.deepEqual(arm.depends_on, [{ experiment_id: baseline.id, condition: "verified_repair_no_pressure", complete: true }]);
    const defaults = { force_edit_after_reads: 5, retry_without_edit: false, prompt_profile: "baseline", read_output_limit: 32768, search_output_limit: 32768 };
    assert.deepEqual(differences({ ...defaults, ...baseline.agent }, { ...defaults, ...arm.agent }), agentDiff, name);
    assert.deepEqual(differences(config.server.profiles[baseline.server_profile], config.server.profiles[arm.server_profile]), serverDiff, name);
  }
});

test("research-only candidates cannot be mistaken for runnable arms", () => {
  const agenda = read("./research-agenda.json");
  assert.equal(agenda.experiments, undefined);
  assert.equal(new Set(agenda.candidates.map((c) => c.id)).size, agenda.candidates.length);
  for (const c of agenda.candidates) {
    assert.equal(c.status, "requires_preparation");
    assert.ok(c.requires.length && c.metrics.length && c.sources.length && c.stop && c.comparison, c.id);
  }
});
