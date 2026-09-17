# Qwen3.6 M1 benchmark: Hebrus vs TurboQuant-MLX

Date: 2026-09-16
Host: Apple M1, 16 GiB unified memory, internal SSD
Fixture: `immer-array-push-fix`, the smallest verified real-repository pilot

## Result

Neither runtime produced a verified repair patch, so no patch-quality score is
assigned. `N/A` is intentional: a missing or malformed edit is a capability
failure, not a zero-quality patch.

Hebrus is the practical recommendation from this screen. The verified
operational profile is the exact Qwen3.6-35B-A3B Stable Affine4 artifact with
SSD streaming, cold streaming, 321 cached experts, `ctx=8192`, and
`n=3072`. It stayed within roughly 1.08 GiB peak server RSS and 6.64 GiB
wired memory during the most useful coding attempt. The model still failed to
complete the repair because the agent emitted an invalid tool call or stopped
without editing.

TurboQuant's same-family `tq3-g32` artifact was verified through TurboQuant
MLX v0.6.1. A 1 GiB expert cache failed the memory-safety gate. A 0.25 GiB
retry passed the read-only gate without that rejection, but the edit gate
ended with a malformed/truncated native-tool completion and no source edit. It
is not recommended on this 16 GiB host under the tested load.

## Controlled setup

- The fixture base is Immer commit `a3be9df762c1dbe9959a011ddbab0ce838cbc468`.
  The reference fix is commit `d2c158f5bac7081a760bbaf501ea5c360b7856e1` and
  changes `src/plugins/arrayMethods.ts` from a numeric patch-key index to a
  string index.
- Both backends used the same disposable Go CLI binary:
  `613b237ae1d44cfba40f000c5767e0f7c536c0da16dab7edb743155df77a99f3`.
- The agent used the same native read/edit/search/write protocol, stable tool
  schema, seed `42`, temperature `0.7`, `top_p=0.8`, `top_k=20`,
  `presence_penalty=1`, `repeat_penalty=1`, 16-turn maximum, 3072-token
  maximum, and a five-minute task timeout. Prompt caching was disabled between
  attempts and verification was outside the model worktree.
- Docker was unavailable (`docker ps` could not connect). No separate loaded
  benchmark was claimed: the available Zellij sessions did not provide a
  verified loaded baseline.

## Baseline versus patched-runtime evidence

- The first stock Hebrus launch was preserved in
  `.optimization-results/20260916T200717294Z-b401994db269/`. It exited before
  inference because the M1 host-memory probe returned the shorter Darwin
  revision (`vm_count=62`) while the SDK constant required the padded count
  (`104`); the server reported that a host-memory snapshot was unavailable.
- The narrowly scoped probe change in
  `benchmarks/optimization/hebrus-m1-vmstats-revision.patch` was then applied
  only in the disposable Hebrus source tree. `make model-free-test` passed,
  the SSD server admitted the exact model, and all Hebrus coding evidence in
  this report is explicitly patched-runtime evidence. It changes admission
  observability, not model math or the agent protocol.
- No upstream TurboQuant source was modified. Its benchmark launcher is a
  disposable `/tmp` wrapper around the v0.6.1 `stream.loader` path and native
  `mlx_lm.server`; its cache-size variants are recorded separately rather than
  presented as stock upstream-server numbers.

## Runtime identities

### Hebrus

- Source commit: `7600fe183325b0c56fbdd8cc31120789724293ba`, with the small
  Darwin `vm_statistics64` revision probe fix recorded in
  `benchmarks/optimization/hebrus-m1-vmstats-revision.patch`.
- Binary SHA-256:
  `f58b34da807422c7f99af06e6d98cba5f650b628cd9b0469d6501b875bc9fbc6`.
- Model: `Qwen3.6-35B-A3B-Hebrus-ExpertMajor-v2-MLX-Affine4-G64.gguf`,
  20,808,566,880 bytes, SHA-256
  `dd17266185833a9f05531ce366fd7284ddca1ed64aa3dcf06e321e8c72c9ea3d`.
- Server command shape:
  `-m MODEL --ssd-streaming --ssd-streaming-cold --ssd-streaming-cache-experts 321 --ctx 8192 -n 3072 --host 127.0.0.1 --port 8126`.
