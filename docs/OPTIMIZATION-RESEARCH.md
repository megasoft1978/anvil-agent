# Evidence for Qwen optimization on the M1

The best first investment is a reproducible tool-calling evaluation, explicit sampling, and an investigation of prompt-cache invalidation. These changes can improve the useful work obtained from existing weights without immediately spending more RAM. Larger quants and speculative decoding remain credible experiments, but their value depends on the measured bottleneck. [The execution handoff](OPTIMIZATION-HANDOFF.md) gives the ordered trials, budgets, and promotion rules.

## Evidence boundaries

Local inspection on 2026-09-13 confirmed Apple M1, 17,179,869,184 physical bytes, 8 CPU cores, macOS 26.6.2, and llama-server b10809 (`5266f24da`). There was no model server running. Swap occupancy was 1,573.44 MiB; `vm_stat` reported 16,384-byte pages and substantial compressed memory. This is an idle-state snapshot, not a measurement of model demand. Apple defines memory pressure using multiple factors including swap rate and wired/cache memory; free pages alone do not decide feasibility. [Apple memory documentation](https://support.apple.com/guide/activity-monitor/view-memory-usage-actmntr1004/mac).[^1]

The existing model file is 11,522,702,304 bytes, about 10.73 GiB. Shipped server flags select Metal offload, 24,576 context, one slot, flash attention, ngram-simple speculation, reasoning off, and 256 logical/physical batch sizes. Historical comments record a cache-related improvement to 16 turns in 245 seconds, with roughly 11.2 GB server RSS. These are earlier observations, not a fresh baseline. The previous cleanup shortened `TESTING.md`, leaving some historical references without their original evidence; use raw trace/config hashes before relying on old runs.

The research did not load a model, download weights, or execute live benchmark attempts. Improvements and memory feasibility remain hypotheses. Local source inspection identifies implementation facts; upstream issue reports establish relevant mechanisms, not proof that this installation currently suffers from them.

## Measurement problems to resolve first

The README's 20/44 result is a historical mixed score: inspection of all nine scenario JSON files found 20 executable bug checks and 24 pattern checks. A regex can reject a valid alternative repair or accept code that only looks appropriate. The old driver additionally requested full corrected files directly rather than executing the Go CLI. It has been removed along with installer speed measurements and bandwidth-scaled chip estimates.

| Scenario | Executable checks | Pattern checks |
|---|---:|---:|
| api-versioning | 0 | 6 |
| auth-session | 1 | 4 |
| cart-checkout | 1 | 3 |
| items-search | 0 | 4 |
| job-queue | 4 | 0 |
| notify-channel | 6 | 0 |
| orders-dashboard | 2 | 3 |
| permissions-cache | 4 | 0 |
| realtime-sync | 2 | 4 |
| Total | 20 | 24 |

The executable grader also needs validation: `grade()` treats the absence of a failure key as success. If an oracle reports one failure and subsequently crashes, it can leave unrelated checks implicitly passing; an early clean exit has a related missing-test problem. Tests should affirm the complete expected result set. `splitFiles()` also concatenates repeated file blocks despite a comment claiming later blocks replace earlier ones. The replacement runner should grade filesystem state directly and avoid making model prose serialization part of correctness.

`TestLiveModel` stops on an unsuccessful CLI status before running the host oracle. This is appropriate as a smoke gate but inadequate as an experiment collector: it loses the distinction between a useful unfinished patch and no fix. Preserve completed-and-verified repairs as the primary metric, while grading timed-out artifacts separately and keeping failures in denominators.

The six local pilot tasks exist, but the baseline verifier still selects two absent legacy keys. Two-step experiments use direct `runAgent` calls and hardcoded greedy sampling, so they are not interchangeable with the production CLI. The replacement orchestration must resolve these differences deliberately. Local references: [grader](../benchmarks/grade.mjs), [live runner](../go-agent/live_test.go), [pilot preparation](../go-agent/realrepo_test.go), [two-step experiment](../go-agent/live_twostep_test.go).

## Sampling and prompt continuity

Qwen recommends non-thinking sampling of temperature 0.7, top-p 0.8, top-k 20, min-p 0, presence penalty 1.5, repetition penalty 1. Its precise-coding thinking profile changes temperature to 0.6, top-p to 0.95, and presence penalty to 0. Thinking must be configured through the template/API; changing the model identifier alone does not enable it. [Qwen model card](https://huggingface.co/Qwen/Qwen3.6-35B-A3B#best-practices).[^2]

Local `profiles.go` sets most of these but never assigns `Request.MinP`. The installed binary's help reports default min-p 0.05. This gives E1 a specific, low-cost hypothesis: make the omitted value explicit, after checking effective server settings. The quality effect is unknown; treat it as an experiment, not a guaranteed fix. [Profiles](../go-agent/profiles.go), [request schema](../go-agent/protocol.go), [version-pinned argument source](https://github.com/ggml-org/llama.cpp/blob/5266f24da/common/arg.cpp).[^3]

The Go loop changes `request.Tools` for forced edits, close-out time, and repeated tools. It also removes reasoning from native tool turns by default and drops all but the first parallel call. Each can change the serialized conversation. Qwen's published template puts tool definitions near the beginning; a change there can reduce the common prefix for the whole conversation. That is a concrete cache-invalidation hypothesis, not yet demonstrated for the actual GGUF template. Capture the loaded template and inspect consecutive rendered token prefixes. [Qwen template](https://huggingface.co/Qwen/Qwen3.6-35B-A3B/raw/main/chat_template.jinja), [Go loop](../go-agent/agent.go).[^4]

llama-server reuses shared prefixes when prompt caching is enabled. Its documentation warns that backend/batch differences can prevent bit-identical behavior even with caching. Thus seed pairing helps control variation but is not a guarantee. Context checkpoints and the prompt-cache RAM pool are distinct from attention KV precision; disabling both checkpoint/cache mechanisms is not a clean “lower RAM” intervention. [Pinned server documentation](https://raw.githubusercontent.com/ggml-org/llama.cpp/5266f24da/tools/server/README.md).[^5]

The upstream hybrid-model cache issue is real: PR #24110 changes unnecessary checkpoint restoration and was merged June 4, 2026. The installed build is later, so repeatedly retesting that old fix should not be the first task. Investigate current traces for actual prefix loss instead. [PR #24110](https://github.com/ggml-org/llama.cpp/pull/24110), [issue #23589](https://github.com/ggml-org/llama.cpp/issues/23589).[^6]

For E7, both ends must preserve useful reasoning: the harness must retain native `reasoning_content`, and the template must render it as intended. Preserve IDs, argument content and message ordering consistently; even tool-call serialization can change the reusable prefix. The local binary exposes reasoning budget and preservation controls, but their effective behavior must be checked with real tool conversations before counting a thinking comparison as valid.

## What memory optimization can realistically buy

Qwen's config has 40 layers, ten full-attention layers, two KV heads, and head dimension 256. A simplified attention-only f16 KV estimate at 24,576 tokens is:

`10 layers × 2 (K,V) × 2 heads × 256 dimensions × 2 bytes × 24,576 tokens = 480 MiB`.

At 16,384 tokens that component is 320 MiB. q8_0 could save roughly half of that component before scales/metadata. This calculation excludes recurrent state, checkpoints, extra buffers, allocation padding and backend implementation details; reconcile it against actual server allocations. It explains why halving a KV component may save hundreds of MiB rather than several GiB. [Official architecture configuration](https://huggingface.co/Qwen/Qwen3.6-35B-A3B/raw/main/config.json).[^7]

`memory.go` currently samples RSS, wired memory, and free pages only. It cannot establish pressure-free operation or attribute compressed/swap usage. Startup logs plus time-series system counters are needed before choosing a memory tradeoff. The GGUF's on-disk size is not another allocation to add to RSS. Unified CPU/GPU memory also means CPU offload does not provide a separate pool that makes an oversized quant fit.

The publisher's file listing bounds plausible same-model candidates. Sizes below are rounded decimal GB; local manifests must record exact bytes and immutable revisions. [Unsloth quant files](https://huggingface.co/unsloth/Qwen3.6-35B-A3B-GGUF/tree/main).[^8]

| Artifact | Published size | Execution judgment |
|---|---:|---|
| UD-IQ2_M, current | 11.5 GB | Baseline |
| UD-Q2_K_XL | 12.3 GB | First alternative if measured headroom permits; tensor allocation/recipe may matter more than the label |
| UD-IQ3_XXS | 13.2 GB | Later quality probe; tight total system budget |
| UD-IQ3_S | 13.7 GB | Lower priority than the smaller candidate |
| UD-IQ4_XS | 17.7 GB | Exceeds 16 GiB physical RAM in file bytes alone; unsuitable for the pressure-free target |

Alternative quant artifacts may differ in calibration, tensor precision, or conversion as well as nominal bits. Compare them as shipped configurations unless tensor identity/recipe is controlled. No quality ranking follows from the filenames. Re-quantizing already heavily quantized weights cannot restore precision.

## Speed levers and their gates

Record `T_total = T_startup + T_prompt + T_generation + T_tools + T_host_verification + other overhead`, with resident-server inference reported separately. Parse only newly evaluated prompt tokens when measuring prefill work; the sum of advertised conversation lengths across turns overstates actual computation when caching works.

A simple upper bound helps reject low-value speculation work. If generation occupies fraction `f` of end-to-end time, even doubling generation speed gives `1 / ((1-f) + f/2)` total speedup. At f=0.10 that is about 1.053×; at f=0.40 it is 1.25×. This is an analytical illustration, not a measured projection. Use the real fraction from tool-driven runs before choosing E8.

ngram speculation drafts from repeated token sequences without a second model, which makes copying exact source into edit arguments a relevant workload. MTP uses model prediction heads. The pinned documentation supports both, but their speedup and memory overhead must be measured on this M1. Draft acceptance alone is insufficient if drafting or verification costs more time than it saves. [Speculative decoding documentation](https://raw.githubusercontent.com/ggml-org/llama.cpp/5266f24da/docs/speculative.md).[^9]

The available MTP-equipped UD-IQ2_M artifact is listed at 11.9 GB, compared with 11.5 GB for the current artifact. Do not assume those files have identical base tensor contents. Test MTP off/on on the same artifact, then test the artifact substitution separately. MTP and Metal improvements are upstream, but published results from other GPU families are not M1 measurements. [MTP artifact listing](https://huggingface.co/unsloth/Qwen3.6-35B-A3B-MTP-GGUF/tree/main), [MTP implementation](https://github.com/ggml-org/llama.cpp/pull/22673), [Metal speculative attention work](https://github.com/ggml-org/llama.cpp/pull/23114).[^10]

MLX is a plausible later Apple-Silicon backend comparison, not an automatic upgrade. Its documentation notes that models large relative to RAM can become slow. A reported Qwen hybrid-cache/speculative incompatibility further supports testing capabilities before benchmarking; an issue report is not proof that every later version remains affected. Keep backend and quantization effects explicit. [MLX-LM](https://github.com/ml-explore/mlx-lm), [hybrid-cache issue #1446](https://github.com/ml-explore/mlx-lm/issues/1446).[^11]

## Decisions deliberately deferred

Do not begin with system-wide wired-memory changes, mlock experiments, large context increases, a quant download matrix, or another serving stack. Inspect memory allocation and trace composition first. Reconsider one bounded nonzero checkpoint/cache-pool cap only if those allocations materially dominate memory, and require a long tool conversation to verify preserved reuse. Avoid interpreting server fit success as proof of normal whole-system memory pressure.

Do not judge quality using the model's own answer, a paid model's unblinded preference, increased completion rate alone, or comparison to provider benchmark headlines. Local tool permissions, budgets, precision, fixture distributions and hardware differ. The useful outcome is a defensible improvement over this project's own reproducible baseline, with correctness and resource tradeoffs visible.

## Sources

All web sources were inspected on 2026-09-13. Mutable model cards/file lists should be resolved to exact revisions during execution. Local references describe checkout `08a66de` before the planning cleanup unless noted.

[^1]: Apple, [View memory usage in Activity Monitor](https://support.apple.com/guide/activity-monitor/view-memory-usage-actmntr1004/mac). Local hardware evidence: `sysctl`, `sw_vers`, `vm_stat`, process listing and `llama-server --version`; read-only inspection.
[^2]: Qwen, [Qwen3.6-35B-A3B model card](https://huggingface.co/Qwen/Qwen3.6-35B-A3B), sampling and thinking sections.
[^3]: ggml-org, [argument implementation at installed commit](https://github.com/ggml-org/llama.cpp/blob/5266f24da/common/arg.cpp); confirmed against local `llama-server --help`.
[^4]: Qwen, [chat template](https://huggingface.co/Qwen/Qwen3.6-35B-A3B/raw/main/chat_template.jinja). Actual GGUF template equivalence remains to be checked.
[^5]: ggml-org, [server documentation at 5266f24da](https://raw.githubusercontent.com/ggml-org/llama.cpp/5266f24da/tools/server/README.md), caching and configuration.
[^6]: ggml-org, [checkpoint-restoration PR #24110](https://github.com/ggml-org/llama.cpp/pull/24110), merged June 4, 2026; [cache issue #23589](https://github.com/ggml-org/llama.cpp/issues/23589).
[^7]: Qwen, [architecture configuration](https://huggingface.co/Qwen/Qwen3.6-35B-A3B/raw/main/config.json). KV arithmetic is an inference from the listed dimensions.
[^8]: Unsloth, [Qwen3.6-35B-A3B GGUF file inventory](https://huggingface.co/unsloth/Qwen3.6-35B-A3B-GGUF/tree/main).
[^9]: ggml-org, [speculative decoding documentation at 5266f24da](https://raw.githubusercontent.com/ggml-org/llama.cpp/5266f24da/docs/speculative.md).
[^10]: Unsloth, [MTP GGUF inventory](https://huggingface.co/unsloth/Qwen3.6-35B-A3B-MTP-GGUF/tree/main); ggml-org, [PR #22673](https://github.com/ggml-org/llama.cpp/pull/22673) and [PR #23114](https://github.com/ggml-org/llama.cpp/pull/23114).
[^11]: Apple ML Research, [MLX-LM repository](https://github.com/ml-explore/mlx-lm); [issue #1446](https://github.com/ml-explore/mlx-lm/issues/1446).
