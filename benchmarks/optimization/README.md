# Local model optimization runner

The [algorithm experiment guide](../../docs/OPTIMIZATION-ALGORITHM-EXPERIMENTS.md) adds a separate
Gemma matrix for ngram-simple, Q8 KV, bounded read/search output, prompt style and stall recovery.
Use `experiments-gemma4-algorithms.json` for a fresh session after selecting that comparison path.
`research-agenda.json` tracks prefix reuse, adaptive/suffix drafting, DFlash, other learned drafters,
diffusion models, expert prefetch and joint cache allocation with explicit preparation gates.
The existing first-fit session and defaults remain unchanged; do not execute the entire matrix.

This directory contains the preparation and execution harness for the local model screen. The
runner is local and resumable. Each scored attempt starts the
compiled Go CLI against a fresh worktree, so the model selects and executes native `read`, `search`,
`edit`, and `write` calls. Host verification runs in a separate copy after the CLI exits.

The default configuration is the verified current local winner, Qwen3-Coder-30B-A3B Q2_K, through
pinned `llama.cpp` in `experiments-local-model-screen-qwen3-coder.json`.
The fastest next fallback is the already screened local Gemma 4 IQ2_M GGUF in
`experiments-local-model-screen-gemma4.json`; the smaller-model fallback is prepared in
`experiments-qwen35-mlx.json`; it uses the installed MLX-LM 0.31.2 OpenAI-compatible server and a
local Qwen3.5-9B 4-bit Safetensors tree. The runner never
downloads model weights or starts a server during `prepare` or `validate`; server startup is explicit
with `run --start-server`. It records a manifest, one JSONL row per attempted run, per-turn
measurements, one-second memory samples, per-run artifacts, and compact `DECISIONS.md` notes under
`.optimization-results/`.

The dense Qwen3.8 GSQ-RCO IQ3_XXS comparator is retained as an archived configuration in
`experiments-qwen38-local.json`. Its 10.09 GB GGUF is no longer in the active model inventory;
the immutable hash and failure evidence are preserved in the dated Qwen3.8 report.

The preparation pass leaves model-dependent validation pending. These build and manifest steps are safe
to perform before memory is freed; they do not start a model server or run fixture checks. After a
session is prepared, run the model-free runner checks with:

```sh
cd /Users/megasoft78/Desktop/Freelance/anvil-agent
CLI=/tmp/anvil-agent-opt
(cd go-agent && go build -o "$CLI" .)
node benchmarks/optimization/prepare-holdouts.mjs
node benchmarks/optimization/prepare-fixtures.mjs
node benchmarks/optimization/runner.mjs prepare --cli "$CLI" --experiments benchmarks/optimization/experiments-local-model-screen-qwen3-coder.json
```

The local Gemma branch needs no model download. It still requires a fresh session and exact file-hash
preparation before the server can load it:

```sh
node benchmarks/optimization/runner.mjs prepare --cli "$CLI" \
  --experiments benchmarks/optimization/experiments-local-model-screen-gemma4.json
```

The prepared Gemma baseline is `GEMMA4-LOCAL-QUICK-REAL`. It is retained as a capability-gate
near-pass and should receive a scored repair only in a fresh session under normal memory pressure.

To prepare the local dense comparator instead, use the same built CLI with its separate configuration:

```sh
node benchmarks/optimization/runner.mjs prepare --cli "$CLI" \
  --experiments benchmarks/optimization/experiments-qwen38-local.json
```

After `prepare` prints a session directory, set `SESSION` to that path. The first documented Gemma batch
is one candidate only:

```sh
SESSION=/Users/megasoft78/Desktop/Freelance/anvil-agent/.optimization-results/<timestamp>-<git12>
sh benchmarks/optimization/run-gemma4-first-batch.sh "$SESSION"
```

The wrapper no longer requires a conversational approval flag. Preparation does not load the model,
start a server, or verify fixtures. The legacy `--approved` spelling remains accepted for old scripts.

