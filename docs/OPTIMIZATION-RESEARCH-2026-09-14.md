# Hardware-focused model and engine review

> Review precedence: [the revised decision package](OPTIMIZATION-ASTRA-DECISION.md#reviewed-decision-and-precedence--2026-09-14) controls priorities and interpretation where this document differs. The first live probe is recorded below; later model-dependent validation remains pending.

Reviewed 2026-09-14 for the 16 GiB Apple M1 and the existing `go-agent` harness. This update
consolidates the local evidence and the current upstream material that changes the probability of a
useful real-repository repair. It is a decision aid, not a claim that any unmeasured candidate works
on this machine.

## Recommendation

The existing Qwen3.6-35B-A3B UD-IQ2 GGUF was the first no-download probe because it is the mandated
current model and the only same-model engine comparison already prepared. Its 8K llama.cpp run has
now completed warm smoke but stopped at the first capability gate: the host reached warning pressure
and rising swap before any scored repair. Do not advance to 16K or ngram from that result, and do not
spend more TurboQuant screen budget on the current 16 GiB host.

If the GGUF fails to make a native call, an edit, and a normal-pressure verified repair, use two
separate decision lanes. The fastest next repair signal is the already available local Gemma 4
attnQ4K GGUF, followed by Qwen3.5-9B 4-bit on MLX and then an official Gemma 4 E4B QAT Q4 GGUF;
keep Qwen3.5-4B 4-bit MLX as the low-memory and speed control. The highest-potential current-model
path is a Qwen3.6 SSD backend, led by Hebrus's exact
Stable Affine4 artifact and then a paired Qwisp strict/bolt comparison. Those backends need roughly
20 GB model artifacts and have no base-M1 Go-agent repair result, so they are later, evidence-driven
choices rather than downloads to make now. Test every lane one candidate at a time through the same Go
CLI, fresh worktree, hidden oracle, native tool gate, and macOS memory sampler. Model-card scores and a
successful HTTP response are priors only.

The practical question is not whether a checkpoint can be loaded. The target is at least one complete
real-bug repair under normal pressure, then a repeatable screen. A small model that reaches an exact
edit reliably is more useful here than a larger model that spends its headroom on weights, cache, or
parser failures.

The fastest available next measurement is now the local Gemma 4 26B-A4B `UD-IQ2_M-attnQ4K` GGUF. It
is already present in the separate research checkout, has a recorded 9,777,197,536-byte file hash,
and has prior same-engine memory and coding-suite evidence. That evidence came from a different agent,
so it does not replace the current Go repair gate. Testing it before the network-dependent Qwen3.5
download reduces time to a decisive signal; Qwen3.5-9B MLX remains the next candidate if Gemma fails or
the MLX runtime becomes available outside the restricted shell.

The separate research checkout also contains a dense Qwen3.8 GSQ-RCO IQ3_XXS GGUF at
10,094,357,632 bytes with SHA-256
`fdfcb6a29b11188956dfbfd904223588a6c1b77eb250c3e8a36e1bd269df91f7`. Its prior same-machine suite
reports strong multi-file repair quality but only about 4.5–4.9 tok/s. It is therefore prepared as a
same-Go-CLI quality ceiling comparator after the Gemma fit path, with an 8K no-speculation probe before
its screen and ngram arm. The file hash is the immutable identity because its source revision was not
retained with the copied artifact.

## Current smaller-model artifact and engine preparation

The first concrete fallback is the Hugging Face
[MLX Community Qwen3.5-9B 4-bit tree](https://huggingface.co/mlx-community/Qwen3.5-9B-MLX-4bit/tree/938d8919941c6e7efd3c7150eff7fe9d12afa631),
whose current file listing was approximately 5.98 GB when inspected. The experiment pins the
full file-list commit `938d8919941c6e7efd3c7150eff7fe9d12afa631` and still treats the resulting local
tree hash as the final identity. The guarded downloader requires the observed size plus a 4 GiB
free-space reserve and downloads only this candidate directory. A shell attempt from the current
sandbox failed during DNS resolution before any model file was written.

The installed MLX-LM 0.31.2 server provides `/health`, `/v1/models`, and OpenAI-compatible chat
completions with tools. Static source inspection confirms `--prompt-concurrency`,
`--decode-concurrency`, `--prefill-step-size`, `--prompt-cache-size`, and
`--chat-template-args`; this revision does not expose the `--max-kv-size` flag used by some other MLX
servers. The prepared profile therefore serializes requests, uses a 512-token prefill step, disables
the prompt cache, and records the lack of a `/slots` drain/erase endpoint. The server entry point was
not started and importing MLX was deliberately avoided after a help/import probe initialized Metal and
crashed before model loading; that runtime issue remains pending for the actual candidate gate.

This is a capacity-first test, not a quality claim. The same Go CLI, native `read`/`search`/`edit`/
`write` calls, repair prompt, hidden oracle, fresh worktree, and one-second memory guard remain fixed.
Only a normal-pressure native-tool gate followed by a verified repair can justify the four-task screen.

## What the local evidence says

The local TurboQuant Qwen3.6 path is already a negative signal for this host. The 1 GiB expert-cache
screen recorded 8/8 truncated attempts, zero edits, zero verified repairs, warning pressure on every
scored attempt, about 3.22 GiB peak server RSS, about 11.09 GiB wired memory, about 1.99 GiB
compressed memory, and about 14 MiB minimum free memory. The mixed KV arm failed its warm native-tool
gate, and the two-prefetch-worker arm hit the critical-memory guard during warmup. Those results are
contaminated by prior host state, but they are strong enough to stop arbitrary TurboQuant permutations
until a new allocation hypothesis exists.

The existing llama.cpp GGUF is 11,522,702,304 bytes on disk. The approved 8K probe used the current Go
CLI and one slot with no speculation. Warm smoke made one native tool call in 15.223 seconds, then
startup/warmup reached 6.80 GiB server RSS, 13.06 GiB wired memory, 1.69 GiB compressed memory, and
55.1 MiB minimum free memory. Kernel pressure was warning and swap-out delta reached 30,716, so the
first gate stopped before scoring. These values overlap in what they account for and must not be added.

The latest post-cleanup read-only snapshot has about 32 GiB available at 93% data-volume capacity (an earlier snapshot during preparation had
about 34 GiB). The two local Qwen3.6 artifacts and rejected
optimization evidence are still needed for the decision. The unrelated
`~/Desktop/Freelance/llm-memory-wall-research` project is about 34 GiB and is not a safe cleanup
candidate without inspecting its contents. Disk cleanup can make one small model download possible;
it does not reduce resident unified-memory pressure.

## Candidate matrix

| Candidate | Evidence available now | Memory outlook on 16 GiB | Tool/agent risk | Decision |
|---|---|---|---|---|
| Qwen3.6-35B-A3B UD-IQ2 GGUF + llama.cpp | Local 11.52 GB GGUF; 8K warm smoke and gate attempt recorded | Failed current host-state fit probe; weight/runtime allocation dominates before repair | Warm smoke made one native call, but warning pressure and swap-out growth stopped the first gate | Do not advance to 16K; preserve evidence and select a smaller model or separately prepared SSD path |
| Gemma 4 26B-A4B UD-IQ2_M-attnQ4K + llama.cpp | Local 9.78 GB GGUF with recorded SHA-256; prior related-agent suite and memory evidence | Most actionable available fit candidate; exact Go-agent pressure remains unknown | Prior real-repo probe localized bugs but failed to close edits; current native parser/tool gate is still required | First no-download fallback; run one Go repair fit probe before any screen |
| Qwen3.8-27B GSQ-RCO IQ3_XXS + llama.cpp | Local 10.09 GB GGUF with recorded SHA-256; prior same-machine dense repair suite | Likely close to the Qwen3.6 weight envelope; live pressure and 8K fit remain unmeasured | Dense decode is expected to be bandwidth-limited around 4.5–4.9 tok/s; current Go-agent result is pending | Same-harness quality ceiling comparator after the Gemma fit probe; stop if it cannot make a normal-pressure repair |
| Qwen3.5-9B 4-bit + MLX | Community MLX tree is about 5.95 GB; official Qwen card has coding/tool priors | Best expected balance of quality and headroom among researched candidates | MLX parser and cache flags are evolving; exact revision and parser need a gate | Next smaller-model candidate after the local Gemma probe |
| Gemma 4 E4B QAT Q4 GGUF + llama.cpp | Google estimates about 4.5 GB base load with overhead; official QAT artifact | Good headroom, with context/KV still to measure | Native function calling is documented; engine/template compatibility still needs a gate | Second smaller-model candidate |
| Qwen3.5-4B 4-bit + MLX | Community MLX tree is about 3.03 GB model data; lower official coding/tool priors than 9B | Safest speed and memory control | Same MLX parser/cache revision risk; quality may be too low | Control or early fallback |
| Gemma 4 E2B QAT Q4 GGUF | Google estimates about 2.9 GB base load; official GGUF is about 3.35 GB | Safest Gemma memory control | Lower quality ceiling; function-call gate required | Use if E4B still pressures the host |
| Gemma 4 12B QAT Q4 GGUF | Google estimates about 6.7 GB base load | Plausible, but less headroom than E4B | Same function-call/template gate | Test only after E4B or if quality requires it |
| Qwen3.6 TurboQuant TQ3 | Local about 16.9 GiB of safetensors; current upstream has a newer v0.6.x streaming path | Poor in the recorded local runtime; newer path is unproven here | Custom runtime/parser; local 1–2 GiB arms failed or pressured the host | Keep current branch closed; allow one versioned rescue only |
| Qwen3.6 Stable Affine4 + Hebrus | Published artifact is about 20.81 GB; direct 16 GiB M1 Pro SSD results exist | Unknown on base M1; likely workable as a capacity path | New artifact/runtime and no Go-agent repair evidence | Top current-model SSD candidate after a residency failure |
| Qwen3.6 MTPLX + Qwisp | Published model is about 20 GB; direct base-M1 16 GB rows exist | Strict is likely too slow; bolt may fit but can affect quality | Strict/bolt exactness and tool repair must be paired | Highest-information base-M1 SSD comparison |
| BitNet/ternary 1.58-bit models | Official framework and a separately trained 2.4B model exist | Potentially excellent memory/speed | No evidence of real-repository repair quality or compatibility with this agent | Later exploratory control only |

The Google Gemma memory table is a base-load estimate with stated overhead; it excludes KV and
software overhead. The Hugging Face MLX tree sizes are mutable file-list observations, so every future
download must pin a revision and record exact bytes. A smaller model is a candidate because it leaves
room for macOS, the server, conversation state, and verification; it is not automatically a better
coder.

## New direct engine evidence

The recent upstream evidence is strong enough to change the later-engine order, but not strong enough
to change the first batch. The results below are claims from the cited projects until this harness
reproduces them on the base M1 with the actual Go tool loop.

| Path | Direct evidence | What it could prove here | Main blocker |
|---|---|---|---|
| Hebrus + Qwen3.6 Stable Affine4 | The project supports Qwen3.6-35B-A3B through a bounded SSD ExpertMajor runtime. Its published 16 GiB M1 Pro table reports zero swap and about 9.58 tok/s decode at 8K, with about 5.72 GiB peak footprint; the 16K row reports about 8.64 tok/s. See the [runtime](https://github.com/hebrus-labs/hebrus) and [M1 Pro benchmark](https://github.com/hebrus-labs/hebrus/blob/main/docs/benchmarks/2026-07-29-qwen-m1-pro-16g-main.md). | Whether the current model can remain useful with exact routing and SSD-backed weights when the local GGUF cannot stay resident. | Base M1 is slower than M1 Pro; the exact Stable Affine4 artifact is about 20.81 GB and must be pinned before download; no Go-agent repair result is published. |
| Qwisp strict/bolt + Qwen3.6 MTPLX | The project reports real base-M1 16 GB rows. Strict mode is about 1.2–1.5 tok/s on the cited base-M1 reports; bolt is about 11.7–26.4 tok/s, but one long run is marked `LOOPY`. It exposes an OpenAI-compatible server and tool continuation. See the [results](https://github.com/penta2himajin/qwisp/blob/main/bench/RESULTS.md), [base-M1 report #55](https://github.com/penta2himajin/qwisp/issues/55), and [base-M1 report #67](https://github.com/penta2himajin/qwisp/issues/67). | A paired strict versus near-lossless comparison can show the quality cost of removing cold expert reads on this exact class of host. | Bolt may change effective expert behavior and therefore quality; strict may be too slow for a repair loop; the model is about 20 GB and requires a new artifact. |
| TurboQuant MLX v0.6.x + Qwen3.6 TQ3 | The current upstream README reports a base-M4 16 GB run at about 4.5 tok/s, about 9.4 GB peak RSS, and 91% hit rate with an 8 GB cache, plus parallel reads, coalescing, and bit-identical claims. See the [Qwen3.6 streaming section](https://github.com/nathannorthcutt/turboquant-mlx#qwen36-35b-a3b-on-a-16-gb-mac-mini-expert-streaming) and [current model artifact](https://huggingface.co/manjunathshiva/Qwen3.6-35B-A3B-tq3-g32). | Whether the local failure was primarily the old `0.25.0` runtime and parser path rather than the model format. | The published host is M4, the recommended cache is much larger than the locally safe profile, and the local 1 GiB/2 GiB Go-agent arms already failed or hit pressure. Only a pinned version change justifies a retest. |
| Mference | A Swift/Metal Qwen3.6-specific server advertises a low-memory path and an OpenAI API, but its published Qwen figures are on M5 and its server does not accept every tool-choice field used by the Go client. See the [project](https://github.com/NeelM0906/Mference) and [API contract](https://github.com/NeelM0906/Mference/blob/main/docs/OPENAI_SERVER.md). | Whether a native Swift path gives a lower resident footprint after an explicit API adapter. | No M1 Qwen3.6 measurement, custom repacked artifact, and a likely `tool_choice`/parallel-field adapter. |
| Swiftlet, TensorFold, BigRig, streamlx | These projects provide Qwen3.6 or Qwen3.5 Apple SSD-streaming designs with OpenAI-compatible endpoints, slot/cache budgets, or low-resident MLX execution. [Swiftlet](https://github.com/leonickson1/Swiftlet) reports 7–11 tok/s on M5 with about 2.6 GB runtime budget; [TensorFold](https://github.com/ashhart/TensorFold) reports a guarded Qwen3.6 run on a 24 GB Mac mini; [BigRig](https://github.com/arjvnv/BigRig) and [streamlx](https://github.com/srcterm/streamlx) expose reusable cache and server ideas. | Which implementation shape is cheapest to adapt if the direct Qwen3.6 candidates fail. | No comparable base-M1 native Go-tool repair result; several require a model conversion or extra packed sidecar. |
| Sparsify and ExpertCache | [Sparsify](https://github.com/daylinkltd/sparsify) reports byte-identical MLX SSD paging on a 16 GB MacBook for other MoE models and includes a tool loop. [ExpertCache](https://github.com/amos-labs/expertcache) reports a page-aware Metal path on a 16 GiB M1 Pro for GPT-OSS, but its 16 GiB decode result is far too slow for interactive use. | Reusable slot ownership, direct views, and telemetry patterns. | Model mismatch; ExpertCache's current-model and speed evidence do not transfer. |

This produces a useful two-axis priority rather than a misleading single leaderboard. For useful repair
probability after a current-model failure, choose the local Gemma 4 attnQ4K probe, then Qwen3.5-9B MLX,
then Gemma 4 E4B. For preserving
Qwen3.6 capability with a storage-backed runtime, choose Hebrus, then Qwisp strict/bolt, then a pinned
TurboQuant v0.6.x rescue. For implementation ideas, reuse stable slot banks, coalesced reads, bounded
prefetch, and direct telemetry before attempting learned predictors or output-changing routing.

The TurboQuant README's optional `--max-active-experts 4` result is not an exact-routing result: it
changes Qwen3.6's native top-8 route and only reports a limited six-test stress comparison. It must not
be used for this repair benchmark unless the same Go tasks prove no quality loss, and the default rescue
must retain native top-8 routing. The upstream streaming speed and bit-identical claims also do not
establish a native tool-call path; the Go capability gate remains mandatory.

## Why the engine order changes the odds

The official Qwen3.6 card describes a 35B-total/3B-active MoE with 40 layers, 256 experts, and a
262,144-token native context, while recommending large GPU-oriented servers for complex work. The
active parameter count does not mean only 3B worth of weights must be resident. On a 16 GiB unified
memory machine, the resident quantized weights, allocator buffers, recurrent state, KV state, prompt
history, and macOS headroom all compete for the same pool.

The current llama.cpp issue history lowers confidence in the Qwen3.6 parser path near the local
build. Issue [#26043](https://github.com/ggml-org/llama.cpp/issues/26043) reports a Qwen3.6 response
where the parser detected tagged XML but the server returned the raw XML as content. Issue
[#24807](https://github.com/ggml-org/llama.cpp/issues/24807) describes malformed duplicate parameter
closers that can drop a complete call, and [#26763](https://github.com/ggml-org/llama.cpp/issues/26763)
describes a missing newline causing subsequent parameter/call text to be consumed into the first
argument. These are upstream reports, not proof of the local build's behavior. The prepared Go agent
now has a strict Qwen XML recovery path as a diagnostic and robustness measure, but a recovered call
is recorded separately and cannot satisfy the native parser gate.

MLX-LM is attractive on Apple Silicon because it is designed around MLX and provides an OpenAI-like
local server. Its current Qwen parser uses the model's `<function=...>` and `<parameter=...>` format
and converts arguments from the declared tool schema; see the [current parser source](https://raw.githubusercontent.com/ml-explore/mlx-lm/main/mlx_lm/tool_parsers/qwen3_coder.py)
and [server source](https://raw.githubusercontent.com/ml-explore/mlx-lm/main/mlx_lm/server.py).
That is a better starting point for a smaller Qwen3.5 candidate, but the parser and server are moving
targets. The MLX-LM [Qwen parser crash report #1604](https://github.com/ml-explore/mlx-lm/issues/1604),
[tool parsing fix #905](https://github.com/ml-explore/mlx-lm/issues/905), and cache growth reports
[#883](https://github.com/ml-explore/mlx-lm/issues/883) and
[#1390](https://github.com/ml-explore/mlx-lm/issues/1390) justify pinning an exact revision, disabling
or tightly bounding prompt cache, and running a native tool gate before any scored repair. The current
server source exposes `--prompt-cache-size`, `--prompt-cache-bytes`, `--kv-bits`, `--kv-group-size`,
and `--quantized-kv-start`; it does not expose `--max-kv-size` at this revision. It also logs a parser
failure and drops that tool call, so a health response can coexist with an unusable tool path. Future
MLX configs must inspect the pinned `--help` output and record unsupported flags as unavailable rather
than copying options from another branch.

Gemma 4 is worth testing because Google publishes QAT Q4 artifacts and documents function calling.
The [Gemma 4 core documentation](https://ai.google.dev/gemma/docs/core) gives smaller E2B/E4B memory
options and distinguishes base load from context overhead. The [function-calling guide](https://ai.google.dev/gemma/docs/capabilities/text/function-calling-gemma4)
confirms the intended capability. The earlier MLX Gemma parser report
[#1096](https://github.com/ml-explore/mlx-lm/issues/1096) shows why the exact MLX/llama.cpp version and
template still need a gate.

## Quantization findings

TurboQuant is principally a vector/KV-cache quantization method. The [TurboQuant paper](https://arxiv.org/abs/2504.19874)
reports strong low-bit KV results in its evaluated settings, but KV compression cannot turn a
16.9 GiB weight artifact into a comfortable 16 GiB resident workload. The current llama.cpp
[TurboQuant discussion](https://github.com/ggml-org/llama.cpp/discussions/20969) also indicates that
stock llama.cpp does not provide the same path without custom work. TurboQuant remains useful as a
long-context research direction after a viable model exists, not as the first rescue for this host.

BitNet is a different category. Microsoft's [BitNet repository](https://github.com/microsoft/BitNet)
provides kernels and a separately trained ternary model; its reported ARM speed/energy results are
promising for edge inference. It is not a drop-in re-quantization of Qwen3.6, and no local evidence
shows that the available small ternary model can perform exact Go-agent repairs. It should not delay
the Qwen3.5/Gemma measurements.

## SSD streaming and predictive expert loading (2026 update)

The suspected research direction is real, and the newest work separates three related problems.
[ST-MoE](https://arxiv.org/abs/2606.15453) reports that expert selections correlate across adjacent
layers and consecutive decode tokens. Its runtime combines a cross-layer correlation table with a
one-token history table, predicts the next layer's experts, and preserves the original router as the
final authority. The paper reports 85% prediction accuracy and large simulated/hardware co-design
speedups, but it does not evaluate an Apple M1 or SSD-backed Qwen3.6. This is the closest match to
"predict what to load next" at the algorithm level.

[Patterns Behind Chaos](https://arxiv.org/abs/2510.05497), published at ISCA 2026, studies
prefill-to-decode, cross-layer, cross-token, expert-pair, and task-dependent regularities in large
MoE workloads. It supports using the task class and recent routing history to plan data movement,
but its target is distributed GPU communication rather than local SSD reads. [Speculating
Experts](https://arxiv.org/abs/2603.19289) and [Pre-Attention Expert Prediction](https://arxiv.org/abs/2511.10676)
reach the same general conclusion with different predictors. The former reports a 5--14% time per
output token reduction in its CUDA CPU/GPU setup; the latter uses a trained lightweight predictor.
Neither establishes Qwen3.6-on-M1 behavior.

Storage research supplies the missing systems pieces:

| Work | Mechanism | What can transfer | Boundary for this project |
|---|---|---|---|
| [MAIO/PPC](https://www.usenix.org/conference/fast26/presentation/liu-yubo) | Programmable page-cache policy, I/O templates, locality-aware prefetch and eviction | Separate startup loading from decode; use access templates and measured locality | Linux/page-cache research; no macOS/M1 result and no per-token Qwen3.6 proof |
| [LLM in a Flash](https://machinelearning.apple.com/research/efficient-large-language) | Windowing, row/column bundling, larger contiguous reads from flash | Reduce request count and bytes moved when weights must stream | Original work is not a Qwen3.6/M1 agent benchmark |
| [mbolt](https://github.com/doramirdor/mbolt) | Profile-guided expert layout and merged `pread` reads | Make predicted or routed expert sets physically contiguous; count reads, not just bytes | Requires a custom layout and loader; it is not a drop-in change to this GGUF |
| [streamlx](https://github.com/srcterm/streamlx) | Apple Silicon MLX, bounded expert pool, residual lookahead, merged reads, S3-FIFO option | Closest practical Apple SSD-streaming candidate; it reports Qwen3.6 experiments | v0.1, M4 measurements, one request at a time, and no Go-agent native-tool result here |
| [Qwen3.6 colibri](https://github.com/RuiRDA/colibri-qwen3.6-35B-A3B) | CPU-only Qwen3.6, per-layer LRU, safetensor expert streaming | A low-memory feasibility path with an OpenAI-compatible API | New converted weights, Windows/i7 evidence, no M1 or repair evidence |
| [edgeMoE](https://github.com/supastishn/edgeMoE) | Direct reads, bounded cache, optional overlap and predictive prefetch | llama.cpp integration ideas and cache telemetry | macOS is unvalidated; its own on-device read-ahead result was not positive |
| [FATE-llama.cpp report](https://github.com/ongunm/llama-moe-cache/blob/main/FATE_IMPLEMENTATION_REPORT.md) | Cross-layer plus temporal GPU expert prefetch | A warning to measure end-to-end throughput and overlap | Its 99.94% cache hit rate still ran slower than its baseline; hit rate alone is not success |

Several newer projects make the implementation priority clearer. [anemll-flash-mlx](https://github.com/Anemll/anemll-flash-mlx)
reports that a stable per-layer slot bank with changing expert IDs is materially cheaper than
rebuilding a K-expert tensor on every token. Its workflow explicitly measures a resident packed-bank
ceiling and an all-hit/oracle-prefetch ceiling before optimizing the miss path. That is a low-risk
design to borrow when a backend exposes expert-level reads. [TurboQuant-MLX](https://github.com/nathannorthcutt/turboquant-mlx)
now reports `os.pread`, read coalescing, an eight-worker pool, and optional one-layer-ahead prefetch;
these are testable systems factors, but the current local runtime result remains the controlling
negative evidence for this host.

The most relevant recent papers are more ambitious and therefore later. [SpecPrefetch](https://arxiv.org/abs/2607.24787)
keeps the native router as the final authority and trains a low-rank adjacent-layer adapter only to
rank asynchronous transfers. Its useful constraint is explicit: a prefetch budget must fit the
available overlap window, `W_l = t_exec(l+1) - t_pred(l)`, and requests outside the window only add
traffic. The reported results use Qwen3-VL and DeepSeek-VL, not Qwen3.6. [NeuroPrefetcher](https://arxiv.org/abs/2608.22643)
uses a 206.8M-parameter predictor, model-specific sparse activation thresholds, centroid training,
and delta-row movement; it reports 7.9–12.0x on Jetson with Mistral/Llama. This changes the executed
model's sparsity and quality contract, requires offline training, and is therefore not an acceptable
first intervention for exact repository repair. It is a research direction for a future custom engine,
not a drop-in cache policy.

The resulting cost model should include prediction and wasted traffic, not only cache hits. Let
`C` be compute time, `B_miss` the bytes synchronously read for exact routed experts, `B_prefetch` the
bytes issued early, `B_waste` the prefetched bytes never used, `R` the number of physical reads, `L` the
per-read latency, and `BW` the sustainable SSD bandwidth. A useful baseline is:

`T_base ≈ C + B_miss / BW + R·L + overhead`

With asynchronous prediction, the visible time is closer to:

`T_pred ≈ max(C, B_prefetch / BW) + B_residual / BW + R_residual·L + P + W`

where `P` is predictor/queue/slot overhead and `W` includes wasted I/O, eviction churn, and queue
contention. The intervention wins only when `T_pred < T_base`, the native route remains unchanged,
and the repair oracle still passes. A high hit rate can lose if it comes from a large resident cache,
small fragmented reads, or a prediction path that costs more than the stalls it hides.

For Qwen3.6, speculative decoding adds another term. If a draft proposes `S` tokens, verification
may need the union of their routed experts, `U = |⋃_{i=1..S} E_i|`, rather than the `K` experts of one
token. In a resident engine, accepted tokens can amortize one weight pass. In an SSD engine, a large
`U` can increase `B_prefetch` and erase the gain. The Qwen hybrid GDN state also needs checkpoint and
rollback on rejection. Therefore n-gram or MTP is a later paired experiment, with accepted tokens,
union size, bytes/token, tool-call validity, and verified repair as the decision metrics. The MLX-LM
[hybrid n-gram proposal](https://github.com/ml-explore/mlx-lm/issues/1497) is promising because it uses
zero draft weights and explicitly handles recurrent-state rollback; it still should be enabled only
after a no-speculation repair baseline.

The important distinction is that prediction must remain below the agent API. The Go harness should
continue sending the same prompt and real native `read`, `search`, `edit`, and `write` calls. A
streaming backend may use the current router result, a cross-layer table, a token history table, or
an MTP signal to start reads early, while the model's actual router still decides which experts are
executed. Predicted-but-unused experts must never change the answer silently. The harness only needs
to record the engine counters: bytes read per token, read count and merged-read size, cache hits and
misses, prefetch requested/used/cancelled/wasted, prediction precision/recall, residual synchronous
misses, queue depth, read latency, and compute/read overlap.

For a streamed token, a useful first-order bound is
`T_token >= T_compute + bytes_read / BW_ssd` without overlap and approximately
`max(T_compute, bytes_prefetched / BW_ssd) + T_residual_miss` with effective overlap. Prediction
cannot beat the bytes-per-token divided by real SSD bandwidth floor. It can reduce visible stalls
only when the prediction arrives early, the layout turns expert requests into sufficiently large
reads, the cache avoids eviction collisions, and the SSD queue has spare bandwidth. SpecMD's
[Least-Stale/cache-policy study](https://arxiv.org/abs/2602.03921) is useful here because it shows
that layer-aware spatial and temporal eviction can outperform plain LRU; its A100/older-model
results still need translation to this host.

For this 16 GiB M1, streaming is a capacity escape hatch, not an automatic speed upgrade. If the
11.5 GiB GGUF can stay resident, ordinary mmap/Metal execution may be faster; if it cannot fit with
runtime and macOS headroom, direct expert streaming may make the model load at the cost of much lower
decode speed. The 8K GGUF probe has now stopped under warning pressure before a repair baseline. A
streaming backend can therefore be considered as a separately prepared capacity branch:
reactive exact routing, one-layer-ahead prediction, adaptive cache eviction, and layout coalescing,
one factor at a time. Every backend must preserve exact top-8 routing, expose an OpenAI-compatible
endpoint, pass the native-tool gate, and use the external repair oracle. No SSD-streaming code,
weight download, or server change is part of the current preparation session.

## Proposed experiment order after preparation

1. Preserve the failed `LLAMA-IQ2-8K-REAL` session as evidence. Its warm smoke made one native call,
   but warning pressure and swap-out growth stopped the first capability gate before scoring.
2. Prepare the already available Gemma 4 attnQ4K file through the same llama.cpp build and Go CLI.
   Start the named fit probe only after model-free validation and a normal memory preflight. If it fails, prepare the
   pinned Qwen3.5-9B 4-bit MLX artifact as the next smaller-model candidate.
3. Only if a later no-speculation candidate produces a normal-pressure verified repair should a screen
   or ngram speculation be considered. Require accepted-token,
   drafted-token, wall-time, native-call, repair, and memory evidence. Do not treat a token-rate gain
   as useful if it loses a verified task.
4. For the current GGUF's pressure failure, pin and inspect one Qwen3.5-9B MLX revision. Record the exact model tree hash, MLX-LM
   revision, parser capability, cache settings, and endpoint command. Use the same Go CLI and oracle.
5. If the local Gemma probe does not fit or cannot repair, test the pinned Qwen3.5-9B MLX candidate,
   then Gemma E4B QAT Q4, then Qwen3.5-4B as a safe control, then Gemma E2B. Consider Gemma 12B only
   if the smaller candidates establish that quality, rather than memory, is the binding failure.
6. If the current model is still the quality priority and the failure is weight residency or expert I/O,
   add a current-model SSD lane in this order: Hebrus Stable Affine4; Qwisp strict; Qwisp bolt only as a
   paired quality-risk arm; then one pinned TurboQuant v0.6.x rescue. Mference, Swiftlet, TensorFold,
   BigRig, streamlx, and a small slot-bank port are fallback implementation studies. Before any new
   artifact, require a disk preflight with at least the model size plus a build/download cushion. For
   each backend, first establish reactive exact-routing bytes/read telemetry, then test one-layer-ahead
   cross-layer/history prefetch, spatial-temporal eviction, and profile-guided layout. Keep native
   routing and the Go CLI/oracle fixed; stop if prediction increases bytes, pressure, or verified-repair
   failures. Published accelerator results cannot substitute for an M1 measurement.
7. After a viable model is found, test harness factors one at a time: stable/dynamic tool schema,
   an oracle-free no-edit retry, bounded read/search output, prompt profile, reasoning preservation,
   forced-edit threshold, close-out reserve, and exact edit feedback. The existing Go harness remains
   the agent under test. The Gemma configuration now contains separate dynamic-schema and no-edit-retry
   fit probes, each dependent on the stable baseline.
8. Finish with a cold-start check, a normal-pressure repeat, and a finalist run with everyday apps
   open as a separate product-usage measurement. Do not mix that result with the controlled fit gate.

Every candidate must use a fresh prepared session and must remain resumable. A native tool call means
the server returned an executable `tool_calls` entry. A recovered call means the model emitted a
strictly parsed textual dialect that the Go agent reconstructed. Both may be useful operationally, but
they answer different questions and must never be merged in the native-parser score.

## Decision thresholds

- Stop an arm on critical pressure or three consecutive one-second samples with increasing swapouts.
- Keep all timeouts, empty completions, malformed calls, zero-edit runs, grading failures, and pressure
  aborts in the denominator.
- Require one verified real-bug repair under normal pressure before spending a screen budget.
- Promote quality only after at least two additional verified task-seed repairs without unacceptable
  pressure.
- Promote speed only with no lost verified repair and either a paired end-to-end improvement of at least
  15%, at least 512 MiB lower measured footprint, or repeatable paging eliminated.
- Treat a model-card benchmark, a health response, a raw answer, or a source-pattern match as
  insufficient evidence.

## Preparation status

Preparation code now includes per-phase memory sampling, kernel pressure and swapout guards, managed
server ownership checks, native/recovered tool-call separation, external grading, input-lock hashes,
resumable execution IDs that preserve interrupted worktrees, and the two opt-in oracle-free Gemma
harness controls described above. Static syntax/format/whitespace checks,
fixture verification, Go tests, model-server startup, warm smoke, live sampling, and the first
capability-gate stop have been recorded for the historical Qwen3.6 work. Scored repair quality, a
memory-valid gate, live resume, and benchmark measurements for the current candidates remain pending
under the runner's persistent safety rules.

The earlier prepared llama session below was generated after the prior runner, Go recovery, experiment,
and fixture changes. The earlier Gemma session
`/Users/megasoft78/Desktop/Freelance/anvil-agent/.optimization-results/20260915T063334836Z-b401994db269`
is retained as restricted-shell bind evidence. A newer Gemma session has been prepared from the
current static-checked CLI:
`/Users/megasoft78/Desktop/Freelance/anvil-agent/.optimization-results/20260915T071413108Z-b401994db269`.
Its model size, SHA-256, CLI hash, and experiment lock include the deferred harness arms. The current Qwen3.8 comparator has
also been prepared at
`/Users/megasoft78/Desktop/Freelance/anvil-agent/.optimization-results/20260915T063004660Z-b401994db269`.
Its model hash and input lock match the final local GGUF, Go CLI, runner, fixtures, and local holdout
trees. The five main fixtures plus the Zod holdout passed; the Undici holdout remains pending because
the current restricted shell cannot bind its localhost test server. A same-shell Qwen3.8 startup attempt
also failed before health on the server bind, so server startup, warm smoke, live memory, native gates,
repairs, and all scored measurements remain pending for this session. If any runner, Go binary,
experiment, fixture, model, or server input changes, prepare a new session so its manifest hashes the
final inputs before execution.

## Primary sources

- [Qwen3.6-35B-A3B official model card](https://huggingface.co/Qwen/Qwen3.6-35B-A3B)
- [Qwen3.5-9B official model card](https://huggingface.co/Qwen/Qwen3.5-9B)
- [Qwen3.5-4B official model card](https://huggingface.co/Qwen/Qwen3.5-4B)
- [Qwen3.5-9B MLX 4-bit files](https://huggingface.co/mlx-community/Qwen3.5-9B-MLX-4bit/tree/938d8919941c6e7efd3c7150eff7fe9d12afa631)
- [Qwen3.5-4B MLX 4-bit files](https://huggingface.co/mlx-community/Qwen3.5-4B-4bit/tree/main)
- [Gemma 4 core documentation](https://ai.google.dev/gemma/docs/core)
- [Gemma 4 function calling](https://ai.google.dev/gemma/docs/capabilities/text/function-calling-gemma4)
- [Gemma 4 QAT Q4 collection](https://huggingface.co/collections/google/gemma-4-qat-q4_0)
- [Gemma 4 E2B QAT Q4 GGUF](https://huggingface.co/google/gemma-4-E2B-it-qat-q4_0-gguf)
- [Gemma 4 E4B QAT Q4 GGUF](https://huggingface.co/google/gemma-4-E4B-it-qat-q4_0-gguf)
- [llama.cpp server documentation](https://github.com/ggml-org/llama.cpp/blob/master/tools/server/README.md)
- [llama.cpp parser development notes](https://github.com/ggml-org/llama.cpp/blob/master/docs/development/parsing.md)
- [llama.cpp Qwen3.6 parser report #26043](https://github.com/ggml-org/llama.cpp/issues/26043)
- [llama.cpp Qwen3.6 malformed-parameter report #24807](https://github.com/ggml-org/llama.cpp/issues/24807)
- [llama.cpp Qwen3.6 parameter-suffix report #26763](https://github.com/ggml-org/llama.cpp/issues/26763)
- [MLX-LM server documentation](https://github.com/ml-explore/mlx-lm/blob/main/mlx_lm/SERVER.md)
- [MLX-LM Qwen parser source](https://raw.githubusercontent.com/ml-explore/mlx-lm/main/mlx_lm/tool_parsers/qwen3_coder.py)
- [MLX-LM server source](https://raw.githubusercontent.com/ml-explore/mlx-lm/main/mlx_lm/server.py)
- [TurboQuant paper](https://arxiv.org/abs/2504.19874)
- [TurboQuant llama.cpp discussion](https://github.com/ggml-org/llama.cpp/discussions/20969)
- [Microsoft BitNet repository](https://github.com/microsoft/BitNet)
- [ST-MoE: A Spatio-Temporal Expert Prefetching Framework](https://arxiv.org/abs/2606.15453)
- [Patterns Behind Chaos: Forecasting Data Movement](https://arxiv.org/abs/2510.05497)
- [Speculating Experts](https://arxiv.org/abs/2603.19289)
- [Pre-Attention Expert Prediction and Prefetching](https://arxiv.org/abs/2511.10676)
- [MAIO/PPC programmable page cache](https://www.usenix.org/conference/fast26/presentation/liu-yubo)
- [LLM in a Flash](https://machinelearning.apple.com/research/efficient-large-language)
- [SpecMD expert prefetching and cache policy](https://arxiv.org/abs/2602.03921)
- [streamlx](https://github.com/srcterm/streamlx), [Qwen3.6 colibri](https://github.com/RuiRDA/colibri-qwen3.6-35B-A3B),
  [edgeMoE](https://github.com/supastishn/edgeMoE), and [mbolt](https://github.com/doramirdor/mbolt)
- [Hebrus](https://github.com/hebrus-labs/hebrus) and its [Qwen3.6 M1 Pro benchmark](https://github.com/hebrus-labs/hebrus/blob/main/docs/benchmarks/2026-07-29-qwen-m1-pro-16g-main.md)
- [Qwisp](https://github.com/penta2himajin/qwisp), its [results](https://github.com/penta2himajin/qwisp/blob/main/bench/RESULTS.md), and [base-M1 reports #55](https://github.com/penta2himajin/qwisp/issues/55) and [#67](https://github.com/penta2himajin/qwisp/issues/67)
- [TurboQuant-MLX Qwen3.6 streaming](https://github.com/nathannorthcutt/turboquant-mlx#qwen36-35b-a3b-on-a-16-gb-mac-mini-expert-streaming) and [Qwen3.6 TQ3 artifact](https://huggingface.co/manjunathshiva/Qwen3.6-35B-A3B-tq3-g32)
- [Mference](https://github.com/NeelM0906/Mference), [Swiftlet](https://github.com/leonickson1/Swiftlet), [TensorFold](https://github.com/ashhart/TensorFold), [BigRig](https://github.com/arjvnv/BigRig), and [Sparsify](https://github.com/daylinkltd/sparsify)
- [anemll-flash-mlx](https://github.com/Anemll/anemll-flash-mlx), [ExpertCache](https://github.com/amos-labs/expertcache), and [Orbit's Qwen3.6 compatibility notes](https://github.com/guelfoweb/orbit/blob/main/docs/QWEN_3_6_COMPATIBILITY.md)
- [NeuroPrefetcher](https://arxiv.org/abs/2608.22643), [SpecPrefetch](https://arxiv.org/abs/2607.24787), [SP-MoE](https://arxiv.org/abs/2510.10302), and [MLX-LM hybrid n-gram proposal](https://github.com/ml-explore/mlx-lm/issues/1497)
