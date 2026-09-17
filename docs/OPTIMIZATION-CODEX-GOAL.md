# Codex goal: find the best local coding-agent stack on the 16 GiB M1

> Review precedence: [the revised decision package](OPTIMIZATION-ASTRA-DECISION.md#reviewed-decision-and-precedence--2026-09-14) controls priorities and interpretation where this document differs. The first live probe is recorded below; later model-dependent validation remains pending.

This is a paste-ready goal for Codex or Luna Max. The authoritative research and evidence package is
[OPTIMIZATION-ASTRA-REVIEW.md](OPTIMIZATION-ASTRA-REVIEW.md). Keep decisions compact and point to the
recorded result files instead of repeating raw traces.

## Goal

Determine whether this Apple M1 / 16 GiB Mac can run a useful coding agent for real repository bugs,
and select the best combination of model, inference engine, quantization, and the existing Go agent
harness by verified repair quality, elapsed time, and memory pressure.

The winner must use the real `go-agent` CLI, real model-selected native `read`, `search`, `edit`, and
`write` calls, a fresh worktree, and an external oracle that grades the resulting files. A final model
answer, a pattern match, a raw completion, or a server health response is not a repair.

## Fixed constraints

- Hardware is the Apple M1 Mac with 16 GiB physical memory. Do not change memory limits, wired-memory
  settings, swap settings, or system configuration.
- Use one local server and one attempt at a time. Never spawn external agents or use another model to
  solve or judge a task.
- Do not download weights automatically. Inspect exact Hugging Face file sizes and provenance first;
  keep at most one new candidate download in the decision path while disk is near capacity.
- Do not close applications automatically. Before a memory-sensitive batch, the operator
  must reboot, close every other application, leave only the terminal running the batch, and record
  idle memory and swap. Do not begin if idle pressure is abnormal or swap is growing.
- Every timeout, empty completion, zero-edit run, malformed tool call, grading error, and pressure abort
  remains in the denominator and in `runs.jsonl`.
- Do not claim a validation was run when its artifact is absent. Unrun fixture checks, model probes,
  benchmarks, and full regressions are `pending`.
- Preparation and static inspection may proceed. The current operator has disabled the conversational
  approval gate, so inference, model warmup, fixture verification, and benchmark runs may execute from
  the documented commands. Preserve the memory, server-ownership, evidence-order, and stopping rules.

## Current evidence to preserve

The TurboQuant Qwen3.6-35B-A3B TQ3 model is present at
`/Users/megasoft78/.anvil-agent/models/Qwen3.6-35B-A3B-tq3-g32` and has 16,925,047,983 safetensor
bytes. Its bounded 1 GiB expert-cache screen made eight scored attempts; all eight truncated with zero
edits and zero verified repairs. Peak server RSS was about 3.22 GiB, wired memory about 11.09 GiB,
compressed memory about 1.99 GiB, minimum free memory about 14 MiB, and every scored run saw warning
pressure. The mixed K8/V3 KV arm failed its warm native-tool gate with an unparsable truncated call.
The two-worker prefetch arm hit the critical memory guard during warmup at about 4.14 GiB server RSS,
12.27 GiB wired memory, 14 MiB minimum free memory, and a swap-out delta of 3,300. Do not spend more
TurboQuant 2 GiB screen budget without a new documented rationale.

The existing local Qwen3.6 GGUF is
`/Users/megasoft78/.anvil-agent/models/Qwen3.6-35B-A3B-UD-IQ2_M.gguf` with 11,522,702,304 bytes.
The approved 8K probe used this model through the current Go CLI and llama.cpp build 10809 at commit
`5266f24da`. Warm smoke made one native tool call, but startup/warmup reached warning pressure and
swap-out growth; the first capability gate was canceled before a scored repair. The full evidence is
in the prepared session's `runs.jsonl`, `memory.jsonl`, and `server.log`. The model's help exposes
Metal, Jinja chat templates, Qwen-compatible XML parsing paths, KV cache types, ngram speculation, and
an OpenAI-compatible server, but those capabilities do not overcome the recorded memory stop.

The local dense Qwen3.8 GSQ-RCO comparator is at
`/Users/megasoft78/Desktop/Freelance/llm-memory-wall-research/models/gguf/qwen3.8-27b-gsq-rco/Qwen3.8-27B-GSQ-RCO-IQ3_XXS.gguf`.
It is 10,094,357,632 bytes with SHA-256
`fdfcb6a29b11188956dfbfd904223588a6c1b77eb250c3e8a36e1bd269df91f7`, and its GGUF header identifies
the `qwen35` architecture. The separate research suite reports strong repair quality but roughly
4.5–4.9 tok/s on this M1; those results used a different harness, so the prepared comparison in
`benchmarks/optimization/experiments-qwen38-local.json` remains pending until the same Go CLI, oracle,
and memory instrumentation produce a result.

## Required execution order

1. Finish static preparation. Keep the corrected gate behavior: capability gates inherit the configured
   prompt profile instead of silently forcing `focused`. Keep the runner’s memory sampler, cancellation
   acknowledgement, per-turn instrumentation, external grading, and resumable `runs.jsonl` state.
2. The existing local GGUF 8K probe has been run and stopped under warning pressure before scoring.
   Preserve the result as the current model/engine feasibility evidence.
3. Only if a separately justified retry or a later candidate produces a native tool call, an applied source edit, and a verified repair with
   no critical pressure, run `LLAMA-IQ2-16K-REAL` at 16K context. Run the complete no-speculation screen in step 4 before considering
   `LLAMA-IQ2-NGRAM-SCREEN`. Change one engine setting at a time.
4. If the existing GGUF path is viable, run the four-task × two-seed screen with the same Go CLI,
   budgets, grading, memory guard, and stable tool schema used by the TurboQuant study. Compare paired
   task outcomes before optimizing speed.
5. If Qwen3.6 remains memory-infeasible or cannot make a verified repair because the model or prompt
   is the binding failure, test one smaller clean candidate at a time. The first prepared fallback is
   the already available local Gemma 4 attnQ4K GGUF in
   `benchmarks/optimization/experiments-gemma4-local.json`. If it fails, test
   `benchmarks/optimization/experiments-qwen35-mlx.json`: Qwen3.5-9B 4-bit MLX-LM at pinned Hugging
   Face revision `938d8919941c6e7efd3c7150eff7fe9d12afa631`. Then test an
   official Gemma 4 E4B or 12B QAT Q4 artifact, with Qwen3.5-4B as a control. Validate
   model/template/engine tool-call compatibility before scoring. Defer Devstral Small 2, GLM-4.7 Flash,
   Qwen3.6 MLX 4-bit and other unprepared large Gemma 4 Q4 artifacts because their published/local
   footprints leave little or no 16 GiB headroom. The already available attnQ4K file is the measured
   no-download exception and must still pass the live memory gate.
   If the Gemma fit probe passes and a same-harness quality ceiling is useful, run the already resident
   dense Qwen3.8 comparator next at 8K with speculation off, then its no-speculation screen, and only
   after that its ngram arm. This is a separate model comparison, not a reason to reopen the failed
   Qwen3.6 context arms.
6. Once a model and engine can complete repairs, test the custom harness factors one at a time:
   stable versus dynamic tool schemas, one oracle-free no-edit retry, bounded read/search results,
   prompt profile, thinking/reasoning preservation, force-edit behavior, close-out reserve, and exact
   edit feedback. Promote only when quality and pressure remain acceptable; a cache or token reduction
   alone is insufficient. The local Gemma configuration already contains the dynamic and no-edit-retry
   fit probes, both gated on a verified stable baseline.
7. If Qwen3.6 quality remains the priority and weight residency or expert I/O is shown to bind, evaluate
   one current-model SSD backend at a time: Hebrus Stable Affine4, Qwisp strict, Qwisp bolt as a paired
   quality-risk arm, then one pinned TurboQuant v0.6.x rescue. Require exact artifact hashes and disk
   space for the model plus build/download cushion. Inside the selected backend, measure reactive exact
   routing, one-layer-ahead cross-layer/history prefetch, spatial-temporal eviction, and profile-guided
   layout in that order. Preserve exact top-8 routing, native tool calls, the same Go CLI/oracle, and
   counters for bytes/read, cache misses, prefetch waste, residual misses, and overlap. Do not add
   expert predictions to the agent context.
8. Test ngram/MTP/speculative decoding only after a viable repair baseline. Record accepted/drafted
   tokens, routed-expert union size, bytes/token, tool-call validity, and extra memory. On a streamed
   MoE, a multi-token verification pass can increase expert I/O even when a resident model would be
   faster. Treat TurboQuant/KV-cache changes as context-memory experiments, not a substitute for a model
   that cannot load or reach an edit.
9. Finish with a fresh confirmation on the winner, a cold-start check, and full regression verification
   outside the model loop. Report uncertainty from the small task sample.

## Research-derived priority

Keep these as separate lanes because a smaller model may maximize repair probability while a streamed
current-model backend may maximize preserved capability.

| Lane | Priority | Rationale and kill rule |
|---|---|---|
| Existing Qwen3.6 UD-IQ2 + llama.cpp | Completed failed fit probe | Local, no download, mandated model. Warm smoke made one native call, then warning pressure and swap-out growth stopped the first gate; do not advance to 16K or ngram. |
| Local Gemma 4 attnQ4K, then Qwen3.5-9B MLX and Gemma 4 E4B QAT Q4 | Fastest repair signal after failure | Gemma is already on disk with prior low-footprint evidence; Qwen3.5 remains the stronger headroom hypothesis. Require immutable provenance and the same Go oracle. |
| Local Qwen3.8 GSQ-RCO IQ3_XXS + llama.cpp | Same-harness quality comparator after Gemma | Already on disk with a recorded 10.09 GB file hash and strong prior dense repair results, but expected to be bandwidth-limited around 4.5–4.9 tok/s. Require a normal-pressure native Go repair before its screen or ngram arm. |
| Hebrus + Qwen3.6 Stable Affine4 | Best current-model SSD candidate | Published 16 GiB M1 Pro zero-swap 8K/16K evidence and OpenAI tool API. Base M1 and Go repair remain unknown; require roughly 20.8 GB artifact plus disk cushion. |
| Qwisp strict then bolt | Best base-M1 SSD comparison | Real base-M1 rows. Strict is exact but slow; bolt is faster but may alter quality. Pair both with identical Go tasks and reject bolt on any verified-repair loss. |
| TurboQuant MLX v0.6.x | One rescue only | New upstream reports exact-output Qwen3.6 streaming, coalesced reads, and parallel prefetch on M4. Local older runtime already failed; retest only with pinned version/artifact change. |
| Stable slots, coalesced reads, bounded prefetch, Least-Stale policy | Reusable engine work | Low semantic risk when the native router remains final. Measure bytes/token and residual misses before learned prediction. |
| N-gram/MTP or learned predictors | Late | N-gram has zero draft weights and hybrid rollback work, but streamed verification can expand the routed-expert union. Learned neuron sparsification changes the model's quality contract and needs training. |

For any streaming candidate, use `T_base ≈ C + B_miss/BW + R·L` and
`T_prefetch ≈ max(C, B_prefetch/BW) + B_residual/BW + R_residual·L + P + W`. Promote only when
the second is lower, exact native routing is preserved, and verified repairs do not fall. Record the
full counters rather than inferring benefit from hit rate.

## Harness requirements

Use `benchmarks/optimization/runner.mjs` and the single Go agent implementation. Do not create a
second prompt or tool implementation. Each session must retain:

- `manifest.json` with git, CLI, model, template, server args, sampling, fixture hashes, budgets,
  hardware, and host baseline;
- `runs.jsonl`, `turns.jsonl`, `memory.jsonl`, and `cache-events.jsonl` with null plus an explanation
  for unavailable measurements;
- per-run prompt/config, stdout/stderr, trace, source patch, oracle report, and server log;
- `DECISIONS.md` with one compact hypothesis/evidence/decision/next-command record per arm.

Keep stable tool schemas stable across turns when measuring that policy. Preserve exact source bytes and
grade only the resulting worktree. Keep hidden tests and oracle bodies outside the model worktree.
The four screening tasks are `permissions-cache`, `job-queue`, `immer-array-push-fix`, and
`zod-int-json-schema`; reserve the remaining prepared tasks and the two completed holdouts for
confirmation according to the handoff.

## Promotion and stopping rules

One failed probe stops that batch, not all investigation of the model. Classify parser, budget,
context, pressure and patch failures first. An 8K success followed by a 16K failure can justify a
separately prepared 8K screen. The revised decision package defines these branches.


Promote a quality candidate only after at least two additional verified task-seed repairs and no
unacceptable pressure. Promote a speed candidate only with no lost verified repairs and either a
paired end-to-end improvement of at least 15%, at least 512 MiB lower measured footprint, or repeatable
inference paging eliminated. Abort on critical pressure or swapout growth in three consecutive samples,
stop before the next attempt, and classify the arm as memory-infeasible/contaminated. Do not continue
an arm after a failed native-tool gate unless a changed parser/template/config is the explicit new
hypothesis.

The practical success criterion is at least one complete verified real-bug repair under normal memory
pressure, followed by a repeatable screen result. If no local candidate reaches that criterion, conclude
that this 16 GiB host is not a viable unattended repair platform for this model class and compare a
larger-memory machine or remote GPU/API rather than spending more time on arbitrary quantization grids.

## Completed first approved batch command

This is the historical command used for the named `LLAMA-IQ2-8K-REAL` batch. Do not rerun it
automatically: the batch stopped under its memory rule before a scored repair, and any retry requires
a fresh rationale and prepared session.

```sh
cd /Users/megasoft78/Desktop/Freelance/anvil-agent
SESSION=/Users/megasoft78/Desktop/Freelance/anvil-agent/.optimization-results/20260914T154835852Z-b401994db269
CLI=/tmp/anvil-agent-opt
node benchmarks/optimization/runner.mjs validate --session "$SESSION" --fetch-reference
node benchmarks/optimization/runner.mjs run --session "$SESSION" --cli "$CLI" --experiment LLAMA-IQ2-8K-REAL --start-server --max-runs 1
node benchmarks/optimization/runner.mjs summarize --session "$SESSION"
```

The server load, warm smoke, memory sampling, and first gate completed. The gate produced no native tool
call under warning pressure, so no scored attempt or external repair grade exists. The 8K profile is
therefore a failed fit probe for the recorded host state; 16K, screen, and ngram remain pending and
closed by the stopping rule.