This host's normal working condition runs close to its swap ceiling with ordinary applications open,
not a rebooted, app-free machine, and ambient system memory pressure from those applications is not
itself a contamination signal -- a run does not require closing other apps or a reboot. Live
vm_stat checks on 2026-09-17 measured reclaimable memory (free + speculative + inactive pages) at
2.9 and 3.67 GiB minutes apart under kernel pressure level 2 (warning) both times. What the memory
guard gates on is the model server's own dirty footprint (`footprint -p <pid>`, `server_footprint_bytes`
in `memory.jsonl`), checked against `policy.max_server_footprint_bytes` (default 3 GiB, deliberately
below that measured range; see `memory-gate.mjs`). The guard still stops on critical system-wide
pressure, an unavailable pressure reading, or three successive one-second samples with increased
swapouts, regardless of footprint.

To fetch the next smaller candidate after the local Gemma probe, use the isolated downloader after the read-only disk preflight.
It resumes a partial tree, requires a model-size plus reserve-space check, records the exact revision,
and never starts MLX or another server:

```sh
node benchmarks/optimization/download-model.mjs \
  --repo mlx-community/Qwen3.5-9B-MLX-4bit \
  --revision 938d8919941c6e7efd3c7150eff7fe9d12afa631 \
  --local-dir /Users/megasoft78/Desktop/Freelance/anvil-agent/.optimization-models/Qwen3.5-9B-MLX-4bit \
  --expected-bytes 5980000000 \
  --reserve-bytes 4294967296 \
  --max-workers 4
```

The file-list size is a mutable Hugging Face observation, while the full revision above identifies the
two-commit tree inspected on Hugging Face. `5,980,000,000` is a download preflight estimate rather
than an integrity check. After the tree is present, `prepare` hashes every
file in it and the later `run` command accepts only that prepared tree. The current sandbox attempted
this download and was blocked by DNS before any file or model process was created; leave the candidate
pending until the command succeeds in an environment with Hugging Face network access.

Prepare the MLX candidate in a fresh session after download:

```sh
node benchmarks/optimization/runner.mjs prepare --cli "$CLI" \
  --experiments benchmarks/optimization/experiments-qwen35-mlx.json
```

Set `SESSION` to the `session_dir` printed by `prepare`, then run the model-free runner checks:

```sh
SESSION=/Users/megasoft78/Desktop/Freelance/anvil-agent/.optimization-results/<timestamp>-<git12>
node benchmarks/optimization/offline-checks.mjs --session "$SESSION"
```

Then validate the fixtures used by the first screen, the capability gate, and the two reserved holdouts:

```sh
node benchmarks/optimization/runner.mjs validate --session "$SESSION" --task permissions-cache,job-queue,immer-array-push-fix,zod-int-json-schema,notify-channel,zod-memoizer-retention,qs-stringify-date-filter,zod-discriminated-union-defaulted-tags,immer-array-methods-noop-sharing,zod-prefault-undefined,undici-mockagent-global-fetch --fetch-reference
```

If validation passes, run the selected experiment in its documented order.
The runner performs the unscored warm smoke and capability gates first, then resumes any unfinished
attempts from the same session:

```sh
node benchmarks/optimization/runner.mjs run --session "$SESSION" --cli "$CLI" --experiment QWEN3-CODER-30B-A3B-LOCAL-QUICK-REAL --start-server
node benchmarks/optimization/runner.mjs summarize --session "$SESSION"
```

While a batch is running, poll a compact state view that does not read or print traces:

```sh
node benchmarks/optimization/runner.mjs status --session "$SESSION"
```

`QWEN3-CODER-30B-A3B-LOCAL-QUICK-REAL` is the current baseline measurement unit. It contains one
180-second real repair attempt,
an unscored warm smoke, and three unscored capability gates. Startup, fixture validation, and grading
remain part of the elapsed time. The local GGUF is about 11.26 GB on disk; peak RSS, wired/compressed
memory, swap, pressure, and pageout behavior are machine-state dependent and must be measured with
the host in its normal condition, not a freshly rebooted one. The memory guard stops after critical
pressure, three successive one-second samples with increased swapouts, or the server's own dirty
footprint exceeding `policy.max_server_footprint_bytes`.

Scenario fixtures contain concise symptom reports rather than a literal imperative. The runner
prepends a fixed repair instruction to every scored report so the model is asked to apply the
repair through native tools; it also lists only the relevant source paths already declared by the
fixture contract, without exposing reference patches. The original report is then passed unchanged.
Pilot reports keep the same wrapper for consistent task framing. A completed-but-zero-tool response
remains a failed attempt in the denominator.

Disk cleanup is optional and separate from RAM preflight. Run the read-only report before deleting
anything:

```sh
node benchmarks/optimization/disk-report.mjs
```

