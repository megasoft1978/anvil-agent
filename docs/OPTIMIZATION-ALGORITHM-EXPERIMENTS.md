# Algorithm experiment integration

Validation on September 15: six model-free tests passed, and the actual runner prepared session
`.optimization-results/20260915T151540842Z-b401994db269` from this matrix. Preparation checked
configuration shape and captured input/model/CLI hashes; this new session's fixture validation,
native gates and live measurements remain pending. The original first-fit session's fixture
results do not automatically transfer to it.

The new runnable matrix is `benchmarks/optimization/experiments-gemma4-algorithms.json`.
The research queue is `benchmarks/optimization/research-agenda.json`. Neither changes the
currently prepared first Gemma session, production defaults, or model files. No inference or
download is part of this integration. A configured arm is not a measured improvement.

## Runnable configurations, pending live validation

The matrix retains the existing Gemma fit and four-task/two-seed baseline screen. Each new arm
requires that complete memory-valid screen with at least one verified repair, plus the runner's
existing holdout/evidence-selection gates. It uses identical tasks, seeds, budgets and model.

| Arm suffix (prefix `GEMMA4-ALGO-`, suffix `-SCREEN`) | Changed factor |
|---|---|
| NGRAM-SIMPLE | Existing engine's model-free ngram-simple speculation |
| KV-Q8 | Key/value cache precision policy: Q8 instead of default F16 |
| READ-8K | Maximum read result: 8 KiB instead of 32 KiB |
| SEARCH-8K | Maximum search result: 8 KiB instead of 32 KiB |
| FOCUSED | Existing focused agent prompt |
| DYNAMIC | Existing dynamic tool policy; close-out stays disabled |
| NO-EDIT-RETRY | One existing oracle-free retry after a no-edit response |

Read and search bounds are bytes, not an 8K-token context. Q8 is a precision tradeoff, not a claim
of exact output equivalence. Dynamic includes both the edit-only window and repeated-call bans.
Ngram parameters are the defaults of pinned build 10809 (simple lookup 12, draft length 48,
minimum hits 1), verified in local `llama-server --help`. Do not assume generic engine support
proves that each model passes the native gates. Gemma keeps its existing checkpoint/cache flags.

Select ONE arm from measured evidence, not all seven. Start with bounded output/prefix reuse when
prefill dominates, ngram when copying/decode dominates, and Q8 when KV memory is material. A failed
baseline closes this matrix for that candidate; use the existing smaller-model fallback order.
The already prepared Qwen3.8 and Qwen3.6 ngram arms remain unchanged and retain their own gates.

## Preparation and execution

The existing prepared Gemma fit session can still run unchanged after its preflight. To evaluate
this expanded matrix, prepare a separate session. It deliberately requires its own baseline;
historical runs are not copied into a new session to satisfy dependencies.

```sh
node --test benchmarks/optimization/algorithm-plan.test.mjs benchmarks/optimization/cache-metrics.test.mjs
node benchmarks/optimization/runner.mjs prepare --cli /tmp/anvil-agent-opt \
  --experiments benchmarks/optimization/experiments-gemma4-algorithms.json
```

Set `SESSION` to the printed path. Validate the fit fixtures, then execute the fit only after the
documented reboot/closed-app preflight. Before a conditional screen, validate its four tasks and
the frozen holdouts and record the evidence selection in that session's `DECISIONS.md`.

```sh
node benchmarks/optimization/runner.mjs validate --session "$SESSION" \
  --task immer-array-push-fix,notify-channel
node benchmarks/optimization/runner.mjs run --session "$SESSION" --cli /tmp/anvil-agent-opt \
  --experiment GEMMA4-LOCAL-8K-REAL --start-server --max-runs 1
# Only after fit success and conditional fixture/holdout validation:
node benchmarks/optimization/runner.mjs run --session "$SESSION" --cli /tmp/anvil-agent-opt \
  --experiment GEMMA4-LOCAL-SCREEN --start-server --evidence-approved
# Example selection ONLY after the complete baseline screen supports speculation:
node benchmarks/optimization/runner.mjs run --session "$SESSION" --cli /tmp/anvil-agent-opt \
  --experiment GEMMA4-ALGO-NGRAM-SIMPLE-SCREEN --start-server --evidence-approved
node benchmarks/optimization/runner.mjs summarize --session "$SESSION"
```

Do not run without an explicit experiment ID or combine different server profiles in one invocation.
Missing counters remain missing. Every failure remains in the denominator. Select a finalist only
after paired repair/memory review, then use unused holdouts and a cold-start check. A 15% total-time
or 512 MiB footprint gain is a screening criterion, not statistical proof. Do not promote from a
token-rate improvement alone. A combined winning stack needs a separate final confirmation.

## Research candidates with explicit prerequisites

The JSON agenda records priorities, prerequisite lists, comparisons, metrics, stop rules and
primary-source links for all these branches:

- Task-scoped prefix reuse: bounded cache plus reliable task reset; MLX cache-off remains the
  baseline until reset or matched restart handling exists.
- Ngram-mod: shared-pool reset or matched process restarts before scoring. Erasing a llama slot
  must not be assumed to erase the ngram pool.
- Adaptive ngram/suffix speculation: engine-level draft control, task-local bounded index and
  correct token/JSON handling. Optimize expected committed tokens divided by measured round time,
  including verification and rollback; include no speculation as an action.
- DFlash diffusion drafting: a compatible target-specific artifact, immutable provenance, memory
  budget, and native tool/sampling/state validation. No artifact is selected or downloaded here.
- MTP, EAGLE-3, DSpark: select one compatible learned-draft method at a time; no blind stacking.
- Evidence selection: exact versioned source spans, token accounting, valid tool-message groups
  and invalidation of references to evicted results before any compaction experiment.
- Joint expert/conversation cache allocation: measured milliseconds saved per MiB, runtime budget
  controls and hysteresis. Cache bytes are competing allocations, not independent free resources.
- Exact expert prefetch: native routing retained, measured physical I/O, residual stalls and wasted
  reads. Start reactive, then coalescing, then cheap predictors before trained predictors.
- Full diffusion: a separate model-stack comparison requiring an Apple runtime and native tools;
  Fast-dLLM, v2, ++ and dKV-style caching are subsequent one-factor arms, not claimed existing support.
- LayerSkip/native low-bit models: separate checkpoint comparisons; no arbitrary layer removal or
  post-training quantization relabeled as native BitNet.
- More aggressive KV compression: only after conservative Q8 and evidence that KV is the bottleneck;
  KIVI/XQuant papers motivate experiments, not existing compatible Apple implementations.

These entries are intentionally not runner experiment definitions. Marking them executable requires
implementing their prerequisites and preparing a pinned configuration. The user authorized their
integration into the research plan; this does not claim that compatible implementations exist.
