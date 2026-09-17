## Current prepared session (September 15, 14:58 UTC)

The later [algorithm comparison plan](OPTIMIZATION-ALGORITHM-EXPERIMENTS.md) is integrated in a
separate configuration and research queue. It does not invalidate or replace this first-fit session.
Use the expanded matrix only through a new preparation; its deferred arms require its baseline
screen and do not authorize running while the clean-host preflight is unmet.

Fresh Gemma session: `.optimization-results/20260915T145811611Z-b401994db269`.
The rebuilt `/tmp/anvil-agent-opt` hash matches its manifest. The 9,777,197,536-byte model
hash and llama.cpp build 10809 / commit `5266f24da` match. The corrected dynamic-policy
configuration and cache-counter helper are included in the input lock. Both first-batch fixtures
(`immer-array-push-fix`, `notify-channel`) passed base/reference/invalid validation.
No model was loaded; capability gates and scored repairs remain pending. The latest host check
reported kernel warning pressure and 6,518.25 MiB occupied swap, so live execution is paused for
the clean-host preflight. After a reboot, recheck the temporary CLI and prepare again if missing.

For capacity planning, allow roughly 11–12 GiB available to the Gemma workload, including temporary
allocations; this is an estimate, not a proven fit threshold. Rechecking every memory sample in
session `20260915T080105644Z-b401994db269` gives a peak server RSS of 10,343,415,808 bytes
(9.63 GiB), correcting the older 10.28 GiB prose below. RSS does not capture every system/Metal
allocation and must not be added to system wired memory. Normal kernel pressure and absence of
sustained swap-out growth are authoritative; a low free-page count alone is not a failure.

## Resume-after-reboot checklist (2026-09-15)

Operator is rebooting and closing every other application to clear the stale wired/compressed memory
state below. After reboot, from a normal Terminal:

```sh
cd /Users/megasoft78/Desktop/Freelance/anvil-agent
free -h 2>/dev/null; vm_stat; sysctl vm.swapusage   # record the clean idle baseline first
CLI=/tmp/anvil-agent-opt
(cd go-agent && go build -o "$CLI" .)
node benchmarks/optimization/prepare-holdouts.mjs
node benchmarks/optimization/prepare-fixtures.mjs
node benchmarks/optimization/runner.mjs prepare --cli "$CLI" \
  --experiments benchmarks/optimization/experiments-gemma4-local.json
# note the printed session_dir, then:
SESSION=/Users/megasoft78/Desktop/Freelance/anvil-agent/.optimization-results/<new-timestamp>-<git12>
sh benchmarks/optimization/run-gemma4-first-batch.sh "$SESSION"
```

The `--slot-save-path` fix is already committed in `experiments-gemma4-local.json`, so the batch should
reach the scored `gate-read-only`/`gate-edit`/`gate-notify-write` gates and the first Immer repair
attempt without the 501 or the `warning`-pressure stop seen below, assuming the reboot actually reaches
`normal` pressure. If it again reports `warning` or worse at idle, stop before running the batch and
re-check for other memory-heavy processes.

## Gemma4 attnQ4K batch attempts on unclean host (2026-09-15)

Two fresh Gemma4 attnQ4K sessions were run today without the mandated reboot/close-apps preflight
(operator accepted the risk explicitly for this batch only). Both used the freshly rebuilt
`/tmp/anvil-agent-opt` matching the current worktree.

- Session `.optimization-results/20260915T075759423Z-b401994db269`: fixtures re-validated, server
  loaded, warm smoke ran, but the slot-cache-clear step failed with `501 not_supported_error` because
  `common_flags` in `benchmarks/optimization/experiments-gemma4-local.json` never passed
  `--slot-save-path` to `llama-server`, so `/slots/0?action=erase` was disabled. This is a harness
  config bug, not a model/memory result. Fixed by adding `--slot-save-path
  /Users/megasoft78/.anvil-agent/slots/gemma4-26b` to that file's `common_flags`. The same gap likely
  exists in the other `experiments-*.json` files (none reference `slot-save-path`); unverified until
  each is actually run to that step.
