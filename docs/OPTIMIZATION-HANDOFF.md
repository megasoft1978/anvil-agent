# Qwen optimization execution handoff

> Review precedence: [the revised decision package](OPTIMIZATION-ASTRA-DECISION.md#reviewed-decision-and-precedence--2026-09-14) controls priorities and interpretation where this document differs. The first live probe is recorded below; later model-dependent validation remains pending.

Optimize verified coding repairs, elapsed time, and memory on the Apple M1 / 16 GiB Mac using Qwen3.6-35B-A3B. Run the actual Go CLI with native tool calls. The operator requested a plan for Luna Max to execute economically; this document is the execution contract. Read [the evidence appendix](OPTIMIZATION-RESEARCH.md) only when an experiment needs its supporting detail.

## Scope and starting state

Research and hardware inspection: 2026-09-13. Starting code: `08a66de`, plus the removal of the response-only benchmark entry points and this handoff. Record the actual checkout hash when executing. Production inference behavior has not been tuned by this planning change.

All measured workloads must contain real model-selected calls and executed tool results. Use `read`, `search`, `edit`, and `write` through the Go CLI. Keep verification outside the model loop. Do not introduce a shell/test tool, feed hidden oracle results back to Qwen, or use an external coding agent. No raw-response quality tests, standalone text-generation speed tests, perplexity ranking, or canned model replies for performance measurements. Fake endpoints are appropriate only for offline harness tests.

The old benchmark driver, installer benchmark/speed modes, chip speed estimates, and associated issue form have been removed. Scenario inputs, graders, and executable oracles remain. Installer health diagnostics are not benchmark observations. Implement the replacement CLI benchmark before collecting new scores.

Use one local model server and one run at a time. The historical Deliverable 0 TurboQuant configuration
was screened and is closed after its recorded zero-repair and critical-pressure results. The existing
local Qwen3.6 GGUF was then tested through the pinned llama.cpp configuration in
`benchmarks/optimization/experiments-llama-qwen36.json`; its 8K fit probe is session
`20260914T154835852Z-b401994db269`. Warm smoke made one native call, but the kernel entered warning
pressure and swap-out growth stopped the first capability gate before a scored repair. Do not advance
to 16K, the no-speculation screen, or ngram from that session. The capability gates inherit the
configured agent prompt profile so gate behavior matches scored attempts. Hardware inspection found
macOS 26.6.2 and 8 CPU cores. Existing swap occupancy is not proof of current paging; the live session
recorded changes in paging counters. Predictive SSD streaming is a later engine branch.

The current hardware/model review is [OPTIMIZATION-RESEARCH-2026-09-14.md](OPTIMIZATION-RESEARCH-2026-09-14.md).
It preserves the existing Qwen3.6 measurement contract, records the completed local GGUF/llama.cpp 8K
probe, and makes the already available Gemma 4 attnQ4K file the fastest next no-download path through
the same Go CLI. If that probe cannot complete a verified repair, use the prepared Qwen3.5/Gemma
fallbacks for a later same-Go-CLI comparison. The research review now separates that
smaller-model repair lane from a current-model SSD lane: evaluate Hebrus Stable Affine4 first, then
Qwisp strict/bolt, with one pinned TurboQuant v0.6.x rescue only after a version change justifies it.
These later candidates need roughly 20 GB artifacts and have no base-M1 Go-agent repair result. The
smaller-model branch is now prepared in
`benchmarks/optimization/experiments-qwen35-mlx.json`: it uses the installed MLX-LM 0.31.2 server,
the same Go CLI/oracle, one serialized request, and a disabled prompt cache. The Qwen3.5-9B tree is
still absent because the current sandbox's Hugging Face DNS request failed before writing files. Do
not load that candidate until its exact local tree hash and a fresh experiment session are recorded.
An already downloaded Gemma 4 attnQ4K GGUF is also prepared in
`benchmarks/optimization/experiments-gemma4-local.json` for the fastest next no-download signal; its
recorded file hash is checked during preparation and execution. Do not load a candidate until its
exact local hash and a fresh experiment session are recorded.
The already resident dense Qwen3.8 GSQ-RCO IQ3_XXS comparator is prepared separately in
`benchmarks/optimization/experiments-qwen38-local.json`; it uses an 8K no-speculation fit probe,
then a no-speculation screen and ngram arm only after the preceding normal-pressure evidence passes.
Its 10,094,357,632-byte file is locked by SHA-256
`fdfcb6a29b11188956dfbfd904223588a6c1b77eb250c3e8a36e1bd269df91f7`.

## Operator preflight for the 16 GiB Mac mini

The controlled memory screen assumes an almost empty host. Reboot the Mac mini immediately before a
memory-sensitive batch so stale swap and compressed-memory state are cleared. After login, close every
other application and keep only the terminal running the documented command; in particular, do not leave
a browser, IDE, desktop client, or other memory-heavy helper open. Record the idle memory and swap
baseline after this cleanup. The runner never closes applications, changes memory settings, or treats a
successful server health check as proof that inference is safe.

This is a controlled feasibility condition for the 16 GiB host. Rebooting and closing applications can
recover headroom but cannot reduce the model's resident weight allocation. If pressure is already
abnormal or swap is increasing at idle, stop before loading the model. Only after a candidate passes the
controlled screen should a separate finalist check measure behavior with the intended everyday
applications open.

The managed runner also needs a shell that can bind its loopback HTTP port and inspect the relevant host
memory/process counters. A restricted Codex sandbox may deny the loopback bind or return `EPERM` for
`ps`, `pgrep`, and kernel memory sysctls; preparation and fixture validation can still complete, but that
environment cannot produce a live model measurement. In that case use the exact prepared-session command
from a normal local Terminal on the Mac. If only server startup is blocked and the shell can connect to
the listener, the runner supports an explicit `--external-server --server-pid PID` mode: start the exact
documented server profile in a normal Terminal, pass its PID, and leave server ownership there. The
runner still samples the PID, checks health and effective settings, applies the same gates and memory
stopping rules, and never stops the external process. It records any remaining unavailable measurements
and does not treat a startup bind failure as model evidence. Because the runner attaches after external
model loading begins, this fallback cannot capture the weight-allocation peak; use managed startup from
a normal Terminal, or a separate startup preflight, for the authoritative memory-feasibility result.

A server health response, warm smoke, or one-file capability task does not establish useful real-bug
performance. The target workload needs enough context for repository navigation, source evidence,
multiple native tool results, and a complete repair loop. A memory profile that only fits by making that
context unavailable does not satisfy this handoff.

Disk space is a separate preflight. Check the data volume and the largest local directories before a
large preparation or model download:

```sh
df -h /System/Volumes/Data
du -sh ~/.cache/huggingface/hub/* ~/.anvil-agent/models/* .optimization-results/* 2>/dev/null | sort -h
```

The unrelated Hugging Face FLUX cache was removed during preparation, recovering about 15 GiB. The
latest post-cleanup snapshot reports about 32 GiB available at 93% data-volume capacity. The remaining reclaim options include regenerable
dependency trees under the separate research result worktrees, old optimization sessions after their
summaries are retained, and user caches after their owning applications are stopped. The legacy 11 GiB
GGUF can be removed only if llama.cpp reproduction is abandoned.
Keep the TurboQuant model, the GGUF until its probe is complete, the isolated runtimes, prepared
pilot/holdout fixtures, and rejected result records until the experiment report is complete. Removing
disk caches does not reduce the model's resident RAM allocation.

## Deliverable 0: trustworthy measurement

Build the CLI once per harness revision. Add a small local benchmark orchestrator and a machine-readable experiment configuration; avoid duplicating `runAgent` or maintaining a second prompt/tool implementation. Spawn the real CLI for every measured attempt. Keep supported experiment overrides in the per-attempt configuration file and record them in traces; do not invent command-line flags or unrecorded behavior for an experiment.

Reuse `benchmarks/scenarios`, `benchmarks/grade.mjs`, and the prepared real-repo manifest. Grade actual resulting files in a separate pristine copy. A model's final answer is never its patch. Preserve raw source bytes and record any existing TypeScript import adaptation separately. Do not pass diffs, source-file allowlists, hidden test names, expected fixes, or oracle bodies into blind tasks. Original repository tests may be visible; newly added evaluation tests must stay outside the model's worktree.

Before inference, validate each chosen fixture with its unchanged buggy base, known reference fix, and an invalid/crashing edit. Assert expected named tests actually ran. A crash after one reported failure must not silently pass the remaining tests; exit zero with no expected results must also fail validation. Current `grade.mjs` infers success from absent `FAIL` lines, so strengthen that contract for decision-grade results. Keep source-pattern checks as a separate compatibility score.

The prepared `go-agent/pilot/manifest.json` contains six tasks. `TestPreparedRealRepoBaselines` now
selects tasks from the manifest and honors each task's declared source files and test command, while
retaining legacy date-fns/dayjs compatibility. Validate base and reference states; do not trust dependency
presence or historical notes. Handle per-task verifier deadlines measured offline; do not run an unrelated
3,000-test suite on every screening attempt when a validated targeted subset suffices. Full regression
suites are required for finalist verification.

Persist the following outside model worktrees:

| Artifact | Required contents |
|---|---|
| `manifest.json` | Hardware/OS, git and binary hashes, model file/tree hash, template hash, effective server settings, complete sampling values, fixture/oracle hashes, seed, budgets, cold/warm policy, background-process baseline |
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
| E4 | TurboQuant memory reduction: expert cache 2 GB → 1 GB | Run only after the baseline establishes useful edits and memory evidence. Measure peak RSS, wired/compressed memory, swap counters, completion, and verified quality together. |
| E5 | TurboQuant KV compression: mixed K8/V3 at the original 2 GB expert cache | Confirm the runtime accepts the configured KV types, then compare request-state memory, prompt/generation time, completion, and verified quality. Unsupported combinations end the arm. |
| E6 | TurboQuant expert prefetch: one worker → two workers | Compare streamed-expert latency and memory pressure with the same model, cache budget, active-expert cap, prompt policy, and task limits. |
| E7 | Bounded thinking with the precise-coding sampling profile, preserving tool-turn reasoning end-to-end | Late quality candidate after cache measurements. Start at 512 reasoning tokens per request, 3,072 total output and 300s total agent budget. Verify actual emitted reasoning and template support. Compare against the winning non-thinking configuration; then ablate preservation only if thinking helps. Account for the coupled mode/sampling change. |
| E8 | MTP, gated by measured generation share and memory | Use the same MTP-equipped artifact with speculation off and on to isolate MTP. Begin draft length 1, then 3 only if useful. Separately compare the artifact with current weights. Verify base tensor provenance, draft activation, cache continuity, and peak memory. |
| E9 | One higher-quality quant of the same model | Only after recovering measured memory headroom: trial UD-Q2_K_XL first, then consider UD-IQ3_XXS. These are alternative quantization recipes, not guaranteed quality improvements. One resident model at a time, fixed sampling/template, full repair grading. |

The earlier TurboQuant-MLX feasibility trial is recorded in the research appendix. It was unscored:
the 7.8 GB automatic expert-cache launch paged critically, while a 2 GB cache completed one
disposable Go CLI read/edit loop in 278.645 seconds with no additional swapouts in the manual
snapshots. The configuration remains available as a historical, explicitly selectable experiment;
it is closed for the current host after the later pressure and zero-repair results. The earlier smoke
does not establish real-bug quality or replace the evidence collected by a current baseline.
The full 1 GB rescue screen then ran the four screening tasks at seeds 42 and 31415. All eight scored
attempts ended truncated with zero edits and zero verified repairs; CLI time ranged from 192.081 to
300.034 seconds with a 245.656-second median. Peak server RSS was about 3.22 GiB, wired memory about
11.09 GiB, compressed memory about 1.99 GiB, minimum free memory about 14 MiB, pageout delta 4,763,
swap-in delta 112, and swap-out delta 0. Every scored attempt recorded warning pressure at least once.
Treat E4 as an unpromoted, complete low-cache screen: it did not establish useful real-bug performance
or satisfy the normal-pressure rule. Keep E1, E2, E3, and TQ-full pending unless a separately
documented rationale selects another arm after a fresh memory preflight.

The subsequent E5 profile accepted mixed K8/V3 KV compression but failed its warm tool-call gate with
an unparsable truncated tool call, so it produced no scored task. E6 increased prefetch workers to two
and was canceled by the critical-memory guard during warmup: peak server RSS was about 4.14 GiB, wired
memory 12.27 GiB, minimum free memory 14 MiB, and swap-out delta 3,300. Do not run TQ-full or another
2 GB profile on the same host state; require a smaller resident model, more physical memory, or a
separately documented profile change before continuing.

A speed or memory candidate advances from screening with no observed lost verified tasks and either at least 15% paired end-to-end improvement, 512 MiB lower measured memory footprint, or elimination of repeatable inference paging. A quality candidate advances with at least two additional verified task-seed outcomes and no unacceptable pressure; if latency rises, retain it only as a quality/speed tradeoff for confirmation. These are practical screening thresholds, not statistical guarantees. Repeat borderline results on the same two seeds plus seed 271828 before choosing.

For RAM feasibility, require normal pressure and no increased swap/pageout activity during controlled inference. Apply the operator preflight above and record idle counters first. Abort a trial on critical pressure or sustained swapout growth (three successive one-second samples is a proposed guard), preserve evidence, and classify it as memory-infeasible/contaminated rather than silently dropping it. After a candidate passes the controlled screen, test the finalist again with the intended everyday applications open and report that result separately. The runner must never close unrelated apps or change system memory limits automatically.

The earlier Gemma 4 project found a large memory reduction by passing `--ctx-checkpoints 0
--cache-ram 0`, with a smaller `-ub 256 -b 256` buffer. Do not copy that configuration into the Qwen
baseline: the Qwen setup previously showed full prompt-prefix-cache discards with those flags disabled,
which increased re-prefill time and reduced completed tool turns. If a Qwen memory-first profile is
needed, treat it as a new arm and require cache reuse, prefill, quality, completion, and pressure
evidence together. Other possible arms are smaller batch buffers, no n-gram speculation, q8 KV types,
or a shorter context; each changes a different allocation and must stay separately labeled. Read/search
caps only limit later conversation growth and cannot repair a startup allocation failure.

## Conditional follow-ups, only with evidence

If most failures hit turn 16 with useful progress, compare 16 versus 24 turns at the same 300-second deadline. Raise the byte ceiling only if `context_limit` is actually binding, and add token-aware headroom checks. If the close-out policy prevents correct last edits, test it separately: the present filter can also leave `write` enabled, and it does not replace a singleton `edit` tool list with an empty list. Inspect actual behavior rather than comments.

The current Go agent also supports one opt-in `retry_without_edit` turn. It appends an ordinary user
correction after a completed response that changed no file, without exposing grader output or hidden
tests. Use it only as a separate harness arm after a viable baseline; read-only gates and baseline
measurements keep it disabled. The Gemma configuration includes this arm alongside a separate dynamic
tool-schema/forced-edit arm because prior real-repository evidence showed diagnosis-without-edit and
repeated-read stalls as distinct failure classes.

If dropped parallel tool calls are frequent, evaluate executing independent reads sequentially within one model turn, preserving all call/result pairs; keep mutation order explicit. Do not equate this with enabling parallel model inference. If exact replacements fail often, first improve bounded mismatch feedback and short unique matches; consider another edit representation only as a separate quality experiment.

If memory remains dominated by model weights, avoid elaborate KV tuning for tiny savings. The first
smaller-model MLX comparison is now wired, but it remains a bundled model/quantization/backend
hypothesis and must prove native tool parsing, a verified repair, and normal pressure before any
screen. CPU expert offload on unified memory does not create extra physical RAM.

## Execution budget and stopping rules

First wave: E0, E1, E2, and at most one evidence-selected arm from E3–E6. Four configurations × four tasks × two seeds = at most 32 full screening runs, or 160 minutes at the 300-second cap, plus fixture validation, smoke tests, startup and grading. If E1 is a no-op, its run allocation stays unused. Finish this wave before any large download or backend change.

Second wave: evaluate only the two most promising remaining hypotheses. At most two new arms × four tasks × two seeds = 16 more runs (80 minutes capped inference), reusing an unchanged valid control; rerun controls if the environment changes. Further optional arms require a documented reason and remaining experiment budget, not curiosity alone.

Final confirmation: original baseline and combined winner on the four reserved existing issues plus two new frozen issues, three seeds = 36 runs, at most 180 minutes capped inference. Also rerun all four development tasks and capability gates to detect regressions; allocate up to 24 further runs if using all three seeds. First-wave results can justify stopping with “inconclusive” and keeping the baseline. Do not spend the confirmation budget on a candidate that already fails its gates.

Keep orchestration in scripts. Read compact summary rows, then inspect at most two representative failed traces per hypothesis; expand only for contradictory evidence. Save raw output without pasting it into Luna's conversation. Record a resumable checkpoint after each experiment. Do not call another paid model for routine scoring or repeatedly ask it to reread the repository. A narrow unresolved design question can be escalated with only the relevant evidence, if a permitted model interface exists.

## Completion criteria

Deliver the runner/configuration, validated fixtures, compact result tables, an evidence-linked decision log, and either a measured winning profile or an explicit conclusion that no candidate passed. Update the shipped defaults only for confirmed winners, with regression tests appropriate to the changed behavior. Run Go checks for harness changes, shell/JS checks for installer/runner changes, and configuration-signature synchronization. Commit coherent changes and report their hashes. Preserve rejected experiment records so subsequent sessions do not repeat them.

No performance or quality improvement is claimed by this plan; all numerical thresholds are proposed decision rules.
