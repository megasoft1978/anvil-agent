# Qwen3.8-27B GSQ-RCO — real-repair evaluation, 2026-09-17

Continuation of the same-day local-model screen and the same-day session that preceded this
document. This document is the Qwen3.8-27B GSQ-RCO half of that evening's work,
using the tool-schema-enforcement fix landed in `go-agent/agent.go` earlier the same session
(commit `236a62e`).

## Result

**Three independent, objectively-graded attempts on the real `immer-array-push-fix` fixture.
Zero produced a correct repair. One introduced two new regressions.** This is a real
model-capability finding, not a memory, context, or tooling limitation — every failure mode
below is distinct from the memory-pressure and tool-schema-violation failures that closed out
Hebrus and Mference earlier the same evening.

## Model and artifacts

- `ISTA-DASLab/Qwen3.8-27B-GSQ-RCO-GGUF`, file `Qwen3.8-27B-GSQ-RCO-IQ3_XXS.gguf`,
  10,094,357,632 bytes, sha256 `fdfcb6a29b11188956dfbfd904223588a6c1b77eb250c3e8a36e1bd269df91f7`
  (matches the hash already pinned in `benchmarks/optimization/experiments-qwen38-local.json`).
  GGUF metadata reports `ftype: IQ3_S - 3.4375 bpw` despite the IQ3_XXS filename — file size and
  param count (26,895,998,464) both match the expected IQ3_XXS artifact; this is a metadata
  labeling quirk in the upstream quant, not evidence of a wrong download.
- `ISTA-DASLab/Qwen3.8-27B-GSQ-RCO-GGUF`, file `Qwen3.8-27B-GSQ-RCO-IQ3_S.gguf`,
  11,771,546,784 bytes, sha256 `64b53b64c7aa39f20a7e54bd80582fe595b1d745624ee8a72e92508c0326d810`.
- Served via stock `llama-server` (build 10809, commit `5266f24da` — the same binary used all
  week), not a custom runtime.
- Harness: `go-agent` CLI built from the current tree (includes the
  force-edit-window/close-out-reserve client-side enforcement fix), invoked directly
  (`-experiment-config`) rather than through the optimization runner, against a
  freshly-copied pristine `immer-array-push-fix` worktree each attempt.

## The download itself was not clean — two infrastructure notes

- `hf download` (huggingface_hub 1.10.2) stalled twice on this repo, once at 74% with zero
  incoming bytes and 0% CPU for 3+ minutes, and once immediately after resume (created a fresh
  zero-byte `.incomplete` rather than resuming). This repo is Xet-backed (chunked
  content-addressed storage via `us.aws.cdn.hf.co`), and the CLI's Xet path did not recover
  cleanly. Switching to a plain `curl -L --retry 10` against the `/resolve/main/...` redirect
  target completed both files without incident. **Prefer `curl` over `hf download` for this
  repo family.**
- The GGUF's own reported `ftype` string does not reliably indicate which named quant variant
  you downloaded; verify by exact byte size and sha256 against the source repo, not by the
  `ftype` field in `/v1/models`.

## A hard Metal buffer ceiling exists, separate from general system memory

`-ngl 99` (full GPU offload) with IQ3_XXS works cleanly through `-c 24576` with
`--cache-type-k q8_0 --cache-type-v q8_0`. Without KV quantization, `-c 24576` and `-c 32768`
both fail identically and reproducibly:

```
common_fit_params: failed to fit params to free device memory: n_gpu_layers already set by user to 99, abort
ggml_metal_synchronize: error: command buffer 0 failed with status 5
error: Insufficient Memory (00000008:kIOGPUCommandBufferCallbackErrorOutOfMemory)
llama_server: model loaded
llama_server: listening on http://127.0.0.1:8114
```

The server prints `model loaded` and `listening` **after** this error and answers `/health`
with 200 — but every actual chat-completion request against it returns
`{"error":{"code":500,"message":"Compute error."}}`. **A 200 on `/health` does not mean the
server is usable; a live completion request is required to confirm a load actually
succeeded.** This reproduced on IQ3_XXS at `-c 32768` (twice, including after freeing ~2 GB of
system RAM — ruling out general memory pressure as the cause) and on IQ3_S at every context
size tried (8192, 16384, 24576, all with quantized KV). The practical, empirically confirmed
ceiling on this M1 for this model family is: **IQ3_XXS fits fully GPU-resident up to `-c
24576` with quantized KV; IQ3_S (11.77 GB, 1.7 GB larger) does not fit fully GPU-resident at
any tested context.** Neither raising nor lowering context rescued IQ3_S — this is a weight-load
ceiling, not a KV-cache ceiling, for that file.