- Raw source/build context is from the [Hebrus repository](https://github.com/hebrus-labs/hebrus).

### TurboQuant-MLX

- Source commit: `1f2fecf64aa3dca2e30f422a34b466e1dfc31db9` (`v0.6.1`).
- Model: official `manjunathshiva/Qwen3.6-35B-A3B-tq3-g32` snapshot
  `22d188ad7951b2b107fd84a332992e432dfc57a6`; four weight files total
  16,925,047,983 bytes. The prepared model tree hash is
  `4d74b7c5d9501cbe1bd6413272f38557d6314fea9990056db7aa62be353013ad`.
- The benchmark used a disposable wrapper around the supported streaming
  loader and native `mlx_lm.server`, not an invented model format. The wrapper
  logged prefill/decode rates and kept the streamed model cache reachable.
- Common server flags included prompt-cache size/bytes `0`, single prompt and
  decode concurrency, prefill step `128`, max tokens `3072`, and port `8127`.
  The tested cache values were `1` GiB and `0.25` GiB. The upstream project is
  [TurboQuant-MLX](https://github.com/manjunathshiva/turboquant-mlx).

## Measurements

Rates below are taken from the raw server logs. Request lengths grow as the
agent history grows, so the ranges are descriptive rather than a single
throughput score.

| Condition | Inference outcome | Prefill | Decode | Peak server RSS | Peak wired | Minimum free | Other evidence |
| --- | --- | ---: | ---: | ---: | ---: | ---: | --- |
| Hebrus, cache 321, focused prompt | 235.879 s, 5 tools, no edit | 83–176 t/s | 5.7–7.5 t/s | 1.08 GiB | 6.64 GiB | 56.4 MiB | pageout delta 1,691; no pressure abort |
| Hebrus, cache 321, force-after-3 | 157.024 s, 3 tools, protocol error | 83–176 t/s | 5.7–7.5 t/s | 1.06 GiB | 6.62 GiB | 54.6 MiB | invalid tool call; no edit |
| Hebrus, cache 321, force-after-4 | 188.543 s, 4 tools, protocol error | 83–175 t/s | 5.8–7.3 t/s | 1.07 GiB | 6.63 GiB | 53.9 MiB | invalid tool call; no edit |
| TurboQuant, cache 1 GiB | read-only gate rejected | 11.22–11.37 t/s | 3.02–3.08 t/s | 3.51 GiB | 13.97 GiB | 24.6 MiB | pressure unsafe; pageout 2,198; swap-in 1,132 |
| TurboQuant, cache 0.25 GiB | edit gate failed, empty/malformed completion | 11.32–11.63 t/s | 2.24–3.70 t/s | 2.76 GiB | 13.52 GiB | 14.1 MiB | pageout 2,329; swap-in 561; no edit |

The TurboQuant 1 GiB warm smoke also reached 13.94 GiB wired memory, about
3.47 GiB server RSS, 1.01 GiB swap used, and 31,624 swap-out pages. The
0.25 GiB run did not crash, but its 14 MiB minimum free-memory observation is
still too close to the floor for a coding recommendation.

## Agent counters

Prompt/completion counts are the CLI-reported totals for each isolated
attempt; the runner wall time includes its surrounding attempt bookkeeping.

- Hebrus cache 321 baseline: `28,258 / 563` tokens, 6 turns, 6 tools,
  255.279 s, truncated before a verified patch.
- Hebrus focused retry: `21,755 / 674` tokens, 6 turns, 5 tools, 235.858 s,
  HTTP 400 after no edit.
- Hebrus force-after-3: `13,394 / 463` tokens, 4 turns, 3 tools, 156.990 s,
  protocol error.
- Hebrus force-after-4: `20,415 / 402` tokens, 5 turns, 4 tools, 188.517 s,
  protocol error.
- TurboQuant cache 1 GiB warm smoke: `1,988 / 116` tokens, 2 turns, 1 tool,
  214.063 s; its read-only gate was `1,644 / 58`, 2 turns, 1 tool, 162.354 s,
  then failed closed on memory.
- TurboQuant cache 0.25 GiB warm smoke: `1,988 / 116` tokens, 2 turns, 1
  tool, 224.866 s; read-only gate `1,644 / 58`, 2 turns, 1 tool, 168.537 s;
  edit gate `2,011 / 170`, 2 turns, 1 tool, 249.119 s, empty completion.

## Repair-gate and coding evidence

- Hebrus cache 321 passed the native read/edit/write capability gates. The
  stable scored attempt explored for 235.879 s and produced no source edit.
  The force-after-3 and force-after-4 retries reached the dynamic edit window,
  then emitted a `search` call where only `edit` was valid; the runtime logged
  `invalid tool call`.
- TurboQuant cache 1 GiB completed warm smoke and a CLI read-only request, but
  the memory gate failed closed before coding.
- TurboQuant cache 0.25 GiB passed the read-only gate. The edit-gate trace
  contains one successful `read` call, followed by a second response with no
  tool call and finish reason `tool_calls`; the server logged
  `Failed to parse tool call (ValueError: No function provided.)`. No file was
  edited.
- Manual review found no generated fixture source diff to assess: the scored
  worktrees remained pristine, so oracle correctness, regression coverage,
  scope, maintainability, and explanation are all `N/A`, not inferred from a
  model's final text.
- Because no scored run reached a native source edit followed by pristine
  oracle verification, there is no patch to inspect or score from 0–10, and no
  later fixture was started.

## Raw artifacts

The runner preserved manifests, exact commands, model hashes, memory samples,
server logs, CLI traces, and gate decisions:

- Hebrus cache 321 baseline: `.optimization-results/20260916T202448239Z-b401994db269/`
- Hebrus focused retry: `.optimization-results/20260916T203541134Z-b401994db269/`
- Hebrus force-after-3: `.optimization-results/20260916T204457713Z-b401994db269/`
- Hebrus force-after-4: `.optimization-results/20260916T205353111Z-b401994db269/`
- TurboQuant cache 1 GiB: `.optimization-results/20260916T212553411Z-b401994db269/`
- TurboQuant cache 0.25 GiB: `.optimization-results/20260916T213502667Z-b401994db269/`

The fixture base/reference worktrees and the existing user changes were left
intact. The large Hebrus model artifact was removed from `/tmp` after its
hashes and raw results were preserved; the TurboQuant model remains available
for a future lower-load retry.
