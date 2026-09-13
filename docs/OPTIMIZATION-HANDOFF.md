# Qwen optimization execution handoff

Optimize verified coding repairs, elapsed time, and memory on the Apple M1 / 16 GiB Mac using Qwen3.6-35B-A3B. Run the actual Go CLI with native tool calls. The operator requested a plan for Luna Max to execute economically; this document is the execution contract. Read [the evidence appendix](OPTIMIZATION-RESEARCH.md) only when an experiment needs its supporting detail.

## Scope and starting state

Research and hardware inspection: 2026-09-13. Starting code: `08a66de`, plus the removal of the response-only benchmark entry points and this handoff. Record the actual checkout hash when executing. Production inference behavior has not been tuned by this planning change.

All measured workloads must contain real model-selected calls and executed tool results. Use `read`, `search`, `edit`, and `write` through the Go CLI. Keep verification outside the model loop. Do not introduce a shell/test tool, feed hidden oracle results back to Qwen, or use an external coding agent. No raw-response quality tests, standalone text-generation speed tests, perplexity ranking, or canned model replies for performance measurements. Fake endpoints are appropriate only for offline harness tests.

The old benchmark driver, installer benchmark/speed modes, chip speed estimates, and associated issue form have been removed. Scenario inputs, graders, and executable oracles remain. Installer health diagnostics are not benchmark observations. Implement the replacement CLI benchmark before collecting new scores.

Use one local model server and one run at a time. The active model file is `/Users/megasoft78/.anvil-agent/models/Qwen3.6-35B-A3B-UD-IQ2_M.gguf` (11,522,702,304 bytes). Installed llama-server: b10809, commit `5266f24da`. Hardware inspection found macOS 26.6.2, 8 CPU cores, and 1,573 MiB swap already used without a model server. Existing swap occupancy is not proof of current paging; measure changes during each run.

## Deliverable 0: trustworthy measurement

Build the CLI once per harness revision. Add a small local benchmark orchestrator and a machine-readable experiment configuration; avoid duplicating `runAgent` or maintaining a second prompt/tool implementation. Spawn the real CLI for every measured attempt. Expose only the experimental configuration needed below, explicitly recorded in traces. Current CLI cannot override seed, turn limit, prompt profile, or preservation policy; do not invent working flags for them.

Reuse `benchmarks/scenarios`, `benchmarks/grade.mjs`, and the prepared real-repo manifest. Grade actual resulting files in a separate pristine copy. A model's final answer is never its patch. Preserve raw source bytes and record any existing TypeScript import adaptation separately. Do not pass diffs, source-file allowlists, hidden test names, expected fixes, or oracle bodies into blind tasks. Original repository tests may be visible; newly added evaluation tests must stay outside the model's worktree.

Before inference, validate each chosen fixture with its unchanged buggy base, known reference fix, and an invalid/crashing edit. Assert expected named tests actually ran. A crash after one reported failure must not silently pass the remaining tests; exit zero with no expected results must also fail validation. Current `grade.mjs` infers success from absent `FAIL` lines, so strengthen that contract for decision-grade results. Keep source-pattern checks as a separate compatibility score.

The prepared `go-agent/pilot/manifest.json` contains six tasks, but `TestPreparedRealRepoBaselines` still hardcodes date-fns/dayjs keys that are absent. Generalize it to selected manifest tasks before using its instructions. Validate base and reference states; do not trust dependency presence or historical notes. Handle per-task verifier deadlines measured offline; do not run an unrelated 3,000-test suite on every screening attempt when a validated targeted subset suffices. Full regression suites are required for finalist verification.

Persist the following outside model worktrees:

| Artifact | Required contents |
|---|---|
| `manifest.json` | Hardware/OS, git and binary hashes, GGUF hash, template hash, effective server settings, complete sampling values, fixture/oracle hashes, seed, budgets, cold/warm policy, background-process baseline |
| `runs.jsonl` | One row per attempted run, including all timeouts, zero-edit runs, context/turn limits, grading errors, pressure aborts |
| `turns.jsonl` | Request index, tool-schema hash, request bytes/tokens, cache tokens, newly evaluated tokens, prompt/generation ms, tool type/time, reasoning/output tokens when available, edit success, limit/finish reason |
| `memory.jsonl` | Time, server RSS/footprint when available, system wired/compressed/free/cache memory, pressure status, swap used, pageout and swapin/swapout counter deltas |
| Per-run directory | CLI arguments/config, full trace, stderr, source patch, oracle report, server log reference |
| `DECISIONS.md` | Experiment ID, hypothesis, baseline, evidence, keep/drop/inconclusive, next command |

Missing measurements must be `null` with an explanation. `cache_n` and speculative counters need extraction from raw response/server data: current summary drops them. Sample memory every second from before server load through grading, and distinguish startup, inference, and verification peaks. Never add server RSS to system wired memory: these overlap. Keep timestamps for cancellation acknowledgment so the next attempt cannot begin while the server finishes an abandoned request.

