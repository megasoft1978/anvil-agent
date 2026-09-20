# Algorithm audit: what the current evidence actually supports

## Outcome

The first problem to solve is a normal-pressure model response fast enough to reach an edit.
The current optimization screen does not justify building a new inference engine or promoting
the agent's fixed read-count interventions. Keep the current model-screen summary and stopping
rules in `LOCAL-MODEL-SUMMARY-2026-09-20.md`. This audit adds source-level findings and a
falsifiable algorithm design; it does not claim a working repair solution.

## Evidence revalidated on September 15

The latest session, `20260915T092827090Z-b401994db269`, is a prepared comparison session with empty
`runs.jsonl`, `turns.jsonl`, and `memory.jsonl`. It is not a new successful run. The two latest
Gemma execution sessions still end at the cache-erasure failure and the warning-pressure gate.

Live read-only inspection found kernel pressure level 2 (warning), 4,719.25 MiB used swap,
152,920 wired pages, and 440,200 compressor pages at 16,384 bytes/page. Browsers and other
applications were running. This is not the controlled terminal-only baseline. Swap occupancy
alone does not demonstrate active paging. No inference was started in this audit.

The authoritative eight-attempt screen is
`.optimization-results/20260913T212402667Z-b401994db269/`. Aggregating its E4 rows in
`turns.jsonl` gives:

| Task | Seed | Recorded requests | Sum of recorded response seconds | Output tokens reported |
|---|---:|---:|---:|---:|
| permissions-cache | 42 | 2 | 228.994 | 102 |
| job-queue | 42 | 2 | 235.691 | 55 |
| immer-array-push-fix | 42 | 2 | 296.303 | 56 |
| zod-int-json-schema | 42 | 2 | 103.995 | 48 |
| zod-int-json-schema | 31415 | 2 | 147.303 | 47 |
| immer-array-push-fix | 31415 | 1 | 192.011 | 28 |
| job-queue | 31415 | 2 | 245.586 | 73 |
| permissions-cache | 31415 | 2 | 222.631 | 76 |

All eight scored rows are truncated, with no verified repairs. The recorded tool calls are reads;
two request rows have no tool type or token usage. Every attempt totals 0–1 ms of reported tool
execution. All prompt/decode timing splits are unavailable. These response sums exclude unreported
request time, startup and grading, so they are not total attempt time or decode speed.
Reported prompt sizes range from 1,147 to 2,872 tokens. There is no evidence here that filling an
8K context or reaching five reads caused the failure. Historical pressure labels must retain their
original provenance; do not relabel the old free-percentage heuristic as a kernel observation.

## Concrete code findings

1. `go-agent/agent.go` checks marshalled request bytes against `MaxHistoryBytes`; the default in
   `main.go` is 64 KiB. This bounds transport/history size, not token count in an 8K context.
   Neither a universal bytes/token ratio nor a byte limit proves that prompt plus generation fits.
   This is a correctness gap in context budgeting, but it is not the demonstrated E4 bottleneck.
2. The dynamic policy counts both reads and searches, then removes all tools except `edit` after
   five calls. It resets on a requested edit before checking whether the edit succeeds. The
   window persists until that reset; it is not a single-turn intervention despite the experiment
   documentation. A `write` does not reset it. Call count is therefore a weak proxy for progress.
3. `GEMMA4-LOCAL-DYNAMIC-8K-REAL` originally changed both tool-schema policy and
   `close_out_reserve` relative to its stable baseline. The September 15 follow-up corrected the
   source configuration to disable close-out in both arms. A Node assertion verified matching
   model/server configuration, tasks, seeds and budgets, and that the effective agent settings
   differ only in `tool_schema_policy` after applying the existing defaults. This tests the entire
   dynamic policy, including repeated-call bans; it does not isolate forcing an edit. Existing
   sessions remain immutable and a fresh session is required.
4. Read deduplication already rereads source and compares exact result bytes before replacing a
   duplicate result with a reference. Adding a second read cache would duplicate existing work.
   Any future history eviction must also invalidate references to evicted tool results.
5. `search.go` bounds matching output and files scanned, but `walk` continues traversing after the
   output is marked truncated; directory enumeration uses `ReadDir(-1)`. Thus bounded result size
   does not imply bounded directory traversal. This is a real large-repository concern, but the
   measured E4 tool times rule it out as the useful first performance target for that screen.