Measured resident footprint for the working IQ3_XXS configuration (`footprint -p`, 20s
samples over full repair attempts): stabilizes in the **2.7–4.7 GiB** range once weights are
paged in, well under the 24576-context Metal ceiling and unrelated to the 9–10 GiB
"dense-resident" ceiling this project measured for other model families — a quantized dense
27B model at this bit-width has a materially smaller working set than that number suggested.

## Speed

Server-reported `print_timing` across multiple turns of real repair attempts (IQ3_XXS,
`-c 24576`, quantized KV):

- Prefill: **27.4–33.5 tok/s**, essentially flat across prompt sizes from ~5,000 to ~15,900
  tokens observed.
- Decode: **2.6–3.7 tok/s**, i.e. roughly 270–380 ms/token — this is a fully dense 27B model at
  IQ3, so the whole file is read every token; there is no MoE-style expert-skip lever available
  to this family in the earlier comparison arms.

## Three real attempts, three distinct outcomes, zero fixes

The fixture: `immer` `arrayMethods.ts`, `handleInsertedValues`. The actual upstream fix
(`d2c158f5bac7081a760bbaf501ea5c360b7856e1`) is a one-line type coercion —
`const index = startIndex + i` → `const index = "" + (startIndex + i)` — making the patch-path
index a string instead of a number. **All three attempts diagnosed a plausible-sounding but
incorrect story about the bug** (an index-computation error related to `pop()` reusing a freed
array slot) and never touched, or touched incorrectly, the line the real fix changes.

| Attempt | Config | Outcome | Edit made | Oracle result |
|---|---|---|---|---|
| 1 (seed 42) | `-c 16384`, unquantized KV | `context_limit`: conversation hit 16,796 tokens against a 16,384 window | Two cosmetic edits: added an explanatory comment (4 lines), added a blank line. **Zero functional change.** | Not reached (cut off) |
| 2 (seed 42) | `-c 24576`, KV q8_0, 90 min budget | `status: "completed"` — model stated **"The fix is complete"** | Refactored `lengthBefore` (captured pre-push) into `insertionStart = state.copy_!.length - args.length` (computed post-push). **Mathematically identical value** — `copy_.length` after appending N items is `lengthBefore + N`, so `insertionStart` always equals `lengthBefore`. A confident, well-commented, functionally inert restructuring. | **FAIL** — `"push onto an index freed by pop() is patched"` still fails, identical to the pristine baseline |
| 3 (seed 31415) | same config, `max_history_bytes` raised to 256 KiB | `context_limit` again — hit go-agent's own history-byte cap (a harness setting, not the model's context window) at turn 14 | Accidentally edited the wrong file first (`finalize.ts`), caught and reverted its own mistake, then restructured `handleInsertedValues` to compute a shared destination index for **both** `push` and `unshift` from `state.copy_!.length - values.length` | **FAIL, and regressed**: original bug still fails, **plus two new failures introduced** — `"can unshift a spread draft back into the array"` and `"can splice a spread draft into the array"` (the shared-index refactor is wrong for `unshift`, which must start at 0, not `length - values.length`) |

All three attempts were graded against the **correct** oracle test
(`__tests__/base.js` as embedded in `go-agent/pilot/manifest.json`, 3,224 tests, not the
120,162-byte version checked into the pilot worktree on disk, which is missing the
regression test entirely and passes 3,215/3,215 on both the pristine and every edited
version — **this stale on-disk copy must not be used to grade this fixture**; always
install the manifest's embedded test file before running vitest against a copy of this
worktree). Confirmed the pristine (unmodified) fixture fails exactly 1/3,224 with the correct
test installed, and passes 3,215/3,215 with the stale one — a silent false-negative trap for
any future manual grading of this fixture.

A genuine, if incidental, capability signal from attempt 3: the model can recognize and
self-correct an accidental wrong-file edit within the same turn sequence, without being told.
That is a real, positive behavior — it just was not enough to compensate for consistently
misdiagnosing the actual defect.

## Conclusion

On this fixture, at this quant, with a context and memory budget that let it run to natural
completion (no truncation, no memory abort) at least once: **Qwen3.8-27B GSQ-RCO IQ3_XXS
reliably produces plausible, confidently-stated, and incorrect fixes.** This is a materially
different failure class from every other model/engine combination tested this week — Hebrus
and the other comparison arms failed on infrastructure (crashes, tool-schema violations, memory) before
ever reaching a real, complete attempt at the fix; Qwen3.8 completed clean attempts and still
got the bug wrong, once while making it worse. The higher-precision IQ3_S quant, which might
plausibly have diagnosed better, does not fit this hardware's Metal buffer ceiling at all.

**Net assessment for this project's central question** (can this class of local model/hardware
solve real repository bugs): no verified repair has been produced by any model or engine
combination tested across this entire multi-day program, and this is now the first case where
the blocker is squarely the model's reasoning quality on a real bug, observed under conditions
with no infrastructure excuse.
