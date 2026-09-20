# Local model evaluation summary — 2026-09-20

Evaluation is paused at the user's request. This report records the evidence
available so far on an Apple M1 Mac mini with 16 GiB unified memory.

The harness used one model server at a time, the same Go agent, native
read/search/edit/write tools, seed 42, and an external fixture oracle. The
rows below use the cleanest representative run available for each retained
model. They are not a statistically controlled benchmark: task, reasoning
mode, context budget, and Bash availability differ between some rows.

## Retained models

`Prefill` and `decode` are llama.cpp-reported tokens/second. `RSS` is peak
server resident memory; `free` is the minimum observed free system memory.
Quality is determined by the external oracle, not by the model's final text.

| Model | GGUF size | Representative result | Quality | Prefill / decode | Wall time | Memory | Decision |
| --- | ---: | --- | --- | ---: | ---: | --- | --- |
| **Qwen3-Coder 30B A3B Q2_K** | 11.259 GB / 10.485 GiB | Native `job-queue` | 4/4 checks; also passed `qs-stringify-date-filter` | 120.5 / 17.0 tok/s | 148.0 s | 8.77 GiB RSS; 55 MB free | Current best validated breadth/speed |
| **Qwen3.6 27B A3B Coder IQ2_M** | 9.058 GB / 8.436 GiB | Native `job-queue`, medium reasoning | 4/4 checks; second task pending | 115.8 / 9.6 tok/s | 552.5 s | 8.43 GiB RSS; 56 MB free | Strong new pass, much slower |
| **Granite 4.2 3B Q4_K_M** | 2.244 GB / 2.090 GiB | Native `job-queue` | 4/4 checks; 2/3 native tasks passed overall | 187.1 / 18.9 tok/s | 476.1 s | 3.86 GiB RSS; 58 MB free | Keep; fast generation but high agent latency |
| **Qwen3.5 4B Q4_K_M** | 2.741 GB / 2.553 GiB | Native `job-queue` | 4/4 checks; QS near-pass; Immer made no edit | 175.6 / 12.6 tok/s | 201.5 s | 3.39 GiB RSS; 64 MB free | Keep; compact and useful |
| **Ministral 3 14B Q4_K_M** | 8.240 GB / 7.674 GiB | Native `qs-stringify-date-filter` | QS oracle passed; `job-queue` failed | 58.7 / 5.9 tok/s | 229.5 s | 9.21 GiB RSS; 56 MB free | Keep pending another task |
| **Ministral 3 8B Q4_K_M** | 5.199 GB / 4.842 GiB | Native `job-queue` | QS near-pass; queue priority check failed | 89.4 / 8.9 tok/s | 284.4 s | 6.20 GiB RSS; 54 MB free | Near-pass; mixed quality |
| **Bonsai 2 27B PQ2_0** | 7.206 GB / 6.711 GiB | Medium-reasoning QS repair | Verified QS repair; low/xhigh follow-ups mostly timed out before editing | ~30–31 / 3.3–4.6 tok/s* | 1,800 s caps* | ~8.7–8.8 GiB RSS; 54–55 MB free* | Keep; reasoning effort is decisive |
| **Gemma 4 26B A4B IQ2_M** | 10.015 GB / 9.327 GiB | Capability gates only | Read/edit/write gates passed; no scored repair yet | — | — | ~10.1 GiB RSS; 57–61 MB free | Keep for later scoring |

\* Bonsai's speed and memory values are from its low-effort follow-up screen;
the verified medium-reasoning repair predates the runner summary fields and did
not capture a directly comparable speed row.

Qwen3-Coder 30B also passed a clean guarded-Bash `job-queue` retest after the
harness redirected npm's cache outside the worktree: 8 calls, 138.3 s,
127.5/16.1 tok/s, and 8.43 GiB peak RSS. Bash-only mode failed to finish,
which supports keeping Bash as a guarded supplement rather than the primary
interface.

## Excluded models

The following were tested but are not part of the retained model set. Their
failures occurred at loading, native tool protocol, capability gates, or
repeated no-edit/truncation points, so a comparable prefill/decode/quality row
would be misleading. Immutable hashes and detailed reasons are in the
[model audit](LOCAL-MODEL-AUDIT-2026-09-19.md).

| Models | Exclusion reason |
| --- | --- |
| LFM2.5-8B, Mellum2-12B, ZAYA1-8B | Repeated no-tool/no-edit or unsupported endpoint behavior |
| Xing4.0-29B-A4B | Loader reached memory failure on the 16 GiB host |
| Qwen2.5-Coder 7B/14B, Qwen3 8B/14B | Native structured-tool protocol or warm-smoke failures |
| Granite 4.0 H-Tiny, Devstral Small 1.1 | Capability/protocol failure before a valid repair |
| Ministral 3 3B Instruct/Reasoning | Two independent partial/no-edit repair failures |
| DeepSeek-Coder-V2-Lite | Loaded, but failed native tool protocol under four template probes |

Their full artifacts were deleted only after the failure evidence and hashes
were recorded. The DeepSeek weight was the last excluded artifact removed.

## Disk inventory at pause

There are eight actual GGUF weight files remaining. Their logical sizes sum to
55,960,893,536 bytes (55.961 decimal GB, 52.118 GiB), and the model directory
occupies about 52G on disk. Jan was not part of this inventory; its unused
38-MB application data was moved to Trash at the user's request and contained
no model weights.

## Next session

Resume with a second scored repair for Qwen3.6, then compare the retained
models under a common task/mode before spending time on speed arms. Keep the
Qwen3-Coder 30B guarded-Bash configuration as the current practical baseline.
