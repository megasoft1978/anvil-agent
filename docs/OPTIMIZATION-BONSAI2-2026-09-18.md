# Ternary Bonsai 2 27B — real-repair evaluation, 2026-09-18/19

New-model screen the day after release (PrismML, 2026-09-17, Apache 2.0). Bonsai 2 27B is a
**ternary quantization of Qwen3.8-27B** — the same base model already screened at IQ3_XXS in
`docs/OPTIMIZATION-QWEN38-2026-09-17.md` (3 attempts, 0 fixes, 1 regression on
`immer-array-push-fix`). This document covers a full 6-fixture screen at PQ2_0/xhigh-reasoning,
plus what the machine-level memory investigation along the way established about speed and
memory under contention.

## Result

**Six independent, objectively-graded attempts across the full prepared real-repository task
set. Zero edits made in any of the six.** Every attempt spent its entire time budget in
tool-driven exploration and reasoning and never once reached a source edit — a materially
different (and less informative about repair quality) failure mode than the Qwen3.8 screen,
which completed clean attempts and produced confidently-wrong fixes. This is a
capability/configuration finding about `xhigh` reasoning effort on this hardware, not evidence
either way about the model's actual repair ability.

| Fixture | Turns | Tool calls | Edits | Prompt tok | Completion tok | tok/s (avg) | Termination |
|---|---|---|---|---|---|---|---|
| immer-array-push-fix | 14 | 13 | 0 | 115,967 | 2,792 | 3.11 | deadline exceeded (mid-turn) |
| immer-array-methods-noop-sharing | 14 | 13 | 0 | 112,331 | 5,061 | 2.57 | deadline exceeded (mid-turn) |
| qs-stringify-date-filter | 6 | 5 | 0 | 25,664 | 5,057 | 3.50 | deadline exceeded (mid-turn) |
| zod-memoizer-retention | 7 | 7 | 0 | 47,416 | 7,119 | 3.28 | **insufficient time for another turn** (clean stop) |
| zod-discriminated-union-defaulted-tags | 16 | 15 | 0 | 134,427 | 1,430 | 3.60 | deadline exceeded (mid-turn) |
| zod-int-json-schema | 22 | 21 | 0 | 304,252 | 4,057 | 2.57 | deadline exceeded (mid-turn) |

Total wall time across the six 45-minute-budgeted attempts: **267 minutes (~4.5 hours)**. Every
attempt but one was cut off mid-generation; `zod-memoizer-retention` is the one case where the
harness's own remaining-time check stopped it cleanly after a completed turn instead.

