import assert from "node:assert/strict";
import { test } from "node:test";
import { cachedPromptTokens } from "./cache-metrics.mjs";

test("MLX usage records hits and measured zero without inference timings", () => {
  for (const count of [0, 942]) {
    assert.equal(cachedPromptTokens({ usage: { prompt_tokens_details: { cached_tokens: count } } }), count);
  }
});

test("llama timing aliases take precedence, including zero", () => {
  for (const key of ["cache_n", "cache_tokens", "cached_tokens"]) {
    assert.equal(cachedPromptTokens({ timings: { [key]: 0 }, usage: { prompt_tokens_details: { cached_tokens: 20 } } }), 0);
  }
  assert.equal(cachedPromptTokens({ Timings: { cache_n: 30 } }), 30);
  assert.equal(cachedPromptTokens({ Usage: { PromptTokensDetails: { cached_tokens: 30 } } }), 30);
});

test("unknown and malformed cache counters remain missing", () => {
  assert.equal(cachedPromptTokens({ usage: { prompt_tokens: 100 } }), null);
  for (const value of [null, undefined, false, "", "20", -1, 1.5, Infinity, NaN]) {
    assert.equal(cachedPromptTokens({ timings: { cache_n: value } }), null);
  }
  assert.equal(cachedPromptTokens({ timings: { cache_n: -1 }, usage: { prompt_tokens_details: { cached_tokens: 12 } } }), 12);
});
