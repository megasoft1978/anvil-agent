<p align="center"><img src="assets/logo.png" width="140" alt="anvil-agent logo"></p>

# anvil-agent

**A local coding model, sized to fit your Mac, driven by this repo's own Go harness. One command to get a
server running; go-agent does the actual read/edit work.**

```
curl -fsSL https://raw.githubusercontent.com/megasoft1978/anvil-agent/main/setup.sh | bash
```

Checks your hardware, downloads the current target model (once), and starts a `llama-server` tuned for it.
No cloud API, no account, nothing leaves your machine once the model's on disk.

The [Go agent harness](go-agent/README.md) is what actually reads and edits against that server —
a small read/edit loop with configurable deadlines and JSONL traces. See its
[offline verification results](go-agent/TESTING.md) for what's been checked so far.

This project is currently tuned for the verified local winner, Qwen3-Coder-30B-A3B Q2_K (see
`go-agent/profiles.go`): its sampling defaults and server flags are kept together so the shipped
configuration stays reproducible. The fresh-install script uses the same 8K-context, low-reasoning
configuration that passed the strongest local repair screen.

## Benchmarks

Bugs fixed across 9 realistic multi-file projects (React + Express + TypeScript), reported the way you'd
actually describe them to a coding agent: by symptom, never by cause. Every model below was run
**agentically** — real Read/Edit tools, no command execution in the loop, same 9 scenarios, same
`benchmarks/grade.mjs` grader. It combines executable checks with source-pattern checks; the
optimization plan reports these separately.

| Model | Bugs fixed | Notes |
|---|---|---|
| **Qwen3.6-35B-A3B-UD-IQ2_M** (historical local configuration, no longer shipped — real `go-agent` binary, Read/Edit tools, no test feedback, 16-turn cap, n=1) | 20/44 (45%) | 7 of 9 scenarios hit the 16-turn cap without finishing (1 of those, `notify-channel`, never made a single edit); only 2 of 9 reached `"completed"` on their own |
| **claude-sonnet-5** (agentic, Claude Code, 2026-09-13) | 41/44 (93%) | avg ~77s wall-clock / ~61k tokens per scenario |
| **claude-opus-5** (agentic, Claude Code, 2026-09-13) | 43/44 (98%) | avg ~34s wall-clock / ~50k tokens per scenario |

The local model's biggest weakness here isn't understanding the bug — it's finishing: this quant tends to
re-read files instead of committing to an edit, and burns its 16-turn budget before wrapping up. That
matches the earlier measured runs documented in `go-agent/TESTING.md`
finding the same "oscillates, never commits" pattern on other tasks. n=1 per scenario here (an agentic run
costs far more than a single completion) — treat the exact number as directional, not a tight measurement.

Sonnet's 3 misses: `cart-checkout`'s discount-before-tax ordering, and 2 of `realtime-sync`'s 6 bugs (the
sender seeing its own broadcast edit, and the `opId`-collision case). Opus's 1 miss: that same
`opId`-collision case — the one bug neither model fixed.

These historical results were recorded on an Apple M1 Mac mini with 16 GiB unified memory.
Future measurements use complete CLI tool-calling runs, with verified repairs, elapsed time,
and peak memory reported together. See [the optimization handoff](docs/OPTIMIZATION-HANDOFF.md).

### Local model screen — paused 2026-09-20

The current local screen uses the same Go harness and external oracle on the
same Apple M1/16 GiB machine. Prefill/decode are llama.cpp-reported tokens per
second; quality is the oracle result; memory is peak server RSS and minimum
free memory. The measurements are representative runs rather than a perfectly
controlled benchmark because reasoning modes and tasks differ. The complete
ledger, including excluded models and immutable artifact identities, is in the
[local model summary](docs/LOCAL-MODEL-SUMMARY-2026-09-20.md) and [audit](docs/LOCAL-MODEL-AUDIT-2026-09-19.md).