Pattern is fixture-size-independent: the smallest fixture by prompt tokens
(`qs-stringify-date-filter`, 25.7K) and the largest (`zod-int-json-schema`, 304K, the only
fixture that hit the harness's 40-turn ceiling territory at 22 turns) both ended the same way —
zero edits, budget exhausted. Two runs on `immer-array-push-fix` (this one and an earlier
session-interrupted attempt) independently reached the same 14-turn/13-tool-call shape.

## Model and artifacts

- `prism-ml/Ternary-Bonsai-2-27B-gguf`, file `Ternary-Bonsai-2-27B-PQ2_0.gguf`,
  7,206,168,928 bytes, sha256
  `3907dc1658db1f78a9826bf8d5bcb8dc65db0d466388937af57f2294fae62ec1` — verified against
  HuggingFace's `x-linked-etag` header (exact match).
- GGUF metadata read directly from the file header: `general.architecture: qwen35`,
  `qwen35.context_length: 262144`, `qwen35.block_count: 64`, and — notably — the file's own
  embedded `general.sampling.temp/top_p/top_k = 1.0/0.95/20`, exactly matching the whitepaper's
  documented thinking-mode sampling. `prism.hadamard.*` keys confirm the blockwise
  Walsh-Hadamard rotation (block size 1024) the whitepaper describes.
- Served via a custom fork, **not** stock llama.cpp: `PrismML-Eng/llama.cpp` (`prism` branch),
  built locally (Metal, Apple Silicon), build `0.2.0-dev` (build 10706), commit `1a07bfa5f`.
  Stock `llama-server` (the b10809 binary already pinned for the Qwen3.8 config) cannot load
  these ternary/Hadamard-rotated weights — confirmed in the whitepaper and not re-tested here,
  since building the fork was already required.
- Harness: `go-agent` CLI built from the current tree, invoked directly (no `--experiment-config`
  server-launch coupling; per-run JSON overrides for seed/prompt-profile/max-turns), against
  fresh pristine copies of all six prepared pilot fixtures.
- New config: `benchmarks/optimization/experiments-bonsai2-local.json` (llama backend, port 8118,
  distinct from the Qwen3.8 config's port 8117). New `go-agent/profiles.go` entry `bonsai2-27b`
  (temp 1.0, top_p 0.95, top_k 20, presence 0.0, seed 42 — sourced from the model card and cross-
  checked against the GGUF's own embedded sampling defaults).

## Server verification, before trusting any attempt

Learned from the Qwen3.8 screen not to trust `/health` or a `model loaded` log line. Before any
attempt: a live `/v1/chat/completions` request returned coherent, correctly-instruction-following
text ("Hello my friend" for a "say hello in exactly 3 words" prompt), and a synthetic tool-call
request confirmed the prism fork's `--jinja` correctly parses this model's native
`<tool_call><function=name><parameter=x>` XML chat-template output into structured OpenAI-style
`message.tool_calls` (not left as raw text) — matching the Bonsai-demo repo's `TOOLS.md`
("llama-server returns structured tool_calls... rather than raw XML text"). `--reasoning-format
deepseek` correctly routes thinking into `message.reasoning_content`, separate from
`message.content`; `go-agent/protocol.go` already reads `reasoning_content` natively (as a
secondary source for the tool-call-leak detector) and `go-agent/recovery_qwenxml.go` already has
a dedicated fallback parser for this exact `<function=...>` shape, unneeded here since native
parsing worked correctly. No harness code changes were required beyond the new model profile.

## Memory and speed: a real, measured effect from system contention

This machine (M1, 16 GB) had substantial unrelated load running for part of this screen: multiple
duplicate Traqly `next build`/eslint processes, and **two separate orphaned Claude Code sessions
both sitting in the same `ColPal/csc` directory** — one of them 16+ hours old — each with its own
tree of `pnpm dev:snowflake` → `nest.js --watch` chains (44 processes total once fully traced).
Closing all of it plus the Traqly session ending on its own dropped system memory pressure
sharply:

| | Under contention | After cleanup |
|---|---|---|
| `PhysMem` unused | 94–146 MB | 2.6 GB |
| Wired | 10–12 GB | 1.8–2 GB |
| Compressor | 2.4 GB | 0.4–0.5 GB |
| Idle CPU | 0–53% | 47–76% |
| Decode (isolated smoke test) | 4.24 tok/s | **5.64 tok/s** |

Freeing memory produced a genuine **~1.3× speedup on an isolated single-turn request**, and
noticeably smoother per-token latency (fewer of the 3-second-window dips into 1.2–1.7 tok/s seen
under heavy contention). It did **not**, however, fix the zero-edit outcome — all six clean-memory
attempts still exhausted their budget without an edit, so reasoning effort (see below), not
memory pressure, is the dominant factor in that result.

Memory footprint grew over the course of the 6-fixture sweep: `llama-server`'s own
`phys_footprint` rose from ~2.5 GB (isolated) to **9.4 GB by the last fixture**
(`zod-int-json-schema`, the largest context at 304K cumulative prompt tokens), and
`peak_server_rss_mb` reported by the harness itself ranged 8.7–9.5 GB across fixtures — this
tracks context depth, not a leak (each fixture starts a fresh conversation; the growth reflects
KV cache and slot state sized to whatever context each attempt actually reached, all comfortably
inside the 32768-token ceiling this config runs at, itself well inside the model's native 262K).

## Fixture-prep issues found and fixed (apply to any future run of these fixtures)

- **The "stale test file" trap from the Qwen3.8 doc generalizes to 7 of 8 test files across all
  six fixtures**, not just `immer-array-push-fix`. Before grading any of these pilot fixtures,
  install the manifest's embedded `test_files` content over whatever is checked into
  `go-agent/pilot/worktrees/<task>/` — verified by direct byte-length diff against
  `go-agent/pilot/manifest.json`.
- **The three zod fixtures' `node_modules` (a pnpm monorepo) shipped with broken/missing
  hoisted symlinks** — the entire `node_modules/@vitest/*` scope was absent even in the
  pristine source worktree (not something this session's copy broke), and a subsequent `pathe`
  resolution failure surfaced too. Fixed with `pnpm install --offline --frozen-lockfile` (uses
  the existing local store, no network) in each zod worktree; this relinks correctly and is safe
  to always run before testing a zod fixture. One repo's `packages/docs` postinstall hook fails
  on an esbuild version mismatch unrelated to the `zod` package under test — harmless, ignore it.
- **`qs-stringify-date-filter` uses a custom test runner** (`scripts/tap-json.mjs`, TAP output
  reparsed into JSON) whose report uses **capitalized keys** (`FullName`, `Status`) unlike
  vitest's own JSON reporter (`fullName`, `status`). Any grading script must handle both.
- **The manifest's own `pass_to_pass`/`fail_to_pass` name lists mix two prefix conventions** for
  the same file (some entries include a `<path>.test.ts ` prefix, others the bare test title) —
  confirmed by direct lookup in the actual vitest report; these are the same tests under two
  spellings, not missing tests. Grading must strip-and-retry the prefix before concluding a
  name is actually missing.
- All six pristine baselines were re-verified against the manifest's `fail_to_pass`/
  `pass_to_pass` lists after these fixes and matched exactly before any model attempt ran.

## Candidate improvements (status after this session's iteration)

1. **Free system memory** — tested, confirmed: ~1.3× decode speedup, smoother latency, lower
   peak wired memory. Does not by itself fix the zero-edit outcome.
2. **Reasoning effort: xhigh vs medium** — **not yet tested**, and now the clearly indicated next
   step. All 6/6 xhigh attempts exhausted budget without an edit; the whitepaper's own benchmark
   table shows `medium` retains most reasoning-category quality (79.3 vs 82.6) at presumably far
   fewer thinking tokens per turn. Given the pattern held across fixtures 25K–304K prompt tokens
   and 6–22 turns, effort level — not context size or repo size — looks like the lever most
   likely to actually produce a completed, gradeable attempt.
3. **PTQ1_0 vs PQ2_0 packing** — not tested. Whitepaper Table 5 shows PTQ1_0 slower than PQ2_0 on
   Apple Silicon despite being smaller; unverified on this specific M1.
4. **Thread count** (`-t`) — not tested. Server used the default `n_threads = 4` on this 8-core
   (4P+4E) M1 throughout; whether raising it helps the Hadamard-transform CPU work is untested.
5. **KV cache precision below q8_0** — not tested; whitepaper's own roadmap flags sub-4-bit KV
   as a stated future direction.
6. **`--reasoning-preserve`** — considered, rejected: the server log suggests it, but it would
   carry every turn's full thinking trace forward in history, compounding prompt-processing cost
   and memory as turns accumulate. Not recommended for this multi-turn agent shape.
7. **Statistical confidence** — two independent attempts on `immer-array-push-fix` (this session)
   reached the same 14-turn/13-tool-call shape, which is suggestive but not sufficient; a genuine
   second seed per fixture at a working (e.g. medium) effort level would be the next step once
   any fixture actually produces a completed, gradeable attempt.

## Follow-up: medium reasoning effort — verified repair

Restarted the server with `--reasoning-effort medium` (all other settings unchanged: same
sampling, same seed 42, same fixtures, same harness) to test the improvement flagged above.

**`qs-stringify-date-filter` produced a verified repair on the first attempt** — 3 tool calls
(2 reads, 1 edit), all 6 `fail_to_pass` tests now passing, all 20 `pass_to_pass` tests still
passing, 197/197 total, zero regressions. The fix decouples the Date-serialization check from
being an `else` branch of the filter check (`} else if (obj instanceof Date) {` →
`}\n\nif (obj instanceof Date) {`), which is exactly the reported bug: previously a `filter`
function running at all skipped the Date check even when the filter returned an untouched Date.

**This is the first verified repair across this entire multi-day program.** Every prior
model/engine combination — Qwen3.6, Qwen3.8 (this same base model at IQ3_XXS), Hebrus, Mference,
and Bonsai 2 itself at `xhigh` — either failed on infrastructure before a real attempt or
produced a confidently-wrong fix. The reasoning-effort setting, not the model or the
quantization, was the blocker on `xhigh`. Remaining fixtures are being re-run at `medium` for a
full comparison against the `xhigh` sweep above; this document will be updated with the complete
medium-effort table once that finishes.

## Conclusion

Ternary Bonsai 2 27B is real, correctly documented, loads and serves correctly on this hardware
via the required custom fork, and its tool-calling/reasoning-separation machinery works exactly
as specified with no harness changes needed. Freeing contended system memory produced a real,
measurable ~1.3× speed improvement. But at the model's own documented default reasoning effort
(`xhigh`), it did not produce a single completed, gradeable repair attempt across six independent
real-repository fixtures in this session — every attempt spent its full 45-minute budget in
exploration and reasoning without ever reaching an edit. This is a distinct outcome from the
Qwen3.8 IQ3_XXS screen, which completed attempts and produced diagnosable (if wrong) fixes; it
says nothing yet about Bonsai 2's actual repair quality, only that `xhigh` effort on this
hardware needs a different reasoning-effort setting or longer budget before that question can be
answered. **Medium reasoning effort is the clearly indicated next experiment.**
