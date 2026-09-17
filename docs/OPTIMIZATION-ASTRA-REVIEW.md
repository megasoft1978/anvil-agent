# Astra review brief: local coding-agent feasibility and next experiments

> Review precedence: [the revised decision package](OPTIMIZATION-ASTRA-DECISION.md#reviewed-decision-and-precedence--2026-09-14) controls priorities and interpretation where this document differs. The first live probe is recorded below; later model-dependent validation remains pending.

Prepared 2026-09-14 for review by GPT Astra. This brief is intentionally compact: it records the
decision-relevant evidence and links to raw artifacts instead of embedding traces. The original
execution contract remains [OPTIMIZATION-HANDOFF.md](OPTIMIZATION-HANDOFF.md), and the paste-ready
Codex objective is [OPTIMIZATION-CODEX-GOAL.md](OPTIMIZATION-CODEX-GOAL.md).

The current source and model review is consolidated in
[OPTIMIZATION-RESEARCH-2026-09-14.md](OPTIMIZATION-RESEARCH-2026-09-14.md). It adds the current
Qwen3.6 llama.cpp parser reports, MLX parser/cache risks, official Gemma QAT options, and the
hardware-ranked fallback order. It now also covers current Qwen3.6 SSD engines, stable slot banks,
predictive I/O, and the distinction between a high-probability smaller-model repair and a
current-model backend. Use the revised decision package for the authoritative priority.

## Decision requested

Choose the next one-at-a-time experiment path for an Apple M1 Mac with 16 GiB unified memory, with the
goal of solving real repository bugs through the existing Go agent harness. The existing Qwen3.6 GGUF
plus llama.cpp has now entered warning pressure and rising swap before its first scored repair. The next
choice is one smaller clean model through the same harness, or a separately prepared SSD-residency path
when preserving Qwen3.6 is the priority. Decide which harness changes should be tested after a viable
model is found.

## Recommended decision now

Stop the current TurboQuant tuning branch. Its 1 GiB screen produced no edit in eight scored attempts,
and its 2 GiB KV/prefetch arms failed before scoring under warning or critical memory pressure. The
existing 10.73 GiB Qwen3.6 UD-IQ2 GGUF with llama.cpp was then tested at 8K: warm smoke made one native
tool call, but startup reached warning pressure and swap-out growth, so the first capability gate was
canceled before scoring. Do not advance to 16K, the screen, or ngram from that result.

Choose between two evidence-driven paths. The fastest available repair hypothesis is the local
Gemma 4 26B-A4B attnQ4K GGUF: it needs no download and has prior low-footprint llama.cpp evidence,
while the current Go harness must independently test real-repository repair. If that probe fails,
use Qwen3.5-9B 4-bit MLX, then Gemma 4 E4B or 12B QAT Q4, because their
footprints leave materially more headroom. The current-model path is Hebrus's
Qwen3.6 Stable Affine4 runtime, followed by a paired Qwisp strict/bolt comparison. Both have direct
Qwen3.6 SSD evidence on related Apple hardware, but neither has a base-M1 Go-agent repair result and
both require a roughly 20 GB artifact. A newer TurboQuant v0.6.x path is a single versioned rescue
hypothesis after those options; the local older runtime remains closed.

The new SSD research supports a later engine branch. Stable slot banks, coalesced `pread`, bounded
prefetch windows, and spatial-temporal eviction are the first reusable layers. ST-MoE, SpecPrefetch,
and related work show how cross-layer and consecutive-token routing history can predict future expert
requests while retaining the native router as final authority. MAIO/PPC and LLM in a Flash motivate
access templates and contiguous reads. These sources do not supply a measured native Go-agent repair
on this M1. The no-download GGUF probe has now reached warning pressure and swap-out growth before a
repair, so compare one smaller repair model or one streaming backend only through a fresh prepared
session and a named batch selection. Preserve exact native top-8 routing and measure
bytes/read, merged-read size, cache behavior, residual misses, prediction waste, overlap, and verified
repairs.

Upstream reports make the Qwen3.6 parser gate especially important: [llama.cpp issue #26043](https://github.com/ggml-org/llama.cpp/issues/26043)
records raw Qwen3.6 XML being returned as content, while [#24807](https://github.com/ggml-org/llama.cpp/issues/24807)
and [#26763](https://github.com/ggml-org/llama.cpp/issues/26763) record malformed or suffix-sensitive
parameter parsing. These reports lower confidence in the engine path; they do not replace the local
probe. The Go agent now has strict Qwen XML recovery for diagnostics, but recovered calls are labeled
separately and cannot satisfy a native-tool gate.

## Local evidence

| Arm | What happened | Decision value |
|---|---|---|
| TurboQuant automatic cache | Automatic selection was about 7.8 GiB and critically paged on a 949-token request without tools. | The automatic fit policy is unsuitable for this host. |
| TurboQuant 2 GiB smoke | One disposable Go CLI read/edit loop completed in 278.645 s; it was unscored and does not prove a real repair. | Shows the runtime can sometimes make a tool call, but not useful quality. |
| TurboQuant E4, 1 GiB cache | 8/8 scored attempts across four tasks and two seeds ended truncated, with zero edits and zero verified repairs. Median CLI time was about 245.656 s. Peak server RSS was about 3.22 GiB; wired memory about 11.09 GiB; compressed memory about 1.99 GiB; minimum free memory about 14 MiB; pageout delta 4,763; every scored attempt saw warning pressure. | Complete low-cache screen; no promotion. |
| TurboQuant E5, mixed K8/V3 KV | Flags were accepted, but warmup ended `empty_completion` after 88.382 s with zero tool calls. The server log shows a truncated/unparsable tool call. | No quality or memory promotion. |
| TurboQuant E6, two prefetch workers | Warmup was canceled by the critical-memory guard after 100.561 s. Peak server RSS was about 4.14 GiB; wired memory about 12.27 GiB; compressed memory about 2.04 GiB; minimum free memory about 14 MiB; swap-out delta 3,300. | 2 GiB profile is memory-infeasible in the recorded host state. |
| TurboQuant syntax-greedy probe | Warm smoke made one valid native tool call in about 180 s. The focused read-only gate then produced no parsed tool call in about 91.6 s, so no scored task ran. | The installed decoder scans JSON structure, while Qwen3.6 emits XML inner function/parameter tags; treat this as a runtime/template mismatch. |
| Existing llama.cpp GGUF, 8K probe | Warm smoke made one native call in 15.223 s. Startup/warmup reached 6.80 GiB server RSS, 13.06 GiB wired memory, 55.1 MiB minimum free memory, warning pressure, and swap-out delta 30,716. The first gate was canceled with zero calls; no scored repair ran. | Memory-infeasible/contaminated for the recorded host state. Preserve evidence and move to a smaller model or a separately selected SSD-residency path. |

Full TurboQuant records are in:

- [main TurboQuant decisions](../.optimization-results/20260913T212402667Z-b401994db269/DECISIONS.md);
- [TurboQuant E4/E5/E6 runs](../.optimization-results/20260913T212402667Z-b401994db269/runs.jsonl);
- [syntax-greedy probe decisions](../.optimization-results/20260914T054218573Z-b401994db269/DECISIONS.md);
- [syntax-greedy probe runs](../.optimization-results/20260914T054218573Z-b401994db269/runs.jsonl).

The historical score of 20/44 is not a 45% verified-repair rate: it combines 20 executable checks
with 24 source-pattern checks. The new runner grades resulting files and keeps unfinished and zero-edit
attempts in the denominator.

## Current host and disk state

Read-only inspection before the live probe reported 16 GiB physical memory. During the probe, the
kernel pressure level changed to warning while the model loaded; the sampler recorded 30,716 swap-outs,
138 swap-ins, and 55.1 MiB minimum free memory before the gate stopped. The server is no longer
running. After the FLUX cache cleanup, the data volume is 460 GiB total and the latest post-run snapshot
showed about 32 GiB available; a controlled future batch still requires a reboot and all other
applications closed, with idle pressure and swap recorded afterward.

The largest identified items are:

| Path | Size | Current handling |
|---|---:|---|
| `~/.anvil-agent/models/Qwen3.6-35B-A3B-tq3-g32` | ~16 GiB | Historical failed TurboQuant arm; retain until its evidence is archived and the branch is explicitly closed. |
| `~/.anvil-agent/models/Qwen3.6-35B-A3B-UD-IQ2_M.gguf` | ~11 GiB | Keep for the next llama.cpp probe. |
| `~/.cache/huggingface/hub/models--black-forest-labs--FLUX.2-klein-4B` | ~15 GiB | Removed after the user emptied the corresponding Trash item; the active cache path is gone and about 15 GiB was recovered. |
| `.optimization-results/20260913T200837400Z-b401994db269` | ~2 GiB | Keep until this review is accepted; it contains rejected-run evidence. |
| `~/Desktop/Freelance/llm-memory-wall-research` | ~34 GiB | Project data; do not delete without inspecting its contents. |

Do not delete the Qwen weights, active evidence, or unrelated project directories merely to make the
disk percentage look better. The recovered space is enough for one carefully selected smaller model
download, but disk cleanup does not reduce model RAM pressure.

## Harness state and preparation changes

The benchmark uses one Go agent implementation and a Node orchestrator. It materializes fresh worktrees,
keeps hidden tests and oracles outside the model worktree, grades resulting files, samples macOS memory
once per second, records cancellation acknowledgement, and writes resumable JSONL state. The CLI trace
records the effective experiment configuration, tool-schema hash, request size, server timings when
available, tool duration, edit result, finish reason, and missing measurements.

The runner now also accepts `mlx-lm` and `mlx-vlm` server profiles, including directory-model tree
hashes, Python package version probes that do not import Metal during preparation, MLX health/model
endpoints, and a serialized no-prompt-cache profile. The first concrete candidate is
`benchmarks/optimization/experiments-qwen35-mlx.json`; its model tree and every model-dependent
validation remain pending because the current Hugging Face download failed at DNS resolution.

The capability-gate runner was corrected after the syntax-greedy probe: it no longer silently forces the
`focused` prompt for read and write gates. Gates now inherit `cli.gate_agent` and therefore exercise the
same configured prompt profile as the scored agent. A profile can still opt into `focused` explicitly.
This makes a gate failure attributable to the model/engine/configuration rather than an unrecorded prompt
change.

The Go agent's recovery registry now also understands complete Qwen `<tool_call>`/
`<function=...>`/`<parameter=...>` output with declared-schema type conversion. It refuses narration,
unknown tools/parameters, duplicate parameters, malformed XML, and truncated values. This path is
useful for recording a parser mismatch and for operational resilience; the benchmark still reports
native and recovered calls separately.

The current tree also contains two opt-in, oracle-free Gemma harness probes. The dynamic-schema arm
removes `read` and `search` for one turn after five reads without an edit, while the no-edit-retry arm
allows one corrective user turn after a completed response with no changed file. Both preserve the
baseline's native tools and external grading; neither exposes hidden oracle results. They are selected
only after the baseline has produced a normal-pressure verified repair.

Preparation-only static checks were run for the current documentation/configuration tree. Offline Go
tests (including race and bounded fuzz), selected prepared real-repository baseline checks,
fixture/oracle validation, and offline runner checks passed for the earlier prepared revision. The
first live server startup, warm smoke, memory sampling, cancellation acknowledgement, and capability-gate
stop are recorded. Current-tree dynamic checks, scored repair quality, a memory-valid capability gate,
live resume, runtime slot-cache erase before an attempt, and benchmark measurements remain pending. The
prepared llama session is:

`/Users/megasoft78/Desktop/Freelance/anvil-agent/.optimization-results/20260914T154835852Z-b401994db269`

Its manifest records the fixture pass, CLI hash preflight, server runtime, and the stopped gate. The
memory-valid gate, effective slot-cache erase before a scored attempt, repair quality, and all
measurements remain pending. The fresh Gemma session is
`/Users/megasoft78/Desktop/Freelance/anvil-agent/.optimization-results/20260915T063334836Z-b401994db269`;
its pinned model hash matches, and the selected fit fixtures pass. A current Codex-shell attempt failed
before health on the restricted loopback bind; live validation remains pending outside that shell. The
latest preparation session, which includes the new harness arms, is
`/Users/megasoft78/Desktop/Freelance/anvil-agent/.optimization-results/20260915T071413108Z-b401994db269`.

## Candidate ranking for this hardware

| Candidate | Published/local footprint | Likely fit | Why it matters | Next status |
|---|---:|---|---|---|
| Qwen3.6-35B-A3B UD-IQ2 GGUF | local file 11.52 GB / 10.73 GiB | Failed current host-state fit probe | Warm smoke parsed one native call, but warning pressure and swap-out growth stopped the first gate before a repair. | Do not advance to 16K; retain only as evidence unless a separately justified clean-state retry is approved. |
| Gemma 4 26B-A4B UD-IQ2_M-attnQ4K | Local 9.78 GB GGUF with recorded SHA-256 and prior related-agent evidence | Most actionable available fit candidate | No download; prior same-engine run measured about 1.02 GiB process footprint and 34/36 synthetic bugs, but its separate agent failed to close a real bug. | First no-download Go-agent fit probe; new memory and repair evidence required. |
| Qwen3.5-9B 4-bit MLX | HF tree reports ~5.98 GiB; local tree absent | Good candidate | Much more RAM headroom; official model card includes tool/agent use; the MLX-LM backend is wired with serialized requests and disabled prompt cache. | Download is pending network access; then prepare a fresh session and run only the one-task native gate/repair fit probe. |
| Gemma 4 E4B QAT Q4 | Google estimates ~4.5 GiB for E4B Q4 | Good candidate | Built-in function calling, coding/agentic training, QAT and memory-oriented variants. | Test parser/template compatibility before scoring. |
| Gemma 4 12B QAT Q4 | Google estimates ~6.7 GiB | Plausible | Higher quality ceiling with a still-manageable weight footprint. | Second small-model candidate. |
| Qwen3.5-4B | Exact artifact inventory pending; earlier BF16 size attribution withdrawn | Very good fit | Useful fast control and likely enough headroom for long tool history. | Test after 9B unless quality target requires it earlier. |
| Qwen3.6 Stable Affine4 + Hebrus | exact artifact about 20.81 GB | Unknown on base M1; SSD-capable | Strongest current-model storage-backed evidence: direct 16 GiB M1 Pro rows, zero-swap 8K/16K runs, OpenAI tool API. | First current-model SSD candidate after a residency failure; pin artifact/runtime before any download. |
| Qwen3.6 MTPLX + Qwisp strict/bolt | model about 20 GB | Strict likely too slow; bolt fit/quality unknown | Only shortlisted project with published base-M1 16 GB rows and a strict/near-lossless quality comparison. | High-information paired comparison after Hebrus or if base-M1 speed is the immediate question. |
| Qwen3.6 TQ3 + TurboQuant MLX v0.6.x | local older tree about 16.9 GiB; current upstream model about 16 GB | Current local path poor; new version unproven | Upstream now reports 16 GB M4 streaming at about 4.5 tok/s and 9.4 GB peak RSS with parallel/coalesced reads. | One versioned rescue only; do not repeat the old cache grid. |
| Mference Qwen3.6 | custom artifact about 19.6 GB | Unknown on base M1 | Qwen-specific Swift/Metal server with low-memory claims. | Defer until a Go API adapter and artifact provenance are ready. |
| Swiftlet / TensorFold / BigRig / streamlx | roughly 18–22 GB model trees or sidecars | Unknown on base M1 | Reusable low-resident MLX, slot-bank, prompt-lookup, and server patterns. | Implementation references, not promotion candidates without the Go repair gate. |
| Gemma 4 26B-A4B Q4 | Google estimates ~14.4 GiB | Too tight | Attractive active parameter count, but little room for macOS, KV, and tools. | Defer on this host. |
| Qwen3.6 MLX 4-bit | HF tree reports ~20.4 GiB | No | Larger than available practical disk/RAM budget. | Reject for this host. |
| Devstral Small 2 24B MLX 4-bit | HF tree/API about 24 GiB | No | Strong agentic coding specialization but published Mac target is 32 GiB and artifact is too large. | Defer. |
| GLM-4.7-Flash MLX 4-bit | HF API about 29.9 GiB | No | Too large before runtime headroom. | Defer. |
| Qwen3-Coder-30B-A3B | Exact artifact inventory pending; earlier BF16 size attribution withdrawn | Uncertain/large | Agentic coding specialization, but no safe local artifact yet. | Consider only after smaller candidates and disk cleanup. |

Published benchmark numbers are priors. They do not replace this harness’s verified repair rate,
native-tool parse rate, paired time, and memory-pressure measurements.

## Why context tuning is not the first lever

Qwen3.6 is a 35B-total / 3B-active MoE with 40 layers and a hybrid attention design. Using the official
architecture dimensions as an estimate, the full-attention KV portion is approximately:

`2 (K,V) × 10 full-attention layers × 2 KV heads × 256 head-dim × 2 bytes ≈ 20,480 bytes/token`

That is roughly 320 MiB at 16K tokens before allocator and recurrent-state overhead. This is an
analytical estimate, not a measurement, and it excludes the resident quantized weights and expert
cache. The previous 16K llama.cpp startup already reached about 10.5 GiB server RSS and 14.5 GiB
wired memory under a non-clean host state, so the weight/device allocation dominates the first fit
question. Context should be expanded only after a model can load and make a verified edit.

The Qwen3.5-9B architecture has fewer full-attention layers but more KV heads; the same rough fp16
calculation is about 32,768 bytes/token, or 512 MiB at 16K. A smaller weight footprint can still win
overall even if its per-token KV estimate is not lower. Gemma 4’s local sliding/global attention and KV
sharing are designed to reduce global KV footprint; Google’s documentation and the technical report
give model-level memory estimates, but local MLX/llama.cpp behavior must still be measured.

TurboQuant and KIVI research support low-bit KV caches as a possible long-context optimization. The
local E5 result shows that a KV flag accepted by the server does not guarantee a valid tool response;
measure native calls and repair quality before attributing any gain to memory compression. Speculative
decoding can produce substantial speedups in some settings, but it needs accepted-token counters and
may add resident draft/cache memory. It is a later speed arm.

For SSD streaming, compare full wall time rather than hit rate alone. If `C` is compute time,
`B_miss` exact synchronous bytes, `B_prefetch` early bytes, `B_residual` bytes still missing at use
time, `R` read count, `L` read latency, `BW` sustainable SSD bandwidth, `P` predictor/queue/slot
overhead, and `W` wasted I/O plus eviction churn:

`T_base ≈ C + B_miss / BW + R·L + overhead`

`T_prefetch ≈ max(C, B_prefetch / BW) + B_residual / BW + R_residual·L + P + W`

Prefetch wins only if it arrives before the next expert layer, the SSD has spare bandwidth, and the
native router still produces the same executed expert set. Record bytes/token, physical read count and
size, residual synchronous misses, prediction precision/recall, wasted bytes, queue depth, and overlap.
A larger cache can improve hit rate while increasing wired memory or read fragmentation and losing
end-to-end time.

Speculative decoding has a similar trap. A draft of `S` tokens can make a resident model amortize one
weight pass, but an SSD verifier may need the union `U = |⋃ E_i|` of routed experts across the draft
tokens rather than one token's `K` experts. If `U` grows, extra I/O can erase the benefit. Qwen's hybrid
GDN state also needs rollback on rejection. The later ngram arm must record accepted and drafted tokens,
union size, bytes/token, tool-call validity, repair outcome, and memory; a token-rate gain alone is not
a promotion.

## Ordered next steps

### Batch 1: existing Qwen3.6 GGUF and llama.cpp — completed stop

The prepared configuration [experiments-llama-qwen36.json](../benchmarks/optimization/experiments-llama-qwen36.json)
used 8K context and no speculation. It was a fit and parser probe with one `immer-array-push-fix`
repair. The runner completed warm smoke, then stopped at the first capability gate under warning
pressure and swap-out growth before dispatching the scored attempt.

The historical command used after explicit approval was:

```sh
cd /Users/megasoft78/Desktop/Freelance/anvil-agent
SESSION=/Users/megasoft78/Desktop/Freelance/anvil-agent/.optimization-results/20260914T154835852Z-b401994db269
CLI=/tmp/anvil-agent-opt
node benchmarks/optimization/runner.mjs validate --session "$SESSION" --fetch-reference
node benchmarks/optimization/runner.mjs run --session "$SESSION" --cli "$CLI" --experiment LLAMA-IQ2-8K-REAL --start-server --max-runs 1
node benchmarks/optimization/runner.mjs summarize --session "$SESSION"
```

The rebuilt `/tmp/anvil-agent-opt` matched the prepared manifest. The llama.cpp server was pinned to
local build 10809 / commit `5266f24da` and ran offline against the existing GGUF.

Observed result: warm smoke made one native tool call in 15.223 seconds. Startup/warmup reached
6.80 GiB server RSS, 13.06 GiB wired memory, 1.69 GiB compressed memory, and 55.1 MiB minimum free
memory. Kernel pressure became warning and swap-out delta reached 30,716. `gate-read-only` was
canceled after 2.514 seconds with zero tool calls. No scored repair or external grade ran. See the
session rows linked in [OPTIMIZATION-NEXT-BATCH.md](OPTIMIZATION-NEXT-BATCH.md).

Expected time is uncertain. Fixture verification may take roughly 5–20 minutes depending on dependency
state. Server load, warmup, and gates may take roughly 8–20 minutes; the scored attempt has a 180-second
agent deadline plus external grading. The likely memory range is also uncertain: the prior 16K startup
reached about 10.5 GiB server RSS and 14.5 GiB wired memory, but rebooting and closing applications
should reduce unrelated pressure, not the model allocation. Abort on critical pressure, three successive
swapout-growth samples, or a server/tool parser failure.

### Batch 2: 16K and screen — closed by Batch 1 stop

Do not run `LLAMA-IQ2-16K-REAL`, the four-task screen, or `LLAMA-IQ2-NGRAM-SCREEN` from the current
session. Batch 1 did not produce a memory-valid repair baseline, so the dependency for these arms is
not satisfied.

### Batch 3: smaller clean model or current-model SSD lane

Batch 1 failed its memory gate before scoring. The fastest next smaller-model signal is the already
available local Gemma 4 attnQ4K file; the runner is prepared to inspect its recorded hash and run it
through the same Go CLI. If it fails, inspect exact revisions and download one candidate at a time:
Qwen3.5-9B 4-bit MLX, then Gemma 4 E4B/12B official QAT Q4, with Qwen3.5-4B as a control. For a
current-model Qwen3.6 path where weight residency or expert I/O is the binding
failure, pin and evaluate Hebrus Stable Affine4 first, then Qwisp strict and Qwisp bolt as a paired
quality-risk arm. A pinned TurboQuant v0.6.x run is the only later TurboQuant rescue justified by the
new upstream evidence. Each path needs a model/artifact hash, template/runtime provenance, native
tool gate, one real repair, and the same oracle. The MLX/Gemma parser concerns still require a
capability gate before any quality screen.

Before any current-model SSD download, require the exact artifact size plus a build and temporary-file
cushion on the data volume. The current free-space snapshot is about 32 GiB at 93% capacity, so a roughly 20.8 GB
artifact leaves little room for duplicate conversions or sidecars.

The exact first smaller-model preparation command is recorded in
[`OPTIMIZATION-NEXT-BATCH.md`](OPTIMIZATION-NEXT-BATCH.md). The downloader requires the observed
Qwen3.5-9B tree estimate plus a 4 GiB reserve, resumes only that local directory, and writes a
download manifest after success. Hugging Face file-list sizes are mutable; `prepare` hashes the
actual tree and `run` refuses a changed tree. No candidate inference, fixture verification, server
startup, warmup, or model measurement is claimed here.

### Batch 4: custom harness improvements

Once a candidate can repair a real task, use paired one-factor tests:

1. stable tool schema versus dynamic tool availability;
2. current read/search limits versus 4 KiB/8 KiB bounded results;
3. baseline versus focused prompt, with the profile recorded in every trace;
4. thinking/reasoning off versus a bounded 512-token thinking budget, preserving reasoning only when the
   server/template supports it;
5. current exact-edit feedback, read deduplication, ledger, force-edit threshold, and close-out reserve,
   each isolated only when traces show that factor is binding.

Do not add a second agent or a second model. The existing harness is the experimental object: it already
has fresh worktrees, native tool execution, external grading, per-turn traces, memory samples, and
resumption. Improve it by making the tool schema/profile/parser relationship explicit and by deciding
from verified repairs rather than token counts.

### Batch 5: SSD-streaming engine lane

Only if a viable current model is blocked by weight residency or measured expert I/O, compare a
compatible backend in this order: Hebrus reactive/exact path, Qwisp strict then bolt, then a versioned
TurboQuant rescue. Inside a selected backend, measure reactive exact routing first, then one-layer-ahead
cross-layer/history prefetch, spatial-temporal eviction, and profile-guided layout. Keep the Go harness
and oracle fixed. A paper speedup, cache-hit rate, or server health response is not a promotion signal
without native tools and a verified repair on this host.

## Promotion rules

Use all attempted task-seeds, including timeouts, empty completions, zero-edit runs, and pressure aborts,
as the denominator. A quality candidate needs at least two additional verified repairs and no unacceptable
pressure. A speed candidate needs no lost verified repairs plus either at least 15% paired end-to-end
improvement, at least 512 MiB lower measured footprint, or repeatable inference paging eliminated. Use
task-clustered paired uncertainty intervals for a finalist; the current sample is too small for broad
claims.

The minimum practical success criterion is one complete verified real-bug repair under normal pressure,
followed by a repeatable screen result. If no candidate reaches that criterion, the evidence supports
moving to a machine with more RAM or a remote GPU/API instead of continuing arbitrary quantization and
context sweeps on this host.

## Primary sources and research basis

The web research was inspected on 2026-09-13/14. Mutable model-card and file-list claims should be
resolved to exact revisions before a download or final comparison.

- Qwen, [Qwen3.6-35B-A3B model card](https://huggingface.co/Qwen/Qwen3.6-35B-A3B) — architecture, tool use, sampling, context, and vendor coding benchmarks.
- Qwen, [Qwen3.6 architecture configuration](https://huggingface.co/Qwen/Qwen3.6-35B-A3B/raw/main/config.json) — dimensions used for the KV estimate.
- TurboQuant, [Qwen3.6 TQ3 model card](https://huggingface.co/manjunathshiva/Qwen3.6-35B-A3B-tq3-g32) — third-party MLX weight/KV quantization and approximate footprint.
- ggml-org, [llama.cpp repository](https://github.com/ggml-org/llama.cpp), [server documentation](https://github.com/ggml-org/llama.cpp/blob/master/tools/server/README.md), and [tool parser development notes](https://github.com/ggml-org/llama.cpp/blob/master/docs/development/parsing.md) — Metal, OpenAI-compatible server, Jinja, tool parsing, grammar, and speculative decoding capabilities.
- ggml-org, [Qwen3.6 parser issue #24807](https://github.com/ggml-org/llama.cpp/issues/24807) and [Qwen3.6 parser regression issue #24644](https://github.com/ggml-org/llama.cpp/issues/24644) — parser/version risks that must be tested locally.
- Qwen, [Qwen3.5-9B model card](https://huggingface.co/Qwen/Qwen3.5-9B) and MLX Community, [Qwen3.5-9B MLX 4-bit files](https://huggingface.co/mlx-community/Qwen3.5-9B-MLX-4bit/tree/938d8919941c6e7efd3c7150eff7fe9d12afa631) — smaller coding/tool candidate and reported file size.
- Google, [Gemma 4 core capabilities and inference memory](https://ai.google.dev/gemma/docs/core), [Gemma 4 function calling](https://ai.google.dev/gemma/docs/capabilities/text/function-calling-gemma4), and [Gemma 4 12B assistant card](https://huggingface.co/google/gemma-4-12B-it-assistant) — function calling, QAT/mobile variants, and published memory/quality priors.
- Google Research, [Gemma 4 technical report](https://arxiv.org/abs/2607.02770) — hybrid attention, KV sharing, and MTP design rationale.
- Mistral, [Devstral Small 2 model card](https://huggingface.co/mistralai/Devstral-Small-2-24B-Instruct-2512) and [MLX 4-bit files](https://huggingface.co/mlx-community/Devstral-Small-2-24B-Instruct-2512-4bit) — strong agentic candidate, but too large for this host’s current headroom.
- MLX-LM, [repository](https://github.com/ml-explore/mlx-lm), [server documentation](https://github.com/ml-explore/mlx-lm/blob/main/mlx_lm/SERVER.md), and [learned quantization notes](https://github.com/ml-explore/mlx-lm/blob/main/mlx_lm/LEARNED_QUANTS.md) — Apple Silicon engine and quantization options.
- MLX-LM, [Gemma 4 tool parser issue #1096](https://github.com/ml-explore/mlx-lm/issues/1096) — engine/template compatibility warning.
- KIVI, [arXiv:2402.02750](https://arxiv.org/abs/2402.02750), and TurboQuant, [arXiv:2504.19874](https://arxiv.org/abs/2504.19874) — rationale and limitations for low-bit KV caches.
- Leviathan et al., [speculative decoding](https://proceedings.mlr.press/v202/leviathan23a), and Jimenez et al., [SWE-agent](https://arxiv.org/abs/2405.15793) — speed/agent-interface research that motivates later controlled harness arms.
- Hebrus, [runtime](https://github.com/hebrus-labs/hebrus) and [16 GiB M1 Pro Qwen3.6 benchmark](https://github.com/hebrus-labs/hebrus/blob/main/docs/benchmarks/2026-07-29-qwen-m1-pro-16g-main.md) — current-model SSD evidence.
- Qwisp, [repository](https://github.com/penta2himajin/qwisp), [results](https://github.com/penta2himajin/qwisp/blob/main/bench/RESULTS.md), and [base-M1 issue #55](https://github.com/penta2himajin/qwisp/issues/55) — strict/near-lossless base-M1 comparison.
- TurboQuant-MLX, [Qwen3.6 streaming section](https://github.com/nathannorthcutt/turboquant-mlx#qwen36-35b-a3b-on-a-16-gb-mac-mini-expert-streaming) and [TQ3 artifact](https://huggingface.co/manjunathshiva/Qwen3.6-35B-A3B-tq3-g32) — newer versioned rescue hypothesis.
- [anemll-flash-mlx](https://github.com/Anemll/anemll-flash-mlx), [TensorFold](https://github.com/ashhart/TensorFold), [Sparsify](https://github.com/daylinkltd/sparsify), [ExpertCache](https://github.com/amos-labs/expertcache), and [Orbit Qwen3.6 compatibility](https://github.com/guelfoweb/orbit/blob/main/docs/QWEN_3_6_COMPATIBILITY.md) — reusable slot, parser, and evidence patterns.
- [NeuroPrefetcher](https://arxiv.org/abs/2608.22643), [SpecPrefetch](https://arxiv.org/abs/2607.24787), [SP-MoE](https://arxiv.org/abs/2510.10302), and the [MLX-LM hybrid n-gram proposal](https://github.com/ml-explore/mlx-lm/issues/1497) — predictive I/O and hybrid speculative-decoding limits.

These sources justify candidate selection and hypotheses. They do not establish that any candidate will
solve this repository’s tasks on this M1.