## Fixtures and evaluation contract

Screen with four blind tasks: `permissions-cache`, `job-queue`, `immer-array-push-fix`, and `zod-int-json-schema`. The first two have entirely executable bug checks. The last two provide real-repository navigation and source-edit work; validate their current fixtures first.

Use tiny read/edit smoke tasks and `notify-channel` as capability gates. The latter requires creating a file and catches accidental loss of `write`; do not assume it remains a universal pass merely because an old note says so. Add a read-only task to ensure forced-edit policy does not force mutation.

Reserve `zod-memoizer-retention`, `qs-stringify-date-filter`, `zod-discriminated-union-defaulted-tags`, and `immer-array-methods-noop-sharing` for confirmation. Freeze two additional, previously unused real issues before tuning; the existing tasks have already been inspected historically and are confirmation tasks, not a pristine holdout. Check both base failure and reference-fix success for new issues. A title-only recollection probe cannot certify absence of training contamination.

Main budget: 300 seconds of agent time, 16 turns, 3,072 maximum output tokens per request. These are explicit experimental limits, not all current CLI defaults (CLI defaults to 120 seconds / 8,192 output tokens). Verification is separately timed and included in total cost. First collect a small 120-second product-default control, then compare all candidates at the same main budget. Budget expansion is its own experiment.

Use seeds 42 and 31415 for screening, paired across baseline and candidate. Counterbalance execution order, and use a fresh worktree and reproducible cache state for each attempt. For the main comparison, warm resident weights using an unscored tool-calling smoke task, then clear conversation state using a supported, verified slot-cache operation. If the backend cannot clear it reliably, restart consistently and label those trials cold-start. Preserve prefix caching within each task. Record startup separately and add a cold-start product check for finalists. A fixed seed does not ensure identical generation across different server builds, batch shapes, or cache histories.

Primary quality: fraction of attempted tasks with a completed run, actual required source edits, all expected hidden tests passing, no regression, and no forbidden file change. Also report executable bug checks passed and verified partial patches from unfinished runs separately. Keep every timeout in the denominator. The historical 20/44 score includes 24 pattern checks and is not a 45% verified-repair rate.

Primary efficiency: verified tasks per total elapsed hour, including failed attempts and verification. Also report median paired wall-time ratios on tasks both configurations solve, completion/first-edit rates, and peak memory. Do not reward a configuration for finishing incorrect answers sooner. Use task-clustered paired bootstrap intervals for finalist differences; the small sample still requires an explicit uncertainty statement. No claims of population-wide non-inferiority from a handful of tasks.

## Ordered experiments

Treat each row as a hypothesis. Promote only after its gate passes. Keep the original baseline reproducible; record cumulative winners and run a final combined confirmation. Do not run a full Cartesian grid.

| ID | Test and contrast | Evidence to collect / next gate |
|---|---|---|
| E0 | Current CLI baseline after measurement setup | Four screening tasks × two seeds. Confirm useful edits, hidden grading, memory validity, and every setting's effective value. |
| E1 | Explicit `min_p=0` versus current omission; all other settings fixed | Current server default is 0.05 and Qwen recommends 0. Test the actual effective difference, then retain an explicit value in every manifest. If the effective baseline is already 0, skip duplicate inference. |
| E2 | Stable tool definitions versus current dynamic definitions | First inspect real traces and rendered/tokenized prompts around forced-edit, close-out, and repeated-tool events. If schema changes destroy the prefix, compare a stable-schema prompt policy at unchanged budgets. Track extra reads, first edits, cache reuse, and quality together. This is a behavioral change; a cache win alone cannot promote it. |
| E3 | Bounded read results plus compact directory/search output | Only if tool payload growth dominates new prefill. Trial a 4 KiB read-output cap with explicit continuation and exact source preservation; retain the current limit as control. Measure extra round trips and exact-edit failures. Do not truncate tool-call arguments or erase earlier edit evidence. |
| E4 | Memory reduction: context 24,576 → 16,384; separately f16 KV → q8_0 K/V | Inspect actual token occupancy first. Test the context reduction only with safe completion headroom. Trial KV types at original context so the effects are separable; unsupported Metal/model combinations end that arm. Require a long tool-calling task plus short quality controls. |
| E5 | Prompt-processing batch: (b,ub)=(256,256) → (512,256), then (512,512) | Run only if newly evaluated prompt time is material. Same tasks, cache policy and limits; measure inference peak as well as first-turn and later-turn time. Try (1024,512) only after a clear earlier gain with headroom. |
| E6 | Speculation: current ngram-simple versus none; optional ngram-mod | Actual CLI runs with both short read/search calls and substantial edit/write arguments. Record accepted/drafted tokens and total wall time. Skip a wider draft-length sweep without a measured gain. |
| E7 | Bounded thinking with the precise-coding sampling profile, preserving tool-turn reasoning end-to-end | Late quality candidate after cache measurements. Start at 512 reasoning tokens per request, 3,072 total output and 300s total agent budget. Verify actual emitted reasoning and template support. Compare against the winning non-thinking configuration; then ablate preservation only if thinking helps. Account for the coupled mode/sampling change. |
| E8 | MTP, gated by measured generation share and memory | Use the same MTP-equipped artifact with speculation off and on to isolate MTP. Begin draft length 1, then 3 only if useful. Separately compare the artifact with current weights. Verify base tensor provenance, draft activation, cache continuity, and peak memory. |
| E9 | One higher-quality quant of the same model | Only after recovering measured memory headroom: trial UD-Q2_K_XL first, then consider UD-IQ3_XXS. These are alternative quantization recipes, not guaranteed quality improvements. One resident model at a time, fixed sampling/template, full repair grading. |

