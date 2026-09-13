<p align="center"><img src="assets/logo.png" width="140" alt="anvil-agent logo"></p>

# anvil-agent

**A local coding model, sized to fit your Mac, driven by this repo's own Go harness. One command to get a
server running; go-agent does the actual read/edit/test work.**

```
curl -fsSL https://raw.githubusercontent.com/megasoft1978/anvil-agent/main/setup.sh | bash
```

Checks your hardware, downloads the current target model (once), and starts a `llama-server` tuned for it.
No cloud API, no account, nothing leaves your machine once the model's on disk.

The [Go agent harness](go-agent/README.md) is what actually reads, edits, and tests against that server —
a small read/edit/test loop with configurable deadlines and JSONL traces. See its
[offline verification results](go-agent/TESTING.md) for what's been checked so far.

This kit is model-agnostic by design (see `go-agent/profiles.go`): the currently-targeted model, its sampling
defaults, and its server flags are one swappable configuration, not baked into the project's identity.

## Benchmarks

Bugs fixed across 9 realistic multi-file projects (React + Express + TypeScript), reported the way you'd
actually describe them to a coding agent: by symptom, never by cause.

Local model under test: **Qwen3.6-35B-A3B-UD-IQ2_M** (this repo's shipped local config — see
`go-agent/profiles.go`), single-shot completion via `bench.mjs` (no tools) — a lighter check than what
`go-agent` itself actually does (see "Single-shot vs. agentic" below). Sonnet 5 / Opus 5 ran the same
9 scenarios agentically (Read/Edit tools, real files) for comparison.

| Model | Bugs fixed | Notes |
|---|---|---|
| **Qwen3.6-35B-A3B-UD-IQ2_M** (local, shipped config, single-shot) | 32/44 (73%) | 22.5 tok/s, 8,959 tokens, 481.1s wall, 55% mean speculative-decode acceptance |
| **claude-sonnet-5** (agentic, Claude Code, 2026-09-13) | 41/44 (93%) | avg ~77s wall-clock / ~61k tokens per scenario |
| **claude-opus-5** (agentic, Claude Code, 2026-09-13) | 43/44 (98%) | avg ~34s wall-clock / ~50k tokens per scenario |

Sonnet's 3 misses: `cart-checkout`'s discount-before-tax ordering, and 2 of `realtime-sync`'s 6 bugs (the
sender seeing its own broadcast edit, and the `opId`-collision case). Opus's 1 miss: that same
`opId`-collision case — the one bug neither model fixed. Sonnet/Opus's per-scenario time/tokens include
full agentic tool use (reads, edits, re-reads) and aren't directly comparable to the local model's raw
decode tok/s — different measurement, not a faster/slower claim on its own (see below for why the local
number is single-shot in the first place).

Measured 2026-09-13 against the shipped config on an Apple M1 Mac mini, 16GB —
`setup.sh --benchmark` reproduces the local-model row on your own hardware (needs a full clone; the
scenario data doesn't fit in a single script). Runs in about 12GB total: ~10.7GB of model weights on disk,
plus working memory while the server runs.

| Chip | tokens/sec (Qwen3.6-35B-A3B-UD-IQ2_M, shipped config) |
|---|---|
| M1 | **22.5 — measured** (mean across the full suite) |
| M2 / M3 | ~33.1 — estimated |
| M2 Pro | ~66.2 — estimated |
| M3 Pro | ~49.6 — estimated |
| M1/M2/M3 Max | ~132.4 — estimated |
| M4 | ~39.7 — estimated |
| M4 Pro | ~90.3 — estimated |
| M4 Max | ~180.7 — estimated |

Non-M1 numbers are estimated by scaling the M1 measurement by each chip's published memory-bandwidth ratio
against M1's ~68GB/s — this harness is memory-bandwidth-bound (a MoE model reads a different slice of
weights per token, not compute-bound math), so tokens/sec tracks bandwidth roughly linearly. Not measured —
run `--report-speed` to contribute a real one.

Sonnet 5 and Opus 5 were graded the same way (same 9 scenarios, same bug reports, same
`benchmarks/grade.mjs` oracle) but run as an agentic Claude Code task (Read/Edit tools, real files on
disk) instead of one raw completion — treat the comparison as "what a coding agent driven by each model
scores here," not a clean apples-to-apples inference benchmark.

**Code-quality verdict, judged by Opus 5 itself** (given both models' diffs for 3 of the 9 scenarios,
told explicitly which set was its own output, asked to be self-critical rather than favor itself):
Opus 5's fixes were judged clearly better on two decisive points — `cart-checkout`'s stock-reservation
route only partially fixed a partial-reservation leak that Sonnet's fix left in place, and `realtime-sync`'s
socket `send()` call that Sonnet left able to throw mid-reconnect. Sonnet's code also renamed a `drain()`
function to no longer actually drain anything, a naming/behavior mismatch Opus avoided. Opus's fixes did
carry some real scope creep (exponential backoff, a monotonic clock helper) beyond what was asked.

**Is the 73% score just the 2-bit quantization?** Partly, but this isn't a clean isolation — Qwen3.6-35B-A3B
run here is both a much smaller model (3B active params per token, MoE) *and* quantized to ~2.5 bits/weight
(IQ2_M) *and* running single-shot with no tools, all at once, versus Sonnet/Opus running full agentic loops.
2-bit-class quantization is well-documented to cause real, measurable quality loss on its own — so it's a
plausible contributor to the gap — but nothing here isolates quantization from model scale or from the
tool-less single-shot setup. An unquantized Qwen3.6-35B-A3B run (not available locally) would be needed to
actually separate those effects.

<details>
<summary><strong>Advanced options</strong></summary>

Passing a flag through a pipe needs `bash -s --` (otherwise bash reads the flag itself):

```
curl -fsSL <raw-url>/setup.sh | bash -s -- --doctor          # diagnose an existing install, read-only
curl -fsSL <raw-url>/setup.sh | bash -s -- --start-only       # (re)start the server with the validated flags
curl -fsSL <raw-url>/setup.sh | bash -s -- --upgrade          # reapply the current config + restart the server
curl -fsSL <raw-url>/setup.sh | bash -s -- --report-speed     # measure real tokens/sec on a non-M1 chip
curl -fsSL <raw-url>/uninstall.sh | bash                      # remove everything
```

`--doctor` is the first thing to run if something's wrong. `setup.sh --help` and `uninstall.sh --help` list
every other flag.

**Requirements:** Apple Silicon Mac (M1/M2/M3/M4), 16GB unified memory or more, ~12GB free disk, and
[Homebrew](https://brew.sh) + [Node.js](https://nodejs.org) if you don't already have them. Building go-agent
also needs [Go](https://go.dev). Rather not pipe a script into `bash`? Read `setup.sh` first — one
self-contained file, every command visible.

</details>