- Session `.optimization-results/20260915T080105644Z-b401994db269` (after the fix): slot-cache clear
  worked, but `gate-read-only` failed with "gate memory pressure unsafe or unknown". `memory.jsonl`
  shows sustained `warning`-level pressure (level 2) through warm smoke and the gate, server RSS ~10.28
  GiB, free memory 12-14%, swap flat at ~208 MiB with zero swapout growth — no critical pressure or
  swap-out growth, but never reaching the `normal` pressure the gate requires. This is consistent with
  the unclean baseline (host was not rebooted, other apps/processes were running), not new evidence
  that the 9.78 GB Gemma file itself is infeasible on a clean 16 GiB host.
- Both server processes exited cleanly on stop; no crash, no leaked process, memory fully recovered
  afterward (verified via `ps`/`vm_stat`/`sysctl vm.swapusage`).
- Classification: inconclusive/host-contaminated, not memory-infeasible. Do not retry
  `GEMMA4-LOCAL-8K-REAL` again on this same unclean host state — repeat attempts without a reboot are
  expected to reproduce the same `warning`-pressure gate stop and burn time for no new evidence. Next
  attempt needs the actual operator preflight from the handoff: reboot, close every other application,
  record idle memory/swap, then rerun `run-gemma4-first-batch.sh` against a freshly prepared session.

# First inference batch result; next batch uses persistent safety guards

The earlier preparation checks passed for the recorded session, and the approved `LLAMA-IQ2-8K-REAL` batch was attempted on
2026-09-14. The managed llama.cpp server loaded the existing Qwen3.6 UD-IQ2_M GGUF, but the runner
stopped at the first live capability gate under the documented memory stopping rule. No scored repair
attempt ran, and no later experiment is approved by this result.

Preparation session:

`/Users/megasoft78/Desktop/Freelance/anvil-agent/.optimization-results/20260914T154835852Z-b401994db269`

The rebuilt `/tmp/anvil-agent-opt` matched the prepared CLI hash. The model and server were unchanged:
Qwen3.6 UD-IQ2_M is 11,522,702,304 bytes and llama.cpp is build 10809, commit `5266f24da`.

The live evidence is recorded in the session's [`runs.jsonl`](../.optimization-results/20260914T154835852Z-b401994db269/runs.jsonl),
[`memory.jsonl`](../.optimization-results/20260914T154835852Z-b401994db269/memory.jsonl), and
[`server.log`](../.optimization-results/20260914T154835852Z-b401994db269/server.log):

- Warm smoke completed one native tool call in 15.223 seconds.
- Startup reached 6.80 GiB server RSS, 13.06 GiB wired memory, 1.69 GiB compressed memory, and
  55.1 MiB minimum free memory.
- Kernel pressure became `warning` while the model loaded and stayed warning through the gate.
- Swap-out delta reached 30,716, with swap-in delta 138.
- `gate-read-only` was canceled after 2.514 seconds with zero tool calls. The server then exited
  cleanly; no scored repair or external grade was produced.

This classifies the existing Qwen3.6 GGUF plus llama.cpp 8K path as
`memory-infeasible/contaminated` for the recorded host state. The 16K probe, no-speculation screen,
and ngram screen must not be run from this result. The measurements do not prove that every possible
clean reboot state will fail, but they do satisfy the handoff's stop rule for this batch.

The fastest next branch is the already available Gemma 4 attnQ4K GGUF in the separate research
checkout, configured in [`experiments-gemma4-local.json`](../benchmarks/optimization/experiments-gemma4-local.json).
It avoids a new download while testing the same Go CLI and oracle. The prior project measured this file
at 9,777,197,536 bytes and recorded a SHA-256; its prior agentic probe used a different CLI and failed
to close edits, so that result is a useful prior rather than a score for this harness. A fresh session
must hash the file before inference. The Qwen3.5-9B MLX branch remains next after this signal; its
download helper and pinned artifact configuration are ready, but the current sandbox's Hugging Face
attempt was blocked by DNS before any file was written. A new batch can run after the model-free
preparation and clean-host memory preflight.

The command used for the completed approved batch was:

```sh
cd /Users/megasoft78/Desktop/Freelance/anvil-agent
sh benchmarks/optimization/run-first-batch.sh \
  /Users/megasoft78/Desktop/Freelance/anvil-agent/.optimization-results/20260914T154835852Z-b401994db269
```

Do not rerun it as an automatic retry. Any changed model, engine, configuration, prompt profile, or
host precondition needs a fresh prepared session and a separate batch invocation. Preserve the failed
session and its denominator entries.

## Earlier Gemma sandbox attempt