A speed or memory candidate advances from screening with no observed lost verified tasks and either at least 15% paired end-to-end improvement, 512 MiB lower measured memory footprint, or elimination of repeatable inference paging. A quality candidate advances with at least two additional verified task-seed outcomes and no unacceptable pressure; if latency rises, retain it only as a quality/speed tradeoff for confirmation. These are practical screening thresholds, not statistical guarantees. Repeat borderline results on the same two seeds plus seed 271828 before choosing.

For RAM feasibility, require normal pressure and no increased swap/pageout activity during controlled inference. Record idle counters first. Abort a trial on critical pressure or sustained swapout growth (three successive one-second samples is a proposed guard), preserve evidence, and classify it as memory-infeasible/contaminated rather than silently dropping it. Test the finalist again with the intended everyday applications open; report that result separately. Do not close unrelated apps or change system memory limits automatically.

## Conditional follow-ups, only with evidence

If most failures hit turn 16 with useful progress, compare 16 versus 24 turns at the same 300-second deadline. Raise the byte ceiling only if `context_limit` is actually binding, and add token-aware headroom checks. If the close-out policy prevents correct last edits, test it separately: the present filter can also leave `write` enabled, and it does not replace a singleton `edit` tool list with an empty list. Inspect actual behavior rather than comments.

If dropped parallel tool calls are frequent, evaluate executing independent reads sequentially within one model turn, preserving all call/result pairs; keep mutation order explicit. Do not equate this with enabling parallel model inference. If exact replacements fail often, first improve bounded mismatch feedback and short unique matches; consider another edit representation only as a separate quality experiment.

If memory remains dominated by model weights, avoid elaborate KV tuning for tiny savings. MLX is a later backend experiment only if a compatible same-model quant fits and supports equivalent tool parsing, prompt caching and memory measurement. Different quantization plus backend is a bundled comparison. CPU expert offload on unified memory does not create extra physical RAM.

## Execution budget and stopping rules

First wave: E0, E1, E2, and at most one evidence-selected arm from E3–E6. Four configurations × four tasks × two seeds = at most 32 full screening runs, or 160 minutes at the 300-second cap, plus fixture validation, smoke tests, startup and grading. If E1 is a no-op, its run allocation stays unused. Finish this wave before any large download or backend change.

Second wave: evaluate only the two most promising remaining hypotheses. At most two new arms × four tasks × two seeds = 16 more runs (80 minutes capped inference), reusing an unchanged valid control; rerun controls if the environment changes. Further optional arms require a documented reason and remaining experiment budget, not curiosity alone.

Final confirmation: original baseline and combined winner on the four reserved existing issues plus two new frozen issues, three seeds = 36 runs, at most 180 minutes capped inference. Also rerun all four development tasks and capability gates to detect regressions; allocate up to 24 further runs if using all three seeds. First-wave results can justify stopping with “inconclusive” and keeping the baseline. Do not spend the confirmation budget on a candidate that already fails its gates.

Keep orchestration in scripts. Read compact summary rows, then inspect at most two representative failed traces per hypothesis; expand only for contradictory evidence. Save raw output without pasting it into Luna's conversation. Record a resumable checkpoint after each experiment. Do not call another paid model for routine scoring or repeatedly ask it to reread the repository. A narrow unresolved design question can be escalated with only the relevant evidence, if a permitted model interface exists.

## Completion criteria

Deliver the runner/configuration, validated fixtures, compact result tables, an evidence-linked decision log, and either a measured winning profile or an explicit conclusion that no candidate passed. Update the shipped defaults only for confirmed winners, with regression tests appropriate to the changed behavior. Run Go checks for harness changes, shell/JS checks for installer/runner changes, and configuration-signature synchronization. Commit coherent changes and report their hashes. Preserve rejected experiment records so subsequent sessions do not repeat them.

No performance or quality improvement is claimed by this plan; all numerical thresholds are proposed decision rules.
