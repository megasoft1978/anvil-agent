# Evidence for Qwen optimization on the M1

The best first investment is a reproducible tool-calling evaluation, explicit sampling, and an investigation of prompt-cache invalidation. These changes can improve the useful work obtained from existing weights without immediately spending more RAM. Larger quants and speculative decoding remain credible experiments, but their value depends on the measured bottleneck. [The execution handoff](OPTIMIZATION-HANDOFF.md) gives the ordered trials, budgets, and promotion rules. The dated [2026 hardware and engine update](OPTIMIZATION-RESEARCH-2026-09-14.md) adds the current predictive expert-loading, SSD-streaming, and page-cache research.

## Evidence boundaries

Local inspection on 2026-09-13 confirmed Apple M1, 17,179,869,184 physical bytes, 8 CPU cores, macOS 26.6.2, and llama-server b10809 (`5266f24da`). There was no model server running. Swap occupancy was 1,573.44 MiB; `vm_stat` reported 16,384-byte pages and substantial compressed memory. This is an idle-state snapshot, not a measurement of model demand. Apple defines memory pressure using multiple factors including swap rate and wired/cache memory; free pages alone do not decide feasibility. [Apple memory documentation](https://support.apple.com/guide/activity-monitor/view-memory-usage-actmntr1004/mac).[^1]

The existing model file is 11,522,702,304 bytes, about 10.73 GiB. Shipped server flags select Metal offload, 24,576 context, one slot, flash attention, ngram-simple speculation, reasoning off, and 256 logical/physical batch sizes. Historical comments record a cache-related improvement to 16 turns in 245 seconds, with roughly 11.2 GB server RSS. These are earlier observations, not a fresh baseline. The previous cleanup shortened `TESTING.md`, leaving some historical references without their original evidence; use raw trace/config hashes before relying on old runs.

The original preparation pass did not load the pinned GGUF or execute the ordered llama.cpp benchmark. A later, separately labeled TurboQuant feasibility trial downloaded a different checkpoint and used its custom MLX server; those observations are not baseline or ordered-experiment scores. Local source inspection identifies implementation facts; upstream issue reports establish relevant mechanisms, not proof that this installation currently suffers from them.

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

The six local pilot tasks exist, and the baseline verifier now selects manifest tasks, honors each
task's declared command/source set, and accepts both the short and test-file-prefixed names emitted
by Vitest. The selected prepared baselines pass. Two-step experiments use direct `runAgent` calls and
hardcoded greedy sampling, so they are not interchangeable with the production CLI. Local references:
[grader](../benchmarks/grade.mjs), [live runner](../go-agent/live_test.go),
[pilot preparation](../go-agent/realrepo_test.go), [two-step experiment](../go-agent/live_twostep_test.go).

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

The controlled 16 GiB preflight therefore matters: reboot the Mac mini to clear prior swap/compression
state and close every other application, leaving only the terminal that runs the approved batch. This
can recover host headroom but cannot change the model-weight allocation. The first measured 16,384-token
rescue on this checkout still reached critical pressure with roughly 10.51 GB server RSS, 14.53 GB
wired memory, 54 MB free memory, and 1.26 GB swap used. A health endpoint response is not sufficient
evidence of safe inference. A separate finalist check may measure normal use with everyday apps open.

Gemma 4 provides a useful comparison, but not a portable Qwen fix. Its earlier memory tuning passed
`--ctx-checkpoints 0 --cache-ram 0` and reduced long-session dirty memory from about 4.87 GB to 1.02 GB;
the same setup used `-ub 256 -b 256`. Qwen's current setup leaves those two flags at their defaults
because an earlier Qwen run with them disabled discarded the whole prompt-prefix cache during long
tool conversations, causing full re-prefills and fewer completed turns. A Qwen memory-first variant may
still be worth a bounded experiment, but it must measure cache reuse, prefill time, completion, and
memory together. It is not an established optimization.

Other bounded alternatives target different allocations. Lowering `-b` and `-ub` can shrink temporary
prompt-processing buffers, although Gemma's measured 128 versus 256 change saved only about 4 MB.
Disabling n-gram speculation may remove a small draft-state allocation but can reduce generation speed.
The q8 KV type can save part of the attention cache, estimated at hundreds of MiB at most for this
architecture, and may have a backend or quality cost. An 8,192-token context is another possible arm,
but the 16,384-token result already left wired memory nearly unchanged and a shorter context may not
hold a complete tool conversation. Read/search output caps reduce later prompt growth but cannot fix
startup pressure. A smaller quant or a host with 24 GiB or more is the most direct way to create real
headroom; either changes the current 16 GiB benchmark constraint.

The benchmark's useful outcome is real issue repair, not a passing health check or a tiny one-file smoke
task. Any context-saving approach must still leave room for repository navigation, source evidence,
multiple tool results, and a complete edit and verification loop. If it only makes the server boot by
excluding that context, it has not solved the target workload.

## Low-bit variants found during the memory review

The likely “tri-state” technique is ternary or trit quantization: expert weights use the three-value
codebook `{-c, 0, +c}` and pack base-3 indices. A Hugging Face TurboQuant build,
`manjunathshiva/Qwen3.6-35B-A3B-tq3a-tqTe-g64`, is 9.4 GB and reports fully resident 16 GB Mac
operation, but its own agent test failed the same multi-step tool task 4/4 times. It is unsuitable for
the real-bug benchmark despite its attractive memory footprint. [Ternary model card](https://huggingface.co/manjunathshiva/Qwen3.6-35B-A3B-tq3a-tqTe-g64)

The related `manjunathshiva/Qwen3.6-35B-A3B-tq3-g32` keeps 3-bit TurboQuant weights and reports a
successful tool-driven repair, with expert streaming and an 8 GB cache budget for 16 GB machines. It
requires the custom `turboquant-mlx-full` server/runtime, so it is a possible later backend comparison,
not a drop-in replacement for the pinned llama-server measurement. [3-bit model card](https://huggingface.co/manjunathshiva/Qwen3.6-35B-A3B-tq3-g32)

### Bounded TurboQuant feasibility trial

On 2026-09-13 the isolated `/tmp/anvil-turboquant-venv` environment installed
`turboquant-mlx-full 0.25.0`, `mlx 0.32.2`, and `mlx-lm 0.31.3`. The model card's header-only planner
reported 16.92 GB of total weights, 1.95 GB resident weights, an 11.34 GB projected working set at
8,192 context, and a `streaming` verdict for a 16 GiB machine. The checkpoint was then downloaded to
`/Users/megasoft78/.anvil-agent/models/Qwen3.6-35B-A3B-tq3-g32`; no conversion was performed.

The first server launch used the wrapper's automatic expert cache, which selected about 7.8 GB. A
single 949-token Go CLI prompt reached critical pressure before any model-selected tool call: free
memory fell to about 63 MiB, wired memory reached about 13.0 GiB, and swapouts increased by 34,280.
The request was cancelled and the server was stopped. This was a memory-feasibility failure, not a
quality result.

A second launch kept the validated top-4 routing but bounded the cache and I/O settings:

```sh
turboquant-serve \
  --model /Users/megasoft78/.anvil-agent/models/Qwen3.6-35B-A3B-tq3-g32 \
  --host 127.0.0.1 --port 8124 \
  --cache-budget-gb 2 --max-active-experts 4 \
  --prefetch-workers 1 --prefetch-ahead 0 --no-page-cache --no-hotlist \
  --prefill-step-size 128 --prompt-concurrency 1 \
  --prompt-cache-size 0 --prompt-cache-bytes 0 \
  --temp 0 --top-p 1 --min-p 0 --max-tokens 3072 \
  --chat-template-args '{"enable_thinking":false}'
```

With a temporary stable-tool-schema override for the Go harness, the actual CLI completed a disposable
read/edit task in 278.645 seconds: three turns, two native tool calls (`read`, `edit`), and one applied
edit. Manual memory snapshots showed no additional swapouts during this run; the run was stopped before
any test command. The evidence proves that the custom server can drive the existing Go CLI through a
real tool-call/edit loop under a 2 GB expert cache. It does not establish real-bug quality, pressure-free
long-context operation, or comparability with the pinned llama.cpp experiments. The stable schema was
needed for this trial because the dynamic close-out policy removed `edit` while the slow backend was
approaching its deadline; this should be evaluated as a separate harness/backend interaction, not folded
into the baseline score. [TurboQuant runtime](https://github.com/manjunathshiva/turboquant-mlx)

The first ordered low-cache screen exposed and then corrected a harness prompt issue: scenario reports
contained symptom bullets without an explicit repair instruction, so one completed response made zero
tool calls. That row remains preserved as an invalid prompt-path observation. With the repair wrapper and
declared relevant source paths supplied, the complete E4 screen ran four tasks at seeds 42 and 31415.
All eight scored attempts ended truncated, made zero edits, and verified zero repairs. CLI time ranged
from 192.081 to 300.034 seconds with a 245.656-second median. Peak server RSS was about 3.22 GiB,
wired memory about 11.09 GiB, compressed memory about 1.99 GiB, minimum free memory about 14 MiB,
pageout delta reached 4,763, swap-in delta 112, and swap-out delta remained zero. Every scored attempt
recorded warning pressure at least once. E4 therefore remains unpromoted and does not establish useful
real-bug performance. The subsequent E5 mixed K8/V3 KV profile was accepted by the runtime but failed
the warm tool-call gate with an unparsable truncated tool call, so it produced no scored task. E6 with
two prefetch workers was canceled by the critical-memory guard during warmup at about 4.14 GiB server
RSS, 12.27 GiB wired memory, 14 MiB minimum free memory, and swap-out delta 3,300. TQ-full and the
remaining behavioral arms remain pending under the handoff stopping rules.

For the current Go CLI and standard llama.cpp path, the same-architecture Unsloth ladder has smaller
drop-in candidates: `UD-IQ2_XXS` at about 10.8 GB and `UD-IQ1_M` at about 10 GB, compared with the
current 11.5 GB `UD-IQ2_M`. The `IQ1_M` card labels its quality extremely low, so `UD-IQ2_XXS` is
the first candidate worth a controlled comparison if a smaller file is approved. [Unsloth file list](https://huggingface.co/unsloth/Qwen3.6-35B-A3B-GGUF/tree/main)

Other new-looking options are not immediate solutions. APEX uses adaptive precision across MoE expert
roles, but its smallest listed I-Mini file is about 14 GB and its results are from a 122 GB NVIDIA
machine. The 128-expert-pruned Qwen build is smaller, but changes the architecture and is explicitly an
experimental artifact. TurboQuant KV compression reduces long-context cache memory only; it does not
reduce the model-weight residency that caused this startup pressure. [APEX model card](https://huggingface.co/mudler/Qwen3.6-35B-A3B-APEX-GGUF), [pruned model card](https://huggingface.co/xero0000/Qwen3.6-35B-A3B-128E-Pruned-GGUF), [TurboQuant KV notes](https://huggingface.co/majentik/Qwen3.6-35B-A3B-TurboQuant)

## Documented TurboQuant-MLX expert-streaming setup for a 16 GB M1

This is a separate documented setup for the target machine: a Mac mini M1 with 16 GB unified
memory, Docker, and multiple Zellij sessions. Qwen3.6-35B-A3B is a Mixture-of-Experts model with
35B total parameters and 3B active per token; it scores 73.4% on SWE-Bench Verified. Standard Q4
needs roughly 20.5 GB resident, so it does not fit in 16 GB unified memory.

TurboQuant-MLX expert streaming keeps the model under 4 GB resident on a 16 GB Mac mini by paging
only router-selected experts from SSD per token. Its output is bit-identical to fully resident
inference. The local model is complementary to a hosted frontier model: use it for inline
autocomplete, single-file refactors, commit messages, and offline work. The 16K context limit makes
multi-file agentic loops impractical at this tier.

### Step 1: quantize with TurboQuant-MLX

Repository: [TurboQuant-MLX](https://github.com/manjunathshiva/turboquant-mlx). Package:
`turboquant-mlx-full`.

TurboQuant-MLX is an MLX implementation of Google's TurboQuant: calibration-free Hadamard rotation
combined with Lloyd-Max codebooks. It supports dense models (LLaMA, Qwen, Mistral), MoE models
(GPT-OSS, Qwen-MoE, DeepSeek-V2/V3), and hybrids, including Qwen3.6's mixed linear/softmax
attention. It also compresses the KV cache by roughly 3.8x. On GPT-OSS-120B, the smaller KV cache
increased generation speed from 6.4 to 8.7 tok/s because the memory-bandwidth saving outweighed
decompression; this is relevant to an M1 with 68 GB/s memory bandwidth.

```bash
pip install turboquant-mlx-full

python -m turboquant_mlx.convert \
    --hf-path Qwen/Qwen3.6-35B-A3B \
    --mlx-path ./qwen36-35b-tq3 \
    --bits 3 --group-size 64
```

### Step 2: serve an OpenAI-compatible endpoint

The serving path is [serve-mlx](https://github.com/IDAH-BITBOX/serve-mlx), package
`mlx-moe-stream`. It supports function calling and structured JSON output, so Aider, OpenCode, and
llama.vscode can point at it directly.

The documented 16 GB configuration is:

```bash
mlx-moe-stream serve \
  --manifest prepared-qwen3.6-35b/manifest.json \
  --resident-budget off \
  --memory-safety-margin auto \
  --scratch-reserve 2GiB \
  --kv-cache 4bit \
  --max-prompt-tokens 16384 \
  --max-tokens 128 \
  --prefill-step-size 256
```

`--resident-budget` also accepts an explicit size such as `2GiB`. Use `--resident-budget 2GiB`
when Docker containers are running so the expert cache cannot grow into memory they need. With these
settings, peak MLX allocation measured 12.86 GiB on a 24 GB M4.

Run the [TurboQuant-MLX benchmark script](../benchmarks/optimization/benchmark-turboquant-mlx.py)
once on an idle machine and once with the usual Docker containers running. Start the documented
server command in each environment, retain its PID, and point the script at the server's
OpenAI-compatible `/v1/chat/completions` endpoint:

```bash
ENDPOINT='<OpenAI-compatible-chat-completions-endpoint>'
SERVER_PID='<server-process-id>'
PROMPT='<benchmark-prompt>'

python3 benchmarks/optimization/benchmark-turboquant-mlx.py \
  --endpoint "$ENDPOINT" \
  --server-pid "$SERVER_PID" \
  --label idle \
  --prompt "$PROMPT"
```

Repeat with `--label docker` and the explicit `--resident-budget 2GiB` server setting. The script
reports the exact response-token rate and peak server RSS. It refuses to estimate tokens/sec when
the endpoint does not return `usage.completion_tokens`.

### Alternative: llama.cpp with mmap

The alternative is llama.cpp with `--mmap` and the Unsloth UD-IQ3_XXS quant, about 13 GB on disk.
It measured 17.3 tok/s on a 16 GB M4 Mac mini, faster on that same machine than a dense 9B model at
12.6 tok/s because only 3B parameters are active per token. With mmap, however, the kernel decides
which pages to evict and competes with Docker's page cache, causing thrashing and unpredictable
latency under memory pressure. Use explicit expert streaming when Docker is running because its
resident budget is bounded and configurable.

### Other projects

- [SwiftLM](https://github.com/SharpAI/SwiftLM) is a native Swift MLX inference server: one binary,
  no Python, strict OpenAI compatibility, and roughly a 10x SSD expert-streaming speedup. It runs
  Qwen3.5-122B (69.6 GB) in about 10 GB resident on a 64 GB Mac and requires macOS 14+ with Apple
  Silicon M1 or newer. Its 2-bit quantization systematically breaks JSON grammars; stay at 4-bit
  when tool calling is required.
- [turbo-fieldfare](https://github.com/drumih/turbo-fieldfare) has about a 2 GB resident footprint,
  14.3 GB on disk, and a 16-slot LFU cache per layer. It supports only Gemma 4 26B-A4B and requires
  macOS 26 with Metal 4. Published measurements are 5.1–6.3 tok/s on an 8 GB M2 MacBook Air and
  31–35 tok/s on a 24 GB M5 Pro.

### What is not known

There are no published throughput figures for explicit expert streaming on an M1 with 16 GB. The
17.3 tok/s figure is an M4 using mmap, not an M1 using expert streaming. The M1 has lower memory
bandwidth (68 GB/s versus roughly 120 GB/s on the M4) and a slower SSD. Do not estimate the M1
throughput from those figures; run the benchmark twice, once idle and once with the usual Docker
containers running, because the second case is the one that matters.

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
