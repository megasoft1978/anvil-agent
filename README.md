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

| | Score |
|---|---|
| Bugs fixed | **32/44 (73%)** |
| Generation speed | **22.5 tok/s** (8,959 tokens generated, 481.1s wall across all 9 scenarios) |
| Mean speculative-decoding acceptance | 55% |

Measured 2026-09-13 against the shipped config (Qwen3.6-35B-A3B-UD-IQ2_M) on an Apple M1 Mac mini, 16GB —
`setup.sh --benchmark` reproduces this on your own hardware (needs a full clone; the scenario data doesn't
fit in a single script). Runs in about 12GB total: ~10.7GB of model weights on disk, plus working memory
while the server runs.

| Chip | tokens/sec |
|---|---|
| M1 | **22.5 — measured** (mean across the full suite, shipped config) |
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