The fresh Gemma session is
`/Users/megasoft78/Desktop/Freelance/anvil-agent/.optimization-results/20260915T055111854Z-b401994db269`.
The selected Immer and notify fixtures passed, and the session input lock was rechecked successfully.
The managed start reached the llama.cpp process but stopped before health because this restricted Codex
shell denied the loopback bind with `EPERM` on `127.0.0.1:8116`. No warmup, inference, or scored repair
ran. The result is an environment limitation, not model evidence; see the session's
[`runs.jsonl`](../.optimization-results/20260915T055111854Z-b401994db269/runs.jsonl) and
[`server.log`](../.optimization-results/20260915T055111854Z-b401994db269/server.log).

This session predates the current runner input-lock fix and is retained only as environment evidence;
do not execute it. Use the fresh Gemma session in the preparation section below. The first batch has
one warm smoke, three native capability gates, and one scored Immer repair. It uses the 9.78 GB Gemma
file at 8K context. The expected duration remains roughly 10–25 minutes, with current memory pressure,
swap behavior, and repair quality unmeasured until a shell permits the server bind.

## Review prerequisite and scope clarification

The model-free preparation, fixture/oracle validation, and offline runner checks were completed for
the recorded earlier revision. The live server, warm smoke, memory sampler, cancellation acknowledgement,
and first capability-gate stop were exercised. Current-tree fixture/dynamic checks, live repair quality,
a memory-valid capability gate, slot-cache erase before an attempt, cancellation/resume of a scored
attempt, and all benchmark measurements remain pending.

The runner does not close applications, change swap or memory settings, or kill unrelated processes.
The downloader fetches only the explicitly named model and refuses to start when the disk reserve is
not available; it does not load or warm a model. A future memory-sensitive batch still requires the
operator's documented clean-host preflight and the runner's memory guards.

## Next candidate: local Gemma 4 preparation

The first available fallback uses the existing Gemma 4 26B-A4B `UD-IQ2_M-attnQ4K` file through the
same llama.cpp build and current Go CLI. The profile keeps the earlier memory-saving checkpoint and
RAM-cache settings (`--ctx-checkpoints 0 --cache-ram 0`) but turns speculation off for the fit probe,
so a successful result will establish native parsing, real-repository repair, and memory headroom
before any speed arm. The model path is outside this checkout and is read-only; no copy or download is
performed by `prepare`.

```sh
cd /Users/megasoft78/Desktop/Freelance/anvil-agent
CLI=/tmp/anvil-agent-opt
(cd go-agent && GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOMAXPROCS=2 go build -o "$CLI" .)
node benchmarks/optimization/runner.mjs prepare --cli "$CLI" \
  --experiments benchmarks/optimization/experiments-gemma4-local.json
```

The earlier Gemma session
`/Users/megasoft78/Desktop/Freelance/anvil-agent/.optimization-results/20260915T063334836Z-b401994db269`
was attempted from the restricted Codex shell and is retained as environment evidence: llama.cpp
could not bind `127.0.0.1:8116`, so it produced no model or repair measurement.

The latest prepared Gemma session is
`/Users/megasoft78/Desktop/Freelance/anvil-agent/.optimization-results/20260915T071413108Z-b401994db269`.
It uses the same pinned model and server plus two deferred, evidence-selected harness arms. Its model
hash and rebuilt CLI hash are recorded in the manifest; all live fixture checks, capability gates,
inference, repair quality, and memory measurements remain pending.

The current prepared session can be run directly from a normal Mac Terminal:

```sh
SESSION=/Users/megasoft78/Desktop/Freelance/anvil-agent/.optimization-results/20260915T071413108Z-b401994db269
CLI=/tmp/anvil-agent-opt-static-current
CLI="$CLI" sh benchmarks/optimization/run-gemma4-first-batch.sh "$SESSION"
```

The wrapper validates the fit fixtures before starting. If a reboot removed the temporary binary,
rebuild it and prepare a new session with the command above before running the batch; the manifest
locks the CLI hash.

If this Codex shell is the only environment available and loopback startup is denied, start the exact
Gemma server profile from a normal Terminal, keep its PID, and use the wrapper's external-server mode:

```sh
/opt/homebrew/bin/llama-server \
  -m /Users/megasoft78/Desktop/Freelance/llm-memory-wall-research/models/gguf/gemma4-26b-a4b/gemma-4-26B-A4B-it-UD-IQ2_M-attnQ4K.gguf \
  -ngl 99 -fa on --no-warmup -np 1 --reasoning off --jinja --offline \
  -ub 256 -b 256 --ctx-checkpoints 0 --cache-ram 0 -c 8192 \
  --spec-type none --host 127.0.0.1 --port 8116 &
SERVER_PID=$!
EXTERNAL_SERVER=1 SERVER_PID="$SERVER_PID" CLI="$CLI" \
  sh benchmarks/optimization/run-gemma4-first-batch.sh "$SESSION"
```

The runner samples the supplied server PID and never stops it in this mode. The same health,
fixture, native-tool, memory, cache, and stopping rules remain active. Stop the server afterward with
`kill "$SERVER_PID"`. Because the runner attaches after external model loading begins, this fallback
cannot capture the weight-allocation peak in `memory.jsonl`; use managed startup from a normal Terminal
for the authoritative memory-feasibility result, or record a separate startup preflight.

The batch contains one warm smoke, three native capability gates, and one scored
`immer-array-push-fix` attempt. It stops on critical pressure, three successive swapout-growth
samples, unknown pressure, malformed tool calls, or a failed gate. The prior Gemma study measured
roughly 1.02 GiB server footprint under a related synthetic workload, but that is not a prediction of
this Go harness; the new run's memory sampler is authoritative. The estimated first-batch duration is
10–25 minutes after validation, with a 180-second scored-attempt cap plus startup/grading uncertainty.

After the baseline produces a verified repair with normal pressure, the same prepared session contains
two separate, one-task harness probes. They must be run in separate server invocations, with the
baseline result preserved:

```sh
node benchmarks/optimization/runner.mjs run --session "$SESSION" --cli "$CLI" \
  --experiment GEMMA4-LOCAL-DYNAMIC-8K-REAL --start-server --evidence-approved --max-runs 1
node benchmarks/optimization/runner.mjs summarize --session "$SESSION"

node benchmarks/optimization/runner.mjs run --session "$SESSION" --cli "$CLI" \
  --experiment GEMMA4-LOCAL-NO-EDIT-RETRY-8K-REAL --start-server --evidence-approved --max-runs 1
node benchmarks/optimization/runner.mjs summarize --session "$SESSION"
```

The dynamic arm keeps only `edit` after five read/search calls until an edit is requested
(even an unsuccessful edit request resets that counter). It also enables the existing repeated-call
tool ban. Close-out reserve is disabled in both the stable baseline and this dynamic arm, so the
comparison changes only the tool-schema policy. This September 15 correction needs a fresh
prepared session; earlier session manifests preserve the old, confounded configuration. The
no-edit arm allows one oracle-free corrective user turn after a completed response with no changed
file. Neither arm receives hidden test output. These arms are preparation additions based on the
separate real-repository Gemma investigation; they are hypotheses, not expected improvements.

Run it only after a reboot, with every other application closed. The runner starts this named batch
without a conversational approval flag and retains its memory and server stop rules.

## Local Qwen3.8 comparator

The already resident dense Qwen3.8 GSQ-RCO IQ3_XXS file is prepared as a separate same-Go-CLI quality
comparator in `benchmarks/optimization/experiments-qwen38-local.json`. It is 10,094,357,632 bytes,
SHA-256 `fdfcb6a29b11188956dfbfd904223588a6c1b77eb250c3e8a36e1bd269df91f7`, and uses the pinned
llama.cpp build at 8K context. The first batch turns speculation off and contains the same warm smoke,
three native capability gates, and one scored Immer repair used by the other local candidates. The
no-speculation four-task screen follows only after a normal-pressure verified repair; ngram follows a
complete screen. Prior research suggests strong quality but only about 4.5–4.9 tok/s on this M1, so
this branch is a quality ceiling check rather than the expected speed winner.

The current fresh preparation session is
`/Users/megasoft78/Desktop/Freelance/anvil-agent/.optimization-results/20260915T063004660Z-b401994db269`.
The five main fixture contracts and the Zod holdout passed, and the model-free runner dry-run passed.
The Undici holdout remains pending because its reference oracle opens a localhost socket, which the
restricted Codex shell denies with `EPERM`; rerun validation from a normal Mac Terminal before any
conditional screen. The model server, capability gates, warm smoke, live memory samples, and scored
repair remain pending.

Prepare it with the current CLI:

```sh
cd /Users/megasoft78/Desktop/Freelance/anvil-agent
CLI=/tmp/anvil-agent-opt
(cd go-agent && GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOMAXPROCS=2 go build -o "$CLI" .)
node benchmarks/optimization/runner.mjs prepare --cli "$CLI" \
  --experiments benchmarks/optimization/experiments-qwen38-local.json
```

After `prepare` prints `SESSION`, run the fixture validation and fit probe from a normal Mac Terminal:

```sh
SESSION=/Users/megasoft78/Desktop/Freelance/anvil-agent/.optimization-results/20260915T063004660Z-b401994db269
CLI=/tmp/anvil-agent-opt-static-current
CLI="$CLI" sh benchmarks/optimization/run-qwen38-first-batch.sh "$SESSION"
```

The screen and ngram commands are resumable from the same session after the dependency gates pass:

```sh
SESSION=/Users/megasoft78/Desktop/Freelance/anvil-agent/.optimization-results/20260915T063004660Z-b401994db269
CLI=/tmp/anvil-agent-opt-static-current
node benchmarks/optimization/runner.mjs run --session "$SESSION" --cli "$CLI" \
  --experiment QWEN38-LOCAL-8K-SCREEN --start-server --evidence-approved
node benchmarks/optimization/runner.mjs summarize --session "$SESSION"

node benchmarks/optimization/runner.mjs run --session "$SESSION" --cli "$CLI" \
  --experiment QWEN38-LOCAL-8K-NGRAM --start-server --evidence-approved
node benchmarks/optimization/runner.mjs summarize --session "$SESSION"
```

The explicit `--evidence-approved` spelling is a dependency-selection flag for conditional experiment
arms; it is not a conversational approval prompt. The runner still requires the prior normal-pressure
verified repair and complete screen before it starts those arms.

## Following candidate: Qwen3.5-9B MLX preparation

The following smaller-model candidate uses the same Go CLI, fixture contract, external grader, tool schema,
repair prompt, and memory stopping rules as the closed Qwen3.6 probe. MLX-LM 0.31.2 is invoked through
its installed `mlx_lm.server` entry point on port 8115. The profile sets one prompt and decode worker,
512-token prefill steps, a 3,072-token server default, and `--prompt-cache-size 0` so the missing
llama.cpp slot-erase endpoint cannot contaminate task comparisons. The `fit-8192` name is a benchmark
budget label because this MLX-LM revision does not expose a hard context-size flag; prompt growth is
controlled by the existing Go agent budgets and bounded tool outputs.

Download and prepare, one candidate at a time:

```sh
cd /Users/megasoft78/Desktop/Freelance/anvil-agent
node benchmarks/optimization/download-model.mjs \
  --repo mlx-community/Qwen3.5-9B-MLX-4bit \
  --revision 938d8919941c6e7efd3c7150eff7fe9d12afa631 \
  --local-dir .optimization-models/Qwen3.5-9B-MLX-4bit \
  --expected-bytes 5980000000 \
  --reserve-bytes 4294967296 \
  --max-workers 4

CLI=/tmp/anvil-agent-opt
(cd go-agent && GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOMAXPROCS=2 go build -o "$CLI" .)
node benchmarks/optimization/runner.mjs prepare --cli "$CLI" \
  --experiments benchmarks/optimization/experiments-qwen35-mlx.json
```

The command printed by `prepare` supplies `SESSION`. The following validation is model-free but is
still an explicit fixture operation; until it runs, all fixture and model-dependent validations remain
pending:

```sh
node benchmarks/optimization/runner.mjs validate --session "$SESSION" --fetch-reference
```

The first candidate batch is one warm smoke, three native capability gates, and one scored
`immer-array-push-fix` attempt. It is the smallest useful measurement unit and must stop on critical
pressure, three successive samples with increasing swapouts, unknown pressure, malformed tool calls,
or a failed gate. The exact execution command is:

```sh
node benchmarks/optimization/runner.mjs run --session "$SESSION" \
  --cli "$CLI" --experiment MLX-QWEN35-9B-8K-REAL --start-server --max-runs 1
node benchmarks/optimization/runner.mjs summarize --session "$SESSION"
```

Run that command only after the clean reboot/closed-app preflight. A screen
(`MLX-QWEN35-9B-SCREEN`) is conditional on a normal-pressure verified repair and a separate evidence
selection; it must not be launched with the first candidate batch.