| Model | GGUF size | Quality result | Prefill / decode | Wall | Peak RSS / min free |
|---|---:|---|---:|---:|---:|
| **Qwen3-Coder 30B A3B** | 11.259 GB | QS + job queue passed; guarded Bash passed | 120.5 / 17.0 tok/s | 148 s | 8.77 GiB / 55 MB |
| **Qwen3.6 27B A3B Coder** | 9.058 GB | Job queue passed 4/4; second task pending | 115.8 / 9.6 tok/s | 552 s | 8.43 GiB / 56 MB |
| **Granite 4.2 3B** | 2.244 GB | 2/3 native repair tasks passed | 187.1 / 18.9 tok/s | 476 s | 3.86 GiB / 58 MB |
| **Qwen3.5 4B** | 2.741 GB | Job queue passed; QS near-pass; Immer no edit | 175.6 / 12.6 tok/s | 202 s | 3.39 GiB / 64 MB |
| **Ministral 3 14B** | 8.240 GB | QS passed; job queue failed | 58.7 / 5.9 tok/s | 230 s | 9.21 GiB / 56 MB |
| **Ministral 3 8B** | 5.199 GB | QS near-pass; queue priority failed | 89.4 / 8.9 tok/s | 284 s | 6.20 GiB / 54 MB |
| **Bonsai 2 27B** | 7.206 GB | Medium QS repair verified; low/xhigh too slow | ~30–31 / 3.3–4.6 tok/s | 1,800 s cap | ~8.7–8.8 GiB / 54–55 MB |
| **Gemma 4 26B A4B** | 10.015 GB | Gates passed; scored repair pending | — | — | ~10.1 GiB / 57–61 MB |

The current practical baseline is Qwen3-Coder 30B A3B: it is the fastest model
with multiple verified repairs. The separate Qwen3.6-27B comparison also demonstrated
a complete multi-file repair, but at substantially higher latency. There are eight actual
GGUF weights remaining, totaling 55,960,893,536 bytes (52.118 GiB).

**Code-quality verdict, judged by Opus 5 itself** (given both models' diffs for 3 of the 9 scenarios,
told explicitly which set was its own output, asked to be self-critical rather than favor itself):
Opus 5's fixes were judged clearly better on two decisive points — `cart-checkout`'s stock-reservation
route only partially fixed a partial-reservation leak that Sonnet's fix left in place, and `realtime-sync`'s
socket `send()` call that Sonnet left able to throw mid-reconnect. Sonnet's code also renamed a `drain()`
function to no longer actually drain anything, a naming/behavior mismatch Opus avoided. Opus's fixes did
carry some real scope creep (exponential backoff, a monotonic clock helper) beyond what was asked.

**Was the historical 45% score just the 2-bit quantization?** Partly, but this isn't a clean isolation — Qwen3.6-35B-A3B
run here is both a much smaller model (3B active params per token, MoE) *and* quantized to ~2.5 bits/weight
(IQ2_M), versus Sonnet/Opus at full precision and far larger scale. 2-bit-class quantization is well-
documented to cause real, measurable quality loss on its own — so it's a plausible contributor to the gap —
but nothing here isolates quantization from model scale. An unquantized Qwen3.6-35B-A3B run (not available
locally) would be needed to actually separate those effects.

<details>
<summary><strong>Advanced options</strong></summary>

Passing a flag through a pipe needs `bash -s --` (otherwise bash reads the flag itself):

```
curl -fsSL <raw-url>/setup.sh | bash -s -- --doctor          # diagnose an existing install, read-only
curl -fsSL <raw-url>/setup.sh | bash -s -- --start-only       # (re)start the server with the validated flags
curl -fsSL <raw-url>/setup.sh | bash -s -- --upgrade          # reapply the current config + restart the server
curl -fsSL <raw-url>/uninstall.sh | bash                      # remove everything
```

`--doctor` is the first thing to run if something's wrong. `setup.sh --help` and `uninstall.sh --help` list
every other flag.

**Requirements:** Apple Silicon Mac (M1/M2/M3/M4), 16GB unified memory or more, ~13GB free disk, and
[Homebrew](https://brew.sh) + [Node.js](https://nodejs.org) if you don't already have them. Building go-agent
also needs [Go](https://go.dev). Rather not pipe a script into `bash`? Read `setup.sh` first — one
self-contained file, every command visible.

</details>