Production agent behavior is unchanged. The deferred experiment configuration, its description,
and runner cache-counter extraction were corrected; no live benefit has been measured.

## Follow-up: a concrete repeated-prefill cost

The frozen comparison manifest and `cache-events.jsonl` explicitly record `--prompt-cache-size 0`,
`--prompt-cache-bytes 0`, and no retained conversation cache. The current Qwen3.5 MLX experiment
file also sets `preserve_prefix_cache_within_task: false`. Consequently, stable
schemas do not by themselves buy prefix reuse in these configurations. Disabling cross-task
reuse through a global cache-off setting also disables useful within-task reuse.

The remaining `server.log` belongs to the later `prefetch-2` invocation, as confirmed by the
manifest's runtime profile and its 2 GB startup line. Its first prompt progresses from 0/942 at
07:34:10.084 to 942/942 at 07:35:15.598: 65.514 seconds, approximately 14.38 prompt tokens/s
over the logged interval. This is a warmup observation in the pressure-aborted E6 arm, not an E4
prefill measurement, and cannot supply the E4 timing split. Earlier E4 server-side timings remain
unavailable; the current runner appends logs but cannot recover historical missing content.

For a future memory-viable MLX baseline, test task-scoped bounded prefix reuse before inventing a
learned predictor. Retain cache within one task and clear it at the task boundary. If the pinned
engine lacks a supported clear operation, a managed restart per attempt is an explicit alternative;
compare both policies with identical restart boundaries and include startup in operational yield.
Do not switch an existing screen to a shared cache silently.