The unrelated FLUX Hugging Face cache has already been removed. Remaining cleanup candidates are old
completed optimization sessions after their summaries are retained, and regenerable Yarn, Go-build,
pnpm, and browser caches. Keep active model weights, isolated runtimes, prepared fixtures, and
rejected evidence while the screen is active; clearing disk caches does not lower resident model
memory.

The current results are summarized in [LOCAL-MODEL-SUMMARY-2026-09-20.md](../../docs/LOCAL-MODEL-SUMMARY-2026-09-20.md),
with immutable artifact and exclusion evidence in [LOCAL-MODEL-AUDIT-2026-09-19.md](../../docs/LOCAL-MODEL-AUDIT-2026-09-19.md).
Recent model-specific investigations are recorded in [OPTIMIZATION-BONSAI2-2026-09-18.md](../../docs/OPTIMIZATION-BONSAI2-2026-09-18.md)
and [OPTIMIZATION-QWEN38-2026-09-17.md](../../docs/OPTIMIZATION-QWEN38-2026-09-17.md).
Do not send raw JSONL traces unless a specific evidence gap requires them.

For a 16 GiB Mac mini, reboot immediately before a memory-sensitive batch, then close every other
application and leave only the terminal running the documented command. A browser, IDE, desktop client,
or other helper can consume the small amount of headroom needed by this model. Rebooting clears stale
swap/compression state but does not make the model smaller. The runner records the baseline and never
closes applications or changes system memory settings. A later finalist check with everyday apps open
is a separate product-usage measurement.

The runner must be launched from an environment that permits a loopback listener. Restricted Codex
shells can allow preparation and fixture validation while denying `bind(127.0.0.1, port)` or process and
kernel-memory inspection; a resulting server bind failure is an environment limitation, not a model
quality or memory result. Reuse the prepared session with the same command from a normal Mac Terminal.

If only server startup is blocked, the runner also supports an explicit external-server mode. Start the
documented server profile in a normal Terminal, retain its PID, and invoke `run` with
`--external-server --server-pid PID`. The runner samples that PID but never stops or replaces the
externally owned server:

```sh
/opt/homebrew/bin/llama-server \
  -m /Users/megasoft78/Desktop/Freelance/llm-memory-wall-research/models/gguf/gemma4-26b-a4b-base/gemma-4-26B-A4B-it-UD-IQ2_M.gguf \
  -ngl 99 -fa on --no-warmup -np 1 --reasoning off --jinja --offline \
  -ub 256 -b 256 --ctx-checkpoints 0 --cache-ram 0 -c 8192 \
  --spec-type none --host 127.0.0.1 --port 8123 &
SERVER_PID=$!
```

Then pass that PID to the prepared batch command. The Gemma wrapper accepts this mode as follows:

```sh
EXTERNAL_SERVER=1 SERVER_PID="$SERVER_PID" CLI="$CLI" \
  sh benchmarks/optimization/run-gemma4-first-batch.sh "$SESSION"
```

This mode still requires the same fixture validation, memory guard, native gates, cache handling, and
stopping rules; it only moves server ownership outside the runner. Stop the externally owned server
yourself after the batch with `kill "$SERVER_PID"`. Since the runner attaches after the external
process has started, its `memory.jsonl` cannot capture the peak during weight allocation; use managed
startup from a normal Terminal, or record a separate startup preflight, for decision-grade memory
feasibility.

Do not add evidence approval flags to a baseline experiment. Conditional experiments are blocked until
two unused real-issue holdouts are frozen in `fixture-contract.json`, validated in the current
session, and selected in `DECISIONS.md`. A server profile change requires a fresh invocation after
the previous server is stopped or restarted consistently. Rerunning the same session resumes
completed attempts and retries an interrupted active attempt from a fresh worktree.

For the MLX candidate, pending validations are the local model tree hash, MLX-LM runtime/version,
chat-template/tool parser gate, fixture base/reference/invalid oracle checks, memory-valid capability
gates, one verified repair, and all screen measurements. MLX-LM exposes no llama.cpp `/slots` erase
endpoint; the configuration sets `--prompt-cache-size 0` and serializes prompt/decode concurrency,
so the runner records cache drain as `not_applicable` rather than pretending it was cleared. No
benchmark, inference request, model warmup, fixture verification, or model-server startup belongs to
`prepare` itself.