The pinned [MLX-LM 0.31.2 server source](https://raw.githubusercontent.com/ml-explore/mlx-lm/v0.31.2/mlx_lm/server.py)
returns reused prompt tokens in `usage.prompt_tokens_details.cached_tokens`. Its
[cache implementation](https://raw.githubusercontent.com/ml-explore/mlx-lm/v0.31.2/mlx_lm/models/cache.py)
enforces entry and byte limits and reuses token prefixes; trimming a longer cache requires that the
cache type support trimming. Thus hybrid-state rollback and the rendered tool-call token prefix
must be checked, not assumed. These upstream sources do not prove the retired runtime's exact
behavior or establish any local speedup.

Source inspection found that the runner ignored the MLX usage counter despite reading its parent
object. `cache-metrics.mjs` now resolves the llama timing aliases and the MLX usage field, preserves
measured zero, rejects malformed counts, and drives the missing-measurement label consistently.
The runner's preparation input lock includes this helper. Three model-free Node tests passed,
covering MLX hits/zero, timing precedence and aliases, and absent/invalid counters. Runner syntax
and `git diff --check` also passed. Old trace rows and summaries were not rewritten. The full
runner integration suite and live MLX cache behavior remain unverified in this follow-up.

A cache arm must measure retained and active cache footprint, reused tokens per request, normal
memory pressure, repair yield and total elapsed time. Its break-even condition is
`saved_prefill_time > cache_lookup_copy_time + added_paging_time + amortized_reset_time`.
The first request cannot benefit from within-task reuse, so this alone cannot solve slow cold
prefill. A retained cache that increases paging can lose even when it saves many prompt tokens.

The live follow-up still found warning pressure (level 2), with 4,393.25 MiB occupied swap.
The prior temporary runner and server artifacts were absent. Rebuild/reprepare the selected Gemma
path when its preflight can pass; do not interpret vanished temporary artifacts as a model failure
or automatically reinstall a retired comparison arm.

## Mathematical decision model

Measure operational yield as `3600 * verified normal-pressure repairs / total elapsed seconds`.
Keep quality, execution status, memory validity and infrastructure failures separately visible.
Do not turn a missing timing counter into zero or discard an unsuccessful attempt.

For a task, use the accounting identity:

`T = T_start + sum(T_prefill + T_decode + T_tool + T_protocol) + T_grade`.

Amdahl's bound for accelerating fraction f by factor s is
`S <= 1 / (1 - f + f/s)`, before new overhead. The observed tool work is negligible relative to
recorded response time. Faster source scanning cannot rescue this E4 result. Missing timing splits
prevent assigning the response bottleneck to prefill, decode, cache misses, or storage.

For append-only histories growing by d tokens each turn from P initial tokens, n requests without
prefix reuse process approximately `n*P + d*n*(n-1)/2` prompt tokens. Ideal prefix reuse processes
approximately `P + (n-1)*d` new prompt tokens. These are token-work bounds, not latency predictions:
decode still attends to retained context, template changes can invalidate a prefix, and actual
cache behavior requires counters. This supports measuring stable schemas before more elaborate
history rewriting.

### Candidate algorithm: evidence selection under a token budget

After a viable model baseline, represent observed source as exact spans carrying path, source
version/hash, line interval, token cost, and retrieval provenance. Preserve the task, tool schema,
recent complete tool-call/result pairs, and exact source needed for the next proposed edit.

For optional spans S, define coverage utility
`U(S) = sum_j w_j * min(1, sum_(i in S) a_ij)`.
Here j indexes task identifiers/symbols and source relationships visible to the agent; a_ij is a
nonnegative observed coverage weight. Hidden tests and reference patches must never contribute.
The capped sum penalizes redundant spans. Select spans by marginal utility per token until the
remaining budget is exhausted, subject to mandatory spans and complete message-pair constraints.
This greedy heuristic is cheap; do not claim the cardinality-greedy 1-1/e guarantee for this
knapsack problem with mandatory groups and template/cache costs.

The actual constraint is `template_tokens + selected_tokens + generation_reserve <= context_limit`.
Use the backend's exact tokenizer/template path when available. Otherwise record an explicitly
estimated budget and retain server-side overflow handling; a byte heuristic is not exact admission.
Do not silently trim the current transcript. An experimental compaction policy must log removed
spans, preserve valid tool-message structure, and make evicted evidence retrievable again.

Eviction also loses prefix reuse. Compact only when required for fit or when estimated future
prefill/context savings exceed the rebuild cost; log both estimates and measured outcomes.
Longer context alone does not guarantee better evidence use: the
[Lost in the Middle study](https://arxiv.org/abs/2307.03172) demonstrates positional sensitivity
on retrieval and QA. That motivates testing evidence selection; it does not establish coding gains.

### Candidate algorithm: exact expert residency and prefetch

Use this branch only when a chosen streaming backend exposes routed expert IDs, logical bytes,
physical reads, cache state and timings. For an expert e, a useful admission score is
`(p_e * stall_saved_e - fetch_cost_e - eviction_cost_e) / resident_bytes_e`.
All terms in the numerator are expected time, evaluated over a declared horizon. Fetch cost must
include false positives and contention; stall saved must account for existing prefetch overlap.
Preserve the native router and fetch any missed required expert before computation. Prediction
controls when bytes arrive, never which experts contribute to the result.

Before implementing a predictor, replay development routing traces to compare reactive LRU with an
offline future-aware cache reference. For equal-size objects and request-count minimization, Belady
is the appropriate ideal reference; variable byte sizes and overlapped latency need a different
cost model. If the achievable residual-miss reduction is small, reject predictor engineering.
An ideal-cache replay is a bound on its declared metric, not measured SSD throughput.

[LLM in a flash](https://arxiv.org/abs/2312.11514) supports accounting for transferred bytes and
contiguous reads, using activation reuse and row/column bundling. Its reported acceleration is
against its own naive-loading baselines. It does not establish speed or exact applicability for
this quantized MoE, this engine, or this Go agent.

## Next evidence and completion criteria

The next live action remains a fresh Gemma fit session after the documented clean-host preflight.
The existing memory guard correctly prohibits the current warning baseline. Preparation is allowed;
repeated model starts under unchanged warning pressure add no useful feasibility evidence.

Before a dynamic-policy comparison, prepare an isolated schema arm with close-out disabled and
describe its actual persistent edit-only behavior. After a normal-pressure repair, collect the
complete screen and timing coverage, then choose context selection or engine work from measured
costs. Validate a selected change on unused tasks and a cold start, retaining all failures.

The goal remains incomplete: no current-stack normal-pressure real-repair baseline, repeatable
screen winner, fresh holdout confirmation, or verified algorithmic improvement is established.
This audit makes progress by ruling out five-read intervention and source-search acceleration as
explanations for the recorded E4 failure, exposing a confounded arm, and specifying what evidence
would justify each algorithm family.
